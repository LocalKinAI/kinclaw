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
	SourceName   string
	SkillName    string
	StagePath    string           // dir on disk
	Kind         StagedSkillKind  // regular / inspire-forged / procedural
	CriticVote   CriticDecision   // accept / warn / reject (empty if critic skipped)
	GateOK       bool             // true if forge gate passed (always true for non-procedural)
	StagedAt     time.Time
	SourceURL    string
	SkillRelPath string
	DeferReason  string // populated only when Kind == StagedKindProcedural
	FromInspire  bool   // true when meta.txt has from_inspire=true
}

// StagedSkillKind classifies a staging entry. Regular = exec-style
// candidate that parsed cleanly. InspireForged = exec-style candidate
// produced by the coder specialist from a procedural-style original.
// Procedural = original procedural-style content the coder refused as
// non-exec'able; staged for human reading only, can NOT be --accept'd.
type StagedSkillKind string

const (
	StagedKindRegular       StagedSkillKind = "regular"
	StagedKindInspireForged StagedSkillKind = "inspire"
	StagedKindProcedural    StagedSkillKind = "procedural"
)

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

// StageInspiredCandidate writes a candidate that was produced by the
// --inspire flow (coder forged a KinClaw exec-style skill from an
// external procedural-style original). Layout matches StageCandidate
// (so --accept handles it identically) but meta.txt records the
// inspiration provenance, and the directory gains an `inspire/`
// subdir with the original procedural content + the coder's full
// response — useful for reviewing concept alignment manually.
func StageInspiredCandidate(home, sourceName, skillName, forgedSkillContent, originalProcedural, coderRawOutput string, critic *CriticVerdict, source Source, skillRel string) (string, error) {
	dir := filepath.Join(StagedRoot(home), sourceName, skillName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir staged dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(forgedSkillContent), 0o644); err != nil {
		return "", fmt.Errorf("write forged SKILL.md: %w", err)
	}

	inspireDir := filepath.Join(dir, "inspire")
	if err := os.MkdirAll(inspireDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir inspire dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(inspireDir, "original.md"), []byte(originalProcedural), 0o644); err != nil {
		return "", fmt.Errorf("write inspire/original.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(inspireDir, "coder_output.txt"), []byte(coderRawOutput), 0o644); err != nil {
		return "", fmt.Errorf("write inspire/coder_output.txt: %w", err)
	}

	if critic != nil {
		var b strings.Builder
		b.WriteString("# Critic review (verdict: ")
		b.WriteString(string(critic.Decision))
		b.WriteString(", inspire-forged)\n\n")
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
	fmt.Fprintf(&meta, "from_inspire=true\n")
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

// StageProcedural writes a candidate the coder soul refused as
// non-exec'able to a separate _procedural/ subarea under the source.
// These can NOT be promoted via --accept (no shell command to run);
// they're staged purely so the human can browse what concept inspirations
// the harvest run found, even when KinClaw can't natively execute them.
func StageProcedural(home, sourceName, procName, originalProcedural, deferReason, coderRawOutput string, source Source, skillRel string) (string, error) {
	dir := filepath.Join(StagedRoot(home), sourceName, "_procedural", procName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir procedural dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "original.md"), []byte(originalProcedural), 0o644); err != nil {
		return "", fmt.Errorf("write original.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "defer_reason.txt"), []byte(deferReason+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write defer_reason.txt: %w", err)
	}
	if coderRawOutput != "" {
		if err := os.WriteFile(filepath.Join(dir, "coder_output.txt"), []byte(coderRawOutput), 0o644); err != nil {
			return "", fmt.Errorf("write coder_output.txt: %w", err)
		}
	}

	var meta strings.Builder
	fmt.Fprintf(&meta, "source_name=%s\n", sourceName)
	fmt.Fprintf(&meta, "source_url=%s\n", source.URL)
	fmt.Fprintf(&meta, "skill_rel=%s\n", skillRel)
	fmt.Fprintf(&meta, "staged_at=%s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&meta, "verdict=defer_to_procedural\n")
	fmt.Fprintf(&meta, "defer_reason=%s\n", deferReason)
	if err := os.WriteFile(filepath.Join(dir, "meta.txt"), []byte(meta.String()), 0o644); err != nil {
		return "", fmt.Errorf("write meta.txt: %w", err)
	}
	return dir, nil
}

// ListStaged enumerates every staged candidate currently on disk,
// sorted by source then skill name. Used by `kinclaw harvest --review`.
//
// Layout it walks:
//
//	staged/<source>/<skill-name>/SKILL.md       → regular or inspire-forged
//	staged/<source>/_procedural/<name>/         → coder said defer_to_procedural
//
// Inspire-forged entries live in the same dir shape as regular ones —
// the difference is recorded in their meta.txt (`from_inspire=true`)
// and surfaces via StagedSkill.FromInspire / .Kind.
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
			// _procedural is a marker dir that holds the deferred
			// candidates. Recurse one level deeper.
			if sk.Name() == "_procedural" {
				procEntries, err := os.ReadDir(filepath.Join(skillsDir, "_procedural"))
				if err != nil {
					continue
				}
				for _, pe := range procEntries {
					if !pe.IsDir() {
						continue
					}
					st := StagedSkill{
						SourceName: sourceDir.Name(),
						SkillName:  pe.Name(),
						StagePath:  filepath.Join(skillsDir, "_procedural", pe.Name()),
						Kind:       StagedKindProcedural,
						GateOK:     false,
					}
					st.fillFromMeta()
					out = append(out, st)
				}
				continue
			}
			st := StagedSkill{
				SourceName: sourceDir.Name(),
				SkillName:  sk.Name(),
				StagePath:  filepath.Join(skillsDir, sk.Name()),
				Kind:       StagedKindRegular,
				GateOK:     true,
			}
			st.fillFromMeta()
			if st.FromInspire {
				st.Kind = StagedKindInspireForged
			}
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceName != out[j].SourceName {
			return out[i].SourceName < out[j].SourceName
		}
		// Procedural after inspire after regular within the same source.
		if out[i].Kind != out[j].Kind {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		return out[i].SkillName < out[j].SkillName
	})
	return out, nil
}

func kindOrder(k StagedSkillKind) int {
	switch k {
	case StagedKindRegular:
		return 0
	case StagedKindInspireForged:
		return 1
	case StagedKindProcedural:
		return 2
	}
	return 99
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
		case "defer_reason":
			s.DeferReason = v
		case "from_inspire":
			s.FromInspire = v == "true"
		}
	}
}

