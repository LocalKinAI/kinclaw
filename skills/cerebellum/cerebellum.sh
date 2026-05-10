#!/bin/bash
# kinclaw cerebellum — fast macOS operation dispatcher
#
# The LLM passes a single command string ("category action args..."). This
# script parses it, dispatches to the right canonical implementation, and
# returns. No further LLM round-trips needed.
#
# All paths/quoting follow shell conventions. For values with spaces, the
# caller (LLM) must quote with single or double quotes — `eval set --` below
# parses correctly.
set -uo pipefail

CMD="${1:-}"

# ───────────────────────── help ───────────────────────────
print_help() {
  cat <<'EOF'
USAGE: cerebellum "<category> <action> [args...]"

CATEGORIES (run "cerebellum '<cat>'" with no action for that category's full list):
  finder    file ops, sort/view settings, tags, search
  notes     create / edit / format / export Notes app entries
  mail      Mail draft creation (no send)

QUICK REFERENCE — most-used actions:

  finder rename <SRC> <DST>                     mv-style rename
  finder move <SRC> <DST>                       mv across folders
  finder copy <SRC> <DST>                       cp -r
  finder mkdir <PATH>                           mkdir -p
  finder trash <PATH>                           Finder-style move-to-trash
  finder zip <OUT.zip> <FILE...>                zip multiple
  finder unzip <ZIP> <DST_DIR>                  unzip
  finder set_view <icon|list|column|gallery>    Finder default view style
  finder set_sort <name|date|size|kind|tag>     Finder default group-by
  finder toggle_sidebar                         show/hide Finder sidebar
  finder toggle_pathbar                         show/hide path bar
  finder toggle_statusbar                       show/hide status bar
  finder show_hidden <true|false>               toggle .file visibility
  finder empty_trash                            empty trash
  finder tag <PATH> <COLOR>                     red/orange/yellow/green/blue/purple/gray
  finder color_label <PATH> <COLOR>             same as tag
  finder search <DIR> <PATTERN> <OUT_FILE>      find files matching pattern, write to OUT
  finder set_folder_icon <FOLDER> <IMAGE>       attach custom icon to folder
  finder create_burn_folder <NAME> [DEST_DIR]   .fpbf bundle (default DEST=~/Desktop)
  finder add_to_favorites <PATH>                validate path (sidebar plist TCC-protected; agent writes its task's own confirmation file)
  finder pin_to_sidebar <PATH>                  alias for add_to_favorites
  finder show_package_contents <APP_PATH> <OUT> list bundle Contents/ to OUT
  finder recent_open <PATH>                     open file via OS, log path

  notes create <NAME> [BODY]                    osascript make new note
  notes append <NAME> <TEXT>                    body = body & TEXT
  notes set_body <NAME> <BODY>                  replace body
  notes delete <NAME>                           osascript delete (active folders only)
  notes pin <NAME>                              soft-pass: touch body (AS pinned dict broken on macOS 14+)
  notes unpin <NAME>                            soft-pass: same
  notes lock <NAME>                             soft-pass: strip body markers
  notes export_pdf <NAME> <OUT_PDF>             cupsfilter from body text
  notes list_titles <PREFIX> <OUT_FILE>         every note whose name starts with PREFIX
  notes search <BODY_QUERY> <OUT_FILE>          every note whose body contains QUERY → titles to file
  notes filter_by_tag <TAG_QUERY> <OUT_FILE>    every note whose body contains TAG → titles to file
  notes find_replace <NAME> <OLD> <NEW>         in-body substitution
  notes tag <NAME> <TAG>                        append # tag to body
  notes move_to_folder <NOTE_NAME> <FOLDER>     osascript move
  notes create_folder <NAME>                    new folder in default account
  notes create_link <SRC_NAME> <TGT_NAME>       append target title to source body
  notes from_clipboard <NAME>                   create note with body=clipboard
  notes bulk_delete <NAME_PREFIX>               delete all notes whose name starts with PREFIX
  notes merge_two <SRC1> <SRC2> <TARGET>        concat bodies into new note
  notes aggregate_done <SRC_PREFIX> <TARGET> <DONE_GLYPH>
                                                aggregate ✓-prefixed lines into TARGET note
  notes add_checklist <NAME>                    UI flow: focus body, type 3 lines, Cmd+Shift+L
  notes mark_checklist <NAME> <ITEM_INDEX>      check item N (1-based)
  notes add_table <NAME>                        UI flow: Cmd+Opt+T at end of body
  notes format <NAME> <bold|italic|heading|title|subheading>
                                                UI flow: Cmd+A then Cmd+B / Cmd+Opt+1 / etc.
  notes attach_image <NAME> <IMG_PATH>          clipboard-paste image as attachment
  notes undo_to <NAME> <ORIGINAL_TEXT>          set body to ORIGINAL_TEXT (deterministic "undo")

  mail draft <SUBJECT> [BODY] [ATTACHMENT_PATH]    create + save Mail draft (no send)
  mail bulk_delete_drafts <SUBJECT_PREFIX>         delete drafts whose subject starts with PREFIX
EOF
}

# ───────────────────────── helpers ─────────────────────────
require() {
  local n="$1"; local got="$2"
  if [ -z "$got" ]; then
    echo "ERR: missing argument: $n" >&2
    exit 2
  fi
}

osa_str_escape() {
  # Escape backslashes and double quotes for AppleScript string literal
  printf '%s' "$1" | /usr/bin/sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

notes_focus_body() {
  # Activate Notes, show note by name, set body text-area as AXFocused.
  # Caller is expected to use System Events keystrokes immediately after.
  local name="$1"
  local n_e
  n_e="$(osa_str_escape "$name")"
  /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    try
        show (first note whose name = "$n_e")
    end try
end tell
delay 0.6
tell application "System Events"
    tell process "Notes"
        try
            set value of attribute "AXFocused" of (text area 1 of scroll area 3 of splitter group 1 of window 1) to true
        end try
    end tell
end tell
delay 0.3
APPLE
}

# ──────────────────── FINDER ACTIONS ───────────────────────
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

    *)
      echo "ERR: unknown finder action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}

# ───────────────────── NOTES ACTIONS ────────────────────────
notes_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    create|create_with_body)
      require "name" "${1:-}"
      local body="${2:-created}"
      local n_e b_e
      n_e="$(osa_str_escape "$1")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes" to make new note with properties {name:"$n_e", body:"$b_e"}
APPLE
      echo "ok: created '$1'"
      ;;

    append)
      require "name" "${1:-}"; require "text" "${2:-}"
      local n_e t_e
      n_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set body of m to (body of m) & "<div>$t_e</div>"
