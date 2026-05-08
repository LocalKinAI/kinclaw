//go:build darwin

package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LocalKinAI/sckit-go"
)

// screenSkill is the "eye" of KinClaw. It wraps sckit-go (ScreenCaptureKit)
// so a soul can take screenshots of the macOS display and enumerate
// attached displays. First use triggers the Screen Recording TCC prompt.
type screenSkill struct {
	allowed   bool
	outputDir string
}

// NewScreenSkill returns a skill that captures the macOS screen. If
// allowed is false, Execute returns a permission error. outputDir is
// where PNG files land; empty means ~/Library/Caches/kinclaw/screens/.
// Leading "~" and "~/" in outputDir are expanded to the user's home.
func NewScreenSkill(allowed bool, outputDir string) Skill {
	if outputDir == "" {
		base, _ := os.UserCacheDir()
		if base == "" {
			base = os.TempDir()
		}
		outputDir = filepath.Join(base, "kinclaw", "screens")
	}
	outputDir = expandHome(outputDir)
	return &screenSkill{allowed: allowed, outputDir: outputDir}
}

func (s *screenSkill) Name() string { return "screen" }

func (s *screenSkill) Description() string {
	return "Capture macOS screen, enumerate UI graph, OCR text, sample pixels, " +
		"diff before/after. Actions: " +
		"screenshot (full display, or sub-region region=x,y,w,h, or single window " +
		"bundle_id=) | screenshot_app (composite all windows of bundle_id) | " +
		"list_displays | list_windows (visible window enumeration) | " +
		"list_apps (apps with visible windows) | " +
		"ocr (concatenated text) | ocr_regions (JSON with bbox + center_x/y for " +
		"direct input.click feed) | diff_screenshots (16×16 heatmap + dirty bbox) | " +
		"color_at_point (sample pixel — RGB hex + rough color name like red/green/gray, " +
		"useful for status indicators / dark-mode detection / spinner-still-spinning) | " +
		"live_stream (mode=start/frame/stop/list — long-lived sckit.Stream gives " +
		"~80-160ms/frame amortized vs 250-400ms per fresh capture; ideal for " +
		"watching UI react to a sequence of clicks). " +
		"Use OCR instead of vision-LLM when you only need the literal text — local, " +
		"~50-200ms, free. Requires Screen Recording permission (macOS TCC)."
}

