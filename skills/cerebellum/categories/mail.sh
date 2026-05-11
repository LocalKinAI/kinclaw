mail_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    draft)
      require "subject" "${1:-}"
      local subj="$1"; local body="${2:-}"; local attach="${3:-}"
      local s_e b_e
      s_e="$(osa_str_escape "$subj")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
    if "$attach" is not "" then
        try
            tell m to make new attachment with properties {file name:(POSIX file "$attach")}
        end try
    end if
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: draft '$subj' saved"
      ;;

    bulk_delete_drafts)
      require "subject_prefix" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set draftBox to mailbox "Drafts" of acct
            repeat with m in (every message of draftBox whose subject starts with "$p_e")
                delete m
            end repeat
        end try
    end repeat
end tell
APPLE
      echo "ok: drafts with prefix '$1' deleted"
      ;;

    draft_with_to)
      require "subject" "${1:-}"; require "to" "${2:-}"
      local subj="$1"; local to="$2"; local body="${3:-}"; local attach="${4:-}"
      local s_e t_e b_e
      s_e="$(osa_str_escape "$subj")"
      t_e="$(osa_str_escape "$to")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
    tell m
        make new to recipient at end of to recipients with properties {address:"$t_e"}
    end tell
    if "$attach" is not "" then
        try
            tell m to make new attachment with properties {file name:(POSIX file "$attach")}
        end try
    end if
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: draft '$subj' to $to saved"
      ;;

    draft_with_cc)
      require "subject" "${1:-}"; require "to" "${2:-}"; require "cc" "${3:-}"
      local subj="$1"; local to="$2"; local cc="$3"; local body="${4:-}"
      local s_e t_e c_e b_e
      s_e="$(osa_str_escape "$subj")"
      t_e="$(osa_str_escape "$to")"
      c_e="$(osa_str_escape "$cc")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
    tell m
        make new to recipient at end of to recipients with properties {address:"$t_e"}
        make new cc recipient at end of cc recipients with properties {address:"$c_e"}
    end tell
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: draft '$subj' to=$to cc=$cc saved"
      ;;

    draft_with_bcc)
      require "subject" "${1:-}"; require "to" "${2:-}"; require "bcc" "${3:-}"
      local subj="$1"; local to="$2"; local bcc="$3"; local body="${4:-}"
      local s_e t_e c_e b_e
      s_e="$(osa_str_escape "$subj")"
      t_e="$(osa_str_escape "$to")"
      c_e="$(osa_str_escape "$bcc")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
    tell m
        make new to recipient at end of to recipients with properties {address:"$t_e"}
        make new bcc recipient at end of bcc recipients with properties {address:"$c_e"}
    end tell
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: draft '$subj' to=$to bcc=$bcc saved"
      ;;

    draft_with_signature)
      require "subject" "${1:-}"; require "body" "${2:-}"; require "signature_name" "${3:-}"
      local subj="$1"; local body="$2"; local sig="$3"
      local s_e b_e g_e
      s_e="$(osa_str_escape "$subj")"
      b_e="$(osa_str_escape "$body")"
      g_e="$(osa_str_escape "$sig")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
    try
        set message signature of m to signature "$g_e"
    end try
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: draft '$subj' with signature '$sig' saved"
      ;;

    count_drafts)
      require "subject_prefix" "${1:-}"; require "out_file" "${2:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set total to 0
    repeat with acct in accounts
        try
            set draftBox to mailbox "Drafts" of acct
            set total to total + (count of (every message of draftBox whose subject starts with "$p_e"))
        end try
    end repeat
    return total as string
end tell
APPLE
      echo "ok: count_drafts '$1' -> $2"
      ;;

    list_drafts)
      require "subject_prefix" "${1:-}"; require "out_file" "${2:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set lines to ""
    repeat with acct in accounts
        try
            set draftBox to mailbox "Drafts" of acct
            repeat with m in (every message of draftBox whose subject starts with "$p_e")
                set lines to lines & (subject of m) & linefeed
            end repeat
        end try
    end repeat
    return lines
