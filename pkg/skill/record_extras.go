//go:build darwin

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LocalKinAI/kinrec"
)

// This file groups the v1.14 "record claw 100%" verbs:
//   - clip: synchronous record-N-seconds-and-return (no manual stop)
//   - list_mics: enumerate microphone devices
//
// Region / window-restricted recording is NOT implemented here:
// kinrec at v0.x only supports full-display capture. Adding
// WithRegion / WithWindow to kinrec is a kit-level change deferred
// to a future kinkit release. The skill returns a clean
// "not yet implemented" error so the verb name appears in the
// surface map without faking capability.

// ---------------------------------------------------------------------
// clip — record for a fixed duration, then return.
//
// Useful for "record a 30s demo of this workflow" — the model doesn't
// have to track a recording_id and remember to stop. kinrec.Record
// is exactly this synchronous helper; we wrap it.
// ---------------------------------------------------------------------

func (s *recordSkill) clip(ctx context.Context, params map[string]string) (string, error) {
	durationSec := atoiDefault(params["duration"], 0)
	if durationSec <= 0 {
		return "", fmt.Errorf("clip: duration (seconds) is required and must be > 0")
	}
	if durationSec > 300 {
		// 5 min cap — protect against typo'd "duration=3600" tying up
		// the disk + permissions. Long recordings should use start/stop
		// explicitly so the user can react.
		return "", fmt.Errorf("clip: duration capped at 300s (got %d) — use start/stop for longer recordings", durationSec)
	}

	outPath := strings.TrimSpace(params["output_path"])
	if outPath == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		outPath = filepath.Join(s.outputDir, fmt.Sprintf("clip-%s.mp4", ts))
	}
	outPath = expandHome(outPath)

	opts := []kinrec.Option{kinrec.WithOutput(outPath)}

	// Audio: off by default (privacy). Caller opts in.
	if parseBoolParam(params["audio"], false) {
		opts = append(opts, kinrec.WithAudio(true))
	}
	if parseBoolParam(params["mic"], false) {
		opts = append(opts, kinrec.WithMic(true))
		if md := strings.TrimSpace(params["mic_device"]); md != "" {
			opts = append(opts, kinrec.WithMicDevice(md))
		}
	}

	// Synchronous record. Blocks for `duration` seconds.
	clipCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+10)*time.Second)
	defer cancel()
	if err := kinrec.Record(clipCtx, time.Duration(durationSec)*time.Second, opts...); err != nil {
		return "", fmt.Errorf("clip: kinrec.Record: %w", err)
	}

	// Sanity: file exists + non-empty.
	info, err := os.Stat(outPath)
	if err != nil {
		return "", fmt.Errorf("clip: output file %s not produced: %w", outPath, err)
	}
	return fmt.Sprintf(
		"clipped %ds → %s (%.1f MB)",
		durationSec, outPath, float64(info.Size())/(1<<20),
	), nil
}

// ---------------------------------------------------------------------
// list_mics — enumerate microphone input devices.
//
// Each entry: UniqueID (pass to mic_device to force selection),
// human-readable Name, IsDefault flag. Useful before clip/start
// when the user has multiple mics (built-in + USB headset + virtual)
// and needs to pick one.
// ---------------------------------------------------------------------

