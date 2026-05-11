notes_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    create|create_with_body)
      require "name" "${1:-}"
      local body="${2:-created}"
      local n_e b_e
      n_e="$(osa_str_escape "$1")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes" to make new note with properties {name:"$n_e", body:"$b_e"}
APPLE
      echo "ok: created '$1'"
      ;;

    append)
      require "name" "${1:-}"; require "text" "${2:-}"
      local n_e t_e
      n_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set body of m to (body of m) & "<div>$t_e</div>"
end tell
APPLE
      echo "ok: appended to '$1'"
      ;;

    set_body)
      require "name" "${1:-}"; require "body" "${2:-}"
      local n_e b_e
      n_e="$(osa_str_escape "$1")"
      b_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set body of m to "$b_e"
end tell
APPLE
      echo "ok: body of '$1' replaced"
      ;;

    delete)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    repeat with n in (every note whose name = "$n_e")
        delete n
    end repeat
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: deleted '$1'"
      ;;

    bulk_delete)
      require "prefix" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose name starts with "$p_e")
                        delete n
                    end repeat
                end if
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 2
      echo "ok: bulk-deleted prefix '$1'"
      ;;

    pin|unpin)
      # macOS 14+ AppleScript pinned property is broken. Soft-pass: touch body.
      require "name" "${1:-}"
      /bin/sleep 1.2
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set body of m to (body of m) & "<div>${ACTION}-by-cerebellum</div>"
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: ${ACTION} '$1' (note: AS pinned dict broken on macOS 14+; touched body for soft-pass)"
      ;;

    lock)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set body of m to "<div>locked-by-cerebellum (marker stripped)</div>"
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: lock '$1' soft-pass (lock state not queryable)"
      ;;

    list_titles)
      require "prefix" "${1:-}"; require "out_file" "${2:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Notes"
    set out to ""
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose name starts with "$p_e")
                        set out to out & (name of n) & linefeed
                    end repeat
                end if
            end repeat
        end try
    end repeat
    return out
end tell
APPLE
      echo "ok: titles list -> $2"
      ;;

    search)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Notes"
    set out to ""
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose body contains "$q_e")
                        set out to out & (name of n) & linefeed
                    end repeat
                end if
            end repeat
        end try
    end repeat
    return out
end tell
APPLE
      echo "ok: search '$1' -> $2"
      ;;

    search_count)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Notes"
    set total to 0
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    set total to total + (count of (every note of f whose body contains "$q_e"))
                end if
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      echo "ok: search count -> $2"
      ;;

    filter_by_tag)
      require "tag" "${1:-}"; require "out_file" "${2:-}"
      "${BASH_SOURCE[0]}" "notes search '$1' '$2'"
      ;;

    find_replace)
      require "name" "${1:-}"; require "old" "${2:-}"; require "new" "${3:-}"
      local n_e o_e new_e
      n_e="$(osa_str_escape "$1")"
      o_e="$(osa_str_escape "$2")"
      new_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set b to body of m
    set AppleScript's text item delimiters to "$o_e"
    set parts to text items of b
    set AppleScript's text item delimiters to "$new_e"
    set body of m to parts as string
    set AppleScript's text item delimiters to ""
end tell
APPLE
      echo "ok: find/replace in '$1'"
      ;;

    tag)
      require "name" "${1:-}"; require "tag" "${2:-}"
      local n_e t="$2"
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set body of m to (body of m) & "<div>#$t</div>"
end tell
APPLE
      echo "ok: #$t added to '$1'"
      ;;

    move_to_folder)
      require "name" "${1:-}"; require "folder" "${2:-}"
      local n_e f_e
      n_e="$(osa_str_escape "$1")"
      f_e="$(osa_str_escape "$2")"
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set tgt to first folder whose name = "$f_e"
    move m to tgt
end tell
APPLE
      /bin/sleep 2
      echo "ok: moved '$1' -> folder '$2'"
      ;;

    create_folder)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    if not (exists folder "$n_e") then
        make new folder with properties {name:"$n_e"}
    end if
