package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/findings"
)

// Report is the machine-readable scan output (findings.json).
type Report struct {
	GeneratedAt     time.Time          `json:"generated_at"`
	Source          string             `json:"source"`
	WindowDays      int                `json:"window_days"`
	CostPerGB       float64            `json:"cost_per_gb"`
	ScanDuration    string             `json:"scan_duration"`
	TotalGBPerMonth float64            `json:"total_savings_gb_per_month"`
	TotalUSDPerMth  float64            `json:"total_savings_usd_per_month"`
	Findings        []findings.Finding `json:"findings"`
	Simulation      *analyze.SimResult `json:"simulation,omitempty"`
}

// NewReport assembles a report from ranked findings.
func NewReport(source string, windowDays int, costPerGB float64, scanDur time.Duration,
	fs []findings.Finding, sim *analyze.SimResult) Report {
	gb, usd := findings.TotalSavings(fs)
	return Report{
		GeneratedAt:     time.Now().UTC(),
		Source:          source,
		WindowDays:      windowDays,
		CostPerGB:       costPerGB,
		ScanDuration:    scanDur.Round(time.Millisecond).String(),
		TotalGBPerMonth: gb,
		TotalUSDPerMth:  usd,
		Findings:        fs,
		Simulation:      sim,
	}
}

// JSON renders findings.json.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Markdown renders waste-report.md: the ranked, human-readable Waste Report.
func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# TELELENS Waste Report\n\n")
	fmt.Fprintf(&b, "- **Scanned:** %s (window: last %d days, source: %s, scan took %s)\n",
		r.GeneratedAt.Format(time.RFC3339), r.WindowDays, r.Source, r.ScanDuration)
	fmt.Fprintf(&b, "- **Pricing basis:** $%.2f per uncompressed-ingest GB (configurable via `--cost-per-gb`)\n", r.CostPerGB)
	fmt.Fprintf(&b, "- **Total identified savings: %.1f GB/month ≈ $%.2f/month**\n\n", r.TotalGBPerMonth, r.TotalUSDPerMth)

	fmt.Fprintf(&b, "| # | Sev | Category | Finding | GB/mo | $/mo |\n")
	fmt.Fprintf(&b, "|---|-----|----------|---------|------:|-----:|\n")
	for _, f := range r.Findings {
		gb, usd := "—", "—"
		if !f.Directional() {
			gb = fmt.Sprintf("%.1f", f.EstGBPerMonth)
			usd = fmt.Sprintf("%.2f", f.EstUSDPerMonth)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			f.ID, f.Severity, f.Category, escapePipes(f.Title), gb, usd)
	}
	b.WriteString("\n")

	for _, f := range r.Findings {
		fmt.Fprintf(&b, "## %s — %s\n\n", f.ID, f.Title)
		fmt.Fprintf(&b, "**Category:** %s · **Severity:** %s · **Estimated saving:** %s\n\n",
			f.Category, f.Severity, savingsLine(f))
		b.WriteString("**Evidence**\n\n")
		for _, e := range f.Evidence {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		fmt.Fprintf(&b, "\n**Remediation:** %s\n\n", f.Remediation)
		if f.Fix.Kind != findings.FixNone {
			fmt.Fprintf(&b, "> Generated fix: see the `%s` block annotated `%s` in `collector-fragment.yaml`.\n\n",
				f.Fix.Kind, f.ID)
		}
	}

	if r.Simulation != nil {
		b.WriteString("## Sampling safety proof\n\n```\n")
		b.WriteString(r.Simulation.SafetyReport())
		b.WriteString("\n```\n\n")
	}

	b.WriteString("---\n*Estimates are uncompressed-ingest figures extrapolated from the scan window; ")
	b.WriteString("findings without a $ figure are directional. TELELENS profilers are read-only ")
	b.WriteString("(SELECT/GET only); nothing is applied without a human running `foundryctl cast`.*\n")
	return b.String()
}

func savingsLine(f findings.Finding) string {
	if f.Directional() {
		return "directional (not reliably priceable)"
	}
	return fmt.Sprintf("%.1f GB/mo ≈ $%.2f/mo", f.EstGBPerMonth, f.EstUSDPerMonth)
}

func escapePipes(s string) string { return strings.ReplaceAll(s, "|", "\\|") }

// PostToMCP is the thin, optional, experimental MCP-evaluation hook (ideas
// board #11666 suggests evaluating data-quality findings with the SigNoz
// MCP). When TELELENS_MCP_URL is set, the findings summary is POSTed there
// as a JSON-RPC-shaped message for downstream evaluation. Failure is
// non-fatal: the hook never affects scan results.
func PostToMCP(ctx context.Context, mcpURL string, r Report) error {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "telelens/findings",
		"params": map[string]any{
			"total_savings_gb_per_month":  r.TotalGBPerMonth,
			"total_savings_usd_per_month": r.TotalUSDPerMth,
			"findings":                    r.Findings,
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("mcp hook: HTTP %d", resp.StatusCode)
	}
	return nil
}
