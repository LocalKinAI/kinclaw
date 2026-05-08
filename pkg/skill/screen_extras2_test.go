//go:build darwin

package skill

import "testing"

// TestColorBucketName covers the achromatic + chromatic bucketing
// used by `screen color_at_point`. Behavior must match the docs:
// achromatic = all-three-within-12, chromatic = HSV hue ranges.
func TestColorBucketName(t *testing.T) {
	cases := []struct {
		r, g, b uint8
		want    string
	}{
		// Achromatic — exact equal, varying brightness.
		{0, 0, 0, "black"},
		{50, 50, 50, "dark gray"},
		{128, 128, 128, "gray"},
		{200, 200, 200, "light gray"},
		{255, 255, 255, "white"},
		// Achromatic with small drift (within 12 → still gray).
		{100, 105, 110, "gray"},
		// Chromatic — primary + secondary colors.
		{255, 0, 0, "red"},
		{0, 255, 0, "green"},
		{0, 0, 255, "blue"},
		{255, 255, 0, "yellow"},
		{0, 255, 255, "teal"},
		// Both #FF00FF and #800080 share hue 300°; the bucketing
		// resolves both to "purple" (extended range to 305° because
		// the everyday name for #800080 is "purple"). The "magenta"
		// label kicks in past 305°, e.g. #FF0080 (hue ≈ 330°).
		{255, 0, 255, "purple"},
		{255, 165, 0, "orange"},
		{128, 0, 128, "purple"},
		{255, 0, 128, "magenta"}, // hue ≈ 330°

	}
	for _, c := range cases {
		got := colorBucketName(c.r, c.g, c.b)
		if got != c.want {
			t.Errorf("colorBucketName(%d,%d,%d) = %q, want %q",
				c.r, c.g, c.b, got, c.want)
		}
	}
}