func (s *screenSkill) ToolDef() json.RawMessage {
	return MakeToolDef("screen", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "screenshot (default) | list_displays | list_windows | list_apps | screenshot_app | ocr | ocr_regions | diff_screenshots | color_at_point | live_stream",
			},
			"mode": {
				"type":        "string",
				"description": "For action=live_stream: start | frame | stop | list. Default: start.",
			},
			"id": {
				"type":        "string",
				"description": "For action=live_stream mode=frame|stop: stream id returned by start (e.g. 'ls-0001').",
			},
			"fps": {
				"type":        "integer",
				"description": "For action=live_stream mode=start: frame rate (default 30). Higher fps = smoother captures but more buffer pressure.",
			},
			"on_screen": {
				"type":        "string",
				"description": "For action=list_windows: 'true' (default) only on-screen windows, 'false' includes minimized.",
			},
			"name_contains": {
				"type":        "string",
				"description": "For action=list_apps: filter apps whose name contains this substring (case-insensitive).",
			},
			"x": {"type": "number", "description": "For color_at_point: X coordinate (display-local)"},
			"y": {"type": "number", "description": "For color_at_point: Y coordinate (display-local)"},
			"display_id": {
				"type":        "string",
				"description": "Optional CGDirectDisplayID from list_displays. Default: main display.",
			},
			"output_path": {
				"type":        "string",
				"description": "Optional explicit PNG path for screenshot. Default: timestamped file in cache dir.",
			},
			"path": {
				"type":        "string",
				"description": "For action=ocr / ocr_regions: path to an existing PNG/JPEG. Omit to OCR a fresh screen capture.",
			},
			"region": {
				"type":        "string",
				"description": "For action=screenshot or diff_screenshots: 'x,y,w,h' in display-local px. Captures only that sub-rectangle. 5-20× token reduction vs full-display capture.",
			},
			"bundle_id": {
				"type":        "string",
				"description": "For action=screenshot: capture a specific app's window (e.g. 'com.apple.Safari'). Pair with title_contains= when the app has multiple windows.",
			},
			"title_contains": {
				"type":        "string",
				"description": "For action=screenshot bundle_id=...: pick the window whose title contains this substring (case-insensitive). When multiple windows match the bundle and you want a specific one.",
			},
			"wait_ms": {
				"type":        "integer",
				"description": "For action=diff_screenshots: ms to wait between before and after captures. Caller is expected to perform the action externally during this window. Default: 0 (caller must split into two calls).",
			},
			"threshold": {
				"type":        "integer",
				"description": "For action=diff_screenshots: per-cell mean-abs-delta threshold (0..255) above which a cell is flagged dirty. Default 8 — sensitive enough to catch text changes, insensitive enough to ignore antialiasing.",
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
	case "list_displays":
		return s.listDisplays(ctx)
	case "screenshot":
		// region= and bundle_id= take screenshot down a more
		// specific capture path (sub-rect or single window). Without
		// them we keep the original full-display behavior so existing
		// callers see no diff.
		if strings.TrimSpace(params["region"]) != "" {
			return s.screenshotRegion(ctx, params)
		}
		if strings.TrimSpace(params["bundle_id"]) != "" {
			return s.screenshotWindow(ctx, params)
		}
		return s.screenshot(ctx, params)
	case "ocr":
		return s.ocr(ctx, params)
	case "ocr_regions":
		return s.ocrRegions(ctx, params)
	case "diff_screenshots":
		return s.diffScreenshots(ctx, params)
	case "list_windows":
		return s.listWindows(ctx, params)
	case "list_apps":
		return s.listApps(ctx, params)
	case "screenshot_app":
		return s.screenshotApp(ctx, params)
	case "color_at_point":
		return s.colorAtPoint(ctx, params)
	case "live_stream":
		return s.liveStream(ctx, params)
	default:
		return "", fmt.Errorf("unknown action %q (expected: screenshot | list_displays | list_windows | list_apps | screenshot_app | ocr | ocr_regions | diff_screenshots | color_at_point | live_stream)", action)
	}
}

// ocr runs sckit-go's Vision-framework wrapper. Two input modes:
//
//   path=<file>    OCR an existing PNG/JPEG/TIFF on disk
//   (no path)      Capture the screen ENTIRELY IN MEMORY (no disk
//                  round-trip), encode to PNG bytes, OCR
//
// The fresh-capture path was redesigned for v1.7.1 — earlier code
// piggy-backed on screenshot() which writes PNG to ~/Library/Caches
// then re-reads it from disk. That worked but burned disk IO on every
// OCR call (~ms latency, but more importantly ~5MB/file × N calls).
// Capturing in memory drops it to a single in-process buffer.
//
// Returns text regions as a compact human-readable list — LLM can
// re-parse if it needs structured access. For machine-friendly
// access the underlying sckit.OCR API is the way; this skill is
// shaped for LLM consumption.
func (s *screenSkill) ocr(ctx context.Context, params map[string]string) (string, error) {
	imgBytes, label, err := s.imageForOCR(ctx, params)
	if err != nil {
		return "", err
	}

	regions, err := sckit.OCR(imgBytes)
	if err != nil {
		return "", fmt.Errorf("sckit.OCR: %w", err)
	}
	if len(regions) == 0 {
		return fmt.Sprintf("OCR on %s: no text recognized", label), nil
	}
	var b []byte
	b = append(b, fmt.Sprintf("OCR on %s — %d text region(s):\n", label, len(regions))...)
	for _, r := range regions {
		b = append(b, fmt.Sprintf("  %q  at (%d,%d) size %dx%d  conf=%.2f\n",
			r.Text, r.X, r.Y, r.W, r.H, r.Confidence)...)
	}
	return string(b), nil
}

