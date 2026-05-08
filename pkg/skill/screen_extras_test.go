//go:build darwin

package skill

import (
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

// Diff-grid tests have moved to sckit-go's diff_test.go (kit owns
// the algorithm now). The kinclaw `screen diff_screenshots` verb
// is just a thin wrapper around `sckit.DiffImages` — its
// integration is covered by the kit's own tests + the live
// integration runner (deferred until macOS CI lands).
