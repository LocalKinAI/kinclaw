//go:build !darwin && !linux

// ui_other.go — stub for non-Mac, non-Linux (BSD, Windows).
// Linux has its own implementation in ui_linux.go.

package skill

import (
	"encoding/json"
	"fmt"
)

type uiSkill struct{}

func NewUISkill(_ bool) Skill { return &uiSkill{} }

func (s *uiSkill) Name() string { return "ui" }
func (s *uiSkill) Description() string {
	return "UI tree navigation via Accessibility API (macOS only — unavailable on this platform)."
}
func (s *uiSkill) ToolDef() json.RawMessage {
	return MakeToolDef("ui", s.Description(), nil, nil)
}
func (s *uiSkill) Execute(_ map[string]string) (string, error) {
	return "", fmt.Errorf("ui skill is macOS-only; not available on this platform")
}