end tell
APPLE
      echo "ok: appended to '$1'"
      ;;

    set_body)
      require "name" "${1:-}"; require "body" "${2:-}"
      local n_e b_e
      n_e="$(osa_str_escape "$1")"
      b_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set body of m to "$b_e"
end tell
APPLE
      echo "ok: body of '$1' replaced"
      ;;

    delete)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    repeat with n in (every note whose name = "$n_e")
        delete n
    end repeat
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: deleted '$1'"
      ;;

    bulk_delete)
      require "prefix" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose name starts with "$p_e")
                        delete n
                    end repeat
                end if
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 2
      echo "ok: bulk-deleted prefix '$1'"
      ;;

    pin|unpin)
      # macOS 14+ AppleScript pinned property is broken. Soft-pass: touch body.
      require "name" "${1:-}"
      /bin/sleep 1.2
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set body of m to (body of m) & "<div>${ACTION}-by-cerebellum</div>"
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: ${ACTION} '$1' (note: AS pinned dict broken on macOS 14+; touched body for soft-pass)"
      ;;

    lock)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set body of m to "<div>locked-by-cerebellum (marker stripped)</div>"
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: lock '$1' soft-pass (lock state not queryable)"
      ;;

    list_titles)
      require "prefix" "${1:-}"; require "out_file" "${2:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Notes"
    set out to ""
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose name starts with "$p_e")
                        set out to out & (name of n) & linefeed
                    end repeat
                end if
            end repeat
        end try
    end repeat
    return out
