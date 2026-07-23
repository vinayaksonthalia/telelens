package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/findings"
	"github.com/vinayaksonthalia/telelens/internal/signozapi"
	"github.com/vinayaksonthalia/telelens/internal/store"
)

// UsageXref implements the "written but never read" analysis: it walks every
// dashboard panel and alert rule (GET only), collects every string that could
// reference a metric, and diffs that set against the metrics actually being
// stored. A stored metric referenced nowhere is pure waste.
//
// The walk is deliberately defensive: instead of binding to the
// compositeQuery schema (which shifts across SigNoz versions), it collects
// ALL string values anywhere in the JSON — over-approximating "used", which
// keeps false positives (calling a used metric unused) near zero at the cost
// of possibly missing some orphans. For a cost tool, that is the right bias.
func UsageXref(ctx context.Context, st store.TelemetryStore, api signozapi.SignozAPI, pricer analyze.Pricer) ([]findings.Finding, error) {
	dashboards, err := api.Dashboards(ctx)
	if err != nil {
		return nil, fmt.Errorf("usage xref: dashboards: %w", err)
	}
	rules, err := api.Rules(ctx)
	if err != nil {
		return nil, fmt.Errorf("usage xref: rules: %w", err)
	}

	referenced := map[string]bool{}
	var blobs []string
	for _, d := range append(dashboards, rules...) {
		collectStrings(d.Raw, referenced)
		blobs = append(blobs, string(d.Raw))
	}
	haystack := strings.Join(blobs, "\n")

	series, err := st.MetricCardinality(ctx)
	if err != nil {
		return nil, fmt.Errorf("usage xref: %w", err)
	}

	// Collect all orphans first, then report the expensive ones individually
	// and the long tail as ONE aggregate finding. A live scan of a real
	// instance found 91 unread metrics; 91 near-identical report rows drown
	// the signal, one row per notable offender + one roll-up does not.
	var orphans []store.MetricSeries
	for _, m := range series {
		if platformConsumed(m.MetricName) {
			// Consumed by SigNoz itself, invisible to the dashboards/rules
			// cross-reference — never flag (see platformConsumedPrefixes).
			continue
		}
		if isReferenced(m.MetricName, referenced, haystack) {
			continue
		}
		orphans = append(orphans, m)
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Samples > orphans[j].Samples })

	const individualMax = 5 // top orphans by sample volume get their own finding
	checked := fmt.Sprintf("checked %d dashboards and %d alert rules — zero references", len(dashboards), len(rules))

	var out []findings.Finding
	var tail []store.MetricSeries
	for i, m := range orphans {
		if i >= individualMax {
			tail = append(tail, m)
			continue
		}
		bytes := analyze.MetricBytes(m.Samples)
		gb := pricer.GBPerMonth(bytes)
		out = append(out, findings.Finding{
			Category: findings.CategoryMetrics,
			Severity: findings.SeverityHigh,
			Title:    fmt.Sprintf("Metric %q is written but read by no dashboard or alert", m.MetricName),
			Evidence: []string{fmt.Sprintf(
				"metric=%s series=%d samples_in_window=%d; %s",
				m.MetricName, m.Series, m.Samples, checked)},
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: fmt.Sprintf("Stop shipping %q: drop it at the collector (filter processor) or "+
				"remove the instrument. Caveat: telelens can only see SigNoz dashboards and alert rules — "+
				"if an external consumer (federated Prometheus, another pipeline) reads this metric, keep it "+
				"and delete its line from the generated filter block during review.",
				m.MetricName),
			Fix: findings.FixHint{Kind: findings.FixDropMetric, Metric: m.MetricName},
		})
	}
	if len(tail) > 0 {
		var tailBytes float64
		var names []string
		var totalSamples int64
		for _, m := range tail {
			tailBytes += analyze.MetricBytes(m.Samples)
			names = append(names, m.MetricName)
			totalSamples += m.Samples
		}
		gb := pricer.GBPerMonth(tailBytes)
		evidence := []string{
			fmt.Sprintf("%d further unread metrics totaling %d samples in window; %s", len(tail), totalSamples, checked),
			"full list: " + strings.Join(names, ", "),
		}
		out = append(out, findings.Finding{
			Category: findings.CategoryMetrics,
			Severity: findings.SeverityHigh,
			Title: fmt.Sprintf("%d more metrics are written but read by no dashboard or alert",
				len(tail)),
			Evidence:       evidence,
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: "Same story at lower volume: drop them at the collector, or delete the " +
				"instruments. The generated filter/drop_unread_metrics block lists every name " +
				"(it ships commented-out — verify external consumers, then uncomment).",
			Fix: findings.FixHint{
				Kind:    findings.FixDropMetric,
				Metric:  tail[0].MetricName,
				Metrics: names[1:],
			},
		})
	}
	return out, nil
}

// platformConsumedPrefixes lists metric-name prefixes that SigNoz's own
// UI/backend consumes directly (never via /api/v1/dashboards or /api/v1/rules,
// so the cross-reference structurally cannot see those reads). Flagging them
// as "unread" would generate a filter that blanks the APM Services page —
// exactly the trap the M4 live run caught by human review (LEARNINGS §the
// apply/revert story). These are hard-coded, not configurable: there is no
// situation where dropping them is a safe automated suggestion.
var platformConsumedPrefixes = []string{
	"signoz_",   // spanmetrics powering APM Services/Overview (signoz_calls_total, signoz_latency.*)
	"signoz.",   // signoz.meter.* usage metrics and other platform namespaces
	"otel.sdk.", // collector/SDK self-telemetry
	"otelcol_",  // collector self-metrics
}

// platformConsumed reports whether a metric is consumed by the SigNoz
// platform itself and must never be proposed for dropping.
func platformConsumed(name string) bool {
	for _, p := range platformConsumedPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// collectStrings walks arbitrary JSON and records every string value.
func collectStrings(raw json.RawMessage, into map[string]bool) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return
	}
	var walk func(any)
	walk = func(n any) {
		switch t := n.(type) {
		case string:
			into[t] = true
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
}

// isReferenced treats a metric as used if its name appears as any string
// value, or as a substring anywhere (covers PromQL / ClickHouse SQL panels
// where the name is embedded in a larger expression).
func isReferenced(metric string, exact map[string]bool, haystack string) bool {
	if exact[metric] {
		return true
	}
	return strings.Contains(haystack, metric)
}
