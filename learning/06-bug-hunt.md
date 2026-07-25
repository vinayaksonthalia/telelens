# Bug Hunt — the war diary

**In one line:** Every real bug pointing TELELENS at a live instance surfaced — an OOM caused by the *database observing itself*, a report that contradicted itself and auto-generated a harmful fix, a config that would have blanked SigNoz's own APM page — each with symptom → hunt → root cause → fix → lesson.

## ELI10

The useful part of a project isn't the shiny demo; it's the list of walls and how we got through each one. Every profiler worked perfectly on the practice data (fixtures) — and half of them fell over the moment they touched a *real* SigNoz. The funniest wall: the biggest waste on the whole box turned out to be the database's own self-notes. Here's the whole diary.

---

## Bug 1 — the OOM caused by the database observing itself

- **Symptom:** The first live scan died with `MEMORY_LIMIT_EXCEEDED`, and kept dying at *random* places even after every query was made memory-safe.
- **The hunt:** The query that failed was using 23 MiB; the server's total memory tracker was at **6.98 GiB.** So the killer wasn't ours — our query was just the process standing nearby when the OvercommitTracker picked a victim. We looked at what ClickHouse itself was doing: `SELECT table, count(), max(peak_memory_usage) FROM system.part_log WHERE event_type='MergeParts' …`.
- **Root cause:** `system.metric_log` — ClickHouse's *internal* metrics table, ~1300 columns, 53k rows, 24 MiB on disk — was merging in a loop: **~80 merges a minute, each peaking 6.2 GiB.** (Bonus find: `system.trace_log` at 1.29 GiB / 76M rows — *larger than all of `signoz_traces`*.) The observability database spent more observing itself than storing the telemetry it exists for.
- **The fix:** `SYSTEM STOP MERGES system.metric_log` (instant, revertable with `START MERGES`) + `metric_log` disabled in the ClickHouse config for the next restart (backup kept, revert commands written into the config's own comment block). After that the scan ran clean in **3.74 s** (`assets/live-scan-transcript.txt`).
- **Lesson:** **Live verification of your feature is an integration test of the platform** (the exact lesson GLASSPANE learned via `histogramQuantile`). These are documented system tables with documented knobs; a small demo box on defaults is where a 1300-column log table gets pathological — a tuning lesson, not a scandal.

---

## Bug 2 — fixtures lied about scale in *both* directions

- **Symptom:** Two heavy queries OOM'd live that were fine on fixtures; and metric pricing was wrong by orders of magnitude.
- **Root cause (too big):** the 24 h `GROUP BY trace_id` over 8.9M spans and a 7-day attribute `ARRAY JOIN` (~100M expanded rows) each blew the server memory limit as single queries.
- **Root cause (wrong number):** metric pricing counted "samples" from `time_series_v4`, which stores **one row per hour per series, not per datapoint.** Real counts live in `distributed_samples_v4`.
- **The fix:** **time-slice and merge in Go** (3 h / 1-day chunks; `max`/`sum` are associative, averages recomputed from summed bytes so the merge is *exact*) + `SETTINGS max_threads=2, max_bytes_before_external_group_by` on every heavy query + a bounded retry for transient ingest-flush pressure. And **price from the table that stores the thing** (`distributed_samples_v4`).
- **Lesson:** **fixtures are for correctness, not for scale** — the store layer is where scale reality lives, and a price is only as right as the table you count.

---

## Bug 3 — `max(status_code=2)=1` is not a bool

- **Symptom:** The simulator's input query failed — but *only live.*
- **Root cause:** ClickHouse renders `max(status_code=2)=1` as `0`/`1`, and Go's `encoding/json` refuses to decode that into a `bool`. The fixture JSON had real `true`/`false`, so it passed offline.
- **The fix:** use the schema's native `has_error Bool` column instead of re-deriving error-ness.
- **Lesson:** **use the schema's own typed columns rather than re-computing them** — a derived integer isn't a boolean on the wire.

---

## Bug 4 — our own report contradicted itself and auto-generated a *harmful* fix

- **Symptom:** The span-attribute-cardinality findings recommended `delete_key` for high-cardinality attributes — including `db.statement`, *the exact attribute ARGUS's flagship RCA used as evidence* — while a different finding (F-025) *praised* IDs living on spans "where cardinality is free." Both can't be right.
- **Root cause:** we'd conflated two different cost axes. High cardinality on **spans** is cheap in this schema; the real, expensive crime is an unbounded ID on a **metric label.**
- **The fix:** the span-cardinality findings became *advisory* (no generated drop), and a new **ecosystem profiler** flags the actual structural crime — an unbounded ID used as a metric label. It promptly found one in our own sibling (see Bug 5).
- **Lesson:** **a cost tool that auto-deletes the evidence an investigation needs is a liability — know which axis actually costs.** This is why TELELENS never auto-applies anything.

---

## Bug 5 — the label bomb TELELENS found in a sibling project (F-024)

- **Symptom:** Nothing on our side — this was a bug in *GLASSPANE*, found while pricing its telemetry on the shared instance.
- **The hunt:** The ecosystem profiler priced GLASSPANE's browser RUM (223 session spans, 38 sessions) and flagged `browser.sessions.count` as a structural label bomb: one new time series *per visitor.*
- **Root cause:** GLASSPANE's celebrated cardinality hard-guard covered *histograms* — and the counter slipped through, carrying `session.id` as a label, unbounded by construction. The SigNoz MCP's `signoz_check_metric_cardinality` **independently confirmed 37 series.**
- **The fix (theirs):** GLASSPANE extended the guard to *all* metric instruments; a live re-test showed the next session created exactly one new series, with no `session.id` label. F-024, loop closed.
- **Lesson, doubled:** **(1)** a carve-out in a principle is a bug waiting for scale; **(2)** **cross-project telemetry review works** — a tool built to find *other people's* waste found real waste in a sibling, reproducibly, via the platform's own MCP.

---

## Bug 6 — "91 findings is zero findings"

- **Symptom:** The unread-metrics cross-reference worked perfectly and produced **91 near-identical rows that drowned everything else.**
- **Root cause:** correct analysis, terrible presentation — a wall of undifferentiated findings is unusable.
- **The fix:** top-5 priced individually + one roll-up finding (the generated collector filter still lists every name).
- **Lesson:** **ranking is a product feature, not a formatting choice.** A cost report you can't act on is noise.

---

## Bug 7 — the generated config that would have blanked SigNoz's APM page

- **Symptom:** An audit of our *own* generated config found the scariest bug of the project: the "drop unread metrics" filter listed `signoz_calls_total` and `signoz_latency.*`.
- **Root cause:** those are the **spanmetrics behind SigNoz's own APM page.** No dashboard "reads" them by query — *the platform does.* Applying our own suggestion would have blanked a user's Services view.
- **The fix:** a denylist of platform-consumed `signoz_*` and OTel-SDK internals, and the whole drop block now ships **commented out** behind a review banner. (We also never applied it live: its set included `argus.tokens` and `browser.web_vitals.*`, live evidence for the sibling projects.)
- **Lesson:** **"unread" can only see the consumers you can enumerate** — the platform itself and external agents are invisible readers, so an unread metric is a *question for a human*, never an auto-drop. **The review step is the product.**

---

## Bug 8 — dashboard/alert schema drift (the shipping traps)

- **Symptom:** A `POST /api/v1/dashboards` returned 2xx with an id — and silently stored an *empty* dashboard.
- **Root cause:** the API wants the dashboard object **bare**; wrapping it in `{data:{...}}` "succeeds" and stores nothing. (Two siblings: TEXTBOX variables substitute into ClickHouse SQL as *quoted strings*, so `$cost_per_gb` needs `toFloat64($var)`; and the MCP tool arg is `metricName`, not `metric_name`.)
- **The fix:** POST the bare object; wrap numeric variables in `toFloat64`; use the right MCP arg name.
- **Lesson:** **with SigNoz's dashboard API, a 2xx doesn't mean "correct"** — verify the stored object renders, not just that the POST succeeded. The failure mode is a welcome screen, not an error.

---

## The apply/revert story (how we measured −95% on healthy traffic, ~6% on an error-storm day, without breaking siblings)

The payoff that ties the diary together. Applied to the live ingester: `tail_sampling` (5% boring successes, 100% errors + slow kept, placed **AFTER `signozspanmetrics`** so RED metrics keep seeing everything) + the `session.id` label transform. Deliberately **NOT** applied: `filter/drop_unread_metrics`, because its "unread" set included `argus.tokens` and `browser.web_vitals.*` — live evidence for the sibling projects. Identical 1,006-request loadgen windows: **13,404 → 672 spans stored (−95.0%)** — the healthy-traffic figure; the whole-instance replay of the same policy on an error-storm day drops only **~6%**, because it refuses to drop errors ([`assets/live-apply-measure-m4-evidence.md`](../assets/live-apply-measure-m4-evidence.md)); injected error storm under sampling: **31/31 error traces kept**; `signoz_calls_total` unchanged. Then reverted from the backup and re-verified full collection (4,059 spans/90 s, matching the before-rate). Lessons: **the operator-review step is the product** (a generated config is a proposal), and **put spanmetrics before tail_sampling** or your RED metrics get sampled too. (The meter connector flushes hourly, so for the sub-hour A/B we counted storage rows — the honest instrument.)

---

## The meta-lesson

Look at the pattern: Bugs 1, 2, 3, and 8 were all the same shape — **fixtures passed, the live platform disagreed**, and only pointing the tool at a *real* instance caught it. Bugs 4, 6, and 7 were the same shape *turned inward* — our own analysis or output had a flaw that only real scrutiny (or a real target) revealed. And Bug 5 was that shape applied to a *sibling.* The recurring lesson of the whole build: **mock tests never mark anything done.** The read-only-by-construction design is what made all this exploration *safe* — every live failure was a crash, never a corruption.

---

## Related

- [01-how-it-works.md](01-how-it-works.md) — the pipeline these bugs live in.
- [02-signoz-deep-dive.md](02-signoz-deep-dive.md) — the ClickHouse schema realities (Bugs 1, 2, 3, 8) in detail.
- [04-faq/honest-limits-what-we-dont-claim.md](04-faq/honest-limits-what-we-dont-claim.md) — the limits these bugs revealed.
- [../LEARNINGS.md](../LEARNINGS.md) — the original engineering notes.
