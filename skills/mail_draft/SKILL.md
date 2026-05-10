---
name: mail_draft
description: |
  Create a Mail draft (saved, not sent) with subject + body and an
  optional attachment. Uses Mail's AppleScript dictionary directly:
  `make new outgoing message` then `save` (NOT `send`). The draft
  appears in the Drafts mailbox of the default account.

  This is the CORRECT path for any task that says "save as Mail
  draft" / "share via Mail as draft". Common wrong path: agents
  open a compose window with Cmd+N, type subject + body, then close
  with Cmd+W — that prompts a Save sheet they often skip, losing
  the draft. This skill bypasses the compose window entirely.
command:
  - sh
  - -c
  - |
    SUBJECT="$1"; BODY="$2"; ATTACH="$3"; TO="$4"
    [ -z "$SUBJECT" ] && { echo "subject required" >&2; exit 1; }

    # Escape backslashes and double-quotes for AppleScript string literals.
    esc() { printf '%s' "$1" | /usr/bin/sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'; }
    SUBJECT_E="$(esc "$SUBJECT")"
    BODY_E="$(esc "$BODY")"
    TO_E="$(esc "$TO")"
    # ATTACH is a path — leave it as-is for POSIX file coercion

    osascript 2>&1 <<APPLE
    tell application "Mail"
        set newMsg to make new outgoing message with properties ¬
            {subject:"$SUBJECT_E", content:"$BODY_E", visible:false}
        if "$TO_E" is not "" then
            try
                tell newMsg to make new to recipient at end of to recipients ¬
                    with properties {address:"$TO_E"}
            end try
        end if
        if "$ATTACH" is not "" then
            try
                tell newMsg to make new attachment ¬
                    with properties {file name:(POSIX file "$ATTACH")}
            on error errMsg
                save newMsg
                return "draft_saved_no_attach: " & errMsg
            end try
        end if
        save newMsg
        return "draft_saved"
    end tell
    APPLE
  - "_"
args:
  - "{{subject}}"
  - "{{body}}"
  - "{{attachment}}"
  - "{{to}}"
schema:
  subject:
    type: string
    description: Email subject line (required)
    required: true
  body:
    type: string
    description: Plain-text body of the email
    required: false
    default: ""
  attachment:
    type: string
    description: Absolute POSIX path to a file to attach (optional)
    required: false
    default: ""
  to:
    type: string
    description: Recipient email address (optional — drafts can have empty to)
    required: false
    default: ""
timeout: 30
---

# mail_draft — create + save a Mail draft (no send)

Wraps the canonical AppleScript Mail draft pattern. `save` commits
the message to the Drafts mailbox without sending. `send` is NEVER
called from this skill.

## Examples

```
mail_draft subject="KinBench Share 167" body="forwarding the note"
  → draft_saved

mail_draft subject="KinBench 369" body="see attached" attachment="/Users/me/Desktop/kinbench/369-note.pdf"
  → draft_saved
```
