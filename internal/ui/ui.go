// Package ui renders TELELENS's terminal output per the project design system
// (research/design-system.md §2b): lipgloss styling with adaptive light/dark
// colors, severity semantics shared with every other surface, hairline table
// rules, right-aligned numerics, and clig.dev error shape (Error/Why/Try).
// lipgloss auto-detects color support and degrades cleanly (NO_COLOR, pipes).
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TELELENS accent (teal — "efficiency, cool and clinical") + shared severity
// palette. Severity colors are semantic and never repurposed.
var (
	Accent = lipgloss.AdaptiveColor{Light: "#0D9488", Dark: "#14B8A6"}

	sevColors = map[string]lipgloss.AdaptiveColor{
		"critical": {Light: "#DC2626", Dark: "#EF4444"},
		"high":     {Light: "#D97706", Dark: "#F59E0B"},
		"medium":   {Light: "#CA8A04", Dark: "#EAB308"},
		"low":      {Light: "#6B7280", Dark: "#9CA3AF"},
	}

	header    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
	hairline  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"})
	bold      = lipgloss.NewStyle().Bold(true)
	dim       = lipgloss.NewStyle().Faint(true)
	accentSty = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	safeSty   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#22C55E"}).Bold(true)
	unsafeSty = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#EF4444"}).Bold(true)
)

// Severity renders a severity token in its semantic color.
func Severity(s string) string {
	if c, ok := sevColors[strings.ToLower(s)]; ok {
		return lipgloss.NewStyle().Foreground(c).Render(s)
	}
	return s
}

// Title renders the scan banner line.
func Title(s string) string { return accentSty.Render(s) }

// Bold renders emphasized text (totals line).
func Bold(s string) string { return bold.Render(s) }

// Dim renders secondary text (progress, timings).
func Dim(s string) string { return dim.Render(s) }

// Verdict colors a simulator verdict line green (SAFE) or red (UNSAFE).
func Verdict(line string) string {
	if strings.Contains(line, "verdict: SAFE") {
		return safeSty.Render(line)
	}
	if strings.Contains(line, "UNSAFE") {
		return unsafeSty.Render(line)
	}
	return line
}

// Column describes one findings-table column.
type Column struct {
	Name  string
	Width int
	Right bool // right-align (numeric columns, tabular figures)
}

// Table renders header (gray, bold, uppercase) + a hairline rule under the
// header only (no full grid — hairline-border philosophy), then rows.
// Cell styling is the caller's job; widths apply to the unstyled text.
func Table(cols []Column, rows [][]string) string {
	var b strings.Builder
	var heads []string
	total := 0
	for _, c := range cols {
		heads = append(heads, pad(strings.ToUpper(c.Name), c.Width, c.Right))
		total += c.Width + 1
	}
	b.WriteString(header.Render(strings.Join(heads, " ")) + "\n")
	b.WriteString(hairline.Render(strings.Repeat("─", total-1)) + "\n")
	for _, r := range rows {
		var cells []string
		for i, c := range cols {
			v := ""
			if i < len(r) {
				v = r[i]
			}
			cells = append(cells, pad(v, c.Width, c.Right))
		}
		b.WriteString(strings.Join(cells, " ") + "\n")
	}
	return b.String()
}

// pad aligns without breaking ANSI-styled cells (visible width via lipgloss).
func pad(s string, w int, right bool) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	space := strings.Repeat(" ", w-vis)
	if right {
		return space + s
	}
	return s + space
}

// Error renders a clig.dev-shaped error: what happened / why / what to try.
// Goes to stderr; Why/Try are optional.
func Error(what, why, try string) string {
	var b strings.Builder
	b.WriteString(unsafeSty.Render("Error: ") + what + "\n")
	if why != "" {
		b.WriteString(dim.Render("Why:   "+why) + "\n")
	}
	if try != "" {
		b.WriteString("Try:   " + try + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// Truncate shortens s to n visible chars with an ellipsis.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// Fmt is fmt.Sprintf, re-exported so callers don't import fmt just for tables.
func Fmt(format string, a ...any) string { return fmt.Sprintf(format, a...) }
