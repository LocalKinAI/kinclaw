//go:build !darwin

package main

import (
	"fmt"
	"os"
	"runtime"
)

// preflightPermissions on non-macOS just logs which platform we're on.
// Linux uses per-call X11/Wayland/D-Bus permissions (no global TCC) and
// Windows has UIPI/UIAccess but it's also per-call. The Linux/Windows
// claws either work or they don't on first invocation — no upfront
// dialog to fire, so we keep this a single ✓ line.
func preflightPermissions() {
	exe, _ := os.Executable()
	fmt.Fprintf(os.Stderr, "[kinclaw] platform=%s · per-call permission model (no preflight)\n", runtime.GOOS)
	fmt.Fprintf(os.Stderr, "[kinclaw]   binary: %s\n", exe)
}
