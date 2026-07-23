package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/findings"
	"github.com/vinayaksonthalia/telelens/internal/store"
)

// Thresholds for the log profiler.
const (
	// FirehoseSharePct: a single service's low-severity logs above this share
	// of total log bytes is a firehose finding.
	FirehoseSharePct = 20.0
	// TemplateSharePct: one mined template above this share of a service's
	// log bytes is called out individually.
	TemplateSharePct = 25.0
	// DuplicateTemplateMin: exact-duplicate template count before a dedup
	// finding fires.
	DuplicateTemplateMin = 10000
)

var lowSeverities = map[string]bool{
	"DEBUG": true, "TRACE": true, "debug": true, "trace": true,
	"Debug": true, "Trace": true,
}

// Logs profiles severity distribution (firehose detection), mines templates
// Drain-style, and detects exact-duplicate log lines.
func Logs(ctx context.Context, st store.TelemetryStore, pricer analyze.Pricer) ([]findings.Finding, error) {
	var out []findings.Finding

	sevs, err := st.LogSeverityBreakdown(ctx)
	if err != nil {
		return nil, fmt.Errorf("log profiler: %w", err)
	}
	var totalLogBytes float64
	perServiceBytes := map[string]float64{}
	for _, s := range sevs {
		b := analyze.LogBytes(s.Count, s.Bytes)
		totalLogBytes += b
		perServiceBytes[s.Service] += b
	}

	samples, err := st.LogBodySamples(ctx)
	if err != nil {
		return nil, fmt.Errorf("log profiler: %w", err)
	}
	records := make([]analyze.LogRecord, 0, len(samples))
	for _, s := range samples {
		records = append(records, analyze.LogRecord{
			Service: s.Service, Severity: s.SeverityText, Body: s.Body, Weight: s.Weight,
		})
	}
	templates := analyze.MineTemplates(records, analyze.DefaultDrainConfig())

	// Firehose detection per (service, severity): any single stream dominating
	// total log bytes is worth a look. The remediation depends on severity —
	// DEBUG/TRACE gets a drop filter; ERROR gets "fix the source" (dropping
	// errors is malpractice); INFO gets a review nudge, never an auto-drop.
	for _, s := range sevs {
		bytes := analyze.LogBytes(s.Count, s.Bytes)
		share := 100 * bytes / max(totalLogBytes, 1)
		if share < FirehoseSharePct {
			continue
		}
		if !lowSeverities[s.SeverityText] {
			sev := strings.ToUpper(s.SeverityText)
			gb := pricer.GBPerMonth(bytes)
			f := findings.Finding{
				Category:       findings.CategoryLogs,
				EstGBPerMonth:  gb,
				EstUSDPerMonth: pricer.USDPerMonth(gb),
				Evidence: []string{fmt.Sprintf(
					"service=%s severity=%s: %d records, %.2f GB in window — %.0f%% of ALL log bytes",
					s.Service, orUnset(sev), s.Count, bytes/1e9, share)},
				Fix: findings.FixHint{Kind: findings.FixNone, Service: s.Service},
			}
			switch sev {
			case "ERROR", "FATAL", "CRITICAL":
				f.Severity = findings.SeverityHigh
				f.Title = fmt.Sprintf("ERROR storm: %s emits %.0f%% of all log bytes at %s", s.Service, share, sev)
				f.Remediation = fmt.Sprintf("Do NOT drop these — they are errors. Fix the failing code path in "+
					"%q; the storage cost disappears when the bug does. (Savings shown = what fixing it reclaims.)", s.Service)
			default: // INFO / WARN / unset
				f.Severity = findings.SeverityMedium
				f.Title = fmt.Sprintf("%s firehose: %s is %.0f%% of your log volume", orUnset(sev), s.Service, share)
				f.Remediation = fmt.Sprintf("Review %q's chattiest %s templates: demote per-request noise to DEBUG "+
					"(then filter), or aggregate to metrics. Not auto-dropped: %s may carry real signal.",
					s.Service, orUnset(sev), orUnset(sev))
			}
			for _, t := range templates {
				if t.Service == s.Service && strings.EqualFold(t.Severity, s.SeverityText) {
					f.Evidence = append(f.Evidence, fmt.Sprintf(
						"dominant template (%d occurrences): %q", t.Count, ellipsis(t.Text, 120)))
					break
				}
			}
			out = append(out, f)
			continue
		}
		evidence := []string{fmt.Sprintf(
			"service=%s severity=%s: %d records, %.1f GB in window — %.0f%% of ALL log bytes",
			s.Service, s.SeverityText, s.Count, bytes/1e9, share)}
		for _, t := range templates {
			if t.Service == s.Service && lowSeverities[t.Severity] {
				evidence = append(evidence, fmt.Sprintf(
					"dominant template (%d occurrences): %q", t.Count, ellipsis(t.Text, 120)))
				break // templates are cost-sorted; the first match is the top one
			}
		}
		gb := pricer.GBPerMonth(bytes)
		out = append(out, findings.Finding{
			Category:       findings.CategoryLogs,
			Severity:       findings.SeverityCritical,
			Title:          fmt.Sprintf("%s-log firehose from %s is %.0f%% of your log volume", strings.ToUpper(s.SeverityText), s.Service, share),
			Evidence:       evidence,
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: fmt.Sprintf("Drop severity < INFO for service %q at the collector (filter "+
				"processor), or fix the logger config at the source. Nothing below INFO from this service "+
				"is ever queried.", s.Service),
			Fix: findings.FixHint{
				Kind:        findings.FixFilterLogs,
				Service:     s.Service,
				MaxSeverity: "INFO",
			},
		})
	}

	// Exact-duplicate log detection (dedup, not sampling).
	for _, t := range templates {
		if !t.Exact || t.Count < DuplicateTemplateMin {
			continue
		}
		bytes := analyze.LogBytes(t.Count, t.Bytes)
		gb := pricer.GBPerMonth(bytes * 0.99) // keep one per interval
		out = append(out, findings.Finding{
			Category: findings.CategoryLogs,
			Severity: findings.SeverityMedium,
			Title:    fmt.Sprintf("%s emits the byte-identical line %d times", t.Service, t.Count),
			Evidence: []string{fmt.Sprintf("service=%s severity=%s line=%q count=%d",
				t.Service, t.Severity, ellipsis(t.Text, 120), t.Count)},
			EstGBPerMonth:  gb,
			EstUSDPerMonth: pricer.USDPerMonth(gb),
			Remediation: "Identical lines carry no information after the first: rate-limit at the " +
				"source or aggregate to a counter metric.",
			Fix: findings.FixHint{Kind: findings.FixNone, Service: t.Service},
		})
	}

	return out, nil
}

func orUnset(sev string) string {
	if strings.TrimSpace(sev) == "" {
		return "UNSET-severity"
	}
	return sev
}

func ellipsis(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
