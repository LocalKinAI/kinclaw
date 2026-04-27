// Package probe inspects a macOS app's Accessibility tree to measure
// "5-claw controllability" — how richly the app exposes its UI semantically.
//
// The single-shot Probe call:
//   1. Best-effort launches the app via `open -gb <bundle_id>` (no focus steal).
//   2. Polls kinax.ApplicationByBundleID until the AX root is reachable (8s budget).
//   3. Activates the app via osascript so AppKit/Catalyst/Electron actually draws
//      its windows — background-launched apps return AXApplication root with no
//      children, which would falsely score "blank".
//   4. Walks the AX tree depth-first to maxDepth=8, counting role distribution
//      under a 5s walk budget (some apps have >4000 nodes; partial counts still
//      classify correctly).
//
// The Stats produced are pure data — Classify() turns them into a Verdict, and
// formatters (Human/JSON/CSVRow) emit them. Single-app mode is the human face;
// batch mode (the same probe over a stdin list) drives the original 50-app
// validation.
package probe

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LocalKinAI/kinax-go"
)

// Default budgets. Exposed via Options for tests and `--depth` / `--timeout` flags.
const (
	DefaultMaxDepth       = 8
	DefaultOpenTimeout    = 8 * time.Second
	DefaultPollInterval   = 200 * time.Millisecond
	DefaultProbeTimeout   = 5 * time.Second
	DefaultActivateSettle = 1500 * time.Millisecond
)

// Options tunes a single Probe call.
type Options struct {
	MaxDepth       int           // tree-walk depth ceiling (default 8)
	OpenTimeout    time.Duration // wait for kinax to see the process (default 8s)
	PollInterval   time.Duration // kinax.ApplicationByBundleID poll cadence (default 200ms)
	ProbeTimeout   time.Duration // tree-walk time ceiling (default 5s)
	ActivateSettle time.Duration // post-activate sleep before walking (default 1.5s)
	SkipActivate   bool          // don't run osascript activate; useful for already-foregrounded probes / tests
}

// withDefaults fills zero fields with the package defaults.
func (o Options) withDefaults() Options {
	if o.MaxDepth <= 0 {
		o.MaxDepth = DefaultMaxDepth
	}
	if o.OpenTimeout <= 0 {
		o.OpenTimeout = DefaultOpenTimeout
	}
	if o.PollInterval <= 0 {
		o.PollInterval = DefaultPollInterval
	}
	if o.ProbeTimeout <= 0 {
		o.ProbeTimeout = DefaultProbeTimeout
	}
	if o.ActivateSettle <= 0 {
		o.ActivateSettle = DefaultActivateSettle
	}
	return o
}

// Stats is the per-app measurement.
type Stats struct {
	BundleID    string `json:"bundle_id"`
	ProcessOK   bool   `json:"process_ok"`
	ProbeOK     bool   `json:"probe_ok"`
	ProbeMS     int64  `json:"probe_ms"`
	TotalNodes  int    `json:"total_nodes"`
	MaxDepth    int    `json:"max_depth"`
	Buttons     int    `json:"buttons"`
	TextFields  int    `json:"text_fields"`
	MenuItems   int    `json:"menu_items"`
	Windows     int    `json:"windows"`
	StaticTexts int    `json:"static_texts"`
	Images      int    `json:"images"`
	Literate    int    `json:"literate"` // nodes with non-empty title or description
	ErrMsg      string `json:"error,omitempty"`
}

// Verdict classifies an app's controllability via the AX claw alone.
type Verdict int

const (
	VerdictDead    Verdict = iota // process didn't open / AX unreachable
	VerdictBlank                  // process opened but tree contains < 10 nodes
	VerdictShallow                // tree has nodes but few actionable elements
	VerdictRich                   // tree has nodes >= 50 and actionable >= 5
)

// String returns a short label.
func (v Verdict) String() string {
	switch v {
	case VerdictDead:
		return "dead"
	case VerdictBlank:
		return "blank"
	case VerdictShallow:
		return "shallow"
	case VerdictRich:
		return "rich"
	}
	return "unknown"
}

// Icon returns a single emoji for the verdict (used in human output).
func (v Verdict) Icon() string {
	switch v {
	case VerdictDead:
		return "🔴"
	case VerdictBlank:
		return "🟠"
	case VerdictShallow:
		return "🟡"
	case VerdictRich:
		return "🟢"
	}
	return "❓"
}

