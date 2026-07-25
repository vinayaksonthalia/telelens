# TELELENS — operator documentation

The telemetry cost & cardinality profiler for SigNoz. Read-only by
construction; the only things it writes are files you review.

- `README.md` — product overview, architecture, profiler catalog.
- `LEARNINGS.md` — what building against a live stack actually taught us.
- `assets/README.md` — index of live-verification evidence.

## TRY IT YOURSELF

### Path 1 — two minutes, zero infrastructure (fixtures)

Requires Go ≥ 1.24. The committed "Noisy Neighborhood" fixture corpus contains
every classic waste pattern as recorded query results.

```bash
cd telelens
go build -o telelens ./cmd/telelens

./telelens scan --fixtures            # ranked Waste Report + generated fixes in out/
./telelens simulate --fixtures        # the sampling safety proof (SAFE verdict)
./telelens scan --fixtures --json | jq '.findings[0]'   # machine-readable
cat out/waste-report.md out/collector-fragment.yaml
```

What you should see: ~26 ranked findings (DEBUG firehose, label bomb, jumbo
attributes, unread metrics, gen_ai/RUM attribution, quality defects), a total
GB/month + $ line, and a simulator verdict that keeps 100% of error traces.

### Path 2 — full live loop against your SigNoz

1. **Reach ClickHouse.** The Foundry compose does not publish `:8123`. Either
   add the port to the `telemetrystore` service, run telelens inside
   `signoz-network`, or run a throwaway forwarder (set `CH_CONTAINER` to your
   ClickHouse container's name — the default below is the Foundry compose name;
   on Helm or plain compose find yours with `docker ps | grep clickhouse`):
   ```bash
   CH_CONTAINER=${CH_CONTAINER:-signoz-telemetrystore-clickhouse-0-0}
   docker run -d --rm --name telelens-ch-proxy --network signoz-network \
     -p 8123:8123 alpine/socat tcp-listen:8123,fork,reuseaddr \
     "tcp-connect:${CH_CONTAINER}:8123"
   ```
   (Remove with `docker rm -f telelens-ch-proxy`; it forwards to a
   read-only-guarded HTTP interface and the removal command is one line.)
   For a production posture,
   create the read-only `telelens_ro` ClickHouse user documented in
   `.env.example` — the profiler also refuses non-SELECT statements in code.
2. **Configure and scan.**
   ```bash
   cp .env.example .env    # CLICKHOUSE_HTTP_URL, SIGNOZ_API_URL, SIGNOZ_API_KEY, COST_PER_GB
   set -a; source .env; set +a
   ./telelens scan --window 7 --cost-per-gb 0.30
   ```
   On startup TELELENS feature-detects the SigNoz schema (`signoz_index_v3` /
   `logs_v2` / `time_series_v4`) and refuses unrecognized versions with a clear
   error. SigNoz Cloud does not expose ClickHouse to customers, so live mode is
   self-hosted only; Cloud users run Path 1, which exercises every profiler and
   generator against the recorded corpus.
3. **Review the fixes.** `out/collector-fragment.yaml` is annotated per finding;
   `out/casting-patch.yaml` is the Foundry `ingester.config.data` variant.
   The review step is the product: drop the blocks that do not fit your
   environment (we did — see LEARNINGS §"The apply/revert story"). The
   `filter/drop_unread_metrics` block ships **commented-out** with a review
   banner: "unread" can only see SigNoz dashboards and alert rules, never
   external consumers — verify, then uncomment.
4. **Prove it safe.** `./telelens simulate` replays the policy over your last
   24h of real traces. A policy that would drop one error trace exits non-zero.
5. **Apply + measure.** Merge the fragment into the ingester config (or the
   casting), restart the collector, and watch the Savings Tracker dashboard.
   Our measured run: **−95.0% span storage on identical known-volume traffic,
   31/31 injected error traces kept, RED metrics unchanged**
   (`assets/live-apply-measure-m4-evidence.md`) — on an error-storm day the same
   policy can only drop **~6%**, because it refuses to drop errors.
