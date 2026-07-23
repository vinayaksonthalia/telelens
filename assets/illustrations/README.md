# TELELENS explainer illustrations

Hand-sketched-style explainer images (1600x840 landscape, rendered at 2x) for the blog
and README. One big idea per image. Sources in `src/` (self-contained
HTML + the tiny `sketch.js` wobble engine); re-render with
`python3 src/render.py <html> <png>` (playwright, channel=chrome, dsf=2).

- `01-where-the-bill-comes-from.png` — debug-log firehose ($11/mo), label bombs ($3.8/mo), write-only telemetry, all pouring into ClickHouse (~$39/mo pure waste); red note: "you're paying to store data nobody reads".
- `02-scan-fix-proof.png` — read-only scan → priced waste report (130 GB/mo ≈ $39) → generated collector config (human applies) → −95%* measured on the live ingester; red footnote: "*measured, not projected — and errors always kept".
- `03-the-safety-proof.png` — the sampling valve keeps 100% error traces (40/40) + 100% slow traces (55/55), drops only boring duplicates; SAFE simulator-verdict stamp; simulated on your real last 24h first.
- `04-system-architecture.png` — ClickHouse (SELECT-only) + SigNoz API (GET-only) → profilers (traces/logs/metrics) → analyzers (cardinality, log templates via Drain, tail-sampling simulator) → priced waste report / collector fragment + casting patch / Telemetry Bill dashboards / guardrail rules & scan --json; coral note: read-only by design.
