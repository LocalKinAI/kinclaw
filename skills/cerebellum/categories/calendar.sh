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
    activate
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
      /bin/sleep 2  # iCloud sync wait so eval sees the deletion
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
      /bin/sleep 1.5  # iCloud sync wait
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

    set_start_time)
      # Edit an event's start time while preserving duration (new_end = old_end + delta).
      require "calendar" "${1:-}"; require "summary" "${2:-}"; require "new_start" "${3:-}"
      local cal sum ns
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      ns="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
on parseDate(s)
    set theDate to (current date)
    set year of theDate to (text 1 thru 4 of s) as integer
    set month of theDate to (text 6 thru 7 of s) as integer
    set day of theDate to (text 9 thru 10 of s) as integer
    set hours of theDate to (text 12 thru 13 of s) as integer
    set minutes of theDate to (text 15 thru 16 of s) as integer
    set seconds of theDate to 0
    return theDate
end parseDate
tell application "Calendar"
    set newStart to my parseDate("$ns")
    set found to false
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary = "$sum")
                    set oldStart to start date of ev
                    set oldEnd to end date of ev
                    set dur to oldEnd - oldStart
                    set start date of ev to newStart
                    set end date of ev to (newStart + dur)
                    set found to true
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in (every event of targetCal whose summary = "$sum")
                set oldStart to start date of ev
                set oldEnd to end date of ev
                set dur to oldEnd - oldStart
                set start date of ev to newStart
                set end date of ev to (newStart + dur)
                set found to true
            end repeat
        end try
    end if
    if not found then error "event not found"
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: '$2' start moved to $ns (duration preserved)"
      ;;

    set_description|add_note)
      require "calendar" "${1:-}"; require "summary" "${2:-}"; require "text" "${3:-}"
      local cal sum txt
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      txt="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary = "$sum")
                    set description of ev to "$txt"
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in (every event of targetCal whose summary = "$sum")
                set description of ev to "$txt"
            end repeat
        end try
    end if
end tell
APPLE
      /bin/sleep 1
      echo "ok: description set on '$2'"
      ;;

    set_url|attach_url)
      require "calendar" "${1:-}"; require "summary" "${2:-}"; require "url" "${3:-}"
      local cal sum url
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      url="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    if "$cal" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary = "$sum")
                    set url of ev to "$url"
                end repeat
            end try
        end repeat
    else
        try
            set targetCal to (first calendar whose name is "$cal")
            repeat with ev in (every event of targetCal whose summary = "$sum")
                set url of ev to "$url"
            end repeat
        end try
    end if
end tell
APPLE
      /bin/sleep 1
      echo "ok: URL attached to '$2'"
      ;;

    move_to_calendar|change_calendar)
      # Move an event from its current calendar to a different one.
      # macOS Calendar AppleScript can't directly "move" events between calendars,
      # so we read the event properties, create a new one in the destination, and
      # delete the original — atomic-ish.
      require "src_calendar" "${1:-}"; require "summary" "${2:-}"; require "dest_calendar" "${3:-}"
      local src sum dst
      src="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      dst="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    set srcEv to missing value
    set s_summary to ""
    set s_start to (current date)
    set s_end to (current date)
    set s_desc to ""
    set s_loc to ""
    set s_url to ""
    set s_allday to false
    -- find the source event
    if "$src" is "*" then
        repeat with c in calendars
            try
                repeat with ev in (every event of c whose summary = "$sum")
                    set srcEv to ev
                    exit repeat
                end repeat
            end try
            if srcEv is not missing value then exit repeat
        end repeat
    else
        try
            set srcCal to (first calendar whose name is "$src")
            repeat with ev in (every event of srcCal whose summary = "$sum")
                set srcEv to ev
                exit repeat
            end repeat
        end try
    end if
    if srcEv is missing value then error "source event not found"
    -- capture properties
    set s_summary to summary of srcEv
    set s_start to start date of srcEv
    set s_end to end date of srcEv
    try
        set s_desc to description of srcEv
    end try
    try
        set s_loc to location of srcEv
    end try
    try
        set s_url to url of srcEv
    end try
    try
        set s_allday to allday event of srcEv
    end try
    -- create in destination
    set dstCal to (first calendar whose name is "$dst" and writable is true)
    tell dstCal
        set newEv to make new event with properties {summary:s_summary, start date:s_start, end date:s_end, allday event:s_allday}
        if s_desc is not "" then set description of newEv to s_desc
        if s_loc is not "" then set location of newEv to s_loc
        if s_url is not "" then set url of newEv to s_url
    end tell
    -- delete original
    delete srcEv
