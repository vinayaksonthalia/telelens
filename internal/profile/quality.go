package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/vinayaksonthalia/telelens/internal/findings"
	"github.com/vinayaksonthalia/telelens/internal/store"
)

// Quality is the data-quality profiler (subsumes the SigNoz ideas-board card
// #11666, "Observability ingested data quality checker — OTel native"). It
// flags telemetry that is malformed or non-conformant with OTel semantic
// conventions: such data is often unqueryable, which makes it waste by
// definition regardless of size.
func Quality(ctx context.Context, st store.TelemetryStore) ([]findings.Finding, error) {
	var out []findings.Finding

	// Spans without service.name: unattributable, invisible in service views.
	volumes, err := st.SpanVolumeByService(ctx)
	if err != nil {
		return nil, fmt.Errorf("quality profiler: %w", err)
	}
	for _, v := range volumes {
		if strings.TrimSpace(v.Service) != "" && v.Service != "unknown_service" {
			continue
		}
		out = append(out, findings.Finding{
			Category: findings.CategoryQuality,
			Severity: findings.SeverityHigh,
			Title:    fmt.Sprintf("%d spans have no service.name resource attribute", v.Spans),
			Evidence: []string{fmt.Sprintf(
				"service=%q spans=%d — these never appear in any service-scoped view or SLO", v.Service, v.Spans)},
			Remediation: "Set OTEL_SERVICE_NAME (or resource attributes) on the emitting SDK/agent; " +
				"unattributable telemetry is paid for but unusable.",
			Fix: findings.FixHint{Kind: findings.FixNone},
		})
	}

	// Logs with missing or non-standard severity: break severity filters.
	sevs, err := st.LogSeverityBreakdown(ctx)
	if err != nil {
		return nil, fmt.Errorf("quality profiler: %w", err)
	}
	standard := map[string]bool{
		"TRACE": true, "DEBUG": true, "INFO": true, "WARN": true,
		"ERROR": true, "FATAL": true, "WARNING": true,
	}
	for _, s := range sevs {
		sev := strings.TrimSpace(s.SeverityText)
		if sev == "" {
			out = append(out, findings.Finding{
				Category: findings.CategoryQuality,
				Severity: findings.SeverityMedium,
				Title:    fmt.Sprintf("%s ships %d log records with no severity", orUnknown(s.Service), s.Count),
				Evidence: []string{fmt.Sprintf("service=%q severity_text='' count=%d — severity filters and alerts skip these",
					s.Service, s.Count)},
				Remediation: "Map the logger's level field to OTel severity in the SDK or with a " +
					"transform/logdedup processor; unlevelled logs cannot be tiered or filtered.",
				Fix: findings.FixHint{Kind: findings.FixNone, Service: s.Service},
			})
			continue
		}
		if !standard[strings.ToUpper(sev)] {
			out = append(out, findings.Finding{
				Category: findings.CategoryQuality,
				Severity: findings.SeverityLow,
				Title:    fmt.Sprintf("%s uses non-standard severity %q", orUnknown(s.Service), sev),
				Evidence: []string{fmt.Sprintf("service=%q severity_text=%q count=%d", s.Service, sev, s.Count)},
				Remediation: "Normalize to OTel severities (TRACE/DEBUG/INFO/WARN/ERROR/FATAL) so " +
					"severity-based queries and the generated filters apply uniformly.",
				Fix: findings.FixHint{Kind: findings.FixNone, Service: s.Service},
			})
		} else if sev != strings.ToUpper(sev) {
			out = append(out, findings.Finding{
				Category: findings.CategoryQuality,
				Severity: findings.SeverityLow,
				Title:    fmt.Sprintf("%s mixes severity casing (%q)", orUnknown(s.Service), sev),
				Evidence: []string{fmt.Sprintf("service=%q severity_text=%q count=%d — case-sensitive filters will miss these",
					s.Service, sev, s.Count)},
				Remediation: "Uppercase severity_text at the SDK or collector so filters match once.",
				Fix:         findings.FixHint{Kind: findings.FixNone, Service: s.Service},
			})
		}
	}

	// Metric names violating OTel naming (spaces, uppercase-heavy, etc.).
	series, err := st.MetricCardinality(ctx)
	if err != nil {
		return nil, fmt.Errorf("quality profiler: %w", err)
	}
	for _, m := range series {
		if isConformantMetricName(m.MetricName) {
			continue
		}
		out = append(out, findings.Finding{
			Category: findings.CategoryQuality,
			Severity: findings.SeverityLow,
			Title:    fmt.Sprintf("Metric name %q violates OTel naming conventions", m.MetricName),
			Evidence: []string{fmt.Sprintf("metric=%q series=%d — expected lowercase dot/underscore-separated names",
				m.MetricName, m.Series)},
			Remediation: "Rename at the source (or metricstransform) to lowercase dot-separated form; " +
				"non-conformant names dodge convention-based dashboards and processors.",
			Fix: findings.FixHint{Kind: findings.FixNone, Metric: m.MetricName},
		})
	}

	return out, nil
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "an unidentified service"
	}
	return s
}

// isConformantMetricName checks the OTel guidance: lowercase letters, digits,
// and separators (. _ /), starting with a letter.
func isConformantMetricName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		case r == '.' || r == '_' || r == '/':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
