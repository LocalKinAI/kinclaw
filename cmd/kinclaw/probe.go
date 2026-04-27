// probe.go — implements the `kinclaw probe` subcommand.
//
//   kinclaw probe                       # probe focused app (TODO: not yet wired)
//   kinclaw probe Notes                  # probe by app name
//   kinclaw probe com.apple.Notes        # probe by bundle ID
//   kinclaw probe -json Notes            # JSON output
//   kinclaw probe -batch < ids.txt       # CSV batch mode (replaces probe-ax)
//   kinclaw probe -depth 4 -timeout 3s Notes  # tighten budgets
//
// The subcommand owns its own flag set (decoupled from the top-level flags)
// so adding more subcommands later (memory / doctor / forge / ...) doesn't
// pollute kinclaw -h with everyone's options.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/LocalKinAI/kinclaw/pkg/applifecycle"
	"github.com/LocalKinAI/kinclaw/pkg/probe"
)

// runProbe is the entrypoint for `kinclaw probe`. argv excludes the program
// name and the "probe" subcommand keyword (i.e. os.Args[2:]).
func runProbe(argv []string) {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `Usage: kinclaw probe [flags] [app]

Inspect a macOS app's Accessibility tree to assess "5-claw controllability":
how richly the app exposes its UI semantically. Result is one of four
verdicts:

  🟢 rich     — `+"`ui`"+` claw alone drives it (nodes >= 50, actionable >= 5)
  🟡 shallow  — needs `+"`ui`"+` + `+"`input`"+` (cmd-keys / type-text) hybrid
  🟠 blank    — needs `+"`record`"+` + screen + vision (menubar app, hostile shell)
  🔴 dead     — process didn't open (TCC / sandbox / not installed)

Args:
  app     Bundle ID ("com.apple.Notes"), app name ("Notes"), or .app path.
          Omit + use -batch for stdin-driven CSV scans.

Examples:
  kinclaw probe Notes
  kinclaw probe com.apple.Notes
  kinclaw probe -json Reminders
  kinclaw probe -batch < bundles.txt > results.csv
  kinclaw probe -depth 4 -no-activate Calculator     # fast, no window steal

Flags:
`)
		fs.PrintDefaults()
	}

	jsonOut := fs.Bool("json", false, "Output JSON instead of human-readable text")
	batch := fs.Bool("batch", false, "Read bundle IDs (one per line) from stdin; emit CSV to stdout")
	depth := fs.Int("depth", probe.DefaultMaxDepth, "Max AX tree walk depth")
	probeTimeout := fs.Duration("timeout", probe.DefaultProbeTimeout, "Probe walk time budget")
	openTimeout := fs.Duration("open-timeout", probe.DefaultOpenTimeout, "How long to wait for process to appear")
	settle := fs.Duration("settle", probe.DefaultActivateSettle, "Sleep after `osascript activate` for AppKit/Catalyst/Electron to draw windows")
	noActivate := fs.Bool("no-activate", false, "Skip `osascript activate` (for already-foregrounded apps; cold-launched apps will report nodes=1)")
	noCleanup := fs.Bool("no-cleanup", false, "Don't quit apps the probe opened (useful for single-app probe when you want to interact afterwards). Default: cleanup ON in -batch, OFF for single-app.")

	if err := fs.Parse(argv); err != nil {
		os.Exit(2)
	}

	if err := probe.EnsureAccessibility(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	opts := probe.Options{
		MaxDepth:       *depth,
		OpenTimeout:    *openTimeout,
		ProbeTimeout:   *probeTimeout,
		ActivateSettle: *settle,
		SkipActivate:   *noActivate,
	}

	if *batch {
		if fs.NArg() > 0 {
			fmt.Fprintln(os.Stderr, "probe: -batch reads bundle IDs from stdin; do not also pass an app argument.")
			os.Exit(2)
		}
		if *jsonOut {
			fmt.Fprintln(os.Stderr, "probe: -batch + -json not yet supported (use -batch alone for CSV).")
			os.Exit(2)
		}
		runBatch(opts, !*noCleanup)
		return
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "probe: must specify an app (or use -batch).")
		fs.Usage()
		os.Exit(2)
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "probe: only one app per call (or use -batch for many).")
		os.Exit(2)
	}

	bundleID, err := probe.ResolveBundleID(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: %v\n", err)
		os.Exit(2)
	}

	stats := probe.Probe(bundleID, opts)

	if *jsonOut {
		out, err := stats.JSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe: json marshal failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		return
	}

	fmt.Print(stats.Human())

	// Exit codes: 0 = rich/shallow, 1 = blank, 2 = dead.
	// Lets shell scripts gate on `kinclaw probe X || ...`.
	switch stats.Classify() {
	case probe.VerdictRich, probe.VerdictShallow:
		os.Exit(0)
	case probe.VerdictBlank:
		os.Exit(1)
	case probe.VerdictDead:
		os.Exit(2)
	}
}

// runBatch implements `kinclaw probe -batch < bundles.txt`. Drop-in
// compatible with the old standalone `probe-ax` binary's contract:
//   - stdin: bundle IDs, one per line, blank/comment lines ignored
//   - stdout: CSV (header + one row per app)
//   - stderr: per-app progress (so you can `tee` the CSV and watch progress)
//
// Batch mode also cleans up by default — apps the probe opened are quit
// at the end, so a 50-app scan doesn't leave you with 50 dock icons.
// Pass `-no-cleanup` to suppress.
func runBatch(opts probe.Options, cleanup bool) {
	var preexisting []string
	if cleanup {
		apps, err := applifecycle.RunningApps()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: probe couldn't snapshot running apps for cleanup: %v\n", err)
		} else {
			preexisting = apps
		}
		defer func() {
			if len(preexisting) == 0 {
				return
			}
			quit, failed := applifecycle.QuitNew(preexisting)
			if len(quit) > 0 {
				fmt.Fprintf(os.Stderr, "─── Cleanup: quit %d new app(s)\n", len(quit))
			}
			if len(failed) > 0 {
				fmt.Fprintf(os.Stderr, "─── Cleanup: %d app(s) refused to quit: %s\n",
					len(failed), strings.Join(failed, ", "))
			}
		}()
	}

	if err := probe.WriteCSVHeader(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "probe: csv header: %v\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	start := time.Now()
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		bundleID, err := probe.ResolveBundleID(line)
		if err != nil {
			// Resolution failure → emit a "dead" stub row so batch consumers
			// can see what was tried.
			stats := probe.Stats{BundleID: line, ErrMsg: "resolve: " + err.Error()}
			_ = probe.WriteCSVRow(os.Stdout, stats)
			fmt.Fprintf(os.Stderr, "  %-40s RESOLVE FAILED  %s\n", line, err)
			count++
			continue
		}
		stats := probe.Probe(bundleID, opts)
		_ = probe.WriteCSVRow(os.Stdout, stats)
		fmt.Fprintf(os.Stderr, "  %-40s nodes=%-5d btns=%-4d depth=%-2d  %s  %s\n",
			stats.BundleID, stats.TotalNodes, stats.Buttons, stats.MaxDepth,
			stats.Classify().Icon(), stats.ErrMsg)
		count++
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "probe: stdin read: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "─── %d apps probed in %s\n", count, time.Since(start).Round(time.Millisecond))
}
