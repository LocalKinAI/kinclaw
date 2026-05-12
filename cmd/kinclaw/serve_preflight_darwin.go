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
// and logs the result to stderr. If either is missing, the system TCC
// dialog fires asynchronously and the user gets a Settings shortcut.
//
// Why preflight at startup instead of lazy-on-first-tool: macOS
// suppresses the TCC dialog after a single denial / stale hash record,
// leaving the user stuck. Triggering at boot gives a clean retry path
// the moment the user opens kinclaw — vs only when they happen to call
// a UI action. The chat surface keeps booting either way; the 5 claws
// just fail with an actionable error until granted.
func preflightPermissions() {
	exe, _ := os.Executable()

	// Accessibility (kinax-go). PromptTrust returns immediately.
	if kinax.PromptTrust() {
		fmt.Fprintf(os.Stderr, "[kinclaw] Accessibility ✓ (binary: %s)\n", exe)
	} else {
		fmt.Fprintf(os.Stderr, "[kinclaw] Accessibility ✗ — system dialog fired\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   binary: %s\n", exe)
		fmt.Fprintf(os.Stderr, "[kinclaw]   Click \"Open System Settings\" in the dialog and toggle ON.\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   If no dialog appeared (stale TCC record from previous build),\n")
		fmt.Fprintf(os.Stderr, "[kinclaw]   run: tccutil reset Accessibility && relaunch.\n")
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