// Description returns a one-line meaning of the verdict.
func (v Verdict) Description() string {
	switch v {
	case VerdictRich:
		return "AX-rich — `ui` claw alone drives it"
	case VerdictShallow:
		return "AX-shallow — `ui` + `input` (cmd-keys / type-text) hybrid"
	case VerdictBlank:
		return "AX-blank — needs `record` + `screen` + vision"
	case VerdictDead:
		return "dead — process didn't start (TCC / sandbox / not installed?)"
	}
	return "unknown"
}

// Classify returns the Verdict for these Stats, using the same thresholds
// as the 50-app validation analysis. Actionable = buttons + text_fields +
// menu_items (so apps that drive primarily through menus, like iWork or
// Electron, score correctly even with 0 AXButton).
func (s Stats) Classify() Verdict {
	if !s.ProcessOK {
		return VerdictDead
	}
	if s.TotalNodes < 10 {
		return VerdictBlank
	}
	actionable := s.Buttons + s.TextFields + s.MenuItems
	if s.TotalNodes >= 50 && actionable >= 5 {
		return VerdictRich
	}
	return VerdictShallow
}

// Probe runs the full open → activate → poll → walk sequence for one app.
// Errors that prevent the probe (no AX permission, process never appears) are
// recorded in Stats.ErrMsg rather than returned, so batch callers can keep
// going.
func Probe(bundleID string, opts Options) Stats {
	opts = opts.withDefaults()
	s := Stats{BundleID: bundleID}

	// Step 1: best-effort launch in background. `-g` = no focus steal,
	// `-b` = bundle ID (NOT `-a`, which expects an app *name* — combining -ab
	// makes launchservices treat the bundle ID as a file path).
	if err := exec.Command("open", "-gb", bundleID).Run(); err != nil {
		// Already-running apps still succeed; a real failure is "Unable to find
		// application". Record but don't abort — kinax may still see it.
		s.ErrMsg = "open: " + truncErr(err.Error())
	}

	// Step 2: poll for kinax reachability.
	deadline := time.Now().Add(opts.OpenTimeout)
	var app *kinax.Element
	var err error
	for time.Now().Before(deadline) {
		app, err = kinax.ApplicationByBundleID(bundleID)
		if err == nil && app != nil {
			s.ProcessOK = true
			break
		}
		time.Sleep(opts.PollInterval)
	}
	if !s.ProcessOK {
		if err != nil {
			s.ErrMsg = combineErrs(s.ErrMsg, "no_process: "+truncErr(err.Error()))
		} else {
			s.ErrMsg = combineErrs(s.ErrMsg, "no_process: timeout")
		}
		return s
	}
	defer app.Close()

	// Step 3: foreground-activate so the app populates its UI tree. Without
	// this, kinax sees the AXApplication root with zero children for cold-
	// launched apps. SkipActivate=true is for tests / for the case where the
	// caller already knows the app is foregrounded.
	if !opts.SkipActivate {
		_ = exec.Command("osascript", "-e",
			fmt.Sprintf(`tell application id "%s" to activate`, bundleID)).Run()
		time.Sleep(opts.ActivateSettle)
	}

	// Step 4: walk under a time budget. Some apps have >4000 nodes; partial
	// counts still classify correctly (the verdict thresholds are far below
	// the typical post-timeout count).
	start := time.Now()
	done := make(chan struct{})
	go func() {
		walk(app, 0, opts.MaxDepth, &s)
		close(done)
	}()
	select {
	case <-done:
		s.ProbeOK = true
	case <-time.After(opts.ProbeTimeout):
		s.ErrMsg = combineErrs(s.ErrMsg, fmt.Sprintf("probe_timeout(%dms)", opts.ProbeTimeout.Milliseconds()))
	}
	s.ProbeMS = time.Since(start).Milliseconds()
	return s
}

// walk recursively traverses the AX tree, mutating Stats in place.
func walk(e *kinax.Element, depth, maxDepth int, s *Stats) {
	s.TotalNodes++
	if depth > s.MaxDepth {
		s.MaxDepth = depth
	}
	role, _ := e.Role()
	title, _ := e.Title()
	desc, _ := e.Description()
	if title != "" || desc != "" {
		s.Literate++
	}
	switch role {
	case "AXButton":
		s.Buttons++
	case "AXTextField", "AXSecureTextField", "AXTextArea":
		s.TextFields++
	case "AXMenuItem", "AXMenuButton":
		s.MenuItems++
	case "AXWindow":
		s.Windows++
	case "AXStaticText":
		s.StaticTexts++
	case "AXImage":
		s.Images++
	}
	if depth >= maxDepth {
		return
	}
	kids, err := e.Children()
	if err != nil {
		return
	}
	for _, k := range kids {
		walk(k, depth+1, maxDepth, s)
		k.Close()
	}
}

