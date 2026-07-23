package profile

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/findings"
	"github.com/vinayaksonthalia/telelens/internal/signozapi"
	"github.com/vinayaksonthalia/telelens/internal/store"
)

// The profiler tests run against the committed "Noisy Neighborhood" fixture
// corpus, which is engineered to exhibit each waste pattern exactly once (or
// a known number of times). No live ClickHouse or SigNoz is required.

func fixtureStore(t *testing.T) *store.FixtureStore {
	t.Helper()
	st, err := store.NewFixtureStore("../../fixtures")
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func testPricer() analyze.Pricer { return analyze.NewPricer(7, 0.30) }

// findWhere returns findings whose title contains all substrings.
func findWhere(fs []findings.Finding, subs ...string) []findings.Finding {
	var out []findings.Finding
	for _, f := range fs {
		ok := true
		for _, s := range subs {
			if !strings.Contains(f.Title, s) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, f)
		}
	}
	return out
}

func TestTracesProfiler(t *testing.T) {
	fs, err := Traces(context.Background(), fixtureStore(t), testPricer())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("duplicate span groups become one tail-sampling finding", func(t *testing.T) {
		var tail []findings.Finding
		for _, f := range fs {
			if f.Fix.Kind == findings.FixTailSampling {
				tail = append(tail, f)
			}
		}
		if len(tail) != 1 {
			t.Fatalf("want exactly 1 tail-sampling finding, got %d", len(tail))
		}
		f := tail[0]
		// gateway healthz (1.4M) + loadbalancer ping (610k); the erroring
		// checkout group and the small catalog group must be excluded.
		if !strings.Contains(f.Title, "2010000") {
			t.Errorf("expected 2010000 spans in title, got %q", f.Title)
		}
		for _, e := range f.Evidence {
			if strings.Contains(e, "checkout") || strings.Contains(e, "catalog") {
				t.Errorf("unsafe/small group leaked into evidence: %s", e)
			}
		}
		if f.Fix.SamplingPct != DefaultSamplingPct || f.Fix.LatencyThresholdMS != DefaultLatencyThresholdMS {
			t.Errorf("unexpected policy hints: %+v", f.Fix)
		}
		if f.EstUSDPerMonth <= 0 {
			t.Errorf("tail-sampling finding must be priced")
		}
	})

	t.Run("jumbo attributes found, small ones ignored", func(t *testing.T) {
		if n := len(findWhere(fs, "Jumbo attribute catalog.db.statement")); n != 1 {
			t.Errorf("db.statement jumbo findings = %d, want 1", n)
		}
		if n := len(findWhere(fs, "Jumbo attribute checkout.http.request.body")); n != 1 {
			t.Errorf("http.request.body jumbo findings = %d, want 1", n)
		}
		if n := len(findWhere(fs, "payment.provider.response")); n != 0 {
			t.Errorf("480-byte attribute wrongly flagged as jumbo")
		}
	})

	t.Run("cardinality offenders above threshold only", func(t *testing.T) {
		for _, key := range []string{"user.id", "session.id", "http.url"} {
			if n := len(findWhere(fs, "\""+key+"\"")); n != 1 {
				t.Errorf("cardinality finding for %s = %d, want 1", key, n)
			}
		}
		for _, key := range []string{"http.route", "http.status_code", "http.method"} {
			if n := len(findWhere(fs, "\""+key+"\"")); n != 0 {
				t.Errorf("low-cardinality key %s wrongly flagged", key)
			}
		}
	})
}