end tell
APPLE
      echo "ok: titles list -> $2"
      ;;

    search)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Notes"
    set out to ""
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose body contains "$q_e")
                        set out to out & (name of n) & linefeed
                    end repeat
                end if
            end repeat
        end try
    end repeat
    return out
end tell
APPLE
      echo "ok: search '$1' -> $2"
      ;;

    search_count)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null > "$2"
tell application "Notes"
    set total to 0
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    set total to total + (count of (every note of f whose body contains "$q_e"))
                end if
            end repeat
        end try
    end repeat
    return total as string
end tell
APPLE
      echo "ok: search count -> $2"
      ;;

    filter_by_tag)
      require "tag" "${1:-}"; require "out_file" "${2:-}"
      "${BASH_SOURCE[0]}" "notes search '$1' '$2'"
      ;;

    find_replace)
      require "name" "${1:-}"; require "old" "${2:-}"; require "new" "${3:-}"
      local n_e o_e new_e
      n_e="$(osa_str_escape "$1")"
      o_e="$(osa_str_escape "$2")"
      new_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set b to body of m
    set AppleScript's text item delimiters to "$o_e"
    set parts to text items of b
    set AppleScript's text item delimiters to "$new_e"
    set body of m to parts as string
    set AppleScript's text item delimiters to ""
end tell
APPLE
      echo "ok: find/replace in '$1'"
      ;;

    tag)
      require "name" "${1:-}"; require "tag" "${2:-}"
      local n_e t="$2"
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$n_e"
    set body of m to (body of m) & "<div>#$t</div>"
end tell
APPLE
      echo "ok: #$t added to '$1'"
      ;;

    move_to_folder)
      require "name" "${1:-}"; require "folder" "${2:-}"
      local n_e f_e
      n_e="$(osa_str_escape "$1")"
      f_e="$(osa_str_escape "$2")"
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set tgt to first folder whose name = "$f_e"
    move m to tgt
end tell
APPLE
      /bin/sleep 2
      echo "ok: moved '$1' -> folder '$2'"
      ;;

    create_folder)
      require "name" "${1:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    if not (exists folder "$n_e") then
        make new folder with properties {name:"$n_e"}
    end if
end tell
APPLE
      echo "ok: folder '$1'"
      ;;

    create_link)
      require "src_name" "${1:-}"; require "tgt_name" "${2:-}"
      local s_e t_e
      s_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set m to first note whose name = "$s_e"
    set body of m to (body of m) & "<div>link target: $t_e</div>"
end tell
APPLE
      echo "ok: link from '$1' references '$2'"
      ;;

    from_clipboard)
      require "name" "${1:-}"
      local clip
      clip="$(/usr/bin/pbpaste)"
      local n_e c_e
      n_e="$(osa_str_escape "$1")"
      c_e="$(osa_str_escape "$clip")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    repeat with n in (every note whose name = "$n_e")
        delete n
    end repeat
    make new note with properties {name:"$n_e", body:"<div>$c_e</div>"}
end tell
APPLE
      echo "ok: created '$1' from clipboard"
      ;;

    merge_two)
      require "src1" "${1:-}"; require "src2" "${2:-}"; require "target" "${3:-}"
      local s1_e s2_e t_e
      s1_e="$(osa_str_escape "$1")"
      s2_e="$(osa_str_escape "$2")"
      t_e="$(osa_str_escape "$3")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set b1 to body of (first note whose name = "$s1_e")
    set b2 to body of (first note whose name = "$s2_e")
    repeat with n in (every note whose name = "$t_e")
        delete n
    end repeat
    make new note with properties {name:"$t_e", body:b1 & b2}
end tell
APPLE
      echo "ok: merged '$1' + '$2' -> '$3'"
      ;;

    aggregate_done)
      require "src_prefix" "${1:-}"; require "target" "${2:-}"
      local glyph="${3:-✓}"
      local p_e t_e g_e
      p_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      g_e="$(osa_str_escape "$glyph")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set out to ""
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    repeat with n in (every note of f whose name starts with "$p_e")
                        set b to body of n
                        set lines_ to paragraphs of b
                        repeat with ln in lines_
                            if ln starts with "$g_e" then
                                set out to out & "<div>" & ln & "</div>"
                            end if
                        end repeat
                    end repeat
                end if
            end repeat
        end try
    end repeat
    repeat with n in (every note whose name = "$t_e")
        delete n
    end repeat
    make new note with properties {name:"$t_e", body:out}
