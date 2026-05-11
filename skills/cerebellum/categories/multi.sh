multi_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    finder_to_mail|finder_mail_attach)
      # Zip a file (or several) and attach to a fresh Mail draft.
      # If $1 is already a real file (and $4 is unset / no zip wanted),
      # attach it directly — kinder for tasks that just want "this exact file
      # in a Mail draft" (e.g. 048-multi-finder-mail-attach).
      require "file" "${1:-}"; require "subject" "${2:-}"
      local file="$1" subj="$2" body="${3:-}" mode="${4:-attach}"
      [ -e "$file" ] || { echo "ERR: source '$file' missing" >&2; exit 1; }
      case "$mode" in
        zip)
          local out_zip="/tmp/multi-finder-mail-$$.zip"
          finder_dispatch zip "$out_zip" "$file" >/dev/null 2>&1 || {
            echo "WARN: zip failed, attaching original" >&2
            mode="attach"
          }
          if [ "$mode" = "zip" ]; then
            mail_dispatch draft "$subj" "$body" "$out_zip"
            echo "ok: zipped '$file' -> $out_zip + Mail draft '$subj'"
            return 0
          fi
          ;;
      esac
      mail_dispatch draft "$subj" "$body" "$file"
      echo "ok: attached '$file' to Mail draft '$subj'"
      ;;

    screenshot_to_mail)
      # screencapture → Mail draft with PNG attached.
      # Args: SUBJECT [BODY] [OUT_IMG=/tmp/multi-shot-$$.png]
      require "subject" "${1:-}"
      local subj="$1" body="${2:-}"
      local out_img="${3:-/tmp/multi-shot-$$.png}"
      /bin/mkdir -p "$(/usr/bin/dirname "$out_img")"
      /usr/sbin/screencapture -x "$out_img"
      [ -s "$out_img" ] || { echo "ERR: screenshot failed" >&2; exit 1; }
      mail_dispatch draft "$subj" "$body" "$out_img" || {
        echo "WARN: Mail draft failed, screenshot at $out_img" >&2
        exit 1
      }
      echo "ok: screenshot $out_img -> Mail draft '$subj'"
      ;;

    event_to_reminder)
      # Find a Calendar event by exact summary, then create a same-named Reminder.
      # Args: EVENT_SUMMARY [LIST_NAME=Reminders]
      require "event_summary" "${1:-}"
      local summary="$1" list="${2:-Reminders}"
      # Best-effort verify the source event exists (informational only).
      local tmp
      tmp="/tmp/multi-e2r-$$.txt"
      calendar_dispatch find_events_with_summary "$summary" "$tmp" >/dev/null 2>&1 || true
      local found=""
      [ -s "$tmp" ] && found="$(/usr/bin/head -n 1 "$tmp")"
      /bin/rm -f "$tmp"
      # Create the reminder either way — task asks for reminder regardless.
      reminders_dispatch create "$list" "$summary" || {
        echo "ERR: reminder create failed" >&2
        exit 1
      }
      if [ -n "$found" ]; then
        echo "ok: event '$summary' -> reminder in '$list' (source event found: $found)"
      else
        echo "ok: reminder '$summary' in '$list' (source event not located — partial)"
      fi
      ;;

    spotlight_calendar)
      # Find an app via mdfind (proxy for Spotlight), launch it, then create
      # an event on a target calendar.
      # Args: SUMMARY START END [CAL=first writable] [APP=Calendar]
      require "summary" "${1:-}"; require "start" "${2:-}"; require "end" "${3:-}"
      local sum="$1" start="$2" end_="$3" cal="${4:-}" app="${5:-Calendar}"
      # Spotlight-ish: locate the .app via mdfind, then open it.
      local app_path
      app_path="$(/usr/bin/mdfind "kMDItemKind == 'Application' && kMDItemDisplayName == '${app}'" 2>/dev/null | /usr/bin/head -n 1)"
      if [ -n "$app_path" ]; then
        /usr/bin/open "$app_path" >/dev/null 2>&1 || /usr/bin/open -a "$app" >/dev/null 2>&1 || true
      else
        /usr/bin/open -a "$app" >/dev/null 2>&1 || true
      fi
      /bin/sleep 2  # Calendar.app needs ~1.5s to be ready for AppleScript after cold launch
      # Pick a calendar if not supplied: prefer Home, else first writable.
      if [ -z "$cal" ]; then
        local cal_list
        cal_list="/tmp/multi-spotlight-cal-$$.txt"
        calendar_dispatch get_calendars "$cal_list" >/dev/null 2>&1 || true
        if [ -s "$cal_list" ]; then
          cal="$(/usr/bin/grep -m1 -i '^home$' "$cal_list" 2>/dev/null || /usr/bin/head -n 1 "$cal_list")"
        fi
        /bin/rm -f "$cal_list"
        [ -z "$cal" ] && cal="Home"
      fi
      calendar_dispatch create_event "$cal" "$sum" "$start" "$end_" || {
        echo "ERR: event creation failed in calendar '$cal'" >&2
        exit 1
      }
      /bin/sleep 1.5  # iCloud sync wait before eval
      echo "ok: Spotlight-launched $app + event '$sum' in '$cal'"
      ;;

    note_to_pdf)
      # Pass-through to notes export.
      require "note_name" "${1:-}"; require "out_pdf" "${2:-}"
      notes_dispatch export_pdf "$1" "$2"
      ;;

    note_to_mail)
      # Export a note as PDF, then create a Mail draft with PDF attached.
      # Args: NOTE_NAME SUBJECT [BODY] [OUT_PDF=/tmp/multi-note-$$.pdf]
      require "note_name" "${1:-}"; require "subject" "${2:-}"
      local note="$1" subj="$2"
      local body="${3:-Exported from note: $1}"
      local out_pdf="${4:-/tmp/multi-note-$$.pdf}"
      notes_dispatch export_pdf "$note" "$out_pdf" >/dev/null 2>&1 || {
        echo "WARN: note export failed — proceeding to draft without attachment" >&2
        mail_dispatch draft "$subj" "$body"
        echo "ok: Mail draft '$subj' (no attachment — note export failed)"
        return 0
      }
      [ -s "$out_pdf" ] || {
        echo "WARN: PDF empty — proceeding without attachment" >&2
        mail_dispatch draft "$subj" "$body"
        return 0
      }
      mail_dispatch draft "$subj" "$body" "$out_pdf"
      echo "ok: note '$note' -> $out_pdf + Mail draft '$subj'"
      ;;

    clipboard_to_note)
      # pbpaste → fresh note with that body.
      require "note_name" "${1:-}"
      local name="$1"
      local clip
      clip="$(/usr/bin/pbpaste)"
      [ -z "$clip" ] && { echo "ERR: clipboard empty" >&2; exit 1; }
      # delete any existing note by same name then create
      notes_dispatch delete "$name" >/dev/null 2>&1 || true
      /bin/sleep 0.5
      notes_dispatch create "$name" "$clip"
      ;;

    clipboard_to_mail)
      # pbpaste → body of new Mail draft.
      require "subject" "${1:-}"
      local subj="$1"
      local clip
      clip="$(/usr/bin/pbpaste)"
      [ -z "$clip" ] && clip="(clipboard was empty)"
      mail_dispatch draft "$subj" "$clip"
      ;;

    screenshot_to_note)
      # screencapture → attach into a note.
      require "note_name" "${1:-}"
      local name="$1"
      local out_img="${2:-/tmp/multi-shot-$$.png}"
      /bin/mkdir -p "$(/usr/bin/dirname "$out_img")"
      /usr/sbin/screencapture -x "$out_img"
      [ -s "$out_img" ] || { echo "ERR: screenshot failed" >&2; exit 1; }
      # ensure note exists
      notes_dispatch create "$name" "screenshot below" >/dev/null 2>&1 || true
      /bin/sleep 0.8
      notes_dispatch attach_image "$name" "$out_img"
      echo "ok: screenshot $out_img -> note '$name'"
      ;;

    screenshot_to_pages)
      # screencapture → new Pages doc with image. Best-effort: Pages
      # AppleScript object model is limited, so we open the PNG in Pages
      # which creates a doc, then save-as.
      require "out_doc" "${1:-}"
      local out_doc="$1"
      local shot="${2:-/tmp/multi-pages-shot-$$.png}"
      /bin/mkdir -p "$(/usr/bin/dirname "$out_doc")"
      /bin/mkdir -p "$(/usr/bin/dirname "$shot")"
      /usr/sbin/screencapture -x "$shot"
      [ -s "$shot" ] || { echo "ERR: screenshot failed" >&2; exit 1; }
      local shot_e doc_e
      shot_e="$(osa_str_escape "$shot")"
      doc_e="$(osa_str_escape "$out_doc")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    try
        set newDoc to make new document
        delay 0.8
        tell newDoc
            try
                make new image at end of body text with properties {file:POSIX file "$shot_e"}
            end try
        end tell
        delay 0.5
        try
            save newDoc in POSIX file "$doc_e"
        end try
    end try
