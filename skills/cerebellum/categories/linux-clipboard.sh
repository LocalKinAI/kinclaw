# cerebellum/categories/linux-clipboard.sh
# Clipboard ops cross-DE. Wayland: wl-clipboard. X11: xclip / xsel.

linux-clipboard_dispatch() {
  local ACTION="$1"; shift || true

  # Pick the right tool based on display server.
  local COPY_CMD="" PASTE_CMD=""
  if [ -n "${WAYLAND_DISPLAY:-}" ] && command -v wl-copy >/dev/null 2>&1; then
    COPY_CMD="wl-copy"
    PASTE_CMD="wl-paste --no-newline"
  elif command -v xclip >/dev/null 2>&1; then
    COPY_CMD="xclip -selection clipboard"
    PASTE_CMD="xclip -selection clipboard -o"
  elif command -v xsel >/dev/null 2>&1; then
    COPY_CMD="xsel --clipboard --input"
    PASTE_CMD="xsel --clipboard --output"
  else
    echo "ERR: no clipboard tool (install wl-clipboard / xclip / xsel)" >&2
    exit 2
  fi

  case "$ACTION" in

    set|copy)
      require "text" "${1:-}"
      printf '%s' "$1" | $COPY_CMD
      echo "ok: clipboard set (${#1} chars)"
      ;;

    set_file)
      require "path" "${1:-}"
      $COPY_CMD < "$1"
      echo "ok: clipboard set from $1"
      ;;

    get|paste)
      $PASTE_CMD
      ;;

    get_to_file)
      require "out_file" "${1:-}"
      $PASTE_CMD > "$1"
      echo "ok: clipboard -> $1"
      ;;

    clear)
      printf '' | $COPY_CMD
      echo "ok: clipboard cleared"
      ;;

    *)
      echo "ERR: unknown linux-clipboard action '$ACTION'." >&2
      echo "Actions: set copy set_file get paste get_to_file clear" >&2
      exit 2
      ;;
  esac
}
