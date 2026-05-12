//go:build linux

// record_linux.go — Linux implementation of the record claw.
//
// Strategy: shell out to ffmpeg, which handles both X11 (x11grab)
// and Wayland (pipewire screencast via xdg-desktop-portal).
//
// Coverage vs macOS record.go:
//   start [out_path]         ✅  ffmpeg backgrounded, writes mp4
//   stop                     ✅  SIGTERM the pid file
//   status                   ✅  reads pid file
//
// Wayland recording requires `xdg-desktop-portal` for screencast permission;
// the agent must accept a permission dialog the first time. ffmpeg uses
// `pipewire:0` device on Wayland; `:0.0` X display on X11.
//
// TODO(linux-verify): ffmpeg invocations vary by distro version.
// Tested syntax is the Debian 12 / Ubuntu 24.04 baseline.

package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type recordSkill struct {
	allowed   bool
	outputDir string
}

func NewRecordSkill(allowed bool, outputDir string) Skill {
	if outputDir == "" {
		base, _ := os.UserCacheDir()
		if base == "" {
			base = os.TempDir()
		}
		outputDir = filepath.Join(base, "kinclaw", "recordings")
	}
	outputDir = expandHome(outputDir)
	_ = os.MkdirAll(outputDir, 0o755)
	return &recordSkill{allowed: allowed, outputDir: outputDir}
}

func (s *recordSkill) Name() string { return "record" }

func (s *recordSkill) Description() string {
	return "Linux screen recording via ffmpeg (x11grab on X11; pipewire/xdg-desktop-portal on Wayland). " +
		"Actions: start [out_path=...] | stop | status. " +
		"Wayland prompts for screencast permission the first time."
}

func (s *recordSkill) ToolDef() json.RawMessage {
	return MakeToolDef("record", s.Description(),
		map[string]map[string]string{
			"action":   {"type": "string", "description": "start | stop | status"},
			"out_path": {"type": "string", "description": "Output .mp4 path. Default: timestamped file in cache dir."},
			"fps":      {"type": "integer", "description": "Frames/sec. Default 30."},
		}, nil)
}

func (s *recordSkill) Execute(params map[string]string) (string, error) {
	if !s.allowed {
		return "", fmt.Errorf("permission denied: soul does not grant `record` capability")
	}

	action := params["action"]
	if action == "" {
		action = "start"
	}

	switch action {
	case "start":
		return s.start(params)
	case "stop":
		return s.stop()
	case "status":
		return s.status()
	default:
		return "", fmt.Errorf("unknown record action %q", action)
	}
}

func (s *recordSkill) pidFile() string {
	return filepath.Join(s.outputDir, "ffmpeg.pid")
}
func (s *recordSkill) currentOutFile() string {
	return filepath.Join(s.outputDir, "ffmpeg.out")
}

func (s *recordSkill) start(params map[string]string) (string, error) {
	// Already running?
	if pid, ok := readPid(s.pidFile()); ok && pidAlive(pid) {
		return fmt.Sprintf("ok: recording already in progress (pid=%d)", pid), nil
	}

	if !commandExists("ffmpeg") {
		return "", fmt.Errorf("ffmpeg not installed (apt install ffmpeg)")
	}

	out := params["out_path"]
	if out == "" {
		ts := time.Now().Format("20060102-150405")
		out = filepath.Join(s.outputDir, fmt.Sprintf("record-%s.mp4", ts))
	}
	fps := atoiOr(params["fps"], 30)

	args := buildFFmpegArgs(out, fps)
	cmd := exec.Command("ffmpeg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("ffmpeg start: %w", err)
	}

	_ = os.WriteFile(s.pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	_ = os.WriteFile(s.currentOutFile(), []byte(out), 0o644)

	// Don't wait — let it run.
	go func() { _ = cmd.Wait() }()

	return fmt.Sprintf("ok: recording started pid=%d → %s", cmd.Process.Pid, out), nil
}

func (s *recordSkill) stop() (string, error) {
	pid, ok := readPid(s.pidFile())
	if !ok {
		return "", fmt.Errorf("no active recording (no pid file at %s)", s.pidFile())
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return "", err
	}
	// SIGINT lets ffmpeg flush + close container cleanly; SIGKILL would corrupt mp4.
	if err := proc.Signal(syscall.SIGINT); err != nil {
		return "", err
	}
	// Wait briefly for it to flush.
	for i := 0; i < 30; i++ {
		if !pidAlive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = os.Remove(s.pidFile())
	outPath, _ := os.ReadFile(s.currentOutFile())
	_ = os.Remove(s.currentOutFile())
	return fmt.Sprintf("ok: recording stopped → %s", strings.TrimSpace(string(outPath))), nil
}

func (s *recordSkill) status() (string, error) {
	pid, ok := readPid(s.pidFile())
	if !ok {
		return "no active recording", nil
	}
	if !pidAlive(pid) {
		_ = os.Remove(s.pidFile())
		return "stale pid file — cleaned", nil
	}
	out, _ := os.ReadFile(s.currentOutFile())
	return fmt.Sprintf("recording active pid=%d → %s", pid, strings.TrimSpace(string(out))), nil
}

// --- helpers ------------------------------------------------------

func buildFFmpegArgs(out string, fps int) []string {
	// Common output args
	common := []string{
		"-y",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		out,
	}
	switch detectServer() {
	case "wayland":
		// PipeWire via xdg-desktop-portal — ffmpeg 5.0+ supports `pipewire`
		// as input format. Stream id may be 0.
		return append([]string{
			"-f", "pipewire",
			"-framerate", strconv.Itoa(fps),
			"-i", "0",
		}, common...)
	default:
		// X11 grab
		display := os.Getenv("DISPLAY")
		if display == "" {
			display = ":0.0"
		}
		return append([]string{
			"-f", "x11grab",
			"-framerate", strconv.Itoa(fps),
			"-i", display,
		}, common...)
	}
}

func readPid(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func pidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Linux, signal 0 = check existence
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// context.Context-free version still uses internal timeouts.
var _ = context.Background
