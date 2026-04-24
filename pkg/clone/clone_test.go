package clone

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LocalKinAI/kinclaw/pkg/soul"
)

const parentSoul = `---
name: "Test Parent"
version: "1.0.0"
brain:
  provider: "claude"
  model: "claude-sonnet-4-5"
permissions:
  shell: false
  network: true
skills:
  enable: ["file_read"]
---

# Test Parent

Hello from the parent soul.
`

func writeParent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "parent.soul.md")
	if err := os.WriteFile(path, []byte(parentSoul), 0o644); err != nil {
		t.Fatalf("write parent: %v", err)
	}
	return path
}

func TestCloneDefaultNaming(t *testing.T) {
	parent := writeParent(t)
	paths, err := Clone(parent, CloneOptions{Count: 3})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 clone paths, got %d", len(paths))
	}
	for i, p := range paths {
		want := "parent.clone_" + itoa(i+1) + ".soul.md"
		if filepath.Base(p) != want {
			t.Errorf("clone #%d: path base = %q, want %q", i+1, filepath.Base(p), want)
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf("clone #%d: stat: %v", i+1, err)
		}
	}
}

func TestCloneVerbatimByDefault(t *testing.T) {
	parent := writeParent(t)
	paths, err := Clone(parent, CloneOptions{Count: 1})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	got, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read clone: %v", err)
	}
	if string(got) != parentSoul {
		t.Errorf("without patch, clone bytes should equal parent verbatim.\nparent len=%d  clone len=%d",
			len(parentSoul), len(got))
	}
}

func TestCloneFrontmatterPatch(t *testing.T) {
	parent := writeParent(t)
	paths, err := Clone(parent, CloneOptions{
		Count: 2,
		FrontmatterPatch: func(i int, meta *soul.Meta) {
			meta.Name = "Child " + itoa(i)
		},
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	for i, p := range paths {
		parsed, err := soul.LoadSoul(p)
		if err != nil {
			t.Fatalf("load clone #%d: %v", i+1, err)
		}
		wantName := "Child " + itoa(i+1)
		if parsed.Meta.Name != wantName {
			t.Errorf("clone #%d: name = %q, want %q", i+1, parsed.Meta.Name, wantName)
		}
		if parsed.Meta.Brain.Provider != "claude" {
			t.Errorf("clone #%d: brain provider lost: %q", i+1, parsed.Meta.Brain.Provider)
		}
		if !strings.Contains(parsed.SystemPrompt, "Hello from the parent soul.") {
			t.Errorf("clone #%d: body missing", i+1)
		}
	}
}

func TestCloneCustomNaming(t *testing.T) {
	parent := writeParent(t)
	paths, err := Clone(parent, CloneOptions{
		Count:      2,
		NameSuffix: func(i int) string { return "spec_" + itoa(i) },
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	wants := []string{"spec_1.soul.md", "spec_2.soul.md"}
	for i, p := range paths {
		if filepath.Base(p) != wants[i] {
			t.Errorf("path #%d = %q, want %q", i+1, filepath.Base(p), wants[i])
		}
	}
}

func TestCloneDestDir(t *testing.T) {
	parent := writeParent(t)
	dest := filepath.Join(t.TempDir(), "nested", "dir")
	paths, err := Clone(parent, CloneOptions{Count: 1, DestDir: dest})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if filepath.Dir(paths[0]) != dest {
		t.Errorf("clone dir = %q, want %q", filepath.Dir(paths[0]), dest)
	}
}

func TestCloneZeroCount(t *testing.T) {
	parent := writeParent(t)
	if _, err := Clone(parent, CloneOptions{Count: 0}); err == nil {
		t.Error("Clone with count=0 should error")
	}
}

func TestCloneMissingParent(t *testing.T) {
	if _, err := Clone("/does/not/exist.soul.md", CloneOptions{Count: 1}); err == nil {
		t.Error("Clone of missing parent should error")
	}
}

// tiny itoa to keep tests dep-free
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