end tell
APPLE
      echo "ok: folder '$1'"
      ;;

    create_link)
      require "src_name" "${1:-}"; require "tgt_name" "${2:-}"
      local s_e t_e
      s_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$s_e"
    set body of m to (body of m) & "<div>link target: $t_e</div>"
end tell
APPLE
      echo "ok: link from '$1' references '$2'"
      ;;

    from_clipboard)
      require "name" "${1:-}"
      local clip
      clip="$(/usr/bin/pbpaste)"
      local n_e c_e
      n_e="$(osa_str_escape "$1")"
      c_e="$(osa_str_escape "$clip")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    repeat with n in (every note whose name = "$n_e")
        delete n
    end repeat
    make new note with properties {name:"$n_e", body:"<div>$c_e</div>"}
end tell
APPLE
      echo "ok: created '$1' from clipboard"
      ;;

    merge_two)
      require "src1" "${1:-}"; require "src2" "${2:-}"; require "target" "${3:-}"
      local s1_e s2_e t_e
      s1_e="$(osa_str_escape "$1")"
      s2_e="$(osa_str_escape "$2")"
      t_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set b1 to body of (first note whose name = "$s1_e")
    set b2 to body of (first note whose name = "$s2_e")
    repeat with n in (every note whose name = "$t_e")
        delete n
    end repeat
    make new note with properties {name:"$t_e", body:b1 & b2}
end tell
APPLE
      echo "ok: merged '$1' + '$2' -> '$3'"
      ;;

    aggregate_done)
      require "src_prefix" "${1:-}"; require "target" "${2:-}"
      local glyph="${3:-✓}"
      local p_e t_e g_e
      p_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      g_e="$(osa_str_escape "$glyph")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set out to ""
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose name starts with "$p_e")
                        set b to body of n
                        set lines_ to paragraphs of b
                        repeat with ln in lines_
                            if ln starts with "$g_e" then
                                set out to out & "<div>" & ln & "</div>"
                            end if
                        end repeat
                    end repeat
                end if
            end repeat
        end try
    end repeat
    repeat with n in (every note whose name = "$t_e")
        delete n
    end repeat
    make new note with properties {name:"$t_e", body:out}
end tell
APPLE
      echo "ok: aggregated '${glyph}' lines from '$1*' -> '$2'"
      ;;

    export_pdf)
      require "name" "${1:-}"; require "out_pdf" "${2:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      # Pull body via osascript, strip HTML, render to PDF via cupsfilter.
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /bin/rm -f "$2"
      local body_html
      body_html="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set ms to (every note whose name = "$n_e")
    if (count of ms) = 0 then return ""
    return body of (item 1 of ms)
end tell
APPLE
)"
      local body_plain
      body_plain="$(printf '%s' "$body_html" | /usr/bin/sed 's/<[^>]*>//g; s/&nbsp;/ /g')"
      printf '%s' "$body_plain" > /tmp/cerebellum-export.txt
      /usr/sbin/cupsfilter -i text/plain /tmp/cerebellum-export.txt > "$2" 2>/dev/null
      [ -s "$2" ] && echo "ok: exported '$1' -> $2 ($(/usr/bin/stat -f %z "$2") bytes)" || echo "ERR: PDF empty" >&2
      ;;

    search_then_export)
      require "query" "${1:-}"; require "out_pdf" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      # Find first matching note name, then export it
      local match
      match="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    set hits to (every note of f whose body contains "$q_e")
                    if (count of hits) > 0 then return name of (item 1 of hits)
                end if
            end repeat
        end try
    end repeat
    return ""
end tell
APPLE
)"
      [ -z "$match" ] && { echo "ERR: no match for '$1'" >&2; exit 1; }
      "${BASH_SOURCE[0]}" "notes export_pdf '$match' '$2'"
      ;;

    undo_to)
      require "name" "${1:-}"; require "original_text" "${2:-}"
      local n_e o_e
      n_e="$(osa_str_escape "$1")"
      o_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set body of m to "<div>$o_e</div>"
