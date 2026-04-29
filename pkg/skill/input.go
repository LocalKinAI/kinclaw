//go:build darwin

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/LocalKinAI/input-go"
)

// inputSkill is the "hand" of KinClaw. It wraps input-go (CGEvent) so
// a soul can move the cursor, click, type, and send key combos.
// First use triggers the Accessibility TCC prompt. Without the
// permission, events silently no-op per macOS semantics.
type inputSkill struct {
	allowed bool
}

// NewInputSkill returns a skill that synthesizes mouse/keyboard events.
func NewInputSkill(allowed bool) Skill {
	return &inputSkill{allowed: allowed}
}

func (s *inputSkill) Name() string { return "input" }

func (s *inputSkill) Description() string {
	return "Synthesize mouse/keyboard events on macOS via CGEvent. Actions: " +
		"move (cursor), click (at point or current), type (UTF-8 text), " +
		"hotkey (modifier+key), scroll (wheel), cursor (read position), " +
		"screen_size. Set `target_pid` to drive an app in the BACKGROUND " +
		"without stealing focus from the user's foreground window — verified " +
		"on Lark / VSCode / Chrome and other Electron + WebKit hosts. " +
		"Requires macOS Accessibility permission."
}

func (s *inputSkill) ToolDef() json.RawMessage {
	return MakeToolDef("input", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "move | click | type | hotkey | scroll | cursor | screen_size",
			},
			"x":      {"type": "number", "description": "X coordinate (move/click/scroll)"},
			"y":      {"type": "number", "description": "Y coordinate (move/click)"},
			"button": {"type": "string", "description": "left | right | other (click; default left)"},
			"clicks": {"type": "integer", "description": "Click count 1/2/3. Default 1."},
			"text":   {"type": "string", "description": "Text to type"},
			"key":    {"type": "string", "description": "Key name for hotkey: c, enter, f5, left..."},
			"mods":   {"type": "string", "description": "Modifiers: 'cmd' / 'cmd+shift' / 'ctrl,alt'"},
			"smooth_ms": {
				"type":        "integer",
				"description": "For move: animate over N ms (default 0 = instant)",
			},
			"target_pid": {
				"type": "integer",
				"description": "Optional. Route input to a SPECIFIC process via CGEventPostToPid " +
					"instead of the global HID event tap. The targeted app receives the event " +
					"but its window does NOT come to front — the user's foreground app keeps " +
					"focus. Get the PID from `ui focused_app` output or the OS. Verified on " +
					"Lark / VSCode / Chrome / Cursor; some Apple sandboxed apps (newer Mail / " +
					"Messages) may ignore — fall back to no target_pid if no effect. Omit (or 0) " +
					"for legacy system-wide behavior.",
			},
		}, []string{"action"})
}

