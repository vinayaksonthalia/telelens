# TELELENS — Learning Folder

This folder explains TELELENS **twice over**: simply enough for a curious 10-year-old, and precisely enough for a skeptical engineer. It's the teaching companion to the product — every claim here is grounded in TELELENS's own evidence (the live runs in `../assets/`, the code in `../internal/`, the notes in `../LEARNINGS.md`), verified against the project's real files rather than paraphrased from memory.

Every file follows the same shape: **In one line → an ELI10 analogy → the real mechanics with real examples → honest limits → why it matters → links to related files.** Illustrations are the hand-drawn explainer set in `../assets/illustrations/`.

> **The one-sentence version:** point it at a SigNoz instance; it reads your real ClickHouse read-only, tells you in GB/month and dollars which telemetry is waste, writes the OpenTelemetry Collector config that fixes it, *proves the fix is safe* by replaying it over your real traces, and — applied live — measured span storage falling **95.0%** with every error trace kept, then reverted cleanly. **Observability for your observability.**

---

## How to read it (pick your path)

- **"Explain it like I'm new"** → [00-the-big-picture.md](00-the-big-picture.md) → [01-how-it-works.md](01-how-it-works.md) → [04-faq/newbie-glossary.md](04-faq/newbie-glossary.md).
- **"Give me substance fast"** → [00-the-big-picture.md](00-the-big-picture.md) → [04-faq/hard-questions-answered.md](04-faq/hard-questions-answered.md) → [04-faq/honest-limits-what-we-dont-claim.md](04-faq/honest-limits-what-we-dont-claim.md).
- **"How does it actually work?"** → [01-how-it-works.md](01-how-it-works.md) → [02-signoz-deep-dive.md](02-signoz-deep-dive.md) → [03-the-tech-stack.md](03-the-tech-stack.md).
- **"Tell me the story"** → [05-why-we-built-it-this-way.md](05-why-we-built-it-this-way.md) → [06-bug-hunt.md](06-bug-hunt.md).

---

## The map

- **[00-the-big-picture.md](00-the-big-picture.md)** — why TELELENS exists: telemetry cost as the loudest complaint in observability, self-hosted SigNoz's missing Ingest Guard, and why we chose it. *For: everyone.*
- **[01-how-it-works.md](01-how-it-works.md)** — the pipeline: read-only store → six profilers → Drain miner → transparent pricing → ranked report → generated fix → the safety simulator. *For: a smart beginner.*
- **[02-signoz-deep-dive.md](02-signoz-deep-dive.md)** — everything SigNoz taught us: the ClickHouse schemas (incl. `signoz_meter`), the two query dialects, the `system.metric_log` OOM, "price from the table that stores the thing," and the dashboard-API traps. *For: engineers.*
- **[03-the-tech-stack.md](03-the-tech-stack.md)** — every choice and the honest trade-off: single Go binary, the read-only store seam, the deterministic LLM-free core, Drain, and golden-file-tested generators. *For: engineers.*
- **[04-faq/](04-faq/)** — the Q&A trio:
  - [hard-questions-answered.md](04-faq/hard-questions-answered.md) — the hardest questions (is −95% cherry-picked? safe on prod? did an AI make up the numbers?), answered with evidence.
  - [honest-limits-what-we-dont-claim.md](04-faq/honest-limits-what-we-dont-claim.md) — every known limitation, stated plainly.
  - [newbie-glossary.md](04-faq/newbie-glossary.md) — every jargon term, one plain paragraph each.
- **[05-why-we-built-it-this-way.md](05-why-we-built-it-this-way.md)** — how we decided to make telemetry itself the subject, subsume idea-card #11666, and exercise the full Query Builder surface. *For: everyone.*
- **[06-bug-hunt.md](06-bug-hunt.md)** — the war diary: the database-observing-itself OOM, the self-contradicting report, the config that would have blanked the APM page, and F-024. *For: engineers.*

---

## The five-line cheat sheet

1. **Scan** — read-only ClickHouse `SELECT`s + SigNoz `GET`s feed six profilers (traces/logs/metrics/usage-xref/quality/ecosystem).
2. **Price** — transparent uncompressed-bytes × `$/GB` math (deterministic, no LLM); unpriceable findings ship "directional."
3. **Rank** — the Waste Report is an order-by over money; ranking is a feature, not formatting.
4. **Generate** — an annotated collector fragment + Foundry casting patch, one block per finding — a proposal a human applies.
5. **Prove** — the simulator replays the policy over your real traces and returns a binary SAFE/UNSAFE verdict; live, −95.0% measured on healthy known-volume traffic with 31/31 errors kept, then reverted (on an error-storm day the same policy only drops ~6%, because it will not drop errors — see [honest-limits](04-faq/honest-limits-what-we-dont-claim.md)).
