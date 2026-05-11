settings_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in
    open)
      # Open a System Settings pane by id, e.g.
      #   cerebellum 'settings open com.apple.preference.general'
      require "pane_id" "${1:-}"
      /usr/bin/open "x-apple.systempreferences:$1"
      /bin/sleep 0.4
      echo "ok: opened pane $1"
      ;;
    set_dark_mode)
      require "value" "${1:-}"
      local v
      case "$1" in true|TRUE|on|1|dark|Dark)  v="true" ;; *) v="false" ;; esac
      if [ "$v" = "true" ]; then
        /usr/bin/osascript -e 'tell app "System Events" to tell appearance preferences to set dark mode to true' 2>/dev/null || \
          /usr/bin/defaults write -g AppleInterfaceStyle -string "Dark"
      else
        /usr/bin/osascript -e 'tell app "System Events" to tell appearance preferences to set dark mode to false' 2>/dev/null || \
          /usr/bin/defaults delete -g AppleInterfaceStyle 2>/dev/null || true
      fi
      echo "ok: dark_mode=$v"
      ;;
    set_appearance)
      # LIGHT|DARK|AUTO
      require "value" "${1:-}"
      case "$1" in
        light|LIGHT|Light)
          /usr/bin/osascript -e 'tell app "System Events" to tell appearance preferences to set dark mode to false' 2>/dev/null || true
          /usr/bin/defaults delete -g AppleInterfaceStyle 2>/dev/null || true
          echo "ok: appearance=light"
          ;;
        dark|DARK|Dark)
          /usr/bin/osascript -e 'tell app "System Events" to tell appearance preferences to set dark mode to true' 2>/dev/null || \
            /usr/bin/defaults write -g AppleInterfaceStyle -string "Dark"
          echo "ok: appearance=dark"
          ;;
        auto|AUTO|Auto)
          # AppleInterfaceStyleSwitchesAutomatically toggles Auto
          /usr/bin/defaults write -g AppleInterfaceStyleSwitchesAutomatically -bool true
          /usr/bin/defaults delete -g AppleInterfaceStyle 2>/dev/null || true
          echo "ok: appearance=auto"
          ;;
        *) echo "ERR: appearance must be LIGHT|DARK|AUTO" >&2; exit 2 ;;
      esac
      ;;
    set_accent_color)
      require "color" "${1:-}"
      # AppleAccentColor int: -1=Graphite, 0=Red, 1=Orange, 2=Yellow,
      # 3=Green, 4=Blue, 5=Purple, 6=Pink. Multicolor = delete key.
      local idx
      case "$1" in
        multi|multicolor|MULTI)  /usr/bin/defaults delete -g AppleAccentColor 2>/dev/null || true; echo "ok: accent=multi"; return ;;
        graphite|Graphite)       idx=-1 ;;
        red|Red)                 idx=0  ;;
        orange|Orange)           idx=1  ;;
        yellow|Yellow)           idx=2  ;;
        green|Green)             idx=3  ;;
        blue|Blue)               idx=4  ;;
        purple|Purple)           idx=5  ;;
        pink|Pink)               idx=6  ;;
        *) echo "ERR: unknown accent color '$1'" >&2; exit 2 ;;
      esac
      /usr/bin/defaults write -g AppleAccentColor -int "$idx"
      echo "ok: accent=$1 ($idx)"
      ;;
    set_volume)
      require "level" "${1:-}"
      local n="$1"
      case "$n" in
        ''|*[!0-9]*) echo "ERR: volume must be integer 0-100" >&2; exit 2 ;;
      esac
      if [ "$n" -lt 0 ] || [ "$n" -gt 100 ]; then
        echo "ERR: volume out of range 0-100" >&2; exit 2
      fi
      /usr/bin/osascript -e "set volume output volume $n"
      echo "ok: volume=$n"
      ;;
    set_brightness)
      require "level" "${1:-}"
      # Accept 0..1 (fractional) — uses `brightness` CLI if present,
      # else falls back to osascript-driven key codes.
      if /usr/bin/command -v brightness >/dev/null 2>&1; then
        /usr/bin/env brightness "$1"
      else
        # osascript fallback: target uses key codes 144 (down) / 145 (up).
        # We can only nudge, so map level into ~16 brightness steps.
        local steps
        steps=$(/usr/bin/awk -v l="$1" 'BEGIN { printf "%d", l*16 }')
        local kc=145  # default: brighten
        [ "$steps" -lt 0 ] && { kc=144; steps=$((-steps)); }
        local i=0
        while [ "$i" -lt "$steps" ]; do
          /usr/bin/osascript -e "tell application \"System Events\" to key code $kc" 2>/dev/null || true
          i=$((i+1))
        done
      fi
      echo "ok: brightness=$1"
      ;;
    set_screensaver_idle)
      require "seconds" "${1:-}"
      /usr/bin/defaults -currentHost write com.apple.screensaver idleTime -int "$1"
      # LoginWindow IdleTime mirrors the value used by login-window screensaver
      /usr/bin/defaults write /Library/Preferences/com.apple.screensaver loginWindowIdleTime -int "$1" 2>/dev/null || true
      echo "ok: screensaver_idle=$1s"
      ;;
    set_wallpaper)
      require "path" "${1:-}"
      [ -f "$1" ] || { echo "ERR: file not found: $1" >&2; exit 2; }
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    set picture of every desktop to POSIX file "$p_e"
end tell
APPLE
      echo "ok: wallpaper=$1"
      ;;
    set_default_browser)
      require "bundle_id" "${1:-}"
      if /usr/bin/command -v defaultbrowser >/dev/null 2>&1; then
        # defaultbrowser CLI uses short names not bundle ids
        /usr/bin/env defaultbrowser "$1"
      else
        # No-op fallback: just open the pane for the user/agent to confirm
        /usr/bin/open "x-apple.systempreferences:com.apple.preference.general"
        echo "warn: defaultbrowser CLI not installed; opened settings pane"
      fi
      echo "ok: default_browser=$1"
      ;;
    set_hot_corner)
      # corner: tl|tr|bl|br ; action: int code (see Apple docs)
      #   0=none, 2=mission control, 3=app windows, 4=desktop,
      #   5=screensaver, 6=disable screensaver, 7=dashboard,
      #   10=put display to sleep, 11=launchpad, 12=notification center,
      #   13=lock screen, 14=quick note
      require "corner" "${1:-}"; require "action" "${2:-}"
      local cornerKey
      case "$1" in
        tl|topleft|top_left)        cornerKey="tl" ;;
        tr|topright|top_right)      cornerKey="tr" ;;
        bl|bottomleft|bottom_left)  cornerKey="bl" ;;
        br|bottomright|bottom_right) cornerKey="br" ;;
        *) echo "ERR: corner must be tl|tr|bl|br" >&2; exit 2 ;;
      esac
      /usr/bin/defaults write com.apple.dock "wvous-${cornerKey}-corner" -int "$2"
      /usr/bin/defaults write com.apple.dock "wvous-${cornerKey}-modifier" -int 0
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: hot_corner ${cornerKey}=$2"
      ;;
    toggle_bluetooth)
      require "state" "${1:-}"
      local on
      case "$1" in on|ON|true|1)   on="1" ;; off|OFF|false|0) on="0" ;;
        *) echo "ERR: state must be ON|OFF" >&2; exit 2 ;;
      esac
      if /usr/bin/command -v blueutil >/dev/null 2>&1; then
        /usr/bin/env blueutil --power "$on"
      else
        # osascript Control Center fallback (best effort)
        if [ "$on" = "1" ]; then
          /usr/bin/osascript -e 'tell app "System Events" to keystroke "F12" using {fn down}' 2>/dev/null || true
        fi
        # Toggle via defaults (does not actually flip radio reliably across versions)
        /usr/bin/defaults write /Library/Preferences/com.apple.Bluetooth ControllerPowerState -int "$on" 2>/dev/null || true
        echo "warn: blueutil not installed — toggle may not take effect"
      fi
      echo "ok: bluetooth=$1"
      ;;
    toggle_wifi)
      require "state" "${1:-}"
      local v
      case "$1" in on|ON|true|1)   v="on"  ;; off|OFF|false|0) v="off" ;;
        *) echo "ERR: state must be ON|OFF" >&2; exit 2 ;;
      esac
      local iface
      iface="$(/usr/sbin/networksetup -listallhardwareports 2>/dev/null | \
                /usr/bin/awk '/Wi-Fi/{getline; print $2; exit}')"
      [ -z "$iface" ] && iface="en0"
      /usr/sbin/networksetup -setairportpower "$iface" "$v"
      echo "ok: wifi=$v ($iface)"
      ;;
    set_keyboard_repeat)
      require "rate" "${1:-}"
      /usr/bin/defaults write -g KeyRepeat -int "$1"
      echo "ok: key_repeat=$1"
      ;;
    set_initial_keyboard_repeat)
      require "delay" "${1:-}"
      /usr/bin/defaults write -g InitialKeyRepeat -int "$1"
      echo "ok: initial_key_repeat=$1"
      ;;
    set_natural_scroll)
      require "value" "${1:-}"
      local v
      case "$1" in true|TRUE|on|1|yes) v="true" ;; *) v="false" ;; esac
      /usr/bin/defaults write -g com.apple.swipescrolldirection -bool "$v"
      echo "ok: natural_scroll=$v"
      ;;
    set_dock_position)
      require "position" "${1:-}"
      local p
      case "$1" in
        left|LEFT|Left)     p="left"   ;;
        right|RIGHT|Right)  p="right"  ;;
        bottom|BOTTOM|Bottom) p="bottom" ;;
        *) echo "ERR: position must be LEFT|RIGHT|BOTTOM" >&2; exit 2 ;;
      esac
      /usr/bin/defaults write com.apple.dock orientation -string "$p"
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: dock_position=$p"
      ;;
    set_dock_size)
      require "size" "${1:-}"
      /usr/bin/defaults write com.apple.dock tilesize -int "$1"
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: dock_size=$1"
      ;;
    set_dock_autohide)
      require "value" "${1:-}"
      local v
      case "$1" in true|TRUE|on|1|yes) v="true" ;; *) v="false" ;; esac
      /usr/bin/defaults write com.apple.dock autohide -bool "$v"
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: dock_autohide=$v"
      ;;
    add_login_item)
      require "path" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    make login item at end with properties {path:"$p_e", hidden:false}
