# LocalKin

**The embodied AI microkernel. ~2300 lines of Go. Zero compromise.**

LocalKin is a minimal, self-evolving AI agent runtime. Define a soul, pick a brain, and let it build its own skills.

## Quick Start

```bash
go install github.com/LocalKinAI/localkin/cmd/localkin@latest

# Login with Claude (free tier)
localkin -login

# Run
localkin -soul souls/coder.soul.md
```

## Architecture

```
┌──────────────────────────────────────────┐
│              Soul (.soul.md)             │
│  YAML config + Markdown system prompt    │
├──────────────────────────────────────────┤
│               Brain (LLM)               │
│  Claude │ OpenAI │ Ollama │ Groq │ any  │
├──────────────────────────────────────────┤
│              Skills (Tools)              │
│  shell │ file_read/write/edit │ web_fetch│
│  web_search │ memory │ forge            │
│  + any SKILL.md plugin                  │
├──────────────────────────────────────────┤
│           Memory (SQLite)               │
│  Chat history + key-value store          │
└──────────────────────────────────────────┘
```

## Soul File

A `.soul.md` file is all you need. YAML frontmatter for config, Markdown body for personality:

```yaml
---
name: "Coder"
brain:
  provider: "claude"        # claude | openai | ollama
  model: "claude-sonnet-4-6"
permissions:
  shell: true               # can it run commands?
  network: true             # can it fetch URLs and search the web?
skills:
  enable: ["shell", "file_read", "file_write", "web_search"]
boot:
  message: "Check system status"  # optional: auto-execute on startup
---
# Coder
You are a senior software engineer. Write clean, working code.
```

## Features

**Dual LLM Engine** — Claude Messages API + OpenAI Chat Completions API. One interface, any provider. Ollama, DeepSeek, Groq — anything OpenAI-compatible works out of the box.

**9 Native Skills** — Shell execution (with regex safety blocklist), file read/write/edit, web search (DuckDuckGo, no API key needed), web fetch (with SSRF protection), persistent memory, and forge (self-evolution).

**Web Search** — `web_search` lets your agent find information on the internet. Zero configuration — powered by DuckDuckGo, no API key required.

**SKILL.md Plugins** — Drop a `SKILL.md` in `./skills/` or `~/.localkin/skills/` and it's instantly available. No code changes. No recompilation.

**Forge** — The agent can create new skills at runtime. It writes a `SKILL.md`, registers it, and starts using it — in the same conversation.

**Circuit Breaker** — Detects when a skill keeps failing (3 consecutive or cumulative) and forces the LLM to stop retrying. Saves your API credits from runaway tool loops.

**Command History** — Up/Down arrows navigate previous commands. History persists to `~/.localkin/readline_history` across sessions.

**Interactive REPL** — `/info` shows token stats, `/reload` hot-reloads your soul file, `/soul` switches agents mid-session. Full CJK character support.

**Boot Message** — Set `boot.message` in your soul to auto-execute a prompt on startup. Great for status checks and initialization.

**Permission Gates** — `shell: false` blocks shell + forge. `network: false` blocks web_fetch + web_search. `skills.enable` whitelists which tools the LLM can see.

**SQLite Memory** — Conversations persist across sessions. Key-value memory lets the agent remember facts long-term.

**Claude OAuth** — `localkin -login` opens your browser, authenticates via PKCE, and saves the token. Same flow as Claude Code.

## Usage

```bash
# Interactive REPL
localkin -soul souls/my-agent.soul.md

# Single command (pipe-friendly)
localkin -soul souls/my-agent.soul.md -exec "what files are in this directory?"

# Debug mode (shows tool calls)
localkin -soul souls/my-agent.soul.md -debug

# Login to Claude
localkin -login

# Show version
localkin -version
```

### REPL Commands

| Command | Action |
|---------|--------|
| `/quit` | Exit |
| `/skills` | List available skills with descriptions |
| `/clear` | Clear conversation history |
| `/info` | Show soul, model, skill count, and token usage |
| `/reload` | Hot-reload current soul file |
| `/soul` | List souls or switch: `/soul researcher` |
| `/help` | Show help |

## Souls

```bash
# Claude with full access
localkin -soul souls/coder.soul.md

# Web researcher (search + fetch, no shell)
localkin -soul souls/researcher.soul.md

# Locked down — read-only, no shell, no network
localkin -soul souls/locked.soul.md

# Local Ollama (zero cloud dependency)
localkin -soul souls/ollama.soul.md

# OpenAI GPT-4o
OPENAI_API_KEY=sk-xxx localkin -soul souls/openai.soul.md

# DeepSeek (cheap coding model)
DEEPSEEK_API_KEY=sk-xxx localkin -soul souls/deepseek.soul.md

# Groq Cloud (free Llama, blazing fast)
GROQ_API_KEY=gsk_xxx localkin -soul souls/groq.soul.md
```

## Creating Skills

Create `./skills/greet/SKILL.md`:

```yaml
---
name: greet
description: Greet someone by name
command: [echo, "Hello, {{name}}!"]
schema:
  name:
    type: string
    description: Name to greet
    required: true
---
```

The agent can now use the `greet` tool. Or better — let the agent forge its own skills with the `forge` tool.

## API Key Resolution

LocalKin checks for API keys in this order:

1. `brain.api_key` in the soul file (supports `$ENV_VAR` syntax)
2. Environment variable (`$ANTHROPIC_API_KEY` or `$OPENAI_API_KEY`)
3. OAuth token from `~/.localkin/auth.json` (Claude only, via `localkin -login`)

## Security

- **Regex shell blocklist**: Catches obfuscated patterns like `rm  -rf  /`, `bash -c`, `eval`, reverse shells (`nc -e`, `/dev/tcp/`)
- **Pipe detection**: `curl ... | bash`, `| python`, `| sh` patterns are blocked
- **Sensitive path protection**: `.ssh/`, `.aws/`, `.env`, `.bashrc` access is blocked
- **Env filtering**: Known API keys (ANTHROPIC, OPENAI, AWS, GitHub) are stripped from subprocess environments
- **SSRF protection**: Private IPs, localhost, and cloud metadata endpoints are blocked in web_fetch
- **Prompt injection defense**: Web content is wrapped in `UNTRUSTED WEB CONTENT` markers
- **Circuit breaker**: Prevents runaway tool loops from burning API credits
- **Permission gates**: Shell and network access are opt-in per soul file

## Project Stats

| | |
|---|---|
| **Source code** | ~2300 lines (non-test Go) |
| **Tests** | ~140 tests across 6 packages |
| **Dependencies** | yaml, sqlite, term |
| **Binary size** | ~15 MB |
| **Packages** | 6 (brain, skill, soul, memory, auth, cmd) |

## License

Apache 2.0 — see [LICENSE](LICENSE).