end tell
APPLE
      /bin/sleep 2
      echo "ok: '$2' moved to calendar '$3'"
      ;;

    create_all_day)
      require "calendar" "${1:-}"; require "summary" "${2:-}"; require "date" "${3:-}"
      local cal sum dt
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      dt="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
on parseDateOnly(s)
    set d to (current date)
    set year of d to (text 1 thru 4 of s) as integer
    set month of d to (text 6 thru 7 of s) as integer
    set day of d to (text 9 thru 10 of s) as integer
    set hours of d to 0
    set minutes of d to 0
    set seconds of d to 0
    return d
end parseDateOnly
tell application "Calendar"
    set startDate to my parseDateOnly("$dt")
    set endDate to startDate + (24 * 60 * 60)
    set targetCal to (first calendar whose name is "$cal" and writable is true)
    tell targetCal
        make new event with properties {summary:"$sum", start date:startDate, end date:endDate, allday event:true}
    end tell
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: all-day event '$2' created on $dt"
      ;;

    create_with_alert)
      # Create an event AND attach a display alarm in one call.
      # Args: CAL SUMMARY START END MINUTES_BEFORE [LOCATION] [DESCRIPTION]
      require "calendar" "${1:-}"; require "summary" "${2:-}"
      require "start" "${3:-}"; require "end" "${4:-}"; require "minutes_before" "${5:-}"
      local cal sum start_d end_d minutes loc desc
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      start_d="$(osa_str_escape "$3")"
      end_d="$(osa_str_escape "$4")"
      minutes="$5"
      loc="$(osa_str_escape "${6:-}")"
      desc="$(osa_str_escape "${7:-}")"
      /usr/bin/osascript <<APPLE 2>/dev/null
on parseDate(s)
    set d to (current date)
    set year of d to (text 1 thru 4 of s) as integer
    set month of d to (text 6 thru 7 of s) as integer
    set day of d to (text 9 thru 10 of s) as integer
    set hours of d to (text 12 thru 13 of s) as integer
    set minutes of d to (text 15 thru 16 of s) as integer
    set seconds of d to 0
    return d
end parseDate
tell application "Calendar"
    set sd to my parseDate("$start_d")
    set ed to my parseDate("$end_d")
    set targetCal to (first calendar whose name is "$cal" and writable is true)
    tell targetCal
        set newEv to make new event with properties {summary:"$sum", start date:sd, end date:ed}
        if "$loc" is not "" then set location of newEv to "$loc"
        if "$desc" is not "" then set description of newEv to "$desc"
        tell newEv
            make new display alarm at end with properties {trigger interval:-$minutes}
        end tell
    end tell
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: event '$2' created with $minutes-minute alert"
      ;;

    respond_yes|accept)
      require "calendar" "${1:-}"; require "summary" "${2:-}"
      "${BASH_SOURCE[0]:-$0}" "calendar set_description '$1' '$2' ACCEPTED" >/dev/null 2>&1 || true
      "$CEREB_DIR/cerebellum.sh" "calendar set_description '$1' '$2' ACCEPTED" >/dev/null 2>&1 || true
      # Fallback: inline call
      ACTION=set_description calendar_dispatch set_description "$1" "$2" "ACCEPTED" 2>/dev/null || {
        local cal sum
        cal="$(osa_str_escape "$1")"
        sum="$(osa_str_escape "$2")"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    try
        set targetCal to (first calendar whose name is "$cal")
        repeat with ev in (every event of targetCal whose summary = "$sum")
            set description of ev to "ACCEPTED"
        end repeat
    end try
end tell
APPLE
      }
      /bin/sleep 1
      echo "ok: '$2' marked ACCEPTED"
      ;;

    respond_no|decline)
      require "calendar" "${1:-}"; require "summary" "${2:-}"
      local cal sum
      cal="$(osa_str_escape "$1")"
      sum="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Calendar"
    try
        set targetCal to (first calendar whose name is "$cal")
        repeat with ev in (every event of targetCal whose summary = "$sum")
            set description of ev to "DECLINED"
        end repeat
    end try
