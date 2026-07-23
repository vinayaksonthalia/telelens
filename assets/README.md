# TELELENS evidence index (live-verification wave, Jul 18 2026)

> **Brand assets** (logo, icon, PNG renders + usage): [`brand/`](brand/README.md).

Everything here was produced against the live SigNoz v0.132.2 instance
(mixed real telemetry: Faultline backend, historical opentelemetry-demo-lite,
GLASSPANE/Meridian browser RUM, ARGUS gen_ai self-telemetry).

## Milestone evidence

| File | What it proves |
|---|---|
| `live-scan-m2-evidence.md` | M2: live scan works (3.9s), what broke vs real schemas + fixes, the metric_log platform find, cross-project findings, reproduction queries |
| `live-scan-transcript.txt` | M2: full `telelens scan --window 7` console output (26 findings, 12.5 GB/mo ≈ $3.74/mo) |
| `live-oom-error-verbatim.md` | The verbatim `MEMORY_LIMIT_EXCEEDED` (code 241) error line the blog opens with, pulled from the live ClickHouse `system.query_log` (112 OOM events; 23 MiB query / 6.98 GiB server total confirmed) |
| `live-waste-report.md` / `live-findings.json` | M2: the real Waste Report (markdown + machine-readable) |
| `live-collector-fragment.yaml` / `live-casting-patch.yaml` | M2: fixes generated from the real findings |
| `live-simulator-m3-evidence.txt` | M3: policy replay over ALL 162,760 real last-24h traces — SAFE at 5%/750ms and 1%/500ms, 100% errors + slow kept; honest 68%-error-day note |
| `live-apply-measure-m4-evidence.md` | M4: APPLY→MEASURE→REVERT — **span storage −95.0% measured** (13,404→672, identical loadgen), 31/31 injected error traces kept, RED metrics unchanged, revert verified |
| `live-mcp-quality-hook-m6-evidence.md` | M6: `scan --json` → claude CLI quality assessment + SigNoz MCP cross-check of F-024 (ideas-board #11666) |
| `live-cli-scan-output.ansi.txt` | M7: colored CLI output capture (lipgloss severity colors; view with `cat` in a terminal) |

## Screenshots (`screenshots/`, retina 2x, real data)

| File | What it shows |
|---|---|
| `m5-01-cost-overview.png` | Telemetry Bill — Cost Overview: ingest by signal, span/log bytes by service, **$22.03 projected monthly cost** value panel |
| `m5-02-cardinality-explorer.png` | Cardinality Explorer: series per metric, label-bomb table (incl. `browser.sessions.count` × `session.id` = 37), span-attr cardinality, RUM EXISTS panel |
| `m5-03-savings-tracker.png` | Savings Tracker: cliff chart, 13.61 trace-GB window value, error-span safety panel |
| `m5-04-guardrail-alert-rules.png` | The 3 TELELENS guardrail rules created via /api/v2/rules, all evaluating OK |

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

- illustrations/ — three hand-sketched explainer illustrations (where the bill comes from, scan-fix-proof, the safety proof); see illustrations/README.md
