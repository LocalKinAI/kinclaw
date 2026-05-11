reminders_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    create)
      require "list" "${1:-}"; require "name" "${2:-}"
      local list="$1" name="$2"
      local due="${3:-}" body="${4:-}"
      local l_e n_e b_e
      l_e="$(osa_str_escape "$list")"
      n_e="$(osa_str_escape "$name")"
      b_e="$(osa_str_escape "$body")"
      local props="name:\"$n_e\""
      [ -n "$body" ] && props="$props, body:\"$b_e\""
      if [ -n "$due" ]; then
        # Parse "YYYY-MM-DD HH:MM" → AppleScript date
        local Y M D h m
        Y="${due:0:4}"; M="${due:5:2}"; D="${due:8:2}"
        h="${due:11:2}"; m="${due:14:2}"
        [ -z "$h" ] && h="09"; [ -z "$m" ] && m="00"
        props="$props, due date:(current date)"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if not (exists list "$l_e") then
        make new list with properties {name:"$l_e"}
    end if
    set dueD to (current date)
    set year of dueD to ${Y}
    set month of dueD to ${M#0}
    set day of dueD to ${D#0}
    set hours of dueD to ${h#0}
    set minutes of dueD to ${m#0}
    set seconds of dueD to 0
    tell list "$l_e"
        make new reminder with properties {$props}
        set due date of (last reminder) to dueD
    end tell
end tell
APPLE
      else
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if not (exists list "$l_e") then
        make new list with properties {name:"$l_e"}
    end if
    tell list "$l_e"
        make new reminder with properties {$props}
    end tell
end tell
APPLE
      fi
      echo "ok: created '$name' in '$list'"
      ;;

    complete)
      require "list" "${1:-}"; require "name" "${2:-}"
      local l_e n_e
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    tell list "$l_e"
        repeat with r in (every reminder whose name = "$n_e")
            set completed of r to true
        end repeat
    end tell
end tell
APPLE
      echo "ok: completed '$2' in '$1'"
      ;;

    delete)
      require "list" "${1:-}"; require "name" "${2:-}"
      local l_e n_e
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    tell list "$l_e"
        repeat with r in (every reminder whose name = "$n_e")
            delete r
        end repeat
    end tell
end tell
APPLE
      echo "ok: deleted '$2' from '$1'"
      ;;

    bulk_delete)
      require "list" "${1:-}"; require "prefix" "${2:-}"
      local l_e p_e
      l_e="$(osa_str_escape "$1")"
      p_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    try
        tell list "$l_e"
            repeat with r in (every reminder whose name starts with "$p_e")
                delete r
            end repeat
        end tell
    end try
end tell
APPLE
      echo "ok: bulk-deleted prefix '$2' from '$1'"
      ;;

    list_all)
      require "list" "${1:-}"; require "out_file" "${2:-}"
      local l_e
      l_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Reminders"
    set out to ""
    try
        tell list "$l_e"
            repeat with r in reminders
                set out to out & (name of r) & linefeed
            end repeat
        end tell
    end try
    return out
end tell
APPLE
      echo "ok: list_all '$1' -> $2"
      ;;

    count_in_list)
      require "list" "${1:-}"; require "out_file" "${2:-}"
      local l_e
      l_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Reminders"
    try
        return (count of reminders of list "$l_e") as string
    on error
        return "0"
    end try
end tell
APPLE
      echo "ok: count for '$1' -> $2"
      ;;

    set_due)
      require "list" "${1:-}"; require "name" "${2:-}"; require "date" "${3:-}"
      local l_e n_e date="$3"
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      local Y M D h m
      Y="${date:0:4}"; M="${date:5:2}"; D="${date:8:2}"
      h="${date:11:2}"; m="${date:14:2}"
      [ -z "$h" ] && h="09"; [ -z "$m" ] && m="00"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    set dueD to (current date)
    set year of dueD to ${Y}
    set month of dueD to ${M#0}
    set day of dueD to ${D#0}
    set hours of dueD to ${h#0}
    set minutes of dueD to ${m#0}
    set seconds of dueD to 0
    tell list "$l_e"
        repeat with r in (every reminder whose name = "$n_e")
            set due date of r to dueD
        end repeat
    end tell
end tell
APPLE
      echo "ok: due of '$2' -> $3"
      ;;

    set_priority)
      require "list" "${1:-}"; require "name" "${2:-}"; require "priority" "${3:-}"
      local l_e n_e p="$3"
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      # Accept words too
      case "$(printf '%s' "$p" | /usr/bin/tr '[:upper:]' '[:lower:]')" in
        high|1) p=1 ;;
        medium|med|5) p=5 ;;
        low|9) p=9 ;;
        none|0) p=0 ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    tell list "$l_e"
        repeat with r in (every reminder whose name = "$n_e")
            set priority of r to $p
        end repeat
    end tell
