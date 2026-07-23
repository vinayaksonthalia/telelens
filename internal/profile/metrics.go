package profile

import (
	"context"
	"fmt"
	"time"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/findings"
	"github.com/vinayaksonthalia/telelens/internal/store"
)

// Thresholds for the metric profiler.
const (
	// SeriesCardinalityThreshold: a metric with more active series than this
	// is an explosion candidate.
	SeriesCardinalityThreshold = 10000
	// LabelBombShare: a single label whose cardinality explains at least this
	// fraction of a metric's series count is named as the bomb.
	LabelBombShare = 0.5
	// StaleAfter: a metric not written for this long (within the window) is
	// reported as stale.
	StaleAfter = 3 * 24 * time.Hour
)

// Metrics profiles series cardinality, identifies which label is the bomb,
// and flags stale series. now is injected for testability.
func Metrics(ctx context.Context, st store.TelemetryStore, pricer analyze.Pricer, now time.Time) ([]findings.Finding, error) {
	var out []findings.Finding

	series, err := st.MetricCardinality(ctx)
	if err != nil {
		return nil, fmt.Errorf("metric profiler: %w", err)
	}
	labels, err := st.MetricLabelCardinality(ctx)
	if err != nil {
		return nil, fmt.Errorf("metric profiler: %w", err)
	}
	// Index: metric → highest-cardinality label.
	topLabel := map[string]store.MetricLabel{}
	for _, l := range labels {
		if cur, ok := topLabel[l.MetricName]; !ok || l.Cardinality > cur.Cardinality {
			topLabel[l.MetricName] = l
		}
	}

	for _, m := range series {
		// Label explosion.
		if m.Series >= SeriesCardinalityThreshold {
			bomb, hasBomb := topLabel[m.MetricName]
			hasBomb = hasBomb && float64(bomb.Cardinality) >= LabelBombShare*float64(m.Series)
			// Dropping the bomb label collapses series (and future samples)
			// roughly by the label's cardinality factor.
			savedBytes := analyze.MetricBytes(m.Samples)
			if hasBomb {
				savedBytes *= 1 - 1/float64(bomb.Cardinality)
			} else {
				savedBytes *= 0.5 // directional without a single named cause
			}
			gb := pricer.GBPerMonth(savedBytes)
			f := findings.Finding{
				Category: findings.CategoryMetrics,
				Severity: findings.SeverityCritical,
				Title:    fmt.Sprintf("Metric %q has %d active time series", m.MetricName, m.Series),
				Evidence: []string{fmt.Sprintf(
					"metric=%s series=%d samples_in_window=%d", m.MetricName, m.Series, m.Samples)},
				EstGBPerMonth:  gb,
				EstUSDPerMonth: pricer.USDPerMonth(gb),
			}
			if hasBomb {
				f.Title = fmt.Sprintf("Label %q on %q explodes it to %d series",
					bomb.Label, m.MetricName, m.Series)
				f.Evidence = append(f.Evidence, fmt.Sprintf(
					"label %q alone has %d distinct values (%.0f%% of the series count) — an unbounded ID used as a label",
					bomb.Label, bomb.Cardinality, 100*float64(bomb.Cardinality)/float64(m.Series)))
				f.Remediation = fmt.Sprintf("Delete label %q at the collector (transform processor); "+
					"if per-%s analysis is needed, it belongs in traces or logs, not metric labels.",
					bomb.Label, bomb.Label)
				f.Fix = findings.FixHint{
					Kind:      findings.FixDropMetricLabel,
					Metric:    m.MetricName,
					Attribute: bomb.Label,
				}
			} else {
				f.Remediation = "Audit this metric's label set; aggregate away unbounded dimensions at the collector."
				f.Fix = findings.FixHint{Kind: findings.FixNone, Metric: m.MetricName}
			}
			out = append(out, f)
		}

		// Stale series: still stored, not written recently.
		if m.LastSeenUnix > 0 {
			last := time.Unix(m.LastSeenUnix, 0)
			if now.Sub(last) > StaleAfter {
				out = append(out, findings.Finding{
					Category: findings.CategoryMetrics,
					Severity: findings.SeverityLow,
					Title:    fmt.Sprintf("Metric %q went silent %s ago", m.MetricName, now.Sub(last).Round(time.Hour)),
					Evidence: []string{fmt.Sprintf("metric=%s last sample %s; %d series linger in the index",
						m.MetricName, last.UTC().Format(time.RFC3339), m.Series)},
					Remediation: "Likely a decommissioned emitter. Confirm and remove any dashboards/alerts " +
						"referencing it; series will age out with TTL.",
					Fix: findings.FixHint{Kind: findings.FixNone, Metric: m.MetricName},
				})
			}
		}
	}

	return out, nil
}
