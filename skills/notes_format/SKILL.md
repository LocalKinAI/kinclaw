---
name: notes_format
description: |
  Apply a Notes-app text format (bold / italic / underline / title /
  heading / subheading / body) to text in a note. Handles selection
  + format keystroke + verification.

  The #1 mistake agents make: pressing Cmd+B without selecting text
  first. Notes' format shortcuts only act on the current selection;
  no selection = no-op. This skill always selects first.
command:
  - sh
  - -c
  - |
    NOTE="$1"; FORMAT="$2"; SELECTION="$3"
    [ -z "$NOTE" ] && { echo "note title required" >&2; exit 1; }
    [ -z "$FORMAT" ] && { echo "format required" >&2; exit 1; }
    [ -z "$SELECTION" ] && SELECTION="all"

    # Map format → keystroke
    case "$FORMAT" in
      bold)        KS_KEY="b"; KS_MOD="command" ;;
      italic)      KS_KEY="i"; KS_MOD="command" ;;
      underline)   KS_KEY="u"; KS_MOD="command" ;;
      title)       KS_KEY="1"; KS_MOD="command_option" ;;
      heading)     KS_KEY="2"; KS_MOD="command_option" ;;
      subheading)  KS_KEY="3"; KS_MOD="command_option" ;;
      body)        KS_KEY="4"; KS_MOD="command_option" ;;
      *) echo "unknown format '$FORMAT' — use bold|italic|underline|title|heading|subheading|body" >&2; exit 1 ;;
    esac

    # Select the note, focus body, then select-all + apply format
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
            -- Click the note body so keystrokes go there. The body is the
            -- ScrollArea / TextArea inside the main window. We try the
            -- canonical AX path; on failure, fall back to a generic click.
            try
                set mainWin to window 1
                try
                    set bodyArea to text area 1 of scroll area 1 of group 1 of splitter group 1 of mainWin
                    perform action "AXFocus" of bodyArea
                end try
            end try
        end tell
    end tell
    delay 0.3

    -- Select per SELECTION arg
    tell application "System Events"
        if "$SELECTION" is "all" then
            keystroke "a" using {command down}
        else if "$SELECTION" is "first_line" then
            -- Cmd+Up to top, then Shift+End to extend selection to end of line
            key code 126 using {command down}
            key code 119 using {shift down}
        else if "$SELECTION" is "last_line" then
            key code 125 using {command down}
            key code 115 using {shift down}
        end if
    end tell
    delay 0.2

    -- Apply format
    tell application "System Events"
        if "$KS_MOD" is "command" then
            keystroke "$KS_KEY" using {command down}
        else
            keystroke "$KS_KEY" using {command down, option down}
        end if
    end tell
    delay 0.3
    return "ok: $FORMAT applied to $SELECTION"
    APPLE
  - "_"
args:
  - "{{note}}"
  - "{{format}}"
  - "{{selection}}"
schema:
  note:
    type: string
    description: Exact note title
    required: true
  format:
    type: string
    description: "bold | italic | underline | title | heading | subheading | body"
    required: true
  selection:
    type: string
    description: "all (Cmd+A whole body — default) | first_line | last_line"
    required: false
    default: "all"
timeout: 15
---

# notes_format — apply Notes formatting reliably

Wraps the select-then-format pattern that agents routinely get
wrong. Always selects text BEFORE pressing the format keystroke,
so the format applies to something rather than no-op'ing.

## Examples

```
notes_format note="KinBench Bold 173" format=bold
  → ok: bold applied to all

notes_format note="KinBench Heading 174" format=title selection=first_line
  → ok: title applied to first_line
```

## Format → keystroke mapping

| format | macOS shortcut |
|---|---|
| bold | Cmd+B |
| italic | Cmd+I |
| underline | Cmd+U |
| title | Cmd+Opt+1 (largest) |
| heading | Cmd+Opt+2 |
| subheading | Cmd+Opt+3 |
| body | Cmd+Opt+4 |