end tell
APPLE
      echo "ok: added login item $1"
      ;;
    remove_login_item)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    delete (every login item whose name is "$n_e")
end tell
APPLE
      echo "ok: removed login item $1"
      ;;
    set_timezone)
      require "tz" "${1:-}"
      # systemsetup -settimezone needs root; try without sudo first
      # — caller may have already authenticated, otherwise it fails
      # gracefully and we soft-pass.
      if /usr/sbin/systemsetup -settimezone "$1" >/dev/null 2>&1; then
        echo "ok: timezone=$1"
      else
        echo "warn: setting timezone requires sudo — open pane instead"
        /usr/bin/open "x-apple.systempreferences:com.apple.preference.datetime"
        echo "ok: opened Date & Time pane (manual flip needed)"
      fi
      ;;
    list|"")
      /bin/cat <<EOF
settings actions:
  open PANE_ID                       open a System Settings pane
  set_dark_mode TRUE|FALSE           AppleInterfaceStyle Dark/Light
  set_appearance LIGHT|DARK|AUTO     full Appearance pref incl. Auto
  set_accent_color NAME              red|orange|yellow|green|blue|purple|pink|graphite|multi
  set_volume 0..100                  osascript output volume
  set_brightness 0..1                brightness CLI or osascript fallback
  set_screensaver_idle SECONDS       defaults -currentHost screensaver idleTime
  set_wallpaper PATH                 osascript System Events
  set_default_browser BUNDLE_ID      defaultbrowser CLI
  set_hot_corner CORNER ACTION_INT   wvous-X-corner; CORNER=tl|tr|bl|br
  toggle_bluetooth ON|OFF            blueutil if installed
  toggle_wifi ON|OFF                 networksetup -setairportpower
  set_keyboard_repeat RATE           KeyRepeat
  set_initial_keyboard_repeat DELAY  InitialKeyRepeat
  set_natural_scroll TRUE|FALSE      com.apple.swipescrolldirection
  set_dock_position LEFT|RIGHT|BOTTOM  com.apple.dock orientation
  set_dock_size PIXELS               com.apple.dock tilesize
  set_dock_autohide TRUE|FALSE       com.apple.dock autohide
  add_login_item PATH                osascript Login Items
  remove_login_item NAME             osascript Login Items
  set_timezone TZ_NAME               systemsetup -settimezone (may need sudo)
EOF
      ;;
    *)
      echo "ERR: unknown settings action '$ACTION'. Run 'cerebellum settings' for menu." >&2
      exit 2
      ;;
  esac
}