6. **Import the dashboard pack + guardrails.**
   ```bash
   export SIGNOZ_URL=http://localhost:8080 SIGNOZ_API_KEY=...
   ./dashboards/import.sh          # 3 dashboards + 3 alert rules
   ```
   The alert channel is resolved at import time (`$SIGNOZ_ALERT_CHANNEL` or
   your first existing channel) — nothing machine-specific ships in the JSON.

## Rollback runbook (before you apply anything)

The M4 live run followed this exact procedure; it is written here as YOUR
runbook, not just our history (`assets/live-apply-measure-m4-evidence.md` is
the worked example with real numbers).

1. **Back up first.** Before merging the fragment, copy the current collector
   config next to itself with a greppable suffix:
   ```bash
   cp ingester.yaml ingester.yaml.pre-telelens-backup
   ```
2. **Know your blast radius.** The fragment only ever adds *processors*:
   tail_sampling affects trace volume (never errors/slow traces — the
   simulator proves it before you apply), filter/transform blocks affect
   exactly the metrics/logs/attributes named in their annotations. Nothing
   touches receivers or exporters. Keep `signozspanmetrics` BEFORE
   `tail_sampling` in the traces pipeline or your RED metrics will be sampled
   too (the generated fragment repeats this warning).
3. **Revert = one command.** Restore the backup and restart the collector:
   ```bash
   cp ingester.yaml.pre-telelens-backup ingester.yaml && docker restart <ingester-container>
   ```
   (Foundry: restore the casting and `foundryctl cast` again.)
4. **Verify the revert.** Re-run a known-volume load (or just watch a minute
   of traffic) and check the stored span rate is back at the pre-apply rate —
   our run re-verified 4,059 spans/90 s, matching the before-rate. Then re-run
   `telelens scan`: the Savings Tracker dashboard's cliff should flatten.
5. **Decision-wait caveat.** `tail_sampling` holds traces for
   `decision_wait: 10s`; traces longer than that are evaluated on partial
   data in a real collector — the replay simulator cannot model this, so
   watch your longest-running workflows in the first minutes after apply.

## The safety proof, in detail

Recommending sampling without proving it safe is malpractice. Before TELELENS
emits a `tail_sampling` policy it replays that policy over real trace summaries
(the last 24 h in live mode, the fixture set offline): keep all errors, keep
everything over the latency threshold, hash-sample the rest deterministically.

The verdict is binary. A policy that would drop **one** error or slow trace is
rejected — `telelens simulate` prints UNSAFE and exits non-zero:

```bash
./telelens simulate --fixtures --sample-pct 5 --latency-ms 750
#   error traces:  40 / 40 kept
#   slow traces:   55 / 55 kept
#   boring traces: 53 / 1100 kept (95.2% dropped)
#   verdict: SAFE — policy retains 100% of error traces and 100% of slow traces
```

That the generator can never emit such a policy in the first place is itself
asserted by `TestSimulateSafetyInvariant`
(`internal/analyze/simulator_test.go`), which sweeps trace sets across error and
slow-trace counts looking for a counter-example. The live whole-instance replay
over all 162,760 last-24h traces is in
`assets/live-simulator-m3-evidence.txt`.

## Dashboards & guardrail alerts

`dashboards/` ships the three-part **Telemetry Bill** pack (import via
`./dashboards/import.sh`, the SigNoz UI, or `POST /api/v1/dashboards`):

- **Cost Overview** — ingest rate by signal and service, projected monthly $.
- **Cardinality Explorer** — series counts, label bombs, builder-vs-SQL panels.
- **Savings Tracker** — the ingest cliff after a fix lands, with the error-span
  safety panel beside it (the screenshot in the README).

`alerts/guardrails.json` seeds three rules — ingest-bytes spike, cardinality
explosion, log-volume anomaly — so waste cannot silently return.

## The MCP / agent data-quality hook (ideas-board #11666)

`telelens scan --json` prints the full findings report as pure JSON on stdout
(progress goes to stderr). That is the agent surface:

```bash
# a data-quality reviewer agent over deterministic findings
telelens scan --json | jq '{findings: [.findings[] | select(.category=="quality")]}' \
  | claude -p "Assess this SigNoz instance's telemetry data quality; top action first."
```

