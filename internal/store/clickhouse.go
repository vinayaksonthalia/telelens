package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ClickHouseStore is the live TelemetryStore backed by ClickHouse's HTTP
// interface (default :8123). It is strictly read-only: every statement is
// checked to be a SELECT/WITH before it is sent (defense in depth on top of
// the recommended least-privilege ClickHouse user, see README).
type ClickHouseStore struct {
	baseURL    string
	user       string
	password   string
	windowDays int
	httpc      *http.Client
}

// ClickHouseConfig configures the live store.
type ClickHouseConfig struct {
	// BaseURL of the ClickHouse HTTP interface, e.g. http://localhost:8123.
	BaseURL  string
	User     string
	Password string
	// WindowDays is the scan window (default 7).
	WindowDays int
	Timeout    time.Duration
}

// NewClickHouseStore builds the live store and runs the schema-drift guard:
// it probes system.tables for the SigNoz tables it depends on and refuses to
// run against an unrecognized schema with a clear error.
func NewClickHouseStore(ctx context.Context, cfg ClickHouseConfig) (*ClickHouseStore, error) {
	if cfg.WindowDays <= 0 {
		cfg.WindowDays = 7
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 90 * time.Second // NFR-1 scan budget
	}
	s := &ClickHouseStore{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		user:       cfg.User,
		password:   cfg.Password,
		windowDays: cfg.WindowDays,
		httpc:      &http.Client{Timeout: cfg.Timeout},
	}
	if err := s.checkSchema(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ClickHouseStore) Description() string {
	return fmt.Sprintf("clickhouse (%s, window=%dd)", s.baseURL, s.windowDays)
}

// requiredTables are the schema surface TELELENS depends on (SigNoz v0.13x).
var requiredTables = []struct{ db, table string }{
	{"signoz_traces", "distributed_signoz_index_v3"},
	{"signoz_logs", "distributed_logs_v2"},
	{"signoz_metrics", "distributed_time_series_v4"},
}

func (s *ClickHouseStore) checkSchema(ctx context.Context) error {
	type row struct {
		Database string `json:"database"`
		Name     string `json:"name"`
	}
	rows, err := query[row](ctx, s,
		`SELECT database, name FROM system.tables WHERE database LIKE 'signoz_%'`)
	if err != nil {
		return fmt.Errorf("schema detection failed (is ClickHouse reachable at %s?): %w", s.baseURL, err)
	}
	have := map[string]bool{}
	for _, r := range rows {
		have[r.Database+"."+r.Name] = true
	}
	var missing []string
	for _, t := range requiredTables {
		if !have[t.db+"."+t.table] {
			missing = append(missing, t.db+"."+t.table)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("unrecognized SigNoz schema: missing tables %v — TELELENS supports the "+
			"signoz_index_v3 / logs_v2 / time_series_v4 generation; refusing to guess (see README §schema)",
			missing)
	}
	return nil
}

// query executes a single read-only statement and decodes JSONEachRow output.
// MEMORY_LIMIT_EXCEEDED responses are retried with backoff: on a busy live
// instance the server-wide memory tracker spikes transiently during ingest
// flushes/merges, and a read that fails at second 0 succeeds at second 5.
func query[T any](ctx context.Context, s *ClickHouseStore, sql string) ([]T, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 5 * time.Second):
			}
		}
		out, err := queryOnce[T](ctx, s, sql)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "MEMORY_LIMIT_EXCEEDED") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("after 3 attempts (transient ClickHouse memory pressure): %w", lastErr)
}

