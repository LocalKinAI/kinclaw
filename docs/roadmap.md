# KinClaw Roadmap

*Last updated: 2026-05-12 — Linux/Windows port officially started.*
*Original capture: 2026-04-24 (late night brainstorm after v1.1.1).*

This roadmap is **intentional but not binding**. Real user feedback
reorders it faster than this file.

## What changed: 2026-04-24 → 2026-05-12 (17 days)

- ✅ v1.2-1.6 functionality **shipped or subsumed** into kinthink + cerebellum.
- ✅ v2.0 **Cerebellum daemon** shipped 60 days early as `kinthink`
  router + `skills/cerebellum/` 478-action library + `web.sh` wrapper —
  see [paper #11 (Grep-Routed Agents)](https://doi.org/10.5281/zenodo.20131046).
- ✅ [macbench v0.2](https://github.com/LocalKinAI/macbench) released
  (paper #10): 379 tasks, kinclaw + Kimi-K2.6 = 48.0% pass,
  reference-verifier ceiling 84.3%.
- ✅ 11 papers on Zenodo, all CC-BY-4.0 — 4-paper thesis chain
  (retrieval / cognition / execution / routing all don't need
  intelligence for bounded domains).
- 🔄 **Linux / Windows port — was non-goal, now Phase 2 work.** Audit
  on 2026-05-12 confirms kinclaw already cross-compiles to Linux
  AMD64 + ARM64 (one `smart_click_other.go` stub away). Windows still
  blocked on upstream `kinax-go` Windows path. See
  ["Cross-platform port"](#cross-platform-port-202605-).

Links:
- Identity: [README.md](../README.md)
- Vision: [../KINCLAW_VISION.md](../../KINCLAW_VISION.md) (lobster metaphor, fission primitives)
- Architectural pattern origin: pi-car B013 milestone (Paper #1 on [localkin.dev/papers](https://www.localkin.dev/papers))

---

## 🦞 Core thesis (recorded so it doesn't drift)

KinClaw is **"your personal agent cloud, running entirely on your own
Mac, accessible from everywhere."** This is deliberate anti-positioning
against Claude Computer Use / OpenAI Operator / trycua:

| They | KinClaw |
|------|---------|
| Cloud-only | **Your hardware** |
| Vendor brain lock-in | **Any brain (incl. local)** |
| Per-message pricing | **$0 marginal cost w/ local model** |
| Virtualized sandbox | **Real session, real login state** |
| SaaS dashboard | **Native menu bar + webview console** |
| Must share data | **100% private by default** |

The monetization follows: software is MIT, **revenue comes from the
relay service** that lets you reach your own KinClaw from anywhere,
not from LLM token markup.

---

## 🧠 The three laws (from pi-car, now applied to KinClaw)

Documented so every future version can be evaluated against them:

1. **Genesis** — Don't ship pre-written app-specific skills. Ship a
   bare kernel + `forge` + hardware/OS probe. Let the agent forge
   its own skills from `probe` + `web_search` on first boot. Every
   user's KinClaw is different after 10 minutes, not 10 months.

2. **Cerebellum** — Not everything needs an LLM round-trip. Put
   deterministic, high-frequency logic (AX cache refresh, user-input
   debouncing, click retry) in a local Go daemon running 20-60Hz.
   LLM says "go to kitchen"; cerebellum handles the wheels.

3. **Compound skills > atomic skills** — `scan_surroundings` ships
   as one tool call, not four (gimbal_pan + camera + yolo + distance).
   Every compound skill shipped cuts LLM round-trips 50-87%.

Every milestone below lists which of the three laws it advances.

---

## Milestones

### v1.2 — Compound skills × 5 (THIS WEEK)
**Law applied:** #3 (Compound).
**Goal:** Cut typical task latency from 5-7 LLM rounds to 1-2.

Ship as `pkg/skill/compound.go` (or separate files). Each is a
hand-written deterministic sequence that wraps existing atomic skills:

| Compound skill | Wraps | Rounds saved |
|----------------|-------|--------------|
| `open_url` | Safari launch + `cmd+L` + type URL + enter | 4-5 → 1 |
| `open_app` | Check running → `open -a Name` → wait + verify focused | 3 → 1 |
| `click_by_title` | `ui find role+title` + `ui click` + error mapping | 2 → 1 |
| `fill_form` | `ui find AXTextField[]` + identifier matching + type loop | N*3 → 1 |
| `copy_between_apps` | Source find → select → ⌘C → app switch → ⌘V | 6-8 → 1 |

**Effort:** Half a day.
**Dependencies:** None. Uses existing skills.
**Distribution moment:** Demo #1 video feels 3x snappier — and that
IS the moment.

### v1.3 — Genesis Protocol + `clone` skill (NEXT WEEK)
**Laws applied:** #1 (Genesis) + paves way for full fission.
**Goal:** First boot produces a per-user skill registry. LLM can
spawn clones as tool calls.

Two additions:

**(a) Macos Genesis boot message** — Extend `pilot.soul.md` with
a `boot.message` that on first run:
1. `shell` probes sw_vers / hw / installed apps / brew formulas
2. Forges 10-30 starter skills based on what's installed:
   - `clipboard_get` / `clipboard_set` (if pbcopy/pbpaste exist)
   - `mdfind_search`
   - `osascript_run`
   - Per-app `<app>_launch_focused` for each in `/Applications`
3. Writes `~/.kinclaw/genesis_result.json`
4. On subsequent starts, loads from JSON instead of re-probing

**(b) `clone` native skill** — Expose `pkg/clone.Clone` to LLMs:
```
clone action=spawn parent=souls/email_reader.soul.md count=10 \
      patch='{"name_prefix":"Email-"}'
  → spawns 10 copies on ports :8020..:8029
```

**Effort:** 2-3 days.
**Dependencies:** None (brain adapter + HTTP API exist).
**Distribution moment:** Demo #2 — "KinClaw spawns 10 lobsters to
read 10 emails in parallel, reconvenes via conductor soul."

### v1.4 — DMG + Developer ID + App Bundle (WEEK 3)
**Goal:** `kinclaw.localkin.ai/download` — user clicks, drags
KinClaw.app to Applications, double-click, it works.

Technical steps:
1. **Apple Developer Program** — $99/yr. **BLOCKS all DMG/App work.**
   Registration + approval takes 24-48h. **Pay this week.**
2. `KinClaw.app` bundle structure (Info.plist, AppIcon.icns, souls/ baked in)
3. Sign with Developer ID certificate
4. Notarize via `xcrun notarytool`
5. Staple ticket to app
6. `create-dmg` with background image + Applications alias
7. Upload to CDN (Cloudflare R2 or similar — we already use R2)
8. First version of DMG: **double-click opens Terminal running REPL**.
   Not beautiful, but installable. GUI is v1.5.

**Effort:** 2-3 days (once Apple Developer cert is in hand).
**Dependencies:** ⚠️ **Apple Developer Program ($99)** ⚠️
**Distribution moment:** First real "non-developer can install" release.

### v1.5 — Wails Console + Menu Bar 🦞 (WEEK 4-5)
**Goal:** "Visible swarm" — users SEE their lobsters.

Architecture:
- [Wails](https://wails.io) wrapper: Go backend (reuses `pkg/`),
  HTML/CSS/JS frontend, WebKit render (no Chromium bundle).
- Menu bar: NSStatusItem with 🦞 icon + count badge.
- Click → dropdown showing all running kinclaw processes (each
  on its own port: :8019, :8020, :8021...).
- Click any one → full-screen webview chat panel for that soul.
- Buttons: `+ Spawn New`, `⏸ Pause All`, `💀 Kill Idle`.
- Process manager watches sub-processes, cleans up zombies.

Reuse from localkin 1.0.0:
- `pkg/server/` (HTTP + SSE streaming chat API).
- `pkg/server/chat.html` (existing Web Chat UI — just embed in webview).
- `/v1/agents` endpoint (auto-discovers running kinclaw processes).

**Effort:** 2-3 weeks.
**Dependencies:** v1.4 (signed app bundle) + Apple Developer.
**Distribution moment:** First product that *feels* like a product,
not a CLI.

### v1.6 — MLX local model + Smart routing (MONTH 2)
**Goal:** "Turn off WiFi, KinClaw still works."
**Strategic importance:** 🏆 **Unique in the market.**

Two additions:

**(a) MLX brain provider** — `pkg/brain/mlx.go`. Uses Apple's
[mlx-lm](https://github.com/ml-explore/mlx-lm) served as local
OpenAI-compatible endpoint. 2-3x faster than Ollama on Apple Silicon
for same model.

Tested default models:
- `Qwen2.5-Coder-7B-Instruct-4bit` — code + tool use, ~7 GB RAM
- `Qwen3-8B-Instruct-4bit` — general reasoning, ~8 GB
- `Mistral-Nemo-12B-Instruct-4bit` — 128K context, ~12 GB
- Benchmark each for tool-pick accuracy vs latency.

**(b) Smart fallback chain** — extend `brain:` YAML:
```yaml
brain:
  primary:    { provider: mlx, model: qwen2.5-coder-7b-4bit }
  fallback:   { provider: ollama, model: kimi-k2.6:cloud }    # long context / vision
  force_cloud_for: ["tool:screen_describe"]                    # VLM tasks
```

**(c) `kinclaw bench` subcommand** — evaluate any brain against a
canonical tool-use suite. Publishes accuracy + latency numbers so
users can pick model for their Mac's RAM.

**(d) Default pilot soul uses local** — `souls/pilot_local.soul.md`
becomes the shipped default. Cloud variants move to
`souls/examples/`.

**Effort:** 1-2 weeks.
**Dependencies:** v1.4 (app bundle — so users who download the DMG
already get the local-ready souls baked in).
**Distribution moment:** Demo #3 video — "**airplane mode**, KinClaw
still operates my Mac." This is the viral one.

### v1.7 — iOS Shortcuts + Siri channel (MONTH 2)
**Goal:** "Hey Siri, ask KinClaw to ..."

KinClaw already exposes HTTP API. iOS Shortcuts is a first-party
Apple app that can POST. Three deliverables:

1. **Shortcuts template file** — downloadable `.shortcut` that users
   import. Has pre-filled URL pointing to their Mac on local net
   or Tailscale.
2. **Setup doc** — how to publish kinclaw HTTP API securely:
   - Local network only (recommended default)
   - Tailscale for remote
   - Cloudflare Tunnel for full public
3. **Share Sheet extension** — "Send to KinClaw" in any iOS share
   menu. Any URL / text / photo goes to Mac for processing.

**Effort:** 3 days (most is docs + a .shortcut file; not Swift).
**Dependencies:** Apple Developer (for Share Sheet extension, Swift).
Can ship without the extension — Shortcuts alone is enough.
**Distribution moment:** Twitter gold. "I sent a PDF from my phone;
5 seconds later my Mac summarized it in my Notes app."

### v2.0 — Cerebellum daemon (MONTH 3)
**Law applied:** #2 (Cerebellum).
**Goal:** Invisible speedup. LLM calls that used to take 300-800ms
(`ui find` walking the AX tree) take 2-5ms.

`pkg/cerebellum/daemon.go`:
- Long-lived goroutine spawned at REPL startup.
- 20-60Hz AX tree cache refresh for frontmost app.
- User-input debouncer — if human is typing, pause agent actions.
- Click retry with exponential backoff (no LLM round-trip on flap).
- Window/focus change event bus for LLM to subscribe to.
- Optional: every second log `{focused_app, focused_element, mouse_xy}`
  to SQLite for long-term user pattern analysis.

**Effort:** 1 week.
**Dependencies:** v1.5 console (to visualize cerebellum state).
**Distribution moment:** Not very user-visible. Matters for power
users who notice latency.

### v2.1 — Relay service (MONTH 3-4)
**Goal:** Remote access without Tailscale setup. Revenue.

`kinclaw.localkin.ai` becomes a relay. User flow:
1. Install KinClaw app on Mac.
2. Sign in with account (email/OAuth).
3. Mac's KinClaw opens persistent WebSocket to relay.
4. User on any device → `kinclaw.localkin.ai/console` → 2FA → sees
   their Mac's lobsters → chat tunnels through relay.
5. **Compute stays on Mac.** Relay only carries encrypted traffic.

Pricing:
- Free tier: 1 Mac, 100 MB/month relay traffic
- Pro ($5/mo): 3 Macs, unlimited relay
- Team ($15/mo): shared fleet dashboard

**Effort:** 3-4 weeks (backend infra).
**Dependencies:** v1.4 (signed app can auth to relay) + v1.5 (console).
**Distribution moment:** First time KinClaw has a monetization
surface that isn't selling LLM tokens.

### v2.2 — Telegram / WeChat / Discord bot channels (MONTH 4)
**Goal:** "Send your lobster a message from anywhere."

One bot per channel. Each is ~200-400 lines of Go:
- Subscribe to channel webhook
- Forward message text to user's kinclaw via their authenticated
  relay tunnel
- Stream response back

**Effort:** 1-2 days per channel.
**Dependencies:** v2.1 (relay) for out-of-LAN delivery.
**Distribution moment:** Chinese market entry via WeChat bot.

---

## Strategic checkpoints

### After v1.2 (week 1)
- [ ] Demo video #1 recorded + posted (r/MacOS + r/LocalLLaMA + HN Show)
- [ ] `./scripts/daily-traffic.sh` run every morning, track baseline
- [ ] First external user installs kinclaw + files ≥1 issue

**Reassess:** If zero external users by end of week 1, stop building.
Re-examine distribution before more features.

### After v1.4 (week 3)
- [ ] DMG downloadable from kinclaw.localkin.ai
- [ ] Notarized, double-click works without scary dialog
- [ ] At least 10 downloads by non-Jacky people

**Reassess:** If downloads under 50, before v1.5 invest a week in
messaging / landing page / videos. The product is ready; the
funnel isn't.

### After v1.6 (month 2)
- [ ] Local-model demo published, airplane-mode screencap on record
- [ ] At least one comparison benchmark vs Claude Computer Use
- [ ] KinClaw is mentioned in one "awesome-agent" / "local LLM tools"
      list without us asking

**Reassess:** If no organic listing pickup, the product-message fit
isn't there yet. Sit with users, don't build v1.7.

---

## Dependency graph — the $99 ceiling

**Apple Developer Program ($99/year) blocks:**

```
v1.4 DMG + notarization      ← blocked
v1.5 Wails .app console      ← blocked (depends v1.4)
v1.6 Local-model default     ← partially blocked (can ship as CLI,
                                  but DMG is the real moment)
v1.7 iOS Shortcuts share ext ← blocked (Swift extension needs cert)
v2.1 Relay auth              ← blocked (relay auth pairs w/ app id)
```

**5 out of 9 milestones block on $99.** Pay it this week. 48h later
the block clears.

---

## Explicit non-goals (things we will NOT do)

Writing these down so future-me doesn't drift:

- ⚠️ **Windows / Linux support — reversed 2026-05-12.** Was a non-goal
  until paper #11's "具身智能 + 万物智能" thesis made portability the
  natural conclusion of cerebellum + grep router (both are POSIX-shell-
  native). Linux port now Phase 2 work; Windows blocked on `kinax-go`
  Windows path. See "Cross-platform port" section.
- ❌ **Fine-tuned KinClaw-specific model.** The brain is swappable.
  Don't marry a model.
- ❌ **Open-source the full Genesis Protocol** (the version from
  localkin-core that generates expert specialists from YouTube /
  docs). Keep it in commercial engine.
- ❌ **Enterprise SSO / audit / policy UI** until an enterprise
  user asks.
- ❌ **Collaboration / shared swarms** — one user, one Mac, for now.
  Multi-Mac comes with v2.1 relay, not before.
- ❌ **Monetize on LLM tokens** — relay service, yes; token markup, no.
- ❌ **"Benchmarks leaderboard"** — don't get trapped in OSWorld
  score optimization. Ship features that users feel, not numbers.
- ❌ **Rewriting skills in Rust / Swift / whatever**. Go everywhere
  unless a specific library forces otherwise.

---

## Inbox (unprioritized ideas — revisit quarterly)

- Voice mode — push-to-talk, KinClaw listens + executes
- Apple Watch complication showing active lobster count
- OBS plugin — KinClaw can drive streaming software (demo-record itself)
- Live-reload of souls on filesystem change (already in localkin 1.0,
  port to kinclaw)
- "Undo layer" — every OS-touching action recorded in a reversible
  journal; `kinclaw undo` rolls back N actions
- SKILL.md community library — shared repo where users publish their
  forged skills; KinClaw can `kinclaw skill install <name>`
- Integration with Raycast / Alfred / Keyboard Maestro as channels
- Email-in gateway (forward email → kinclaw processes → stores)
- Prompt injection detection — scan tool results for "ignore previous
  instructions" patterns before feeding back to LLM
- Live pair mode — two KinClaws on two Macs coordinate via relay

---

## One-line log

- **2026-04-23** — KinKit 四件套全部 public (sckit / kinrec / input / kinax).
  localkin-core privately reverted back to clean. kinclaw public.
- **2026-04-24 早** — v1.1.0 ships (localkin → kinclaw rename + claws
  + Soul Clone lib).
- **2026-04-24 下午** — First live demo: Kimi 2.6 drives Mac, reads
  AX tree, handles multi-step tool chains (海鲜超市 dialog).
- **2026-04-24 夜** — v1.1.1 ships (Go SemVer fix to v1.x line;
  brain.go content-field fix for Ollama Cloud).
- **2026-04-24 深夜** — This roadmap captured. Jacky articulated
  three architectural priorities (compound skills, local model,
  channel expansion) and the DMG + console product shape. All
  recorded. Work starts fresh tomorrow with v1.2.
- **2026-04-24 → 2026-05-08** — Compound skills + soul-clone harvested
  into `cerebellum` library (478 actions, 15 categories) instead of
  shipped as v1.2's "5 compound skills". Architecture grew bigger
  than roadmap predicted. macbench v0.1 + paper #10 ship 2026-05-08.
- **2026-05-09 → 2026-05-11** — `kinthink` grep router prototyped
  + integrated into kinclaw kernel + soul flags
  (`cerebellum.{exit_on_ok,grep_route}`) + paper #11 drafted (DOI
  10.5281/zenodo.20131046). macbench v0.2 with 10 web tasks +
  cleanup tooling. Daily-driver pilot soul opts into grep_route.
- **2026-05-12 凌晨** — GitHub issue #1 (tinkerbaj) asks for Linux
  support. Audit confirms cross-compile is one stub away. Roadmap's
  "❌ Windows/Linux" non-goal reversed. Phase 1-4 cross-platform
  work begins this session.

---

## Cross-platform port (2026-05-)

Reversal of the 2026-04-24 non-goal. Driven by:

1. **Paper #11 thesis** (DOI 10.5281/zenodo.20131046): the cerebellum
   pattern works on any POSIX surface. Constraining to macOS
   contradicts the architecture.
2. **具身智能 + 万物智能 long-term goal** (Jacky, 2026-05-11): the
   path to embedded AI / IoT runs through Linux on Raspberry Pi
   first, not through more macOS depth.
3. **First external issue** ([#1](https://github.com/LocalKinAI/kinclaw/issues/1)):
   real demand exists; ignoring it leaves the field to LLM-tax-heavy
   competitors.

### Phase 1 — Cross-compile audit + scaffolding (done 2026-05-12)

- [x] Audit `//go:build darwin` files vs `_other.go` stubs across `pkg/skill/`.
- [x] Add missing `smart_click_other.go` stub.
- [x] Confirm `GOOS=linux GOARCH=amd64 go build` produces 17.5 MB ELF.
- [x] Confirm `GOOS=linux GOARCH=arm64` produces 16.6 MB ARM aarch64
      (Raspberry Pi 4/5 ready).
- [x] Document `GOOS=windows` blocker: `kinax-go` uses `purego.RTLD_LAZY`
      which is POSIX-only. Windows needs an upstream `kinax-go` PR
      first (LoadLibrary/UIAutomation path).

### Phase 2 — First Linux claw: `screen` (in progress)

Implementation strategy per claw:
- Detect display server at runtime: Wayland (`$WAYLAND_DISPLAY`) vs X11 (`$DISPLAY`).
- Use the standard CLI tool for that server.
- Fall back gracefully when the tool isn't installed.

For `screen`:
- Wayland → `grim` (preferred) or `wlr-randr` for multi-monitor
- X11 → `scrot` or `import` (ImageMagick)
- Headless → `ffmpeg` against `/dev/video0` (for robotics)

### Phase 3 — Linux `input` / `ui` / `record`

- `input` — Wayland: `ydotool` (uinput-based, needs daemon).
  X11: `xdotool`.
- `ui` — AT-SPI 2 via `godbus`. Reads accessibility tree from the
  D-Bus accessibility bus. Works on GNOME, KDE, Sway+plugin.
- `record` — `ffmpeg` driven `x11grab` (X11) or PipeWire screencast
  portal (Wayland, via xdg-desktop-portal).

### Phase 4 — Linux cerebellum (per-DE skill library) ✅ partial

Plus: `skills/location/SKILL.md` rewritten cross-platform on 2026-05-12
(geoclue2 via gdbus + Nominatim reverse-geocode + ipapi.co fallback).
Linux pilot soul now reaches feature parity with macOS pilot minus
`app_open_clean` (which is genuinely macOS-specific — welcome-modal
dismissal isn't a Linux convention).

### Phase 5 — Linux ui claw → AT-SPI 2 ✅ shipped 2026-05-12

`pkg/skill/ui_linux.go` upgraded from xdotool-only MVP to **AT-SPI 2
accessibility tree walking via godbus**. New actions:
  - `tree [depth=4] [app=...]` — full a11y tree dump
  - `find name=X role=Y` — search by name/role substring
  - `click_by_name` / `click_by_role` — invoke first action on a
    matching actionable accessible (uses `org.a11y.atspi.Action.DoAction`)

Window-level actions (focused_app / window_list / window_geometry)
remain available via xdotool/wmctrl for X11. Tested compile only;
needs runtime validation on GNOME (Wayland + X11). Added
`github.com/godbus/dbus/v5` direct dep.



The macOS `cerebellum/categories/*.sh` library has 15 categories
heavily tied to AppleScript. Linux equivalents will be **leaner**
because Linux app surfaces are less standardized:

- `linux-files.sh` — file ops via POSIX (mostly portable from `finder.sh`)
- `linux-apps.sh` — `gtk-launch`, `xdg-open`, `wmctrl` for window mgmt
- `linux-settings.sh` — `gsettings` (GNOME), `kwriteconfig5` (KDE),
  `nmcli` (network), `pactl` (audio)
- `linux-clipboard.sh` — `wl-copy` / `wl-paste` (Wayland), `xclip` (X11)
- `linux-terminal.sh` — already works (terminal is universal)
- `linux-web.sh` — already works (Playwright / curl / Scrapling cross-platform)

Skipped on Linux for now (less standard, defer to Phase 5+):
- Mail (Thunderbird via D-Bus exists but spotty)
- Calendar (Evolution-data-server via D-Bus; not all distros)
- Notes (no standard; would need custom journal)
- iWork-equivalents (LibreOffice via UNO bridge — separate effort)

### Phase 5+ — Linux-native macbench-style benchmark

Once Phases 2-4 land, a `linux-bench` repo: 200-300 tasks covering
file ops, apps, settings, clipboard, terminal, web. Different surface
than macbench but same evaluation methodology (setup.sh + eval.sh +
dual scoring).

### Windows (Phase 6+, blocked on upstream `kinax-go`)

1. Upstream PR to `kinax-go` adding Windows path
   (`LoadLibrary` / `GetProcAddress` for COM-based UIAutomation).
2. Once kinclaw compiles on Windows, port `screen` (GDI BitBlt or
   DXGI Desktop Duplication), `input` (`SendInput`), `record`
   (`ffmpeg gdigrab`).
3. `cerebellum/categories/windows-*.sh` — PowerShell-driven.
4. Likely 2-3× slower than Linux port because the upstream lib gap
   is real.
