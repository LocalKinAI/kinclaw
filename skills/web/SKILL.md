---
name: web
description: |
  Universal web tool — fetches and interacts with web pages through
  real Chromium (Playwright). Renders JavaScript, follows redirects,
  handles cookies, bypasses many anti-scrape stubs that block plain
  `web_fetch`. Each call launches a fresh browser, executes the
  requested flow, closes. ~2-3s cold start; no persistent state.

  Use INSTEAD of `web_fetch` when:
    - Target is a JS-heavy SPA (prices via XHR, lazy content)
    - `web_fetch` returned an anti-scrape placeholder (Zhihu's
      "click here" stub, JD's chrome-only HTML)
    - You need to interact (click, fill input) before extracting

  Common patterns:
    fetch rendered text:           url=X
    wait for content then read:    url=X wait_for=".price"
    extract specific element:      url=X selector=".product-list"
    click and read result:         url=X click=".search-button"
    fill form, then read:          url=X click="input[name=q]" type_text="kinclaw"
    screenshot the page:           url=X screenshot=true (returns image:// marker)
    run JS and get result:         url=X js="document.title"

  First-time setup (one-time per machine, ~500MB Chromium download):
    pip install playwright
    playwright install chromium
command:
  - sh
  - -c
  - |
    URL="$1"
    WAIT_FOR="$2"
    SELECTOR="$3"
    CLICK="$4"
    TYPE_TEXT="$5"
    SCREENSHOT="$6"
    JS_CODE="$7"
    TIMEOUT="$8"

    [ -z "$URL" ] && { echo "url required" >&2; exit 1; }

    # Build args array — only include flags that have values.
    set -- "$URL"
    [ -n "$WAIT_FOR" ]   && set -- "$@" --wait-for "$WAIT_FOR"
    [ -n "$SELECTOR" ]   && set -- "$@" --selector "$SELECTOR"
    [ -n "$CLICK" ]      && set -- "$@" --click "$CLICK"
    [ -n "$TYPE_TEXT" ]  && set -- "$@" --type-text "$TYPE_TEXT"
    [ -n "$JS_CODE" ]    && set -- "$@" --js "$JS_CODE"
    [ -n "$TIMEOUT" ]    && set -- "$@" --timeout-ms "$TIMEOUT"

    # Screenshot mode: generate output path under cache dir, pass it in.
    if [ "$SCREENSHOT" = "true" ]; then
      OUT="$HOME/Library/Caches/kinclaw/web/web-$(date +%Y%m%d-%H%M%S-%N).png"
      mkdir -p "$(dirname "$OUT")"
      set -- "$@" --screenshot "$OUT"
    fi

    # Fail fast if playwright isn't installed — clear hint instead of
    # cryptic Python ModuleNotFoundError reaching the agent.
    if ! python3 -c "import playwright" 2>/dev/null; then
      echo "playwright not installed — first-time setup:" >&2
      echo "  pip install playwright" >&2
      echo "  playwright install chromium" >&2
      exit 1
    fi

    # cwd is already the skill directory (set by kinclaw), so a plain
    # relative path is reliable regardless of how SKILL_DIR is resolved.
    python3 ./web.py "$@"
  - "_"
args:
  - "{{url}}"
  - "{{wait_for}}"
  - "{{selector}}"
  - "{{click}}"
  - "{{type_text}}"
  - "{{screenshot}}"
  - "{{js}}"
  - "{{timeout_ms}}"
schema:
  url:
    type: string
    description: Target URL to load. Required.
    required: true
  wait_for:
    type: string
    description: Optional CSS selector to await visibility before extracting (e.g. ".price", "#main"). Useful for SPAs where content arrives via XHR.
  selector:
    type: string
    description: Optional CSS selector for the element whose inner text to return. Default body (full page text).
  click:
    type: string
    description: Optional CSS selector to click after load, before extracting. Combined with type_text, fills an input.
  type_text:
    type: string
    description: Text to fill into the click target (only used with click=). Useful for search boxes.
  screenshot:
    type: string
    description: Set to "true" to return a PNG of the viewport (with image:// marker for vision-capable brains). When true, no text is returned.
  js:
    type: string
    description: JavaScript expression to evaluate in the page context. Result is JSON-stringified. When set, no other extraction happens.
  timeout_ms:
    type: integer
    description: Max ms for goto and wait operations. Default 15000.
timeout: 60
---

# web — universal Playwright-backed page tool

Single skill, single Python script (`web.py`), single Chromium per
call. Drop-in replacement for `web_fetch` when JS rendering / dynamic
content / minor interaction is needed.

## Why one cold start per call is fine

Most web tasks the agent runs are **one-shot**: "fetch this page",
"click this and read the result", "screenshot this URL". Cold start
~2s amortizes over a single useful task. Multi-step flows where state
needs to persist (login → navigate → fill form → submit → read) are
rare in agent usage; if you need them, write the whole flow as one
`web` call by chaining `click=...` `type_text=...` `wait_for=...`.

## Architecture

```
agent → kinclaw shell skill → python3 web.py <args>
                              → Playwright launches Chromium
                              → executes flow
                              → closes Chromium
                              → prints result
                              → exits
```

No sidecar, no port management, no lifecycle skill. Just `pip install
playwright && playwright install chromium` once per machine, then
`web` works whenever the agent needs it.

## What it doesn't do

- Persistent session (cookies, login state) across calls — each call
  is a fresh browser. For real-account tasks, drive the user's actual
  Safari via `osascript activate Safari` + `ui` skill.
- Anti-bot bypass on Cloudflare / DataDome / advanced fingerprinting
  — Playwright defaults are detectable. Closed-source `web_scrape`
  via `WEB_ENDPOINT` env is the answer when this matters.
- Sub-agent autonomy — this skill executes literal instructions, it
  doesn't plan multi-step research. For "find me cheapest X" the
  agent itself drives the loop (web_search → pick URLs → web fetch
  each → synthesize).
