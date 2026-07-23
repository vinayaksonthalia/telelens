package analyze

import (
	"fmt"
	"hash/fnv"

	"github.com/vinayaksonthalia/telelens/internal/store"
)

// Tail-sampling simulator: before TELELENS recommends a tail_sampling policy
// it replays the policy over real (or fixture) trace summaries and reports
// exactly what would have been kept and lost. Safety invariant (NFR-4): a
// policy is only considered safe if it retains 100% of error traces and 100%
// of traces over the latency threshold; the CLI refuses to emit an unsafe one.

// Policy mirrors the generated tail_sampling processor: keep all errors, keep
// all traces slower than LatencyThresholdMS, keep SamplingPct% of the rest.
type Policy struct {
	SamplingPct        float64 `json:"sampling_pct"`
	LatencyThresholdMS float64 `json:"latency_threshold_ms"`
}

// SimResult is the replay outcome.
type SimResult struct {
	Policy Policy `json:"policy"`

	TotalTraces  int `json:"total_traces"`
	KeptTraces   int `json:"kept_traces"`
	ErrorTraces  int `json:"error_traces"`
	ErrorsKept   int `json:"errors_kept"`
	SlowTraces   int `json:"slow_traces"`
	SlowKept     int `json:"slow_kept"`
	BoringTraces int `json:"boring_traces"`
	BoringKept   int `json:"boring_kept"`

	TotalSpans int `json:"total_spans"`
	KeptSpans  int `json:"kept_spans"`

	// Safe is the NFR-4 invariant: no error or slow trace dropped.
	Safe bool `json:"safe"`
}

// DroppedPct is the share of spans the policy would have dropped.
func (r SimResult) DroppedPct() float64 {
	if r.TotalSpans == 0 {
		return 0
	}
	return 100 * float64(r.TotalSpans-r.KeptSpans) / float64(r.TotalSpans)
}

// Simulate replays policy over traces. Sampling decisions are deterministic:
// a trace is kept probabilistically iff FNV64(trace_id) mod 10000 falls under
// SamplingPct — the same decision a hash-seeded sampler would make, and fully
// reproducible across runs (tests assert exact counts).
func Simulate(policy Policy, traces []store.TraceSummary) SimResult {
	r := SimResult{Policy: policy}
	for _, t := range traces {
		r.TotalTraces++
		r.TotalSpans += t.SpanCount
		keep := false
		switch {
		case t.HasError:
			r.ErrorTraces++
			keep = true
			r.ErrorsKept++
		case t.MaxDurationMS >= policy.LatencyThresholdMS && policy.LatencyThresholdMS > 0:
			r.SlowTraces++
			keep = true
			r.SlowKept++
		default:
			r.BoringTraces++
			if sampledIn(t.TraceID, policy.SamplingPct) {
				keep = true
				r.BoringKept++
			}
		}
		if keep {
			r.KeptTraces++
			r.KeptSpans += t.SpanCount
		}
	}
	r.Safe = r.ErrorsKept == r.ErrorTraces && r.SlowKept == r.SlowTraces
	return r
}

// sampledIn deterministically maps a trace id into [0,10000) and keeps it
// when it falls below pct*100 (basis points).
func sampledIn(traceID string, pct float64) bool {
	if pct >= 100 {
		return true
	}
	if pct <= 0 {
		return false
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(traceID))
	return h.Sum64()%10000 < uint64(pct*100)
}

// SafetyReport renders the human-readable replay summary shown after scans.
func (r SimResult) SafetyReport() string {
	verdict := "SAFE — policy retains 100% of error traces and 100% of slow traces"
	if !r.Safe {
		verdict = fmt.Sprintf("UNSAFE — would drop %d error and %d slow traces; policy REJECTED (NFR-4)",
			r.ErrorTraces-r.ErrorsKept, r.SlowTraces-r.SlowKept)
	}
	return fmt.Sprintf(
		`Sampling simulator replay (%d traces, %d spans)
  policy: keep errors + keep >= %.0f ms + sample %.1f%% of the rest
  error traces:  %d / %d kept
  slow traces:   %d / %d kept
  boring traces: %d / %d kept (%.1f%% dropped)
  span volume dropped: %.1f%%
  verdict: %s`,
		r.TotalTraces, r.TotalSpans,
		r.Policy.LatencyThresholdMS, r.Policy.SamplingPct,
		r.ErrorsKept, r.ErrorTraces,
		r.SlowKept, r.SlowTraces,
		r.BoringKept, r.BoringTraces,
		pctDropped(r.BoringTraces, r.BoringKept),
		r.DroppedPct(), verdict)
}

func pctDropped(total, kept int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(total-kept) / float64(total)
}
