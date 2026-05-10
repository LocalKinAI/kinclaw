---
name: notes_move_to_folder
description: |
  Move a note (by exact title match) into a Notes folder (by exact
  folder name). Pure AppleScript — no UI scripting, no flakiness.
  The folder must already exist; create it with osascript or via the
  Notes UI before calling this skill.

  Why a skill (not raw shell osascript): models routinely emit the
  wrong AppleScript syntax for `move`, hit "Can't make ... into type
  specifier" errors, or move into the wrong account's folder. This
  skill encapsulates the canonical pattern + cross-account search.
command:
  - sh
  - -c
  - |
    NOTE="$1"; FOLDER="$2"
    [ -z "$NOTE" ] && { echo "note title required" >&2; exit 1; }
    [ -z "$FOLDER" ] && { echo "folder name required" >&2; exit 1; }

    osascript 2>&1 <<APPLE
    tell application "Notes"
        set theNote to missing value
        set targetFolder to missing value
        repeat with acct in accounts
            try
                set ms to (every note of acct whose name = "$NOTE")
                if (count of ms) > 0 then set theNote to item 1 of ms
            end try
            try
                set fs to (every folder of acct whose name = "$FOLDER")
                if (count of fs) > 0 then set targetFolder to item 1 of fs
            end try
        end repeat
        if theNote is missing value then
            return "ERR: note '$NOTE' not found in any account"
        end if
        if targetFolder is missing value then
            return "ERR: folder '$FOLDER' not found in any account"
        end if
        move theNote to targetFolder
        return "moved '$NOTE' -> '$FOLDER'"
    end tell
    APPLE
  - "_"
args:
  - "{{note}}"
  - "{{folder}}"
schema:
  note:
    type: string
    description: Exact note title to move
    required: true
  folder:
    type: string
    description: Exact name of the destination folder
    required: true
timeout: 30
---

# notes_move_to_folder — move a Notes note into a folder

Pure AppleScript wrapper for the `move <note> to <folder>` pattern,
with cross-account search so it works regardless of whether the note
lives in iCloud, "On My Mac", or another account.

## Examples

```
notes_move_to_folder note="KinBench Move 176" folder="kinbench-folder"
  → moved 'KinBench Move 176' -> 'kinbench-folder'
```

If the note or folder is missing, returns a clear `ERR:` line — the
caller should create the folder first via osascript or skip the move.
