//go:build darwin

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LocalKinAI/kinrec"
)

// recordSkill is the "memory" claw of KinClaw — a video recorder. Wraps
// kinrec (ScreenCaptureKit + AVAudioEngine) so a soul can record the
// macOS screen, including microphone and system audio, while continuing
// to drive the UI in the same process. Unlike `screen` (still images),
// `record` is non-blocking: 'start' returns a recording_id immediately,
// the agent keeps clicking + typing, then 'stop' finalizes the MP4.
//
// First use of system-audio capture triggers Screen Recording TCC
// (already required by the `screen` skill). First mic use triggers
// the separate Microphone TCC prompt.
type recordSkill struct {
	allowed   bool
	outputDir string

	mu      sync.Mutex
	active  map[string]*activeRec
	counter int
}

type activeRec struct {
	rec     *kinrec.Recorder
	path    string
	started time.Time
	// User-supplied metadata that lands in the .mp4.json sidecar
	// when the recording finalizes. Pure passthrough — kinclaw
	// doesn't interpret these fields, just writes them so a future
	// reader (replayer / harvest tool / forge importer) can identify
	// the recording's provenance.
	sessionID string
	soul      string
	taskNote  string
}

// NewRecordSkill returns the record skill. If allowed is false every
// call returns a permission error. outputDir is where MP4 files land
// when the caller doesn't pass an explicit path; empty means
// ~/Library/Caches/kinclaw/recordings/.
func NewRecordSkill(allowed bool, outputDir string) Skill {
	if outputDir == "" {
		base, _ := os.UserCacheDir()
		if base == "" {
			base = os.TempDir()
		}
		outputDir = filepath.Join(base, "kinclaw", "recordings")
	}
	outputDir = expandHome(outputDir)
	return &recordSkill{
		allowed:   allowed,
		outputDir: outputDir,
		active:    make(map[string]*activeRec),
	}
}

func (s *recordSkill) Name() string { return "record" }

func (s *recordSkill) Description() string {
	return "Record the macOS screen to a video file (MP4). " +
		"Actions: " +
		"start (non-blocking, returns recording_id) | stop (finalize) | " +
		"list (active recordings) | stats (frame/audio counters) | " +
		"clip duration=N (synchronous record-N-seconds-and-return — useful " +
		"for short demos when you don't want to track an id) | " +
		"list_mics (enumerate microphone devices) | " +
		"with_ax duration=N (record N-second clip + parallel AX-event " +
		"JSONL sidecar — output is .mp4 + .mp4.ax.jsonl, ideal for " +
		"forge-harvest of user demos into replayable SKILL.md) | " +
		"region (NOT YET — kinrec full-display only; planned for kit upgrade) | " +
		"window (NOT YET — same gap). " +
		"Optional system audio + microphone. Requires Screen Recording " +
		"permission; microphone use also requires Microphone permission."
}

