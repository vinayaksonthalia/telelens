<div align="center">

<img src="assets/brand/logo-1200.png" alt="TELELENS — telemetry cost and cardinality profiler for SigNoz" width="420">

# TELELENS

**A telemetry cost & cardinality profiler for SigNoz: it finds the waste, generates the fix, and proves it drops zero error traces first.**

**−95.0% span storage measured on a live ingester** (honest counterweight: only **~6%** on an error-storm day — the policy refuses to drop errors) · **14.1M spans profiled in 895 ms** · **31 / 31 error traces kept** · **41 tests, 7 packages, 86 subtests** · **read-only by design** · **MIT**

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go 1.24+](https://img.shields.io/badge/go-1.24%2B-00ADD8.svg)](go.mod)
[![Last commit](https://img.shields.io/github/last-commit/vinayaksonthalia/telelens.svg)](https://github.com/vinayaksonthalia/telelens/commits)
[![Repo size](https://img.shields.io/github/repo-size/vinayaksonthalia/telelens.svg)](https://github.com/vinayaksonthalia/telelens)

[See it work](#see-it-work) · [The 15-minute tour](#the-15-minute-tour) · [Quickstart](#quickstart) · [Profilers](#what-the-profilers-find) · [Architecture](#architecture) · [Status](#honest-status) · [Compatibility](#compatibility--uninstall) · [Learn](#learn)

</div>

## See it work

<p align="center">
  <img src="assets/scan.gif" alt="Terminal recording of telelens scan against a live SigNoz instance: six profilers report in, a severity-coloured findings table appears with 2.5 GB/month of identified savings, and a sampling-simulator replay over 75,586 real traces ends in a green SAFE verdict — 895 ms end to end." width="100%">
</p>

One command, 895 ms, against a live SigNoz instance holding 14.1M spans: six profilers, a ranked bill in GB/month and dollars, and a replay of the recommended sampling policy over 75,586 real traces that keeps every error and slow trace. Nothing was written to the cluster.

Cutting telemetry blindly is scary — drop the wrong span and you are blind during the next incident. TELELENS is observability *for* your observability: it ranks the waste with hard numbers, generates the exact OpenTelemetry Collector config that fixes it, and **proves by replay that the fix keeps every error and slow trace before you ever apply it.** It writes files to `out/`; a human reviews the diff and applies it.

The **Savings Tracker** dashboard is the other half of the proof — ingest falls off a cliff when the generated config lands, while the error-span panel stays flat.

[<img src="assets/screenshots/m5-03-savings-tracker.png" alt="SigNoz 'Telemetry Bill — Savings Tracker' dashboard: the total ingest-rate cliff chart drops to near zero after the generated config is applied, a 13.61 GB-in-window counter, and an error-spans-per-service safety panel that stays intact." width="100%">](assets/screenshots/m5-03-savings-tracker.png)

**Measured, not projected** — applied to a live SigNoz ingester, then reverted and the revert verified:

| Result | Measured | Evidence |
|---|---|---|
| Span storage cut on healthy known-volume traffic | **−95.0%** | [`assets/live-apply-measure-m4-evidence.md`](assets/live-apply-measure-m4-evidence.md) |
| Honest counterweight: droppable on an error-storm day | **~6%** (the policy refuses to drop errors) | same |
| Injected error traces kept | **31 / 31** (live) · **40 / 40** (fixtures) | same |
| Simulator replayed over real last-24h traces | **162,760 traces — SAFE** | [`assets/live-simulator-m3-evidence.txt`](assets/live-simulator-m3-evidence.txt) |

Console capture behind the recording: [`assets/live-scan-2026-07-25.txt`](assets/live-scan-2026-07-25.txt); [`assets/README.md`](assets/README.md) indexes the rest.

## The story

<p align="center">
  <img src="assets/illustrations/05-the-story.png" alt="Four panels: a climbing storage bill; one telelens scan ranking the waste; the sampling valve keeping every error and slow trace, verdict SAFE; span storage dropping 95 percent on healthy traffic (~6% on an error-storm day), measured not projected." width="900">
</p>

## The 15-minute tour

Copy-pasteable, in order. **Steps 1–6 need no SigNoz, no Docker, no network** — everything runs against the committed fixture corpus; the live path (7–8) is optional.

**1. Clone and build** — one static binary, one non-stdlib dependency.

```bash
git clone https://github.com/vinayaksonthalia/telelens
cd telelens
go build -o telelens ./cmd/telelens
```
> *Expect:* no output; `./telelens --help` lists `scan`, `simulate`, `report`, `generate`.

**2. Prove the suite is green** — offline.

```bash
go test ./...
```
> *Expect:* `ok` for 7 packages, including `TestSimulateSafetyInvariant` in `internal/analyze`.

**3. Find the waste** — the ranked bill, in colour, in well under a second.

```bash
./telelens scan --fixtures
```
> *Expect:* six profilers, a severity-coloured table, then `Total identified savings: 130.2 GB/month ≈ $39.06/month` and `scan completed in 6ms · outputs in out/`.

**4. Prove the fix is safe** — the policy is replayed trace by trace before anything is applied.

```bash
./telelens simulate --fixtures --sample-pct 5 --latency-ms 750
```
> *Expect:* `error traces: 40 / 40 kept`, `slow traces: 55 / 55 kept`, `verdict: SAFE`. One dropped error or slow trace prints UNSAFE and exits non-zero ([how the invariant is enforced](DOCS.md#the-safety-proof-in-detail)).

**5. Read the generated fix** — this is the artifact you would actually merge.

```bash
./telelens report   --findings out/findings.json    # re-renders out/waste-report.md
./telelens generate --findings out/findings.json    # re-renders the collector fragment + casting patch
sed -n '40,60p' out/collector-fragment.yaml
```
> *Expect:* `wrote out/collector-fragment.yaml` / `wrote out/casting-patch.yaml`, and a `filter/drop_unread_metrics` block shipped **deliberately commented out** under a review banner — the review step is the product ([why](DOCS.md#try-it-yourself)).

**6. Take the agent surface** — the same findings as stable JSON on stdout, progress on stderr.

```bash
./telelens scan --fixtures --json | jq '.findings[] | select(.category=="quality")'
```
> *Expect:* well-formed JSON objects; `NO_COLOR=1 ./telelens scan --fixtures | cat` degrades cleanly too.

**7. (Optional) Point it at your own SigNoz** — read-only: ClickHouse `SELECT`s and SigNoz `GET`s only.

```bash
cp .env.example .env    # CLICKHOUSE_HTTP_URL, SIGNOZ_API_URL, SIGNOZ_API_KEY, COST_PER_GB
set -a; source .env; set +a
./telelens scan --window 7 --cost-per-gb 0.30
```
> *Expect:* `telelens scan · source: clickhouse (…, window=7d)` and a real ranked bill. Reaching ClickHouse and the least-privilege `telelens_ro` user: [DOCS § Path 2](DOCS.md#path-2--full-live-loop-against-your-signoz).

**8. (Optional) Import the dashboards and guardrails.**

```bash
export SIGNOZ_URL=http://localhost:8080 SIGNOZ_API_KEY=...
./dashboards/import.sh
```
> *Expect:* 3 dashboards + 3 alert rules created, including the Savings Tracker shown above ([what is in the pack](DOCS.md#dashboards--guardrail-alerts)).

## Quickstart

Requires Go ≥ 1.24. One static binary; the only non-stdlib dependency is charmbracelet/lipgloss.

```bash
git clone https://github.com/vinayaksonthalia/telelens
cd telelens
go build ./cmd/telelens

# Demo mode — no SigNoz needed. Scans the committed "Noisy Neighborhood"
# fixture corpus (every classic waste pattern, recorded as query results):
./telelens scan --fixtures

# Outputs land in out/:
#   waste-report.md          ranked findings with evidence, GB/mo and $
#   findings.json            machine-readable report
#   collector-fragment.yaml  annotated otelcol processors (tail_sampling, filter, transform)
#   casting-patch.yaml       the same fix as a Foundry ingester.config.data patch
```

Once the repo is public, `go install github.com/vinayaksonthalia/telelens/cmd/telelens@latest` installs the CLI directly. Re-render a previous scan without re-profiling via `telelens report` / `telelens generate` (step 5). Full flag and exit-code reference: [DOCS § Command reference](DOCS.md#command-reference).

## What the profilers find

| Profiler | Findings |
|---|---|
| **traces** | volume+bytes by service · attribute cardinality offenders (`user.id` as a span attribute) · jumbo attributes (4 KB `db.statement` on every span) · near-duplicate success spans (tail-sampling candidates) |
| **logs** | Drain-style template mining ("this one DEBUG line is half your log bytes") · per-service severity distribution · exact-duplicate line detection |
| **metrics** | per-metric series cardinality · **which label** is the bomb (`user_id` → 210k series) · stale/silent metrics |
| **usage-xref** | metrics **written but read by no dashboard or alert** — diffed against `GET /api/v1/dashboards` + `GET /api/v1/rules`, walked across schema versions |
| **quality** | missing `service.name` · missing/non-standard/miscased log severity · non-conformant metric names — unqueryable data is waste at any size |
| **ecosystem** | cost attribution for modern workloads: prices AI-agent telemetry (`gen_ai.*` spans + tokens) and browser-RUM sessions (`session.id` spans, KB/session); flags any unbounded ID used as a **metric label** as structural at ANY series count |

Pricing is deliberately transparent — uncompressed-ingest bytes × 30 days × `--cost-per-gb`; unpriceable findings ship as **directional** ([model](DOCS.md#pricing-model)). Every profiler query lives in `internal/store/`, and together they exercise all six SigNoz Query Builder showcase capabilities ([mapping](DOCS.md#query-builder-coverage)).

## Architecture

```
      ClickHouse :8123 ──SELECT only──┐            ┌── SigNoz API :8080 (GET only)
                                      ▼            ▼
                       ┌──────────── telelens (Go, one binary) ───────────┐
                       │ store: TelemetryStore ── clickhouse | fixtures   │
                       │ profilers: traces · logs · metrics · usage-xref  │
                       │            · data-quality · ecosystem            │
                       │ analyzers: Drain template miner · pricing ·      │
                       │            tail-sampling simulator (safety proof) │
                       │ generators (only writes = files in out/):        │
                       │   waste-report.md · findings.json                │
                       │   collector-fragment.yaml · casting-patch.yaml   │
                       └──────────────────────────────────────────────────┘
                                      │
                       human reviews out/, merges the casting patch,
                       runs `foundryctl cast` — TELELENS never applies anything
```

**Read-only by design (the product invariant).** Profilers issue only ClickHouse `SELECT`s and SigNoz `GET`s; the live client refuses non-SELECT statements in code, and `.env.example` documents the least-privilege `telelens_ro` ClickHouse user that enforces it at the database. The only writes are files in `out/`. (Foundry is SigNoz's official deployment tool — [github.com/SigNoz/foundry](https://github.com/SigNoz/foundry); `foundryctl cast` applies a *casting*, its config.)

## Honest status

| Scope | Status |
|---|---|
| **P0** — all six profilers, ranked waste report (md+json), config generator (tail_sampling / filter / transform), casting patch, fixture mode, offline test suite | ✅ Done |
| **P1** — sampling simulator with safety invariant, Telemetry Bill dashboard pack + guardrail alerts, usage cross-reference, `--json` agent/MCP surface, design-system CLI | ✅ Done |
| **Live-verified (Jul 18)** — full scan of a real mixed-workload instance (3.74 s, 26 findings); simulator over ALL 162,760 last-24h traces (SAFE); generated config applied to a live ingester measuring **−95.0% span storage** with zero error-trace loss on healthy known-volume traffic (on an error-storm day the same policy can only drop **~6%** — it refuses to touch errors), then reverted; 3 dashboards + 3 alert rules via API; MCP data-quality hook demoed with a real agent | ✅ Evidence in [`assets/`](assets/) |
| **P2 / roadmap** — web panel, `--explain` LLM narration, `telelens verify` automated savings assertion, `--import` flag, `otelcol validate` in CI, multi-cluster scan | ❌ Not done |
| **Known limits** — pricing is a transparent ingest model, not a cloud invoice; the simulator replays per-trace summaries, not packet-level collector behaviour; SigNoz v0.13x schemas with startup drift detection; heavy queries time-sliced + memory-guarded; meter buckets are hourly, so sub-hour A/B measurements count storage rows instead | ⚠️ [DOCS § Honest caveats](DOCS.md#honest-caveats) |

## Compatibility & uninstall

Live-verified against **self-hosted SigNoz v0.132.2** (`signoz_index_v3` / `logs_v2` / `time_series_v4`, with startup drift detection). **SigNoz Cloud** does not expose ClickHouse to customers, so live mode is self-hosted only — Cloud users run the fixtures demo, which exercises every profiler and generator against the recorded corpus.

**Uninstall:** delete the `telelens` binary and the `out/` directory — there is no other state. Drop the `telelens_ro` ClickHouse user if you created one; remove imported dashboards and alerts from the SigNoz UI or its APIs.

## Learn

[`DOCS.md`](DOCS.md) — operator manual: live setup, rollback runbook, command reference, the agent/MCP hook, honest caveats. [`learning/README.md`](learning/README.md) — teaching curriculum: pipeline walkthrough, SigNoz deep-dive, trade-offs, FAQ, design rationale, bug-hunt diary. [`LEARNINGS.md`](LEARNINGS.md) — what the live stack actually taught us. Licensed [MIT](LICENSE).

---

## AI disclosure

AI coding assistants were used during development. The profilers, config generator, sampling simulator, and dashboards are original work; the analysis itself is deterministic SQL and arithmetic — there is no LLM in the cost loop — and every live number in this README is backed by evidence under [`assets/`](assets/).

<div align="center"><sub>Built for the SigNoz observability ecosystem · observability for your observability.</sub></div>
