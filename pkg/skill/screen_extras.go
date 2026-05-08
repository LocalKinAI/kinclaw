//go:build darwin

package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LocalKinAI/sckit-go"
)

// This file groups the v1.13+ "screen claw" verbs that go beyond the
// basic full-display screenshot: region/window capture, OCR with
// bounding-box output, and pre/post diff for action verification.
//
// All four new verbs are layered on top of existing sckit-go
// primitives — no kit-level changes required.

// ---------------------------------------------------------------------
// region — capture a sub-rectangle of a display.
//
// sckit.Region{Display, Bounds} maps directly to SCContentFilter's
// region capture. Use case: take a snapshot of just the calculator's
// display area, just the chat composer, just the modal dialog —
// instead of capturing the full 5120×2880 retina display and asking
// the model to describe it. 5-20x token reduction on average.
// ---------------------------------------------------------------------

func (s *screenSkill) screenshotRegion(ctx context.Context, params map[string]string) (string, error) {
	// region="x,y,w,h" — origin top-left, size in display-local px.
	regionStr := strings.TrimSpace(params["region"])
	if regionStr == "" {
		return "", fmt.Errorf("region: param `region=x,y,w,h` is required")
	}
	parts := strings.Split(regionStr, ",")
	if len(parts) != 4 {
		return "", fmt.Errorf("region: expected x,y,w,h (got %q)", regionStr)
	}
	var nums [4]int
	for i, p := range parts {
		n, err := parseIntPart(strings.TrimSpace(p))
		if err != nil {
			return "", fmt.Errorf("region: part %d %q: %w", i+1, p, err)
		}
		nums[i] = n
	}
	x, y, w, h := nums[0], nums[1], nums[2], nums[3]
	if w <= 0 || h <= 0 {
		return "", fmt.Errorf("region: width/height must be positive (got %dx%d)", w, h)
	}

	display, err := s.pickDisplay(ctx, params)
	if err != nil {
		return "", err
	}
	target := sckit.Region{
		Display: display,
		Bounds:  image.Rect(x, y, x+w, y+h),
	}

	img, err := sckit.Capture(ctx, target)
	if err != nil {
		return "", fmt.Errorf("sckit.Capture region: %w", err)
	}

	outPath := params["output_path"]
	if outPath == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		outPath = filepath.Join(s.outputDir, fmt.Sprintf("region-%s.png", ts))
	}
	if err := writePNG(outPath, img); err != nil {
		return "", err
	}
	bounds := img.Bounds()
	return fmt.Sprintf("path: %s\nimage://%s\ndimensions: %dx%d\ndisplay_id: %d\nregion: (%d,%d) %dx%d",
		outPath, outPath, bounds.Dx(), bounds.Dy(), display.ID, x, y, w, h), nil
}

// ---------------------------------------------------------------------
// window — capture a specific app's window by bundle id (or first
// window of bundle).
//
// sckit-go's ListWindows returns every on-screen window with its
// owning app's bundle id. Filter by bundle and pick the first match
// (or the one whose title contains a hint, if title_contains given).
// ---------------------------------------------------------------------

func (s *screenSkill) screenshotWindow(ctx context.Context, params map[string]string) (string, error) {
	bundle := strings.TrimSpace(params["bundle_id"])
	if bundle == "" {
		return "", fmt.Errorf("window: bundle_id is required (e.g. com.apple.Safari)")
	}
	wins, err := sckit.ListWindows(ctx)
	if err != nil {
		return "", fmt.Errorf("ListWindows: %w", err)
	}
	titleHint := strings.ToLower(params["title_contains"])

	var match *sckit.Window
	for i := range wins {
		w := &wins[i]
		// Prefer fields that exist on Window — sckit-go exposes Owner
		// (app bundle id) and Title; some builds use OwnerBundleID.
		ownerOK := windowMatchesBundle(w, bundle)
		if !ownerOK {
			continue
		}
		if titleHint != "" && !strings.Contains(strings.ToLower(windowTitle(w)), titleHint) {
			continue
		}
		match = w
		break
	}
	if match == nil {
		return "", fmt.Errorf("window: no window found for bundle=%q title_contains=%q",
			bundle, titleHint)
	}

	img, err := sckit.Capture(ctx, *match)
	if err != nil {
		return "", fmt.Errorf("sckit.Capture window: %w", err)
	}

	outPath := params["output_path"]
	if outPath == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		safe := safeFilenameFragment(bundle)
		outPath = filepath.Join(s.outputDir, fmt.Sprintf("window-%s-%s.png", safe, ts))
	}
	if err := writePNG(outPath, img); err != nil {
		return "", err
	}
	bounds := img.Bounds()
	return fmt.Sprintf("path: %s\nimage://%s\ndimensions: %dx%d\nwindow_title: %q\nbundle_id: %s",
		outPath, outPath, bounds.Dx(), bounds.Dy(), windowTitle(match), bundle), nil
}

// windowMatchesBundle / windowTitle isolate the sckit-go field-name
// surface so kit version churn only touches one place. As of
// sckit-go v0.x: BundleID + Title.
func windowMatchesBundle(w *sckit.Window, bundle string) bool {
	return w.BundleID == bundle
}

func windowTitle(w *sckit.Window) string { return w.Title }

// ---------------------------------------------------------------------
// ocr_regions — OCR a screenshot/screen and return text WITH bounding
// boxes (instead of just concatenated text).
//
// The existing `ocr` verb returns a flat human-readable summary. The
// new `ocr_regions` returns JSON-shaped lines so the model can parse
// them directly + feed (x,y) into input.click without a separate
// "find this text" round-trip.
// ---------------------------------------------------------------------

