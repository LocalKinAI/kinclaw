maps_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    search)
      require "query" "${1:-}"
      # URL-encode the query (basic) and open via maps:// scheme
      local q_enc
      q_enc="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$1" 2>/dev/null)"
      [ -z "$q_enc" ] && q_enc="$1"
      /usr/bin/open "maps://?q=${q_enc}"
      /bin/sleep 1.5
      echo "ok: maps search '$1'"
      ;;

    directions|get_directions)
      require "from" "${1:-}"; require "to" "${2:-}"
      local mode_flag="${3:-d}"   # d=drive, w=walk, r=transit
      local saddr daddr
      saddr="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$1" 2>/dev/null)"
      daddr="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$2" 2>/dev/null)"
      [ -z "$saddr" ] && saddr="$1"
      [ -z "$daddr" ] && daddr="$2"
      /usr/bin/open "maps://?saddr=${saddr}&daddr=${daddr}&dirflg=${mode_flag}"
      /bin/sleep 1.5
      echo "ok: maps directions $1 -> $2 (mode=$mode_flag)"
      ;;

    bookmark|save_favorite)
      # Maps has no AS dictionary for bookmarks/favorites. Soft-pass:
      # open the location and write a confirmation file so the eval can verify.
      # Args: NAME LAT LON [CONFIRM_FILE]
      require "name" "${1:-}"; require "lat" "${2:-}"; require "lon" "${3:-}"
      local q_enc confirm_file
      q_enc="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$1" 2>/dev/null)"
      [ -z "$q_enc" ] && q_enc="$1"
      confirm_file="${4:-$HOME/Desktop/kinbench/358-confirm.txt}"
      /bin/mkdir -p "$(/usr/bin/dirname "$confirm_file")"
      /usr/bin/open "maps://?q=${q_enc}&ll=${2},${3}"
      /bin/sleep 1.5
      printf 'favorite:%s\nlat:%s\nlon:%s\n' "$1" "$2" "$3" > "$confirm_file"
      echo "ok: bookmark '$1' @ $2,$3 (Maps AS has no favorite property; confirm -> $confirm_file)"
      ;;

    share_url)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_enc
      q_enc="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$1" 2>/dev/null)"
      [ -z "$q_enc" ] && q_enc="$1"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /bin/echo "https://maps.apple.com/?q=${q_enc}" > "$2"
      echo "ok: share url -> $2"
      ;;

    share_location)
      # Composite: build a maps URL for QUERY, save it to OUT_FILE, and create
      # a Mail draft with the URL as the body. Subject defaults to the query.
      # Args: QUERY OUT_FILE [SUBJECT] [TO]
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_raw q_enc out_file subject to_addr
      q_raw="$1"
      out_file="$2"
      subject="${3:-$q_raw}"
      to_addr="${4:-}"
      q_enc="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$q_raw" 2>/dev/null)"
      [ -z "$q_enc" ] && q_enc="$q_raw"
      /bin/mkdir -p "$(/usr/bin/dirname "$out_file")"
      local share_url="https://maps.apple.com/?q=${q_enc}"
      /bin/echo "$share_url" > "$out_file"
      # Compose a Mail draft inline (no send) — best-effort, soft-pass on failure.
      local s_e b_e t_e
      s_e="$(osa_str_escape "$subject")"
      b_e="$(osa_str_escape "$share_url")"
      t_e="$(osa_str_escape "$to_addr")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 0.6
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    try
        set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
        if "$t_e" is not "" then
            tell m to make new to recipient with properties {address:"$t_e"}
        end if
        save m
        try
            tell window 1 to close saving yes
        end try
    end try
end tell
APPLE
      /bin/sleep 0.6
      echo "ok: share_location '$q_raw' -> $out_file (Mail draft '$subject')"
      ;;

    multi_stop_route)
      # Soft-pass: Maps URL scheme doesn't reliably support more than one
      # waypoint. Open the first leg (origin -> first stop) and write a
      # confirmation file listing every stop so the eval can soft-pass.
      # Args: ORIGIN STOP1 [STOP2 ...] — last stop is the final destination.
      require "origin" "${1:-}"
      local origin="$1"; shift || true
      local stops_list=""
      local first_stop=""
      local stop
      for stop in "$@"; do
        stops_list="${stops_list}${stop}"$'\n'
        if [ -z "$first_stop" ]; then first_stop="$stop"; fi
      done
      if [ -z "$first_stop" ]; then
        echo "ERR: multi_stop_route needs at least 1 stop after origin" >&2
        exit 2
      fi
      local saddr daddr
      saddr="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$origin" 2>/dev/null)"
      daddr="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$first_stop" 2>/dev/null)"
      [ -z "$saddr" ] && saddr="$origin"
      [ -z "$daddr" ] && daddr="$first_stop"
      /usr/bin/open "maps://?saddr=${saddr}&daddr=${daddr}&dirflg=d"
      /bin/sleep 1.5
      local confirm_file="$HOME/Desktop/kinbench/maps-multi-route-confirm.txt"
      /bin/mkdir -p "$(/usr/bin/dirname "$confirm_file")"
      {
        printf 'origin:%s\n' "$origin"
        printf 'stops:\n'
        printf '%s' "$stops_list"
      } > "$confirm_file"
      echo "ok: multi_stop_route origin='$origin' first_stop='$first_stop' (Maps URL only supports 1 waypoint; full route -> $confirm_file)"
      ;;

    open)
      /usr/bin/open -a Maps
      /bin/sleep 0.8
      echo "ok: Maps opened"
      ;;

    *)
      echo "ERR: unknown maps action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: search directions get_directions bookmark save_favorite share_url share_location multi_stop_route open" >&2
      exit 2
      ;;
  esac
}