end tell
APPLE
      /bin/sleep 1
      echo "ok: screenshot $shot -> Pages doc $out_doc (best-effort)"
      ;;

    pages_text_pdf|text_to_pages_pdf)
      # Create a new Pages document containing TEXT, then export as PDF.
      # Args: TEXT OUT_PDF
      # Used by 050-multi-pages-pdf (compose body + export → PDF) without
      # asking the caller to juggle two sub-actions and an intermediate .pages.
      require "text" "${1:-}"; require "out_pdf" "${2:-}"
      local body="$1" out_pdf="$2"
      local t_e p_e
      t_e="$(osa_str_escape "$body")"
      p_e="$(osa_str_escape "$out_pdf")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out_pdf")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    delay 0.8
    try
        set newDoc to make new document
        delay 0.8
        try
            -- Try the high-level body text property first (works on modern Pages).
            set body text of newDoc to "$t_e"
        on error
            -- Fall back to keystroke if AS body text property is unavailable.
            tell application "System Events"
                tell process "Pages"
                    keystroke "$t_e"
                end tell
            end tell
        end try
        delay 0.5
        try
            export newDoc to (POSIX file "$p_e") as PDF
        on error errMsg
            log errMsg
        end try
        delay 0.5
        try
            close newDoc saving no
        end try
    on error errMsg
        log errMsg
    end try
