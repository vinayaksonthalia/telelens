// Package findings defines the Waste Report's data model: ranked findings
// with evidence, savings estimates, and machine-readable fix hints that the
// config generator consumes.
package findings

import (
	"fmt"
	"sort"
)

// Category buckets findings by signal (plus the data-quality category, which
// subsumes the ideas-board "ingested data quality checker" card #11666).
type Category string

const (
	CategoryTraces  Category = "traces"
	CategoryLogs    Category = "logs"
	CategoryMetrics Category = "metrics"
	CategoryQuality Category = "quality"
)

// Severity expresses how urgently a finding is worth acting on.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// FixKind selects which collector-config remediation the generator emits.
type FixKind string

const (
	// FixTailSampling emits/extends a tail_sampling processor policy set.
	FixTailSampling FixKind = "tail_sampling"
	// FixFilterLogs emits a filter processor dropping low-severity logs of a service.
	FixFilterLogs FixKind = "filter_logs"
	// FixDropAttribute emits a transform processor deleting a span attribute.
	FixDropAttribute FixKind = "drop_attribute"
	// FixTruncateAttribute emits a transform processor truncating jumbo values.
	FixTruncateAttribute FixKind = "truncate_attribute"
	// FixDropMetric emits a filter processor excluding whole metrics.
	FixDropMetric FixKind = "drop_metric"
	// FixDropMetricLabel emits a (metrics) transform deleting a label bomb.
	FixDropMetricLabel FixKind = "drop_metric_label"
	// FixNone means the finding is advisory (e.g. quality findings).
	FixNone FixKind = "none"
)

// FixHint carries everything the generator needs to render a remediation.
type FixHint struct {
	Kind FixKind `json:"kind"`
	// Service scopes the fix (filter_logs, tail sampling evidence).
	Service string `json:"service,omitempty"`
	// Attribute names a span attribute (drop/truncate) or metric label.
	Attribute string `json:"attribute,omitempty"`
	// Metric names a metric (drop_metric / drop_metric_label).
	Metric string `json:"metric,omitempty"`
	// Metrics names additional metrics for aggregate drop_metric findings
	// (e.g. one "N unread metrics" finding covering a long tail).
	Metrics []string `json:"metrics,omitempty"`
	// MaxSeverity: logs strictly below this severity are dropped (filter_logs).
	MaxSeverity string `json:"max_severity,omitempty"`
	// SamplingPct is the probabilistic keep-rate for boring traces.
	SamplingPct float64 `json:"sampling_pct,omitempty"`
	// LatencyThresholdMS: traces slower than this are always kept.
	LatencyThresholdMS float64 `json:"latency_threshold_ms,omitempty"`
	// TruncateBytes for truncate_attribute.
	TruncateBytes int `json:"truncate_bytes,omitempty"`
}

// Finding is one ranked entry of the Waste Report.
type Finding struct {
	ID       string   `json:"id"` // assigned after ranking: F-001, F-002, ...
	Category Category `json:"category"`
	Severity Severity `json:"severity"`
	Title    string   `json:"title"`
	Evidence []string `json:"evidence"`
	// EstGBPerMonth / EstUSDPerMonth are uncompressed-ingest estimates
	// (see analyze/pricing.go for the heuristics). Zero means "directional":
	// real, but not reliably priceable.
	EstGBPerMonth  float64 `json:"est_gb_per_month"`
	EstUSDPerMonth float64 `json:"est_usd_per_month"`
	Remediation    string  `json:"remediation"`
	Fix            FixHint `json:"fix"`
}

// Directional reports whether the finding ships without a hard $ figure.
func (f Finding) Directional() bool { return f.EstUSDPerMonth == 0 }

var severityRank = map[Severity]int{
	SeverityCritical: 0, SeverityHigh: 1, SeverityMedium: 2, SeverityLow: 3,
}

// Rank orders findings by estimated $/mo (desc), then severity, then title
// (stable for tests), and assigns sequential IDs (F-001...). It returns the
// same slice for convenience.
func Rank(fs []Finding) []Finding {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].EstUSDPerMonth != fs[j].EstUSDPerMonth {
			return fs[i].EstUSDPerMonth > fs[j].EstUSDPerMonth
		}
		if severityRank[fs[i].Severity] != severityRank[fs[j].Severity] {
			return severityRank[fs[i].Severity] < severityRank[fs[j].Severity]
		}
		return fs[i].Title < fs[j].Title
	})
	for i := range fs {
		fs[i].ID = fmt.Sprintf("F-%03d", i+1)
	}
	return fs
}

// TotalSavings sums the priced savings across findings.
func TotalSavings(fs []Finding) (gbPerMonth, usdPerMonth float64) {
	for _, f := range fs {
		gbPerMonth += f.EstGBPerMonth
		usdPerMonth += f.EstUSDPerMonth
	}
	return
}
