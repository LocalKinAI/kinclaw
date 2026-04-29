package harvest

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Inspire spawns the coder specialist to forge a KinClaw exec-style
// SKILL.md from an external procedural-style original. v1.6+ this
// only runs at `--accept` time — one specific candidate at a time,
// chosen by the user after `--review`. Scan-time judgment uses the
// curator soul via Judge() instead.
//
// Returns one of three structured outcomes:
//
//   InspireForged       — coder produced a SKILL.md between the
//                         BEGIN/END markers. Caller round-trip
//                         validates and stages.
//   InspireDeferred     — coder said the capability can't be expressed
//                         as a single shell exec. Caller falls back to
//                         ./skills/library/ (preserve as inspiration).
//   InspireUnparseable  — coder responded but neither verdict nor
//                         content could be extracted. Caller surfaces
//                         the FullText for human review.
//
// Mirrors the spawn skill's exec pattern (cmd.Output + KINCLAW_SPAWN_
// DEPTH=1 env guard so coder can't itself spawn).
func Inspire(ctx context.Context, kinclawBin, coderSoulPath, candidateContent, sourceURL, skillRel string) (*InspireResult, error) {
	prompt := buildInspirePrompt(candidateContent, sourceURL, skillRel)

	ctx, cancel := withTimeout(ctx, 240*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, kinclawBin,
		"-soul", coderSoulPath,
		"-exec", prompt)
	cmd.Env = append(cmd.Environ(), "KINCLAW_SPAWN_DEPTH=1")

	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("coder timed out after 240s")
	}
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("coder exec failed: %w\n--- stderr ---\n%s", err, truncate(stderr, 500))
	}

	body := strings.TrimSpace(string(out))
	return parseInspireResponse(body), nil
}

// InspireResult is the structured outcome of one coder forge attempt.
type InspireResult struct {
	FullText      string
	Verdict       InspireVerdict
	ForgedContent string
	DeferReason   string
}

type InspireVerdict string

const (
	InspireForged      InspireVerdict = "forged"
	InspireDeferred    InspireVerdict = "defer_to_procedural"
	InspireUnparseable InspireVerdict = "unparseable"
)

var (
	inspireVerdictRe    = regexp.MustCompile(`(?im)^verdict\s*[:：]\s*(\S+)`)
	inspireSkillBlockRe = regexp.MustCompile(`(?ms)^---KINCLAW_SKILL_BEGIN---\s*\n(.*?)\n---KINCLAW_SKILL_END---`)
	inspireReasonRe     = regexp.MustCompile(`(?im)^(?:reason|why_not_exec)\s*[:：]\s*(.+)$`)
)

func parseInspireResponse(body string) *InspireResult {
	r := &InspireResult{FullText: body}

	verdictMatch := inspireVerdictRe.FindStringSubmatch(body)
	if verdictMatch == nil {
		r.Verdict = InspireUnparseable
		return r
	}

	v := strings.ToLower(strings.TrimSpace(verdictMatch[1]))
	switch v {
	case "forged", "forge", "ok", "accept", "pass":
		if blk := inspireSkillBlockRe.FindStringSubmatch(body); blk != nil {
			r.Verdict = InspireForged
			r.ForgedContent = strings.TrimSpace(blk[1])
			return r
		}
		// "forged" without the block → unparseable so caller surfaces it.
		r.Verdict = InspireUnparseable
		return r
	case "defer_to_procedural", "defer", "deferred", "skip", "reject":
		r.Verdict = InspireDeferred
		if rm := inspireReasonRe.FindStringSubmatch(body); rm != nil {
			r.DeferReason = strings.TrimSpace(rm[1])
		}
		return r
	default:
		r.Verdict = InspireUnparseable
		return r
	}
}

