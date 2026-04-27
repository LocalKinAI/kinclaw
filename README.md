# KinClaw

> **The self-fissioning lobster. Breeds its own swarm on demand.**
> 可以裂变的龙虾 — 根据需求自己造龙虾群。

KinClaw is a computer-use agent for macOS. It sees your screen,
understands your UI semantically, clicks, types, and — the part no
one else has — **reproduces on demand** via two primitives:

- **Soul Clone** (`pkg/clone`) — duplicate a specialist into N
  parallel workers with small per-clone divergence.
- **Skill Forge** (`pkg/skill` forge) — when an existing skill
  can't handle a task, KinClaw drafts, writes, registers, and tests
  a new one, then retries.

Single binary, ~25 MB, Go 1.22+, MIT licensed. Runs on your actual
Mac (not in a virtualized container like Anthropic's Computer Use or
OpenAI's Operator).

> *Same starter lobster for everyone. Every user's swarm is unique
> after a month.*

KinClaw grew out of the earlier `localkin` runtime (a minimal
embodied-AI microkernel, ~2,300 lines). This repo is that same
skeleton with the **three claws** bolted on: eye (ScreenCaptureKit),
hand (CGEvent), visual cortex (Accessibility API) — each via its own
zero-cgo KinKit library.

## Quick start

```bash
go install github.com/LocalKinAI/kinclaw/cmd/kinclaw@latest

# Default pilot runs Kimi K2.6 via Ollama Cloud. Sign in once:
ollama signin

# The demo soul: a pilot that drives your Mac
kinclaw -soul souls/pilot.soul.md

# Then ask it something like:
# > "What app is in front? Click the Save button if there is one."
```

Want a different brain? Pick another soul:

```bash
# Claude (OAuth, free tier works for testing)
kinclaw -login
kinclaw -soul souls/coder.soul.md         # Claude
kinclaw -soul souls/researcher.soul.md    # Claude
kinclaw -soul souls/openai.soul.md        # set $OPENAI_API_KEY
kinclaw -soul souls/ollama.soul.md        # 100% local Llama via Ollama
```

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
| Voice synthesis | Kokoro / LocalKin Service Audio | `TTS_ENDPOINT` | run server on `:8001` |
| Voice recognition | SenseVoice | `STT_ENDPOINT` | run server on `:8000` |
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
│  forge  — author new skills (with kernel quality gate)      │
│  learn  — append cross-session lessons to learned.md        │
│  clone  — duplicate souls into N parallel workers (lib)     │
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
  model: "kimi-k2.6:cloud"
permissions:
  shell: false
  network: false
  screen: true       # sckit-go capability
  input: true        # input-go capability
  ui: true           # kinax-go capability
  record: true       # kinrec capability — video MP4 + audio
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

## The four claws in action

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
`at_point` (hit-test a coordinate).

### `input` — raw mouse + keyboard

When there's no AX element (canvas apps, games, some WebGL), fall
back to coordinates:

```
LLM: input action=click x=842 y=523
LLM: input action=type text="hello 世界 👋"
LLM: input action=hotkey mods="cmd+shift" key="t"
```

### `screen` — just take a picture

```
LLM: screen action=screenshot
     → ~/Library/Caches/kinclaw/screens/screen-20260424-001312.000.png
```

The LLM can then read the PNG back (if `file_read` is enabled) and
reason about it visually.

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

## Audio I/O — talk to your Mac, hear it back

`tts` and `stt` ship as **external SKILL.md plugins** in `skills/tts/`
and `skills/stt/`. They wrap the [LocalKin Service Audio API](https://github.com/LocalKinAI/) — a local-first
audio server running Kokoro (TTS) on `:8001` and SenseVoice (STT) on
`:8000` by default. Override with `TTS_ENDPOINT` / `STT_ENDPOINT` env
vars.

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

## CLI reference

```
kinclaw -soul PATH         Launch REPL with a specific soul
kinclaw -soul PATH -exec S Run one message, print response, exit
kinclaw -login             Claude OAuth PKCE (Max subscription)
kinclaw -login-openai      OpenAI API key setup
kinclaw -soul PATH -debug  Print tool calls & raw API traffic

In-REPL commands:
  /soul [name]     List / switch soul files
  /reload          Re-read the current soul + discover new skills
  /skills          List active skills
  /history         Show session messages
  /info            Version / soul / model / skill count / tokens
  /quit            Exit
```

## Not in v0.1 scope

This is the computer-use skeleton. The full "self-fissioning swarm"
vision layers these on top later:

- **Conductor** — multi-clone orchestration (pick N, assign, reconvene).
- **Genesis Protocol** — generate new specialist souls from a knowledge
  corpus (YouTube / PDF / doc site → new expert). Lives in LocalKin's
  private commercial engine today; partial open-source release TBD.
- **Observer subscriptions** (kinax-go v0.2) — react to UI change
  events rather than poll.
- **Cross-window coordination** — two KinClaws across two displays.
- **Undo layer** — record every OS-touching action so it can be
  reversed.

Each happens when the v0.1 pilot has real users filing real issues.

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
