keynote_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    new)
      require "out_path" "${1:-}"
      local out p_e
      out="$1"
      p_e="$(osa_str_escape "$out")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    set newDoc to make new document
    delay 0.5
    try
        save newDoc in (POSIX file "$p_e")
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: new keynote document -> $out"
      ;;

    open)
      require "path" "${1:-}"
      /usr/bin/open -a Keynote "$1"
      /bin/sleep 2
      echo "ok: opened $1"
      ;;

    save_as_pdf)
      require "src_path" "${1:-}"; require "out_pdf" "${2:-}"
      local src out s_e o_e
      src="$1"; out="$2"
      s_e="$(osa_str_escape "$src")"
      o_e="$(osa_str_escape "$out")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    set theDoc to open (POSIX file "$s_e")
    delay 1.5
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
      echo "ok: exported keynote pdf -> $out"
      ;;

    add_slide)
      # add_slide DOC_PATH TITLE BODY
      require "doc_path" "${1:-}"
      local title body t_e b_e
      title="${2:-}"
      body="${3:-}"
      t_e="$(osa_str_escape "$title")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            set newSlide to make new slide
            try
                set the object text of (first text item of newSlide) to "$t_e"
            end try
            if "$b_e" is not "" then
                try
                    set the object text of (second text item of newSlide) to "$b_e"
                end try
            end if
        end tell
    on error errMsg
        log errMsg
    end try
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: added slide"
      ;;

    set_theme)
      # set_theme DOC_PATH THEME_NAME
      require "doc_path" "${1:-}"; require "theme" "${2:-}"
      local theme th_e
      theme="$2"
      th_e="$(osa_str_escape "$theme")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            set tt to theme "$th_e"
            set its theme to tt
        end tell
    on error errMsg
        log errMsg
    end try
end tell
APPLE
      /bin/sleep 0.4
      echo "ok: theme set to $theme"
      ;;

    play_presentation)
      require "doc_path" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Keynote"
    activate
    try
        start front document from first slide of front document
    end try
end tell
APPLE
      echo "ok: presentation started"
      ;;

    add_image)
      # add_image SLIDE_INDEX IMG_PATH
      require "slide_index" "${1:-}"; require "img_path" "${2:-}"
      local idx img i_e
      idx="$1"; img="$2"
      i_e="$(osa_str_escape "$img")"
      [ -e "$img" ] || { echo "ERR: image $img not found" >&2; exit 2; }
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            tell slide $idx
                make new image with properties {file:(POSIX file "$i_e")}
            end tell
        end tell
    on error errMsg
        log errMsg
    end try
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: image added to slide $idx"
      ;;

    slide_count)
      require "doc_path" "${1:-}"; require "out_file" "${2:-}"
      local out="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      local n
      n="$(/usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Keynote"
    try
        return (count slides of front document)
    on error
        return 0
    end try
end tell
APPLE
)"
      echo "${n:-0}" > "$out"
      echo "ok: slide count -> $out (${n:-0})"
      ;;

    save_as)
      require "doc_path" "${1:-}"; require "new_path" "${2:-}"
      local new_p p_e
      new_p="$2"
      p_e="$(osa_str_escape "$new_p")"
      /bin/mkdir -p "$(/usr/bin/dirname "$new_p")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        save front document in (POSIX file "$p_e")
    end try
end tell
APPLE
      /bin/sleep 1.2
      echo "ok: saved as $new_p"
      ;;

    *)
      echo "ERR: unknown keynote action '$ACTION'. Try: new|open|save_as_pdf|add_slide|set_theme|play_presentation|add_image|slide_count|save_as" >&2
      exit 2
      ;;
  esac
}
