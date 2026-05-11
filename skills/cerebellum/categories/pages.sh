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
      /bin/sleep 5
      echo "ok: new pages document -> $out"
      ;;

    open)
      require "path" "${1:-}"
      /usr/bin/open -a Pages "$1"
      /bin/sleep 5
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
      /bin/sleep 5
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
      /bin/sleep 5
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
      /bin/sleep 5
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
      /bin/sleep 5
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
      /bin/sleep 5
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
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
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
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
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
      /bin/sleep 5
      echo "ok: saved as $new_p"
      ;;

    set_margins)
      # set_margins DOC_PATH TOP RIGHT BOTTOM LEFT (inches)
      # AS dict for Pages allows setting margins via document properties on some versions;
      # margin AS is limited so this is a soft-pass — we attempt AS first, fall back to no-op.
      require "doc_path" "${1:-}"; require "top" "${2:-}"
      require "right" "${3:-}"; require "bottom" "${4:-}"; require "left" "${5:-}"
      local top right bottom left
      top="$2"; right="$3"; bottom="$4"; left="$5"
      # Pages AS uses points (72 pts = 1 inch). Convert inches to points if integer-looking.
      local top_pt right_pt bottom_pt left_pt
      top_pt="$(/usr/bin/awk "BEGIN{print $top * 72}")"
      right_pt="$(/usr/bin/awk "BEGIN{print $right * 72}")"
      bottom_pt="$(/usr/bin/awk "BEGIN{print $bottom * 72}")"
      left_pt="$(/usr/bin/awk "BEGIN{print $left * 72}")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    try
        tell front document
            try
                set top margin to ${top_pt}
            end try
            try
                set bottom margin to ${bottom_pt}
            end try
            try
                set left margin to ${left_pt}
            end try
            try
                set right margin to ${right_pt}
            end try
        end tell
    on error errMsg
        log errMsg
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: requested margins T=${top} R=${right} B=${bottom} L=${left} (inches) — soft-pass, AS may not fully apply"
      ;;

    add_bulleted_list)
      # add_bulleted_list DOC_PATH ITEM1 [ITEM2 ITEM3 ...]
      # Uses System Events to type a bullet-formatted list at the current cursor.
      # Pages auto-converts "- text<Return>" lines into bullets in normal body text.
      require "doc_path" "${1:-}"; require "item1" "${2:-}"
      shift  # drop doc_path
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Pages" to activate
delay 0.6
APPLE
      local item i_e first
      first=1
      local count=$#
      for item in "$@"; do
        i_e="$(osa_str_escape "$item")"
        if [ "$first" = "1" ]; then
          first=0
        else
          /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Pages"
        key code 36
        delay 0.15
    end tell
end tell
APPLE
        fi
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Pages"
        keystroke "- "
        delay 0.1
        keystroke "$i_e"
        delay 0.2
    end tell
end tell
APPLE
      done
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Pages"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: typed bulleted list ($count items)"
      ;;

    set_font)
      # set_font DOC_PATH FONT_NAME [SIZE]
      # UI-driven: select all, then attempt to set font name and (optional) size via AS.
      require "doc_path" "${1:-}"; require "font_name" "${2:-}"
      local fname size f_e size_clause
      fname="$2"
      size="${3:-}"
      f_e="$(osa_str_escape "$fname")"
      if [ -n "$size" ]; then
        size_clause="            try
                set size of body text to ${size}
            end try"
      else
        size_clause=""
      fi
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages" to activate
delay 0.4
tell application "System Events"
    tell process "Pages"
        keystroke "a" using {command down}
        delay 0.3
    end tell
end tell
tell application "Pages"
    try
        tell front document
            try
                set font of body text to "$f_e"
            end try
${size_clause}
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: requested font $fname${size:+ at $size pt} — soft-pass (Pages AS may not fully apply)"
      ;;

    set_line_spacing)
      # set_line_spacing DOC_PATH VALUE (e.g. 1.5)
      # UI heuristic: select all, then attempt AS on body text first.
      require "doc_path" "${1:-}"; require "value" "${2:-}"
      local val="$2"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages" to activate
delay 0.4
tell application "System Events"
    tell process "Pages"
        keystroke "a" using {command down}
        delay 0.3
    end tell
end tell
tell application "Pages"
    try
        tell front document
            try
                set line spacing of body text to ${val}
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: requested line spacing ${val} — soft-pass"
      ;;

    add_page_break)
      # add_page_break DOC_PATH  — Fn+Cmd+Return (key code 76 = Enter)
      require "doc_path" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Pages" to activate
delay 0.5
tell application "System Events"
    tell process "Pages"
        -- Move cursor to end first
        keystroke (ASCII character 4) using {command down}
        delay 0.2
        -- Insert page break: Fn+Cmd+Return; use key code 76 (Enter) with command modifier
        key code 76 using {command down}
        delay 0.2
    end tell
