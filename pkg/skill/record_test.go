//go:build darwin

package skill

import (
	"strings"
	"testing"
)

// These tests cover the input-validation and state-management surface
// of the record skill. The actual recording path runs through kinrec
// (ScreenCaptureKit + TCC) and isn't unit-testable; integration tests
// for the full capture pipeline live in kinrec itself.

func TestRecordSkill_PermissionDenied(t *testing.T) {
	s := NewRecordSkill(false, "")
	for _, action := range []string{"start", "stop", "list", "stats", ""} {
		_, err := s.Execute(map[string]string{"action": action})
		if err == nil {
			t.Errorf("action=%q with allowed=false: expected permission error, got nil", action)
			continue
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("action=%q: expected 'permission denied' in error, got %q", action, err.Error())
		}
	}
}

func TestRecordSkill_UnknownAction(t *testing.T) {
	s := NewRecordSkill(true, "")
	_, err := s.Execute(map[string]string{"action": "rewind"})
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("expected 'unknown action' in error, got %q", err.Error())
	}
}

func TestRecordSkill_StopRequiresID(t *testing.T) {
	s := NewRecordSkill(true, "")
	_, err := s.Execute(map[string]string{"action": "stop"})
	if err == nil {
		t.Fatal("stop without id: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "id") {
		t.Errorf("expected 'id' in error, got %q", err.Error())
	}
}

func TestRecordSkill_StopUnknownID(t *testing.T) {
	s := NewRecordSkill(true, "")
	_, err := s.Execute(map[string]string{"action": "stop", "id": "rec-bogus"})
	if err == nil {
		t.Fatal("stop with bogus id: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no active recording") {
		t.Errorf("expected 'no active recording' in error, got %q", err.Error())
	}
}

func TestRecordSkill_StatsRequiresID(t *testing.T) {
	s := NewRecordSkill(true, "")
	_, err := s.Execute(map[string]string{"action": "stats"})
	if err == nil {
		t.Fatal("stats without id: expected error, got nil")
	}
}

func TestRecordSkill_StatsUnknownID(t *testing.T) {
	s := NewRecordSkill(true, "")
	_, err := s.Execute(map[string]string{"action": "stats", "id": "nope"})
	if err == nil {
		t.Fatal("stats with unknown id: expected error, got nil")
	}
}

func TestRecordSkill_ListEmpty(t *testing.T) {
	s := NewRecordSkill(true, "")
	out, err := s.Execute(map[string]string{"action": "list"})
	if err != nil {
		t.Fatalf("list: unexpected error %v", err)
	}
	if !strings.Contains(out, "no active recordings") {
		t.Errorf("expected 'no active recordings' on empty list, got %q", out)
	}
}

func TestRecordSkill_NameAndDescription(t *testing.T) {
	s := NewRecordSkill(true, "")
	if s.Name() != "record" {
		t.Errorf("Name() = %q, want %q", s.Name(), "record")
	}
	if !strings.Contains(s.Description(), "record") {
		t.Errorf("Description() should mention recording, got %q", s.Description())
	}
}

func TestRecordSkill_DisplayIDValidation(t *testing.T) {
	s := NewRecordSkill(true, "")
	_, err := s.Execute(map[string]string{
		"action":     "start",
		"display_id": "not-a-uint",
	})
	if err == nil {
		t.Fatal("expected error for non-numeric display_id, got nil")
	}
	if !strings.Contains(err.Error(), "display_id") {
		t.Errorf("expected 'display_id' in error, got %q", err.Error())
	}
}

func TestRecordSkill_FPSValidation(t *testing.T) {
	s := NewRecordSkill(true, "")
	_, err := s.Execute(map[string]string{
		"action": "start",
		"fps":    "fast",
	})
	if err == nil {
		t.Fatal("expected error for non-integer fps, got nil")
	}
	if !strings.Contains(err.Error(), "fps") {
		t.Errorf("expected 'fps' in error, got %q", err.Error())
	}
}

func TestParseBoolParam(t *testing.T) {
	tests := []struct {
		in   string
		dflt bool
		want bool
	}{
		{"", false, false},
		{"", true, true},
		{"true", false, true},
		{"false", true, false},
		{"True", false, true},
		{"FALSE", true, false},
		{"1", false, true},
		{"0", true, false},
		{"t", false, true},
		{"f", true, false},
		// Unparsable values (anything strconv.ParseBool rejects, e.g.
		// "yes"/"no"/"maybe") fall back to the default rather than
		// erroring. Matches the lenient style of the other claw skills.
		{"yes", false, false},
		{"no", true, true},
		{"maybe", false, false},
		{"maybe", true, true},
	}
	for _, tt := range tests {
		got := parseBoolParam(tt.in, tt.dflt)
		if got != tt.want {
			t.Errorf("parseBoolParam(%q, %v) = %v, want %v", tt.in, tt.dflt, got, tt.want)
		}
	}
}
