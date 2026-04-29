package harvest

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LocalKinAI/kinclaw/pkg/skill"
)

// Options configures one harvest pipeline run.
type Options struct {
	Home           string        // user home dir; staging + cache live under here
	KinclawBin     string        // path to the running kinclaw binary (for critic / coder spawn)
	CriticSoulPath string        // resolved souls/critic.soul.md path; empty disables critic
	CoderSoulPath  string        // resolved souls/coder.soul.md path; empty disables --inspire
	SkipCritic     bool          // explicit override even if CriticSoulPath is set (cron / CI)
	Inspire        bool          // route procedural-style SKILL.md through coder-forge re-implementation
	DryRun         bool          // true == --diff, don't write to staging
	Out            io.Writer     // human-readable progress, default os.Stderr
	CloneTimeout   time.Duration // per-source git clone/pull deadline (default 120s)
}

// Result is the per-source outcome of a pipeline run.
type Result struct {
	SourceName string
	Candidates int      // total files matched by skill_paths
	Passed     []string // staged skill names (exec-style + inspire-forged)
	Inspired   []string // staged via --inspire (subset of Passed; tracked separately for the summary)
	Procedural []string // staged into _procedural/ as defer_to_procedural (no exec impl)
	Rejected   []Reject // rejected by forge gate / parse error / license
	Errors     []string // non-fatal errors that didn't fit a single candidate
}

// Reject describes one candidate that didn't make it to staging.
type Reject struct {
	Path   string // path inside the source repo
	Reason string
}

// RunSource runs the pipeline against a single source. Returns a Result
// even on partial failure — the caller can render a per-source summary
// regardless of how many candidates passed.
func RunSource(ctx context.Context, src Source, opts Options) Result {
	r := Result{SourceName: src.Name}
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	if opts.CloneTimeout == 0 {
		opts.CloneTimeout = 120 * time.Second
	}

	fmt.Fprintf(out, "── %s\n", src.Name)
	cloneCtx, cancel := withTimeout(ctx, opts.CloneTimeout)
	defer cancel()
	repoDir, err := PullSource(cloneCtx, src, opts.Home)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("clone/pull: %v", err))
		fmt.Fprintf(out, "   ✗ clone/pull failed: %v\n", err)
		return r
	}

	license := FindLicense(repoDir)
	if !LicenseAllowed(license, src.LicenseAllow) {
		detected := license
		if detected == "" {
			detected = "(none detected)"
		}
		msg := fmt.Sprintf("license %s not in allowlist %v", detected, src.LicenseAllow)
		r.Errors = append(r.Errors, msg)
		fmt.Fprintf(out, "   ✗ %s\n", msg)
		return r
	}
	if license != "" {
		fmt.Fprintf(out, "   license: %s ✓\n", license)
	} else {
		fmt.Fprintf(out, "   license: (none detected, allowed by *)\n")
	}

	matches, err := globFiles(repoDir, src.SkillPaths)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("glob: %v", err))
		return r
	}
	r.Candidates = len(matches)
	fmt.Fprintf(out, "   matched %d candidate(s)\n", len(matches))

	if !opts.DryRun {
		if err := CleanSourceStage(opts.Home, src.Name); err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("clean staged: %v", err))
		}
	}

	for _, path := range matches {
		rel, _ := filepath.Rel(repoDir, path)
		rel = filepath.ToSlash(rel)
		if err := processCandidate(ctx, &r, src, opts, repoDir, path, rel, out); err != nil {
			r.Rejected = append(r.Rejected, Reject{Path: rel, Reason: err.Error()})
			fmt.Fprintf(out, "   ✗ %s — %s\n", rel, err)
		}
	}
	fmt.Fprintf(out, "   %d passed, %d rejected\n", len(r.Passed), len(r.Rejected))
	return r
}

// processCandidate is the per-file pipeline body. Failure at any stage
// returns an error describing why; the caller logs + records as a
// rejection. Success appends the skill name to r.Passed (and r.Inspired
// for inspire-forged) or r.Procedural (for deferred), and (unless
// DryRun) writes the staging dir.
func processCandidate(
	ctx context.Context,
	r *Result,
	src Source,
	opts Options,
	repoDir, path, rel string,
	out io.Writer,
) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	skillContent, ext, err := translateToSkillMD(content, rel)
	if err != nil {
		return fmt.Errorf("translate (%s): %w", ext, err)
	}

	// First try the exec-style happy path: parse via LoadExternalSkill,
	// run forge gate, run critic, stage.
	loaded, parseErr := loadFromString(skillContent)
	if parseErr == nil {
		return processExecCandidate(ctx, r, src, opts, skillContent, rel, loaded, out)
	}

	// Parse failed. Two sub-cases:
	//   1. Procedural-style (Anthropic / Hermes / Cursor) + --inspire ON →
	//      route to coder for re-implementation.
	//   2. Anything else → reject with the parse error.
	if opts.Inspire && opts.CoderSoulPath != "" && opts.KinclawBin != "" && looksProcedural(content) {
		return processProceduralCandidate(ctx, r, src, opts, skillContent, rel, out)
	}
	return fmt.Errorf("parse: %w", parseErr)
}

