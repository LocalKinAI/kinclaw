# cerebellum/categories/windows-clipboard.sh
# Clipboard ops on Windows via PowerShell built-in cmdlets
# (Set-Clipboard / Get-Clipboard, present in PS 5.1+ which ships with
# every supported Windows). Same action surface as
# linux-clipboard.sh / macOS notes.

windows-clipboard_dispatch() {
  local ACTION="$1"; shift || true
  local PS="powershell.exe -NoProfile -NonInteractive -Command"

  case "$ACTION" in

    set|copy)
      require "text" "${1:-}"
      # Use -Value (positional) so embedded quotes don't get parsed by
      # the surrounding PS context. Set-Clipboard accepts any string.
      $PS "Set-Clipboard -Value '${1//\'/\'\'}'"
      echo "ok: clipboard set (${#1} chars)"
      ;;

    set_file)
      require "path" "${1:-}"
      $PS "Set-Clipboard -Value (Get-Content -Raw -LiteralPath '$1')"
      echo "ok: clipboard set from $1"
      ;;

    get|paste)
      $PS "Get-Clipboard"
      ;;

    get_to_file)
      require "out_file" "${1:-}"
      $PS "Get-Clipboard" > "$1"
      echo "ok: clipboard -> $1"
      ;;

    clear)
      $PS "Set-Clipboard -Value ''"
      echo "ok: clipboard cleared"
      ;;

    *)
      echo "ERR: unknown windows-clipboard action '$ACTION'." >&2
      echo "Actions: set copy set_file get paste get_to_file clear" >&2
      exit 2
      ;;
  esac
}
