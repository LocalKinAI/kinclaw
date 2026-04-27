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
	return "Capture a screenshot of the macOS display, or list attached displays. " +
		"Use 'screenshot' to save a PNG (returns path); use 'list_displays' to " +
		"enumerate options. Call this before any UI action to see the current " +
		"state. Requires Screen Recording permission (macOS TCC)."
}

func (s *screenSkill) ToolDef() json.RawMessage {
	return MakeToolDef("screen", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "screenshot (default) or list_displays",
			},
			"display_id": {
				"type":        "string",
				"description": "Optional CGDirectDisplayID from list_displays. Default: main display.",
			},
			"output_path": {
				"type":        "string",
				"description": "Optional explicit PNG path. Default: timestamped file in cache dir.",
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
	default:
		return "", fmt.Errorf("unknown action %q (expected: screenshot, list_displays)", action)
	}
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
