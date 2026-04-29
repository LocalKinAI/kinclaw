# KinClaw

> **The self-fissioning lobster. Breeds its own swarm on demand.**
> 可以裂变的龙虾 — 根据需求自己造龙虾群。

KinClaw is a computer-use agent for macOS. It sees your screen,
understands your UI semantically, clicks, types, and — the part no
one else has — **reproduces on demand** via three primitives:

- **Soul Clone** (`pkg/clone`) — duplicate a specialist into N
  parallel workers with small per-clone divergence.
- **Skill Forge** (`pkg/skill` forge) — when an existing skill
  can't handle a task, KinClaw drafts, writes, registers, and tests
  a new one, then retries.
- **Sub-agent dispatch** (`spawn` skill) — fork-exec a specialist
  child on a different brain (researcher / eye / critic / coder ship
  in-box; hierarchical, kernel-capped at depth 1).

Single binary, ~17 MB, Go 1.22+, MIT licensed. Runs on your actual
Mac (not in a virtualized container like Anthropic's Computer Use or
OpenAI's Operator).

> *Same starter lobster for everyone. Every user's swarm is unique
> after a month.*

KinClaw grew out of the earlier `localkin` runtime (a minimal
embodied-AI microkernel, ~2,300 lines). This repo is that same
skeleton with the **five claws** bolted on: `screen` (ScreenCaptureKit),
`input` (CGEvent), `ui` (Accessibility API), `record` (kinrec MP4 +
audio), and `web` (Playwright) — the first four via their own zero-cgo
KinKit libraries.

## Quick start

```bash
go install github.com/LocalKinAI/kinclaw/cmd/kinclaw@latest

# Default pilot runs Kimi K2.5 via Ollama Cloud. Sign in once:
ollama signin

# The pilot soul — the generalist that drives your Mac
kinclaw -soul souls/pilot.soul.md

# Then ask it something like:
# > "What app is in front? Click the Save button if there is one."
```

Want a specialist instead of the generalist pilot? KinClaw ships four
focused souls; pilot dispatches to them via `spawn`, but you can also
launch them directly:

```bash
kinclaw -soul souls/researcher.soul.md    # Kimi K2.6 (1T, 256k ctx) — deep web research
kinclaw -soul souls/eye.soul.md           # Kimi K2.6 multimodal — visual verification
kinclaw -soul souls/critic.soul.md        # Minimax M2.7 — adversarial review
kinclaw -soul souls/coder.soul.md         # DeepSeek V4 Pro — harvest --inspire forge specialist
```

All four use Ollama Cloud routing (`ollama signin` once). Different labs
on purpose: pilot+researcher+eye on Moonshot, critic on Minimax, coder
on DeepSeek — different model lineage means different blind spots.

First run triggers two macOS TCC prompts:
- **Screen Recording** (for `screen` + `record` skills via sckit-go / kinrec)
- **Accessibility** (for `input` + `ui` skills via input-go + kinax-go)

Grant both; rerun. `record mic=true` adds a Microphone prompt; `location` skill adds a Location Services prompt; first browser launch downloads Chromium (~500MB).

### Optional sidecars (peripheral capabilities)

KinClaw stays a small Go binary; capabilities that need heavy deps
ship as opt-in sidecars selected via env var:

| Capability | Sidecar | Env var | Setup |
|---|---|---|---|
| Web research | SearXNG | `SEARXNG_ENDPOINT` | self-host (default: `http://localhost:8080`) |
| Voice synthesis | Kokoro (via [localkin-service-audio](https://github.com/LocalKinAI/localkin-service-audio)) | `TTS_ENDPOINT` | run server on `:8001` |
| Voice recognition | SenseVoice (via [localkin-service-audio](https://github.com/LocalKinAI/localkin-service-audio)) | `STT_ENDPOINT` | run server on `:8000` |
| Web automation | Playwright (Python) | none — `web` skill uses `python3 ./web.py` directly | `pip install playwright && playwright install chromium` |
| Real-time GPS | corelocationcli | none — `location` skill calls binary | `brew install corelocationcli` |

### Per-user context (auto-injected to every soul prompt)

| Variable | Where it comes from |
|---|---|
| `{{current_date}}` | `time.Now()` at boot |
| `{{tz}}` | local timezone (e.g. `PDT (UTC-7)`) |
| `{{platform}}` | `runtime.GOOS` mapped to `macOS`/`Linux`/`Windows` |
| `{{arch}}` | `runtime.GOARCH` (`arm64` / `amd64`) |
| `{{location}}` `{{lat}}` `{{lon}}` `{{city}}` `{{country}}` | `$KINCLAW_LOCATION="lat,lon[,city[,country]]"` env var |
| `## 已学到的` section | `~/.localkin/learned.md` (8KB tail) |

After a few weeks of use, the agent boots with rich context: knows
its OS, knows the user's general location + timezone, and remembers
what worked + what didn't on every app it has driven.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  Soul (.soul.md)                            │
│  YAML frontmatter + Markdown system prompt                  │
│  template subs: {{platform}} {{tz}} {{location}} ...        │
│  + auto-loaded ~/.localkin/learned.md (cross-session)       │
├─────────────────────────────────────────────────────────────┤
│                       Brain (LLM)                           │
│  Claude · OpenAI · Ollama · Kimi · GLM · Qwen · any         │
│  multimodal images attached when brain supports vision      │
├─────────────────────────────────────────────────────────────┤
│                       Skills (Tools)                        │
│                                                             │
│  ─── The five claws ───                                     │
│  screen ─ eye          ─► sckit-go  (ScreenCaptureKit)      │
│  input  ─ hand         ─► input-go  (CGEvent)               │
│  ui     ─ visual cortex─► kinax-go  (AX, semantic UI)       │
│  record ─ memory       ─► kinrec    (video MP4 + audio)     │
│  web    ─ open net     ─► Playwright (DOM render + scrape)  │
│                                                             │
│  ─── Classic kernel ───                                     │
│  shell · file_read/write/edit · web_fetch · web_search      │
│                                                             │
│  ─── Self-evolution ───                                     │
│  forge   — author new skills (with kernel quality gate)     │
│  learn   — append cross-session lessons to learned.md       │
│  clone   — duplicate souls into N parallel workers (lib)    │
│  spawn   — dispatch subtask to specialist child (depth-1)   │
│  harvest — pull skills from other agent repos (CLI subcmd)  │
│                                                             │
│  ─── External SKILL.md plugins (./skills/) ───              │
│  tts / stt — Kokoro / SenseVoice via :8001 / :8000          │
│  location  — corelocationcli (real-time GPS)                │
│  app_open_clean — open + dismiss welcome modal              │
│  any forge'd or hand-written SKILL.md is auto-loaded        │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  Kernel guards (4-trigger circuit breaker)                  │
│  · same-error consecutive  · cumulative failures           │
│  · same-output no-progress · per-turn usage cap            │
│  + ui click ambiguity refusal · destructive-target refusal  │
├─────────────────────────────────────────────────────────────┤
│       SQLite memory per session + learned.md across them    │
└─────────────────────────────────────────────────────────────┘
```

### KinKit — the open-source claws

All four sibling libraries are MIT, zero cgo, `go install`-able:

| Library | Role | Dylib |
|---------|------|-------|
| [sckit-go](https://github.com/LocalKinAI/sckit-go) | ScreenCaptureKit — screenshots + live streams | ~130 KB |
| [kinrec](https://github.com/LocalKinAI/kinrec) | Screen + audio recorder (MP4, h264/hevc) | ~130 KB |
| [input-go](https://github.com/LocalKinAI/input-go) | CGEvent mouse + keyboard synthesis | ~85 KB |
| [kinax-go](https://github.com/LocalKinAI/kinax-go) | Accessibility API UI tree access | ~88 KB |

Each uses the **embedded dylib pattern** (`purego` + `//go:embed`),
so downstream users never need `clang` or `CGO_ENABLED`. See
[Paper #9 on localkin.dev](https://www.localkin.dev/papers/embedded-dylib)
for the full architectural story.

## Soul schema

A soul file is YAML frontmatter + a Markdown system prompt.

```yaml
---
name: "KinClaw Pilot"
brain:
  provider: "ollama"
  model: "kimi-k2.5:cloud"
permissions:
  shell: false
  network: false
  screen: true       # sckit-go capability
  input: true        # input-go capability
  ui: true           # kinax-go capability
  record: true       # kinrec capability — video MP4 + audio
  spawn: false       # opt in to sub-agent dispatch (default off)
skills:
  enable: ["screen", "input", "ui", "record", "file_read", "tts", "stt"]
---

# You are KinClaw Pilot...
```

The `screen / input / ui / record` bits are the KinClaw additions. Each
corresponds to one or two TCC prompts and one KinKit library. If a bit
is false, the matching skill returns `permission denied: soul does not
grant X capability` regardless of what the LLM asks for. `record`
shares Screen Recording TCC with `screen`; mic capture additionally
requires Microphone TCC.

## The five claws in action

### `ui` — semantic UI control (the killer feature)

Click a button by its **title**, not pixel coordinates. This is what
makes KinClaw different from Computer Use / Operator, which look at
screenshots and guess.

```
user: Click "Save"
LLM:  ui action=click role=AXButton title=Save
      → clicked AXButton "Save" (matched role=AXButton title="Save")
```

Other `ui` actions: `focused_app`, `tree` (dump the AX tree),
`find` (list matching elements), `read` (read element value),
`at_point` (hit-test a coordinate), `watch` (subscribe to AX
events — see below).

**`ui action=watch`** (v1.7+) blocks for `duration_ms` collecting
push-based AX notifications via kinax-go's `Observer`. Cheaper than
polling `ui tree` for "did anything change":

```
ui action=watch events=AXFocusedWindowChanged duration_ms=5000
ui action=watch events=AXValueChanged,AXMenuOpened duration_ms=3000 pid=12345
```

Returns the events that fired during the window. Use it when you
need to wait for a specific UI event (window focus shifted, dialog
appeared, value updated post-click) instead of guessing when to
re-tree.

### `input` — raw mouse + keyboard

When there's no AX element (canvas apps, games, some WebGL), fall
back to coordinates:

```
LLM: input action=click x=842 y=523
LLM: input action=type text="hello 世界 👋"
LLM: input action=hotkey mods="cmd+shift" key="t"
```

**Background mode** (v1.4+): pass `target_pid=<N>` and the event
routes directly to that process via `CGEventPostToPid` — the
targeted app receives the input but its window does **not** come
to front. The user's foreground app keeps focus, so the agent can
drive a background app (Music, Reminders, Slack, ...) while the
user keeps working in their editor. Verified on Lark / VSCode /
Chrome / Cursor and other Electron + WebKit hosts; some Apple
sandboxed apps (newer Mail / Messages) may ignore PID-targeted
events — fall back to omitting the param.

```
LLM: input action=click x=400 y=300 target_pid=12345
LLM: input action=type text="hello" target_pid=12345
```

### Reading the screen — three-tier cascade (v1.7+)

```
Layer 1 (cheapest)   ui claw            ~50ms     $0       deterministic
Layer 2              screen action=ocr  ~50-200ms $0       probabilistic
Layer 3 (priciest)   screen + vision    ~3s       ~$0.005  generic
```

- **Layer 1 (AX, default)** — `ui find` / `ui tree` / `ui read`. Works
  on 94% of macOS apps because they expose Accessibility. Use this
  ALWAYS unless AX literally returns nothing.
- **Layer 2 (OCR)** — `screen action=ocr`. Drops to text-extraction
  via Vision framework when AX is absent (canvas apps, status bars,
  rendered images). ~99% accurate on digits / version strings; word
  accuracy varies (see `screen action=ocr` notes below).
- **Layer 3 (vision LLM)** — `screen action=screenshot` + `file_read`
  + multimodal brain. The expensive fallback for "understand this
  screen" tasks where Layer 1 + 2 give text + boxes but no semantics.

The pilot soul has the doctrine baked in: never skip Layer 1 just
because Layer 3 is more flexible.

### `screen` — just take a picture (and read text out of it, v1.7+)

```
LLM: screen action=screenshot
     → ~/Library/Caches/kinclaw/screens/screen-20260424-001312.000.png

LLM: screen action=ocr
     → OCR on /Users/.../screen-20260429-143012.png — 7 text region(s):
       "Save"           at (412,85)  size 48x14   conf=1.00
       "Cancel"         at (480,85)  size 56x14   conf=1.00
       "今天天气怎么样"   at (200,300) size 280x40 conf=0.99
       ...

LLM: screen action=ocr path=/tmp/saved.png   # OCR an existing image
```

The LLM can then read the PNG back (if `file_read` is enabled) and
reason about it visually, OR use `action=ocr` to extract text in
~50-200ms with no vision-LLM round-trip — local, offline, free
(via Apple Vision framework / sckit-go v0.2+).

Use `ocr` when you need the **literal text**; use the screenshot
+ vision LLM when you need to **understand** the screen.

### `record` — non-blocking video capture

`start` returns a recording_id immediately; the agent keeps operating
the Mac while kinrec writes MP4 in the background. `stop` finalizes
the file. Audio sources are independent: `audio=true` taps system
output (everything coming out of your speakers), `mic=true` adds the
microphone track. Both can be on at once for live-narrated demos.

```
LLM: record action=start audio=true show_clicks=true
     → recording_id: rec-1745627812-1
       path: ~/Library/Caches/kinclaw/recordings/rec-20260425-225612.mp4
LLM: ui action=click title=Save
LLM: record action=stop id=rec-1745627812-1
     → path: ~/.../rec-20260425-225612.mp4
       duration: 12.4s  bytes: 8.3M  frames: 372
```

Other actions: `list` (active recordings), `stats id=...` (live frame
counters).

### `web` — drive the open internet

When the task lives outside macOS apps (login flows, dynamic SPAs,
sites without a public API), the `web` claw runs Playwright headless-
or-headed on top of Chromium. Ships as an external `SKILL.md` in
`skills/web/` so it stays a thin Python shim around `python3 web.py`
— forge can rewrite it without recompiling kinclaw.

```
LLM: web action=goto url="https://news.ycombinator.com"
LLM: web action=text selector="h1"
     → "Hacker News"
LLM: web action=click selector="text=login"
LLM: web action=type selector="input[name=acct]" text="..."
```

First launch downloads Chromium (~500 MB) into Playwright's cache.
Subsequent launches reuse it.

## Audio I/O — talk to your Mac, hear it back

`tts` and `stt` ship as **external SKILL.md plugins** in `skills/tts/`
and `skills/stt/`. They wrap [localkin-service-audio](https://github.com/LocalKinAI/localkin-service-audio)
— a local-first audio server running Kokoro (TTS) on `:8001` and
SenseVoice (STT) on `:8000` by default. See that repo's README for
install + run instructions; KinClaw discovers the endpoints via the
`TTS_ENDPOINT` / `STT_ENDPOINT` env vars (override the defaults if
you put the server elsewhere).

```
LLM: tts text="接下来打开计算器"
     → CJK auto-detected; speaker=zf_xiaoxiao; Kokoro synthesizes;
       afplay plays through speakers; record captures it as system
       audio if a recording is in flight.
LLM: tts text="Then I'll search for kinclaw" speaker=af_bella
     → English voice on demand.
LLM: stt path=~/Library/Caches/kinclaw/recordings/rec-XXXX.mp4
     → text: "今天天气怎么样"
       language: zh
```

> **Note on voice selection.** LocalKin Service Audio's `/synthesize`
> takes the parameter `speaker`, not `voice` — passing `voice=...` is
> silently ignored and falls back to the English-only Kokoro pipeline,
> which mispronounces Chinese text as the literal phrase "chinese
> letter". The `tts` SKILL.md auto-picks `zf_xiaoxiao` whenever the
> text contains non-ASCII characters; override with `speaker=...` for
> a different voice.

Why external SKILL.md and not native? Because they're HTTP wrappers,
exactly the shape `forge` would author. Keeping them external means
the kernel stays thin and users can fork either file without
recompiling. They also serve as forge templates for any next HTTP
service you want to integrate.

## Soul Clone (fission primitive #1)

```go
import (
    "github.com/LocalKinAI/kinclaw/pkg/clone"
    "github.com/LocalKinAI/kinclaw/pkg/soul"
)

// Make 10 parallel email readers, each assigned one email.
paths, _ := clone.Clone("souls/email_reader.soul.md", clone.CloneOptions{
    Count: 10,
    FrontmatterPatch: func(i int, meta *soul.Meta) {
        meta.Name = fmt.Sprintf("Email Reader #%d", i)
    },
})
// Clones land next to the parent, discovered on /reload.
```

Cheap (kilobytes), fast (milliseconds), no model calls. Task
fission becomes an N-way parallel tool invocation.

## Skill Forge (fission primitive #2)

Inherited from the `localkin` base. When the LLM asks for a skill
that doesn't exist, `forge` drafts a `SKILL.md` + implementation
script, validates syntax, registers it in the live registry, and
retries the original task. See `pkg/skill/native.go` for the forge
skill.

## Skill harvest — `kinclaw harvest`

`kinclaw harvest` pulls candidate `SKILL.md` files from other agent
repos (Claude Code, Hermes Agent, your own private repos), runs them
through the forge quality gate v2 + critic soul review, and stages
survivors at `~/.localkin/harvest/staged/` for human approval. Final
acceptance into `./skills/` is always manual — the pipeline never
auto-merges.

Three commands:

```bash
kinclaw harvest                          # scan all sources, curator triages → stage yes/maybe
kinclaw harvest --review                 # show what's staged + verdicts
kinclaw harvest --accept claude-code/foo # coder forges this one into ./skills/<name>/
```

### Scan = triage, not forge

`kinclaw harvest` runs the **curator** specialist soul
(`souls/curator.soul.md`, Kimi K2.6 / 1T params) over each external
candidate. Curator knows:

- KinClaw's architecture (5 claws, soul system, exec philosophy, non-goals)
- Your **actual** `./skills/` inventory (auto-injected at run start)
- The candidate's name + description + body excerpt

Curator returns one of three verdicts per candidate, with a one-line
reason:

| Verdict | Action |
|---|---|
| **yes** | obvious gap-filler that fits exec form → stage |
| **maybe** | partial overlap or unclear → stage with the doubt noted |
| **no** | already have it / pure LLM workflow / out of scope → drop |

Cost is small per call (~3s × ~500 tokens on Kimi K2.6). A full scan
over Hermes Agent's 85 skills runs in ~4 minutes / ~40k tokens —
much cheaper than forging anything.

### Forge happens at `--accept` time

When you've reviewed and want to actually use one of the staged
candidates, `kinclaw harvest --accept <source>/<skill-name>` spawns
the **coder** specialist (`souls/coder.soul.md`, DeepSeek V4 Pro)
to forge a real KinClaw exec-style `SKILL.md`. Three outcomes:

| Coder result | Lands at |
|---|---|
| **forged** + parses + passes forge gate v2 | `./skills/<forged_name>/` (runnable) |
| **defer_to_procedural** (capability needs LLM/AX/vision) | `./skills/library/<source>/<name>/original.md` (kept as inspiration) |
| forge errors (unparseable / forge gate fail / duplicate) | clear error, nothing written |

You only pay the forge cost (~30s / ~2k tokens) on candidates you
actually want — not on every procedural skill in the source repos.

### Cron mode

```bash
kinclaw harvest --no-judge               # cron-cheap: clone caches + count, no LLM
kinclaw harvest --diff                   # dry-run: scan + triage, write nothing
```

The launchd cron template (`scripts/com.localkin.kinclaw-harvest.plist`)
runs `--no-judge` — 3 AM jobs only refresh source caches + report counts.
Run `kinclaw harvest` (no flags) interactively when you want the curator
triage.

Manifest at `~/.localkin/harvest.toml`:

```toml
[[source]]
name         = "claude-code"
url          = "https://github.com/anthropics/claude-code"
skill_paths  = ["plugin-source/skills/**/SKILL.md"]
license_allow = ["MIT", "Apache-2.0"]

[[source]]
name         = "openclaw"
url          = "file:///Users/you/Code/openclaw"   # local, no clone
skill_paths  = ["skills/**/SKILL.md"]
license_allow = ["*"]                              # self-owned
```

See `harvest.example.toml` at the repo root for the canonical template.

A nightly cron template ships at
`scripts/com.localkin.kinclaw-harvest.plist` — runs `kinclaw harvest
--all --stage --no-critic` at 03:00 daily. Replace `USERNAME` then
`launchctl load` it. New candidates flow into staging while you sleep;
review them in the morning with `kinclaw harvest --review`.

## Sub-agent dispatch — `spawn`

When a subtask wants a different brain than the pilot's main lineage —
multimodal verification, deep web research, adversarial review — pilot
can dispatch to a specialist child:

```
spawn(soul=researcher, prompt="...", timeout_s=180)
  → child stdout (text)
```

The child boots from `souls/<name>.soul.md`, runs its own toolchain
on its own model, and returns a string. **Hierarchical** (not peer),
**synchronous** (not ambient), and kernel-capped at depth 1 — children
cannot themselves spawn. Sub-agent dispatch ≠ multi-agent: peer-swarm
coordination stays an explicit non-goal in the kernel.

Four specialists ship in `souls/`:

| Soul | Brain | When to dispatch |
|---|---|---|
| `researcher` | `kimi-k2.6:cloud` (1T, 256k ctx) | external facts, deep web research |
| `eye` | `kimi-k2.6:cloud` (multimodal) | AX-blind UI verification (canvas, dense icons) |
| `critic` | `minimax-m2.7:cloud` | adversarial second opinion on plans / forge'd skills |
| `coder` | `deepseek-v4-pro:cloud` | re-implement an external SKILL.md as KinClaw exec form (used by `harvest --inspire`) |

Different labs on purpose: pilot + researcher + eye on Moonshot Kimi,
critic on Minimax, coder on DeepSeek — different model lineage means
different blind spots, which is the whole point of asking for a second
opinion (or a different style).

Opt in via `permissions.spawn: true` in the soul. Specialists default
to `false` — even if a child somehow got the schema it can't dispatch.
See `pkg/skill/spawn.go` for the implementation.

## CLI reference

```
kinclaw -soul PATH                  Launch REPL with a specific soul
kinclaw -soul PATH -exec S          Run one message, print response, exit
kinclaw -soul PATH -cleanup-apps    On exit, quit any apps kinclaw started
                                    (preserves apps you already had open)
kinclaw -login                      Claude OAuth PKCE (Max subscription)
kinclaw -soul PATH -debug           Print tool calls & raw API traffic

Subcommands (own their own flag sets):
  kinclaw probe Notes               Audit one app's AX tree, get a verdict
  kinclaw probe -json com.apple.Notes
  kinclaw probe -batch < ids.txt    CSV scan for many apps (auto-cleanup)
  kinclaw probe -h                  Full probe help

  kinclaw harvest                   Scan external repos, curator triages
                                    candidates, stages yes/maybe ones
  kinclaw harvest --review          Show staged + verdict + reason
  kinclaw harvest --accept <id>     Coder forges this one → ./skills/<name>/
  kinclaw harvest --diff            Dry-run; triage but write nothing
  kinclaw harvest --no-judge        Cron-cheap: just refresh caches, no LLM
  kinclaw harvest -h                Full harvest help

In-REPL commands:
  /soul [name]     List / switch soul files
  /reload          Re-read the current soul + discover new skills
  /skills          List active skills
  /history         Show session messages
  /info            Version / soul / model / skill count / tokens
  /quit            Exit
```

### `kinclaw probe` — 1-second app audit

Before driving a new app, run `kinclaw probe <name>` to see if its AX
surface is rich enough for the `ui` claw, or whether you'll need to fall
back to `input` keystrokes / vision. Four verdicts:

```
🟢 rich     — `ui` claw alone drives it (≥ 50 nodes, ≥ 5 actionable)
🟡 shallow  — `ui` + `input` (cmd-keys / type-text) hybrid
🟠 blank    — needs `record` + screen + vision (menubar app, hostile shell)
🔴 dead     — process didn't open (TCC / sandbox / not installed)
```

The same probe, fed bundle IDs from stdin, produced the 50-app validation
that recorded **94% controllable, 88% pure-AX** on a real dev Mac — empirical
evidence that the 5-claw thesis holds. It now ships in the box.

## Roadmap (post-1.4)

What's shipped, what's next, and what's an explicit non-goal.

**Shipped (1.0–1.5)**: 5 claws, soul clone, skill forge with v2 quality
gate, sub-agent dispatch (4 specialists), `kinclaw probe` AX audit,
`kinclaw harvest` skill ETL pipeline + `--inspire` re-forge mode,
background-safe input via `target_pid`, batched AX IPC
(`Element.GetMany`, 2-5× tree dump speedup), launchd cron template,
cross-session memory.

**Near-term v1.6+ candidates** (fluid):

- **`kinclaw memory`** — list / search / forget against the
  cross-session `~/.localkin/learned.md` (currently write-mostly).
- **`kinclaw doctor`** — sidecar health check (TTS / STT / SearXNG /
  Playwright / kinrec). New-user pain point #1.
- **Observer subscriptions** in `kinax-go` — push-based AX event
  notifications (`AXObserverCreate` + CFRunLoop). Pairs with a TTL
  element cache for quasi-realtime UI tracking. *Note: `kinax-go`
  v0.2.0 shipped `GetMany` batched fetch, not Observer — that's still
  ahead.*
- **Homebrew tap** — `brew install localkinai/tap/kinclaw`.
- **More specialists** — `quick` (DeepSeek Flash, fast yes/no
  verifications), `linguist` (GLM 5.1, EN ↔ 中文 style rewrite).

**Apple-cert blocked** (待 $99 Apple Developer Program 解锁):

- **DMG + signing + notarization** — `kinclaw.localkin.ai/download`
  one-click install for non-developers.
- **Wails console + 🦞 menu bar** — visible swarm UI on top of CLI.
- **Relay service** — reach your own KinClaw from anywhere
  (`kinclaw.localkin.ai`, the actual monetization layer).
- **iOS Shortcuts / Siri** integration.

**Explicit non-goals** (not changing):

- ❌ **Multi-agent peer swarm in the kernel.** Sub-agent dispatch
  (`spawn`, hierarchical, depth-1) is OK and shipped. AutoGen-style
  peer coordination belongs in the LocalKin platform layer, not in
  KinClaw itself.
- ❌ **Windows / Linux support.** macOS-native IS the positioning
  (the Hermès craft, not the Zara mass-market).
- ❌ **Token-markup pricing.** Software stays MIT. Revenue model is
  the relay service when it ships.
- ❌ **Fine-tuned KinClaw-specific model.** Brain stays swappable.
- ❌ **OSWorld / benchmark leaderboard chasing.** Real-app,
  real-task validation (`kinclaw probe -batch` 50-app reports) is
  what we report.
- ❌ **Rewriting in Rust / Swift.** `openclaw` (private Rust port
  experiment) hit the `objc2` interop wall before the architectural
  fun parts. Go + purego is the right shape.

## Why not Computer Use / Operator?

Both of those products are architecturally single agents with a
fixed toolbelt, clicking around a virtualized browser in Anthropic's
or OpenAI's infrastructure. KinClaw:

- **Runs on your actual Mac**, not a container. macOS native
  (ScreenCaptureKit, CGEvent, Accessibility) rather than a
  virtualized X11.
- **Go, not Python** — single binary, no `pip install` drift, no
  environment setup.
- **Swarm, not singleton** — Soul Clone is table stakes.
- **Local-first brain option** — Ollama / Qwen3-VL for
  privacy-sensitive tasks.
- **Self-forges skills** — when a skill doesn't exist, it's written
  and registered at runtime. No competitor has this.

None of this is magic; it's boring engineering applied consistently.

## Contributing

```bash
git clone https://github.com/LocalKinAI/kinclaw
cd kinclaw
go build -o kinclaw ./cmd/kinclaw
go test ./...
```

A soul file that does something interesting is the best first
contribution. PRs adding `souls/community/<your_name>.soul.md`
welcome.

## License

MIT. See `LICENSE`.

## See also

- The four KinKit libraries (table above).
- [LocalKin.dev papers](https://www.localkin.dev/papers) —
  architectural essays: Grep Retrieval, Thin Soul + Fat Skill,
  Autonomous Heart, Embedded Dylib (paper #9 explains this repo's
  claw layer), more.
