//go:build darwin

package skill

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LocalKinAI/kinax-go"
)

// uiSkill is the "visual cortex" of KinClaw. It wraps kinax-go
// (AXUIElement) so a soul can navigate and manipulate macOS UI by
// semantic identity (role, title, identifier) rather than pixel
// coordinates. Requires Accessibility TCC permission (shared with
// inputSkill). Unlike inputSkill (silent noop without permission),
// uiSkill returns explicit errors.
type uiSkill struct {
	allowed bool
}

// NewUISkill returns a skill that queries and acts on the AX tree.
func NewUISkill(allowed bool) Skill {
	return &uiSkill{allowed: allowed}
}

func (s *uiSkill) Name() string { return "ui" }

func (s *uiSkill) Description() string {
	return "Navigate and control macOS UI by semantic identity (role, title, " +
		"identifier) via the Accessibility API. Prefer this over 'input click " +
		"at x,y' whenever the element has an AX title — it's faster, reliable " +
		"across resolutions, and survives app-layout changes. Actions: " +
		"focused_app, tree, find, click, click_sequence, read, at_point. Requires macOS " +
		"Accessibility permission."
}

func (s *uiSkill) ToolDef() json.RawMessage {
	return MakeToolDef("ui", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "focused_app | tree | find | click | click_sequence | read | at_point | watch",
			},
			"bundle_id": {
				"type":        "string",
				"description": "Target app by bundle ID (e.g. com.apple.Safari). Default: focused app.",
			},
			"pid":  {"type": "integer", "description": "Target app by PID (alternative to bundle_id)"},
			"role": {"type": "string", "description": "AXRole: AXButton, AXTextField, AXMenuItem..."},
			"title": {
				"type":        "string",
				"description": "Element title (exact match)",
			},
			"title_contains": {
				"type":        "string",
				"description": "Element title substring (case-insensitive)",
			},
			"identifier": {
				"type":        "string",
				"description": "AXIdentifier — stable automation ID set by app developer",
			},
			"description": {
				"type":        "string",
				"description": "AXDescription — short human-readable label, used by icon/symbol buttons that have no AXTitle (media play/pause, toolbar icons, dialpads, map zoom controls, calculator number keys, etc.). Check the `desc=...` column in `ui tree` output to find the right value.",
			},
			"depth": {"type": "integer", "description": "Search depth (default 20 for find/click, 6 for tree)"},
			"x":     {"type": "number", "description": "X (for at_point)"},
			"y":     {"type": "number", "description": "Y (for at_point)"},
			"titles": {
				"type":        "string",
				"description": "For click_sequence: comma-separated AXButton titles to click in order. Whole sequence runs in ONE tool call — saves N round-trips for multi-button flows.",
			},
			"identifiers": {
				"type":        "string",
				"description": "For click_sequence: comma-separated AXIdentifiers — most reliable when the app exposes them.",
			},
			"descriptions": {
				"type":        "string",
				"description": "For click_sequence: comma-separated AXDescriptions — the right field for icon/symbol buttons (media controls, toolbar icons, dialpads, calculator-style number keys, etc.). Read the desc=... column from `ui tree` first.",
			},
			"force": {
				"type":        "string",
				"description": "Bypass click safety guards (true/false). Default false. The ui click action refuses by default to (a) act when the matcher hits 2+ elements, and (b) press destructive targets like AXCloseButton / 'Quit' / '关闭'. Pass force=true only after you've inspected the candidates with a 'find' first and you really mean to click.",
			},
			"events": {
				"type":        "string",
				"description": "For action=watch: comma-separated AX notifications to subscribe to on the target application root. Common: AXFocusedWindowChanged, AXValueChanged, AXTitleChanged, AXWindowCreated, AXMenuOpened, AXApplicationActivated. Default: AXFocusedWindowChanged.",
			},
			"duration_ms": {
				"type":        "integer",
				"description": "For action=watch: block this many milliseconds collecting events, then return everything observed. Default: 3000ms. Max: 30000ms.",
			},
		}, []string{"action"})
}

