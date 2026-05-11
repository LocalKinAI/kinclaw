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

    play_pause|playpause|toggle)
      # Toggle between play/pause based on current state.
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Music"
    try
        if player state is playing then
            pause
        else
            play
        end if
    on error
        try
            playpause
        end try
    end try
end tell
APPLE
      echo "ok: play_pause"
      ;;

    next|skip|skip_forward)
      # Optional N count — skip N tracks forward.
      local n="${1:-1}"
      if [ "$n" -lt 1 ] 2>/dev/null; then n=1; fi
      local i=0
      while [ "$i" -lt "$n" ]; do
        /usr/bin/osascript -e 'tell application "Music" to next track' 2>/dev/null
        i=$((i + 1))
      done
      echo "ok: next track x$n"
      ;;

    prev|previous|skip_backward|skip_back)
      # Optional N count — skip N tracks backward.
      local n="${1:-1}"
      if [ "$n" -lt 1 ] 2>/dev/null; then n=1; fi
      local i=0
      while [ "$i" -lt "$n" ]; do
        /usr/bin/osascript -e 'tell application "Music" to previous track' 2>/dev/null
        i=$((i + 1))
      done
      echo "ok: previous track x$n"
      ;;

    search_track|search_library)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Music"
    set out to ""
    try
        if "$q_e" is "" then
            set hits to (every track of library playlist 1)
        else
            set hits to (every track of library playlist 1 whose (name contains "$q_e" or artist contains "$q_e"))
        end if
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
      /bin/mkdir -p "$(/usr/bin/dirname "$1")"
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

    shuffle_toggle|toggle_shuffle)
      # Flip current shuffle state — read, invert, set.
      local cur
      cur="$(/usr/bin/osascript -e 'tell application "Music" to shuffle enabled' 2>/dev/null)"
      local nv
      case "$cur" in
        true) nv="false" ;;
        *) nv="true" ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Music" to set shuffle enabled to $nv
APPLE
      echo "ok: shuffle toggled $cur -> $nv"
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
      /bin/mkdir -p "$(/usr/bin/dirname "$1")"
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

    export_playlist)
      # Write the track titles of a specific playlist to OUT_FILE (one per line).
      require "playlist_name" "${1:-}"; require "out_file" "${2:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Music"
    set out to ""
    try
        if exists (user playlist "$p_e") then
            set pl to user playlist "$p_e"
            repeat with t in (every track of pl)
                set out to out & (name of t) & linefeed
            end repeat
        end if
    end try
    return out
end tell
APPLE
      echo "ok: playlist '$1' tracks -> $2"
      ;;

    volume_down)
      # Decrement by N (default 10).
      local step="${1:-10}"
      local cur
      cur="$(/usr/bin/osascript -e 'tell application "Music" to sound volume' 2>/dev/null)"
      [ -z "$cur" ] && cur=50
      local nv=$((cur - step))
      [ "$nv" -lt 0 ] && nv=0
      /usr/bin/osascript -e "tell application \"Music\" to set sound volume to $nv" 2>/dev/null
      echo "ok: volume $cur -> $nv"
      ;;

    volume_up)
      # Increment by N (default 10).
      local step="${1:-10}"
      local cur
      cur="$(/usr/bin/osascript -e 'tell application "Music" to sound volume' 2>/dev/null)"
      [ -z "$cur" ] && cur=50
      local nv=$((cur + step))
      [ "$nv" -gt 100 ] && nv=100
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
      /bin/mkdir -p "$(/usr/bin/dirname "$1")"
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
      echo "Actions: play pause play_pause next skip_forward prev skip_back search_track search_library create_playlist delete_playlist add_to_playlist current_track set_volume volume_up volume_down shuffle shuffle_toggle repeat list_playlists export_playlist play_album most_played" >&2
      exit 2
      ;;
  esac
}
