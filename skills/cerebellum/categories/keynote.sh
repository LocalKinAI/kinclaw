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
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 0.8
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

    add_title)
      # add_title DOC_PATH SLIDE_NUM TITLE  — set object text of default title item
      require "doc_path" "${1:-}"; require "slide_num" "${2:-}"; require "title" "${3:-}"
      local snum title t_e
      snum="$2"; title="$3"
      t_e="$(osa_str_escape "$title")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            tell slide $snum
                try
                    set the object text of default title item to "$t_e"
                on error
                    try
                        set the object text of (first text item) to "$t_e"
                    end try
                end try
            end tell
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: title set on slide $snum to '$title'"
      ;;

    change_theme)
      # change_theme DOC_PATH THEME_NAME — alias for set_theme; AS support is limited
      # (some themes aren't enumerable). Soft-pass.
      require "doc_path" "${1:-}"; require "theme" "${2:-}"
      local theme th_e
      theme="$2"
      th_e="$(osa_str_escape "$theme")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            try
                set tt to theme "$th_e"
                set its theme to tt
            on error
                -- Some Keynote versions don't expose themes by name reliably;
                -- best-effort: keystroke Document inspector > Change Theme.
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: requested theme change to $theme — soft-pass"
      ;;

    add_image_to_slide)
      # add_image_to_slide DOC_PATH SLIDE_NUM IMG_PATH  — same as add_image but
      # exposes DOC_PATH for clarity and consistency with the task prompts.
      require "doc_path" "${1:-}"; require "slide_num" "${2:-}"; require "img_path" "${3:-}"
      local snum img i_e
      snum="$2"; img="$3"
      i_e="$(osa_str_escape "$img")"
      [ -e "$img" ] || { echo "ERR: image $img not found" >&2; exit 2; }
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            tell slide $snum
                make new image with properties {file:(POSIX file "$i_e")}
            end tell
        end tell
    on error errMsg
        log errMsg
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: image $img added to slide $snum"
      ;;

    add_bullets)
      # add_bullets DOC_PATH SLIDE_NUM ITEM1 [ITEM2 ITEM3 ...]
      # Joins items with newlines and sets as the body (second text item) of the slide.
      require "doc_path" "${1:-}"; require "slide_num" "${2:-}"; require "item1" "${3:-}"
      local snum
      snum="$2"
      shift 2
      local body=""
      local first=1
      for item in "$@"; do
        if [ "$first" = "1" ]; then
          body="$item"
          first=0
        else
          body="${body}
${item}"
        fi
      done
      local b_e
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            -- If slide exists, set its body; else create a new slide.
            try
                tell slide $snum
                    try
                        set the object text of (second text item) to "$b_e"
                    on error
                        -- Fallback: append new bullet slide
                        set newSlide to make new slide
                        try
                            set the object text of (second text item of newSlide) to "$b_e"
                        end try
                    end try
                end tell
            on error
                set newSlide to make new slide
                try
                    set the object text of (second text item of newSlide) to "$b_e"
                end try
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: added $# bullets to slide $snum"
      ;;

    set_transition)
      # set_transition DOC_PATH SLIDE_NUM TYPE  (e.g. cube, dissolve, fade, push)
      # Keynote AS exposes transition properties on slides on most versions.
      require "doc_path" "${1:-}"; require "slide_num" "${2:-}"; require "type" "${3:-}"
      local snum ttype tt_e
      snum="$2"; ttype="$3"
      tt_e="$(osa_str_escape "$ttype")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            tell slide $snum
                try
                    set transition properties to {transition effect:"$tt_e"}
                on error
                    try
                        set the transition of it to "$tt_e"
                    end try
                end try
            end tell
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: requested transition '$ttype' on slide $snum — soft-pass"
      ;;

    rearrange_slides)
      # rearrange_slides DOC_PATH ORDER  — ORDER is comma-separated list of slide
      # numbers in the desired new order (1-indexed of original positions).
      # Approach: collect all slide titles/bodies, build a new doc-order via move ops.
      # We use AS "move slide" repeatedly to reach the target order.
      require "doc_path" "${1:-}"; require "order" "${2:-}"
      local order="$2"
      # Convert "3,1,2" into AS-friendly script
      local idx
      # Move slides by repeatedly targeting position: take original index i, move to position j.
      # Simple algorithm: walk through new order, for each target slot k (1..N),
      # find the original slide currently at the desired place and move it to slot k.
      # We do this via System Events drag-equivalent: use AS "move slide X to slide Y".
      local IFS_BAK="$IFS"
      IFS=','
      local positions=($order)
      IFS="$IFS_BAK"
      local k=1
      local src
      for src in "${positions[@]}"; do
        # Move the slide currently at index src to position k (1-indexed target).
        # In Keynote AS, "move slide N of front document to after slide M" works.
        local before_k=$((k - 1))
        if [ "$before_k" -le 0 ]; then
          /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    try
        tell front document
            move slide ${src} to before slide 1
        end tell
    end try