func TestLogsProfiler(t *testing.T) {
	fs, err := Logs(context.Background(), fixtureStore(t), testPricer())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("debug firehose detected with template evidence", func(t *testing.T) {
		hose := findWhere(fs, "DEBUG-log firehose from catalog")
		if len(hose) != 1 {
			t.Fatalf("firehose findings = %d, want 1", len(hose))
		}
		f := hose[0]
		if f.Fix.Kind != findings.FixFilterLogs || f.Fix.Service != "catalog" || f.Fix.MaxSeverity != "INFO" {
			t.Errorf("bad fix hint: %+v", f.Fix)
		}
		var hasTemplate bool
		for _, e := range f.Evidence {
			if strings.Contains(e, "dominant template") && strings.Contains(e, "<*>") {
				hasTemplate = true
			}
		}
		if !hasTemplate {
			t.Errorf("firehose evidence lacks the mined template: %v", f.Evidence)
		}
		if f.Severity != findings.SeverityCritical || f.EstUSDPerMonth <= 0 {
			t.Errorf("firehose should be critical and priced: %+v", f)
		}
	})

	t.Run("exact-duplicate line detected", func(t *testing.T) {
		dupes := findWhere(fs, "catalog emits the byte-identical line")
		if len(dupes) != 1 {
			t.Fatalf("duplicate-line findings for catalog = %d, want 1", len(dupes))
		}
		if !strings.Contains(dupes[0].Title, "2600000") {
			t.Errorf("expected merged weight 2600000, got %q", dupes[0].Title)
		}
	})

	t.Run("INFO services are not firehoses", func(t *testing.T) {
		if n := len(findWhere(fs, "firehose from gateway")); n != 0 {
			t.Errorf("gateway INFO volume wrongly flagged as firehose")
		}
	})
}

