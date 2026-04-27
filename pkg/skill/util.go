package skill

import (
	"os"
	"path/filepath"
)

// expandHome replaces a leading "~" or "~/" in p with the user's home
// directory. A literal "~" as a filename (e.g. "~foo") is left alone.
// Go's os/filepath doesn't do this — shells do, and CLI users expect it.
//
// Cross-platform on purpose: darwin claws (screen, record, tts) and
// the cross-platform stt skill all need it. Living here means none of
// them have to import each other or duplicate the helper.
func expandHome(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if len(p) > 1 && (p[1] == '/' || p[1] == filepath.Separator) {
		return filepath.Join(home, p[2:])
	}
	return p
}
