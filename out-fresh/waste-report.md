# TELELENS Waste Report

- **Scanned:** 2026-07-25T00:31:40Z (window: last 7 days, source: clickhouse (http://localhost:8123, window=7d), scan took 5.944s)
- **Pricing basis:** $0.30 per uncompressed-ingest GB (configurable via `--cost-per-gb`)
- **Total identified savings: 2.5 GB/month ≈ $0.75/month**

| # | Sev | Category | Finding | GB/mo | $/mo |
|---|-----|----------|---------|------:|-----:|
| F-001 | high | traces | 1856787 near-identical success spans are prime tail-sampling candidates | 2.3 | 0.68 |
| F-002 | high | metrics | Metric "checkout.cart.value.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-003 | high | metrics | 36 more metrics are written but read by no dashboard or alert | 0.0 | 0.01 |
| F-004 | high | metrics | Metric "http.server.duration.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-005 | high | metrics | Metric "http.server.response.size.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-006 | high | metrics | Metric "http.server.request.size.bucket" is written but read by no dashboard or alert | 0.0 | 0.01 |
| F-007 | medium | traces | Span attribute "url.full" has 57315 distinct values | 0.0 | 0.01 |
| F-008 | high | metrics | Metric "http.client.duration.bucket" is written but read by no dashboard or alert | 0.0 | 0.00 |
| F-009 | high | traces | Span attribute "order.id" has 158675 distinct values | 0.0 | 0.00 |
| F-010 | medium | logs | INFO firehose: gateway is 76% of your log volume | 0.0 | 0.00 |
| F-011 | medium | traces | Span attribute "app.order.id" has 65591 distinct values | 0.0 | 0.00 |
| F-012 | medium | logs | INFO firehose: orders is 23% of your log volume | 0.0 | 0.00 |
| F-013 | low | traces | Browser RUM telemetry bill: "meridian-web" ships 170 session spans across 21 sessions | 0.0 | 0.00 |
| F-014 | medium | traces | 9 more span attributes exceed 10000 distinct values | — | — |
| F-015 | low | quality | Cardinality discipline held: 2 unbounded ID attribute(s) stay off metric labels | — | — |

## F-001 — 1856787 near-identical success spans are prime tail-sampling candidates

**Category:** traces · **Severity:** high · **Estimated saving:** 2.3 GB/mo ≈ $0.68/mo

**Evidence**

- catalog "catalog.db_query": 169007 near-identical spans, error rate 0.000%, distinct-duration ratio 0.25
- catalog "catalog.cache_check": 169007 near-identical spans, error rate 0.000%, distinct-duration ratio 0.06
- catalog "catalog.enrich_pricing": 169006 near-identical spans, error rate 0.000%, distinct-duration ratio 0.13
- catalog "catalog.lookup_products": 169006 near-identical spans, error rate 0.000%, distinct-duration ratio 0.30
- checkout "checkout.validate_cart": 118079 near-identical spans, error rate 0.000%, distinct-duration ratio 0.26
- checkout "checkout.apply_discount": 118078 near-identical spans, error rate 0.000%, distinct-duration ratio 0.16
- gateway "gateway.auth_middleware": 118077 near-identical spans, error rate 0.000%, distinct-duration ratio 0.11
- payment "payment.fraud_check": 118077 near-identical spans, error rate 0.000%, distinct-duration ratio 0.41
- checkout "checkout.process_order": 118077 near-identical spans, error rate 0.000%, distinct-duration ratio 0.77
- payment "payment.validate_card": 118076 near-identical spans, error rate 0.000%, distinct-duration ratio 0.22
- payment "payment.gateway_call": 118075 near-identical spans, error rate 0.000%, distinct-duration ratio 0.73
- payment "payment.charge_card": 118075 near-identical spans, error rate 0.000%, distinct-duration ratio 0.75
- gateway "gateway.build_response": 118074 near-identical spans, error rate 0.000%, distinct-duration ratio 0.05
- gateway "gateway.handle_checkout_request": 118073 near-identical spans, error rate 0.000%, distinct-duration ratio 0.77

**Remediation:** Add a tail_sampling processor: keep 100% of errors, keep traces >= 750 ms, sample the remaining successes at 5%. Run `telelens simulate` for the safety proof.

> Generated fix: see the `tail_sampling` block annotated `F-001` in `collector-fragment.yaml`.

## F-002 — Metric "checkout.cart.value.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=checkout.cart.value.bucket series=16 samples_in_window=325120; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "checkout.cart.value.bucket": drop it at the collector (filter processor) or remove the instrument. Caveat: telelens can only see SigNoz dashboards and alert rules — if an external consumer (federated Prometheus, another pipeline) reads this metric, keep it and delete its line from the generated filter block during review.

> Generated fix: see the `drop_metric` block annotated `F-002` in `collector-fragment.yaml`.

## F-003 — 36 more metrics are written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- 36 further unread metrics totaling 284783 samples in window; checked 31 dashboards and 21 alert rules — zero references
- full list: checkout.cart.value.sum, checkout.cart.value.min, checkout.cart.value.count, checkout.cart.value.max, checkout.orders.count, http.server.duration.count, http.server.duration.sum, http.server.duration.min, http.server.response.size.count, http.server.response.size.min, http.server.response.size.sum, http.server.duration.max, http.server.response.size.max, http.server.active_requests, http.server.request.size.count, http.server.request.size.sum, http.server.request.size.max, http.server.request.size.min, http.client.duration.count, http.client.duration.min, http.client.duration.sum, http.client.duration.max, browser.web_vitals.ttfb.count, browser.web_vitals.ttfb.min, browser.web_vitals.ttfb.sum, browser.web_vitals.ttfb.max, browser.web_vitals.lcp.sum, browser.web_vitals.lcp.max, browser.web_vitals.fcp.sum, browser.web_vitals.lcp.min, browser.web_vitals.fcp.max, browser.web_vitals.fcp.min, browser.web_vitals.fcp.count, browser.web_vitals.inp.min, browser.web_vitals.inp.max, browser.web_vitals.inp.sum

**Remediation:** Same story at lower volume: drop them at the collector, or delete the instruments. The generated filter/drop_unread_metrics block lists every name (it ships commented-out — verify external consumers, then uncomment).

> Generated fix: see the `drop_metric` block annotated `F-003` in `collector-fragment.yaml`.

## F-004 — Metric "http.server.duration.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=http.server.duration.bucket series=528 samples_in_window=244960; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "http.server.duration.bucket": drop it at the collector (filter processor) or remove the instrument. Caveat: telelens can only see SigNoz dashboards and alert rules — if an external consumer (federated Prometheus, another pipeline) reads this metric, keep it and delete its line from the generated filter block during review.

> Generated fix: see the `drop_metric` block annotated `F-004` in `collector-fragment.yaml`.

## F-005 — Metric "http.server.response.size.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=http.server.response.size.bucket series=528 samples_in_window=244960; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "http.server.response.size.bucket": drop it at the collector (filter processor) or remove the instrument. Caveat: telelens can only see SigNoz dashboards and alert rules — if an external consumer (federated Prometheus, another pipeline) reads this metric, keep it and delete its line from the generated filter block during review.

> Generated fix: see the `drop_metric` block annotated `F-005` in `collector-fragment.yaml`.

## F-006 — Metric "http.server.request.size.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- metric=http.server.request.size.bucket series=256 samples_in_window=126208; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "http.server.request.size.bucket": drop it at the collector (filter processor) or remove the instrument. Caveat: telelens can only see SigNoz dashboards and alert rules — if an external consumer (federated Prometheus, another pipeline) reads this metric, keep it and delete its line from the generated filter block during review.

> Generated fix: see the `drop_metric` block annotated `F-006` in `collector-fragment.yaml`.

## F-007 — Span attribute "url.full" has 57315 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.01/mo

**Evidence**

- key="url.full" cardinality=57315 occurrences=65037 avg value 60B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "url.full" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-008 — Metric "http.client.duration.bucket" is written but read by no dashboard or alert

**Category:** metrics · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- metric=http.client.duration.bucket series=160 samples_in_window=73392; checked 31 dashboards and 21 alert rules — zero references

**Remediation:** Stop shipping "http.client.duration.bucket": drop it at the collector (filter processor) or remove the instrument. Caveat: telelens can only see SigNoz dashboards and alert rules — if an external consumer (federated Prometheus, another pipeline) reads this metric, keep it and delete its line from the generated filter block during review.

> Generated fix: see the `drop_metric` block annotated `F-008` in `collector-fragment.yaml`.

## F-009 — Span attribute "order.id" has 158675 distinct values

**Category:** traces · **Severity:** high · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="order.id" cardinality=158675 occurrences=192612 avg value 12B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "order.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-010 — INFO firehose: gateway is 76% of your log volume

**Category:** logs · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- service=gateway severity=INFO: 9266 records, 0.00 GB in window — 76% of ALL log bytes
- dominant template (973 occurrences): "HTTP Request: <*> <*> \"HTTP/1.1 200 OK\""

**Remediation:** Review "gateway"'s chattiest INFO templates: demote per-request noise to DEBUG (then filter), or aggregate to metrics. Not auto-dropped: INFO may carry real signal.

## F-011 — Span attribute "app.order.id" has 65591 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- key="app.order.id" cardinality=65591 occurrences=65581 avg value 25B — bloats attribute indexes and filter latency

**Remediation:** No action needed for cost — span cardinality is cheap here. Watch two edges: "app.order.id" must never become a metric label, and if its values grow large the jumbo-attribute finding will propose a truncation.

## F-012 — INFO firehose: orders is 23% of your log volume

**Category:** logs · **Severity:** medium · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- service=orders severity=INFO: 2759 records, 0.00 GB in window — 23% of ALL log bytes
- dominant template (946 occurrences): "HTTP Request: POST http://payments:8092/charge \"HTTP/1.1 200 OK\""

**Remediation:** Review "orders"'s chattiest INFO templates: demote per-request noise to DEBUG (then filter), or aggregate to metrics. Not auto-dropped: INFO may carry real signal.

## F-013 — Browser RUM telemetry bill: "meridian-web" ships 170 session spans across 21 sessions

**Category:** traces · **Severity:** low · **Estimated saving:** 0.0 GB/mo ≈ $0.00/mo

**Evidence**

- service=meridian-web spans_with_session.id=170 sessions=21 attr_bytes=864 (2.5 KB/session uncompressed)

**Remediation:** Attribution, not waste: this is the frontend-observability bill. Scale it by expected sessions/month before rollout; RUM SDK sampleRate is the knob.

## F-014 — 9 more span attributes exceed 10000 distinct values

**Category:** traces · **Severity:** medium · **Estimated saving:** directional (not reliably priceable)

**Evidence**

- attributes (cardinality): app.email.message_id (54780), db.statement (54630), payment.transaction.id (53243), app.payment.transaction.id (53062), shipping.tracking.id (53051), app.shipping.tracking.id (53045), app.user.id (43377), http.url (10068), app.email.recipient (10011) — same advisory as above: cheap on spans, dangerous as metric labels

**Remediation:** Same advisory at lower rank: none of these needs action for cost; keep every one of them off metric labels.

## F-015 — Cardinality discipline held: 2 unbounded ID attribute(s) stay off metric labels

**Category:** quality · **Severity:** low · **Estimated saving:** directional (not reliably priceable)

**Evidence**

- order.id (158675 distinct values on spans)
- user.id (500 distinct values on spans)
- these IDs live on spans (where cardinality is free) and appear on NO metric series — exactly right

**Remediation:** Nothing to fix — keep it this way. This is the discipline the label-bomb findings ask for.

## Sampling safety proof

```
Sampling simulator replay (75586 traces, 845103 spans)
  policy: keep errors + keep >= 750 ms + sample 5.0% of the rest
  error traces:  27 / 27 kept
  slow traces:   22 / 22 kept
  boring traces: 3925 / 75537 kept (94.8% dropped)
  span volume dropped: 94.7%
  verdict: SAFE — policy retains 100% of error traces and 100% of slow traces
```

---
*Estimates are uncompressed-ingest figures extrapolated from the scan window; findings without a $ figure are directional. TELELENS profilers are read-only (SELECT/GET only); nothing is applied without a human running `foundryctl cast`.*
