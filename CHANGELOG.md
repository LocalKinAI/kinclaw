# Changelog

## [0.2.0] - 2026-03-10

### Added
- **`file_edit` skill** — Search-and-replace file editing. Requires exact unique match, prevents accidental overwrites. Read a file before editing.
- **API retry with backoff** — Both Claude and OpenAI brains retry on 429/5xx (3 attempts, 1-2s exponential backoff). No more crashes on rate limits.
- **`shell_timeout` config** — `permissions.shell_timeout` in soul YAML overrides the default 30s timeout for long-running commands.
- **`pkg/` package structure** — Reorganized from flat `package localkin` into 6 packages: `brain`, `skill`, `soul`, `memory`, `auth`, `cmd`. Clean dependency graph, proper Go project layout.
- **88 unit tests** — Comprehensive test coverage across all packages: soul parsing, brain factory, skill execution, memory persistence, security blocklists, SSRF protection.

### Changed
- **Improved tool descriptions** — shell, file_read, file_write descriptions now guide LLMs to pick the right tool (e.g. "use file_edit instead of sed", "read before editing").
- **`examples/` → `souls/`** — Renamed example directory to match the soul file convention.

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
