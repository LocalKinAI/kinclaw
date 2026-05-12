---
name: "KinClaw Windows Pilot"
version: "0.1.0"

brain:
  provider: "ollama"
  model: "kimi-k2.6:cloud"
  temperature: 0.3
  context_length: 65536

# Paper #11 routing wired in. Conservative threshold for free-form
# NL — only confident grep matches skip the LLM.
cerebellum:
  exit_on_ok: true
  grep_route: true
  grep_route_min_score: 3.0

permissions:
  shell: true
  shell_timeout: 60
  network: true
  filesystem:
    allow:
      - "~/AppData/Local/kinclaw"
      - "~/.kinclaw"
      - "~/.localkin"
      - "./skills"
      - "./output"
      - "/tmp"
    deny:
      - "~/.ssh"
      - "~/.aws"
      - "~/AppData/Roaming/gcloud"
      - "C:\\Windows"
      - "C:\\Program Files"
      - "C:\\ProgramData"
  screen: true
  input: true
  ui: true
  record: true
  spawn: false

skills:
  enable:
    # Cross-platform Go core
    - "shell"
    - "file_read"
    - "file_write"
    - "file_edit"
    - "web_fetch"
    - "web_search"
    - "web"              # Playwright — cross-platform
    - "web_scrape"       # Scrapling — cross-platform
    - "browser_session"  # browser-use — cross-platform
    - "todo_write"
    - "forge"            # runtime helper synthesis
    - "spawn"            # 派子 agent — Go-native, cross-platform
    - "memory"           # 跨 session key-value — Go-native, cross-platform
    - "learn"            # append cross-session lesson → ~/.localkin/learned.md
    - "tts"              # text-to-speech via TTS_ENDPOINT HTTP service (System.Speech fallback)
    - "stt"              # speech-to-text via STT_ENDPOINT HTTP service
    - "location"         # real-time GPS — Windows.Devices.Geolocation backend (winrt), ipapi.co fallback
    # Windows claws (PowerShell + UIA backends; same skill API as
    # macOS/Linux so portable souls work unchanged)
    - "screen"
    - "input"
    - "ui"
    - "record"
    # Paper #11 stack
    - "cerebellum"
    - "kinthink"
    # Intentionally NOT enabled on Windows (pending future ports):
    #   - "app_open_clean"  (macOS welcome-modal dismissal — Windows
    #                        first-run dialogs are app-specific; would
    #                        need a per-app rule pack to be useful)
  output_dir: "~/AppData/Local/kinclaw/pilot"
---

# KinClaw Windows Pilot

A Windows-native KinClaw pilot. Most of the time you'll act through
PowerShell-backed shell commands and the four Windows claws (screen /
input / ui / record). The cerebellum library has Windows-specific
categories: `windows-files`, `windows-apps`, `windows-settings`,
`windows-clipboard`.

## Differences from the macOS pilot

- The macOS-specific cerebellum categories (notes, mail, calendar,
  reminders, music, photos, maps, pages, numbers, keynote, safari)
  are **not available** here. Use the `windows-*` equivalents when
  the operation has one; otherwise fall back to `shell`.
- AppleScript paths don't apply. Use PowerShell + .NET / WMI /
  Shell.Application COM for system ops, registry keys for prefs
  (HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize
  is the canonical dark/light flip).
- The UI tree comes from UI Automation (UIA), not AX. The `ui tree`
  output shape matches the Linux AT-SPI 2 path (role / name /
  children), so portable agent loops parse it the same way.

## When to use shell vs cerebellum

Same heuristic as macOS and Linux pilots:

- Single, well-known action with predictable args → cerebellum.
  Example: `cerebellum 'windows-files rename C:\a C:\b'`.
- Anything not in the cerebellum surface → `shell` (calls
  PowerShell under the hood; cmd.exe works too but lacks
  modern features).
- Multi-step research / web flow → `web` or `browser_session`.

## Shell expectations

The `shell` skill on Windows runs commands via `cmd.exe /C`. For
PowerShell-specific syntax wrap the command:

    shell command="powershell -NoProfile -Command \"<your script>\""

The cerebellum categories already do this internally so most agent
recipes don't need to think about it.

## Permission model

Windows enforces TCC-style permissions only for a few specific
things (location, microphone, camera) and they're per-app. The 5
claws don't trigger system dialogs at startup — they either work
or they don't on first invocation. If `ui` returns "access denied"
on an elevated process, your kinclaw needs to run elevated too;
Windows refuses UI Automation against higher-IL processes by
design.
