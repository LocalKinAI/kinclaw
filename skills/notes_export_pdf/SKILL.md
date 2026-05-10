---
name: notes_export_pdf
description: |
  Export a Notes note to a PDF file at an absolute path. Notes
  doesn't expose Export-as-PDF via AppleScript dictionary; this
  skill drives the File menu + Save sheet via UI scripting.

  The Save sheet is the standard NSSavePanel — we type the filename
  into the focused field, use Cmd+Shift+G to set the destination
  directory, then press Return to commit. The skill does NOT use
  the Print → Save as PDF route (3+ extra dialog clicks); it uses
  Notes' direct File → Export as PDF... menu.

  After the save, the skill polls for the file's existence to
  confirm completion before returning.
command:
  - sh
  - -c
  - |
    NOTE="$1"; OUT_PATH="$2"
    [ -z "$NOTE" ] && { echo "note title required" >&2; exit 1; }
    [ -z "$OUT_PATH" ] && { echo "output path required" >&2; exit 1; }

    # Split path into directory + filename
    OUT_DIR="$(/usr/bin/dirname "$OUT_PATH")"
    OUT_FILE="$(/usr/bin/basename "$OUT_PATH")"
    /bin/mkdir -p "$OUT_DIR"
    # Remove existing file so we can detect creation precisely
    /bin/rm -f "$OUT_PATH"

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
                click menu item "Export as PDF…" of menu "File" of menu bar 1
            on error
                -- Fallback: try without ellipsis
                try
                    click menu item "Export as PDF" of menu "File" of menu bar 1
                on error e
                    return "ERR_MENU: " & e
                end try
            end try
        end tell
    end tell
    delay 1.5

    -- Save sheet is now open. Filename field is auto-focused.
    -- 1) clear current name (Cmd+A in text field selects, then type new)
    tell application "System Events"
        keystroke "a" using {command down}
        delay 0.1
        keystroke "$OUT_FILE"
        delay 0.3
        -- 2) Cmd+Shift+G opens Go to: sheet inside the save panel
        keystroke "g" using {command down, shift down}
        delay 0.5
        keystroke "$OUT_DIR"
        delay 0.3
        -- 3) Press return: closes Go-to sheet
        keystroke return
        delay 0.5
        -- 4) Press return again: triggers default Save button
        keystroke return
        delay 0.5
    end tell
    APPLE

    # Poll for file creation (up to 8 seconds)
    for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16; do
      if [ -f "$OUT_PATH" ]; then
        SIZE=$(/usr/bin/stat -f %z "$OUT_PATH")
        if [ "$SIZE" -gt 1000 ]; then
          echo "ok: saved $OUT_PATH ($SIZE bytes)"
          exit 0
        fi
      fi
      /bin/sleep 0.5
    done

    echo "ERR: PDF not created at $OUT_PATH after 8s" >&2
    exit 1
  - "_"
args:
  - "{{note}}"
  - "{{out_path}}"
schema:
  note:
    type: string
    description: Exact note title
    required: true
  out_path:
    type: string
    description: Absolute POSIX path where the PDF should be saved (e.g. /Users/me/Desktop/kinbench/168-note.pdf)
    required: true
timeout: 30
---

# notes_export_pdf — export a Notes note to PDF at a path

Notes' File menu has direct "Export as PDF..." which opens an
NSSavePanel. The agent's typical failure mode is to open the
File menu + click Export as PDF, then get lost in the Save sheet
— filename field, location field (which is hidden behind a
Cmd+Shift+G shortcut), and the actual Save button. This skill
drives all of that deterministically.

## Examples

```
notes_export_pdf note="KinBench Print 168" out_path="/Users/me/Desktop/kinbench/168-note.pdf"
  → ok: saved /Users/me/Desktop/kinbench/168-note.pdf (47823 bytes)

notes_export_pdf note="KinBench Export 182" out_path="/Users/me/Desktop/kinbench/182-note.pdf"
  → ok: saved /Users/me/Desktop/kinbench/182-note.pdf (45612 bytes)
```

## Why not Print → Save as PDF

The Print → Save as PDF route requires (1) Cmd+P, (2) click PDF
dropdown bottom-left, (3) click "Save as PDF…", (4) navigate Save
sheet. Notes' direct File → Export as PDF... is one click + Save
sheet — half the friction. This skill uses the direct path.
