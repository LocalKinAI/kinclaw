//go:build linux

// ui_linux.go — Linux implementation of the ui claw.
//
// Two-tier strategy mirroring the macOS uiSkill:
//
//   1. Window-level queries via xdotool + wmctrl
//      (fast, universal, no setup):
//        focused_app | window_list | window_geometry
//
//   2. Accessibility-tree queries via AT-SPI 2 over D-Bus
//      (requires at-spi2-core daemon + GNOME-class DE):
//        tree | find | click_by_name | click_by_role
//
// AT-SPI 2 implementation notes:
//   - Connects to the session bus, queries org.a11y.Bus for the
//     dedicated a11y bus address, opens a second connection there.
//   - Walks org.a11y.atspi.Registry to enumerate top-level apps,
//     then walks each app's accessible tree (depth-limited to avoid
//     unbounded recursion on JS-heavy apps like Chrome).
//   - For "click" we use the Action interface's DoAction method
//     (every actionable AT-SPI object exposes click as action 0).
//
// Wayland caveat: AT-SPI 2 works on GNOME Wayland because GNOME
// implements the bridge; on Sway / Hyprland / other compositors
// without an a11y bridge, the registry will be empty. The skill
// degrades gracefully — `tree` returns "(no top-level apps found)"
// rather than erroring.
//
// TODO(linux-verify): the AT-SPI D-Bus paths and signatures below
// are written from the AT-SPI 2 spec without runtime testing.
// Smoke test on Ubuntu 24.04 + GNOME (Wayland and X11) before
// claiming feature parity.

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

type uiSkill struct {
	allowed bool
}

func NewUISkill(allowed bool) Skill {
	return &uiSkill{allowed: allowed}
}

func (s *uiSkill) Name() string { return "ui" }

func (s *uiSkill) Description() string {
	return "Linux UI inspection. Window-level: focused_app | window_list | " +
		"window_geometry (via xdotool/wmctrl, X11 only). " +
		"Accessibility tree (AT-SPI 2 via D-Bus): tree | find | click_by_name | " +
		"click_by_role (requires at-spi2-core daemon; GNOME-Wayland or any X11 DE)."
}

func (s *uiSkill) ToolDef() json.RawMessage {
	return MakeToolDef("ui", s.Description(),
		map[string]map[string]string{
			"action": {
				"type":        "string",
				"description": "focused_app | window_list | window_geometry | tree | find | click_by_name | click_by_role",
			},
			"name": {
				"type":        "string",
				"description": "For find / click_by_name: substring of accessible name to match (case-insensitive)",
			},
			"role": {
				"type":        "string",
				"description": "For find / click_by_role: AT-SPI role name (e.g. 'push button', 'menu item', 'text')",
			},
			"depth": {
				"type":        "integer",
				"description": "For tree: max walk depth (default 4). Higher = more verbose, larger token cost.",
			},
			"app": {
				"type":        "string",
				"description": "For tree / find: limit walk to one app (substring match on app name). Default: all visible apps.",
			},
		}, nil)
}

func (s *uiSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `ui` capability")
	}
	action := params["action"]
	if action == "" {
		action = "focused_app"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	switch action {
	// ── window-level (xdotool/wmctrl) ──
	case "focused_app":
		return s.focusedApp(ctx)
	case "window_list":
		return s.windowList(ctx)
	case "window_geometry":
		return s.windowGeometry(ctx)

	// ── AT-SPI 2 tree-walking ──
	case "tree":
		return s.atspiTree(params)
	case "find":
		return s.atspiFind(params)
	case "click_by_name":
		return s.atspiClick(params, "name")
	case "click_by_role":
		return s.atspiClick(params, "role")

	default:
		return "", fmt.Errorf("ui action %q not implemented on Linux (see ui_linux.go for coverage)", action)
	}
}

// =============== Window-level (X11 / xdotool / wmctrl) ===============

