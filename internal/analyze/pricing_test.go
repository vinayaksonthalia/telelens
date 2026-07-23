package analyze

import (
	"math"
	"testing"
)

func TestPricerGBPerMonth(t *testing.T) {
	tests := []struct {
		name       string
		windowDays int
		costPerGB  float64
		bytes      float64
		wantGB     float64
		wantUSD    float64
	}{
		{"7d window scales to 30d", 7, 0.30, 7e9, 30.0, 9.0},
		{"30d window is identity", 30, 0.30, 30e9, 30.0, 9.0},
		{"1d window scales 30x", 1, 1.00, 1e9, 30.0, 30.0},
		{"zero bytes is zero", 7, 0.30, 0, 0, 0},
		{"negative bytes clamps to zero", 7, 0.30, -5e9, 0, 0},
		{"zero cost falls back to default", 7, 0, 7e9, 30.0, 30.0 * DefaultCostPerGB},
		{"negative cost falls back to default", 7, -1, 7e9, 30.0, 30.0 * DefaultCostPerGB},
		{"zero window falls back to 7d", 0, 0.30, 7e9, 30.0, 9.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPricer(tt.windowDays, tt.costPerGB)
			gb := p.GBPerMonth(tt.bytes)
			if !approx(gb, tt.wantGB) {
				t.Errorf("GBPerMonth(%v) = %v, want %v", tt.bytes, gb, tt.wantGB)
			}
			usd := p.USDPerMonth(gb)
			if !approx(usd, tt.wantUSD) {
				t.Errorf("USDPerMonth(%v) = %v, want %v", gb, usd, tt.wantUSD)
			}
		})
	}
}

func TestPricerLinearity(t *testing.T) {
	p := NewPricer(7, 0.42)
	for _, base := range []float64{1e6, 1e9, 5e11} {
		one := p.GBPerMonth(base)
		ten := p.GBPerMonth(10 * base)
		if !approx(ten, 10*one) {
			t.Errorf("projection not linear: f(10x)=%v, 10*f(x)=%v", ten, 10*one)
		}
		if one < 0 || p.USDPerMonth(one) < 0 {
			t.Errorf("projection went negative for %v", base)
		}
	}
}

func TestByteEstimators(t *testing.T) {
	if got := SpanBytes(10, 1000); got != 1000+10*SpanEnvelopeBytes {
		t.Errorf("SpanBytes = %v", got)
	}
	if got := LogBytes(10, 1000); got != 1000+10*LogEnvelopeBytes {
		t.Errorf("LogBytes = %v", got)
	}
	if got := MetricBytes(10); got != 10*MetricSampleBytes {
		t.Errorf("MetricBytes = %v", got)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9*(1+math.Abs(b)) }
