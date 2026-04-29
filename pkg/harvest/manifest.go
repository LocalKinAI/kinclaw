// Package harvest implements `kinclaw harvest` — a pipeline that pulls
// candidate skills from third-party agent repos, runs them through the
// forge quality gate v2 + critic soul review, and stages survivors for
// human approval.
//
// Pipeline (per the v1.3.1 design):
//
//	clone source repo (cached + git pull)
//	  → glob skill_paths from manifest
//	  → translate to SKILL.md form (identity for v1; cross-format later)
//	  → critic soul review (annotation, doesn't auto-reject)
//	  → forge quality gate v2 (auto-rejects malformed)
//	  → stage to ~/.localkin/harvest/staged/<source>/<skill-name>/
//
// Final acceptance into ./skills/ is always a manual step. The pipeline
// never auto-merges. This is the explicit design choice (Jacky 2026-04-28):
// "stage 为待审而不是直接 commit. 你 review 后批准才进 main."
package harvest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Manifest is the on-disk shape of ~/.localkin/harvest.toml.
type Manifest struct {
	Sources []Source `toml:"source"`
}

// Source is one [[source]] block in the manifest.
type Source struct {
	Name         string   `toml:"name"`          // unique key, used as staging subdir
	URL          string   `toml:"url"`           // git URL or file:// path for local repos
	SkillPaths   []string `toml:"skill_paths"`   // globs (** supported), relative to repo root
	LicenseAllow []string `toml:"license_allow"` // SPDX IDs, or ["*"] to skip the check
	Branch       string   `toml:"branch"`        // optional, defaults to repo's default branch
}

// LoadManifest reads + parses a TOML manifest. Returns a wrapped error
// with the path so callers can show the user where the bad config lives.
func LoadManifest(path string) (*Manifest, error) {
	if path == "" {
		return nil, fmt.Errorf("manifest path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", path, err)
	}
	return &m, nil
}

// Validate checks the manifest is structurally usable. Doesn't try to
// hit the network or filesystem; just verifies fields are present.
func (m *Manifest) Validate() error {
	if len(m.Sources) == 0 {
		return fmt.Errorf("no [[source]] entries; nothing to harvest")
	}
	seen := map[string]bool{}
	for i, s := range m.Sources {
		if s.Name == "" {
			return fmt.Errorf("source[%d]: name is required", i)
		}
		if seen[s.Name] {
			return fmt.Errorf("source[%d]: duplicate name %q", i, s.Name)
		}
		seen[s.Name] = true
		if s.URL == "" {
			return fmt.Errorf("source[%d] %s: url is required", i, s.Name)
		}
		if len(s.SkillPaths) == 0 {
			return fmt.Errorf("source[%d] %s: skill_paths must list at least one glob", i, s.Name)
		}
	}
	return nil
}

// FindSource returns the source with matching name, or nil if not found.
func (m *Manifest) FindSource(name string) *Source {
	for i := range m.Sources {
		if m.Sources[i].Name == name {
			return &m.Sources[i]
		}
	}
	return nil
}

// DefaultManifestPath is ~/.localkin/harvest.toml (canonical location).
// Falls back to ./harvest.toml if HOME is unset (CI / sandbox edge cases).
func DefaultManifestPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "harvest.toml"
	}
	return filepath.Join(home, ".localkin", "harvest.toml")
}
