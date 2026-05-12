//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// captureScreenJPEG shells out to macOS screencapture(1) and returns
// the resulting JPEG bytes. The server caches the result 800ms so
// this is invoked at most ~1.25 times/sec under heavy polling.
//
// First invocation prompts the user for Screen Recording permission
// in System Settings. While the prompt is unanswered (or denied),
// screencapture writes a blank/black image — the call still succeeds
// from our perspective so the SSE pipeline keeps flowing; the user
// just sees darkness in their Cowork pane until they grant the perm.
//
// Format choice: JPEG at default quality (~75) — small enough that a
// 1440p capture lands ~150-300KB, big enough that text on the screen
// remains legible. PNG would be lossless but 5-10x bigger and the
// extra bandwidth doesn't help an agent that's parsing the frame
// through a vision model.
func captureScreenJPEG() ([]byte, error) {
	tmp, err := os.CreateTemp("", "kinclaw-screen-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	// -x   no shutter sound
	// -t   format
	// -S   capture screen instead of window
	// -o   no shadow
	// -C   capture cursor (so the agent sees what the user is pointing at)
	cmd := exec.Command("/usr/sbin/screencapture",
		"-x", "-t", "jpg", "-S", "-o", "-C", tmpPath)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("screencapture failed: %w", err)
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read capture: %w", err)
	}
	return data, nil
}

// activeAppName returns the localized name of the frontmost app via
// AppleScript. Cheap (~5ms) and matches what the macOS menubar shows.
// Empty string on error — the UI then labels the feed just "LIVE"
// rather than guessing wrong.
func activeAppName() string {
	out, err := exec.Command("/usr/bin/osascript", "-e",
		`tell application "System Events" to get name of first application process whose frontmost is true`,
	).Output()
	if err != nil {
		return ""
	}
	name := string(out)
	// osascript appends a trailing newline.
	for len(name) > 0 && (name[len(name)-1] == '\n' || name[len(name)-1] == ' ') {
		name = name[:len(name)-1]
	}
	return name
}
