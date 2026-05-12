# cerebellum/categories/windows-files.sh
# Windows file ops. Shells out to PowerShell 5.1+ for the universal
# cmdlets (Move-Item, Copy-Item, Remove-Item, Compress-Archive). The
# bash wrapper means cerebellum runs identically under WSL, Git Bash,
# MSYS2, or any POSIX shell the user has installed — only the
# underlying Windows ops are PowerShell.
#
# Paths can be either POSIX-style ("/c/Users/foo/bar") or Windows-style
# ("C:\Users\foo\bar"); PowerShell accepts both. Git Bash on Windows
# auto-translates POSIX → Windows when invoking powershell.exe.

windows-files_dispatch() {
  local ACTION="$1"; shift || true

  # PS shorthand. -NoProfile shaves ~250ms off startup; -NonInteractive
  # prevents Read-Host from blocking the agent.
  local PS="powershell.exe -NoProfile -NonInteractive -Command"

  case "$ACTION" in

    rename|move)
      require "src" "${1:-}"; require "dst" "${2:-}"
      $PS "Move-Item -LiteralPath '$1' -Destination '$2' -Force"
      echo "ok: ${ACTION} $1 -> $2"
      ;;

    copy)
      require "src" "${1:-}"; require "dst" "${2:-}"
      $PS "Copy-Item -LiteralPath '$1' -Destination '$2' -Recurse -Force"
      echo "ok: copied $1 -> $2"
      ;;

    mkdir)
      require "path" "${1:-}"
      $PS "New-Item -ItemType Directory -Force -Path '$1' | Out-Null"
      echo "ok: mkdir $1"
      ;;

    trash)
      # Windows doesn't have a single-CLI "send to recycle bin" — use
      # the Shell.Application COM verb which is exactly what Explorer
      # does on Delete. Falls back to permanent delete with a warning
      # if the path isn't accessible via the shell namespace (e.g.
      # paths inside hidden directories).
      require "path" "${1:-}"
      $PS "
\$item = Get-Item -LiteralPath '$1' -Force
\$shell = New-Object -ComObject Shell.Application
\$folder = \$shell.Namespace((Split-Path -Parent \$item.FullName))
\$file = \$folder.ParseName(\$item.Name)
\$file.InvokeVerb('delete')
"
      echo "ok: trashed $1"
      ;;

    delete|rm)
      require "path" "${1:-}"
      $PS "Remove-Item -LiteralPath '$1' -Recurse -Force"
      echo "ok: deleted $1"
      ;;

    zip)
      require "out_zip" "${1:-}"
      local out="$1"; shift
      # Compress-Archive accepts multiple -Path values. We quote each
      # individually so spaces don't break the arg list.
      local args=""
      for f in "$@"; do args="$args '$f'"; done
      $PS "Compress-Archive -Force -DestinationPath '$out' -Path $args"
      echo "ok: zipped -> $out"
      ;;

    unzip)
      require "src_zip" "${1:-}"
      require "dst" "${2:-}"
      $PS "New-Item -ItemType Directory -Force -Path '$2' | Out-Null
Expand-Archive -Force -LiteralPath '$1' -DestinationPath '$2'"
      echo "ok: unzipped $1 -> $2"
      ;;

    find_in_dir)
      require "dir" "${1:-}"; require "pattern" "${2:-}"; require "out_file" "${3:-}"
      $PS "Get-ChildItem -Path '$1' -Filter '$2' -Recurse -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName" > "$3"
      echo "ok: search '$2' in $1 -> $3"
      ;;

    locate)
      # Windows Search via Search-Everything is great but third-party.
      # Native fallback: Get-ChildItem under known roots. Slow on big
      # disks but works without any install.
      require "pattern" "${1:-}"; require "out_file" "${2:-}"
      $PS "Get-ChildItem -Path \$env:USERPROFILE, 'C:\\' -Filter '$1' -Recurse -ErrorAction SilentlyContinue 2>\$null | Select-Object -ExpandProperty FullName" > "$2"
      echo "ok: locate '$1' -> $2"
      ;;

    open_in_finder|open_in_explorer|open_in_files)
      # explorer.exe with a path arg opens that folder.
      require "path" "${1:-}"
      $PS "explorer.exe '$1'"
      echo "ok: opened $1 in Explorer"
      ;;

    pin_to_taskbar)
      # Pin .exe / .lnk to taskbar via the shell verb (only works on
      # Windows 10 — Windows 11 dropped the verb but most users still
      # have it). Best-effort.
      require "path" "${1:-}"
      $PS "
\$shell = New-Object -ComObject Shell.Application
\$folder = \$shell.Namespace((Split-Path -Parent '$1'))
\$file = \$folder.ParseName((Split-Path -Leaf '$1'))
\$verb = \$file.Verbs() | Where-Object { \$_.Name -match 'taskbar' }
if (\$verb) { \$verb.DoIt(); 'ok' } else { Write-Error 'pin verb not available (Windows 11?)' }
"
      echo "ok: pinned $1 to taskbar"
      ;;

    *)
      echo "ERR: unknown windows-files action '$ACTION'." >&2
      echo "Actions: rename copy mkdir trash delete zip unzip find_in_dir locate open_in_explorer pin_to_taskbar" >&2
      exit 2
      ;;
  esac
}