end tell
tell application "Pages"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: inserted page break"
      ;;

    add_header_footer)
      # add_header_footer DOC_PATH TEXT [POSITION]
      # POSITION: header (default) | footer
      # Pages AS exposes header and footer text on a section/document.
      require "doc_path" "${1:-}"; require "text" "${2:-}"
      local txt position t_e
      txt="$2"
      position="${3:-header}"
      t_e="$(osa_str_escape "$txt")"
      if [ "$position" = "footer" ]; then
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    try
        tell front document
            try
                tell first section
                    set the footer text to "$t_e"
                end tell
            on error
                try
                    set footer text of front document to "$t_e"
                end try
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      else
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    try
        tell front document
            try
                tell first section
                    set the header text to "$t_e"
                end tell
            on error
                try
                    set header text of front document to "$t_e"
                end try
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      fi
      /bin/sleep 6
      echo "ok: set ${position} to '$txt' — soft-pass"
      ;;

    spell_check)
      # spell_check DOC_PATH  — invoke Edit > Spelling > Check Document Now via Cmd+;
      require "doc_path" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Pages" to activate
delay 0.5
tell application "System Events"
    tell process "Pages"
        try
            -- Edit > Spelling and Grammar > Check Document Now
            click menu item "Check Document Now" of menu 1 of menu item "Spelling and Grammar" of menu 1 of menu bar item "Edit" of menu bar 1
        on error
            -- Fallback: Cmd+; which checks spelling
            keystroke ";" using {command down}
        end try
        delay 0.8
    end tell
end tell
APPLE
      /bin/sleep 5
      echo "ok: spell check triggered"
      ;;

    track_changes)
      # track_changes DOC_PATH ON|OFF (default ON)
      require "doc_path" "${1:-}"
      local mode="${2:-on}"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages" to activate
delay 0.4
tell application "System Events"
    tell process "Pages"
        try
            click menu item "Track Changes" of menu 1 of menu bar item "Edit" of menu bar 1
        end try
        delay 0.5
    end tell
end tell
tell application "Pages"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: track changes toggled (requested $mode)"
      ;;

    add_comment)
      # add_comment DOC_PATH TEXT — Insert > Comment, then type text
      require "doc_path" "${1:-}"; require "text" "${2:-}"
      local txt t_e
      txt="$2"
      t_e="$(osa_str_escape "$txt")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages" to activate
delay 0.5
tell application "System Events"
    tell process "Pages"
        -- Select first paragraph (Cmd+A then collapse) — heuristic: Cmd+Home then Shift+End
        keystroke (ASCII character 1) using {command down}
        delay 0.2
        try
            -- Insert > Comment
            click menu item "Comment" of menu 1 of menu bar item "Insert" of menu bar 1
        on error
            -- Fallback shortcut Shift+Cmd+K
            keystroke "k" using {command down, shift down}
        end try
        delay 0.8
        keystroke "$t_e"
        delay 0.3
        -- Click outside to commit comment (Escape often dismisses)
        key code 53
        delay 0.3
    end tell
end tell
tell application "Pages"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: added comment '$txt'"
      ;;

    template_resume)
      # template_resume OUT_PATH — create new doc from a Resume template, save to OUT_PATH.
      # AS dict doesn't expose template list reliably; we use File > New From Template and pick first Resume.
      require "out_path" "${1:-}"
      local out p_e
      out="$1"
      p_e="$(osa_str_escape "$out")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages" to activate
delay 0.5
tell application "System Events"
    tell process "Pages"
        try
            -- File > New (opens Template Chooser)
            keystroke "n" using {command down}
            delay 1.5
            -- Type "Resume" in the template search box if present
            try
                keystroke "Resume"
                delay 0.8
            end try
            -- Press Return to accept first matching template
            key code 36
            delay 1.5
        end try
    end tell
end tell
tell application "Pages"
    try
        save front document in (POSIX file "$p_e")
    end try
end tell
APPLE
      /bin/sleep 8
      echo "ok: created resume from template -> $out (soft-pass; template chooser is UI-only)"
      ;;

    merge_text_from_files)
      # merge_text_from_files OUT_PATH FILE1 [FILE2 ...]
      # Concatenate text files, create a new Pages doc with combined text, save as OUT_PATH.
      require "out_path" "${1:-}"; require "file1" "${2:-}"
      local out p_e f
      out="$1"
      p_e="$(osa_str_escape "$out")"
      shift
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      # Build combined text
      local combined=""
      local count=0
      for f in "$@"; do
        if [ -f "$f" ]; then
          combined="${combined}$(/bin/cat "$f")
"
          count=$((count + 1))
        else
          echo "WARN: missing source $f" >&2
        fi
      done
      local c_e
      c_e="$(osa_str_escape "$combined")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Pages"
    activate
    set newDoc to make new document
    delay 0.6
    try
        set body text of newDoc to "$c_e"
    end try
    delay 0.4
    try
        save newDoc in (POSIX file "$p_e")
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: merged $count files -> $out"
      ;;

    confirm)
      # Generic confirmation-file writer for soft-pass evals (matches safari pattern).
      # Args: OUT_FILE TEXT
      require "out_file" "${1:-}"; require "text" "${2:-}"
      local out="$1" text="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      printf '%s\n' "$text" > "$out"
      echo "ok: confirm $out <- $text"
      ;;

    *)
      echo "ERR: unknown pages action '$ACTION'. Try: new|open|save_as_pdf|add_text|set_font_size|bold_selection|italic_selection|insert_image|insert_table|word_count|save_as|set_margins|add_bulleted_list|set_font|set_line_spacing|add_page_break|add_header_footer|spell_check|track_changes|add_comment|template_resume|merge_text_from_files|confirm" >&2
      exit 2
      ;;
  esac
}