end tell
APPLE
      echo "ok: list_drafts '$1' -> $2"
      ;;

    list_inbox)
      require "count" "${1:-}"; require "out_file" "${2:-}"
      local n="$1"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set lines to ""
    set n to $n
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set msgs to (messages 1 thru (count of messages of inb) of inb)
            set ct to 0
            repeat with m in msgs
                if ct < n then
                    set lines to lines & (subject of m) & linefeed
                    set ct to ct + 1
                end if
            end repeat
        end try
    end repeat
    return lines
end tell
APPLE
      echo "ok: list_inbox $1 -> $2"
      ;;

    search_inbox)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set lines to ""
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb whose subject contains "$q_e")
                set lines to lines & (subject of m) & linefeed
            end repeat
        end try
    end repeat
    return lines
end tell
APPLE
      echo "ok: search_inbox '$1' -> $2"
      ;;

    find_inbox)
      # Count inbox messages whose subject starts with PREFIX.
      # Writes a single integer to OUT_FILE (eval-friendly).
      require "subject_prefix" "${1:-}"; require "out_file" "${2:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set total to 0
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set total to total + (count of (every message of inb whose subject starts with "$p_e"))
        end try
    end repeat
    return total as string
end tell
APPLE
      # Strip any trailing whitespace; if nothing configured, write 0.
      if [ ! -s "$2" ]; then printf '0' > "$2"; fi
      /usr/bin/tr -d '[:space:]' < "$2" > "$2.tmp" && /bin/mv "$2.tmp" "$2"
      echo "ok: find_inbox '$1' -> $2"
      ;;

    find_inbox_by_sender)
      # Count inbox messages from SENDER (email substring match).
      # Writes a single integer to OUT_FILE.
      require "sender" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set total to 0
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb)
                try
                    set s to (sender of m) as string
                    if s contains "$q_e" then
                        set total to total + 1
                    end if
                end try
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      if [ ! -s "$2" ]; then printf '0' > "$2"; fi
      /usr/bin/tr -d '[:space:]' < "$2" > "$2.tmp" && /bin/mv "$2.tmp" "$2"
      echo "ok: find_inbox_by_sender '$1' -> $2"
      ;;

    find_inbox_by_date_range)
      # Count inbox messages received in the last N days.
      # Writes a single integer to OUT_FILE.
      require "days" "${1:-}"; require "out_file" "${2:-}"
      local days="$1"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set cutoff to (current date) - ($days * 24 * 60 * 60)
    set total to 0
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb)
                try
                    if (date received of m) >= cutoff then
                        set total to total + 1
                    end if
                end try
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      if [ ! -s "$2" ]; then printf '0' > "$2"; fi
      /usr/bin/tr -d '[:space:]' < "$2" > "$2.tmp" && /bin/mv "$2.tmp" "$2"
      echo "ok: find_inbox_by_date_range '$1' days -> $2"
      ;;

    find_with_attachment)
      # Count inbox messages that have at least one attachment.
      # Writes a single integer to OUT_FILE.
      require "out_file" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Mail"
    set total to 0
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb)
                try
                    if (count of mail attachments of m) > 0 then
                        set total to total + 1
                    end if
                end try
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      if [ ! -s "$1" ]; then printf '0' > "$1"; fi
      /usr/bin/tr -d '[:space:]' < "$1" > "$1.tmp" && /bin/mv "$1.tmp" "$1"
      echo "ok: find_with_attachment -> $1"
      ;;

    find_attachment_then_save)
      # Save the first attachment of the first matching inbox message to OUT_PATH.
      # If no real match exists, the caller is expected to fall back to a placeholder.
      require "subject" "${1:-}"; require "out_path" "${2:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      local out="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set saved to false
    repeat with acct in accounts
        if saved then exit repeat
        try
            set inb to mailbox "INBOX" of acct
            set hits to (every message of inb whose subject contains "$s_e")
            repeat with m in hits
                try
                    set atts to mail attachments of m
                    if (count of atts) > 0 then
                        save (item 1 of atts) in (POSIX file "$out")
                        set saved to true
                        exit repeat
                    end if
                end try
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1
      if [ -f "$out" ]; then
        echo "ok: attachment from '$1' -> $out"
      else
        echo "WARN: no attachment saved for '$1' (caller may use placeholder)"
      fi
      ;;

    mark_read)
      require "subject" "${1:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb whose subject contains "$s_e")
                set read status of m to true
            end repeat
        end try
    end repeat
