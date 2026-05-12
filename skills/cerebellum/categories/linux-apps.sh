# cerebellum/categories/linux-apps.sh
# Launch / focus / list desktop applications on Linux.
# Cross-DE via xdg-open, gtk-launch, wmctrl, ydotool.

linux-apps_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    open)
      # Open file or URL with default handler (xdg-open) — universal.
      require "target" "${1:-}"
      if command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$1" >/dev/null 2>&1 &
        echo "ok: opened $1 via xdg-open"
      else
        echo "ERR: xdg-open not installed (apt install xdg-utils)" >&2
        exit 2
      fi
      ;;

    launch)
      # Launch a .desktop application by id (e.g. "firefox", "org.gnome.Files").
      require "desktop_id" "${1:-}"
      if command -v gtk-launch >/dev/null 2>&1; then
        gtk-launch "$1" >/dev/null 2>&1 &
        echo "ok: launched $1 via gtk-launch"
      elif command -v gio >/dev/null 2>&1; then
        gio launch "/usr/share/applications/$1.desktop" >/dev/null 2>&1 &
        echo "ok: launched $1 via gio"
      else
        echo "ERR: neither gtk-launch nor gio installed" >&2
        exit 2
      fi
      ;;

    focus)
      # Bring an existing window matching a substring to front.
      # X11 only via wmctrl. Wayland has no cross-app focus API
      # without compositor extensions.
      require "title_substring" "${1:-}"
      if ! command -v wmctrl >/dev/null 2>&1; then
        echo "ERR: wmctrl not installed (X11 only)" >&2; exit 2
      fi
      wmctrl -a "$1"
      echo "ok: focused window containing '$1'"
      ;;

    quit|kill)
      # Kill an app by exact process name.
      require "process_name" "${1:-}"
      if command -v pkill >/dev/null 2>&1; then
        pkill -x "$1" || true
      else
        killall "$1" || true
      fi
      echo "ok: killed $1"
      ;;

    list_running)
      # All windows with their PIDs + classes (X11 only).
      require "out_file" "${1:-}"
      if command -v wmctrl >/dev/null 2>&1; then
        wmctrl -l -p -x > "$1"
        echo "ok: window list -> $1"
      else
        # Fallback: enumerate processes with GUI windows roughly via /proc + ps
        /bin/ps -eo pid,comm > "$1"
        echo "ok: process list (no window assoc) -> $1"
      fi
      ;;

    list_installed)
      # List all installed .desktop applications.
      require "out_file" "${1:-}"
      find /usr/share/applications "$HOME/.local/share/applications" \
        -name "*.desktop" 2>/dev/null \
        | while read -r f; do
          name=$(grep -m1 "^Name=" "$f" 2>/dev/null | cut -d= -f2)
          [ -n "$name" ] && echo "$name | $f"
        done > "$1"
      echo "ok: installed apps -> $1"
      ;;

    is_running)
      require "process_name" "${1:-}"
      if /usr/bin/pgrep -x "$1" >/dev/null 2>&1; then
        echo "ok: $1 is running"
      else
        echo "ok: $1 not running"
        exit 3
      fi
      ;;

    *)
      echo "ERR: unknown linux-apps action '$ACTION'." >&2
      echo "Actions: open launch focus quit list_running list_installed is_running" >&2
      exit 2
      ;;
  esac
}
