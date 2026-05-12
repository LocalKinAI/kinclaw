//go:build windows

// screen_windows.go — Windows implementation of the screen claw.
//
// PowerShell + .NET System.Drawing.Graphics.CopyFromScreen is the
// universal "capture the desktop" path on Windows — works on every
// supported Windows version from 7 SP1 forward and doesn't need any
// third-party binaries. Equivalent to grim/scrot on Linux and
// screencapture(1) on macOS.
//
// Actions exposed:
//   screenshot               — full virtual desktop → PNG
//   screenshot path=...      — same, but write to caller-specified path
//   list_displays            — enumerate physical monitors via WMI
//   capture_region x= y= w= h=  — crop to bounding box (CSS-style)
//
// Outputs use the same `image://path` marker convention as the Linux
// claws so the chat UI's path-detection regex picks them up unchanged.
package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type screenSkill struct {
	allowed   bool
	outputDir string
}

// NewScreenSkill matches the darwin/linux signature so the kernel's
// generic New() call compiles unchanged. outputDir defaults to
// ~/AppData/Local/kinclaw/screen if blank.
func NewScreenSkill(allowed bool, outputDir string) Skill {
	if outputDir == "" {
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			outputDir = filepath.Join(appdata, "kinclaw", "screen")
		} else {
			outputDir = filepath.Join(os.TempDir(), "kinclaw-screen")
		}
	}
	return &screenSkill{allowed: allowed, outputDir: outputDir}
}

func (s *screenSkill) Name() string { return "screen" }

func (s *screenSkill) Description() string {
	return "Capture the Windows desktop. Actions: screenshot, list_displays, capture_region. " +
		"Uses PowerShell + .NET System.Drawing (no third-party deps). " +
		"Returns 'image://PATH' so the chat UI can render the result."
}

func (s *screenSkill) ToolDef() json.RawMessage {
	return MakeToolDef("screen", s.Description(),
		map[string]map[string]string{
			"action": {"type": "string", "description": "screenshot | list_displays | capture_region"},
			"path":   {"type": "string", "description": "Output file path (PNG). Default: auto in output_dir."},
			"x":      {"type": "integer", "description": "capture_region: left edge (px from virtual desktop origin)"},
			"y":      {"type": "integer", "description": "capture_region: top edge"},
			"w":      {"type": "integer", "description": "capture_region: width"},
			"h":      {"type": "integer", "description": "capture_region: height"},
		},
		[]string{"action"})
}

func (s *screenSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("screen skill not enabled in soul")
	}
	if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	switch params["action"] {
	case "screenshot":
		return s.screenshot(params["path"])
	case "list_displays":
		return s.listDisplays()
	case "capture_region":
		return s.captureRegion(params)
	default:
		return "", fmt.Errorf("screen: unknown action %q (want: screenshot|list_displays|capture_region)",
			params["action"])
	}
}

// screenshot captures the entire virtual desktop. Multi-monitor setups
// are flattened into one image by walking SystemInformation.VirtualScreen,
// which matches what macOS/Linux full captures do.
func (s *screenSkill) screenshot(path string) (string, error) {
	if path == "" {
		path = filepath.Join(s.outputDir,
			fmt.Sprintf("screen-%d.png", time.Now().UnixNano()))
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$bounds = [System.Windows.Forms.SystemInformation]::VirtualScreen
$bmp = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
$gfx = [System.Drawing.Graphics]::FromImage($bmp)
$gfx.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$gfx.Dispose()
$bmp.Dispose()
`, strings.ReplaceAll(path, `'`, `''`))
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("CopyFromScreen failed: %w", err)
	}
	return "image://" + path, nil
}

// captureRegion crops to a CSS-style x/y/w/h box. Coordinates are
// in virtual-desktop space (origin at top-left of the primary monitor).
func (s *screenSkill) captureRegion(p map[string]string) (string, error) {
	x, y, w, h := p["x"], p["y"], p["w"], p["h"]
	if x == "" || y == "" || w == "" || h == "" {
		return "", fmt.Errorf("capture_region needs x= y= w= h=")
	}
	path := p["path"]
	if path == "" {
		path = filepath.Join(s.outputDir,
			fmt.Sprintf("region-%d.png", time.Now().UnixNano()))
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing
$bmp = New-Object System.Drawing.Bitmap %s, %s
$gfx = [System.Drawing.Graphics]::FromImage($bmp)
$gfx.CopyFromScreen(%s, %s, 0, 0, $bmp.Size)
$bmp.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)
$gfx.Dispose()
$bmp.Dispose()
`, w, h, x, y, strings.ReplaceAll(path, `'`, `''`))
	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("region capture failed: %w", err)
	}
	return "image://" + path, nil
}

// listDisplays returns one line per monitor: name, resolution, position.
// WmiObject Win32_VideoController gives device names; Screen.AllScreens
// gives current bounds — combined they're equivalent to macOS's display
// ID list + sckit's per-display rect.
func (s *screenSkill) listDisplays() (string, error) {
	script := `
Add-Type -AssemblyName System.Windows.Forms
$screens = [System.Windows.Forms.Screen]::AllScreens
$out = ""
foreach ($s in $screens) {
  $primary = if ($s.Primary) { " (primary)" } else { "" }
  $out += "$($s.DeviceName)  $($s.Bounds.Width)x$($s.Bounds.Height)  at ($($s.Bounds.X),$($s.Bounds.Y))$primary` + "`n" + `"
}
[Console]::Write($out)
`
	out, err := runPowerShellOut(script)
	if err != nil {
		return "", fmt.Errorf("list displays: %w", err)
	}
	return strings.TrimRight(out, "\r\n "), nil
}

// runPowerShell invokes powershell.exe -NoProfile -NonInteractive with
// the given script via -Command. PowerShell 5.1 ships with every
// supported Windows version so we don't need pwsh.exe (Core 7+).
func runPowerShell(script string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runPowerShellOut is the same but returns stdout for parsing.
func runPowerShellOut(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
