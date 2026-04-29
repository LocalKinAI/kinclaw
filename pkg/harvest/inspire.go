package harvest

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/LocalKinAI/kinclaw/pkg/soul"
	"gopkg.in/yaml.v3"
)

// Inspire spawns the coder specialist on a procedural-style SKILL.md
// (Anthropic / Hermes / Cursor rules — name + description + markdown
// body, no shell command) and asks it to **re-implement** that
// capability as a KinClaw exec-style SKILL.md via forge.
//
// This is the "harvest --inspire" path: not a translator (procedural
// instructions don't have a deterministic shell mapping), but a
// concept-borrow + native re-creation. Coder either produces a
// KinClaw-form SKILL.md or refuses with `verdict: defer_to_procedural`
// when the original capability can't be expressed as a single exec
// (needs LLM round-trips, AX/vision, or pure prompt template).
//
// Mirrors CriticReview's spawn pattern (cmd.Output + KINCLAW_SPAWN_DEPTH=1
// env guard so coder can't itself spawn).
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
	FullText      string          // raw coder output (kept for debugging / staging artifact)
	Verdict       InspireVerdict  // forged | deferred | unparseable
	ForgedContent string          // SKILL.md content if Verdict == InspireForged
	DeferReason   string          // human-readable explanation if Verdict == InspireDeferred
}

type InspireVerdict string

const (
	// InspireForged — coder produced a KinClaw exec-style SKILL.md
	// between the BEGIN/END markers. Caller still needs to round-trip
	// it through ValidateSkillMeta + critic before staging.
	InspireForged InspireVerdict = "forged"
	// InspireDeferred — coder refused: the original capability can't be
	// expressed as a single shell exec (needs LLM round-trips, AX/vision,
	// pure prompt template, ...). Stage to _procedural/ for human review.
	InspireDeferred InspireVerdict = "defer_to_procedural"
	// InspireUnparseable — coder responded but neither verdict nor
	// content could be extracted. Treated as deferred for staging
	// purposes; the FullText is preserved so a human can see what went
	// wrong.
	InspireUnparseable InspireVerdict = "unparseable"
)

// inspireVerdictRe matches a verdict line in the coder output. Same
// permissive shape as the critic's verdictPattern (case-insensitive,
// EN/中文 friendly).
var inspireVerdictRe = regexp.MustCompile(`(?im)^verdict\s*[:：]\s*(\S+)`)

// inspireSkillBlockRe extracts the forged SKILL.md content between
// the explicit markers the coder soul is told to emit. Greedy `.*?`
// in DOTALL mode so newlines inside the block are captured.
var inspireSkillBlockRe = regexp.MustCompile(`(?ms)^---KINCLAW_SKILL_BEGIN---\s*\n(.*?)\n---KINCLAW_SKILL_END---`)

// inspireReasonRe pulls a "reason:" annotation out of the coder
// output for deferred verdicts. Optional — if missing, DeferReason
// stays empty and we just keep the FullText for the staging artifact.
var inspireReasonRe = regexp.MustCompile(`(?im)^(?:reason|why_not_exec)\s*[:：]\s*(.+)$`)

func parseInspireResponse(body string) *InspireResult {
	r := &InspireResult{FullText: body}

	verdictMatch := inspireVerdictRe.FindStringSubmatch(body)
	if verdictMatch == nil {
		r.Verdict = InspireUnparseable
		return r
	}

	// Normalize verdict literal. Coder's soul tells it to use
	// "forged" or "defer_to_procedural"; accept variants defensively.
	v := strings.ToLower(strings.TrimSpace(verdictMatch[1]))
	switch v {
	case "forged", "forge", "ok", "accept", "pass":
		// Need the SKILL.md block to actually count as forged.
		if blk := inspireSkillBlockRe.FindStringSubmatch(body); blk != nil {
			r.Verdict = InspireForged
			r.ForgedContent = strings.TrimSpace(blk[1])
			return r
		}
		// Said forged but didn't include the block — treat as
		// unparseable so the caller surfaces it for human review.
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
	b.WriteString("\n\nOriginal SKILL.md (procedural style — has `name + description + markdown body`, no `command`):\n```\n")
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

// looksProcedural returns true if the candidate has YAML frontmatter with
// `name + description` but no `command` field — the structural shape of
// Anthropic / Hermes / Cursor procedural skills. The harvest pipeline
// uses this to decide whether a parse-failed candidate is a
// candidate-for-inspire or a genuinely malformed file.
func looksProcedural(content []byte) bool {
	rawYAML, _, err := soul.SplitFrontmatter(content)
	if err != nil {
		return false
	}
	var fm map[string]any
	if err := yaml.Unmarshal(rawYAML, &fm); err != nil {
		return false
	}
	name, _ := fm["name"].(string)
	desc, _ := fm["description"].(string)
	_, hasCmd := fm["command"]
	return name != "" && desc != "" && !hasCmd
}

// splitFrontmatterStr is the string-shaped wrapper around the byte-slice
// soul.SplitFrontmatter. Returns (rawYAML, body, err) — same contract.
// Caller is the pipeline's procedure-name fallback; uses YAML to prefer
// the original author's intended name over a slug we'd derive from the
// file path.
func splitFrontmatterStr(content string) (string, string, error) {
	yamlBytes, body, err := soul.SplitFrontmatter([]byte(content))
	return string(yamlBytes), string(body), err
}

// extractYAMLName pulls the top-level `name` field out of raw YAML
// frontmatter. Returns "" on any failure (malformed YAML, missing
// name, or non-string value). Used when a procedural-style candidate
// needs a stable identifier for the _procedural/ staging dir.
func extractYAMLName(rawYAML string) string {
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(rawYAML), &fm); err != nil {
		return ""
	}
	if n, ok := fm["name"].(string); ok {
		return n
	}
	return ""
}

// procNamePattern keeps the staging dir name to lowercase letters,
// digits, hyphens, underscores. Anything else (spaces, CJK, punctuation)
// is collapsed to a single underscore. Empty result falls back to
// "unnamed".
var procNamePattern = regexp.MustCompile(`[^a-z0-9_-]+`)

// sanitizeProcName normalizes an arbitrary string to a filesystem-safe
// identifier suitable for the _procedural/ staging subdir. Lowercases,
// replaces non-allowed chars with `_`, trims leading/trailing `_`.
func sanitizeProcName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = procNamePattern.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_-")
	if s == "" {
		return "unnamed"
	}
	return s
}