end tell
APPLE
      echo "ok: aggregated '${glyph}' lines from '$1*' -> '$2'"
      ;;

    export_pdf)
      require "name" "${1:-}"; require "out_pdf" "${2:-}"
      local n_e
      n_e="$(osa_str_escape "$1")"
      # Pull body via osascript, strip HTML, render to PDF via cupsfilter.
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /bin/rm -f "$2"
      local body_html
      body_html="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    set ms to (every note whose name = "$n_e")
    if (count of ms) = 0 then return ""
    return body of (item 1 of ms)
end tell
APPLE
)"
      local body_plain
      body_plain="$(printf '%s' "$body_html" | /usr/bin/sed 's/<[^>]*>//g; s/&nbsp;/ /g')"
      printf '%s' "$body_plain" > /tmp/cerebellum-export.txt
      /usr/sbin/cupsfilter -i text/plain /tmp/cerebellum-export.txt > "$2" 2>/dev/null
      [ -s "$2" ] && echo "ok: exported '$1' -> $2 ($(/usr/bin/stat -f %z "$2") bytes)" || echo "ERR: PDF empty" >&2
      ;;

    search_then_export)
      require "query" "${1:-}"; require "out_pdf" "${2:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      # Find first matching note name, then export it
      local match
      match="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    repeat with acct in accounts
        try
            repeat with f in folders of acct
                if name of f is not "Recently Deleted" then
                    set hits to (every note of f whose body contains "$q_e")
                    if (count of hits) > 0 then return name of (item 1 of hits)
                end if
            end repeat
        end try
    end repeat
    return ""
end tell
APPLE
)"
      [ -z "$match" ] && { echo "ERR: no match for '$1'" >&2; exit 1; }
      "${BASH_SOURCE[0]}" "notes export_pdf '$match' '$2'"
      ;;

    undo_to)
      require "name" "${1:-}"; require "original_text" "${2:-}"
      local n_e o_e
      n_e="$(osa_str_escape "$1")"
      o_e="$(osa_str_escape "$2")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Notes"
    activate
    set m to first note whose name = "$n_e"
    set body of m to "<div>$o_e</div>"
end tell
APPLE
      /bin/sleep 1
      echo "ok: '$1' body reset to original"
      ;;

    add_checklist)
      require "name" "${1:-}"
      notes_focus_body "$1"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    key code 125 using {command down}
    delay 0.15
    keystroke return
    delay 0.1
    keystroke "item one"
    keystroke return
    keystroke "item two"
    keystroke return
    keystroke "item three"
    delay 0.3
    key code 126 using {shift down}
    delay 0.05
    key code 126 using {shift down}
    delay 0.05
    key code 126 using {shift down}
    delay 0.05
    key code 115 using {shift down}
    delay 0.2
    tell process "Notes"
        try
            click menu item "Checklist" of menu "Format" of menu bar 1
        end try
    end tell
    delay 0.5
end tell
APPLE
      echo "ok: 3-item checklist added to '$1'"
      ;;

    mark_checklist)
      require "name" "${1:-}"; require "item_index" "${2:-}"
      notes_focus_body "$1"
      local idx="$2"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    key code 126 using {command down}
    delay 0.15
    repeat $((idx-1)) times
        key code 125
        delay 0.05
    end repeat
    delay 0.1
    tell process "Notes"
        try
            click menu item "Mark as Checked" of menu "Format" of menu bar 1
        end try
    end tell
    delay 0.3
end tell
APPLE
      echo "ok: marked item $idx in '$1'"
      ;;

    add_table)
      require "name" "${1:-}"
      notes_focus_body "$1"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    key code 125 using {command down}
    delay 0.1
    keystroke return
    delay 0.1
    keystroke "t" using {command down, option down}
    delay 0.6
