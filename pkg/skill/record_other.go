//go:build !darwin && !linux

// record_other.go — stub for non-Mac, non-Linux (BSD, Windows).
// Linux has its own implementation in record_linux.go.

package skill

import (
	"encoding/json"
	"fmt"
)

type recordSkill struct{}

func NewRecordSkill(_ bool, _ string) Skill { return &recordSkill{} }

func (s *recordSkill) Name() string { return "record" }
func (s *recordSkill) Description() string {
	return "Screen recording (macOS only — unavailable on this platform)."
}
func (s *recordSkill) ToolDef() json.RawMessage {
	return MakeToolDef("record", s.Description(), nil, nil)
}
func (s *recordSkill) Execute(_ map[string]string) (string, error) {
	return "", fmt.Errorf("record skill is macOS-only; not available on this platform")
}