func (s *recordSkill) ToolDef() json.RawMessage {
	return MakeToolDef("record", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "start | stop | list | stats | clip | list_mics | region | window | with_ax",
			},
			"ax_bundle_id": {
				"type":        "string",
				"description": "For action=with_ax: AX-observation target bundle (e.g. 'com.apple.Numbers'). Default: focused app at recording start.",
			},
			"ax_pid": {
				"type":        "string",
				"description": "For action=with_ax: AX-observation target pid (alternative to ax_bundle_id).",
			},
			"events": {
				"type":        "string",
				"description": "For action=with_ax: comma-separated AX notifications to subscribe (default: focus/value/title/menu/window/app activated set).",
			},
			"duration": {
				"type":        "integer",
				"description": "For action=clip: seconds to record (1..300). Synchronous — call returns when the clip is finalized.",
			},
			"output_path": {
				"type":        "string",
				"description": "For action=clip: optional explicit MP4 path. Default: timestamped file under recordings cache dir.",
			},
			"mic_device": {
				"type":        "string",
				"description": "For action=clip with mic=true: UniqueID from `list_mics` to force a specific microphone (e.g. external USB headset). Default: system default mic.",
			},
			"session_id": {
				"type":        "string",
				"description": "For action=start: optional session id to embed in the .mp4.json sidecar metadata. Useful for replay tooling to correlate recordings with chat sessions.",
			},
			"soul": {
				"type":        "string",
				"description": "For action=start: optional soul name to embed in the sidecar (e.g. 'KinClaw Pilot'). Replay tooling uses this to know which agent produced the recording.",
			},
			"task_note": {
				"type":        "string",
				"description": "For action=start: free-form short description of the task (e.g. 'Numbers spreadsheet demo'). Stored in sidecar.",
			},
			"id": {
				"type":        "string",
				"description": "Recording id returned by 'start'. Required for 'stop' and 'stats'.",
			},
			"output": {
				"type":        "string",
				"description": "Optional MP4 output path for 'start'. Default: timestamped file under the recordings cache dir. Leading '~' is expanded.",
			},
			"audio": {
				"type":        "string",
				"description": "Capture system audio in 'start': true|false. Default: false.",
			},
			"mic": {
				"type":        "string",
				"description": "Capture microphone in 'start': true|false. Default: false. First use triggers Microphone TCC prompt.",
			},
			"show_clicks": {
				"type":        "string",
				"description": "Highlight the cursor + clicks in the video for 'start': true|false. Default: true (recommended for demos).",
			},
			"display_id": {
				"type":        "string",
				"description": "Optional CGDirectDisplayID for 'start' (use 'screen list_displays' to enumerate). Default: main display.",
			},
			"fps": {
				"type":        "string",
				"description": "Frame rate for 'start', e.g. '30' or '60'. Default: kinrec default.",
			},
		},
		[]string{"action"},
	)
}

func (s *recordSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `record` capability")
	}
	action := params["action"]
	if action == "" {
		action = "start"
	}
	ctx := context.Background()
	switch action {
	case "start":
		return s.start(params)
	case "stop":
		return s.stop(params)
	case "list":
		return s.list()
	case "stats":
		return s.stats(params)
	case "clip":
		return s.clip(ctx, params)
	case "list_mics":
		return s.listMics(ctx)
	case "region":
		return s.regionStub(ctx, params)
	case "window":
		return s.windowStub(ctx, params)
	case "with_ax":
		return s.recordWithAX(ctx, params)
	default:
		return "", fmt.Errorf("unknown action %q (expected: start | stop | list | stats | clip | list_mics | region | window | with_ax)", action)
	}
}

// parseBoolParam parses a string flag with a default; missing or
// unparsable values fall back to the default rather than erroring,
// which matches how the other claws treat optional bools.
func parseBoolParam(v string, dflt bool) bool {
	if v == "" {
		return dflt
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return dflt
	}
	return b
}

