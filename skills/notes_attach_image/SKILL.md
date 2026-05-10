---
name: notes_attach_image
description: |
  Attach an image file to a Notes note. Notes' AppleScript
  dictionary does NOT support `make new attachment` for image files,
  so this skill uses the clipboard-paste path:

  1. Read the image into the system pasteboard via `osascript`'s
     `(read POSIX file "X" as JPEG picture)` coercion (works for
     JPEG / PNG / TIFF — checks file extension).
  2. Activate Notes, select the target note, focus the body.
  3. Move cursor to end of body, then Cmd+V — Notes pastes a
     pasteboard image as an attachment in the note.

  This is more robust than the Edit-menu "Attach File" UI flow,
  which opens a file picker that's brittle to AX-walk.
command:
  - sh
  - -c
  - |
    NOTE="$1"; IMG="$2"
    [ -z "$NOTE" ] && { echo "note title required" >&2; exit 1; }
    [ -z "$IMG" ] && { echo "image path required" >&2; exit 1; }
    [ ! -f "$IMG" ] && { echo "image file not found: $IMG" >&2; exit 1; }

    # Detect type by extension for AppleScript pasteboard coercion
    case "${IMG##*.}" in
      [Jj][Pp][Gg]|[Jj][Pp][Ee][Gg]) AS_TYPE="JPEG picture" ;;
      [Pp][Nn][Gg])                  AS_TYPE="«class PNGf»" ;;
      [Tt][Ii][Ff]|[Tt][Ii][Ff][Ff]) AS_TYPE="TIFF picture" ;;
      [Hh][Ee][Ii][Cc])              AS_TYPE="«class heic»" ;;
      *)                             AS_TYPE="«class furl»" ;;  # fall back to file URL
    esac

    # Step 1: load image into clipboard
    LOAD_RESULT="$(osascript 2>&1 <<APPLE
    try
        set the clipboard to (read POSIX file "$IMG" as $AS_TYPE)
        return "loaded"
    on error e
        try
            -- Fallback: just put the file:// URL on clipboard
            set the clipboard to POSIX file "$IMG"
            return "loaded_as_file_url"
        on error e2
            return "ERR_CLIPBOARD: " & e2
        end try
    end try
    APPLE
    )"
    if [[ "$LOAD_RESULT" == ERR* ]]; then
      echo "$LOAD_RESULT" >&2
      exit 1
    fi

    # Step 2: paste into Notes
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
        -- Move to end of body
        key code 125 using {command down}
        delay 0.1
        keystroke return
        delay 0.1
        -- Paste image
        keystroke "v" using {command down}
        delay 0.8
    end tell
    return "ok: pasted (mode: $LOAD_RESULT)"
    APPLE
  - "_"
args:
  - "{{note}}"
  - "{{image}}"
schema:
  note:
    type: string
    description: Exact note title
    required: true
  image:
    type: string
    description: Absolute POSIX path to JPEG / PNG / TIFF / HEIC image
    required: true
timeout: 20
---

# notes_attach_image — paste an image as a Notes attachment

Notes attachments via AppleScript are not supported in modern
macOS; the canonical path is via the clipboard. This skill loads
the image into the pasteboard with the correct image-type coercion,
then pastes into the focused note body — Notes recognizes the
pasteboard image and stores it as a real attachment.

## Examples

```
notes_attach_image note="KinBench Image 171" image="/Users/me/Pictures/cat.jpg"
  → ok: pasted (mode: loaded)
```
