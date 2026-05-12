# cerebellum/categories/windows-apps.sh
# Launch / focus / list desktop apps on Windows via PowerShell.
# Bash wrapper means it works from WSL / Git Bash / MSYS2 / any POSIX
# shell with powershell.exe on PATH.

windows-apps_dispatch() {
  local ACTION="$1"; shift || true
  local PS="powershell.exe -NoProfile -NonInteractive -Command"

  case "$ACTION" in

    open)
      # Universal "open with default handler". Start-Process with a
      # path/URL hands off to the registered handler (browser for URLs,
      # editor for files, etc.). On Windows 10/11 this also opens UWP
      # apps via ms-* protocol URLs.
      require "target" "${1:-}"
      $PS "Start-Process '$1'"
      echo "ok: opened $1"
      ;;

    launch)
      # Launch by executable name or full path. Use Start-Process so
      # we don't block until the app exits.
      require "exe" "${1:-}"
      $PS "Start-Process '$1'"
      echo "ok: launched $1"
      ;;

    focus)
      # Bring a window matching a substring of its title to front.
      # Uses AppActivate from WScript.Shell — works for classic Win32
      # apps; UWP apps need PowerShell + UI Automation (handled by
      # the ui claw, not cerebellum's fast path).
      require "title_substring" "${1:-}"
      $PS "
\$ws = New-Object -ComObject WScript.Shell
\$proc = Get-Process | Where-Object { \$_.MainWindowTitle -like '*$1*' } | Select-Object -First 1
if (\$proc) { \$ws.AppActivate(\$proc.Id) | Out-Null; 'ok' } else { Write-Error 'no window matching $1' }
"
      echo "ok: focused window containing '$1'"
      ;;

    quit|kill)
      # Graceful shutdown first (CloseMainWindow → respects unsaved-
      # work prompts on most well-behaved apps), then taskkill /F for
      # the stubborn ones after a short grace period.
      require "process_name" "${1:-}"
      $PS "
\$procs = Get-Process -Name '$1' -ErrorAction SilentlyContinue
foreach (\$p in \$procs) { \$p.CloseMainWindow() | Out-Null }
Start-Sleep -Seconds 2
Get-Process -Name '$1' -ErrorAction SilentlyContinue | Stop-Process -Force
"
      echo "ok: killed $1"
      ;;

    list_running)
      # All processes with a top-level window.
      require "out_file" "${1:-}"
      $PS "Get-Process | Where-Object { \$_.MainWindowHandle -ne 0 } | Select-Object Id, ProcessName, MainWindowTitle | ConvertTo-Csv -NoTypeInformation" > "$1"
      echo "ok: window list -> $1"
      ;;

    list_installed)
      # Installed apps via Get-Package (covers MSI + MSU + Appx). Less
      # complete than Get-StartApps but doesn't require admin.
      require "out_file" "${1:-}"
      $PS "Get-StartApps | Select-Object Name, AppID | ConvertTo-Csv -NoTypeInformation" > "$1"
      echo "ok: installed apps -> $1"
      ;;

    is_running)
      require "process_name" "${1:-}"
      $PS "if (Get-Process -Name '$1' -ErrorAction SilentlyContinue) { 'running' } else { 'stopped'; exit 3 }"
      ;;

    *)
      echo "ERR: unknown windows-apps action '$ACTION'." >&2
      echo "Actions: open launch focus quit list_running list_installed is_running" >&2
      exit 2
      ;;
  esac
}
