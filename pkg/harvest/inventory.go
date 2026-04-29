package harvest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LocalKinAI/kinclaw/pkg/skill"
)

// SkillInventory captures the current `./skills/` directory state at
// the start of a harvest run. Injected into the curator's per-candidate
// prompt so the LLM can decide "is this a genuine gap or do we already
// have it?" with grounded data instead of memory.
type SkillInventory struct {
	Skills []InventorySkill // sorted by name
}

// InventorySkill is one entry in the inventory — a name + a one-line
// description, what curator needs to detect duplicates.
type InventorySkill struct {
	Name        string
	Description string
}

// LoadInventory walks dir (typically `./skills/`) and parses each
// SKILL.md it finds, extracting the name + description. Failures on
// individual files are silently skipped — the inventory is best-
// effort context, not a contract.
func LoadInventory(dir string) (*SkillInventory, error) {
	inv := &SkillInventory{}
	if dir == "" {
		return inv, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return inv, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Each subdir holds a SKILL.md. Skip dirs without one (could
		// be a docs / assets / library subfolder).
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		s, err := skill.LoadExternalSkill(path)
		if err != nil {
			continue
		}
		desc := s.Description()
		// Curator wants ONE sentence — trim multi-line / aggressively
		// shorten so the inventory block stays compact even with 20+
		// skills. ~80 chars is enough for "wraps X for Y".
		desc = firstSentence(desc)
		if len(desc) > 120 {
			desc = desc[:117] + "..."
		}
		inv.Skills = append(inv.Skills, InventorySkill{
			Name:        s.Name(),
			Description: desc,
		})
	}
	sort.Slice(inv.Skills, func(i, j int) bool {
		return inv.Skills[i].Name < inv.Skills[j].Name
	})
	return inv, nil
}

// String renders the inventory as the markdown block the curator
// soul's prompt expects under `## current_skills`. One line per skill,
// "  name — description" indented two spaces.
func (inv *SkillInventory) String() string {
	if inv == nil || len(inv.Skills) == 0 {
		return "  (empty — no skills installed yet)"
	}
	var b strings.Builder
	for _, s := range inv.Skills {
		fmt.Fprintf(&b, "  %s — %s\n", s.Name, s.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}

// firstSentence returns the input compacted to ~one sentence, suitable
// for the inventory's `name — description` line. Boundaries:
//
//	paragraph break (\n\n)        keep period if any, take everything before
//	line break (\n)               keep period if any, take everything before
//	". " sentence-end             cut before the period (drops trailing period)
//	otherwise                     trim a trailing "." but preserve in-word
//	                              periods like wttr.in / linear.app / etc.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "\n\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	if i := strings.Index(s, ". "); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(strings.TrimRight(s, "."))
}
