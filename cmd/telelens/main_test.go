package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/vinayaksonthalia/telelens/internal/findings"
)

// H2: one failing profiler (e.g. a transient ClickHouse MEMORY_LIMIT_EXCEEDED)
// must not abort the scan — the healthy profilers' findings are kept, the
// failure is reported with Why:/Try: guidance, and the scan maps to the
// distinct "partial" exit code.
func TestRunProfilersDegradesGracefully(t *testing.T) {
	ok := func(title string) profilerStep {
		return profilerStep{title, func() ([]findings.Finding, error) {
			return []findings.Finding{{Title: title}}, nil
		}}
	}
	boom := profilerStep{"traces", func() ([]findings.Finding, error) {
		return nil, fmt.Errorf("after 3 attempts: code 241: MEMORY_LIMIT_EXCEEDED: would use 6.98 GiB")
	}}

	var progress bytes.Buffer
	all, skipped := runProfilers([]profilerStep{boom, ok("logs"), ok("metrics")}, &progress)

	if len(all) != 2 {
		t.Fatalf("healthy profilers' findings thrown away: got %d findings, want 2", len(all))
	}
	if len(skipped) != 1 || skipped[0].name != "traces" {
		t.Fatalf("skipped = %+v, want exactly the traces profiler", skipped)
	}
	out := progress.String()
	for _, want := range []string{"traces profiler skipped", "Why:", "Try:", "memory pressure", "re-run"} {
		if !strings.Contains(out, want) {
			t.Errorf("progress output missing %q:\n%s", want, out)
		}
	}

	err := scanResultErr(skipped)
	var p partialScanErr
	if !errors.As(err, &p) {
		t.Fatalf("scanResultErr should return partialScanErr, got %T: %v", err, err)
	}
	if !strings.Contains(p.Error(), "traces") || !strings.Contains(p.Error(), "partial results") {
		t.Errorf("degraded-scan error unclear: %q", p.Error())
	}
}

// P5: the README's first code block IS the demo — pin the fixture scan's
// console summary so it cannot silently rot when fixtures change. If this
// fails, either fix the fixtures or update README.md's sample output.
func TestFixtureScanMatchesREADMESample(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	scanErr := run([]string{"scan", "--fixtures=../../fixtures", "--out", t.TempDir()})
	w.Close()
	os.Stdout = old
	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)
	if scanErr != nil {
		t.Fatalf("fixture scan failed: %v", scanErr)
	}
	for _, want := range []string{
		"F-001",
		"DEBUG-log firehose from catalog is 50% of your log volume",
		"Total identified savings: 130.2 GB/month ≈ $39.06/month",
		"verdict: SAFE — policy retains 100% of error traces and 100% of slow traces",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fixture scan output drifted from the README sample; missing %q", want)
		}
	}
}

// A clean run must return no error (exit 0) and skip nothing.
func TestRunProfilersCleanRun(t *testing.T) {
	step := profilerStep{"quality", func() ([]findings.Finding, error) { return nil, nil }}
	var progress bytes.Buffer
	_, skipped := runProfilers([]profilerStep{step}, &progress)
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", skipped)
	}
	if err := scanResultErr(skipped); err != nil {
		t.Fatalf("clean scan must not error, got %v", err)
	}
}
