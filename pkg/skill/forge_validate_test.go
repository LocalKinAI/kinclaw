package skill

import (
	"strings"
	"testing"
)

func TestValidateForgeArgs_OsascriptDanglingE(t *testing.T) {
	cmd := []string{"osascript"}
	cases := []struct {
		name    string
		args    []string
		wantErr string // substring; empty = expect nil
	}{
		{
			name:    "ok_paired",
			args:    []string{"-e", "tell app \"Music\" to play"},
			wantErr: "",
		},
		{
			name:    "ok_two_pairs",
			args:    []string{"-e", "activate", "-e", "play"},
			wantErr: "",
		},
		{
			name:    "trailing_e",
			args:    []string{"-e", "tell app to play", "-e"},
			wantErr: "no script after it",
		},
		{
			name:    "two_e_in_a_row",
			args:    []string{"-e", "-e", "tell app to play"},
			wantErr: "followed by another flag",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateForgeArgs(cmd, c.args, nil)
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("expected nil, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", c.wantErr)
				return
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("expected %q in error, got: %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateForgeArgs_HardcodedCoords(t *testing.T) {
	cmd := []string{"osascript"}
	cases := []struct {
		name string
		args []string
		bad  bool
	}{
		{"plain_click_at", []string{"-e", "tell app to click at {760, 150}"}, true},
		{"with_extra_whitespace", []string{"-e", "click at  { 100 , 200 }"}, true},
		{"case_insensitive", []string{"-e", "Click At {300, 400}"}, true},
		{"keystroke_ok", []string{"-e", "tell app to keystroke \"Apple Park\""}, false},
		{"click_button_by_name_ok", []string{"-e", "click button \"Save\""}, false},
		{"absolute_path_with_braces_ok", []string{"-c", "echo '{a, b}'"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateForgeArgs(cmd, c.args, nil)
			if c.bad && err == nil {
				t.Errorf("expected hardcoded-coord rejection, got nil")
			}
			if !c.bad && err != nil {
				t.Errorf("expected ok, got: %v", err)
			}
		})
	}
}

func TestValidateForgeArgs_TemplateVarMustBeInSchema(t *testing.T) {
	cmd := []string{"osascript"}
	args := []string{"-e", "tell application \"Reminders\" to make new reminder with properties {name:\"{{title}}\"}"}
	t.Run("declared_passes", func(t *testing.T) {
		schema := map[string]interface{}{
			"title": map[string]interface{}{"type": "string"},
		}
		if err := validateForgeArgs(cmd, args, schema); err != nil {
			t.Errorf("expected ok, got: %v", err)
		}
	})
	t.Run("undeclared_fails", func(t *testing.T) {
		err := validateForgeArgs(cmd, args, nil)
		if err == nil {
			t.Error("expected rejection for undeclared template var")
		}
		if !strings.Contains(err.Error(), "{{title}}") {
			t.Errorf("error should mention the missing variable name, got: %v", err)
		}
	})
	t.Run("multiple_vars_partial_schema", func(t *testing.T) {
		schema := map[string]interface{}{"title": map[string]interface{}{"type": "string"}}
		twoVars := []string{"-e", "{{title}} and {{body}}"}
		err := validateForgeArgs(cmd, twoVars, schema)
		if err == nil {
			t.Error("expected rejection because {{body}} is undeclared")
		}
	})
}

func TestValidateForgeArgs_NonOsascriptSkipsEPairCheck(t *testing.T) {
	// `-e` has different meanings for different binaries (e.g. `sed -e` is
	// extended regex). We only enforce -e pairing when the binary is
	// osascript; other tools get to define their own flag semantics.
	cmd := []string{"sed"}
	args := []string{"-e"}
	if err := validateForgeArgs(cmd, args, nil); err != nil {
		t.Errorf("non-osascript should not trip the -e pair check: %v", err)
	}
}

func TestValidateForgeArgs_OsascriptViaAbsolutePath(t *testing.T) {
	// `/usr/bin/osascript` should still trigger the -e pairing check —
	// filepath.Base() handles both bare names and absolute paths.
	cmd := []string{"/usr/bin/osascript"}
	args := []string{"-e"} // dangling
	err := validateForgeArgs(cmd, args, nil)
	if err == nil {
		t.Error("absolute-path osascript should still get the -e check")
	}
}

func TestForgeNamePattern(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"reminders_add", true},
		{"music_play", true},
		{"calc1plus1", true},
		{"ABC", true},
		{"1starts_with_digit", false},
		{"has space", false},
		{"has-dash", false},
		{"has.dot", false},
		{"", false},
		{"name_is_way_too_long_to_be_useful_and_should_be_rejected_definitely_yes_indeed_overlong", false}, // > 64 chars
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := forgeNamePattern.MatchString(c.name)
			if got != c.ok {
				t.Errorf("forgeNamePattern.Match(%q) = %v, want %v", c.name, got, c.ok)
			}
		})
	}
}
