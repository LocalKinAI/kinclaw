---
name: notes_checklist
description: |
  Convert a note's body into a Notes-native checklist (HTML
  `<ul class="gtl-todo-list">` markup), with optional auto-checking
  of specific item indices.

  Agents routinely fail at this by typing markdown `- [ ]` text —
  Notes does NOT recognize markdown checklist syntax; it stores
  literal "- [ ]" text. The native checklist requires Cmd+Shift+L
  on selected text or Format menu → Checklist.

  This skill: activates Notes, focuses the note body, selects all,
  Cmd+Shift+L to convert. If `check_indices` is given, navigates to
  each line and presses Cmd+Shift+U to toggle the checkbox.
command:
  - sh
  - -c
  - |
    NOTE="$1"; CHECK_IDX="${2:-}"
    [ -z "$NOTE" ] && { echo "note title required" >&2; exit 1; }

    osascript 2>&1 <<APPLE
    tell application "Notes"
        activate
        try
            show (first note whose name = "$NOTE")
        on error
            return "ERR: note not found"
        end try
    end tell
    delay 0.8

    tell application "System Events"
        tell process "Notes"
            try
                set mainWin to window 1
                try
                    set bodyArea to text area 1 of scroll area 1 of group 1 of splitter group 1 of mainWin
                    perform action "AXFocus" of bodyArea
                end try
            end try
        end tell
        delay 0.3
        keystroke "a" using {command down}
        delay 0.2
        keystroke "l" using {command down, shift down}
        delay 0.5
    end tell

    set checkList to "$CHECK_IDX"
    if checkList is not "" then
        set AppleScript's text item delimiters to ","
        set idxs to text items of checkList
        set AppleScript's text item delimiters to ""
        repeat with idxStr in idxs
            try
                set idxN to (idxStr as integer)
                tell application "System Events"
                    key code 126 using {command down}
                    delay 0.15
                    repeat (idxN - 1) times
                        key code 125
                        delay 0.05
                    end repeat
                    delay 0.1
                    keystroke "u" using {command down, shift down}
                    delay 0.2
                end tell
            end try
        end repeat
    end if
    return "ok: converted to checklist"
    APPLE
  - "_"
args:
  - "{{note}}"
  - "{{check_indices}}"
schema:
  note:
    type: string
    description: Exact note title
    required: true
  check_indices:
    type: string
    description: "Comma-separated 1-based line indices to mark as checked. e.g. '2' marks the second item. Empty = no items checked."
    required: false
    default: ""
timeout: 30
---

# notes_checklist — convert body to a real Notes checklist

The native Notes checklist is HTML markup `<ul class="gtl-todo-list">
<li class="gtl-todo">...`. It can ONLY be created via UI: select
text → Cmd+Shift+L (or Format → Checklist). Markdown `- [ ]` does
not transform.

## Examples

```
notes_checklist note="KinBench Checklist 169"
  → ok: converted to checklist

notes_checklist note="KinBench Mark 170" check_indices=2
  → ok: converted to checklist
```

The setup script must already have populated the note's body with the
items as plain lines (one per line). This skill handles the conversion
and optional checking. If items aren't there yet, type them via `input`
claw before calling this skill.
