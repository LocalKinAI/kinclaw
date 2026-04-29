// Package harvest implements `kinclaw harvest` — pull external agent
// skill libraries, ask the curator specialist to judge each candidate
// against KinClaw's actual skill inventory, stage yes/maybe survivors
// for human review.
//
// v1.6 reframed harvest from "forge runnable skills at scan time" to
// "scan + lightweight LLM triage". The heavy work (forge → real exec
// SKILL.md) only happens at `--accept` time, on one specific
// candidate the user picked, never in bulk. See AcceptStaged.
//
// Pipeline (per candidate, fast):
//
//	read content + frontmatter
//	  → spawn curator with (KinClaw skill inventory + candidate excerpt)
//	  → curator returns verdict: yes | maybe | no + reason + domain
//	  → if yes/maybe, stage original markdown + judge.txt + meta.txt
//	  → if no, drop (counted in summary)
package harvest

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LocalKinAI/kinclaw/pkg/soul"
	"gopkg.in/yaml.v3"
)

// Options configures one harvest pipeline run.
type Options struct {
	Home             string         // user home; staging + cache live under here
	KinclawBin       string         // self-path for spawning curator (and coder at accept-time)
	CuratorSoulPath  string         // resolved souls/curator.soul.md; empty disables judging
	SkipJudge        bool           // --no-judge: just count candidates, no curator spawn (cron / cheap mode)
	Inventory        *SkillInventory // current ./skills/ state, injected into curator prompts
	DryRun           bool           // --diff: scan + judge but don't write to staging
	Out              io.Writer      // human-readable progress, default os.Stderr
	CloneTimeout     time.Duration  // per-source git clone/pull deadline (default 120s)
}

// Result is the per-source outcome of a pipeline run.
type Result struct {
	SourceName string
	Candidates int      // total files matched by skill_paths
	Yes        []string // staged with verdict=yes
	Maybe      []string // staged with verdict=maybe
	No         []string // dropped — curator said no
	Pending    []string // candidates we'd judge but couldn't (--no-judge or curator unavailable)
	Errors     []string // source-level errors (clone / license / glob)
}

// RunSource executes the pipeline against one [[source]] from the
// manifest. Always returns a Result, even on partial failure.
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
		fmt.Fprintf(out, "   ✗ license: %s — not in allowlist %v\n", detected, src.LicenseAllow)
		switch detected {
		case "proprietary":
			fmt.Fprintf(out, "      → proprietary (\"All rights reserved\"). Set license_allow=[\"*\"] to opt in.\n")
		case "(none detected)":
			fmt.Fprintf(out, "      → no LICENSE file found. Set license_allow=[\"*\"] if you've inspected the repo.\n")
		default:
			fmt.Fprintf(out, "      → add %q to license_allow for this source to include it.\n", detected)
		}
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
	if len(matches) == 0 {
		fmt.Fprintf(out, "   matched 0 candidates under skill_paths=%v\n", src.SkillPaths)
		fmt.Fprintf(out, "      → check the manifest's skill_paths globs against the repo's actual layout.\n")
		return r
	}
	fmt.Fprintf(out, "   matched %d candidate(s)\n", len(matches))

	if !opts.DryRun {
		if err := CleanSourceStage(opts.Home, src.Name); err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("clean staged: %v", err))
		}
	}

	for _, path := range matches {
		rel, _ := filepath.Rel(repoDir, path)
		rel = filepath.ToSlash(rel)
		processCandidate(ctx, &r, src, opts, repoDir, path, rel, out)
	}
	renderSourceSummary(out, &r, opts.SkipJudge)
	return r
}