func (s *recordSkill) listMics(ctx context.Context) (string, error) {
	mics, err := kinrec.ListMics(ctx)
	if err != nil {
		return "", fmt.Errorf("list_mics: %w", err)
	}
	if len(mics) == 0 {
		return "no microphone devices available (check Microphone permission)", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d mic(s):\n", len(mics))
	for _, m := range mics {
		mark := ""
		if m.IsDefault {
			mark = " (default)"
		}
		fmt.Fprintf(&sb, "  uniqueID=%s  name=%q%s\n", m.UniqueID, m.Name, mark)
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------
// region / window — NOT YET. kinrec at v0.x only supports full-display
// capture. The skill returns a clear error so callers know it's a kit
// gap, not a permission / setup issue.
// ---------------------------------------------------------------------

func (s *recordSkill) regionStub(_ context.Context, _ map[string]string) (string, error) {
	return "", fmt.Errorf("record region: not yet supported — kinrec captures full display only. Track: kinkit roadmap. Workaround: record full screen, crop in post via ffmpeg")
}

func (s *recordSkill) windowStub(_ context.Context, _ map[string]string) (string, error) {
	return "", fmt.Errorf("record window: not yet supported — kinrec captures full display only. Track: kinkit roadmap. Workaround: bring target window to front + record full screen, or use `screen screenshot bundle_id=...` for stills")
}

// ---------------------------------------------------------------------
// record_with_ax — clip + AX event stream sidecar.
//
// Records a video clip AND a parallel JSONL of AX events (focus
// changes, value changes, menu opens, window creation, app switches)
// with timestamps relative to the recording start. Output: one MP4
// + one .ax.jsonl alongside it.
//
// Use case: forge harvest. The user does a demo (clicks around an
// app for 30s). Output:
//   demo-20260507.mp4              ← the visual record
//   demo-20260507.mp4.json         ← provenance (id, soul, task)
//   demo-20260507.mp4.ax.jsonl     ← every AX event with t-offset
//
// A model can read .ax.jsonl + watch the .mp4 and produce a
// SKILL.md that replays the flow at the AX level. Replay is more
// robust than pixel-coordinate macro because AX identity survives
// resolution / window-position changes.
// ---------------------------------------------------------------------

func (s *recordSkill) recordWithAX(ctx context.Context, params map[string]string) (string, error) {
	durationSec := atoiDefault(params["duration"], 0)
	if durationSec <= 0 {
		return "", fmt.Errorf("record_with_ax: duration (seconds) required and must be > 0")
	}
	if durationSec > 300 {
		return "", fmt.Errorf("record_with_ax: duration capped at 300s (got %d)", durationSec)
	}

	outPath := strings.TrimSpace(params["output_path"])
	if outPath == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		outPath = filepath.Join(s.outputDir, fmt.Sprintf("demo-%s.mp4", ts))
	}
	outPath = expandHome(outPath)
	axPath := outPath + ".ax.jsonl"

	// Resolve AX target (frontmost app at recording-start, unless
	// caller forces bundle_id / pid). Reuse the existing
	// observerTarget helper from input_extras.go.
	axParams := map[string]string{
		"bundle_id": params["ax_bundle_id"],
		"pid":       params["ax_pid"],
	}
	app, pid, err := observerTarget(axParams)
	if err != nil {
		return "", fmt.Errorf("record_with_ax: AX target: %w", err)
	}
	defer app.Close()

	// Run video recording (synchronous via kinrec.Record helper) +
	// AX observer (also goroutine-coordinated) in parallel.
	type result struct {
		err error
	}
	videoDone := make(chan result, 1)
	axDone := make(chan struct {
		transcript *axTranscript
		err        error
	}, 1)

	durationMs := durationSec * 1000

	// Goroutine A: video.
	go func() {
		opts := []kinrec.Option{kinrec.WithOutput(outPath)}
		if parseBoolParam(params["audio"], false) {
			opts = append(opts, kinrec.WithAudio(true))
		}
		if parseBoolParam(params["mic"], false) {
			opts = append(opts, kinrec.WithMic(true))
		}
		recCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+15)*time.Second)
		defer cancel()
		err := kinrec.Record(recCtx, time.Duration(durationSec)*time.Second, opts...)
		videoDone <- result{err}
	}()

	// Goroutine B: AX observer.
	go func() {
		notifications := splitCSV(params["events"])
		if len(notifications) == 0 {
			notifications = defaultRecordNotifications()
		}
		obsCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+15)*time.Second)
		defer cancel()
		t, err := observeAXEvents(obsCtx, app, pid, notifications, durationMs)
		axDone <- struct {
			transcript *axTranscript
			err        error
		}{t, err}
	}()

	videoRes := <-videoDone
	axRes := <-axDone
	if videoRes.err != nil {
		return "", fmt.Errorf("record_with_ax: video: %w", videoRes.err)
	}
	if axRes.err != nil {
		// Video was already saved; AX failure is non-fatal — we just
		// don't have the .ax.jsonl. Surface in the result so the
		// caller knows.
		return fmt.Sprintf(
			"record_with_ax: video saved → %s (warning: AX observer failed: %v — no .ax.jsonl)",
			outPath, axRes.err,
		), nil
	}

	if err := os.WriteFile(axPath, []byte(axRes.transcript.String()), 0o644); err != nil {
		return "", fmt.Errorf("record_with_ax: write ax sidecar: %w", err)
	}

	info, _ := os.Stat(outPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return fmt.Sprintf(
		"record_with_ax: %ds clip\n  video: %s (%.1f MB)\n  ax_events: %s (%d events, pid=%d)",
		durationSec, outPath, float64(size)/(1<<20),
		axPath, axRes.transcript.eventCount, pid,
	), nil
}

