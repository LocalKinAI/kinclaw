package probe

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerdict_Classify(t *testing.T) {
	cases := []struct {
		name string
		s    Stats
		want Verdict
	}{
		{"process_failed", Stats{ProcessOK: false, TotalNodes: 100, Buttons: 10}, VerdictDead},
		{"empty_tree", Stats{ProcessOK: true, TotalNodes: 1}, VerdictBlank},
		{"under_threshold_nodes", Stats{ProcessOK: true, TotalNodes: 9, Buttons: 5}, VerdictBlank},
		{"shallow_few_nodes", Stats{ProcessOK: true, TotalNodes: 25, Buttons: 1}, VerdictShallow},
		{"shallow_many_nodes_no_action", Stats{ProcessOK: true, TotalNodes: 200, Buttons: 0, MenuItems: 2}, VerdictShallow},
		{"rich_via_buttons", Stats{ProcessOK: true, TotalNodes: 250, Buttons: 27}, VerdictRich},
		{"rich_via_menu_items", Stats{ProcessOK: true, TotalNodes: 459, Buttons: 0, MenuItems: 410}, VerdictRich},
		{"rich_via_text_fields", Stats{ProcessOK: true, TotalNodes: 100, TextFields: 6}, VerdictRich},
		{"rich_actionable_split", Stats{ProcessOK: true, TotalNodes: 150, Buttons: 2, TextFields: 2, MenuItems: 1}, VerdictRich},
		{"borderline_actionable_4", Stats{ProcessOK: true, TotalNodes: 200, Buttons: 2, TextFields: 1, MenuItems: 1}, VerdictShallow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.s.Classify(); got != c.want {
				t.Errorf("Classify() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestVerdict_StringIcon(t *testing.T) {
	cases := []struct {
		v        Verdict
		wantStr  string
		wantIcon string
	}{
		{VerdictRich, "rich", "🟢"},
		{VerdictShallow, "shallow", "🟡"},
		{VerdictBlank, "blank", "🟠"},
		{VerdictDead, "dead", "🔴"},
		{Verdict(99), "unknown", "❓"},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.wantStr {
			t.Errorf("Verdict(%d).String() = %q, want %q", c.v, got, c.wantStr)
		}
		if got := c.v.Icon(); got != c.wantIcon {
			t.Errorf("Verdict(%d).Icon() = %q, want %q", c.v, got, c.wantIcon)
		}
	}
}

func TestStats_CSVRow_MatchesHeader(t *testing.T) {
	s := Stats{
		BundleID: "com.apple.test", ProcessOK: true, ProbeOK: true,
		ProbeMS: 100, TotalNodes: 250, MaxDepth: 7, Buttons: 27,
		TextFields: 0, MenuItems: 194, Windows: 1, StaticTexts: 1,
		Images: 0, Literate: 196, ErrMsg: "",
	}
	row := s.CSVRow()
	if len(row) != len(CSVHeader) {
		t.Fatalf("CSVRow length=%d, CSVHeader length=%d — must match", len(row), len(CSVHeader))
	}
	if row[0] != "com.apple.test" {
		t.Errorf("row[0] = %q, want %q", row[0], "com.apple.test")
	}
	if row[1] != "1" || row[2] != "1" {
		t.Errorf("expected process_ok and probe_ok = '1', got %q %q", row[1], row[2])
	}
}

func TestStats_JSON_RoundTrip(t *testing.T) {
	s := Stats{
		BundleID: "com.apple.calculator", ProcessOK: true, ProbeOK: true,
		ProbeMS: 175, TotalNodes: 250, MaxDepth: 7, Buttons: 27,
		MenuItems: 194, Windows: 1, StaticTexts: 1, Literate: 196,
	}
	out, err := s.JSON()
	if err != nil {
		t.Fatalf("JSON(): %v", err)
	}
	// Decode into a generic map and verify key fields are present, including
	// the synthetic "verdict" field added by Stats.JSON wrapper.
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["bundle_id"] != "com.apple.calculator" {
		t.Errorf("bundle_id = %v, want com.apple.calculator", got["bundle_id"])
	}
	if got["verdict"] != "rich" {
		t.Errorf("verdict = %v, want rich", got["verdict"])
	}
	if got["total_nodes"].(float64) != 250 {
		t.Errorf("total_nodes = %v, want 250", got["total_nodes"])
	}
}

func TestStats_Human_Survives_AllVerdicts(t *testing.T) {
	cases := []Stats{
		{BundleID: "com.test.dead", ProcessOK: false, ErrMsg: "no_process: timeout"},
		{BundleID: "com.test.blank", ProcessOK: true, TotalNodes: 1},
		{BundleID: "com.test.shallow", ProcessOK: true, TotalNodes: 50, Buttons: 1, ProbeOK: true, ProbeMS: 50},
		{BundleID: "com.test.rich", ProcessOK: true, TotalNodes: 500, Buttons: 30, MenuItems: 100, ProbeOK: true, ProbeMS: 200},
	}
	for _, s := range cases {
		t.Run(s.BundleID, func(t *testing.T) {
			out := s.Human()
			if out == "" {
				t.Error("Human() returned empty")
			}
			if !strings.Contains(out, s.BundleID) {
				t.Errorf("Human() output missing bundle ID, got: %s", out)
			}
			if !strings.Contains(out, "Verdict:") {
				t.Errorf("Human() output missing verdict line, got: %s", out)
			}
		})
	}
}

func TestLooksLikeBundleID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"com.apple.Notes", true},
		{"com.apple.iWork.Pages", true},
		{"io.tailscale.ipn.macos", true},
		{"Notes", false},                      // no dot
		{"/Applications/Notes.app", false},    // has slash
		{"com apple Notes", false},            // has space
		{"", false},                           // empty
		{".", false},                          // single dot, no parts
		{"com..apple", false},                 // empty middle segment
		{"com.", false},                       // trailing dot, empty segment
	}
	for _, c := range cases {
		if got := looksLikeBundleID(c.in); got != c.want {
			t.Errorf("looksLikeBundleID(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestResolveBundleID_PassThrough(t *testing.T) {
	// A well-formed bundle ID should be returned as-is, no disk lookup.
	in := "com.example.test.bundle"
	got, err := ResolveBundleID(in)
	if err != nil {
		t.Fatalf("ResolveBundleID(%q): %v", in, err)
	}
	if got != in {
		t.Errorf("got %q, want %q (bundle IDs should pass through)", got, in)
	}
}

func TestResolveBundleID_AppName_Calculator(t *testing.T) {
	// Calculator is in /System/Applications on every modern macOS install,
	// so name resolution should always work in CI on darwin.
	got, err := ResolveBundleID("Calculator")
	if err != nil {
		t.Skipf("Calculator not resolvable on this host (expected only on darwin): %v", err)
	}
	if got != "com.apple.calculator" {
		t.Errorf("got %q, want %q", got, "com.apple.calculator")
	}
}

func TestResolveBundleID_NotFound(t *testing.T) {
	_, err := ResolveBundleID("DefinitelyNotARealAppName_zzzzzz")
	if err == nil {
		t.Error("expected error for unresolvable name, got nil")
	}
}

func TestResolveBundleID_Empty(t *testing.T) {
	_, err := ResolveBundleID("")
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestOptions_DefaultsApplied(t *testing.T) {
	opts := Options{}.withDefaults()
	if opts.MaxDepth != DefaultMaxDepth {
		t.Errorf("MaxDepth not defaulted: got %d", opts.MaxDepth)
	}
	if opts.OpenTimeout != DefaultOpenTimeout {
		t.Errorf("OpenTimeout not defaulted: got %v", opts.OpenTimeout)
	}
	if opts.ProbeTimeout != DefaultProbeTimeout {
		t.Errorf("ProbeTimeout not defaulted: got %v", opts.ProbeTimeout)
	}
	if opts.ActivateSettle != DefaultActivateSettle {
		t.Errorf("ActivateSettle not defaulted: got %v", opts.ActivateSettle)
	}
}

func TestOptions_NonDefaults_Preserved(t *testing.T) {
	opts := Options{
		MaxDepth:     4,
		ProbeTimeout: 1,
		SkipActivate: true,
	}.withDefaults()
	if opts.MaxDepth != 4 {
		t.Errorf("MaxDepth overwritten: got %d", opts.MaxDepth)
	}
	if !opts.SkipActivate {
		t.Errorf("SkipActivate flipped to false")
	}
}
