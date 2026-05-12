# cerebellum/categories/linux-files.sh
# Linux file ops — mirror of macOS finder.sh, but using POSIX tools
# instead of Finder AppleScript. Most actions are direct ports
# (the underlying ops — mv, cp, rm, mkdir, zip, find — are universal);
# the few macOS-specific actions (Finder sidebar, tags, Quick Look,
# Spotlight, smart folders) are SKIP_LINUX or use Linux equivalents
# (Nautilus bookmarks file, extended attributes for tags, mlocate).

linux-files_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    rename|move)
      require "src" "${1:-}"; require "dst" "${2:-}"
      /bin/mv "$1" "$2"
      echo "ok: ${ACTION} $1 -> $2"
      ;;

    copy)
      require "src" "${1:-}"; require "dst" "${2:-}"
      /bin/cp -R "$1" "$2"
      echo "ok: copied $1 -> $2"
      ;;

    mkdir)
      require "path" "${1:-}"
      /bin/mkdir -p "$1"
      echo "ok: mkdir $1"
      ;;

    trash)
      # gio is the standard freedesktop "send to trash" cmd; fall
      # back to mv-to-~/.local/share/Trash if not present.
      require "path" "${1:-}"
      if command -v gio >/dev/null 2>&1; then
        gio trash "$1"
      else
        /bin/mkdir -p "$HOME/.local/share/Trash/files"
        /bin/mv "$1" "$HOME/.local/share/Trash/files/"
      fi
      echo "ok: trashed $1"
      ;;

    delete|rm)
      require "path" "${1:-}"
      /bin/rm -rf "$1"
      echo "ok: deleted $1"
      ;;

    zip)
      require "out_zip" "${1:-}"
      local out="$1"; shift
      /bin/rm -f "$out"
      /usr/bin/zip -r "$out" "$@" >/dev/null
      echo "ok: zipped -> $out"
      ;;

    unzip)
      require "src_zip" "${1:-}"
      require "dst" "${2:-}"
      /bin/mkdir -p "$2"
      /usr/bin/unzip -o "$1" -d "$2" >/dev/null
      echo "ok: unzipped $1 -> $2"
      ;;

    find_in_dir)
      # find files matching a pattern, write paths to OUT
      require "dir" "${1:-}"; require "pattern" "${2:-}"; require "out_file" "${3:-}"
      /usr/bin/find "$1" -name "$2" > "$3" 2>/dev/null || true
      echo "ok: search '$2' in $1 -> $3"
      ;;

    locate)
      # mlocate / plocate index-based search (much faster than find for system-wide)
      require "pattern" "${1:-}"; require "out_file" "${2:-}"
      if command -v plocate >/dev/null 2>&1; then
        plocate "$1" > "$2" 2>/dev/null || true
      elif command -v locate >/dev/null 2>&1; then
        /usr/bin/locate "$1" > "$2" 2>/dev/null || true
      else
        echo "ERR: install plocate or mlocate for locate action" >&2
        exit 2
      fi
      echo "ok: locate '$1' -> $2"
      ;;

    open_in_finder|open_in_files)
      # Cross-DE: nautilus / dolphin / nemo / pcmanfm / xdg-open
      require "path" "${1:-}"
      if command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$1" >/dev/null 2>&1 &
      elif command -v nautilus >/dev/null 2>&1; then
        nautilus --no-desktop "$1" >/dev/null 2>&1 &
      elif command -v dolphin >/dev/null 2>&1; then
        dolphin "$1" >/dev/null 2>&1 &
      fi
      echo "ok: opened $1 in file manager"
      ;;

    tag)
      # Set extended-attribute "user.xdg.tags" (XDG tags spec) for one or more tags.
      # Args: PATH TAG1 [TAG2 ...]
      require "path" "${1:-}"
      local path="$1"; shift
      local tags="$(IFS=,; echo "$*")"
      /usr/bin/setfattr -n user.xdg.tags -v "$tags" "$path" 2>/dev/null || {
        # ext-attr fallback for non-FS-supporting filesystems
        echo "$tags" > "${path}.tags"
      }
      echo "ok: tagged $path with $tags"
      ;;

    add_to_favorites|bookmark)
      # GNOME / freedesktop bookmark — write to ~/.config/gtk-3.0/bookmarks
      # KDE has its own location; skip for now (Phase 5+).
      require "path" "${1:-}"
      local bookmarks="$HOME/.config/gtk-3.0/bookmarks"
      /bin/mkdir -p "$(dirname "$bookmarks")"
      echo "file://$1" >> "$bookmarks"
      echo "ok: added $1 to GTK bookmarks"
      ;;

    *)
      echo "ERR: unknown linux-files action '$ACTION'." >&2
      echo "Actions: rename copy mkdir trash delete zip unzip find_in_dir locate open_in_files tag add_to_favorites" >&2
      exit 2
      ;;
  esac
}
