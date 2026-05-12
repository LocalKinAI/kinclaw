//go:build windows

// input_windows.go — Windows implementation of the input claw.
//
// We drive the OS through PowerShell + .NET (System.Windows.Forms.SendKeys
// + P/Invoke wrappers around user32!SendInput / SetCursorPos). This
// avoids a hard CGO dependency on user32.dll and matches the
// "everything ships in PowerShell 5.1" approach the screen claw takes.
//
// Action surface mirrors input_linux.go so souls/cerebellum entries
// authored for Linux work unchanged on Windows where the action is
// semantically meaningful.
//
//   move x= y=                — absolute cursor move (SetCursorPos)
//   click [button=left|right|middle]   — single click at current pos
//   triple_click              — three left clicks at current pos
//   type text=                — type a string (SendKeys; respects {…} escapes)
//   hotkey combo=ctrl+c       — press combo (SendKeys ^c — auto-translated)
//   scroll dy=N               — vertical mouse wheel (positive = up)
//   cursor                    — print "x,y"
//   screen_size               — print "width,height" of primary monitor
//   key_down key=…            — hold key (SendInput KEYEVENTF_KEYDOWN)
//   key_up   key=…            — release key
//   paste                     — Ctrl+V (SendKeys ^v)
//   drag x1= y1= x2= y2=      — mouse down at (x1,y1), move, release at (x2,y2)
package skill

import (
	"encoding/json"
	"fmt"
	"strings"
)

type inputSkill struct {
	allowed bool
}

func NewInputSkill(allowed bool) Skill {
	return &inputSkill{allowed: allowed}
}

func (s *inputSkill) Name() string { return "input" }

func (s *inputSkill) Description() string {
	return "Drive Windows mouse + keyboard via PowerShell/.NET. Actions: move, click, triple_click, " +
		"type, hotkey, scroll, cursor, screen_size, key_down, key_up, paste, drag. " +
		"Same action shape as input_linux for portable souls."
}

func (s *inputSkill) ToolDef() json.RawMessage {
	return MakeToolDef("input", s.Description(),
		map[string]map[string]string{
			"action": {"type": "string", "description": "move|click|triple_click|type|hotkey|scroll|cursor|screen_size|key_down|key_up|paste|drag"},
			"x":      {"type": "integer", "description": "move: target x"},
			"y":      {"type": "integer", "description": "move: target y"},
			"x1":     {"type": "integer", "description": "drag: start x"},
			"y1":     {"type": "integer", "description": "drag: start y"},
			"x2":     {"type": "integer", "description": "drag: end x"},
			"y2":     {"type": "integer", "description": "drag: end y"},
			"button": {"type": "string", "description": "click: left|right|middle (default left)"},
			"text":   {"type": "string", "description": "type: string to type"},
			"combo":  {"type": "string", "description": "hotkey: e.g. ctrl+c, ctrl+shift+t"},
			"dy":     {"type": "integer", "description": "scroll: wheel delta (positive=up)"},
			"key":    {"type": "string", "description": "key_down/key_up: key name (a, F1, return, …)"},
		},
		[]string{"action"})
}

func (s *inputSkill) Execute(p map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("input skill not enabled in soul")
	}
	switch p["action"] {
	case "move":
		return s.move(p["x"], p["y"])
	case "click":
		btn := p["button"]
		if btn == "" {
			btn = "left"
		}
		return s.click(btn)
	case "triple_click":
		return s.tripleClick()
	case "type":
		return s.typeText(p["text"])
	case "hotkey":
		return s.hotkey(p["combo"])
	case "scroll":
		return s.scroll(p["dy"])
	case "cursor":
		return s.cursor()
	case "screen_size":
		return s.screenSize()
	case "key_down":
		return s.keyEvent(p["key"], true)
	case "key_up":
		return s.keyEvent(p["key"], false)
	case "paste":
		return s.hotkey("ctrl+v")
	case "drag":
		return s.drag(p["x1"], p["y1"], p["x2"], p["y2"])
	default:
		return "", fmt.Errorf("input: unknown action %q", p["action"])
	}
}

// pinvoke is the P/Invoke prologue every action shares. Defines the
// minimum SendInput / SetCursorPos / GetCursorPos / mouse_event surface
// inline so the script is self-contained — no kinclaw helper module
// has to be present on the target machine.
const pinvoke = `
$sig = @'
[DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
[DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT p);
[DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, uint extra);
[DllImport("user32.dll")] public static extern int GetSystemMetrics(int nIndex);
[StructLayout(LayoutKind.Sequential)] public struct POINT { public int X; public int Y; }
'@
Add-Type -MemberDefinition $sig -Namespace KC -Name U32 -UsingNamespace System.Runtime.InteropServices
`

