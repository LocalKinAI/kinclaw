# Changelog

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
