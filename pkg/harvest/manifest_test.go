package harvest

import (
	"os"
	"path/filepath"
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

