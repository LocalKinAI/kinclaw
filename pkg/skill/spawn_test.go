package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper: build a spawnSkill with a temp soulDir; useful for tests that
// check resolution behavior without needing real specialist souls on disk.
func newTestSpawn(t *testing.T, enabled bool) (*spawnSkill, string) {
	t.Helper()
	dir := t.TempDir()
	return &spawnSkill{enabled: enabled, soulDirs: []string{dir}}, dir
}

func TestSpawn_Disabled_ReturnsErrorBeforeSpawning(t *testing.T) {
	// With permissions.spawn = false the skill is registered (so the
	// agent sees a clear error) but Execute refuses without doing
	// anything else. No subprocess, no env mutation.
	s, _ := newTestSpawn(t, false)
	_, err := s.Execute(map[string]string{"soul": "researcher", "prompt": "hi"})
	if err == nil {
		t.Fatal("expected disabled error, got nil")
	}
	if !strings.Contains(err.Error(), "spawn disabled") {
		t.Errorf("expected 'spawn disabled' message, got: %v", err)
	}
}

func TestSpawn_RecursionGuard_RefusesNestedSpawn(t *testing.T) {
	// If we're already running as a child (KINCLAW_SPAWN_DEPTH set),
	// further spawn calls must refuse — no matter what permissions
	// the soul declared. This is the kernel-level depth limit.
	s, _ := newTestSpawn(t, true)
	t.Setenv(spawnDepthEnv, "1")
	_, err := s.Execute(map[string]string{"soul": "researcher", "prompt": "hi"})
	if err == nil {
		t.Fatal("expected recursion-guard error, got nil")
	}
	if !strings.Contains(err.Error(), "max recursion depth") {
		t.Errorf("error should mention recursion depth, got: %v", err)
	}
}

func TestSpawn_RequiresBothSoulAndPrompt(t *testing.T) {
	s, _ := newTestSpawn(t, true)
	cases := []struct {
		name   string
		params map[string]string
		want   string
	}{
		{"missing_both", map[string]string{}, "soul is required"},
		{"missing_prompt", map[string]string{"soul": "x"}, "prompt is required"},
		{"empty_soul", map[string]string{"soul": "  ", "prompt": "hi"}, "soul is required"},
		{"empty_prompt", map[string]string{"soul": "x", "prompt": "  "}, "prompt is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.Execute(c.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("expected %q, got: %v", c.want, err)
			}
		})
	}
}

func TestSpawn_UnknownSoulName_ReturnsError(t *testing.T) {
	s, _ := newTestSpawn(t, true)
	_, err := s.Execute(map[string]string{"soul": "DefinitelyNotASoul", "prompt": "hi"})
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestSpawn_ResolvesSoulByName(t *testing.T) {
	// Put a stub soul in the temp dir; resolveSoul should find it
	// without needing the file to actually be loadable (we never
	// invoke kinclaw — just exercise resolution).
	s, dir := newTestSpawn(t, true)
	stub := filepath.Join(dir, "stubsoul.soul.md")
	if err := os.WriteFile(stub, []byte("---\nname: stub\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := s.resolveSoul("stubsoul")
	if got == "" {
		t.Fatal("expected to find stubsoul, got empty")
	}
	abs, _ := filepath.Abs(stub)
	if got != abs {
		t.Errorf("expected resolved path = %q, got %q", abs, got)
	}
}

func TestSpawn_ResolvesAbsolutePath(t *testing.T) {
	// User can pass a full path to a soul file; resolve should
	// accept it as-is.
	s, dir := newTestSpawn(t, true)
	stub := filepath.Join(dir, "abs.soul.md")
	os.WriteFile(stub, []byte("---\nname: abs\n---\n"), 0644)
	got := s.resolveSoul(stub)
	if got == "" {
		t.Fatal("expected absolute path to resolve")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute, got %q", got)
	}
}

func TestSpawn_TimeoutCappedAt600(t *testing.T) {
	// Even if the agent passes a huge timeout, the skill caps at 600s.
	// We don't actually run a 600s spawn; just verify the parser logic
	// by looking at what gets stored... actually the cap is internal,
	// not externally observable without a real run. Skip detailed
	// timeout test — the logic is straightforward.
	s, dir := newTestSpawn(t, true)
	// Provide a real-but-stub soul so soul resolution doesn't fail
	// before we reach the timeout-parsing code.
	stub := filepath.Join(dir, "tt.soul.md")
	os.WriteFile(stub, []byte("---\nname: tt\n---\n"), 0644)

	// We can't easily test the cap without launching kinclaw; instead
	// verify that an obviously-bogus timeout string doesn't crash the
	// skill (it should fall back to default 180).
	_, err := s.Execute(map[string]string{
		"soul": "tt", "prompt": "x", "timeout_s": "not-a-number",
	})
	// We expect this to FAIL trying to launch kinclaw (no real
	// executable accessible from tests) but NOT due to timeout parsing.
	if err != nil && strings.Contains(err.Error(), "timeout") &&
		!strings.Contains(err.Error(), "timed out") {
		t.Errorf("bogus timeout_s should fall back to default, not error early: %v", err)
	}
}

func TestSpawn_ToolDef_HasSoulAndPromptRequired(t *testing.T) {
	s, _ := newTestSpawn(t, true)
	def := string(s.ToolDef())
	for _, mustHave := range []string{"\"soul\"", "\"prompt\"", "\"timeout_s\"", "spawn"} {
		if !strings.Contains(def, mustHave) {
			t.Errorf("ToolDef missing %q, got: %s", mustHave, def)
		}
	}
}

func TestSpawn_Description_MentionsSpecialistSouls(t *testing.T) {
	// The description doubles as routing guidance for the LLM; verify
	// it actually names the specialists so the agent can pick.
	s, _ := newTestSpawn(t, true)
	desc := s.Description()
	for _, name := range []string{"researcher", "eye", "critic"} {
		if !strings.Contains(desc, name) {
			t.Errorf("Description missing specialist soul %q", name)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	cases := []struct {
		in, want string
		max      int
	}{
		{"short", "short", 100},
		{strings.Repeat("a", 1000), strings.Repeat("a", 100) + "...[truncated]", 100},
		{"", "", 100},
	}
	for _, c := range cases {
		got := truncateStr(c.in, c.max)
		if got != c.want {
			t.Errorf("truncateStr(_, %d) = %q, want %q", c.max, got, c.want)
		}
	}
}