func TestMetricsProfiler(t *testing.T) {
	fs, err := Metrics(context.Background(), fixtureStore(t), testPricer(), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("label bomb identified by name", func(t *testing.T) {
		bombs := findWhere(fs, "user_id", "http_server_duration")
		if len(bombs) != 1 {
			t.Fatalf("label bomb findings = %d, want 1", len(bombs))
		}
		f := bombs[0]
		if f.Fix.Kind != findings.FixDropMetricLabel || f.Fix.Attribute != "user_id" || f.Fix.Metric != "http_server_duration" {
			t.Errorf("bad fix hint: %+v", f.Fix)
		}
		if f.EstUSDPerMonth <= 0 {
			t.Errorf("label bomb must be priced")
		}
	})

	t.Run("healthy metrics not flagged", func(t *testing.T) {
		for _, m := range []string{"cpu_usage", "memory_usage", "request_count"} {
			for _, f := range findWhere(fs, m) {
				t.Errorf("healthy metric %s flagged: %s", m, f.Title)
			}
		}
	})

	t.Run("stale metric flagged", func(t *testing.T) {
		stale := findWhere(fs, "went silent")
		if len(stale) != 1 || !strings.Contains(stale[0].Title, "shipping.Legacy Sync Timer") {
			t.Errorf("stale findings = %+v, want exactly the shipping timer", stale)
		}
	})
}

func TestUsageXref(t *testing.T) {
	api := &signozapi.FixtureAPI{Dir: "../../fixtures"}
	fs, err := UsageXref(context.Background(), fixtureStore(t), api, testPricer())
	if err != nil {
		t.Fatal(err)
	}
	wantOrphans := map[string]bool{
		"temp_debug_counter":         false,
		"legacy_queue_depth":         false,
		"shipping.Legacy Sync Timer": false,
	}
	for _, f := range fs {
		if f.Fix.Kind != findings.FixDropMetric {
			t.Errorf("unexpected fix kind %s", f.Fix.Kind)
		}
		if _, ok := wantOrphans[f.Fix.Metric]; !ok {
			t.Errorf("referenced metric %q wrongly reported as unread", f.Fix.Metric)
			continue
		}
		wantOrphans[f.Fix.Metric] = true
	}
	for m, seen := range wantOrphans {
		if !seen {
			t.Errorf("orphan metric %q not reported", m)
		}
	}
}

// B1: metrics consumed by the SigNoz platform itself (APM spanmetrics,
// collector self-telemetry) are invisible to the dashboards/rules
// cross-reference and must NEVER be flagged as unread.
func TestPlatformConsumedDenylist(t *testing.T) {
	tests := []struct {
		name string
		deny bool
	}{
		{"signoz_calls_total", true},
		{"signoz_latency.count", true},
		{"signoz_latency.bucket", true},
		{"signoz.meter.spans.size", true},
		{"otelcol_exporter_sent_spans", true},
		{"otel.sdk.exporter.spans", true},
		{"legacy_queue_depth", false},
		{"http_server_duration", false},
		{"browser.sessions.count", false},
	}
	for _, tt := range tests {
		if got := platformConsumed(tt.name); got != tt.deny {
			t.Errorf("platformConsumed(%q) = %v, want %v", tt.name, got, tt.deny)
		}
	}
}

// B1 (integration): even when a signoz_* metric is stored and referenced
// nowhere, UsageXref must not propose dropping it.
func TestUsageXrefNeverFlagsPlatformMetrics(t *testing.T) {
	st := &cardinalityOverrideStore{FixtureStore: fixtureStore(t)}
	st.metricSeries = []store.MetricSeries{
		{MetricName: "signoz_calls_total", Series: 100, Samples: 9_000_000},
		{MetricName: "signoz_latency.bucket", Series: 900, Samples: 90_000_000},
		{MetricName: "legacy_queue_depth", Series: 3, Samples: 50_000},
	}
	api := &signozapi.FixtureAPI{Dir: "../../fixtures"}
	fs, err := UsageXref(context.Background(), st, api, testPricer())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range fs {
		got = append(got, f.Fix.Metric)
		if strings.HasPrefix(f.Fix.Metric, "signoz_") {
			t.Errorf("platform metric %q flagged as unread — this filter would blank the APM page", f.Fix.Metric)
		}
	}
	if len(got) != 1 || got[0] != "legacy_queue_depth" {
		t.Errorf("want exactly [legacy_queue_depth] flagged, got %v", got)
	}
}

// cardinalityOverrideStore lets tests inject metric/attribute rows on top of
// the fixture store.
type cardinalityOverrideStore struct {
	*store.FixtureStore
	metricSeries []store.MetricSeries
	spanAttrs    []store.AttributeCardinality
}

func (s *cardinalityOverrideStore) MetricCardinality(ctx context.Context) ([]store.MetricSeries, error) {
	if s.metricSeries != nil {
		return s.metricSeries, nil
	}
	return s.FixtureStore.MetricCardinality(ctx)
}

func (s *cardinalityOverrideStore) SpanAttributeCardinality(ctx context.Context) ([]store.AttributeCardinality, error) {
	if s.spanAttrs != nil {
		return s.spanAttrs, nil
	}
	return s.FixtureStore.SpanAttributeCardinality(ctx)
}

// P1: many near-identical attribute-cardinality advisories roll up into
// top-3 individual rows + ONE aggregate row ("91 findings is zero findings").
func TestAttributeCardinalityRollUp(t *testing.T) {
	st := &cardinalityOverrideStore{FixtureStore: fixtureStore(t)}
	for i := 0; i < 12; i++ {
		st.spanAttrs = append(st.spanAttrs, store.AttributeCardinality{
			Key:         fmt.Sprintf("attr.%02d", i),
			Cardinality: int64(20000 + i*1000),
			Occurrences: 1000,
		})
	}
	fs, err := Traces(context.Background(), st, testPricer())
	if err != nil {
		t.Fatal(err)
	}
	individual := findWhere(fs, "Span attribute ")
	if len(individual) != 3 {
		t.Errorf("individual cardinality rows = %d, want top-3", len(individual))
	}
	agg := findWhere(fs, "9 more span attributes exceed")
	if len(agg) != 1 {
		t.Fatalf("aggregate roll-up rows = %d, want 1", len(agg))
	}
	// The aggregate must name every rolled-up attribute in its evidence.
	if !strings.Contains(agg[0].Evidence[0], "attr.00") {
		t.Errorf("aggregate evidence must list the rolled-up attributes: %v", agg[0].Evidence)
	}
	// Top-3 must be the highest-cardinality keys.
	for _, key := range []string{"attr.11", "attr.10", "attr.09"} {
		if n := len(findWhere(fs, "\""+key+"\"")); n != 1 {
			t.Errorf("top offender %s not reported individually", key)
		}
	}
}

func TestQualityProfiler(t *testing.T) {
	fs, err := Quality(context.Background(), fixtureStore(t))
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		subs []string
	}{
		{"missing service.name", []string{"no service.name"}},
		{"missing severity", []string{"gateway", "no severity"}},
		{"non-standard severity", []string{"NOTICE"}},
		{"mixed casing", []string{"mixes severity casing"}},
		{"bad metric name", []string{"violates OTel naming"}},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if n := len(findWhere(fs, c.subs...)); n != 1 {
				t.Errorf("findings matching %v = %d, want 1", c.subs, n)
			}
		})
	}
	for _, f := range fs {
		if f.Category != findings.CategoryQuality {
			t.Errorf("quality profiler emitted category %s", f.Category)
		}
		if !f.Directional() {
			t.Errorf("quality findings must be directional (unpriced): %+v", f)
		}
	}
}

