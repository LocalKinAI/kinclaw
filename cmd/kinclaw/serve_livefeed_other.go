//go:build !darwin

package main

// Non-macOS live-feed stubs. The Cowork pane in the chat UI uses these
// to fetch a thumbnail of the user's screen + the frontmost app name.
// On Linux/Windows, the per-OS screen claw (screen_linux.go /
// screen_windows.go) handles capture but it's heavier — we don't run
// it on the live-feed poll cadence (~1.25Hz) because the user is
// usually not looking at the Cowork pane on those platforms yet.
//
// Returning ([]byte{}, nil) keeps the SSE pipeline flowing; the
// frontend treats zero bytes as "no feed available" and shows the
// text-only chat layout.
func captureScreenJPEG() ([]byte, error) { return nil, nil }
func activeAppName() string              { return "" }
