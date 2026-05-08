//go:build darwin

package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"
	"time"

	input "github.com/LocalKinAI/input-go"
	"github.com/LocalKinAI/sckit-go"
)

// smartClickSkill is a cross-claw verb: when AX find can't locate a
// button (Canvas-rendered apps, Electron with stripped accessibility,
// custom-drawn UI like Figma / Numbers / TablePlus), fall back to
// finding text via OCR then clicking the matched coordinate via
// input. This combination unlocks the use case Anthropic's Computer
// Use was built around — visual click — but routes through the
// kinclaw-native stack (sckit OCR + input CGEvent) instead of a
// vision-LLM round-trip.
//
// Pipeline:
//   1. Capture screen (full or region or window via screen claw)
//   2. Run sckit.OCR on the bytes
//   3. Find the region whose Text matches the requested needle
//      (exact / contains / regex modes; pick best confidence)
//   4. Compute center_x / center_y of the region
//   5. Call input.ClickAtButton at that center
//   6. Return: matched text, bbox, confidence, click coords
//
// Why a separate skill (rather than wiring into ui or screen): this
// is genuinely a TWO-claw composition. Putting it in `screen` would
// mean screen-claw needs to know how to call input (creates a loop
// in skill imports). Putting it in `input` would mean input depends
// on sckit OCR (also bad). A standalone composite skill is the
// right home.
type smartClickSkill struct {
	allowed bool
}

// NewSmartClickSkill returns a cross-claw skill that locates UI
// targets by text via OCR and clicks them via input synthesis. It
// inherits the screen skill's permission gate (Screen Recording for
// the OCR pass) AND the input skill's permission gate (Accessibility
// for the click). The constructor takes a single allowed bool that
// callers should AND of both: the soul must grant `screen` AND `input`
// permissions for smart_click to be enabled.
func NewSmartClickSkill(allowed bool) Skill {
	return &smartClickSkill{allowed: allowed}
}

func (s *smartClickSkill) Name() string { return "smart_click" }

func (s *smartClickSkill) Description() string {
	return "Find a UI element by its visible text via OCR, then click " +
		"its center via CGEvent input. Use when the AX tree (`ui find`) " +
		"can't locate the target — typically Canvas-rendered apps " +
		"(Figma / Numbers / Sketch / WebGL surfaces), heavily-styled " +
		"Electron without accessibility metadata, or any UI where the " +
		"button label is rendered as a glyph rather than as an AXTitle. " +
		"This skill chains screen.OCR → text-match → input.click in a " +
		"single tool call instead of three round-trips. Requires both " +
		"Screen Recording (for OCR) and Accessibility (for click) " +
		"permissions."
}

func (s *smartClickSkill) ToolDef() json.RawMessage {
	return MakeToolDef("smart_click", s.Description(),
		map[string]map[string]string{
			"text": {
				"type":        "string",
				"description": "The visible text on the target element. Examples: 'Submit', '提交', 'OK', 'Sign In'. Required.",
			},
			"match": {
				"type":        "string",
				"description": "Match mode: 'exact' (default — case-insensitive equal), 'contains' (case-insensitive substring), 'prefix' (case-insensitive starts-with). Pick exact unless you've already tried it and it didn't find anything.",
			},
			"region": {
				"type":        "string",
				"description": "Optional 'x,y,w,h' to OCR only a sub-rectangle (faster + less ambiguous when multiple matches exist on screen). Coordinates in display-local px.",
			},
			"display_id": {
				"type":        "string",
				"description": "Optional CGDirectDisplayID. Default: main display.",
			},
			"button": {
				"type":        "string",
				"description": "Mouse button: left (default) | right | other. Right-click for context menus.",
			},
			"clicks": {
				"type":        "integer",
				"description": "Click count: 1 (default), 2 (double-click). Use 2 for opening files in Finder, selecting words, etc.",
			},
			"min_confidence": {
				"type":        "string",
				"description": "Minimum OCR confidence (0.0..1.0) to accept a text match. Default 0.5 — Vision framework's confidence floor for English/Chinese mixed content. Lower if you're seeing 'no match' on text you can clearly see.",
			},
			"target_pid": {
				"type":        "integer",
				"description": "Optional PID for background-mode click (no focus steal). Same semantics as `input click target_pid=`. Recommend for Electron apps that visually update without coming to front.",
			},
			"dry_run": {
				"type":        "string",
				"description": "If 'true', perform OCR + match but DON'T click. Returns the would-click coordinates + confidence so you can verify before committing. Useful when match=contains might catch the wrong thing.",
			},
		}, []string{"text"})
}

