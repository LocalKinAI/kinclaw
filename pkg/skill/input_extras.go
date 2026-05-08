//go:build darwin

package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LocalKinAI/input-go"
	"github.com/LocalKinAI/kinax-go"
)

// This file groups the v1.13+ "input claw" verbs that go beyond the
// basic move/click/type/hotkey set: paste (clipboard-based fast text
// for long Chinese / IME-fraught content), drag (mousedown→move→up
// as one atomic action), and record_user_input (capture user demo
// for forge harvesting — currently a documented stub pending an
// input-go CGEventTap addition).

// ---------------------------------------------------------------------
// paste — clipboard-based fast text input.
//
// `input.Type` synthesizes a key event per character. For long ASCII
// strings that's fine (~100 chars/s). For Chinese / Japanese / any
// IME-fronted text, character-by-character synthesis frequently
// dropouts mid-stream because the IME's composition state desyncs
// from CGEvent injection. Clipboard paste sidesteps the IME entirely
// — set the clipboard, fire ⌘V, restore the previous clipboard.
//
// Restore semantics: we save the current clipboard text BEFORE the
// paste and write it back AFTER, so the user's clipboard isn't
// silently clobbered. Non-text clipboards (image / file / RTF)
// don't survive the round-trip — by macOS pasteboard design — but
// we restore plain text which is the 95% case.
// ---------------------------------------------------------------------

func (s *inputSkill) paste(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	text := params["text"]
	if text == "" {
		return "", fmt.Errorf("paste: missing `text`")
	}

	restoreClip := parseBoolParam(params["restore_clipboard"], true)

	// 1. Save current clipboard (best-effort).
	prev, _ := pbpaste(ctx)

	// 2. Write our text to the clipboard.
	if err := pbcopy(ctx, text); err != nil {
		return "", fmt.Errorf("paste: pbcopy: %w", err)
	}

	// 3. Fire ⌘V. Use the existing input.Hotkey path so target_pid +
	//    delay opts route consistently with the rest of the skill.
	mods, _ := input.ParseModifiers("cmd")
	v, ok := input.KeyByName("v")
	if !ok {
		return "", fmt.Errorf("paste: cannot resolve key 'v' for ⌘V")
	}
	if err := input.Hotkey(ctx, mods, v, opts...); err != nil {
		return "", fmt.Errorf("paste: ⌘V: %w", err)
	}

	// 4. Tiny settle so the target app reads the clipboard before we
	//    overwrite it. 100ms is generous for non-Electron; Electron
	//    apps can take 200-300ms in pathological cases — bump if you
	//    see truncated paste in Electron.
	time.Sleep(150 * time.Millisecond)

	// 5. Restore previous clipboard.
	if restoreClip && prev != "" {
		if err := pbcopy(ctx, prev); err != nil {
			// Don't fail the paste call — text was already injected.
			// Surface the warning in the result message.
			return fmt.Sprintf("pasted %d chars%s (warning: clipboard restore failed: %v)",
				len([]rune(text)), pidLabel, err), nil
		}
	}
	return fmt.Sprintf("pasted %d chars via clipboard%s", len([]rune(text)), pidLabel), nil
}

