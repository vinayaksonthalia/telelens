# F-024 re-verification after the GLASSPANE SDK fix (live, Jul 18 2026)

TELELENS's live scan found `session.id` used as a metric label on GLASSPANE's
`browser.sessions.count` (finding F-024 in `live-scan-m2-evidence.md`, 37 series —
one per session, independently confirmed via the SigNoz MCP). GLASSPANE extended
its cardinality hard-guard from histograms to ALL metric instruments and removed
the label. This file is the end-to-end proof against the live instance.

## Method

1. Baseline query (ClickHouse via the :8123 proxy, read-only SELECT):
   ```sql
   SELECT count() AS series,
          countIf(JSONExtractString(labels,'session.id') != '') AS with_sid
   FROM signoz_metrics.time_series_v4
   WHERE metric_name = 'browser.sessions.count'
   -- → 37, 37   (every historical series carries a session.id label)
   ```
2. Started the Meridian demo webshop serving the REBUILT SDK
   (`packages/browser/dist/glasspane.iife.js`, post-fix, `debug: true` — the
   guard THROWS in debug, so a leak would have crashed init), opened
   `http://localhost:3333/?otlp=1` in a fresh browser tab (new sessionStorage →
   brand-new session `d319ff0a-fec8-4be7-9075-8ac1bc550e93`), clicked an
   add-to-cart, waited past the flush interval.
3. Re-queried.

## Result

| Check | Before | After | Verdict |
|---|---|---|---|
| `browser.sessions.count` series | 37 | **38** | one new series from the new session |
| …of which carry a `session.id` label | 37 | **37** | **the new series is label-free** ✅ |
| new samples on the counter since baseline | — | 1 | session VOLUME still counted ✅ |
| series (any metric) containing the new session's id | — | **0** | no metric label anywhere ✅ |
| spans carrying the new session's id | — | 2 | uniqueness via `count_distinct(session.id)` over spans works ✅ |

New series' label set (full): `page.route`, `device.type`, `connection.type`,
`browser.language`, `browser.mobile` + resource/SDK attrs — fixed and
low-cardinality, exactly the NFR-4 set.

## Honest framing

A `telelens scan --window 7` still reports the finding (F-015 in
`out-live-refix/`), because the 37 pre-fix series remain inside the scan window —
that is the profiler working correctly on historical data. The fix stops NEW
series: session growth is now zero-per-session instead of one-per-session, and
the finding ages out with the retention window. The same re-scan also confirms
the B1 fix live: the generated `filter/drop_unread_metrics` block (now
commented-out with a review banner) contains **zero** `signoz_*` metrics — the
platform-consumed denylist excluded all of them (previously it listed
`signoz_calls_total` + `signoz_latency.*`, the APM-page spanmetrics).

Cross-project loop closed: TELELENS found it (M2) → fix generated + measured
under the M4 apply window → GLASSPANE fixed the SDK at the source → TELELENS
re-verified live. Demo webshop stopped after; nothing else touched.
