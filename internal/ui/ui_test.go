package ui

import (
	"strings"
	"testing"
)

func TestTableAlignsAndRules(t *testing.T) {
	out := Table(
		[]Column{{Name: "id", Width: 5}, {Name: "gb", Width: 6, Right: true}},
		[][]string{{"F-001", "37.0"}, {"F-002", "2.2"}},
	)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want header+rule+2 rows, got %d lines: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], "ID") || !strings.Contains(lines[0], "GB") {
		t.Errorf("header should be uppercased: %q", lines[0])
	}
	if !strings.Contains(lines[1], "─") {
		t.Errorf("want hairline rule under header, got %q", lines[1])
	}
	// Right-aligned numeric column: the value ends the cell.
	if !strings.HasSuffix(strings.TrimRight(lines[2], " "), "37.0") {
		t.Errorf("numeric column should be right-aligned: %q", lines[2])
	}
}

func TestErrorShape(t *testing.T) {
	out := Error("boom", "because", "this")
	for _, want := range []string{"Error: boom", "Why:   because", "Try:   this"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	if got := Error("boom", "", ""); strings.Contains(got, "Why:") || strings.Contains(got, "Try:") {
		t.Errorf("empty why/try must be omitted: %q", got)
	}
}

func TestVerdictColorsOnlyVerdictLines(t *testing.T) {
	if Verdict("  boring traces: 5/100 kept") != "  boring traces: 5/100 kept" {
		t.Error("non-verdict lines must pass through unstyled")
	}
}
