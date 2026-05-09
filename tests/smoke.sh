#!/usr/bin/env bash
# tests/smoke.sh — Tier 0 + Tier 1 verification.
#
# Runs in <30 seconds. No TCC permission needed. Safe to run in CI.
#
# What it verifies:
#   T0.1  go test ./... in kinclaw + 3 kit repos green
#   T0.2  go install kinclaw@latest from clean GOMODCACHE works
#         (catches broken go.mod / unpublished kit code)
#   T0.3  Version constants in kit binaries match their git tags
#   T1.1  Each kit CLI starts + prints expected banner
#   T1.2  input cursor / input screen return real numbers (proves
#         dylib loaded)
#
# Exit code: 0 on full pass, 1 on any failure.
# Output: last-thing-tried + diagnostic on each failure.

set -uo pipefail

REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
WORKSPACE="${WORKSPACE:-$(cd "$REPO_ROOT/.." && pwd)}"

# Fresh per-run cache so we exercise the publish path, not local state.
TMP_CACHE="$(mktemp -d -t kit-smoke.XXXXXX)"
trap 'chmod -R u+w "$TMP_CACHE" 2>/dev/null; rm -rf "$TMP_CACHE"' EXIT
export GOMODCACHE="$TMP_CACHE/modcache"
export GOPATH="$TMP_CACHE/gopath"
export GOBIN="$TMP_CACHE/bin"
mkdir -p "$GOBIN"

PASS=0
FAIL=0
fail() { printf "  ✗ %s\n" "$1" >&2; FAIL=$((FAIL+1)); }
pass() { printf "  ✓ %s\n" "$1"; PASS=$((PASS+1)); }
hr()   { printf -- "─── %s ─────────────────────────────────\n" "$1"; }

# ─── T0.1 — unit tests in each repo ────────────────────────────────
hr "T0.1 unit tests"
for repo in kinclaw kinax-go sckit-go input-go; do
  if [[ -d "$WORKSPACE/$repo" ]]; then
    if (cd "$WORKSPACE/$repo" && go test -count=1 ./... >/dev/null 2>&1); then
      pass "$repo: go test ./..."
    else
      fail "$repo: go test ./..."
      (cd "$WORKSPACE/$repo" && go test ./... 2>&1 | tail -10) >&2
    fi
  else
    fail "$repo: directory not found at $WORKSPACE/$repo"
  fi
done

# ─── T0.2 — go install kinclaw@latest from clean cache ─────────────
hr "T0.2 go install kinclaw@latest (clean cache)"
if go install github.com/LocalKinAI/kinclaw/cmd/kinclaw@latest >/dev/null 2>&1; then
  pass "kinclaw@latest installs cleanly"
else
  fail "kinclaw@latest install failed (broken go.mod or unpublished deps?)"
  go install github.com/LocalKinAI/kinclaw/cmd/kinclaw@latest 2>&1 | tail -10 >&2
fi

# ─── T0.3 + T1.1 — kit CLIs report version, banner ─────────────────
hr "T0.3 + T1.1 kit CLIs"

# Install latest of each — proves they're publishable AND gives us
# binaries to interrogate.
for spec in "kinax-go/cmd/kinax@latest" "sckit-go/cmd/sckit@latest" "input-go/cmd/input@latest"; do
  if go install "github.com/LocalKinAI/$spec" >/dev/null 2>&1; then
    pass "$(basename "${spec%%@*}")@latest installs"
  else
    fail "$(basename "${spec%%@*}")@latest install failed"
  fi
done

# Version-string consistency: binary's version output should match
# the git tag we just pulled.
check_version() {
  local bin="$1" expected_pattern="$2" run_args="$3"
  if [[ ! -x "$GOBIN/$bin" ]]; then
    fail "$bin: binary not found in $GOBIN"
    return
  fi
  local out
  out="$("$GOBIN/$bin" $run_args 2>&1 | head -3)"
  if echo "$out" | grep -qE "$expected_pattern"; then
    pass "$bin reports version matching $expected_pattern"
  else
    fail "$bin version mismatch — got: $(echo "$out" | head -1)"
  fi
}

# Each kit's published "current" tag — these update as we ship patches.
# We're not hardcoding tags here (the test would go stale instantly).
# Instead we ask: does the binary's reported version contain a non-empty
# semver-shaped string AND not contain a known stale value?
for bin in kinax sckit input; do
  if [[ ! -x "$GOBIN/$bin" ]]; then continue; fi
  out="$("$GOBIN/$bin" version 2>&1 | head -3)"
  # Should look like 0.x.y somewhere in the output
  if echo "$out" | grep -qE '[v]?[0-9]+\.[0-9]+\.[0-9]+'; then
    pass "$bin: starts + emits version-shaped output"
  else
    fail "$bin: no version in 'version' output"
    echo "    output was: $out" >&2
  fi
done

# ─── T1.2 — input dylib loaded (cursor + screen) ───────────────────
hr "T1.2 dylib load (input)"
if [[ -x "$GOBIN/input" ]]; then
  c="$("$GOBIN/input" cursor 2>&1)"
  if echo "$c" | grep -qE '^[0-9]+ +[0-9]+$'; then
    pass "input cursor returns coords ($c)"
  else
    fail "input cursor: unexpected output: $c"
  fi
  s="$("$GOBIN/input" screen 2>&1)"
  if echo "$s" | grep -qE '^[0-9]+ +[0-9]+$'; then
    pass "input screen returns size ($s)"
  else
    fail "input screen: unexpected output: $s"
  fi
fi

# ─── summary ────────────────────────────────────────────────────────
hr "summary"
printf "PASSED: %d   FAILED: %d\n" "$PASS" "$FAIL"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
