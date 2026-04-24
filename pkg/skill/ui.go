//go:build darwin

package skill

import (
	"encoding/json"
	"fmt"
	"strings"

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
		"focused_app, tree, find, click, read, at_point. Requires macOS " +
		"Accessibility permission."
}

func (s *uiSkill) ToolDef() json.RawMessage {
	return MakeToolDef("ui", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "focused_app | tree | find | click | read | at_point",
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
			"depth": {"type": "integer", "description": "Search depth (default 20 for find/click, 6 for tree)"},
			"x":     {"type": "number", "description": "X (for at_point)"},
			"y":     {"type": "number", "description": "Y (for at_point)"},
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
	case "read":
		return s.read(params)
	case "at_point":
		return s.atPoint(params)
	default:
		return "", fmt.Errorf("unknown action %q", action)
	}
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

func dumpTree(sb *strings.Builder, e *kinax.Element, indent, maxDepth int) {
	role, _ := e.Role()
	title, _ := e.Title()
	id, _ := e.Identifier()
	pad := strings.Repeat("  ", indent)
	line := pad + role
	if title != "" {
		line += fmt.Sprintf(" %q", title)
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

	if clickFirst {
		el, ok := app.FindFirst(m, depth)
		if !ok {
			return "", fmt.Errorf("no element matching %s", desc)
		}
		defer el.Close()
		if err := el.Perform(kinax.ActionPress); err != nil {
			return "", fmt.Errorf("AXPress: %w", err)
		}
		role, _ := el.Role()
		title, _ := el.Title()
		return fmt.Sprintf("clicked %s %q (matched %s)", role, title, desc), nil
	}

	hits := app.FindAll(m, depth)
	if len(hits) == 0 {
		return fmt.Sprintf("no elements matching %s", desc), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d match(es) for %s:\n", len(hits), desc))
	for _, h := range hits {
		r, _ := h.Role()
		t, _ := h.Title()
		id, _ := h.Identifier()
		sb.WriteString(fmt.Sprintf("  %s\t%q\t[%s]\n", r, t, id))
		h.Close()
	}
	return sb.String(), nil
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
