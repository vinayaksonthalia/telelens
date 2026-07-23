# Verbatim ClickHouse OOM error — the line that opens the blog

**What this proves:** the blog and LEARNINGS.md open with "the first live scan died
with `MEMORY_LIMIT_EXCEEDED` … the query that failed was using 23 MiB … the server's
total memory tracker was at 6.98 GiB." This file is the raw evidence for that line,
captured directly from the live SigNoz ClickHouse's own `system.query_log` (not
reconstructed from memory).

## How this was captured (reproducible)

ClickHouse reached live on 2026-07-18 through the documented `telelens-ch-proxy`
socat container (HTTP interface on `localhost:8123`):

```bash
curl -s "http://localhost:8123/?default_format=Vertical" --data-binary \
  "SELECT event_time, memory_usage, read_rows, query, exception
   FROM system.query_log
   WHERE exception_code = 241        -- MEMORY_LIMIT_EXCEEDED
     AND event_time > now() - INTERVAL 3 DAY
   ORDER BY event_time DESC"
```

`exception_code = 241` is ClickHouse's `MEMORY_LIMIT_EXCEEDED`. In the trailing
3-day window this returned **112 OOM events** (first `2026-07-18 00:00:45`, last
`2026-07-18 06:19:22`) — consistent with the pathological `system.metric_log`
merge loop documented in LEARNINGS.md that was killing
whatever telemetry query happened to be running.

## The verbatim error line (as ClickHouse emitted it)

```
Code: 241. DB::Exception: (total) memory limit exceeded: would use 6.98 GiB
(attempt to allocate chunk of 4.01 MiB), current RSS: 2.69 GiB, maximum: 6.98 GiB.
OvercommitTracker decision: Query was selected to stop by OvercommitTracker:
While executing AggregatingTransform. (MEMORY_LIMIT_EXCEEDED)
(version 25.12.5.44 (official build))
```

## Where the blog's two numbers come from (both evidenced)

- **"6.98 GiB — the server's total memory tracker."** This is the `would use` /
  `maximum` figure in every one of the 112 error lines: the *server-wide* (`(total)`)
  limit, not the query's own footprint. Identical across all rows.
- **"the query that failed was using ~23 MiB."** This is the failing query's own
  `memory_usage`. Across the OOM rows it clustered at **19–26 MiB**
  (representative rows: 19.00, 23.28, 24.43, 26.07 MiB) — i.e. the profiler's query
  was tiny; the box was already pinned at its total limit and the OvercommitTracker
  picked whatever telemetry query was running as the victim. "~23 MiB" is a faithful
  mid-range figure for that column.

The failing queries were exactly the shape TELELENS issues — e.g. a per-service log
scan that read 2.86M rows before the tracker killed it:

```
event_time: 2026-07-18 05:05:00   query_mib: 78.14   read_rows: 2,863,873
query: SELECT resources_string['service.name'] AS service, severity_text,
       count() AS count, sum(length(body)) AS bytes
       FROM signoz_logs.distributed_logs_v2 WHERE ts_bucket_start >= ...
exception: Code: 241. DB::Exception: (total) memory limit exceeded: would use
           6.98 GiB (attempt to allocate chunk of 4.36 MiB), current RSS: 2.19 GiB,
           maximum: 6.98 GiB. OvercommitTracker decision: ... While executing
           MergeTreeSelect(...). (MEMORY_LIMIT_EXCEEDED) (version 25.12.5.44)
```

(That larger-footprint row — 78 MiB / 2.86M rows — is the un-time-sliced log-body
scan later fixed by the 1-day sampling window; see LEARNINGS.md and
`internal/store/clickhouse.go` retry/slice logic.)

_Captured 2026-07-18 from the live reference instance's `system.query_log`. The blog's
opening figures (23 MiB query / 6.98 GiB server total) are supported verbatim; no blog
edit was needed._
