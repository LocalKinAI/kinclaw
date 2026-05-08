//go:build darwin

package skill

import (
	"context"
	"fmt"
	"time"

	input "github.com/LocalKinAI/input-go"
)

// This file groups the v1.14 "input claw 100%" verbs — exposing
// kit-level capabilities that input-go already provides but the
// kinclaw skill surface didn't expose previously: TypeSlow (per-char
// pacing for IMEs), KeyDown/KeyUp (atomic key state for hold-while-
// click flows), Press (single key without modifier), triple_click
// (paragraph selection), MoveBy (relative cursor movement),
// ScrollSmooth (paced scroll for animated containers).
//
// Each verb is a thin wrapper — input-go is the kit, kinclaw skill
// is the model-facing surface. The skill's job is to translate
// LLM-friendly param names into kit calls.

// type_slow — per-character delay typing. The default `type` action
// hammers chars at full keyboard rate (~100/s), which IME front-ends
// (Pinyin / Wubi / Kotoeri) often can't keep up with — characters
// get dropped or the IME's composition state desyncs. type_slow
// paces at a configurable per-char delay (default 50ms = 20 chars/s,
// reliable across all Mac IMEs).
//
// Optional `jitter_pct` introduces random variation (±N% of base
// delay) to mimic human typing rhythm — useful for capchas or
// anti-bot frontends that detect uniform timing as automation.
// Default 0 (no jitter) keeps deterministic behavior the norm.
func (s *inputSkill) typeSlow(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	text := params["text"]
	if text == "" {
		return "", fmt.Errorf("type_slow: missing `text`")
	}
	delayMs := atoiDefault(params["per_char_delay_ms"], 50)
	if delayMs < 1 {
		delayMs = 1
	}
	if delayMs > 1000 {
		delayMs = 1000 // user-typo guard; nobody types at 1 char/s on purpose
	}
	jitterPct := atoiDefault(params["jitter_pct"], 0)
	if jitterPct < 0 {
		jitterPct = 0
	}
	if jitterPct > 80 {
		jitterPct = 80 // beyond ±80% the cadence becomes erratic-not-human
	}

	if jitterPct == 0 {
		if err := input.TypeSlow(ctx, text, time.Duration(delayMs)*time.Millisecond, opts...); err != nil {
			return "", err
		}
		return fmt.Sprintf("type_slow %d chars at %dms/char%s",
			len([]rune(text)), delayMs, pidLabel), nil
	}

	// Jittered path: type runes one-by-one with per-char delays drawn
	// from [delayMs * (1 - jitterPct/100), delayMs * (1 + jitterPct/100)].
	// We construct the input.TypeSlow equivalent manually so each
	// inter-char wait can use a different value.
	rng := newJitterRNG()
	for _, r := range text {
		if err := input.Type(ctx, string(r), opts...); err != nil {
			return "", err
		}
		base := delayMs
		spread := base * jitterPct / 100
		// Range [base-spread, base+spread]
		d := base + rng.intn(2*spread+1) - spread
		if d < 1 {
			d = 1
		}
		select {
		case <-time.After(time.Duration(d) * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return fmt.Sprintf("type_slow %d chars at %dms/char ±%d%% jitter%s",
		len([]rune(text)), delayMs, jitterPct, pidLabel), nil
}

// newJitterRNG / intn — tiny non-crypto RNG using time-based seed.
// We intentionally don't pull math/rand here to keep the package's
// import set tight; this is enough for typing cadence variation.
type jitterRNG struct{ state uint64 }

func newJitterRNG() *jitterRNG {
	return &jitterRNG{state: uint64(time.Now().UnixNano())}
}

func (r *jitterRNG) intn(n int) int {
	if n <= 0 {
		return 0
	}
	// xorshift64
	r.state ^= r.state << 13
	r.state ^= r.state >> 7
	r.state ^= r.state << 17
	return int(r.state % uint64(n))
}

// key_down / key_up — atomic keyboard state primitives. Most flows
// use `hotkey` (modifier+key fully synthesized) or `type` (text
// typed end-to-end). For "hold ⇧ while clicking three rows to
// select range" or "hold ⌥ to drag a copy" you need to be able
// to bring a modifier down, do other input events, then bring
// it up. Without these primitives that flow is impossible from
// the skill surface.
//
// Mods are accepted on key_down/key_up too because some hotkey
// chords use multiple sticky modifiers (⌘⇧⌥), and you might want
// to lift only the ⌥ leaving ⌘⇧ held — kinkit input-go's KeyDown
// / KeyUp accepts a mods bitmask to make this composable.
func (s *inputSkill) keyDown(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	keyStr := params["key"]
	if keyStr == "" {
		return "", fmt.Errorf("key_down: missing `key`")
	}
	k, ok := input.KeyByName(keyStr)
	if !ok {
		return "", fmt.Errorf("key_down: unknown key %q", keyStr)
	}
	mods, ok := input.ParseModifiers(params["mods"])
	if !ok {
		return "", fmt.Errorf("key_down: bad mods %q", params["mods"])
	}
	if err := input.KeyDown(ctx, k, mods, opts...); err != nil {
		return "", err
	}
	return fmt.Sprintf("key_down %s (mods=%q)%s", keyStr, params["mods"], pidLabel), nil
}

func (s *inputSkill) keyUp(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	keyStr := params["key"]
	if keyStr == "" {
		return "", fmt.Errorf("key_up: missing `key`")
	}
	k, ok := input.KeyByName(keyStr)
	if !ok {
		return "", fmt.Errorf("key_up: unknown key %q", keyStr)
	}
	mods, ok := input.ParseModifiers(params["mods"])
	if !ok {
		return "", fmt.Errorf("key_up: bad mods %q", params["mods"])
	}
	if err := input.KeyUp(ctx, k, mods, opts...); err != nil {
		return "", err
	}
	return fmt.Sprintf("key_up %s (mods=%q)%s", keyStr, params["mods"], pidLabel), nil
}

// triple_click — selects an entire paragraph in most text editors.
// Composes as ClickAtButton with clicks=3; we expose it as its own
// verb so the soul protocol can document the use case ("click line
// 3 times to select the whole sentence") without having to reach
// for the generic clicks=3 trick.
func (s *inputSkill) tripleClick(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	if params["x"] == "" || params["y"] == "" {
		return "", fmt.Errorf("triple_click: x and y are required")
	}
	x, y, err := xy(params)
	if err != nil {
		return "", err
	}
	if err := input.ClickAtButton(ctx, x, y, input.ButtonLeft, 3, opts...); err != nil {
		return "", err
	}
	return fmt.Sprintf("triple-clicked at (%.0f, %.0f)%s", x, y, pidLabel), nil
}

// move_by — relative cursor movement (delta from current position).
// Useful when the model has no absolute coordinate but knows "nudge
// 10 px right" — e.g., scrolling a slider, fine-tuning a selection.
func (s *inputSkill) moveBy(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	dxStr := params["dx"]
	dyStr := params["dy"]
	if dxStr == "" && dyStr == "" {
		return "", fmt.Errorf("move_by: at least one of dx, dy is required")
	}
	dx, dy := 0.0, 0.0
	if dxStr != "" {
		f, err := parseFloatParam(params, "dx")
		if err != nil {
			return "", err
		}
		dx = f
	}
	if dyStr != "" {
		f, err := parseFloatParam(params, "dy")
		if err != nil {
			return "", err
		}
		dy = f
	}
	if err := input.MoveBy(ctx, dx, dy, opts...); err != nil {
		return "", err
	}
	return fmt.Sprintf("moved cursor by (%+.0f, %+.0f)%s", dx, dy, pidLabel), nil
}

// scroll_smooth — paced scroll over a duration. The default `scroll`
// action fires the entire delta in one event — apps with momentum-
// style scrolling (Safari, Mail, Notes) often jump or skip. paced
// scroll over 200-800ms gives the kinetic engine time to interpolate
// and feels natural. Same dx/dy semantics as `scroll`.
func (s *inputSkill) scrollSmooth(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	dx := atoiDefault(params["x"], 0)
	dy := atoiDefault(params["y"], 0)
	if dx == 0 && dy == 0 {
		return "", fmt.Errorf("scroll_smooth: x or y must be non-zero")
	}
	durationMs := atoiDefault(params["duration_ms"], 300)
	if durationMs < 50 {
		durationMs = 50
	}
	if durationMs > 5000 {
		durationMs = 5000
	}
	if err := input.ScrollSmooth(ctx, dx, dy, time.Duration(durationMs)*time.Millisecond, opts...); err != nil {
		return "", err
	}
	return fmt.Sprintf("scroll_smooth (%d, %d) over %dms%s",
		dx, dy, durationMs, pidLabel), nil
}
