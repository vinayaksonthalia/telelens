package generate

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/findings"
)

var update = flag.Bool("update", false, "rewrite golden files")

// cannedFindings is a stable findings set covering every fix kind, used for
// the golden-file tests (spec §8.2).
func cannedFindings() []findings.Finding {
	fs := []findings.Finding{
		{
			Category: findings.CategoryLogs, Severity: findings.SeverityCritical,
			Title:         "DEBUG-log firehose from catalog is 43% of your log volume",
			EstGBPerMonth: 61.0, EstUSDPerMonth: 18.30,
			Fix: findings.FixHint{Kind: findings.FixFilterLogs, Service: "catalog", MaxSeverity: "INFO"},
		},
		{
			Category: findings.CategoryTraces, Severity: findings.SeverityHigh,
			Title:         "1400000 near-identical success spans are prime tail-sampling candidates",
			EstGBPerMonth: 38.0, EstUSDPerMonth: 11.40,
			Fix: findings.FixHint{Kind: findings.FixTailSampling, SamplingPct: 5, LatencyThresholdMS: 750},
		},
		{
			Category: findings.CategoryTraces, Severity: findings.SeverityHigh,
			Title:         "Jumbo attribute catalog.db.statement averages 4200 bytes per span",
			EstGBPerMonth: 27.0, EstUSDPerMonth: 8.10,
			Fix: findings.FixHint{Kind: findings.FixTruncateAttribute, Service: "catalog", Attribute: "db.statement", TruncateBytes: 256},
		},
		{
			Category: findings.CategoryMetrics, Severity: findings.SeverityCritical,
			Title:         `Label "user_id" on "http_server_duration" explodes it to 210000 series`,
			EstGBPerMonth: 12.0, EstUSDPerMonth: 3.60,
			Fix: findings.FixHint{Kind: findings.FixDropMetricLabel, Metric: "http_server_duration", Attribute: "user_id"},
		},
		{
			Category: findings.CategoryTraces, Severity: findings.SeverityMedium,
			Title:         `Span attribute "session.id" has 450000 distinct values`,
			EstGBPerMonth: 0.6, EstUSDPerMonth: 0.18,
			Fix: findings.FixHint{Kind: findings.FixDropAttribute, Attribute: "session.id"},
		},
		{
			Category: findings.CategoryMetrics, Severity: findings.SeverityHigh,
			Title:         `Metric "legacy_queue_depth" is written but read by no dashboard or alert`,
			EstGBPerMonth: 0.2, EstUSDPerMonth: 0.07,
			Fix: findings.FixHint{Kind: findings.FixDropMetric, Metric: "legacy_queue_depth"},
		},
		{
			Category: findings.CategoryQuality, Severity: findings.SeverityLow,
			Title: "advisory finding with no fix",
			Fix:   findings.FixHint{Kind: findings.FixNone},
		},
	}
	return findings.Rank(fs)
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file (run `go test ./internal/generate -update`): %v", err)
	}
	if string(want) != got {
		t.Errorf("%s drifted from golden file.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestOtelcolFragmentGolden(t *testing.T) {
	checkGolden(t, "collector-fragment.yaml", OtelcolFragment(cannedFindings()))
}

func TestCastingPatchGolden(t *testing.T) {
	checkGolden(t, "casting-patch.yaml", CastingPatch(cannedFindings()))
}

func TestOtelcolFragmentStructure(t *testing.T) {
	out := OtelcolFragment(cannedFindings())
	for _, want := range []string{
		"tail_sampling:",
		"status_codes: [ERROR]",
		"threshold_ms: 750",
		"sampling_percentage: 5",
		`filter/logs_catalog_below_info:`,
		`severity_number < SEVERITY_NUMBER_INFO`,
		`delete_key(attributes, "session.id")`,
		`Substring(attributes["db.statement"], 0, 256)`,
		`delete_key(attributes, "user_id") where metric.name == "http_server_duration"`,
		"- legacy_queue_depth",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fragment missing %q", want)
		}
	}
	// Every fixable finding's ID must be annotated somewhere.
	for _, f := range cannedFindings() {
		if f.Fix.Kind == findings.FixNone {
			continue
		}
		if !strings.Contains(out, f.ID) {
			t.Errorf("fragment lacks annotation for %s", f.ID)
		}
	}
	if strings.Contains(out, "advisory finding") {
		t.Errorf("advisory (FixNone) finding leaked into the fragment")
	}
}