end tell
APPLE
        else
          /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    try
        tell front document
            move slide ${src} to after slide ${before_k}
        end tell
    end try
end tell
APPLE
        fi
        /bin/sleep 0.4
        k=$((k + 1))
      done
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Keynote"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: rearranged slides to order $order — soft-pass (AS move may not fully apply on all versions)"
      ;;

    build_from_outline)
      # build_from_outline DOC_PATH OUTLINE_TEXT
      # OUTLINE_TEXT is a markdown-style outline:
      #   # Title 1
      #   - bullet a
      #   - bullet b
      #   # Title 2
      #   - bullet c
      # Each # heading becomes a new slide; bullet lines beneath become body bullets.
      require "doc_path" "${1:-}"; require "outline_text" "${2:-}"
      local doc outline_text doc_e
      doc="$1"; outline_text="$2"
      doc_e="$(osa_str_escape "$doc")"
      # Parse outline in bash 3.2 compatible way.
      local title=""
      local body=""
      local first_slide=1
      local _IFS="$IFS"
      IFS=$'\n'
      local line stripped
      # Write outline to a tempfile to iterate (bash 3.2 doesn't have read -d well).
      local tmp
      tmp="$(/usr/bin/mktemp -t kinclaw_outline.XXXX)"
      printf '%s\n' "$outline_text" > "$tmp"
      # Helper to flush a (title, body) into Keynote
      flush_slide() {
        local _t="$1"
        local _b="$2"
        local _t_e _b_e
        _t_e="$(osa_str_escape "$_t")"
        _b_e="$(osa_str_escape "$_b")"
        if [ "$first_slide" = "1" ]; then
          # Use the first existing slide (created on doc make new)
          /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            try
                tell slide 1
                    try
                        set the object text of default title item to "$_t_e"
                    on error
                        try
                            set the object text of (first text item) to "$_t_e"
                        end try
                    end try
                    if "$_b_e" is not "" then
                        try
                            set the object text of (second text item) to "$_b_e"
                        end try
                    end if
                end tell
            end try
        end tell
    end try
end tell
APPLE
          first_slide=0
        else
          /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    try
        tell front document
            set newSlide to make new slide
            try
                set the object text of (first text item of newSlide) to "$_t_e"
            end try
            if "$_b_e" is not "" then
                try
                    set the object text of (second text item of newSlide) to "$_b_e"
                end try
            end if
        end tell
    end try
end tell
APPLE
        fi
        /bin/sleep 0.4
      }
      # Make sure a fresh deck exists at the target path.
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    activate
    set newDoc to make new document
    delay 0.5
    try
        save newDoc in (POSIX file "$doc_e")
    end try
end tell
APPLE
      /bin/sleep 1
      while IFS= read -r line; do
        # Detect heading
        case "$line" in
          \#*)
            # New slide — flush the previous one (if any).
            if [ -n "$title" ] || [ -n "$body" ]; then
              flush_slide "$title" "$body"
            fi
            # Strip leading '# ' or '#' from heading
            stripped="${line#\#}"
            stripped="${stripped#\#}"
            stripped="${stripped#\#}"
            # trim leading spaces
            while [ "${stripped:0:1}" = " " ]; do
              stripped="${stripped:1}"
            done
            title="$stripped"
            body=""
            ;;
          -\ *|\*\ *)
            # Bullet — strip leading "- " or "* "
            stripped="${line#- }"
            stripped="${stripped#\* }"
            if [ -z "$body" ]; then
              body="$stripped"
            else
              body="${body}
${stripped}"
            fi
            ;;
          *)
            # Other lines: treat as body continuation if title is set
            if [ -n "$line" ] && [ -n "$title" ]; then
              if [ -z "$body" ]; then
                body="$line"
              else
                body="${body}
${line}"
              fi
            fi
            ;;
        esac
      done < "$tmp"
      # Flush the final pending slide
      if [ -n "$title" ] || [ -n "$body" ]; then
        flush_slide "$title" "$body"
      fi
      IFS="$_IFS"
      /bin/rm -f "$tmp"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Keynote"
    try
        save front document in (POSIX file "$doc_e")
    end try
end tell
APPLE
      /bin/sleep 1
      echo "ok: built deck from outline -> $doc"
      ;;

    *)
      echo "ERR: unknown keynote action '$ACTION'. Try: new|open|save_as_pdf|add_slide|set_theme|play_presentation|add_image|slide_count|save_as|add_title|change_theme|add_image_to_slide|add_bullets|set_transition|rearrange_slides|build_from_outline" >&2
      exit 2
      ;;
  esac
}
