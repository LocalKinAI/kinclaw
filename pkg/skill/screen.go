//go:build darwin

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
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
	return "Capture a screenshot, list displays, or run on-device OCR via Vision " +
		"framework. 'screenshot' saves a PNG (returns path); 'list_displays' " +
		"enumerates options; 'ocr' returns recognized text + bounding boxes from " +
		"either the current screen (omit `path`) or a specified PNG/JPEG file. " +
		"Use OCR instead of vision-LLM when you only need the literal text — it's " +
		"local, ~50-200ms, free. Requires Screen Recording permission (macOS TCC)."
}

func (s *screenSkill) ToolDef() json.RawMessage {
	return MakeToolDef("screen", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "screenshot (default) | list_displays | ocr",
			},
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
				"description": "For action=ocr: path to an existing PNG/JPEG. Omit to OCR a fresh screen capture.",
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
		return s.screenshot(ctx, params)
	case "ocr":
		return s.ocr(ctx, params)
	default:
		return "", fmt.Errorf("unknown action %q (expected: screenshot, list_displays, ocr)", action)
	}
}

// ocr runs sckit-go's Vision-framework wrapper on either an existing
// image file (path=...) or a fresh screen capture (path omitted).
// Returns text regions as a compact human-readable list — LLM can
// re-parse if it needs structured access. For machine-friendly access
// the underlying sckit.OCR API is the way; this skill is shaped for
// LLM consumption.
func (s *screenSkill) ocr(ctx context.Context, params map[string]string) (string, error) {
	var imgBytes []byte
	var label string
	if p := params["path"]; p != "" {
		p = expandHome(p)
		b, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", p, err)
		}
		imgBytes = b
		label = p
	} else {
		// Capture the screen first, then OCR. We keep the screenshot
		// path in the result so the soul has a reference if it later
		// wants to ALSO show it to a vision LLM.
		shotResult, err := s.screenshot(ctx, params)
		if err != nil {
			return "", fmt.Errorf("screenshot for ocr: %w", err)
		}
		// shotResult's first line is the file path (see screenshot()).
		shotPath := shotResult
		if i := indexOfFirstNewline(shotResult); i > 0 {
			shotPath = shotResult[:i]
		}
		shotPath = expandHome(shotPath)
		b, err := os.ReadFile(shotPath)
		if err != nil {
			return "", fmt.Errorf("read fresh screenshot %s: %w", shotPath, err)
		}
		imgBytes = b
		label = shotPath
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

func indexOfFirstNewline(s string) int {
	for i, c := range s {
		if c == '\n' {
			return i
		}
	}
	return -1
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
	ds, err := sckit.ListDisplays(ctx)
	if err != nil {
		return "", fmt.Errorf("sckit.ListDisplays: %w", err)
	}
	if len(ds) == 0 {
		return "", fmt.Errorf("no displays available")
	}

	// sckit-go returns the main display first by convention.
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
			return "", fmt.Errorf("display_id %q not found", want)
		}
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
