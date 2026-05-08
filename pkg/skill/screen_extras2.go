//go:build darwin

package skill

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LocalKinAI/sckit-go"
)

// This file groups the v1.14 "screen claw 100%" verbs:
//   - list_windows: enumerate visible windows (bundle / title / pid / frame)
//   - list_apps: enumerate apps with at least one on-screen window
//   - screenshot_app: capture all windows of an app composited
//   - color_at_point: sample the pixel color at (x, y)
//
// These four expand screen-claw coverage from "capture displays /
// regions / single-windows" to "discover the visible UI graph + sample
// arbitrary pixels". Together they let an LLM ask "is this app open?",
// "what window is the second-most-relevant for my task?", "is the
// status indicator green or red right now?" — without an AX-tree
// walk or a vision-LLM round-trip.

// list_windows — enumerate currently-visible windows on this Mac.
//
// Returns one line per window with bundle / title / pid / frame.
// Useful at the start of any screenshot-heavy flow as a "what are my
// options" pass before deciding which window to capture.
func (s *screenSkill) listWindows(ctx context.Context, params map[string]string) (string, error) {
	wins, err := sckit.ListWindows(ctx)
	if err != nil {
		return "", fmt.Errorf("ListWindows: %w", err)
	}
	if len(wins) == 0 {
		return "no visible windows", nil
	}
	bundleFilter := strings.TrimSpace(params["bundle_id"])
	titleFilter := strings.ToLower(strings.TrimSpace(params["title_contains"]))
	onScreenOnly := parseBoolParam(params["on_screen"], true)

	var sb strings.Builder
	count := 0
	for _, w := range wins {
		if onScreenOnly && !w.OnScreen {
			continue
		}
		if bundleFilter != "" && w.BundleID != bundleFilter {
			continue
		}
		if titleFilter != "" && !strings.Contains(strings.ToLower(w.Title), titleFilter) {
			continue
		}
		count++
		fmt.Fprintf(&sb,
			"  id=%d  pid=%d  bundle=%s  app=%q  title=%q  frame=(%d,%d %dx%d)  layer=%d  on_screen=%v\n",
			w.ID, w.PID, w.BundleID, w.App, w.Title,
			w.Frame.Min.X, w.Frame.Min.Y, w.Frame.Dx(), w.Frame.Dy(),
			w.Layer, w.OnScreen)
	}
	if count == 0 {
		return fmt.Sprintf("0 windows match (bundle=%q title_contains=%q on_screen_only=%v)",
			bundleFilter, titleFilter, onScreenOnly), nil
	}
	header := fmt.Sprintf("%d window(s) (of %d total visible)\n", count, len(wins))
	return header + sb.String(), nil
}

// list_apps — enumerate apps with at least one visible window.
//
// Cheaper than list_windows when you just want to know "what's running
// that I might want to interact with". One line per app, includes
// the bundle id (the canonical identifier for ui/screen targeting).
func (s *screenSkill) listApps(ctx context.Context, params map[string]string) (string, error) {
	apps, err := sckit.ListApps(ctx)
	if err != nil {
		return "", fmt.Errorf("ListApps: %w", err)
	}
	if len(apps) == 0 {
		return "no apps with visible windows", nil
	}
	bundleFilter := strings.TrimSpace(params["bundle_id"])
	nameFilter := strings.ToLower(strings.TrimSpace(params["name_contains"]))

	var sb strings.Builder
	count := 0
	for _, a := range apps {
		if bundleFilter != "" && appBundleID(&a) != bundleFilter {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(appName(&a)), nameFilter) {
			continue
		}
		count++
		fmt.Fprintf(&sb, "  bundle=%s  name=%q  pid=%d\n",
			appBundleID(&a), appName(&a), appPID(&a))
	}
	if count == 0 {
		return fmt.Sprintf("0 apps match (bundle=%q name_contains=%q)",
			bundleFilter, nameFilter), nil
	}
	return fmt.Sprintf("%d app(s) with visible windows (of %d total):\n%s",
		count, len(apps), sb.String()), nil
}

