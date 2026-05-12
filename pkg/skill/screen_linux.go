//go:build linux

// screen_linux.go — Linux implementation of the screen claw.
//
// Strategy: detect display server at runtime, shell out to the
// standard CLI tool for it.
//
//   Wayland → `grim`     (preferred; also handles HiDPI cleanly)
//   X11     → `scrot`    (most distros) or `import` (ImageMagick)
//
// Coverage vs macOS screen.go (Aug 2026 status):
//   screenshot          ✅  via grim / scrot
//   list_displays       ✅  via wlr-randr (Wayland) / xrandr (X11)
//   list_windows        ⚠️  via wmctrl (X11 only); Wayland is privacy-locked
//                            without an explicit compositor extension
//   list_apps           ⚠️  same as list_windows; X11 only
//   ocr / ocr_regions   ⏳  not yet ported; need tesseract + bbox parsing
//   screenshot_app      ⏳  X11: by-window-id; Wayland: needs portal API
//   diff_screenshots    ⏳  pixel-diff is OS-agnostic but the capture is OS-specific
//   color_at_point      ⏳  via xdotool getmouselocation + image sample
//   live_stream         ⏳  PipeWire / xdg-desktop-portal screencast
//
// TODO(linux-verify): all paths below are written from documentation
// without hardware in the build environment. Smoke-test on Wayland
// (Sway / GNOME 45+) and X11 (Xfce) before claiming feature-parity.

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type screenSkill struct {
	allowed   bool
	outputDir string
}

// NewScreenSkill returns the Linux screen skill. Signature matches
// the darwin version verbatim so the kernel's constructor call
// compiles unchanged across platforms.
func NewScreenSkill(allowed bool, outputDir string) Skill {
	if outputDir == "" {
		base, _ := os.UserCacheDir()
		if base == "" {
			base = os.TempDir()
		}
		outputDir = filepath.Join(base, "kinclaw", "screens")
	}
	outputDir = expandHome(outputDir)
	_ = os.MkdirAll(outputDir, 0o755)
	return &screenSkill{allowed: allowed, outputDir: outputDir}
}

func (s *screenSkill) Name() string { return "screen" }

func (s *screenSkill) Description() string {
	return "Linux screen capture via grim (Wayland) or scrot (X11). " +
		"Actions: screenshot (full screen, or region=x,y,w,h) | " +
		"list_displays (wlr-randr / xrandr) | list_windows (wmctrl, X11 only). " +
		"OCR / live_stream / screenshot_app are TODO (planned for Phase 3+)."
}

func (s *screenSkill) ToolDef() json.RawMessage {
	return MakeToolDef("screen", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "screenshot (default) | list_displays | list_windows",
			},
			"region": {
				"type":        "string",
				"description": "For action=screenshot: 'x,y,w,h' in pixels. Captures only that sub-rectangle.",
			},
			"output_path": {
				"type":        "string",
				"description": "Optional explicit PNG path for screenshot. Default: timestamped file in cache dir.",
			},
		}, nil)
}

func (s *screenSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `screen` capability")
	}

	action := params["action"]
	if action == "" {
		action = "screenshot"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	switch action {
	case "screenshot":
		return s.screenshot(ctx, params)
	case "list_displays":
		return s.listDisplays(ctx)
	case "list_windows":
		return s.listWindows(ctx)
	default:
		return "", fmt.Errorf("screen action %q not yet ported to Linux (see screen_linux.go header for coverage)", action)
	}
}

// detectServer returns "wayland" or "x11" based on environment.
// Wayland is preferred when both are present (XWayland fallback).
func detectServer() string {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return "wayland"
	}
	if os.Getenv("DISPLAY") != "" {
		return "x11"
	}
	return ""
}

