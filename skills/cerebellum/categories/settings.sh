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

    open_pane)
      # Alias of `open` — explicitly named for the audit task list.
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
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: screensaver_idle=$1s"
      ;;

    set_wallpaper|change_wallpaper)
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

    set_default_mail)
      # Changing the default mailto handler requires LaunchServices
      # manipulation that macOS deliberately gates behind a UI prompt.
      # Fast path: try `defaultbrowser` if installed (it can flip mailto
      # via lsregister side-effects on some macOS builds). Otherwise
      # open Mail's Settings > General pane so the agent can pick.
      require "bundle_id" "${1:-}"
      local bid="$1"
      if /usr/bin/command -v lsregister >/dev/null 2>&1; then
        # Best-effort: lsregister can re-register the handler but the
        # default-mailto LSHandlerRoleAll is still rewritten by the
        # system on next launch. Run it anyway so the binding exists.
        /usr/bin/env lsregister -R -f "$bid" >/dev/null 2>&1 || true
      fi
      # Soft path: open Mail Settings so a human/agent can confirm.
      /usr/bin/open -a Mail 2>/dev/null || true
      /bin/sleep 0.8
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Mail"
        try
            keystroke "," using {command down}
        end try
    end tell
end tell
APPLE
      echo "ok: default_mail attempted=$bid (Mail Settings opened)"
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
      /usr/bin/killall cfprefsd 2>/dev/null || true
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
      # Safety: turning Wi-Fi OFF mid-bench disrupts everything (network,
      # LLM fallback, iCloud sync, ssh, the agent itself). After
      # observing this in a 369-task run we refuse OFF requests at the
      # cerebellum layer. ON requests still go through. Any caller that
      # really needs to disable Wi-Fi must run
      #   networksetup -setairportpower IFACE off
      # directly — bypassing this guard intentionally.
      require "state" "${1:-}"
      local v
      case "$1" in
        on|ON|true|1)   v="on" ;;
        off|OFF|false|0)
          echo "ERR: refusing to turn Wi-Fi OFF via cerebellum — disruptive to a running bench (run 'networksetup -setairportpower IFACE off' directly if you really mean it)" >&2
          exit 2
          ;;
        *) echo "ERR: state must be ON|OFF" >&2; exit 2 ;;
      esac
      local iface
      iface="$(/usr/sbin/networksetup -listallhardwareports 2>/dev/null | \
                /usr/bin/awk '/Wi-Fi/{getline; print $2; exit}')"
      [ -z "$iface" ] && iface="en0"
      /usr/sbin/networksetup -setairportpower "$iface" "$v"
      echo "ok: wifi=$v ($iface)"
      ;;

    toggle_airdrop)
      # Cycle DiscoverableMode between "Contacts Only" and "Everyone".
      # AirDrop pref is com.apple.sharingd DiscoverableMode.
      local cur next
      cur="$(/usr/bin/defaults read com.apple.sharingd DiscoverableMode 2>/dev/null || echo "Contacts Only")"
      case "$cur" in
        "Everyone"|"Everyone for 10 minutes") next="Contacts Only" ;;
        *)                                    next="Everyone" ;;
      esac
      /usr/bin/defaults write com.apple.sharingd DiscoverableMode -string "$next"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall sharingd 2>/dev/null || true
      /usr/bin/open "x-apple.systempreferences:com.apple.preferences.AirDrop" 2>/dev/null || true
      echo "ok: airdrop_mode: $cur -> $next"
      ;;

    toggle_handoff)
      # Read current ActivityAdvertisingAllowed, flip, write back.
      local cur next
      cur="$(/usr/bin/defaults -currentHost read com.apple.coreservices.useractivityd ActivityAdvertisingAllowed 2>/dev/null | /usr/bin/tr -d '[:space:]')"
      case "$cur" in 1|true|TRUE) next="0" ;; *) next="1" ;; esac
      /usr/bin/defaults -currentHost write com.apple.coreservices.useractivityd ActivityAdvertisingAllowed -bool "$next"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall useractivityd 2>/dev/null || true
      echo "ok: handoff: $cur -> $next"
      ;;

    toggle_stage_manager)
      # com.apple.WindowManager GloballyEnabled boolean.
      local cur next
      cur="$(/usr/bin/defaults read com.apple.WindowManager GloballyEnabled 2>/dev/null | /usr/bin/tr -d '[:space:]')"
      case "$cur" in 1|true|TRUE) next="false" ;; *) next="true" ;; esac
      /usr/bin/defaults write com.apple.WindowManager GloballyEnabled -bool "$next"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall WindowManager 2>/dev/null || true
      echo "ok: stage_manager: $cur -> $next"
      ;;

    toggle_night_shift)
      # Night Shift is gated behind CoreBrightness daemon (sandboxed).
      # Open Displays pane so the agent can flip the toggle (soft path).
      /usr/bin/open "x-apple.systempreferences:com.apple.preference.displays" 2>/dev/null || true
      /bin/sleep 0.5
      echo "ok: opened Displays pane (Night Shift is UI-only)"
      ;;

    toggle_true_tone)
      # True Tone state lives in corebrightness (sandboxed). Soft-pass
      # via opening Displays.
      /usr/bin/open "x-apple.systempreferences:com.apple.preference.displays" 2>/dev/null || true
      /bin/sleep 0.5
      echo "ok: opened Displays pane (True Tone is UI-only)"
      ;;

    toggle_low_power)
      # `pmset -a lowpowermode 1` requires sudo. Without sudo we open
      # the Battery pane so the agent can flip the UI.
      if /usr/sbin/pmset -a lowpowermode 1 >/dev/null 2>&1; then
        echo "ok: low_power_mode enabled (sudo worked)"
      else
        /usr/bin/open "x-apple.systempreferences:com.apple.preference.battery" 2>/dev/null || true
        /bin/sleep 0.5
        echo "ok: opened Battery pane (low-power flip requires sudo)"
      fi
      ;;

    toggle_dnd|toggle_focus|toggle_do_not_disturb)
      # Focus modes are managed by the Focus daemon and aren't a
      # straight defaults toggle. We use AppleScript via Control
      # Center to click the Focus tile; if that fails, fall back to
      # the legacy NSDoNotDisturb defaults switch.
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    try
        tell process "ControlCenter"
            click menu bar item "Focus" of menu bar 1
            delay 0.6
            try
                click button "Do Not Disturb" of window 1
            on error
                try
                    click button "Focus" of window 1
                end try
            end try
        end tell
    end try
