package harvest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// StagedRoot is the on-disk root for staged candidates.
// Layout: ~/.localkin/harvest/staged/<source>/<skill-name>/{SKILL.md, critic.md, meta.txt}
func StagedRoot(home string) string {
	return filepath.Join(home, ".localkin", "harvest", "staged")
}

// StagedSkill describes one staged candidate as exposed to `harvest --review`.
type StagedSkill struct {
	SourceName  string
	SkillName   string
	StagePath   string         // dir on disk
	CriticVote  CriticDecision // accept / warn / reject (empty if critic skipped)
	GateOK      bool           // true if forge gate passed (always true for staged)
	StagedAt    time.Time
	SourceURL   string
	SkillRelPath string
}

// CleanSourceStage removes any prior staged candidates for `sourceName`,
// so re-running harvest doesn't leave stale entries from a previous
// version of the upstream repo.
func CleanSourceStage(home, sourceName string) error {
	dir := filepath.Join(StagedRoot(home), sourceName)
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// StageCandidate writes the SKILL.md, the critic annotation, and a
// minimal meta.txt to the staging dir for one candidate. Overwrites
// any existing entry at the same path (CleanSourceStage handles the
// per-source wipe; this just lays down the new files).
func StageCandidate(home, sourceName, skillName, skillContent string, critic *CriticVerdict, source Source, skillRel string) (string, error) {
	dir := filepath.Join(StagedRoot(home), sourceName, skillName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir staged dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		return "", fmt.Errorf("write SKILL.md: %w", err)
	}

	if critic != nil {
		var b strings.Builder
		b.WriteString("# Critic review (verdict: ")
		b.WriteString(string(critic.Decision))
		b.WriteString(")\n\n")
		b.WriteString(critic.FullText)
		b.WriteString("\n")
		if err := os.WriteFile(filepath.Join(dir, "critic.md"), []byte(b.String()), 0o644); err != nil {
			return "", fmt.Errorf("write critic.md: %w", err)
		}
	}

	var meta strings.Builder
	fmt.Fprintf(&meta, "source_name=%s\n", sourceName)
	fmt.Fprintf(&meta, "source_url=%s\n", source.URL)
	fmt.Fprintf(&meta, "skill_rel=%s\n", skillRel)
	fmt.Fprintf(&meta, "staged_at=%s\n", time.Now().UTC().Format(time.RFC3339))
	if critic != nil {
		fmt.Fprintf(&meta, "critic_verdict=%s\n", critic.Decision)
	} else {
		fmt.Fprintf(&meta, "critic_verdict=skipped\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.txt"), []byte(meta.String()), 0o644); err != nil {
		return "", fmt.Errorf("write meta.txt: %w", err)
	}
	return dir, nil
}

// ListStaged enumerates every staged candidate currently on disk,
// sorted by source then skill name. Used by `kinclaw harvest --review`.
func ListStaged(home string) ([]StagedSkill, error) {
	root := StagedRoot(home)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []StagedSkill
	for _, sourceDir := range entries {
		if !sourceDir.IsDir() {
			continue
		}
		skillsDir := filepath.Join(root, sourceDir.Name())
		skillEntries, err := os.ReadDir(skillsDir)
		if err != nil {
			continue
		}
		for _, sk := range skillEntries {
			if !sk.IsDir() {
				continue
			}
			st := StagedSkill{
				SourceName: sourceDir.Name(),
				SkillName:  sk.Name(),
				StagePath:  filepath.Join(skillsDir, sk.Name()),
				GateOK:     true, // staged ⇒ already passed
			}
			st.fillFromMeta()
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceName != out[j].SourceName {
			return out[i].SourceName < out[j].SourceName
		}
		return out[i].SkillName < out[j].SkillName
	})
	return out, nil
}

func (s *StagedSkill) fillFromMeta() {
	data, err := os.ReadFile(filepath.Join(s.StagePath, "meta.txt"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "source_url":
			s.SourceURL = v
		case "skill_rel":
			s.SkillRelPath = v
		case "staged_at":
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				s.StagedAt = t
			}
		case "critic_verdict":
			s.CriticVote = CriticDecision(v)
		}
	}
}

// AcceptStaged copies a staged candidate into the live skills dir.
// Refuses to clobber an existing skill — caller has to remove it first.
// `skillID` is "<source>/<skill-name>", matching the format printed by
// `harvest --review`.
func AcceptStaged(home, skillsDir, skillID string) (string, error) {
	source, name, ok := strings.Cut(skillID, "/")
	if !ok {
		return "", fmt.Errorf("skill id %q must be of form <source>/<skill-name>", skillID)
	}
	src := filepath.Join(StagedRoot(home), source, name)
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("staged skill %q: %w", skillID, err)
	}
	dst := filepath.Join(skillsDir, name)
	if _, err := os.Stat(dst); err == nil {
		return "", fmt.Errorf("destination %s already exists; remove it first or rename the staged candidate", dst)
	}
	if err := copyDir(src, dst); err != nil {
		return "", fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}
	// Drop the meta + critic — they're harvest-pipeline artifacts, not
	// part of the live skill. SKILL.md plus any companion script(s) is
	// what the registry needs.
	for _, f := range []string{"meta.txt", "critic.md"} {
		_ = os.Remove(filepath.Join(dst, f))
	}
	return dst, nil
}

// copyDir is a shallow recursive copy. Sufficient for SKILL.md +
// optional companion scripts (web.py, etc.) which is all a forged or
// harvested skill ever has.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(sp, dp); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(sp, dp); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
