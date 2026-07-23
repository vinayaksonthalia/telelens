// Package store defines the read-only telemetry data source used by all
// profilers. Two implementations exist: a live ClickHouse client (HTTP
// interface, SELECT-only) and a fixture-backed store used for tests, the
// offline demo mode, and development without a running SigNoz stack.
package store

import "context"

// ServiceSpanVolume is one row of the span volume-by-service query (Q1).
type ServiceSpanVolume struct {
	Service   string `json:"service"`
	Spans     int64  `json:"spans"`
	AttrBytes int64  `json:"attr_bytes"`
}

// AttributeCardinality is one row of the span attribute cardinality query (Q2).
type AttributeCardinality struct {
	Key          string  `json:"key"`
	Cardinality  int64   `json:"cardinality"`
	Occurrences  int64   `json:"occurrences"`
	AvgValueSize float64 `json:"avg_value_bytes"`
}

// JumboAttribute is one row of the jumbo-attribute query (Q3): attributes whose
// values are disproportionately large (SQL statements, payload dumps).
type JumboAttribute struct {
	Service     string  `json:"service"`
	Key         string  `json:"key"`
	AvgBytes    float64 `json:"avg_bytes"`
	MaxBytes    int64   `json:"max_bytes"`
	Occurrences int64   `json:"occurrences"`
}

// DuplicateSpanGroup is one row of the near-duplicate success span query (Q4):
// span groups that are prime tail-sampling candidates.
type DuplicateSpanGroup struct {
	Service               string  `json:"service"`
	Name                  string  `json:"name"`
	Spans                 int64   `json:"spans"`
	ErrorRate             float64 `json:"error_rate"`
	DistinctDurationRatio float64 `json:"distinct_duration_ratio"`
}

// LogSeverity is one row of the log severity breakdown query (Q5).
type LogSeverity struct {
	Service      string `json:"service"`
	SeverityText string `json:"severity_text"`
	Count        int64  `json:"count"`
	Bytes        int64  `json:"bytes"`
}

// LogSample is a sampled log body used locally for template mining (Q6).
// Weight carries how many stored records the sample represents (fixture data
// pre-aggregates; live sampling uses Weight=1).
type LogSample struct {
	Service      string `json:"service"`
	SeverityText string `json:"severity_text"`
	Body         string `json:"body"`
	Weight       int64  `json:"weight"`
}

// MetricSeries is one row of the metric cardinality query (Q7).
type MetricSeries struct {
	MetricName   string `json:"metric_name"`
	Series       int64  `json:"series"`
	Samples      int64  `json:"samples"`
	LastSeenUnix int64  `json:"last_seen_unix"`
}

// MetricLabel is one row of the label-explosion query (Q8): per-metric,
// per-label distinct value counts.
type MetricLabel struct {
	MetricName  string `json:"metric_name"`
	Label       string `json:"label"`
	Cardinality int64  `json:"cardinality"`
}

// TraceSummary is one row of the simulator input query (Q9): one whole trace
// reduced to the fields tail-sampling policies decide on.
type TraceSummary struct {
	TraceID       string  `json:"trace_id"`
	Service       string  `json:"service"`
	RootName      string  `json:"root_name"`
	HasError      bool    `json:"has_error"`
	MaxDurationMS float64 `json:"max_duration_ms"`
	SpanCount     int     `json:"span_count"`
}

// GenAISpanVolume is one row of the AI-telemetry attribution query (Q10):
// per-service volume of spans carrying gen_ai.* semantic-convention
// attributes (LLM/agent telemetry).
type GenAISpanVolume struct {
	Service   string  `json:"service"`
	Spans     int64   `json:"spans"`
	AttrBytes int64   `json:"attr_bytes"`
	Tokens    float64 `json:"tokens"`
}

// RUMSpanVolume is one row of the browser-RUM attribution query (Q11):
// per-service volume of spans carrying a session.id attribute.
type RUMSpanVolume struct {
	Service   string `json:"service"`
	Spans     int64  `json:"spans"`
	AttrBytes int64  `json:"attr_bytes"`
	Sessions  int64  `json:"sessions"`
}

// MetricLabelUsage is one row of the label-usage query (Q12): which metrics
// carry which label keys (used by the cardinality-discipline check).
type MetricLabelUsage struct {
	Label      string `json:"label"`
	MetricName string `json:"metric_name"`
	Series     int64  `json:"series"`
}

// TelemetryStore is the read-only source all profilers depend on.
// Implementations MUST NOT mutate any external state (product invariant
// NFR-2): the live implementation issues only SELECT statements.
type TelemetryStore interface {
	SpanVolumeByService(ctx context.Context) ([]ServiceSpanVolume, error)
	SpanAttributeCardinality(ctx context.Context) ([]AttributeCardinality, error)
	JumboAttributes(ctx context.Context) ([]JumboAttribute, error)
	DuplicateSpanGroups(ctx context.Context) ([]DuplicateSpanGroup, error)
	LogSeverityBreakdown(ctx context.Context) ([]LogSeverity, error)
	LogBodySamples(ctx context.Context) ([]LogSample, error)
	MetricCardinality(ctx context.Context) ([]MetricSeries, error)
	MetricLabelCardinality(ctx context.Context) ([]MetricLabel, error)
	TraceSummaries(ctx context.Context) ([]TraceSummary, error)
	// GenAISpanVolume attributes span volume to AI/LLM telemetry
	// (gen_ai.* semantic conventions) per service.
	GenAISpanVolume(ctx context.Context) ([]GenAISpanVolume, error)
	// RUMSpanVolume attributes span volume to browser RUM telemetry
	// (spans carrying session.id) per service.
	RUMSpanVolume(ctx context.Context) ([]RUMSpanVolume, error)
	// MetricLabelUsage lists every (label key, metric) pair in the window.
	MetricLabelUsage(ctx context.Context) ([]MetricLabelUsage, error)
	// Description identifies the data source in reports ("fixtures", DSN, ...).
	Description() string
}