// AcceptStaged copies a staged candidate into the live skills dir.
// Refuses to clobber an existing skill — caller has to remove it first.
// `skillID` is "<source>/<skill-name>", matching the format printed by
// `harvest --review`.
//
// Procedural deferred candidates (under <source>/_procedural/<name>/)
// are NOT accept-able: they have no SKILL.md to load, just the
// original procedural content + a defer_reason. AcceptStaged refuses
// them with an explanatory error instead of silently copying junk.
func AcceptStaged(home, skillsDir, skillID string) (string, error) {
	source, name, ok := strings.Cut(skillID, "/")
	if !ok {
		return "", fmt.Errorf("skill id %q must be of form <source>/<skill-name>", skillID)
	}
	if name == "_procedural" || strings.HasPrefix(name, "_procedural/") {
		return "", fmt.Errorf("skill id %q points at a deferred procedural candidate — these have no exec form and can't be accepted; "+
			"see staged/<source>/_procedural/<name>/original.md for the inspiration content", skillID)
	}
	src := filepath.Join(StagedRoot(home), source, name)
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("staged skill %q: %w", skillID, err)
	}
	if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil {
		return "", fmt.Errorf("staged skill %q has no SKILL.md (expected at %s); cannot accept", skillID, src)
	}
	dst := filepath.Join(skillsDir, name)
	if _, err := os.Stat(dst); err == nil {
		return "", fmt.Errorf("destination %s already exists; remove it first or rename the staged candidate", dst)
	}
	if err := copyDir(src, dst); err != nil {
		return "", fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}
	// Drop harvest-pipeline artifacts that don't belong in the live skill:
	// meta.txt + critic.md + any inspire/ subdir (which holds the original
	// procedural content + raw coder output, useful only at review time).
	for _, f := range []string{"meta.txt", "critic.md"} {
		_ = os.Remove(filepath.Join(dst, f))
	}
	_ = os.RemoveAll(filepath.Join(dst, "inspire"))
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
