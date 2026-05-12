# cerebellum/categories/windows-settings.sh
# Windows system preferences via PowerShell + a handful of registry
# pokes for things PS doesn't expose first-class (dark mode).
#
# Actions follow the macOS settings.sh / linux-settings.sh shapes so
# portable souls work cross-platform.

windows-settings_dispatch() {
  local ACTION="$1"; shift || true
  local PS="powershell.exe -NoProfile -NonInteractive -Command"

  case "$ACTION" in

    set_volume)
      # 0-100 system volume. Win10/11 doesn't ship a built-in cmdlet
      # but a 5-line SendKeys loop hitting VolumeMute / VolumeUp /
      # VolumeDown is the universal idiom that works without admin.
      require "volume" "${1:-}"
      $PS "
Add-Type -AssemblyName presentationCore
\$obj = New-Object -ComObject WScript.Shell
# Mute first so we have a known floor, then bump up the target %.
for (\$i=0; \$i -lt 50; \$i++) { \$obj.SendKeys([char]174) }
\$bumps = [math]::Round($1 / 2)
for (\$i=0; \$i -lt \$bumps; \$i++) { \$obj.SendKeys([char]175) }
"
      echo "ok: volume = $1%"
      ;;

    volume_up)
      $PS "(New-Object -ComObject WScript.Shell).SendKeys([char]175)"
      echo "ok: volume +2%"
      ;;

    volume_down)
      $PS "(New-Object -ComObject WScript.Shell).SendKeys([char]174)"
      echo "ok: volume -2%"
      ;;

    mute|toggle_mute)
      $PS "(New-Object -ComObject WScript.Shell).SendKeys([char]173)"
      echo "ok: mute toggled"
      ;;

    set_brightness)
      # WMI WmiSetBrightness — supported on most laptops, no-op on
      # external monitors. Range 0-100.
      require "value" "${1:-}"
      $PS "(Get-WmiObject -Namespace root/WMI -Class WmiMonitorBrightnessMethods).WmiSetBrightness(1, $1)" \
        >/dev/null
      echo "ok: brightness = $1%"
      ;;

    set_appearance)
      # Dark/light via registry. Affects Apps theme + (optionally)
      # System theme. Effect is immediate for new app launches; for
      # already-running apps the user has to reload.
      require "mode" "${1:-}"
      local v
      case "$1" in
        dark|DARK)   v=0 ;;
        light|LIGHT) v=1 ;;
        *) echo "ERR: mode must be dark|light" >&2; exit 2 ;;
      esac
      $PS "
Set-ItemProperty -Path 'HKCU:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize' -Name AppsUseLightTheme -Value $v
Set-ItemProperty -Path 'HKCU:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize' -Name SystemUsesLightTheme -Value $v
"
      echo "ok: appearance = $1"
      ;;

    toggle_wifi)
      # SAFETY: refuse OFF (paper #11 §5.1 bench-disruption mitigation).
      # Same guard as macOS settings.toggle_wifi + linux-settings.
      require "state" "${1:-}"
      case "$1" in
        on|ON|true|1) ;;
        off|OFF|false|0)
          echo "ERR: refusing to turn Wi-Fi OFF via cerebellum (disruptive)." >&2
          exit 2 ;;
        *) echo "ERR: state must be ON|OFF" >&2; exit 2 ;;
      esac
      # netsh wlan can't actually toggle the radio on/off in modern
      # Windows; use the UWP Radios API via PowerShell instead.
      $PS "
\$radios = [Windows.Devices.Radios.Radio,Windows.Devices.Radios,ContentType=WindowsRuntime]
\$asyncOp = [Windows.Devices.Radios.Radio]::GetRadiosAsync()
\$task = \$asyncOp.AsTask()
[void]\$task.Wait()
\$wifi = \$task.Result | Where-Object { \$_.Kind -eq 'WiFi' } | Select-Object -First 1
if (\$wifi) { [void]\$wifi.SetStateAsync('On').AsTask().Wait(); 'ok' } else { Write-Error 'no Wi-Fi radio' }
"
      echo "ok: wifi=on"
      ;;

    list_wifi_networks)
      require "out_file" "${1:-}"
      $PS "netsh wlan show networks mode=Bssid" > "$1"
      echo "ok: wifi list -> $1"
      ;;

    toggle_bluetooth)
      require "state" "${1:-}"
      local v
      case "$1" in on|ON|true|1) v="On" ;; *) v="Off" ;; esac
      $PS "
\$asyncOp = [Windows.Devices.Radios.Radio,Windows.Devices.Radios,ContentType=WindowsRuntime]::GetRadiosAsync()
\$task = \$asyncOp.AsTask()
[void]\$task.Wait()
\$bt = \$task.Result | Where-Object { \$_.Kind -eq 'Bluetooth' } | Select-Object -First 1
if (\$bt) { [void]\$bt.SetStateAsync('$v').AsTask().Wait(); 'ok' } else { Write-Error 'no Bluetooth radio' }
"
      echo "ok: bluetooth = $1"
      ;;

    open_settings)
      # ms-settings: protocol opens the modern Settings app to a
      # specific pane. Bare "ms-settings:" opens the home page.
      $PS "Start-Process 'ms-settings:'"
      echo "ok: opened Settings"
      ;;

    *)
      echo "ERR: unknown windows-settings action '$ACTION'." >&2
      echo "Actions: set_volume volume_up volume_down mute set_brightness set_appearance toggle_wifi toggle_bluetooth list_wifi_networks open_settings" >&2
      exit 2
      ;;
  esac
}
