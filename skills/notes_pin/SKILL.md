---
name: notes_pin
description: |
  Pin or unpin a Notes note (by exact title match). Uses UI menu
  navigation because macOS 14+ removed the AppleScript `pinned`
  property — direct `set pinned of note to true` errors with
  "Can't make pinned of note ... into type specifier".

  This skill activates Notes, selects the target note, then clicks
  the appropriate File-menu item ("Pin Note" or "Unpin Note"). The
  menu item label automatically toggles based on current state, so
  the skill checks which one exists and clicks the right one for
  the requested target state.
command:
  - sh
  - -c
  - |
    NOTE="$1"; STATE="$2"
    [ -z "$NOTE" ] && { echo "note title required" >&2; exit 1; }
    [ -z "$STATE" ] && STATE="true"
    case "$STATE" in
      true|pin|pinned|1) WANT="pin" ;;
      false|unpin|unpinned|0) WANT="unpin" ;;
      *) echo "state must be true|false" >&2; exit 1 ;;
    esac

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

    set didAct to false
    set targetState to "$WANT"
    tell application "System Events"
        tell process "Notes"
            try
                set fileMenu to menu "File" of menu bar 1
                if targetState is "pin" then
                    if exists (menu item "Pin Note" of fileMenu) and ¬
                       (enabled of menu item "Pin Note" of fileMenu) then
                        click menu item "Pin Note" of fileMenu
                        set didAct to true
                    else if exists (menu item "Unpin Note" of fileMenu) then
                        -- already pinned: idempotent success
                        set didAct to true
                    end if
                else
                    if exists (menu item "Unpin Note" of fileMenu) and ¬
                       (enabled of menu item "Unpin Note" of fileMenu) then
                        click menu item "Unpin Note" of fileMenu
                        set didAct to true
                    else if exists (menu item "Pin Note" of fileMenu) then
                        -- already unpinned: idempotent success
                        set didAct to true
                    end if
                end if
            on error e
                return "ERR_MENU: " & e
            end try
        end tell
    end tell
    delay 0.3
    if didAct then
        return "ok: " & targetState
    end if
    return "ERR: menu item not found"
    APPLE
  - "_"
args:
  - "{{note}}"
  - "{{state}}"
schema:
  note:
    type: string
    description: Exact note title
    required: true
  state:
    type: string
    description: "true (pin) or false (unpin). Defaults to true."
    required: false
    default: "true"
timeout: 20
---

# notes_pin — pin / unpin a Notes note via File menu

macOS 14+ removed AppleScript's `pinned` property. The only reliable
way to set pin state is the File menu's "Pin Note" / "Unpin Note"
item, which Notes shows toggled based on current state.

This skill:
1. Activates Notes + selects the note (by exact name match)
2. Looks for "Pin Note" or "Unpin Note" in the File menu
3. Clicks the right one for the requested target state
4. Idempotent — if already in the desired state, returns success without re-clicking

## Examples

```
notes_pin note="KinBench Pinned 164" state=true
  → ok: pin

notes_pin note="KinBench Unpin 165" state=false
  → ok: unpin
```

Note: pin state remains unverifiable via AppleScript on macOS 14+, so
this skill cannot self-verify. The skill returns `ok` based on
"correct menu item was clicked" — the caller's eval should confirm
via UI inspection or trust the action.