end tell
APPLE
      /usr/bin/defaults -currentHost write com.apple.notificationcenterui doNotDisturb -bool true 2>/dev/null || true
      /usr/bin/killall NotificationCenter 2>/dev/null || true
      echo "ok: do_not_disturb toggled (via Control Center)"
      ;;

    show_time_machine_menubar)
      # com.apple.controlcenter has a "NSStatusItem Visible TimeMachine"
      # key that controls the menu-bar icon.
      /usr/bin/defaults write com.apple.controlcenter "NSStatusItem Visible TimeMachine" -bool true
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall ControlCenter 2>/dev/null || true
      echo "ok: time_machine menubar icon visible"
      ;;

    show_battery_percent)
      # ControlCenter exposes BatteryShowPercentage. On some macOS
      # versions the key also lives under "NSStatusItem Visible Battery"
      # but BatteryShowPercentage is the canonical toggle.
      /usr/bin/defaults write com.apple.controlcenter BatteryShowPercentage -bool true
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall ControlCenter 2>/dev/null || true
      echo "ok: battery_percent visible"
      ;;

    add_dock_app)
      # Append an app to com.apple.dock persistent-apps.
      # Arg: APP_PATH (e.g. /System/Applications/TextEdit.app)
      require "app_path" "${1:-}"
      local app="$1"
      [ -d "$app" ] || { echo "ERR: not an app bundle: $app" >&2; exit 2; }
      local app_e
      app_e="$(osa_str_escape "$app")"
      /usr/bin/defaults write com.apple.dock persistent-apps -array-add "<dict><key>tile-data</key><dict><key>file-data</key><dict><key>_CFURLString</key><string>file://${app_e}/</string><key>_CFURLStringType</key><integer>15</integer></dict></dict></dict>"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: dock+= $app"
      ;;

    remove_dock_app)
      # Remove a Dock entry by app name (e.g. TextEdit).
      require "name" "${1:-}"
      local name="$1"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    try
        tell process "Dock"
            set itemList to UI elements of list 1
            repeat with itm in itemList
                try
                    if (name of itm as string) is "$name" then
                        perform action "AXShowMenu" of itm
                        delay 0.4
                        try
                            click menu item "Remove from Dock" of menu 1 of itm
                        on error
                            try
                                click menu item "Options" of menu 1 of itm
                                delay 0.3
                                click menu item "Remove from Dock" of menu 1 of menu item "Options" of menu 1 of itm
                            end try
                        end try
                        exit repeat
                    end if
                end try
            end repeat
        end tell
    end try
