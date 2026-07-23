# The Big Picture — why TELELENS exists

**In one line:** TELELENS is observability *for* your observability — point it at a SigNoz instance and it tells you, in gigabytes and dollars, exactly which telemetry is wasting your storage, generates the OpenTelemetry Collector config that fixes it, and *proves the fix is safe* by replaying it against your real traces before you apply anything.

---

## ELI10

Imagine your phone keeps warning you it's out of storage. You could just buy a bigger phone — or you could open the settings screen that shows *which apps and photos are actually eating the space*, delete the junk, and keep everything that matters. Most people never see that screen, so they just keep paying for more storage.

SigNoz stores your telemetry (all the little notes your computers write about what they're doing), and that storage costs money. But there's no "what's eating my storage" screen. TELELENS is that screen. It reads your telemetry database, adds up who's wasting the most, writes you a to-do list *with dollar amounts*, and even writes the exact fix — then, before you touch anything, it runs a fire drill to prove the fix won't accidentally throw away the important stuff (the error reports you actually need at 2am).

---

## The problem, in plain words

Telemetry is data your systems emit about themselves — traces, logs, metrics. It's incredibly useful, and it's incredibly easy to emit *too much* of. A single chatty `DEBUG` log line, left on in production, can become half your log bill. A well-meaning engineer adds `user_id` as a label on a metric, and suddenly that one metric explodes into hundreds of thousands of separate time series. Health-check spans that all say "everything's fine" pile up by the millions. And a huge fraction of what gets stored is **never queried by anyone** — write-only telemetry you pay to keep and never read.

The industry number is stark: the CNCF estimates **60–95% of telemetry spend is reducible.** But reducing it is scary, because the obvious lever — *sampling* (keeping only some traces) — feels like gambling with the exact data you need during an incident. "Turn on sampling" is a sentence anyone can say; "here's proof you won't lose a single error trace" is a product almost nobody builds.

So the real problem is two problems stacked: **you can't see where the waste is, and even if you could, you're afraid to cut it.**

---

## The market gap we aimed at

SigNoz *Cloud* has a feature called Ingest Guard for exactly this. **Self-hosted SigNoz has nothing.** The 27,000-star open-source community — the people running SigNoz on their own hardware precisely to control costs — has *no* ingestion intelligence at all. TELELENS is the missing community edition of Ingest Guard: the cost-and-cardinality brain for the users who chose self-hosted to save money and then can't see where the money goes.

And the output is the most legible impact a tool like this can have: **money.** Not "a prettier dashboard" — an itemized bill with a savings number at the bottom.

(How we did that positioning research — reading the official idea board and deciding to make *telemetry itself* the subject — is in [05-why-we-built-it-this-way.md](05-why-we-built-it-this-way.md).)

![Where the bill comes from: a DEBUG-log firehose, label bombs, and write-only telemetry all pouring into ClickHouse — money spent storing data nobody reads.](../assets/illustrations/01-where-the-bill-comes-from.png)

---

## Why *we* chose it

Three reasons, in order:

1. **The output is dollars, and dollars are undeniable.** Most observability tooling builds a dashboard *about your app.* TELELENS builds the dashboard about *your telemetry itself*, and reports it in GB/month and $/month. Anyone who runs an observability stack feels that in their budget.

2. **The safety proof is the mic-drop nobody else has.** Recommending sampling without proving it's safe is malpractice. So TELELENS ships a **simulator** that replays a proposed tail-sampling policy over your *real* traces and returns a binary verdict — SAFE or UNSAFE — where a policy that would drop even *one* error trace is rejected in code. "What did you lose?" is the question every cost tool dodges; TELELENS answers it with a receipt.

3. **It can be pointed at production without fear.** The whole tool is **read-only by construction** — it issues only ClickHouse `SELECT`s and SigNoz `GET`s, refuses non-SELECT statements in code, and documents a least-privilege database user. The only things it writes are files you review. That's the difference between a demo and a tool you'd actually run.

---

## The one-paragraph version

TELELENS is a read-only telemetry cost & cardinality profiler for self-hosted SigNoz. It runs `SELECT`-only queries against SigNoz's ClickHouse and `GET`s against the SigNoz API, then produces three things: a **ranked Waste Report** priced in GB/month and dollars (DEBUG firehoses, label bombs, jumbo attributes, duplicate spans, write-only metrics, data-quality defects, even the cost of AI-agent and browser-RUM telemetry); a **generated OpenTelemetry Collector config** that fixes what it found, annotated one block per finding; and a **tail-sampling simulator** that replays the proposed policy over your real traces and returns a hard SAFE/UNSAFE verdict. Applied to a live SigNoz ingester, the generated config cut span storage **95.0%** on healthy traffic while keeping **31/31** injected error traces and leaving RED metrics untouched — then was reverted and the revert verified. The honest counterweight: on an error-storm day the same replay reports only ~6% droppable, *because the policy refuses to drop errors — and that refusal is the feature.* The thesis: everyone else builds a dashboard about their app; TELELENS makes telemetry itself the subject, prices the waste, and proves the cure is safe.

---

## Why it matters

- **Measured, not projected.** The headline is a *before/after receipt* from a live ingester (−95.0%), not a hopeful estimate. Every commercial rival stops at "recommendations."
- **It audits its own siblings.** On a shared instance, TELELENS priced ARGUS's AI-agent telemetry and GLASSPANE's browser sessions — and *caught a real cardinality bug in GLASSPANE's own metrics.* An ecosystem story a single tool rarely gets to tell (see [06-bug-hunt.md](06-bug-hunt.md)).
- **The most on-theme bug we hit.** The first thing TELELENS found on the live box was that *ClickHouse's own internal logging was out-consuming the telemetry it stores.* A cost profiler found the platform's introspection to be the biggest waste. (Full story in [02-signoz-deep-dive.md](02-signoz-deep-dive.md).)

---

## Related

- [01-how-it-works.md](01-how-it-works.md) — the profilers, the pricing, and the safety simulator.
- [02-signoz-deep-dive.md](02-signoz-deep-dive.md) — the ClickHouse schemas, the two query dialects, and the metric_log OOM.
- [05-why-we-built-it-this-way.md](05-why-we-built-it-this-way.md) — how we decided to make telemetry itself the subject.
- [04-faq/hard-questions-answered.md](04-faq/hard-questions-answered.md) — the hard questions, answered.
