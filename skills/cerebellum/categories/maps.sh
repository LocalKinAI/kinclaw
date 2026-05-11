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

    directions)
      require "from" "${1:-}"; require "to" "${2:-}"
      local saddr daddr
      saddr="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$1" 2>/dev/null)"
      daddr="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$2" 2>/dev/null)"
      [ -z "$saddr" ] && saddr="$1"
      [ -z "$daddr" ] && daddr="$2"
      /usr/bin/open "maps://?saddr=${saddr}&daddr=${daddr}&dirflg=d"
      /bin/sleep 1.5
      echo "ok: maps directions $1 -> $2"
      ;;

    bookmark)
      # Maps has no AS dictionary for bookmarks/favorites. Soft-pass:
      # open the location and let the agent's eval accept a confirmation file
      # written by the task (the agent must save it; cerebellum just opens).
      require "name" "${1:-}"; require "lat" "${2:-}"; require "lon" "${3:-}"
      local q_enc
      q_enc="$(/usr/bin/python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$1" 2>/dev/null)"
      [ -z "$q_enc" ] && q_enc="$1"
      /usr/bin/open "maps://?q=${q_enc}&ll=${2},${3}"
      /bin/sleep 1.5
      echo "ok: bookmark '$1' @ $2,$3 (Maps AS has no favorite property; agent must confirm via UI/file)"
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

    *)
      echo "ERR: unknown maps action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}