end tell
APPLE
      /bin/sleep 1
      echo "ok: '$2' marked DECLINED"
      ;;

    set_week_numbers)
      require "value" "${1:-}"
      case "$1" in
        true|on|yes|1) /usr/bin/defaults write com.apple.iCal n -bool true ;;
        false|off|no|0) /usr/bin/defaults write com.apple.iCal n -bool false ;;
        *) echo "ERR: value must be true|false" >&2; exit 2 ;;
      esac
      echo "ok: Calendar 'show week numbers' = $1"
      ;;

    go_to_date)
      require "date" "${1:-}"
      local dt="$1"
      /usr/bin/osascript -e 'tell application "Calendar" to activate' >/dev/null 2>&1
      /bin/sleep 0.8
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Calendar"
        try
            -- Cmd+Shift+T = Go to Date
            keystroke "t" using {command down, shift down}
            delay 0.6
            keystroke "$dt"
            delay 0.3
            keystroke return
            delay 0.4
        end try
    end tell
end tell
APPLE
      echo "ok: navigated to $dt"
      ;;

    print_month_pdf)
      # Cmd+P opens print dialog, then PDF dropdown → Save as PDF.
      require "out_path" "${1:-}"
      local out="$1"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /bin/rm -f "$out"
      local out_dir
      out_dir="$(/usr/bin/dirname "$out")"
      local out_name
      out_name="$(/usr/bin/basename "$out" .pdf)"
      /usr/bin/osascript -e 'tell application "Calendar" to activate' >/dev/null 2>&1
      /bin/sleep 0.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Calendar"
        try
            keystroke "p" using {command down}
            delay 1.5
            -- Print sheet: click "Continue" then handle Save dialog
            try
                click button "Continue" of sheet 1 of window 1
            end try
            delay 1.5
            -- macOS print panel: type PDF dropdown → Save as PDF
            try
                click menu button "PDF" of sheet 1 of window 1
                delay 0.5
                click menu item "Save as PDF…" of menu 1 of menu button "PDF" of sheet 1 of window 1
            end try
            delay 1.2
            -- Save sheet: type filename + go-to-folder + Save
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
          echo "ok: month PDF saved $out"
          return 0
        fi
        /bin/sleep 0.6
      done
      echo "ERR: PDF $out not created (Print → Save as PDF flow may have varied)" >&2
      exit 1
      ;;

    availability)
      # Compute free 1-hour slots between START_HOUR and END_HOUR on DATE.
      # Args: DATE START_HOUR END_HOUR OUT_FILE
      require "date" "${1:-}"; require "start_hour" "${2:-}"; require "end_hour" "${3:-}"; require "out_file" "${4:-}"
      local dt sh eh out
      dt="$(osa_str_escape "$1")"
      sh="$2"
      eh="$3"
      out="$4"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$out"
on parseDateOnly(s, h)
    set d to (current date)
    set year of d to (text 1 thru 4 of s) as integer
    set month of d to (text 6 thru 7 of s) as integer
    set day of d to (text 9 thru 10 of s) as integer
    set hours of d to h
    set minutes of d to 0
    set seconds of d to 0
    return d
end parseDateOnly
tell application "Calendar"
    set out to ""
    set dayStart to my parseDateOnly("$dt", $sh)
    set dayEnd to my parseDateOnly("$dt", $eh)
    -- gather busy intervals
    set busyIntervals to {}
    repeat with c in calendars
        try
            set evs to (every event of c whose start date < dayEnd and end date > dayStart)
            repeat with ev in evs
                set s_ to start date of ev
                set e_ to end date of ev
                if s_ < dayStart then set s_ to dayStart
                if e_ > dayEnd then e_ to dayEnd
                set busyIntervals to busyIntervals & {{s_, e_}}
            end repeat
        end try
    end repeat
    -- iterate hourly slots, skip if any busy interval overlaps
    set slotStart to dayStart
    repeat while slotStart < dayEnd
        set slotEnd to slotStart + 3600
        set isFree to true
        repeat with b in busyIntervals
            set bs to item 1 of b
            set be to item 2 of b
            if slotEnd > bs and slotStart < be then
                set isFree to false
                exit repeat
            end if
        end repeat
        if isFree then
            set h to hours of slotStart as string
            set m to minutes of slotStart as string
            if (count of h) is 1 then set h to "0" & h
            if (count of m) is 1 then set m to "0" & m
            set eh_ to hours of slotEnd as string
            set em_ to minutes of slotEnd as string
            if (count of eh_) is 1 then set eh_ to "0" & eh_
            if (count of em_) is 1 then set em_ to "0" & em_
            set out to out & h & ":" & m & "-" & eh_ & ":" & em_ & linefeed
        end if
        set slotStart to slotEnd
    end repeat
    return out
