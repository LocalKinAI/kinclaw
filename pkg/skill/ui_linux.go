//go:build linux

// ui_linux.go — Linux implementation of the ui claw.
//
// Strategy: minimal MVP that wraps xdotool + wmctrl for window/process
// queries. Full AT-SPI 2 accessibility tree walking via D-Bus is
// deferred to Phase 4 (needs godbus dependency + interface mapping).
//
// Coverage vs macOS ui.go (which wraps kinax-go AX tree):
//   focused_app           ✅  xdotool getactivewindow + getwindowname
//   window_list           ✅  wmctrl -lp (X11) ; Wayland: locked
//   window_geometry       ✅  xdotool getwindowgeometry
//   tree (AT-SPI)         ⏳  Phase 4 — needs godbus + at-spi-2-core
//   find / click_by_*     ⏳  Phase 4 — depends on tree
//   watch (focus events)  ⏳  Phase 4 — D-Bus signal subscription
//
// Linux's accessibility surface is genuinely less unified than macOS's.
// GNOME exposes AT-SPI well; Sway / Wayland compositors don't. This MVP
// focuses on what's universal (window enumeration); full a11y tree
// walking will be GNOME-first in Phase 4.
//
// TODO(linux-verify): all xdotool / wmctrl paths untested in this build env.

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type uiSkill struct {
	allowed bool
}

func NewUISkill(allowed bool) Skill {
	return &uiSkill{allowed: allowed}
}

func (s *uiSkill) Name() string { return "ui" }

func (s *uiSkill) Description() string {
	return "Linux UI inspection via xdotool + wmctrl. " +
		"Actions: focused_app (active window's app + title) | " +
		"window_list (all visible windows, X11 only) | " +
		"window_geometry (focused window's x,y,w,h). " +
		"AT-SPI accessibility tree walking is Phase 4."
}

func (s *uiSkill) ToolDef() json.RawMessage {
	return MakeToolDef("ui", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "focused_app | window_list | window_geometry",
			},
		}, nil)
}

func (s *uiSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `ui` capability")
	}

	action := params["action"]
	if action == "" {
		action = "focused_app"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch action {
	case "focused_app":
		return s.focusedApp(ctx)
	case "window_list":
		return s.windowList(ctx)
	case "window_geometry":
		return s.windowGeometry(ctx)
	default:
		return "", fmt.Errorf("ui action %q not yet ported to Linux (see ui_linux.go header)", action)
	}
}

func (s *uiSkill) focusedApp(ctx context.Context) (string, error) {
	if !commandExists("xdotool") {
		return "", fmt.Errorf("xdotool not installed")
	}
	// Get the active window's ID, then its name + class.
	idOut, err := exec.CommandContext(ctx, "xdotool", "getactivewindow").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xdotool getactivewindow: %w (%s)", err, strings.TrimSpace(string(idOut)))
	}
	wid := strings.TrimSpace(string(idOut))
	if wid == "" {
		return "", fmt.Errorf("no active window")
	}
	nameOut, _ := exec.CommandContext(ctx, "xdotool", "getwindowname", wid).CombinedOutput()
	classOut, _ := exec.CommandContext(ctx, "xdotool", "getwindowclassname", wid).CombinedOutput()
	return fmt.Sprintf("{\"window_id\":%q,\"title\":%q,\"class\":%q}",
		wid,
		strings.TrimSpace(string(nameOut)),
		strings.TrimSpace(string(classOut))), nil
}

func (s *uiSkill) windowList(ctx context.Context) (string, error) {
	if !commandExists("wmctrl") {
		return "", fmt.Errorf("wmctrl not installed")
	}
	out, err := exec.CommandContext(ctx, "wmctrl", "-l", "-p", "-x").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wmctrl: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *uiSkill) windowGeometry(ctx context.Context) (string, error) {
	if !commandExists("xdotool") {
		return "", fmt.Errorf("xdotool not installed")
	}
	idOut, err := exec.CommandContext(ctx, "xdotool", "getactivewindow").CombinedOutput()
	if err != nil {
		return "", err
	}
	wid := strings.TrimSpace(string(idOut))
	geoOut, err := exec.CommandContext(ctx, "xdotool", "getwindowgeometry", wid).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xdotool getwindowgeometry: %w (%s)", err, strings.TrimSpace(string(geoOut)))
	}
	return strings.TrimSpace(string(geoOut)), nil
}
