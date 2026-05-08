//go:build darwin

package skill

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/LocalKinAI/kinax-go"
)

// This file groups the eight v1.13 "deep AX" verbs for the ui skill —
// added on top of the original tree/find/click set so a soul can
// orchestrate macOS UI without repeated screen captures and timing
// guesses. All eight piggyback on the existing kinax-go primitives
// (Observer, GetMany, AttributeElements, Perform). Order in this file
// roughly mirrors the order they're introduced in the soul protocol.

// ---------------------------------------------------------------------
// #5 — actions: list AX actions available on a matched element.
//
// Models often try AXPress when AXShowMenu is the right action (or
// vice versa) and burn a turn on the wrong call. This verb lets the
// model ask "what can this element do?" first — typically AXPress
// for buttons, AXShowMenu for popUpButton/menuButton, AXIncrement/
// AXDecrement for sliders + steppers, AXPick for menu items.
// ---------------------------------------------------------------------

func (s *uiSkill) actions(params map[string]string) (string, error) {
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
	names, err := el.ActionNames()
	if err != nil {
		return "", fmt.Errorf("ActionNames: %w", err)
	}
	if len(names) == 0 {
		return fmt.Sprintf("%s %q has no AX actions registered (likely a static / read-only element)",
			role, title), nil
	}
	sort.Strings(names)
	return fmt.Sprintf("%s %q supports %d action(s):\n  %s",
		role, title, len(names), strings.Join(names, "\n  ")), nil
}

// ---------------------------------------------------------------------
// #8 — app_state: snapshot of windows + main + focused for a target app.
//
// Resolves "I called ui tree but I want context first — what windows
// are open, which is frontmost, which is hidden". Cheap (one IPC per
// window for title), fits in <1KB output for typical apps.
// ---------------------------------------------------------------------