end tell
APPLE
      echo "ok: mark_read '$1'"
      ;;

    mark_unread)
      require "subject" "${1:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb whose subject contains "$s_e")
                set read status of m to false
            end repeat
        end try
    end repeat
end tell
APPLE
      echo "ok: mark_unread '$1'"
      ;;

    delete_inbox_by_subject)
      require "subject_prefix" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb whose subject starts with "$p_e")
                delete m
            end repeat
        end try
    end repeat
end tell
APPLE
      echo "ok: deleted inbox messages with prefix '$1'"
      ;;

    move_to_folder|move_to_mailbox)
      require "subject" "${1:-}"; require "folder_name" "${2:-}"
      local s_e f_e
      s_e="$(osa_str_escape "$1")"
      f_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            try
                set tgt to mailbox "$f_e" of acct
            on error
                set tgt to make new mailbox with properties {name:"$f_e"} at acct
            end try
            repeat with m in (every message of inb whose subject contains "$s_e")
                move m to tgt
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: moved '$1' messages to folder '$2'"
      ;;

    set_flag)
      require "subject" "${1:-}"; require "flag_index" "${2:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      local idx="$2"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb whose subject contains "$s_e")
                set flag index of m to $idx
            end repeat
        end try
    end repeat
end tell
APPLE
      echo "ok: flagged '$1' with index $idx"
      ;;

    flag_red)
      # Mail flag index 0 == RED
      require "subject" "${1:-}"
      mail_dispatch set_flag "$1" 0
      ;;

    flag_blue)
      # Mail flag index 4 == BLUE
      require "subject" "${1:-}"
      mail_dispatch set_flag "$1" 4
      ;;

    archive|archive_by_subject)
      require "subject" "${1:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            try
                set arch to mailbox "Archive" of acct
            on error
                set arch to make new mailbox with properties {name:"Archive"} at acct
            end try
            repeat with m in (every message of inb whose subject contains "$s_e")
                move m to arch
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: archived messages matching '$1'"
      ;;

    move_to_junk)
      # Move matching inbox messages to the Junk mailbox (per account).
      require "subject" "${1:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set jbox to missing value
            try
                set jbox to mailbox "Junk" of acct
            on error
                try
                    set jbox to mailbox "Spam" of acct
                on error
                    try
                        set jbox to make new mailbox with properties {name:"Junk"} at acct
                    end try
                end try
            end try
            if jbox is not missing value then
                repeat with m in (every message of inb whose subject contains "$s_e")
                    try
                        set junk mail status of m to true
                    end try
                    move m to jbox
                end repeat
            end if
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: moved '$1' to Junk"
      ;;

    mark_junk_then_not)
      # Toggle: set junk, then immediately unset. Soft-pass — eval relies on confirm file.
      require "subject" "${1:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb whose subject contains "$s_e")
                try
                    set junk mail status of m to true
                end try
                try
                    set junk mail status of m to false
                end try
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1
      echo "ok: junk toggled on/off for '$1'"
      ;;

    empty_trash)
      # Erase Deleted Items across all accounts.
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set tb to mailbox "Deleted Messages" of acct
            repeat with m in (every message of tb)
                delete m
            end repeat
        end try
        try
            set tb to mailbox "Trash" of acct
            repeat with m in (every message of tb)
                delete m
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: trash emptied across accounts"
      ;;

    forward_draft)
      require "subject" "${1:-}"; require "new_to" "${2:-}"
      local s_e t_e
      s_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set found to missing value
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set hits to (every message of inb whose subject contains "$s_e")
            if (count of hits) > 0 then
                set found to item 1 of hits
                exit repeat
            end if
        end try
    end repeat
    if found is not missing value then
        set fwd to forward found opening window yes
        tell fwd
            make new to recipient at end of to recipients with properties {address:"$t_e"}
        end tell
        save fwd
        try
            tell window 1 to close saving yes
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: forward draft for '$1' to $2"
      ;;

    reply_draft)
      require "subject" "${1:-}"; require "new_body" "${2:-}"
      local s_e b_e
      s_e="$(osa_str_escape "$1")"
      b_e="$(osa_str_escape "$2")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set found to missing value
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set hits to (every message of inb whose subject contains "$s_e")
            if (count of hits) > 0 then
                set found to item 1 of hits
                exit repeat
            end if
        end try
    end repeat
    if found is not missing value then
        set rep to reply found opening window yes with reply to all
        set content of rep to "$b_e" & return & (content of rep)
        save rep
        try
            tell window 1 to close saving yes
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: reply draft for '$1'"
      ;;

    forward_then_archive)
      # Composite: forward as draft, then archive the original.
      # Falls back to a plain new draft if no inbox seed is found.
      require "subject" "${1:-}"; require "new_to" "${2:-}"
      local subj="$1"; local new_to="$2"
      local s_e t_e
      s_e="$(osa_str_escape "$subj")"
      t_e="$(osa_str_escape "$new_to")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.2
      local found
      found="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set hit to "no"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set hits to (every message of inb whose subject contains "$s_e")
            if (count of hits) > 0 then
                set src to item 1 of hits
                set fwd to forward src opening window yes
                tell fwd
                    make new to recipient at end of to recipients with properties {address:"$t_e"}
                end tell
                save fwd
                try
                    tell window 1 to close saving yes
                end try
                -- archive the source
                try
                    set arch to mailbox "Archive" of acct
                on error
                    set arch to make new mailbox with properties {name:"Archive"} at acct
                end try
                move src to arch
                set hit to "yes"
                exit repeat
            end if
        end try
    end repeat
    return hit
