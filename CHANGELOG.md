# Changelog

## [1.8.0] - 2026-05-01

**Browser-based floating chat UI: `kinclaw serve`.** KinClaw was a
CLI all the way through v1.7.x — REPL or `-exec` one-shot. v1.8.0
adds a chat UI as a sibling subcommand. The form is **a single
compact floating window** designed to sit in a corner of your
desktop while the agent operates your real Mac alongside it. Not a
remote-desktop view of a virtual sandbox; not a split-pane "watch
the agent's screen" — just a chat box you talk to, while you watch
your actual macOS screen change because of what you said.

This is "Spotlight for agentic computer-use" as the form factor.
It's also the first cross-platform-ready piece of the project: the
shell (chat / SSE / markdown / voice) is platform-agnostic; only
the 5 claws underneath are macOS-specific. Linux / Windows / Android
ports become "write the claws" rather than "rebuild the UI."

### Added — `kinclaw serve` subcommand

```
kinclaw serve [-soul PATH] [-port N | -addr HOST:PORT]
              [-no-record] [-replay JSONL_PATH] [-debug]
```

Default port **8020** (avoid macOS AirPlay Receiver on 5000/7000;
collision now caught with a clear hint at startup). Open the printed
URL in any browser:

- **Single-column compact layout** — topbar + trace timeline + floating
  glass-blur input bar. Designed for ~380×600 floating windows.
- **Chrome `--app=URL` mode** for app-like floating window today;
  v0.2 will ship a native Swift WKWebView shell with real
  always-on-top.
- **Streaming markdown** in the trace (tables / code fences / lists /
  links / blockquote / hr) with `requestAnimationFrame`-coalesced
  re-render so a long streamed reply doesn't jank.
- **Per-tool result styling**:
  - `shell` — terminal vibes, prepended `$` prompt
  - `spawn` — sub-agent's report rendered as full markdown (violet
    accent)
  - `web_search` — parsed into clickable link list with title / URL /
    snippet
  - generic — monospace + collapse-when-long with explicit expand
    button (no forever-scroll mini-pane)
- **Voice** — `🎙` push-to-talk mic button + `🔊` TTS toggle. STT
  proxies to `${STT_ENDPOINT:-:8000}/transcribe` (LocalKin Service
  Audio / SenseVoice); TTS to `${TTS_ENDPOINT:-:8001}/synthesize`
  (Kokoro). CJK auto-picks `zf_xiaoxiao`; non-CJK lets server
  default.
- **Soul switcher** — click soul name in topbar → dropdown lists all
  souls under `./souls/` + `~/.localkin/souls/` with active soul
  highlighted. Hot-swap mid-session.
- **Session JSONL recording** — every event captured to
  `~/.localkin/serve-sessions/<ts>.jsonl` by default. `--replay`
  plays back a recorded session at original timing (delta-capped at
  2s for snappy review). `--no-record` opts out.
- **Esc-to-cancel** — DELETE `/api/chat` cancels the in-flight turn
  via context cancellation. Browser shows a `⨯` button while running.

### Server endpoints (`pkg/server`)

- `GET /` — single-file HTML UI (embedded via `//go:embed`)
- `GET /api/events` — Server-Sent Events stream (text_delta /
  tool_call / tool_result / soul_switched / turn_done / error)
- `POST /api/chat {message}` — start a turn (echoes via SSE)
- `DELETE /api/chat` — cancel current turn
- `GET /api/souls` — list discoverable souls
- `POST /api/soul {path}` — hot-swap soul (refuses mid-turn)
- `POST /api/voice/transcribe` (multipart) — STT proxy
- `POST /api/voice/tts {text, speaker?}` — TTS proxy → audio/wav
- `GET /file/<allowlisted-path>` — serve images/videos referenced in
  tool results (allow-list: `~/Library/Caches/kinclaw`,
  `~/.localkin`, `./output`, soul.OutputDir)

### Fixed — pilot soul behavior

- **`ui tree` defaults to `depth=2`** — previous default `depth=6`
  was dumping 11K+ chars of irrelevant menu trees (entire Apple
  menu / Recent Items / Window arrangement submenus) for simple
  apps like Calculator. Doctrine added: use minimum sufficient
  depth, drill down only when needed.
- **Read-vs-compute distinction** — pilot was caught doing mental
  math and reporting it as a screen-read result (`ui read` returned
  `value=""` but agent still answered "459+443=902" with
  confidence). New doctrine: if you can't trace the answer to a
  specific tool output, don't report it; either screenshot it for
  the user, try a different read path, or explicitly mark the
  answer as inferred not observed.

### Fixed — web skill

- New `press_enter=true` parameter — for React/Vue/Svelte forms
  whose submit button is gated on framework-internal state and
  won't enable from `page.fill()` alone. After the fill, sends a
  real keyboard `Enter` keypress (not synthetic) so the form's
  keydown handler actually fires + 500ms settle.

### Notes

- macOS-only as before; the new UI shell is platform-portable but
  the 5 claws still only have darwin implementations. Linux / Windows
  / Android ports = write the claws + reuse the shell.
- Chrome's `--app=URL` mode gives 80% of the floating-window UX
  today. A native Swift WKWebView shell with real always-on-top is
  v0.2 work, not in this release.
- OpenClaw (`openclaw/openclaw`) shares the lobster theme and
  "personal AI assistant" framing but operates a different domain
  (multi-channel messaging hub: WhatsApp / Discord / Slack / etc).
  KinClaw operates the OS itself. Different products, different
  niches; brand collision noted but not a problem.

## [1.7.2] - 2026-04-29

**Doctrine demotion: OCR is no longer in the cascade.** v1.7.0
introduced OCR as Layer 2 between AX and vision-LLM in the
"reading the screen" cascade. Real reflection on KinClaw's actual
workflow showed OCR's middle slot wasn't earning its keep:

- AX already covers ~94% of macOS apps cleanly
- The remaining 6% is mostly canvas / image-rendered UI where the
  agent doesn't just want literal text — it wants understanding,
  which is vision-LLM territory anyway
- OCR's character-confusion failure modes (`W↔H`, `M↔N`, `l↔I↔1`,
  `O↔0`, `B↔8`) — even at conf=1.0 — meant agents had to verify
  OCR results against another source, often vision-LLM, making
  OCR an extra hop instead of a shortcut
- Three layers added decision overhead at every "what's on screen"
  question; two layers (AX → vision LLM) is the honest shape

### Changed — cascade is now two-tier

```
Layer 1   ui claw           ~50ms      $0       deterministic
Layer 2   screen + vision   ~3s        ~$0.005  generic
```

OCR is documented as a **side tool** for narrow niches:
- Bulk extracting many numeric values where vision-LLM cost would
  be prohibitive
- Pure text + bounding-box jobs where you don't need semantics
- Offline runs without brain auth

Pilot soul body updated with explicit "❌ don't default to OCR" rules:
- "Read this button's label" → AX (`ui read`), not OCR
- "What's on screen" → vision LLM directly, not OCR-then-vision
- canvas understanding → vision LLM, OCR's text-without-semantics
  doesn't help

### Kept — `screen action=ocr` API

The action stays exposed and the sckit-go v0.2.0 OCR primitive stays
shipped. This is a **doctrine** change, not an API removal — when
the niche fits, it's right there. The README's `screen` section
mentions it as a sibling utility instead of leading with it.

### Why this matters

Letting OCR sit in the default cascade was a v1.7.0 over-engineering
mistake. We had a hammer (Vision framework) and made it look like a
nail (the middle layer). Two layers + occasional side-tool reach is
the honest mental model for KinClaw's real workflow; pretending
otherwise just trains pilot to take detours.

## [1.7.1] - 2026-04-29

Polish on v1.7.0. The OCR action's "fresh capture" path was wasteful:
v1.7.0 piggybacked on `screenshot()` which writes a PNG to
`~/Library/Caches/kinclaw/screens/` and then re-reads it from disk
just to feed into Vision. Honest data flow was:

  screen action=ocr → call screenshot() → png.Encode TO FILE →
    parse path out of result string → os.ReadFile FROM FILE →
    sckit.OCR

A user-visible disk round-trip on every OCR call. ~5MB written +
read back into the same buffer, ~5-20ms wasted, and a stray PNG
file dropped on every call.

### Changed — `screen action=ocr` is now in-memory

```
screen action=ocr  (fresh capture)
  → pickDisplay
  → sckit.Capture            (returns image.Image)
  → png.Encode to bytes.Buffer  (no file)
  → sckit.OCR                (reads buffer)
  → text + boxes

screen action=ocr path=<file>
  → os.ReadFile              (still file-backed since user pointed there)
  → sckit.OCR
```

Result label changed accordingly:

  Before:  `OCR on /Users/.../kinclaw/screens/screen-20260429-143012.000.png`
  After:   `OCR on <in-memory capture display=1 1920x1080>`

### Refactored — shared `pickDisplay` helper

Extracted `pickDisplay(ctx, params)` from `screenshot()` so both the
file-writing screenshot path and the in-memory OCR path share the
same `display_id` resolution logic. Pure cleanup; behavior preserved.

`screenshot` still writes a file as before — that's its job. Only
`ocr` (no path) skips disk.

### Build

`go build / vet / test ./...` — all green; no test changes (OCR
integration tests live in sckit-go).

## [1.7.0] - 2026-04-29

**Two new claw-level capabilities** wired up from upstream KinKit
releases the same day:

- **`screen action=ocr`** — on-device text extraction via Apple
  Vision framework (sckit-go v0.2.0). Gets text + bounding boxes
  out of any screen region without burning vision-LLM tokens.
- **`ui action=watch`** — push-based AX event subscriptions
  (kinax-go v0.3.0). Block on specific UI changes (focus / value /
  window-create / menu-open) instead of polling `ui tree`.

Both belong in the **5-claw + extensions** part of the architecture
diagram — `screen` and `ui` gain one new action each, no new
top-level claws. Five claws thesis stays intact ("does KinClaw
have enough?" answer: still yes; these are sharper edges on the
existing claws, not new claws).

### Added — `screen action=ocr`

```
screen action=ocr                       # OCR a fresh screenshot
screen action=ocr path=/tmp/foo.png     # OCR an existing image
```

Output:

```
OCR on /Users/.../screen-20260429-143012.000.png — 7 text region(s):
  "Save"                  at (412,85)  size 48x14   conf=1.00
  "Cancel"                at (480,85)  size 56x14   conf=1.00
  "今天天气怎么样"          at (200,300) size 280x40 conf=0.99
  ...
```

