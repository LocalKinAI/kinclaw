package skill

import (
	"strings"
	"testing"
)

func TestWebSearchSkill_Name(t *testing.T) {
	s := NewWebSearchSkill()
	if s.Name() != "web_search" {
		t.Errorf("Name() = %q, want web_search", s.Name())
	}
}

func TestWebSearchSkill_ToolDef(t *testing.T) {
	s := NewWebSearchSkill()
	def := s.ToolDef()
	if def == nil {
		t.Fatal("ToolDef() returned nil")
	}
	raw := string(def)
	if !strings.Contains(raw, "web_search") {
		t.Errorf("ToolDef missing web_search: %s", raw)
	}
	if !strings.Contains(raw, "query") {
		t.Errorf("ToolDef missing query parameter: %s", raw)
	}
}

func TestWebSearchSkill_EmptyQuery(t *testing.T) {
	s := NewWebSearchSkill()
	_, err := s.Execute(map[string]string{"query": ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"<b>bold</b>", "bold"},
		{"plain text", "plain text"},
		{"<a href='#'>link</a>", "link"},
		{"<b>Go</b> is <i>fast</i>", "Go is fast"},
		{"  spaces  ", "spaces"},
	}
	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
