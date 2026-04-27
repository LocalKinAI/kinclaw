// resolve.go — turn user-friendly app references into bundle IDs.
//
// `kinclaw probe Notes` and `kinclaw probe com.apple.Notes` should both work.
// The CLI accepts:
//   - bundle ID:   "com.apple.Notes"  → returned as-is
//   - app name:    "Notes"            → resolved by scanning standard app dirs
//   - app path:    "/Applications/Foo.app"  → read CFBundleIdentifier
//
// Resolution order favors deterministic disk lookup (parsing Info.plist via
// `defaults read`) over Spotlight (`mdfind`) because Spotlight indexing is
// unreliable on freshly-installed apps and on dev machines that disabled it.

package probe

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// standardAppDirs lists the directories scanned for app names. Order matters
// — /Applications wins over /System/Applications when both have a same-named
// stub (rare, but happens for apps the user's installed alongside an Apple
// stub).
var standardAppDirs = []string{
	"/Applications",
	"/Applications/Utilities",
	"/System/Applications",
	"/System/Applications/Utilities",
	expandHome("~/Applications"),
}

// ResolveBundleID turns input into a bundle ID. Accepts bundle IDs (returned
// as-is), .app paths (Info.plist read), and bare names ("Notes" → searches
// standard app dirs).
func ResolveBundleID(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("empty input")
	}

	// Already looks like a bundle ID? (contains at least one dot, no slashes,
	// no spaces, all-lowercase-ish). Heuristic — bundle IDs are reverse-DNS.
	if looksLikeBundleID(input) {
		return input, nil
	}

	// Path to a .app? Read its Info.plist.
	if strings.HasSuffix(input, ".app") || strings.Contains(input, "/") {
		path := input
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		if bid, err := bundleIDFromAppPath(path); err == nil {
			return bid, nil
		} else {
			return "", fmt.Errorf("%q: %w", path, err)
		}
	}

	// Bare app name — scan standard dirs for `<input>.app`.
	for _, dir := range standardAppDirs {
		candidate := filepath.Join(dir, input+".app")
		if bid, err := bundleIDFromAppPath(candidate); err == nil {
			return bid, nil
		}
	}

	return "", fmt.Errorf("could not resolve %q to a bundle ID (not a valid bundle ID, not an existing .app path, not found in standard app dirs)", input)
}

// bundleIDFromAppPath shells out to `defaults read` to read CFBundleIdentifier.
// Pure-Go plist parsing would be cleaner but adds a dep for marginal gain;
// `defaults` is always present on macOS and handles binary plists transparently.
func bundleIDFromAppPath(appPath string) (string, error) {
	out, err := exec.Command("defaults", "read",
		filepath.Join(appPath, "Contents", "Info"), "CFBundleIdentifier").Output()
	if err != nil {
		return "", fmt.Errorf("defaults read: %w", err)
	}
	bid := strings.TrimSpace(string(out))
	if bid == "" {
		return "", errors.New("CFBundleIdentifier missing or empty")
	}
	return bid, nil
}

// looksLikeBundleID is the heuristic used to short-circuit resolution. We
// don't need to be perfect — wrong-positives just fall through to the kinax
// "no_process" path.
func looksLikeBundleID(s string) bool {
	if !strings.Contains(s, ".") {
		return false
	}
	if strings.Contains(s, "/") || strings.Contains(s, " ") {
		return false
	}
	// Bundle IDs are typically 2+ segments separated by dots.
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}

// expandHome is a local re-implementation of os/user lookups that matches
// pkg/skill/util.expandHome — kept here to avoid an import cycle. (pkg/skill
// imports pkg/probe wouldn't happen, but going the other way is also a smell.)
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		out, err := exec.Command("sh", "-c", "echo $HOME").Output()
		if err == nil {
			home := strings.TrimSpace(string(out))
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