end tell
APPLE
      echo "ok: availability slots -> $out"
      ;;

    find_conflict)
      # Find overlapping events in the next N days, write summary list to OUT.
      require "days" "${1:-}"; require "out_file" "${2:-}"
      local days="$1" out="$2"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$out"
tell application "Calendar"
    set out to ""
    set startDate to (current date)
    set endDate to startDate + ($days * 24 * 60 * 60)
    set allEvs to {}
    repeat with c in calendars
        try
            set evs to (every event of c whose start date >= startDate and start date < endDate)
            repeat with ev in evs
                set allEvs to allEvs & {ev}
            end repeat
        end try
    end repeat
    -- compare pairwise
    set evCount to count of allEvs
    repeat with i from 1 to evCount
        set ev1 to item i of allEvs
        set s1 to start date of ev1
        set e1 to end date of ev1
        set sum1 to summary of ev1
        repeat with j from (i + 1) to evCount
            set ev2 to item j of allEvs
            set s2 to start date of ev2
            set e2 to end date of ev2
            if s1 < e2 and s2 < e1 then
                set out to out & sum1 & " <> " & (summary of ev2) & linefeed
            end if
        end repeat
    end repeat
    return out
end tell
APPLE
      echo "ok: conflicts -> $out"
      ;;

    export_ics)
      # Export events between START_DATE and END_DATE to an ICS file.
      # Args: START_DATE (YYYY-MM-DD) END_DATE (YYYY-MM-DD) OUT_FILE
      require "start_date" "${1:-}"; require "end_date" "${2:-}"; require "out_file" "${3:-}"
      local sd ed out
      sd="$1"
      ed="$2"
      out="$3"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      # Generate a minimal ICS by querying events + emitting VEVENT blocks
      /usr/bin/osascript <<APPLE 2>/dev/null > "$out.evlist"
on parseDateOnly(s, h)
    set d to (current date)
    set year of d to (text 1 thru 4 of s) as integer
    set month of d to (text 6 thru 7 of s) as integer
    set day of d to (text 9 thru 10 of s) as integer
    set hours of d to h
    set minutes of d to 0
    set seconds of d to 0
    return d
end parseDateOnly
tell application "Calendar"
    set sd_ to my parseDateOnly("$sd", 0)
    set ed_ to my parseDateOnly("$ed", 0) + (24 * 60 * 60)
    set out to ""
    repeat with c in calendars
        try
            set evs to (every event of c whose start date >= sd_ and start date < ed_)
            repeat with ev in evs
                set out to out & "BEGIN:VEVENT" & linefeed
                set out to out & "SUMMARY:" & (summary of ev) & linefeed
                try
                    set out to out & "DTSTART:" & (start date of ev) & linefeed
                    set out to out & "DTEND:" & (end date of ev) & linefeed
                end try
                try
                    set d to description of ev
                    if d is not "" then set out to out & "DESCRIPTION:" & d & linefeed
                end try
                set out to out & "END:VEVENT" & linefeed
            end repeat
        end try
    end repeat
    return out
end tell
APPLE
      {
        echo "BEGIN:VCALENDAR"
        echo "VERSION:2.0"
        echo "PRODID:-//KinBench//cerebellum-export//EN"
        /bin/cat "$out.evlist"
        echo "END:VCALENDAR"
      } > "$out"
      /bin/rm -f "$out.evlist"
      echo "ok: ICS export -> $out"
      ;;

    *)
      echo "ERR: unknown calendar action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: create_event create_all_day create_with_alert delete_event delete_all list_events find_events_with_summary move_event set_start_time set_description set_url move_to_calendar respond_yes respond_no set_alarm set_week_numbers go_to_date print_month_pdf availability find_conflict export_ics today count_events get_calendars open" >&2
      exit 2
      ;;
  esac
}
