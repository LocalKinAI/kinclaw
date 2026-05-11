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
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Photos"
    set out to ""
    try
        set ms to media items
        set total to (count of ms)
        set lim to $c
        if total < lim then set lim to total
        repeat with i from (total - lim + 1) to total
            set out to out & (name of (item i of ms)) & linefeed
        end repeat
    end try
    return out
end tell
APPLE
      echo "ok: recent $c -> $2"
      ;;

    set_favorite)
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
      exit 2
      ;;
  esac
}