func (s *uiSkill) focusedApp(ctx context.Context) (string, error) {
	if !commandExists("xdotool") {
		return "", fmt.Errorf("xdotool not installed")
	}
	idOut, err := exec.CommandContext(ctx, "xdotool", "getactivewindow").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xdotool getactivewindow: %w (%s)", err, strings.TrimSpace(string(idOut)))
	}
	wid := strings.TrimSpace(string(idOut))
	if wid == "" {
		return "", fmt.Errorf("no active window")
	}
	nameOut, _ := exec.CommandContext(ctx, "xdotool", "getwindowname", wid).CombinedOutput()
	classOut, _ := exec.CommandContext(ctx, "xdotool", "getwindowclassname", wid).CombinedOutput()
	return fmt.Sprintf("{\"window_id\":%q,\"title\":%q,\"class\":%q}",
		wid,
		strings.TrimSpace(string(nameOut)),
		strings.TrimSpace(string(classOut))), nil
}

func (s *uiSkill) windowList(ctx context.Context) (string, error) {
	if !commandExists("wmctrl") {
		return "", fmt.Errorf("wmctrl not installed")
	}
	out, err := exec.CommandContext(ctx, "wmctrl", "-l", "-p", "-x").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wmctrl: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *uiSkill) windowGeometry(ctx context.Context) (string, error) {
	if !commandExists("xdotool") {
		return "", fmt.Errorf("xdotool not installed")
	}
	idOut, err := exec.CommandContext(ctx, "xdotool", "getactivewindow").CombinedOutput()
	if err != nil {
		return "", err
	}
	wid := strings.TrimSpace(string(idOut))
	geoOut, err := exec.CommandContext(ctx, "xdotool", "getwindowgeometry", wid).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xdotool getwindowgeometry: %w (%s)", err, strings.TrimSpace(string(geoOut)))
	}
	return strings.TrimSpace(string(geoOut)), nil
}

// =============== AT-SPI 2 tree walking (godbus) ===============

// atspiConn opens a connection to the a11y bus. Per AT-SPI spec, the
// session bus exposes org.a11y.Bus which returns the address of a
// separate bus dedicated to accessibility traffic.
func atspiConn() (*dbus.Conn, error) {
	sess, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("session bus: %w", err)
	}
	var addr string
	obj := sess.Object("org.a11y.Bus", "/org/a11y/bus")
	if err := obj.Call("org.a11y.Bus.GetAddress", 0).Store(&addr); err != nil {
		return nil, fmt.Errorf("org.a11y.Bus.GetAddress: %w (is at-spi2-core daemon running?)", err)
	}
	if addr == "" {
		return nil, fmt.Errorf("a11y bus address is empty (at-spi2 not started)")
	}
	conn, err := dbus.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("dial a11y bus %q: %w", addr, err)
	}
	if err := conn.Auth(nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("a11y bus auth: %w", err)
	}
	if err := conn.Hello(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("a11y bus hello: %w", err)
	}
	return conn, nil
}

// atspiRef encodes a (bus_name, object_path) tuple as used throughout
// AT-SPI 2. Children/parent references are returned as this struct.
type atspiRef struct {
	Name string
	Path dbus.ObjectPath
}

func (r atspiRef) zero() bool { return r.Name == "" || r.Path == "" || r.Path == "/org/a11y/atspi/null" }

// atspiObj returns a generic D-Bus object handle for an AT-SPI ref.
func atspiObj(conn *dbus.Conn, ref atspiRef) dbus.BusObject {
	return conn.Object(ref.Name, ref.Path)
}

// atspiRoot returns the root of the registry — its children are the
// top-level applications currently registered with AT-SPI.
func atspiRoot() atspiRef {
	return atspiRef{
		Name: "org.a11y.atspi.Registry",
		Path: dbus.ObjectPath("/org/a11y/atspi/accessible/root"),
	}
}

