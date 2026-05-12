//go:build !darwin && !linux && !windows

// screen_other.go — catch-all stub for platforms with no native
// screen implementation yet (BSDs, etc.). darwin / linux / windows
// ship dedicated files (screen.go, screen_linux.go, screen_windows.go).

package skill

import (
	"encoding/json"
	"fmt"
)

type screenSkill struct{}

func NewScreenSkill(_ bool, _ string) Skill { return &screenSkill{} }

func (s *screenSkill) Name() string { return "screen" }
func (s *screenSkill) Description() string {
	return "Screen capture (macOS only — unavailable on this platform)."
}
func (s *screenSkill) ToolDef() json.RawMessage {
	return MakeToolDef("screen", s.Description(), nil, nil)
}
func (s *screenSkill) Execute(_ map[string]string) (string, error) {
	return "", fmt.Errorf("screen skill is macOS-only; not available on this platform")
}