// writeRecordingSidecar writes a JSON file next to the MP4 with all
// the metadata kinclaw knows about the recording (id, session, soul,
// task note, start time, duration, frame count, file size, kinclaw
// version). Returns the sidecar path on success, "" on failure (we
// don't fail the stop call — sidecar is best-effort metadata).
//
// The sidecar approach replaces "embed metadata in MP4 container"
// (which would need kinrec kit work to wire AVMetadataItem into the
// AVAssetWriter). Same use case (replay tools / harvest reads
// recording provenance) without the cross-repo work.
//
// Convention: <recording>.mp4 → <recording>.mp4.json.
//
// Schema is intentionally simple JSON — easy to extend, easy to
// re-emit by other tools (forge harvest, replay UI, marketing
// dashboard).
func writeRecordingSidecar(rec *activeRec, id string, dur time.Duration, size int64, stats interface{}) string {
	type sidecarStats struct {
		Frames       uint32 `json:"frames"`
		AudioBuffers uint32 `json:"audio_buffers"`
	}
	type sidecar struct {
		Schema       string       `json:"schema"`
		ID           string       `json:"recording_id"`
		Path         string       `json:"path"`
		StartedAt    string       `json:"started_at"`
		EndedAt      string       `json:"ended_at"`
		DurationMs   int64        `json:"duration_ms"`
		Bytes        int64        `json:"bytes"`
		Stats        sidecarStats `json:"stats"`
		SessionID    string       `json:"session_id,omitempty"`
		Soul         string       `json:"soul,omitempty"`
		TaskNote     string       `json:"task_note,omitempty"`
		KinclawTool  string       `json:"kinclaw_tool"`
	}

	// Type-assert stats — kinrec.Stats has Frames + AudioBuffers
	// fields. We pass interface{} from the caller because importing
	// kinrec here would couple this helper to that package's exact
	// shape; the caller has the real type and casts in.
	var sStats sidecarStats
	if v, ok := stats.(interface {
		Frames() uint32
		AudioBuffers() uint32
	}); ok {
		sStats.Frames = v.Frames()
		sStats.AudioBuffers = v.AudioBuffers()
	}
	// Fallback: caller may pass the kinrec.Stats struct directly.
	type rawStatsLike struct {
		Frames       uint32
		AudioBuffers uint32
	}
	if v, ok := stats.(rawStatsLike); ok {
		sStats.Frames = v.Frames
		sStats.AudioBuffers = v.AudioBuffers
	}

	out := sidecar{
		Schema:      "kinclaw-recording-sidecar/v1",
		ID:          id,
		Path:        rec.path,
		StartedAt:   rec.started.Format(time.RFC3339Nano),
		EndedAt:     time.Now().Format(time.RFC3339Nano),
		DurationMs:  dur.Milliseconds(),
		Bytes:       size,
		Stats:       sStats,
		SessionID:   rec.sessionID,
		Soul:        rec.soul,
		TaskNote:    rec.taskNote,
		KinclawTool: "kinclaw-record",
	}
	blob, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return ""
	}
	sidecarPath := rec.path + ".json"
	if err := os.WriteFile(sidecarPath, blob, 0o644); err != nil {
		return ""
	}
	return sidecarPath
}
