package soul

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitFrontmatter_Valid(t *testing.T) {
	data := []byte("---\nname: test\n---\nHello body")
	yamlBlock, body, err := SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(yamlBlock), "name: test") {
		t.Errorf("expected yaml to contain 'name: test', got %q", string(yamlBlock))
	}
	if !strings.Contains(body, "Hello body") {
		t.Errorf("expected body to contain 'Hello body', got %q", body)
	}
}

func TestSplitFrontmatter_LeadingNewlines(t *testing.T) {
	data := []byte("\n\n---\nname: test\n---\nbody")
	_, body, err := SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "body") {
		t.Errorf("expected body 'body', got %q", body)
	}
}

func TestSplitFrontmatter_NoOpeningDelim(t *testing.T) {
	data := []byte("no frontmatter here")
	_, _, err := SplitFrontmatter(data)
	if err == nil {
		t.Fatal("expected error for missing opening ---")
	}
	if !strings.Contains(err.Error(), "must start with ---") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSplitFrontmatter_NoClosingDelim(t *testing.T) {
	data := []byte("---\nname: test\nno closing")
	_, _, err := SplitFrontmatter(data)
	if err == nil {
		t.Fatal("expected error for missing closing ---")
	}
	if !strings.Contains(err.Error(), "missing closing ---") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSplitFrontmatter_EmptyBody(t *testing.T) {
	data := []byte("---\nname: test\n---\n")
	_, body, err := SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
}

func TestParseSoul_MinimalValid(t *testing.T) {
	data := []byte("---\nname: testsoul\n---\nYou are a test assistant.")
	s, err := ParseSoul(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Meta.Name != "testsoul" {
		t.Errorf("expected name 'testsoul', got %q", s.Meta.Name)
	}
	if s.Meta.Brain.Provider != "claude" {
		t.Errorf("expected default provider 'claude', got %q", s.Meta.Brain.Provider)
	}
	if s.Meta.Brain.Endpoint != "https://api.anthropic.com" {
		t.Errorf("expected default claude endpoint, got %q", s.Meta.Brain.Endpoint)
	}
	if s.Meta.Brain.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", s.Meta.Brain.Temperature)
	}
	if s.Meta.Brain.ContextLength != 8192 {
		t.Errorf("expected default context_length 8192, got %d", s.Meta.Brain.ContextLength)
	}
	if s.Meta.Skills.OutputDir != "./output" {
		t.Errorf("expected default output_dir './output', got %q", s.Meta.Skills.OutputDir)
	}
	if !strings.Contains(s.SystemPrompt, "You are a test assistant.") {
		t.Errorf("expected system prompt to contain body text")
	}
	if !strings.Contains(s.SystemPrompt, "## Security") {
		t.Errorf("expected system prompt to contain security suffix")
	}
}

func TestParseSoul_MissingName(t *testing.T) {
	data := []byte("---\nversion: 1.0\n---\nBody")
	_, err := ParseSoul(data)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "missing required field: name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseSoul_ProviderDefaults(t *testing.T) {
	tests := []struct {
		provider string
		endpoint string
	}{
		{"claude", "https://api.anthropic.com"},
		{"openai", "https://api.openai.com"},
		{"ollama", "http://localhost:11434"},
	}
	for _, tt := range tests {
		data := []byte("---\nname: test\nbrain:\n  provider: " + tt.provider + "\n---\nBody")
		s, err := ParseSoul(data)
		if err != nil {
			t.Fatalf("provider %s: %v", tt.provider, err)
		}
		if s.Meta.Brain.Endpoint != tt.endpoint {
			t.Errorf("provider %s: expected endpoint %q, got %q", tt.provider, tt.endpoint, s.Meta.Brain.Endpoint)
		}
	}
}

func TestParseSoul_CustomEndpoint(t *testing.T) {
	data := []byte("---\nname: test\nbrain:\n  endpoint: http://custom:1234\n---\nBody")
	s, err := ParseSoul(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Meta.Brain.Endpoint != "http://custom:1234" {
		t.Errorf("expected custom endpoint, got %q", s.Meta.Brain.Endpoint)
	}
}

func TestParseSoul_EnvAPIKey(t *testing.T) {
	t.Setenv("TEST_API_KEY_XYZ", "sk-test-secret")
	data := []byte("---\nname: test\nbrain:\n  api_key: $TEST_API_KEY_XYZ\n---\nBody")
	s, err := ParseSoul(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Meta.Brain.APIKey != "sk-test-secret" {
		t.Errorf("expected API key from env, got %q", s.Meta.Brain.APIKey)
	}
}

func TestParseSoul_TemplateReplacement(t *testing.T) {
	data := []byte("---\nname: test\n---\nToday is {{current_date}}.")
	s, err := ParseSoul(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(s.SystemPrompt, "{{current_date}}") {
		t.Error("expected {{current_date}} to be replaced")
	}
}

func TestParseSoul_FullFields(t *testing.T) {
	data := []byte(`---
name: fulltest
version: "2.0"
brain:
  provider: openai
  model: gpt-4
  endpoint: https://api.openai.com
  temperature: 0.5
  context_length: 4096
  api_key: sk-inline-key
permissions:
  shell: true
  shell_timeout: 60
  network: true
  filesystem:
    allow: ["/tmp"]
    deny: ["/etc"]
skills:
  enable: [shell, file_read]
  output_dir: /tmp/output
  dir: /tmp/skills
---
You are a full test soul.`)
	s, err := ParseSoul(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Meta.Brain.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", s.Meta.Brain.Model)
	}
	if s.Meta.Brain.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", s.Meta.Brain.Temperature)
	}
	if s.Meta.Brain.ContextLength != 4096 {
		t.Errorf("expected context_length 4096, got %d", s.Meta.Brain.ContextLength)
	}
	if !s.Meta.Permissions.Shell {
		t.Error("expected shell permission to be true")
	}
	if s.Meta.Permissions.ShellTimeout != 60 {
		t.Errorf("expected shell_timeout 60, got %d", s.Meta.Permissions.ShellTimeout)
	}
	if !s.Meta.Permissions.Network {
		t.Error("expected network permission to be true")
	}
	if len(s.Meta.Permissions.Filesystem.Allow) != 1 || s.Meta.Permissions.Filesystem.Allow[0] != "/tmp" {
		t.Errorf("unexpected filesystem allow: %v", s.Meta.Permissions.Filesystem.Allow)
	}
	if s.Meta.Skills.OutputDir != "/tmp/output" {
		t.Errorf("expected output_dir '/tmp/output', got %q", s.Meta.Skills.OutputDir)
	}
	if s.Meta.Skills.Dir != "/tmp/skills" {
		t.Errorf("expected skills dir '/tmp/skills', got %q", s.Meta.Skills.Dir)
	}
}

func TestParseSoul_BootMessage(t *testing.T) {
	data := []byte(`---
name: booter
boot:
  message: "hello boot"
---
Boot test.`)
	s, err := ParseSoul(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Meta.Boot.Message != "hello boot" {
		t.Errorf("expected boot message 'hello boot', got %q", s.Meta.Boot.Message)
	}
}

func TestLoadSoul_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.soul.md")
	content := "---\nname: fromfile\n---\nBody text."
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSoul(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Meta.Name != "fromfile" {
		t.Errorf("expected name 'fromfile', got %q", s.Meta.Name)
	}
	if s.FilePath != path {
		t.Errorf("expected FilePath %q, got %q", path, s.FilePath)
	}
}

func TestLoadSoul_FileNotFound(t *testing.T) {
	_, err := LoadSoul("/nonexistent/path.soul.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadSoul_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.soul.md")
	content := "---\n: : invalid\n---\nBody"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSoul(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
