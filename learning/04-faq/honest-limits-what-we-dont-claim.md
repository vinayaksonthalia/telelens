# Honest Limits — what we do NOT claim

**In one line:** The boundaries of TELELENS, stated plainly — because a tool whose whole value is *trustworthy numbers* is only as credible as its willingness to name where the numbers stop.

## ELI10

A good accountant tells you "this is a solid estimate of your bill, but it's not the exact invoice, and here's the one thing it can't see." That honesty is what makes you trust the rest of the math. This is our list of "here's what TELELENS can't do (yet)," said out loud on purpose — including the number we're *least* flattered by.

---

## Limit 1 — pricing is a transparent model, not your cloud invoice

**We do NOT claim** to reproduce your exact bill.

Pricing is uncompressed-ingest bytes × a configurable `$/GB` — the dimension ingestion-based pricing bills on, with documented envelope constants (300 B/span, 150 B/log record, 32 B/metric sample). It's built to *rank offenders* and give a defensible magnitude, not to match your AWS line item. Findings that can't be priced reliably ship as **"directional"** rather than inventing a figure. On a small instance, read the *ratios*, not the absolute dollars.

---

## Limit 2 — the −95% is a healthy-traffic number; error storms yield ~6%

**We do NOT claim** 95% savings on every workload.

The measured −95% is the tail-sampling *sweet spot*: mostly-healthy traffic where boring successes dominate. On an error-storm day (68% of traces carrying errors in our live window), the same policy reported **only ~6% droppable** — because it refuses to drop errors. That's not a failure; **it's the feature.** On a failing system, TELELENS tells you your problem is the errors, not the storage. The report always says which regime you're in; we lead with the honest range, not the flattering peak.

---

## Limit 3 — the simulator models tail_sampling, not a packet-level collector

**We do NOT claim** a full collector emulation.

The simulator faithfully replays the **3 shipped `tail_sampling` policy types** over per-trace summaries — it's a model of the decision logic, not a packet-level emulation of the OpenTelemetry Collector. One real-world edge it *can't* model: `tail_sampling` holds traces for `decision_wait` (default 10 s), and traces longer than that are evaluated on partial data in a live collector. The rollback runbook flags this: watch your longest-running workflows in the first minutes after apply.

---

## Limit 4 — applying the fix requires human review (by design)

**We do NOT claim** autonomous remediation.

TELELENS *never applies anything.* It writes a proposal (`out/`), a human reviews it, drops the blocks that don't fit, backs up the config, and applies it. This isn't a missing feature — it's the safety model. Our own generated config once listed platform-critical spanmetrics in a drop block (see [../06-bug-hunt.md](../06-bug-hunt.md)); the review step is what caught it. **The operator-review step is the product.** The `filter/drop_unread_metrics` block even ships *commented out* behind a review banner.

---

## Limit 5 — "unread" can only see the consumers it can enumerate

**We do NOT claim** to know every reader of a metric.

The usage cross-reference finds metrics no *SigNoz dashboard or alert rule* references. It **cannot see** external consumers — the platform's own internal reads (spanmetrics feeding the APM page), or an external Grafana, script, or agent querying your data. So an "unread" finding is a **question for a human**, never an automatic drop. We denylist the known platform-consumed `signoz_*` metrics, but the general limit stands: absence of a *known* reader isn't proof of no reader.

---

## Limit 6 — schema coverage is SigNoz v0.13x

**We do NOT claim** support for every SigNoz version.

Live mode feature-detects the schema (`signoz_index_v3` / `logs_v2` / `time_series_v4`) and **refuses unrecognized versions with a clear error** rather than guessing (guessing a schema is how you corrupt a price). Older schema families are roadmap. Refusing cleanly is more honest than silently mispricing an unknown layout.

---

## Limit 7 — sub-hour measurements count rows, because the meter is hourly

**We do NOT claim** the meter can resolve short windows.

The `signoz_meter` connector flushes **hourly buckets**, so a sub-hour before/after A/B (like our apply-and-measure run) can't be read from the meter — we counted *storage rows* directly instead, and both numbers are in the evidence. Measured savings also depend on workload stationarity; we always report the measurement window and traffic volumes alongside the %.

---

## Limit 8 — the demo instance is small; multi-TB needs the documented degradations

**We do NOT claim** it scales to a multi-terabyte store unchanged.

The performance budget (≤90 s) is enforced, and heavy queries are time-sliced and memory-guarded — but a multi-TB store would need the documented sampled-read degradations. We built and measured on a small live instance and say so; the ratios generalize, the absolute runtimes don't.

---

## The one-paragraph version (for a reader in a hurry)

Pricing is a **transparent model, not a cloud invoice** (directional where it can't be sure). The **−95% is a healthy-traffic number**; error storms honestly yield ~6%, because the policy refuses to drop errors. The **simulator models tail_sampling's decision logic**, not a packet-level collector, and can't model `decision_wait` timing. **Applying is always a human** — the review step is the product, and it's what caught our own near-miss. **"Unread" can't see external consumers**, so it's a question, never an auto-drop. Schema coverage is **v0.13x** (unknown versions refused cleanly), sub-hour measurements **count rows** because the meter is hourly, and the demo instance is small. Everything on the hero path — the live scan, the safety proof, the measured savings, the revert — is verified with evidence in `assets/`.

---

## Related

- [hard-questions-answered.md](hard-questions-answered.md) — the full FAQ.
- [newbie-glossary.md](newbie-glossary.md) — definitions for tail sampling, cardinality, meter, spanmetrics, and more.
- [../06-bug-hunt.md](../06-bug-hunt.md) — how we found these limits (including the ones we caught in ourselves).
- [../../DOCS.md](../../DOCS.md) — the canonical honest-caveats list and rollback runbook.
