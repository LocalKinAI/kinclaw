//go:build !darwin && !linux

// input_other.go — stub for non-Mac, non-Linux (BSD, Windows).
// Linux has its own implementation in input_linux.go.

package skill

import (
	"encoding/json"
	"fmt"
)

type inputSkill struct{}

func NewInputSkill(_ bool) Skill { return &inputSkill{} }

func (s *inputSkill) Name() string { return "input" }
func (s *inputSkill) Description() string {
	return "Mouse/keyboard synthesis (macOS only — unavailable on this platform)."
}
func (s *inputSkill) ToolDef() json.RawMessage {
	return MakeToolDef("input", s.Description(), nil, nil)
}
func (s *inputSkill) Execute(_ map[string]string) (string, error) {
	return "", fmt.Errorf("input skill is macOS-only; not available on this platform")
}