// processCandidate is the v1.6 per-file body. Single path: extract
// name+description+body excerpt, ask curator (unless --no-judge),
// stage based on verdict. No forge, no critic, no special branching
// between exec-style and procedural-style — they all flow through
// the same triage.
func processCandidate(
	ctx context.Context,
	r *Result,
	src Source,
	opts Options,
	repoDir, path, rel string,
	out io.Writer,
) {
	content, err := os.ReadFile(path)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("%s: read: %v", rel, err))
		return
	}

	cand := extractCandidate(content, rel, src)

	// --no-judge mode: just record presence, don't spawn curator.
	// Used by cron / CI to keep source caches warm without LLM cost.
	// Per-candidate output is silent here; the source summary line
	// shows "N pending" and the README/help explains how to triage.
	if opts.SkipJudge || opts.CuratorSoulPath == "" || opts.KinclawBin == "" {
		r.Pending = append(r.Pending, cand.Name)
		return
	}

	res, err := Judge(ctx, opts.KinclawBin, opts.CuratorSoulPath, opts.Inventory, cand)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("%s: judge: %v", rel, err))
		fmt.Fprintf(out, "   ⚠ %s — judge failed: %v\n", rel, err)
		return
	}

	switch res.Verdict {
	case JudgeYes:
		r.Yes = append(r.Yes, cand.Name)
		fmt.Fprintf(out, "   ✓ %s — %s\n", cand.Name, res.Reason)
		if !opts.DryRun {
			if _, serr := StageJudged(opts.Home, src, cand, string(content), res); serr != nil {
				r.Errors = append(r.Errors, fmt.Sprintf("%s: stage: %v", rel, serr))
			}
		}
	case JudgeMaybe:
		r.Maybe = append(r.Maybe, cand.Name)
		fmt.Fprintf(out, "   ? %s — %s\n", cand.Name, res.Reason)
		if !opts.DryRun {
			if _, serr := StageJudged(opts.Home, src, cand, string(content), res); serr != nil {
				r.Errors = append(r.Errors, fmt.Sprintf("%s: stage: %v", rel, serr))
			}
		}
	case JudgeNo:
		r.No = append(r.No, cand.Name)
		fmt.Fprintf(out, "   ✗ %s — %s\n", cand.Name, res.Reason)
	case JudgeUnparseable:
		r.Errors = append(r.Errors, fmt.Sprintf("%s: curator response unparseable", rel))
		fmt.Fprintf(out, "   ⚠ %s — curator response unparseable; raw output kept\n", rel)
	}
}

// extractCandidate pulls the bits curator needs — name, description,
// body excerpt — from a candidate file. Tries YAML frontmatter first
// (Anthropic / Hermes / Cursor / KinClaw all use it); falls back to
// the file's parent dir name + first markdown paragraph for files
// without frontmatter (`.cursorrules`, plain `.md` rules collections).
func extractCandidate(content []byte, rel string, src Source) JudgeCandidate {
	c := JudgeCandidate{
		SourceURL:    src.URL,
		SourceName:   src.Name,
		SkillRelPath: rel,
	}

	rawYAML, body, err := soul.SplitFrontmatter(content)
	if err == nil && len(rawYAML) > 0 {
		var fm map[string]any
		if yaml.Unmarshal(rawYAML, &fm) == nil {
			if n, ok := fm["name"].(string); ok {
				c.Name = n
			}
			if d, ok := fm["description"].(string); ok {
				c.Description = d
			}
		}
		c.BodyExcerpt = strings.TrimSpace(string(body))
	}
	if c.BodyExcerpt == "" {
		c.BodyExcerpt = strings.TrimSpace(string(content))
	}

	if c.Name == "" {
		// Use parent dir name (Anthropic / Hermes nest as
		// <topic>/<name>/SKILL.md) with file base as fallback.
		dir := filepath.Dir(rel)
		if dir != "" && dir != "." {
			c.Name = filepath.Base(dir)
		} else {
			base := filepath.Base(rel)
			c.Name = strings.TrimSuffix(base, filepath.Ext(base))
		}
	}
	if c.Description == "" {
		// First paragraph of body, capped to 200 chars.
		body := c.BodyExcerpt
		if i := strings.Index(body, "\n\n"); i > 0 {
			body = body[:i]
		}
		body = strings.TrimSpace(body)
		if len(body) > 200 {
			body = body[:197] + "..."
		}
		c.Description = body
	}
	return c
}

func renderSourceSummary(out io.Writer, r *Result, skipJudge bool) {
	parts := []string{}
	if len(r.Yes) > 0 {
		parts = append(parts, fmt.Sprintf("%d ✓", len(r.Yes)))
	}
	if len(r.Maybe) > 0 {
		parts = append(parts, fmt.Sprintf("%d ?", len(r.Maybe)))
	}
	if len(r.No) > 0 {
		parts = append(parts, fmt.Sprintf("%d ✗", len(r.No)))
	}
	if len(r.Pending) > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", len(r.Pending)))
	}
	if len(parts) == 0 {
		parts = []string{"0 candidates"}
	}
	fmt.Fprintf(out, "   ── %s\n", strings.Join(parts, ", "))

	if skipJudge && len(r.Pending) > 0 {
		fmt.Fprintf(out, "      Run without --no-judge to triage these via curator.\n")
	}
}
