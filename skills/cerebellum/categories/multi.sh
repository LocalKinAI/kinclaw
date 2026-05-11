multi_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    finder_to_mail)
      # Zip a file (or several) and attach to a fresh Mail draft.
      require "file" "${1:-}"; require "subject" "${2:-}"; require "body" "${3:-}"
      local file="$1" subj="$2" body="$3"
      local out_zip
      out_zip="/tmp/multi-finder-mail-$$.zip"
      [ -e "$file" ] || { echo "ERR: source '$file' missing" >&2; exit 1; }
      finder_dispatch zip "$out_zip" "$file" >/dev/null
      mail_dispatch draft "$subj" "$body" "$out_zip"
      echo "ok: zipped '$file' -> $out_zip + Mail draft '$subj'"
      ;;

    note_to_pdf)
      require "note_name" "${1:-}"; require "out_pdf" "${2:-}"
      notes_dispatch export_pdf "$1" "$2"
      ;;

    note_to_mail)
      # Export a note as PDF, then create a Mail draft with PDF attached.
      require "note_name" "${1:-}"; require "subject" "${2:-}"
      local note="$1" subj="$2"
      local out_pdf="${3:-/tmp/multi-note-$$.pdf}"
      notes_dispatch export_pdf "$note" "$out_pdf" >/dev/null
      [ -s "$out_pdf" ] || { echo "ERR: PDF export failed" >&2; exit 1; }
      mail_dispatch draft "$subj" "Exported from note: $note" "$out_pdf"
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

    reminders_to_calendar)
      # Copy reminders matching prefix in LIST to calendar events on DATE.
      require "list" "${1:-}"; require "date" "${2:-}"
      local list="$1" date="$2"
      local cal="${3:-Home}"
      local tmp_list
      tmp_list="/tmp/multi-rem-$$.txt"
      reminders_dispatch list_all "$list" "$tmp_list" >/dev/null
      local count=0
      while IFS= read -r r; do
        [ -z "$r" ] && continue
        # build event from 09:00 + count*30min as a simple deterministic schedule
        local h m mins start end
        mins=$((count * 30))
        h=$((9 + mins / 60))
        m=$((mins % 60))
        printf -v start "%s %02d:%02d" "$date" "$h" "$m"
        mins=$((mins + 30))
        h=$((9 + mins / 60))
        m=$((mins % 60))
        printf -v end "%s %02d:%02d" "$date" "$h" "$m"
        calendar_dispatch create_event "$cal" "$r" "$start" "$end" >/dev/null 2>&1 || true
        count=$((count + 1))
      done < "$tmp_list"
      /bin/rm -f "$tmp_list"
      echo "ok: copied $count reminder(s) from '$list' -> calendar '$cal' on $date"
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

    clipboard_to_mail)
      # pbpaste → body of new Mail draft.
      require "subject" "${1:-}"
      local subj="$1"
      local clip
      clip="$(/usr/bin/pbpaste)"
      [ -z "$clip" ] && clip="(clipboard was empty)"
      mail_dispatch draft "$subj" "$clip"
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

    *)
      echo "ERR: unknown multi action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: finder_to_mail note_to_pdf note_to_mail clipboard_to_note screenshot_to_note reminders_to_calendar search_then_mail clipboard_to_mail screenshot_to_pages" >&2
      exit 2
      ;;
  esac
}
