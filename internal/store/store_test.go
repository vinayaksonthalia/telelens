package store

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFixtureStoreLoadsEveryDataset(t *testing.T) {
	st, err := NewFixtureStore("../../fixtures")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	checks := []struct {
		name string
		load func() (int, error)
	}{
		{"span_volume", count(st.SpanVolumeByService, ctx)},
		{"span_attr_cardinality", count(st.SpanAttributeCardinality, ctx)},
		{"jumbo_attributes", count(st.JumboAttributes, ctx)},
		{"duplicate_spans", count(st.DuplicateSpanGroups, ctx)},
		{"log_severity", count(st.LogSeverityBreakdown, ctx)},
		{"log_bodies", count(st.LogBodySamples, ctx)},
		{"metric_cardinality", count(st.MetricCardinality, ctx)},
		{"metric_labels", count(st.MetricLabelCardinality, ctx)},
		{"trace_summaries", count(st.TraceSummaries, ctx)},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			n, err := c.load()
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				t.Errorf("fixture %s is empty", c.name)
			}
		})
	}
}

func count[T any](f func(context.Context) ([]T, error), ctx context.Context) func() (int, error) {
	return func() (int, error) {
		rows, err := f(ctx)
		return len(rows), err
	}
}

func TestFixtureStoreRelativeTimestamps(t *testing.T) {
	st, err := NewFixtureStore("../../fixtures")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := st.MetricCardinality(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	for _, r := range rows {
		if r.LastSeenUnix <= 0 {
			t.Errorf("offset for %s not resolved to an absolute timestamp", r.MetricName)
		}
		if r.LastSeenUnix > now+1 {
			t.Errorf("last_seen for %s is in the future", r.MetricName)
		}
	}
}

func TestFixtureStoreMissingDir(t *testing.T) {
	if _, err := NewFixtureStore("no-such-dir"); err == nil {
		t.Fatal("expected error for missing fixture dir")
	}
}

// TestSelectOnlyGuard enforces the product invariant (NFR-2) at the client
// level: the live store must refuse to send anything but SELECT/WITH.
func TestSelectOnlyGuard(t *testing.T) {
	s := &ClickHouseStore{baseURL: "http://127.0.0.1:1", httpc: &http.Client{Timeout: time.Second}}
	for _, stmt := range []string{
		"INSERT INTO signoz_traces.x VALUES (1)",
		"DROP TABLE signoz_logs.logs_v2",
		"ALTER TABLE t DELETE WHERE 1",
		"TRUNCATE TABLE t",
		"  optimize table t",
	} {
		_, err := query[struct{}](context.Background(), s, stmt)
		if err == nil || !strings.Contains(err.Error(), "refusing to execute non-SELECT") {
			t.Errorf("statement %q was not refused: %v", stmt, err)
		}
	}
	// SELECT passes the guard (and then fails on the dead endpoint, which is fine).
	_, err := query[struct{}](context.Background(), s, "SELECT 1")
	if err != nil && strings.Contains(err.Error(), "refusing to execute") {
		t.Errorf("SELECT wrongly refused: %v", err)
	}
}