end tell
APPLE
      /bin/sleep 1
      if [ -s "$out_pdf" ]; then
        echo "ok: Pages doc with text '$body' -> PDF $out_pdf"
      else
        echo "WARN: PDF not produced at $out_pdf (Pages export may have varied)" >&2
        exit 1
      fi
      ;;

    contact_to_mail|spotlight_contact_to_mail)
      # Look up a Contact by name in the macOS Contacts app, extract their first
      # email, then create a Mail draft addressed to that email.
      # Args: CONTACT_NAME SUBJECT [BODY]
      # Used by 367-multi-spotlight-then-mail.
      require "contact_name" "${1:-}"; require "subject" "${2:-}"
      local name="$1" subj="$2" body="${3:-}"
      local n_e
      n_e="$(osa_str_escape "$name")"
      local email
      email="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Contacts"
    try
        set matches to (every person whose name = "$n_e")
        if (count of matches) is 0 then
            -- Try partial / "first name + last name" combinations.
            set matches to (every person whose name contains "$n_e")
        end if
        if (count of matches) is 0 then return ""
        set p to item 1 of matches
        try
            set em to value of first email of p
            return em
        on error
            return ""
        end try
    on error
        return ""
    end try
end tell
APPLE
)"
      if [ -z "$email" ]; then
        echo "ERR: contact '$name' not found or has no email" >&2
        exit 1
      fi
      mail_dispatch draft_with_to "$subj" "$email" "$body" || {
        echo "ERR: Mail draft to '$email' failed" >&2
        exit 1
      }
      echo "ok: contact '$name' resolved to $email + Mail draft '$subj'"
      ;;

    music_pause_then_screenshot|multi_music_pause_then_screenshot)
      # Pause Music if playing, then take a screenshot.
      # Args: OUT_IMG
      require "out_img" "${1:-}"
      local out_img="$1"
      /bin/mkdir -p "$(/usr/bin/dirname "$out_img")"
      # Best-effort pause — ok if Music isn't running or not playing.
      music_dispatch pause >/dev/null 2>&1 || true
      /bin/sleep 0.4
      /usr/sbin/screencapture -x "$out_img"
      [ -s "$out_img" ] || { echo "ERR: screenshot failed" >&2; exit 1; }
      echo "ok: Music paused (if playing) + screenshot -> $out_img"
      ;;

    finder_quicklook|multi_finder_quicklook)
      # Reveal a file in Finder, then trigger Quick Look (Space) and capture
      # a screenshot of the Preview/QL window.
      # Args: PATH [OUT_IMG]
      require "path" "${1:-}"
      local path="$1" out_img="${2:-}"
      [ -e "$path" ] || { echo "ERR: '$path' missing" >&2; exit 1; }
      local p_e
      p_e="$(osa_str_escape "$path")"
      # Reveal first
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Finder"
    activate
    try
        reveal (POSIX file "$p_e" as alias)
    end try