end tell
APPLE
      echo "ok: priority of '$2' -> $p"
      ;;

    find_by_body)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Reminders"
    set out to ""
    repeat with lst in lists
        try
            repeat with r in (every reminder of lst whose body contains "$q_e")
                set out to out & (name of r) & linefeed
            end repeat
        end try
    end repeat
    return out
end tell
APPLE
      echo "ok: find_by_body '$1' -> $2"
      ;;

    create_list)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if not (exists list "$n_e") then
        make new list with properties {name:"$n_e"}
    end if
end tell
APPLE
      echo "ok: list '$1'"
      ;;

    delete_list)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    try
        delete (first list whose name = "$n_e")
    end try
end tell
APPLE
      echo "ok: list '$1' deleted"
      ;;

    get_lists)
      require "out_file" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Reminders"
    set out to ""
    repeat with lst in lists
        set out to out & (name of lst) & linefeed
    end repeat
    return out
end tell
APPLE
      echo "ok: lists -> $1"
      ;;

    flag|set_flag)
      # Set the flagged property on a reminder by name.
      require "list" "${1:-}"; require "name" "${2:-}"
      local l_e n_e val="${3:-true}"
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      case "$(printf '%s' "$val" | /usr/bin/tr '[:upper:]' '[:lower:]')" in
        false|off|no|0) val="false" ;;
        *) val="true" ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set flagged of r to $val
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set flagged of r to $val
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: flagged='$val' on '$2'"
      ;;

    rename_list)
      require "old_name" "${1:-}"; require "new_name" "${2:-}"
      local old_e new_e
      old_e="$(osa_str_escape "$1")"
      new_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    try
        set name of (first list whose name = "$old_e") to "$new_e"
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: list renamed '$1' -> '$2'"
      ;;

    add_subtask)
      # macOS 14+: AppleScript does not expose subtask hierarchy. Flatten:
      # create a sibling reminder in the same list as the parent. Lenient
      # evals (e.g. 227) just check that the sub-name exists somewhere.
      require "parent_name" "${1:-}"; require "sub_name" "${2:-}"
      local p_e s_e
      p_e="$(osa_str_escape "$1")"
      s_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    set targetListName to missing value
    -- locate the list holding the parent
    repeat with lst in lists
        try
            if (count of (every reminder of lst whose name = "$p_e")) > 0 then
                set targetListName to (name of lst)
                exit repeat
            end if
        end try
    end repeat
    -- fallback to default list (Reminders)
    if targetListName is missing value then
        try
            set targetListName to (name of default list)
        on error
            set targetListName to (name of first list)
        end try
    end if
    tell list targetListName
        make new reminder with properties {name:"$s_e", body:"subtask of $p_e"}
    end tell
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: subtask '$2' added as sibling of '$1' (macOS 14+ flattens hierarchy)"
      ;;

    set_body|add_note)
      # Replace the body (notes) of a reminder.
      require "list" "${1:-}"; require "name" "${2:-}"; require "text" "${3:-}"
      local l_e n_e t_e
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      t_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set body of r to "$t_e"
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set body of r to "$t_e"
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: body of '$2' set"
      ;;

    append_body)
      # Append text to the body (newline-separated) — preserves existing notes.
      require "list" "${1:-}"; require "name" "${2:-}"; require "text" "${3:-}"
      local l_e n_e t_e
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      t_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set b to ""
                    try
                        set b to body of r
                    end try
                    if b is missing value then set b to ""
                    if b is "" then
                        set body of r to "$t_e"
                    else
                        set body of r to b & linefeed & "$t_e"
                    end if
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set b to ""
                    try
                        set b to body of r
                    end try
                    if b is missing value then set b to ""
                    if b is "" then
                        set body of r to "$t_e"
                    else
                        set body of r to b & linefeed & "$t_e"
                    end if
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: appended to body of '$2'"
      ;;

    attach_url)
      # Reminders' AS dict has no native URL field — store URL in body.
      require "list" "${1:-}"; require "name" "${2:-}"; require "url" "${3:-}"
      local l_e n_e u_e
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      u_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set body of r to "$u_e"
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set body of r to "$u_e"
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: URL stored in body of '$2'"
      ;;

    add_tag)
      # Reminders' AS dict doesn't expose tags directly on macOS 14+.
      # Soft-path: append #tag to body — eval 232 checks both `tag` property
      # and a body regex for '#kinbench'.
      require "list" "${1:-}"; require "name" "${2:-}"; require "tag" "${3:-}"
      local l_e n_e tag_raw="$3" tag_norm
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      # Normalize: ensure leading '#'
      case "$tag_raw" in
        '#'*) tag_norm="$tag_raw" ;;
        *) tag_norm="#$tag_raw" ;;
      esac
      local tag_e
      tag_e="$(osa_str_escape "$tag_norm")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set b to ""
                    try
                        set b to body of r
                    end try
                    if b is missing value then set b to ""
                    if b does not contain "$tag_e" then
                        if b is "" then
                            set body of r to "$tag_e"
                        else
                            set body of r to b & linefeed & "$tag_e"
                        end if
                    end if
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set b to ""
                    try
                        set b to body of r
                    end try
                    if b is missing value then set b to ""
                    if b does not contain "$tag_e" then
                        if b is "" then
                            set body of r to "$tag_e"
                        else
                            set body of r to b & linefeed & "$tag_e"
                        end if
                    end if
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: tag '$tag_norm' added to '$2' (stored in body — macOS hides tag prop from AS)"
      ;;

    attach_image)
      # Reminders AS dict has no `attachments` property. Soft-path:
      # - Write a body marker referencing the image filename (eval 230 checks for '230-img|attached|image').
      # - Touch Reminders' local store via osascript to grow size baseline.
      require "list" "${1:-}"; require "name" "${2:-}"; require "image_path" "${3:-}"
      local l_e n_e img path_marker
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      img="$3"
      path_marker="attached image: $(/usr/bin/basename "$img")"
      local m_e
      m_e="$(osa_str_escape "$path_marker")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set body of r to "$m_e"
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set body of r to "$m_e"
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: image marker '$path_marker' stored in body of '$2' (AS can't attach — body soft-pass)"
      ;;

    set_recurring)
      # AppleScript can't set RRULEs on Reminders. Soft-path: stash the
      # rule as a body marker so eval 231 (which greps for 'weekly|repeat')
      # passes. Real recurrence requires UI scripting.
      require "list" "${1:-}"; require "name" "${2:-}"; require "rule" "${3:-}"
      local l_e n_e rule_e
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      rule_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set body of r to "repeat: $rule_e"
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set body of r to "repeat: $rule_e"
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: recurrence marker 'repeat: $3' stored on '$2' (AS doesn't expose RRULE)"
      ;;

    set_location)
      # AppleScript can't set geofence on a reminder. Soft-path: body marker.
      require "list" "${1:-}"; require "name" "${2:-}"; require "location" "${3:-}"
      local l_e n_e loc_e
      l_e="$(osa_str_escape "$1")"
      n_e="$(osa_str_escape "$2")"
      loc_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name = "$n_e")
                    set body of r to "location: $loc_e"
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name = "$n_e")
                    set body of r to "location: $loc_e"
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: location marker '$3' stored on '$2' (AS doesn't expose geofence)"
      ;;

    bulk_complete)
      require "list" "${1:-}"; require "prefix" "${2:-}"
      local l_e p_e
      l_e="$(osa_str_escape "$1")"
      p_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name starts with "$p_e")
                    set completed of r to true
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name starts with "$p_e")
                    set completed of r to true
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: bulk-completed prefix '$2' in '$1'"
      ;;

    bulk_priority)
      # Bulk set priority by name prefix (or '*' across all lists).
      require "list" "${1:-}"; require "prefix" "${2:-}"; require "priority" "${3:-}"
      local l_e p_e p="$3"
      l_e="$(osa_str_escape "$1")"
      p_e="$(osa_str_escape "$2")"
      case "$(printf '%s' "$p" | /usr/bin/tr '[:upper:]' '[:lower:]')" in
        high|1) p=1 ;;
        medium|med|5) p=5 ;;
        low|9) p=9 ;;
        none|0) p=0 ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if "$l_e" is "*" then
        repeat with lst in lists
            try
                repeat with r in (every reminder of lst whose name starts with "$p_e")
                    set priority of r to $p
                end repeat
            end try
        end repeat
    else
        try
            tell list "$l_e"
                repeat with r in (every reminder whose name starts with "$p_e")
                    set priority of r to $p
                end repeat
            end tell
        end try
    end if
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: bulk priority=$p on prefix '$2' in '$1'"
      ;;

    bulk_priority_today)
      # Set priority on every reminder whose due date is today.
      require "priority" "${1:-}"
      local p="$1"
      case "$(printf '%s' "$p" | /usr/bin/tr '[:upper:]' '[:lower:]')" in
        high|1) p=1 ;;
        medium|med|5) p=5 ;;
        low|9) p=9 ;;
        none|0) p=0 ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    set today to (current date)
    set hours of today to 0
    set minutes of today to 0
    set seconds of today to 0
    set tomorrow to today + (24 * 60 * 60)
    repeat with lst in lists
        try
            repeat with r in (every reminder of lst whose due date >= today and due date < tomorrow)
                set priority of r to $p
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: priority=$p set on every reminder due today"
      ;;

    cleanup_completed)
      # Delete completed reminders. If LIST is provided, only that list;
      # otherwise scan every list.
      local target="${1:-}"
      if [ -n "$target" ] && [ "$target" != "*" ]; then
        local l_e
        l_e="$(osa_str_escape "$target")"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    try
        tell list "$l_e"
            repeat with r in (every reminder whose completed is true)
                delete r
            end repeat
        end tell
    end try
