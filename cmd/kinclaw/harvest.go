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

Read external agent skill libraries (Claude Code / Hermes / Cursor /
LangChain / your own), let the coder specialist forge KinClaw versions
of the good ideas, stage candidates for your review.

Three commands:

  kinclaw harvest                  scan + forge → stage candidates
  kinclaw harvest --review         list what's staged
  kinclaw harvest --accept ID      copy one staged candidate into ./skills/

Manifest: %s

Flags:
`, harvest.DefaultManifestPath())
		fs.PrintDefaults()
	}

	manifestPath := fs.String("manifest", harvest.DefaultManifestPath(), "Path to harvest TOML manifest")
	sourceName := fs.String("source", "", "Run for one source only (manifest [[source]] name)")
	diff := fs.Bool("diff", false, "Dry-run — scan + report, write nothing to staging")
	review := fs.Bool("review", false, "List staged candidates and exit")
	accept := fs.String("accept", "", "Copy a staged candidate (id: <source>/<skill-name>) into ./skills/")
	noCritic := fs.Bool("no-critic", false, "Skip the critic spawn (cron / CI / offline)")
	noInspire := fs.Bool("no-inspire", false, "Skip the coder forge step. Just count procedural candidates without spawning LLM (cron / dry-look)")
	skillsDir := fs.String("skills-dir", "skills", "--accept destination (default ./skills)")

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
	// Inspire is default-on. --no-inspire opts out (cron / CI / cost-
	// sensitive runs). Resolve coder soul unless explicitly disabled —
	// missing soul file is a soft fall-back to "no inspire", not a hard
	// error, since the v1.5+ default flow shouldn't punish users whose
	// souls/ directory is incomplete.
	inspireOn := !*noInspire
	coderSoul := ""
	if inspireOn {
		coderSoul = resolveCoderSoul()
		if coderSoul == "" {
			fmt.Fprintln(os.Stderr, "harvest: souls/coder.soul.md not found in ./souls/ or ~/.localkin/souls/.")
			fmt.Fprintln(os.Stderr, "         procedural-style candidates will be counted but not forged.")
			fmt.Fprintln(os.Stderr, "         Add coder.soul.md or pass --no-inspire to silence this notice.")
			inspireOn = false
		}
	}

	opts := harvest.Options{
		Home:           home,
		KinclawBin:     bin,
		CriticSoulPath: criticSoul,
		CoderSoulPath:  coderSoul,
		SkipCritic:     *noCritic,
		Inspire:        inspireOn,
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
		printSummary([]harvest.Result{r}, *diff, inspireOn)
		return
	}

	results := make([]harvest.Result, 0, len(manifest.Sources))
	for _, s := range manifest.Sources {
		results = append(results, harvest.RunSource(ctx, s, opts))
	}
	printSummary(results, *diff, inspireOn)
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
		var kindMark, kindLabel string
		switch s.Kind {
		case harvest.StagedKindInspireForged:
			kindMark, kindLabel = "✨", "inspire-forged"
		case harvest.StagedKindProcedural:
			kindMark, kindLabel = "📜", "procedural (deferred)"
		default:
			kindMark, kindLabel = "·", "regular"
		}
		// Critic mark overrides kind when the critic was opinionated.
		mark := kindMark
		switch s.CriticVote {
		case harvest.CriticAccept:
			mark = "✓"
		case harvest.CriticWarn:
			mark = "⚠"
		case harvest.CriticReject:
			mark = "✗"
		}
		fmt.Printf("%s  %s/%s  [%s]\n", mark, s.SourceName, s.SkillName, kindLabel)
		fmt.Printf("    source : %s\n", s.SourceURL)
		fmt.Printf("    file   : %s\n", s.SkillRelPath)
		if s.Kind == harvest.StagedKindProcedural {
			fmt.Printf("    defer  : %s\n", s.DeferReason)
			fmt.Printf("    review : less %s/original.md\n\n", s.StagePath)
			continue
		}
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
	return resolveSoulFile("critic.soul.md")
}

// resolveCoderSoul finds souls/coder.soul.md — used by --inspire to
// re-implement procedural-style external SKILL.md candidates as
// KinClaw exec form. Same search path as resolveCriticSoul.
func resolveCoderSoul() string {
	return resolveSoulFile("coder.soul.md")
}

func resolveSoulFile(name string) string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join("souls", name),
		filepath.Join(home, ".localkin", "souls", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func printSummary(results []harvest.Result, dryRun, inspireOn bool) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "── summary")

	var totalCand, totalStaged, totalInspired, totalProcDef, totalProcPend, totalBroken, totalErr int
	for _, r := range results {
		totalCand += r.Candidates
		totalStaged += len(r.Passed)
		totalInspired += len(r.Inspired)
		totalProcDef += len(r.Procedural)
		totalProcPend += len(r.ProceduralPending)
		totalBroken += len(r.Rejected)
		totalErr += len(r.Errors)
	}
	fmt.Fprintf(os.Stderr, "  %d candidates across %d source(s)\n", totalCand, len(results))
	if totalStaged > 0 {
		if totalInspired > 0 {
			fmt.Fprintf(os.Stderr, "  ✓  %d staged (%d ✨ inspire-forged)\n", totalStaged, totalInspired)
		} else {
			fmt.Fprintf(os.Stderr, "  ✓  %d staged\n", totalStaged)
		}
	}
	if totalProcDef > 0 {
		fmt.Fprintf(os.Stderr, "  📜 %d procedural deferred (browse-only, see staged/<src>/_procedural/)\n", totalProcDef)
	}
	if totalProcPend > 0 {
		hint := "rerun with --inspire to forge them via the coder specialist"
		if inspireOn {
			hint = "still pending after --inspire — see logs above"
		}
		fmt.Fprintf(os.Stderr, "  ⏸ %d procedural pending — %s\n", totalProcPend, hint)
	}
	if totalBroken > 0 {
		fmt.Fprintf(os.Stderr, "  ✗  %d broken (forge-gate fail / unparseable / etc — see lines above)\n", totalBroken)
	}
	if totalErr > 0 {
		fmt.Fprintf(os.Stderr, "  ⚠  %d source-level error(s) — license / clone / glob mismatches\n", totalErr)
	}

	if dryRun {
		fmt.Fprintln(os.Stderr, "\n  --diff: nothing was written.")
		return
	}
	fmt.Fprintln(os.Stderr)
	if totalStaged > 0 {
		fmt.Fprintln(os.Stderr, "  Review:        kinclaw harvest --review")
		fmt.Fprintln(os.Stderr, "  Accept one:    kinclaw harvest --accept <source>/<skill-name>")
	}
	if totalProcPend > 0 && !inspireOn {
		fmt.Fprintln(os.Stderr, "  Forge them:    kinclaw harvest --inspire   (burns LLM tokens; see CHANGELOG)")
	}
}
