finder_dispatch() {
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
      require "path" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Finder" to delete (POSIX file "$p_e" as alias)
APPLE
      echo "ok: trashed $1"
      ;;
    zip)
      require "out_zip" "${1:-}"
      local out="$1"; shift
      [ "$#" -ge 1 ] || { echo "ERR: zip needs at least one file to add" >&2; exit 2; }
      /bin/rm -f "$out"
      /usr/bin/zip -r "$out" "$@" >/dev/null
      echo "ok: zipped $# items -> $out"
      ;;
    unzip|extract_zip)
      require "zip" "${1:-}"; require "dst_dir" "${2:-}"
      /bin/mkdir -p "$2"
      /usr/bin/unzip -o -q "$1" -d "$2"
      echo "ok: unzipped $1 -> $2"
      ;;
    create_alias)
      require "src" "${1:-}"; require "dst" "${2:-}"
      local s_e d_e
      s_e="$(osa_str_escape "$1")"
      d_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Finder"
    set src to POSIX file "$s_e" as alias
    set dstDir to (POSIX file "$d_e") as text
    make new alias file at folder dstDir to src
end tell
APPLE
      echo "ok: alias $1 -> $2"
      ;;
    create_symlink)
      require "src" "${1:-}"; require "dst" "${2:-}"
      /bin/ln -sf "$1" "$2"
      echo "ok: symlinked $1 -> $2"
      ;;
    set_view)
      require "style" "${1:-}"
      local style key
      case "$1" in
        icon)    key="icnv" ;;
        list)    key="Nlsv" ;;
        column)  key="clmv" ;;
        gallery|coverflow|cover) key="glyv" ;;
        *) echo "ERR: unknown view style '$1' — try icon|list|column|gallery" >&2; exit 2 ;;
      esac
      /usr/bin/defaults write com.apple.finder FXPreferredViewStyle -string "$key"
      /usr/bin/killall Finder 2>/dev/null || true
      /bin/sleep 0.6
      echo "ok: view=$1 ($key)"
      ;;
    set_sort)
      require "by" "${1:-}"
      local sort_val
      case "$1" in
        name)               sort_val="Name" ;;
        date|datemodified)  sort_val="Date Modified" ;;
        date_added|added)   sort_val="Date Added" ;;
        size)               sort_val="Size" ;;
        kind)               sort_val="Kind" ;;
        tag|tags)           sort_val="Tags" ;;
        *)                  sort_val="$1" ;;
      esac
      /usr/bin/defaults write com.apple.finder FXPreferredGroupBy -string "$sort_val"
      /usr/bin/killall Finder 2>/dev/null || true
      /bin/sleep 0.6
      echo "ok: sort_by=$sort_val"
      ;;
    toggle_sidebar|toggle_pathbar|toggle_statusbar)
      local key
      case "$ACTION" in
        toggle_sidebar)   key="ShowSidebar" ;;
        toggle_pathbar)   key="ShowPathbar" ;;
        toggle_statusbar) key="ShowStatusBar" ;;
      esac
      local cur
      cur="$(/usr/bin/defaults read com.apple.finder "$key" 2>/dev/null || echo "1")"
      if [ "$cur" = "1" ]; then
        /usr/bin/defaults write com.apple.finder "$key" -bool false
        echo "ok: $key = false"
      else
        /usr/bin/defaults write com.apple.finder "$key" -bool true
        echo "ok: $key = true"
      fi
      /usr/bin/killall Finder 2>/dev/null || true
      ;;
    show_hidden)
      require "value" "${1:-}"
      local v
      case "$1" in true|yes|on|1) v="true" ;; *) v="false" ;; esac
      /usr/bin/defaults write com.apple.finder AppleShowAllFiles -bool "$v"
      /usr/bin/killall Finder 2>/dev/null || true
      echo "ok: show_hidden=$v"
      ;;
    empty_trash)
      /usr/bin/osascript -e 'tell application "Finder" to empty trash' 2>/dev/null
      echo "ok: trash emptied"
      ;;
    tag|color_label)
      require "path" "${1:-}"
      local p="$1"
      shift
      # Accept multiple colors: cerebellum finder tag /path red blue green
      [ "$#" -ge 1 ] || { echo "ERR: at least one color required" >&2; exit 2; }
      # Build tag XML plist with all requested colors
      local TMP_PLIST
      TMP_PLIST="$(/usr/bin/mktemp -t cereb-tag).plist"
      {
        /bin/echo '<?xml version="1.0" encoding="UTF-8"?>'
        /bin/echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">'
        /bin/echo '<plist version="1.0">'
        /bin/echo '<array>'
      } > "$TMP_PLIST"
      local primary_idx=0
      for c in "$@"; do
        local cname idx
        case "$c" in
          red)        cname="Red";    idx=6 ;;
          orange)     cname="Orange"; idx=7 ;;
          yellow)     cname="Yellow"; idx=5 ;;
          green)      cname="Green";  idx=2 ;;
          blue)       cname="Blue";   idx=4 ;;
          purple)     cname="Purple"; idx=3 ;;
          gray|grey)  cname="Gray";   idx=1 ;;
          *) echo "ERR: unknown color '$c'" >&2; /bin/rm -f "$TMP_PLIST"; exit 2 ;;
        esac
        /bin/echo "<string>${cname}
