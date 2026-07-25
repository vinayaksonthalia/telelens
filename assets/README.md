# TELELENS evidence index (live-verification wave, Jul 18 2026)

> **Brand assets** (logo, icon, PNG renders + usage): [`brand/`](brand/README.md).

Everything here was produced against the live SigNoz v0.132.2 instance
(mixed real telemetry: Faultline backend, historical opentelemetry-demo-lite,
GLASSPANE/Meridian browser RUM, ARGUS gen_ai self-telemetry).

## Milestone evidence

| File | What it proves |
|---|---|
| `live-scan-m2-evidence.md` | M2: live scan works (3.74s), what broke vs real schemas + fixes, the metric_log platform find, cross-project findings, reproduction queries |
| `live-scan-transcript.txt` | M2: full `telelens scan --window 7` console output (26 findings, 12.5 GB/mo ≈ $3.75/mo) |
| `live-oom-error-verbatim.md` | The verbatim `MEMORY_LIMIT_EXCEEDED` (code 241) error line the blog opens with, pulled from the live ClickHouse `system.query_log` (112 OOM events; 23 MiB query / 6.98 GiB server total confirmed) |
| `live-waste-report.md` / `live-findings.json` | M2: the real Waste Report (markdown + machine-readable) |
| `live-collector-fragment.yaml` / `live-casting-patch.yaml` | M2: fixes generated from the real findings |
| `live-simulator-m3-evidence.txt` | M3: policy replay over ALL 162,760 real last-24h traces — SAFE at 5%/750ms and 1%/500ms, 100% errors + slow kept; honest 68%-error-day note |
| `live-apply-measure-m4-evidence.md` | M4: APPLY→MEASURE→REVERT — **span storage −95.0% measured** on healthy traffic (13,404→672, identical loadgen) — with the honest counterweight in the same file: only ~6% droppable on an error-storm day; 31/31 injected error traces kept, RED metrics unchanged, revert verified |
| `live-mcp-quality-hook-m6-evidence.md` | M6: `scan --json` → claude CLI quality assessment + SigNoz MCP cross-check of F-024 (ideas-board #11666) |
| `live-cli-scan-output.ansi.txt` | M7: colored CLI output capture (lipgloss severity colors; view with `cat` in a terminal) |
| `live-scan-2026-07-25-film.txt` | Same day, a quieter window: 16 findings, 2.1 GB/mo, simulator replay over 109,686 traces — SAFE, **1.011 s** end to end. This is the run the demo film shows on screen (shots 4-6). |
| `live-scan-2026-07-25.txt` | Fresh live scan against the same instance grown to 14,103,470 spans: 15 findings, 2.5 GB/mo ≈ $0.75/mo, simulator replay over 75,586 traces — SAFE, **895 ms** end to end. This is the run `scan.gif` animates. |
| `scan.gif` | The `live-scan-2026-07-25.txt` run rendered frame by frame from a pty capture with the process's own per-chunk wall-clock timings (no re-typing, no sped-up or slowed-down playback) |

## Screenshots (`screenshots/`, retina 2x, real data)

| File | Captured | What it shows |
|---|---|---|
| `m5-01-cost-overview.png` | Jul 18 | Telemetry Bill — Cost Overview: ingest by signal, span/log bytes by service, **$22.03 projected monthly cost** value panel. Kept at the Jul 18 window deliberately — that week contains the high-volume day the cost panels were built to show. |
| `m5-02-cardinality-explorer.png` | **Jul 25** | Cardinality Explorer on current data: series per metric (`signoz_latency.bucket` = 4,122), label-bomb table (`operation` = 59 distinct), span-attr cardinality (`order.id` = 158,575 distinct over 192,511 spans), RUM `session.id` EXISTS panel |
| `m5-03-savings-tracker.png` | Jul 18 | Savings Tracker: cliff chart, 13.61 trace-GB window value, error-span safety panel. **Not re-shot on purpose** — the cliff *is* the apply/measure/revert experiment of Jul 17–18; a current 1-week window no longer contains it. |
| `m5-04-guardrail-alert-rules.png` | Jul 18 | The 3 TELELENS guardrail rules created via /api/v2/rules, all evaluating OK |

## Why the simulator trace counts differ between files

Three separate real replays are recorded here, taken minutes to hours apart
against a live instance that kept ingesting the whole time — so their totals
differ and are *supposed* to:

| Run | Where | Traces / spans replayed | Error traces kept |
|---|---|---|---|
| M2 scan-embedded (transcript) | `live-scan-transcript.txt` | 166,180 / 8,938,987 | 110,597 / 110,597 |
| M2 scan-embedded (report JSON, re-captured for M7) | `live-findings.json`, rendered by `docs/index.html` | 168,711 / 8,953,804 | 110,430 / 110,430 |
| M3 standalone `telelens simulate` | `live-simulator-m3-evidence.txt` | 162,760 / 8,900,958 | 110,597 / 110,597 |

The number that matters is identical in all three: **100% of error traces and
100% of slow traces kept.** The headline figure quoted in the README
(162,760) is the M3 standalone run.

Scan *duration* differs between captures for the same reason: the M2 run printed
`scan completed in 3.74s` (`live-scan-transcript.txt`, the figure the README
quotes), while the M7 re-capture of the same report printed `8.701s`
(`live-cli-scan-output.ansi.txt`, `live-waste-report.md`) against a busier,
larger instance. Both are real runs of the same command; neither is the other's
correction.

## The cross-project moment (in `live-waste-report.md`)

- F-023: ARGUS's AI telemetry priced (35 gen_ai spans, 204,665 tokens tracked).
- F-022: GLASSPANE's RUM bill (223 session spans / 38 sessions ≈ 1.8 KB/session).
- F-024: `session.id` found as a metric label on GLASSPANE's own
  `browser.sessions.count` — flagged structural, fix generated AND live-applied
  in M4. **Loop closed:** GLASSPANE extended its SDK cardinality guard to all
  metric instruments; live re-verification in
  `live-f024-refix-verification.md` (new sessions add zero labeled series;
  count preserved; uniqueness via count_distinct over spans).
- F-025: `order.id`/`user.id` correctly kept off all metric labels — discipline
  reported as a positive finding.

- illustrations/ — five hand-sketched explainer illustrations (where the bill comes from, scan-fix-proof, the safety proof, the system architecture, the story), plus four blog illustrations in illustrations/blog/; see illustrations/README.md
