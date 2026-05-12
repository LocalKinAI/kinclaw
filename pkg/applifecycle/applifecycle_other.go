//go:build !darwin

// Non-darwin stub for the applifecycle package. The macOS implementation
// snapshots user-facing apps via osascript and quits ones kinclaw
// opened. Linux equivalent (wmctrl-based or D-Bus listdesktop) and
// Windows equivalent (UIA tree of running apps) are future work.
//
// For now we return empty snapshots so kinclaw on Linux/Windows simply
// doesn't auto-quit apps at exit — same as if the user had passed an
// imaginary --no-cleanup flag.
package applifecycle

// RunningApps returns nil on non-darwin. main.go records this as the
// pre-existing snapshot, so QuitNew below sees zero apps to quit.
func RunningApps() ([]string, error) {
	return nil, nil
}

// QuitNew is a no-op on non-darwin. Both return slices are empty so
// the shutdown log line reads "quit 0 apps, 0 failed" — clean and
// truthful.
func QuitNew(preexisting []string) (quit, failed []string) {
	return nil, nil
}
