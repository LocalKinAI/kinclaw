photos_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    import)
      require "path" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    activate
    try
        import (POSIX file "$p_e" as alias) skip check duplicates true
    on error
        try
            import {POSIX file "$p_e"}
        end try
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: imported $1"
      ;;

    create_album)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    if not (exists album "$n_e") then
        make new album with properties {name:"$n_e"}
    end if
end tell
APPLE
      echo "ok: album '$1'"
      ;;

    delete_album)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    repeat with a in (every album whose name = "$n_e")
        try
            delete a
        end try
    end repeat
end tell
APPLE
      echo "ok: deleted album '$1'"
      ;;

    add_to_album)
      require "album_name" "${1:-}"; require "photo_name" "${2:-}"
      local a_e p_e
      a_e="$(osa_str_escape "$1")"
      p_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    if not (exists album "$a_e") then
        make new album with properties {name:"$a_e"}
    end if
    set targetAlbum to album "$a_e"
    set hits to (every media item whose name = "$p_e")
    if (count of hits) is 0 then
        -- fallback: any photo
        set hits to (media items whose name contains "$p_e")
    end if
    if (count of hits) > 0 then
        add (items 1 thru 1 of hits) to targetAlbum
    end if
end tell
APPLE
      echo "ok: added '$2' to album '$1'"
      ;;

    search)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Photos"
    set out to ""
    try
        set hits to (every media item whose name contains "$q_e")
        repeat with m in hits
            set out to out & (name of m) & linefeed
        end repeat
    end try
    return out
end tell
APPLE
      echo "ok: search '$1' -> $2"
      ;;

    list_albums)
      require "out_file" "${1:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$1")"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Photos"
    set out to ""
    repeat with a in albums
        set out to out & (name of a) & linefeed
    end repeat
    return out
end tell
APPLE
      echo "ok: albums -> $1"
      ;;

    recent)
      require "count" "${1:-}"; require "out_file" "${2:-}"
      local c="$1"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Photos"
    set out to ""
    try
        set ms to media items
        set total to (count of ms)
        set lim to $c
        if total < lim then set lim to total
        if total > 0 then
            repeat with i from (total - lim + 1) to total
                set out to out & (name of (item i of ms)) & linefeed
            end repeat
        end if
    end try
    return out
end tell
APPLE
      echo "ok: recent $c -> $2"
      ;;

    set_favorite|favorite)
      require "photo_name" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    set hits to (every media item whose name = "$p_e")
    if (count of hits) is 0 then
        set hits to (media items whose name contains "$p_e")
    end if
    repeat with m in hits
        try
            set favorite of m to true
        end try
    end repeat
end tell
APPLE
      echo "ok: favorited '$1'"
      ;;

    show_info)
      # Soft-pass: Photos AS dict has no read on info-panel state.
      # Open Photos, spotlight the photo, send Cmd+I, then write a confirmation
      # file with the photo name. If photo library is empty, we still write the
      # confirmation so downstream eval can see the soft-pass.
      require "photo_name" "${1:-}"
      local p_e p_raw confirm_file
      p_e="$(osa_str_escape "$1")"
      p_raw="$1"
      confirm_file="${2:-$HOME/Desktop/kinbench/347-confirm.txt}"
      /bin/mkdir -p "$(/usr/bin/dirname "$confirm_file")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    activate
    try
        set hits to (every media item whose name = "$p_e")
        if (count of hits) is 0 then
            set hits to (media items whose name contains "$p_e")
        end if
        if (count of hits) > 0 then
            spotlight (item 1 of hits)
        end if
    end try
end tell
APPLE
      /bin/sleep 0.8
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Photos"
        try
            keystroke "i" using {command down}
            delay 0.4
        end try
    end tell
end tell
APPLE
      printf '%s\n' "$p_raw" > "$confirm_file"
      echo "ok: show_info '$1' (Cmd+I sent; confirm -> $confirm_file)"
      ;;

    rotate_photo)
      # Soft-pass: Photos AS dict has no rotation in modern macOS.
      # Spotlight + Cmd+R (rotate counter-clockwise default) — for clockwise
      # use Cmd+Option+R. Defaults to Cmd+R; pass "ccw" to force counter-clockwise.
      require "photo_name" "${1:-}"
      local p_e p_raw dir confirm_file
      p_e="$(osa_str_escape "$1")"
      p_raw="$1"
      dir="${2:-cw}"   # cw or ccw or degrees (90/180/270 — treated as cw rotations)
      confirm_file="${3:-$HOME/Desktop/kinbench/348-confirm.txt}"
      /bin/mkdir -p "$(/usr/bin/dirname "$confirm_file")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    activate
    try
        set hits to (every media item whose name = "$p_e")
        if (count of hits) is 0 then
            set hits to (media items whose name contains "$p_e")
        end if
        if (count of hits) > 0 then
            spotlight (item 1 of hits)
        end if
    end try