func queryOnce[T any](ctx context.Context, s *ClickHouseStore, sql string) ([]T, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(sql))
	if !strings.HasPrefix(trimmed, "SELECT") && !strings.HasPrefix(trimmed, "WITH") {
		// Product invariant NFR-2: the profiler physically refuses to write.
		return nil, fmt.Errorf("refusing to execute non-SELECT statement: %.40q", sql)
	}
	u := s.baseURL + "/?" + url.Values{
		"default_format": {"JSONEachRow"},
		"output_format_json_quote_64bit_integers": {"0"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(sql))
	if err != nil {
		return nil, err
	}
	if s.user != "" {
		req.Header.Set("X-ClickHouse-User", s.user)
		req.Header.Set("X-ClickHouse-Key", s.password)
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("clickhouse HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out []T
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var v T
		if err := json.Unmarshal(line, &v); err != nil {
			return nil, fmt.Errorf("decode row: %w", err)
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

func (s *ClickHouseStore) since() string {
	return fmt.Sprintf("toUnixTimestamp(now() - INTERVAL %d DAY) - 1800", s.windowDays)
}

// chSettings is appended to every heavy aggregation so a scan degrades
// gracefully instead of OOM-ing a small ClickHouse (NFR-1). Found the hard
// way: an 8.9M-span GROUP BY on the live instance tripped the server's total
// memory limit without external aggregation.
const chSettings = `
SETTINGS max_threads = 2,
         max_bytes_before_external_group_by = 300000000,
         max_bytes_before_external_sort = 300000000`

func (s *ClickHouseStore) SpanVolumeByService(ctx context.Context) ([]ServiceSpanVolume, error) {
	return query[ServiceSpanVolume](ctx, s, fmt.Sprintf(`
SELECT resource_string_service$$name AS service,
       count() AS spans,
       sum(length(attributes_string)) AS attr_bytes
FROM signoz_traces.distributed_signoz_index_v3
WHERE ts_bucket_start >= %s
GROUP BY service
ORDER BY spans DESC
LIMIT 100`+chSettings, s.since()))
}

func (s *ClickHouseStore) SpanAttributeCardinality(ctx context.Context) ([]AttributeCardinality, error) {
	return query[AttributeCardinality](ctx, s, `
SELECT tag_key AS key,
       uniqCombined(string_value) AS cardinality,
       count() AS occurrences,
       avg(length(string_value)) AS avg_value_bytes
FROM signoz_traces.distributed_tag_attributes_v2
WHERE tag_data_type = 'string'
GROUP BY key
HAVING cardinality > 100
ORDER BY cardinality DESC
LIMIT 50`+chSettings)
}

// JumboAttributes finds attributes whose values are disproportionately large.
// The live query is DAY-SLICED and merged in Go: the ARRAY JOIN over every
// span attribute in a 7-day window (12.4M spans -> 100M+ expanded rows on the
// reference instance) exceeded ClickHouse's server memory limit as a single
// query, while per-day slices complete in under a second each. Averages are
// computed from summed bytes so the merge is exact, and the >256 B filter is
// applied after the merge (a per-slice HAVING would bias the result).
func (s *ClickHouseStore) JumboAttributes(ctx context.Context) ([]JumboAttribute, error) {
	type sliceRow struct {
		Service     string `json:"service"`
		Key         string `json:"key"`
		SumBytes    int64  `json:"sum_bytes"`
		MaxBytes    int64  `json:"max_bytes"`
		Occurrences int64  `json:"occurrences"`
	}
	type agg struct {
		sumBytes, maxBytes, occurrences int64
	}
	merged := map[[2]string]*agg{}
	for day := 0; day < s.windowDays; day++ {
		rows, err := query[sliceRow](ctx, s, fmt.Sprintf(`
SELECT resource_string_service$$name AS service,
       k AS key,
       sum(length(v)) AS sum_bytes,
       max(length(v)) AS max_bytes,
       count() AS occurrences
FROM signoz_traces.distributed_signoz_index_v3
ARRAY JOIN mapKeys(attributes_string) AS k, mapValues(attributes_string) AS v
WHERE ts_bucket_start >= toUnixTimestamp(now() - INTERVAL %d DAY) - 1800
  AND ts_bucket_start <  toUnixTimestamp(now() - INTERVAL %d DAY) - 1800
GROUP BY service, key
SETTINGS max_threads = 2, max_block_size = 8192,
         max_bytes_before_external_group_by = 300000000`, day+1, day))
		if err != nil {
			return nil, fmt.Errorf("jumbo attributes day slice -%dd: %w", day+1, err)
		}
		for _, r := range rows {
			k := [2]string{r.Service, r.Key}
			a, ok := merged[k]
			if !ok {
				a = &agg{}
				merged[k] = a
			}
			a.sumBytes += r.SumBytes
			a.occurrences += r.Occurrences
			if r.MaxBytes > a.maxBytes {
				a.maxBytes = r.MaxBytes
			}
		}
	}
	var out []JumboAttribute
	for k, a := range merged {
		if a.occurrences == 0 {
			continue
		}
		avg := float64(a.sumBytes) / float64(a.occurrences)
		if avg <= 256 {
			continue
		}
		out = append(out, JumboAttribute{
			Service: k[0], Key: k[1],
			AvgBytes: avg, MaxBytes: a.maxBytes, Occurrences: a.occurrences,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AvgBytes*float64(out[i].Occurrences) > out[j].AvgBytes*float64(out[j].Occurrences)
	})
	if len(out) > 50 {
		out = out[:50]
	}
	return out, nil
}

func (s *ClickHouseStore) DuplicateSpanGroups(ctx context.Context) ([]DuplicateSpanGroup, error) {
	return query[DuplicateSpanGroup](ctx, s, fmt.Sprintf(`
SELECT resource_string_service$$name AS service,
       name,
       count() AS spans,
       countIf(status_code = 2) / count() AS error_rate,
       uniqCombined(duration_nano) / count() AS distinct_duration_ratio
FROM signoz_traces.distributed_signoz_index_v3
WHERE ts_bucket_start >= %s
GROUP BY service, name
HAVING spans > 10000 AND error_rate < 0.01
ORDER BY spans DESC
LIMIT 50`+chSettings, s.since()))
}

func (s *ClickHouseStore) LogSeverityBreakdown(ctx context.Context) ([]LogSeverity, error) {
	return query[LogSeverity](ctx, s, fmt.Sprintf(`
SELECT resources_string['service.name'] AS service,
       severity_text,
       count() AS count,
       sum(length(body)) AS bytes
FROM signoz_logs.distributed_logs_v2
WHERE ts_bucket_start >= %s
GROUP BY service, severity_text
ORDER BY bytes DESC
LIMIT 200`+chSettings, s.since()))
}

func (s *ClickHouseStore) LogBodySamples(ctx context.Context) ([]LogSample, error) {
	// Bounded sample per service; bodies are only ever used locally for
	// template mining (NFR-7) and never leave the process. The sample window
	// is capped at 1 day regardless of the scan window: templates are stable
	// over days, and an unbounded LIMIT BY over a week of bodies OOM'd the
	// live instance's ClickHouse.
	return query[LogSample](ctx, s, `
SELECT resources_string['service.name'] AS service,
       severity_text,
       body,
       1 AS weight
FROM signoz_logs.distributed_logs_v2
WHERE ts_bucket_start >= toUnixTimestamp(now() - INTERVAL 1 DAY) - 1800
LIMIT 1000 BY service
LIMIT 10000`+chSettings)
}

// MetricCardinality reports per-metric series counts (from the time-series
// index) and REAL datapoint counts (from samples_v4). The original single
// query counted time_series_v4 rows as "samples", but that table holds one
// row per (hour, series) — underpricing every metric by orders of magnitude
// on the live instance. Found during the live-scan wave.
func (s *ClickHouseStore) MetricCardinality(ctx context.Context) ([]MetricSeries, error) {
	series, err := query[MetricSeries](ctx, s, fmt.Sprintf(`
SELECT metric_name,
       uniqCombined(fingerprint) AS series,
       toInt64(max(unix_milli) / 1000) AS last_seen_unix
FROM signoz_metrics.distributed_time_series_v4
WHERE unix_milli >= toUnixTimestamp64Milli(now64() - INTERVAL %d DAY)
GROUP BY metric_name
ORDER BY series DESC
LIMIT 100`+chSettings, s.windowDays))
	if err != nil {
		return nil, err
	}
	type sampleRow struct {
		MetricName string `json:"metric_name"`
		Samples    int64  `json:"samples"`
	}
	samples, err := query[sampleRow](ctx, s, fmt.Sprintf(`
SELECT metric_name, count() AS samples
FROM signoz_metrics.distributed_samples_v4
WHERE unix_milli >= toUnixTimestamp64Milli(now64() - INTERVAL %d DAY)
GROUP BY metric_name`+chSettings, s.windowDays))
	if err != nil {
		return nil, err
	}
	byName := make(map[string]int64, len(samples))
	for _, r := range samples {
		byName[r.MetricName] = r.Samples
	}
	for i := range series {
		series[i].Samples = byName[series[i].MetricName]
	}
	return series, nil
}

func (s *ClickHouseStore) MetricLabelCardinality(ctx context.Context) ([]MetricLabel, error) {
	return query[MetricLabel](ctx, s, fmt.Sprintf(`
SELECT metric_name,
       kv.1 AS label,
       uniqCombined(kv.2) AS cardinality
FROM signoz_metrics.distributed_time_series_v4
ARRAY JOIN JSONExtractKeysAndValues(labels, 'String') AS kv
WHERE unix_milli >= toUnixTimestamp64Milli(now64() - INTERVAL %d DAY)
  AND kv.1 NOT IN ('__name__', 'le')
GROUP BY metric_name, label
HAVING cardinality > 50
ORDER BY cardinality DESC
LIMIT 100`+chSettings, s.windowDays))
}

func (s *ClickHouseStore) GenAISpanVolume(ctx context.Context) ([]GenAISpanVolume, error) {
	return query[GenAISpanVolume](ctx, s, fmt.Sprintf(`
SELECT resource_string_service$$name AS service,
       count() AS spans,
       sum(length(attributes_string)) AS attr_bytes,
       sum(attributes_number['gen_ai.usage.input_tokens'])
         + sum(attributes_number['gen_ai.usage.output_tokens']) AS tokens
FROM signoz_traces.distributed_signoz_index_v3
WHERE ts_bucket_start >= %s
  AND (mapContains(attributes_string, 'gen_ai.provider.name')
       OR mapContains(attributes_string, 'gen_ai.request.model')
       OR mapContains(attributes_string, 'gen_ai.system'))
GROUP BY service
ORDER BY spans DESC
LIMIT 50`+chSettings, s.since()))
}

func (s *ClickHouseStore) RUMSpanVolume(ctx context.Context) ([]RUMSpanVolume, error) {
	return query[RUMSpanVolume](ctx, s, fmt.Sprintf(`
SELECT resource_string_service$$name AS service,
       count() AS spans,
       sum(length(attributes_string)) AS attr_bytes,
       uniqCombined(attributes_string['session.id']) AS sessions
FROM signoz_traces.distributed_signoz_index_v3
WHERE ts_bucket_start >= %s
  AND mapContains(attributes_string, 'session.id')
GROUP BY service
ORDER BY spans DESC
LIMIT 50`+chSettings, s.since()))
}

func (s *ClickHouseStore) MetricLabelUsage(ctx context.Context) ([]MetricLabelUsage, error) {
	return query[MetricLabelUsage](ctx, s, fmt.Sprintf(`
SELECT label,
       metric_name,
       count() AS series
FROM (
    SELECT metric_name, arrayJoin(JSONExtractKeys(labels)) AS label
    FROM signoz_metrics.distributed_time_series_v4
    WHERE unix_milli >= toUnixTimestamp64Milli(now64() - INTERVAL %d DAY)
)
WHERE label NOT LIKE '\\_\\_%%'
GROUP BY label, metric_name`+chSettings, s.windowDays))
}

// TraceSummaries reduces the last 24h of spans to one row per trace for the
// sampling simulator. The live query is TIME-SLICED into 3h chunks that are
// merged in Go: a single GROUP BY trace_id over the full day (8.9M spans on
// the reference instance) exceeded ClickHouse's server-wide memory limit even
// with external aggregation, while per-slice grouping is cheap. Traces that
// straddle a slice boundary are merged exactly (max/sum are associative;
// the root fields take the first non-empty value).
func (s *ClickHouseStore) TraceSummaries(ctx context.Context) ([]TraceSummary, error) {
	const sliceHours = 3
	const totalHours = 24
	merged := make(map[string]*TraceSummary, 1<<17)
	for offset := 0; offset < totalHours; offset += sliceHours {
		rows, err := query[TraceSummary](ctx, s, fmt.Sprintf(`
SELECT trace_id,
       anyIf(resource_string_service$$name, parent_span_id = '') AS service,
       anyIf(name, parent_span_id = '') AS root_name,
       toBool(max(has_error)) AS has_error,
       max(duration_nano) / 1e6 AS max_duration_ms,
       toInt32(count()) AS span_count
FROM signoz_traces.distributed_signoz_index_v3
WHERE ts_bucket_start >= toUnixTimestamp(now() - INTERVAL %d HOUR) - 1800
  AND ts_bucket_start <  toUnixTimestamp(now() - INTERVAL %d HOUR) - 1800
GROUP BY trace_id`+chSettings, offset+sliceHours, offset))
		if err != nil {
			return nil, fmt.Errorf("trace summaries slice [-%dh,-%dh): %w", offset+sliceHours, offset, err)
		}
		for _, r := range rows {
			cur, ok := merged[r.TraceID]
			if !ok {
				row := r
				merged[r.TraceID] = &row
				continue
			}
			cur.HasError = cur.HasError || r.HasError
			if r.MaxDurationMS > cur.MaxDurationMS {
				cur.MaxDurationMS = r.MaxDurationMS
			}
			cur.SpanCount += r.SpanCount
			if cur.Service == "" {
				cur.Service = r.Service
			}
			if cur.RootName == "" {
				cur.RootName = r.RootName
			}
		}
	}
	out := make([]TraceSummary, 0, len(merged))
	for _, v := range merged {
		out = append(out, *v)
	}
	return out, nil
}