func (s *smartClickSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `smart_click` (needs both `screen` and `input` capabilities)")
	}
	needle := params["text"]
	if needle == "" {
		return "", fmt.Errorf("smart_click: `text` is required (the visible text on the target element)")
	}
	matchMode := strings.ToLower(strings.TrimSpace(params["match"]))
	if matchMode == "" {
		matchMode = "exact"
	}
	switch matchMode {
	case "exact", "contains", "prefix":
	default:
		return "", fmt.Errorf("smart_click: match must be exact|contains|prefix (got %q)", matchMode)
	}

	// 1. Capture for OCR. Use the existing screen-skill imageForOCR
	//    plumbing so display_id + region work the same as direct
	//    `screen ocr` calls. We'd ideally call s.screen.imageForOCR
	//    but smart_click is its own skill — so we duplicate the
	//    minimal capture path here.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	imgBytes, label, err := captureForSmartClick(ctx, params)
	if err != nil {
		return "", fmt.Errorf("smart_click: %w", err)
	}

	// 2. OCR.
	regions, err := sckit.OCR(imgBytes)
	if err != nil {
		return "", fmt.Errorf("smart_click: sckit.OCR: %w", err)
	}
	if len(regions) == 0 {
		return "", fmt.Errorf("smart_click: no text recognized on %s", label)
	}

	// 3. Filter by min_confidence.
	minConf := 0.5
	if mc := strings.TrimSpace(params["min_confidence"]); mc != "" {
		var f float64
		if _, err := fmt.Sscanf(mc, "%f", &f); err == nil && f >= 0 && f <= 1 {
			minConf = f
		}
	}

	// 4. Match. We rank by (confidence) descending among matches.
	type cand struct {
		text       string
		x, y, w, h int
		conf       float64
	}
	var matches []cand
	low := strings.ToLower(needle)
	for _, r := range regions {
		if r.Confidence < minConf {
			continue
		}
		t := strings.ToLower(r.Text)
		var ok bool
		switch matchMode {
		case "exact":
			ok = strings.TrimSpace(t) == low
		case "contains":
			ok = strings.Contains(t, low)
		case "prefix":
			ok = strings.HasPrefix(strings.TrimSpace(t), low)
		}
		if ok {
			matches = append(matches, cand{
				text: r.Text, x: r.X, y: r.Y, w: r.W, h: r.H, conf: r.Confidence,
			})
		}
	}
	if len(matches) == 0 {
		// Helpful debug output: list top 5 OCR'd texts so the model
		// can see what's actually visible (and thus pick a better
		// needle / match mode next try).
		var topN []string
		for i, r := range regions {
			if i >= 5 {
				break
			}
			topN = append(topN, fmt.Sprintf("%q(%.2f)", r.Text, r.Confidence))
		}
		return "", fmt.Errorf(
			"smart_click: no OCR region matched %q (mode=%s, min_conf=%.2f). Top recognized text: %s",
			needle, matchMode, minConf, strings.Join(topN, ", "),
		)
	}
	// Best match = highest confidence.
	best := matches[0]
	for _, m := range matches[1:] {
		if m.conf > best.conf {
			best = m
		}
	}
	cx := best.x + best.w/2
	cy := best.y + best.h/2

	// 5. Dry run? Return coords without clicking.
	if parseBoolParam(params["dry_run"], false) {
		return fmt.Sprintf(
			"smart_click DRY RUN — would click %q at (%d,%d) bbox=(%d,%d %dx%d) conf=%.2f%s",
			best.text, cx, cy, best.x, best.y, best.w, best.h, best.conf,
			matchSummary(len(matches)),
		), nil
	}

	// 6. Click.
	button, err := parseButton(params["button"])
	if err != nil {
		return "", fmt.Errorf("smart_click: %w", err)
	}
	clicks := atoiDefault(params["clicks"], 1)
	if clicks < 1 {
		clicks = 1
	}

	var opts []input.PostOption
	pidLabel := ""
	if pid := atoiDefault(params["target_pid"], 0); pid > 0 {
		opts = append(opts, input.WithPID(int32(pid)))
		pidLabel = fmt.Sprintf(" → pid %d (no focus steal)", pid)
	}
	if err := input.ClickAtButton(ctx, float64(cx), float64(cy), button, clicks, opts...); err != nil {
		return "", fmt.Errorf("smart_click: input.ClickAtButton: %w", err)
	}
	return fmt.Sprintf(
		"smart_click: clicked %q at (%d,%d) bbox=(%d,%d %dx%d) conf=%.2f buttons=%d%s%s",
		best.text, cx, cy, best.x, best.y, best.w, best.h, best.conf, clicks,
		pidLabel, matchSummary(len(matches)),
	), nil
}

// captureForSmartClick picks display + (optional region), captures
// in-memory, returns PNG bytes + a label string. Mirrors
// screen.imageForOCR's logic but standalone so this skill stays
// self-contained (no cross-skill helper coupling).
func captureForSmartClick(ctx context.Context, params map[string]string) ([]byte, string, error) {
	displays, err := sckit.ListDisplays(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("ListDisplays: %w", err)
	}
	if len(displays) == 0 {
		return nil, "", fmt.Errorf("no displays available")
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
			return nil, "", fmt.Errorf("display_id %q not found", want)
		}
	}

	var captured image.Image
	var label string
	if regionStr := strings.TrimSpace(params["region"]); regionStr != "" {
		parts := strings.Split(regionStr, ",")
		if len(parts) != 4 {
			return nil, "", fmt.Errorf("region: expected x,y,w,h")
		}
		nums := [4]int{}
		for i, p := range parts {
			n, err := parseIntPart(strings.TrimSpace(p))
			if err != nil {
				return nil, "", fmt.Errorf("region: %w", err)
			}
			nums[i] = n
		}
		region := sckit.Region{
			Display: display,
			Bounds:  image.Rect(nums[0], nums[1], nums[0]+nums[2], nums[1]+nums[3]),
		}
		c, err := sckit.Capture(ctx, region)
		if err != nil {
			return nil, "", fmt.Errorf("Capture region: %w", err)
		}
		captured = c
		label = fmt.Sprintf("region (%d,%d %dx%d) on display=%d",
			nums[0], nums[1], nums[2], nums[3], display.ID)
	} else {
		c, err := sckit.Capture(ctx, display)
		if err != nil {
			return nil, "", fmt.Errorf("Capture display: %w", err)
		}
		captured = c
		b := captured.Bounds()
		label = fmt.Sprintf("display=%d %dx%d", display.ID, b.Dx(), b.Dy())
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, captured); err != nil {
		return nil, "", fmt.Errorf("png encode: %w", err)
	}
	return buf.Bytes(), label, nil
}

func matchSummary(n int) string {
	if n <= 1 {
		return ""
	}
	return fmt.Sprintf(" (best of %d matches)", n)
}
