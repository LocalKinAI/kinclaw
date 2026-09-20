//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	kinax "github.com/LocalKinAI/kinax-go"
	sckit "github.com/LocalKinAI/sckit-go"
)

// preflightPermissions checks Accessibility + Screen Recording on macOS
// and logs the result to stderr. A missing Accessibility grant raises the
// system dialog — once per build (see serve_preflight_asked.go), not at
// every boot.
//
// Why preflight at startup at all instead of lazy-on-first-tool: a rebuild
// leaves a stale TCC record behind, and the first boot of the new build is
// the clean moment to say so — vs only when the user happens to call a UI
// action. After that one dialog the log line carries the instructions, and
// the ui skill asks again at the moment somebody actually needs the claw.
// The chat surface keeps booting either way; the 5 claws just fail with an
// actionable error until granted.
func preflightPermissions() {
	exe, _ := os.Executable()

	// Accessibility (kinax-go). Trusted never prompts; PromptTrust raises
	// the dialog and returns immediately.
	ledger, stamp := askedLedger(), buildStamp(exe)
	switch {
	case kinax.Trusted():
		fmt.Fprintf(os.Stderr, "[kinclaw] Accessibility ✓ (binary: %s)\n", exe)
	case askedAlready(ledger, "accessibility", stamp):
		fmt.Fprintf(os.Stderr, "[kinclaw] Accessibility ✗ — this build already asked, not asking again\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   binary: %s\n", exe)
		fmt.Fprintf(os.Stderr, "[kinclaw]   System Settings → Privacy & Security → Accessibility: remove any old\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   `kinclaw` entry (a rebuild orphans it even while it reads ON), add the\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   path above, toggle ON. tccutil cannot do this for us: it only knows app\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   bundles, and a bare binary is not one (bare `tccutil reset Accessibility`\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   works, by resetting every app on the machine).\n")
	default:
		_ = kinax.PromptTrust()
		rememberAsked(ledger, "accessibility", stamp)
		fmt.Fprintf(os.Stderr, "[kinclaw] Accessibility ✗ — system dialog fired (once for this build)\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   binary: %s\n", exe)
		fmt.Fprintf(os.Stderr, "[kinclaw]   Click \"Open System Settings\" in the dialog and toggle ON.\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   If the entry already reads ON, it belongs to a previous build:\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   remove it (−) and add this binary again (+). tccutil cannot target a\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   bare binary — it only knows app bundles.\n")
	}

	// Screen Recording (sckit-go). No no-side-effect preflight, so we
	// poke ListDisplays — that triggers the TCC prompt on first call
	// and returns ErrPermissionDenied if not allowed. ~10ms when granted.
	if probeScreenRecording() {
		fmt.Fprintf(os.Stderr, "[kinclaw] Screen Recording ✓ (binary: %s)\n", exe)
	} else {
		fmt.Fprintf(os.Stderr, "[kinclaw] Screen Recording ✗ — system dialog fired (or stale TCC)\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   binary: %s\n", exe)
		fmt.Fprintf(os.Stderr, "[kinclaw]   Click \"Open System Settings\" in the dialog and toggle ON.\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   record / screen skills will fail until granted.\n")
	}
}

// probeScreenRecording does the cheapest TCC probe via ListDisplays.
// True on success, false on ErrPermissionDenied OR any other error.
// "Any other error" conservatively assumes not-granted so the user
// sees the actionable message instead of a silent green ✓.
func probeScreenRecording() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := sckit.ListDisplays(ctx)
	return err == nil
}
