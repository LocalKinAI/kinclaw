// Package applifecycle tracks which apps were running when kinclaw started
// so it can quit ones IT opened on the way out, leaving the user's pre-
// existing workspace untouched.
//
// The contract is "snapshot at start, quit-new at end" — kinclaw never
// quits an app the user already had open. If the user starts kinclaw
// with Calculator already running, Calculator stays running. If the
// agent opens Reminders during a task, Reminders gets quit at exit (via
// graceful AppleScript `quit`, which respects unsaved-work dialogs).
//
// Why osascript and not pkill: we want graceful shutdown that lets apps
// flush state and prompt the user to save anything unsaved. SIGTERM on
// AppKit apps doesn't always run -applicationWillTerminate; AppleScript
// quit does, and surfaces save dialogs the user expects to see.
package applifecycle

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RunningApps returns the bundle IDs of currently-running, user-facing
// (non-background-only) apps. System processes (Dock, WindowServer,
// loginwindow, etc.) are filtered out by `background only is false`.
//
// On error, returns nil + the error — callers should treat it as a
// "skip cleanup, surface a warning" signal rather than abort.
func RunningApps() ([]string, error) {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get bundle identifier of every process whose background only is false`).Output()
	if err != nil {
		return nil, fmt.Errorf("osascript list processes: %w", err)
	}
	return parseAppleScriptList(string(out)), nil
}

// QuitNew quits any currently-running app whose bundle ID is NOT in
// `preexisting`. Returns the lists of bundles that quit cleanly and
// those that refused (e.g. unsaved-work modal blocking, or app
// declined the AE quit event).
//
// Each quit is best-effort with a 3s budget — apps that show a save
// dialog return ~immediately from `quit` (the dialog is the user's
// responsibility to handle), so the budget is mostly to bound the
// "app ignores us" case.
func QuitNew(preexisting []string) (quit, failed []string) {
	current, err := RunningApps()
	if err != nil {
		return nil, nil
	}
	pre := make(map[string]struct{}, len(preexisting))
	for _, p := range preexisting {
		pre[p] = struct{}{}
	}
	var newApps []string
	for _, c := range current {
		if _, was := pre[c]; !was {
			newApps = append(newApps, c)
		}
	}
	for _, b := range newApps {
		if quitOne(b, 3*time.Second) {
			quit = append(quit, b)
		} else {
			failed = append(failed, b)
		}
	}
	return quit, failed
}

// quitOne tries `osascript tell application id "X" to quit` with a
// timeout. Returns true if the command exited cleanly (which generally
// means the app accepted the AE event — actual termination is async,
// but for our purposes this is enough).
func quitOne(bundleID string, timeout time.Duration) bool {
	cmd := exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application id %q to quit`, bundleID))
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err == nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return false
	}
}

// parseAppleScriptList turns AppleScript's comma-separated list output
// into a clean string slice. AppleScript's "get every X" returns lines
// like:
//
//	com.apple.finder, com.apple.Safari, com.tencent.xinWeChat
//
// (sometimes with a trailing newline, sometimes not). We split on
// comma, trim whitespace, drop empties.
func parseAppleScriptList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
