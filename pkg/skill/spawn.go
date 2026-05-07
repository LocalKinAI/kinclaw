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
// SpawnResult is the payload pushed to the kernel when a detached
// spawn job finishes. Kernel decides what to do with it (typically:
// SSE event for UI + queue for next-turn history injection).
type SpawnResult struct {
	JobID    string        // ab12cd-style short id for UI threading
	Soul     string        // e.g. "researcher"
	Prompt   string        // what the child was asked
	Output   string        // child's final text (stdout)
	Err      error         // non-nil if cmd.Output failed or timed out
	Started  time.Time     // for "took 4m 23s" rendering
	Duration time.Duration // wall-clock from goroutine start to finish
}

// SpawnResultCallback is invoked from the spawn skill's goroutine when
// a detached child completes. Implementations should be cheap + non-
// blocking — typically they push an SSE event and append to a per-
// session pending-results queue. nil = sync-only mode (legacy behavior).
type SpawnResultCallback func(SpawnResult)

type spawnSkill struct {
	enabled  bool
	soulDirs []string

	// onResult runs from a goroutine when a detached spawn finishes.
	// Set via SetResultCallback after the kernel's SSE server is wired
	// up (chicken-and-egg: skill is built before serve.go has its
	// Server). nil → all spawns run sync regardless of detach hint,
	// which is the right fallback for `kinclaw -exec` one-shot mode
	// where there's no SSE consumer to deliver async results to.
	onResult SpawnResultCallback
}

// NewSpawnSkill is registered when the soul declares permissions.spawn:
// true. Pass the same soulDirs the kinclaw CLI uses to find soul files —
// makes "soul=researcher" resolve to "souls/researcher.soul.md" or
// "~/.localkin/souls/researcher.soul.md", whichever wins.
func NewSpawnSkill(enabled bool, soulDirs []string) Skill {
	return &spawnSkill{enabled: enabled, soulDirs: soulDirs}
}

// SetResultCallback wires the async-completion hook. Called by serve.go
// after the SSE Server is constructed. Safe to call multiple times
// (e.g. after a soul switch); the latest callback wins.
func (s *spawnSkill) SetResultCallback(cb SpawnResultCallback) {
	s.onResult = cb
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
		"  - `coder`      — DeepSeek V4 Pro, harvest --inspire forge specialist (re-implements\n" +
		"                  external SKILL.md as KinClaw exec form, refuses non-exec'able ones)\n" +
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
				"description": "Optional timeout in seconds. Default 180, capped at 900 (15 min). Use 600+ for deep research / multi-source synthesis (researcher running 8+ knowledge_search calls + 5+ web_search rounds + Step 5 file_write commonly takes 2-7 min). Use 60-90 for quick verifications (critic review, eye glance) — those auto-stay-sync at <=90s.",
			},
			"detach": {
				"type":        "string",
				"description": "Optional. \"true\" forces detached/async mode (child runs in background, parent turn ends immediately, result is delivered via UI event when child finishes). \"false\" forces sync. Default: auto — sync if timeout_s ≤ 90, detached if > 90. Use detached for deep-research / long synthesis so the user can keep chatting; sync for quick verifications (critic review, eye glance) that complete in under a minute.",
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
			// 900s = 15 min cap. Researcher's deep-dive (8 masters via
			// knowledge_search + 5+ web_search rounds + Step 5 file_write)
			// has been observed taking 2-7 min; 5-min hard cap was
			// causing borderline timeouts. 15 min gives comfortable
			// headroom while still bounding runaway children.
			if n > 900 {
				n = 900
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

	// Decide sync vs detached. Auto-rule: long-running spawns
	// (timeout > 90s) detach by default so the parent's turn can
	// release turnMu and the user can keep chatting. Quick spawns
	// (critic review, eye glance) stay sync so pilot can branch on
	// the result inline. Explicit detach=true/false param overrides.
	detach := timeoutS > 90
	switch strings.ToLower(strings.TrimSpace(params["detach"])) {
	case "true", "1", "yes":
		detach = true
	case "false", "0", "no":
		detach = false
	}
	// Detach requires the result callback be wired. In `kinclaw -exec`
	// one-shot mode there's no SSE consumer to deliver async results,
	// so we degrade to sync (still works, just blocks).
	if detach && s.onResult == nil {
		detach = false
	}

	if detach {
		jobID := newJobID()
		go s.runDetached(jobID, bin, soulPath, soulName, prompt, timeoutS)
		return fmt.Sprintf(
			"Detached spawn started: soul=%s job=%s timeout=%ds.\n"+
				"The child is running in the background — your current turn ends now, the user can continue chatting, and the result will be delivered via a `spawn_done` UI event when the child finishes (typically a few minutes for research / synthesis tasks).\n"+
				"Tell the user briefly that you've dispatched the work and that they're free to ask other things while it runs. Do NOT call spawn again for the same task.",
			soulName, jobID, timeoutS,
		), nil
	}

	// Sync path (legacy + short tasks).
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

// runDetached executes the child kinclaw subprocess in a goroutine and
// hands the result back to the kernel via s.onResult. Errors don't
// propagate to the parent's tool_result (that already returned the
// "Detached spawn started" ack); they ride along in SpawnResult.Err
// so the kernel can surface them in the spawn_done UI event.
func (s *spawnSkill) runDetached(jobID, bin, soulPath, soulName, prompt string, timeoutS int) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-soul", soulPath, "-exec", prompt)
	cmd.Env = append(os.Environ(), spawnDepthEnv+"=1")

	stdout, runErr := cmd.Output()

	res := SpawnResult{
		JobID:    jobID,
		Soul:     soulName,
		Prompt:   prompt,
		Started:  start,
		Duration: time.Since(start),
		Output:   strings.TrimSpace(string(stdout)),
	}
	if ctx.Err() == context.DeadlineExceeded {
		res.Err = fmt.Errorf("child timed out after %ds", timeoutS)
	} else if runErr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		res.Err = fmt.Errorf("%w\nstderr: %s", runErr, truncateStr(stderr, 500))
	}

	if s.onResult != nil {
		s.onResult(res)
	}
}

// newJobID returns a short hex token good enough to disambiguate
// detached spawns within a session. Not cryptographic — just a UI
// thread handle so user / pilot can refer to "job ab12cd".
func newJobID() string {
	return fmt.Sprintf("%06x", time.Now().UnixNano()&0xFFFFFF)
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