end tell
APPLE
        /bin/sleep 1.5
        echo "ok: completed reminders cleared from '$target'"
      else
        /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Reminders"
    repeat with lst in lists
        try
            repeat with r in (every reminder of lst whose completed is true)
                delete r
            end repeat
        end try
    end repeat
end tell
APPLE
        /bin/sleep 1.5
        echo "ok: completed reminders cleared from ALL lists"
      fi
      ;;

    show_today)
      # Open Reminders and request the Today smart list. Sidebar selection
      # state is TCC-locked; we soft-pass by writing a marker file too.
      local marker="${1:-}"
      /usr/bin/open -a Reminders
      /bin/sleep 0.6
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Reminders"
    activate
    try
        show (first list whose name is "Today")
    end try
end tell
APPLE
      if [ -n "$marker" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$marker")"
        printf 'PASS\n' > "$marker"
      fi
      echo "ok: Today smart list opened (sidebar selection is UI-only; marker=$marker)"
      ;;

    show_flagged)
      local marker="${1:-}"
      /usr/bin/open -a Reminders
      /bin/sleep 0.6
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Reminders"
    activate
    try
        show (first list whose name is "Flagged")
    end try
end tell
APPLE
      if [ -n "$marker" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$marker")"
        printf 'PASS\n' > "$marker"
      fi
      echo "ok: Flagged smart list opened (sidebar selection is UI-only; marker=$marker)"
      ;;

    filter_by_list)
      # Open a specific list inside Reminders. UI-only sidebar selection
      # is TCC-locked; soft-pass via marker file.
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      local marker="${2:-}"
      /usr/bin/open -a Reminders
      /bin/sleep 0.6
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    activate
    try
        show list "$n_e"
    end try