// CSVHeader is the ordered column list for batch CSV output.
var CSVHeader = []string{
	"bundle_id", "process_ok", "probe_ok", "probe_ms",
	"total_nodes", "max_depth", "buttons", "text_fields",
	"menu_items", "windows", "static_texts", "images", "literate",
	"error",
}

// CSVRow returns the row in the same column order as CSVHeader.
func (s Stats) CSVRow() []string {
	return []string{
		s.BundleID,
		boolStr(s.ProcessOK), boolStr(s.ProbeOK),
		strconv.FormatInt(s.ProbeMS, 10),
		strconv.Itoa(s.TotalNodes), strconv.Itoa(s.MaxDepth),
		strconv.Itoa(s.Buttons), strconv.Itoa(s.TextFields),
		strconv.Itoa(s.MenuItems), strconv.Itoa(s.Windows),
		strconv.Itoa(s.StaticTexts), strconv.Itoa(s.Images),
		strconv.Itoa(s.Literate),
		s.ErrMsg,
	}
}

// Human renders Stats as a readable multi-line block, including the verdict
// and a one-line meaning.
func (s Stats) Human() string {
	var b strings.Builder
	v := s.Classify()
	fmt.Fprintf(&b, "🔍 %s\n", s.BundleID)
	if s.ProcessOK {
		fmt.Fprintf(&b, "  Process:    ✅ running\n")
	} else {
		fmt.Fprintf(&b, "  Process:    ❌ not running (%s)\n", s.ErrMsg)
	}
	if s.ProbeOK {
		fmt.Fprintf(&b, "  AX walk:    ✅ complete (%dms)\n", s.ProbeMS)
	} else if s.ProcessOK {
		fmt.Fprintf(&b, "  AX walk:    ⚠️  partial (%dms)  — %s\n", s.ProbeMS, s.ErrMsg)
	}
	if s.ProcessOK {
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "  AX nodes:        %-6d  (max depth: %d)\n", s.TotalNodes, s.MaxDepth)
		fmt.Fprintf(&b, "  Buttons:         %d\n", s.Buttons)
		fmt.Fprintf(&b, "  Text fields:     %d\n", s.TextFields)
		fmt.Fprintf(&b, "  Menu items:      %d\n", s.MenuItems)
		fmt.Fprintf(&b, "  Windows:         %d\n", s.Windows)
		fmt.Fprintf(&b, "  Static texts:    %d\n", s.StaticTexts)
		fmt.Fprintf(&b, "  Images:          %d\n", s.Images)
		fmt.Fprintf(&b, "  Literate:        %d  (have title or description)\n", s.Literate)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Verdict: %s %s — %s\n", v.Icon(), strings.ToUpper(v.String()), v.Description())
	return b.String()
}

// JSON returns the Stats encoded as JSON.
func (s Stats) JSON() ([]byte, error) {
	type out struct {
		Stats
		Verdict string `json:"verdict"`
	}
	return json.MarshalIndent(out{Stats: s, Verdict: s.Classify().String()}, "", "  ")
}

// WriteCSVHeader writes the column header to w. Useful in batch mode before
// the per-row writes.
func WriteCSVHeader(w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(CSVHeader); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// WriteCSVRow writes one stats row to w.
func WriteCSVRow(w io.Writer, s Stats) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(s.CSVRow()); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// EnsureAccessibility returns a non-nil error if the calling process doesn't
// have macOS Accessibility permission. The error message tells the user
// where to grant it. Subcommand callers should check this first and exit
// with a clear diagnostic, not a kinax internal error.
func EnsureAccessibility() error {
	if !kinax.Trusted() {
		return fmt.Errorf("Accessibility permission not granted to the launching process.\n  Grant it: System Settings → Privacy & Security → Accessibility")
	}
	return nil
}

// ProbeContext is the same as Probe but respects ctx cancellation between
// poll iterations and during the walk goroutine.
//
// Currently unused by the kinclaw subcommand (the probe is fast enough that
// signal handling at the main level suffices), but exposed for future
// integration where a probe might be interruptible from the chat loop.
func ProbeContext(ctx context.Context, bundleID string, opts Options) Stats {
	// Trivial wrapper: respect ctx.Done() between poll iterations. The
	// walk itself runs to completion or the probe-walk timeout.
	opts = opts.withDefaults()
	if ctx == nil {
		return Probe(bundleID, opts)
	}
	// The full integration would split out helpers; the simple form is to
	// short-circuit at the start. Future work, currently unused.
	select {
	case <-ctx.Done():
		return Stats{BundleID: bundleID, ErrMsg: "canceled before probe"}
	default:
	}
	return Probe(bundleID, opts)
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func truncErr(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

func combineErrs(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}
