# Hard Questions, Answered

**In one line:** Every sharp question a skeptical engineer (or a curious beginner) would fire at TELELENS, answered honestly, with a pointer to the evidence.

## ELI10

We built a tool that reads your telemetry bill and drafts the fix. A skeptic walks up and pokes it: "Is that '95%' real or a lucky demo? How do I know your fix won't throw away the error I need at 2am? Is it safe to run on my production database? Did an AI make up those dollar numbers?" This page is our honest answers — no bluffing, every answer pointing at a real file or a live-verified fact.

---

## Is the −95% real, or a cherry-picked demo?

It's a **measured before/after receipt from a live ingester**, not a projection — and we publish the unflattering counter-number in the same breath. We applied the generated config to the live SigNoz ingester, ran *identical known-volume* traffic (1,006 requests before, 1,007 after), and span storage went **13,404 → 672, a measured −95.0%.** Then, config still applied, we injected an error storm: loadgen saw 31 HTTP 502s, and ClickHouse stored **31 of 31** error traces. `signoz_calls_total` showed the same traffic on both sides. Then we reverted and verified full collection returned. Evidence: `assets/live-apply-measure-m4-evidence.md`.

**The honest counter:** −95% is a *healthy-traffic* number (the tail-sampling sweet spot). On an error-storm day where 68% of traces carried errors, the same replay reported **only ~6% droppable** — because the policy refuses to drop errors. That refusal is the feature, and the report tells you which regime you're in.

---

## How do I know your fix won't throw away the errors I need?

Because we prove it *before* you apply, and the proof is a property of the code. `telelens simulate` replays the proposed tail-sampling policy over your *real* traces (24 h live, all 162,760 of them in our run) and returns a **binary SAFE/UNSAFE verdict.** A policy that would drop even *one* error or slow trace is rejected and the command **exits non-zero.** That invariant is pinned by a test (`TestSimulateSafetyInvariant`) that hard-fails if any emitted policy would drop an error trace. "Turn on sampling" is a sentence; "here's the replay proving nothing you care about is lost" is the product. (Evidence: `assets/live-simulator-m3-evidence.txt`.)

---

## Is it safe to point at my production database?

Yes, by construction. TELELENS is **read-only** — the profilers issue only ClickHouse `SELECT`s and SigNoz `GET`s, and the live client *refuses any non-SELECT statement in code* (there's a test for it). `.env.example` documents a least-privilege `telelens_ro` ClickHouse user that enforces the same rule at the database, so even a bug can't write. The only things TELELENS writes are files in `out/` that you review; applying them is always a human. During live verification, pointing unfinished profilers at the shared instance was safe the whole time — every failure was a crash, never a corruption.

---

## Did an AI make up the dollar figures?

No — and that's a deliberate design line. The analysis core is **deterministic SQL + Go math with zero LLM in the loop.** Every number in the Waste Report reproduces, and the pricing model is transparent: uncompressed-ingest bytes (documented envelope constants: 300 B/span, 150 B/log record, 32 B/metric sample) × your `--cost-per-gb`, extrapolated to 30 days. Findings that can't be priced reliably ship as **"directional"** rather than inventing a figure. The LLM only appears *optionally*, downstream, narrating `scan --json` — it never computes the bill. A team that asks an LLM "what's expensive?" can't defend a single number; we can defend every one.

---

## Isn't "uncompressed bytes × $/GB" a fake bill?

It's a *transparent model, not a cloud invoice* — and we say so plainly. It's uncompressed-ingest bytes, which is exactly the dimension ingestion-based pricing bills on, times a $/GB you set. The point isn't to reproduce your exact AWS line item; it's to rank offenders by cost and give you a defensible, reproducible magnitude. On the tiny demo instance, read the *ratios*, not the absolute dollars (F-001 alone was 7.8M near-identical success spans). Unpriceable findings are labeled directional so we never launder a guess as a number.

---

## Did it really find a bug in a sibling project?

Yes, and it's the ecosystem story no solo project can tell. Scanning the shared instance, TELELENS priced ARGUS's AI-agent telemetry (35 `gen_ai.*` spans, 204,665 tracked tokens) and GLASSPANE's browser RUM (223 session spans across 38 sessions) — and caught a **real cardinality bug**: GLASSPANE's `browser.sessions.count` metric carried `session.id` as a label, one series per session, unbounded by construction. Their celebrated cardinality hard-guard covered *histograms* and the counter slipped through. The SigNoz MCP's `signoz_check_metric_cardinality` **independently confirmed 37 series.** GLASSPANE extended the guard to all instruments; a live re-test showed the next session created exactly one new series with no `session.id` label. Finding F-024, loop closed. (Evidence: `assets/live-scan-m2-evidence.md`.)

---

## What was the "database observing itself" bug?

The most on-theme bug of the weekend. The first live scan kept getting OOM-killed on a server claiming ~7 GiB of memory pressure that wasn't ours. The culprit: `system.metric_log` (ClickHouse's *own* internal metrics table, ~1300 columns) was merging in a loop — **~80 merges/minute, each peaking 6.2 GiB** — and the OvercommitTracker killed whatever query was running. We also found `system.trace_log` at 1.29 GiB / 76M rows, *larger than all of `signoz_traces`.* A cost profiler found the platform's own introspection to be the biggest waste on the box. Documented system tables with documented knobs; the mitigation was two revertable lines. (Full story: [../02-signoz-deep-dive.md](../02-signoz-deep-dive.md).)

