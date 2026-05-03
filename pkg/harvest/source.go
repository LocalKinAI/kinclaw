package harvest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SourceCacheDir is the on-disk cache root for cloned sources.
// First harvest of a source clones into here; subsequent runs `git pull`.
// One cache per source name (not per URL) — renaming a source forces a
// fresh clone, which is fine.
func SourceCacheDir(home, sourceName string) string {
	return filepath.Join(home, ".kinclaw", "harvest", "sources", sourceName)
}

// PullSource ensures the source's git repo exists locally and is up to
// date. Returns the path to the working tree.
//
// First call: shallow clone (depth=1). Subsequent calls: `git pull --ff-only`.
// Local file:// URLs skip git entirely — they're treated as a "use this
// directory directly" pointer (useful for the openclaw private repo case).
//
// Network/git failures surface as errors; the pipeline continues with
// other sources rather than aborting the whole run (caller decides).
func PullSource(ctx context.Context, src Source, home string) (string, error) {
	// Local file:// — point at it directly, no clone.
	if strings.HasPrefix(src.URL, "file://") {
		path := strings.TrimPrefix(src.URL, "file://")
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("file:// source %s: %w", path, err)
		}
		return path, nil
	}

	cacheDir := SourceCacheDir(home, src.Name)
	parent := filepath.Dir(cacheDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("mkdir cache parent: %w", err)
	}

	// Already cloned? Update.
	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); err == nil {
		return cacheDir, gitPull(ctx, cacheDir)
	}

	// Fresh clone.
	args := []string{"clone", "--depth=1"}
	if src.Branch != "" {
		args = append(args, "--branch", src.Branch)
	}
	args = append(args, src.URL, cacheDir)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone %s: %w\n%s", src.URL, err, truncate(string(out), 800))
	}
	return cacheDir, nil
}

func gitPull(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "pull", "--ff-only", "--quiet")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// `git pull` from a shallow clone can fail with "no tracking" or
		// detached HEAD. Fetch+reset is more forgiving but also more
		// destructive — for v1, just surface the failure clearly.
		return fmt.Errorf("git pull in %s: %w\n%s", dir, err, truncate(string(out), 800))
	}
	return nil
}

// FindLicense looks for a LICENSE / LICENSE.md / COPYING file at the
// repo root and returns its detected SPDX-ish identifier. Best-effort
// detection by header keyword — covers the three licenses the manifest
// allowlist will mostly contain (MIT / Apache-2.0 / BSD-3-Clause).
// Returns "" when no license file is found, which the gate treats as
// "unknown" (rejected unless allowlist contains "*").
func FindLicense(repoDir string) string {
	for _, name := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"} {
		path := filepath.Join(repoDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		head := strings.ToLower(string(data))
		if len(head) > 4096 {
			head = head[:4096]
		}
		switch {
		case strings.Contains(head, "mit license"):
			return "MIT"
		// MIT body without the literal "MIT License" title — common
		// when authors paste the MIT body straight after a copyright
		// line (e.g. kylehughes/apple-platform-build-tools). The
		// "permission is hereby granted, free of charge" + "without
		// restriction" pair is the MIT preamble's signature phrase.
		case strings.Contains(head, "permission is hereby granted, free of charge") &&
			strings.Contains(head, "without restriction"):
			return "MIT"
		case strings.Contains(head, "apache license") && strings.Contains(head, "version 2.0"):
			return "Apache-2.0"
		case strings.Contains(head, "bsd 3-clause") || strings.Contains(head, `redistributions in binary form must reproduce`) && strings.Contains(head, "neither the name"):
			return "BSD-3-Clause"
		case strings.Contains(head, "bsd 2-clause"):
			return "BSD-2-Clause"
		case strings.Contains(head, "mozilla public license"):
			return "MPL-2.0"
		case strings.Contains(head, "gnu general public license"):
			if strings.Contains(head, "version 3") {
				return "GPL-3.0"
			}
			return "GPL-2.0"
		case strings.Contains(head, "all rights reserved"):
			// "© <Company>. All rights reserved" + commercial terms
			// links — not an OSS license. Tag explicitly so the reject
			// message says "proprietary" instead of "(none detected)".
			return "proprietary"
		}
	}
	return ""
}

// LicenseAllowed checks whether the detected license is in the allowlist.
// "*" in the allowlist short-circuits to true (used for private repos
// the user owns). Empty allowlist defaults to MIT/Apache-2.0/BSD-3-Clause.
func LicenseAllowed(license string, allowlist []string) bool {
	if len(allowlist) == 0 {
		allowlist = []string{"MIT", "Apache-2.0", "BSD-3-Clause"}
	}
	for _, a := range allowlist {
		if a == "*" {
			return true
		}
		if a == license {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}

// withTimeout returns a context with the given timeout, or the parent
// unchanged if timeout is zero. Helper to keep call sites tidy.
func withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, d)
}