// imageForOCR returns the PNG bytes the OCR call should consume,
// plus a label for the human-readable result line. Either reads a
// file from `path=<file>` or captures the screen in-memory (no
// disk write — ocr does NOT call the screenshot action and does
// NOT produce a file as side effect).
func (s *screenSkill) imageForOCR(ctx context.Context, params map[string]string) ([]byte, string, error) {
	if p := params["path"]; p != "" {
		p = expandHome(p)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, "", fmt.Errorf("read %s: %w", p, err)
		}
		return b, p, nil
	}

	// Fresh capture, in-memory only. Same display-pick logic as
	// screenshot() so display_id behaves the same across actions.
	target, err := s.pickDisplay(ctx, params)
	if err != nil {
		return nil, "", err
	}
	img, err := sckit.Capture(ctx, target)
	if err != nil {
		return nil, "", fmt.Errorf("sckit.Capture: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", fmt.Errorf("png encode in-memory: %w", err)
	}
	bounds := img.Bounds()
	label := fmt.Sprintf("<in-memory capture display=%d %dx%d>",
		target.ID, bounds.Dx(), bounds.Dy())
	return buf.Bytes(), label, nil
}

// pickDisplay resolves the target display from params (display_id
// optional; default = main = displays[0]). Shared by screenshot()
// and the OCR fresh-capture path.
func (s *screenSkill) pickDisplay(ctx context.Context, params map[string]string) (sckit.Display, error) {
	ds, err := sckit.ListDisplays(ctx)
	if err != nil {
		return sckit.Display{}, fmt.Errorf("sckit.ListDisplays: %w", err)
	}
	if len(ds) == 0 {
		return sckit.Display{}, fmt.Errorf("no displays available")
	}
	target := ds[0]
	if want := params["display_id"]; want != "" {
		var found bool
		for _, d := range ds {
			if fmt.Sprintf("%d", d.ID) == want {
				target = d
				found = true
				break
			}
		}
		if !found {
			return sckit.Display{}, fmt.Errorf("display_id %q not found", want)
		}
	}
	return target, nil
}

func (s *screenSkill) listDisplays(ctx context.Context) (string, error) {
	ds, err := sckit.ListDisplays(ctx)
	if err != nil {
		return "", fmt.Errorf("sckit.ListDisplays: %w", err)
	}
	if len(ds) == 0 {
		return "no displays found", nil
	}
	out := fmt.Sprintf("%d display(s):\n", len(ds))
	for i, d := range ds {
		tag := ""
		if i == 0 {
			tag = "  (main)"
		}
		out += fmt.Sprintf("  id=%d  %dx%d  origin=(%d,%d)%s\n",
			d.ID, d.Width, d.Height, d.X, d.Y, tag)
	}
	return out, nil
}

func (s *screenSkill) screenshot(ctx context.Context, params map[string]string) (string, error) {
	target, err := s.pickDisplay(ctx, params)
	if err != nil {
		return "", err
	}

	img, err := sckit.Capture(ctx, target)
	if err != nil {
		return "", fmt.Errorf("sckit.Capture: %w", err)
	}

	outPath := params["output_path"]
	if outPath == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		outPath = filepath.Join(s.outputDir, fmt.Sprintf("screen-%s.png", ts))
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", fmt.Errorf("png encode: %w", err)
	}

	bounds := img.Bounds()
	// Lead with the path on its own line so LLMs that like to summarize
	// can't accidentally drop it. The `image://` marker line is stripped
	// by the registry's extractImageMarkers and rerouted into the
	// ToolResult.Images list — that's how vision-capable brains end up
	// with the actual pixel data inlined into their next API call.
	// Brains without vision support never see the marker either way:
	// the line is removed before the model gets the tool result.
	return fmt.Sprintf("path: %s\nimage://%s\ndimensions: %dx%d\ndisplay_id: %d",
		outPath, outPath, bounds.Dx(), bounds.Dy(), target.ID), nil
}
