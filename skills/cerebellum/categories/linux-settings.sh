# cerebellum/categories/linux-settings.sh
# OS preferences on Linux. Per-DE because there's no single registry:
#   - GNOME           → gsettings
#   - KDE             → kwriteconfig5 / qdbus
#   - Network         → nmcli  (NetworkManager — almost universal)
#   - Audio           → pactl (PulseAudio / PipeWire compat)
#   - Brightness      → brightnessctl (kernel-DRM) or xbacklight (X11)
#   - Bluetooth       → bluetoothctl
#
# Actions follow the macOS settings.sh convention: set_X / toggle_X.

linux-settings_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    set_volume)
      # Volume 0-100 via pactl. Args: VOLUME (0-100)
      require "volume" "${1:-}"
      pactl set-sink-volume @DEFAULT_SINK@ "${1}%"
      echo "ok: volume = $1%"
      ;;

    volume_up)
      pactl set-sink-volume @DEFAULT_SINK@ +5%
      echo "ok: volume +5%"
      ;;

    volume_down)
      pactl set-sink-volume @DEFAULT_SINK@ -5%
      echo "ok: volume -5%"
      ;;

    mute|toggle_mute)
      pactl set-sink-mute @DEFAULT_SINK@ toggle
      echo "ok: mute toggled"
      ;;

    set_brightness)
      # Brightness 0-100 via brightnessctl (DRM/kernel; works on Wayland + X11)
      require "value" "${1:-}"
      if command -v brightnessctl >/dev/null 2>&1; then
        brightnessctl set "${1}%" >/dev/null
        echo "ok: brightness = $1%"
      elif command -v xbacklight >/dev/null 2>&1; then
        xbacklight -set "$1"
        echo "ok: brightness = $1% (via xbacklight)"
      else
        echo "ERR: install brightnessctl or xbacklight" >&2
        exit 2
      fi
      ;;

    set_appearance)
      # GNOME dark/light mode via gsettings.
      # Args: dark|light
      require "mode" "${1:-}"
      local theme
      case "$1" in
        dark|DARK) theme="prefer-dark" ;;
        light|LIGHT) theme="default" ;;
        *) echo "ERR: mode must be dark|light" >&2; exit 2 ;;
      esac
      if command -v gsettings >/dev/null 2>&1; then
        gsettings set org.gnome.desktop.interface color-scheme "$theme"
        echo "ok: appearance = $1 (GNOME)"
      else
        echo "ERR: gsettings not installed (GNOME only for now)" >&2
        exit 2
      fi
      ;;

    toggle_wifi)
      # SAFETY: refuse OFF, same as macOS settings.toggle_wifi guard
      # (paper #11 §5.1 bench-disruption mitigation).
      require "state" "${1:-}"
      local v
      case "$1" in
        on|ON|true|1) v="on" ;;
        off|OFF|false|0)
          echo "ERR: refusing to turn Wi-Fi OFF via cerebellum (disruptive)." >&2
          exit 2 ;;
        *) echo "ERR: state must be ON|OFF" >&2; exit 2 ;;
      esac
      if command -v nmcli >/dev/null 2>&1; then
        nmcli radio wifi "$v"
        echo "ok: wifi=$v"
      else
        echo "ERR: nmcli not installed" >&2
        exit 2
      fi
      ;;

    list_wifi_networks)
      require "out_file" "${1:-}"
      if command -v nmcli >/dev/null 2>&1; then
        nmcli -t -f SSID,SIGNAL,SECURITY dev wifi > "$1"
        echo "ok: wifi list -> $1"
      else
        echo "ERR: nmcli not installed" >&2
        exit 2
      fi
      ;;

    toggle_bluetooth)
      require "state" "${1:-}"
      if command -v bluetoothctl >/dev/null 2>&1; then
        local v
        case "$1" in on|ON|true|1) v="on" ;; *) v="off" ;; esac
        bluetoothctl power "$v" >/dev/null
        echo "ok: bluetooth = $v"
      else
        echo "ERR: bluetoothctl not installed" >&2; exit 2
      fi
      ;;

    open_settings)
      # Open the system Settings app (GNOME / KDE / Xfce).
      if command -v gnome-control-center >/dev/null 2>&1; then
        gnome-control-center >/dev/null 2>&1 &
        echo "ok: opened GNOME Settings"
      elif command -v systemsettings5 >/dev/null 2>&1; then
        systemsettings5 >/dev/null 2>&1 &
        echo "ok: opened KDE Settings"
      else
        echo "ERR: no recognized DE Settings app" >&2; exit 2
      fi
      ;;

    *)
      echo "ERR: unknown linux-settings action '$ACTION'." >&2
      echo "Actions: set_volume volume_up volume_down mute set_brightness set_appearance toggle_wifi toggle_bluetooth list_wifi_networks open_settings" >&2
      exit 2
      ;;
  esac
}
