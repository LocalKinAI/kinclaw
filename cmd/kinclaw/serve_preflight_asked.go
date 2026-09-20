package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// A system permission dialog is an interruption, and the preflight used to
// raise one at every boot for as long as the grant was missing — which, for a
// binary that is rebuilt often, is most of the time: an ad-hoc signature is
// matched by its cdhash, so every rebuild orphans the grant, and the panel
// restarts the kernel a dozen times a day. One dialog per build says
// everything the hundredth one does.
//
// The ledger below remembers which build last asked for what. A rebuild
// changes the stamp, and that is exactly when the grant went stale and
// asking again is worth it.

// askedLedger is where the ledger lives: ~/.kinclaw/tcc-asked.json.
func askedLedger() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kinclaw", "tcc-asked.json")
}

// buildStamp names this binary the way its grant dies: by path, size and
// mtime. Empty when the binary cannot be examined, which reads as "never
// asked" — the old behaviour, and the safe side to fail on.
func buildStamp(exe string) string {
	info, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s|%d|%d", exe, info.Size(), info.ModTime().UnixNano())
}

// askedAlready reports whether this build has already raised the dialog for
// the named permission.
func askedAlready(ledger, permission, stamp string) bool {
	if ledger == "" || stamp == "" {
		return false
	}
	return readAsked(ledger)[permission] == stamp
}

// rememberAsked records that this build has raised the dialog. Failing to
// write costs one more dialog at the next boot and nothing else, so errors
// are not reported.
func rememberAsked(ledger, permission, stamp string) {
	if ledger == "" || stamp == "" {
		return
	}
	asked := readAsked(ledger)
	asked[permission] = stamp
	data, err := json.MarshalIndent(asked, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(ledger, append(data, '\n'), 0o644)
}

func readAsked(ledger string) map[string]string {
	asked := map[string]string{}
	data, err := os.ReadFile(ledger)
	if err != nil {
		return asked
	}
	if json.Unmarshal(data, &asked) != nil {
		return map[string]string{}
	}
	return asked
}
