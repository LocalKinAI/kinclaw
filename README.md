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

# Claude OAuth login (free tier works for testing)
kinclaw -login

# The demo soul: a pilot that drives your Mac
export ANTHROPIC_API_KEY=sk-ant-...
kinclaw -soul souls/pilot.soul.md

# Then ask it something like:
# > "What app is in front? Click the Save button if there is one."
```

First run triggers two macOS TCC prompts:
- **Screen Recording** (for `screen` skill via sckit-go)
- **Accessibility** (for `input` + `ui` skills via input-go + kinax-go)

Grant both; rerun.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│               Soul (.soul.md)                           │
│  YAML frontmatter + Markdown system prompt              │
│  capabilities: screen / input / ui                      │
├─────────────────────────────────────────────────────────┤
│                   Brain (LLM)                           │
│  Claude · OpenAI · Ollama · Groq · DeepSeek · any       │
├─────────────────────────────────────────────────────────┤
│                     Skills (Tools)                      │
│  shell · file_read/write/edit · web_fetch · web_search  │
│  forge  (self-generate new skills)                      │
│                                                         │
│  ─── The three claws ───                                │
│  screen ─ eye          ─► sckit-go  (ScreenCaptureKit)  │
│  input  ─ hand         ─► input-go  (CGEvent)           │
│  ui     ─ visual cortex─► kinax-go  (Accessibility API) │
│                                                         │
├─────────────────────────────────────────────────────────┤
│              Fission primitives                         │
│  clone  (Soul Clone)     — duplicate N specialists      │
│  forge  (Skill Forge)    — author new skills from fail. │
├─────────────────────────────────────────────────────────┤
│            Memory (SQLite per session)                  │
└─────────────────────────────────────────────────────────┘
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
  provider: "claude"
  model: "claude-sonnet-4-5"
  api_key: "$ANTHROPIC_API_KEY"
permissions:
  shell: false
  network: false
  screen: true       # sckit-go capability
  input: true        # input-go capability
  ui: true           # kinax-go capability
skills:
  enable: ["screen", "input", "ui", "file_read"]
---

# You are KinClaw Pilot...
```

The `screen / input / ui` bits are the KinClaw addition. Each
corresponds to one TCC prompt and one KinKit library. If a bit is
false, the matching skill returns `permission denied: soul does not
grant X capability` regardless of what the LLM asks for.

## The three claws in action

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
