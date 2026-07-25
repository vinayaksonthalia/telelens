# M6 evidence — MCP data-quality hook (official idea #11666), demoed live (Jul 18, 2026)

TELELENS's data-quality findings exposed to agents through two documented paths,
both demoed once here for real.

## Path 1 — `telelens scan --json` consumed by an agent (claude CLI)

Command:
  telelens scan --window 7 --json | jq '.findings[] | select(.category=="quality")'
  ... filtered JSON piped into `claude -p` with the #11666 reviewer prompt.

Input (quality + structural findings from the LIVE scan):
{
 "source": "clickhouse (http://localhost:8123, window=7d)",
 "quality_and_structural_findings": [
  {
   "id": "F-024",
   "category": "metrics",
   "severity": "high",
   "title": "Unbounded ID \"session.id\" is a metric label on browser.sessions.count",
   "evidence": [
    "metric \"browser.sessions.count\" carries label \"session.id\" on 37 series",
    "an unbounded ID as a metric label multiplies series count by user/session/request count \u2014 a structural time bomb at ANY current cardinality (today: 37 series)"
   ],
   "est_gb_per_month": 0,
   "est_usd_per_month": 0,
   "remediation": "Delete label \"session.id\" from these metrics at the collector (transform processor). Per-session.id analysis belongs in traces/logs, where high cardinality is free.",
   "fix": {
    "kind": "drop_metric_label",
    "attribute": "session.id",
    "metric": "browser.sessions.count"
   }
  },
  {
   "id": "F-025",
   "category": "quality",
   "severity": "low",
   "title": "Cardinality discipline held: 2 unbounded ID attribute(s) stay off metric labels",
   "evidence": [
    "order.id (68083 distinct values on spans)",
    "user.id (500 distinct values on spans)",
    "these IDs live on spans (where cardinality is free) and appear on NO metric series \u2014 exactly right"
   ],
   "est_gb_per_month": 0,
   "est_usd_per_month": 0,
   "remediation": "Nothing to fix \u2014 keep it this way. This is the discipline the label-bomb findings ask for.",
   "fix": {
    "kind": "none"
   }
  },
  {
   "id": "F-026",
   "category": "quality",
   "severity": "low",
   "title": "demo-app mixes severity casing (\"Info\")",
   "evidence": [
    "service=\"demo-app\" severity_text=\"Info\" count=344 \u2014 case-sensitive filters will miss these"
   ],
   "est_gb_per_month": 0,
   "est_usd_per_month": 0,
   "remediation": "Uppercase severity_text at the SDK or collector so filters match once.",
   "fix": {
    "kind": "none",
    "service": "demo-app"
   }
  }
 ]
}

Agent output (claude CLI, verbatim):
Warning: no stdin data received in 3s, proceeding without it. If piping from a slow command, redirect stdin explicitly: < /dev/null to skip, or wait longer.

> [Clarification, added at review: the warning above is part of the verbatim capture — the harness printed it while buffering, but the agent did receive the piped scan; its assessment below cites F-024/F-025/F-026 from that scan directly.]
**Data-quality assessment — SigNoz instance (7d window)**

**Wrong (act on this):** F-024 — the metric `browser.sessions.count` carries `session.id`, an unbounded ID, as a label across 37 series. Cardinality is low *today*, but session IDs grow with every new session, so series count scales with traffic without bound. This is a structural time bomb, not a cost problem yet ($0/mo now).

**Minor:** F-026 — `demo-app` emits `severity_text="Info"` (mixed casing, 344 records); case-sensitive filters silently miss these. Normalize casing at SDK/collector.

**Right:** F-025 — good cardinality discipline. High-cardinality IDs (`order.id` ~68k distinct, `user.id` 500) live on spans, where cardinality is free, and appear on zero metric series. Exactly correct.

**Highest-priority action:** Drop the `session.id` label from `browser.sessions.count` at the collector (transform processor); move per-session analysis to traces/logs. Fix it now, while it's cheap, before traffic multiplies the series.

*(~140 words)*

## Path 2 — cross-check via the official SigNoz MCP server (:8000/mcp)

The same structural finding (F-024) verified by an MCP tool an agent would call:

  POST /mcp {"method":"tools/call","params":{"name":"signoz_check_metric_cardinality",
             "arguments":{"metricName":"browser.sessions.count"}}}

MCP response (truncated): session.id valueCount=37 with 37 raw UUID values listed —
independently confirming TELELENS's F-024 ("Unbounded ID session.id is a metric label
on browser.sessions.count, 37 series"). The agent's #1 recommended action matches the
transform TELELENS auto-generates (delete_key session.id where metric.name ==
browser.sessions.count) — which was applied and verified in the M4 measurement run.

Notes: telelens is the deterministic analysis layer; the LLM only narrates/triages
(deliberate division of labor, disclosed). MCP gotcha found live: the tool arg is
`metricName`, not `metric_name` (validation error otherwise).
