#!/usr/bin/env bash
# browser_session — first-time setup.
#
# Creates a per-skill venv at ./.venv/ and installs browser-use +
# its Chromium runtime. Idempotent: re-running upgrades pip and
# verifies playwright. The skill itself (runner.py) is invoked via
# .venv/bin/python so the system Python stays untouched.
#
# Why per-skill venv: browser-use pulls in ~50 transitive deps
# (playwright, lxml, pydantic, anthropic SDK, openai SDK, ...).
# Polluting the system Python or even a user-wide pip is rude;
# isolating to ./.venv/ means uninstalling = `rm -rf .venv/`.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Pick a Python ≥3.11 (browser-use requirement). pyenv default
# might be 3.10 or older; homebrew typically ships 3.13.
PY3=""
for cand in python3.13 python3.12 python3.11 /opt/homebrew/bin/python3.13 \
            /opt/homebrew/bin/python3.12 /opt/homebrew/bin/python3.11 \
            /usr/local/bin/python3.13 /usr/local/bin/python3.12; do
  if command -v "$cand" >/dev/null 2>&1; then
    PY3=$(command -v "$cand")
    break
  fi
done

if [ -z "$PY3" ]; then
  echo "browser_session: need Python 3.11+ — install via:" >&2
  echo "  brew install python@3.13   # macOS" >&2
  echo "  apt install python3.11     # Linux" >&2
  exit 1
fi

echo "── Python: $PY3 ($($PY3 --version))"

if [ ! -d .venv ]; then
  echo "── creating venv at $SCRIPT_DIR/.venv"
  "$PY3" -m venv .venv
fi

echo "── upgrading pip"
.venv/bin/pip install --quiet --upgrade pip

echo "── installing browser-use + playwright (~50 deps, takes a minute)"
# browser-use 0.12.x doesn't pull playwright as a hard dep — install
# both explicitly so we can be sure the chromium binary is reachable.
.venv/bin/pip install --quiet browser-use playwright

echo "── installing Chromium for Playwright (~92MB)"
.venv/bin/python -m playwright install chromium 2>&1 | tail -3

echo ""
echo "── verifying"
.venv/bin/python -c "import browser_use; print('browser_use', browser_use.__version__)"

echo ""
echo "✓ browser_session ready. The skill is now callable from any"
echo "  soul that lists 'browser_session' in permissions.skills.enable."