func (s *inputSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `input` capability")
	}
	action := params["action"]
	if action == "" {
		return "", fmt.Errorf("missing required parameter: action")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// target_pid > 0 routes events directly to the target process via
	// CGEventPostToPid (no focus steal). Empty/0 keeps legacy system-
	// wide kCGHIDEventTap behavior. Resolved once per Execute call so
	// every event in a multi-step action (Click = move+down+up, Hotkey
	// = mods down + key + mods up) routes consistently.
	opts, pidLabel := postOpts(params)

	switch action {
	case "cursor":
		x, y, err := input.CursorPosition()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("cursor at (%.0f, %.0f)", x, y), nil

	case "screen_size":
		w, h, err := input.ScreenSize()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("main display: %.0fx%.0f", w, h), nil

	case "move":
		x, y, err := xy(params)
		if err != nil {
			return "", err
		}
		if ms := atoiDefault(params["smooth_ms"], 0); ms > 0 {
			if err := input.MoveSmooth(ctx, x, y, time.Duration(ms)*time.Millisecond, opts...); err != nil {
				return "", err
			}
			return fmt.Sprintf("moved cursor to (%.0f, %.0f) over %dms%s", x, y, ms, pidLabel), nil
		}
		if err := input.Move(ctx, x, y, opts...); err != nil {
			return "", err
		}
		return fmt.Sprintf("moved cursor to (%.0f, %.0f)%s", x, y, pidLabel), nil

	case "click":
		button, err := parseButton(params["button"])
		if err != nil {
			return "", err
		}
		clicks := atoiDefault(params["clicks"], 1)
		if params["x"] != "" || params["y"] != "" {
			x, y, err := xy(params)
			if err != nil {
				return "", err
			}
			if err := input.ClickAtButton(ctx, x, y, button, clicks, opts...); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s-click at (%.0f, %.0f) x%d%s",
				buttonName(button), x, y, clicks, pidLabel), nil
		}
		if err := input.Click(ctx, button, clicks, opts...); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s-click at current cursor x%d%s",
			buttonName(button), clicks, pidLabel), nil

	case "type":
		text := params["text"]
		if text == "" {
			return "", fmt.Errorf("type: missing `text`")
		}
		if err := input.Type(ctx, text, opts...); err != nil {
			return "", err
		}
		return fmt.Sprintf("typed %d chars%s", len([]rune(text)), pidLabel), nil

	case "hotkey":
		modsStr := params["mods"]
		keyStr := params["key"]
		if keyStr == "" {
			return "", fmt.Errorf("hotkey: missing `key`")
		}
		mods, ok := input.ParseModifiers(modsStr)
		if !ok {
			return "", fmt.Errorf("hotkey: bad mods %q", modsStr)
		}
		k, ok := input.KeyByName(keyStr)
		if !ok {
			return "", fmt.Errorf("hotkey: unknown key %q", keyStr)
		}
		if err := input.Hotkey(ctx, mods, k, opts...); err != nil {
			return "", err
		}
		if modsStr == "" {
			return fmt.Sprintf("pressed %s%s", keyStr, pidLabel), nil
		}
		return fmt.Sprintf("pressed %s+%s%s", modsStr, keyStr, pidLabel), nil

	case "scroll":
		dx := atoiDefault(params["x"], 0)
		dy := atoiDefault(params["y"], 0)
		if err := input.Scroll(ctx, dx, dy, opts...); err != nil {
			return "", err
		}
		return fmt.Sprintf("scrolled (%d, %d) px%s", dx, dy, pidLabel), nil

	default:
		return "", fmt.Errorf("unknown action %q", action)
	}
}

// postOpts resolves the input.PostOption set from the action params.
// Returns the option slice (nil when no special routing) plus a
// human-readable suffix the result message appends so the soul + logs
// make it obvious whether a call ran in PID-targeted background mode
// or default system-wide mode.
func postOpts(params map[string]string) ([]input.PostOption, string) {
	pid := atoiDefault(params["target_pid"], 0)
	if pid <= 0 {
		return nil, ""
	}
	return []input.PostOption{input.WithPID(int32(pid))}, fmt.Sprintf(" → pid %d (no focus steal)", pid)
}

// ─── helpers ─────────────────────────────────────────────────

func xy(p map[string]string) (float64, float64, error) {
	x, err := strconv.ParseFloat(p["x"], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("bad x %q: %w", p["x"], err)
	}
	y, err := strconv.ParseFloat(p["y"], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("bad y %q: %w", p["y"], err)
	}
	return x, y, nil
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func parseButton(s string) (input.MouseButton, error) {
	switch s {
	case "", "left":
		return input.ButtonLeft, nil
	case "right":
		return input.ButtonRight, nil
	case "other", "middle":
		return input.ButtonOther, nil
	}
	return 0, fmt.Errorf("bad button %q", s)
}

func buttonName(b input.MouseButton) string {
	switch b {
	case input.ButtonLeft:
		return "left"
	case input.ButtonRight:
		return "right"
	case input.ButtonOther:
		return "other"
	}
	return "?"
}
