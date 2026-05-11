calendar_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    create_event)
      require "calendar" "${1:-}"; require "summary" "${2:-}"
      require "start" "${3:-}"; require "end" "${4:-}"
      local cal sum start_d end_d loc desc
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      start_d="$(osa_str_escape "$3")"
      end_d="$(osa_str_escape "$4")"
      loc="$(osa_str_escape "${5:-}")"
      desc="$(osa_str_escape "${6:-}")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    set startDate to my parseDate("$start_d")
    set endDate to my parseDate("$end_d")
    set targetCal to missing value
    try
        set targetCal to (first calendar whose name is "$cal" and writable is true)
    on error
        repeat with c in calendars
            if (writable of c) is true then
                set targetCal to c
                exit repeat
            end if
        end repeat
    end try
    if targetCal is missing value then error "no writable calendar"
    tell targetCal
        set newEv to make new event with properties {summary:"$sum", start date:startDate, end date:endDate}
        if "$loc" is not "" then set location of newEv to "$loc"
        if "$desc" is not "" then set description of newEv to "$desc"
    end tell
end tell

on parseDate(s)
    -- accepts "YYYY-MM-DD HH:MM" or "YYYY-MM-DD HH:MM:SS"
    set theDate to (current date)
    set yr to (text 1 thru 4 of s) as integer
    set mo to (text 6 thru 7 of s) as integer
    set dy to (text 9 thru 10 of s) as integer
    set hr to (text 12 thru 13 of s) as integer
    set mn to (text 15 thru 16 of s) as integer
    set year of theDate to yr
    set month of theDate to mo
    set day of theDate to dy
    set hours of theDate to hr
    set minutes of theDate to mn
    set seconds of theDate to 0
    return theDate
end parseDate
APPLE
      echo "ok: created event '$2' in calendar '$1'"
      ;;

    delete_event)
      require "calendar" "${1:-}"; require "summary" "${2:-}"
      local cal sum
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary = "$sum")
                    delete ev
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in (every event of targetCal whose summary = "$sum")
                delete ev
            end repeat
        end try
    end if
end tell
APPLE
      echo "ok: deleted events with summary '$2' from '$1'"
      ;;

    delete_all)
      require "calendar" "${1:-}"; require "prefix" "${2:-}"
      local cal pfx
      cal="$(osa_str_escape "$1")"
      pfx="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary starts with "$pfx")
                    delete ev
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in (every event of targetCal whose summary starts with "$pfx")
                delete ev
            end repeat
        end try
    end if
end tell
APPLE
      echo "ok: bulk deleted events with prefix '$2'"
      ;;

    list_events)
      require "calendar" "${1:-}"; require "out_file" "${2:-}"
      local cal out
      cal="$(osa_str_escape "$1")"
      out="$2"
      local result
      result="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    set buf to ""
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in events of c
                    set buf to buf & (summary of ev) & linefeed
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in events of targetCal
                set buf to buf & (summary of ev) & linefeed
            end repeat
        end try
    end if
    return buf
end tell
APPLE
)"
      printf '%s' "$result" > "$out"
      echo "ok: events of '$1' -> $out"
      ;;

    find_events_with_summary)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q out
      q="$(osa_str_escape "$1")"
      out="$2"
      local result
      result="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    set buf to ""
    repeat with c in calendars
        try
            repeat with ev in (every event of c whose summary contains "$q")
                set buf to buf & (name of c) & "|" & (summary of ev) & "|" & ((start date of ev) as string) & linefeed
            end repeat
        end try
    end repeat
    return buf
end tell
APPLE
)"
      printf '%s' "$result" > "$out"
      echo "ok: search '$1' -> $out"
      ;;

    move_event)
      require "calendar" "${1:-}"; require "summary" "${2:-}"
      require "new_start" "${3:-}"; require "new_end" "${4:-}"
      local cal sum ns ne
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      ns="$(osa_str_escape "$3")"
      ne="$(osa_str_escape "$4")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    set newStart to my parseDate("$ns")
    set newEnd to my parseDate("$ne")
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary = "$sum")
                    set start date of ev to newStart
                    set end date of ev to newEnd
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in (every event of targetCal whose summary = "$sum")
                set start date of ev to newStart
                set end date of ev to newEnd
            end repeat
        end try
    end if
end tell

on parseDate(s)
    set theDate to (current date)
    set yr to (text 1 thru 4 of s) as integer
    set mo to (text 6 thru 7 of s) as integer
    set dy to (text 9 thru 10 of s) as integer
    set hr to (text 12 thru 13 of s) as integer
    set mn to (text 15 thru 16 of s) as integer
    set year of theDate to yr
    set month of theDate to mo
    set day of theDate to dy
    set hours of theDate to hr
    set minutes of theDate to mn
    set seconds of theDate to 0
    return theDate
end parseDate
APPLE
      echo "ok: moved '$2' -> $3 / $4"
      ;;

    set_alarm)
      require "calendar" "${1:-}"; require "summary" "${2:-}"; require "minutes_before" "${3:-}"
      local cal sum mins
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      mins="$3"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary = "$sum")
                    tell ev to make new display alarm at end of display alarms with properties {trigger interval:-$mins}
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in (every event of targetCal whose summary = "$sum")
                tell ev to make new display alarm at end of display alarms with properties {trigger interval:-$mins}
            end repeat
        end try
    end if
end tell
APPLE
      echo "ok: alarm $3min before '$2'"
      ;;

    today)
      require "out_file" "${1:-}"
      local out="$1"
      local result
      result="$(/usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Calendar"
    set today to (current date)
    set hours of today to 0
    set minutes of today to 0
    set seconds of today to 0
    set tomorrow to today + (1 * days)
    set buf to ""
    repeat with c in calendars
        try
            repeat with ev in (every event of c whose start date ≥ today and start date < tomorrow)
                set buf to buf & (summary of ev) & "|" & ((start date of ev) as string) & linefeed
            end repeat
        end try
    end repeat
    return buf
end tell
APPLE
)"
      printf '%s' "$result" > "$out"
      echo "ok: today's events -> $out"
      ;;

    count_events)
      require "calendar" "${1:-}"; require "out_file" "${2:-}"
      local cal out
      cal="$(osa_str_escape "$1")"
      out="$2"
      local result
      result="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    set total to 0
    if "$cal" is "*" then
        repeat with c in calendars
            try
                set total to total + (count of events of c)
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            set total to (count of events of targetCal)
        end try
    end if
    return total as string
end tell
APPLE
)"
      printf '%s\n' "$result" > "$out"
      echo "ok: count of '$1' = $result -> $out"
      ;;

    get_calendars)
      require "out_file" "${1:-}"
      local out="$1"
      local result
      result="$(/usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Calendar"
    set buf to ""
    repeat with c in calendars
        set buf to buf & (name of c) & linefeed
    end repeat
    return buf
end tell
APPLE
)"
      printf '%s' "$result" > "$out"
      echo "ok: calendar names -> $out"
      ;;

    open)
      /usr/bin/open -a Calendar
      /bin/sleep 0.5
      echo "ok: Calendar opened"
      ;;

    *)
      echo "ERR: unknown calendar action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: create_event delete_event delete_all list_events find_events_with_summary move_event set_alarm today count_events get_calendars open" >&2
      exit 2
      ;;
  esac
}
