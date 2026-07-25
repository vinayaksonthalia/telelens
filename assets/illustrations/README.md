# TELELENS explainer illustrations

Hand-sketched-style explainer images (1600x840 landscape, rendered at 2x) for the blog
and README. One big idea per image. Sources in `src/` (self-contained
HTML + the tiny `sketch.js` wobble engine); re-render with
`python3 src/render.py <html> <png>` (playwright, channel=chrome, dsf=2).

The recurring character **Pager** — a small on-call engineer, the reader's proxy —
lives in the shared `character.js` (a seeded-wobble SVG figure with named poses:
`frustrated`, `focused`, `relieved`, `happy`, …); `story-builder.js` lays out the
four-panel story strip. Same face/hair/proportions everywhere; only pose and
expression change.

- `01-where-the-bill-comes-from.png` — debug-log firehose ($11/mo), label bombs ($3.8/mo), write-only telemetry, all pouring into ClickHouse (~$39/mo pure waste); red note: "you're paying to store data nobody reads".
- `02-scan-fix-proof.png` — read-only scan → priced waste report (130 GB/mo ≈ $39) → generated collector config (human applies) → −95%* measured on the live ingester (healthy traffic; ~6% on an error-storm day); red footnote: "*measured, not projected — and errors always kept".
- `03-the-safety-proof.png` — the sampling valve keeps 100% error traces (40/40) + 100% slow traces (55/55), drops only boring duplicates; SAFE simulator-verdict stamp; simulated on your real last 24h first.
- `04-system-architecture.png` — ClickHouse (SELECT-only) + SigNoz API (GET-only) → profilers (traces/logs/metrics) → analyzers (cardinality, log templates via Drain, tail-sampling simulator) → priced waste report / collector fragment + casting patch / Telemetry Bill dashboards / guardrail rules & scan --json; coral note: read-only by design.
- `05-the-story.png` — a four-panel strip starring **Pager**, the on-call engineer: (1) winces at a "$39/mo and climbing" storage bill; (2) one `telelens scan` ranks the waste (F-001 crit logs, F-002 high attrs, F-006 crit label …); (3) the sampling valve keeps every error (40/40) and every slow trace (55/55) — verdict SAFE; (4) span storage drops −95% on healthy traffic (~6% on an error-storm day), Pager happy. Caption: waste found, fix generated, and proven safe — measured, not projected.