func (s *uiSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `ui` capability")
	}
	if !kinax.Trusted() {
		return "", fmt.Errorf("kinax: Accessibility permission not granted — grant it in System Settings → Privacy & Security → Accessibility, then retry")
	}
	action := params["action"]
	if action == "" {
		return "", fmt.Errorf("missing required parameter: action")
	}

	switch action {
	case "focused_app":
		return s.focusedApp()
	case "tree":
		return s.tree(params)
	case "find":
		return s.find(params, false)
	case "click":
		return s.find(params, true)
	case "click_sequence":
		return s.clickSequence(params)
	case "read":
		return s.read(params)
	case "at_point":
		return s.atPoint(params)
	case "watch":
		return s.watch(params)
	default:
		return "", fmt.Errorf("unknown action %q", action)
	}
}

// watch subscribes to AX notifications on the target application's
// root element via kinax.Observer (push-based, kinax-go v0.3+),
// blocks for duration_ms collecting events, returns a summary.
//
// Cheaper than polling `ui tree` repeatedly to detect change. The
// agent calls this when it wants to react to a specific UI event
// (window focus shifted, dialog appeared, value updated post-click)
// instead of guessing when to re-tree.
func (s *uiSkill) watch(params map[string]string) (string, error) {
	events := params["events"]
	if events == "" {
		events = kinax.NotifFocusedWindowChanged
	}
	notifications := splitCSV(events)
	if len(notifications) == 0 {
		return "", fmt.Errorf("watch: events must be a comma-separated list of AX notifications")
	}

	durationMs := atoiDefault(params["duration_ms"], 3000)
	if durationMs <= 0 {
		durationMs = 3000
	}
	if durationMs > 30000 {
		durationMs = 30000
	}

	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()
	pid := kinax.FrontmostPID()
	if p := params["pid"]; p != "" {
		pid = atoiDefault(p, pid)
	}
	if pid <= 0 {
		return "", fmt.Errorf("watch: could not resolve target pid")
	}

	obs, err := kinax.NewObserver(pid)
	if err != nil {
		return "", fmt.Errorf("watch: NewObserver(pid=%d): %w", pid, err)
	}
	defer obs.Close()

	if err := obs.Subscribe(app, notifications...); err != nil {
		return "", fmt.Errorf("watch: %w", err)
	}

	deadline := time.Now().Add(time.Duration(durationMs) * time.Millisecond)
	type seen struct {
		notif    string
		role     string
		title    string
		atMillis int64
	}
	var collected []seen
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		// Use 200ms chunks so a hot stream still drains promptly even
		// near the deadline; longer single waits would risk missing the
		// final tail of the window.
		poll := remaining
		if poll > 200*time.Millisecond {
			poll = 200 * time.Millisecond
		}
		ev, err := obs.Next(poll)
		if err != nil {
			// Timeout is expected — keep looping until deadline.
			continue
		}
		role, _ := ev.Element.Role()
		title, _ := ev.Element.Title()
		ev.Element.Close()
		collected = append(collected, seen{
			notif:    ev.Notification,
			role:     role,
			title:    title,
			atMillis: time.Since(deadline.Add(-time.Duration(durationMs) * time.Millisecond)).Milliseconds(),
		})
	}

	if len(collected) == 0 {
		return fmt.Sprintf("watched pid=%d for %dms (events: %v) — no notifications fired",
			pid, durationMs, notifications), nil
	}
	out := fmt.Sprintf("watched pid=%d for %dms (events: %v) — %d notification(s):\n",
		pid, durationMs, notifications, len(collected))
	for _, e := range collected {
		out += fmt.Sprintf("  +%dms  %s  %s %q\n", e.atMillis, e.notif, e.role, e.title)
	}
	return out, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *uiSkill) openTarget(params map[string]string) (*kinax.Element, error) {
	if b := params["bundle_id"]; b != "" {
		return kinax.ApplicationByBundleID(b)
	}
	if p := params["pid"]; p != "" {
		pid := atoiDefault(p, 0)
		if pid == 0 {
			return nil, fmt.Errorf("bad pid %q", p)
		}
		return kinax.ApplicationByPID(pid)
	}
	return kinax.FocusedApplication()
}