// atspiName / atspiRole / atspiChildren — minimal Accessible interface
// wrappers. Each one is one D-Bus call; on a deeply-nested tree these
// add up, so the tree walker depth-caps aggressively.
func atspiName(conn *dbus.Conn, r atspiRef) string {
	if r.zero() {
		return ""
	}
	v, err := atspiObj(conn, r).GetProperty("org.a11y.atspi.Accessible.Name")
	if err != nil {
		return ""
	}
	if s, ok := v.Value().(string); ok {
		return s
	}
	return ""
}

func atspiRole(conn *dbus.Conn, r atspiRef) string {
	if r.zero() {
		return ""
	}
	var name string
	err := atspiObj(conn, r).Call("org.a11y.atspi.Accessible.GetRoleName", 0).Store(&name)
	if err != nil {
		return ""
	}
	return name
}

func atspiChildren(conn *dbus.Conn, r atspiRef) []atspiRef {
	if r.zero() {
		return nil
	}
	var v dbus.Variant
	err := atspiObj(conn, r).Call("org.freedesktop.DBus.Properties.Get", 0,
		"org.a11y.atspi.Accessible", "ChildCount").Store(&v)
	if err != nil {
		return nil
	}
	count, ok := v.Value().(int32)
	if !ok || count <= 0 {
		return nil
	}
	out := make([]atspiRef, 0, count)
	for i := int32(0); i < count; i++ {
		var child atspiRef
		if err := atspiObj(conn, r).Call("org.a11y.atspi.Accessible.GetChildAtIndex", 0, i).Store(&child); err == nil {
			out = append(out, child)
		}
	}
	return out
}

// atspiTree walks the accessibility tree under the registry root and
// returns a JSON-shaped indented text dump.
func (s *uiSkill) atspiTree(params map[string]string) (string, error) {
	depth := atoiOr(params["depth"], 4)
	appFilter := strings.ToLower(strings.TrimSpace(params["app"]))

	conn, err := atspiConn()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	apps := atspiChildren(conn, atspiRoot())
	if len(apps) == 0 {
		return "(no top-level apps found — at-spi2 may not be running, " +
			"or the compositor may not have an a11y bridge)", nil
	}

	var sb strings.Builder
	for _, app := range apps {
		appName := atspiName(conn, app)
		if appFilter != "" && !strings.Contains(strings.ToLower(appName), appFilter) {
			continue
		}
		fmt.Fprintf(&sb, "● %s [%s]\n", appName, atspiRole(conn, app))
		walkATSPI(conn, app, 0, depth, &sb)
	}
	if sb.Len() == 0 {
		return fmt.Sprintf("(no app matched filter %q)", appFilter), nil
	}
	return sb.String(), nil
}

func walkATSPI(conn *dbus.Conn, r atspiRef, level, maxDepth int, sb *strings.Builder) {
	if level >= maxDepth {
		return
	}
	for _, child := range atspiChildren(conn, r) {
		name := atspiName(conn, child)
		role := atspiRole(conn, child)
		fmt.Fprintf(sb, "%s├─ %s [%s]\n", strings.Repeat("  ", level+1), name, role)
		walkATSPI(conn, child, level+1, maxDepth, sb)
	}
}

// atspiFind locates the first matching accessible by name and/or role
// across all visible apps. Returns a path-like representation.
func (s *uiSkill) atspiFind(params map[string]string) (string, error) {
	nameQ := strings.ToLower(strings.TrimSpace(params["name"]))
	roleQ := strings.ToLower(strings.TrimSpace(params["role"]))
	if nameQ == "" && roleQ == "" {
		return "", fmt.Errorf("find requires at least 'name' or 'role' param")
	}
	appFilter := strings.ToLower(strings.TrimSpace(params["app"]))

	conn, err := atspiConn()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	apps := atspiChildren(conn, atspiRoot())
	for _, app := range apps {
		appName := atspiName(conn, app)
		if appFilter != "" && !strings.Contains(strings.ToLower(appName), appFilter) {
			continue
		}
		if hit, ok := findInSubtree(conn, app, []string{appName}, nameQ, roleQ, 0, 8); ok {
			return hit, nil
		}
	}
	return "", fmt.Errorf("no accessible matched name=%q role=%q (across %d apps)", nameQ, roleQ, len(apps))
}

