// Package analyze holds the deterministic analysis primitives: pricing math,
// the Drain-style log template miner, and the tail-sampling simulator.
package analyze

// Pricing heuristics — deliberately simple, documented, and honest: these are
// UNCOMPRESSED-INGEST estimates. ClickHouse compresses well on disk, but
// ingest pipelines, network egress, and most vendor pricing are driven by
// uncompressed bytes, so that is the currency of the Waste Report.
const (
	// SpanEnvelopeBytes approximates the per-span cost beyond attribute
	// payloads (ids, timestamps, name, resource copy, index rows).
	SpanEnvelopeBytes = 300
	// LogEnvelopeBytes approximates the per-record cost beyond the body.
	LogEnvelopeBytes = 150
	// MetricSampleBytes approximates one stored sample (v4 sample row).
	MetricSampleBytes = 32
	// DefaultCostPerGB is the default $/GB used when none is configured.
	DefaultCostPerGB = 0.30
)

// Pricer converts observed bytes over a scan window into GB/month and $/month.
type Pricer struct {
	WindowDays float64
	CostPerGB  float64
}

// NewPricer clamps inputs to sane values (never negative, never zero window).
func NewPricer(windowDays int, costPerGB float64) Pricer {
	w := float64(windowDays)
	if w <= 0 {
		w = 7
	}
	if costPerGB < 0 {
		costPerGB = 0
	}
	if costPerGB == 0 {
		costPerGB = DefaultCostPerGB
	}
	return Pricer{WindowDays: w, CostPerGB: costPerGB}
}

// GBPerMonth extrapolates bytes observed in the window to a 30-day month.
func (p Pricer) GBPerMonth(bytesInWindow float64) float64 {
	if bytesInWindow <= 0 {
		return 0
	}
	return bytesInWindow * (30.0 / p.WindowDays) / 1e9
}

// USDPerMonth prices a GB/month figure.
func (p Pricer) USDPerMonth(gbPerMonth float64) float64 {
	if gbPerMonth <= 0 {
		return 0
	}
	return gbPerMonth * p.CostPerGB
}

// SpanBytes estimates total stored bytes for spans with the given summed
// attribute payload.
func SpanBytes(spans, attrBytes int64) float64 {
	return float64(attrBytes) + float64(spans)*SpanEnvelopeBytes
}

// LogBytes estimates total stored bytes for count records with summed body bytes.
func LogBytes(count, bodyBytes int64) float64 {
	return float64(bodyBytes) + float64(count)*LogEnvelopeBytes
}

// MetricBytes estimates stored bytes for a number of samples.
func MetricBytes(samples int64) float64 {
	return float64(samples) * MetricSampleBytes
}