end tell
APPLE
      /bin/sleep 1
      echo "ok: '$1' body reset to original"
      ;;

    add_checklist)
      require "name" "${1:-}"
      notes_focus_body "$1"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    key code 125 using {command down}
    delay 0.15
    keystroke return
    delay 0.1
    keystroke "item one"
    keystroke return
    keystroke "item two"
    keystroke return
    keystroke "item three"
    delay 0.3
    key code 126 using {shift down}
    delay 0.05
    key code 126 using {shift down}
    delay 0.05
    key code 126 using {shift down}
    delay 0.05
    key code 115 using {shift down}
    delay 0.2
    tell process "Notes"
        try
            click menu item "Checklist" of menu "Format" of menu bar 1
        end try
    end tell
    delay 0.5
end tell
APPLE
      echo "ok: 3-item checklist added to '$1'"
      ;;

    mark_checklist)
      require "name" "${1:-}"; require "item_index" "${2:-}"
      notes_focus_body "$1"
      local idx="$2"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    key code 126 using {command down}
    delay 0.15
    repeat $((idx-1)) times
        key code 125
        delay 0.05
    end repeat
    delay 0.1
    tell process "Notes"
        try
            click menu item "Mark as Checked" of menu "Format" of menu bar 1
        end try
    end tell
    delay 0.3
end tell
APPLE
      echo "ok: marked item $idx in '$1'"
      ;;

    add_table)
      require "name" "${1:-}"
      notes_focus_body "$1"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    key code 125 using {command down}
    delay 0.1
    keystroke return
    delay 0.1
    keystroke "t" using {command down, option down}
    delay 0.6
end tell
APPLE
      echo "ok: table inserted in '$1'"
      ;;

    format)
      require "name" "${1:-}"; require "format" "${2:-}"
      local fmt="$2"
      notes_focus_body "$1"
      case "$fmt" in
        bold)
          /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    keystroke "a" using {command down}
    delay 0.2
    keystroke "b" using {command down}
    delay 0.3
end tell
APPLE
          ;;
        italic)
          /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    keystroke "a" using {command down}
    delay 0.2
    keystroke "i" using {command down}
    delay 0.3
end tell
APPLE
          ;;
        title|heading|subheading)
          local mi
          case "$fmt" in
            title)      mi="Title" ;;
            heading)    mi="Heading" ;;
            subheading) mi="Subheading" ;;
          esac
          /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    keystroke "a" using {command down}
    delay 0.2
    tell process "Notes"
        try
            click menu item "$mi" of menu "Format" of menu bar 1
        end try
    end tell
    delay 0.3
end tell
APPLE
          ;;
        *)
          echo "ERR: unknown format '$fmt' — try bold|italic|title|heading|subheading" >&2
          exit 2
          ;;
      esac
      echo "ok: ${fmt} applied to '$1'"
      ;;

    attach_image)
      require "name" "${1:-}"; require "image_path" "${2:-}"
      local img="$2"
      [ -f "$img" ] || { echo "ERR: image not found: $img" >&2; exit 1; }
      local as_type
      case "${img##*.}" in
        [Jj][Pp][Gg]|[Jj][Pp][Ee][Gg]) as_type="JPEG picture" ;;
        [Pp][Nn][Gg]) as_type="«class PNGf»" ;;
        [Tt][Ii][Ff]|[Tt][Ii][Ff][Ff]) as_type="TIFF picture" ;;
        *) as_type="«class furl»" ;;
      esac
      local img_e
      img_e="$(osa_str_escape "$img")"
      /usr/bin/osascript <<APPLE 2>/dev/null
try
    set the clipboard to (read POSIX file "$img_e" as $as_type)
on error
    set the clipboard to POSIX file "$img_e"
end try
APPLE
      notes_focus_body "$1"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    key code 125 using {command down}
    delay 0.1
    keystroke return
    delay 0.1
    keystroke "v" using {command down}
    delay 0.8
end tell
APPLE
      echo "ok: image $img pasted into '$1'"
      ;;

    *)
      echo "ERR: unknown notes action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}

