package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "harvest.toml")
	contents := `
[[source]]
name = "claude-code"
url = "https://github.com/anthropics/claude-code"
skill_paths = ["plugin-source/skills/**/SKILL.md"]
license_allow = ["MIT", "Apache-2.0"]

[[source]]
name = "openclaw"
url = "file:///Users/dev/Code/openclaw"
skill_paths = ["skills/*"]
license_allow = ["*"]
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(m.Sources))
	}
	if m.Sources[0].Name != "claude-code" {
		t.Errorf("source[0].name = %q", m.Sources[0].Name)
	}
	if m.FindSource("openclaw") == nil {
		t.Error("FindSource(\"openclaw\") = nil, want pointer")
	}
	if m.FindSource("nope") != nil {
		t.Error("FindSource(\"nope\") = non-nil, want nil")
	}
}

func TestLoadManifest_Invalid(t *testing.T) {
	cases := map[string]string{
		"empty manifest": `# nothing here`,
		"missing url": `
[[source]]
name = "x"
skill_paths = ["**/SKILL.md"]
`,
		"missing skill_paths": `
[[source]]
name = "x"
url = "https://example.com/x"
`,
		"duplicate name": `
[[source]]
name = "x"
url = "https://a"
skill_paths = ["*"]

[[source]]
name = "x"
url = "https://b"
skill_paths = ["*"]
`,
	}
	for label, body := range cases {
		t.Run(label, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "harvest.toml")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadManifest(path)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", label)
			}
		})
	}
}

func TestParseCriticVerdict(t *testing.T) {
	cases := map[string]CriticDecision{
		"text\nverdict: accept\n":     CriticAccept,
		"text\nverdict: pass":         CriticAccept,
		"text\nverdict：通过":            CriticAccept,
		"text\nverdict: warn":         CriticWarn,
		"text\nverdict: reject":       CriticReject,
		"text\nverdict: 不通过":          CriticReject,
		"text without a verdict line": CriticWarn,
		"verdict: gibberish":          CriticWarn,
	}
	for body, want := range cases {
		got := parseCriticVerdict(body)
		if got != want {
			t.Errorf("parseCriticVerdict(%q) = %q, want %q", strings.ReplaceAll(body, "\n", "⏎"), got, want)
		}
	}
}