func (s *recordSkill) start(params map[string]string) (string, error) {
	out := params["output"]
	if out == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405")
		out = filepath.Join(s.outputDir, fmt.Sprintf("rec-%s.mp4", ts))
	} else {
		out = expandHome(out)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(out), err)
		}
	}

	opts := []kinrec.Option{
		kinrec.WithOutput(out),
		kinrec.WithAudio(parseBoolParam(params["audio"], false)),
		kinrec.WithMic(parseBoolParam(params["mic"], false)),
		kinrec.WithCursorHighlight(parseBoolParam(params["show_clicks"], true)),
	}
	if v := params["display_id"]; v != "" {
		id, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return "", fmt.Errorf("display_id must be a uint32: %w", err)
		}
		opts = append(opts, kinrec.WithDisplay(uint32(id)))
	}
	if v := params["fps"]; v != "" {
		fps, err := strconv.Atoi(v)
		if err != nil {
			return "", fmt.Errorf("fps must be an integer: %w", err)
		}
		opts = append(opts, kinrec.WithFrameRate(fps))
	}

	// NewRecorder + Start can take a few seconds (TCC prompt on first run,
	// dylib load, AVCaptureSession setup), so allow some headroom.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	r, err := kinrec.NewRecorder(ctx, opts...)
	if err != nil {
		return "", fmt.Errorf("kinrec.NewRecorder: %w", err)
	}
	if err := r.Start(ctx); err != nil {
		return "", fmt.Errorf("recorder.Start: %w", err)
	}

	// Wait for kinrec's capture pipeline to actually deliver its first
	// frame before returning. Without this, `record start` succeeds at
	// the API level but kinrec is still spinning up its
	// ScreenCaptureKit session — any tool call that fires immediately
	// after (osascript activate, ui click, etc.) happens BEFORE the
	// recording sees anything. Live observation: frame 1 of a demo
	// recording showed Calculator already at "2" because all the
	// activation + clicking happened during the warmup window.
	//
	// Cap at 1 second so a TCC-pending or busted kinrec doesn't hang
	// the whole agent forever. If we hit the cap we return success
	// anyway — better to record from frame N+1 than to error out.
	frameDeadline := time.Now().Add(time.Second)
	frameWaited := time.Duration(0)
	for time.Now().Before(frameDeadline) {
		if r.Stats().Frames > 0 {
			frameWaited = time.Since(rec0Time(r))
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	s.mu.Lock()
	s.counter++
	id := fmt.Sprintf("rec-%d-%d", time.Now().Unix(), s.counter)
	s.active[id] = &activeRec{
		rec: r, path: out, started: time.Now(),
		sessionID: strings.TrimSpace(params["session_id"]),
		soul:      strings.TrimSpace(params["soul"]),
		taskNote:  strings.TrimSpace(params["task_note"]),
	}
	s.mu.Unlock()

	warmup := ""
	if frameWaited > 0 {
		warmup = fmt.Sprintf("\nfirst_frame_after: %s", frameWaited.Round(time.Millisecond))
	}
	return fmt.Sprintf("recording_id: %s\npath: %s\nstarted_at: %s%s",
		id, out, time.Now().Format(time.RFC3339), warmup), nil
}

// rec0Time returns a "now" reference suitable for measuring warmup
// latency. Standalone helper so the path through Stats() in the
// caller stays narrow.
func rec0Time(_ *kinrec.Recorder) time.Time { return time.Now() }

func (s *recordSkill) stop(params map[string]string) (string, error) {
	id := params["id"]
	if id == "" {
		return "", fmt.Errorf("stop requires 'id' parameter (returned by a prior start)")
	}
	s.mu.Lock()
	rec, ok := s.active[id]
	if !ok {
		s.mu.Unlock()
		return "", fmt.Errorf("no active recording with id %q", id)
	}
	delete(s.active, id)
	s.mu.Unlock()

	// Stop finalizes the MP4 (writes the moov atom). Give it room.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := rec.rec.Stop(ctx); err != nil {
		return "", fmt.Errorf("recorder.Stop: %w", err)
	}

	dur := time.Since(rec.started).Round(time.Millisecond)
	stats := rec.rec.Stats()
	var size int64
	if fi, err := os.Stat(rec.path); err == nil {
		size = fi.Size()
	}

	// Sidecar metadata. Writing into the MP4 container itself would
	// require kinrec-side AVMetadataItem plumbing (a kit change).
	// A sidecar JSON does the same job for our use case: any tool
	// reading the recording can ask "what soul made this, what task,
	// when did it start/end" by reading the .json next to the .mp4.
	// Convention: <recording>.mp4 → <recording>.mp4.json.
	sidecarPath := writeRecordingSidecar(rec, id, dur, size, stats)

	out := fmt.Sprintf("path: %s\nduration: %s\nbytes: %d\nframes: %d\naudio_buffers: %d",
		rec.path, dur, size, stats.Frames, stats.AudioBuffers)
	if sidecarPath != "" {
		out += "\nsidecar: " + sidecarPath
	}
	return out, nil
}

func (s *recordSkill) list() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.active) == 0 {
		return "no active recordings", nil
	}
	out := fmt.Sprintf("%d active recording(s):\n", len(s.active))
	for id, r := range s.active {
		out += fmt.Sprintf("  %s  path=%s  elapsed=%s\n",
			id, r.path, time.Since(r.started).Round(time.Second))
	}
	return out, nil
}

func (s *recordSkill) stats(params map[string]string) (string, error) {
	id := params["id"]
	if id == "" {
		return "", fmt.Errorf("stats requires 'id' parameter")
	}
	s.mu.Lock()
	rec, ok := s.active[id]
	s.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("no active recording with id %q", id)
	}
	st := rec.rec.Stats()
	return fmt.Sprintf("id: %s\nelapsed: %s\nframes: %d\naudio_buffers: %d\npath: %s",
		id, time.Since(rec.started).Round(time.Millisecond),
		st.Frames, st.AudioBuffers, rec.path), nil
}
