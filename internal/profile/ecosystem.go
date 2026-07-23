package profile

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/findings"
	"github.com/vinayaksonthalia/telelens/internal/store"
)

// Ecosystem attributes telemetry cost to the workload archetypes that
// dominate modern stacks — AI agents (gen_ai.* spans) and browser RUM
// (session.id spans) — and runs the cardinality-discipline check: unbounded
// ID attributes are fine on spans but must never become metric labels.
//
// These are attribution findings: they price what a workload's observability
// costs (so the bill has names on it), and they catch the one structural
// mistake (an ID used as a metric label) that no volume threshold can, because
// it is a time bomb at ANY current cardinality.

// idLikeLabels are attribute keys that are unbounded identifiers by
// construction (semantic conventions + common variants). Their presence as a
// METRIC LABEL is a structural cardinality bomb regardless of today's series
// count.
var idLikeLabels = []string{
	"session.id", "session_id",
	"user.id", "user_id", "enduser.id",
	"request.id", "request_id",
	"order.id", "order_id",
	"trace_id", "span_id",
	"transaction.id", "message_id",
}

func isIDLike(key string) bool {
	k := strings.ToLower(key)
	for _, id := range idLikeLabels {
		if k == id {
			return true
		}
	}
	return strings.HasSuffix(k, ".uuid") || strings.HasSuffix(k, "_uuid")
}

// Ecosystem profiles AI-agent and RUM telemetry cost plus label discipline.
func Ecosystem(ctx context.Context, st store.TelemetryStore, pricer analyze.Pricer) ([]findings.Finding, error) {
	var out []findings.Finding

	// --- AI / LLM telemetry attribution ---
	genai, err := st.GenAISpanVolume(ctx)
	if err != nil {
		return nil, fmt.Errorf("ecosystem profiler: %w", err)
	}
	for _, g := range genai {
		if g.Spans == 0 {
			continue
		}
		bytes := analyze.SpanBytes(g.Spans, g.AttrBytes)
		gb := pricer.GBPerMonth(bytes)
		out = append(out, findings.Finding{
			Category: findings.CategoryTraces,
			Severity: findings.SeverityLow,
			Title: fmt.Sprintf("AI-agent telemetry bill: %q ships %d gen_ai spans (%.0f tokens tracked)",
				g.Service, g.Spans, g.Tokens),
			Evidence: []string{fmt.Sprintf(
				"service=%s gen_ai spans=%d attr_bytes=%d tokens_tracked=%.0f — LLM telemetry per the gen_ai.* semantic conventions",
				g.Service, g.Spans, g.AttrBytes, g.Tokens)},
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: "Attribution, not waste: this is what observing your AI agent costs. " +
				"Compare it against the agent's own LLM spend to budget observability per investigation.",
			Fix: findings.FixHint{Kind: findings.FixNone, Service: g.Service},
		})
	}

	// --- Browser RUM telemetry attribution ---
	rum, err := st.RUMSpanVolume(ctx)
	if err != nil {
		return nil, fmt.Errorf("ecosystem profiler: %w", err)
	}
	for _, r := range rum {
		if r.Spans == 0 {
			continue
		}
		bytes := analyze.SpanBytes(r.Spans, r.AttrBytes)
		gb := pricer.GBPerMonth(bytes)
		perSession := ""
		if r.Sessions > 1 {
			perSession = fmt.Sprintf(" (%.1f KB/session uncompressed)", bytes/float64(r.Sessions)/1e3)
		}
		out = append(out, findings.Finding{
			Category: findings.CategoryTraces,
			Severity: findings.SeverityLow,
			Title: fmt.Sprintf("Browser RUM telemetry bill: %q ships %d session spans across %d sessions",
				r.Service, r.Spans, r.Sessions),
			Evidence: []string{fmt.Sprintf(
				"service=%s spans_with_session.id=%d sessions=%d attr_bytes=%d%s",
				r.Service, r.Spans, r.Sessions, r.AttrBytes, perSession)},
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: "Attribution, not waste: this is the frontend-observability bill. " +
				"Scale it by expected sessions/month before rollout; RUM SDK sampleRate is the knob.",
			Fix: findings.FixHint{Kind: findings.FixNone, Service: r.Service},
		})
	}

	// --- Cardinality-discipline check ---
	usage, err := st.MetricLabelUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("ecosystem profiler: %w", err)
	}
	// ID-like keys used as metric labels: structural bombs at any scale.
	byLabel := map[string][]store.MetricLabelUsage{}
	for _, u := range usage {
		byLabel[u.Label] = append(byLabel[u.Label], u)
	}
	var bombLabels []string
	for label := range byLabel {
		if isIDLike(label) {
			bombLabels = append(bombLabels, label)
		}
	}
	sort.Strings(bombLabels)
	for _, label := range bombLabels {
		metrics := byLabel[label]
		sort.Slice(metrics, func(i, j int) bool { return metrics[i].Series > metrics[j].Series })
		var names []string
		var evidence []string
		var totalSeries int64
		for _, m := range metrics {
			names = append(names, m.MetricName)
			totalSeries += m.Series
			evidence = append(evidence, fmt.Sprintf("metric %q carries label %q on %d series",
				m.MetricName, label, m.Series))
		}
		evidence = append(evidence, fmt.Sprintf(
			"an unbounded ID as a metric label multiplies series count by user/session/request count — "+
				"a structural time bomb at ANY current cardinality (today: %d series)", totalSeries))
		out = append(out, findings.Finding{
			Category: findings.CategoryMetrics,
			Severity: findings.SeverityHigh,
			Title: fmt.Sprintf("Unbounded ID %q is a metric label on %s",
				label, strings.Join(names, ", ")),
			Evidence: evidence,
			Remediation: fmt.Sprintf("Delete label %q from these metrics at the collector "+
				"(transform processor). Per-%s analysis belongs in traces/logs, where high "+
				"cardinality is free.", label, label),
			Fix: findings.FixHint{
				Kind:      findings.FixDropMetricLabel,
				Metric:    metrics[0].MetricName,
				Attribute: label,
			},
		})
	}

	// Positive finding: ID-like attributes present on spans but kept OFF all
	// metric labels — discipline held; say so (praise is information too).
	attrs, err := st.SpanAttributeCardinality(ctx)
	if err != nil {
		return nil, fmt.Errorf("ecosystem profiler: %w", err)
	}
	var disciplined []string
	for _, a := range attrs {
		if !isIDLike(a.Key) {
			continue
		}
		if _, onMetrics := byLabel[a.Key]; !onMetrics {
			disciplined = append(disciplined, fmt.Sprintf("%s (%d distinct values on spans)", a.Key, a.Cardinality))
		}
	}
	if len(disciplined) > 0 {
		out = append(out, findings.Finding{
			Category: findings.CategoryQuality,
			Severity: findings.SeverityLow,
			Title: fmt.Sprintf("Cardinality discipline held: %d unbounded ID attribute(s) stay off metric labels",
				len(disciplined)),
			Evidence: append(disciplined,
				"these IDs live on spans (where cardinality is free) and appear on NO metric series — exactly right"),
			Remediation: "Nothing to fix — keep it this way. This is the discipline the label-bomb findings ask for.",
			Fix:         findings.FixHint{Kind: findings.FixNone},
		})
	}

	return out, nil
}