func (s *uiSkill) focusedApp() (string, error) {
	app, err := kinax.FocusedApplication()
	if err != nil {
		return "", err
	}
	defer app.Close()
	title, _ := app.Title()
	role, _ := app.Role()
	pid := kinax.FrontmostPID()
	return fmt.Sprintf("pid=%d role=%s title=%q", pid, role, title), nil
}

func (s *uiSkill) tree(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	depth := atoiDefault(params["depth"], 6)
	var sb strings.Builder
	dumpTree(&sb, app, 0, depth)
	return sb.String(), nil
}

// treeAttrs lists the AX attributes dumpTree wants per node. Pinned as
// a constant so kinax-go's GetMany batches all 5 into a single AX IPC
// instead of running 5 separate AXUIElementCopyAttributeValue calls.
// On dense trees (Cursor / Slack / Xcode) this is the v1.4.0 4× win.
var treeAttrs = []string{
	kinax.AttrRole, kinax.AttrTitle, kinax.AttrIdentifier,
	kinax.AttrDescription, kinax.AttrValue,
}

func dumpTree(sb *strings.Builder, e *kinax.Element, indent, maxDepth int) {
	// One IPC for all 5 scalar attributes via
	// AXUIElementCopyMultipleAttributeValues. Element-valued attrs
	// (AXChildren) still go through Children() below — GetMany doesn't
	// return handle-typed values.
	attrs, err := e.GetMany(treeAttrs...)
	if err != nil {
		// Fail soft: an empty map produces an empty tree line, same
		// as the old per-attr code did when each call individually
		// errored (kAXErrorCannotComplete on flaky targets).
		attrs = map[string]any{}
	}
	role := strAttr(attrs, kinax.AttrRole)
	title := strAttr(attrs, kinax.AttrTitle)
	id := strAttr(attrs, kinax.AttrIdentifier)
	desc := strAttr(attrs, kinax.AttrDescription)
	value := strAttr(attrs, kinax.AttrValue)

	pad := strings.Repeat("  ", indent)
	line := pad + role
	if title != "" {
		line += fmt.Sprintf(" %q", title)
	}
	// Description is the matcher of choice for icon / symbol buttons
	// where title is empty (Calculator number keys, media controls,
	// toolbar icons). Showing it here makes the LLM's tree-reading
	// step actually informative — earlier dumps hid this and led to
	// "no usable matcher, fall back to input type" wrong calls.
	if desc != "" && desc != title {
		line += fmt.Sprintf(" desc=%q", desc)
	}
	// AXValue holds the *current displayed text* of static text
	// readouts (Calculator's display, status labels, sliders, text
	// fields). Without this column, verification after an action
	// required a separate `ui read` call that often hit the wrong
	// element. With it, a single re-tree shows what changed.
	if value != "" && value != title && value != desc {
		line += fmt.Sprintf(" value=%q", truncateValue(value))
	}
	if id != "" {
		line += fmt.Sprintf(" [%s]", id)
	}
	sb.WriteString(line)
	sb.WriteByte('\n')
	if indent >= maxDepth {
		return
	}
	kids, err := e.Children()
	if err != nil {
		return
	}
	for _, k := range kids {
		dumpTree(sb, k, indent+1, maxDepth)
		k.Close()
	}
}