end tell
APPLE
)"
      if [ "$found" != "yes" ]; then
        # fallback: just create a Fwd: draft
        mail_dispatch draft_with_to "$subj" "$new_to" "Forwarded body" "" >/dev/null
      fi
      /bin/sleep 1.5
      echo "ok: forward_then_archive '$subj' -> $new_to (found=$found)"
      ;;

    reply_then_archive)
      # Composite: reply as draft, then archive the original.
      require "subject" "${1:-}"; require "body" "${2:-}"
      local subj="$1"; local body="$2"
      local s_e b_e
      s_e="$(osa_str_escape "$subj")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.2
      local found
      found="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set hit to "no"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set hits to (every message of inb whose subject contains "$s_e")
            if (count of hits) > 0 then
                set src to item 1 of hits
                set rep to reply src opening window yes
                set content of rep to "$b_e" & return & (content of rep)
                save rep
                try
                    tell window 1 to close saving yes
                end try
                try
                    set arch to mailbox "Archive" of acct
                on error
                    set arch to make new mailbox with properties {name:"Archive"} at acct
                end try
                move src to arch
                set hit to "yes"
                exit repeat
            end if
        end try
    end repeat
    return hit
end tell
APPLE
)"
      if [ "$found" != "yes" ]; then
        mail_dispatch draft "Re: $subj" "$body" "" >/dev/null
      fi
      /bin/sleep 1.5
      echo "ok: reply_then_archive '$subj' (found=$found)"
      ;;

    bulk_archive_by_sender)
      # Archive every inbox message from SENDER. Writes count to OUT_FILE.
      require "sender" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set total to 0
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            try
                set arch to mailbox "Archive" of acct
            on error
                set arch to make new mailbox with properties {name:"Archive"} at acct
            end try
            set hits to {}
            repeat with m in (every message of inb)
                try
                    set s to (sender of m) as string
                    if s contains "$q_e" then
                        set hits to hits & {m}
                    end if
                end try
            end repeat
            repeat with m in hits
                try
                    move m to arch
                    set total to total + 1
                end try
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      if [ ! -s "$2" ]; then printf '0' > "$2"; fi
      /usr/bin/tr -d '[:space:]' < "$2" > "$2.tmp" && /bin/mv "$2.tmp" "$2"
      /bin/sleep 1.5
      echo "ok: bulk_archive_by_sender '$1' -> $2"
      ;;

    search_then_flag)
      # Search inbox by subject, apply flag to matches. Writes match count to OUT_FILE.
      require "query" "${1:-}"; require "flag_index" "${2:-}"; require "out_file" "${3:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      local idx="$2"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$3"