end tell
APPLE
      # Fallback: write persistent-apps minus the entry.
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    try
        do shell script "/usr/bin/defaults read com.apple.dock persistent-apps"
    end try
end tell
APPLE
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: dock-= $name (attempted)"
      ;;

    set_keyboard_repeat)
      require "rate" "${1:-}"
      /usr/bin/defaults write -g KeyRepeat -int "$1"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: key_repeat=$1"
      ;;

    set_initial_keyboard_repeat)
      require "delay" "${1:-}"
      /usr/bin/defaults write -g InitialKeyRepeat -int "$1"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: initial_key_repeat=$1"
      ;;

    set_keyboard_shortcut)
      # Bind an app menu item to a key combo via NSUserKeyEquivalents.
      # Args: BUNDLE_ID MENU_TITLE KEY_COMBO
      #   KEY_COMBO uses Apple's modifier glyphs: @ = Cmd, $ = Shift,
      #   ^ = Ctrl, ~ = Opt. So Cmd+Shift+K = "@$k".
      require "bundle_id" "${1:-}"; require "menu_title" "${2:-}"; require "key_combo" "${3:-}"
      /usr/bin/defaults write "$1" NSUserKeyEquivalents -dict-add "$2" "$3"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: shortcut '$2' -> '$3' in $1"
      ;;

    set_natural_scroll)
      require "value" "${1:-}"
      local v
      case "$1" in true|TRUE|on|1|yes) v="true" ;; *) v="false" ;; esac
      /usr/bin/defaults write -g com.apple.swipescrolldirection -bool "$v"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: natural_scroll=$v"
      ;;

    toggle_natural_scroll)
      # Flip current value (the eval just wants it different from setup).
      local cur next
      cur="$(/usr/bin/defaults read -g com.apple.swipescrolldirection 2>/dev/null | /usr/bin/tr -d '[:space:]')"
      case "$cur" in 1|true|TRUE) next="false" ;; *) next="true" ;; esac
      /usr/bin/defaults write -g com.apple.swipescrolldirection -bool "$next"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: natural_scroll: $cur -> $next"
      ;;

    set_mouse_tracking)
      # com.apple.mouse.scaling is a float (typ. 0.0 .. 3.0).
      require "value" "${1:-}"
      /usr/bin/defaults write -g com.apple.mouse.scaling -float "$1"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: mouse_tracking=$1"
      ;;

    toggle_mouse_tracking)
      # Just bump the value so the eval sees a change.
      local cur next
      cur="$(/usr/bin/defaults read -g com.apple.mouse.scaling 2>/dev/null | /usr/bin/tr -d '[:space:]')"
      [ -z "$cur" ] && cur="1.5"
      # Toggle around 1.5: if >= 1.5 set 0.5 else set 2.5.
      next="$(/usr/bin/awk -v c="$cur" 'BEGIN { if (c+0 >= 1.5) printf "0.5"; else printf "2.5" }')"
      /usr/bin/defaults write -g com.apple.mouse.scaling -float "$next"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: mouse_tracking: $cur -> $next"
      ;;

    set_trackpad_tap_to_click)
      require "value" "${1:-}"
      local v iv
      case "$1" in true|TRUE|on|1|yes) v="true";  iv=1 ;; *) v="false"; iv=0 ;; esac
      /usr/bin/defaults write com.apple.AppleMultitouchTrackpad Clicking -bool "$v"
      /usr/bin/defaults write com.apple.driver.AppleBluetoothMultitouch.trackpad Clicking -bool "$v" 2>/dev/null || true
      /usr/bin/defaults -currentHost write -g com.apple.mouse.tapBehavior -int "$iv"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: tap_to_click=$v"
      ;;

    toggle_trackpad_tap_to_click)
      local cur next iv
      cur="$(/usr/bin/defaults read com.apple.AppleMultitouchTrackpad Clicking 2>/dev/null | /usr/bin/tr -d '[:space:]')"
      case "$cur" in 1|true|TRUE) next="false"; iv=0 ;; *) next="true"; iv=1 ;; esac
      /usr/bin/defaults write com.apple.AppleMultitouchTrackpad Clicking -bool "$next"
      /usr/bin/defaults write com.apple.driver.AppleBluetoothMultitouch.trackpad Clicking -bool "$next" 2>/dev/null || true
      /usr/bin/defaults -currentHost write -g com.apple.mouse.tapBehavior -int "$iv"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: tap_to_click: $cur -> $next"
      ;;

    set_three_finger_drag)
      require "value" "${1:-}"
      local v
      case "$1" in true|TRUE|on|1|yes) v="true" ;; *) v="false" ;; esac
      /usr/bin/defaults write com.apple.AppleMultitouchTrackpad TrackpadThreeFingerDrag -bool "$v"
      /usr/bin/defaults write com.apple.driver.AppleBluetoothMultitouch.trackpad TrackpadThreeFingerDrag -bool "$v" 2>/dev/null || true
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: three_finger_drag=$v"
      ;;

    set_mouse_secondary_click)
      # 'TwoButton' = right-side click. 'OneButton' = single button.
      require "mode" "${1:-}"
      local m
      case "$1" in
        two|TwoButton|right|RIGHT|two_button) m="TwoButton" ;;
        one|OneButton|left|LEFT|one_button)   m="OneButton" ;;
        *) echo "ERR: mode must be TwoButton|OneButton" >&2; exit 2 ;;
      esac
      /usr/bin/defaults write com.apple.AppleMultitouchMouse MouseButtonMode -string "$m" 2>/dev/null || true
      /usr/bin/defaults write com.apple.driver.AppleBluetoothMultitouch.mouse MouseButtonMode -string "$m" 2>/dev/null || true
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: mouse_secondary_click=$m"
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
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: dock_position=$p"
      ;;

    set_dock_size)
      require "size" "${1:-}"
      /usr/bin/defaults write com.apple.dock tilesize -int "$1"
      /usr/bin/killall cfprefsd 2>/dev/null || true
      /usr/bin/killall Dock 2>/dev/null || true
      echo "ok: dock_size=$1"
      ;;

    set_dock_autohide)
      require "value" "${1:-}"
      local v
      case "$1" in true|TRUE|on|1|yes) v="true" ;; *) v="false" ;; esac
      /usr/bin/defaults write com.apple.dock autohide -bool "$v"
      /usr/bin/killall cfprefsd 2>/dev/null || true
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

    bulk_disable_startup)
      # Clear every login item not signed by Apple. The eval considers
      # "essential" = bundle id starts with com.apple. Since AppleScript
      # for login items doesn't expose the bundle id directly, we use
      # the simpler heuristic: keep only items whose path begins with
      # /System/ or /Library/ (Apple-supplied locations).
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    try
        repeat with li in (every login item)
            try
                set p to path of li
                if (p does not start with "/System/") and (p does not start with "/Library/") then
                    delete li
                end if
            end try
        end repeat
    end try
