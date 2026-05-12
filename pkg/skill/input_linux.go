//go:build linux

// input_linux.go — Linux implementation of the input claw.
//
// Strategy: detect display server, shell out to the standard
// input-synthesis tool:
//
//   Wayland → `ydotool`   (uinput-based; needs ydotoold running and
//                          /dev/uinput accessible — usually a setup step
//                          per machine)
//   X11     → `xdotool`   (most distros' default; works out of the box)
//
// Coverage vs macOS input.go:
//   move x y              ✅
//   move_by dx dy         ✅
//   click [button]        ✅
//   triple_click          ✅
//   type "text"           ✅
//   hotkey key+modifiers  ✅
//   scroll dx dy          ✅
//   cursor                ✅  (xdotool getmouselocation; Wayland: only via compositor extension)
//   screen_size           ✅
//   key_down / key_up     ✅
//   drag                  ⚠️  composed from down+move+up; X11 reliable, Wayland less tested
//   paste                 ✅  via xclip/wl-paste + Ctrl+V hotkey
//   record_user_input     ⏳  (X11: libxdo events; Wayland: needs evdev)
//   target_pid routing    ❌  Linux input synthesis is system-wide;
//                              the focus-preserving "target_pid" trick
//                              has no clean equivalent. Defer to Phase 5.
//
// TODO(linux-verify): smoke-test all paths on Wayland (Sway / GNOME)
// + X11 (Xfce) before claiming feature parity.

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type inputSkill struct {
	allowed bool
}

// NewInputSkill returns the Linux input skill. Same signature as darwin.
func NewInputSkill(allowed bool) Skill {
	return &inputSkill{allowed: allowed}
}

func (s *inputSkill) Name() string { return "input" }

func (s *inputSkill) Description() string {
	return "Linux mouse/keyboard event synthesis via xdotool (X11) or " +
		"ydotool (Wayland). Actions: move | move_by | click | triple_click | " +
		"type | hotkey | scroll | cursor | screen_size | key_down | key_up | " +
		"drag | paste. Wayland requires /dev/uinput access (one-time setup)."
}

func (s *inputSkill) ToolDef() json.RawMessage {
	return MakeToolDef("input", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "move | click | type | hotkey | scroll | cursor | screen_size | paste | drag | key_down | key_up | triple_click | move_by",
			},
			"x":       {"type": "number", "description": "X coordinate (pixels)"},
			"y":       {"type": "number", "description": "Y coordinate (pixels)"},
			"dx":      {"type": "number", "description": "Delta X for move_by or scroll"},
			"dy":      {"type": "number", "description": "Delta Y for move_by or scroll"},
			"button":  {"type": "string", "description": "left (default) | right | middle"},
			"text":    {"type": "string", "description": "For action=type or paste"},
			"keys":    {"type": "string", "description": "For action=hotkey, e.g. 'ctrl+c' or 'super+space'"},
			"key":     {"type": "string", "description": "For key_down / key_up"},
			"per_char_delay_ms": {"type": "integer", "description": "For type: ms between chars (default 12 → ~80 cps)"},
		}, nil)
}