func (s *uiSkill) appState(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	appTitle, _ := app.Title()
	wins, _ := app.AttributeElements(kinax.AttrWindows)
	defer closeAll(wins)
	mainWin, _ := app.AttributeElement(kinax.AttrMainWindow)
	if mainWin != nil {
		defer mainWin.Close()
	}
	focusedWin, _ := app.AttributeElement(kinax.AttrFocusedWindow)
	if focusedWin != nil {
		defer focusedWin.Close()
	}
	mainTitle := ""
	if mainWin != nil {
		mainTitle, _ = mainWin.Title()
	}
	focusedTitle := ""
	if focusedWin != nil {
		focusedTitle, _ = focusedWin.Title()
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "app=%q windows=%d main=%q focused=%q\n",
		appTitle, len(wins), mainTitle, focusedTitle)
	for i, w := range wins {
		t, _ := w.Title()
		minimized, _ := w.AttributeBool(kinax.AttrMinimized)
		fullscreen, _ := w.AttributeBool(kinax.AttrFullscreen)
		flag := ""
		if minimized {
			flag = " [minimized]"
		} else if fullscreen {
			flag = " [fullscreen]"
		}
		mark := ""
		if mainWin != nil && t == mainTitle {
			mark = " *main"
		}
		fmt.Fprintf(&sb, "  %d. %q%s%s\n", i+1, t, flag, mark)
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------
// #4 — tree pruning: visible-only / role-filter / max-depth on tree.
//
// Hooks into the existing dumpTree path. The new params are read by
// the caller in the dispatch switch and threaded through here so the
// existing simple `ui tree` (no extra params) keeps its old behavior.
// ---------------------------------------------------------------------

type treeFilter struct {
	maxDepth     int
	roles        map[string]bool // empty = no filter
	visibleOnly  bool
	skipChildren bool
}

func (s *uiSkill) treeFiltered(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	f := treeFilter{
		maxDepth:    atoiDefault(params["depth"], 6),
		visibleOnly: parseBoolParam(params["visible_only"], false),
	}
	if rf := strings.TrimSpace(params["role_filter"]); rf != "" {
		f.roles = make(map[string]bool)
		for _, r := range splitCSV(rf) {
			f.roles[r] = true
		}
	}

	var sb strings.Builder
	dumpTreeFiltered(&sb, app, 0, f)
	return sb.String(), nil
}

// dumpTreeFiltered mirrors dumpTree but applies the filter set. When a
// role filter is in effect, intermediate nodes still recurse (we need
// to walk past containers to reach buttons inside) but they don't emit
// their own line — only role-matching nodes appear.
func dumpTreeFiltered(sb *strings.Builder, e *kinax.Element, indent int, f treeFilter) {
	attrs, err := e.GetMany(treeAttrs...)
	if err != nil {
		attrs = map[string]any{}
	}
	role := strAttr(attrs, kinax.AttrRole)
	title := strAttr(attrs, kinax.AttrTitle)
	id := strAttr(attrs, kinax.AttrIdentifier)
	desc := strAttr(attrs, kinax.AttrDescription)
	value := strAttr(attrs, kinax.AttrValue)

	emit := true
	if len(f.roles) > 0 && !f.roles[role] {
		emit = false
	}
	if f.visibleOnly {
		// AXVisible isn't always set; fall back to non-zero size as a
		// proxy. If AXVisible IS present and false, drop. If size = 0x0,
		// drop. Otherwise emit.
		if vis, err := e.AttributeBool(kinax.AttrVisible); err == nil && !vis {
			emit = false
		}
		if emit {
			sz, err := e.Size()
			if err == nil && sz.X == 0 && sz.Y == 0 {
				emit = false
			}
		}
	}

	if emit {
		pad := strings.Repeat("  ", indent)
		line := pad + role
		if title != "" {
			line += fmt.Sprintf(" %q", title)
		}
		if desc != "" && desc != title {
			line += fmt.Sprintf(" desc=%q", desc)
		}
		if value != "" && value != title && value != desc {
			line += fmt.Sprintf(" value=%q", truncateValue(value))
		}
		if id != "" {
			line += fmt.Sprintf(" [%s]", id)
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	if indent >= f.maxDepth {
		return
	}
	kids, err := e.Children()
	if err != nil {
		return
	}
	for _, k := range kids {
		dumpTreeFiltered(sb, k, indent+1, f)
		k.Close()
	}
}

// ---------------------------------------------------------------------
// #3 — state_diff: snapshot AX state, run optional action, return diff.
//
// Two-mode: if `action_after` is empty, just snapshot + return the
// signature so the caller can re-invoke later. If non-empty, snapshot
// before, perform the action, snapshot after, return JSON diff. The
// "signature" is a compact set of (path → role + title + value) tuples
// that's small enough to serialize-and-diff cheaply.
//
// Replaces the "screenshot before / screenshot after / model compares
// images" pattern with a token-cheap structured diff. Critical for
// verifying multi-step flows reliably.
// ---------------------------------------------------------------------

type stateNode struct {
	Path  string `json:"path"` // role/title/id breadcrumb
	Role  string `json:"role"`
	Title string `json:"title,omitempty"`
	Value string `json:"value,omitempty"`
}

func (s *uiSkill) stateDiff(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()
	depth := atoiDefault(params["depth"], 5)

	before := snapshotState(app, depth)

	// Optional: do an action between snapshots. Two ways to specify:
	//   (a) action="press" + role/title/identifier — find element + AXPress
	//   (b) action_perform="AXPress" + role/title/identifier — perform
	//       a specific named action
	if params["click_after_role"] != "" || params["click_after_title"] != "" ||
		params["click_after_identifier"] != "" {
		// Build a sub-params map with role/title/id keys so matcher()
		// works without knowing about our prefix.
		sub := map[string]string{
			"role":       params["click_after_role"],
			"title":      params["click_after_title"],
			"identifier": params["click_after_identifier"],
			"force":      "true", // diff mode bypasses ambiguity guard
			"depth":      params["depth"],
		}
		if _, err := s.find(sub, true); err != nil {
			return "", fmt.Errorf("state_diff click_after failed: %w", err)
		}
		// Tiny settle delay so post-click AX state has flushed before
		// we re-snapshot. 200ms is empirically enough for most apps.
		time.Sleep(200 * time.Millisecond)
	}

	after := snapshotState(app, depth)
	return formatDiff(before, after), nil
}

// snapshotState walks the element subtree to depth and returns the
// flat list of nodes keyed by their breadcrumb path. Closing the
// element handles is the caller's responsibility — but we close
// children inside this function as we walk.
func snapshotState(root *kinax.Element, depth int) map[string]stateNode {
	out := map[string]stateNode{}
	walkSnapshot(out, root, depth, "")
	return out
}

func walkSnapshot(out map[string]stateNode, e *kinax.Element, depth int, path string) {
	if depth < 0 {
		return
	}
	attrs, err := e.GetMany(treeAttrs...)
	if err != nil {
		attrs = map[string]any{}
	}
	role := strAttr(attrs, kinax.AttrRole)
	title := strAttr(attrs, kinax.AttrTitle)
	id := strAttr(attrs, kinax.AttrIdentifier)
	val := strAttr(attrs, kinax.AttrValue)

	// Path: parent path + role[title|id]. Stable enough across an
	// action that didn't restructure the tree (which is the common
	// case for click / type / value-change actions we're verifying).
	leaf := role
	if id != "" {
		leaf = fmt.Sprintf("%s[%s]", role, id)
	} else if title != "" {
		leaf = fmt.Sprintf("%s[%s]", role, title)
	}
	myPath := leaf
	if path != "" {
		myPath = path + "/" + leaf
	}
	out[myPath] = stateNode{
		Path:  myPath,
		Role:  role,
		Title: title,
		Value: truncateValue(val),
	}

	if depth == 0 {
		return
	}
	kids, err := e.Children()
	if err != nil {
		return
	}
	for _, k := range kids {
		walkSnapshot(out, k, depth-1, myPath)
		k.Close()
	}
}

// formatDiff produces a compact text rendering of what changed. Three
// sections: added paths, removed paths, value-changed paths. Values
// truncate at 80 chars per line.
func formatDiff(before, after map[string]stateNode) string {
	var added, removed, changed []string
	for p := range after {
		if _, ok := before[p]; !ok {
			added = append(added, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			removed = append(removed, p)
		}
	}
	for p, b := range before {
		a, ok := after[p]
		if !ok {
			continue
		}
		if a.Value != b.Value || a.Title != b.Title {
			changed = append(changed, fmt.Sprintf("%s: %q → %q",
				p, valueOrTitle(b), valueOrTitle(a)))
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)

	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		return "ax state diff: no changes"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ax state diff: +%d -%d ~%d\n",
		len(added), len(removed), len(changed)))
	if len(added) > 0 {
		sb.WriteString("added:\n")
		for _, p := range added {
			fmt.Fprintf(&sb, "  + %s\n", p)
		}
	}
	if len(removed) > 0 {
		sb.WriteString("removed:\n")
		for _, p := range removed {
			fmt.Fprintf(&sb, "  - %s\n", p)
		}
	}
	if len(changed) > 0 {
		sb.WriteString("changed:\n")
		for _, c := range changed {
			fmt.Fprintf(&sb, "  ~ %s\n", c)
		}
	}
	return sb.String()
}

func valueOrTitle(n stateNode) string {
	if n.Value != "" {
		return n.Value
	}
	return n.Title
}

// ---------------------------------------------------------------------
// #1 — wait_until: block until a predicate on a found element is true.
//
// Built on existing FindFirst polling loop with a deadline. We don't
// piggyback on the kinax Observer here because Observer's notifications
// are coarse (AXValueChanged, AXFocusedWindowChanged) and don't always
// fire for the boolean state predicates we care about (AXEnabled,
// AXSelected, AXFocused). A 250ms poll + 30s deadline is the right
// trade-off for human-driven UI flows.
// ---------------------------------------------------------------------

func (s *uiSkill) waitUntil(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	m, desc, err := s.matcher(params)
	if err != nil {
		return "", err
	}
	_ = atoiDefault(params["depth"], 20) // tryPredicateOnce uses default 20
	timeoutMs := atoiDefault(params["timeout_ms"], 10000)
	if timeoutMs <= 0 {
		timeoutMs = 10000
	}
	if timeoutMs > 60000 {
		timeoutMs = 60000
	}

	predicate := strings.ToLower(strings.TrimSpace(params["predicate"]))
	if predicate == "" {
		predicate = "appears"
	}

	// Hybrid loop strategy:
	//
	//   1. For "value-changes-on-event" predicates (appears /
	//      disappears / focused) we attempt an Observer fast-path:
	//      subscribe to AXFocusedUIElementChanged + AXValueChanged +
	//      AXWindowCreated and wake on each event for an immediate
	//      re-check. Median latency: tens of ms (event → check)
	//      instead of 0-250ms (poll cycle).
	//
	//   2. For pure-state predicates (enabled / disabled / selected /
	//      visible) AX doesn't reliably emit a notification when the
	//      bool flips, so we keep the 250ms poll. The Observer
	//      subscription is harmless either way.
	//
	// We also take an immediate snapshot before subscribing so a
	// predicate already true doesn't wait for a tick.
	if matched, msg := tryPredicateOnce(app, m, desc, predicate); matched {
		return msg, nil
	}

	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	start := time.Now()

	pid := kinax.FrontmostPID()
	if p := params["pid"]; p != "" {
		pid = atoiDefault(p, pid)
	}
	var obs *kinax.Observer
	if pid > 0 {
		if o, err := kinax.NewObserver(pid); err == nil {
			subErr := o.Subscribe(app,
				kinax.NotifFocusedUIElementChanged,
				kinax.NotifValueChanged,
				kinax.NotifWindowCreated,
				kinax.NotifTitleChanged,
			)
			if subErr == nil {
				obs = o
				defer o.Close()
			} else {
				o.Close()
			}
		}
	}

	pollInterval := 250 * time.Millisecond
	for time.Now().Before(deadline) {
		// Try once before sleeping — the Observer wake fires us back
		// here and we want to check immediately, not after a poll
		// interval.
		if matched, msg := tryPredicateOnce(app, m, desc, predicate); matched {
			startMs := time.Since(start).Milliseconds()
			return fmt.Sprintf("%s (waited %dms)", msg, startMs), nil
		}

		// Wait for either an Observer event OR the poll interval,
		// whichever comes first. Observer.Next blocks up to its arg
		// duration; if no event by then, we poll-tick.
		var waitFor time.Duration
		if obs != nil {
			waitFor = pollInterval
		} else {
			waitFor = pollInterval
		}
		remaining := time.Until(deadline)
		if waitFor > remaining {
			waitFor = remaining
		}
		if waitFor <= 0 {
			break
		}
		if obs != nil {
			if ev, err := obs.Next(waitFor); err == nil && ev != nil {
				ev.Element.Close()
				continue // event fired — re-check at top of loop immediately
			}
		} else {
			time.Sleep(waitFor)
		}
	}
	return "", fmt.Errorf("wait_until: predicate %q on %s did not become true within %dms",
		predicate, desc, timeoutMs)
}

// tryPredicateOnce checks the predicate against the current AX state
// without sleeping. Used both for the pre-subscribe immediate snapshot
// and inside the loop after each Observer wake. Returns (true, msg)
// on match, (false, "") otherwise.
func tryPredicateOnce(app *kinax.Element, m kinax.Matcher, desc, predicate string) (bool, string) {
	el, ok := app.FindFirst(m, 20)
	if ok {
		match, label := evaluatePredicate(el, predicate)
		el.Close()
		if match {
			return true, fmt.Sprintf("matched %s (predicate: %s)", desc, label)
		}
	} else if predicate == "disappears" || predicate == "gone" {
		return true, fmt.Sprintf("matched (disappeared) %s", desc)
	}
	return false, ""
}

// evaluatePredicate runs the named predicate against a found element.
// "appears" is the default — element existence is itself the signal.
// Other predicates check element state attributes.
func evaluatePredicate(el *kinax.Element, predicate string) (bool, string) {
	switch predicate {
	case "appears", "exists":
		return true, "appears"
	case "enabled":
		v, err := el.Enabled()
		if err != nil {
			return false, "enabled (error)"
		}
		return v, "enabled"
	case "disabled":
		v, err := el.Enabled()
		if err != nil {
			return false, "disabled (error)"
		}
		return !v, "disabled"
	case "focused":
		v, err := el.Focused()
		return err == nil && v, "focused"
	case "selected":
		v, err := el.AttributeBool(kinax.AttrSelected)
		return err == nil && v, "selected"
	case "visible":
		v, err := el.AttributeBool(kinax.AttrVisible)
		return err == nil && v, "visible"
	case "disappears", "gone":
		// Reaches here only when FindFirst found something — i.e. NOT
		// gone. The disappears branch is checked at the FindFirst==false
		// case earlier; here we report no-match to keep polling.
		return false, "disappears"
	default:
		return false, "unknown predicate"
	}
}

// ---------------------------------------------------------------------
// #2 — menu_path: navigate macOS menu bar by string path.
//
// Walks `App > File > Export > PDF...` style paths through AXMenuBar →
// AXMenuItem trees. AXPress on each item except the last (which we
// AXPick to commit the action). Replaces the "screenshot, find Format,
// click, screenshot, find Cell..." multi-turn flow with a single call.
// ---------------------------------------------------------------------

func (s *uiSkill) menuPath(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	pathStr := strings.TrimSpace(params["path"])
	if pathStr == "" {
		return "", fmt.Errorf("menu_path: 'path' is required (e.g. 'File > Export > PDF...')")
	}

	// kinax v0.4+ exposes the menu walk + commit logic as a single
	// method. We just call through; the helper handles AXMenuBar →
	// AXMenuBarItem → AXMenu → AXMenuItem layering, sub-menu opens,
	// and the final AXPress.
	if err := app.NavigateMenu(pathStr); err != nil {
		return "", err
	}
	return fmt.Sprintf("menu_path: navigated %s", pathStr), nil
}

// ---------------------------------------------------------------------
// #7 — shortcut: read AXMenuItem's command-key equivalent.
//
// macOS menu items expose AXMenuItemCmdChar (the character) and
// AXMenuItemCmdModifiers (bitfield: 1=shift, 2=option, 4=control,
// 8=command). When a menu item has a shortcut, calling input.key with
// it is FAR faster + more reliable than menu-walking.
// ---------------------------------------------------------------------

func (s *uiSkill) shortcut(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	pathStr := strings.TrimSpace(params["path"])
	if pathStr == "" {
		return "", fmt.Errorf("shortcut: 'path' required (e.g. 'File > Save')")
	}

	// Walk the menu path WITHOUT committing the final action — we
	// just want to read the keyboard equivalent. NavigateMenu
	// presses the leaf, which we DON'T want here. Use the matcher
	// approach: find the leaf AXMenuItem by walking children of
	// AXMenuBar via the same logic kinax uses internally.
	//
	// Future kinax: split NavigateMenu into FindMenuItem(path) +
	// PressMenuItem(item). Until then, do the final-step lookup
	// here, then call MenuItemShortcut on the matched leaf.
	leaf, err := findMenuLeafForShortcut(app, pathStr)
	if err != nil {
		return "", err
	}
	defer leaf.Close()
	char, mods, vk, err := leaf.MenuItemShortcut()
	if err != nil {
		return fmt.Sprintf("shortcut: %s has no keyboard equivalent", pathStr), nil
	}
	return formatShortcut(char, mods, vk, pathStr), nil
}

// findMenuLeafForShortcut walks the menu path WITHOUT pressing any
// item. Returns the leaf AXMenuItem so the caller can read its
// keyboard equivalent via MenuItemShortcut. This is a kinclaw-
// internal helper because it duplicates partial logic that should
// eventually live in kinax-go as `FindMenuItem(path)` (separate
// from the press-leaf NavigateMenu). Tracked as a kit follow-up.
func findMenuLeafForShortcut(app *kinax.Element, path string) (*kinax.Element, error) {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '>' || r == '/' || r == '→'
	})
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	// Filter empty parts.
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("shortcut: empty path")
	}

	menubar, err := app.AttributeElement(kinax.AttrMenuBar)
	if err != nil || menubar == nil {
		return nil, fmt.Errorf("shortcut: app has no AXMenuBar")
	}
	defer menubar.Close()

	// Walk by child-title at each level. We DO need to open submenus
	// for AppKit to populate AXMenu contents, so we still press
	// intermediate items — just not the leaf.
	current := menubar
	for i, name := range cleaned {
		isLast := i == len(cleaned)-1
		kids, err := current.Children()
		if err != nil {
			return nil, fmt.Errorf("shortcut: at step %d (%q): %w", i+1, name, err)
		}
		var match *kinax.Element
		for _, k := range kids {
			t, _ := k.Title()
			if t == name && match == nil {
				match = k
			} else {
				k.Close()
			}
		}
		if match == nil {
			return nil, fmt.Errorf("shortcut: no menu item titled %q at step %d/%d", name, i+1, len(cleaned))
		}
		if isLast {
			return match, nil // caller closes
		}
		// Intermediate: open submenu so its children populate.
		_ = match.Perform(kinax.ActionPress)
		// Find the AXMenu child container.
		subKids, _ := match.Children()
		var menuChild *kinax.Element
		for _, sk := range subKids {
			r, _ := sk.Role()
			if r == kinax.RoleMenu && menuChild == nil {
				menuChild = sk
			} else {
				sk.Close()
			}
		}
		match.Close()
		if menuChild == nil {
			return nil, fmt.Errorf("shortcut: %q has no submenu", name)
		}
		current = menuChild
		// Brief settle for AppKit to populate.
		time.Sleep(80 * time.Millisecond)
	}
	return nil, fmt.Errorf("shortcut: unreachable")
}

func formatShortcut(char string, modBits int, vk int, path string) string {
	var mods []string
	if modBits&8 == 0 { // bit 3: NO command means cmd present (Apple's
		// convention is inverted here — 0 = ⌘ implicit, 1 = no ⌘).
		mods = append(mods, "⌘")
	}
	if modBits&1 != 0 {
		mods = append(mods, "⇧")
	}
	if modBits&2 != 0 {
		mods = append(mods, "⌥")
	}
	if modBits&4 != 0 {
		mods = append(mods, "⌃")
	}
	keyLabel := strings.ToUpper(char)
	if char == "" && vk != 0 {
		keyLabel = fmt.Sprintf("(virtual_key=%d)", vk)
	}
	return fmt.Sprintf("shortcut for %s: %s%s",
		path, strings.Join(mods, ""), keyLabel)
}

// ---------------------------------------------------------------------
// #6 — select_text: read or set selected text on the focused text element.
//
// kinax-go's SetString already does whole-value replacement; this verb
// adds the useful sub-cases: read current selection, replace selection
// with new text, append at cursor without overwriting. Built atop
// AXSelectedText (read) and AXSelectedText (write — same attr name,
// settable). For range-precise insertion we'd need AXSelectedTextRange
// which is a CFRange and requires kinax-go bridging support — skipped
// in v1.13, comes when needed.
// ---------------------------------------------------------------------

func (s *uiSkill) selectText(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	// Find target. Prefer the focused UI element (where the cursor
	// actually is) when no matcher is given; fall back to matcher
	// when caller supplies one.
	var target *kinax.Element
	if hasMatcherParam(params) {
		m, desc, err := s.matcher(params)
		if err != nil {
			return "", err
		}
		depth := atoiDefault(params["depth"], 20)
		t, ok := app.FindFirst(m, depth)
		if !ok {
			return "", fmt.Errorf("select_text: no element matching %s", desc)
		}
		target = t
	} else {
		t, err := app.AttributeElement(kinax.AttrFocusedElement)
		if err != nil || t == nil {
			return "", fmt.Errorf("select_text: app has no focused element (focus the text field first)")
		}
		target = t
	}
	defer target.Close()

	mode := strings.ToLower(strings.TrimSpace(params["mode"]))
	if mode == "" {
		mode = "read"
	}

	switch mode {
	case "read":
		sel, _ := target.Attribute(kinax.AttrSelectedText)
		if sel == "" {
			return "select_text: no text currently selected", nil
		}
		return fmt.Sprintf("selected text (%d chars):\n%s",
			len(sel), truncateValue(sel)), nil
	case "replace":
		newText := params["text"]
		// Setting AXSelectedText replaces the current selection with
		// the new value in most AXTextField / AXTextArea targets.
		if err := target.SetString(kinax.AttrSelectedText, newText); err != nil {
			return "", fmt.Errorf("select_text replace: %w", err)
		}
		return fmt.Sprintf("replaced selection with %d chars", len(newText)), nil
	default:
		return "", fmt.Errorf("select_text: mode must be 'read' or 'replace' (got %q)", mode)
	}
}

func hasMatcherParam(p map[string]string) bool {
	return p["role"] != "" || p["title"] != "" || p["title_contains"] != "" ||
		p["identifier"] != "" || p["description"] != ""
}