func TestMetricNameConformance(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"http_server_duration", true},
		{"system.cpu.utilization", true},
		{"process/cpu_seconds", true},
		{"My.Bad Metric", false},
		{"shipping.Legacy Sync Timer", false},
		{"", false},
		{"9lives", false},
		{".leading.dot", false},
	}
	for _, tt := range tests {
		if got := isConformantMetricName(tt.name); got != tt.ok {
			t.Errorf("isConformantMetricName(%q) = %v, want %v", tt.name, got, tt.ok)
		}
	}
}

func TestEcosystemProfiler(t *testing.T) {
	fs, err := Ecosystem(context.Background(), fixtureStore(t), testPricer())
	if err != nil {
		t.Fatal(err)
	}

	// AI-agent telemetry attribution (argus fixture).
	if got := findWhere(fs, "AI-agent telemetry bill", "argus"); len(got) != 1 {
		t.Fatalf("want 1 gen_ai attribution finding for argus, got %d", len(got))
	}

	// Browser RUM attribution (meridian-web fixture, 38 sessions).
	rum := findWhere(fs, "Browser RUM telemetry bill", "meridian-web")
	if len(rum) != 1 {
		t.Fatalf("want 1 RUM attribution finding for meridian-web, got %d", len(rum))
	}
	if !strings.Contains(rum[0].Title, "38 sessions") {
		t.Errorf("RUM finding should count 38 sessions, got %q", rum[0].Title)
	}

	// Structural label bombs: session.id on browser.sessions.count AND
	// user_id on http_server_duration (both in metric_label_usage fixture).
	sid := findWhere(fs, `Unbounded ID "session.id"`, "browser.sessions.count")
	if len(sid) != 1 {
		t.Fatalf("want session.id label-bomb finding, got %d", len(sid))
	}
	if sid[0].Fix.Kind != findings.FixDropMetricLabel || sid[0].Fix.Attribute != "session.id" {
		t.Errorf("session.id finding should carry a drop_metric_label fix, got %+v", sid[0].Fix)
	}
	if got := findWhere(fs, `Unbounded ID "user_id"`, "http_server_duration"); len(got) != 1 {
		t.Fatalf("want user_id label-bomb finding, got %d", len(got))
	}

	// Bounded labels (service.name, page.route) must NOT be flagged.
	if got := findWhere(fs, `Unbounded ID "page.route"`); len(got) != 0 {
		t.Errorf("page.route is bounded and must not be flagged, got %d findings", len(got))
	}

	// The discipline finding fires only for ID-like span attributes that are
	// absent from every metric label (user.id is in span_attr_cardinality
	// but not in metric_label_usage).
	disc := findWhere(fs, "Cardinality discipline held")
	if len(disc) != 1 {
		t.Fatalf("want 1 discipline finding, got %d", len(disc))
	}
	if disc[0].Fix.Kind != findings.FixNone {
		t.Errorf("discipline finding must be advisory, got fix %v", disc[0].Fix.Kind)
	}
}