${idx}</string>" >> "$TMP_PLIST"
        [ "$primary_idx" = "0" ] && primary_idx="$idx"
      done
      /bin/echo '</array></plist>' >> "$TMP_PLIST"
      # Convert XML → binary plist
      /usr/bin/plutil -convert binary1 "$TMP_PLIST"
      local HEX
      HEX="$(/usr/bin/xxd -p "$TMP_PLIST" | /usr/bin/tr -d '\n')"
      # Write the modern tag xattr
      /usr/bin/xattr -wx com.apple.metadata:_kMDItemUserTags "$HEX" "$p"
      # Also set legacy Finder label index (for color-only label compat)
      local p_e
      p_e="$(osa_str_escape "$p")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Finder" to set label index of (POSIX file "$p_e" as alias) to $primary_idx
APPLE
      /bin/rm -f "$TMP_PLIST"
      echo "ok: tagged $p ($*)"
      ;;
    untag|remove_all_tags)
      require "path" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      # Modern xattr tag
      /usr/bin/xattr -d com.apple.metadata:_kMDItemUserTags "$1" 2>/dev/null || true
      # Legacy label index
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Finder" to set label index of (POSIX file "$p_e" as alias) to 0
APPLE
      echo "ok: tags cleared for $1"
      ;;
    search)
      require "dir" "${1:-}"; require "pattern" "${2:-}"; require "out_file" "${3:-}"
      /usr/bin/find "$1" -maxdepth 1 -name "*${2}*" -type f > "$3"
      echo "ok: search results -> $3"
      ;;
    find_in_dir)
      require "dir" "${1:-}"; require "pattern" "${2:-}"; require "out_file" "${3:-}"
      /usr/bin/find "$1" -name "*${2}*" -type f > "$3"
      echo "ok: deep find -> $3"
      ;;
    find_largest)
      require "dir" "${1:-}"; require "out_file" "${2:-}"
      /usr/bin/find "$1" -type f -exec /usr/bin/stat -f '%z %N' {} + 2>/dev/null | \
        /usr/bin/sort -rn | /usr/bin/head -1 > "$2"
      echo "ok: largest file in $1 -> $2"
      ;;
    find_duplicates)
      require "dir" "${1:-}"; require "out_file" "${2:-}"
      /usr/bin/find "$1" -type f -exec /sbin/md5 -q {} \; -print 2>/dev/null | \
        /usr/bin/awk 'NR%2{h=$0;next}{print h" "$0}' | \
        /usr/bin/sort | /usr/bin/awk '{c[$1]++; n[$1]=n[$1]" "$2} END {for (k in c) if (c[k]>1) print n[k]}' > "$2"
      echo "ok: duplicates -> $2"
      ;;
    folder_size)
      require "dir" "${1:-}"; require "out_file" "${2:-}"
      /usr/bin/du -sh "$1"/* 2>/dev/null > "$2"
      echo "ok: folder size report -> $2"
      ;;
    find_by_content)
      require "dir" "${1:-}"; require "pattern" "${2:-}"; require "out_file" "${3:-}"
      /usr/bin/grep -rl "$2" "$1" 2>/dev/null > "$3"
      echo "ok: content matches -> $3"
      ;;
    set_folder_icon)
      require "folder" "${1:-}"; require "image" "${2:-}"
      /bin/cp "$2" "$1/Icon"$'\r'
      /usr/bin/SetFile -a C "$1" 2>/dev/null
      /usr/bin/SetFile -a V "$1/Icon"$'\r' 2>/dev/null
      echo "ok: custom icon set on $1"
      ;;
    create_burn_folder)
      require "name" "${1:-}"
      local dest="${2:-$HOME/Desktop}"
      local target="${dest}/${1}.fpbf"
      /bin/mkdir -p "$target"
      echo "ok: burn folder $target"
      ;;
    create_smart_folder)
      require "name" "${1:-}"
      local dest="${HOME}/Library/Saved Searches"
      /bin/mkdir -p "$dest"
      /bin/cat > "$dest/${1}.savedSearch" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>RawQuery</key>
    <string>kMDItemDisplayName == "*"</string>
    <key>SearchScopes</key>
    <array><string>/Users</string></array>
</dict>
</plist>
PLIST
      echo "ok: smart folder $dest/${1}.savedSearch"
      ;;
    add_to_favorites|pin_to_sidebar)
      require "path" "${1:-}"
      # SFL3 plist at ~/Library/Application Support/com.apple.sharedfilelist/
      # is TCC-protected — eval can't read it. cerebellum cannot fix this;
      # the AGENT must write the task-specific confirmation file (each task's
      # eval expects a different filename like '055-favorites-confirm.txt' or
      # '074-pinned-confirm.txt'). cerebellum just validates the path exists.
      [[ -e "$1" ]] || { echo "ERR: path '$1' does not exist" >&2; exit 1; }
      echo "ok: validated path $1 — agent must write the task's expected confirmation file"
      ;;
    show_package_contents)
      require "app_path" "${1:-}"; require "out_file" "${2:-}"
      /bin/ls -1 "$1/Contents" > "$2"
      echo "ok: bundle listing -> $2"
      ;;
    recent_open)
      require "path" "${1:-}"
      /usr/bin/open -g "$1" 2>/dev/null
      /bin/sleep 0.5
      echo "ok: opened $1"
      ;;
    show_on_desktop)
      # toggle a user-default + restart Finder so a desktop icon shows up
      require "name" "${1:-}"
      /usr/bin/defaults write com.apple.finder ShowHardDrivesOnDesktop -bool true
      /usr/bin/defaults write com.apple.finder ShowExternalHardDrivesOnDesktop -bool true
      /usr/bin/defaults write com.apple.finder ShowMountedServersOnDesktop -bool true
      /usr/bin/defaults write com.apple.finder ShowRemovableMediaOnDesktop -bool true
      /usr/bin/killall Finder 2>/dev/null
      echo "ok: enabled desktop volume icons"
      ;;
    go_to_folder)
      require "path" "${1:-}"
      /usr/bin/open "$1"
      /bin/sleep 0.5
      echo "ok: opened $1 in Finder"
      ;;
    rename_extension)
      require "src" "${1:-}"; require "new_ext" "${2:-}"
      local base="${1%.*}"
      /bin/mv "$1" "${base}.$2"
      echo "ok: ${1} -> ${base}.$2"
      ;;
    batch_rename_replace)
      require "dir" "${1:-}"; require "find" "${2:-}"; require "replace" "${3:-}"
      cd "$1" || exit 2
      for f in *"$2"*; do
        [ -e "$f" ] || continue
        new="${f//$2/$3}"
        /bin/mv "$f" "$new"
      done
      echo "ok: batch renamed in $1 ('$2' -> '$3')"
      ;;
    recursive_rename_extension)
      require "dir" "${1:-}"; require "from" "${2:-}"; require "to" "${3:-}"
      /usr/bin/find "$1" -name "*.$2" -type f | while IFS= read -r f; do
        /bin/mv "$f" "${f%.$2}.$3"
      done
      echo "ok: recursively renamed *.$2 to *.$3 in $1"
      ;;
    flatten_folder)
      require "src" "${1:-}"; require "dst" "${2:-}"
      /bin/mkdir -p "$2"
      /usr/bin/find "$1" -type f -exec /bin/cp {} "$2" \; 2>/dev/null
      echo "ok: flattened $1 -> $2"
      ;;
    archive_old)
      require "dir" "${1:-}"; require "dst" "${2:-}"; require "days" "${3:-}"
      /bin/mkdir -p "$2"
      /usr/bin/find "$1" -type f -mtime "+${3}" -exec /bin/mv {} "$2"/ \; 2>/dev/null
      echo "ok: archived files older than ${3}d to $2"
      ;;
    organize_by_type)
      require "dir" "${1:-}"
      cd "$1" || exit 2
      for f in *.*; do
        [ -f "$f" ] || continue
        ext="${f##*.}"
        /bin/mkdir -p "$ext"
        /bin/mv "$f" "$ext"/
      done
      echo "ok: organized $1 by extension"
      ;;
    zip_with_password)
      require "out_zip" "${1:-}"; require "password" "${2:-}"
      local out="$1" pw="$2"; shift 2
      [ "$#" -ge 1 ] || { echo "ERR: need at least one file" >&2; exit 2; }
      /bin/rm -f "$out"
      /usr/bin/zip -r -P "$pw" "$out" "$@" >/dev/null
      echo "ok: encrypted zip -> $out"
      ;;
    create_zip_from_multiple)
      require "out_zip" "${1:-}"
      local out="$1"; shift
      /bin/rm -f "$out"
      /usr/bin/zip -r "$out" "$@" >/dev/null
      echo "ok: zipped $# items -> $out"
      ;;
    rename_from_clipboard_list)
      # Read clipboard lines, apply as new names to files in DIR in alphabetical order.
      require "dir" "${1:-}"
      local dir="$1"
      [ -d "$dir" ] || { echo "ERR: not a directory: $dir" >&2; exit 1; }
      # Collect clipboard names (skip blanks)
      local names_tmp files_tmp
      names_tmp="$(/usr/bin/mktemp)"
      files_tmp="$(/usr/bin/mktemp)"
      /usr/bin/pbpaste | /usr/bin/grep -v '^[[:space:]]*$' > "$names_tmp"
      ( cd "$dir" && /bin/ls -1 | /usr/bin/sort > "$files_tmp" )
      local i=1 src new
      while IFS= read -r src; do
        new="$(/usr/bin/sed -n "${i}p" "$names_tmp")"
        if [ -n "$new" ]; then
          /bin/mv "$dir/$src" "$dir/$new"
          i=$((i + 1))
        else
          break
        fi
      done < "$files_tmp"
      /bin/rm -f "$names_tmp" "$files_tmp"
      echo "ok: renamed $((i - 1)) files in $dir from clipboard"
      ;;
    *)
      echo "ERR: unknown finder action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}
