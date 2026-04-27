---
name: learn
description: |
  Append a learning note to ~/.localkin/learned.md, organized by
  bundle_id / topic. The kernel automatically reads this file at every
  boot and injects it into the agent's system prompt — so anything you
  learn will be remembered next session, across all souls.

  Use this AFTER completing a task to record:
    - AX schema quirks (depth needed, weird matchers)
    - Working keyboard / shell shortcuts that beat ui clicks
    - Failed approaches + their error codes (so you don't retry next time)
    - First-launch modal patterns
    - bundle_id name spelling (some apps use lowercase like com.apple.mail)

  Note appends to a section keyed on `topic` (usually a bundle_id).
  Creates the section if it doesn't exist; appends bullet lines if it does.
  No-op if the exact same line already exists in that section (idempotent).
command:
  - sh
  - -c
  - |
    TOPIC="$1"
    NOTE="$2"
    [ -z "$TOPIC" ] && { echo "topic required" >&2; exit 1; }
    [ -z "$NOTE" ] && { echo "note required" >&2; exit 1; }
    FILE="$HOME/.localkin/learned.md"
    mkdir -p "$HOME/.localkin"
    [ -f "$FILE" ] || printf '# KinClaw — learned across sessions\n\n' > "$FILE"

    # Idempotent: if exact line already exists, no-op. The `--` is
    # important — leading "- " in $LINE would otherwise get parsed as
    # a grep flag. Same for the section header check below.
    LINE="- $NOTE"
    if grep -Fq -- "$LINE" "$FILE" 2>/dev/null; then
      echo "already known: $TOPIC :: $NOTE"
      exit 0
    fi

    # Append to existing section if header exists, else create section.
    HEADER="## $TOPIC"
    if grep -Fq -- "$HEADER" "$FILE"; then
      # Insert line right after the section header using awk so order is preserved.
      awk -v hdr="$HEADER" -v line="$LINE" '
        $0 == hdr { print; print line; next }
        { print }
      ' "$FILE" > "$FILE.new" && mv "$FILE.new" "$FILE"
    else
      printf '\n%s\n%s\n' "$HEADER" "$LINE" >> "$FILE"
    fi
    echo "learned: $TOPIC :: $NOTE"
  - "_"
args:
  - "{{topic}}"
  - "{{note}}"
schema:
  topic:
    type: string
    description: |
      Section header to file the note under — typically a bundle_id like "com.apple.calculator", "com.apple.Notes", or a generic category like "Common: focus protection".
    required: true
  note:
    type: string
    description: |
      Single-line lesson learned. Concise. No outer "- " (added automatically). Examples — "AX tree depth ≥ 6 to see number buttons", "cmd+N + type more reliable than ui click 'New Note'", "AXError -25205 means the element is offscreen / collapsed".
    required: true
timeout: 10
---

# learn — append to the cross-session notebook

KinClaw's persistence layer for Genesis Protocol. Every successful
task or hard-won failure is an opportunity to make next session
smarter. This skill is the standardized way to write into
`~/.localkin/learned.md` — kernel auto-loads that file at boot and
injects it into the agent's system prompt.

## Idempotent

Calling `learn` with the same topic + note twice is a no-op. Safe to
spam without polluting the notebook.

## Examples

```
learn topic=com.apple.calculator note="AX tree depth ≥ 6 to see number buttons"
learn topic=com.apple.Notes note="cmd+N more reliable than ui click 'New Note'"
learn topic=com.apple.reminders note="ui click description='Add Reminder' fails with AXError -25205; use cmd+N + type"
learn topic="Common: focus protection" note="osascript activate from Terminal-driven KinClaw rarely takes frontmost"
```

## Why a SKILL.md and not native

Pure shell + awk. No Go state. The notebook lives at a known path
that the kernel already reads — this skill is just an idempotent
append helper. Nothing here belongs in the kernel.
