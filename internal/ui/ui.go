// Package ui renders TELELENS's terminal output per the project design system
// (research/design-system.md §2b): lipgloss styling with adaptive light/dark
// colors, severity semantics shared with every other surface, hairline table
// rules, right-aligned numerics, and clig.dev error shape (Error/Why/Try).
// lipgloss auto-detects color support and degrades cleanly (NO_COLOR, pipes).
package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// bgDetectBudget caps how long we wait for the terminal to answer the OSC 11
// "what is your background colour?" query that AdaptiveColor needs.
const bgDetectBudget = 250 * time.Millisecond

// rnd is the renderer every style below is built from. It is deliberately a
// dedicated renderer rather than lipgloss's package-level default: the default
// serialises all of its methods behind one mutex, so an in-flight background
// probe would block every Render call for the full OSC timeout. With a private
// renderer we can resolve the probe on the default renderer, apply the answer
// here, and never contend with it.
var rnd = lipgloss.NewRenderer(os.Stdout)

// init resolves the terminal background once, with a deadline.
//
// AdaptiveColor needs to know whether the terminal is light or dark, which
// lipgloss answers by sending an OSC 11 query and waiting up to five seconds
// for a reply. Terminals that never reply — `script`, `docker run -t`, some
// multiplexer and CI configurations — spend that full five seconds, so a scan
// that finishes in 18 ms shows nothing at all and looks hung. Resolving it
// once, up front, with a short budget keeps answering terminals exact (they
// reply in single-digit milliseconds) and caps silent ones at 250 ms.
// Measured on a non-answering pty: 5.02 s to first output before, 0.27 s after.
func init() {
	rnd.SetColorProfile(rnd.ColorProfile()) // env-only detection; no terminal query
	dark := make(chan bool, 1)
	go func() { dark <- lipgloss.HasDarkBackground() }()
	select {
	case d := <-dark:
		rnd.SetHasDarkBackground(d)
	case <-time.After(bgDetectBudget):
		// No answer in time: assume dark, the overwhelmingly common default.
		rnd.SetHasDarkBackground(true)
	}
}

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

	header    = rnd.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
	hairline  = rnd.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"})
	bold      = rnd.NewStyle().Bold(true)
	dim       = rnd.NewStyle().Faint(true)
	accentSty = rnd.NewStyle().Foreground(Accent).Bold(true)
	safeSty   = rnd.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#22C55E"}).Bold(true)
	unsafeSty = rnd.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#EF4444"}).Bold(true)
)

// Severity renders a severity token in its semantic color.
func Severity(s string) string {
	if c, ok := sevColors[strings.ToLower(s)]; ok {
		return rnd.NewStyle().Foreground(c).Render(s)
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