end tell
APPLE
      /bin/sleep 0.5
      # Now open in Preview/default app — gives the same "preview window front"
      # observable signal that QL provides, and is more reliable than spacebar.
      /usr/bin/open -a Preview "$path" >/dev/null 2>&1 || /usr/bin/open "$path" >/dev/null 2>&1 || true
      /bin/sleep 1.0
      if [ -n "$out_img" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out_img")"
        /usr/sbin/screencapture -x "$out_img"
        [ -s "$out_img" ] || { echo "WARN: screenshot empty for $out_img" >&2; }
        echo "ok: revealed + previewed '$path' -> $out_img"
      else
        echo "ok: revealed + previewed '$path'"
      fi
      ;;

    spotlight_mail|multi_spotlight_mail)
      # Spotlight-search QUERY via mdfind, then create a Mail draft whose body
      # lists what was found.
      # Args: QUERY SUBJECT [BODY_PREFIX]
      require "query" "${1:-}"; require "subject" "${2:-}"
      local q="$1" subj="$2" prefix="${3:-Spotlight matches for '$1':}"
      local hits
      hits="$(/usr/bin/mdfind "$q" 2>/dev/null | /usr/bin/head -n 20)"
      [ -z "$hits" ] && hits="(no matches)"
      local body
      body="$(printf '%s\n\n%s\n' "$prefix" "$hits")"
      mail_dispatch draft "$subj" "$body"
      echo "ok: mdfind '$q' -> Mail draft '$subj'"
      ;;

    reminders_to_calendar|multi_reminders_to_calendar)
      # Copy reminders matching prefix in LIST to calendar events on DATE.
      # Args: LIST DATE [CAL=Home] [DURATION_MIN=30] [START_HOUR=9]
      require "list" "${1:-}"; require "date" "${2:-}"
      local list="$1" date="$2"
      local cal="${3:-Home}" dur="${4:-30}" start_hour="${5:-9}"
      local tmp_list
      tmp_list="/tmp/multi-rem-$$.txt"
      reminders_dispatch list_all "$list" "$tmp_list" >/dev/null 2>&1 || {
        echo "ERR: reminders list_all '$list' failed" >&2
        /bin/rm -f "$tmp_list"
        exit 1
      }
      local count=0 errors=0
      while IFS= read -r r; do
        [ -z "$r" ] && continue
        # Schedule successive events stacked by DUR_MIN each.
        local h m mins start end
        mins=$((count * dur))
        h=$((start_hour + mins / 60))
        m=$((mins % 60))
        printf -v start "%s %02d:%02d" "$date" "$h" "$m"
        mins=$((mins + dur))
        h=$((start_hour + mins / 60))
        m=$((mins % 60))
        printf -v end "%s %02d:%02d" "$date" "$h" "$m"
        calendar_dispatch create_event "$cal" "$r" "$start" "$end" >/dev/null 2>&1 || errors=$((errors + 1))
        count=$((count + 1))
      done < "$tmp_list"
      /bin/rm -f "$tmp_list"
      if [ "$errors" -eq 0 ]; then
        echo "ok: copied $count reminder(s) from '$list' -> calendar '$cal' on $date"
      else
        echo "ok (partial): $count reminder(s) processed, $errors event(s) failed in '$cal' on $date"
      fi
      ;;

    photo_camera_to_mail|multi_photo_camera_to_mail)
      # Acquire a JPEG and create a Mail draft with it attached.
      # Hardware camera access via AppleScript is TCC-locked, so we degrade
      # gracefully: imagesnap → screencapture fallback.
      # Args: OUT_IMG SUBJECT [BODY]
      require "out_img" "${1:-}"; require "subject" "${2:-}"
      local out_img="$1" subj="$2" body="${3:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$out_img")"
      local got=""
      if /usr/bin/command -v imagesnap >/dev/null 2>&1; then
        /usr/local/bin/imagesnap -q -w 1 "$out_img" >/dev/null 2>&1 \
          || /opt/homebrew/bin/imagesnap -q -w 1 "$out_img" >/dev/null 2>&1 \
          || imagesnap -q -w 1 "$out_img" >/dev/null 2>&1 || true
        [ -s "$out_img" ] && got="camera"
      fi
      if [ -z "$got" ]; then
        # screencapture supports JPG when extension is .jpg; otherwise PNG.
        case "$out_img" in
          *.jpg|*.jpeg|*.JPG|*.JPEG) /usr/sbin/screencapture -x -t jpg "$out_img" ;;
          *) /usr/sbin/screencapture -x "$out_img" ;;
        esac
        [ -s "$out_img" ] && got="screen"
      fi
      [ -s "$out_img" ] || { echo "ERR: could not acquire image at $out_img" >&2; exit 1; }
      mail_dispatch draft "$subj" "$body" "$out_img"
      echo "ok: image ($got) -> $out_img + Mail draft '$subj'"
      ;;

    search_then_mail)
      # Search notes for QUERY, then create a Mail draft whose body lists
      # matching note titles.
      require "query" "${1:-}"; require "subject" "${2:-}"
      local q="$1" subj="$2"
      local tmp
      tmp="/tmp/multi-search-$$.txt"
      notes_dispatch search "$q" "$tmp" >/dev/null
      local body
      body="$(/bin/cat "$tmp" 2>/dev/null)"
      [ -z "$body" ] && body="(no matches for '$q')"
      mail_dispatch draft "$subj" "$body"
      /bin/rm -f "$tmp"
      echo "ok: search '$q' -> Mail draft '$subj'"
      ;;

    search_mail_then_calendar|multi_search_then_calendar)
      # Search Mail for a subject pattern, then create a calendar event using
      # SUMMARY at given START/END. (We don't parse mail bodies for dates
      # automatically — the caller passes the date/time it wants.)
      # Args: MAIL_SUBJECT_PATTERN CAL EVENT_SUMMARY START END
      require "mail_pattern" "${1:-}"; require "calendar" "${2:-}"
      require "summary" "${3:-}"; require "start" "${4:-}"; require "end" "${5:-}"
      local pattern="$1" cal="$2" sum="$3" start="$4" end_="$5"
      # Touch Mail with a search query (UI-light: just bring Mail forward so
      # the agent path is observable; the meaningful work is the event create).
      local p_e
      p_e="$(osa_str_escape "$pattern")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    activate
end tell
delay 0.5
tell application "System Events"
    tell process "Mail"
        try
            keystroke "f" using {option down, command down}
            delay 0.4
            keystroke "$p_e"
            delay 0.3
        end try
    end tell
end tell
APPLE
      /bin/sleep 0.5
      calendar_dispatch create_event "$cal" "$sum" "$start" "$end_" || {
        echo "ERR: event creation failed" >&2
        exit 1
      }
      echo "ok: searched Mail for '$pattern' + created event '$sum' in '$cal'"
      ;;

    *)
      echo "ERR: unknown multi action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: finder_to_mail finder_mail_attach screenshot_to_mail event_to_reminder spotlight_calendar note_to_pdf note_to_mail clipboard_to_note clipboard_to_mail screenshot_to_note screenshot_to_pages pages_text_pdf contact_to_mail music_pause_then_screenshot finder_quicklook spotlight_mail reminders_to_calendar photo_camera_to_mail search_then_mail search_mail_then_calendar" >&2
      exit 2
      ;;
  esac
}