Uses [VNRecognizeTextRequest](https://developer.apple.com/documentation/vision/vnrecognizetextrequest)
inside sckit-go's existing dylib. ~50-200ms per screen-sized image,
local-only, free, deterministic. Recognition level: Accurate
(opinionated; no knob in v1.7).

When to use:
- Read a value out of a Calculator / Photoshop status bar
- Extract chart labels / canvas-rendered text
- Verify a rendered text matches expected (post-action verification)

When NOT to use: if the question is "what does this screen MEAN"
(intent / structure / next action) — that's still a vision-LLM job.
OCR returns text + boxes, nothing more.

Cost compare:

| Op | vision-LLM | OCR |
|---|---|---|
| "What does this textbox say" | ~$0.005 + ~3s | ~50-200ms / $0 |

### Added — `ui action=watch`

```
ui action=watch events=AXFocusedWindowChanged duration_ms=5000
ui action=watch events=AXValueChanged,AXMenuOpened duration_ms=3000 pid=12345
ui action=watch events=AXApplicationActivated bundle_id=com.apple.Cursor duration_ms=10000
```

Output (synchronous block-until-deadline):

```
watched pid=12345 for 5000ms (events: [AXFocusedWindowChanged]) — 2 notification(s):
  +1234ms  AXFocusedWindowChanged  AXWindow "Settings"
  +3812ms  AXFocusedWindowChanged  AXWindow "Cursor — main.go"
```

Backed by kinax-go v0.3's `Observer`: dedicated worker thread,
CFRunLoop, AXObserver subscription, condvar-driven event queue.
The skill subscribes at the **application root** for the watch
duration, blocks the caller for `duration_ms`, returns everything
that fired.

Defaults:
- `events`: `AXFocusedWindowChanged`
- `duration_ms`: 3000 (max 30000)
- target: focused application (or pid / bundle_id)

When to use:
- Wait-for-confirmation patterns: "I clicked Save; tell me when the
  value updated" → `events=AXValueChanged duration_ms=2000`
- Catch a dialog: `events=AXWindowCreated duration_ms=5000`
- Observe user activity: `events=AXApplicationActivated`

When NOT to use:
- Replacing `ui tree` — watch tells you WHAT changed, tree tells you
  WHAT IT LOOKS LIKE NOW. Compose: watch → fire → tree.
- Long-running monitoring (>30s) — that's a future streaming-mode
  primitive (`ui watch_stream`) not yet shipped. Loop watch calls
  if you need it today.

Common notifications:
`AXFocusedWindowChanged` / `AXFocusedUIElementChanged` /
`AXValueChanged` / `AXTitleChanged` / `AXWindowCreated` /
`AXWindowResized` / `AXMenuOpened` / `AXMenuClosed` /
`AXApplicationActivated`. Full list in
[Apple's AXNotificationConstants](https://developer.apple.com/documentation/applicationservices/axuielement_h/ax_notification_constants).

### Pilot soul

`souls/pilot.soul.md` gains a new "v1.7+: OCR 抽文字 / Observer 等事件"
block in the `## 裂变` section, with the 派/别派 decision rules:

  派 OCR: 要"读"屏幕里的文本（不需要理解 → 不烧 vision LLM）
  派 watch: 要"等"UI 事件（替代 sleep + re-tree 的轮询）

  别派 OCR: 要"理解"屏幕含义 → 还是 vision LLM
  别派 watch: 想知道"现在屏幕长啥样" → `ui tree` 才行

### Dependencies

- `github.com/LocalKinAI/sckit-go` v0.1.0 → **v0.2.0** (OCR)
- `github.com/LocalKinAI/kinax-go` v0.2.0 → **v0.3.0** (Observer)

Both libs released today alongside this kinclaw release. Local
`replace` directives in go.mod since the libs aren't pushed yet
(per the binding "no push" workflow rule established 2026-04-29).
Drop the replace directives once libs are tagged + go-cached.

### Build

`go build / vet / test ./...` — all green; no kinclaw test changes
(integration tests for OCR live in sckit-go; for Observer in
kinax-go, both with self-contained or unit-style coverage).

### Why this matters

The 5-claw thesis says "few primitives, deep" — `screen` was always
"just take a picture", `ui` was always "drive AX semantically". With
this release, `screen` gains a CHEAP local text-extraction path
(no vision-LLM round-trip) and `ui` gains an EFFICIENT change-
detection path (no polling). Two big classes of agent task get
faster + cheaper, but the architecture stays five-claw.

Indicative impact on a typical agent loop:

| Old pattern | New pattern | Savings |
|---|---|---|
| `screen` + vision-LLM "read this textbox" | `screen action=ocr` | ~$0.005 + ~3s → ~50-200ms / $0 |
| Loop `ui tree` waiting for state change | `ui action=watch` | 2-10× fewer AX IPCs, sub-second responsiveness |

## [1.6.0] - 2026-04-29

**Harvest reframed: triage at scan, forge at accept.** v1.5.x pushed
the heavy work (coder spawn per procedural candidate) into the scan
pass — a single `kinclaw harvest` against Hermes Agent burned ~80 LLM
calls and 30+ minutes regardless of whether the user wanted to
actually use any of those skills. Wrong shape.

v1.6.0 splits the two questions:

- **Scan-time** (`kinclaw harvest`) — a strong KinClaw-aware LLM
  (curator, Kimi K2.6 / 1T params) reads the current `./skills/`
  inventory + each external candidate, returns one of `yes / maybe /
  no` with a one-sentence reason. Cheap (~3s per candidate, ~500
  tokens), parallelizable, gives the user a triage list to look at.
- **Accept-time** (`kinclaw harvest --accept ID`) — coder forges THIS
  ONE candidate into a real KinClaw exec-style SKILL.md. Forge
  succeeds → `./skills/<name>/`. Coder defers (capability genuinely
  can't be exec'd) → `./skills/library/<source>/<name>/original.md`
  preserved as inspiration. Forge errors → clear message, nothing
  written.

The total LLM cost moves from "every candidate scanned" to "every
candidate the user actually wants to use" — drops by 10-50× on real
manifests.

### Added — `souls/curator.soul.md`

New specialist soul. Brain: `kimi-k2.6:cloud` (1T, 256k context).
Permissions: `file_read` only (reads `./skills/` for current state at
spawn). Job: triage external skill candidates against KinClaw's
actual inventory + design philosophy. Outputs three lines:

```
verdict: <yes | maybe | no>
reason: <one sentence — gap filled / already have / out of scope>
domain: <short tag — apple / git / web / ml / creative / ...>
```

Soul body has the full KinClaw architecture digest (5 claws, exec
philosophy, explicit non-goals) so judgments are grounded, not
hallucinated. Pipeline injects the actual `./skills/` inventory in
every per-candidate prompt.

### Added — `pkg/harvest/judge.go` + `pkg/harvest/inventory.go`

`LoadInventory(dir)` walks `./skills/` at run start, parses each
`SKILL.md`, builds a compact `name — one-line description` digest
that gets injected into every curator prompt.

`Judge(ctx, kinclawBin, soulPath, inventory, candidate)` spawns
curator with the inventory + candidate excerpt, parses the
three-line response. Returns `JudgeResult{Verdict, Reason, Domain,
FullText}`. ~3s per call, ~500 tokens — vs the v1.5 forge spawn at
~30s + ~2k tokens per call.

### Changed — `pkg/harvest/pipeline.go` simplified to one path

The v1.5 split between `processExecCandidate` (parse + forge gate +
critic) and `processProceduralCandidate` (coder forge + critic + 3
output kinds) collapsed into a single `processCandidate`:

```
read content
  → extract name + description + body excerpt (yaml frontmatter or
    file path fallback for `.cursorrules`-style entries)
  → spawn curator with current inventory + candidate
  → if yes/maybe → stage original.md + judge.txt + meta.txt
  → if no → drop, count in summary
```

No forge gate at scan time. No critic at scan time. Both happen at
`--accept` time only.

### Changed — `pkg/harvest/stage.go` simplified shape

Staged dirs now carry just three files:

```
~/.localkin/harvest/staged/<source>/<name>/
  ├── original.md       (verbatim external content)
  ├── judge.txt         (curator's full response)
  └── meta.txt          (verdict / reason / domain / source url)
```

`StageInspiredCandidate`, `StageProcedural`, the `_procedural/`
subarea — all gone. Layout is uniform across yes/maybe.

### Changed — `AcceptStaged` is the new heavyweight path

```go
func AcceptStaged(ctx, opts AcceptOptions, skillID string) (*AcceptResult, error)
```

Reads `original.md` from staging, spawns coder via `Inspire()` (the
kept-from-v1.5 forge primitive), routes the result:

| Coder verdict | Action |
|---|---|
| `forged` + valid | write SKILL.md to `<SkillsDir>/<forged_name>/` |
| `forged` but parse fails / forge gate fails | error, nothing written |
| `defer_to_procedural` | copy `original.md` to `<LibraryDir>/<source>/<name>/` |
| `unparseable` | error, nothing written |
| destination already exists | refuse (`AcceptDuplicate`) |

The forge gate v2 still applies (validates the coder output before
placement); the critic is no longer in this path (was redundant
with the gate + the user's own review).

### Removed

- `pkg/harvest/critic.go` — no critic at any stage in harvest now.
  `souls/critic.soul.md` itself stays as a spawn target for pilot
  outside of harvest.
- `looksProcedural()`, `splitFrontmatterStr()`, `extractYAMLName()`,
  `sanitizeProcName()` from `inspire.go` — only used by v1.5's
  procedural-vs-exec branching, no longer needed (uniform path).
- `--inspire` and `--no-inspire` flags from `kinclaw harvest`. v1.5
  default-on-with-opt-out is gone; the new flag set is `--no-judge`
  for cron mode.
- `--no-critic` flag — no critic anywhere in harvest.
- `CriticReview` / `CriticReviewInspired` / `CriticVerdict` /
  `CriticDecision` — not used.
- v1.5.1 era CHANGELOG mentioned `--inspire is now default`; this
  release retires the concept entirely.

### Changed — `kinclaw harvest -h`

```
Usage: kinclaw harvest [flags]

Read external agent skill libraries (configured in your harvest.toml
manifest), let the curator soul triage them against KinClaw's actual
skill inventory, stage candidates for review.

Three commands:

  kinclaw harvest                  scan + triage → stage yes/maybe candidates
  kinclaw harvest --review         list what's staged
  kinclaw harvest --accept ID      coder forges this one into ./skills/<name>/
                                   (or copies to ./skills/library/ if coder defers)
```

### Changed — cron plist defaults to just `--no-judge`

```xml
<key>ProgramArguments</key>
<array>
  <string>/usr/local/bin/kinclaw</string>
  <string>harvest</string>
  <string>--no-judge</string>
</array>
```

`--no-critic --no-inspire` (v1.5.1's combo) is gone. Cron's job
becomes: keep source caches warm + count what's there. Triage is
explicit interactive `kinclaw harvest`.

### Tests

`pkg/harvest/judge_test.go` — 13 cases:

- `parseJudgeResponse`: yes / maybe / no / Chinese punctuation /
  no-verdict-line / optional-domain
- `extractCandidate`: yaml-frontmatter happy path / no-frontmatter
  fallback (`.cursorrules` shape)
- `SkillInventory.String()` rendering + empty case
- `LoadInventory` on nonexistent dir / on real SKILL.md fixture
- `firstSentence` boundary cases (paragraph / line / `". "` / no
  terminator / leading whitespace / `wttr.in` non-stripping)

`pkg/harvest/inspire_test.go` trimmed to just the
`parseInspireResponse` cases (still in use at accept-time).

`go test ./...` — all green; full pkg/harvest exercise + 4 new
testfunc additions.

### Why this matters

The v1.5.x design pushed the user toward a "decide nothing, run
everything" mode where harvest tried to forge anything procedural-
style every time it ran. The result was a flooded staging area and a
confused mental model. v1.6.0 makes it scannable: harvest is a
**search**, not a build. Reading the staged list tells you what
exists in the wider ecosystem that curator thinks fits KinClaw's
shape — pick what you actually want, pay the forge cost only there.

The cost arithmetic:

| Op | v1.5.x cost | v1.6.0 cost | Note |
|---|---|---|---|
| Full scan over Hermes (85) | 85 × ~30s × ~2k tok = **42 min / ~170k tok** | 85 × ~3s × ~500 tok = **4 min / ~42k tok** | curator triage |
| Per-skill forge (when wanted) | included in scan | 1 × ~30s × ~2k tok = **30s / ~2k tok** | only for accepted ones |
| Typical user flow (scan + accept 5) | 42 min / 170k tok | 4 min + 5×30s = **~7 min / ~52k tok** | **3-7× cheaper** |

## [1.5.1] - 2026-04-29

**UX simplification.** v1.5.0 introduced `kinclaw harvest --inspire`
as opt-in; first-run feedback was that the design was too modal — too
many flags, the relationship between exec-style and procedural-style
flow wasn't clear, and the default `kinclaw harvest` produced 85
identical "must have name, description, and command" lines for the
common Hermes / Anthropic / Cursor case (procedural with no `command`).

This release flips it to **one mental model**: `kinclaw harvest`
scans, forges, stages — then `--review` to see, `--accept` to copy.

### Changed — `--inspire` is now default; opt OUT with `--no-inspire`

The procedural-forge path is the common case for any external skill
library worth harvesting from. Making it opt-in meant the default
`kinclaw harvest` did almost nothing useful for ~95% of input. The
flip: forge by default, opt out for cron / cost-saving runs.

```bash
# Before (v1.5.0)
kinclaw harvest --inspire        # opt in to forge procedural skills

# After (v1.5.1)
kinclaw harvest                  # forge by default
kinclaw harvest --no-inspire     # opt out (cron / cheap mode)
```

`--inspire` is silently accepted as a no-op for backward compat (old
docs / old plists keep working).

### Removed — compat no-op flags

Three v1.3.1-era compat flags (`--all` / `--apply` / `--stage`) were
no-ops kept around so the launchd plist would tolerate the docs as
written. Two minor versions later they're just clutter — removed.
The example plist updated to drop them.

### Changed — top-level `kinclaw -h` now lists subcommands

Previously `kinclaw -h` only printed top-level flags; new users had no
way to discover `kinclaw harvest` / `kinclaw probe` from the help
output. New shape:

```
kinclaw — macOS computer-use agent (5 claws + soul + forge + spawn + harvest)

Usage:
  kinclaw -soul PATH [-exec MSG]    Run a soul (REPL or one-shot)
  kinclaw harvest                    Pull external skill libraries; coder forges
                                     KinClaw versions of good ideas; stage for review
  kinclaw harvest --review           Show staged candidates
  kinclaw harvest --accept ID        Copy one staged candidate into ./skills/
  kinclaw probe APP                  Audit one app's AX surface (1-second verdict)
  kinclaw -login                     Claude OAuth (free tier)
  kinclaw -version                   Show version

Top-level flags: ...

Subcommand help: kinclaw harvest -h  /  kinclaw probe -h
```

The "no soul file found" error path also points at `kinclaw -h` for
discovery instead of just dying with a one-liner.

### Changed — `kinclaw harvest -h` slimmed

v1.5.0's harvest help was a 30-line modal description. v1.5.1's is
a 3-line action menu plus the flags. The pipeline-stage list moved
to CHANGELOG / README; the help text is for "what do I run."

### Changed — launchd cron defaults to both `--no-critic --no-inspire`

The shipped plist (`scripts/com.localkin.kinclaw-harvest.plist`) used
to run with `--all --stage --no-critic`. With inspire now default-on,
"run as before" means the cron would burn LLM tokens nightly on every
new procedural candidate (×80+ × ~30s = expensive). The plist now
explicitly opts out of both LLM steps:

```xml
<string>kinclaw</string>
<string>harvest</string>
<string>--no-critic</string>
<string>--no-inspire</string>
```

Cron's job becomes "keep source caches warm + report what's new";
interactive `kinclaw harvest` is when you actually spend tokens.

This affects only NEW installs of the plist — already-installed plists
on user machines keep their existing args and behave per the v1.5.0
cron pattern.

### Why this matters

`kinclaw harvest` was supposed to be the "skill library grows itself"
flow. v1.5.0 made it a power-user feature with three flag combinations
to learn. v1.5.1 makes it one command — same as `git pull` or `brew
upgrade`. The cron and dry-run modes are still there as opt-outs; they
just don't define the mental model anymore.

## [1.5.0] - 2026-04-29

**`kinclaw harvest --inspire`** — the harvest pipeline now treats
procedural-style external SKILL.md files (Anthropic / Hermes / Cursor —
`name + description + markdown body`, no `command` field) as
**inspirations**, not as files to translate.

The old harvest pipeline's premise (pre-1.5) was "SKILL.md is a
universal schema; copy across ecosystems." That premise was wrong:
KinClaw SKILL.md is an exec wrapper (`command + args` ran via
`exec.Command`), the Anthropic family is a procedural prompt for an
LLM. Same name, different things. v1.4.1 cron showed it bluntly —
85/85 Hermes skills rejected as "must have name, description, and
command."

v1.5.0 reframes harvest: **吸取思想，不抄实现.** Read external
procedural skills as concept prompts, then ask the `coder` specialist
to **re-implement** the same capability as a KinClaw exec-style
skill. Not translation — re-creation in our native form.

### Added — `kinclaw harvest --inspire`

```
kinclaw harvest --inspire                       # all sources, run inspire on procedural candidates
kinclaw harvest --source claude-code --inspire  # one source
kinclaw harvest --inspire --diff                # dry-run; show what would happen
```

When `--inspire` is set and a candidate fails the regular parse
(missing `command`), the pipeline checks if the file is procedural-
shaped (`looksProcedural`: has YAML frontmatter with `name +
description`, no `command`). If yes, it spawns the **coder soul**
(`souls/coder.soul.md`, DeepSeek V4 Pro) with the original SKILL.md
content. Coder reads, judges, and outputs one of two shapes:

- `verdict: forged` + a complete KinClaw SKILL.md between
  `---KINCLAW_SKILL_BEGIN--- … ---KINCLAW_SKILL_END---` markers.
  Pipeline re-parses + runs forge gate v2 + critic on it (with the
  original supplied for **alignment review**), and stages it under
  `staged/<source>/<skill-name>/` with `from_inspire=true` in
  `meta.txt`. Marked ✨ in `--review` output.
- `verdict: defer_to_procedural` + `reason:` — coder refused because
  the original capability genuinely needs LLM round-trips, AX/vision,
  or pure prompt template (things a single shell exec can't capture).
  Pipeline stages the original to
  `staged/<source>/_procedural/<name>/` with the defer reason. Marked
  📜 in `--review`. **These can NOT be `--accept`'d** — there's no
  exec form to promote — but they're preserved so a human can browse
  what concept inspirations the harvest run found.

The `coder` specialist soul (repurposed in v1.4.1 for exactly this
role) carries the honesty invariant: it refuses to fabricate exec
mappings for capabilities that genuinely don't have one, instead of
producing a fake-but-passing SKILL.md.

### Added — alignment-aware critic (`CriticReviewInspired`)

When critic reviews a forged-from-inspiration skill, it now sees
**both** the original procedural content **and** the coder's forged
version. Same critic soul, new prompt that adds:

> Specifically check:
>   - command[0] is a real binary likely available on macOS
>   - schema parameters cover what the original implied
>   - the forge doesn't pretend to do what needs LLM round-trips
>   - no trivially broken patterns (osascript -e pairing, hardcoded
>     coords, schema/template mismatch)

Verdict shape unchanged (`accept | warn | reject`) — annotation only,
the staging decision is still on the human. Per-skill critic note
saved alongside as `critic.md` in the staging dir.

### Added — `_procedural/` staging area + `--review` distinguishes kinds

Staging layout grew one dimension:

```
~/.localkin/harvest/staged/
├── claude-code/
│   ├── reminders_add/             ← regular or inspire-forged
│   │   ├── SKILL.md
│   │   ├── meta.txt               (from_inspire=true if applicable)
│   │   ├── critic.md
│   │   └── inspire/               (only when from_inspire)
│   │       ├── original.md
│   │       └── coder_output.txt
│   └── _procedural/               ← deferred (no exec form)
│       └── dogfood/
│           ├── original.md
│           ├── defer_reason.txt
│           ├── coder_output.txt
│           └── meta.txt
```

`kinclaw harvest --review` now prints kind labels:

```
✓  claude-code/reminders_add  [regular]      — exec parsed cleanly
✨ claude-code/dogfood          [inspire-forged] — coder produced
📜 claude-code/yuanbao          [procedural (deferred)] — concept only
```

`AcceptStaged` refuses procedural entries with a clear error
explaining there's no exec form to promote.

### Added — `harvest.Result.Inspired` and `harvest.Result.Procedural`

The `Result` struct gains two slices alongside `Passed` so callers
(and the summary line) can distinguish how candidates resolved. The
summary now reads:

```
── summary
  hermes-agent  85 cand, 23 pass (12 ✨), 38 📜, 24 rej, 0 err
```

(12 inspire-forged candidates entered the regular skill pile via
coder; 38 deferred to `_procedural/`; 24 still legitimately broken.)

### Tests

`pkg/harvest/inspire_test.go` — coverage for the new pure-Go bits:

- `parseInspireResponse` — forged with full block, deferred with
  reason, missing verdict line, "forged" without block, Chinese
  punctuation (`verdict：`)
- `looksProcedural` — Anthropic-style yes, KinClaw-style no, missing
  fields no, malformed YAML no
- `sanitizeProcName` — spaces / hyphens / slashes / CJK / emoji /
  empty all normalize to safe identifier
- `extractYAMLName` — quoted / unquoted / missing / non-string

`go test ./...` — all green; coverage on net-new code excludes the
spawn-coder integration path (needs Ollama signin, can't run in unit
tests).

### Cron note

The launchd plist (`scripts/com.localkin.kinclaw-harvest.plist`)
still runs `--no-critic` and **does not** add `--inspire` by default.
Inspire is opt-in because it burns LLM tokens (one forge call + one
critic call per procedural candidate; 80+ Hermes skills × 2 = 170+
round-trips). Run `kinclaw harvest --inspire` manually when you're
ready to spend the budget.

### Why this matters

This closes the loop on the v1.3 / v1.4 pipeline. The harvest cron
now serves three populations of external skills cleanly:

1. Already-exec-style → parse, gate, critic, stage (cheap, automatic)
2. Procedural-style + `--inspire` → coder forges native form (medium-
   cost, opt-in, manual review)
3. Procedural-style without `--inspire`, or that coder defers →
   archived to `_procedural/` for browse-only

The skill library can now grow with the wider agent ecosystem instead
of rejecting it. **吸取思想，不抄实现** — KinClaw stays its own form
even as it absorbs what other agents have figured out is worth doing.

## [1.4.1] - 2026-04-29

Maintenance release. No kernel or claw behavior changes — `souls/`
trimmed to what's actually wired, README brought into sync with
reality after several minor versions of drift.

### Changed — `souls/` cleared of demo files; `coder` repurposed

Removed five demo / generic-brain souls that nothing in the kernel
referenced:

- `souls/deepseek.soul.md` ("Deep" — generic DeepSeek-direct demo)
- `souls/groq.soul.md` ("Bolt" — generic Groq Llama demo)
- `souls/locked.soul.md` ("Locked" — sandboxed Claude demo)
- `souls/ollama.soul.md` ("Local" — generic local Llama demo)
- `souls/openai.soul.md` ("Sage" — generic GPT-4o demo)

These were "switch-brain demos" from the pre-spawn era. Since v1.3.0
shipped `spawn` + 4 specialist souls, pilot already covers all
brain-routing use cases through specialization. Demo souls were noise.

`souls/coder.soul.md` repurposed from a thin generic "engineer" soul
on Claude Sonnet 4.6 into the specialist for the upcoming `kinclaw
harvest --inspire`:

- Brain: `deepseek-v4-pro:cloud`
- Tools: `forge` + `file_read` + `file_write` (no shell, no network,
  no spawn)
- Job: given a procedural-style SKILL.md from another agent ecosystem
  (Claude Code / Hermes / Cursor rules), re-implement the SAME
  capability as a KinClaw exec-style SKILL.md via forge — NOT machine
  translation. Refuses (`verdict: defer_to_procedural`) when the
  original needs LLM round-trips, AX/vision, or pure prompt-
  engineering that the exec form can't capture.

`pkg/skill/spawn.go` ToolDef updated: `coder` is no longer marked
"(when added)" in the available-specialist list.

### Changed — README cleanup (multi-version drift fixed)

The README accumulated staleness across v1.0 → v1.4. Rewritten /
corrected:

- Intro paragraph: "three claws" → "five claws" (record + web added
  in v1.2.0); specialist count "researcher / eye / critic" → "/ coder";
  binary size 25 MB → 17 MB (actual current)
- Quick Start: "Default pilot runs Kimi K2.6" → K2.5 (matches actual
  `souls/pilot.soul.md`)
- Soul schema example: same K2.6 → K2.5 fix
- "The four claws in action" → "The five claws in action"; added the
  missing `web` claw subsection (Playwright over Chromium, ships as
  external SKILL.md in `skills/web/`)
- Sub-agent dispatch table: was 3 specialists, now lists all 4 with
  `coder` and its harvest --inspire role
- CLI reference: removed `-login-openai` (flag doesn't exist in
  `main.go`; was misleading documentation)
- Renamed "Not in v0.1 scope" (we're at v1.4.0!) to "Roadmap (post-1.4)"
  — split into Shipped / Near-term v1.5+ / Apple-cert blocked /
  Explicit non-goals. Corrected the misleading "Observer subscriptions
  in kinax-go v0.2" hint — `kinax-go` v0.2 was `GetMany`, observer
  is still ahead.
- Removed dropped quick-start lines for the deleted demo souls
  (already done in the cleanup commit, repeated here for the
  changelog record).

### Build

`go build` / `go vet` / `go test ./...` — all green; no test changes.

## [1.4.0] - 2026-04-29

**Behavior-defining minor.** v1.4.0 is the first KinClaw that doesn't
have to take over your Mac to do its job. Two upstream KinKit features
that landed yesterday (input-go v0.2.0, kinax-go v0.2.0) get wired up
to the kernel — and one of them changes the *kind* of agent KinClaw
is.

### Added — `input target_pid` (background-safe input)

The `input` skill takes an optional `target_pid` integer. When set
(>0), every synthesized event routes directly to that process via
[CGEventPostToPid](https://developer.apple.com/documentation/coregraphics/cgeventposttopid):

```
input action=click x=400 y=300 target_pid=12345
input action=type text="hello" target_pid=12345
input action=hotkey mods=cmd key=s target_pid=12345
```

The targeted app receives the event but **its window does not come to
front**. The user's foreground app keeps focus — your editor doesn't
lose its insertion point, your YouTube tutorial doesn't pause, and
multi-window workflows finally work. KinClaw is no longer "an agent
that takes over your Mac." It's an agent that helps in the background
while you keep working.

Verified on the same lineup axcli (Rust) proved: Lark / VSCode /
Chrome / Cursor and other Electron + WebKit hosts. Some Apple
sandboxed apps (newer Mail / Messages) may ignore PID-targeted
events — fall back to omitting `target_pid` if no effect.

Pilot soul gains a new section "**D. 后台模式**" in `## 裂变` with the
派/别派 decision table:

- **派 target_pid**: user said "in the background" / "don't disturb my
  current X"; multi-app parallel tasks; PID known from `ui focused_app`
- **别派**: demo / screen recording (focus change is the show);
  user's current foreground IS the target; sandboxed app doesn't
  respond (fall back)

### Changed — `ui tree` is 2-5× faster (Element.GetMany)

The tree dump that powers `ui tree` and `ui find` now batches the 5
attribute fetches per node (Role / Title / Identifier / Description /
Value) into a single AX IPC call via
[AXUIElementCopyMultipleAttributeValues](https://developer.apple.com/documentation/applicationservices/1462091-axuielementcopymultipleattribute).
Indicative on a populated Cursor window subtree (~400 nodes):

| Op                              | v1.3.1   | v1.4.0   | Speedup |
|---------------------------------|----------|----------|---------|
| `ui tree` 7 attrs × ~400 nodes  | ~280 ms  | ~70 ms   | 4.0×    |
| `ui tree` 4 attrs × ~150 nodes  | ~70 ms   | ~22 ms   | 3.2×    |

Pattern lifted from AXSwift's `getMultipleAttributes` during the
2026-04-28 cross-language survey. Tree dump is the hottest path in
any AX-driving agent — the speedup compounds across the multiple
`ui tree` calls pilot makes per turn (planning + verification +
post-action re-tree). Indirect win: the agent falls back to vision
(token cost) less often.

The change is transparent to the soul / forge / agent — same skill
surface, same output format. `dumpTree` in [pkg/skill/ui.go](pkg/skill/ui.go)
gained a `treeAttrs` constant and a `strAttr` helper for type-safe
extraction from the GetMany result map.

### Dependencies

- `github.com/LocalKinAI/input-go` v0.1.0 → **v0.2.0**
- `github.com/LocalKinAI/kinax-go` v0.1.0 → **v0.2.0**

Both KinKit libs released yesterday alongside this work; see their
respective CHANGELOGs for the full story.

### Why this matters

This is the **first KinClaw release where the agent's relationship to
the user is fundamentally different**. v1.0-v1.3 was "an automation
tool that uses your Mac via your foreground." v1.4 is "an agent that
operates apps in the background while you keep working." The same
binary still does both — `target_pid` is opt-in — but the option is
now there, and pilot's soul knows when to reach for it.

The `Element.GetMany` win is quieter but compounds harder: every `ui
tree` is now cheap enough that the agent can re-tree after every
action without thinking about cost. That makes the verification loop
("did my click actually do what I wanted?") tighter, which is the
bedrock of self-correction.

## [1.3.1] - 2026-04-28

**Polish on v1.3.0.** Sub-agent dispatch landed yesterday; today we point
the gun at our own ecosystem. v1.3.1 ships a `kinclaw harvest` subcommand
that pulls candidate skills from third-party agent repos, runs them
through the existing forge quality gate v2 + critic soul, and stages
survivors for human approval. No kernel changes — every new capability
is a thin tool layer on top of what v1.2.x and v1.3.0 already shipped.

### Added — `kinclaw harvest` subcommand

```
kinclaw harvest                          # all sources, run pipeline → stage
kinclaw harvest --source claude-code     # one source
kinclaw harvest --diff                   # dry-run, write nothing
kinclaw harvest --review                 # list staged candidates
kinclaw harvest --accept <id>            # promote staged → ./skills/
kinclaw harvest --no-critic              # skip the critic spawn (cron / CI)
```

The pipeline (per source):

1. `git clone --depth=1` to `~/.localkin/harvest/sources/<name>/`
   (cached; re-runs do `git pull --ff-only`)
2. **License gate** — auto-detects `LICENSE` / `LICENSE.md` / `COPYING`
   header and matches against `license_allow` list (defaults: MIT /
   Apache-2.0 / BSD-3-Clause; `["*"]` for self-owned repos)
3. Glob `skill_paths` from manifest (supports `**` recursive matching)
4. Parse via `LoadExternalSkill` — same loader the kinclaw registry
   uses at boot, so anything that survives is guaranteed to load
5. **Forge quality gate v2** — name pattern, `command[0]` in `$PATH`,
   osascript `-e` pairing, no hardcoded coords, schema/template var
   consistency. Hard reject; the candidate doesn't get staged.
6. **Critic soul review** — spawns `souls/critic.soul.md` against each
   surviving candidate. Critic *annotates* (`accept` / `warn` / `reject`)
   but does not auto-reject — staging includes the verdict so human
   review can sort fastest. Different lab from pilot on purpose
   (Minimax M2.7 vs Kimi K2.5) — different model lineage, different
   blind spots.
7. **Stage** at `~/.localkin/harvest/staged/<source>/<skill-name>/`
   with `SKILL.md` + `critic.md` + `meta.txt`. Final acceptance into
   `./skills/` is always a manual `--accept` step. The pipeline never
   auto-merges.

Manifest is TOML at `~/.localkin/harvest.toml`:

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

### Added — launchd cron template

`scripts/com.localkin.kinclaw-harvest.plist` runs `kinclaw harvest
--all --stage --no-critic` nightly at 03:00. Replace `USERNAME` and
the `kinclaw` binary path, then:

```bash
cp scripts/com.localkin.kinclaw-harvest.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.localkin.kinclaw-harvest.plist
```

Cron defaults to `--no-critic` because critic is an LLM call and
cron-time auth is brittle. Run `kinclaw harvest --review` in the
morning, sort by critic verdict (which you can re-add manually), pick
candidates, `--accept` them.

### Added — `pkg/harvest/` package + `pkg/skill.ValidateSkillMeta`

Reusable building blocks the harvest pipeline composes from:

- `pkg/harvest/manifest.go` — TOML manifest schema + validation
- `pkg/harvest/glob.go` — `**` doublestar globbing (no new dep, ~30
  LOC backtracking matcher)
- `pkg/harvest/source.go` — git cache + license header detection
  for MIT / Apache-2.0 / BSD-3-Clause / MPL-2.0 / GPL-2.0/3.0
- `pkg/harvest/critic.go` — wraps the critic soul spawn pattern
  (mirror of `pkg/skill/spawn.go`); parses verdict line in EN + 中文
- `pkg/harvest/pipeline.go` — orchestrator
- `pkg/harvest/stage.go` — staging IO, `--review` listing, `--accept`
  promotion (refuses to clobber existing skills)

`pkg/skill/validate.go` exposes a public `ValidateSkillMeta` —
previously the forge gate v2 lived only inside the forge skill.
Lifting it lets the harvest pipeline (and any future linter) call the
same checks the forge runs at write time, without re-implementing
them.

### Tests

- `pkg/harvest/glob_test.go` — 16 cases for the glob matcher
  (literal, `*`, `**`, trailing `**`, no-match) + 7 cases for license
  allowlist semantics + `globFiles` skips `.git` / `node_modules`
- `pkg/harvest/manifest_test.go` — valid manifest round-trip,
  4 invalid-manifest rejection cases (empty, missing url, missing
  skill_paths, duplicate name), 8 critic-verdict parse cases (EN +
  中文 + missing verdict line falls back to `warn`)
- End-to-end smoke (manual): point manifest at the kinclaw repo
  itself (`file://` URL, 12 SKILL.md files in `skills/`), `--diff`
  shows all 12 pass; real run stages all 12 to
  `~/.localkin/harvest/staged/kinclaw-self/`

`go test ./...` → 91 test functions across 10 packages (+9 from v1.3.0).

### Changed — `pkg/skill.ExternalSkill.Meta()`

Added a public getter so external code (the harvest pipeline,
future linters) can re-validate a loaded skill's frontmatter without
re-parsing the file.

### Dependencies

One new direct dep: `github.com/BurntSushi/toml v1.6.0` (TOML parser
for the manifest). Self-contained, single package, well-maintained,
~200KB binary impact. The harvest manifest format was chosen as TOML
deliberately — a flat array-of-tables scheme reads cleanly under
human edits and resists the fragility of YAML's whitespace + escape
quirks.

### Why this matters

Harvest closes the loop on Genesis. v1.2.x produced skills via forge
when the agent ran into a missing capability mid-task. v1.3.1 lets the
agent (and the user behind it) absorb good ideas from the wider agent
ecosystem — Claude Code, Hermes Agent, the user's own private repos —
without writing them by hand. The forge gate keeps the bar honest;
the critic soul adds a second-opinion review; staging keeps the human
the final approver. No PR auto-merge, no surprise capability bumps.

The `kinclaw harvest --no-critic` path also makes the launchd cron
self-sufficient: clones stay warm, new candidates flow into staging
nightly, the morning review session is one `--review` away.

## [1.3.0] - 2026-04-28

**First minor after the v1.2 fortification.** v1.2.0 grew the 5 claws and
v1.2.1 hardened the gates around them; v1.3.0 starts spending the
capability they bought. The headline is sub-agent dispatch — pilot can
hand a focused subtask to a specialist child running on a different
brain, and recombine the result back into the main thread.

This is **hierarchical, not peer**. Synchronous, not ambient. Kernel-
hard-capped at depth 1. Sub-agent ≠ multi-agent: peer-swarm coordination
stays an explicit non-goal in the KinClaw kernel — that layer belongs in
the LocalKin platform.

### Added — `spawn` skill (sub-agent dispatch)

```
spawn(soul=researcher, prompt="...", timeout_s=180)
  → child stdout (text)
```

How it works (`pkg/skill/spawn.go`, 189 LOC):

1. `exec.Command(self, -soul <resolved>, -exec <prompt>)` with
   `KINCLAW_SPAWN_DEPTH=1` set in the child's env.
2. `cmd.Output()` captures the child's `-exec` response. Boot banner
   goes to stderr and is dropped, so the parent sees only the answer.
3. Default 180s timeout, capped at 600s. Bogus `timeout_s` strings
   fall back to default rather than erroring early.
4. **Recursion guard**: the child's own `spawn` skill checks the env
   var on boot and refuses any further dispatch. Max depth = 1, kernel-
   enforced — the LLM cannot talk its way past it.

Soul resolution: name `"researcher"` resolves against
`./souls/researcher.soul.md` then `~/.localkin/souls/researcher.soul.md`
(same dirs the kinclaw CLI already uses). Absolute paths pass through
unchanged.

Permission gate: `NewSpawnSkill(enabled, soulDirs)` is registered
unconditionally but self-disables when `permissions.spawn` is false.
Specialist souls don't set the bit, so even if a child somehow got the
schema it can't actually dispatch — belt-and-suspenders with the env-
var guard.

### Added — 3 specialist souls

Each plays to its model's strength. Different labs on purpose: pilot is
on Moonshot Kimi, critic is on Minimax — different model lineage means
different blind spots, which is the whole point of asking for a second
opinion.

| Soul | Brain | Role |
|---|---|---|
| `souls/researcher.soul.md` | `kimi-k2.6:cloud` (1T, 256k ctx) | Deep web search + long-context synthesis. Read-only: only `web_*`, `file_read`, `file_write`. The honesty invariant from pilot is repeated verbatim — every fact must trace to a fetched source or be marked "未确认". |
| `souls/eye.soul.md` | `kimi-k2.6:cloud` (multimodal) | Pure visual verification. 2 skills only: `screen`, `file_read`. Answers 3 question shapes (*where is X / is state Y / is Z present*) with rigid output formats (coords + 1-line evidence). Forbidden from summarizing whole screens or fabricating non-visible elements. |
| `souls/critic.soul.md` | `minimax-m2.7:cloud` | Adversarial second opinion on plans / forge'd skills / soul edits. Output is a fixed 3-section structure: ✓ what passes / ⚠ risks ranked / overall verdict. Strictly read-only. |

All three set `permissions.spawn: false` explicitly — the YAML makes the
"sub-agents can't themselves spawn" contract obvious at a glance, in
addition to the env-var guard.

### Changed — soul schema gains `permissions.spawn`

```yaml
permissions:
  spawn: true     # default false; only pilot opts in today
```

`pkg/soul/soul.go` adds the bool field. Defaults to false to preserve
existing soul behavior — no surprise capability bumps on upgrade.

### Changed — pilot soul: routing guidance

Pilot now opts in (`permissions.spawn: true`) and adds `"spawn"` to
`skills.enable`. The soul body grows by 23 lines (135 → 173, still well
under the 433-line bloat v1.2.0 cut from) with a decision-table-shaped
section between the honesty axioms and `## 裂变`:

**派 (when to dispatch):**
- external facts (ratings / prices / docs) → `researcher`
- AX-blind UI elements (canvas / dense icons) → `eye`
- non-trivial skill about to be forged → `critic` first
- genuinely parallel subtasks → multiple `spawn`

**别派 (when NOT to dispatch):**
- one-or-two-step task — just do it inline
- answer already in current trace
- pure UI driving is the agent's day job — don't delegate it
- recursion is impossible anyway — kernel-capped at depth 1

The text is a decision table, not prescriptive prose — matches the
"thin soul" ethos. Without it, two failure modes were live: agent
spawns for everything (over-decomposition; slow + expensive), or
agent never spawns (specialists waste away unused).

### Tests

`pkg/skill/spawn_test.go` — 11 new cases:

- Disabled-state refusal (no subprocess launched)
- Recursion-guard via env var (child refuses second-level spawn)
- Empty/whitespace `soul` or `prompt` rejected up front
- Unknown soul name → clear not-found error
- `resolveSoul`: by name from soul dirs, by absolute path
- Bogus `timeout_s` string falls back to default 180 (no early error)
- `ToolDef` contains `soul` / `prompt` / `timeout_s` + skill name
- Description names `researcher` / `eye` / `critic` so LLM can route

End-to-end smoke (manual): `kinclaw -soul souls/pilot.soul.md -exec
"用 spawn 派 researcher 查 X"` → pilot dispatches → researcher boots,
runs, returns `未找到` rather than fabricating. Honesty invariant held
across the process boundary.

`go test ./...` → 82 test functions across 9 packages (+11 from v1.2.1).

### Why this matters

The 5 claws answered "can KinClaw drive any Mac app?" (94% on the 50-
app probe). Sub-agent dispatch starts answering the next question:
"can KinClaw be smart about *which* model drives *which* part of a
task?" Verification belongs on a multimodal brain. Web research belongs
on a long-context brain. Adversarial review belongs on a different lab
entirely. The pilot stays a generalist; the specialists are cheap to
add.

Sub-agent dispatch also makes the kernel-thin / soul-thin / fat-skills-
and-memory architecture pay rent: a new specialist is one `.soul.md`
file. No code changes, no kernel touches, no new schema. Just text.

## [1.2.1] - 2026-04-27

**Polish on v1.2.0.** Genesis-loop validation surfaced four edges that
needed sharpening: a real CLI for AX probing, automatic cleanup after
sessions, deeper forge quality gates, and a doctrine correction that
puts the UI claw back at the front of the line. No new architectural
primitives — every change reinforces what v1.2.0 already shipped.

### Added — `kinclaw probe` subcommand

First-class CLI for "is this app driveable by the `ui` claw, or do I
need to fall back to `input` / vision?" Same probe binary used in the
50-app validation now ships in the box. Three modes:

```
kinclaw probe Notes                   # by app name (resolved via /Applications)
kinclaw probe -json com.apple.Notes   # JSON output, machine-readable
kinclaw probe -batch < ids.txt        # CSV scan, many apps, auto-cleanup
```

Verdicts (same thresholds as the 50-app validation):

| Tier | Threshold | Meaning |
|---|---|---|
| 🟢 rich | nodes ≥ 50 AND actionable ≥ 5 | `ui` claw drives it |
| 🟡 shallow | nodes ≥ 10 | `ui` + `input` hybrid |
| 🟠 blank | nodes < 10 | needs `record` + vision |
| 🔴 dead | process didn't open | TCC / sandbox / not installed |

Where `actionable = AXButton + AXTextField + AXMenuItem` — counts
menu-driven apps (iWork, QuickTime, Freeform) and Electron apps
(VSCode, Cursor, Claude desktop) correctly even when they expose
0 AXButton.

Bundle resolution: bundle IDs pass through unchanged, app names
resolve against `/Applications`, `/System/Applications`,
`/System/Applications/Utilities`, `/Applications/Utilities`,
`~/Applications`. Spotlight is intentionally not used — indexing
is unreliable on dev machines and freshly-installed apps.

`pkg/probe/` is the reusable core; `cmd/kinclaw/probe.go` wires it
to the CLI via a git-style subcommand dispatch (preserves all
existing top-level flags). Pattern is ready for the followups
already on the polish list (`kinclaw memory`, `kinclaw doctor`).

### Added — `-cleanup-apps` flag

The 10-task validation surfaced this: after running `kinclaw -exec`
through 10 different apps, all 10 dock icons stayed open. Now:

```bash
kinclaw -cleanup-apps -exec "..."
```

snapshots running apps at startup, quits any new ones at exit (defer
+ SIGINT handler). Pre-existing apps stay alive — your workspace is
untouched. `kinclaw probe -batch` enables the same behavior by
default; pass `-no-cleanup` to suppress when you want to interact
with the probed apps afterwards.

Why `osascript quit` and not `pkill`: graceful shutdown lets apps
run `applicationWillTerminate` and surface unsaved-work dialogs the
user expects to see. Each quit is bounded to 3s; refusals are
reported but don't fail the cleanup.

System processes (Dock / WindowServer / loginwindow) are filtered by
AppleScript's `background only is false` — they never appear in the
snapshot, so they never get quit. New `pkg/applifecycle/` package
holds the snapshot/diff/quit primitives.

### Changed — forge quality gate v2 (deeper validation)

The v1.2.0 gate caught command[0] internal-name mistakes
(`["ui", ...]` etc.) but missed the next layer of LLM forge bugs.
Live observation: in tonight's 10-task validation, the agent
forge'd 4 skills, 2 of which had unparseable YAML and silently
crashed on every kinclaw boot. The gate now catches those before
they get written:

1. **Args parsed as JSON, re-emitted as YAML.** Agent used to pass
   `args: [-e tell app "X" to play]` (YAML-flow style) which we
   dumped into SKILL.md verbatim — invalid YAML. Now we
   `json.Unmarshal` into `[]string` and let `yaml.Marshal` handle
   quoting. Reject up front if not parseable.
2. **Round-trip via `LoadExternalSkill` before registering.**
   Failed reload triggers full dir cleanup + clear "produced
   SKILL.md doesn't reload" rejection. (Before: forge wrote
   SKILL.md, ignored LoadExternalSkill error, returned "success",
   left a broken file.)
3. **`osascript -e` pairing.** Each `-e` must be followed by a
   script string. Catches `["-e", "...", "-e"]` (dangling) and
   `["-e", "-e", "..."]` (consecutive flags).
4. **No hardcoded screen coordinates.** Reject `click at {N, M}`
   patterns — these worked once on the agent's bench and are broken
   on any other resolution. Live observed: a `maps_search_location`
   forged with `click at {760, 150}`. Now agent gets a clear
   rejection pointing at `keystroke` / `cmd-key` / AX-relative
   click as alternatives.
5. **Template var ↔ schema consistency.** Every `{{var}}`
   referenced in args must appear in the schema. Otherwise the
   template engine strips unknown vars to `""` and the skill
   silently loses parameterization.

`forgeNamePattern` restricts skill names to `[a-zA-Z][a-zA-Z0-9_]{0,63}`.

Tests: 25 new test cases across `pkg/skill/forge_validate_test.go`
(unit) and `pkg/skill/forge_e2e_test.go` (full forge.Execute with
`t.TempDir()`). Verifies AppleScript with nested quotes survives
YAML round-trip intact, confirms bad inputs leave no on-disk droppings.

### Fixed — UI claw is the FIRST resort, not a fallback

Earlier soul + forge description nudged the agent toward "shortest
path = AppleScript". Net effect on Apple-stock apps: agent stops
driving UI after the first run, which empties out KinClaw's whole
5-claw thesis on every reuse. Both texts reframed:

`souls/pilot.soul.md` `## 裂变` section B:
> Was: "可复用的模式要 forge"
> Now: "UI 先行；走不通才 forge"

Try `ui` claw first, even if slower. UI working = no forge needed
(the claw IS the skill). Forge ONLY when UI is genuinely blocked:
no AX surface (Docker menubar), reliable modal interruption, or ≥ 2
consecutive ui failures.

`pkg/skill/native.go` forge.Description():
> Was: "Choose the SHORTEST execution path"
> Now: "A correctly-forged skill is a confession that the UI claw
> couldn't do this on this app — never 'I chose the faster path'."

3 legitimate forge triggers (no AX surface / blocked modal /
repeated ui failures) and 3 anti-cases (UI worked, single-shot
task, learn would suffice).

### Cleanup

- Removed two broken forge'd skills from prior runs:
  `skills/reminders_add/` and `skills/maps_search_location/` — both
  had unparseable YAML that triggered boot warnings every session.
  Boot is now warning-clean.
- Promoted two GOOD forge'd skills as evidence the loop produces
  useful artifacts when inputs are clean: `skills/music_play/` +
  `skills/music_pause/` (both legitimate fallbacks — UI clicks fail
  when Music is backgrounded, AppleScript works either way).

### Removed — `cmd/probe-ax/`

The standalone research binary used in the 50-app validation. Its
logic moved into `pkg/probe/` and `cmd/kinclaw/probe.go` as the new
subcommand. Drop-in compatible with the old binary's stdin/stdout
contract, so the 50-app probe shell wrappers still work via
`kinclaw probe -batch`.

### Validation: 50-app probe + 10-task end-to-end

While polishing v1.2.0, two empirical validation runs landed in
`~/.localkin/research/`:

- `50-app-validation.md` — AX-tree probe over 50 curated apps from
  6 categories (Apple Native / Apple System / Utilities / Apple Pro
  / Electron / Heavyweight). Result: 94% controllable today, 88%
  pure-AX, 0 dead. Concrete proof the 5-claw thesis holds across
  the macOS ecosystem.
- `10-task-validation/REPORT.md` — End-to-end task validation across
  10 categorically-different apps (Reminders / Music / Pages / Cursor
  / Photos / Maps / Activity Monitor / Screenshot / Docker / Xcode).
  Result: 8/10 ✅ via the agent's own self-reported markers, 2 timeouts
  (Cursor + Photos — surfaced real edge cases worth follow-up).

The probe subcommand is the productized form of the first; the
`-cleanup-apps` flag is the reusable infrastructure surfaced by the
second.

### Build

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./...` ✅ (71 test functions across 9 packages)
- `GOOS=linux go build ./...` ✅ (non-darwin stubs intact)

---

## [1.2.0] - 2026-04-26

**The lobster grows up.** Five claws (added `web` for the open
internet via Playwright), audio in/out, real-time GPS, multimodal
vision passthrough, cross-session memory, self-evolving skills with
quality gates, and an honesty invariant. The pilot soul shrunk from
433 prescriptive lines to ~90 lines of identity + safety axioms,
trusting the kernel guards to catch failures.

This release is the architectural shape Genesis Protocol asked for:
a thin kernel, a growing helper layer, identity-not-prescription in
the soul, and a memory file that travels across sessions.

### Major: 5th claw — `web` (Playwright)

`pkg/skill` doesn't host this one — it's an external SKILL.md +
Python script (`skills/web/SKILL.md` + `web.py`). Single skill, 7
flexible parameters covering the common web automation patterns:

```
web url=X                                   → fetch rendered text
web url=X selector=".price" wait_for=...    → extract specific element
web url=X click=".search" type_text="kc"    → fill form, then read
web url=X screenshot=true                   → returns image:// marker
web url=X js="document.title"               → run JS, JSON result
```

Each call launches a fresh Chromium (~2-3s cold start), executes the
flow, closes. Stateless by design — multi-step tasks chain
parameters in one call rather than splitting across rounds. No
sidecar process, no port management. Setup once: `pip install
playwright && playwright install chromium`.

For sites Playwright can't crack (Cloudflare / DataDome / advanced
anti-bot), the user can drive their own logged-in Safari via
`osascript activate Safari` + `ui` skill — the slogan-true path no
cloud agent has ("your real browser, your real session").

### Major: audio I/O — `tts` + `stt`

Two external SKILL.md plugins wrapping LocalKin Service Audio:
- `tts` POSTs text to localhost:8001/synthesize (Kokoro), plays the
  WAV via `afplay`. CJK text auto-routes to a Chinese voice;
  ASCII goes server default.
- `stt` POSTs an audio file to localhost:8000/transcribe (SenseVoice).

Both endpoints overridable via `TTS_ENDPOINT` / `STT_ENDPOINT`. Both
default to `wait=false` (background playback) so demos don't burn
recording time on dead air; closing tts uses `wait=true` to give the
final frame time to render before `record stop`.

### Major: 4th claw — `record` (kinrec)

`pkg/skill/record.go` wraps kinrec (ScreenCaptureKit + AVAudioEngine)
for non-blocking video capture. Actions: `start` / `stop` / `list` /
`stats`. Independent system-audio + microphone toggles, click
highlighting, fps + display selection. `record start` blocks until
the first frame is actually captured, so subsequent tool calls
(activate / click / etc.) reliably appear in the recording.

`permissions.record: bool` added to soul schema. Mic capture
additionally requires Microphone TCC permission.

### Major: `web_search` — SearXNG backend

Multi-engine meta-search via local SearXNG (`SEARXNG_ENDPOINT` env
var, default localhost:8080). Falls back to DuckDuckGo HTML scrape
when SearXNG is unreachable. The result text includes `(via searxng)`
or `(via duckduckgo)` so both LLM and user can see which backend
served the query.

### Major: vision passthrough for tool-result images

`brain.Message` gains `Images []string`. The OpenAI brain builds a
multimodal `content` array with `image_url` blocks; the Claude brain
emits `image` source blocks in `tool_result`. Skills opt in by
emitting `image://<path>` markers in their text output; the registry
strips those markers and populates `ToolResult.Images`. Currently
used by `screen` (screenshots) and `web` (Playwright PNGs).

A multimodal brain (Kimi K2.5/2.6, Claude Sonnet 4.5+, GPT-5) now
actually sees the pixels — for the first time, "drive UI by sight"
works alongside "drive UI by AX tree".

### Major: cross-session memory — `~/.localkin/learned.md`

Soul.go reads the file at every boot, appends the content to the
system prompt under a `## 已学到的` header (capped at 8KB tail-
preserved). The `learn` SKILL.md is the agent's standardized way to
write back, with an idempotent dedup check.

The kernel doesn't enforce schema; the agent organizes notes by
bundle_id or topic. Net effect: every user's KinClaw becomes uniquely
expert in their own apps + workflows over time.

### Major: real-time GPS via `corelocationcli`

`skills/location/SKILL.md` wraps `corelocationcli` (`brew install
corelocationcli`). 4 modes: coords / address / city / full. Each
output is wrapped in a labelled "GPS:" preamble so smaller models
don't return empty responses on bare lat/lon.

Static soul context: `KINCLAW_LOCATION="lat,lon[,city[,country]]"`
env var feeds `{{location}}` / `{{lat}}` / `{{lon}}` / `{{city}}` /
`{{country}}` substitutions. Static is for "where the user generally
is"; the skill is for "where the user is right now."

### Major: kernel template substitutions — date / tz / platform / arch / location

Soul prompts can reference `{{current_date}}` `{{tz}}` `{{platform}}`
`{{arch}}` `{{location}}` `{{lat}}` `{{lon}}` `{{city}}`
`{{country}}`. Soul YAML stays portable — same file runs on any
host; the rendered prompt adapts to the runtime context. This is the
seed of cross-OS portability (macOS today, Linux/Windows when KinKit
ports arrive).

### Major: Genesis loop infrastructure

Soul prompt's `## 裂变` section frames `forge` + `learn` + final
report as **all part of task completion** — task isn't "done" until
all three are. Identity-level invariant.

`forge` got a kernel pre-flight quality gate: `command[0]` must be
in `$PATH` AND must not be a kinclaw internal skill name (`ui`,
`input`, `screen`, `record`, `shell`, `tts`, `stt`, `forge`,
`learn`, `web_*`). Live observation: agent forged a `reminders_add`
skill whose Python ran `subprocess.run(["ui", "action=click", ...])`,
silently failing every call but printing "success" — produces a
forever-lying skill. Pre-flight refuses, agent has to retry with a
real binary as `command[0]`.

Tool description rewritten to teach agents the **shortest execution
path** — direct AppleScript / shell APIs over UI-driving when the
app supports it.

### Major: pilot soul slim — 433 lines → ~90 lines

Removed: 7-round demo flow, schema discovery 4-step protocol,
matcher priority table, GUI hard constraints, speed rules, app-
specific advice (Calculator etc.). Kept: identity (you're a lobster,
4+1 claws, forge/learn/clone), safety axioms, style preferences.

The kernel guards (ambiguity refusal, destructive refusal, no-
progress loop detector, per-turn usage cap, record-start-blocks-
until-first-frame) catch the cases the prescriptive rules used to.
Trusting them frees the agent to discover task structure rather than
follow prescriptive flow.

### Major: honesty invariant in safety axioms

Same shelf as "don't type passwords" / "don't sudo":

> **不编造工具没抓到的事实**。任何写进给用户回复里的具体数字 /
> 评分 / 奖项 / 价格 / 电话 / 地址 / 年份 / 商家名 / URL 必须能
> 在你这一轮的某个 tool result 里字面找到。找不到就别写，或者
> 明说"未确认"。宁可模糊不可造假。

Live verification: agent driving a "find Thai restaurants near me"
flow now fetches multiple restaurant websites (amarin / shanathai /
tommy-thai) directly when Yelp blocks Playwright; reports only the
addresses / phones / hours / ratings actually present in the fetched
HTML. The 4.6⭐ / 40,323 reviews on Tommy Thai's listing came from
Tommy's own embedded Google Maps widget — a real number, not a
training-data hallucination.

### Added — generic helper skills

- `skills/app_open_clean/SKILL.md` — `open -a <app>` + AppleScript
  walks frontmost windows/sheets, dismisses any of {Continue / Get
  Started / Skip / Later / Not Now / Got It / Maybe Later / Done /
  Cancel}. Fixes the "agent typed into the welcome modal" failure
  mode for first-launch macOS apps (Reminders / Mail / Photos / Maps).
- `skills/learn/SKILL.md` — idempotent append helper for the
  cross-session notebook. `learn topic=<bundle_id> note=<line>`
  appends if new, no-ops if exact line already exists.
- `skills/location/SKILL.md` — corelocationcli wrapper, 4 output
  modes, K2.5-friendly text-framed output.

### Added — kernel guards (4-trigger circuit breaker)

`pkg/skill/circuit.go`:
1. Same skill + same error 3× consecutive (tight error loop).
2. Same skill fails 3× total this turn (cumulative).
3. Same skill returns same successful output 3× consecutive (no-
   progress loop — `ui find` returning "no elements matching"
   without changing matcher).
4. Same skill called ≥ 8× this turn regardless of outcome (over-
   iteration / fix-and-retry spiral).

Each trigger emits a `[SYSTEM]` hint into the conversation; agent
sees it and is expected to replan rather than burn round budget.

### Added — `ui` skill safety + ergonomics

- `ui click` ambiguity refusal: when ≥ 2 elements match, kernel
  refuses with the candidate list and instructions to add filters.
  Override with `force=true`. Caught the live failure where an
  AXCloseButton + the real button both matched a broad query and
  the kernel happily clicked the close button (= window gone, demo
  broken, agent narrating to empty desktop).
- `ui click` destructive-target refusal: AXCloseButton /
  AXMinimizeButton / AXFullScreenButton / titles matching
  Close|Quit|Exit|Log Out|Sign Out (English word-boundary) or
  退出|关闭|注销|结束 (Chinese substring) refuse without
  `force=true`.
- New action `ui click_sequence` — N buttons in one tool call,
  saves N-1 round trips for calculator-style flows. Three matcher
  modes (`titles=` / `descriptions=` / `identifiers=`).
- `ui tree` / `ui find` output now shows AXDescription and AXValue
  alongside title and identifier. Calculator's number buttons have
  empty titles but rich descriptions; without this column the agent
  saw "no usable matcher" and (wrongly) fell back to `input type`,
  which fails under macOS focus protection.

### Added — `record start` blocks until first frame

Without this, kinrec returned its `recording_id` while the
ScreenCaptureKit pipeline was still warming up; the next tool calls
(activate / click) ran during the warmup window and never appeared
in the final video. Frame 1 of every demo showed Calculator already
in its result state, with no demo content. Now `record start` polls
`r.Stats().Frames` until first frame is captured (1s cap), so
subsequent calls are guaranteed to be in-frame.

### Fixed — chatLoop stranded the conversation on error

When `chatLoop` errored mid-sequence (e.g. tool-call round budget
exhausted), `handleUserMessage` printed the error and returned
without persisting the partial tool history or any assistant
message. The conversation became `user→user→user→...` with no
assistant turns, which the brain on the next user message read as
"keep working on the prior task" → re-ran the same compound action,
exhausted the budget again, etc. Live observation: typing "你好"
right after a failed demo hit the round limit immediately.

Now the partial `toolHistory` is persisted and a synthesized abort
note is added: `"Turn aborted: <err>. Reply 'continue' to resume or
rephrase to start fresh."`

### Changed — `maxToolRounds` 20 → 50

20 was sized for kernel-only flows. Compound demos (record start +
tts + multi-step ui find/click/verify + tts + record stop) routinely
take 30+ rounds even when nothing goes wrong.

### Fixed — `{{var}}` substitution bugs (two)

- `{{var}}` inside a SKILL.md `command:` element used to leak
  through literally — only `args:` was substituted. Affected the
  shipped forge'd skills (git_commit, weather, summarize, translate)
  on every call. Substitution now applies to both Command and Args.
- Leftover `{{name}}` placeholders (param not provided) used to stay
  literal in the rendered command. SKILL.md authors used this to
  detect "param missing" via sentinel comparison
  (`[ "$X" = "{{name}}" ] && X=""`); when the caller DID provide
  `name="true"`, the substitution rewrote BOTH sides of the test
  and the param value got clobbered. Kernel now strips unsubstituted
  templates to "" so authors detect missing with the cleaner
  `[ -n "$X" ]` idiom.

### Changed — pilot soul collapsed to one file

`souls/pilot.soul.md` is now the Kimi-driven canonical pilot. The
old Claude-driven `pilot.soul.md` was deleted; the Kimi pilot was
renamed in via `git mv` so history is preserved.

### Build

- `go build ./...` ✅
- `GOOS=linux go build ./...` ✅ (non-darwin stubs intact)
- `go test ./...` ✅ — all pre-existing tests + 50+ new cases pass

### Added — the fourth claw

- **`record` skill** (`pkg/skill/record.go`) — wraps
  [kinrec](https://github.com/LocalKinAI/kinrec) (ScreenCaptureKit +
  AVAudioEngine). Non-blocking: `start` returns a `recording_id`
  immediately so the agent keeps operating the Mac while kinrec writes
  MP4 in the background; `stop` finalizes the file. Actions: `start`,
  `stop`, `list`, `stats`. Independent system-audio (`audio=true`) +
  microphone (`mic=true`) toggles, optional click-highlight, frame-rate
  override, display selection. `_other.go` no-op stub keeps non-darwin
  builds clean.
- **`permissions.record: bool`** added to soul schema. Defaults to
  false; older souls written before the bit existed continue to parse
  cleanly. Mic capture additionally requires the Microphone TCC bucket
  on first use.

### Added — audio I/O via external SKILL.md plugins

- **`skills/tts/SKILL.md`** — synthesize speech via LocalKin Service
  Audio (`:8001` / Kokoro by default), play through `afplay`. When
  `record audio=true` is in flight, kinrec captures the spoken output
  as system audio — high-quality multilingual narration in demo videos
  with no extra plumbing. Replaces `shell say` in the pilot demo flow.
  The voice parameter is `speaker:` (matches the server's actual JSON
  field — `voice:` is silently ignored and falls back to English-only
  Kokoro). The skill auto-picks `zf_xiaoxiao` whenever the text
  contains CJK characters so naive `tts text="你好"` calls don't
  mispronounce Chinese as the literal phrase "chinese letter".
- **`skills/stt/SKILL.md`** — transcribe audio files via LocalKin
  Service Audio (`:8000` / SenseVoice by default). Pairs with
  `record mic=true` to turn a mic track into text.
- Both shipped as external SKILL.md (not native) on principle: HTTP
  wrappers belong in fat-skill territory, not the kernel. They also
  serve as forge templates for any next local HTTP service.
- Endpoints overridable via `TTS_ENDPOINT` / `STT_ENDPOINT` env vars
  for users running their audio servers on different ports or hosts.

### Changed — pilot soul prompt hardened

- New `## GUI 操作硬约束` section codifying the lessons from the first
  v1.2 demo run: every `ui click` must follow `ui find`; never press
  AXCloseButton / AXMinimizeButton / AXFullScreenButton; never press
  Close/Quit/退出/关闭 labeled buttons; after `shell open -a` the
  agent must verify `focused_app` before proceeding; every successful
  click must be followed by an observation step.
- App-launch recipe rewritten around the macOS focus-protection
  reality: from a Terminal-driven agent, the OS frequently refuses
  to bring another app frontmost. The pilot used to insist on
  `focused_app == X` after `open -a` / `osascript activate`, which
  put it in a doomed loop (live observation: `打开 Safari` → activate
  4 times in a row, all returning "Terminal still focused", each time
  Kimi tried a different trick). The new prompt teaches:
  - **Most tasks don't need frontmost.** `ui` works on background
    apps via `bundle_id` (`ui click bundle_id=com.apple.Safari ...`).
    "Open Safari" succeeds when Safari is launched and reachable, not
    when it's frontmost.
  - **One activate, no retry.** If the user explicitly wants visual
    front (e.g. recording a demo), the pilot does ONE
    `osascript activate` + `focused_app` check. If still not frontmost,
    it stops, asks the user to click the app's window, and waits.
    No retry, no cmd+tab, no dock-click — focus protection won't yield.
  - Default operation order updated to lead with "**`ui` + `bundle_id`**"
    as the canonical pattern.
- First-run ritual marked **session-once-only** with explicit "do not
  re-run this on every user message" callout — Kimi was happily
  burning 5 tool calls per turn re-running the boot self-check, on
  top of the actual task.
- Self-summary text fixed: "三把爪子" (three claws) → "四把爪子"
  (four claws) + tts/stt to reflect v1.2's actual lineup.

### Changed — pilot soul collapsed to one file

- **`souls/pilot.soul.md`** is now the Kimi K2.6 (Ollama Cloud) version
  by default. The old Claude-driven `pilot.soul.md` was deleted; the
  Kimi pilot was renamed in via `git mv` so history is preserved. The
  rationale: Kimi K2.6 has the strongest free Chinese tool-use today,
  and shipping it as the default means a `kinclaw -soul souls/pilot.soul.md`
  works for someone with `ollama signin` already done — no API key
  setup required.
- Pilot soul body rewritten to introduce four claws, the two audio
  external skills, and a `## 录 demo 视频` section showing the
  `record + tts + ui` pipeline for self-recorded narrated demos.
- `souls/pilot_kimi.soul.md` removed (was the predecessor of the new
  default).

### Fixed — `forge` quality gate (refuse to write broken skills)

Day-1 of the Genesis loop produced a forged `reminders_add` skill
that **silently lied about success on every call**: the LLM wrote a
Python script that did `subprocess.run(["ui", "action=click", ...])`,
treating kinclaw's internal skill names as shell binaries. Those
don't exist in `$PATH`; the subprocess errored every call but the
script's terminal `print("Created reminder: X")` ran regardless,
producing a tool that confidently misreports success forever after.

Two-line kernel pre-flight in `pkg/skill/native.go::Execute`:

1. **Reject internal skill names as `command[0]`** — `ui`, `input`,
   `screen`, `record`, `shell`, `tts`, `stt`, `forge`, `learn`,
   `web_*`, etc. The error message names the violator and points at
   the right alternative (`osascript`, `sh`, `python3`, `curl`...).
2. **Reject `command[0]` not found in `$PATH`** via `exec.LookPath`
   — catches typos / hallucinated binaries (`reminderctl`, etc.)
   before the SKILL.md gets written.

`forge` tool description rewritten with concrete examples of the
shortest-path execution for common Apple apps (Reminders / Notes /
Music / Safari / Calculator via `osascript` or `bc`, no UI driving)
plus the hard rules and a complete correct `reminders_add` recipe
showing 3-line shape.

Pilot soul's `## 裂变` section reframed: forge / learn / report are
**all part of task completion** — task isn't "done" until the
checklist's 4 items are done. Identity-level invariant, same shelf
as the safety axioms.

`pkg/skill/skill_test.go` adds 15 new test cases:
- `TestForgeSkill_RejectsInternalSkillName` — 10 sub-tests, one per
  internal skill name, each confirming forge rejects the call.
- `TestForgeSkill_RejectsCommandNotInPath` — typo / missing binary.
- `TestForgeSkill_AcceptsRealBinary` — 4 sub-tests confirming the
  happy path still works for `sh`, `osascript`, `python3`, `bc`.

### Added — `learn` SKILL.md (cross-session lesson appender)

External SKILL.md at `skills/learn/`. Idempotent append helper for
the agent's notebook at `~/.localkin/learned.md` — kernel auto-loads
that file at boot. Usage: `learn topic=<bundle_id> note=<one line>`.
Creates section if missing, appends bullet if section exists, no-ops
if the exact line is already there. Pure shell + awk; no Go state.

Pilot soul's `## 你能裂变` section now frames forge + learn + the
final report as **one task — the task isn't "done" until forge,
learn, AND report are all done**. Identity-level rule, same shelf as
the safety axioms. Tighter than the previous "by the way" framing,
which the agent ignored on a live Reminders demo (created the
reminder ✓, forgot to forge `reminder_add` ✗, forgot to learn the
AXError -25205 quirk ✗).

The `forge` skill's tool description also gains explicit when-to-use
/ when-NOT-to-use guidance — naming examples (calc_compute,
notes_create, reminder_add) and the warning to skip when a skill
with the same name already exists.

### Added — `app_open_clean` SKILL.md (welcome-modal dismisser)

External SKILL.md at `skills/app_open_clean/`. Wraps `open -a` with
a two-pass AppleScript that walks the frontmost app's windows +
sheets and clicks any modal-dismiss button it finds (priority list:
Continue · Get Started · Skip · Later · Not Now · Got It · Maybe
Later · Done · Cancel). Solves the "agent typed into the welcome
sheet instead of the app" failure mode observed live with Reminders,
Mail, Photos and other Apple apps on first session-launch.

Generic — handles any app following the standard macOS modal
pattern. No-op when no modal is present, so safe to substitute for
plain `shell open -a` everywhere.

### Added — `~/.localkin/learned.md` cross-session memory

Persistent notebook the agent writes to after discovering an app's
AX schema quirks, working matchers, or workflow gotchas. Kernel
auto-loads it at boot (in `pkg/soul/soul.go`) and appends the
content to every soul's system prompt under a `## 已学到的` header.
Capped at 8KB (tail-preserved) to bound context usage on long-lived
notebooks.

This is the **persistence layer for Genesis Protocol** — every user's
KinClaw learns from its own operational history. Day 1 the notebook
is empty; week 4 it has notes for ~20 apps and the agent boots
already-knowing the schema quirks of every macOS app on this user's
Mac.

Pilot soul gets a new `## 你能裂变` section framing the loop as
identity (capability), not behavioral prescription:
- Successful multi-step on a new app → forge `<app>_<verb>` SKILL.md
- Learned quirks → append to learned.md
- First time opening unfamiliar app → use `app_open_clean` first

`pkg/soul/soul_test.go` adds three regression cases: notebook
injects when present, system prompt clean when missing, runaway
notebooks tail-truncate at 8KB.

### Added — vision passthrough for tool-result images

Until now KinClaw shipped a multimodal brain (Kimi K2.6, Claude Sonnet 4)
talking through a text-only adapter. Screenshots returned by the
`screen` skill were just file paths in the tool message — the model
had no way to see the pixels. Symptom: agent ran demos that involved
"look at the page", read AX descriptions, and confidently fabricated
specific values (prices, names, URLs) it had never actually seen.

The fix threads images end-to-end:

- `pkg/brain.Message` gets an `Images []string` field carrying paths
  to image files attached to that message.
- `pkg/brain/images.go` reads and base64-encodes PNG / JPG / GIF /
  WebP files for inlining.
- `openAIBrain.Chat` builds an OpenAI vision-style content array
  (`[{type:text}, {type:image_url, image_url:{url:data:image/png;base64,...}}, ...]`)
  when a message has images attached. Falls back to plain string
  content otherwise — preserves wire compatibility with strict
  OpenAI-compat servers (Ollama Cloud, Groq) that may reject array
  content in unexpected places.
- `claudeBrain.Chat` does the equivalent for Anthropic's API, putting
  image source blocks (`{type:image, source:{type:base64, ...}}`)
  inside the `tool_result` block alongside the text.
- `pkg/skill.ToolResult` gets `Images []string`. Skills opt in by
  emitting `image://<path>` marker lines in their text output;
  `extractImageMarkers` strips the markers and populates the list.
- `pkg/skill/screen.go` now emits an `image://<path>` line so its
  PNG output reaches vision-capable brains as actual pixels, not
  just a path string.
- `cmd/kinclaw/main.go` chatLoop copies `r.Images` into the
  `brain.Message.Images` field of the tool message it constructs.

Tests:
- `pkg/brain/images_test.go` — generates a 4×4 PNG fixture and
  verifies `imageToDataURL` / `imageToBase64` / `mimeForExtension`
  behavior including unsupported-extension and missing-file errors.
- `pkg/skill/skill_test.go::TestExtractImageMarkers` — table-tests
  marker scanning: no markers, single, multiple, dedup, indent
  trimming, marker-only.

The `Images []string` field is `json:"-"` on `brain.Message` — image
paths shouldn't be serialized into chat history (the bytes go on the
wire each round, but the path list is regenerated from tool results).

### Added — `web_search` SearXNG backend with DDG fallback

`pkg/skill/web_search.go` now supports routing through a local
SearXNG meta-search instance via the `$SEARXNG_ENDPOINT` env var.
DDG HTML scrape stays as the default and as a fallback when SearXNG
is unreachable. The result string includes `(via searxng)` /
`(via duckduckgo)` so the LLM and user can see which backend served
the query.

Why: the live DDG scrape is brittle (rate limits, occasional 200-with-
empty-results, structural HTML changes). For users running a local
SearXNG (e.g. on `localhost:8080`), routing through it gives:
- multi-engine aggregation (Google / Bing / Yahoo / Wikipedia /...)
- privacy (queries stay local, SearXNG proxies to engines)
- better reliability (less likely to hit a single-engine ratelimit)

Usage:
```bash
SEARXNG_ENDPOINT=http://localhost:8080 ./kinclaw -soul souls/pilot.soul.md
```

Soul YAML stays unchanged — keeping the configuration off-soul means
the same soul file works whether SearXNG is up, down, or absent.

`pkg/skill/web_search_test.go` adds three regression cases — happy
path JSON parse, non-200 surfaces clearly, and the env-var-driven
backend dispatch.

### Fixed — `record start` returned before kinrec captured first frame

Live observation across multiple v1.2 demo runs: the very first frame
of every recording showed Calculator already in its final "2" state.
The whole "open Calculator → click 1+1= → see 2" sequence happened
DURING kinrec's startup window and got missed entirely — viewers see
a finished calculator from frame 1, with no demo content.

Root cause: `record start` returned the `recording_id` as soon as
`kinrec.NewRecorder().Start()` returned, but kinrec's
ScreenCaptureKit pipeline takes another 200-500ms to actually deliver
frames. With the pilot prompt's parallel batch
(`record start + osascript activate + tts` in one tool_calls array),
osascript and tts goroutines completed in tens of milliseconds while
kinrec was still warming up. By the time kinrec captured its first
frame, the LLM was already in round 3+ doing click_sequence.

`pkg/skill/record.go`: after `r.Start(ctx)`, the skill now polls
`r.Stats().Frames` every 20ms with a 1-second cap. Returns success
once the first frame is observed (or, on timeout, anyway — better
to lose the first beat than to hang). The returned message includes
`first_frame_after: Xms` so the LLM can see the warmup actually
happened.

Pilot prompt: `record start` is now mandated as **its own LLM round**
(not parallel-batched with activate/tts). The 6-round demo flow
becomes 7 rounds, but the visible-from-frame-1 ordering is now
guaranteed:
- Round 1: `record start` alone (kernel blocks until first frame)
- Round 2: parallel `osascript activate <app>` + opening `tts`
- Round 3+: tree, click_sequence, closing tts, stop, report

Speed-rule 1 updated to highlight the exception ("but `record start`
must be alone"). Recordings will be ~1-2s longer than before, but
will actually show the demo from the start.

### Fixed — `{{var}}` substitution in shell payload self-defeated optional-param sentinels

When v1.2 added substitution to `Command` parts (so `weather`'s
`[curl, "https://wttr.in/{{location}}"]` would actually work), it
introduced a subtler bug: SKILL.md authors using `{{var}}` literally
inside a shell command as a "param-missing" sentinel
(`[ "$X" = "{{wait}}" ] && X=""`) had their checks self-defeat. When
the caller DID pass `wait=true`, the substitution rewrote BOTH the
arg AND the literal sentinel, so the comparison became
`[ "true" = "true" ]` → true → param value clobbered.

Live observation: `tts wait=true` was silently treated as
`wait=false`. The "closing tts that blocks 2-4s to give the result
frame time to render" recipe documented in the pilot prompt didn't
actually block — kinrec stopped the recording right after the last
button press and the audio cut mid-sentence. Same bug masked the
explicit `speaker=` parameter, falling back to the auto-detect path
silently.

`pkg/skill/external.go`: after named substitution, any leftover
`{{name}}` placeholder is regex-stripped to "". SKILL.md authors now
detect missing optional params with the cleaner `[ -n "$X" ]` idiom
instead of the broken sentinel pattern.

`skills/tts/SKILL.md` and `skills/stt/SKILL.md`: removed the
`[ "$X" = "{{name}}" ] && X=""` lines — no longer needed.

`pkg/skill/skill_test.go`: two new regression cases —
`TestLoadExternalSkill_UnpassedTemplateStripped` (kernel strips
unsubstituted placeholders to empty) and
`TestLoadExternalSkill_SentinelPatternNotSelfDefeating` (SKILL.md
using `[ -n "$X" ]` correctly distinguishes "passed as 'true'" from
"omitted").

Net effect on demo recordings: closing TTS now actually blocks for
its playback duration (~3s for "等于二"), giving kinrec time to
capture the result frame. Recordings will be a few seconds longer
than the ones produced under the bug, but that's the correct
behavior — the bug-version recordings sometimes truncated mid-
narration when kinrec's stop fired before afplay's background
process had a chance to flush.

### Changed — pilot prompt: explicit `fps=30` and TTS numeral preprocessing

Two demo-quality nits hardened in the pilot prompt after a clean
8.7s end-to-end run revealed them:

- **`record start` must pass `fps=30`** for demos. kinrec's default
  is conservative (~7 fps); recordings at that rate look choppy on
  release video — fine for headless verification but not shippable
  content. Speed-rules section gains rule 7 making fps=30 mandatory
  for demos.
- **Chinese TTS text must pre-render numerals + symbols as words**
  before calling `tts`. Kokoro's Chinese tokenizer has known
  ambiguities: `"1+1"` reads as "一亿" (one hundred million),
  `"10x"` reads as "十次", `"GPT-4"` reads as "G P T 四" only if
  spaced. Pilot prompt's speed-rule 8 now requires LLMs to rewrite:
  `"1+1"` → `"一加一"`, `"100%"` → `"百分之一百"`, etc. English
  speakers don't have this issue; rule scoped to CJK speakers only.

### Added — circuit breaker per-turn usage cap

A fourth trigger added to `pkg/skill/circuit.go`: any single skill
called `cbUsageMax` (8) or more times in one user turn fires the
breaker, regardless of whether each call succeeded or failed and
regardless of whether outputs differed.

Live observation: a v1.2 demo run where the LLM did
ui tree → click_sequence → ui find → ui read → ui click → ui tree
→ ui find → ui read → ui click ... bouncing between methods to
"fix" an ambiguous verification. Each individual call was
legitimate (no error, no identical-output streak), so triggers 1-3
didn't fire. By call 12 the LLM had ground for ~60 seconds without
making any actual progress.

The cap catches over-iteration directly. A healthy demo uses ui 3-4
times (tree + click_sequence + maybe one more). 8+ is the unmistakable
"stuck in a fix-and-retry loop" signal.

`circuit_test.go` adds two cases: trip at the 8th call to the same
skill, and counting failures + successes together.

### Changed — pilot demo flow drops the verification round

Live observation: the LLM repeatedly succeeded at rounds 1-3 (record
start + tree + click_sequence, all kernel-confirmed) then collapsed
in round 4 trying to "verify the result" — Calculator-style apps
have multiple AXStaticText elements (equation history strip + main
display + hint label), `ui read` picked the wrong one, the LLM
mis-interpreted the equation as the answer, decided clicks "didn't
work", went into clean-and-retry, and eventually lost the Calculator
process entirely.

Insight: **for demo recording, the recording is the verification.**
Asking the LLM to also verify-then-narrate just introduces a place
the agent can tie itself in knots interpreting ambiguous AX output.

The `## 录 demo 视频` section now codifies a 6-round demo flow with
**no in-flight verification**:

1. Parallel record start + activate + tts (1 round)
2. ui tree (1 round)
3. ui click_sequence — trust kernel return (1 round)
4. closing tts wait=true (1 round, doubles as render-pad)
5. record stop (1 round)
6. report path (1 round)

A separate "**何时才该验证**" section addresses non-demo tasks where
the LLM genuinely needs the result value (e.g., "open Calc, compute
1+1, tell me the answer"): single `ui find role=AXStaticText` returns
all matches with their values inline (kernel change from earlier),
LLM lists candidates rather than guessing which is "the result".

`ui read` is now flagged as wrong tool for verification when multiple
matches likely — it returns FindFirst, hiding the ambiguity.

Speed-rules section also gained a rule 7: **on [SYSTEM] circuit-
breaker warning, stop immediately** — don't retry, don't fallback,
report current state and finish.

### Fixed — `ui tree` and `ui find` hid AXValue, making result-verification expensive

Live observation of the second v1.2 demo run: rounds 1-3 stayed
fast, but round 4 (verify result) broke down — the LLM tried `ui
read role=AXStaticText` to read Calculator's display, hit the wrong
StaticText (Calculator has several), got nothing useful, and went
into a 1-minute "is `=` the right key? try `Enter`. try `return`."
loop without ever reading the actual displayed value.

Root cause same shape as the AXDescription miss: `dumpTree` and
`ui find` output never showed AXValue. Calculator's display is an
`AXStaticText` whose `Value()` returns the current number; without
that column, the LLM had to guess which StaticText was the display
and call `ui read` separately, often hitting the wrong one.

`pkg/skill/ui.go`:
- **`dumpTree` adds `value="..."`** — every element with a non-empty
  AXValue (and value ≠ title/desc) shows it inline. Status labels,
  text-field contents, slider positions, calculator displays all
  visible directly in tree output.
- **`ui find` output adds `value="..."`** — a single `find` call now
  doubles as a read for the matched elements. No separate round-trip.
- `truncateValue` caps any single value at 200 chars so a tree dump
  of a text editor doesn't blow context.

Pilot prompt updated:
- Round 2 default tree depth bumped from 3 → 6, with explicit
  guidance to retry at depth=8/10 if target buttons aren't visible
  (Calculator's number buttons are at depth 8).
- Round 4 verification rewritten — re-run `ui tree`, look at the
  `value=...` column for the changed display value. Single tool
  call. No `ui read`. No `screen`.
- Schema-discovery table now lists all five tree-output columns
  (role / title / desc / value / [id]) with concrete examples and
  flags `[_NS:n]`-style identifiers as auto-generated/unstable.

### Fixed — `ui tree` hid AXDescription, sending the agent down `input type`

Live observation of the v1.2 demo run: rounds 1-3 were ~3 seconds
total (the new speed protocol working), but round 4 collapsed into
~1m of retries. Diagnosis: the LLM correctly read `ui tree`, saw
that Calculator's number/operator buttons had empty titles and
auto-generated `[_NS:35]`-style identifiers, concluded "no usable
matcher", and fell back to `input action=type text="1+1="`.
`input type` requires focus on the target app; Calculator wasn't
focused (focus protection from Terminal-launched agents); keys
landed in Terminal; chaos.

Root cause was on **our** side: `dumpTree` only printed
role/title/identifier — it never showed AXDescription. Calculator's
buttons have empty titles but rich descriptions ("1", "Add",
"Equals"). The LLM made the right call given the tree it saw; the
tree just wasn't telling the truth.

Three changes in `pkg/skill/ui.go`:

1. **`dumpTree` now prints `desc="..."`** for every element whose
   AXDescription is non-empty and differs from the title. Tree
   output for Calculator now looks like:
   ```
   AXButton desc="1" [_NS:35]
   AXButton desc="Add" [_NS:36]
   AXButton desc="Equals" [_NS:39]
   ```
   The LLM can immediately see which matcher to use.

2. **`ui find` / `ui click` accept a `description="..."` param**.
   kinax-go v0.1 doesn't ship `MatchDescription`, but the Matcher
   type is just a func, so we plug in a tiny custom matcher that
   calls `e.Description()`.

3. **`ui click_sequence` accepts `descriptions="..."`** alongside
   the existing `titles=` and `identifiers=` modes. Same comma-
   separated grammar; same per-step ambiguity / destructive guards.

Pilot prompt updated to make `ui click*` the **blessed method** and
`input type` strictly conditional on focus being verified on the
target. The old "use input type if app accepts keyboard" guidance
removed — it was right in theory but wrong in practice for any
KinClaw running from a Terminal session.

### Added — `ui click_sequence` for fast multi-button flows

A new `ui` action that presses N elements in a single tool call,
saving the per-call LLM round-trip. Each round-trip with a cloud
brain is 1-3 seconds; for a "tap 1+1=" flow that's 4 individual
clicks → 4 rounds → 4-12 seconds of pure round-trip overhead with
nothing happening locally.

```
ui action=click_sequence bundle_id=com.apple.calculator titles="1,+,1,="
```

Or with stable AX identifiers (preferred when the app exposes them):

```
ui action=click_sequence bundle_id=com.apple.X identifiers="btn-save,btn-confirm"
```

Same safety guards as `click`: ambiguous match refuses unless
`force=true`; destructive-target check applies to each step. Aborts
mid-sequence on the first failure and reports which step / why,
returning a partial log of clicks that did succeed.

Generic by design — usable for calculator-like apps, dialpads, code
entry, sequential menu navigation, anywhere the agent needs to push
N buttons in order.

### Changed — pilot prompt rewritten for round-count optimization

Live observation: a 15-second target Calculator demo took **1m49s**
because the LLM did 30+ rounds of `ui find` + `screen screenshot` +
single `ui click` per button + verify-after-each-step. The kernel
work was milliseconds; the round-trips to the cloud brain were the
real cost.

New `## 录 demo 视频` section in `souls/pilot.soul.md` codifies a
**7-round upper-bound protocol** that's independent of which app is
being driven:

1. Round 1 batches `record start` + `osascript activate` + `tts
   wait=false` in **parallel tool calls** (kernel runs them
   concurrently via `ExecuteToolCalls`).
2. Round 2: `ui tree` once — never re-tree, the output is already
   in the conversation history.
3. Round 3: a single `ui click_sequence` for multi-button flows, OR
   `input type` for keyboard-driven apps (Calculator, text fields,
   most native apps).
4. Round 4: a single `ui read` for verification — never `screen`
   unless the value isn't in the AX tree.
5. Round 5: closing `tts wait=true` (which doubles as the GUI
   render-pad before stop).
6. Round 6: `record stop`.
7. Round 7: report the path back to the user.

Seven explicit speed rules in the prompt's "速度规则" subsection:
parallelize within rounds, never re-tree, prefer click_sequence over
individual click, prefer input type over button click when
applicable, ui read over screen, no per-step verification (only
final), tts wait=false except the closer.

Also tightened the discovery protocol's step 3 — verification
happens at **logical-action-chain** boundaries (one read after
click_sequence completes), not after every single button press.

### Added — circuit breaker no-progress trigger

The existing breaker tripped on consecutive errors, but didn't catch
the much subtler "successful but stuck" loop: same skill returning
the same successful output 3+ times in a row, signalling no actual
progress. Live observation: pilot calling `ui find title="+"` three
times getting "no elements matching" each time, no error, no
intervention.

`pkg/skill/circuit.go` adds a third trigger keyed on `skill name +
first 200 chars of output`. When three consecutive identical results
come back from the same skill, the breaker emits a `[SYSTEM]` hint
asking the LLM to replan, change the matcher, or ask the user.
Generic by design — works for any skill, any task, any app.

False-positive shape (same skill + same args legitimately repeated,
e.g. typing `1` three times) is acceptable: the breaker emits a hint,
not a hard block, so the LLM can ignore it when warranted.

`pkg/skill/circuit_test.go` adds 4 cases: same-output trip, different
output resets the streak, different skill resets the streak, error in
the middle resets the streak.

### Changed — pilot prompt rewritten as a generic GUI protocol

The original prompt accumulated app-specific advice ("Calculator's `+`
is in description, not title"). That doesn't generalize and makes the
pilot brittle when it encounters a new app. New section
`## 操作未知 GUI 的通用流程（适用于任何 app）` codifies a four-step
protocol that works regardless of which app the agent is driving:

1. **Discover the AX schema** with `ui tree` before assuming anything.
2. **Match by the right field** in priority `identifier > description
   > role+title > title alone` — and always inspect first.
3. **Verify each action with an observation** — a successful tool
   return is not the same as the GUI actually changing. `input type
   "1+1="` returning "typed 4 chars" only means CGEvent fired; it
   doesn't mean those keys landed on the target app.
4. **Pad the demo recording's tail** — `ui read` to verify, then a
   `tts wait=true` final line to give the result frame time to render
   into the recording, THEN `record stop`. GUI render lag is 50-300ms;
   stopping immediately after the input keystroke captures pre-result
   frames.

Drops all Calculator-specific (and any other app-specific) hints. The
protocol is the contract; the LLM applies it to whatever app the user
points it at.

### Changed — `tts` SKILL.md default switched to `wait=false`

The `wait=true` default made `tts` block its caller for the full
synthesis + playback duration (~3-8s for a typical sentence), which
during a `record` session burned recording time on dead air while the
agent waited to continue. New default: `wait=false` plays in the
background and returns immediately, so the agent keeps acting while
narration plays — recording captures both the audio and the actions.

The pilot prompt's demo recipe now uses `wait=false` for narration
calls and reserves `wait=true` for the **final** tts before
`record stop` (which doubles as a GUI-render-pad as it blocks 2-4s,
giving the result frame time to land in the recording).

### Fixed — chatLoop strands the conversation when it errors

When `chatLoop` returned an error (most often "too many tool call
rounds"), `handleUserMessage` printed the error and returned without
saving the partial tool history or any assistant response — leaving
the persisted conversation as `user → user → user → ...` with no
assistant turns between. The brain on the next user turn read those
back-to-back user messages as "the prior task isn't done, keep
going" and reran the same compound action, blowing the round budget
again. Live observation: typing "你好" right after a failed demo
hit the round limit immediately.

Fix in `cmd/kinclaw/main.go`:
- Persist the partial `toolHistory` even on error.
- Synthesize an explicit assistant abort note ("Turn aborted:
  <err>. Reply 'continue' to resume or rephrase to start fresh.")
  and store it. Conversation structure stays valid; the next user
  message sees a clean prior turn.

### Changed — round budget bumped 20 → 50

The 20-round cap was sized for kernel-only workflows. With v1.2's
compound demos (record start + tts + multi-step ui find/click/verify
loop + tts + record stop), 30+ rounds is normal even when nothing
goes wrong. Bumped to 50; the existing circuit breaker + the new
ambiguity guards catch genuine runaways earlier than the round cap
would anyway.

### Fixed — `ui click` ambiguity & destructive-target safety net

Kernel-layer hardening prompted by an early v1.2 demo run where the
pilot was supposed to drive Calculator + 1+1=2 but instead closed
Calculator's window and continued narrating to an empty desktop. Root
cause: the `ui click` action ran `FindFirst` and pressed whichever
element came first in AX-tree traversal, with no check for ambiguity
or for destructive targets. A broad matcher hit AXCloseButton + the
real target, the close button came first, the window was gone, and the
agent had no safety net.

Two guards added in `pkg/skill/ui.go`:

- **Ambiguity refusal.** `ui click` now uses `FindAll` and refuses
  with a listing of candidates when ≥2 elements match. The caller
  must add filters (identifier / role / parent) — or pass the new
  `force=true` parameter to explicitly opt into "click the first
  hit anyway".
- **Destructive-target refusal.** `ui click` refuses on
  AXCloseButton / AXMinimizeButton / AXFullScreenButton roles, and
  on titles matching word-boundary `Close|Quit|Exit|Log Out|Sign Out`
  (English) or substring `退出|关闭|注销|结束` (Chinese). Same
  `force=true` opt-out. Conservative bias on purpose: false-refuse
  is recoverable, false-press is not.

Both guards documented in the new `## GUI 操作硬约束` section of
`souls/pilot.soul.md`, which mandates `ui find` before every `ui
click`, post-action verification, and `sleep 1` after `shell open
-a` before further interaction.

`pkg/skill/ui_test.go` covers `isDestructiveTarget` with 27 cases
including the conservative false-positive ("Close Friends" → refused;
the LLM uses force=true if it really means it).

### Fixed — `{{var}}` substitution in external SKILL.md `command:`

- Previously, only the `args:` array was templated. Any SKILL.md that
  placed `{{var}}` directly inside a `command:` element (the pattern
  used by all four shipped forge'd examples — `git_commit`, `weather`,
  `summarize`, `translate`) leaked the literal `{{var}}` into the
  executed command and silently misbehaved (e.g. weather hit
  `https://wttr.in/{{location}}` and got nonsense back).
- `pkg/skill/external.go` now substitutes templates in **both**
  `Command` and `Args`. Backward-compatible: skills using only `args:`
  behave identically. The four shipped forge'd skills now actually
  work without a per-file edit.
- Strengthened `TestLoadExternalSkill_Execute` to assert both sides of
  the substitution; added focused `TestLoadExternalSkill_CommandSubstitution`
  as regression cover.

### Added — tests

- `pkg/skill/record_test.go` — input-validation surface of the record
  skill (permission gate, unknown action, stop/stats id requirements,
  empty list, display_id / fps validation, name + description
  invariants) plus `parseBoolParam` table-driven coverage. Actual
  capture path runs through kinrec and isn't unit-testable; integration
  tests live in kinrec itself.
- `pkg/skill/util_test.go` — `expandHome` table tests covering empty,
  bare `~`, `~/`, `~/path`, `~user` (left literal), absolute paths,
  embedded tildes.
- `pkg/soul/soul_test.go` — `TestParseSoul_FullFields` extended to
  cover all four claw permission bits including `record`. New
  `TestParseSoul_ClawPermissions` table test covers the all-off
  default, all-on case, single-bit case, and a "legacy soul without
  the new key" case to prove backward compatibility.

### Changed — internals

- Extracted `expandHome` from `pkg/skill/screen.go` (darwin-only) into
  `pkg/skill/util.go` (cross-platform) so any skill — darwin claw or
  cross-platform helper — can reuse it without an internal dependency
  cycle.

### Dependencies

- `github.com/LocalKinAI/kinrec` v0.1.0 — the video claw's dylib.
- LocalKin Service Audio (`:8001` Kokoro / `:8000` SenseVoice) — used
  by `tts` / `stt` skills, **optional**: pilot continues to function
  without them and falls back to `shell say` for narration when
  documented.

### Build

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./...` ✅ (all pre-existing tests + new claw / soul / util
  tests pass on darwin and linux cross-build)
- `GOOS=linux go build ./...` ✅ (non-darwin stubs intact)

---

## [1.1.0] - 2026-04-24

**The claws grow in.** `localkin` renamed to **KinClaw** and extended
with the three computer-use claws + the first fission primitive
(Soul Clone) + a `~` expansion fix and full-stack pilot souls. Same
minimal core (~2,300 lines of runtime) + ~1,500 lines of claw +
clone + upgrade.

*On the version number: this was originally shipped as 2.0.0 → 2.0.1
but Go's Semantic Import Versioning requires v2+ modules to carry a
`/v2` suffix in the import path. Since KinClaw 1.1 is purely additive
over localkin 1.0 (no breaking API changes), collapsing back to a
minor bump on the v1 line is the correct move. The v2.0.0 / v2.0.1
tags were deleted before anyone relied on them.*

### Rename

- Module path: `github.com/LocalKinAI/localkin` → `github.com/LocalKinAI/kinclaw`.
- Binary: `localkin` → `kinclaw`.
- CLI directory: `cmd/localkin/` → `cmd/kinclaw/` (git-mv, history preserved).
- Repo: `LocalKinAI/localkin` renamed on GitHub to `LocalKinAI/kinclaw`;
  old URL 301-redirects via GitHub, old imports still resolve through
  the module proxy.

### Added — the three claws

- **`screen` skill** (`pkg/skill/screen.go`) — wraps
  [sckit-go](https://github.com/LocalKinAI/sckit-go) (ScreenCaptureKit).
  Actions: `screenshot` (save PNG + return path), `list_displays`.
  Triggers the macOS Screen Recording TCC prompt on first use.
- **`input` skill** (`pkg/skill/input.go`) — wraps
  [input-go](https://github.com/LocalKinAI/input-go) (CGEvent).
  Actions: `move`, `click`, `type` (UTF-8), `hotkey`, `scroll`,
  `cursor`, `screen_size`. Triggers the Accessibility TCC prompt.
- **`ui` skill** (`pkg/skill/ui.go`) — wraps
  [kinax-go](https://github.com/LocalKinAI/kinax-go) (AXUIElement).
  Actions: `focused_app`, `tree`, `find`, `click`, `read`,
  `at_point`. This is the killer feature: clicking buttons by their
  **semantic title** instead of pixel coordinates. Shares
  Accessibility permission with `input`.
- Each claw has a `_other.go` no-op stub for non-darwin builds so
  Linux/Windows compiles still pass (skills return a clean
  "macOS-only" error).

### Added — Soul Clone (fission primitive #1)

- **`pkg/clone`** — the `Clone(parentPath, opts)` primitive:
  produces N copies of a soul file with optional per-clone
  frontmatter patches (`FrontmatterPatch func(i int, meta *soul.Meta)`).
  Verbatim byte-copy by default (cheapest, preserves comments);
  re-marshal via yaml.v3 when the caller wants structural divergence.
- 7 unit tests covering default naming, custom naming, verbatim
  preservation, frontmatter patching, custom destination dir, zero
  count, missing parent.

### Added — Soul schema

- `permissions.screen / input / ui` bits added to `pkg/soul`.
  Each gates its corresponding skill at registry build time; an
  LLM that asks to use a disallowed claw gets a structured
  permission-denied error.

### Added — souls

- **`souls/pilot.soul.md`** — Claude Sonnet 4.5 pilot. Full 10-skill
  stack (screen/input/ui/shell/file_read/file_write/file_edit/
  web_fetch/web_search/forge). Guardrails in system prompt (never
  type passwords, never send/commit without in-turn consent, never
  bypass "are you sure" dialogs, no sudo, no curl-pipe-sh, no
  writing to ~/.ssh ~/.aws ~/.config/gcloud). First-run ritual
  that verifies each claw + shell + lists existing forged skills.
- **`souls/pilot_kimi.soul.md`** — same guardrails + skill stack
  but running Kimi K2.6 via Ollama Cloud (`provider: ollama`,
  `model: kimi-k2.6:cloud`). Chinese-leaning style.

### Added — Makefile

- `make sign` — rebuild + sign with stable `com.localkinai.kinclaw`
  adhoc identifier. TCC grants (Screen Recording, Accessibility)
  key off the identifier, so a stable one means the macOS permission
  entry survives every rebuild.
- `make run` / `make run-claude` / `make tcc-reset` / `make clean`.

### Fixed

- **`~` / `~/...` in `output_dir`** was being treated as a literal
  directory name (Go's filepath package doesn't expand tildes — shells
  do). Added `expandHome` helper in `pkg/skill/screen.go`. Before this
  fix, screenshots from pilot souls landed in `./~/Library/...` under
  the kinclaw cwd instead of `$HOME/Library/Caches/kinclaw/pilot/`.
- Screenshot tool result is now formatted across three lines
  (`path:`, `dimensions:`, `display_id:`) so LLMs that summarize
  can't accidentally drop the file path.

### Dependencies

- `github.com/LocalKinAI/sckit-go` v0.1.0
- `github.com/LocalKinAI/input-go`  v0.1.0
- `github.com/LocalKinAI/kinax-go`  v0.1.0
- `github.com/ebitengine/purego`    v0.8.0 (transitive)

All four KinKit libraries are MIT and independent of this repo —
they can be used standalone outside KinClaw.

### Preserved intentionally

- **`~/.localkin/` config dir** — where `auth.json`, `readline_history`,
  `memory.db`, and user skills live. Renaming to `~/.kinclaw/` would
  strand existing 1.x users' auth tokens and history. The dir name is
  an implementation detail; the new identity is in the module path,
  binary name, and branding.

### Preserved from 1.0.0

Everything the `localkin` 1.0.0 ship had: soul parser, brain
adapter (Claude / OpenAI / Ollama / Groq / DeepSeek / any
OpenAI-compatible), 7 native skills (shell, file_read/write/edit,
web_fetch, web_search, forge), external SKILL.md plugins, SQLite
memory, Claude OAuth, REPL, /reload, /soul switching, circuit
breaker, shell safety blocklist, SSRF protection, env filtering.

### Build

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./...` ✅ (all pre-existing tests + new clone tests pass)

---

## [1.0.0] - 2026-03-13

### Added
- **`web_search` skill** — DuckDuckGo web search with zero configuration. No API key needed. Returns titles, URLs, and snippets. Gated on `permissions.network`.
- **Command history** — Up/Down arrows navigate previous commands in the REPL. Persisted to `~/.localkin/readline_history` across sessions. Consecutive dedup, 500 entry max.
- **Circuit breaker** — Detects runaway tool loops (3 consecutive or cumulative failures per skill) and forces the LLM to stop retrying. Saves API credits.
- **`/info` command** — Shows version, soul, model, skill count, history messages, and estimated token usage.
- **`/reload` command** — Hot-reload the current soul file and rebuild brain + skills without restarting.
- **`/soul` command** — List available souls (`/soul`) or switch mid-session (`/soul researcher`).
- **Boot message** — `boot.message` in soul YAML auto-sends a prompt on startup before the REPL.
- **`researcher.soul.md`** — Example soul optimized for web research tasks (search + fetch, no shell).

### Changed
- **Shell safety upgraded** — Regex-based blocklist replaces string matching. Catches obfuscated patterns (`bash -c`, `eval`, `rm  -rf  /`), data exfiltration (`curl | bash`, reverse shells), and sensitive path access (`.ssh/`, `.aws/`, `.env`).
- **Environment filtering** — Explicit key denylist (ANTHROPIC_API_KEY, OPENAI_API_KEY, GITHUB_TOKEN, AWS_SECRET_ACCESS_KEY, etc.) replaces heuristic pattern matching.
- **SSRF protection** — `isPrivateURL` rewritten with `net/url.Parse` for correctness. Now also blocks cloud metadata endpoints (169.254.169.254).
- **`/skills` command** — Now shows skill descriptions alongside names.
- **Version 1.0** — Stable API. Soul file format, skill interface, and CLI considered stable.

### Fixed
- **Private URL detection** — Previous string-slicing approach could misparse URLs with unusual schemes.

## [0.3.0] - 2026-03-10

### Added
- **`file_edit` skill** — Search-and-replace file editing. Requires exact unique match, prevents accidental overwrites.
- **API retry with backoff** — Both Claude and OpenAI brains retry on 429/5xx (3 attempts, 1-2s exponential backoff).
- **`shell_timeout` config** — `permissions.shell_timeout` in soul YAML overrides the default 30s timeout.
- **`pkg/` package structure** — 6 packages: `brain`, `skill`, `soul`, `memory`, `auth`, `cmd`.
- **107 unit tests** — Comprehensive coverage: soul parsing, brain factory, skill execution, memory, security.
- **Groq soul file** — Cloud-hosted Llama via Groq (free tier, OpenAI-compatible).

### Changed
- **Improved tool descriptions** — Guide LLMs to pick the right tool (e.g. "use file_edit instead of sed").
- **`examples/` → `souls/`** — Renamed to match the soul file convention.
- **Removed `x/net` dependency** — htmlToText rewritten with pure string processing.

### Fixed
- **Claude OAuth login** — Fixed missing `state` parameter, correct `scope`, JSON token exchange.
- **API key hint** — Claude provider now suggests `localkin -login` when no key is set.

## [0.1.0] - 2025-03-08

### Added
- Soul file parser with YAML frontmatter + Markdown body
- Dual LLM engine: Claude (Messages API) and OpenAI-compatible (GPT, Ollama, DeepSeek, Groq)
- 6 native skills: shell, file_read, file_write, web_fetch, memory, forge
- SKILL.md external plugin system with auto-discovery
- SQLite persistent memory (chat history + key-value store)
- CLI with interactive REPL and single-exec mode (`-exec`)
- Claude OAuth PKCE login (`-login`)
- Parallel tool execution with configurable round limits
- Permission gates: `shell` and `network` toggles as core safety controls
- Shell safety: command blocklist + pipe-to-interpreter detection + env var filtering
- Web fetch: SSRF protection + HTML-to-text + prompt injection defense
- Forge: runtime skill generation with auto-registration
- Raw-mode readline with full CJK/UTF-8 support
- 5 soul files (Claude, OpenAI, Ollama, DeepSeek, locked)