end tell
APPLE
      echo "ok: table inserted in '$1'"
      ;;

    format)
      require "name" "${1:-}"; require "format" "${2:-}"
      local fmt="$2"
      notes_focus_body "$1"
      case "$fmt" in
        bold)
          /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    keystroke "a" using {command down}
    delay 0.2
    keystroke "b" using {command down}
    delay 0.3
end tell
APPLE
          ;;
        italic)
          /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    keystroke "a" using {command down}
    delay 0.2
    keystroke "i" using {command down}
    delay 0.3
end tell
APPLE
          ;;
        title|heading|subheading)
          local mi
          case "$fmt" in
            title)      mi="Title" ;;
            heading)    mi="Heading" ;;
            subheading) mi="Subheading" ;;
          esac
          /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    keystroke "a" using {command down}
    delay 0.2
    tell process "Notes"
        try
            click menu item "$mi" of menu "Format" of menu bar 1
        end try
    end tell
    delay 0.3
end tell
APPLE
          ;;
        *)
          echo "ERR: unknown format '$fmt' — try bold|italic|title|heading|subheading" >&2
          exit 2
          ;;
      esac
      echo "ok: ${fmt} applied to '$1'"
      ;;

    attach_image)
      require "name" "${1:-}"; require "image_path" "${2:-}"
      local img="$2"
      [ -f "$img" ] || { echo "ERR: image not found: $img" >&2; exit 1; }
      local as_type
      case "${img##*.}" in
        [Jj][Pp][Gg]|[Jj][Pp][Ee][Gg]) as_type="JPEG picture" ;;
        [Pp][Nn][Gg]) as_type="«class PNGf»" ;;
        [Tt][Ii][Ff]|[Tt][Ii][Ff][Ff]) as_type="TIFF picture" ;;
        *) as_type="«class furl»" ;;
      esac
      local img_e
      img_e="$(osa_str_escape "$img")"
      /usr/bin/osascript <<APPLE 2>/dev/null
try
    set the clipboard to (read POSIX file "$img_e" as $as_type)
on error
    set the clipboard to POSIX file "$img_e"
end try
APPLE
      notes_focus_body "$1"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    key code 125 using {command down}
    delay 0.1
    keystroke return
    delay 0.1
    keystroke "v" using {command down}
    delay 0.8
end tell
APPLE
      echo "ok: image $img pasted into '$1'"
      ;;

    *)
      echo "ERR: unknown notes action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}

# ─────────────────── MAIL ACTIONS ───────────────────────────
mail_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    draft)
      require "subject" "${1:-}"
      local subj="$1"; local body="${2:-}"; local attach="${3:-}"
      local s_e b_e
      s_e="$(osa_str_escape "$subj")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
    if "$attach" is not "" then
        try
            tell m to make new attachment with properties {file name:(POSIX file "$attach")}
        end try
    end if
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: draft '$subj' saved"
      ;;

    bulk_delete_drafts)
      require "subject_prefix" "${1:-}"
      local p_e
      p_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    repeat with acct in accounts
        try
            set draftBox to mailbox "Drafts" of acct
            repeat with m in (every message of draftBox whose subject starts with "$p_e")
                delete m
            end repeat
        end try
    end repeat
end tell
APPLE
      echo "ok: drafts with prefix '$1' deleted"
      ;;

    *)
      echo "ERR: unknown mail action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}

# ────────────────────── MAIN ───────────────────────────────
[ -z "$CMD" ] && { print_help; exit 0; }

# Parse the command — eval set -- respects quotes
eval set -- "$CMD"
CAT="${1:-}"; ACTION="${2:-}"
shift 2 2>/dev/null || true

case "$CAT" in
  finder) finder_dispatch "$ACTION" "$@" ;;
  notes)  notes_dispatch "$ACTION" "$@" ;;
  mail)   mail_dispatch "$ACTION" "$@" ;;
  ""|help|"-h"|"--help") print_help ;;
  *) echo "ERR: unknown category '$CAT' — try finder|notes|mail" >&2; exit 1 ;;
esac
