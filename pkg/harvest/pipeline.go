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
	KinclawBin     string        // path to the running kinclaw binary (for critic spawn)
	CriticSoulPath string        // resolved souls/critic.soul.md path; empty disables critic
	SkipCritic     bool          // explicit override even if CriticSoulPath is set (cron / CI)
	DryRun         bool          // true == --diff, don't write to staging
	Out            io.Writer     // human-readable progress, default os.Stderr
	CloneTimeout   time.Duration // per-source git clone/pull deadline (default 120s)
}

// Result is the per-source outcome of a pipeline run.
type Result struct {
	SourceName string
	Candidates int      // total files matched by skill_paths
	Passed     []string // staged skill names
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
// rejection. Success appends the skill name to r.Passed and (unless
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

	// Round-trip parse via the same loader the registry uses at boot.
	// Writes to a temp file because LoadExternalSkill takes a path.
	tmpDir, err := os.MkdirTemp("", "kinclaw-harvest-*")
	if err != nil {
		return fmt.Errorf("tmpdir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpSkill := filepath.Join(tmpDir, "SKILL.md")
	if err := os.WriteFile(tmpSkill, []byte(skillContent), 0o644); err != nil {
		return fmt.Errorf("tmp write: %w", err)
	}

	loaded, err := skill.LoadExternalSkill(tmpSkill)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	skillName := loaded.Name()

	// Forge gate v2 (the parts applicable to harvested skills — name
	// pattern, command in $PATH, osascript -e pairing, no hardcoded
	// coords, schema/template var consistency).
	if err := skill.ValidateSkillMeta(loaded.Meta()); err != nil {
		return fmt.Errorf("forge gate: %w", err)
	}

	// Critic review (annotates, does not auto-reject).
	var critic *CriticVerdict
	if !opts.SkipCritic && opts.CriticSoulPath != "" && opts.KinclawBin != "" {
		v, cerr := CriticReview(ctx, opts.KinclawBin, opts.CriticSoulPath, skillContent, src.URL, rel)
		if cerr != nil {
			fmt.Fprintf(out, "   ⚠ %s — critic skipped (%s)\n", rel, cerr)
		} else {
			critic = v
		}
	}

	if opts.DryRun {
		mark := "✓"
		if critic != nil {
			mark = fmt.Sprintf("✓ [%s]", critic.Decision)
		}
		fmt.Fprintf(out, "   %s %s → %s (would stage)\n", mark, rel, skillName)
		r.Passed = append(r.Passed, skillName)
		return nil
	}

	stageDir, err := StageCandidate(opts.Home, src.Name, skillName, skillContent, critic, src, rel)
	if err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	mark := "✓"
	if critic != nil {
		mark = fmt.Sprintf("✓ [%s]", critic.Decision)
	}
	fmt.Fprintf(out, "   %s %s → %s\n", mark, rel, skillName)
	_ = stageDir
	r.Passed = append(r.Passed, skillName)
	return nil
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
