// harvest.go — implements the `kinclaw harvest` subcommand.
//
//   kinclaw harvest                          # all sources, run pipeline → stage
//   kinclaw harvest --source claude-code     # one source
//   kinclaw harvest --diff                   # dry-run, no writes
//   kinclaw harvest --review                 # list staged candidates
//   kinclaw harvest --accept <source>/<skill-name>
//
// Compatibility flags (no-ops, default behavior already matches):
//   --all     — explicit "all sources"
//   --apply   — explicit "actually run the pipeline" (default)
//   --stage   — explicit "stage results" (default; matches launchd plist)
//   --no-critic — skip the critic spawn (cron / CI / offline runs)
//
// The pipeline always ends at staging (~/.localkin/harvest/staged/).
// Final acceptance into ./skills/ is always a manual `--accept` step.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LocalKinAI/kinclaw/pkg/harvest"
)

// runHarvest is the entrypoint for `kinclaw harvest`. argv excludes the
// program name and the "harvest" subcommand keyword (i.e. os.Args[2:]).
func runHarvest(argv []string) {
	fs := flag.NewFlagSet("harvest", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `Usage: kinclaw harvest [flags]

Pull candidate skills from third-party agent repos, validate them, and
stage survivors for human review.

Pipeline (per source):
  git clone --depth=1   (cached at ~/.localkin/harvest/sources/<name>/)
    → glob skill_paths from manifest
    → translate to SKILL.md form (identity for v1)
    → critic soul review (annotation, doesn't auto-reject)
    → forge quality gate v2 (auto-rejects malformed)
    → stage to ~/.localkin/harvest/staged/<source>/<skill-name>/

Manifest: %s

Modes (mutually exclusive — last one wins):
  (default)        Run pipeline for all sources; stage results
  --source NAME    Run pipeline for one source only
  --diff           Run pipeline but DON'T write to staging (dry-run)
  --review         List currently staged candidates (no pipeline run)
  --accept ID      Promote staged candidate ID into ./skills/
                   (ID is "<source>/<skill-name>" as printed by --review)

Flags:
`, harvest.DefaultManifestPath())
		fs.PrintDefaults()
	}

	manifestPath := fs.String("manifest", harvest.DefaultManifestPath(), "Path to harvest TOML manifest")
	sourceName := fs.String("source", "", "Run pipeline for one source only")
	diff := fs.Bool("diff", false, "Dry-run: show what would happen, write nothing")
	review := fs.Bool("review", false, "List staged candidates and exit")
	accept := fs.String("accept", "", "Promote a staged candidate (id: <source>/<skill-name>) into ./skills/")
	noCritic := fs.Bool("no-critic", false, "Skip the critic spawn (cron / CI / offline runs)")
	skillsDir := fs.String("skills-dir", "skills", "Destination for --accept (default ./skills)")

	// Compatibility flags — match the user's documented spec verbatim
	// even though they don't change behavior. Lets `kinclaw harvest
	// --all --stage` (the launchd plist form) work as-given.
	_ = fs.Bool("all", false, "Explicit 'all sources' (default behavior)")
	_ = fs.Bool("apply", false, "Explicit 'actually run' (default behavior)")
	_ = fs.Bool("stage", false, "Explicit 'stage results' (default behavior)")

	if err := fs.Parse(argv); err != nil {
		os.Exit(2)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "harvest: cannot resolve $HOME: %v\n", err)
		os.Exit(1)
	}

	// --review: list staged candidates and exit. No manifest needed.
	if *review {
		runHarvestReview(home)
		return
	}

	// --accept: promote one staged candidate. No manifest, no pipeline run.
	if *accept != "" {
		runHarvestAccept(home, *skillsDir, *accept)
		return
	}

	// All other modes need a manifest.
	manifest, err := harvest.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harvest: %v\n", err)
		if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
			fmt.Fprintf(os.Stderr, "\nCreate one based on the example at:\n  https://github.com/LocalKinAI/kinclaw/blob/main/harvest.example.toml\nor copy harvest.example.toml from the kinclaw repo to %s\n", *manifestPath)
		}
		os.Exit(1)
	}

	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "harvest: cannot locate kinclaw binary: %v\n", err)
		os.Exit(1)
	}

	criticSoul := resolveCriticSoul()
	if *noCritic {
		criticSoul = ""
	}

	opts := harvest.Options{
		Home:           home,
		KinclawBin:     bin,
		CriticSoulPath: criticSoul,
		SkipCritic:     *noCritic,
		DryRun:         *diff,
		Out:            os.Stderr,
	}

	ctx := context.Background()
	if *sourceName != "" {
		src := manifest.FindSource(*sourceName)
		if src == nil {
			fmt.Fprintf(os.Stderr, "harvest: source %q not in manifest\n", *sourceName)
			os.Exit(1)
		}
		r := harvest.RunSource(ctx, *src, opts)
		printSummary([]harvest.Result{r}, *diff)
		return
	}

	results := make([]harvest.Result, 0, len(manifest.Sources))
	for _, s := range manifest.Sources {
		results = append(results, harvest.RunSource(ctx, s, opts))
	}
	printSummary(results, *diff)
}