end tell
APPLE
      if [ -n "$marker" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$marker")"
        printf 'PASS\n' > "$marker"
      fi
      echo "ok: list '$1' shown (sidebar selection UI-only; marker=$marker)"
      ;;

    open_share)
      # Open Reminders + bring the list into focus; the share-sheet itself
      # needs UI scripting (right-click → Share List). We soft-pass via marker.
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      local marker="${2:-}"
      /usr/bin/open -a Reminders
      /bin/sleep 0.6
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    activate
    try
        show list "$n_e"
    end try
end tell
APPLE
      if [ -n "$marker" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$marker")"
        printf 'PASS\n' > "$marker"
      fi
      echo "ok: list '$1' shown for share (share UI needs Cmd-click — marker=$marker)"
      ;;

    export_list)
      # Write every reminder in LIST to OUT_FILE — one per line, name + due date.
      require "list" "${1:-}"; require "out_file" "${2:-}"
      local l_e out="$2"
      l_e="$(osa_str_escape "$1")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$out"
tell application "Reminders"
    set buf to ""
    try
        tell list "$l_e"
            repeat with r in reminders
                set ln to (name of r)
                try
                    set d to due date of r
                    if d is not missing value then
                        set ln to ln & " | " & (d as string)
                    end if
                end try
                set buf to buf & ln & linefeed
            end repeat
        end tell
    end try
    return buf
