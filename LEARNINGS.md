# LEARNINGS.md — what pointing TELELENS at a real instance actually taught us

_First-person notes from the live-verification wave (July 18, 2026). Raw
material for the blog; failures are the useful part._

## The one-sentence summary

Every profiler worked on fixtures and half of them fell over on the first real
scan — and the single biggest thing wrong with the live observability database
turned out to be its own observability.

## Things that failed, and what the failure taught us

### ClickHouse's own telemetry was the biggest resource consumer on the box

The first live scan died with `MEMORY_LIMIT_EXCEEDED` — and kept dying at
random places even after we made every query memory-safe. The query that
failed was using 23 MiB; the server's total memory tracker was at 6.98 GiB.
The culprit: `system.metric_log` (ClickHouse's internal metrics table, ~1300
columns, 53k rows, 24 MiB on disk) was merging in a pathological loop —
**~80 merges/minute, each peaking 6.2 GiB** — and the OvercommitTracker killed
whatever telemetry query happened to be running. Bonus find: `system.trace_log`
was **1.29 GiB / 76M rows — larger than all of `signoz_traces`**. The
observability database spent more on observing itself than on the telemetry it
exists to store. A profiler that prices telemetry waste found the platform's
introspection to be the waste. Mitigation: `SYSTEM STOP MERGES
system.metric_log` (instant, revertable) + metric_log disabled in the pours
ClickHouse config (takes effect on restart; backup file kept next to it).
Lesson (same one GLASSPANE learned via histogramQuantile): **live verification
of your feature is an integration test of the platform.**

### Fixtures lied about scale in both directions

- The 24h `GROUP BY trace_id` for the simulator (8.9M spans) and the 7-day
  attribute `ARRAY JOIN` (~100M expanded rows) each blew the server memory
  limit as single queries. Fix: **time-slice and merge in Go** (3h/1-day
  chunks; max/sum are associative, averages computed from summed bytes so the
  merge is exact) + `SETTINGS max_threads=2, max_bytes_before_external_group_by`
  on every heavy query + a bounded retry for transient ingest-flush pressure.
- Metric pricing was wrong by orders of magnitude the other way: we counted
  "samples" from `time_series_v4`, which stores one row per **hour** per
  series, not per datapoint. Real counts come from `distributed_samples_v4`.
  A finding that prices $0.14/mo on fixtures can be $8/mo in reality — or vice
  versa. **Price from the table that stores the thing you're pricing.**

### `max(status_code=2)=1` is not a bool

ClickHouse renders it as `0`/`1`, `encoding/json` refuses to decode that into
a Go `bool`, and the whole simulator input query fails — but only live,
because the fixture JSON had real `true`/`false`. The index table has a native
`has_error Bool`; use the schema instead of re-deriving it.

### Our own report contradicted itself (and auto-generated a harmful fix)

The span-attribute-cardinality findings recommended `delete_key` for
high-cardinality attributes — including `db.statement`, the exact attribute
ARGUS's flagship RCA used for evidence — while finding F-025 praised IDs
living on spans "where cardinality is free". Both can't be right. The truth:
high cardinality on SPANS is cheap in this schema; it's a label-on-METRICS
problem. The findings are now advisory (no generated drop), and the new
ecosystem profiler flags the actual structural crime: **an unbounded ID used
as a metric label** — which it promptly found in our own sibling project
(GLASSPANE's `browser.sessions.count` carries `session.id`; its celebrated
hard-guard covers histograms only). **A cost tool that auto-deletes the
evidence an investigation needs is a liability; know which axis actually
costs.**

### 91 findings is zero findings

The unread-metrics cross-reference worked perfectly and produced 91
near-identical rows that drowned everything else. Top-5 priced individually +
one roll-up finding (the generated collector filter still lists every name).
**Ranking is a product feature, not a formatting choice.**

### The apply/revert story (how we measured −95% without breaking siblings)

Applied to the live ingester: `tail_sampling` (5% boring successes, 100%
errors + slow kept, placed AFTER `signozspanmetrics` so RED metrics keep
seeing everything) + the session.id label transform. Deliberately NOT applied:
`filter/drop_unread_metrics` — the "unread" set included `argus.tokens` and
`browser.web_vitals.*`, live evidence for the sibling projects. Identical
1,006-request loadgen windows: 13,404 → 672 spans stored (−95.0%); injected
error storm under sampling: 31/31 error traces kept; `signoz_calls_total`
unchanged. Then reverted from the backup and re-verified full collection.
Lessons: **the operator-review step is the product** (a generated config is a
proposal), and **put spanmetrics before tail_sampling** or your RED metrics
get sampled too. Also: the meter connector flushes hourly — for sub-hour
A/B measurement, storage-row counts are the honest instrument.

### Dashboard/alert schema drift, continued (GLASSPANE's list grows)

- `POST /api/v1/dashboards` wants the dashboard object **bare**; wrapping it
  in `{data:{...}}` succeeds (2xx, id assigned!) and silently stores an empty
  dashboard. The failure mode is a welcome screen, not an error.
- TEXTBOX dashboard variables substitute into ClickHouse SQL as **quoted
  strings**: `... * '0.30'` → `Illegal types Float64 and String`. Wrap with
  `toFloat64($var)`.
- The MCP tool arg is `metricName`, not `metric_name` — validation error
  otherwise.

### Permissions are part of the runbook

Container restarts were blocked for one service and allowed for another by the
session's permission layer. The ClickHouse metric_log config fix is staged on
disk but needs an operator restart to take effect. **Write the revert/finish
commands down at the moment you stage a change, not later** — ours are in the
config file's own comment block.

## Design decisions that held up

- **Read-only by construction** — the SELECT-only guard meant every live
  failure was a crash, never a corruption. Pointing an unfinished profiler at
  the shared demo instance was safe the whole time.
- **Deterministic core** — every number in the report reproduces; when the
  agent demo (M6) narrated the findings, its "top action" matched the
  transform we had already generated and measured.
- **Fixture/live behind one interface** — every live fix landed without
  touching a single profiler; the store absorbed reality.
- **The safety invariant as an exit code** — `simulate` returning non-zero on
  UNSAFE made the apply step scriptable without being scary.

## If you're joining this codebase

Start with `assets/README.md` (evidence index), then run Path 1 in DOCS.md.
`go build ./... && go test ./...` must be green (38 tests across 7 packages)
before you believe anything else.
