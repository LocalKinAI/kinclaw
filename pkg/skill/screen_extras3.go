//go:build darwin

package skill

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LocalKinAI/sckit-go"
)

// This file groups the v1.14 "live_stream" verbs on the screen claw —
// long-lived sckit.Stream wrappers so a soul can pull frames over
// time without paying the per-frame setup cost of a fresh full
// screenshot. Three verbs:
//
//   live_stream action=start   → opens a stream against display /
//                                 region / window, returns stream_id
//   live_stream action=frame   → returns the next frame as a PNG
//                                 path + image:// marker (~80-160ms
//                                 amortized, vs 250-400ms for a
//                                 fresh-capture-then-encode cycle)
//   live_stream action=stop    → closes the stream, releases the
//                                 underlying SCStream + buffers
//   live_stream action=list    → list active streams (id, target,
//                                 frames pulled, age)
//
// Use case: Cowork wants to check "did the click work" by pulling 1-3
// frames at 200ms intervals — orders of magnitude cheaper than 3 fresh
// full-display captures, AND the stream's drop-old-frame buffer means
// `frame` always returns the most recent visible state, not a stale
// shot from when start() was called.

// activeStream tracks a live sckit.Stream + bookkeeping for the live
// stream registry. The stream itself is goroutine-safe per kinkit
// docs; we add per-stream age + frame counter for `list`.
type activeStream struct {
	id          string
	target      string // human-readable (e.g. "display=1" / "bundle=com.apple.Safari")
	stream      *sckit.Stream
	startedAt   time.Time
	framesPulled int
}

// liveStreams holds all active live streams keyed by id. Module-level
// because the screenSkill struct gets re-created on soul switch but
// in-flight streams are per-process. Mutex-guarded — `frame` and `stop`
// can be called from different turns concurrently if pilot is racing
// itself somehow.
var (
	liveStreamsMu sync.Mutex
	liveStreams   = map[string]*activeStream{}
	liveStreamSeq int
)

// liveStream is the unified entry point for action=live_stream on the
// screen skill. Sub-action (start / frame / stop / list) is taken from
// the `mode` param so we can reuse `region`, `bundle_id`, `display_id`,
// `output_path` from the rest of the screen verb set.
func (s *screenSkill) liveStream(ctx context.Context, params map[string]string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(params["mode"]))
	if mode == "" {
		mode = "start"
	}
	switch mode {
	case "start":
		return s.liveStreamStart(ctx, params)
	case "frame":
		return s.liveStreamFrame(ctx, params)
	case "stop":
		return s.liveStreamStop(params)
	case "list":
		return s.liveStreamList()
	default:
		return "", fmt.Errorf("live_stream: mode must be start | frame | stop | list (got %q)", mode)
	}
}

