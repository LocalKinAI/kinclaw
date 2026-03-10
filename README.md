# LocalKin

**The embodied AI microkernel. 2000 lines of Go. Zero compromise.**

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
│  Claude │ OpenAI │ Ollama │ DeepSeek    │
├──────────────────────────────────────────┤
│              Skills (Tools)              │
│  shell │ file_read │ file_write │ file_edit│
│  web_fetch │ memory │ forge             │
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
  network: true             # can it fetch URLs?
skills:
  enable: ["shell", "file_read", "file_write"]
---
# Coder
You are a senior software engineer. Write clean, working code.
```

## Features

**Dual LLM Engine** — Claude Messages API + OpenAI Chat Completions API. One interface, any provider. Ollama, DeepSeek, Groq — anything OpenAI-compatible works out of the box.

**7 Native Skills** — Shell execution (with safety blocklist), file read/write/edit, web fetch (with SSRF protection), persistent memory, and forge (self-evolution).

**SKILL.md Plugins** — Drop a `SKILL.md` in `./skills/` or `~/.localkin/skills/` and it's instantly available. No code changes. No recompilation.

**Forge** — The agent can create new skills at runtime. It writes a `SKILL.md`, registers it, and starts using it — in the same conversation.

**Permission Gates** — `shell: false` blocks shell + forge. `network: false` blocks web_fetch. `skills.enable` whitelists which tools the LLM can see.

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
| `/skills` | List available skills |
| `/clear` | Clear conversation history |
| `/help` | Show help |

## Souls

```bash
# Claude with full access
localkin -soul souls/coder.soul.md

# Locked down — read-only, no shell, no network
localkin -soul souls/locked.soul.md

# Local Ollama (zero cloud dependency)
localkin -soul souls/ollama.soul.md

# OpenAI GPT-4o
OPENAI_API_KEY=sk-xxx localkin -soul souls/openai.soul.md

# DeepSeek (cheap coding model)
DEEPSEEK_API_KEY=sk-xxx localkin -soul souls/deepseek.soul.md
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

- **Shell blocklist**: `rm -rf /`, `mkfs.`, `shutdown`, etc. are always blocked
- **Pipe detection**: `curl ... | bash` patterns are caught and rejected
- **Env filtering**: API keys, secrets, and tokens are stripped from subprocess environments
- **SSRF protection**: Private IPs and localhost are blocked in web_fetch (including DNS rebinding)
- **Prompt injection defense**: Web content is wrapped in `UNTRUSTED WEB CONTENT` markers with instructions to never execute it
- **Permission gates**: Shell and network access are opt-in per soul file

## Project Stats

| | |
|---|---|
| **Source code** | 2000 lines (non-test Go) |
| **Tests** | 1790 lines, 107 tests |
| **Dependencies** | yaml, sqlite, term |
| **Binary size** | ~15 MB |
| **Packages** | 6 (brain, skill, soul, memory, auth, cmd) |

## License

Apache 2.0 — see [LICENSE](LICENSE).