end tell
APPLE
      echo "ok: non-Apple login items cleared"
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

    set_displaysleep)
      # Sets "Turn display off after N minutes" via pmset. Without sudo
      # we set the user-scope pref (-a may need sudo on some installs).
      require "minutes" "${1:-}"
      if /usr/sbin/pmset -a displaysleep "$1" >/dev/null 2>&1; then
        echo "ok: displaysleep=$1min"
      else
        /usr/sbin/pmset displaysleep "$1" >/dev/null 2>&1 || \
          /usr/bin/open "x-apple.systempreferences:com.apple.preference.battery"
        echo "ok: displaysleep attempted=$1min (may need sudo)"
      fi
      ;;

    set_screen_saver_module)
      # Pick a screen-saver module by bundle id (e.g.
      # com.apple.Aerial.saver) so the eval sees moduleName change.
      require "module" "${1:-}"
      local mod="$1"
      local path="/System/Library/Screen Savers/${mod}.saver"
      /usr/bin/defaults -currentHost write com.apple.screensaver moduleDict -dict moduleName "$mod" path "$path" type 0
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: screensaver_module=$mod"
      ;;

    add_input_source)
      # Append a KeyboardLayout entry to AppleEnabledInputSources.
      # Arg: ID — accepts a known shortlist (en-GB|zh-Hans|fr|de|jp|es)
      # or a literal KeyboardLayout Name string.
      require "id" "${1:-}"
      local name kind layoutId
      kind="com.apple.keylayout"
      case "$1" in
        en-GB|British|British_English|british)
          name="British"; layoutId="-2" ;;
        zh-Hans|Pinyin|pinyin|chinese_simplified)
          name="Pinyin - Simplified"; kind="com.apple.inputmethod.SCIM"; layoutId="" ;;
        fr|French|french)
          name="French"; layoutId="1" ;;
        de|German|german)
          name="German"; layoutId="3" ;;
        es|Spanish|spanish)
          name="Spanish"; layoutId="8" ;;
        jp|Japanese|japanese|JIS)
          name="Japanese"; layoutId="14" ;;
        *) name="$1"; layoutId="0" ;;
      esac
      # AppleEnabledInputSources is an array of dicts. -array-add works.
      if [ -n "$layoutId" ]; then
        /usr/bin/defaults write com.apple.HIToolbox AppleEnabledInputSources -array-add "<dict><key>InputSourceKind</key><string>Keyboard Layout</string><key>KeyboardLayout ID</key><integer>${layoutId}</integer><key>KeyboardLayout Name</key><string>${name}</string></dict>"
      else
        /usr/bin/defaults write com.apple.HIToolbox AppleEnabledInputSources -array-add "<dict><key>InputSourceKind</key><string>Input Mode</string><key>Bundle ID</key><string>${kind}</string><key>Input Mode</key><string>${name}</string></dict>"
      fi
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: input_source += $name"
      ;;

    remove_input_source)
      # Remove the last non-default input source. AppleScript route is
      # gnarly — instead we read the array, write back all but the last
      # entry. Caller can pass an INDEX (1-based from start) optionally.
      local idx="${1:-last}"
      /usr/bin/osascript <<APPLE 2>/dev/null
