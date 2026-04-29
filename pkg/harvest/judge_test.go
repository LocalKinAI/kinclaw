package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseJudgeResponse_Yes(t *testing.T) {
	body := `verdict: yes
reason: wraps remindctl CLI for Apple Reminders — fills gap, no overlap with current skills
domain: apple
`
	r := parseJudgeResponse(body)
	if r.Verdict != JudgeYes {
		t.Fatalf("verdict = %q, want %q", r.Verdict, JudgeYes)
	}
	if !strings.Contains(r.Reason, "remindctl") {
		t.Errorf("reason missing key phrase: %q", r.Reason)
	}
	if r.Domain != "apple" {
		t.Errorf("domain = %q, want apple", r.Domain)
	}
}

func TestParseJudgeResponse_Maybe(t *testing.T) {
	body := `verdict: maybe
reason: partial overlap with web claw — could complement Playwright for headless testing scenarios
domain: web
`
	r := parseJudgeResponse(body)
	if r.Verdict != JudgeMaybe {
		t.Errorf("verdict = %q, want %q", r.Verdict, JudgeMaybe)
	}
}

func TestParseJudgeResponse_No(t *testing.T) {
	body := `verdict: no
reason: duplicates existing git_commit
domain: git
`
	r := parseJudgeResponse(body)
	if r.Verdict != JudgeNo {
		t.Errorf("verdict = %q, want %q", r.Verdict, JudgeNo)
	}
}

func TestParseJudgeResponse_Chinese(t *testing.T) {
	body := `verdict：是
reason：补 macOS 提醒空白
domain: apple
`
	r := parseJudgeResponse(body)
	if r.Verdict != JudgeYes {
		t.Errorf("verdict = %q, want %q (Chinese full-width colon + 是)", r.Verdict, JudgeYes)
	}
}

func TestParseJudgeResponse_NoVerdictLine(t *testing.T) {
	body := `I think this skill is interesting because...
(LLM rambled instead of using the format)
`
	r := parseJudgeResponse(body)
	if r.Verdict != JudgeUnparseable {
		t.Errorf("verdict = %q, want %q", r.Verdict, JudgeUnparseable)
	}
}

func TestParseJudgeResponse_OptionalDomain(t *testing.T) {
	body := `verdict: yes
reason: useful gap-filler
`
	r := parseJudgeResponse(body)
	if r.Verdict != JudgeYes {
		t.Errorf("verdict = %q, want yes", r.Verdict)
	}
	if r.Domain != "" {
		t.Errorf("domain should be empty when missing, got %q", r.Domain)
	}
}

func TestExtractCandidate_FromFrontmatter(t *testing.T) {
	content := []byte(`---
name: yuanbao
description: "Yuanbao group: @mention users."
version: 1.0.0
---

# Yuanbao

Body content here.
`)
	src := Source{Name: "hermes-agent", URL: "https://example.com/hermes"}
	c := extractCandidate(content, "skills/yuanbao/SKILL.md", src)
	if c.Name != "yuanbao" {
		t.Errorf("Name = %q, want yuanbao", c.Name)
	}
	if !strings.Contains(c.Description, "@mention") {
		t.Errorf("Description = %q, want '@mention' phrase", c.Description)
	}
	if c.SourceName != "hermes-agent" {
		t.Errorf("SourceName = %q, want hermes-agent", c.SourceName)
	}
	if c.SkillRelPath != "skills/yuanbao/SKILL.md" {
		t.Errorf("SkillRelPath = %q", c.SkillRelPath)
	}
}

func TestExtractCandidate_NoFrontmatter(t *testing.T) {
	// `.cursorrules`-style: pure markdown, no YAML frontmatter.
	content := []byte(`# Project Coding Style

Use 2-space indents. Prefer named functions. Avoid arrow functions in classes.
`)
	src := Source{Name: "cursor-rules", URL: "https://example.com/cursor"}
	c := extractCandidate(content, "rules/project-style/.cursorrules", src)
	if c.Name != "project-style" {
		t.Errorf("Name = %q, want project-style (parent dir fallback)", c.Name)
	}
	if !strings.Contains(c.Description, "Project Coding Style") {
		t.Errorf("Description should fall back to body excerpt, got %q", c.Description)
	}
}

func TestSkillInventory_String(t *testing.T) {
	inv := &SkillInventory{Skills: []InventorySkill{
		{Name: "git_commit", Description: "git add + commit"},
		{Name: "weather", Description: "wttr.in via curl"},
	}}
	got := inv.String()
	for _, want := range []string{"git_commit — git add + commit", "weather — wttr.in via curl"} {
		if !strings.Contains(got, want) {
			t.Errorf("inventory.String() missing %q\n got %q", want, got)
		}
	}
}

func TestSkillInventory_Empty(t *testing.T) {
	inv := &SkillInventory{}
	got := inv.String()
	if !strings.Contains(got, "empty") {
		t.Errorf("empty inventory should mention 'empty', got %q", got)
	}
}

func TestLoadInventory_NonexistentDir(t *testing.T) {
	inv, err := LoadInventory(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("LoadInventory on missing dir should not error, got %v", err)
	}
	if len(inv.Skills) != 0 {
		t.Errorf("expected empty inventory, got %d skills", len(inv.Skills))
	}
}

func TestLoadInventory_RealSkills(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "weather"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: weather
description: Get current weather via wttr.in.
command: [curl]
args: ["-s", "https://wttr.in/{{location}}"]
schema:
  location:
    type: string
    description: City name
---
# weather
`
	if err := os.WriteFile(filepath.Join(dir, "weather", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := LoadInventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("expected 1 skill loaded, got %d", len(inv.Skills))
	}
	if inv.Skills[0].Name != "weather" {
		t.Errorf("name = %q, want weather", inv.Skills[0].Name)
	}
	if !strings.Contains(inv.Skills[0].Description, "wttr.in") {
		t.Errorf("desc missing wttr.in: %q", inv.Skills[0].Description)
	}
}

func TestFirstSentence(t *testing.T) {
	cases := map[string]string{
		"Single sentence.":                              "Single sentence",
		"First.\nSecond.":                               "First.",
		"First.\n\nSecond paragraph.":                   "First.",
		"With period mid-text. After period.":           "With period mid-text",
		"":                                              "",
		"  Leading whitespace gets trimmed.  ":          "Leading whitespace gets trimmed",
		"No terminator at all":                          "No terminator at all",
	}
	for in, want := range cases {
		got := firstSentence(in)
		if got != want {
			t.Errorf("firstSentence(%q) = %q, want %q", in, got, want)
		}
	}
}