func (s *screenSkill) screenshot(ctx context.Context, params map[string]string) (string, error) {
	out := params["output_path"]
	if out == "" {
		ts := time.Now().Format("20060102-150405")
		out = filepath.Join(s.outputDir, fmt.Sprintf("screen-%s.png", ts))
	}

	server := detectServer()
	region := strings.TrimSpace(params["region"])

	switch server {
	case "wayland":
		if !commandExists("grim") {
			return "", fmt.Errorf("Wayland detected but `grim` not installed (install via package manager: apt install grim / dnf install grim / pacman -S grim)")
		}
		args := []string{}
		if region != "" {
			// grim takes -g "X,Y WxH"
			gr, err := parseRegionForGrim(region)
			if err != nil {
				return "", fmt.Errorf("bad region %q: %w", region, err)
			}
			args = append(args, "-g", gr)
		}
		args = append(args, out)
		if err := runCmd(ctx, "grim", args...); err != nil {
			return "", fmt.Errorf("grim: %w", err)
		}

	case "x11":
		if commandExists("scrot") {
			args := []string{}
			if region != "" {
				// scrot uses "-a x,y,w,h"
				args = append(args, "-a", region)
			}
			args = append(args, out)
			if err := runCmd(ctx, "scrot", args...); err != nil {
				return "", fmt.Errorf("scrot: %w", err)
			}
		} else if commandExists("import") {
			// ImageMagick fallback. Always full-screen; region clipped via ImageMagick later if needed.
			if err := runCmd(ctx, "import", "-window", "root", out); err != nil {
				return "", fmt.Errorf("import: %w", err)
			}
		} else {
			return "", fmt.Errorf("X11 detected but neither `scrot` nor ImageMagick `import` installed")
		}

	default:
		return "", fmt.Errorf("no display server detected (set $DISPLAY for X11 or $WAYLAND_DISPLAY for Wayland)")
	}

	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		return "", fmt.Errorf("screenshot did not produce a non-empty file at %s", out)
	}

	// Return the image:// marker so the brain attaches the bytes
	// inline for vision-capable models. Matches macOS conventions.
	return fmt.Sprintf("image://%s", out), nil
}

func (s *screenSkill) listDisplays(ctx context.Context) (string, error) {
	server := detectServer()
	switch server {
	case "wayland":
		if !commandExists("wlr-randr") {
			return "", fmt.Errorf("Wayland detected but `wlr-randr` not installed (apt install wlr-randr)")
		}
		out, err := exec.CommandContext(ctx, "wlr-randr").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("wlr-randr: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return string(out), nil

	case "x11":
		if !commandExists("xrandr") {
			return "", fmt.Errorf("X11 detected but `xrandr` not installed")
		}
		out, err := exec.CommandContext(ctx, "xrandr", "--query").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("xrandr: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return string(out), nil
	}
	return "", fmt.Errorf("no display server detected")
}

func (s *screenSkill) listWindows(ctx context.Context) (string, error) {
	// wmctrl is X11-only. Wayland (per protocol design) doesn't allow
	// other apps to enumerate windows except via compositor-specific
	// extensions. Skip on Wayland — agent must use ui claw (AT-SPI)
	// to get focused-app structure instead.
	if detectServer() != "x11" {
		return "", fmt.Errorf("list_windows: Wayland intentionally restricts cross-app window enumeration. Use the `ui` skill (AT-SPI) for focused-app structure")
	}
	if !commandExists("wmctrl") {
		return "", fmt.Errorf("wmctrl not installed (apt install wmctrl)")
	}
	out, err := exec.CommandContext(ctx, "wmctrl", "-l", "-x").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wmctrl: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// --- small helpers ------------------------------------------------

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runCmd(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (output: %s)",
			name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// parseRegionForGrim converts our "x,y,w,h" format to grim's "X,Y WxH".
func parseRegionForGrim(region string) (string, error) {
	parts := strings.Split(region, ",")
	if len(parts) != 4 {
		return "", fmt.Errorf("expected 4 comma-separated values, got %d", len(parts))
	}
	x := strings.TrimSpace(parts[0])
	y := strings.TrimSpace(parts[1])
	w := strings.TrimSpace(parts[2])
	h := strings.TrimSpace(parts[3])
	return fmt.Sprintf("%s,%s %sx%s", x, y, w, h), nil
}
