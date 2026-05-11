music_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    play)
      /usr/bin/osascript -e 'tell application "Music" to play' 2>/dev/null
      echo "ok: play"
      ;;

    pause)
      /usr/bin/osascript -e 'tell application "Music" to pause' 2>/dev/null
      echo "ok: pause"
      ;;

    next|skip|skip_forward)
      /usr/bin/osascript -e 'tell application "Music" to next track' 2>/dev/null
      echo "ok: next track"
      ;;

    prev|previous|skip_backward)
      /usr/bin/osascript -e 'tell application "Music" to previous track' 2>/dev/null
      echo "ok: previous track"
      ;;

    search_track)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Music"
    set out to ""
    try
        set hits to (every track of library playlist 1 whose (name contains "$q_e" or artist contains "$q_e"))
        repeat with t in hits
            set out to out & (name of t) & " — " & (artist of t) & linefeed
        end repeat
    end try
    return out
end tell
APPLE
      echo "ok: search '$1' -> $2"
      ;;

    create_playlist)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music"
    if not (exists (user playlist "$n_e")) then
        make new user playlist with properties {name:"$n_e"}
    end if
end tell
APPLE
      echo "ok: playlist '$1'"
      ;;

    delete_playlist)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music"
    repeat with p in (every playlist whose name = "$n_e")
        try
            delete p
        end try
    end repeat
end tell
APPLE
      echo "ok: deleted playlist '$1'"
      ;;

    add_to_playlist)
      require "playlist_name" "${1:-}"; require "track_name" "${2:-}"
      local p_e t_e
      p_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music"
    if not (exists (user playlist "$p_e")) then
        make new user playlist with properties {name:"$p_e"}
    end if
    set targetPL to user playlist "$p_e"
    set hits to (every track of library playlist 1 whose name = "$t_e")
    if (count of hits) is 0 then
        set hits to (every track of library playlist 1 whose name contains "$t_e")
    end if
    if (count of hits) > 0 then
        duplicate (item 1 of hits) to targetPL
    end if
end tell
APPLE
      echo "ok: added '$2' to '$1'"
      ;;

    current_track)
      require "out_file" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Music"
    try
        return (name of current track) & " — " & (artist of current track)
    on error
        return ""
    end try
end tell
APPLE
      echo "ok: current track -> $1"
      ;;

    set_volume)
      require "level" "${1:-}"
      local lvl="$1"
      # Clamp 0..100
      if [ "$lvl" -lt 0 ] 2>/dev/null; then lvl=0; fi
      if [ "$lvl" -gt 100 ] 2>/dev/null; then lvl=100; fi
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music" to set sound volume to $lvl
APPLE
      echo "ok: volume=$lvl"
      ;;

    shuffle)
      require "value" "${1:-}"
      local v
      case "$1" in
        on|true|yes|1) v="true" ;;
        off|false|no|0) v="false" ;;
        *) echo "ERR: shuffle needs on|off" >&2; exit 2 ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music" to set shuffle enabled to $v
APPLE
      echo "ok: shuffle=$v"
      ;;

    repeat)
      require "mode" "${1:-}"
      local m
      case "$1" in
        off|none)  m="off" ;;
        one|song)  m="one" ;;
        all|list)  m="all" ;;
        *) echo "ERR: repeat needs off|one|all" >&2; exit 2 ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music" to set song repeat to $m
APPLE
      echo "ok: repeat=$m"
      ;;

    list_playlists)
      require "out_file" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Music"
    set out to ""
    repeat with p in user playlists
        set out to out & (name of p) & linefeed
    end repeat
    return out
end tell
APPLE
      echo "ok: playlists -> $1"
      ;;

    volume_down)
      # decrement by 10
      local cur
      cur="$(/usr/bin/osascript -e 'tell application "Music" to sound volume' 2>/dev/null)"
      [ -z "$cur" ] && cur=50
      local nv=$((cur - 10))
      [ "$nv" -lt 0 ] && nv=0
      /usr/bin/osascript -e "tell application \"Music\" to set sound volume to $nv" 2>/dev/null
      echo "ok: volume $cur -> $nv"
      ;;

    play_album)
      require "album_name" "${1:-}"
      local a_e
      a_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music"
    activate
    set hits to (every track of library playlist 1 whose album = "$a_e")
    if (count of hits) is 0 then
        set hits to (every track of library playlist 1 whose album contains "$a_e")
    end if
    if (count of hits) > 0 then
        play (item 1 of hits)
    end if
end tell
APPLE
      echo "ok: play album '$1'"
      ;;

    most_played)
      require "out_file" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Music"
    set winner to missing value
    set winnerCount to -1
    repeat with t in (every track of library playlist 1)
        try
            set pc to played count of t
            if pc > winnerCount then
                set winnerCount to pc
                set winner to t
            end if
        end try
    end repeat
    if winner is missing value then return ""
    return (name of winner) & " — " & (artist of winner)
end tell
APPLE
      echo "ok: most played -> $1"
      ;;

    *)
      echo "ERR: unknown music action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}
