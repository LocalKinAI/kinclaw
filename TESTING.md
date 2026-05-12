# TESTING — runtime validation guide for Linux & Windows

KinClaw's macOS path is daily-driver-tested by the maintainer. The
**Linux** and **Windows** ports were written from API docs on
2026-05-12 (commits [70bbd18 → bfc655f](https://github.com/LocalKinAI/kinclaw/commits/main))
without hardware available to verify runtime behaviour. They build
green for `linux/amd64`, `linux/arm64`, `windows/amd64`, and
`windows/arm64` — the missing piece is **community runtime testing**.

This file is the smoothest on-ramp.

## Get a binary

**Option A — prebuilt release (recommended).** Grab the latest from
[Releases](https://github.com/LocalKinAI/kinclaw/releases). Six
binaries per release (`darwin-{amd64,arm64}`, `linux-{amd64,arm64}`,
`windows-{amd64,arm64}.exe`). 16–18 MB each.

**Option B — CI artifact (newer than release).** [Latest cross-compile
run](https://github.com/LocalKinAI/kinclaw/actions/workflows/cross-compile.yml)
uploads the same six binaries; kept 7 days, signed by GitHub.

**Option C — build it yourself.**
```bash
git clone https://github.com/LocalKinAI/kinclaw
cd kinclaw
GOOS=linux   GOARCH=amd64 go build -o kinclaw         ./cmd/kinclaw
GOOS=windows GOARCH=amd64 go build -o kinclaw.exe     ./cmd/kinclaw
```

## 3-command smoke test (≤10 minutes)

### Linux

```bash
chmod +x kinclaw

# 1. The screen claw (writes a PNG, prints image://path)
./kinclaw -soul souls/pilot_linux.soul.md -exec "take a screenshot of my desktop"

# 2. The ui claw (window-level via xdotool/wmctrl)
./kinclaw -soul souls/pilot_linux.soul.md -exec "what app is focused"

# 3. The cerebellum fast path (no LLM round-trip)
./kinclaw -soul souls/pilot_linux.soul.md -exec "copy hello to clipboard"
```

### Windows (in PowerShell)

```powershell
# 1. screen
.\kinclaw.exe -soul souls\pilot_windows.soul.md -exec "take a screenshot of my desktop"

# 2. ui  (UI Automation 2.0)
.\kinclaw.exe -soul souls\pilot_windows.soul.md -exec "what app is focused"

# 3. cerebellum
.\kinclaw.exe -soul souls\pilot_windows.soul.md -exec "copy hello to clipboard"
```

A **brain endpoint** (e.g. local Ollama, Anthropic API key, OpenAI key)
is needed for the first two — the third is a Layer-0 grep match and
runs with zero LLM tokens.

## What to report

Open or comment on **[#1 (Linux)](https://github.com/LocalKinAI/kinclaw/issues/1)**
or **[#2 (Windows)](https://github.com/LocalKinAI/kinclaw/issues/2)**
with this skeleton — copy-paste, fill in the blanks:

```
**Environment**
- OS / version:       (Ubuntu 24.04 / Windows 11 23H2 / Fedora 40 / …)
- Display server:     (Wayland / X11 — Linux only)
- Desktop env:        (GNOME 46 / KDE Plasma 6 / Sway / N/A)
- Architecture:       (amd64 / arm64)
- kinclaw version:    (output of `kinclaw -version` if available, or commit sha)
- Brain provider:     (Ollama qwen2.5 / Anthropic Claude / OpenAI GPT-4 / …)

**Command**
   $ ./kinclaw -soul … -exec "…"

**Expected**
   <one line>

**Actual**
   <paste output; trim long stack traces>

**Notes**
   <anything else — package versions, weirdness, "this would be 10x more useful if …">
```

## Highest-value tests (if you have hours, not minutes)

### Linux

1. **AT-SPI 2 tree walk** (the meatiest new code path):
   ```
   kinclaw -soul souls/pilot_linux.soul.md -exec "list buttons in Firefox"
   ```
   The `ui tree` action walks `org.a11y.atspi.accessible.Registry` over
   D-Bus. Worth a report whether GNOME-Wayland, GNOME-X11, KDE-X11,
   and Sway each return useful trees.
2. **Wayland vs X11 input parity** — does `input click` work on both?
   `ydotool` (Wayland) needs a daemon; `xdotool` (X11) doesn't.
3. **`record` on Wayland** — does the PipeWire screencast portal
   prompt fire correctly? Does the MP4 play afterwards?
4. **`location` skill** — `gdbus call … org.freedesktop.GeoClue2.Manager`
   needs `geoclue2` running. Some distros disable it by default.

### Windows

1. **UIA on UWP apps** — `ui find role=Button` against a Settings panel
   (UWP) vs Notepad (Win32). UWP buttons sometimes only expose
   `SelectionItemPattern` or `TogglePattern`, not `InvokePattern`.
2. **PowerShell escape edge cases** — try `cerebellum 'windows-clipboard set "string with 'single' quotes"'`
   or text containing `{` / `}` / `~` / `^`.
3. **ffmpeg gdigrab on Win 11 with a DRM video** — does the recording
   show black where the protected window was, or does the whole
   capture fail?
4. **`Windows.Devices.Radios` Bluetooth toggle** — does it need admin?
   Does it work without internet (which the UWP API stack sometimes
   silently requires)?

## Known degradations

These the maintainer already knows are imperfect — useful to confirm,
not a surprise:

- **Linux `app_open_clean` skill** — not enabled in `pilot_linux.soul.md`.
  macOS-specific welcome-modal dismissal pattern; needs per-DE rule pack.
- **Windows `key_down` / `key_up`** — degraded. SendKeys auto-releases.
  Future work: raw `SendInput` with `KEYEVENTF_KEYDOWN/KEYUP`.
- **Both** — agent-side `kinclaw probe` (the AX-tree controllability
  scorer) is darwin-only. AT-SPI 2 / UIA equivalents would be a future
  PR. On non-darwin, `kinclaw probe` exits with a clear message.

## Read more

- [`CHANGELOG.md`](CHANGELOG.md) — what landed when, file-by-file
- [`docs/roadmap.md`](docs/roadmap.md) — Phase 2-6 history + caveats
- [`souls/pilot_linux.soul.md`](souls/pilot_linux.soul.md) — 23-skill Linux pilot
- [`souls/pilot_windows.soul.md`](souls/pilot_windows.soul.md) — 23-skill Windows pilot