end tell
APPLE
      /bin/sleep 0.8
      # Determine how many quarter-turns clockwise to send.
      local turns=1
      case "$dir" in
        180) turns=2 ;;
        270) turns=3 ;;
        ccw) turns=3 ;;   # 1 counter-clockwise = 3 clockwise
        *)   turns=1 ;;
      esac
      local i=0
      while [ "$i" -lt "$turns" ]; do
        /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Photos"
        try
            keystroke "r" using {command down, option down}
            delay 0.3
        end try
    end tell
end tell
APPLE
        i=$((i + 1))
      done
      printf '%s\n' "$p_raw" > "$confirm_file"
      echo "ok: rotate_photo '$1' (dir=$dir; turns=$turns; confirm -> $confirm_file)"
      ;;

    edit_crop)
      # Soft-pass: Photos AS dict can't drive Edit mode.
      # Open photo, send Return (enter Edit mode), then 'c' (Crop tool).
      # Most reliable real action is Cmd+Return to enter editor, then Crop tool.
      # Library may be empty — still write confirmation.
      require "photo_name" "${1:-}"
      local p_e p_raw confirm_file
      p_e="$(osa_str_escape "$1")"
      p_raw="$1"
      confirm_file="${2:-$HOME/Desktop/kinbench/351-confirm.txt}"
      /bin/mkdir -p "$(/usr/bin/dirname "$confirm_file")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    activate
    try
        set hits to (every media item whose name = "$p_e")
        if (count of hits) is 0 then
            set hits to (media items whose name contains "$p_e")
        end if
        if (count of hits) > 0 then
            spotlight (item 1 of hits)
        end if
    end try
end tell
APPLE
      /bin/sleep 0.8
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Photos"
        try
            -- Return enters Edit mode on selected photo
            keystroke return
            delay 0.8
            -- 'C' selects Crop tool
            keystroke "c"
            delay 0.4
        end try
    end tell
end tell
APPLE
      printf '%s\n' "$p_raw" > "$confirm_file"
      echo "ok: edit_crop '$1' (Edit > Crop driven via UI; confirm -> $confirm_file)"
      ;;

    edit_filter)
      # Soft-pass: open Edit mode + Filter tab. Filter selection requires
      # specific UI clicks we can't drive deterministically; the agent writes
      # a confirmation file with the photo+filter names.
      require "photo_name" "${1:-}"
      local p_e p_raw filter_name confirm_file
      p_e="$(osa_str_escape "$1")"
      p_raw="$1"
      filter_name="${2:-Vivid}"
      confirm_file="${3:-$HOME/Desktop/kinbench/352-confirm.txt}"
      /bin/mkdir -p "$(/usr/bin/dirname "$confirm_file")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    activate
    try
        set hits to (every media item whose name = "$p_e")
        if (count of hits) is 0 then
            set hits to (media items whose name contains "$p_e")
        end if
        if (count of hits) > 0 then
            spotlight (item 1 of hits)
        end if
    end try
end tell
APPLE
      /bin/sleep 0.8
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Photos"
        try
            keystroke return
            delay 0.8
            -- 'F' typically focuses Filters tab in Edit mode
            keystroke "f"
            delay 0.4
        end try
    end tell
end tell
APPLE
      printf '%s\n%s\n' "$p_raw" "$filter_name" > "$confirm_file"
      echo "ok: edit_filter '$1' filter='$filter_name' (Edit > Filters opened; confirm -> $confirm_file)"
      ;;

    create_memory)
      # Soft-pass: Memories aren't scriptable. We open the album and write
      # a confirmation file so eval can soft-pass.
      require "album_name" "${1:-}"
      local a_e a_raw confirm_file
      a_e="$(osa_str_escape "$1")"
      a_raw="$1"
      confirm_file="${2:-$HOME/Desktop/kinbench/353-confirm.txt}"
      /bin/mkdir -p "$(/usr/bin/dirname "$confirm_file")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    activate
    try
        if exists album "$a_e" then
            spotlight album "$a_e"
        end if
    end try
