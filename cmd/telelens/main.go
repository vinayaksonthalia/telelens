// telelens — telemetry cost & cardinality profiler for SigNoz.
//
//	telelens scan     profile traces/logs/metrics, write the Waste Report and
//	                  the generated collector config (the full loop)
//	telelens report   re-render waste-report.md from an existing findings.json
//	telelens generate re-render collector-fragment.yaml + casting-patch.yaml
//	telelens simulate replay a tail-sampling policy and print the safety report
//
// Profilers are strictly read-only (ClickHouse SELECTs and SigNoz GETs).
// The only writes are files in --out; applying them is always a human action.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
	"github.com/vinayaksonthalia/telelens/internal/config"
	"github.com/vinayaksonthalia/telelens/internal/findings"
	"github.com/vinayaksonthalia/telelens/internal/generate"
	"github.com/vinayaksonthalia/telelens/internal/profile"
	"github.com/vinayaksonthalia/telelens/internal/signozapi"
	"github.com/vinayaksonthalia/telelens/internal/store"
	"github.com/vinayaksonthalia/telelens/internal/ui"
)

// hintErr carries clig.dev-shaped context (what/why/try) to the top-level
// error printer.
type hintErr struct {
	err      error
	why, try string
}

func (h hintErr) Error() string { return h.err.Error() }
func (h hintErr) Unwrap() error { return h.err }

// partialScanErr signals that the scan completed with one or more profilers
// skipped: results were written, but the report is degraded. It maps to a
// distinct exit code (2) so scripts can tell "partial" from "failed" (1).
type partialScanErr struct{ skipped []string }

func (p partialScanErr) Error() string {
	return fmt.Sprintf("scan degraded: %d profiler(s) skipped (%s) — partial results written",
		len(p.skipped), strings.Join(p.skipped, ", "))
}

const exitPartial = 2

