---
name: browser_session
description: |
  Multi-step browser automation via browser-use — the open-source
  framework for autonomous browser navigation (91K+ stars, MIT). Use
  INSTEAD of `web` when the task spans more than a single page fetch:

    - Login + navigate + extract (auth state preserved across steps)
    - 5+ page interaction sequences
    - DOM-numbered element targeting (the framework shows the agent
      which element index to click — far more reliable than guessing
      CSS selectors)
    - JS-heavy SPAs that need real interaction state, not just
      rendered HTML

  Cost: cold start ~10-20s (browser warmup + LLM init); per-step
  ~2-5s. **Don't use for one-shot fetches** — `web` is faster and
  cheaper for those. Use this when the task description naturally
  contains "log in", "navigate to", "fill the form", "find on page X
  then click", or any sentence with multiple verbs of interaction.

  First-time setup (one machine, ~5 min):
    cd skills/browser_session
    ./setup.sh

  This creates a per-skill venv at ./.venv/ and installs browser-use
  + Chromium. Subsequent calls use the venv directly — zero pollution
  of the system Python.

  LLM selection (env-driven, in order of preference):
    ANTHROPIC_API_KEY         → Claude
    OPENAI_API_KEY            → GPT-4o
    OLLAMA_BASE_URL=URL       → local Ollama via OpenAI-compat
  Optional: BROWSER_USE_MODEL=<name> to override the model.
  Optional: BROWSER_USE_HEADLESS=false to watch the browser run.
command:
  - sh
  - -c
  - |
    TASK="$1"
    [ -z "$TASK" ] && { echo "task required" >&2; exit 1; }

    # cwd is set to the skill dir by the kernel.
    if [ ! -x ./.venv/bin/python ]; then
      echo "browser_session: venv missing — first-time setup required:" >&2
      echo "  cd skills/browser_session && ./setup.sh" >&2
      exit 1
    fi

    if ! ./.venv/bin/python -c "import browser_use" 2>/dev/null; then
      echo "browser_session: venv exists but browser-use not installed —" >&2
      echo "  re-run: cd skills/browser_session && ./setup.sh" >&2
      exit 1
    fi

    # Sensible defaults so the skill works out of the box on a typical
    # KinClaw setup (local Ollama running, no external API keys
    # required). User-set env (ANTHROPIC_API_KEY etc.) still takes
    # precedence — runner.py picks Anthropic > OpenAI > Ollama.
    : "${OLLAMA_BASE_URL:=http://localhost:11434}"
    : "${BROWSER_USE_MODEL:=kimi-k2.6:cloud}"
    export OLLAMA_BASE_URL BROWSER_USE_MODEL

    ./.venv/bin/python ./runner.py "$TASK"
  - "_"
args:
  - "{{task}}"
schema:
  task:
    type: string
    description: |
      High-level natural language task description. browser-use's
      internal LLM agent will plan + execute the steps. Examples:

      "Open Hacker News, find the top story right now, return its
       title + URL + first paragraph of the article"

      "Search GitHub for repositories matching 'computer use agent',
       sorted by stars, return the top 5 with their star counts"

      "Go to weather.com for 94025, extract today's high/low and the
       7-day forecast as a markdown table"

      The task should be self-contained — browser-use can't ask
      followup questions, it just plans and acts.
    required: true
timeout: 600
---

# browser_session — multi-step browser automation

A `web` claw on steroids. While `web` is a one-shot Playwright
wrapper good for "fetch this page and read it", `browser_session`
hands the entire interaction loop to [browser-use](https://github.com/browser-use/browser-use)
— a framework that keeps a persistent browser, decorates the DOM
with numbered element IDs, and has its own LLM-driven planning loop
to execute multi-step tasks.

## When to use which

| Need | Tool | Why |
|---|---|---|
| Read one page's content | `web` | Cheap (~3s cold start), trivial |
| Run one JS expression | `web` | Same |
| Take one screenshot | `web` | Same |
| Click + type once | `web` | Same — `click=` + `type_text=` |
| Login + then do stuff | `browser_session` | Persistent session |
| 5+ page navigation | `browser_session` | No cold-start per step |
| Form + multi-page wizard | `browser_session` | Framework handles state |
| Anything where a CSS selector won't survive | `browser_session` | Element-index targeting |

## Why a super-skill

`browser_session` is the first member of LocalKin's **super-skill**
pattern: skills that wrap a battle-tested third-party OSS framework
(here, browser-use's 91K-star codebase) inside a thin SKILL.md
adapter. The kinclaw kernel doesn't care that there's an entire
LLM-driven planning agent inside; it just sees one tool call going
in, one result coming out.

This is "thin soul, fat skill" pushed to its useful extreme:

- We don't reinvent browser-use. We host it.
- Other LocalKin souls (researcher / marketer / curator) can also
  enable `browser_session` and get the same capability.
- If browser-use ships a new version, we re-run `setup.sh` — the
  SKILL.md interface stays the same.

## Setup details

`./setup.sh`:

1. Picks a Python ≥3.11 (homebrew, pyenv, or system) — browser-use
   doesn't support 3.10
2. Creates `./.venv/` (per-skill venv, isolated)
3. `pip install browser-use playwright`
4. `playwright install chromium` (~92 MB Chrome Headless Shell)
5. Verifies the imports work

The venv lives in `./.venv/`. **Add `.venv/` to .gitignore** so the
~500 MB of dependencies don't get committed.

## LLM selection

The framework drives a real LLM internally. Pick one with vision
support — browser-use feeds screenshots to the LLM for visual
reasoning. Recommended (in descending preference):

1. **`claude-sonnet-4.5`** via `ANTHROPIC_API_KEY` — current SOTA
   for browser-use's WebVoyager benchmark
2. **`gpt-4o`** via `OPENAI_API_KEY` — second-best, cheaper
3. **Local Ollama** with a vision model (e.g. `qwen2.5-vl:7b`) via
   `OLLAMA_BASE_URL=http://localhost:11434` — free but ~3-5x slower
   per step; good for long batch runs you don't want to pay for

Set `BROWSER_USE_MODEL=<name>` to override the default for the
selected provider.

## Returns

stdout: the agent's final answer (a string — could be markdown if
the task asked for structured output).

stderr: errors, warnings, the playwright/chromium boot noise.

Non-zero exit: agent failed or reported errors. The kernel will
surface this as a tool error to the calling soul.

## Costs

A typical 5-step task with Claude Sonnet:
- ~30-60 seconds wall-clock
- ~$0.05-0.15 in API tokens (browser-use sends DOM + screenshot at
  every step)

For dev/test use the local Ollama path; for production runs use
Anthropic.