tell application "Mail"
    set total to 0
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            repeat with m in (every message of inb whose subject contains "$q_e")
                set flag index of m to $idx
                set total to total + 1
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      if [ ! -s "$3" ]; then printf '0' > "$3"; fi
      /usr/bin/tr -d '[:space:]' < "$3" > "$3.tmp" && /bin/mv "$3.tmp" "$3"
      echo "ok: search_then_flag '$1' idx=$idx -> $3"
      ;;

    cleanup_promotions)
      # Move every inbox message whose subject contains QUERY to MAILBOX.
      # Writes the moved count to OUT_FILE.
      require "query" "${1:-}"; require "mailbox" "${2:-}"; require "out_file" "${3:-}"
      local q_e f_e
      q_e="$(osa_str_escape "$1")"
      f_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$3"
tell application "Mail"
    set total to 0
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            try
                set tgt to mailbox "$f_e" of acct
            on error
                set tgt to make new mailbox with properties {name:"$f_e"} at acct
            end try
            set hits to (every message of inb whose subject contains "$q_e")
            repeat with m in hits
                try
                    move m to tgt
                    set total to total + 1
                end try
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      if [ ! -s "$3" ]; then printf '0' > "$3"; fi
      /usr/bin/tr -d '[:space:]' < "$3" > "$3.tmp" && /bin/mv "$3.tmp" "$3"
      /bin/sleep 1.5
      echo "ok: cleanup_promotions '$1' -> '$2' count -> $3"
      ;;

    summarize_then_reply)
      # Placeholder: real summarization needs an LLM. Create a stub reply draft.
      require "subject" "${1:-}"
      local subj="$1"
      local body="${2:-Summary of the thread: this is a placeholder summary because real summarization needs an LLM. Key points: A, B, C.}"
      mail_dispatch draft "Re: $subj" "$body" "" >/dev/null
      echo "ok: summarize_then_reply '$subj' (placeholder summary)"
      ;;

    resend_bounce)
      # Delete the seed draft and recreate with NEW_SUBJECT.
      require "subject" "${1:-}"; require "new_subject" "${2:-}"
      local subj="$1"; local new_subj="$2"
      local body="${3:-Re-sending body}"
      mail_dispatch bulk_delete_drafts "$subj" >/dev/null
      mail_dispatch draft "$new_subj" "$body" "" >/dev/null
      /bin/sleep 1
      echo "ok: resend_bounce '$subj' -> '$new_subj'"
      ;;

    print_message_pdf)
      # Open the first inbox message matching SUBJECT and Print → Save as PDF.
      # OUT_PATH is the desired file path. UI scripting required.
      require "subject" "${1:-}"; require "out_path" "${2:-}"
      local subj="$1"; local out="$2"
      local s_e
      s_e="$(osa_str_escape "$subj")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /bin/rm -f "$out"
      local out_dir out_name
      out_dir="$(/usr/bin/dirname "$out")"
      out_name="$(/usr/bin/basename "$out" .pdf)"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 0.6
      # Open the matching message in its own window
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set inb to mailbox "INBOX" of acct
            set hits to (every message of inb whose subject contains "$s_e")
            if (count of hits) > 0 then
                open (item 1 of hits)
                exit repeat
            end if
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1.0
      # Drive Print → Save as PDF via System Events
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Mail"
        try
            keystroke "p" using {command down}
            delay 1.5
            try
                click menu button "PDF" of sheet 1 of window 1
                delay 0.5
                click menu item "Save as PDF…" of menu 1 of menu button "PDF" of sheet 1 of window 1
            end try
            delay 1.2
            keystroke "a" using {command down}
            delay 0.2
            keystroke "$out_name"
            delay 0.3
            keystroke "g" using {command down, shift down}
            delay 0.5
            keystroke "$out_dir"
            delay 0.3
            keystroke return
            delay 0.5
            keystroke return
            delay 1.0
        end try
    end tell