// strAttr extracts a string-valued attribute from the GetMany result.
// Returns "" for missing or non-string values — matches the old
// per-attribute path's "treat empty + error the same" semantics.
func strAttr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		// Whole numbers print without trailing decimals (the AX API
		// returns counts + indices as numbers occasionally).
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func (s *uiSkill) matcher(params map[string]string) (kinax.Matcher, string, error) {
	var matchers []kinax.Matcher
	var desc []string
	if r := params["role"]; r != "" {
		matchers = append(matchers, kinax.MatchRole(r))
		desc = append(desc, "role="+r)
	}
	if t := params["title"]; t != "" {
		matchers = append(matchers, kinax.MatchTitle(t))
		desc = append(desc, fmt.Sprintf("title=%q", t))
	} else if tc := params["title_contains"]; tc != "" {
		matchers = append(matchers, kinax.MatchTitleContains(tc))
		desc = append(desc, fmt.Sprintf("title~%q", tc))
	}
	if d := params["description"]; d != "" {
		matchers = append(matchers, matchDescription(d))
		desc = append(desc, fmt.Sprintf("desc=%q", d))
	}
	if id := params["identifier"]; id != "" {
		matchers = append(matchers, kinax.MatchIdentifier(id))
		desc = append(desc, "id="+id)
	}
	if len(matchers) == 0 {
		return nil, "", fmt.Errorf("at least one of role, title, title_contains, identifier is required")
	}
	return kinax.MatchAll(matchers...), strings.Join(desc, " "), nil
}

func (s *uiSkill) find(params map[string]string, clickFirst bool) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	m, desc, err := s.matcher(params)
	if err != nil {
		return "", err
	}
	depth := atoiDefault(params["depth"], 20)
	force := parseBoolParam(params["force"], false)

	hits := app.FindAll(m, depth)

	// `find` (clickFirst=false): observation-only. List everything, no
	// safety checks — the LLM is supposed to look before acting.
	if !clickFirst {
		defer closeAll(hits)
		if len(hits) == 0 {
			return fmt.Sprintf("no elements matching %s", desc), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%d match(es) for %s:\n", len(hits), desc))
		for _, h := range hits {
			r, _ := h.Role()
			t, _ := h.Title()
			id, _ := h.Identifier()
			d, _ := h.Description()
			v, _ := h.Value()
			// Output schema: role / title / desc / value / [id].
			// Showing desc + value inline means a single `ui find` call
			// is also a read — no separate `ui read` round-trip needed
			// to grab a value off the matched element.
			parts := []string{r}
			if t != "" {
				parts = append(parts, fmt.Sprintf("title=%q", t))
			}
			if d != "" && d != t {
				parts = append(parts, fmt.Sprintf("desc=%q", d))
			}
			if v != "" && v != t && v != d {
				parts = append(parts, fmt.Sprintf("value=%q", truncateValue(v)))
			}
			if id != "" {
				parts = append(parts, "["+id+"]")
			}
			sb.WriteString("  " + strings.Join(parts, " ") + "\n")
		}
		return sb.String(), nil
	}

	// `click` (clickFirst=true): action — apply safety guards.
	if len(hits) == 0 {
		return "", fmt.Errorf("no element matching %s", desc)
	}

	// Guard 1 — ambiguity refusal. The pilot session that closed
	// Calculator did so because a broad matcher hit AXCloseButton +
	// the actual button, and the kernel happily clicked the first.
	// Default behavior is now to refuse and list candidates so the
	// caller can narrow with parent/window/identifier filters.
	if len(hits) > 1 && !force {
		defer closeAll(hits)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(
			"ui click refused: %d elements matched %s. "+
				"Add filters (identifier / role / parent) or pass force=true "+
				"to click the first match. Candidates:\n",
			len(hits), desc,
		))
		const maxList = 8
		for i, h := range hits {
			if i >= maxList {
				sb.WriteString(fmt.Sprintf("  ... (%d more)\n", len(hits)-maxList))
				break
			}
			r, _ := h.Role()
			t, _ := h.Title()
			id, _ := h.Identifier()
			sb.WriteString(fmt.Sprintf("  %s\t%q\t[%s]\n", r, t, id))
		}
		return "", fmt.Errorf("%s", sb.String())
	}

	// Take the first hit, dispose of any others (only present when
	// force=true with multiple matches).
	el := hits[0]
	for i := 1; i < len(hits); i++ {
		hits[i].Close()
	}
	defer el.Close()

	role, _ := el.Role()
	title, _ := el.Title()

	// Guard 2 — destructive-target refusal. Don't press window
	// close/minimize buttons or buttons labeled Close/Quit/退出/关闭
	// without explicit force=true. This is the second line of defense
	// even if the matcher unambiguously hit the close button.
	if !force && isDestructiveTarget(role, title) {
		return "", fmt.Errorf(
			"ui click refused: target %s %q looks destructive "+
				"(close/quit/minimize). Pass force=true to click anyway.",
			role, title,
		)
	}

	if err := el.Perform(kinax.ActionPress); err != nil {
		return "", fmt.Errorf("AXPress: %w", err)
	}
	return fmt.Sprintf("clicked %s %q (matched %s)", role, title, desc), nil
}

