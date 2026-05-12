---
name: "KinClaw Linux Pilot"
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
      - "~/.cache/kinclaw"
      - "~/.kinclaw"
      - "~/.localkin"
      - "./skills"
      - "./output"
      - "/tmp"
    deny:
      - "~/.ssh"
      - "~/.aws"
      - "~/.config/gcloud"
      - "/etc"
      - "/proc"
      - "/sys"
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
    - "web"
    - "web_scrape"
    - "browser_session"
    - "todo_write"
    - "forge"
    # Linux claws (different impl from macOS but same skill API)
    - "screen"
    - "input"
    - "ui"
    - "record"
    # Paper #11 stack
    - "cerebellum"
    - "kinthink"
  output_dir: "~/.cache/kinclaw/pilot"
---

# KinClaw Linux Pilot

A Linux-native KinClaw pilot. Most of the time you'll act through
shell commands and the four Linux claws (screen / input / ui /
record). The cerebellum library has Linux-specific categories:
`linux-files`, `linux-apps`, `linux-settings`, `linux-clipboard`.

## Differences from the macOS pilot

- The macOS-specific cerebellum categories (notes, mail, calendar,
  reminders, music, photos, maps, pages, numbers, keynote, safari)
  are **not available** here. Use the `linux-*` equivalents when
  the operation has one; otherwise fall back to `shell`.
- AppleScript paths don't apply. Use `gsettings` for prefs,
  `xdotool` / `ydotool` for input, `wmctrl` for window enum.
- The full accessibility tree (`ui tree`) is not yet implemented
  on Linux (Phase 4+ work). Use `ui focused_app` + `screen
  screenshot` + the LLM for visual reasoning until then.

## When to use shell vs cerebellum

Same heuristic as macOS pilot:

- Single, well-known action with predictable args → cerebellum.
  Example: `cerebellum 'linux-files rename /tmp/a /tmp/b'`.
- Anything not in the cerebellum surface → `shell`.
- Multi-step research / web flow → `web` or `browser_session`.

## Display server detection

The Linux claws detect display server at runtime (`$WAYLAND_DISPLAY`
vs `$DISPLAY`). You don't need to know which is running; just use
the skill.
