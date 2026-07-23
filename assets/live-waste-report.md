# TELELENS Waste Report

- **Scanned:** 2026-07-18T06:08:02Z (window: last 7 days, source: clickhouse (http://localhost:8123, window=7d), scan took 8.7s)
- **Pricing basis:** $0.30 per uncompressed-ingest GB (configurable via `--cost-per-gb`)
- **Total identified savings: 12.5 GB/month ≈ $3.75/month**

| # | Sev | Category | Finding | GB/mo | $/mo |
|---|-----|----------|---------|------:|-----:|
| F-001 | high | traces | 7801417 near-identical success spans are prime tail-sampling candidates | 9.7 | 2.90 |
| F-002 | medium | logs | INFO firehose: checkout is 35% of your log volume | 1.9 | 0.58 |
| F-003 | high | metrics | 86 more metrics are written but read by no dashboard or alert | 0.3 | 0.08 |
| F-004 | high | metrics | Metric "signoz_latency.bucket" is written but read by no dashboard or alert | 0.3 | 0.08 |
| F-005 | low | traces | Browser RUM telemetry bill: "browser-frontend" ships 120358 session spans across 1 sessions | 0.2 | 0.05 |
| F-006 | high | metrics | Metric "http.server.duration.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-007 | high | metrics | Metric "http.server.response.size.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-008 | high | metrics | Metric "checkout.cart.value.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-009 | high | metrics | Metric "otel.sdk.metric_reader.collection.duration.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-010 | medium | traces | Span attribute "url.full" has 57315 distinct values | 0.0 | 0.01 |
| F-011 | medium | traces | Span attribute "db.statement" has 54630 distinct values | 0.0 | 0.00 |
| F-012 | medium | traces | Span attribute "shipping.tracking.id" has 53051 distinct values | 0.0 | 0.00 |
| F-013 | medium | traces | Span attribute "payment.transaction.id" has 53243 distinct values | 0.0 | 0.00 |
| F-014 | medium | traces | Span attribute "app.shipping.tracking.id" has 53045 distinct values | 0.0 | 0.00 |
| F-015 | medium | traces | Span attribute "app.payment.transaction.id" has 53062 distinct values | 0.0 | 0.00 |
| F-016 | medium | traces | Span attribute "app.order.id" has 65591 distinct values | 0.0 | 0.00 |
| F-017 | medium | traces | Span attribute "http.url" has 10068 distinct values | 0.0 | 0.00 |
| F-018 | medium | traces | Span attribute "order.id" has 68176 distinct values | 0.0 | 0.00 |
| F-019 | medium | traces | Span attribute "app.email.message_id" has 54780 distinct values | 0.0 | 0.00 |
| F-020 | medium | traces | Span attribute "app.email.recipient" has 10011 distinct values | 0.0 | 0.00 |
| F-021 | medium | traces | Span attribute "app.user.id" has 43377 distinct values | 0.0 | 0.00 |
| F-022 | low | traces | Browser RUM telemetry bill: "meridian-web" ships 223 session spans across 38 sessions | 0.0 | 0.00 |
| F-023 | low | traces | AI-agent telemetry bill: "argus" ships 35 gen_ai spans (204665 tokens tracked) | 0.0 | 0.00 |
| F-024 | high | metrics | Unbounded ID "session.id" is a metric label on browser.sessions.count | — | — |
| F-025 | low | quality | Cardinality discipline held: 2 unbounded ID attribute(s) stay off metric labels | — | — |
| F-026 | low | quality | demo-app mixes severity casing ("Info") | — | — |

## F-001 — 7801417 near-identical success spans are prime tail-sampling candidates

**Category:** traces · **Severity:** high · **Estimated saving:** 9.7 GB/mo ≈ $2.90/mo

**Evidence**

- browser-frontend "HTTP GET": 481468 near-identical spans, error rate 0.000%, distinct-duration ratio 0.67
- frontend "HTTP GET": 481466 near-identical spans, error rate 0.000%, distinct-duration ratio 0.53
- product-catalog "sql.rows": 361053 near-identical spans, error rate 0.000%, distinct-duration ratio 0.03
- product-catalog "sql.conn.query": 361053 near-identical spans, error rate 0.000%, distinct-duration ratio 0.03
- product-catalog "GetProduct": 361053 near-identical spans, error rate 0.000%, distinct-duration ratio 0.07
- recommendation "GET / http send": 240736 near-identical spans, error rate 0.000%, distinct-duration ratio 0.02
- recommendation "get_product_list": 240733 near-identical spans, error rate 0.000%, distinct-duration ratio 0.05
- recommendation "GET /recommendations http send": 240730 near-identical spans, error rate 0.000%, distinct-duration ratio 0.02
- ad "getAds": 240730 near-identical spans, error rate 0.000%, distinct-duration ratio 0.16
- quote-python "POST /quote http send": 228734 near-identical spans, error rate 0.000%, distinct-duration ratio 0.02
- catalog "catalog.enrich_pricing": 137352 near-identical spans, error rate 0.000%, distinct-duration ratio 0.15
- catalog "catalog.cache_check": 137352 near-identical spans, error rate 0.000%, distinct-duration ratio 0.07
- catalog "catalog.lookup_products": 137352 near-identical spans, error rate 0.000%, distinct-duration ratio 0.35
- catalog "catalog.db_query": 137351 near-identical spans, error rate 0.000%, distinct-duration ratio 0.29
- browser-frontend "submit - checkout-form": 120370 near-identical spans, error rate 0.000%, distinct-duration ratio 0.04
- browser-frontend "web-vitals": 120370 near-identical spans, error rate 0.000%, distinct-duration ratio 0.03
- frontend "GET /api/cart": 120368 near-identical spans, error rate 0.000%, distinct-duration ratio 0.98
- recommendation "GET /": 120368 near-identical spans, error rate 0.000%, distinct-duration ratio 0.45
- frontend "GET /api/products": 120367 near-identical spans, error rate 0.000%, distinct-duration ratio 0.71
- frontend "GET /api/ads": 120366 near-identical spans, error rate 0.000%, distinct-duration ratio 0.58
- frontend "GET /api/recommendations": 120365 near-identical spans, error rate 0.000%, distinct-duration ratio 0.68
- recommendation "GET /recommendations": 120365 near-identical spans, error rate 0.000%, distinct-duration ratio 0.42
- checkout "getAds": 120360 near-identical spans, error rate 0.000%, distinct-duration ratio 0.49
- checkout "PlaceOrder": 120360 near-identical spans, error rate 0.000%, distinct-duration ratio 0.99
- checkout "chargeCard": 120360 near-identical spans, error rate 0.000%, distinct-duration ratio 0.45
- checkout "getCurrencyConversion": 120360 near-identical spans, error rate 0.000%, distinct-duration ratio 0.23
- checkout "getProductDetails": 120360 near-identical spans, error rate 0.000%, distinct-duration ratio 0.62
- checkout "getRecommendations": 120360 near-identical spans, error rate 0.000%, distinct-duration ratio 0.64
- checkout "prepareOrderItemsAndShippingQuoteFromCart": 120360 near-identical spans, error rate 0.000%, distinct-duration ratio 0.99
- frontend "HTTP POST": 120359 near-identical spans, error rate 0.000%, distinct-duration ratio 0.99
- frontend "POST /api/checkout": 120359 near-identical spans, error rate 0.000%, distinct-duration ratio 1.00
- browser-frontend "page-session": 120358 near-identical spans, error rate 0.000%, distinct-duration ratio 0.99
- browser-frontend "HTTP POST": 120358 near-identical spans, error rate 0.000%, distinct-duration ratio 0.99
- product-catalog "ListProducts": 120353 near-identical spans, error rate 0.000%, distinct-duration ratio 0.07
- currency "Convert": 120352 near-identical spans, error rate 0.000%, distinct-duration ratio 0.05
- quote-python "POST /quote": 114367 near-identical spans, error rate 0.000%, distinct-duration ratio 0.39
- quote-python "calculate-quote": 114367 near-identical spans, error rate 0.000%, distinct-duration ratio 0.07
- quote-python "POST /quote http receive": 114367 near-identical spans, error rate 0.000%, distinct-duration ratio 0.03
- checkout "shipOrder": 114362 near-identical spans, error rate 0.000%, distinct-duration ratio 0.67
- checkout "sendOrderConfirmation": 114362 near-identical spans, error rate 0.000%, distinct-duration ratio 0.48
- checkout "orders publish": 114362 near-identical spans, error rate 0.000%, distinct-duration ratio 0.86
- fraud-detection "orders receive": 114355 near-identical spans, error rate 0.000%, distinct-duration ratio 0.06
- shipping "HTTP POST": 114355 near-identical spans, error rate 0.000%, distinct-duration ratio 0.63
- shipping "createQuoteFromCount": 114355 near-identical spans, error rate 0.000%, distinct-duration ratio 0.64
- fraud-detection "detectFraud": 114355 near-identical spans, error rate 0.000%, distinct-duration ratio 0.02
- shipping "ship": 114355 near-identical spans, error rate 0.000%, distinct-duration ratio 0.64
- accounting "processOrder": 114347 near-identical spans, error rate 0.000%, distinct-duration ratio 0.04
- accounting "orders receive": 114347 near-identical spans, error rate 0.000%, distinct-duration ratio 0.08

**Remediation:** Add a tail_sampling processor: keep 100% of errors, keep traces >= 750 ms, sample the remaining successes at 5%. Run `telelens simulate` for the safety proof.

> Generated fix: see the `tail_sampling` block annotated `F-001` in `collector-fragment.yaml`.

## F-002 — INFO firehose: checkout is 35% of your log volume

**Category:** logs · **Severity:** medium · **Estimated saving:** 1.9 GB/mo ≈ $0.58/mo

**Evidence**

- service=checkout severity=INFO: 2724075 records, 0.45 GB in window — 35% of ALL log bytes
- dominant template (132 occurrences): "FetchProduct"

**Remediation:** Review "checkout"'s chattiest INFO templates: demote per-request noise to DEBUG (then filter), or aggregate to metrics. Not auto-dropped: INFO may carry real signal.

## F-003 — 86 more metrics are written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.3 GB/mo ≈ $0.08/mo

**Evidence**

- 86 further unread metrics totaling 1936809 samples in window; checked 31 dashboards and 21 alert rules — zero references
- full list: system.cpu.time, http.client.request.duration.bucket, http.server.request.size.bucket, signoz_calls_total, signoz_latency.count, signoz_latency.sum, system.cpu.utilization, app.email.latency.bucket, app.payment.latency.bucket, http.client.duration.bucket, otel.sdk.processor.span.processed, app.payment.transactions, otel.sdk.span.started, system.cpu.load_average.5m, system.cpu.load_average.1m, system.cpu.load_average.15m, checkout.orders.count, app.frontend.requests, system.network.io, http.server.response.size.max, http.server.response.size.count, http.server.response.size.sum, http.server.response.size.min, http.server.duration.sum, http.server.duration.max, http.server.duration.count, http.server.duration.min, system.network.dropped, system.network.errors, system.network.packets, otel.sdk.span.live, checkout.cart.value.min, checkout.cart.value.sum, checkout.cart.value.max, otel.sdk.metric_reader.collection.duration.count, otel.sdk.metric_reader.collection.duration.max, otel.sdk.metric_reader.collection.duration.sum, system.memory.utilization, system.memory.usage, signoz_external_call_latency_sum, signoz_external_call_latency_count, http.server.active_requests, system.network.connections, app.email.sent, http.client.request.duration.min, http.client.request.duration.sum, http.client.request.duration.count, http.server.request.size.sum, http.server.request.size.max, http.server.request.size.min, http.server.request.size.count, quote.amount.bucket, process.cpu.time, signoz_db_latency_sum, signoz_db_latency_count, http.client.duration.max, http.client.duration.sum, http.client.duration.count, http.client.duration.min, argus.tokens, system.paging.operations, process.runtime.cpython.gc_count, app.accounting.revenue_total, app.accounting.orders_processed, process.runtime.cpython.memory, process.runtime.cpython.cpu_time, process.open_file_descriptor.count, app.recommendations.count, process.runtime.cpython.cpu.utilization, process.runtime.cpython.thread_count, redis.cmd.latency, redis.cpu.time, browser.web_vitals.ttfb.sum, browser.web_vitals.ttfb.max, browser.web_vitals.ttfb.count, browser.web_vitals.ttfb.min, browser.web_vitals.fcp.sum, browser.web_vitals.lcp.min, browser.web_vitals.fcp.max, browser.web_vitals.fcp.min, browser.web_vitals.fcp.count, browser.web_vitals.lcp.sum, browser.web_vitals.lcp.max, browser.web_vitals.inp.min, browser.web_vitals.inp.sum, browser.web_vitals.inp.max

**Remediation:** Same story at lower volume: drop them at the collector, or delete the instruments. The generated filter/drop_unread_metrics block lists every name.

> Generated fix: see the `drop_metric` block annotated `F-003` in `collector-fragment.yaml`.

## F-004 — Metric "signoz_latency.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.3 GB/mo ≈ $0.08/mo

**Evidence**

- metric=signoz_latency.bucket series=8516 samples_in_window=1869768; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "signoz_latency.bucket": drop it at the collector (filter processor) or remove the instrument. If it is consumed outside SigNoz, allow-list it in telelens config.

> Generated fix: see the `drop_metric` block annotated `F-004` in `collector-fragment.yaml`.

## F-005 — Browser RUM telemetry bill: "browser-frontend" ships 120358 session spans across 1 sessions

**Category:** traces · **Severity:** low · **Estimated saving:** 0.2 GB/mo ≈ $0.05/mo

**Evidence**

- service=browser-frontend spans_with_session.id=120358 sessions=1 attr_bytes=240716

**Remediation:** Attribution, not waste: this is the frontend-observability bill. Scale it by expected sessions/month before rollout; RUM SDK sampleRate is the knob.

## F-006 — Metric "http.server.duration.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=http.server.duration.bucket series=832 samples_in_window=296912; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "http.server.duration.bucket": drop it at the collector (filter processor) or remove the instrument. If it is consumed outside SigNoz, allow-list it in telelens config.

> Generated fix: see the `drop_metric` block annotated `F-006` in `collector-fragment.yaml`.

## F-007 — Metric "http.server.response.size.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=http.server.response.size.bucket series=832 samples_in_window=296912; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "http.server.response.size.bucket": drop it at the collector (filter processor) or remove the instrument. If it is consumed outside SigNoz, allow-list it in telelens config.

> Generated fix: see the `drop_metric` block annotated `F-007` in `collector-fragment.yaml`.

## F-008 — Metric "checkout.cart.value.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=checkout.cart.value.bucket series=32 samples_in_window=263792; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "checkout.cart.value.bucket": drop it at the collector (filter processor) or remove the instrument. If it is consumed outside SigNoz, allow-list it in telelens config.

> Generated fix: see the `drop_metric` block annotated `F-008` in `collector-fragment.yaml`.

## F-009 — Metric "otel.sdk.metric_reader.collection.duration.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=otel.sdk.metric_reader.collection.duration.bucket series=32 samples_in_window=263696; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "otel.sdk.metric_reader.collection.duration.bucket": drop it at the collector (filter processor) or remove the instrument. If it is consumed outside SigNoz, allow-list it in telelens config.

> Generated fix: see the `drop_metric` block annotated `F-009` in `collector-fragment.yaml`.

## F-010 — Span attribute "url.full" has 57315 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- key="url.full" cardinality=57315 occurrences=65033 avg value 60B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "url.full" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-011 — Span attribute "db.statement" has 54630 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="db.statement" cardinality=54630 occurrences=62897 avg value 52B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "db.statement" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-012 — Span attribute "shipping.tracking.id" has 53051 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="shipping.tracking.id" cardinality=53051 occurrences=53199 avg value 36B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "shipping.tracking.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-013 — Span attribute "payment.transaction.id" has 53243 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="payment.transaction.id" cardinality=53243 occurrences=53183 avg value 36B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "payment.transaction.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-014 — Span attribute "app.shipping.tracking.id" has 53045 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="app.shipping.tracking.id" cardinality=53045 occurrences=53182 avg value 36B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "app.shipping.tracking.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-015 — Span attribute "app.payment.transaction.id" has 53062 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="app.payment.transaction.id" cardinality=53062 occurrences=52987 avg value 36B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "app.payment.transaction.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-016 — Span attribute "app.order.id" has 65591 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="app.order.id" cardinality=65591 occurrences=65582 avg value 25B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "app.order.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-017 — Span attribute "http.url" has 10068 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="http.url" cardinality=10068 occurrences=27724 avg value 55B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "http.url" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-018 — Span attribute "order.id" has 68176 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="order.id" cardinality=68176 occurrences=95794 avg value 12B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "order.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-019 — Span attribute "app.email.message_id" has 54780 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="app.email.message_id" cardinality=54780 occurrences=54829 avg value 12B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "app.email.message_id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-020 — Span attribute "app.email.recipient" has 10011 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="app.email.recipient" cardinality=10011 occurrences=27212 avg value 21B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "app.email.recipient" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-021 — Span attribute "app.user.id" has 43377 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="app.user.id" cardinality=43377 occurrences=55262 avg value 10B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "app.user.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-022 — Browser RUM telemetry bill: "meridian-web" ships 223 session spans across 38 sessions

**Category:** traces · **Severity:** low · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- service=meridian-web spans_with_session.id=223 sessions=38 attr_bytes=1188 (1.8 KB/session uncompressed)

**Remediation:** Attribution, not waste: this is the frontend-observability bill. Scale it by expected sessions/month before rollout; RUM SDK sampleRate is the knob.

## F-023 — AI-agent telemetry bill: "argus" ships 35 gen_ai spans (204665 tokens tracked)

**Category:** traces · **Severity:** low · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- service=argus gen_ai spans=35 attr_bytes=208 tokens_tracked=204665 — LLM telemetry per the gen_ai.* semantic conventions

**Remediation:** Attribution, not waste: this is what observing your AI agent costs. Compare it against the agent's own LLM spend to budget observability per investigation.

## F-024 — Unbounded ID "session.id" is a metric label on browser.sessions.count

**Category:** metrics · **Severity:** high · **Estimated saving:** directional (not reliably priceable)

**Evidence**

- metric "browser.sessions.count" carries label "session.id" on 37 series
- an unbounded ID as a metric label multiplies series count by user/session/request count — a structural time bomb at ANY current cardinality (today: 37 series)

**Remediation:** Delete label "session.id" from these metrics at the collector (transform processor). Per-session.id analysis belongs in traces/logs, where high cardinality is free.

> Generated fix: see the `drop_metric_label` block annotated `F-024` in `collector-fragment.yaml`.

## F-025 — Cardinality discipline held: 2 unbounded ID attribute(s) stay off metric labels

**Category:** quality · **Severity:** low · **Estimated saving:** directional (not reliably priceable)

**Evidence**

- order.id (68176 distinct values on spans)
- user.id (500 distinct values on spans)
- these IDs live on spans (where cardinality is free) and appear on NO metric series — exactly right

**Remediation:** Nothing to fix — keep it this way. This is the discipline the label-bomb findings ask for.

## F-026 — demo-app mixes severity casing ("Info")

**Category:** quality · **Severity:** low · **Estimated saving:** directional (not reliably priceable)

**Evidence**

- service="demo-app" severity_text="Info" count=344 — case-sensitive filters will miss these

**Remediation:** Uppercase severity_text at the SDK or collector so filters match once.

## Sampling safety proof

```
Sampling simulator replay (168711 traces, 8953804 spans)
  policy: keep errors + keep >= 750 ms + sample 5.0% of the rest
  error traces:  110430 / 110430 kept
  slow traces:   145 / 145 kept
  boring traces: 2947 / 58136 kept (94.9% dropped)
  span volume dropped: 6.9%
  verdict: SAFE — policy retains 100% of error traces and 100% of slow traces
```

---
*Estimates are uncompressed-ingest figures extrapolated from the scan window; findings without a $ figure are directional. TELELENS profilers are read-only (SELECT/GET only); nothing is applied without a human running `foundryctl cast`.*