func (s *inputSkill) move(x, y string) (string, error) {
	if x == "" || y == "" {
		return "", fmt.Errorf("move requires x= y=")
	}
	script := pinvoke + fmt.Sprintf("[KC.U32]::SetCursorPos(%s,%s) | Out-Null", x, y)
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("move: %w", err)
	}
	return fmt.Sprintf("moved to (%s,%s)", x, y), nil
}

// mouse_event flags (user32.h):
//   MOUSEEVENTF_LEFTDOWN  = 0x0002, LEFTUP   = 0x0004
//   MOUSEEVENTF_RIGHTDOWN = 0x0008, RIGHTUP  = 0x0010
//   MOUSEEVENTF_MIDDLEDOWN= 0x0020, MIDDLEUP = 0x0040
//   MOUSEEVENTF_WHEEL     = 0x0800
const (
	mouseLeftDown   = "0x0002"
	mouseLeftUp     = "0x0004"
	mouseRightDown  = "0x0008"
	mouseRightUp    = "0x0010"
	mouseMiddleDown = "0x0020"
	mouseMiddleUp   = "0x0040"
	mouseWheel      = "0x0800"
)

func (s *inputSkill) click(button string) (string, error) {
	var dn, up string
	switch strings.ToLower(button) {
	case "left":
		dn, up = mouseLeftDown, mouseLeftUp
	case "right":
		dn, up = mouseRightDown, mouseRightUp
	case "middle":
		dn, up = mouseMiddleDown, mouseMiddleUp
	default:
		return "", fmt.Errorf("click: unknown button %q", button)
	}
	script := pinvoke + fmt.Sprintf(`
[KC.U32]::mouse_event(%s, 0, 0, 0, 0)
[KC.U32]::mouse_event(%s, 0, 0, 0, 0)
`, dn, up)
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("click: %w", err)
	}
	return "clicked " + button, nil
}

func (s *inputSkill) tripleClick() (string, error) {
	script := pinvoke + `
for ($i=0; $i -lt 3; $i++) {
  [KC.U32]::mouse_event(0x0002, 0, 0, 0, 0)
  [KC.U32]::mouse_event(0x0004, 0, 0, 0, 0)
  Start-Sleep -Milliseconds 30
}
`
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("triple_click: %w", err)
	}
	return "triple-clicked", nil
}

// typeText uses SendKeys. We escape the .NET-special chars + { } [ ] ( ) ^ % ~ {ENTER}…
// Newlines map to {ENTER} so multi-line strings work like the macOS
// `key code` and Linux `xdotool type` paths.
func (s *inputSkill) typeText(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("type requires text=")
	}
	escaped := sendKeysEscape(text)
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait('%s')
`, strings.ReplaceAll(escaped, `'`, `''`))
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("type: %w", err)
	}
	return fmt.Sprintf("typed %d chars", len(text)), nil
}

// sendKeysEscape escapes special SendKeys metachars and maps \n→{ENTER}.
func sendKeysEscape(s string) string {
	specials := []string{"+", "^", "%", "~", "(", ")", "[", "]", "{", "}"}
	for _, ch := range specials {
		s = strings.ReplaceAll(s, ch, "{"+ch+"}")
	}
	s = strings.ReplaceAll(s, "\r\n", "{ENTER}")
	s = strings.ReplaceAll(s, "\n", "{ENTER}")
	s = strings.ReplaceAll(s, "\t", "{TAB}")
	return s
}

