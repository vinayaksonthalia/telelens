# Why We Built It This Way — how we decided to make telemetry itself the subject

**In one line:** We read all 22 of SigNoz's public idea cards, noticed the whole Query Builder
column was really a *capability checklist* (not project ideas) and that one card (#11666, a
data-quality checker) gestured at exactly the analysis we wanted to build — so we built the
analysis layer that *subsumes* the card, took it all the way to dollars, and made sure the real
analysis exercised every query capability on the list.

## ELI10

Before building, we looked at what everyone else would build. Most teams would make "a dashboard
about their app." We asked a weirder question: what if the *subject* of the dashboard was the
telemetry itself? Then we noticed SigNoz had basically published a *list* of query skills worth
showing off — so we made sure our real analysis used every single one, and we found the one idea
card that was closest to our plan and made our tool the bigger thing it was reaching for. This
page is that thinking.

---

## Step 1 — read the whole idea board

SigNoz's ideas board lists 22 GitHub issues across five columns. We fetched and read every one.
Two observations shaped everything:

1. **The "Showcase Query Builder" column (#11673–#11678) isn't a set of project ideas — it's a
   syllabus.** It lists capabilities worth exercising *in anger*: parenthesized boolean logic,
   EXISTS/NOT EXISTS, JSON-body predicates, array `hasAll`/`hasAny`, `sumIf`/`count_distinct`/
   `rate`, cross-context group-by + having, order-by + limit. A project that *exercises* those
   hardest is demonstrating real mastery of SigNoz's query surface.

2. **One card, #11666 (an ingested data-quality checker, "OTel native, evaluated via SigNoz
   MCP"), gestured at exactly the analysis we wanted** — but stopped at "quality." We wanted to go
   further: detect the waste, *price it*, generate the fix, and *prove it safe.*

---

## Step 2 — the contrarian read: make telemetry the subject

Here's the swerve. Most observability projects build a dashboard *about their application* —
request rates, error charts, an SLO pack. We built the dashboard about *the telemetry itself.*
Observability *for* your observability.

That reframing is what unlocks the whole project's differentiation. Once telemetry is the subject:

- The output is **money**, the most legible impact a tool like this can have.
- The natural features are ones that *exist nowhere in OSS* — a sampling-safety simulator, a
  "written but never read" cross-reference.
- And the tool can do something a single project rarely gets to: **audit its own siblings.** On a
  shared instance, TELELENS priced ARGUS's `gen_ai.*` telemetry and GLASSPANE's browser sessions
  — an ecosystem story built into the analysis.

The verdict we wrote down: TELELENS is *the analysis layer the data-quality card gestures at, with
money attached* — each idea card is a single feature, and TELELENS is the product that subsumes
several of them.

---

## Step 3 — exercise the full Query Builder surface

Because the Query Builder column is really a capability checklist, we made TELELENS *demonstrably*
exercise each capability in service of a real analysis (not as a contrived demo):

| Capability | Where TELELENS uses it |
|---|---|
| boolean / EXISTS / NOT EXISTS | usage-xref is NOT-EXISTS over stored-vs-referenced metrics; "BOTH tracking IDs" cardinality panel |
| JSON body paths | the log profiler's body mining |
| `hasAll` array search | span-events array filter in the Cardinality Explorer |
| `sumIf` / `count_distinct` / `rate` | `rate` on meter metrics, `count_distinct` on attributes, `countIf` in the duplicate-span SQL |
| group-by + having | *every* profiler query (`GROUP BY … HAVING …`) |
| order-by + limit | every profiler ends `ORDER BY <cost> DESC LIMIT n` — the Waste Report *is* an order-by over money |

Every one of those capabilities is used because a real cost-analysis query needed it, not to check
a box.

---

## What the research *changed* about the build

Three concrete refinements, all traceable in the shipped product:

1. **We added the MCP data-quality hook.** Card #11666's exact phrasing ("evaluated via SigNoz
   MCP") became a real, disclosed feature: `scan --json` is the agent surface, the quality
   findings are pure and deterministic, and an agent can cross-check any finding through the
   official SigNoz MCP (its `signoz_check_metric_cardinality` *independently confirmed* our
   `session.id` label finding). We tick the card's exact box while subsuming it.

2. **We committed to a deterministic, LLM-free core** *because* a measured, defensible number
   beats a plausible one. Cost findings are SQL + math with unit tests; the LLM only narrates.
   That's the strongest differentiator — a tool that asks an LLM "what's expensive?" can't defend
   a single figure under scrutiny.

3. **We made "measured, not projected" the headline** *because* it is the single most defensible
   impact claim a cost tool can make. The commercial tools we could find stop at recommendations; we applied
   the config to a live ingester and measured the drop. That decision is why the apply-and-measure
   wave (−95%, reverted, verified) exists at all.

---

## The differentiators we ended up with

When other cost-dashboard projects appear, these are the things a quick copy can't reproduce:

- **A deterministic, LLM-free core** — reproducible numbers that survive scrutiny.
- **The safety proof** — a binary SAFE/UNSAFE verdict backed by a replay over real traces, and a
  test that hard-fails if any emitted policy would drop an error trace.
- **Measured-not-projected savings** — a live before/after receipt nobody can fake without
  building the whole loop.
- **Read-only by construction** — safe to point at prod, the difference between a demo and a tool.
- **The cross-project audit** — pricing two sibling projects' telemetry (and catching a real bug
  in one). Unfakeable without an ecosystem.

---


## Related

- [00-the-big-picture.md](00-the-big-picture.md) — the market gap (self-hosted SigNoz has no Ingest Guard) this research uncovered.
- [02-signoz-deep-dive.md](02-signoz-deep-dive.md) — the two query dialects this analysis pushed us to master.
- [04-faq/hard-questions-answered.md](04-faq/hard-questions-answered.md) — the hard questions about these decisions, answered.
