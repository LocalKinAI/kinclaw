package harvest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Exact / star at file level
		{"SKILL.md", "SKILL.md", true},
		{"*.md", "SKILL.md", true},
		{"*.md", "skill.txt", false},
		{"skills/*", "skills/foo", true},
		{"skills/*", "skills/foo/bar", false},
		{"skills/*.md", "skills/foo.md", true},
		{"skills/*.md", "skills/sub/foo.md", false},

		// ** semantics
		{"**/SKILL.md", "SKILL.md", true},
		{"**/SKILL.md", "skills/SKILL.md", true},
		{"**/SKILL.md", "skills/sub/deep/SKILL.md", true},
		{"**/SKILL.md", "skills/foo.md", false},

		{"skills/**/SKILL.md", "skills/SKILL.md", true},
		{"skills/**/SKILL.md", "skills/foo/SKILL.md", true},
		{"skills/**/SKILL.md", "skills/foo/bar/SKILL.md", true},
		{"skills/**/SKILL.md", "other/SKILL.md", false},

		{"skills/**/*.md", "skills/foo.md", true},
		{"skills/**/*.md", "skills/sub/foo.md", true},
		{"skills/**/*.md", "skills/sub/foo.txt", false},

		// trailing **
		{"skills/**", "skills/a", true},
		{"skills/**", "skills/a/b/c", true},
		{"skills/**", "other/a", false},
	}
	for _, tc := range cases {
		got := matchGlob(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestGlobFiles(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"a/SKILL.md",
		"a/run.py",
		"a/sub/SKILL.md",
		"b/foo.md",
		"b/SKILL.md",
		".git/HEAD",        // must be skipped
		"node_modules/x.md", // must be skipped
	} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	matches, err := globFiles(root, []string{"**/SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Errorf("expected 3 SKILL.md matches, got %d: %v", len(matches), matches)
	}
}

func TestLicenseAllowed(t *testing.T) {
	cases := []struct {
		license string
		allow   []string
		want    bool
	}{
		{"MIT", []string{"MIT", "Apache-2.0"}, true},
		{"GPL-3.0", []string{"MIT", "Apache-2.0"}, false},
		{"", []string{"MIT"}, false},
		{"", []string{"*"}, true},                     // wildcard accepts anything incl. unknown
		{"GPL-3.0", []string{"*"}, true},
		{"MIT", nil, true},                            // default allowlist accepts MIT
		{"GPL-3.0", nil, false},                       // default allowlist refuses GPL
	}
	for _, tc := range cases {
		got := LicenseAllowed(tc.license, tc.allow)
		if got != tc.want {
			t.Errorf("LicenseAllowed(%q, %v) = %v, want %v", tc.license, tc.allow, got, tc.want)
		}
	}
}