func runHarvestReview(home string) {
	staged, err := harvest.ListStaged(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harvest: list staged: %v\n", err)
		os.Exit(1)
	}
	if len(staged) == 0 {
		fmt.Println("No staged candidates. Run `kinclaw harvest` first.")
		return
	}

	fmt.Printf("Staged at %s\n\n", harvest.StagedRoot(home))
	for _, s := range staged {
		mark := "✓"
		switch s.CriticVote {
		case harvest.CriticAccept:
			mark = "✓"
		case harvest.CriticWarn:
			mark = "⚠"
		case harvest.CriticReject:
			mark = "✗"
		case "skipped", "":
			mark = "·"
		}
		fmt.Printf("%s  %s/%s\n", mark, s.SourceName, s.SkillName)
		fmt.Printf("    source : %s\n", s.SourceURL)
		fmt.Printf("    file   : %s\n", s.SkillRelPath)
		fmt.Printf("    critic : %s\n", criticLabel(s.CriticVote))
		fmt.Printf("    accept : kinclaw harvest --accept %s/%s\n\n", s.SourceName, s.SkillName)
	}
}

func criticLabel(d harvest.CriticDecision) string {
	switch d {
	case harvest.CriticAccept:
		return "accept"
	case harvest.CriticWarn:
		return "warn"
	case harvest.CriticReject:
		return "reject"
	case "":
		return "(unknown)"
	default:
		return string(d)
	}
}

func runHarvestAccept(home, skillsDir, skillID string) {
	dst, err := harvest.AcceptStaged(home, skillsDir, skillID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harvest: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ accepted %s → %s\n", skillID, dst)
	fmt.Printf("  Run kinclaw to pick it up — external skills are auto-discovered from %s.\n", skillsDir)
}

// resolveCriticSoul finds souls/critic.soul.md in the standard search
// path. Returns empty string if not found — the pipeline then runs
// without critic review.
func resolveCriticSoul() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		"souls/critic.soul.md",
		filepath.Join(home, ".localkin", "souls", "critic.soul.md"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func printSummary(results []harvest.Result, dryRun bool) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "── summary")
	totalCandidates, totalPassed, totalRejected := 0, 0, 0
	for _, r := range results {
		totalCandidates += r.Candidates
		totalPassed += len(r.Passed)
		totalRejected += len(r.Rejected)
		fmt.Fprintf(os.Stderr, "  %-20s %d candidates, %d passed, %d rejected, %d errors\n",
			r.SourceName, r.Candidates, len(r.Passed), len(r.Rejected), len(r.Errors))
	}
	fmt.Fprintf(os.Stderr, "  %-20s %d candidates, %d passed, %d rejected\n",
		"total", totalCandidates, totalPassed, totalRejected)

	if dryRun {
		fmt.Fprintln(os.Stderr, "\n  --diff: nothing was written.")
		return
	}
	if totalPassed > 0 {
		fmt.Fprintln(os.Stderr, "\n  Review with:    kinclaw harvest --review")
		fmt.Fprintln(os.Stderr, "  Accept one:     kinclaw harvest --accept <source>/<skill-name>")
	}
}
