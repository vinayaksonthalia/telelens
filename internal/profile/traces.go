// Package profile turns raw telemetry statistics from the store into ranked
// waste findings. All profilers are pure functions over read-only data.
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

// Thresholds for the trace profiler. Exported-by-constant so the README can
// document them; tuned on the fixture corpus.
const (
	// AttrCardinalityThreshold: a span attribute with more distinct values
	// than this is a cardinality offender.
	AttrCardinalityThreshold = 10000
	// JumboAttrAvgBytes: average value size beyond which an attribute is
	// "jumbo" (SQL statements, payload dumps).
	JumboAttrAvgBytes = 1024
	// DuplicateSpanMin: minimum group size before near-identical success
	// spans are worth tail sampling.
	DuplicateSpanMin = 100000
	// DuplicateSpanMaxErrorRate keeps the recommendation safe: only groups
	// that essentially never fail are sampling candidates.
	DuplicateSpanMaxErrorRate = 0.005
	// DefaultSamplingPct is the recommended keep-rate for boring traces.
	DefaultSamplingPct = 5.0
	// DefaultLatencyThresholdMS is the "always keep slow traces" bound.
	DefaultLatencyThresholdMS = 750.0
)

// Traces profiles span volume, attribute cardinality, jumbo attributes and
// near-duplicate success spans.
func Traces(ctx context.Context, st store.TelemetryStore, pricer analyze.Pricer) ([]findings.Finding, error) {
	var out []findings.Finding

	volumes, err := st.SpanVolumeByService(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace profiler: %w", err)
	}
	// Near-duplicate success spans → tail-sampling finding (the big one).
	dups, err := st.DuplicateSpanGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace profiler: %w", err)
	}
	var dupSpans int64
	var dupEvidence []string
	for _, d := range dups {
		if d.Spans < DuplicateSpanMin || d.ErrorRate > DuplicateSpanMaxErrorRate {
			continue
		}
		dupSpans += d.Spans
		dupEvidence = append(dupEvidence, fmt.Sprintf(
			"%s %q: %d near-identical spans, error rate %.3f%%, distinct-duration ratio %.2f",
			d.Service, d.Name, d.Spans, d.ErrorRate*100, d.DistinctDurationRatio))
	}
	if dupSpans > 0 {
		// Sampling keeps DefaultSamplingPct% of these spans' bytes.
		perSpanBytes := avgSpanBytes(volumes)
		savedBytes := float64(dupSpans) * perSpanBytes * (1 - DefaultSamplingPct/100)
		gb := pricer.GBPerMonth(savedBytes)
		out = append(out, findings.Finding{
			Category: findings.CategoryTraces,
			Severity: findings.SeverityHigh,
			Title: fmt.Sprintf("%d near-identical success spans are prime tail-sampling candidates",
				dupSpans),
			Evidence:       dupEvidence,
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: fmt.Sprintf("Add a tail_sampling processor: keep 100%% of errors, keep traces "+
				">= %.0f ms, sample the remaining successes at %.0f%%. Run `telelens simulate` for the safety proof.",
				DefaultLatencyThresholdMS, DefaultSamplingPct),
			Fix: findings.FixHint{
				Kind:               findings.FixTailSampling,
				SamplingPct:        DefaultSamplingPct,
				LatencyThresholdMS: DefaultLatencyThresholdMS,
			},
		})
	}

	// Jumbo attributes.
	jumbos, err := st.JumboAttributes(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace profiler: %w", err)
	}
	for _, j := range jumbos {
		if j.AvgBytes < JumboAttrAvgBytes {
			continue
		}
		bytes := j.AvgBytes * float64(j.Occurrences)
		gb := pricer.GBPerMonth(bytes * 0.9) // truncation keeps a stub
		out = append(out, findings.Finding{
			Category: findings.CategoryTraces,
			Severity: findings.SeverityHigh,
			Title: fmt.Sprintf("Jumbo attribute %s.%s averages %.0f bytes per span",
				j.Service, j.Key, j.AvgBytes),
			Evidence: []string{fmt.Sprintf(
				"service=%s attribute=%q avg=%.0fB max=%dB occurrences=%d (≈%.1f GB stored in window)",
				j.Service, j.Key, j.AvgBytes, j.MaxBytes, j.Occurrences, bytes/1e9)},
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: fmt.Sprintf("Truncate %q to a bounded prefix (or drop it) with a transform "+
				"processor; full payloads belong in logs at DEBUG, not on every span.", j.Key),
			Fix: findings.FixHint{
				Kind:          findings.FixTruncateAttribute,
				Service:       j.Service,
				Attribute:     j.Key,
				TruncateBytes: 256,
			},
		})
	}

	// Attribute cardinality offenders (directional: cardinality is a query
	// and index cost more than a linear byte cost).
	cards, err := st.SpanAttributeCardinality(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace profiler: %w", err)
	}
	// "91 findings is zero findings": a live scan produced 12 near-identical
	// medium advisory rows here. Report the top offenders individually and
	// the rest as ONE aggregate row (same roll-up as unread metrics).
	var offenders []store.AttributeCardinality
	for _, c := range cards {
		if c.Cardinality < AttrCardinalityThreshold {
			continue
		}
		offenders = append(offenders, c)
	}
	sort.Slice(offenders, func(i, j int) bool { return offenders[i].Cardinality > offenders[j].Cardinality })
	const cardIndividualMax = 3
	var cardTail []store.AttributeCardinality
	for i, c := range offenders {
		if i >= cardIndividualMax {
			cardTail = append(cardTail, c)
			continue
		}
		bytes := c.AvgValueSize * float64(c.Occurrences)
		gb := pricer.GBPerMonth(bytes)
		sev := findings.SeverityMedium
		if c.Cardinality >= 10*AttrCardinalityThreshold {
			sev = findings.SeverityHigh
		}
		out = append(out, findings.Finding{
			Category: findings.CategoryTraces,
			Severity: sev,
			Title: fmt.Sprintf("Span attribute %q has %d distinct values",
				c.Key, c.Cardinality),
			Evidence: []string{fmt.Sprintf(
				"key=%q cardinality=%d occurrences=%d avg value %.0fB — bloats attribute indexes and filter latency",
				c.Key, c.Cardinality, c.Occurrences, c.AvgValueSize)},
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			// ADVISORY, no auto-generated drop: high cardinality on SPANS is
			// largely free in ClickHouse (it is a label-on-METRICS problem —
			// the ecosystem profiler catches that). Auto-dropping attributes
			// like db.statement would delete exactly the evidence an
			// investigation needs. Only jumbo values earn a transform.
			Remediation: fmt.Sprintf("No action needed for cost — span cardinality is cheap here. "+
				"Watch two edges: %q must never become a metric label, and if its values grow large "+
				"the jumbo-attribute finding will propose a truncation.", c.Key),
			Fix: findings.FixHint{Kind: findings.FixNone, Attribute: c.Key},
		})
	}
	if len(cardTail) > 0 {
		var names []string
		for _, c := range cardTail {
			names = append(names, fmt.Sprintf("%s (%d)", c.Key, c.Cardinality))
		}
		out = append(out, findings.Finding{
			Category: findings.CategoryTraces,
			Severity: findings.SeverityMedium,
			Title: fmt.Sprintf("%d more span attributes exceed %d distinct values",
				len(cardTail), AttrCardinalityThreshold),
			Evidence: []string{"attributes (cardinality): " + strings.Join(names, ", ") +
				" — same advisory as above: cheap on spans, dangerous as metric labels"},
			Remediation: "Same advisory at lower rank: none of these needs action for cost; " +
				"keep every one of them off metric labels.",
			Fix: findings.FixHint{Kind: findings.FixNone},
		})
	}

	return out, nil
}

func avgSpanBytes(volumes []store.ServiceSpanVolume) float64 {
	var spans int64
	var bytes float64
	for _, v := range volumes {
		spans += v.Spans
		bytes += analyze.SpanBytes(v.Spans, v.AttrBytes)
	}
	if spans == 0 {
		return analyze.SpanEnvelopeBytes
	}
	return bytes / float64(spans)
}