// screenshot_app — capture all visible windows of a target app
// composited together (sckit handles the layer ordering). Useful
// when an app has multiple coordinated windows (e.g. Numbers with a
// document + the inspector palette) and you want a single image
// showing both.
func (s *screenSkill) screenshotApp(ctx context.Context, params map[string]string) (string, error) {
	bundle := strings.TrimSpace(params["bundle_id"])
	if bundle == "" {
		return "", fmt.Errorf("screenshot_app: bundle_id is required")
	}
	target := sckit.App{BundleID: bundle}
	img, err := sckit.Capture(ctx, target)
	if err != nil {
		return "", fmt.Errorf("sckit.Capture App: %w", err)
	}

	outPath := params["output_path"]
	if outPath == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		safe := safeFilenameFragment(bundle)
		outPath = filepath.Join(s.outputDir, fmt.Sprintf("app-%s-%s.png", safe, ts))
	}
	if err := writePNG(outPath, img); err != nil {
		return "", err
	}
	bounds := img.Bounds()
	return fmt.Sprintf("path: %s\nimage://%s\ndimensions: %dx%d\nbundle_id: %s",
		outPath, outPath, bounds.Dx(), bounds.Dy(), bundle), nil
}

// color_at_point — sample one pixel's color from a fresh full-display
// capture. Returns RGB hex + decimal + "looks like" name (red/green/
// blue/gray buckets) so the model can reason without doing color
// arithmetic itself.
//
// Use case: "is the status badge green (success) or red (error)?",
// "is dark mode on by sampling a known background pixel?", "is the
// loading spinner still spinning by checking if pixel changed?".
func (s *screenSkill) colorAtPoint(ctx context.Context, params map[string]string) (string, error) {
	x, y, err := xy(params)
	if err != nil {
		return "", fmt.Errorf("color_at_point: %w", err)
	}
	display, err := s.pickDisplay(ctx, params)
	if err != nil {
		return "", err
	}
	img, err := sckit.Capture(ctx, display)
	if err != nil {
		return "", fmt.Errorf("Capture: %w", err)
	}
	bounds := img.Bounds()
	ix, iy := int(x), int(y)
	if ix < bounds.Min.X || ix >= bounds.Max.X || iy < bounds.Min.Y || iy >= bounds.Max.Y {
		return "", fmt.Errorf("color_at_point: (%d,%d) outside display bounds (%v)",
			ix, iy, bounds)
	}
	c := img.At(ix, iy)
	r, g, b, _ := c.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	hex := fmt.Sprintf("#%02X%02X%02X", r8, g8, b8)
	name := colorBucketName(uint8(r8), uint8(g8), uint8(b8))
	return fmt.Sprintf(
		"color at (%d,%d) on display=%d: %s (rgb=%d,%d,%d) ≈ %s",
		ix, iy, display.ID, hex, r8, g8, b8, name,
	), nil
}

// colorBucketName classifies an RGB triple into a coarse color label
// (red / orange / yellow / green / teal / blue / purple / magenta /
// black / gray / white) for LLM-friendly reasoning. Not a precise
// HSV palette — just enough to answer "is this badge green or red".
func colorBucketName(r, g, b uint8) string {
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	min := r
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}
	// Achromatic detection: if all three within ~12 of each other,
	// it's a gray.
	if int(max)-int(min) < 12 {
		switch {
		case max < 32:
			return "black"
		case max < 96:
			return "dark gray"
		case max < 192:
			return "gray"
		case max < 240:
			return "light gray"
		default:
			return "white"
		}
	}
	// Chromatic — rough hue bucketing.
	rf, gf, bf := float64(r), float64(g), float64(b)
	maxF, minF := float64(max), float64(min)
	d := maxF - minF
	var h float64
	switch max {
	case r:
		h = 60 * (gf - bf) / d
		if h < 0 {
			h += 360
		}
	case g:
		h = 60*(bf-rf)/d + 120
	default:
		h = 60*(rf-gf)/d + 240
	}
	switch {
	case h < 15 || h >= 345:
		return "red"
	case h < 45:
		return "orange"
	case h < 70:
		return "yellow"
	case h < 165:
		return "green"
	case h < 195:
		return "teal"
	case h < 255:
		return "blue"
	case h < 305:
		// "purple" extends to 305° to capture CSS #800080 (h=300°)
		// which everyone calls purple, even though the strict
		// magenta range starts around 290°.
		return "purple"
	default:
		return "magenta"
	}
}

// _ ensures the color package is referenced (used by the bucketing
// helper internally — Go's import-detection picks up the alias).
var _ color.Color = color.RGBA{}

// appBundleID / appName / appPID isolate sckit.App field-name churn.
// As of sckit-go v0.x the fields are BundleID + Name + PID — adapter
// pattern same as windowMatchesBundle.
func appBundleID(a *sckit.App) string { return a.BundleID }
func appName(a *sckit.App) string     { return a.Name }
func appPID(a *sckit.App) int32       { return a.PID }
