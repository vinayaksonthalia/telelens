# M4 evidence — Generate → APPLY → MEASURE → REVERT (Jul 18, 2026)

The "we cut X%" number, measured on the live instance — not projected.

## What was applied (and what deliberately was not)

From the generated `collector-fragment.yaml` (findings F-001, F-024), merged into
`signoz/pours/deployment/ingester/ingester.yaml` (backup:
`ingester.yaml.pre-telelens-backup`, one-command revert):

- `tail_sampling` — keep 100% error traces + 100% traces ≥ 750 ms, sample boring
  successes at 5% (F-001; simulator verdict on the last 24h of real traces: SAFE).
  Placed **after** `signozspanmetrics/delta` so RED metrics keep seeing 100% of
  spans — sampling storage, not signal.
- `transform/defuse_label_bombs` — strip the unbounded `session.id` label from
  `browser.sessions.count` (F-024).

**Operator-review exclusions (the review step is the product):**
`filter/drop_unread_metrics` (91 names) was NOT applied — the unread set includes
`argus.tokens` and `browser.web_vitals.*`, which are live evidence surfaces for
the sibling ARGUS/GLASSPANE projects. The generated fragment is a proposal;
a human applies what fits the environment.

## Measurement protocol (known-volume traffic, identical windows)

Traffic: Faultline `loadgen.py --rps 4 --duration 300` (gateway→catalog/orders/
payments), all faults idle. Ground truth = rows stored in
`signoz_traces.distributed_signoz_index_v3` for exactly the run window (the
`signoz_meter` rollup flushes hourly — too coarse to separate two windows inside
one hour, so storage rows are the honest measure).

| Window (UTC) | Config | Requests | Spans stored | Traces stored | Attr bytes |
|---|---|---|---|---|---|
| 05:28:09–05:33:10 | full collection | 1,006 (all 200) | **13,404** | 1,500 | 39,199 |
| 05:36:29–05:41:29 | TELELENS config | 1,007 (all 200) | **672** | 73 | 1,955 |

## Headline

- **Span storage: −95.0%** (13,404 → 672). Traces −95.1% (1,500 → 73) — the 5%
  keep-rate landed exactly as simulated.
- **Attribute bytes: −95.0%** (39,199 → 1,955).

## Nothing that matters was lost (verified, not asserted)

1. **Error preservation under sampling:** with the config still applied, injected
   the payments error-storm (`faultctl inject error-storm`) + 90 s loadgen →
   loadgen observed **31× HTTP 502**; ClickHouse stored **31 error traces
   (248 error spans)** in that window — **100% of errors kept** while total
   traces stored were 47 (31 errors + 16 sampled successes). Fault cleared after.
2. **RED metrics unchanged:** `signoz_calls_total` (spanmetrics) for the four
   services: BEFORE window 12,428 calls vs AFTER window 12,827 calls — identical
   traffic seen by metrics while span storage fell 95% (spanmetrics runs before
   tail_sampling by design).

## Revert (full collection restored — verified)

- 05:47 UTC: `ingester.yaml.pre-telelens-backup` restored, ingester restarted,
  "Everything is ready" logged. GLASSPANE's CORS block intact.
- Verification run 05:49:42–05:51:12 (308 requests / 90 s): **4,059 spans /
  460 traces stored** — consistent with the full-collection BEFORE rate
  (≈45 spans/s vs ≈44 spans/s), i.e. sampling is OFF and collection is back to
  100%. PROJECT-LOG coordination note closed.

## Honest caveats

- 95% is the reduction on *this* traffic mix (fast, non-error backend calls) —
  the tail-sampling sweet spot. The whole-instance last-24h replay predicted
  only 6.3% span-volume reduction because 68% of that day's traces carry errors
  (the historical demo-lite cart storm) and the policy refuses to drop errors.
  Both numbers are real; together they ARE the product's honesty story: sampling
  saves on healthy traffic, and TELELENS tells you when your real blocker is an
  error storm, not sampling.
- Two-window measurement on a small instance; loadgen request counts differ by
  0.1% (1,006 vs 1,007).
- The `session.id` label transform was active during the AFTER window; no
  browser traffic ran, so its effect is proven by config load + the metrics
  pipeline staying healthy, not by a before/after series count.
