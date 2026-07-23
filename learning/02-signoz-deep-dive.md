# SigNoz Deep Dive — everything building TELELENS taught us about SigNoz

**In one line:** To price SigNoz's telemetry we had to learn its ClickHouse schemas from the inside — two query dialects, the undocumented `signoz_meter` DB, the difference between a table that stores *hourly buckets* and one that stores *datapoints* — and the biggest waste on the box turned out to be ClickHouse observing itself.

---

## ELI10

SigNoz keeps all its data in a giant, very fast filing system called ClickHouse. To tell you what's expensive, TELELENS had to open the actual filing cabinets and count. That meant learning which drawer holds what, which labels lie about how much is inside, and — the funniest discovery — that one of the filing system's *own* self-notes was fatter than all the real files combined. This is the map of the cabinets, and where the labels lie.

---

## The surfaces TELELENS uses

TELELENS engages SigNoz at the *schema* level — deeper than reading one API:

| Surface | What TELELENS does | How |
|---|---|---|
| **Traces / logs / metrics** | count volume, bytes, cardinality | raw ClickHouse `SELECT` over `signoz_index_v3`, `logs_v2`, `time_series_v4` |
| **The meter DB** | ingest rate by signal/service | `signoz_meter.*` usage metrics, `query_range` v5 with `source:"meter"` |
| **Dashboards + rules** | find write-only telemetry | `GET /api/v1/dashboards` + `GET /api/v1/rules` |
| **Dashboards (write)** | the Telemetry Bill pack | `POST /api/v1/dashboards` |
| **Alerts (write)** | guardrail rules | `POST /api/v2/rules` |
| **Cost-aware querying** | know what our own scan costs | `meta.rowsScanned` on responses |

Now the sharp edges, in the order they cost us time.

---

## Gotcha 1 — the observability database was observing itself to death

The first live scan died with `MEMORY_LIMIT_EXCEEDED` — and kept dying at *random* places even after every query was made memory-safe. The failing query used 23 MiB; the server's total memory tracker was at **6.98 GiB.** Something else was eating the box.

The culprit: `system.metric_log` — ClickHouse's *internal* metrics table, ~1300 columns, 53k rows, 24 MiB on disk — was merging in a pathological loop: **~80 merges a minute, each peaking at 6.2 GiB**, and the OvercommitTracker killed whatever telemetry query happened to be running. While in there, we found `system.trace_log` at **1.29 GiB across 76M rows — larger than all of `signoz_traces`** on that instance. The observability database spent more on observing itself than on the telemetry it exists to store. A tool built to price telemetry waste found the platform's own introspection to be the biggest waste on the box.

To be fair to ClickHouse: these are documented system tables with documented knobs, and a small demo box on default settings is exactly where a 1300-column log table gets pathological. A tuning lesson, not a scandal. Mitigation was two lines: `SYSTEM STOP MERGES system.metric_log` (instant, revertable with `START MERGES`) + `metric_log` disabled in the pours config for the next restart, backup kept. **Lesson (the same one GLASSPANE learned via `histogramQuantile`): live verification of your feature is an integration test of the platform.**

---

## Gotcha 2 — price from the table that stores the thing

Metric pricing was wrong by *orders of magnitude* at first. We counted "samples" from `time_series_v4` — which stores **one row per hour per series, not per datapoint.** Real datapoint counts come from `distributed_samples_v4`. A finding that prices $0.14/mo off the wrong table can be $8/mo in reality (or vice versa). **Lesson: price from the table that stores the thing you're pricing** — the schema has a table for the label dimension and a *different* table for the sample volume, and confusing them corrupts every dollar figure.

Related: the meter connector (`signoz_meter`) flushes **hourly buckets.** For a sub-hour before/after measurement (like our apply-and-measure run), the meter can't resolve the change in time, so we counted **storage rows** directly instead. Both numbers are in the evidence. Lesson: know your instrument's resolution before you trust a short-window A/B.

---

## Gotcha 3 — two query dialects, and when to use each

SigNoz speaks two query languages, and mastering both is what real query work demands:

