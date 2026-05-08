//go:build darwin

package skill

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// TestParseIntPart covers the region-parsing helper. Accepts negative
// (multi-display setups have negative-origin secondary displays).
func TestParseIntPart(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"0", 0, false},
		{"100", 100, false},
		{"-100", -100, false},
		{"+50", 50, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1.5", 0, true},
	}
	for _, c := range cases {
		got, err := parseIntPart(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseIntPart(%q): want err, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseIntPart(%q): unexpected err: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseIntPart(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestComputeDiffGrid: identical images produce all-zero grid.
func TestComputeDiffGrid_NoChange(t *testing.T) {
	a := uniformImage(64, 64, color.RGBA{128, 128, 128, 255})
	b := uniformImage(64, 64, color.RGBA{128, 128, 128, 255})
	grid := computeDiffGrid(a, b, 8)
	for r, row := range grid {
		for c, v := range row {
			if v != 0 {
				t.Errorf("identical images: grid[%d][%d] = %.2f, want 0", r, c, v)
			}
		}
	}
}

// TestComputeDiffGrid_Change: a single dirty quadrant is detected.
func TestComputeDiffGrid_Change(t *testing.T) {
	a := uniformImage(64, 64, color.RGBA{0, 0, 0, 255})
	b := uniformImage(64, 64, color.RGBA{0, 0, 0, 255})
	// Paint top-right quadrant of b white.
	for y := 0; y < 32; y++ {
		for x := 32; x < 64; x++ {
			b.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	grid := computeDiffGrid(a, b, 8)

	// Cells (rows 0..3, cols 4..7) should be ~white delta = 255.
	dirtyCount := 0
	for r := 0; r < 4; r++ {
		for c := 4; c < 8; c++ {
			if grid[r][c] > 200 {
				dirtyCount++
			}
		}
	}
	if dirtyCount < 12 { // 4×4 = 16 cells, allow some boundary noise
		t.Errorf("expected most cells in top-right quadrant to be dirty, got %d/16", dirtyCount)
	}
	// Bottom-left should still be quiet.
	for r := 4; r < 8; r++ {
		for c := 0; c < 4; c++ {
			if grid[r][c] > 8 {
				t.Errorf("bottom-left should be quiet, got grid[%d][%d] = %.2f",
					r, c, grid[r][c])
			}
		}
	}
}

// TestDirtyBoundingBox: returns the union rectangle of dirty cells.
func TestDirtyBoundingBox(t *testing.T) {
	cells := 8
	dirty := make([][]bool, cells)
	for i := range dirty {
		dirty[i] = make([]bool, cells)
	}
	// Mark a 2x2 block at rows 2-3, cols 3-4.
	for r := 2; r <= 3; r++ {
		for c := 3; c <= 4; c++ {
			dirty[r][c] = true
		}
	}
	full := image.Rect(0, 0, 800, 800)
	bbox, ok := dirtyBoundingBox(dirty, full, cells)
	if !ok {
		t.Fatal("expected ok=true for non-empty dirty set")
	}
	// 800/8 = 100 px per cell. Rows 2-3 → y in [200, 400). Cols 3-4 → x in [300, 500).
	if bbox.Min.X != 300 || bbox.Min.Y != 200 || bbox.Max.X != 500 || bbox.Max.Y != 400 {
		t.Errorf("bbox = %v, want (300,200)-(500,400)", bbox)
	}
}

func TestDirtyBoundingBox_Empty(t *testing.T) {
	cells := 8
	dirty := make([][]bool, cells)
	for i := range dirty {
		dirty[i] = make([]bool, cells)
	}
	_, ok := dirtyBoundingBox(dirty, image.Rect(0, 0, 800, 800), cells)
	if ok {
		t.Error("empty dirty set should report ok=false")
	}
}

// uniformImage creates a w×h RGBA filled with the given color. Used
// for the diff tests so we don't depend on real captures.
func uniformImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// TestSafeFilenameFragment: bundle ids → safe filenames.
func TestSafeFilenameFragment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"com.apple.Safari", "com-apple-Safari"},
		{"com.google.Chrome", "com-google-Chrome"},
		{"a/b\\c:d", "a-b-c-d"},
		{"alphanum_123", "alphanum_123"},
	}
	for _, c := range cases {
		got := safeFilenameFragment(c.in)
		if got != c.want {
			t.Errorf("safeFilenameFragment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMatchSummary covers the smart_click match-count footnote.
func TestMatchSummary(t *testing.T) {
	if s := matchSummary(0); s != "" {
		t.Errorf("matchSummary(0) = %q, want empty", s)
	}
	if s := matchSummary(1); s != "" {
		t.Errorf("matchSummary(1) = %q, want empty", s)
	}
	s := matchSummary(5)
	if !strings.Contains(s, "5 matches") {
		t.Errorf("matchSummary(5) = %q, want to contain '5 matches'", s)
	}
}