func main() {
	if err := run(os.Args[1:]); err != nil {
		var p partialScanErr
		if errors.As(err, &p) {
			fmt.Fprintln(os.Stderr, ui.Error(p.Error(),
				"a profiler failed mid-scan (often transient ClickHouse memory pressure); the remaining profilers' findings were kept.",
				"re-run the scan (transient pressure usually clears), reduce --window, or raise the ClickHouse memory limits."))
			os.Exit(exitPartial)
		}
		var h hintErr
		if errors.As(err, &h) {
			fmt.Fprintln(os.Stderr, ui.Error(h.err.Error(), h.why, h.try))
		} else {
			fmt.Fprintln(os.Stderr, ui.Error(err.Error(), "", ""))
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return hintErr{
			err: fmt.Errorf("missing subcommand"),
			why: "telelens needs to know what to do: scan, report, generate, or simulate.",
			try: "start with `telelens scan --fixtures` — it needs no SigNoz instance and writes a full report to out/.",
		}
	}
	switch args[0] {
	case "scan":
		return cmdScan(args[1:])
	case "report":
		return cmdReport(args[1:])
	case "generate":
		return cmdGenerate(args[1:])
	case "simulate":
		return cmdSimulate(args[1:])
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		usage()
		return hintErr{
			err: fmt.Errorf("unknown subcommand %q", args[0]),
			why: "the only subcommands are scan, report, generate, simulate and help.",
			try: "run `telelens help` for the full usage, or `telelens scan --fixtures` for the offline demo.",
		}
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `telelens — telemetry cost & cardinality profiler for SigNoz (read-only)

usage:
  telelens scan     [--fixtures[=dir]] [--window N] [--cost-per-gb X] [--out dir]
  telelens report   [--findings out/findings.json] [--out dir]
  telelens generate [--findings out/findings.json] [--out dir]
  telelens simulate [--fixtures[=dir]] [--sample-pct 5] [--latency-ms 750] [--json]

environment (see .env.example): CLICKHOUSE_HTTP_URL, CLICKHOUSE_USER,
CLICKHOUSE_PASSWORD, SIGNOZ_API_URL, SIGNOZ_API_KEY, COST_PER_GB,
TELELENS_WINDOW_DAYS, TELELENS_OUT_DIR, TELELENS_MCP_URL
`)
}

// fixtureFlag lets --fixtures work both bare (--fixtures) and with a
// directory (--fixtures=path); the stdlib flag package supports this via
// IsBoolFlag on a custom Value.
type fixtureFlag struct {
	set bool
	dir string
}

func (f *fixtureFlag) String() string   { return f.dir }
func (f *fixtureFlag) IsBoolFlag() bool { return true }
func (f *fixtureFlag) Set(v string) error {
	f.set = true
	if v != "" && v != "true" {
		f.dir = v
	}
	return nil
}
func (f *fixtureFlag) Dir() string {
	if f.dir == "" {
		return "fixtures"
	}
	return f.dir
}

// commonFlags registers the flags shared by scan/simulate and returns the
// fixture selector (unset = live mode).
func commonFlags(fs *flag.FlagSet, cfg *config.Config) *fixtureFlag {
	fixtures := &fixtureFlag{}
	fs.Var(fixtures, "fixtures", "run offline against fixture data; optional value is the fixture dir (default ./fixtures)")
	fs.IntVar(&cfg.WindowDays, "window", cfg.WindowDays, "scan window in days")
	fs.Float64Var(&cfg.CostPerGB, "cost-per-gb", cfg.CostPerGB, "price per uncompressed-ingest GB in $")
	fs.StringVar(&cfg.OutDir, "out", cfg.OutDir, "output directory for generated files")
	return fixtures
}

// openSources builds the telemetry store and API client per the config.
func openSources(ctx context.Context, cfg config.Config, fixtures *fixtureFlag) (store.TelemetryStore, signozapi.SignozAPI, error) {
	if fixtures.set {
		dir := fixtures.Dir()
		st, err := store.NewFixtureStore(dir)
		if err != nil {
			return nil, nil, err
		}
		return st, &signozapi.FixtureAPI{Dir: dir}, nil
	}
	st, err := store.NewClickHouseStore(ctx, store.ClickHouseConfig{
		BaseURL:    cfg.ClickHouseURL,
		User:       cfg.ClickHouseUser,
		Password:   cfg.ClickHousePassword,
		WindowDays: cfg.WindowDays,
	})
	if err != nil {
		return nil, nil, hintErr{
			err: err,
			why: fmt.Sprintf("live mode needs the ClickHouse HTTP interface reachable at %s (the default Foundry compose does not publish :8123 to the host).", cfg.ClickHouseURL),
			try: "run offline with `telelens scan --fixtures`, or set CLICKHOUSE_HTTP_URL / publish the port (see README §Live mode).",
		}
	}
	return st, signozapi.NewClient(cfg.SignozURL, cfg.SignozAPIKey), nil
}

func cmdScan(args []string) error {
	cfg := config.FromEnv()
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	fixtures := commonFlags(fs, &cfg)
	jsonOut := fs.Bool("json", false, "emit the full findings report as JSON on stdout (for agents/MCP pipelines); progress goes to stderr")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	start := time.Now()

	st, api, err := openSources(ctx, cfg, fixtures)
	if err != nil {
		return err
	}
	pricer := analyze.NewPricer(cfg.WindowDays, cfg.CostPerGB)
	progress := os.Stdout
	if *jsonOut {
		progress = os.Stderr // keep stdout pure JSON for machine consumers
	}
	fmt.Fprintf(progress, "%s %s\n", ui.Title("telelens scan"), ui.Dim("· source: "+st.Description()))

	steps := []profilerStep{
		{"traces", func() ([]findings.Finding, error) { return profile.Traces(ctx, st, pricer) }},
		{"logs", func() ([]findings.Finding, error) { return profile.Logs(ctx, st, pricer) }},
		{"metrics", func() ([]findings.Finding, error) { return profile.Metrics(ctx, st, pricer, time.Now()) }},
		{"usage-xref", func() ([]findings.Finding, error) { return profile.UsageXref(ctx, st, api, pricer) }},
		{"ecosystem", func() ([]findings.Finding, error) { return profile.Ecosystem(ctx, st, pricer) }},
		{"quality", func() ([]findings.Finding, error) { return profile.Quality(ctx, st) }},
	}
	all, skipped := runProfilers(steps, progress)
	if len(skipped) == len(steps) {
		// Nothing survived: this is a hard failure, not a degraded scan.
		return hintErr{
			err: fmt.Errorf("all %d profilers failed (first: %v)", len(steps), skipped[0].err),
			why: "every profiler query failed — the data source itself is unhealthy or unreachable.",
			try: "check ClickHouse/SigNoz connectivity and memory pressure, then re-run; or run offline with --fixtures.",
		}
	}
	ranked := findings.Rank(all)

	// Replay the recommended sampling policy for the safety proof.
	var sim *analyze.SimResult
	if f, ok := tailPolicy(ranked); ok {
		traces, err := st.TraceSummaries(ctx)
		if err != nil {
			return hintErr{
				err: fmt.Errorf("simulator: %w", err),
				why: "the profilers found a tail-sampling candidate, but fetching the trace summaries for the safety replay failed — so the policy could not be proven safe, and shipping an unproven policy is not something telelens will do.",
				try: "re-run with a smaller --window (the replay reads every trace in the window), or check ClickHouse memory pressure.",
			}
		}
		res := analyze.Simulate(analyze.Policy{
			SamplingPct:        f.Fix.SamplingPct,
			LatencyThresholdMS: f.Fix.LatencyThresholdMS,
		}, traces)
		sim = &res
	}

	report := generate.NewReport(st.Description(), cfg.WindowDays, pricer.CostPerGB, time.Since(start), ranked, sim)
	if err := writeOutputs(cfg.OutDir, report, ranked); err != nil {
		return err
	}

	if *jsonOut {
		jsonBytes, err := report.JSON()
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		fmt.Fprintf(progress, "scan completed in %s · outputs in %s/\n", time.Since(start).Round(time.Millisecond), cfg.OutDir)
		return scanResultErr(skipped)
	}

	// Console summary (design-system table: gray uppercase header, hairline
	// rule, right-aligned numerics, severity in semantic color).
	cols := []ui.Column{
		{Name: "id", Width: 6}, {Name: "sev", Width: 8}, {Name: "category", Width: 9},
		{Name: "finding", Width: 58}, {Name: "gb/mo", Width: 8, Right: true}, {Name: "$/mo", Width: 8, Right: true},
	}
	var rows [][]string
	for _, f := range ranked {
		gb, usd := "—", "—"
		if !f.Directional() {
			gb = fmt.Sprintf("%.1f", f.EstGBPerMonth)
			usd = fmt.Sprintf("%.2f", f.EstUSDPerMonth)
		}
		rows = append(rows, []string{
			f.ID, ui.Severity(string(f.Severity)), string(f.Category),
			ui.Truncate(f.Title, 58), gb, usd,
		})
	}
	fmt.Println()
	fmt.Print(ui.Table(cols, rows))
	gb, usd := findings.TotalSavings(ranked)
	fmt.Println("\n" + ui.Bold(fmt.Sprintf("Total identified savings: %.1f GB/month ≈ $%.2f/month", gb, usd)))
	if sim != nil {
		fmt.Println()
		for _, line := range strings.Split(sim.SafetyReport(), "\n") {
			fmt.Println(ui.Verdict(line))
		}
	}
	fmt.Println("\n" + ui.Dim(fmt.Sprintf("scan completed in %s · outputs in %s/", time.Since(start).Round(time.Millisecond), cfg.OutDir)))

	if cfg.MCPURL != "" {
		if err := generate.PostToMCP(ctx, cfg.MCPURL, report); err != nil {
			fmt.Fprintf(os.Stderr, "warning: MCP hook failed (non-fatal): %v\n", err)
		} else {
			fmt.Println("findings summary posted to MCP hook:", cfg.MCPURL)
		}
	}
	return scanResultErr(skipped)
}

// scanResultErr turns skipped profilers into the degraded-scan error (exit
// code 2); a clean scan returns nil.
func scanResultErr(skipped []skippedProfiler) error {
	if len(skipped) == 0 {
		return nil
	}
	names := make([]string, len(skipped))
	for i, s := range skipped {
		names[i] = s.name
	}
	return partialScanErr{skipped: names}
}

// profilerStep is one named profiler invocation in the scan pipeline.
type profilerStep struct {
	name string
	run  func() ([]findings.Finding, error)
}

// skippedProfiler records a profiler that failed and was skipped.
type skippedProfiler struct {
	name string
	err  error
}

// runProfilers executes every step, degrading gracefully: a failing profiler
// (e.g. a transient ClickHouse MEMORY_LIMIT_EXCEEDED) is skipped with a
// warning instead of aborting the scan — five healthy profilers' findings
// must not be thrown away because one query died (H2). The caller decides
// what a fully-failed run means.
func runProfilers(steps []profilerStep, progress io.Writer) ([]findings.Finding, []skippedProfiler) {
	var all []findings.Finding
	var skipped []skippedProfiler
	for _, step := range steps {
		found, err := step.run()
		if err != nil {
			skipped = append(skipped, skippedProfiler{step.name, err})
			why := "the profiler's query failed; the scan continues without its findings."
			try := "re-run the scan; if it persists, reduce --window."
			if strings.Contains(err.Error(), "MEMORY_LIMIT_EXCEEDED") {
				why = "ClickHouse refused the query under memory pressure (often transient merge/tracker pressure, not the query itself)."
				try = "re-run the scan in a minute, reduce --window, or raise max_server_memory_usage / restart merges on the ClickHouse side."
			}
			fmt.Fprintln(progress, ui.Error(
				fmt.Sprintf("%s profiler skipped: %v", step.name, err), why, try))
			continue
		}
		fmt.Fprintln(progress, ui.Dim(fmt.Sprintf("  %-10s %d findings", step.name, len(found))))
		all = append(all, found...)
	}
	return all, skipped
}

func tailPolicy(fs []findings.Finding) (findings.Finding, bool) {
	for _, f := range fs {
		if f.Fix.Kind == findings.FixTailSampling {
			return f, true
		}
	}
	return findings.Finding{}, false
}

func writeOutputs(outDir string, report generate.Report, ranked []findings.Finding) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonBytes, err := report.JSON()
	if err != nil {
		return err
	}
	files := map[string][]byte{
		"findings.json":           jsonBytes,
		"waste-report.md":         []byte(report.Markdown()),
		"collector-fragment.yaml": []byte(generate.OtelcolFragment(ranked)),
		"casting-patch.yaml":      []byte(generate.CastingPatch(ranked)),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(outDir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// deriveOutDir defaults report/generate output next to the findings file
// (P2): `report --findings other/findings.json` used to silently write to
// ./out. An explicit --out flag still wins.
func deriveOutDir(fs *flag.FlagSet, cfg *config.Config, findingsPath string) {
	outSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "out" {
			outSet = true
		}
	})
	if !outSet {
		cfg.OutDir = filepath.Dir(findingsPath)
	}
}

func loadReport(path string) (generate.Report, error) {
	var r generate.Report
	data, err := os.ReadFile(path)
	if err != nil {
		return r, hintErr{
			err: fmt.Errorf("read findings: %s", path),
			why: "report and generate re-render a report that a previous scan produced; that findings file does not exist yet.",
			try: "run `telelens scan --fixtures` (or a live scan) first, then re-run with --findings pointing at out/findings.json.",
		}
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, hintErr{
			err: fmt.Errorf("parse %s: %w", path, err),
			why: "the file exists but is not a findings report written by `telelens scan` (truncated, hand-edited, or a different JSON file).",
			try: "re-run `telelens scan` to regenerate it, or point --findings at the out/findings.json a scan produced.",
		}
	}
	return r, nil
}

func cmdReport(args []string) error {
	cfg := config.FromEnv()
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	path := fs.String("findings", filepath.Join(cfg.OutDir, "findings.json"), "path to findings.json from a previous scan")
	fs.StringVar(&cfg.OutDir, "out", cfg.OutDir, "output directory (default: the findings file's directory)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	deriveOutDir(fs, &cfg, *path)
	r, err := loadReport(*path)
	if err != nil {
		return err
	}
	out := filepath.Join(cfg.OutDir, "waste-report.md")
	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(r.Markdown()), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote", out)
	return nil
}

func cmdGenerate(args []string) error {
	cfg := config.FromEnv()
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	path := fs.String("findings", filepath.Join(cfg.OutDir, "findings.json"), "path to findings.json from a previous scan")
	fs.StringVar(&cfg.OutDir, "out", cfg.OutDir, "output directory (default: the findings file's directory)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	deriveOutDir(fs, &cfg, *path)
	r, err := loadReport(*path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return err
	}
	for name, data := range map[string]string{
		"collector-fragment.yaml": generate.OtelcolFragment(r.Findings),
		"casting-patch.yaml":      generate.CastingPatch(r.Findings),
	} {
		p := filepath.Join(cfg.OutDir, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", p)
	}
	return nil
}

func cmdSimulate(args []string) error {
	cfg := config.FromEnv()
	fs := flag.NewFlagSet("simulate", flag.ExitOnError)
	// simulate deliberately does NOT take --cost-per-gb/--out: it prices
	// nothing and writes nothing (it used to silently accept both — P3).
	fixtures := &fixtureFlag{}
	fs.Var(fixtures, "fixtures", "run offline against fixture data; optional value is the fixture dir (default ./fixtures)")
	fs.IntVar(&cfg.WindowDays, "window", cfg.WindowDays, "scan window in days")
	pct := fs.Float64("sample-pct", profile.DefaultSamplingPct, "probabilistic keep-rate for boring traces (%)")
	latency := fs.Float64("latency-ms", profile.DefaultLatencyThresholdMS, "always-keep latency threshold (ms)")
	jsonOut := fs.Bool("json", false, "emit the simulation result as JSON on stdout (for agents/MCP pipelines)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	st, _, err := openSources(ctx, cfg, fixtures)
	if err != nil {
		return err
	}
	traces, err := st.TraceSummaries(ctx)
	if err != nil {
		return err
	}
	res := analyze.Simulate(analyze.Policy{SamplingPct: *pct, LatencyThresholdMS: *latency}, traces)
	if *jsonOut {
		out, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	} else {
		fmt.Println(res.SafetyReport())
	}
	if !res.Safe {
		return hintErr{
			err: fmt.Errorf("policy rejected by safety invariant (NFR-4)"),
			why: "the replay dropped at least one error or slow trace. TELELENS refuses to recommend a policy that loses the traces you need during an incident — this is the product's core invariant, not a warning.",
			try: "raise --sampling-pct or lower --latency-ms until the replay keeps 100% of error and slow traces, then re-run; the report above shows which class was lost.",
		}
	}
	return nil
}