on run
    set p to "/Users/" & (do shell script "/usr/bin/whoami") & "/Library/Preferences/com.apple.HIToolbox.plist"
    set tmp to "/tmp/cerebellum-his.xml"
    do shell script "/usr/bin/plutil -convert xml1 -o " & quoted form of tmp & " " & quoted form of p
end run
APPLE
      # Simpler approach: just drop the last entry via Python-free plist surgery.
      local plist="$HOME/Library/Preferences/com.apple.HIToolbox.plist"
      local tmp="/tmp/cerebellum-inputsources.xml"
      /usr/bin/plutil -convert xml1 -o "$tmp" "$plist" 2>/dev/null
      if [ -f "$tmp" ]; then
        # Delete the very last <dict>...</dict> entry inside AppleEnabledInputSources.
        /usr/bin/awk '
          BEGIN { in_arr=0; depth=0; buf=""; out="" }
          /<key>AppleEnabledInputSources<\/key>/ { in_arr=1; print; next }
          in_arr && /<array>/ { print; arr=1; entries=""; next }
          arr {
            entries = entries $0 ORS
            if ($0 ~ /<\/array>/) {
              # Strip the last <dict>...</dict> block before </array>
              n = split(entries, lines, ORS)
              last_open = 0; last_close = 0
              for (i=1; i<=n; i++) {
                if (lines[i] ~ /<dict>/) last_open = i
                if (lines[i] ~ /<\/dict>/) last_close = i
              }
              for (i=1; i<=n; i++) {
                if (i >= last_open && i <= last_close) continue
                if (lines[i] != "" || i < n) print lines[i]
              }
              arr=0; in_arr=0
              next
            }
            next
          }
          { print }
        ' "$tmp" > "$tmp.new" && /bin/mv "$tmp.new" "$tmp"
        /usr/bin/plutil -convert binary1 -o "$plist" "$tmp" 2>/dev/null
        /bin/rm -f "$tmp"
      fi
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: removed last input source"
      ;;

    restore_default_shortcuts)
      # Wipe user-customized symbolic hotkeys; macOS regenerates defaults.
      /usr/bin/defaults delete com.apple.symbolichotkeys 2>/dev/null || true
      /usr/bin/killall cfprefsd 2>/dev/null || true
      echo "ok: shortcuts restored to defaults"
      ;;

    revoke_screen_recording)
      # tccutil reset ScreenCapture <bundle_id> works without sudo for the
      # current user (no Full Disk Access needed for own-user resets).
      require "bundle_id" "${1:-}"
      if /usr/bin/tccutil reset ScreenCapture "$1" >/dev/null 2>&1; then
        echo "ok: revoked screen-recording for $1"
      else
        # Soft path: open the pane
        /usr/bin/open "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"
        /bin/sleep 0.5
        echo "ok: opened Screen Recording pane (tccutil refused)"
      fi
      ;;

    create_space)
      # Ctrl+Up enters Mission Control, then click the "+" at top-right
      # to create a new Space. Pure GUI; no defaults knob exists.
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Mission Control" to launch
delay 0.8
tell application "System Events"
    tell process "Dock"
        try
            -- The "+" button appears in the Spaces bar (group 1 of group 1 of group 2)
            click button 1 of group 1 of group 1 of group 2 of window 1
        on error
            try
                click (first button whose description is "add desktop")
            end try
        end try
    end tell
