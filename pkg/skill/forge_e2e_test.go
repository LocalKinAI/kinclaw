package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the full forge flow (parse → validate → write →
// round-trip load) using a temp directory so we don't pollute the real
// skills/ tree. Failure cases must leave the temp dir clean.

func newForgeForTest(t *testing.T) (*forgeSkill, string) {
	t.Helper()
	dir := t.TempDir()
	reg := NewRegistry()
	return &forgeSkill{skillsDir: dir, registry: reg}, dir
}

func TestForge_HappyPath_MusicPlay(t *testing.T) {
	f, dir := newForgeForTest(t)
	out, err := f.Execute(map[string]string{
		"name":        "music_play",
		"description": "Play in Apple Music",
		"command":     "osascript",
		"args":        `["-e", "tell application \"Music\" to play"]`,
	})
	if err != nil {
		t.Fatalf("happy path forge failed: %v", err)
	}
	if !strings.Contains(out, "music_play") {
		t.Errorf("expected output to mention skill name, got: %s", out)
	}
	skillPath := filepath.Join(dir, "music_play", "SKILL.md")
	body, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md not written: %v", err)
	}
	// Round-trip ourselves to confirm produced YAML parses.
	if _, err := LoadExternalSkill(skillPath); err != nil {
		t.Fatalf("written SKILL.md doesn't reload — forge produced unparseable YAML: %v\n---\n%s", err, body)
	}
}

func TestForge_RejectsUnparseableArgs(t *testing.T) {
	// What the agent did before the fix: passing the args as a YAML-flow
	// array string instead of a JSON array. Should now be rejected up
	// front, not silently produce a broken file.
	f, dir := newForgeForTest(t)
	_, err := f.Execute(map[string]string{
		"name":        "broken_args",
		"description": "test",
		"command":     "osascript",
		"args":        `[-e tell application "Music" to pause]`, // YAML-flow, not JSON
	})
	if err == nil {
		t.Fatal("expected rejection of YAML-flow-style args, got nil")
	}
	if !strings.Contains(err.Error(), "args must be a JSON array") {
		t.Errorf("error should explain JSON-array contract, got: %v", err)
	}
	// No partial state on disk.
	if _, statErr := os.Stat(filepath.Join(dir, "broken_args")); !os.IsNotExist(statErr) {
		t.Error("broken_args dir should not exist after rejection")
	}
}

func TestForge_RejectsHardcodedCoords(t *testing.T) {
	f, _ := newForgeForTest(t)
	_, err := f.Execute(map[string]string{
		"name":        "maps_search",
		"description": "test",
		"command":     "osascript",
		"args":        `["-e", "tell process \"Maps\" to click at {760, 150}"]`,
	})
	if err == nil {
		t.Fatal("expected rejection of hardcoded coordinates")
	}
	if !strings.Contains(err.Error(), "hardcoded screen coordinates") {
		t.Errorf("error should mention coords, got: %v", err)
	}
}

func TestForge_RejectsUndeclaredTemplateVar(t *testing.T) {
	f, _ := newForgeForTest(t)
	_, err := f.Execute(map[string]string{
		"name":        "reminders_add",
		"description": "test",
		"command":     "osascript",
		"args":        `["-e", "tell application \"Reminders\" to make new reminder with properties {name:\"{{title}}\"}"]`,
		// schema omitted — {{title}} has no declaration
	})
	if err == nil {
		t.Fatal("expected rejection because {{title}} not in schema")
	}
	if !strings.Contains(err.Error(), "{{title}}") {
		t.Errorf("error should mention {{title}}, got: %v", err)
	}
}

func TestForge_DeclaredTemplateVarPasses(t *testing.T) {
	f, dir := newForgeForTest(t)
	_, err := f.Execute(map[string]string{
		"name":        "reminders_add",
		"description": "Add a reminder with a title",
		"command":     "osascript",
		"args":        `["-e", "tell application \"Reminders\" to make new reminder with properties {name:\"{{title}}\"}"]`,
		"schema":      `{"title": {"type": "string", "required": true}}`,
	})
	if err != nil {
		t.Fatalf("schema-declared template var should pass, got: %v", err)
	}
	if _, err := LoadExternalSkill(filepath.Join(dir, "reminders_add", "SKILL.md")); err != nil {
		t.Fatalf("produced SKILL.md doesn't reload: %v", err)
	}
}

func TestForge_RejectsBadName(t *testing.T) {
	f, _ := newForgeForTest(t)
	for _, badName := range []string{"has space", "has-dash", "1starts_with_digit", ""} {
		t.Run(badName, func(t *testing.T) {
			_, err := f.Execute(map[string]string{
				"name":        badName,
				"description": "test",
				"command":     "echo",
			})
			if err == nil {
				t.Errorf("expected rejection of name=%q", badName)
			}
		})
	}
}

func TestForge_RejectsInternalSkillNameAsCommand(t *testing.T) {
	// Pre-existing rule (validateForgeCommand), kept covered.
	f, _ := newForgeForTest(t)
	_, err := f.Execute(map[string]string{
		"name":        "wrap_ui",
		"description": "test",
		"command":     "ui",
	})
	if err == nil {
		t.Fatal("expected rejection of command=ui (internal skill name)")
	}
	if !strings.Contains(err.Error(), "internal skill") {
		t.Errorf("error should mention internal skill, got: %v", err)
	}
}

func TestForge_RejectsDanglingOsascriptE(t *testing.T) {
	f, _ := newForgeForTest(t)
	_, err := f.Execute(map[string]string{
		"name":        "bad_e",
		"description": "test",
		"command":     "osascript",
		"args":        `["-e", "tell app to play", "-e"]`,
	})
	if err == nil {
		t.Fatal("expected rejection of trailing -e")
	}
}

func TestForge_OsascriptOpensQuotedString_RoundTrips(t *testing.T) {
	// AppleScript with all the YAML-hostile chars: nested quotes,
	// braces, colons. The fix's whole point is that yaml.Marshal
	// handles this — verify it does.
	f, dir := newForgeForTest(t)
	hairyScript := `tell application "Notes" to make new note with properties {name:"{{title}}", body:"{{body}}"}`
	_, err := f.Execute(map[string]string{
		"name":        "notes_create",
		"description": "Create a note",
		"command":     "osascript",
		"args":        `["-e", ` + jsonString(hairyScript) + `]`,
		"schema":      `{"title": {"type":"string"}, "body": {"type":"string"}}`,
	})
	if err != nil {
		t.Fatalf("hairy AppleScript forge failed: %v", err)
	}
	loaded, err := LoadExternalSkill(filepath.Join(dir, "notes_create", "SKILL.md"))
	if err != nil {
		t.Fatalf("round-trip load failed: %v", err)
	}
	// Verify the script payload survived intact (quotes preserved, no mangling).
	args := loaded.meta.Args
	if len(args) < 2 || args[1] != hairyScript {
		t.Errorf("AppleScript text mangled by YAML round-trip:\n  want: %q\n  got args: %v",
			hairyScript, args)
	}
}

// jsonString escapes a Go string so it can be embedded inside a JSON
// literal. Used to build the args parameter for tests.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
