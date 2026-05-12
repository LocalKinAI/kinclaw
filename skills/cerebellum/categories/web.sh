web_dispatch() {
  local ACTION="$1"; shift || true
  # Skill paths (absolute so cerebellum works from any cwd)
  local WEB_PY=/Users/jackysun/Documents/Workspace/kinclaw/skills/web/web.py
  local SCRAPER_PY=/Users/jackysun/Documents/Workspace/localkin/tools/scraper/scraper.py
  local BROWSER_PY=/Users/jackysun/Documents/Workspace/kinclaw/skills/browser_session/runner.py
  local SEARCH_PY=/Users/jackysun/Documents/Workspace/localkin/skills/web_search_ddg/run.py

  # Pythons. /usr/bin/python3 is the system one (good for stdlib-only
  # scripts like web_search_ddg). Scrapling needs the pyenv 3.10 where
  # it's actually installed. web.py (Playwright) lives in the kinclaw
  # browser_session venv.
  local PY_SYS=/usr/bin/python3
  local PY_SCRAPING=/Users/jackysun/.pyenv/versions/3.10.0/bin/python
  [ -x "$PY_SCRAPING" ] || PY_SCRAPING=/usr/bin/python3
  local PY_BROWSER=/Users/jackysun/Documents/Workspace/kinclaw/skills/browser_session/.venv/bin/python
  [ -x "$PY_BROWSER" ] || PY_BROWSER=/usr/bin/python3

  case "$ACTION" in

    fetch)
      # Simple HTTP GET (no JS rendering). Forwards to curl since
      # kinclaw's native web_fetch is in-process Go — not directly
      # shell-callable. curl matches its behavior (follow redirects,
      # accept gzip). Use this for static pages, JSON APIs, files.
      # Args: URL OUT_FILE
      require "url" "${1:-}"; require "out_file" "${2:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /usr/bin/curl -sSL --max-time 30 \
        -H 'User-Agent: Mozilla/5.0 (Macintosh) KinClaw/1.5' \
        "$1" -o "$2"
      local rc=$?
      if [ "$rc" -ne 0 ]; then
        echo "ERR: curl failed (rc=$rc) fetching $1" >&2
        exit "$rc"
      fi
      local sz; sz=$(/usr/bin/stat -f %z "$2" 2>/dev/null)
      echo "ok: fetched $1 -> $2 ($sz bytes)"
      ;;

    fetch_js)
      # Playwright-rendered fetch (JS executed, SPA-ready). Forwards
      # to skills/web/web.py.  Args: URL OUT_FILE [SELECTOR=body]
      require "url" "${1:-}"; require "out_file" "${2:-}"
      local sel="${3:-body}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /usr/bin/python3 "$WEB_PY" "$1" --selector "$sel" > "$2" 2>/dev/null
      local rc=$?
      if [ "$rc" -ne 0 ]; then
        echo "ERR: web.py (Playwright) failed (rc=$rc) for $1" >&2
        exit "$rc"
      fi
      local sz; sz=$(/usr/bin/stat -f %z "$2" 2>/dev/null)
      echo "ok: js-fetched $1 [$sel] -> $2 ($sz bytes)"
      ;;

    screenshot)
      # Playwright screenshot. Args: URL OUT_PNG [SELECTOR (clip box)]
      require "url" "${1:-}"; require "out_png" "${2:-}"
      local clip="${3:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      if [ -n "$clip" ]; then
        /usr/bin/python3 "$WEB_PY" "$1" --screenshot "$2" --screenshot-selector "$clip" >/dev/null 2>&1
      else
        /usr/bin/python3 "$WEB_PY" "$1" --screenshot "$2" >/dev/null 2>&1
      fi
      if [ -s "$2" ]; then
        local sz; sz=$(/usr/bin/stat -f %z "$2" 2>/dev/null)
        echo "ok: screenshot of $1 -> $2 ($sz bytes)"
      else
        echo "ERR: screenshot empty for $1" >&2; exit 1
      fi
      ;;

    js)
      # Evaluate JS in Playwright-rendered page. Args: URL CODE OUT_FILE
      # OUT receives the JSON-stringified return value.
      require "url" "${1:-}"; require "code" "${2:-}"; require "out_file" "${3:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$3")"
      /usr/bin/python3 "$WEB_PY" "$1" --js "$2" > "$3" 2>/dev/null
      local rc=$?
      if [ "$rc" -ne 0 ]; then
        echo "ERR: web.py --js failed (rc=$rc)" >&2; exit "$rc"
      fi
      echo "ok: js-eval on $1 -> $3"
      ;;

    search)
      # Multi-engine search via local SearXNG (Google/DDG/Brave/Startpage
      # aggregated). Output: JSON array of {title, url, snippet, engine}.
      # Args: QUERY OUT_FILE [MAX=10]
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local n="${3:-10}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      # web_search_ddg/run.py reads JSON {"input":"...","max_results":N} or plain string
      /usr/bin/python3 "$SEARCH_PY" \
        "{\"input\":\"$(printf '%s' "$1" | /usr/bin/sed 's/"/\\"/g')\",\"max_results\":$n}" \
        > "$2" 2>/dev/null
      local rc=$?
      if [ "$rc" -ne 0 ]; then
        echo "ERR: search via SearXNG failed (rc=$rc) — is localhost:8080 up?" >&2
        exit "$rc"
      fi
      local count; count=$(/usr/bin/grep -c '"title"' "$2" 2>/dev/null || echo 0)
      echo "ok: search '$1' -> $2 ($count results)"
      ;;

    scrape)
      # Anti-bot fetch via Scrapling (Cloudflare/Akamai/DataDome bypass +
      # TLS fingerprint). Args: URL OUT_FILE [SELECTOR (CSS)]
      require "url" "${1:-}"; require "out_file" "${2:-}"
      local sel="${3:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      if [ -n "$sel" ]; then
        "$PY_SCRAPING" "$SCRAPER_PY" fetch "$1" --css "$sel" --output "$2" 2>/dev/null
      else
        "$PY_SCRAPING" "$SCRAPER_PY" fetch "$1" --output "$2" 2>/dev/null
      fi
      local rc=$?
      if [ "$rc" -ne 0 ]; then
        echo "ERR: Scrapling fetch failed (rc=$rc) for $1" >&2; exit "$rc"
      fi
      local sz; sz=$(/usr/bin/stat -f %z "$2" 2>/dev/null)
      echo "ok: scraped $1 -> $2 ($sz bytes)"
      ;;

    download)
      # Raw file download via Scrapling (binaries / PDFs).
      # Args: URL OUT_FILE
      require "url" "${1:-}"; require "out_file" "${2:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      "$PY_SCRAPING" "$SCRAPER_PY" download "$1" --output "$2" 2>/dev/null
      local rc=$?
      if [ "$rc" -ne 0 ]; then
        echo "ERR: download failed (rc=$rc) for $1" >&2; exit "$rc"
      fi
      local sz; sz=$(/usr/bin/stat -f %z "$2" 2>/dev/null)
      echo "ok: downloaded $1 -> $2 ($sz bytes)"
      ;;

    session_run)
      # Multi-step browser automation via browser-use. Pass the high-
      # level task string; browser-use's LLM-driven Agent loop plans
      # + executes (10-20s cold start, multi-step). Note: this DOES
      # consume LLM tokens (that's intrinsic to browser-use) — use
      # only when fetch / scrape / fetch_js can't do the job.
      # Args: TASK_DESCRIPTION OUT_FILE
      require "task" "${1:-}"; require "out_file" "${2:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      "$PY_BROWSER" "$BROWSER_PY" "$1" > "$2" 2>&1
      local rc=$?
      if [ "$rc" -ne 0 ]; then
        echo "ERR: browser-use session failed (rc=$rc)" >&2; exit "$rc"
      fi
      echo "ok: session ran '$1' -> $2"
      ;;

    *)
      echo "ERR: unknown web action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: fetch fetch_js screenshot js search scrape download session_run" >&2
      exit 2
      ;;
  esac
}
