# Newbie Glossary

**In one line:** Every nerdy word in the TELELENS docs, explained the way you'd explain it to a smart 10-year-old, with one line tying it back to TELELENS.

## ELI10

Grown-ups invented a lot of fancy words for simple ideas. This page un-fancies them. Read any term you bumped into and you'll get a plain picture plus how *we* actually use it. Terms are alphabetical; each ends with **→ in TELELENS:** so you always see the real connection.

---

### cardinality
How many *different values* a label can take. Low cardinality (`region: us/eu`) is cheap; high cardinality (a unique ID per user) explodes a metric into millions of separate series and wrecks the database.
**→ in TELELENS:** the metrics profiler finds *which label* is the bomb (e.g. `user_id` → 210k series) and prices it.

### ClickHouse
The very fast column database SigNoz stores all its telemetry in. TELELENS reads it directly.
**→ in TELELENS:** every profiler is a read-only ClickHouse `SELECT`; the biggest live bug was ClickHouse's *own* `system.metric_log` eating memory.

### collector (OpenTelemetry Collector)
The pipeline program that receives telemetry, processes it, and forwards it to storage. You configure it with "processors" (sampling, filtering, transforming).
**→ in TELELENS:** the generated fix is a collector config fragment — `tail_sampling`, `filter`, `transform` processors, one block per finding.

### Drain
An algorithm that groups log lines into templates by treating the changing bits (numbers, IDs) as wildcards, so thousands of similar lines become one pattern.
**→ in TELELENS:** the log profiler uses Drain to say "this *one template* is half your log bytes."

### findings.json / waste report
The machine-readable and human-readable versions of the ranked list of waste, each item priced in GB/month and dollars.
**→ in TELELENS:** `scan` writes both; `--json` is the agent surface, the markdown is for humans.

### gen_ai.* / tokens
Standard OpenTelemetry attributes describing an LLM call (model, input/output **tokens** — the chunks of text LLMs bill by).
**→ in TELELENS:** the ecosystem profiler prices AI-agent telemetry — e.g. 35 `gen_ai.*` spans carrying 204,665 tracked tokens (ARGUS's own).

### guardrail alert
An alert rule that fires if a problem *comes back* after you fixed it.
**→ in TELELENS:** the pack seeds three (ingest-bytes spike, cardinality explosion, log-volume anomaly) so waste can't silently return.

### ingest / ingest bytes
"Ingest" is telemetry coming *in* to be stored. Ingestion-based pricing bills on how many bytes you ingest.
**→ in TELELENS:** pricing is uncompressed-ingest bytes × your `$/GB` — the dimension the bill is actually charged on.

### jumbo attribute
An attribute (a field on a span) that's huge and repeated everywhere — like a 4 KB SQL statement stamped on every single span.
**→ in TELELENS:** a jumbo `db.statement` averaging 4200 bytes across millions of spans is a top trace finding.

### label / series (metrics)
A metric's **labels** are its tags; each unique *combination* of label values is one **time series** the database stores separately. More label values = more series = more cost.
**→ in TELELENS:** a label bomb is a label whose values explode the series count; the metrics profiler names it.

### meter (signoz_meter) / spanmetrics
`signoz_meter` is SigNoz's usage-accounting database (ingest rate by signal/service). **Spanmetrics** (`signoz_calls_total`, `signoz_latency.*`) are metrics SigNoz derives from spans to power its APM page.
**→ in TELELENS:** the Cost Overview reads `signoz_meter.*`; and we learned *never* to "drop" spanmetrics — the platform reads them even when no dashboard queries them.

### query_range (v5) / query builder vs raw SQL
`query_range` is SigNoz's v5 query API. The **Query Builder** is its high-level, dashboard-friendly form; **raw ClickHouse SQL** is the low-level form.
**→ in TELELENS:** the dashboards mix both on purpose — builder panels for portability, raw SQL for schema-level forensics.

### RED metrics
Rate, Errors, Duration — the three vital-sign metrics for a service. Sampling must never blind them.
**→ in TELELENS:** the generated config keeps `signozspanmetrics` *before* `tail_sampling`, so RED metrics still see every span even while traces are sampled.

### read-only by construction
The profiler refuses to write to your telemetry data — the client rejects any non-`SELECT` statement, and the least-privilege `telelens_ro` user in `.env.example` enforces the same rule at the database. Enforced in code, not just promised.
**→ in TELELENS:** the live client refuses any non-`SELECT` statement, and a least-privilege `telelens_ro` DB user enforces it at the database.

### sampling / tail sampling
**Sampling** = keeping only some telemetry to save money. **Tail sampling** decides *after* seeing a whole trace — so it can keep every error and slow trace and drop only boring ones.
**→ in TELELENS:** the generated policy is tail sampling; the simulator proves it keeps 100% of errors before you apply it.

### schema drift / feature-detection
Different SigNoz versions store data in different table shapes ("schemas"). **Feature-detection** checks which shape you have before querying.
**→ in TELELENS:** startup detects `signoz_index_v3`/`logs_v2`/`time_series_v4` and *refuses* unknown versions rather than mispricing them.

### series count vs sample count
A **series** is one label-combination; a **sample** is one datapoint written to it. They live in *different tables* (`time_series_v4` stores hourly series rows; `distributed_samples_v4` stores datapoints).
**→ in TELELENS:** pricing must come from the table that stores the thing — mixing them was wrong by orders of magnitude.

### simulator / safety invariant
A replay that tries a proposed sampling policy on your *real* traces and shows you, before you apply it, exactly which traces it would keep and drop. The **invariant** is the rule the tool refuses to violate: never emit a policy that drops an error trace — a violation prints UNSAFE and exits non-zero. (What the replay cannot model is the collector's `decision_wait` window; see [honest-limits-what-we-dont-claim.md](honest-limits-what-we-dont-claim.md).)
**→ in TELELENS:** `simulate` returns a binary SAFE/UNSAFE verdict and *exits non-zero* if the policy would drop one error trace.

### usage cross-reference (write-only telemetry)
Checking which stored metrics are actually *read* by any dashboard or alert — the ones that aren't are write-only waste.
**→ in TELELENS:** the usage-xref profiler is NOT-EXISTS semantics: stored-but-never-referenced metrics, flagged (but never auto-dropped).

### Foundry / casting patch
Foundry is SigNoz's deployment tool; a "casting" is its config. A **casting patch** is a change to that config.
**→ in TELELENS:** one of the two generated fix formats is a Foundry `ingester.config.data` casting patch — the install tool used as an application surface.

---

## Related

- [hard-questions-answered.md](hard-questions-answered.md) — the FAQ that uses all these terms.
- [honest-limits-what-we-dont-claim.md](honest-limits-what-we-dont-claim.md) — the honest boundaries.
- [../01-how-it-works.md](../01-how-it-works.md) — the terms in action across the pipeline.
- [../02-signoz-deep-dive.md](../02-signoz-deep-dive.md) — deeper on the ClickHouse schema terms.