---

## Your own generated config almost broke SigNoz — isn't that damning?

We caught it in our own audit, fixed it, and kept it as the honest centerpiece. The "drop unread metrics" cross-reference correctly noticed 91 metrics no dashboard or alert *queries* — and the generated filter listed `signoz_calls_total` and `signoz_latency.*` among them. Those are the **spanmetrics behind SigNoz's own APM page**: no dashboard reads them by query, *the platform does.* Applying it would have blanked a user's Services view. The fix: a denylist of platform-consumed metrics, and the whole block ships **commented out** behind a review banner. This is exactly why **the operator-review step is the product** — a generated config is a proposal, not a command. We also never applied that block live, because its set included `argus.tokens` and `browser.web_vitals.*`, live evidence for the sibling projects.

---

## Why not just let the tool apply the fix automatically?

Because a cost tool that auto-applies is a liability, and we learned that the hard way. An early version's cardinality findings recommended *deleting* high-cardinality span attributes — including `db.statement`, the exact attribute ARGUS's flagship RCA used as evidence. **A cost tool that auto-deletes the data an investigation needs is worse than useless.** The truth we encoded: high cardinality on *spans* is cheap in this schema; the real crime is an unbounded ID on a *metric label.* So the span findings are now advisory (no generated drop), and TELELENS never applies anything. Know which axis actually costs.

---

## How is this "Best Use of SigNoz" if it's just reading ClickHouse?

Because it's the *deepest schema-level engagement in the field*: raw ClickHouse over `signoz_index_v3`/`logs_v2`/`time_series_v4` **and** the `distributed_samples_v4` datapoint table **and** the undocumented `signoz_meter` DB, `query_range` v5 with `source:"meter"`, the dashboards API (a 3-pack), `/api/v2/rules` guardrails, `meta.rowsScanned` cost-aware querying, and the Collector config applied through Foundry — the mandated install tool used as an *application surface.* And it maps the six official Query Builder showcase cards to real analyses (EXISTS/NOT-EXISTS, `sumIf`/`count_distinct`/`rate`, group-by+having, order-by+limit). Reading one API is table stakes; turning the platform's own storage into a bill is the flex.

---

## Related

- [honest-limits-what-we-dont-claim.md](honest-limits-what-we-dont-claim.md) — the boundaries we deliberately don't cross.
- [newbie-glossary.md](newbie-glossary.md) — every term above, in plain English.
- [../06-bug-hunt.md](../06-bug-hunt.md) — the OOM, the self-contradicting report, and F-024 in full.
- [../DOCS.md](../DOCS.md) — the rollback runbook and the honest caveats.
