package analyze

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vinayaksonthalia/telelens/internal/store"
)

// labeledSet builds a synthetic trace set with hand-known keep/drop labels.
func labeledSet(errors, slow, boring int) []store.TraceSummary {
	var ts []store.TraceSummary
	for i := 0; i < errors; i++ {
		ts = append(ts, store.TraceSummary{
			TraceID: fmt.Sprintf("err-%d", i), HasError: true, MaxDurationMS: 100, SpanCount: 3,
		})
	}
	for i := 0; i < slow; i++ {
		ts = append(ts, store.TraceSummary{
			TraceID: fmt.Sprintf("slow-%d", i), MaxDurationMS: 2000, SpanCount: 5,
		})
	}
	for i := 0; i < boring; i++ {
		ts = append(ts, store.TraceSummary{
			TraceID: fmt.Sprintf("ok-%d", i), MaxDurationMS: 10, SpanCount: 1,
		})
	}
	return ts
}

func TestSimulateExactCounts(t *testing.T) {
	tests := []struct {
		name                 string
		policy               Policy
		errors, slow, boring int
		wantBoringKept       *int // nil = don't pin (probabilistic middle ground)
	}{
		{"0% drops every boring trace", Policy{SamplingPct: 0, LatencyThresholdMS: 750}, 10, 20, 500, ptr(0)},
		{"100% keeps everything", Policy{SamplingPct: 100, LatencyThresholdMS: 750}, 10, 20, 500, ptr(500)},
		{"5% keeps roughly 5%", Policy{SamplingPct: 5, LatencyThresholdMS: 750}, 40, 55, 1100, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Simulate(tt.policy, labeledSet(tt.errors, tt.slow, tt.boring))
			if r.ErrorTraces != tt.errors || r.ErrorsKept != tt.errors {
				t.Errorf("errors: %d/%d kept, want %d/%d", r.ErrorsKept, r.ErrorTraces, tt.errors, tt.errors)
			}
			if r.SlowTraces != tt.slow || r.SlowKept != tt.slow {
				t.Errorf("slow: %d/%d kept, want %d/%d", r.SlowKept, r.SlowTraces, tt.slow, tt.slow)
			}
			if r.BoringTraces != tt.boring {
				t.Errorf("boring total = %d, want %d", r.BoringTraces, tt.boring)
			}
			if tt.wantBoringKept != nil && r.BoringKept != *tt.wantBoringKept {
				t.Errorf("boring kept = %d, want %d", r.BoringKept, *tt.wantBoringKept)
			}
			if tt.wantBoringKept == nil {
				// Probabilistic band: 5% ±3pp over 1100 traces.
				frac := float64(r.BoringKept) / float64(tt.boring)
				if frac < 0.02 || frac > 0.08 {
					t.Errorf("boring keep fraction = %.3f, want ≈0.05", frac)
				}
			}
			if !r.Safe {
				t.Errorf("policy reported unsafe on a set where errors+slow are always kept")
			}
			wantKept := r.ErrorsKept + r.SlowKept + r.BoringKept
			if r.KeptTraces != wantKept {
				t.Errorf("KeptTraces = %d, want %d", r.KeptTraces, wantKept)
			}
		})
	}
}

// TestSimulateSafetyInvariant is the NFR-4 battery: across randomized-ish
// compositions, the recommended policy shape must never drop an error or
// slow trace. A violation is a hard failure.
func TestSimulateSafetyInvariant(t *testing.T) {
	policy := Policy{SamplingPct: 5, LatencyThresholdMS: 750}
	for errors := 0; errors <= 50; errors += 7 {
		for slow := 0; slow <= 50; slow += 11 {
			r := Simulate(policy, labeledSet(errors, slow, 200))
			if r.ErrorsKept != errors || r.SlowKept != slow {
				t.Fatalf("SAFETY VIOLATION: errors %d/%d, slow %d/%d kept",
					r.ErrorsKept, errors, r.SlowKept, slow)
			}
			if !r.Safe {
				t.Fatalf("Safe=false with nothing dropped (errors=%d slow=%d)", errors, slow)
			}
		}
	}
}

func TestSimulateDeterministic(t *testing.T) {
	set := labeledSet(5, 5, 300)
	p := Policy{SamplingPct: 5, LatencyThresholdMS: 750}
	a, b := Simulate(p, set), Simulate(p, set)
	if a != b {
		t.Errorf("simulation not deterministic: %+v vs %+v", a, b)
	}
}

func TestSimulateZeroLatencyThresholdDisablesSlowKeep(t *testing.T) {
	// With no latency policy, slow traces are just boring traces.
	r := Simulate(Policy{SamplingPct: 0}, labeledSet(0, 10, 0))
	if r.SlowTraces != 0 || r.BoringTraces != 10 {
		t.Errorf("expected slow traces to be classified boring: %+v", r)
	}
}

func TestSafetyReportVerdicts(t *testing.T) {
	safe := Simulate(Policy{SamplingPct: 0, LatencyThresholdMS: 750}, labeledSet(3, 3, 10))
	if !strings.Contains(safe.SafetyReport(), "SAFE") {
		t.Errorf("expected SAFE verdict:\n%s", safe.SafetyReport())
	}
	unsafe := SimResult{Policy: Policy{}, ErrorTraces: 2, ErrorsKept: 1, Safe: false}
	if !strings.Contains(unsafe.SafetyReport(), "REJECTED") {
		t.Errorf("expected REJECTED verdict:\n%s", unsafe.SafetyReport())
	}
}

func ptr(n int) *int { return &n }