func (s *screenSkill) ocrRegions(ctx context.Context, params map[string]string) (string, error) {
	imgBytes, label, err := s.imageForOCR(ctx, params)
	if err != nil {
		return "", err
	}
	regions, err := sckit.OCR(imgBytes)
	if err != nil {
		return "", fmt.Errorf("sckit.OCR: %w", err)
	}
	if len(regions) == 0 {
		return fmt.Sprintf("ocr_regions on %s: no text recognized", label), nil
	}
	type regionOut struct {
		Text       string  `json:"text"`
		X          int     `json:"x"`
		Y          int     `json:"y"`
		W          int     `json:"w"`
		H          int     `json:"h"`
		CenterX    int     `json:"center_x"`
		CenterY    int     `json:"center_y"`
		Confidence float64 `json:"confidence"`
	}
	out := make([]regionOut, 0, len(regions))
	for _, r := range regions {
		out = append(out, regionOut{
			Text:       r.Text,
			X:          r.X,
			Y:          r.Y,
			W:          r.W,
			H:          r.H,
			CenterX:    r.X + r.W/2,
			CenterY:    r.Y + r.H/2,
			Confidence: r.Confidence,
		})
	}
	blob, _ := json.MarshalIndent(out, "", "  ")
	return fmt.Sprintf("ocr_regions on %s — %d region(s):\n%s",
		label, len(regions), string(blob)), nil
}

// ---------------------------------------------------------------------
// diff_screenshots — capture before, optionally do an action, capture
// after, return a structural diff (bbox of changed regions + change
// magnitude). Token-cheap alternative to "model compares two
// screenshots" for action verification.
//
// Implementation: divide both images into a 16x16 grid, compute mean
// abs delta of pixel intensity per cell, report cells that crossed
// a threshold. Output is a textual heatmap + bbox of dirty area.
// ---------------------------------------------------------------------

func (s *screenSkill) diffScreenshots(ctx context.Context, params map[string]string) (string, error) {
	before, err := s.captureForDiff(ctx, params)
	if err != nil {
		return "", fmt.Errorf("diff_screenshots: capture before: %w", err)
	}

	// Optional inline action between snapshots — chain via input click
	// or hotkey by passing click_x/click_y or hotkey_key/hotkey_mods.
	if params["click_x"] != "" || params["click_y"] != "" {
		// We don't have a direct handle to the input skill here; the
		// model is expected to call `input click` separately and then
		// invoke this verb in two steps. The click-during-diff plumbing
		// is left for a follow-up — keeping this verb input-skill
		// independent makes the cross-claw boundary clean.
		return "", fmt.Errorf("diff_screenshots: inline click_x/click_y not supported — split into `input click` then `screen diff_screenshots wait_ms=N`")
	}
	waitMs := atoiDefault(params["wait_ms"], 0)
	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	after, err := s.captureForDiff(ctx, params)
	if err != nil {
		return "", fmt.Errorf("diff_screenshots: capture after: %w", err)
	}

	// Delegate the grid math to sckit-go's DiffImages helper. v0.3+
	// gives us DiffGrid with Dirty / BoundingBox / Render methods,
	// so this verb stays a thin LLM-shape wrapper around the kit
	// primitive instead of re-implementing pixel comparison.
	const cells = 16
	threshold := float64(atoiDefault(params["threshold"], 8))
	grid, err := sckit.DiffImages(before, after, cells, cells)
	if err != nil {
		return "", fmt.Errorf("diff_screenshots: %w", err)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "diff_screenshots: %dx%d grid, threshold=%.0f/255, dirty=%d cell(s)\n",
		grid.Rows, grid.Cols, threshold, grid.Dirty(threshold))
	sb.WriteString(grid.Render(threshold))
	if bbox, ok := grid.BoundingBox(threshold); ok {
		fmt.Fprintf(&sb, "dirty_bbox: x=%d y=%d w=%d h=%d (display-local px)\n",
			bbox.Min.X, bbox.Min.Y, bbox.Dx(), bbox.Dy())
	} else {
		sb.WriteString("dirty_bbox: (no change above threshold)\n")
	}
	return sb.String(), nil
}

func (s *screenSkill) captureForDiff(ctx context.Context, params map[string]string) (image.Image, error) {
	// Same target picker used by region/screenshot. If region= is set,
	// we capture that region; else full display.
	display, err := s.pickDisplay(ctx, params)
	if err != nil {
		return nil, err
	}
	if regionStr := strings.TrimSpace(params["region"]); regionStr != "" {
		parts := strings.Split(regionStr, ",")
		if len(parts) != 4 {
			return nil, fmt.Errorf("region: expected x,y,w,h")
		}
		nums := [4]int{}
		for i, p := range parts {
			n, err := parseIntPart(strings.TrimSpace(p))
			if err != nil {
				return nil, err
			}
			nums[i] = n
		}
		x, y, w, h := nums[0], nums[1], nums[2], nums[3]
		return sckit.Capture(ctx, sckit.Region{Display: display, Bounds: image.Rect(x, y, x+w, y+h)})
	}
	return sckit.Capture(ctx, display)
}

// ---------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("png encode: %w", err)
	}
	return nil
}

func parseIntPart(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	// Accept negative values for X/Y (multi-display setups have
	// negative origins on secondary displays).
	var n int
	for i, c := range s {
		if i == 0 && (c == '-' || c == '+') {
			continue
		}
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("bad digit %q", c)
		}
	}
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, err
	}
	return n, nil
}

func safeFilenameFragment(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-' || c == '_':
			out = append(out, c)
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}

// helper byte buffer for image encode (used by tests).
var _ = bytes.Buffer{}