func buildInspirePrompt(candidateContent, sourceURL, skillRel string) string {
	var b strings.Builder
	b.WriteString("You are forging a KinClaw exec-style SKILL.md inspired by an external procedural-style skill.\n\n")
	b.WriteString("Read the original skill below. Decide per your soul's rules whether the capability\n")
	b.WriteString("can be expressed as a single shell exec (one `command` + `args`), or whether it\n")
	b.WriteString("genuinely needs LLM round-trips / AX-driven UI / pure prompt-engineering — in which\n")
	b.WriteString("case `verdict: defer_to_procedural`.\n\n")
	b.WriteString("Source: ")
	b.WriteString(sourceURL)
	b.WriteString("\nFile in repo: ")
	b.WriteString(skillRel)
	b.WriteString("\n\nOriginal SKILL.md:\n```\n")
	b.WriteString(candidateContent)
	b.WriteString("\n```\n\n")
	b.WriteString("Output format — choose ONE shape, no other prose:\n\n")
	b.WriteString("Shape A — successful forge:\n")
	b.WriteString("```\n")
	b.WriteString("verdict: forged\n")
	b.WriteString("capability: <one-line summary of what this skill does>\n")
	b.WriteString("inputs: <comma-separated schema parameter names>\n")
	b.WriteString("caveats: <dependencies / limits / parts you punt>\n")
	b.WriteString("---KINCLAW_SKILL_BEGIN---\n")
	b.WriteString("---\n")
	b.WriteString("name: <snake_case_identifier>\n")
	b.WriteString("description: <one sentence>\n")
	b.WriteString("command: [<real_binary_in_PATH>, ...]\n")
	b.WriteString("args: [...]\n")
	b.WriteString("schema:\n")
	b.WriteString("  param_name:\n")
	b.WriteString("    type: string\n")
	b.WriteString("    description: ...\n")
	b.WriteString("---\n")
	b.WriteString("\n# <name>\n")
	b.WriteString("\n<optional human-readable comment>\n")
	b.WriteString("---KINCLAW_SKILL_END---\n")
	b.WriteString("```\n\n")
	b.WriteString("Shape B — defer (cannot be exec'd):\n")
	b.WriteString("```\n")
	b.WriteString("verdict: defer_to_procedural\n")
	b.WriteString("original_concept: <one-line summary>\n")
	b.WriteString("reason: <needs LLM round-trips / needs AX or vision / pure prompt template / other>\n")
	b.WriteString("```\n\n")
	b.WriteString("Critical rules (your soul has these — repeated for emphasis):\n\n")
	b.WriteString("  RULE 1 — `name` matches /^[a-zA-Z][a-zA-Z0-9_]{0,63}$/. NO hyphens, NO spaces, NO dots.\n")
	b.WriteString("    Anthropic / Hermes use hyphens (apple-reminders, design-md); convert to underscores.\n")
	b.WriteString("    WRONG:  name: apple-notes-search\n")
	b.WriteString("    RIGHT:  name: apple_notes_search\n\n")
	b.WriteString("  RULE 2 — `command` is a YAML list of strings, not a bare string.\n")
	b.WriteString("    Go reads command[0] as the binary; rest as argv. Bare-string yaml fails to parse.\n")
	b.WriteString("    WRONG:  command: opencode\n")
	b.WriteString("    WRONG:  command: python3 script.py\n")
	b.WriteString("    RIGHT:  command: [opencode]\n")
	b.WriteString("    RIGHT:  command: [python3, script.py]\n\n")
	b.WriteString("  RULE 3 — command[0] is a real binary in $PATH (osascript / curl / sh / python3 / installed CLI).\n")
	b.WriteString("    NEVER a KinClaw internal skill name (ui / screen / input / record / web).\n\n")
	b.WriteString("  RULE 4 — args is a JSON-style YAML array. Every {{var}} placeholder must be declared in schema.\n\n")
	b.WriteString("  RULE 5 — NO hardcoded screen coordinates (`click at {N,M}`) — forge gate v2 will reject.\n\n")
	b.WriteString("  RULE 6 — If the original references a binary that may not be installed by default (e.g. remindctl), say so in `caveats:`.\n\n")
	b.WriteString("  RULE 7 — Be honest. Partial implementation OK; document what was punted in `caveats:`. Don't fake.\n")
	return b.String()
}
