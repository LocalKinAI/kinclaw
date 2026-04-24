// Package clone implements **Soul Clone** — one of KinClaw's two
// fission primitives (the other is Skill Forge, in pkg/skill/forge).
//
// Given an existing soul file, Clone produces N copies that can be
// independently specialized via small frontmatter patches. The
// clones are cheap (kilobytes), fast (milliseconds), and lossless
// (byte-identical parent persona/permissions at birth). Divergence
// is emergent, not designed.
//
// Clones are written next to the parent as
// <parent_stem>.clone_<N>.soul.md. The runtime discovers them on
// next /reload, so clone-then-reload is the canonical flow.
//
// # Why this primitive matters
//
// Task fission — "summarize these 10 emails" — becomes Clone(×10)
// of an email_reader soul, each handed one email ID, results
// reconvened by a conductor. No model calls for the cloning itself;
// the cost is paid only when clones reason.
//
// Over time, Clone + Skill Forge accumulate a per-user fingerprint
// of specialists and skill patches that no competitor can replicate
// without the same usage history. This is the KinClaw moat
// articulated in the project's vision doc.
package clone

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LocalKinAI/kinclaw/pkg/soul"
)

// CloneOptions configures a single Clone call.
type CloneOptions struct {
	// Count is the number of clones to produce. Must be >= 1.
	Count int

	// NameSuffix, if non-empty, overrides the default naming scheme.
	// Default: "<parent_stem>.clone_<index>.soul.md" (1-indexed).
	NameSuffix func(index int) string

	// FrontmatterPatch, if non-nil, is called on each clone's parsed
	// Meta before writing. This is where callers inject per-clone
	// divergence (e.g. distinct names, model choices, bound data).
	FrontmatterPatch func(index int, meta *soul.Meta)

	// DestDir, if non-empty, is where clones land. Default: same
	// directory as the parent soul file.
	DestDir string
}

// Clone reads the parent soul file, produces opts.Count copies per
// the options, and returns the filesystem paths of the written
// clones in clone order (1..N).
//
// Clone is safe to call concurrently on different parents; it is
// NOT safe to call concurrently on the same parent with overlapping
// clone indices (the file writes would race).
func Clone(parentPath string, opts CloneOptions) ([]string, error) {
	if opts.Count < 1 {
		return nil, fmt.Errorf("clone: count must be >= 1, got %d", opts.Count)
	}

	// Read parent raw bytes — we preserve the full contents so the
	// clone inherits the parent's system-prompt body verbatim.
	raw, err := os.ReadFile(parentPath)
	if err != nil {
		return nil, fmt.Errorf("clone: read parent %s: %w", parentPath, err)
	}

	// Parse once to validate and, if the caller patches meta, to
	// produce a patched frontmatter we can splice back in.
	parent, err := soul.ParseSoul(raw)
	if err != nil {
		return nil, fmt.Errorf("clone: parse parent: %w", err)
	}

	parentDir := filepath.Dir(parentPath)
	parentStem := strings.TrimSuffix(filepath.Base(parentPath), ".soul.md")
	destDir := opts.DestDir
	if destDir == "" {
		destDir = parentDir
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("clone: mkdir %s: %w", destDir, err)
	}

	paths := make([]string, 0, opts.Count)
	for i := 1; i <= opts.Count; i++ {
		// Compute clone filename.
		var name string
		if opts.NameSuffix != nil {
			name = opts.NameSuffix(i)
			if !strings.HasSuffix(name, ".soul.md") {
				name = name + ".soul.md"
			}
		} else {
			name = fmt.Sprintf("%s.clone_%d.soul.md", parentStem, i)
		}
		out := filepath.Join(destDir, name)

		// If the caller wants to patch frontmatter, re-marshal with
		// the patched meta; otherwise preserve the parent bytes
		// verbatim. Verbatim is cheaper and preserves comments/formatting.
		var body []byte
		if opts.FrontmatterPatch != nil {
			metaCopy := parent.Meta
			opts.FrontmatterPatch(i, &metaCopy)
			body, err = renderSoul(metaCopy, parent.SystemPrompt)
			if err != nil {
				return nil, fmt.Errorf("clone #%d: render: %w", i, err)
			}
		} else {
			body = raw
		}

		if err := os.WriteFile(out, body, 0o644); err != nil {
			return nil, fmt.Errorf("clone #%d: write %s: %w", i, out, err)
		}
		paths = append(paths, out)
	}
	return paths, nil
}

// renderSoul produces a .soul.md representation of (meta, body) —
// used when the caller patches the frontmatter and we need to
// regenerate the file.
//
// We intentionally do NOT include the auto-appended securitySuffix
// that ParseSoul adds to SystemPrompt; that's applied at parse time,
// not persisted on disk. We strip it here if present so round-trips
// don't accumulate duplicates.
func renderSoul(meta soul.Meta, systemPrompt string) ([]byte, error) {
	y, err := marshalMeta(meta)
	if err != nil {
		return nil, err
	}
	body := stripSecuritySuffix(systemPrompt)
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(y)
	if !strings.HasSuffix(string(y), "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("---\n\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// securityMarker is the first line of soul.securitySuffix. We key
// off it to strip the auto-injected security block on round-trip.
const securityMarker = "\n\n## Security\n"

func stripSecuritySuffix(prompt string) string {
	if idx := strings.LastIndex(prompt, securityMarker); idx >= 0 {
		return prompt[:idx]
	}
	return prompt
}