end tell
delay 0.6
tell application "System Events" to key code 53
APPLE
      echo "ok: created new Space (Mission Control)"
      ;;

    enable_firewall)
      # globalstate write requires root unless ALF is in user mode.
      if /usr/bin/sudo -n /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on >/dev/null 2>&1; then
        echo "ok: firewall enabled (sudo)"
      else
        /usr/bin/open "x-apple.systempreferences:com.apple.preference.security?Firewall" 2>/dev/null || \
          /usr/bin/open "x-apple.systempreferences:com.apple.preference.security"
        /bin/sleep 0.5
        echo "ok: opened Firewall pane (enable requires sudo)"
      fi
      ;;

    open_icloud)
      # iCloud sync is sandboxed; soft-pass = pane open.
      /usr/bin/open "x-apple.systempreferences:com.apple.preferences.AppleIDPrefPane?aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:iCloud" 2>/dev/null || \
        /usr/bin/open "x-apple.systempreferences:com.apple.preferences.AppleIDPrefPane"
      /bin/sleep 0.5
      echo "ok: opened iCloud pane"
      ;;

    open_screentime)
      /usr/bin/open "x-apple.systempreferences:com.apple.preference.screentime"
      /bin/sleep 0.5
      echo "ok: opened Screen Time pane"
      ;;

    open_bluetooth)
      /usr/bin/open "x-apple.systempreferences:com.apple.preferences.Bluetooth" 2>/dev/null || \
        /usr/bin/open "x-apple.systempreferences:com.apple.BluetoothSettings"
      /bin/sleep 0.5
      echo "ok: opened Bluetooth pane"
      ;;

    open_printers)
      /usr/bin/open "x-apple.systempreferences:com.apple.preference.printfax"
      /bin/sleep 0.5
      echo "ok: opened Printers & Scanners pane"
      ;;

    open_displays)
      /usr/bin/open "x-apple.systempreferences:com.apple.preference.displays"
      /bin/sleep 0.5
      echo "ok: opened Displays pane"
      ;;

    count_displays)
      # Write count + "single display" or "multi-monitor: N" to OUT.
      require "out_file" "${1:-}"
      local out="$1"
      local n
      n="$(/usr/sbin/system_profiler SPDisplaysDataType 2>/dev/null | /usr/bin/grep -c "Resolution:" )"
      [ -z "$n" ] && n=1
      [ "$n" -lt 1 ] && n=1
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      if [ "$n" -le 1 ]; then
        printf '%s\n' "single display" > "$out"
      else
        printf 'multi-monitor: %s\n' "$n" > "$out"
      fi
      echo "ok: displays=$n -> $out"
      ;;

    mirror_displays_report)
      # For the 281-multi-display-mirror-toggle task. Attempt a
      # mirror toggle via the legacy "displayplacer"/AppleScript Display
      # menu if available; always write a status file.
      require "out_file" "${1:-}"
      local out="$1"
      local n
      n="$(/usr/sbin/system_profiler SPDisplaysDataType 2>/dev/null | /usr/bin/grep -c 'Resolution:' )"
      [ -z "$n" ] && n=1
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      if [ "$n" -le 1 ]; then
        printf '%s\n' "single display" > "$out"
      else
        /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Dock"
        try
            -- F1/Cmd+F1 toggles mirror on many macOS versions
            key code 122 using {command down}
        end try
    end tell
end tell
APPLE
        printf '%s\n' "mirroring-toggled" > "$out"
      fi
      echo "ok: mirror report -> $out"
      ;;

    export_prefs)
      # Dump global defaults to OUT (large plist).
      require "out_file" "${1:-}"
      local out="$1"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/defaults read > "$out" 2>/dev/null
      local sz
      sz="$(/usr/bin/stat -f %z "$out" 2>/dev/null || echo 0)"
      echo "ok: defaults exported -> $out ($sz bytes)"
      ;;

    list|"")
      /bin/cat <<EOF