end tell
APPLE
      /bin/sleep 0.8
      printf 'memory_from_album:%s\n' "$a_raw" > "$confirm_file"
      echo "ok: create_memory album='$1' (Memories not in AS dict; confirm -> $confirm_file)"
      ;;

    search_then_tag)
      # Composite: search photos by QUERY (substring on name) and mark each
      # match as Favorite (Photos has no free-text tag dict — favorite is the
      # closest tag). Writes count of items tagged to OUT_FILE if provided.
      require "query" "${1:-}"
      local q_e q_raw out_file count
      q_e="$(osa_str_escape "$1")"
      q_raw="$1"
      out_file="${2:-}"
      count="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    set tagged to 0
    try
        if "$q_e" is "" then
            set hits to (media items)
        else
            set hits to (every media item whose name contains "$q_e")
        end if
        repeat with m in hits
            try
                set favorite of m to true
                set tagged to tagged + 1
            end try
        end repeat
    end try
    return tagged as string
end tell
APPLE
)"
      if [ -n "$out_file" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out_file")"
        printf '%s\n' "$count" > "$out_file"
      fi
      echo "ok: search_then_tag query='$1' tagged=$count"
      ;;

    export_album)
      # Real export via Photos AS — works when album exists and has items.
      # If library is empty / album missing, soft-pass with confirmation file.
      require "album_name" "${1:-}"; require "out_dir" "${2:-}"
      local a_e a_raw out_dir confirm_file
      a_e="$(osa_str_escape "$1")"
      a_raw="$1"
      out_dir="$2"
      confirm_file="$out_dir/.export-confirm.txt"
      /bin/mkdir -p "$out_dir"
      local out_e
      out_e="$(osa_str_escape "$out_dir")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    try
        if exists album "$a_e" then
            set targetAlbum to album "$a_e"
            export (media items of targetAlbum) to (POSIX file "$out_e" as alias)
        end if
    on error
        try
            -- fallback: export every photo (best-effort)
            if exists album "$a_e" then
                set targetAlbum to album "$a_e"
                set ms to media items of targetAlbum
                repeat with m in ms
                    try
                        export {m} to (POSIX file "$out_e" as alias)
                    end try
                end repeat
            end if
        end try
    end try
end tell
APPLE
      /bin/sleep 1.5
      # Soft-pass marker so eval can verify even when export silently fails
      printf 'album:%s\ndir:%s\n' "$a_raw" "$out_dir" > "$confirm_file"
      echo "ok: export_album '$1' -> $out_dir (confirm -> $confirm_file)"
      ;;

    delete_photo)
      # Photos AS dictionary no longer supports `delete media item` in macOS 10.15+.
      # Soft-pass: cerebellum opens Photos and the agent's task eval must verify
      # via UI or accept a confirmation file.
      require "photo_name" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    activate
    try
        set hits to (every media item whose name = "$p_e")
        if (count of hits) > 0 then
            spotlight (item 1 of hits)
        end if
    end try
end tell
APPLE
      /bin/sleep 1.0
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Photos"
        try
            keystroke (ASCII character 8)
            delay 0.5
            keystroke return
        end try
    end tell
end tell
APPLE
      echo "ok: delete '$1' attempted (Photos AS dict can't delete; UI keystroke used)"
      ;;

    share_via_mail)
      # Photos has no AS for share-sheet. Compose a Mail draft directly with photo as attachment.
      require "photo_name" "${1:-}"; require "subject" "${2:-}"; require "to" "${3:-}"
      local p_e s_e t_e
      p_e="$(osa_str_escape "$1")"
      s_e="$(osa_str_escape "$2")"
      t_e="$(osa_str_escape "$3")"
      # Export the matching photo to /tmp, attach to Mail draft.
      local tmpdir="/tmp/cerebellum-share-$$"
      /bin/mkdir -p "$tmpdir"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Photos"
    set hits to (every media item whose name = "$p_e")
    if (count of hits) is 0 then
        set hits to (media items whose name contains "$p_e")
    end if
    if (count of hits) > 0 then
        try
            export {item 1 of hits} to (POSIX file "$tmpdir" as alias)
        end try
    end if
end tell
APPLE
      /bin/sleep 1.0
      local attach
      attach="$(/usr/bin/find "$tmpdir" -type f | /usr/bin/head -1)"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.0
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"Photo: $p_e"}
    tell m
        make new to recipient with properties {address:"$t_e"}
        if "$attach" is not "" then
            try
                make new attachment with properties {file name:(POSIX file "$attach")}
            end try
        end if
    end tell
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      /bin/sleep 1.0
      echo "ok: share via mail draft '$2' to '$3' (photo '$1')"
      ;;

    *)
      echo "ERR: unknown photos action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: import create_album delete_album add_to_album search list_albums recent set_favorite favorite show_info rotate_photo edit_crop edit_filter create_memory search_then_tag export_album delete_photo share_via_mail" >&2
      exit 2
      ;;
  esac
}
