package harvest

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// matchGlob is a doublestar-style glob matcher: supports `*`, `?`, and
// `**` (matches any number of path segments including zero). Slash-only
// — patterns + paths use `/` regardless of platform. Behavior chosen to
// match shell globstar semantics so manifest authors can write the
// patterns they expect:
//
//	**/SKILL.md           — any SKILL.md anywhere in the tree
//	skills/**/SKILL.md    — any SKILL.md under skills/, any depth
//	skills/*              — top-level entries under skills/
//	skills/*.md           — top-level *.md files under skills/
//
// We only need this for harvest manifests — globs come from human-edited
// TOML, not adversarial input. Keep the implementation small over
// correctness in pathological cases.
func matchGlob(pattern, path string) bool {
	pParts := splitSlash(pattern)
	sParts := splitSlash(path)
	return matchSegments(pParts, sParts)
}

// matchSegments backtracks: when we hit `**`, try matching it against
// 0, 1, 2, ... leading segments of the remaining path. Without `**`
// it's a straight zip with filepath.Match per segment.
func matchSegments(pParts, sParts []string) bool {
	for len(pParts) > 0 {
		p := pParts[0]
		if p == "**" {
			// Trailing ** matches any remainder, including empty.
			if len(pParts) == 1 {
				return true
			}
			rest := pParts[1:]
			for i := 0; i <= len(sParts); i++ {
				if matchSegments(rest, sParts[i:]) {
					return true
				}
			}
			return false
		}
		if len(sParts) == 0 {
			return false
		}
		ok, err := filepath.Match(p, sParts[0])
		if err != nil || !ok {
			return false
		}
		pParts = pParts[1:]
		sParts = sParts[1:]
	}
	return len(sParts) == 0
}

// splitSlash splits on "/", dropping leading + trailing empties so
// "/a/b/" and "a/b" produce the same parts.
func splitSlash(s string) []string {
	parts := strings.Split(s, "/")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// globFiles walks rootDir and returns every path that matches pattern,
// where the match is computed against the path RELATIVE to rootDir
// (with forward slashes). Skips directories from the result list —
// patterns target files, not dirs.
func globFiles(rootDir string, patterns []string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Always recurse — `**` patterns require descending into every
		// subtree until we know whether anything inside matches.
		if d.IsDir() {
			// Skip .git and node_modules — they're never the answer
			// and they're always huge.
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".venv" {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		for _, pat := range patterns {
			if matchGlob(pat, rel) {
				matches = append(matches, path)
				return nil
			}
		}
		return nil
	})
	return matches, err
}
