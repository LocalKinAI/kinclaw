#!/usr/bin/env bash
# tests/tier3.sh — drive a real app via kinclaw -exec to verify the
# v1.14.2 kit-bridge verbs.
#
# Requires:
#   - kinclaw binary at ./kinclaw (run `make cli` first)
#   - kinclaw has TCC: Accessibility + Screen Recording granted
#   - Safari + TextEdit installed (they ship with macOS)
#
# This is interactive — it'll open apps and move things on your
# screen. Don't run during a demo.
#
# Each test prints what it ran + what it observed; you decide PASS/FAIL.
# Exit code: 0 if every command returned 0, but you should still
# eyeball the screen behavior because exit 0 doesn't mean the verb
# did the right thing.

set -uo pipefail

REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
KINCLAW="$REPO_ROOT/kinclaw"

if [[ ! -x "$KINCLAW" ]]; then
  echo "✗ $KINCLAW not found. Run 'make cli' in $REPO_ROOT first." >&2
  exit 2
fi

hr()   { printf "\n─── %s ──────────────────────────\n" "$1"; }
run()  { printf "$ %s\n" "$*"; eval "$@"; local rc=$?; printf "(exit=%d)\n" "$rc"; return $rc; }

# Save & restore clipboard so paste test doesn't trash user state.
ORIG_CLIP="$(pbpaste 2>/dev/null || true)"
restore_clip() { printf '%s' "$ORIG_CLIP" | pbcopy; }
trap restore_clip EXIT

# ─── T1 · ui menu_path (kinax NavigateMenu) ─────────────────────────
hr "T1 — ui menu_path: open Safari → File → New Window"
open -a Safari
sleep 1
run "$KINCLAW -exec \"ui menu_path 'Safari > File > New Window'\" --bundle com.apple.Safari"
echo "EYE-CHECK: did a new Safari window open?"
sleep 1

# ─── T2 · ui shortcut (kinax MenuItemShortcut) ──────────────────────
hr "T2 — ui shortcut: read keyboard equivalent of File > New Window"
run "$KINCLAW -exec \"ui shortcut 'Safari > File > New Window'\" --bundle com.apple.Safari"
echo "EYE-CHECK: output should contain ⌘⇧N"

# ─── T3 · screen diff_screenshots (sckit DiffImages) ────────────────
hr "T3 — screen diff_screenshots: identical → 0 dirty"
SHOT_A="/tmp/kinclaw-tier3-a.png"
SHOT_B="/tmp/kinclaw-tier3-b.png"
run "$KINCLAW -exec \"screen capture --target display --out $SHOT_A\""
sleep 0.5
run "$KINCLAW -exec \"screen diff_screenshots $SHOT_A $SHOT_A\""
echo "EYE-CHECK: should report '0 dirty cells' / no changes"

hr "T3b — screen diff_screenshots: real change → non-zero dirty"
# Cause a visible change between captures.
run "$KINCLAW -exec \"screen capture --target display --out $SHOT_A\""
osascript -e 'tell application "Safari" to make new document' 2>/dev/null
sleep 0.6
run "$KINCLAW -exec \"screen capture --target display --out $SHOT_B\""
run "$KINCLAW -exec \"screen diff_screenshots $SHOT_A $SHOT_B\""
echo "EYE-CHECK: should report >0 dirty cells + show ASCII heatmap"

# ─── T4 · input paste (input PasteText, IME-safe) ───────────────────
hr "T4 — input paste: 你好世界 should not drop chars"
open -a TextEdit
sleep 1
osascript -e 'tell application "TextEdit" to make new document' 2>/dev/null
sleep 0.5
# Pre-load clipboard with a sentinel
SENTINEL="ORIGINAL_$(date +%s)"
echo -n "$SENTINEL" | pbcopy
run "$KINCLAW -exec \"input paste '你好世界 1234'\""
sleep 0.3
echo "Clipboard now contains: $(pbpaste)"
echo "EYE-CHECK: TextEdit shows '你好世界 1234' (no dropped chars)"
echo "          + clipboard restored to '$SENTINEL'"

# ─── T5 · ui scroll_to (kinax ActionScrollToVisible) ────────────────
hr "T5 — ui scroll_to: System Settings → Privacy → Lockdown Mode"
echo "(skipping by default — Privacy & Security UI changes a lot;"
echo " run manually if you want to verify scroll_to specifically)"
echo "Manual command:"
echo "  $KINCLAW -exec \"ui scroll_to 'Lockdown Mode' --bundle com.apple.systempreferences\""

hr "done"
echo "Review the EYE-CHECK lines above; the kit-debt repayment is OK"
echo "if T1+T2+T3+T3b+T4 all behaved as described."
