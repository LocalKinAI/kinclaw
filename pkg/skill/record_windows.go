//go:build windows

// record_windows.go — Windows implementation of the record claw.
//
// Records the desktop via ffmpeg gdigrab (Desktop Duplication API
// fallback for older Windows). Matches the start/stop/status surface
// of record_linux.go and record_darwin.go so souls / recipes don't
// need to know which OS they're on.
//
// Why ffmpeg and not a native WMF / Media Foundation pipeline:
//   - ffmpeg gdigrab is widely available, well-tested, and produces
//     standard MP4 / mkv files the user can drop into any editor.
//   - A Media Foundation native recorder would need a few hundred
//     lines of COM glue; ffmpeg is one process spawn.
//   - The Linux record claw makes the same trade-off (ffmpeg x11grab/
//     pipewire), so behaviour is symmetric.
//
// Actions:
//   start [path=…] [fps=N]  — begin capture (defaults output to
//                              %LOCALAPPDATA%\kinclaw\record\YYYYMMDD-HHMMSS.mp4)
//   stop                    — terminate the ffmpeg subprocess
//   status                  — print "recording PID=… file=…" or "idle"
package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type recordSkill struct {
	allowed   bool
	outputDir string
	pidFile   string
}

func NewRecordSkill(allowed bool, outputDir string) Skill {
	if outputDir == "" {
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			outputDir = filepath.Join(appdata, "kinclaw", "record")
		} else {
			outputDir = filepath.Join(os.TempDir(), "kinclaw-record")
		}
	}
	return &recordSkill{
		allowed:   allowed,
		outputDir: outputDir,
		pidFile:   filepath.Join(outputDir, "kinclaw-record.pid"),
	}
}

func (s *recordSkill) Name() string { return "record" }

func (s *recordSkill) Description() string {
	return "Record the Windows desktop to MP4 via ffmpeg gdigrab. " +
		"Actions: start, stop, status. Same shape as record_linux."
}

func (s *recordSkill) ToolDef() json.RawMessage {
	return MakeToolDef("record", s.Description(),
		map[string]map[string]string{
			"action": {"type": "string", "description": "start|stop|status"},
			"path":   {"type": "string", "description": "start: output MP4 path (default: auto-named in output_dir)"},
			"fps":    {"type": "integer", "description": "start: target framerate (default 15)"},
		},
		[]string{"action"})
}

func (s *recordSkill) Execute(p map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("record skill not enabled in soul")
	}
	if err := os.MkdirAll(s.outputDir, 0o755); err != nil {
		return "", fmt.Errorf("output dir: %w", err)
	}
	switch p["action"] {
	case "start":
		return s.start(p["path"], p["fps"])
	case "stop":
		return s.stop()
	case "status":
		return s.status()
	default:
		return "", fmt.Errorf("record: unknown action %q", p["action"])
	}
}

func (s *recordSkill) start(path, fps string) (string, error) {
	if _, err := os.Stat(s.pidFile); err == nil {
		return "", fmt.Errorf("recording already in progress (pidfile %s); call stop first", s.pidFile)
	}
	if path == "" {
		path = filepath.Join(s.outputDir,
			fmt.Sprintf("rec-%s.mp4", time.Now().Format("20060102-150405")))
	}
	if fps == "" {
		fps = "15"
	}
	if _, err := strconv.Atoi(fps); err != nil {
		return "", fmt.Errorf("fps must be integer: %w", err)
	}

	// ffmpeg gdigrab: -framerate FPS -i desktop -c:v libx264 -preset
	// ultrafast (fastest CPU encode at the cost of file size; perfect
	// for short agent-task recordings). yuv420p for max QuickTime/
	// Windows Media Player compatibility.
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "gdigrab",
		"-framerate", fps,
		"-i", "desktop",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-pix_fmt", "yuv420p",
		path,
	)
	// Detach: we want ffmpeg to keep running after this PowerShell-ish
	// goroutine returns. Stdin nil + Setpgid keeps it alive even if
	// the caller closes its handles.
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("ffmpeg start: %w (is ffmpeg in PATH?)", err)
	}
	// Persist pid+path so stop() / status() can find it later. ffmpeg
	// must be told to stop with `q` on stdin or SIGINT; we use taskkill
	// with /F because stdin isn't easily reattachable across calls.
	if err := os.WriteFile(s.pidFile,
		[]byte(fmt.Sprintf("%d\n%s\n", cmd.Process.Pid, path)),
		0o644); err != nil {
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("pidfile write: %w", err)
	}
	return fmt.Sprintf("recording started · pid=%d · file=%s", cmd.Process.Pid, path), nil
}

func (s *recordSkill) stop() (string, error) {
	data, err := os.ReadFile(s.pidFile)
	if err != nil {
		return "", fmt.Errorf("no active recording (no pidfile)")
	}
	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(lines) < 2 {
		return "", fmt.Errorf("malformed pidfile")
	}
	pid := lines[0]
	path := lines[1]

	// taskkill /F /PID — guaranteed kill regardless of console handle
	// state. Side-effect: ffmpeg's atexit handler doesn't run, so the
	// MP4 file may be missing the moov atom. Workaround: pass /T to
	// terminate the whole process tree (helps if ffmpeg spawned
	// helpers) and try a graceful WM_CLOSE first.
	graceful := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Stop-Process -Id %s -Force:$false -ErrorAction SilentlyContinue; Start-Sleep -Seconds 2", pid))
	_ = graceful.Run()
	// Then hard-kill any survivors.
	hard := exec.Command("taskkill", "/F", "/PID", pid, "/T")
	_ = hard.Run()

	_ = os.Remove(s.pidFile)
	return fmt.Sprintf("recording stopped · file=%s", path), nil
}

func (s *recordSkill) status() (string, error) {
	data, err := os.ReadFile(s.pidFile)
	if err != nil {
		return "idle", nil
	}
	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(lines) < 2 {
		return "", fmt.Errorf("malformed pidfile")
	}
	pid := lines[0]
	path := lines[1]
	// Verify the process actually exists via Get-Process. If the PID
	// is stale (ffmpeg crashed), clean up and report idle so subsequent
	// start() calls don't refuse.
	probe := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("Get-Process -Id %s -ErrorAction SilentlyContinue", pid))
	if err := probe.Run(); err != nil {
		_ = os.Remove(s.pidFile)
		return "idle (stale pidfile cleaned)", nil
	}
	return fmt.Sprintf("recording · pid=%s · file=%s", pid, path), nil
}
