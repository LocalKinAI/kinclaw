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

    move_to_folder)
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

    archive_by_subject)
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
      echo "ok: archived messages matching '$1'"
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

    create_folder)
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
      echo "ok: create_folder '$2' in account '$1'"
      ;;

    delete_folder)
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
      echo "ok: delete_folder '$2' in account '$1'"
      ;;

    rename_folder)
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

    *)
      echo "ERR: unknown mail action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}