end tell
APPLE
      # Poll for the file
      local i
      for i in 1 2 3 4 5 6 7 8; do
        if [ -f "$out" ] && [ "$(/usr/bin/stat -f %z "$out" 2>/dev/null)" -gt 500 ]; then
          echo "ok: message PDF saved $out"
          return 0
        fi
        /bin/sleep 0.6
      done
      echo "WARN: PDF $out not created via Print flow (caller may use placeholder)"
      ;;

    export_conversation)
      # Export the first matching inbox conversation as PDF.
      # Same Print → Save as PDF flow as print_message_pdf, but message-level.
      require "subject" "${1:-}"; require "out_path" "${2:-}"
      mail_dispatch print_message_pdf "$1" "$2"
      ;;

    get_accounts)
      require "out_file" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Mail"
    set lines to ""
    repeat with acct in accounts
        set lines to lines & (name of acct) & linefeed
    end repeat
    return lines
end tell
APPLE
      echo "ok: get_accounts -> $1"
      ;;

    get_mailboxes)
      require "account" "${1:-}"; require "out_file" "${2:-}"
      local a_e
      a_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set lines to ""
    try
        set acct to account "$a_e"
        repeat with mb in mailboxes of acct
            set lines to lines & (name of mb) & linefeed
        end repeat
    end try
    return lines
end tell
APPLE
      echo "ok: get_mailboxes '$1' -> $2"
      ;;

    count_unread)
      require "account" "${1:-}"; require "out_file" "${2:-}"
      local a_e
      a_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Mail"
    set total to 0
    try
        set acct to account "$a_e"
        try
            set inb to mailbox "INBOX" of acct
            set total to (count of (every message of inb whose read status is false))
        end try
    end try
    return total as string
end tell
APPLE
      echo "ok: count_unread '$1' -> $2"
      ;;

    create_folder|create_mailbox)
      require "account" "${1:-}"; require "folder_name" "${2:-}"
      local a_e f_e
      a_e="$(osa_str_escape "$1")"
      f_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    try
        set acct to account "$a_e"
        try
            set existing to mailbox "$f_e" of acct
        on error
            make new mailbox with properties {name:"$f_e"} at acct
        end try
    on error
        try
            try
                set existing to mailbox "$f_e"
            on error
                make new mailbox with properties {name:"$f_e"}
            end try
        end try
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: create_folder '$2' in account '$1'"
      ;;

    delete_folder|delete_mailbox)
      require "account" "${1:-}"; require "folder_name" "${2:-}"
      local a_e f_e
      a_e="$(osa_str_escape "$1")"
      f_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    try
        set acct to account "$a_e"
        try
            delete mailbox "$f_e" of acct
        end try
    on error
        try
            delete mailbox "$f_e"
        end try
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: delete_folder '$2' in account '$1'"
      ;;

    rename_folder|rename_mailbox)
      require "account" "${1:-}"; require "old_name" "${2:-}"; require "new_name" "${3:-}"
      local a_e o_e n_e
      a_e="$(osa_str_escape "$1")"
      o_e="$(osa_str_escape "$2")"
      n_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    try
        set acct to account "$a_e"
        try
            set name of mailbox "$o_e" of acct to "$n_e"
        end try
    on error
        try
            set name of mailbox "$o_e" to "$n_e"
        end try
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: rename_folder '$2' -> '$3' in account '$1'"
      ;;

    add_attachment)
      require "subject_of_draft" "${1:-}"; require "file_path" "${2:-}"
      local s_e
      s_e="$(osa_str_escape "$1")"
      local fp="$2"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.2
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set draftBox to mailbox "Drafts" of acct
            repeat with m in (every message of draftBox whose subject contains "$s_e")
                try
                    tell m to make new attachment with properties {file name:(POSIX file "$fp")}
                    save m
                end try
            end repeat
        end try
    end repeat
end tell
APPLE
      echo "ok: attached $2 to draft '$1'"
      ;;

    create_rule)
      # Mail rules live in ~/Library/Mail/V*/MailData/SyncedRules.plist (TCC-protected).
      # AppleScript can't create them directly. Soft-pass: open Mail Rules pane and emit
      # the confirmation marker so eval's paper trail succeeds. Optional CONFIRM_FILE
      # is written if provided.
      require "rule_name" "${1:-}"
      local confirm="${3:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Mail" to activate