- **The v5 Query Builder** (`query_range`, `source:"meter"` for the meter DB) — the high-level, dashboard-friendly form.
- **Raw ClickHouse `SELECT`** — the low-level form, where the profilers live (queries catalogued Q1–Q9 in the implementation plan).

The Telemetry Bill dashboards deliberately *mix both* — v5 builder panels next to raw-ClickHouse panels — because the deep cardinality and duplicate-span analysis simply can't be expressed in the builder, and showing both is the mastery display. Lesson: **the builder is right for portable dashboard panels; raw ClickHouse is right for schema-level forensics** — and a serious cost tool needs both.

---

## Gotcha 4 — `max(status_code=2)=1` is not a bool

A subtle live-only failure: ClickHouse renders `max(status_code=2)=1` as `0`/`1`, and Go's `encoding/json` refuses to decode that into a `bool` — so the simulator's input query failed *only live* (the fixture JSON had real `true`/`false`). The fix: use the schema's native `has_error Bool` column instead of re-deriving error-ness. **Lesson: use the schema's own typed columns rather than re-computing them; a derived integer isn't a boolean on the wire.**

---

## Gotcha 5 — `POST /api/v1/dashboards` wants the object *bare*

A shipping-quality trap: `POST /api/v1/dashboards` wants the dashboard object **bare.** Wrap it in `{data:{...}}` and the API returns **2xx with an id assigned** — and silently stores an *empty* dashboard. The failure mode is a welcome screen, not an error. Related dashboard drift: TEXTBOX variables substitute into ClickHouse SQL as **quoted strings** (`... * '0.30'` → `Illegal types Float64 and String`), so a `$cost_per_gb` variable must be wrapped `toFloat64($var)`. **Lesson: with SigNoz's dashboard API, a 2xx doesn't mean "correct" — verify the stored object renders, not just that the POST succeeded.**

---

## Gotcha 6 — "unread" can't see the platform's own consumers

The scariest near-miss. The usage cross-reference correctly found metrics no dashboard or alert *queries* — and listed `signoz_calls_total` and `signoz_latency.*` among them. Those are the **spanmetrics behind SigNoz's own APM page**: no dashboard "reads" them by query; *the platform does.* Applying our own suggestion would have blanked a user's Services view. The fix: a denylist of platform-consumed `signoz_*` and OTel-SDK internals, and the whole drop-block ships commented out. **Lesson: "written but never read" can only see the consumers you can enumerate (dashboards, alerts) — the platform itself and external agents are invisible readers, so an unread-metric is a *question for a human*, never an automatic drop.**

---

## Gotcha 7 — the MCP tool arg is `metricName`, not `metric_name`

When we cross-checked findings through the official SigNoz MCP server (its `signoz_check_metric_cardinality` independently confirmed our `session.id` label-bomb finding), the tool argument is `metricName`, not `metric_name` — a validation error otherwise. Small, but the kind of thing that costs ten minutes. Lesson: the MCP tool schema uses camelCase arg names.

---

## The good part — SigNoz's schema is deep enough to profile itself

The whole project is only possible because SigNoz stores telemetry in a *queryable* ClickHouse with a coherent schema you can `SELECT` against — separate tables for the label dimension and the sample volume, an undocumented-but-real `signoz_meter` usage DB, native typed columns like `has_error`, and `meta.rowsScanned` on every response so a cost tool can measure *its own* query cost. And the guardrail loop closes: TELELENS writes back three Telemetry Bill dashboards and three alert rules (ingest-spike, cardinality-explosion, log-anomaly) via the same APIs, so waste can't silently return after a cleanup. **Lesson: the deepest "Best Use of SigNoz" isn't reading one endpoint — it's engaging the storage schema honestly enough to turn the platform's own data into a bill.**

---

## Related

- [01-how-it-works.md](01-how-it-works.md) — where each query and API gets used.
- [06-bug-hunt.md](06-bug-hunt.md) — the OOM hunt and the apply/revert story in full.
- [04-faq/newbie-glossary.md](04-faq/newbie-glossary.md) — ClickHouse, cardinality, tail sampling, meter, spanmetrics explained.
- [../DOCS.md](../DOCS.md) — the live-loop commands and the rollback runbook.