// loadFromString writes skillContent to a temp SKILL.md and runs
// LoadExternalSkill on it (the same loader the registry uses at boot).
// Refactored out of processCandidate so the exec-style path and the
// inspire-forged validation step both reuse it.
func loadFromString(skillContent string) (*skill.ExternalSkill, error) {
	tmpDir, err := os.MkdirTemp("", "kinclaw-harvest-*")
	if err != nil {
		return nil, fmt.Errorf("tmpdir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpSkill := filepath.Join(tmpDir, "SKILL.md")
	if err := os.WriteFile(tmpSkill, []byte(skillContent), 0o644); err != nil {
		return nil, fmt.Errorf("tmp write: %w", err)
	}
	return skill.LoadExternalSkill(tmpSkill)
}

// processExecCandidate handles the v1.3.1 happy path: a candidate that
// already parses as a KinClaw exec-style SKILL.md. Forge gate v2 +
// critic + stage.
func processExecCandidate(
	ctx context.Context,
	r *Result,
	src Source,
	opts Options,
	skillContent, rel string,
	loaded *skill.ExternalSkill,
	out io.Writer,
) error {
	skillName := loaded.Name()

	if err := skill.ValidateSkillMeta(loaded.Meta()); err != nil {
		return fmt.Errorf("forge gate: %w", err)
	}

	var critic *CriticVerdict
	if !opts.SkipCritic && opts.CriticSoulPath != "" && opts.KinclawBin != "" {
		v, cerr := CriticReview(ctx, opts.KinclawBin, opts.CriticSoulPath, skillContent, src.URL, rel)
		if cerr != nil {
			fmt.Fprintf(out, "   ⚠ %s — critic skipped (%s)\n", rel, cerr)
		} else {
			critic = v
		}
	}

	mark := "✓"
	if critic != nil {
		mark = fmt.Sprintf("✓ [%s]", critic.Decision)
	}

	if opts.DryRun {
		fmt.Fprintf(out, "   %s %s → %s (would stage)\n", mark, rel, skillName)
		r.Passed = append(r.Passed, skillName)
		return nil
	}

	if _, err := StageCandidate(opts.Home, src.Name, skillName, skillContent, critic, src, rel); err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	fmt.Fprintf(out, "   %s %s → %s\n", mark, rel, skillName)
	r.Passed = append(r.Passed, skillName)
	return nil
}

// processProceduralCandidate is the --inspire branch. The candidate is
// a procedural-style SKILL.md (frontmatter has name + description but
// no command, body is markdown procedural instruction). We spawn the
// coder specialist and ask it to either:
//
//  1. forge a KinClaw exec-style equivalent (run that through forge
//     gate + critic + stage as a regular passing candidate, marked
//     "from inspire" in meta), or
//  2. defer_to_procedural — the original capability genuinely needs
//     LLM round-trips / AX / vision, can't be expressed as a single
//     shell exec. We stage it to staged/<source>/_procedural/<name>/
//     so the human can still see what was found, but it can't be
//     promoted to ./skills/ (no exec).
func processProceduralCandidate(
	ctx context.Context,
	r *Result,
	src Source,
	opts Options,
	candidateContent, rel string,
	out io.Writer,
) error {
	procName := procedureNameFromContent(candidateContent, rel)
	fmt.Fprintf(out, "   ✨ %s → coder forging…\n", rel)

	res, ierr := Inspire(ctx, opts.KinclawBin, opts.CoderSoulPath, candidateContent, src.URL, rel)
	if ierr != nil {
		return fmt.Errorf("coder: %w", ierr)
	}

	switch res.Verdict {
	case InspireForged:
		// Validate the forged content the same way the registry would
		// at boot. If it doesn't parse, treat as a failed forge —
		// stage it under _failed_forge/ so a human can fix it.
		loaded, perr := loadFromString(res.ForgedContent)
		if perr != nil {
			return fmt.Errorf("inspire-forged but unparseable: %w", perr)
		}
		if verr := skill.ValidateSkillMeta(loaded.Meta()); verr != nil {
			return fmt.Errorf("inspire-forged failed forge gate: %w", verr)
		}
		skillName := loaded.Name()

		// Critic on the inspire-forged version. Pass the original
		// procedural content so critic can also judge concept
		// alignment, not just implementation quality.
		var critic *CriticVerdict
		if !opts.SkipCritic && opts.CriticSoulPath != "" && opts.KinclawBin != "" {
			v, cerr := CriticReviewInspired(ctx, opts.KinclawBin, opts.CriticSoulPath,
				candidateContent, res.ForgedContent, src.URL, rel)
			if cerr != nil {
				fmt.Fprintf(out, "   ⚠ %s — critic skipped (%s)\n", rel, cerr)
			} else {
				critic = v
			}
		}

		mark := "✨"
		if critic != nil {
			mark = fmt.Sprintf("✨ [%s]", critic.Decision)
		}
		if opts.DryRun {
			fmt.Fprintf(out, "   %s %s → %s (would stage, inspire-forged)\n", mark, rel, skillName)
			r.Passed = append(r.Passed, skillName)
			r.Inspired = append(r.Inspired, skillName)
			return nil
		}
		if _, err := StageInspiredCandidate(opts.Home, src.Name, skillName,
			res.ForgedContent, candidateContent, res.FullText, critic, src, rel); err != nil {
			return fmt.Errorf("stage inspire: %w", err)
		}
		fmt.Fprintf(out, "   %s %s → %s (inspire-forged)\n", mark, rel, skillName)
		r.Passed = append(r.Passed, skillName)
		r.Inspired = append(r.Inspired, skillName)
		return nil

	case InspireDeferred, InspireUnparseable:
		// Stage to _procedural/ so the human can review. Can't be
		// accepted (no exec form), but the original is preserved.
		reason := res.DeferReason
		if reason == "" {
			reason = "(no reason given)"
		}
		if res.Verdict == InspireUnparseable {
			reason = "coder response unparseable; raw output kept"
		}
		if opts.DryRun {
			fmt.Fprintf(out, "   📜 %s → %s (would stage as procedural — %s)\n", rel, procName, reason)
			r.Procedural = append(r.Procedural, procName)
			return nil
		}
		if _, err := StageProcedural(opts.Home, src.Name, procName,
			candidateContent, reason, res.FullText, src, rel); err != nil {
			return fmt.Errorf("stage procedural: %w", err)
		}
		fmt.Fprintf(out, "   📜 %s → %s (procedural — %s)\n", rel, procName, reason)
		r.Procedural = append(r.Procedural, procName)
		return nil
	}
	// Unreachable — InspireResult.Verdict is always one of the three.
	return fmt.Errorf("inspire: unknown verdict %q", res.Verdict)
}

// procedureNameFromContent picks a stable identifier for staging a
// deferred procedural candidate, even though it never had a parsed
// SkillMeta.Name. Tries the YAML frontmatter `name` first; falls back
// to a slug derived from the file's parent dir or its base name.
func procedureNameFromContent(content, rel string) string {
	// Try YAML frontmatter — same approach as looksProcedural.
	if rawYAML, _, err := splitFrontmatterStr(content); err == nil {
		if n := extractYAMLName(rawYAML); n != "" {
			return sanitizeProcName(n)
		}
	}
	// Fallback: parent dir name (Anthropic / Hermes nest skills as
	// `<topic>/<name>/SKILL.md`), then file base.
	dir := filepath.Dir(rel)
	if dir != "" && dir != "." {
		return sanitizeProcName(filepath.Base(dir))
	}
	base := filepath.Base(rel)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return sanitizeProcName(base)
}

// translateToSkillMD reads raw bytes from a candidate file and returns
// the canonical SKILL.md form plus a label for diagnostics.
//
// v1 supports identity translation only — input must already be a
// SKILL.md (YAML frontmatter + optional script body). Cross-format
// translation (Hermes / OpenAI tool schema / Cursor rules / Rust doc
// comments) is the v1.3.2+ scope deliberately punted from this MVP.
func translateToSkillMD(content []byte, srcPath string) (string, string, error) {
	// The strictly-required SKILL.md shape is "starts with `---\n` (YAML
	// frontmatter delimiter)". If a candidate lacks that, it's not a
	// SKILL.md and v1 doesn't know what to do with it.
	trimmed := strings.TrimLeft(string(content), " \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return "", filepath.Ext(srcPath), fmt.Errorf("not a SKILL.md (no YAML frontmatter); cross-format translation arrives in v1.3.2+")
	}
	return string(content), "skill.md", nil
}