Agents can cross-check any finding through the official SigNoz MCP server
(e.g. `signoz_check_metric_cardinality` independently confirmed our
session.id label finding — transcript in
`assets/live-mcp-quality-hook-m6-evidence.md`). Division of labor is
deliberate and disclosed: telelens computes (SQL + math, testable); the LLM
only narrates. `TELELENS_MCP_URL` additionally POSTs the findings summary to
any collector endpoint (experimental).

## Command reference

| Command | Purpose |
|---|---|
| `telelens scan [--fixtures] [--window N] [--cost-per-gb X] [--json] [--out dir]` | full profile → Waste Report + generated fixes |
| `telelens simulate [--sample-pct 5] [--latency-ms 750] [--json]` | replay a tail-sampling policy; non-zero exit on UNSAFE |
| `telelens report --findings out/findings.json` | re-render waste-report.md (writes next to the findings file unless `--out`) |
| `telelens generate --findings out/findings.json` | re-render collector fragment + casting patch (same output-dir rule) |

Exit codes: `0` success · `1` failure (bad flags, unreachable source, UNSAFE
simulate verdict) · `2` **degraded scan** — one or more profilers failed
(e.g. transient ClickHouse memory pressure) and were skipped; partial results
were still written, with `Why:/Try:` guidance on stderr (re-run, reduce
`--window`, or ease ClickHouse memory limits).

Environment: `CLICKHOUSE_HTTP_URL`, `CLICKHOUSE_USER`/`CLICKHOUSE_PASSWORD`,
`SIGNOZ_API_URL`, `SIGNOZ_API_KEY`, `COST_PER_GB`, `TELELENS_WINDOW_DAYS`,
`TELELENS_OUT_DIR`, `TELELENS_MCP_URL`. Errors follow `Error:/Why:/Try:`;
output degrades cleanly under `NO_COLOR` and pipes; `--json` is stable for
machines.

## Pricing model

Every priced finding uses one transparent formula: uncompressed-ingest bytes,
extrapolated to 30 days, multiplied by `--cost-per-gb` (default `$0.30`). The
envelope constants are documented in `internal/analyze/pricing.go`. Findings
that cannot be priced reliably ship as **directional** — ranked by severity,
with no hard dollar figure attached.

## Query Builder coverage

SigNoz's ideas board has six Query Builder showcase cards; TELELENS exercises
each capability in service of a real analysis, not a demo for its own sake.

| Card | Capability | Where it's used |
|---|---|---|
| #11673 | boolean search, EXISTS / NOT EXISTS | Cardinality Explorer "BOTH tracking IDs" panel; usage-xref is NOT-EXISTS over stored vs referenced metrics |
| #11674 | JSON body paths | log profiler's body mining; body-path filters in the debug-logs panel |
| #11675 | hasAll array search | span-events array filter in Cardinality Explorer |
| #11676 | sumIf / count_distinct / rate | `rate` on meter metrics; `count_distinct` on attributes; `countIf` in the duplicate-span profiler |
| #11677 | cross-context group-by + having | every profiler query (`GROUP BY … HAVING …`); dashboards group by resource attributes |
| #11678 | order-by + limit | every profiler query ends `ORDER BY <cost> DESC LIMIT n` — the Waste Report *is* an order-by over money |

Queries Q1–Q12 are defined in `internal/store/store.go`; their raw-ClickHouse
SQL forms live in `internal/store/clickhouse.go`.

## Honest caveats

- Pricing is uncompressed-ingest bytes × `$/GB` — a transparent model, not a
  cloud invoice. Unpriceable findings ship as "directional".
- The simulator models the 3 shipped tail_sampling policy types over per-trace
  summaries; it is not a packet-level collector emulation.
- Live schema support is SigNoz v0.13x (feature-detected; unknown schemas are
  refused with a clear error).
- The measured −95% is for a healthy, low-error traffic mix — the tail-sampling
  sweet spot. On an error-storm day the same replay honestly reports ~6%
  droppable, because the policy refuses to drop errors (that refusal is the
  feature).
- Meter (`signoz_meter`) buckets are hourly — for sub-hour before/after
  measurement, count storage rows (we did; both numbers are in the evidence).
