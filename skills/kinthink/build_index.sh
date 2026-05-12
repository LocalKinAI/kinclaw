#!/bin/bash
# Rebuild kinthink's NL → cerebellum index from macbench task prompts.
# Each macbench task.json has a `prompt` like:
#   "Rename foo.txt to bar.txt. Fast path: cerebellum 'finder rename foo.txt bar.txt'."
# We extract the NL portion (before "Fast path:") and the cerebellum
# call (single-quoted argument to `cerebellum`).
#
# Output: TSV with three columns: task_id, NL description, cerebellum command.
# 239 / 369 macbench tasks have a "Fast path" line — these become our
# training set for the grep router.

set -u
SCRIPT_DIR="$(/usr/bin/dirname "$(/usr/bin/realpath "${BASH_SOURCE[0]:-$0}" 2>/dev/null || echo "${BASH_SOURCE[0]:-$0}")")"
OUT="${1:-$SCRIPT_DIR/actions.tsv}"
MACBENCH_DIR="${MACBENCH_DIR:-/Users/jackysun/Documents/Workspace/macbench}"

: > "$OUT"

n=0
for f in "$MACBENCH_DIR"/tasks/*/task.json; do
  id="$(/usr/bin/basename "$(/usr/bin/dirname "$f")")"
  prompt="$(/usr/bin/jq -r '.prompt // ""' "$f" 2>/dev/null)"
  [ -z "$prompt" ] && continue
  case "$prompt" in *"Fast path:"*) ;; *) continue ;; esac

  nl="${prompt%%Fast path:*}"
  cmd_part="${prompt##*Fast path:}"
  cmd_part="${cmd_part//\`/}"

  # Extract first `cerebellum '…'` (or "…")
  cereb="$(printf '%s' "$cmd_part" | /usr/bin/sed -nE "s/.*cerebellum[[:space:]]+['\"]([^'\"]+)['\"].*/\1/p" | head -1)"
  [ -z "$cereb" ] && continue

  nl="$(printf '%s' "$nl" | /usr/bin/tr -s '[:space:]' ' ' | /usr/bin/sed 's/^ *//; s/ *$//')"
  printf '%s\t%s\t%s\n' "$id" "$nl" "$cereb" >> "$OUT"
  n=$((n+1))
done

echo "wrote $n NL → cerebellum pairs to $OUT"
