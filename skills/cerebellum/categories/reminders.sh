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

    *)
      echo "ERR: unknown reminders action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}