// pbcopy / pbpaste shell out to the macOS pasteboard CLI tools. They
// pipe via stdin/stdout so binary-safe up to the user's locale —
// good enough for arbitrary UTF-8 text. For richer typed pasteboard
// content we'd need NSPasteboard via purego; not needed for this
// verb's "fast text" charter.
func pbcopy(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func pbpaste(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "pbpaste").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ---------------------------------------------------------------------
// drag — atomic mousedown → smooth move → mouseup at coordinates.
//
// Routes through input.Drag() (kit-level CGEvent drag), which holds
// the left button down across a duration-paced move and releases at
// the destination. Apps that detect "click without movement" as a
// no-op (Photos, many web canvases, Figma) need this exact sequence.
// ---------------------------------------------------------------------

func (s *inputSkill) drag(ctx context.Context, params map[string]string, opts []input.PostOption, pidLabel string) (string, error) {
	fx, err := parseFloatParam(params, "from_x")
	if err != nil {
		return "", fmt.Errorf("drag: %w", err)
	}
	fy, err := parseFloatParam(params, "from_y")
	if err != nil {
		return "", fmt.Errorf("drag: %w", err)
	}
	tx, err := parseFloatParam(params, "to_x")
	if err != nil {
		return "", fmt.Errorf("drag: %w", err)
	}
	ty, err := parseFloatParam(params, "to_y")
	if err != nil {
		return "", fmt.Errorf("drag: %w", err)
	}
	durationMs := atoiDefault(params["duration_ms"], 200)
	if durationMs < 30 {
		durationMs = 30 // anything faster looks like a click, not a drag
	}
	if durationMs > 5000 {
		durationMs = 5000 // cap so a typo doesn't lock the cursor for 60s
	}

	if err := input.Drag(ctx, fx, fy, tx, ty,
		time.Duration(durationMs)*time.Millisecond, opts...); err != nil {
		return "", fmt.Errorf("drag: %w", err)
	}
	return fmt.Sprintf("drag (%.0f,%.0f)→(%.0f,%.0f) over %dms%s",
		fx, fy, tx, ty, durationMs, pidLabel), nil
}

// ---------------------------------------------------------------------
// record_user_input — capture a user demo via AX-event observation.
//
// **Architectural choice**: rather than CGEventTap-style raw key+click
// capture (which would need kit-level work in input-go to bridge a
// run-loop callback through purego), we use kinax.Observer to
// subscribe to Accessibility notifications on the focused application.
// AX events are higher-level than raw HID — "AXFocusedUIElementChanged
// to AXButton 'Send' on Mail.app" is more replayable than "click at
// (1284, 962)" because AX identity survives screen-resolution and
// window-position changes. This trades capture fidelity (won't catch
// keystrokes per-character) for replay robustness (replays survive
// app updates).
//
// The output is a JSONL transcript of AX events + timestamps. Future
// forge-harvest: feed this to a model with the prompt "convert to a
// kinclaw SKILL.md that replays this flow" — produces a sequence of
// `ui click` / `ui menu_path` / `input type` / etc. calls.
//
// For TRUE keystroke capture, input-go's planned CGEventTap support
// (kinkit v0.5) will add a complementary `record_user_keystrokes`
// verb. But this AX-event recorder ships TODAY and covers most demo
// use cases (any user demo that involves clicking + menu nav + form
// fills shows up at the AX level).
// ---------------------------------------------------------------------

func (s *inputSkill) recordUserInput(ctx context.Context, params map[string]string) (string, error) {
	durationMs := atoiDefault(params["duration_ms"], 30000)
	if durationMs <= 0 {
		durationMs = 30000
	}
	if durationMs > 300000 {
		// 5-minute cap. Long demos should be split into multiple
		// recordings — 5 min of AX events from a busy app already
		// produces a multi-MB JSONL.
		durationMs = 300000
	}

	// Subject: focused app at start time (the user's "active" app).
	// kinax doesn't accept a target by bundle for Observer — must
	// pass an Element + the PID. We resolve via the input-skill's
	// helper-free path: import kinax inline.
	app, pid, err := observerTarget(params)
	if err != nil {
		return "", fmt.Errorf("record_user_input: %w", err)
	}
	defer app.Close()

	// Default subscription set: the events most likely to fire during
	// a typical demo flow. Caller can override via `events=` (CSV).
	notifications := splitCSV(params["events"])
	if len(notifications) == 0 {
		notifications = defaultRecordNotifications()
	}

	transcript, err := observeAXEvents(ctx, app, pid, notifications, durationMs)
	if err != nil {
		return "", err
	}

	// Persist the JSONL to disk so harvest can pick it up later.
	outPath := strings.TrimSpace(params["output_path"])
	if outPath == "" {
		base, _ := os.UserCacheDir()
		if base == "" {
			base = os.TempDir()
		}
		outDir := filepath.Join(base, "kinclaw", "demos")
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return "", fmt.Errorf("record_user_input: mkdir %s: %w", outDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		outPath = filepath.Join(outDir, fmt.Sprintf("demo-%s.jsonl", ts))
	}
	outPath = expandHome(outPath)
	if err := os.WriteFile(outPath, []byte(transcript.String()), 0o644); err != nil {
		return "", fmt.Errorf("record_user_input: write %s: %w", outPath, err)
	}
	return fmt.Sprintf(
		"record_user_input: captured %d AX event(s) over %dms on pid=%d → %s",
		transcript.eventCount, durationMs, pid, outPath,
	), nil
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

func parseFloatParam(p map[string]string, key string) (float64, error) {
	v := strings.TrimSpace(p[key])
	if v == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: bad number %q", key, v)
	}
	return f, nil
}

// ---------------------------------------------------------------------
// AX-event observer helpers used by record_user_input.
// ---------------------------------------------------------------------

// observerTarget resolves the application Element + PID for the
// recording subject. Honors the standard target-picker params
// (bundle_id / pid) but falls back to the user's frontmost app at
// recording-start — typical demo flow ("user clicks record_user_input,
// switches to Mail, clicks around for 30s, recording captures Mail").
func observerTarget(params map[string]string) (*kinax.Element, int, error) {
	if b := params["bundle_id"]; b != "" {
		app, err := kinax.ApplicationByBundleID(b)
		if err != nil {
			return nil, 0, fmt.Errorf("ApplicationByBundleID(%s): %w", b, err)
		}
		// Resolve PID from the Element so caller can pass to
		// NewObserver(pid). kinax-go doesn't expose Element.PID()
		// directly; we read it via attribute.
		pid := atoiDefault(params["pid"], 0)
		if pid == 0 {
			// Fallback: use the AXProcess-pid via raw attribute when
			// caller didn't pass an explicit pid.
			if v, _ := app.AttributeInt("AXProcessIdentifier"); v > 0 {
				pid = int(v)
			}
		}
		return app, pid, nil
	}
	if p := params["pid"]; p != "" {
		pid := atoiDefault(p, 0)
		if pid <= 0 {
			return nil, 0, fmt.Errorf("bad pid %q", p)
		}
		app, err := kinax.ApplicationByPID(pid)
		if err != nil {
			return nil, 0, err
		}
		return app, pid, nil
	}
	app, err := kinax.FocusedApplication()
	if err != nil {
		return nil, 0, err
	}
	return app, kinax.FrontmostPID(), nil
}

// defaultRecordNotifications is the AX-event subscription set the
// demo recorder uses when the caller doesn't specify their own. The
// goal: capture the events that show up during typical user demos —
// click on a button, navigate menus, fill a form, switch windows.
func defaultRecordNotifications() []string {
	return []string{
		kinax.NotifFocusedUIElementChanged, // every "focus moved" event
		kinax.NotifValueChanged,            // text-field edits, slider changes
		kinax.NotifTitleChanged,            // window title shifts (URL bar etc.)
		kinax.NotifSelectedTextChanged,     // selection edits
		kinax.NotifMenuOpened,              // menu navigation start
		kinax.NotifMenuClosed,              // menu commit (close = picked or canceled)
		kinax.NotifWindowCreated,           // new window appears (modal, sheet)
		kinax.NotifApplicationActivated,    // app switch
	}
}

// axTranscript is the running JSONL buffer the recorder writes events
// into. Each line is one event row.
type axTranscript struct {
	buf        bytes.Buffer
	eventCount int
}

func (t *axTranscript) String() string { return t.buf.String() }

func (t *axTranscript) write(line map[string]any) {
	b, err := json.Marshal(line)
	if err != nil {
		return // skip malformed event
	}
	t.buf.Write(b)
	t.buf.WriteByte('\n')
	t.eventCount++
}

// observeAXEvents subscribes the kinax Observer to the given app for
// `durationMs` and returns a transcript of every event. Used by
// record_user_input.
func observeAXEvents(_ context.Context, app *kinax.Element, pid int, notifications []string, durationMs int) (*axTranscript, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("observer needs a valid pid (got %d)", pid)
	}
	obs, err := kinax.NewObserver(pid)
	if err != nil {
		return nil, fmt.Errorf("NewObserver(pid=%d): %w", pid, err)
	}
	defer obs.Close()
	if err := obs.Subscribe(app, notifications...); err != nil {
		return nil, fmt.Errorf("Subscribe: %w", err)
	}

	transcript := &axTranscript{}
	startedAt := time.Now()
	transcript.write(map[string]any{
		"type":          "session_start",
		"timestamp_ms":  0,
		"pid":           pid,
		"notifications": notifications,
	})

	deadline := startedAt.Add(time.Duration(durationMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		// 250ms chunks so we drain promptly near deadline.
		poll := remaining
		if poll > 250*time.Millisecond {
			poll = 250 * time.Millisecond
		}
		ev, err := obs.Next(poll)
		if err != nil {
			// Timeout in this chunk — keep looping until deadline.
			continue
		}
		role, _ := ev.Element.Role()
		title, _ := ev.Element.Title()
		ident, _ := ev.Element.Identifier()
		ev.Element.Close()
		transcript.write(map[string]any{
			"type":         "ax_event",
			"timestamp_ms": time.Since(startedAt).Milliseconds(),
			"notification": ev.Notification,
			"role":         role,
			"title":        title,
			"identifier":   ident,
		})
	}

	transcript.write(map[string]any{
		"type":         "session_end",
		"timestamp_ms": time.Since(startedAt).Milliseconds(),
		"events":       transcript.eventCount,
	})
	return transcript, nil
}