// closeAll releases every element in a FindAll result. Use when the
// caller errors out before consuming the slice.
func closeAll(els []*kinax.Element) {
	for _, e := range els {
		e.Close()
	}
}

// matchDescription is a kinax.Matcher that selects elements whose
// AXDescription equals `want`. kinax-go v0.1 doesn't ship a built-in
// MatchDescription helper, but the Matcher type is just a func, so
// we plug it in here. AXDescription is the canonical matcher for
// icon-only / symbol buttons where AXTitle is empty (Calculator's
// "1" / "+" / "Equals" keys, media play/pause, toolbar pins).
func matchDescription(want string) kinax.Matcher {
	return func(e *kinax.Element) bool {
		got, _ := e.Description()
		return got == want
	}
}

// truncateValue caps very long AXValue strings so a tree dump of a
// text editor or terminal doesn't blow the LLM's context. 200 chars
// is plenty for verification — Calculator displays, status labels,
// short text fields all fit.
func truncateValue(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// clickSequence presses N elements in order in a single tool call.
// Saves the per-call LLM round-trip for flows like calculator
// (1 + 1 =), dialpads, multi-digit codes, sequential menu navigation.
//
// Either `titles` (comma-separated AXButton titles) or `identifiers`
// (comma-separated AXIdentifiers) drives the sequence. Identifiers are
// strongly preferred when available — they're stable across versions
// and unambiguous.
//
// Each press follows the same safety rules as `click`: ambiguous
// matches refuse unless force=true; destructive titles refuse unless
// force=true.
func (s *uiSkill) clickSequence(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	depth := atoiDefault(params["depth"], 20)
	force := parseBoolParam(params["force"], false)

	type field int
	const (
		byTitle field = iota
		byIdentifier
		byDescription
	)
	var items []string
	var by field
	switch {
	case params["identifiers"] != "":
		items = splitAndTrim(params["identifiers"])
		by = byIdentifier
	case params["descriptions"] != "":
		items = splitAndTrim(params["descriptions"])
		by = byDescription
	case params["titles"] != "":
		items = splitAndTrim(params["titles"])
		by = byTitle
	default:
		return "", fmt.Errorf("click_sequence requires 'titles', 'identifiers', or 'descriptions'")
	}
	if len(items) == 0 {
		return "", fmt.Errorf("click_sequence: empty list after parsing")
	}

	var log strings.Builder
	for i, item := range items {
		var matcher kinax.Matcher
		var desc string
		switch by {
		case byIdentifier:
			matcher = kinax.MatchIdentifier(item)
			desc = "id=" + item
		case byDescription:
			matcher = kinax.MatchAll(
				kinax.MatchRole("AXButton"),
				matchDescription(item),
			)
			desc = fmt.Sprintf("role=AXButton desc=%q", item)
		default: // byTitle
			matcher = kinax.MatchAll(
				kinax.MatchRole("AXButton"),
				kinax.MatchTitle(item),
			)
			desc = fmt.Sprintf("role=AXButton title=%q", item)
		}

		hits := app.FindAll(matcher, depth)
		if len(hits) == 0 {
			closeAll(hits)
			return log.String(), fmt.Errorf(
				"click_sequence aborted at step %d/%d: no element matching %s",
				i+1, len(items), desc,
			)
		}
		if len(hits) > 1 && !force {
			closeAll(hits)
			return log.String(), fmt.Errorf(
				"click_sequence aborted at step %d/%d: %d elements matched %s — narrow with identifiers or pass force=true",
				i+1, len(items), len(hits), desc,
			)
		}
		el := hits[0]
		for j := 1; j < len(hits); j++ {
			hits[j].Close()
		}
		role, _ := el.Role()
		title, _ := el.Title()
		if !force && isDestructiveTarget(role, title) {
			el.Close()
			return log.String(), fmt.Errorf(
				"click_sequence aborted at step %d/%d: %s %q looks destructive",
				i+1, len(items), role, title,
			)
		}
		if err := el.Perform(kinax.ActionPress); err != nil {
			el.Close()
			return log.String(), fmt.Errorf(
				"click_sequence step %d/%d (%s): AXPress: %w",
				i+1, len(items), desc, err,
			)
		}
		el.Close()
		fmt.Fprintf(&log, "  %d/%d: clicked %s %q\n", i+1, len(items), role, title)
	}
	return fmt.Sprintf("clicked %d elements:\n%s", len(items), log.String()), nil
}

// splitAndTrim splits on commas and trims whitespace from each part,
// dropping empties. Used for the comma-separated list params.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// isDestructiveTarget flags AX elements whose press tends to take a
// window/app away from the user. Refusing these by default is the
// kernel's safety net against the common LLM failure mode of broadly
// matching "Calculator" and accidentally hitting AXCloseButton.
//
// The English keyword set uses word-boundary semantics (exact match
// or "<keyword> <rest>") so legitimate non-destructive labels like
// "Close-up View" or "Closed Captions" pass through. The Chinese
// keyword set uses substring match — Chinese button labels are short
// and 关闭/退出/注销 are unambiguous in context.
func isDestructiveTarget(role, title string) bool {
	switch role {
	case "AXCloseButton", "AXMinimizeButton", "AXFullScreenButton":
		return true
	}
	t := strings.ToLower(strings.TrimSpace(title))
	if t != "" {
		for _, kw := range []string{"close", "quit", "exit", "log out", "sign out"} {
			if t == kw || strings.HasPrefix(t, kw+" ") {
				return true
			}
		}
	}
	for _, kw := range []string{"退出", "关闭", "注销", "结束"} {
		if strings.Contains(title, kw) {
			return true
		}
	}
	return false
}

func (s *uiSkill) read(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	m, desc, err := s.matcher(params)
	if err != nil {
		return "", err
	}
	depth := atoiDefault(params["depth"], 20)
	el, ok := app.FindFirst(m, depth)
	if !ok {
		return "", fmt.Errorf("no element matching %s", desc)
	}
	defer el.Close()

	role, _ := el.Role()
	title, _ := el.Title()
	val, _ := el.Value()
	pos, _ := el.Position()
	size, _ := el.Size()

	return fmt.Sprintf("role=%s title=%q value=%q pos=(%d,%d) size=(%d,%d)",
		role, title, val, pos.X, pos.Y, size.X, size.Y), nil
}

func (s *uiSkill) atPoint(params map[string]string) (string, error) {
	x, y, err := xy(params)
	if err != nil {
		return "", err
	}
	el, err := kinax.ElementAtPoint(x, y)
	if err != nil {
		return "", err
	}
	defer el.Close()
	role, _ := el.Role()
	title, _ := el.Title()
	return fmt.Sprintf("at (%.0f, %.0f): role=%s title=%q", x, y, role, title), nil
}
