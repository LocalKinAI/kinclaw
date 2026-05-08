//go:build darwin

package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
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

	// Compare. Resize-equalize would be needed if region changed; we
	// assume same target between captures and bail if dims differ.
	bb := before.Bounds()
	ab := after.Bounds()
	if bb != ab {
		return "", fmt.Errorf("diff_screenshots: before/after dims differ (%v vs %v)", bb, ab)
	}
	const cells = 16
	grid := computeDiffGrid(before, after, cells)
	threshold := atoiDefault(params["threshold"], 8) // mean delta out of 255
	dirty := flagDirtyCells(grid, float64(threshold))

	var sb strings.Builder
	fmt.Fprintf(&sb, "diff_screenshots: %dx%d grid, threshold=%d/255, dirty=%d cell(s)\n",
		cells, cells, threshold, countDirty(dirty))
	for r := 0; r < cells; r++ {
		for c := 0; c < cells; c++ {
			if dirty[r][c] {
				sb.WriteByte('#')
			} else if grid[r][c] >= float64(threshold)/2 {
				sb.WriteByte('.')
			} else {
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	if bbox, ok := dirtyBoundingBox(dirty, bb, cells); ok {
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

func computeDiffGrid(a, b image.Image, cells int) [][]float64 {
	bounds := a.Bounds()
	cw := bounds.Dx() / cells
	ch := bounds.Dy() / cells
	grid := make([][]float64, cells)
	for r := 0; r < cells; r++ {
		grid[r] = make([]float64, cells)
		for c := 0; c < cells; c++ {
			x0 := bounds.Min.X + c*cw
			y0 := bounds.Min.Y + r*ch
			x1 := x0 + cw
			y1 := y0 + ch
			if c == cells-1 {
				x1 = bounds.Max.X
			}
			if r == cells-1 {
				y1 = bounds.Max.Y
			}
			grid[r][c] = meanAbsDelta(a, b, x0, y0, x1, y1)
		}
	}
	return grid
}

// meanAbsDelta samples a coarse subset of pixels in the rect (every
// 4th pixel in each axis) and returns mean abs delta of grayscale
// intensity (0..255). Keeps diff fast on retina-resolution captures.
func meanAbsDelta(a, b image.Image, x0, y0, x1, y1 int) float64 {
	var sum, n int
	const stride = 4
	for y := y0; y < y1; y += stride {
		for x := x0; x < x1; x += stride {
			ar, ag, ab_, _ := a.At(x, y).RGBA()
			br, bg, bb_, _ := b.At(x, y).RGBA()
			ai := int((ar + ag + ab_) / 3 / 257) // / 257 ≈ 16-bit → 8-bit
			bi := int((br + bg + bb_) / 3 / 257)
			d := ai - bi
			if d < 0 {
				d = -d
			}
			sum += d
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

func flagDirtyCells(grid [][]float64, threshold float64) [][]bool {
	cells := len(grid)
	out := make([][]bool, cells)
	for r := 0; r < cells; r++ {
		out[r] = make([]bool, cells)
		for c := 0; c < cells; c++ {
			if grid[r][c] >= threshold {
				out[r][c] = true
			}
		}
	}
	return out
}

func countDirty(g [][]bool) int {
	n := 0
	for _, row := range g {
		for _, v := range row {
			if v {
				n++
			}
		}
	}
	return n
}

func dirtyBoundingBox(dirty [][]bool, full image.Rectangle, cells int) (image.Rectangle, bool) {
	minR, minC := cells, cells
	maxR, maxC := -1, -1
	for r := 0; r < cells; r++ {
		for c := 0; c < cells; c++ {
			if dirty[r][c] {
				if r < minR {
					minR = r
				}
				if c < minC {
					minC = c
				}
				if r > maxR {
					maxR = r
				}
				if c > maxC {
					maxC = c
				}
			}
		}
	}
	if maxR < 0 {
		return image.Rectangle{}, false
	}
	cw := full.Dx() / cells
	ch := full.Dy() / cells
	x0 := full.Min.X + minC*cw
	y0 := full.Min.Y + minR*ch
	x1 := full.Min.X + (maxC+1)*cw
	y1 := full.Min.Y + (maxR+1)*ch
	if x1 > full.Max.X {
		x1 = full.Max.X
	}
	if y1 > full.Max.Y {
		y1 = full.Max.Y
	}
	return image.Rect(x0, y0, x1, y1), true
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

// imageDistance is a sanity helper used by tests — returns the L2
// pixel intensity distance between two images of equal size, divided
// by pixel count. Not used in production diff (cell-grid is faster).
func imageDistance(a, b image.Image) float64 {
	ab := a.Bounds()
	bb := b.Bounds()
	if ab != bb {
		return math.Inf(1)
	}
	var sum, n float64
	const stride = 4
	for y := ab.Min.Y; y < ab.Max.Y; y += stride {
		for x := ab.Min.X; x < ab.Max.X; x += stride {
			ar, ag, ab_, _ := a.At(x, y).RGBA()
			br, bg, bb_, _ := b.At(x, y).RGBA()
			dr := float64(ar) - float64(br)
			dg := float64(ag) - float64(bg)
			db := float64(ab_) - float64(bb_)
			sum += dr*dr + dg*dg + db*db
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return math.Sqrt(sum / n)
}

// helper byte buffer for image encode (used by tests).
var _ = bytes.Buffer{}