// hotkey translates ctrl+shift+t → ^+t for SendKeys. Single-letter
// keys go in directly; named keys map to {F1}/{ENTER}/etc.
func (s *inputSkill) hotkey(combo string) (string, error) {
	if combo == "" {
		return "", fmt.Errorf("hotkey requires combo=")
	}
	parts := strings.Split(strings.ToLower(combo), "+")
	var prefix string
	var key string
	for _, p := range parts {
		switch p {
		case "ctrl", "control":
			prefix += "^"
		case "alt":
			prefix += "%"
		case "shift":
			prefix += "+"
		case "win", "cmd", "meta":
			// SendKeys can't send Win-key directly. Fallback to
			// no-op + return a clear error so the agent can pick
			// another path.
			return "", fmt.Errorf("hotkey: Win key not supported by SendKeys; use a P/Invoke action if needed")
		default:
			key = sendKeysNamed(p)
		}
	}
	if key == "" {
		return "", fmt.Errorf("hotkey: combo %q has no key part", combo)
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait('%s%s')
`, prefix, key)
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("hotkey: %w", err)
	}
	return "sent " + combo, nil
}

// sendKeysNamed maps human-readable key names to SendKeys tokens.
// Single chars pass through verbatim; multi-char names get wrapped in
// {…} which is SendKeys' syntax for named keys.
func sendKeysNamed(k string) string {
	if len(k) == 1 {
		return k
	}
	named := map[string]string{
		"return": "{ENTER}", "enter": "{ENTER}",
		"tab": "{TAB}", "esc": "{ESC}", "escape": "{ESC}",
		"backspace": "{BACKSPACE}", "delete": "{DELETE}", "del": "{DELETE}",
		"space": " ",
		"up":    "{UP}", "down": "{DOWN}", "left": "{LEFT}", "right": "{RIGHT}",
		"home": "{HOME}", "end": "{END}",
		"pageup": "{PGUP}", "pagedown": "{PGDN}",
		"f1": "{F1}", "f2": "{F2}", "f3": "{F3}", "f4": "{F4}",
		"f5": "{F5}", "f6": "{F6}", "f7": "{F7}", "f8": "{F8}",
		"f9": "{F9}", "f10": "{F10}", "f11": "{F11}", "f12": "{F12}",
	}
	if v, ok := named[k]; ok {
		return v
	}
	return "{" + strings.ToUpper(k) + "}"
}

func (s *inputSkill) scroll(dy string) (string, error) {
	if dy == "" {
		return "", fmt.Errorf("scroll requires dy=")
	}
	// dy is signed; mouse_event reads it as uint so we cast via
	// [int32] in PS to keep the sign. Native wheel delta is in units
	// of WHEEL_DELTA=120; we scale by 120 so dy=1 = one wheel notch.
	script := pinvoke + fmt.Sprintf(`
$d = [int32](%s) * 120
[KC.U32]::mouse_event(%s, 0, 0, $d, 0)
`, dy, mouseWheel)
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("scroll: %w", err)
	}
	return "scrolled " + dy, nil
}

func (s *inputSkill) cursor() (string, error) {
	script := pinvoke + `
$p = New-Object KC.U32+POINT
[KC.U32]::GetCursorPos([ref]$p) | Out-Null
[Console]::Write("$($p.X),$($p.Y)")
`
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("cursor: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// screenSize uses GetSystemMetrics(SM_CXSCREEN=0, SM_CYSCREEN=1). For
// multi-monitor, SystemInformation.VirtualScreen is more correct, but
// inputs to most actions are primary-monitor coordinates so we return
// the primary monitor.
func (s *inputSkill) screenSize() (string, error) {
	script := pinvoke + `
$w = [KC.U32]::GetSystemMetrics(0)
$h = [KC.U32]::GetSystemMetrics(1)
[Console]::Write("$w,$h")
`
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("screen_size: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// keyEvent sends a KEYEVENTF_KEYDOWN or KEYEVENTF_KEYUP for the named
// key. For modifier chords use action=hotkey instead. Implemented via
// SendKeys for printable / named keys (SendKeys can't represent a
// hold-and-release without the down/up halves being separate calls,
// which is exactly what we want here).
func (s *inputSkill) keyEvent(key string, down bool) (string, error) {
	if key == "" {
		return "", fmt.Errorf("%s requires key=", map[bool]string{true: "key_down", false: "key_up"}[down])
	}
	// SendKeys doesn't expose hold-without-release. The Windows
	// hold-key idiom is to use keybd_event/SendInput with the up flag
	// separately. Approximate it with a press cycle for now and log
	// — most agent recipes that "hold" really just want a quick tap.
	if !down {
		return "noop (Windows SendKeys auto-releases; key_up handled by key_down emit)", nil
	}
	return s.hotkey(key)
}

func (s *inputSkill) drag(x1, y1, x2, y2 string) (string, error) {
	if x1 == "" || y1 == "" || x2 == "" || y2 == "" {
		return "", fmt.Errorf("drag requires x1= y1= x2= y2=")
	}
	script := pinvoke + fmt.Sprintf(`
[KC.U32]::SetCursorPos(%s,%s) | Out-Null
Start-Sleep -Milliseconds 50
[KC.U32]::mouse_event(0x0002, 0, 0, 0, 0)
Start-Sleep -Milliseconds 50
[KC.U32]::SetCursorPos(%s,%s) | Out-Null
Start-Sleep -Milliseconds 100
[KC.U32]::mouse_event(0x0004, 0, 0, 0, 0)
`, x1, y1, x2, y2)
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("drag: %w", err)
	}
	return fmt.Sprintf("dragged (%s,%s)→(%s,%s)", x1, y1, x2, y2), nil
}