func findInSubtree(conn *dbus.Conn, r atspiRef, breadcrumb []string, nameQ, roleQ string, level, maxDepth int) (string, bool) {
	if level >= maxDepth {
		return "", false
	}
	for _, child := range atspiChildren(conn, r) {
		name := atspiName(conn, child)
		role := atspiRole(conn, child)
		nameMatch := nameQ == "" || strings.Contains(strings.ToLower(name), nameQ)
		roleMatch := roleQ == "" || strings.Contains(strings.ToLower(role), roleQ)
		if nameMatch && roleMatch && (nameQ != "" || roleQ != "") {
			path := append(append([]string{}, breadcrumb...), fmt.Sprintf("%s [%s]", name, role))
			return strings.Join(path, " > ") + "  (at " + string(child.Path) + " via " + child.Name + ")", true
		}
		if hit, ok := findInSubtree(conn, child,
			append(breadcrumb, fmt.Sprintf("%s [%s]", name, role)),
			nameQ, roleQ, level+1, maxDepth); ok {
			return hit, ok
		}
	}
	return "", false
}

// atspiClick finds an actionable accessible matching the criteria
// and invokes its first action (typically "click" or "press").
func (s *uiSkill) atspiClick(params map[string]string, by string) (string, error) {
	var nameQ, roleQ string
	switch by {
	case "name":
		nameQ = strings.ToLower(strings.TrimSpace(params["name"]))
		if nameQ == "" {
			return "", fmt.Errorf("click_by_name requires 'name' param")
		}
	case "role":
		roleQ = strings.ToLower(strings.TrimSpace(params["role"]))
		if roleQ == "" {
			return "", fmt.Errorf("click_by_role requires 'role' param")
		}
	}
	appFilter := strings.ToLower(strings.TrimSpace(params["app"]))

	conn, err := atspiConn()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	apps := atspiChildren(conn, atspiRoot())
	for _, app := range apps {
		appName := atspiName(conn, app)
		if appFilter != "" && !strings.Contains(strings.ToLower(appName), appFilter) {
			continue
		}
		if target, ok := findActionableInSubtree(conn, app, nameQ, roleQ, 0, 8); ok {
			// Invoke action 0 on the Action interface.
			if err := atspiObj(conn, target).Call("org.a11y.atspi.Action.DoAction", 0, int32(0)).Err; err != nil {
				return "", fmt.Errorf("DoAction(0): %w", err)
			}
			return fmt.Sprintf("ok: clicked %s [%s] in %s",
				atspiName(conn, target),
				atspiRole(conn, target),
				appName), nil
		}
	}
	return "", fmt.Errorf("no actionable accessible matched (name=%q role=%q)", nameQ, roleQ)
}

func findActionableInSubtree(conn *dbus.Conn, r atspiRef, nameQ, roleQ string, level, maxDepth int) (atspiRef, bool) {
	if level >= maxDepth {
		return atspiRef{}, false
	}
	for _, child := range atspiChildren(conn, r) {
		name := atspiName(conn, child)
		role := atspiRole(conn, child)
		nameMatch := nameQ == "" || strings.Contains(strings.ToLower(name), nameQ)
		roleMatch := roleQ == "" || strings.Contains(strings.ToLower(role), roleQ)
		if nameMatch && roleMatch && (nameQ != "" || roleQ != "") {
			// Check the Action interface for at least one action.
			var n int32
			err := atspiObj(conn, child).Call("org.a11y.atspi.Action.GetNActions", 0).Store(&n)
			if err == nil && n > 0 {
				return child, true
			}
		}
		if hit, ok := findActionableInSubtree(conn, child, nameQ, roleQ, level+1, maxDepth); ok {
			return hit, ok
		}
	}
	return atspiRef{}, false
}

var _ = strconv.Atoi // keep strconv import for atoiOr helper file
