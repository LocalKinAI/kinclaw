//go:build !darwin

// Non-darwin stub for `kinclaw probe`. The probe subcommand walks the
// macOS Accessibility tree (kinax-go) to score app controllability —
// AT-SPI 2 / UI Automation equivalents are future work.
package main

import (
	"fmt"
	"os"
	"runtime"
)

// runProbe on non-darwin prints a short notice and exits non-zero so
// scripts that gate on the verdict (e.g. `kinclaw probe Foo && step2`)
// fail loud instead of silently passing.
func runProbe(args []string) {
	fmt.Fprintf(os.Stderr,
		"kinclaw probe: not yet supported on %s — walks macOS Accessibility tree.\n"+
			"  Linux: AT-SPI 2 walker is planned for a future release.\n"+
			"  Windows: UI Automation walker is planned for a future release.\n",
		runtime.GOOS)
	os.Exit(2)
}
