//go:build darwin

package skill

import (
	"strings"
	"testing"
)

// TestSplitMenuPath covers the menu-path parser used by `ui menu_path`
// and `ui shortcut`. Three accepted separators: ' > ', '>', '/', '→'
// (with or without surrounding spaces). Empty parts get dropped.
func TestSplitMenuPath(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"File > Save", []string{"File", "Save"}},
		{"File>Save", []string{"File", "Save"}},
		{"Format > Cell > Conditional Highlighting",
			[]string{"Format", "Cell", "Conditional Highlighting"}},
		{"View / Show Toolbar", []string{"View", "Show Toolbar"}},
		{"Edit→Find→Find...", []string{"Edit", "Find", "Find..."}},
		{"OnlyOne", []string{"OnlyOne"}},
		{"  Spaced  >  Out  ", []string{"Spaced", "Out"}},
	}
	for _, c := range cases {
		got := splitMenuPath(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitMenuPath(%q): got %d parts, want %d (got=%v)",
				c.in, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitMenuPath(%q) part %d: got %q, want %q",
					c.in, i, got[i], c.want[i])
			}
		}
	}
}

// TestFormatShortcut covers the bit decoding for AXMenuItemCmdModifiers.
// Apple's quirk: bit 3 (value 8) means "no command" — set when ⌘ is
// NOT part of the shortcut. So bit 3 cleared → ⌘ is implicit.
func TestFormatShortcut(t *testing.T) {
	cases := []struct {
		char string
		mods int
		vk   int
		want string
	}{
		// Cmd+S (just ⌘): mod=0
		{"s", 0, 0, "⌘S"},
		// Cmd+Shift+S: mod=1 (shift)
		{"s", 1, 0, "⌘⇧S"},
		// Cmd+Option+S: mod=2 (option)
		{"s", 2, 0, "⌘⌥S"},
		// Cmd+Shift+Option+S: mod=3
		{"s", 3, 0, "⌘⇧⌥S"},
		// No-cmd shortcut: mod=8 → bit 3 set → no ⌘ in output
		{"f", 8, 0, "F"},
		// virtual key fallback (function keys, arrows)
		{"", 0, 122, "⌘(virtual_key=122)"},
	}
	for _, c := range cases {
		got := formatShortcut(c.char, c.mods, c.vk, "Test > Path")
		if !strings.Contains(got, c.want) {
			t.Errorf("formatShortcut(char=%q mods=%d vk=%d): expected to contain %q, got %q",
				c.char, c.mods, c.vk, c.want, got)
		}
	}
}

// TestFormatDiff covers the state-diff renderer. Uses the stateNode
// struct directly (no AX needed) to verify the +/-/~ classification
// and output sections.
func TestFormatDiff(t *testing.T) {
	before := map[string]stateNode{
		"app/win/btn[Save]":       {Path: "app/win/btn[Save]", Role: "AXButton", Title: "Save", Value: ""},
		"app/win/field[entry]":    {Path: "app/win/field[entry]", Role: "AXTextField", Title: "Entry", Value: "before"},
		"app/win/btn[Cancel]":     {Path: "app/win/btn[Cancel]", Role: "AXButton", Title: "Cancel"},
	}
	after := map[string]stateNode{
		"app/win/btn[Save]":    {Path: "app/win/btn[Save]", Role: "AXButton", Title: "Save"},
		"app/win/field[entry]": {Path: "app/win/field[entry]", Role: "AXTextField", Title: "Entry", Value: "after"},
		// Cancel removed
		"app/win/dialog[Confirm]": {Path: "app/win/dialog[Confirm]", Role: "AXSheet", Title: "Confirm"},
	}
	out := formatDiff(before, after)
	if !strings.Contains(out, "+1") || !strings.Contains(out, "-1") || !strings.Contains(out, "~1") {
		t.Errorf("expected diff header '+1 -1 ~1', got: %s", out)
	}
	if !strings.Contains(out, "+ app/win/dialog[Confirm]") {
		t.Errorf("expected added Confirm dialog, got: %s", out)
	}
	if !strings.Contains(out, "- app/win/btn[Cancel]") {
		t.Errorf("expected removed Cancel button, got: %s", out)
	}
	if !strings.Contains(out, "before") || !strings.Contains(out, "after") {
		t.Errorf("expected before→after value diff, got: %s", out)
	}
}

// TestFormatDiff_NoChanges — sanity for the happy path.
func TestFormatDiff_NoChanges(t *testing.T) {
	state := map[string]stateNode{
		"app/win/btn[Save]": {Path: "app/win/btn[Save]", Role: "AXButton", Title: "Save"},
	}
	out := formatDiff(state, state)
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected 'no changes' message, got: %s", out)
	}
}
