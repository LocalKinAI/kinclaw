//go:build !darwin

// smart_click_other.go — non-darwin stub.
// smart_click depends on Vision (macOS-only OCR + screen capture pipeline)
// so on Linux / Windows it returns "not available". A future Linux port
// will hook this up to tesseract + xdotool/wlroots; for now the agent
// kernel can register the skill so souls don't error at startup.

package skill

import (
	"encoding/json"
	"fmt"
)

type smartClickSkill struct{}

// NewSmartClickSkill — match the darwin signature so the kernel's
// New() call compiles unchanged on all platforms. The bool arg
// (whether vision-based dispatch is enabled) is ignored on non-Mac.
func NewSmartClickSkill(_ bool) Skill { return &smartClickSkill{} }

func (s *smartClickSkill) Name() string { return "smart_click" }
func (s *smartClickSkill) Description() string {
	return "Click on UI element by visual description (macOS-only — unavailable on this platform; Linux/Windows port pending)."
}
func (s *smartClickSkill) ToolDef() json.RawMessage {
	return MakeToolDef("smart_click", s.Description(), nil, nil)
}
func (s *smartClickSkill) Execute(_ map[string]string) (string, error) {
	return "", fmt.Errorf("smart_click skill is macOS-only; not available on this platform (Linux/Windows port pending)")
}
