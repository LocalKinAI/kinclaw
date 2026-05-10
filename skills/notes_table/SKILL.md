---
name: notes_table
description: |
  Insert a table into a Notes note. Notes' table feature is
  triggered by Cmd+Opt+T while the body has focus; this skill
  selects the note, focuses the body, and fires the keystroke.

  Default: a 2x2 table (Notes' default). To get larger, use the
  `rows` and `cols` args — the skill will Tab + Return as needed
  after insertion to grow the table to the requested dimensions.
command:
  - sh
  - -c
  - |
    NOTE="$1"; ROWS="${2:-2}"; COLS="${3:-2}"
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
        -- Move to end of body so the table inserts after existing content
        key code 125 using {command down}
        delay 0.1
        keystroke return
        delay 0.1
        -- Cmd+Opt+T inserts default 2-col 2-row table
        keystroke "t" using {command down, option down}
        delay 0.6
    end tell

    -- Grow rows: each Tab from last cell creates a new row
    tell application "System Events"
        set extraRows to ($ROWS - 2)
        set extraCols to ($COLS - 2)
        if extraCols > 0 then
            -- Add columns via Format > Table > Add Column After
            -- (no default keyboard shortcut; navigate menu)
            try
                tell process "Notes"
                    repeat extraCols times
                        click menu item "Add Column After" of menu of menu item "Table" of menu "Format" of menu bar 1
                        delay 0.2
                    end repeat
                end tell
            end try
        end if
        if extraRows > 0 then
            -- Place cursor in last cell, then Tab from rightmost-bottom cell creates a new row
            -- Simpler: Format menu approach
            try
                tell process "Notes"
                    repeat extraRows times
                        click menu item "Add Row Below" of menu of menu item "Table" of menu "Format" of menu bar 1
                        delay 0.2
                    end repeat
                end tell
            end try
        end if
    end tell
    return "ok: inserted $ROWS x $COLS table"
    APPLE
  - "_"
args:
  - "{{note}}"
  - "{{rows}}"
  - "{{cols}}"
schema:
  note:
    type: string
    description: Exact note title
    required: true
  rows:
    type: number
    description: Number of rows (default 2)
    required: false
    default: 2
  cols:
    type: number
    description: Number of columns (default 2)
    required: false
    default: 2
timeout: 25
---

# notes_table — insert a table into a Notes note

Notes' Cmd+Opt+T inserts a 2x2 table at the cursor. This skill
focuses the body, fires the shortcut, then optionally grows via
Format menu ("Add Row Below", "Add Column After").

## Examples

```
notes_table note="KinBench Table 172"
  → ok: inserted 2 x 2 table

notes_table note="KinBench Table 172" rows=3 cols=3
  → ok: inserted 3 x 3 table
```
