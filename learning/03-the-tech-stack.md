# The Tech Stack — every choice, and the honest trade-off

**In one line:** A single static Go binary with one UI dependency, a read-only store seam that swaps fixtures for live ClickHouse, a deterministic SQL-and-math core with *zero LLM in the analysis loop*, the Drain template miner, and golden-file-tested config generators — each choice made so every dollar figure reproduces and the tool is safe to point at production.

---

## ELI10

Building TELELENS is like designing a very trustworthy accountant. It has to add up your bill the *same way every time* (no guessing, no "the AI thinks it's about $40"), it has to be safe to let into your house (it can look at everything but touch nothing), and it has to work whether or not you're connected to the real bank. This page walks each part and says, honestly, why — and what we gave up.

---

## The choices, at a glance

| Decision | Choice | Why | The trade-off we accepted |
|---|---|---|---|
| Language / runtime | **Go ≥ 1.24, single static binary** | spec-fit; one file to ship, easy to run anywhere | — |
| UI dependency | `charmbracelet/lipgloss` only | design-system terminal output; everything else is stdlib | one dependency, chosen deliberately |
| Analysis core | **deterministic SQL + Go math, zero LLM** | every number reproduces and survives scrutiny | no natural-language "insights" without a bolt-on |
| Data access | one `TelemetryStore` interface, `fixtures` \| `clickhouse` | offline demo + live, identical profilers | live fixes must land in the store, not profilers |
| Safety | read-only by construction (SELECT-only guard) | safe to point at production | can't auto-apply fixes (by design) |
| Log analysis | Drain-style template miner | "one template is half your bytes" needs clustering, not grep | a heuristic miner, table-tested |
| Config generation | annotated otelcol fragment + Foundry casting patch, golden-file tested | the fix is a reviewable proposal | we generate, a human applies |
| Simulator | policy replay over per-trace summaries, hard safety invariant | prove sampling safe before apply | models 3 policy types, not a packet-level collector |

---

## The load-bearing decision: a deterministic, LLM-free core

The single most important choice was **keeping the LLM out of the analysis loop entirely.** Cost findings are SQL plus arithmetic with unit tests — so every number in the Waste Report reproduces, and every figure survives someone asking "where did that come from?" A tool that asks an LLM "what's expensive?" cannot defend a single dollar under questioning.

Where does the LLM go, then? *Only narration, and only optionally.* `telelens scan --json` prints the full findings report as pure JSON on stdout (progress to stderr) — that's the agent surface. An agent can read it, summarize it, or cross-check a finding through the SigNoz MCP, but **telelens computes; the LLM only narrates.** The division of labor is deliberate and disclosed. (This also answers the official idea-board card #11666's "data-quality checker evaluated via MCP" — the quality findings are pure, deterministic, and MCP-consumable.)

**The honest trade-off:** no free-form "insights." If you want a paragraph of narration, you pipe `--json` to a model yourself. For a tool whose entire value is *trustworthy numbers*, that's the right call — the moment an LLM computes the bill, the bill stops being defensible.

---

## The store seam (why every live bug landed safely)

Everything reads through one `TelemetryStore` interface with two backends: `fixtures` (committed recorded query results) and `clickhouse` (live). This one seam bought three things:

1. **Offline everything** — `scan --fixtures` runs the whole pipeline with no SigNoz, so the demo and the tests need no infrastructure.
2. **Fixtures/live parity** — every live fix (time-slicing heavy queries, the `has_error` bool fix, the metric-pricing table swap) landed *in the store*, without touching a single profiler. The store absorbed reality.
3. **Safety by construction** — the live client refuses any non-`SELECT` statement in code, so every live failure was a *crash, never a corruption.* Pointing an unfinished profiler at the shared demo instance was safe the whole time. `.env.example` documents a least-privilege `telelens_ro` ClickHouse user that enforces the same rule at the database.

---

## Handling scale: fixtures lied in both directions

A tech-stack lesson that shaped real code. The fixtures were *small*, so two things bit only live:

- **Heavy queries blew the server memory limit.** The 24 h `GROUP BY trace_id` over 8.9M spans and a 7-day attribute `ARRAY JOIN` (~100M expanded rows) each OOM'd as single queries. Fix: **time-slice and merge in Go** (3 h / 1-day chunks; `max` and `sum` are associative, and averages are recomputed from summed bytes so the merge is *exact*), plus `SETTINGS max_threads=2, max_bytes_before_external_group_by` on every heavy query, plus a bounded retry for transient ingest-flush pressure.
- **Pricing was wrong by orders of magnitude** because the fixture counts came from the wrong table (hourly buckets, not datapoints — see [02-signoz-deep-dive.md](02-signoz-deep-dive.md)).

Lesson baked into the stack: **fixtures are for correctness, not for scale.** The store layer is where scale reality lives, so the profilers stay simple.

---

## The simulator: a safety invariant as an exit code

The tail-sampling simulator reduces traces to per-trace summaries and replays the proposed policy: keep all errors, keep everything over the latency threshold, hash-sample the rest deterministically. Its verdict is **binary**, and — the design choice that matters — it's expressed as an **exit code**. A policy that would drop one error or slow trace makes `telelens simulate` exit non-zero, which makes the whole apply step *scriptable without being scary.* The invariant is pinned by a test (`TestSimulateSafetyInvariant`). Honest boundary: it faithfully models the 3 shipped `tail_sampling` policy types over trace summaries — it's not a packet-level collector emulation, and it can't model `decision_wait` timing for traces longer than the window (documented in the rollback runbook).

---

## The testing strategy (sketch)

Everything runs offline (`go test ./...`), and the suite is the reason the numbers are trustworthy:

- **Analyzers:** table-driven tests for the Drain miner, the pricing math, and the simulator — including the safety battery that hard-fails if any emitted policy would drop an error trace.
- **Profilers:** run over the fixture corpus asserting that each *engineered* waste pattern is found — *and* that healthy telemetry is **not** flagged (avoiding false positives is as tested as finding real ones).
- **Generators:** golden-file tests for the collector fragment and the casting patch (`testdata/golden/`, refreshed with `go test ./internal/generate -update`).
- **Store:** a SELECT-only-guard test proving the live client refuses non-`SELECT` statements.

The result is a deterministic core where, as we saw live, the agent demo's "top action" narration matched the transform TELELENS had *already* generated and measured — because both came from the same reproducible numbers.

---

## Related

- [01-how-it-works.md](01-how-it-works.md) — where each component runs in the pipeline.
- [02-signoz-deep-dive.md](02-signoz-deep-dive.md) — the ClickHouse realities the store seam absorbs.
- [../internal/store/clickhouse.go](../internal/store/clickhouse.go) — the query catalogue (Q1–Q9) in code.
- [04-faq/honest-limits-what-we-dont-claim.md](04-faq/honest-limits-what-we-dont-claim.md) — where these trade-offs bite.