func (s *inputSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `input` capability")
	}

	action := params["action"]
	if action == "" {
		action = "move"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tool, err := pickInputTool()
	if err != nil && action != "screen_size" {
		return "", err
	}

	switch action {
	case "move":
		x, y, err := xyFromParams(params)
		if err != nil {
			return "", err
		}
		return runInput(ctx, tool, "mousemove", x, y)

	case "move_by":
		dx, dy := atoiOr(params["dx"], 0), atoiOr(params["dy"], 0)
		return runInput(ctx, tool, "mousemove_relative", "--", strconv.Itoa(dx), strconv.Itoa(dy))

	case "click":
		btn := buttonCode(params["button"])
		return runInput(ctx, tool, "click", btn)

	case "triple_click":
		btn := buttonCode(params["button"])
		return runInput(ctx, tool, "click", "--repeat", "3", "--delay", "50", btn)

	case "key_down":
		k := params["key"]
		if k == "" {
			return "", fmt.Errorf("key required")
		}
		return runInput(ctx, tool, "keydown", k)

	case "key_up":
		k := params["key"]
		if k == "" {
			return "", fmt.Errorf("key required")
		}
		return runInput(ctx, tool, "keyup", k)

	case "type":
		text := params["text"]
		if text == "" {
			return "", fmt.Errorf("text required")
		}
		delay := atoiOr(params["per_char_delay_ms"], 12)
		return runInput(ctx, tool, "type", "--delay", strconv.Itoa(delay), text)

	case "hotkey":
		keys := params["keys"]
		if keys == "" {
			return "", fmt.Errorf("keys required (e.g. 'ctrl+c')")
		}
		// Both xdotool and ydotool accept "ctrl+c"-style with `key`.
		return runInput(ctx, tool, "key", keys)

	case "scroll":
		// xdotool: button 4 = scroll up, 5 = down, 6 = left, 7 = right.
		dx, dy := atoiOr(params["dx"], 0), atoiOr(params["dy"], 0)
		switch {
		case dy < 0:
			return runInput(ctx, tool, "click", "--repeat", strconv.Itoa(-dy), "4")
		case dy > 0:
			return runInput(ctx, tool, "click", "--repeat", strconv.Itoa(dy), "5")
		case dx < 0:
			return runInput(ctx, tool, "click", "--repeat", strconv.Itoa(-dx), "6")
		case dx > 0:
			return runInput(ctx, tool, "click", "--repeat", strconv.Itoa(dx), "7")
		}
		return "ok: no-op (dx=dy=0)", nil

	case "cursor":
		// xdotool getmouselocation prints "x:NNN y:NNN screen:0 window:0x..."
		if tool != "xdotool" {
			return "", fmt.Errorf("cursor: Wayland does not expose global cursor position without compositor extension")
		}
		out, err := exec.CommandContext(ctx, tool, "getmouselocation").CombinedOutput()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil

	case "screen_size":
		return screenSizeFromXrandr(ctx)

	case "paste":
		text := params["text"]
		if text == "" {
			return "", fmt.Errorf("text required for paste")
		}
		if err := setClipboard(ctx, text); err != nil {
			return "", err
		}
		// Then Ctrl+V via the same tool.
		return runInput(ctx, tool, "key", "ctrl+v")

	case "drag":
		// down → move → up
		x, y, err := xyFromParams(params)
		if err != nil {
			return "", err
		}
		// Move to start (assumed already there if not specified)
		_, _ = runInput(ctx, tool, "mousemove", x, y)
		_, _ = runInput(ctx, tool, "mousedown", "1")
		dx, dy := atoiOr(params["dx"], 0), atoiOr(params["dy"], 0)
		_, _ = runInput(ctx, tool, "mousemove_relative", "--", strconv.Itoa(dx), strconv.Itoa(dy))
		_, err = runInput(ctx, tool, "mouseup", "1")
		if err != nil {
			return "", err
		}
		return "ok: drag complete", nil

	default:
		return "", fmt.Errorf("input action %q not yet ported to Linux (see input_linux.go header)", action)
	}
}

// --- helpers ------------------------------------------------------

func pickInputTool() (string, error) {
	if detectServer() == "wayland" && commandExists("ydotool") {
		return "ydotool", nil
	}
	if commandExists("xdotool") {
		return "xdotool", nil
	}
	if commandExists("ydotool") {
		return "ydotool", nil
	}
	return "", fmt.Errorf("neither xdotool (X11) nor ydotool (Wayland) installed")
}

func runInput(ctx context.Context, tool string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, tool, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w (%s)",
			tool, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("ok: %s %s", tool, strings.Join(args, " ")), nil
}

func xyFromParams(p map[string]string) (string, string, error) {
	x, y := p["x"], p["y"]
	if x == "" || y == "" {
		return "", "", fmt.Errorf("x and y required")
	}
	return x, y, nil
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func buttonCode(b string) string {
	// xdotool / ydotool both use 1=left, 2=middle, 3=right.
	switch strings.ToLower(strings.TrimSpace(b)) {
	case "right", "secondary":
		return "3"
	case "middle":
		return "2"
	default:
		return "1"
	}
}

func setClipboard(ctx context.Context, text string) error {
	if detectServer() == "wayland" && commandExists("wl-copy") {
		cmd := exec.CommandContext(ctx, "wl-copy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if commandExists("xclip") {
		cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if commandExists("xsel") {
		cmd := exec.CommandContext(ctx, "xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return fmt.Errorf("no clipboard tool found (install wl-clipboard, xclip, or xsel)")
}

func screenSizeFromXrandr(ctx context.Context) (string, error) {
	if commandExists("xrandr") {
		out, err := exec.CommandContext(ctx, "xrandr").CombinedOutput()
		if err != nil {
			return "", err
		}
		// Parse first "current WIDTH x HEIGHT" pattern.
		s := string(out)
		i := strings.Index(s, "current ")
		if i < 0 {
			return strings.TrimSpace(s), nil
		}
		rest := s[i+len("current "):]
		end := strings.IndexAny(rest, ",\n")
		if end < 0 {
			end = len(rest)
		}
		return strings.TrimSpace(rest[:end]), nil
	}
	if commandExists("wlr-randr") {
		out, err := exec.CommandContext(ctx, "wlr-randr").CombinedOutput()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	// Fallback: read /sys/class/drm to estimate
	if data, err := os.ReadFile("/sys/class/graphics/fb0/virtual_size"); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	return "", fmt.Errorf("no screen-size tool found")
}
