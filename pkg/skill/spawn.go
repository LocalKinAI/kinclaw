package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// spawnSkill lets a parent agent dispatch a focused subtask to a child
// kinclaw process running a different soul. The child runs in an isolated
// process with isolated context, executes one prompt, and returns its
// final response. Used for:
//   - context savings: hand off "go research X" to a sub-agent so the
//     parent context stays clean
//   - specialization: pilot dispatches researcher / eye / critic
//   - parallel: three independent fetches as three sub-agents
//
// Hard rules enforced here:
//   - max recursion depth = 1 (sub-agents cannot themselves spawn — env
//     guard + permission gate together prevent towers of agents)
//   - per-spawn timeout (default 180s, max 600s) — runaway children
//     don't trap the parent
//   - returns child stdout only (boot banner goes to stderr, gets dropped)
type spawnSkill struct {
	enabled  bool
	soulDirs []string
}

// NewSpawnSkill is registered when the soul declares permissions.spawn:
// true. Pass the same soulDirs the kinclaw CLI uses to find soul files —
// makes "soul=researcher" resolve to "souls/researcher.soul.md" or
// "~/.localkin/souls/researcher.soul.md", whichever wins.
func NewSpawnSkill(enabled bool, soulDirs []string) Skill {
	return &spawnSkill{enabled: enabled, soulDirs: soulDirs}
}

func (s *spawnSkill) Name() string { return "spawn" }

func (s *spawnSkill) Description() string {
	return "Dispatch a focused subtask to a child kinclaw agent running a " +
		"different soul. The child runs in a separate process with isolated " +
		"context, executes ONE prompt, returns its final text response.\n\n" +
		"**Use when**:\n" +
		"  - context is filling up — push exploration / multi-step research to a sub-agent\n" +
		"  - a specialized soul fits better — researcher / eye / critic / coder\n" +
		"  - parallel work — three independent fetches can run as three sub-agents\n\n" +
		"**Hard limits (kernel-enforced)**:\n" +
		"  - max recursion depth = 1 (sub-agents cannot themselves spawn)\n" +
		"  - timeout = 180s default, 600s max per spawn\n" +
		"  - the child gets ONLY the prompt you pass — no chat history, no shared state\n\n" +
		"**Don't use** when the task is short and stays in your context budget anyway. " +
		"Spawning is for genuine context savings or genuine specialization, not " +
		"\"decompose for the sake of it.\"\n\n" +
		"**Available specialist souls** (each tuned to a model's strength):\n" +
		"  - `researcher` — Kimi K2.6 cloud, deep web search + long-context synthesis\n" +
		"  - `eye`        — Kimi K2.6 cloud, multimodal screenshot understanding\n" +
		"  - `critic`     — Minimax M2.7, second opinion on plans / produced artifacts\n" +
		"  - `coder`      — DeepSeek V4 Pro, code generation / review (when added)\n" +
		"  - `quick`      — DeepSeek Flash, fast yes/no verifications (when added)\n" +
		"  - `linguist`   — GLM 5.1, CN↔EN translation + style rewrite (when added)\n\n" +
		"The child's response comes back as your tool result. Only the final text is " +
		"returned — boot banners and intermediate logs are stripped."
}

func (s *spawnSkill) ToolDef() json.RawMessage {
	return MakeToolDef("spawn", s.Description(),
		map[string]map[string]string{
			"soul": {
				"type":        "string",
				"description": "Soul name to dispatch to (e.g. researcher / eye / critic). Resolved against ./souls/ then ~/.localkin/souls/.",
			},
			"prompt": {
				"type":        "string",
				"description": "The user message for the child agent. Single-shot — no conversation history is passed.",
			},
			"timeout_s": {
				"type":        "integer",
				"description": "Optional timeout in seconds. Default 180, capped at 600.",
			},
		},
		[]string{"soul", "prompt"})
}

// spawnDepthEnv is the env var children use to detect they're already a
// child and refuse further spawns. Set by the parent before exec.
const spawnDepthEnv = "KINCLAW_SPAWN_DEPTH"

func (s *spawnSkill) Execute(params map[string]string) (string, error) {
	if !s.enabled {
		return "", fmt.Errorf("spawn disabled — set permissions.spawn: true in your soul")
	}

	// Recursion guard. Belt-and-suspenders with the permission gate:
	// even if a child soul declared spawn: true (it shouldn't), this env
	// check refuses second-level spawns at the kernel layer.
	if os.Getenv(spawnDepthEnv) != "" {
		return "", fmt.Errorf("spawn refused: already running as a child agent (max recursion depth = 1). " +
			"Plan your sub-tasks at one level only — child agents cannot themselves spawn.")
	}

	soulName := strings.TrimSpace(params["soul"])
	if soulName == "" {
		return "", fmt.Errorf("soul is required")
	}
	prompt := strings.TrimSpace(params["prompt"])
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	timeoutS := 180
	if t := strings.TrimSpace(params["timeout_s"]); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 {
			if n > 600 {
				n = 600
			}
			timeoutS = n
		}
	}

	soulPath := s.resolveSoul(soulName)
	if soulPath == "" {
		return "", fmt.Errorf("spawn: soul %q not found in %v", soulName, s.soulDirs)
	}

	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("spawn: cannot locate kinclaw binary: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-soul", soulPath, "-exec", prompt)
	// Child gets the parent's env plus the recursion-depth marker. The
	// child's spawn skill (if registered) sees the marker and refuses.
	cmd.Env = append(os.Environ(), spawnDepthEnv+"=1")

	stdout, runErr := cmd.Output()
	// CombinedOutput-style fallback: capture stderr separately for error
	// reporting, but only the prompt RESPONSE (stdout of -exec) goes back
	// to the parent agent as the tool result.
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("spawn(%s): child agent timed out after %ds", soulName, timeoutS)
	}
	if runErr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return "", fmt.Errorf("spawn(%s) child exited with error: %w\n--- stderr (truncated) ---\n%s",
			soulName, runErr, truncateStr(stderr, 500))
	}

	return strings.TrimSpace(string(stdout)), nil
}

// resolveSoul searches the configured soul dirs for "<name>.soul.md".
// Mirrors cmd/kinclaw findSoulByName, kept local to avoid importing
// the cmd package.
func (s *spawnSkill) resolveSoul(name string) string {
	// If user passed a path that exists, use it.
	if strings.HasSuffix(name, ".soul.md") {
		if _, err := os.Stat(name); err == nil {
			abs, _ := filepath.Abs(name)
			return abs
		}
	}
	for _, dir := range s.soulDirs {
		path := filepath.Join(dir, name+".soul.md")
		if _, err := os.Stat(path); err == nil {
			abs, _ := filepath.Abs(path)
			return abs
		}
	}
	return ""
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}
