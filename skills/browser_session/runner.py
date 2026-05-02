#!/usr/bin/env python3
"""
browser_session — kinclaw skill runner.

Wraps browser-use's Agent for multi-step browser automation. The
high-level task description is passed verbatim to browser-use; its
internal LLM-driven loop plans + executes the steps.

LLM selection (in order):
  1. ANTHROPIC_API_KEY → ChatAnthropic (BROWSER_USE_MODEL or claude-sonnet-4-5)
  2. OPENAI_API_KEY → ChatOpenAI (BROWSER_USE_MODEL or gpt-4o)
  3. OLLAMA_BASE_URL → ChatOpenAI compat (BROWSER_USE_MODEL or kimi-k2.5:cloud)

Output: prints the final result string to stdout. Errors go to
stderr with non-zero exit so the kernel reports them as tool errors.
"""
from __future__ import annotations

import asyncio
import os
import sys
import traceback


def pick_llm():
    """Return a browser-use chat-LLM instance based on env, or None."""
    anth_key = os.getenv("ANTHROPIC_API_KEY")
    if anth_key:
        from browser_use.llm import ChatAnthropic

        model = os.getenv("BROWSER_USE_MODEL", "claude-sonnet-4-5")
        kwargs = {"model": model}
        # Respect ANTHROPIC_BASE_URL so users behind a proxy / internal
        # router (e.g. litellm) can point browser-use at the same
        # endpoint they already use elsewhere.
        base = os.getenv("ANTHROPIC_BASE_URL")
        if base:
            kwargs["base_url"] = base
        return ChatAnthropic(**kwargs)

    openai_key = os.getenv("OPENAI_API_KEY")
    if openai_key:
        from browser_use.llm import ChatOpenAI

        model = os.getenv("BROWSER_USE_MODEL", "gpt-4o")
        kwargs = {"model": model}
        base = os.getenv("OPENAI_BASE_URL")
        if base:
            kwargs["base_url"] = base
        return ChatOpenAI(**kwargs)

    # Local Ollama via OpenAI-compat — needs base_url + a non-empty
    # api_key string (Ollama ignores its value but the SDK refuses
    # empty). The user passes their model name via BROWSER_USE_MODEL.
    ollama_base = os.getenv("OLLAMA_BASE_URL") or os.getenv("OLLAMA_HOST")
    if ollama_base:
        from browser_use.llm import ChatOpenAI

        model = os.getenv("BROWSER_USE_MODEL", "kimi-k2.5:cloud")
        return ChatOpenAI(
            model=model,
            base_url=f"{ollama_base.rstrip('/')}/v1",
            api_key="ollama",
        )

    return None


async def run_task(task: str) -> int:
    try:
        from browser_use import Agent
    except ImportError as e:
        print(
            f"browser-use not importable in this venv: {e}\n"
            f"  Run: cd $(dirname '{__file__}') && ./setup.sh",
            file=sys.stderr,
        )
        return 1

    llm = pick_llm()
    if llm is None:
        print(
            "browser_session: no LLM configured. Set one of:\n"
            "  ANTHROPIC_API_KEY      (Claude)\n"
            "  OPENAI_API_KEY         (GPT-4o)\n"
            "  OLLAMA_BASE_URL=...    (local Ollama via OpenAI-compat)\n"
            "Optional: BROWSER_USE_MODEL=<model-name> to override default.",
            file=sys.stderr,
        )
        return 1

    # Headless by default — we don't need to see the browser. Set
    # BROWSER_USE_HEADLESS=false to debug interactively.
    headless = os.getenv("BROWSER_USE_HEADLESS", "true").lower() != "false"

    try:
        agent = Agent(
            task=task,
            llm=llm,
            # browser-use 0.12.x: pass headless via browser_profile
            # would be the new way; for now Agent's defaults are fine,
            # and the kernel-level timeout (600s) caps any runaway.
        )
    except Exception as e:
        print(f"browser_session: failed to build Agent: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        return 1

    try:
        history = await agent.run()
    except KeyboardInterrupt:
        print("browser_session: interrupted", file=sys.stderr)
        return 130
    except Exception as e:
        print(f"browser_session: agent.run failed: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        return 1

    # Pull a clean string out of the AgentHistoryList. final_result()
    # gives the last "done" action's content; if the agent didn't
    # finish cleanly, fall back to the most recent extracted_content.
    final = None
    try:
        if hasattr(history, "final_result"):
            final = history.final_result()
        if not final and hasattr(history, "extracted_content"):
            extracted = history.extracted_content()
            if extracted:
                final = extracted[-1] if isinstance(extracted, list) else str(extracted)
    except Exception:
        pass

    if not final:
        final = "(browser_session completed but extracted no explicit result)"

    print(final)

    # browser-use's `has_errors()` is too sensitive — it flags ANY
    # transient mid-stream issue (LLM call timeouts, slow page loads
    # that the agent retried and recovered from). The user's task
    # may have completed perfectly but we'd still exit 1 and the
    # kinclaw kernel would report "tool error" — confusing.
    #
    # Use `is_successful()` instead: True iff the agent reached a
    # `done` action with success=True. That's the actual outcome
    # signal we care about. Fall back to "we got a result" if the
    # method isn't on this version of AgentHistoryList.
    try:
        if hasattr(history, "is_successful") and callable(history.is_successful):
            success = bool(history.is_successful())
        elif hasattr(history, "is_done") and callable(history.is_done):
            success = bool(history.is_done())
        else:
            success = bool(final and final != "(browser_session completed but extracted no explicit result)")
    except Exception:
        success = bool(final)

    if not success:
        print(
            "(browser_session: agent did not reach a successful 'done' state; "
            "see stderr above for details)",
            file=sys.stderr,
        )
        return 1

    return 0


def main():
    if len(sys.argv) < 2 or not sys.argv[1].strip():
        print("usage: runner.py <task>", file=sys.stderr)
        sys.exit(2)
    task = sys.argv[1]
    rc = asyncio.run(run_task(task))
    sys.exit(rc)


if __name__ == "__main__":
    main()