func (s *screenSkill) liveStreamStart(ctx context.Context, params map[string]string) (string, error) {
	// Decide target by precedence:
	//   bundle_id   → window / app
	//   region=...  → sub-rect of display
	//   else        → full display
	displays, err := sckit.ListDisplays(ctx)
	if err != nil {
		return "", fmt.Errorf("live_stream start: ListDisplays: %w", err)
	}
	if len(displays) == 0 {
		return "", fmt.Errorf("live_stream start: no displays available")
	}
	display := displays[0]
	if want := params["display_id"]; want != "" {
		var found bool
		for _, d := range displays {
			if fmt.Sprintf("%d", d.ID) == want {
				display = d
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("live_stream start: display_id %q not found", want)
		}
	}

	var (
		target sckit.Target
		label  string
	)
	bundle := strings.TrimSpace(params["bundle_id"])
	regionStr := strings.TrimSpace(params["region"])

	switch {
	case bundle != "":
		// Stream the App's composited windows.
		target = sckit.App{BundleID: bundle}
		label = fmt.Sprintf("bundle=%s", bundle)
	case regionStr != "":
		parts := strings.Split(regionStr, ",")
		if len(parts) != 4 {
			return "", fmt.Errorf("live_stream start: region must be x,y,w,h")
		}
		nums := [4]int{}
		for i, p := range parts {
			n, err := parseIntPart(strings.TrimSpace(p))
			if err != nil {
				return "", fmt.Errorf("live_stream start region: %w", err)
			}
			nums[i] = n
		}
		target = sckit.Region{
			Display: display,
			Bounds:  image.Rect(nums[0], nums[1], nums[0]+nums[2], nums[1]+nums[3]),
		}
		label = fmt.Sprintf("display=%d region=(%d,%d %dx%d)",
			display.ID, nums[0], nums[1], nums[2], nums[3])
	default:
		target = display
		label = fmt.Sprintf("display=%d", display.ID)
	}

	// FPS option — default 30 unless caller overrides. Higher fps =
	// more buffer pressure, lower fps = staler "frame" results.
	var opts []sckit.Option
	if fps := atoiDefault(params["fps"], 30); fps > 0 {
		opts = append(opts, sckit.WithFrameRate(fps))
	}

	streamCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	stream, err := sckit.NewStream(streamCtx, target, opts...)
	if err != nil {
		return "", fmt.Errorf("live_stream start: NewStream: %w", err)
	}

	liveStreamsMu.Lock()
	liveStreamSeq++
	id := fmt.Sprintf("ls-%04d", liveStreamSeq)
	liveStreams[id] = &activeStream{
		id:        id,
		target:    label,
		stream:    stream,
		startedAt: time.Now(),
	}
	liveStreamsMu.Unlock()

	return fmt.Sprintf("live_stream started: id=%s target=%s (%dx%d)",
		id, label, stream.Width(), stream.Height()), nil
}

func (s *screenSkill) liveStreamFrame(ctx context.Context, params map[string]string) (string, error) {
	id := strings.TrimSpace(params["id"])
	if id == "" {
		return "", fmt.Errorf("live_stream frame: id is required (from start)")
	}
	liveStreamsMu.Lock()
	rec, ok := liveStreams[id]
	liveStreamsMu.Unlock()
	if !ok {
		return "", fmt.Errorf("live_stream frame: id %q not found (started? expired?)", id)
	}

	// Per-call frame fetch deadline. Streams typically deliver frames
	// at the configured fps; if we're idle it can take up to 1/fps.
	frameCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	img, err := rec.stream.NextFrame(frameCtx)
	if err != nil {
		return "", fmt.Errorf("live_stream frame: NextFrame: %w", err)
	}
	liveStreamsMu.Lock()
	rec.framesPulled++
	liveStreamsMu.Unlock()

	outPath := params["output_path"]
	if outPath == "" {
		if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("live_stream frame: mkdir %s: %w", s.outputDir, err)
		}
		ts := time.Now().Format("20060102-150405.000")
		outPath = filepath.Join(s.outputDir, fmt.Sprintf("ls-%s-%s.png", id, ts))
	}
	if err := writePNG(outPath, img); err != nil {
		return "", err
	}
	bounds := img.Bounds()
	return fmt.Sprintf(
		"path: %s\nimage://%s\ndimensions: %dx%d\nstream_id: %s\nframe_n: %d",
		outPath, outPath, bounds.Dx(), bounds.Dy(), id, rec.framesPulled,
	), nil
}

func (s *screenSkill) liveStreamStop(params map[string]string) (string, error) {
	id := strings.TrimSpace(params["id"])
	if id == "" {
		return "", fmt.Errorf("live_stream stop: id is required")
	}
	liveStreamsMu.Lock()
	rec, ok := liveStreams[id]
	delete(liveStreams, id)
	liveStreamsMu.Unlock()
	if !ok {
		return "", fmt.Errorf("live_stream stop: id %q not found", id)
	}
	if err := rec.stream.Close(); err != nil {
		return "", fmt.Errorf("live_stream stop: Close: %w", err)
	}
	dur := time.Since(rec.startedAt).Round(time.Second)
	return fmt.Sprintf("live_stream stopped: id=%s ran for %s pulled %d frame(s)",
		id, dur, rec.framesPulled), nil
}

func (s *screenSkill) liveStreamList() (string, error) {
	liveStreamsMu.Lock()
	defer liveStreamsMu.Unlock()
	if len(liveStreams) == 0 {
		return "no active live streams", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d active stream(s):\n", len(liveStreams))
	for _, r := range liveStreams {
		fmt.Fprintf(&sb, "  id=%s  target=%s  frames=%d  age=%s\n",
			r.id, r.target, r.framesPulled,
			time.Since(r.startedAt).Round(time.Second))
	}
	return sb.String(), nil
}
