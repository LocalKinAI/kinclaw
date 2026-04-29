package harvest

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// CriticReview spawns the critic specialist soul on a candidate SKILL.md
// and returns the structured verdict. Mirrors the spawn skill's exec
// pattern (cmd.Output + KINCLAW_SPAWN_DEPTH=1 env guard) so the critic
// can't itself spawn a sub-agent.
//
// The critic soul produces a 3-section response (✓ what passes / ⚠ risks
// ranked / overall verdict). We require it to end with a parseable
// `verdict: <accept|warn|reject>` line so the pipeline can sort
// staged candidates by confidence in `kinclaw harvest --review`.
//
// The critic ANNOTATES, does not auto-reject. Even a "reject" verdict
// stages the candidate (with the annotation file), because the critic is
// a soft signal — Jacky reviews everything anyway. Hard rejection comes
// from the forge gate (next stage).
func CriticReview(ctx context.Context, kinclawBin, criticSoulPath, skillContent, sourceURL, skillRel string) (*CriticVerdict, error) {
	prompt := buildCriticPrompt(skillContent, sourceURL, skillRel)

	ctx, cancel := withTimeout(ctx, 180*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, kinclawBin,
		"-soul", criticSoulPath,
		"-exec", prompt)
	// Mark depth=1 so critic's own spawn skill (if it ever gets one) refuses.
	cmd.Env = append(cmd.Environ(), "KINCLAW_SPAWN_DEPTH=1")

	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("critic timed out after 180s")
	}
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("critic exec failed: %w\n--- stderr ---\n%s", err, truncate(stderr, 500))
	}

	body := strings.TrimSpace(string(out))
	verdict := parseCriticVerdict(body)
	return &CriticVerdict{
		FullText: body,
		Decision: verdict,
	}, nil
}

// CriticVerdict is the structured outcome of a critic spawn.
type CriticVerdict struct {
	FullText string         // raw 3-section critic body
	Decision CriticDecision // parsed verdict line, defaults to Warn if missing
}

type CriticDecision string

const (
	CriticAccept CriticDecision = "accept"
	CriticWarn   CriticDecision = "warn"
	CriticReject CriticDecision = "reject"
)

// verdictPattern accepts both English and 中文 because the critic soul
// runs on Minimax and may output either depending on the input language.
// All three forms collapse to one of {accept, warn, reject}.
var verdictPattern = regexp.MustCompile(`(?im)^verdict\s*[:：]\s*(\S+)`)

func parseCriticVerdict(body string) CriticDecision {
	m := verdictPattern.FindStringSubmatch(body)
	if m == nil {
		return CriticWarn // missing verdict line → don't auto-pass, don't auto-fail
	}
	v := strings.ToLower(strings.TrimSpace(m[1]))
	switch v {
	case "accept", "pass", "ok", "通过":
		return CriticAccept
	case "reject", "fail", "no", "不通过":
		return CriticReject
	default:
		return CriticWarn
	}
}

// CriticReviewInspired is the inspire-aware variant: instead of just
// reviewing a single SKILL.md for quality, the critic sees BOTH the
// original procedural-style content (the inspiration) and the
// KinClaw exec-style version the coder forged. This lets it judge
// **concept alignment** ("does the forged skill cover the same
// capability?") in addition to **implementation quality**.
//
// Returns the same CriticVerdict shape as CriticReview — caller
// handles forged-but-rejected vs forged-and-passing the same way
// (annotation only, doesn't auto-reject from staging).
func CriticReviewInspired(ctx context.Context, kinclawBin, criticSoulPath, originalContent, forgedContent, sourceURL, skillRel string) (*CriticVerdict, error) {
	prompt := buildInspiredCriticPrompt(originalContent, forgedContent, sourceURL, skillRel)

	ctx, cancel := withTimeout(ctx, 180*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, kinclawBin,
		"-soul", criticSoulPath,
		"-exec", prompt)
	cmd.Env = append(cmd.Environ(), "KINCLAW_SPAWN_DEPTH=1")

	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("critic (inspired) timed out after 180s")
	}
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("critic (inspired) exec failed: %w\n--- stderr ---\n%s", err, truncate(stderr, 500))
	}

	body := strings.TrimSpace(string(out))
	verdict := parseCriticVerdict(body)
	return &CriticVerdict{
		FullText: body,
		Decision: verdict,
	}, nil
}

func buildInspiredCriticPrompt(originalContent, forgedContent, sourceURL, skillRel string) string {
	var b strings.Builder
	b.WriteString("You are reviewing an inspire-forged candidate: the `coder` specialist read an external\n")
	b.WriteString("procedural-style SKILL.md (which describes a behavior in markdown rather than as a shell\n")
	b.WriteString("command) and produced a KinClaw exec-style equivalent. Your job is to judge BOTH\n")
	b.WriteString("implementation quality AND concept alignment — does the forged version actually cover\n")
	b.WriteString("the same capability the original described?\n\n")
	b.WriteString("Source: ")
	b.WriteString(sourceURL)
	b.WriteString("\nFile in repo: ")
	b.WriteString(skillRel)
	b.WriteString("\n\nORIGINAL (procedural style — the inspiration):\n```\n")
	b.WriteString(originalContent)
	b.WriteString("\n```\n\nFORGED (KinClaw exec style — what coder produced):\n```\n")
	b.WriteString(forgedContent)
	b.WriteString("\n```\n\n")
	b.WriteString("Apply your standard 3-section structure:\n")
	b.WriteString("✓ what passes — concrete strengths (implementation + alignment)\n")
	b.WriteString("⚠ risks ranked — concrete failure modes (broken exec / missing capability / ...)\n")
	b.WriteString("verdict: <accept|warn|reject>\n\n")
	b.WriteString("Specifically check:\n")
	b.WriteString("  - The forged `command[0]` is a real binary likely available on macOS (or noted as a dependency).\n")
	b.WriteString("  - The forged schema parameters cover the inputs the original implied (or coder honestly punted in caveats).\n")
	b.WriteString("  - The forged version doesn't pretend to do something the original requires LLM round-trips for.\n")
	b.WriteString("  - The forged version isn't trivially broken (osascript -e pairing, hardcoded coords, schema/template mismatch).\n\n")
	b.WriteString("End with a single `verdict:` line.\n")
	return b.String()
}

func buildCriticPrompt(skillContent, sourceURL, skillRel string) string {
	var b strings.Builder
	b.WriteString("You are reviewing a candidate SKILL.md harvested from a third-party agent repo for inclusion in KinClaw's skill library.\n\n")
	b.WriteString("Source: ")
	b.WriteString(sourceURL)
	b.WriteString("\nFile in repo: ")
	b.WriteString(skillRel)
	b.WriteString("\n\nSKILL.md content:\n```\n")
	b.WriteString(skillContent)
	b.WriteString("\n```\n\n")
	b.WriteString("Apply your standard 3-section structure:\n")
	b.WriteString("✓ what passes — concrete strengths\n")
	b.WriteString("⚠ risks ranked — concrete failure modes, ordered by severity\n")
	b.WriteString("verdict: <accept|warn|reject>\n\n")
	b.WriteString("End with a single `verdict:` line so an automated pipeline can route the candidate.\n")
	b.WriteString("- accept = ship it, low risk\n")
	b.WriteString("- warn   = ship behind a manual review (default for ambiguity)\n")
	b.WriteString("- reject = don't ship — explain why above\n")
	return b.String()
}
