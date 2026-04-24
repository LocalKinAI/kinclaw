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
		"screen_size. Requires macOS Accessibility permission."
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
			if err := input.MoveSmooth(ctx, x, y, time.Duration(ms)*time.Millisecond); err != nil {
				return "", err
			}
			return fmt.Sprintf("moved cursor to (%.0f, %.0f) over %dms", x, y, ms), nil
		}
		if err := input.Move(ctx, x, y); err != nil {
			return "", err
		}
		return fmt.Sprintf("moved cursor to (%.0f, %.0f)", x, y), nil

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
			if err := input.ClickAtButton(ctx, x, y, button, clicks); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s-click at (%.0f, %.0f) x%d",
				buttonName(button), x, y, clicks), nil
		}
		if err := input.Click(ctx, button, clicks); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s-click at current cursor x%d",
			buttonName(button), clicks), nil

	case "type":
		text := params["text"]
		if text == "" {
			return "", fmt.Errorf("type: missing `text`")
		}
		if err := input.Type(ctx, text); err != nil {
			return "", err
		}
		return fmt.Sprintf("typed %d chars", len([]rune(text))), nil

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
		if err := input.Hotkey(ctx, mods, k); err != nil {
			return "", err
		}
		if modsStr == "" {
			return fmt.Sprintf("pressed %s", keyStr), nil
		}
		return fmt.Sprintf("pressed %s+%s", modsStr, keyStr), nil

	case "scroll":
		dx := atoiDefault(params["x"], 0)
		dy := atoiDefault(params["y"], 0)
		if err := input.Scroll(ctx, dx, dy); err != nil {
			return "", err
		}
		return fmt.Sprintf("scrolled (%d, %d) px", dx, dy), nil

	default:
		return "", fmt.Errorf("unknown action %q", action)
	}
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
