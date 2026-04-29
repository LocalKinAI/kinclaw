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
	body := `verdict：defer_to_procedural
reason：纯 prompt 模板，无法 exec 化
`
	res := parseInspireResponse(body)
	if res.Verdict != InspireDeferred {
		t.Errorf("verdict = %q, want %q (Chinese colon should still parse)", res.Verdict, InspireDeferred)
	}
}