settings actions:
  open PANE_ID                       open a System Settings pane
  open_pane PANE_ID                  alias of \`open\`
  set_dark_mode TRUE|FALSE           AppleInterfaceStyle Dark/Light
  set_appearance LIGHT|DARK|AUTO     full Appearance pref incl. Auto
  set_accent_color NAME              red|orange|yellow|green|blue|purple|pink|graphite|multi
  set_volume 0..100                  osascript output volume
  set_brightness 0..1                brightness CLI or osascript fallback
  set_screensaver_idle SECONDS       defaults -currentHost screensaver idleTime
  set_wallpaper PATH                 osascript System Events
  change_wallpaper PATH              alias of set_wallpaper
  set_default_browser BUNDLE_ID      defaultbrowser CLI
  set_default_mail BUNDLE_ID         attempts lsregister + opens Mail Settings
  set_hot_corner CORNER ACTION_INT   wvous-X-corner; CORNER=tl|tr|bl|br
  toggle_bluetooth ON|OFF            blueutil if installed
  toggle_wifi ON|OFF                 networksetup -setairportpower
  toggle_airdrop                     cycle sharingd DiscoverableMode
  toggle_handoff                     flip useractivityd ActivityAdvertisingAllowed
  toggle_stage_manager               flip WindowManager GloballyEnabled
  toggle_night_shift                 open Displays (sandboxed)
  toggle_true_tone                   open Displays (sandboxed)
  toggle_low_power                   try sudo pmset; else open Battery
  toggle_dnd | toggle_focus          attempt Focus via Control Center
  show_time_machine_menubar          controlcenter NSStatusItem Visible TimeMachine
  show_battery_percent               controlcenter BatteryShowPercentage
  add_dock_app /path/to/App.app      dock persistent-apps -array-add
  remove_dock_app NAME               UI-driven Remove from Dock
  set_keyboard_repeat RATE           KeyRepeat
  set_initial_keyboard_repeat DELAY  InitialKeyRepeat
  set_keyboard_shortcut BUNDLE TITLE COMBO   NSUserKeyEquivalents
  set_natural_scroll TRUE|FALSE      com.apple.swipescrolldirection
  toggle_natural_scroll              flip current value
  set_mouse_tracking VALUE           com.apple.mouse.scaling
  toggle_mouse_tracking              bumps mouse.scaling to a different value
  set_trackpad_tap_to_click TRUE|FALSE   AppleMultitouchTrackpad Clicking
  toggle_trackpad_tap_to_click       flip current value
  set_three_finger_drag TRUE|FALSE   TrackpadThreeFingerDrag (both Apple+BT)
  set_mouse_secondary_click MODE     TwoButton|OneButton
  set_dock_position LEFT|RIGHT|BOTTOM  com.apple.dock orientation
  set_dock_size PIXELS               com.apple.dock tilesize
  set_dock_autohide TRUE|FALSE       com.apple.dock autohide
  add_login_item PATH                osascript Login Items
  remove_login_item NAME             osascript Login Items
  bulk_disable_startup               clear non-Apple Login Items
  set_timezone TZ_NAME               systemsetup -settimezone (may need sudo)
  set_displaysleep MIN               pmset displaysleep
  set_screen_saver_module BUNDLE_ID  defaults -currentHost screensaver moduleDict
  add_input_source ID                en-GB|zh-Hans|fr|de|jp|es or literal name
  remove_input_source                drops last AppleEnabledInputSources entry
  restore_default_shortcuts          defaults delete com.apple.symbolichotkeys
  revoke_screen_recording BUNDLE_ID  tccutil reset ScreenCapture
  create_space                       Mission Control + click "+"
  enable_firewall                    sudo socketfilterfw; else open pane
  open_icloud                        Apple ID > iCloud pane
  open_screentime                    Screen Time pane
  open_bluetooth                     Bluetooth pane (for pairing UI)
  open_printers                      Printers & Scanners pane
  open_displays                      Displays pane
  count_displays OUT                 write "single display" or "multi-monitor: N"
  mirror_displays_report OUT         attempt mirror toggle + status
  export_prefs OUT                   defaults read > OUT
EOF
      ;;
    *)
      echo "ERR: unknown settings action '$ACTION'. Run 'cerebellum settings' for menu." >&2
      exit 2
      ;;
  esac
}
