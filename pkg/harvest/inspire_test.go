package harvest

import (
	"strings"
	"testing"
)

func TestParseInspireResponse_Forged(t *testing.T) {
	body := `verdict: forged
capability: add a Reminders item via remindctl
inputs: title, list
caveats: depends on remindctl being installed (brew install remindctl)

---KINCLAW_SKILL_BEGIN---
---
name: reminders_add
description: Add a Reminders item using remindctl.
command: ["remindctl"]
args: ["add", "{{title}}", "--list", "{{list}}"]
schema:
  title:
    type: string
    description: Reminder text
  list:
    type: string
    description: Reminders list name
---
# reminders_add
---KINCLAW_SKILL_END---
`

	res := parseInspireResponse(body)
	if res.Verdict != InspireForged {
		t.Fatalf("verdict = %q, want %q", res.Verdict, InspireForged)
	}
	if !strings.Contains(res.ForgedContent, "name: reminders_add") {
		t.Errorf("ForgedContent missing name: %q", res.ForgedContent)
	}
	if !strings.Contains(res.ForgedContent, `args: ["add"`) {
		t.Errorf("ForgedContent missing args: %q", res.ForgedContent)
	}
	// Markers themselves should NOT appear in the extracted content.
	if strings.Contains(res.ForgedContent, "KINCLAW_SKILL_BEGIN") {
		t.Errorf("ForgedContent leaked the BEGIN marker")
	}
}

func TestParseInspireResponse_Deferred(t *testing.T) {
	body := `verdict: defer_to_procedural
original_concept: dogfood QA testing of web apps
reason: needs multi-turn LLM round-trips for exploration + bug judgement, not a single shell exec
`
	res := parseInspireResponse(body)
	if res.Verdict != InspireDeferred {
		t.Fatalf("verdict = %q, want %q", res.Verdict, InspireDeferred)
	}
	if !strings.Contains(res.DeferReason, "multi-turn LLM") {
		t.Errorf("DeferReason missing key phrase: %q", res.DeferReason)
	}
}

func TestParseInspireResponse_NoVerdict(t *testing.T) {
	body := `Sure! Here's how I would think about this skill...

It looks like a Reminders integration. Would be straightforward to forge.`
	res := parseInspireResponse(body)
	if res.Verdict != InspireUnparseable {
		t.Fatalf("verdict = %q, want %q", res.Verdict, InspireUnparseable)
	}
}

func TestParseInspireResponse_ForgedWithoutBlock(t *testing.T) {
	// Coder said "forged" but didn't include the BEGIN/END markers.
	// Should fall back to unparseable so the human catches it.
	body := `verdict: forged
capability: do something cool

(forgot the SKILL.md block)
`
	res := parseInspireResponse(body)
	if res.Verdict != InspireUnparseable {
		t.Fatalf("verdict = %q, want %q (forged without block must be unparseable)", res.Verdict, InspireUnparseable)
	}
}

func TestParseInspireResponse_ChineseVerdict(t *testing.T) {
	// Coder soul might output Chinese punctuation (：not :). The
	// permissive verdict regex should still match.
	body := `verdict：defer_to_procedural
reason：纯 prompt 模板，无法 exec 化
`
	res := parseInspireResponse(body)
	if res.Verdict != InspireDeferred {
		t.Errorf("verdict = %q, want %q (Chinese colon should still parse)", res.Verdict, InspireDeferred)
	}
}

func TestLooksProcedural(t *testing.T) {
	cases := map[string]struct {
		content string
		want    bool
	}{
		"anthropic-style (no command)": {
			content: `---
name: yuanbao
description: "Yuanbao groups: @mention users."
version: 1.0.0
---

# Yuanbao

Body here.
`,
			want: true,
		},
		"kinclaw-style (has command)": {
			content: `---
name: weather
description: Get the weather.
command: ["curl"]
args: ["https://wttr.in/{{location}}"]
---
`,
			want: false,
		},
		"missing description": {
			content: `---
name: x
---
body
`,
			want: false,
		},
		"missing name": {
			content: `---
description: missing the name
---
body
`,
			want: false,
		},
		"no frontmatter at all": {
			content: `# just markdown, no yaml`,
			want:    false,
		},
		"malformed yaml": {
			content: `---
name: [
---
body
`,
			want: false,
		},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			got := looksProcedural([]byte(tc.content))
			if got != tc.want {
				t.Errorf("looksProcedural() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSanitizeProcName(t *testing.T) {
	cases := map[string]string{
		"yuanbao":                "yuanbao",
		"Apple Reminders":        "apple_reminders",
		"  Trailing Spaces  ":    "trailing_spaces",
		"snake_case_already":     "snake_case_already",
		"hyphen-name":            "hyphen-name",
		"with/slash":             "with_slash",
		"中文 name":                "name", // CJK collapses to _, then trimmed
		"":                       "unnamed",
		"___":                    "unnamed",
		"emoji 🦞 name":           "emoji_name",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			got := sanitizeProcName(in)
			if got != want {
				t.Errorf("sanitizeProcName(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestExtractYAMLName(t *testing.T) {
	cases := map[string]string{
		`name: foo
description: bar`: "foo",
		`name: "quoted name"`: "quoted name",
		`description: no name here`: "",
		`name: 42`:                  "", // non-string → empty
		``:                          "",
	}
	for raw, want := range cases {
		t.Run(strings.ReplaceAll(raw, "\n", "⏎"), func(t *testing.T) {
			got := extractYAMLName(raw)
			if got != want {
				t.Errorf("extractYAMLName(%q) = %q, want %q", raw, got, want)
			}
		})
	}
}
