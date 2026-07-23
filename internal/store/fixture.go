package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FixtureStore implements TelemetryStore from recorded query results on disk
// (fixtures/*.json). It powers `--fixtures` mode, all unit tests, and
// development without a live ClickHouse.
type FixtureStore struct {
	dir string
}

// NewFixtureStore returns a store reading from dir. The directory must exist.
func NewFixtureStore(dir string) (*FixtureStore, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("fixture dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fixture path %q is not a directory", dir)
	}
	return &FixtureStore{dir: dir}, nil
}

func (f *FixtureStore) Description() string { return "fixtures (" + f.dir + ")" }

// loadFixture decodes fixtures/<name>.json into out. A missing file is an
// error: fixtures must be complete so profiler behavior is deterministic.
func loadFixture[T any](f *FixtureStore, name string) ([]T, error) {
	path := filepath.Join(f.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load fixture %s: %w", name, err)
	}
	var out []T
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", path, err)
	}
	return out, nil
}

func (f *FixtureStore) SpanVolumeByService(context.Context) ([]ServiceSpanVolume, error) {
	return loadFixture[ServiceSpanVolume](f, "span_volume")
}

func (f *FixtureStore) SpanAttributeCardinality(context.Context) ([]AttributeCardinality, error) {
	return loadFixture[AttributeCardinality](f, "span_attr_cardinality")
}

func (f *FixtureStore) JumboAttributes(context.Context) ([]JumboAttribute, error) {
	return loadFixture[JumboAttribute](f, "jumbo_attributes")
}

func (f *FixtureStore) DuplicateSpanGroups(context.Context) ([]DuplicateSpanGroup, error) {
	return loadFixture[DuplicateSpanGroup](f, "duplicate_spans")
}

func (f *FixtureStore) LogSeverityBreakdown(context.Context) ([]LogSeverity, error) {
	return loadFixture[LogSeverity](f, "log_severity")
}

func (f *FixtureStore) LogBodySamples(context.Context) ([]LogSample, error) {
	return loadFixture[LogSample](f, "log_bodies")
}

func (f *FixtureStore) MetricCardinality(context.Context) ([]MetricSeries, error) {
	rows, err := loadFixture[MetricSeries](f, "metric_cardinality")
	if err != nil {
		return nil, err
	}
	// Fixtures store last_seen_unix as a non-positive offset in seconds
	// relative to "now" (e.g. -300 = five minutes ago), so staleness
	// detection behaves identically whenever the fixtures are run.
	now := time.Now().Unix()
	for i := range rows {
		if rows[i].LastSeenUnix <= 0 {
			rows[i].LastSeenUnix = now + rows[i].LastSeenUnix
		}
	}
	return rows, nil
}

func (f *FixtureStore) MetricLabelCardinality(context.Context) ([]MetricLabel, error) {
	return loadFixture[MetricLabel](f, "metric_labels")
}

func (f *FixtureStore) TraceSummaries(context.Context) ([]TraceSummary, error) {
	return loadFixture[TraceSummary](f, "trace_summaries")
}

func (f *FixtureStore) GenAISpanVolume(context.Context) ([]GenAISpanVolume, error) {
	return loadFixture[GenAISpanVolume](f, "genai_spans")
}

func (f *FixtureStore) RUMSpanVolume(context.Context) ([]RUMSpanVolume, error) {
	return loadFixture[RUMSpanVolume](f, "rum_spans")
}

func (f *FixtureStore) MetricLabelUsage(context.Context) ([]MetricLabelUsage, error) {
	return loadFixture[MetricLabelUsage](f, "metric_label_usage")
}
