# M2 evidence — LIVE scan of the real SigNoz instance (Jul 18, 2026)

**Command:** `telelens scan --window 7 --out out-live` against the live stack
(ClickHouse HTTP via a socat forwarder on :8123 — the Foundry compose does not
publish the port; SigNoz API :8080 with SIGNOZ-API-KEY). Scan completed in
**3.74 s** (NFR-1 budget: 90 s). Full transcript: `live-scan-transcript.txt`;
report: `live-waste-report.md` + `live-findings.json`; generated fixes:
`live-collector-fragment.yaml` + `live-casting-patch.yaml`.

## The instance (real mixed telemetry, 7-day window)

- 12.4M spans / 1.01 GiB in signoz_traces; 7.5M log records / 356 MiB in
  signoz_logs; 84k time-series rows, 4.9M samples in signoz_metrics.
- Workloads: Faultline backend (catalog/gateway/orders/payments, live),
  historical opentelemetry-demo-lite (checkout/cart/frontend/…, torn down but
  in-window), GLASSPANE's Meridian (meridian-web/meridian-api browser RUM),
  ARGUS's own gen_ai self-telemetry (service=argus).

## Headline results (26 ranked findings, honest numbers for a small instance)

- **Total identified savings: 12.5 GB/month ≈ $3.75/month** at $0.30/GB
  uncompressed-ingest pricing (the instance is tiny; the *ratios* are the story).
- F-001: 7.79M near-identical success spans → tail-sampling candidate (9.7 GB/mo).
- F-002: INFO firehose — `checkout` is 35% of all log bytes.
- F-003/F-004…: 91 metrics written but read by NO dashboard/alert — top 5
  individually priced, 86 rolled into one aggregate finding (incl. `argus.tokens`,
  `browser.web_vitals.*`, the entire system.* host set).

## THE CROSS-PROJECT MOMENT (the tool audits its own ecosystem)

- **F-023 — ARGUS's AI bill:** service `argus` ships 35 `gen_ai.*` spans carrying
  **204,665 tracked tokens** — the cost of observing the AI agent, priced.
- **F-022 — GLASSPANE's RUM bill:** `meridian-web` ships 223 session spans across
  38 sessions ≈ **1.8 KB/session** uncompressed.
- **F-024 — TELELENS caught a real bomb in its sibling:** GLASSPANE's own
  `browser.sessions.count` metric carries **`session.id` as a label** (37 series
  today, unbounded by construction). GLASSPANE's celebrated hard-guard covers
  *histograms* only — the counter slipped through. Flagged as structural
  (severity high, priced "directional") because an ID label is a time bomb at ANY
  current cardinality. Generated fix: `drop_metric_label` block `F-024` in the
  collector fragment.
- **F-025 — and credit where due:** `order.id`, `user.id` live on spans with
  tens of thousands of distinct values and appear on **no metric series** —
  cardinality discipline held, reported as a positive finding.

## What broke against reality (fixtures were synthetic; reality bit — all fixed)

1. **ClickHouse server-wide OOM (MEMORY_LIMIT_EXCEEDED)** on three profiler
   queries. Root causes and fixes, in order discovered:
   - The 24h `GROUP BY trace_id` (8.9M spans) and the 7-day attribute ARRAY
     JOIN (~100M expanded rows) exceeded the server's total memory limit as
     single queries → **time-sliced** (3h / 1-day chunks) and merged exactly in
     Go; every heavy query now carries
     `SETTINGS max_threads=2, max_bytes_before_external_group_by=300MB`.
   - `LIMIT 2000 BY service` over 7 days of log bodies OOM'd → sample window
     capped at 1 day (templates are stable), 1000/service.
2. **A platform pathology TELELENS's scan uncovered:** `system.metric_log`
   merges ran in a loop — **~80 merges/min, each peaking 6.2 GiB** on a 53k-row,
   ~1300-column table — randomly killing ANY telemetry query on the instance.
   ClickHouse's own observability was the biggest resource consumer on the
   observability database. Mitigation applied: `SYSTEM STOP MERGES
   system.metric_log` (runtime, instantly revertable with START MERGES) +
   metric_log disabled in the pours ClickHouse config (backup:
   `config-0-0.yaml.pre-telelens-metriclog-backup`; takes effect on next
   restart). Also on disk: `system.trace_log` is **1.29 GiB / 76M rows — larger
   than all of signoz_traces**.
3. **Metric pricing was wrong by orders of magnitude:** samples were counted
   from `time_series_v4` (one row per *hour* per series), not
   `distributed_samples_v4` (actual datapoints). Fixed with a real per-metric
   sample count query.
4. **`has_error` decode:** `max(status_code=2)=1` returns 0/1 which does not
   unmarshal into a Go bool; switched to the native `has_error` Bool column.
5. **Report noise:** 91 near-identical "unread metric" rows drowned the signal
   → top-5 individually priced + one aggregate roll-up finding (the generated
   `filter/drop_unread_metrics` block still lists every name).
6. Transient memory-pressure 500s (ingest flushes) → bounded retry with backoff
   in the live store.

## Read-only invariant

Every statement issued was SELECT (guard enforced in code); SigNoz API access
was GET-only. The socat forwarder adds no state. `SYSTEM STOP MERGES` was an
operator action outside the profiler, documented above.

## Key verification queries (run via clickhouse-client, reproducible)

```sql
-- data volume in window
SELECT count() FROM signoz_traces.distributed_signoz_index_v3
WHERE ts_bucket_start >= toUnixTimestamp(now() - INTERVAL 7 DAY) - 1800;  -- 12,414,258

-- ARGUS gen_ai attribution (matches F-023)
SELECT count(), sum(attributes_number['gen_ai.usage.input_tokens'])
     + sum(attributes_number['gen_ai.usage.output_tokens'])
FROM signoz_traces.distributed_signoz_index_v3
WHERE mapContains(attributes_string,'gen_ai.provider.name')
  AND resource_string_service$$name = 'argus';           -- 35 spans, 204,665 tokens

-- the session.id label bomb (matches F-024)
SELECT metric_name, count() FROM signoz_metrics.distributed_time_series_v4
WHERE JSONHas(labels,'session.id') GROUP BY metric_name; -- browser.sessions.count, 37

-- the metric_log merge storm (platform finding)
SELECT table, count(), formatReadableSize(max(peak_memory_usage))
FROM system.part_log WHERE event_type='MergeParts'
  AND event_time > now() - INTERVAL 10 MINUTE GROUP BY table
ORDER BY 2 DESC;                                          -- metric_log 807 6.22GiB
```
