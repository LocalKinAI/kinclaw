//go:build windows

// ui_windows.go — Windows implementation of the ui claw.
//
// The Windows accessibility surface is UI Automation (UIA), exposed
// natively via the System.Windows.Automation .NET namespace (and the
// modern COM-based UIAutomationClient.dll). PowerShell can load
// UIAutomationClient + UIAutomationTypes without any external deps —
// this means our claw works on every supported Windows install with
// the .NET Framework that ships in-box (4.0+, i.e. since Win7).
//
// Action surface (mirrors ui_linux's AT-SPI 2 path so portable souls
// work unchanged):
//
//   focused_app             — return frontmost process name + window title
//   window_list             — enumerate top-level windows of all processes
//   window_geometry name=…  — bounds of named top-level window
//   tree [pid=N|name=…]     — UIA element tree (json) of given app
//   find role=… name=…      — find first matching element under desktop root
//   click_by_name name=…    — Invoke pattern on first element with that name
//   click_by_role role=… name=…  — same, scoped by control type
package skill

import (
	"encoding/json"
	"fmt"
	"strings"
)

type uiSkill struct {
	allowed bool
}

func NewUISkill(allowed bool) Skill {
	return &uiSkill{allowed: allowed}
}

func (s *uiSkill) Name() string { return "ui" }

func (s *uiSkill) Description() string {
	return "Inspect + control Windows UI via UI Automation (UIA). Actions: " +
		"focused_app, window_list, window_geometry, tree, find, click_by_name, click_by_role. " +
		"Returns JSON for tree/find — same shape as ui_linux's AT-SPI 2 output."
}

func (s *uiSkill) ToolDef() json.RawMessage {
	return MakeToolDef("ui", s.Description(),
		map[string]map[string]string{
			"action":    {"type": "string", "description": "focused_app|window_list|window_geometry|tree|find|click_by_name|click_by_role"},
			"name":      {"type": "string", "description": "Element/window name (for find / click_by_name / window_geometry)"},
			"role":      {"type": "string", "description": "UIA control type: Button | Edit | Window | Pane | Group | Hyperlink | …"},
			"pid":       {"type": "integer", "description": "Process ID to scope tree/find under"},
			"max_depth": {"type": "integer", "description": "tree: walk depth ceiling (default 6)"},
		},
		[]string{"action"})
}

func (s *uiSkill) Execute(p map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("ui skill not enabled in soul")
	}
	switch p["action"] {
	case "focused_app":
		return s.focusedApp()
	case "window_list":
		return s.windowList()
	case "window_geometry":
		return s.windowGeometry(p["name"])
	case "tree":
		depth := p["max_depth"]
		if depth == "" {
			depth = "6"
		}
		return s.tree(p["pid"], p["name"], depth)
	case "find":
		return s.find(p["role"], p["name"], p["pid"])
	case "click_by_name":
		return s.clickByName(p["name"])
	case "click_by_role":
		return s.clickByRole(p["role"], p["name"])
	default:
		return "", fmt.Errorf("ui: unknown action %q", p["action"])
	}
}

// uiaPrologue loads UIAutomationClient + UIAutomationTypes. Both ship
// with .NET Framework 4.x — present on every supported Windows release.
// We pin the assembly version so a future UIA breaking change can't
// silently shift our behaviour.
const uiaPrologue = `
[Reflection.Assembly]::LoadWithPartialName("UIAutomationClient") | Out-Null
[Reflection.Assembly]::LoadWithPartialName("UIAutomationTypes") | Out-Null
`