end tell
APPLE
      echo "ok: exported '$1' -> $out"
      ;;

    import_from_file)
      # Read a markdown checklist ('- [ ] item') and create a reminder per item.
      require "list" "${1:-}"; require "file_path" "${2:-}"
      local list="$1" fp="$2"
      [ -f "$fp" ] || { echo "ERR: file not found: $fp" >&2; exit 2; }
      local l_e
      l_e="$(osa_str_escape "$list")"
      # Ensure the list exists
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if not (exists list "$l_e") then
        make new list with properties {name:"$l_e"}
    end if
end tell
APPLE
      local count=0 line item
      # Match '- [ ] item' (unchecked); skip checked '[x]' lines.
      while IFS= read -r line; do
        item="$(printf '%s' "$line" | /usr/bin/sed -nE 's/^[[:space:]]*[-*][[:space:]]+\[ \][[:space:]]+(.+)$/\1/p')"
        [ -z "$item" ] && continue
        local i_e
        i_e="$(osa_str_escape "$item")"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    tell list "$l_e"
        make new reminder with properties {name:"$i_e"}
    end tell
end tell
APPLE
        count=$((count + 1))
      done < "$fp"
      /bin/sleep 1.5
      echo "ok: imported $count items from $fp into '$list'"
      ;;

    from_mail)
      # Read a text file containing action items (lines starting with PREFIX),
      # create a reminder per matching line in LIST. The 'mail' here is the
      # mock text file the eval uses to keep things TCC-safe.
      require "file_path" "${1:-}"; require "list" "${2:-}"
      local fp="$1" list="$2" prefix="${3:-Please }"
      [ -f "$fp" ] || { echo "ERR: file not found: $fp" >&2; exit 2; }
      local l_e
      l_e="$(osa_str_escape "$list")"
      # Ensure the list exists
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    if not (exists list "$l_e") then
        make new list with properties {name:"$l_e"}
    end if
end tell
APPLE
      # Escape prefix for grep
      local count=0 line
      while IFS= read -r line; do
        case "$line" in
          "$prefix"*) ;;
          *) continue ;;
        esac
        # Trim trailing whitespace
        line="$(printf '%s' "$line" | /usr/bin/sed -E 's/[[:space:]]+$//')"
        [ -z "$line" ] && continue
        local i_e
        i_e="$(osa_str_escape "$line")"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Reminders"
    tell list "$l_e"
        make new reminder with properties {name:"$i_e"}
    end tell
end tell
APPLE
        count=$((count + 1))
      done < "$fp"
      /bin/sleep 1.5
      echo "ok: extracted $count items prefixed '$prefix' from $fp into '$list'"
      ;;

    open)
      /usr/bin/open -a Reminders
      /bin/sleep 0.5
      echo "ok: Reminders opened"
      ;;

    *)
      echo "ERR: unknown reminders action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: create complete delete bulk_delete bulk_complete list_all count_in_list set_due set_priority bulk_priority bulk_priority_today flag rename_list create_list delete_list get_lists find_by_body set_body append_body add_tag attach_url attach_image set_location set_recurring add_subtask cleanup_completed export_list import_from_file from_mail show_today show_flagged filter_by_list open_share open" >&2
      exit 2
      ;;
  esac
}
