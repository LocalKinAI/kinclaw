package skill

import (
	"strings"
	"testing"
)

func TestTodoSkill_Execute_HappyPath(t *testing.T) {
	s := NewTodoSkill().(*todoSkill)
	out, err := s.Execute(map[string]string{
		"todos": `[
            {"content":"Open Numbers","activeForm":"Opening Numbers","status":"in_progress"},
            {"content":"Type 42 in A1","activeForm":"Typing 42 in A1","status":"pending"},
            {"content":"Screenshot","activeForm":"Taking screenshot","status":"pending"}
        ]`,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, "(3 items, 1 in_progress, 0 completed)") {
		t.Errorf("summary missing or wrong: %q", out)
	}
	// Active item label uses activeForm, not content.
	if !strings.Contains(out, "Opening Numbers") {
		t.Errorf("expected activeForm 'Opening Numbers' in summary, got: %q", out)
	}

	items := s.Items()
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Status != "in_progress" {
		t.Errorf("item 0 status: want in_progress, got %q", items[0].Status)
	}
}

func TestTodoSkill_OverwriteSemantics(t *testing.T) {
	s := NewTodoSkill().(*todoSkill)
	// First call seeds two items.
	_, err := s.Execute(map[string]string{
		"todos": `[
            {"content":"a","activeForm":"a-ing","status":"pending"},
            {"content":"b","activeForm":"b-ing","status":"pending"}
        ]`,
	})
	if err != nil {
		t.Fatalf("seed err: %v", err)
	}

	// Second call REPLACES with a single item — old two should be gone.
	_, err = s.Execute(map[string]string{
		"todos": `[
            {"content":"c","activeForm":"c-ing","status":"completed"}
        ]`,
	})
	if err != nil {
		t.Fatalf("overwrite err: %v", err)
	}
	items := s.Items()
	if len(items) != 1 || items[0].Content != "c" {
		t.Errorf("overwrite failed: %+v", items)
	}
}

func TestTodoSkill_RejectsBadInput(t *testing.T) {
	s := NewTodoSkill().(*todoSkill)
	cases := []struct {
		name string
		raw  string
	}{
		{"missing", ""},
		{"not json", "not-json"},
		{"empty content", `[{"content":"","activeForm":"x","status":"pending"}]`},
		{"empty activeForm", `[{"content":"x","activeForm":"","status":"pending"}]`},
		{"bad status", `[{"content":"x","activeForm":"y","status":"banana"}]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params := map[string]string{}
			if c.raw != "" {
				params["todos"] = c.raw
			}
			if _, err := s.Execute(params); err == nil {
				t.Errorf("expected error for %q, got nil", c.name)
			}
		})
	}
	// State should still be empty after rejected calls.
	if items := s.Items(); len(items) != 0 {
		t.Errorf("rejected calls should not mutate state, got %d items", len(items))
	}
}

func TestTodoSkill_MultipleInProgressWarns(t *testing.T) {
	s := NewTodoSkill().(*todoSkill)
	out, err := s.Execute(map[string]string{
		"todos": `[
            {"content":"a","activeForm":"a-ing","status":"in_progress"},
            {"content":"b","activeForm":"b-ing","status":"in_progress"}
        ]`,
	})
	if err != nil {
		t.Fatalf("unexpected err (warning is soft): %v", err)
	}
	if !strings.Contains(out, "more than one item is in_progress") {
		t.Errorf("expected single-tasking warning, got: %q", out)
	}
}