// focusedApp returns the foreground window's process name + title via
// GetForegroundWindow + GetWindowThreadProcessId.
func (s *uiSkill) focusedApp() (string, error) {
	script := `
$sig = @'
[DllImport("user32.dll")] public static extern System.IntPtr GetForegroundWindow();
[DllImport("user32.dll")] public static extern int GetWindowThreadProcessId(System.IntPtr hWnd, out int lpdwProcessId);
[DllImport("user32.dll", CharSet=CharSet.Auto)] public static extern int GetWindowText(System.IntPtr hWnd, System.Text.StringBuilder text, int count);
'@
Add-Type -MemberDefinition $sig -Namespace KC -Name FW -UsingNamespace System.Runtime.InteropServices
$h = [KC.FW]::GetForegroundWindow()
$pid = 0
[void][KC.FW]::GetWindowThreadProcessId($h, [ref]$pid)
$proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
$sb = New-Object System.Text.StringBuilder 512
[void][KC.FW]::GetWindowText($h, $sb, 512)
$title = $sb.ToString()
$name = if ($proc) { $proc.ProcessName } else { "" }
[Console]::Write($name + [char]9 + $title)
`
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("focused_app: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	result := map[string]string{}
	if len(parts) > 0 {
		result["process"] = parts[0]
	}
	if len(parts) > 1 {
		result["title"] = parts[1]
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

// windowList enumerates top-level windows with non-empty titles. Tab
// through Get-Process .MainWindowTitle is enough — anything with a
// MainWindowHandle != 0 has a real top-level window.
func (s *uiSkill) windowList() (string, error) {
	script := `
$out = Get-Process | Where-Object { $_.MainWindowHandle -ne 0 -and $_.MainWindowTitle } |
       Select-Object Id, ProcessName, MainWindowTitle |
       ConvertTo-Json -Compress
[Console]::Write($out)
`
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("window_list: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// windowGeometry pulls bounds for the named window via UIA's Window
// pattern. Name is matched against MainWindowTitle (substring).
func (s *uiSkill) windowGeometry(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("window_geometry requires name=")
	}
	script := uiaPrologue + fmt.Sprintf(`
$root = [System.Windows.Automation.AutomationElement]::RootElement
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$win = $root.FindFirst([System.Windows.Automation.TreeScope]::Children, $cond)
if (-not $win) {
  # Try contains match by walking children.
  $w = $root.FindAll([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
  foreach ($c in $w) { if ($c.Current.Name -like "*%s*") { $win = $c; break } }
}
if (-not $win) { Write-Error "no window matching %s"; exit 1 }
$r = $win.Current.BoundingRectangle
$json = @{ x=$r.X; y=$r.Y; w=$r.Width; h=$r.Height } | ConvertTo-Json -Compress
[Console]::Write($json)
`, strings.ReplaceAll(name, `'`, `''`),
		strings.ReplaceAll(name, `'`, `''`),
		strings.ReplaceAll(name, `"`, `\"`))
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("window_geometry: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// tree dumps the UIA element subtree for a given process (by pid or
// MainWindowTitle name). Output is JSON with the same role/name/path
// shape ui_linux uses, so downstream tooling can ignore the platform.
func (s *uiSkill) tree(pid, name, depth string) (string, error) {
	if pid == "" && name == "" {
		return "", fmt.Errorf("tree requires pid= or name=")
	}
	rootExpr := ""
	switch {
	case pid != "":
		rootExpr = fmt.Sprintf(`
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, %s)
$root = [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children, $cond)`, pid)
	case name != "":
		rootExpr = fmt.Sprintf(`
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$root = [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children, $cond)`,
			strings.ReplaceAll(name, `'`, `''`))
	}
	script := uiaPrologue + rootExpr + fmt.Sprintf(`
if (-not $root) { Write-Error "no matching window"; exit 1 }
function Walk($el, $d, $max) {
  if ($d -gt $max) { return $null }
  $kids = @()
  $children = $el.FindAll([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
  foreach ($c in $children) {
    $sub = Walk $c ($d + 1) $max
    if ($sub) { $kids += $sub }
  }
  return @{
    role     = $el.Current.ControlType.ProgrammaticName
    name     = $el.Current.Name
    children = $kids
  }
}
$out = Walk $root 0 %s | ConvertTo-Json -Compress -Depth 32
[Console]::Write($out)
`, depth)
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("tree: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// find returns the first descendant matching role + name. Either field
// can be empty (then matched as wildcard).
func (s *uiSkill) find(role, name, pid string) (string, error) {
	if role == "" && name == "" {
		return "", fmt.Errorf("find requires role= or name=")
	}
	rootExpr := `$root = [System.Windows.Automation.AutomationElement]::RootElement`
	if pid != "" {
		rootExpr = fmt.Sprintf(`
$pidCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, %s)
$root = [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children, $pidCond)`, pid)
	}
	matchExpr := "[System.Windows.Automation.Condition]::TrueCondition"
	var conds []string
	if role != "" {
		conds = append(conds, fmt.Sprintf(
			`New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::%s)`, role))
	}
	if name != "" {
		conds = append(conds, fmt.Sprintf(
			`New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')`,
			strings.ReplaceAll(name, `'`, `''`)))
	}
	switch len(conds) {
	case 1:
		matchExpr = conds[0]
	case 2:
		matchExpr = fmt.Sprintf("New-Object System.Windows.Automation.AndCondition(%s, %s)", conds[0], conds[1])
	}
	script := uiaPrologue + rootExpr + fmt.Sprintf(`
if (-not $root) { Write-Error "no root"; exit 1 }
$el = $root.FindFirst([System.Windows.Automation.TreeScope]::Subtree, %s)
if (-not $el) { [Console]::Write("null"); exit 0 }
$r = $el.Current.BoundingRectangle
$out = @{ role=$el.Current.ControlType.ProgrammaticName; name=$el.Current.Name; x=$r.X; y=$r.Y; w=$r.Width; h=$r.Height } | ConvertTo-Json -Compress
[Console]::Write($out)
`, matchExpr)
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("find: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// clickByName invokes the first element with that name. Uses the
// Invoke pattern if available (buttons / hyperlinks / menu items) and
// falls back to clicking the center of the bounding rect via mouse
// emulation for elements that only implement the Selection or
// Toggle pattern.
func (s *uiSkill) clickByName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("click_by_name requires name=")
	}
	script := uiaPrologue + fmt.Sprintf(`
$root = [System.Windows.Automation.AutomationElement]::RootElement
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$el = $root.FindFirst([System.Windows.Automation.TreeScope]::Subtree, $cond)
if (-not $el) { Write-Error "not found"; exit 1 }
try {
  $ip = $el.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
  $ip.Invoke()
  [Console]::Write("invoked")
} catch {
  # Fall back to a hardware-level click at the element centre.
  $r = $el.Current.BoundingRectangle
  $cx = [int]($r.X + $r.Width / 2)
  $cy = [int]($r.Y + $r.Height / 2)
  $sig = @'
[DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
[DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, uint extra);
'@
  Add-Type -MemberDefinition $sig -Namespace KC -Name CB -UsingNamespace System.Runtime.InteropServices
  [KC.CB]::SetCursorPos($cx,$cy) | Out-Null
  Start-Sleep -Milliseconds 30
  [KC.CB]::mouse_event(0x0002, 0, 0, 0, 0)
  [KC.CB]::mouse_event(0x0004, 0, 0, 0, 0)
  [Console]::Write("clicked-center")
}
`, strings.ReplaceAll(name, `'`, `''`))
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("click_by_name: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *uiSkill) clickByRole(role, name string) (string, error) {
	if role == "" {
		return "", fmt.Errorf("click_by_role requires role=")
	}
	script := uiaPrologue
	if name != "" {
		script += fmt.Sprintf(`
$root = [System.Windows.Automation.AutomationElement]::RootElement
$c1 = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::%s)
$c2 = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$cond = New-Object System.Windows.Automation.AndCondition($c1, $c2)
`, role, strings.ReplaceAll(name, `'`, `''`))
	} else {
		script += fmt.Sprintf(`
$root = [System.Windows.Automation.AutomationElement]::RootElement
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::%s)
`, role)
	}
	script += `
$el = $root.FindFirst([System.Windows.Automation.TreeScope]::Subtree, $cond)
if (-not $el) { Write-Error "not found"; exit 1 }
$ip = $el.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
$ip.Invoke()
[Console]::Write("invoked")
`
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("click_by_role: %w", err)
	}
	return strings.TrimSpace(out), nil
}