// B1: the drop_unread_metrics block must ship commented-out with a review
// banner — "unread" cannot see external consumers, and an enabled block once
// listed the spanmetrics powering SigNoz's own APM page.
func TestDropUnreadMetricsShipsCommentedOut(t *testing.T) {
	out := OtelcolFragment(cannedFindings())
	if !strings.Contains(out, "REVIEW BEFORE ENABLING") {
		t.Errorf("fragment lacks the review banner for drop_unread_metrics")
	}
	if !strings.Contains(out, "# filter/drop_unread_metrics:") {
		t.Errorf("drop_unread_metrics block missing (commented form)")
	}
	if strings.Contains(out, "\n  filter/drop_unread_metrics:") {
		t.Errorf("drop_unread_metrics block is active — it must ship commented-out")
	}
	if !strings.Contains(out, "#         - legacy_queue_depth") {
		t.Errorf("orphan metric names must still be listed inside the commented block")
	}
}

// H1: the pipeline-placement hint must carry the spanmetrics-before-
// tail_sampling ordering rule the M4 live run proved essential.
func TestPipelineHintWarnsSpanmetricsOrdering(t *testing.T) {
	out := OtelcolFragment(cannedFindings())
	if !strings.Contains(out, "signozspanmetrics BEFORE tail_sampling") {
		t.Errorf("pipeline hint lacks the spanmetrics-before-tail_sampling warning")
	}
	// Without a tail_sampling finding the warning is irrelevant noise.
	noTail := OtelcolFragment([]findings.Finding{{
		Category: findings.CategoryMetrics, Severity: findings.SeverityHigh,
		Title: "orphan", Fix: findings.FixHint{Kind: findings.FixDropMetric, Metric: "m1"},
	}})
	if strings.Contains(noTail, "signozspanmetrics") {
		t.Errorf("spanmetrics warning emitted without a tail_sampling processor")
	}
}

func TestCastingPatchWrapsFragment(t *testing.T) {
	out := CastingPatch(cannedFindings())
	for _, want := range []string{"spec:", "  ingester:", "    config:", "      data:", "        processors:"} {
		if !strings.Contains(out, want) {
			t.Errorf("casting patch missing %q", want)
		}
	}
}

func TestReportRendering(t *testing.T) {
	fs := cannedFindings()
	sim := analyze.Simulate(analyze.Policy{SamplingPct: 5, LatencyThresholdMS: 750}, nil)
	r := NewReport("fixtures (test)", 7, 0.30, 42*time.Millisecond, fs, &sim)

	md := r.Markdown()
	for _, want := range []string{
		"# TELELENS Waste Report",
		"Total identified savings",
		"F-001", "F-002",
		"Sampling safety proof",
		"read-only",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
	gb, usd := findings.TotalSavings(fs)
	if r.TotalGBPerMonth != gb || r.TotalUSDPerMth != usd {
		t.Errorf("report totals %v/%v, want %v/%v", r.TotalGBPerMonth, r.TotalUSDPerMth, gb, usd)
	}
	if _, err := r.JSON(); err != nil {
		t.Fatalf("JSON render: %v", err)
	}
}

func TestRankOrdersByDollarsAndAssignsIDs(t *testing.T) {
	fs := cannedFindings()
	for i := 1; i < len(fs); i++ {
		if fs[i].EstUSDPerMonth > fs[i-1].EstUSDPerMonth {
			t.Errorf("ranking not descending at %d", i)
		}
	}
	for i, f := range fs {
		want := "F-00" + string(rune('1'+i))
		if f.ID != want {
			t.Errorf("ID at %d = %s, want %s", i, f.ID, want)
		}
	}
}