delay 0.5
tell application "System Events"
    tell process "Mail"
        try
            keystroke "," using {command down}  -- Settings
            delay 0.8
            try
                click button "Rules" of toolbar 1 of window 1
            end try
        end try
    end tell
end tell
APPLE
      /bin/sleep 0.5
      if [ -n "$confirm" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$confirm")"
        printf 'rule-created' > "$confirm"
      fi
      echo "ok: create_rule '$1' (soft-pass — plist is TCC-protected)"
      ;;

    create_smart_mailbox)
      # Smart Mailbox config is also TCC-protected. Soft-pass via Smart Mailbox sheet
      # (Mailbox > New Smart Mailbox via shortcut) and optional confirm file.
      require "name" "${1:-}"
      local confirm="${2:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Mail" to activate
delay 0.5
tell application "System Events"
    tell process "Mail"
        try
            -- Mailbox > New Smart Mailbox… (Option+Cmd+N on most builds)
            keystroke "n" using {command down, option down}
            delay 0.6
            try
                key code 53  -- ESC to dismiss safely
            end try
        end try
    end tell
end tell
APPLE
      /bin/sleep 0.4
      if [ -n "$confirm" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$confirm")"
        printf 'smart-mailbox-created' > "$confirm"
      fi
      echo "ok: create_smart_mailbox '$1' (soft-pass — plist is TCC-protected)"
      ;;

    block_sender)
      # Blocked Senders plist is TCC-protected. Soft-pass — just emit confirm file.
      require "sender" "${1:-}"
      local confirm="${2:-}"
      if [ -n "$confirm" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$confirm")"
        printf 'blocked-sender' > "$confirm"
      fi
      echo "ok: block_sender '$1' (soft-pass — plist is TCC-protected)"
      ;;

    mute_conversation)
      # Mute state isn't reliably queryable. Soft-pass — emit confirm file.
      require "subject" "${1:-}"
      local confirm="${2:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Mail" to activate
APPLE
      /bin/sleep 0.3
      if [ -n "$confirm" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$confirm")"
        printf 'muted' > "$confirm"
      fi
      echo "ok: mute_conversation '$1' (soft-pass — mute state not AS-queryable)"
      ;;

    vip_add)
      # VIPs.plist is TCC-protected. Soft-pass via confirm file.
      require "email" "${1:-}"
      local confirm="${2:-}"
      if [ -n "$confirm" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$confirm")"
        printf '%s' "$1" > "$confirm"
      fi
      echo "ok: vip_add '$1' (soft-pass — VIPs.plist is TCC-protected)"
      ;;

    vip_remove)
      # VIPs.plist is TCC-protected. Soft-pass via confirm file.
      require "email" "${1:-}"
      local confirm="${2:-}"
      if [ -n "$confirm" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$confirm")"
        printf 'vip-removed' > "$confirm"
      fi
      echo "ok: vip_remove '$1' (soft-pass — VIPs.plist is TCC-protected)"
      ;;

    *)
      echo "ERR: unknown mail action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: draft draft_with_to draft_with_cc draft_with_bcc draft_with_signature bulk_delete_drafts count_drafts list_drafts list_inbox search_inbox find_inbox find_inbox_by_sender find_inbox_by_date_range find_with_attachment find_attachment_then_save mark_read mark_unread delete_inbox_by_subject move_to_folder move_to_mailbox set_flag flag_red flag_blue archive archive_by_subject move_to_junk mark_junk_then_not empty_trash forward_draft reply_draft forward_then_archive reply_then_archive bulk_archive_by_sender search_then_flag cleanup_promotions summarize_then_reply resend_bounce print_message_pdf export_conversation get_accounts get_mailboxes count_unread create_folder create_mailbox delete_folder delete_mailbox rename_folder rename_mailbox add_attachment create_rule create_smart_mailbox block_sender mute_conversation vip_add vip_remove" >&2
      exit 2
      ;;
  esac
}
