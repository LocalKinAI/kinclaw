#!/usr/bin/env bash
# kinclaw warmup — pre-flight check before benchmark / demo / research session.
#
# Verifies that kinclaw is in a runnable state. Doesn't fix problems
# automatically (except rebuild + re-sign) — it tells you what's wrong
# and where to fix it.
#
# Run before:
#   - macbench runs (to make sure agent is healthy before testing)
#   - public demos / video recording
#   - long research sessions
#   - after a major kit-debt rebase
#
# Six checks, in order:
#   [1/6] binary fresh + signed with stable TCC identity
#   [2/6] codesign identifier matches what TCC remembers
#   [3/6] Accessibility TCC (via kinclaw probe Finder)
#   [4/6] Screen Recording TCC (via screen claw list)
#   [5/6] brain reachable (simple "respond hello" round-trip)
#   [6/6] required kits (kinax/sckit/input/kinrec) on PATH or in
#         ~/.localkin/bin

set -uo pipefail

cd "$(dirname "$0")/.." || exit 1

OK="✓"
WARN="⚠"
FAIL="✗"
fail=0

BIN="./kinclaw"
IDENT="com.localkinai.kinclaw"

# ─── 1. Build fresh + sign ───────────────────────────────────────────────────
echo "[1/6] build + sign…"
if make sign >/tmp/kinclaw-warmup-build.log 2>&1; then
  echo "  $OK kinclaw built + signed"
else
  echo "  $FAIL build failed — see /tmp/kinclaw-warmup-build.log"
  fail=$((fail+1))
fi

# ─── 2. Codesign identity ────────────────────────────────────────────────────
echo "[2/6] codesign identity…"
# codesign -dv writes lines like "Identifier=com.localkinai.kinclaw" to stderr
# alongside other =-containing lines (TeamIdentifier, Signature, etc.).
# Filter strictly to the Identifier= line, then split on the first =.
ACTUAL_IDENT="$(codesign -dv "$BIN" 2>&1 | grep '^Identifier=' | head -1 | sed 's/^Identifier=//')"
if [[ "$ACTUAL_IDENT" == "$IDENT" ]]; then
  echo "  $OK identifier = $IDENT (TCC grants are sticky)"
else
  echo "  $FAIL identifier = '$ACTUAL_IDENT' (expected $IDENT) — TCC won't remember grants"
  echo "       fix: make sign"
  fail=$((fail+1))
fi

# ─── 3. Accessibility TCC ────────────────────────────────────────────────────
# kinclaw probe takes a bundle ID, not a display name.
echo "[3/6] Accessibility TCC (via kinclaw probe com.apple.finder)…"
PROBE_OUT="$(timeout 10 "$BIN" probe com.apple.finder 2>&1 || true)"
if echo "$PROBE_OUT" | grep -q "AX\|role\|title" 2>/dev/null; then
  echo "  $OK Accessibility granted"
elif echo "$PROBE_OUT" | grep -qi "not trusted\|denied\|permission"; then
  echo "  $FAIL Accessibility DENIED"
  echo "       fix: System Settings → Privacy & Security → Accessibility → toggle kinclaw on"
  fail=$((fail+1))
else
  echo "  $WARN Accessibility uncertain — probe didn't return expected structure:"
  echo "       $(echo "$PROBE_OUT" | head -2 | tr '\n' ' ')"
fi

# ─── 4. Screen Recording TCC ─────────────────────────────────────────────────
# kinclaw inherits sckit-go's TCC heuristic. We probe by asking for the screen
# claw skill via -exec with a short timeout.
echo "[4/6] Screen Recording TCC (via screen claw)…"
SR_OUT="$(timeout 15 "$BIN" -soul souls/macbench.soul.md -exec "List the displays connected to this Mac. Use the screen claw. Just give the count, no commentary." 2>&1 | tail -20 || true)"
if echo "$SR_OUT" | grep -qiE 'display|screen|monitor|^\s*[0-9]+\s*$'; then
  echo "  $OK Screen Recording grants the screen claw"
elif echo "$SR_OUT" | grep -qiE 'permission|denied|not authorized'; then
  echo "  $FAIL Screen Recording DENIED"
  echo "       fix: System Settings → Privacy & Security → Screen Recording → toggle kinclaw on"
  fail=$((fail+1))
else
  echo "  $WARN Screen Recording uncertain — agent didn't surface display info"
fi

# ─── 5. Brain reachable ──────────────────────────────────────────────────────
echo "[5/6] brain reachable (simple round-trip)…"
BRAIN_OUT="$(timeout 30 "$BIN" -soul souls/macbench.soul.md -exec "Respond with exactly the word 'hello' and nothing else." 2>&1 | tail -10 || true)"
if echo "$BRAIN_OUT" | grep -qiE 'hello|hi'; then
  echo "  $OK brain online ($(grep -A2 '^brain:' souls/macbench.soul.md | grep model: | tr -d ' ' | cut -d: -f2-))"
elif echo "$BRAIN_OUT" | grep -qiE 'connection|timeout|refused|offline|api error|unauthorized'; then
  echo "  $FAIL brain unreachable"
  echo "       check: brain provider running? api key set? network up?"
  fail=$((fail+1))
else
  echo "  $WARN brain responded oddly:"
  echo "       $(echo "$BRAIN_OUT" | head -3 | tr '\n' ' ' | head -c 200)"
fi

# ─── 6. Sibling kits ─────────────────────────────────────────────────────────
echo "[6/6] sibling kits…"
for kit in kinax sckit input kinrec; do
  if command -v "$kit" >/dev/null 2>&1; then
    echo "  $OK $kit ($(command -v $kit))"
  elif [[ -x "$HOME/.localkin/bin/$kit" ]]; then
    echo "  $OK $kit ($HOME/.localkin/bin/$kit — not on PATH but findable)"
  else
    echo "  $WARN $kit not on PATH (some bench tasks may need it via kinclaw shell skill)"
  fi
done

# ─── summary ─────────────────────────────────────────────────────────────────
echo ""
if [[ "$fail" -eq 0 ]]; then
  echo "ready — kinclaw is healthy. you can run 'make bench' or whatever you had planned."
else
  echo "$fail check(s) failed. fix above before benching, otherwise scores will be misleading."
  exit 1
fi
