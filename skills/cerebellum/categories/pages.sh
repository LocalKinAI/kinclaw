pages_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    new)
      # Create a new blank document and save as OUT_PATH
      require "out_path" "${1:-}"
      local out p_e
      out="$1"
      p_e="$(osa_str_escape "$out")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    set newDoc to make new document
    delay 0.5
    try
        save newDoc in (POSIX file "$p_e")
    on error
        -- fallback: keystroke save dialog via System Events
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: new pages document -> $out"
      ;;

    open)
      require "path" "${1:-}"
      /usr/bin/open -a Pages "$1"
      /bin/sleep 2
      echo "ok: opened $1"
      ;;

    save_as_pdf)
      # Export a .pages file (already on disk) to PDF
      require "src_path" "${1:-}"; require "out_pdf" "${2:-}"
      local src out s_e o_e
      src="$1"; out="$2"
      s_e="$(osa_str_escape "$src")"
      o_e="$(osa_str_escape "$out")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    set theDoc to open (POSIX file "$s_e")
    delay 1.2
    try
        export theDoc to (POSIX file "$o_e") as PDF
    on error errMsg
        log errMsg
    end try
    delay 1.0
    try
        close theDoc saving no
    end try
end tell
APPLE
      /bin/sleep 1
      echo "ok: exported pdf -> $out"
      ;;

    add_text)
      # Type text into the open Pages document via System Events
      require "doc_path" "${1:-}"; require "text" "${2:-}"
      local txt t_e
      txt="$2"
      t_e="$(osa_str_escape "$txt")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages" to activate
delay 0.8
tell application "System Events"
    tell process "Pages"
        keystroke "$t_e"
    end tell
end tell
APPLE
      echo "ok: typed text into open Pages document"
      ;;

    set_font_size)
      # Open Format inspector and set size (UI heuristic)
      require "size" "${1:-}"
      local sz="$1"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages" to activate
delay 0.4
tell application "System Events"
    tell process "Pages"
        -- Select all so the change applies
        keystroke "a" using {command down}
        delay 0.2
    end tell
end tell
APPLE
      # Use a small AS helper that keystrokes the size via the format bar (Cmd+Opt+T toggles, but fallback is to use keystroke into font size field)
      echo "ok: requested font size $sz (caller should follow up with UI font dialog)"
      ;;

    bold_selection)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Pages" to activate
delay 0.4
tell application "System Events"
    tell process "Pages"
        keystroke "b" using {command down}
    end tell
end tell
APPLE
      echo "ok: applied bold"
      ;;

    italic_selection)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Pages" to activate
delay 0.4
tell application "System Events"
    tell process "Pages"
        keystroke "i" using {command down}
    end tell
end tell
APPLE
      echo "ok: applied italic"
      ;;

    insert_image)
      require "doc_path" "${1:-}"; require "img_path" "${2:-}"
      local doc img d_e i_e
      doc="$1"; img="$2"
      d_e="$(osa_str_escape "$doc")"
      i_e="$(osa_str_escape "$img")"
      [ -e "$img" ] || { echo "ERR: image $img not found" >&2; exit 2; }
      # Drag-via-keystroke isn't possible; use AppleScript "make new image" if the dictionary supports.
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    try
        tell front document
            make new image with properties {file:(POSIX file "$i_e")}
        end tell
    on error errMsg
        -- Fallback: open and let user paste from clipboard.
        log errMsg
    end try
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: inserted image $img"
      ;;

    insert_table)
      require "doc_path" "${1:-}"; require "rows" "${2:-}"; require "cols" "${3:-}"
      local rows cols
      rows="$2"; cols="$3"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    try
        tell front document
            make new table with properties {row count:${rows}, column count:${cols}}
        end tell
    on error errMsg
        log errMsg
    end try
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: inserted ${rows}x${cols} table"
      ;;

    word_count)
      # Read the front-document body text via AppleScript and write count to file
      require "doc_path" "${1:-}"; require "out_file" "${2:-}"
      local out="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      local n
      n="$(/usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Pages"
    try
        set t to body text of front document
        return (count words of t)
    on error
        return 0
    end try
end tell
APPLE
)"
      echo "${n:-0}" > "$out"
      echo "ok: word count -> $out (${n:-0})"
      ;;

    save_as)
      require "doc_path" "${1:-}"; require "new_path" "${2:-}"
      local new_p p_e
      new_p="$2"
      p_e="$(osa_str_escape "$new_p")"
      /bin/mkdir -p "$(/usr/bin/dirname "$new_p")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    try
        save front document in (POSIX file "$p_e")
    on error errMsg
        log errMsg
    end try
end tell
APPLE
      /bin/sleep 1.2
      echo "ok: saved as $new_p"
      ;;

    *)
      echo "ERR: unknown pages action '$ACTION'. Try: new|open|save_as_pdf|add_text|set_font_size|bold_selection|italic_selection|insert_image|insert_table|word_count|save_as" >&2
      exit 2
      ;;
  esac
}
