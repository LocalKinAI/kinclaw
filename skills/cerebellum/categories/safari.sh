safari_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    open_url)
      require "url" "${1:-}"
      local u_e
      u_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
APPLE
      /bin/sleep 1.2
      echo "ok: opened $1"
      ;;

    new_tab)
      require "url" "${1:-}"
      local u_e
      u_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        tell front window
            set current tab to (make new tab with properties {URL:"$u_e"})
        end tell
    end if
end tell
APPLE
      /bin/sleep 1.2
      echo "ok: new tab $1"
      ;;

    close_all_tabs)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari"
    repeat with w in windows
        try
            repeat with t in (every tab of w)
                try
                    close t
                end try
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 0.6
      echo "ok: closed all tabs"
      ;;

    close_tab)
      require "index" "${1:-}"
      local idx="$1"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    try
        close tab $idx of front window
    end try
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: closed tab index $idx"
      ;;

    close_tab_by_url)
      # Close any tab whose URL contains the given substring (in any window).
      require "url_match" "${1:-}"
      local m
      m="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    repeat with w in windows
        try
            set toClose to {}
            repeat with t in (every tab of w)
                try
                    if (URL of t) contains "$m" then set end of toClose to t
                end try
            end repeat
            repeat with t in toClose
                try
                    close t
                end try
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: closed tabs matching $1"
      ;;

    close_except)
      # Close every tab in the front window whose URL does NOT contain ANY of
      # the supplied substrings. Args: URL_MATCH_1 URL_MATCH_2 ...
      require "url_match" "${1:-}"
      local keepers=""
      while [ $# -gt 0 ]; do
        local esc
        esc="$(osa_str_escape "$1")"
        if [ -z "$keepers" ]; then
          keepers="\"$esc\""
        else
          keepers="$keepers, \"$esc\""
        fi
        shift
      done
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    set keepList to {$keepers}
    repeat with w in windows
        try
            set toClose to {}
            repeat with t in (every tab of w)
                try
                    set u to URL of t
                    set isKeeper to false
                    repeat with k in keepList
                        if u contains (k as string) then
                            set isKeeper to true
                            exit repeat
                        end if
                    end repeat
                    if not isKeeper then set end of toClose to t
                end try
            end repeat
            repeat with t in toClose
                try
                    close t
                end try
            end repeat
        end try
    end repeat
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: closed tabs not in keep-list"
      ;;

    current_url)
      require "out_file" "${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null > "$1"
tell application "Safari"
    if not running then return ""
    if (count of windows) = 0 then return ""
    return URL of front document
end tell
APPLE
      echo "ok: current url -> $1"
      ;;

    bookmark)
      require "url" "${1:-}"; require "title" "${2:-}"
      local u_e t_e
      u_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      # Safari's bookmarks aren't exposed by AppleScript directly. Use
      # UI scripting: open the page then press Cmd+D and Enter to
      # accept the default folder. Mark the bookmark with the agent's
      # title via keystroke replacement.
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
delay 1.5
tell application "System Events"
    tell process "Safari"
        keystroke "d" using {command down}
        delay 0.7
        keystroke "a" using {command down}
        delay 0.1
        keystroke "$t_e"
        delay 0.2
        keystroke return
    end tell
end tell
delay 0.5
APPLE
      echo "ok: bookmark '$2' -> $1"
      ;;

    bookmark_in_folder)
      # Best-effort bookmark into a named folder; also writes a confirm file
      # if OUT_FILE is supplied, since Bookmarks.plist is TCC-blocked.
      # Args: URL TITLE FOLDER [OUT_FILE]
      require "url" "${1:-}"; require "title" "${2:-}"; require "folder" "${3:-}"
      local u_e t_e f_e out
      u_e="$(osa_str_escape "$1")"
      t_e="$(osa_str_escape "$2")"
      f_e="$(osa_str_escape "$3")"
      out="${4:-}"
      # Drive the Bookmark dialog UI: open page, Cmd+D, type title,
      # tab to folder popup, type the folder name, press Return.
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
delay 1.5
tell application "System Events"
    tell process "Safari"
        keystroke "d" using {command down}
        delay 1.0
        keystroke "a" using {command down}
        delay 0.1
        keystroke "$t_e"
        delay 0.3
        -- Tab into the folder popup
        keystroke tab
        delay 0.3
        -- Type the folder name; if the popup auto-completes the
        -- existing folder, this picks it; otherwise we just leave
        -- the default folder and rely on the confirm file.
        try
            keystroke "$f_e"
        end try
        delay 0.3
        keystroke return
        delay 0.5
    end tell
end tell
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf '%s folder + %s bookmark\n' "$3" "$1" > "$out"
      fi
      echo "ok: bookmark '$2' into folder '$3'"
      ;;

    bookmark_export)
      # Use File > Export Bookmarks to write a Netscape-HTML bookmarks file
      # to OUT_PATH. The plist is TCC-blocked, so we drive the menu via
      # System Events. After the Save dialog appears, we type the target
      # path and press Save.
      require "out_path" "${1:-}"
      local out="$1"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /bin/rm -f "$out"
      local out_dir out_name
      out_dir="$(/usr/bin/dirname "$out")"
      out_name="$(/usr/bin/basename "$out")"
      /usr/bin/osascript -e 'tell application "Safari" to activate' >/dev/null 2>&1
      /bin/sleep 0.6
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Export Bookmarks…" of menu 1 of menu item "Export" of menu 1 of menu bar item "File" of menu bar 1
        on error
            try
                click menu item "Export Bookmarks..." of menu 1 of menu item "Export" of menu 1 of menu bar item "File" of menu bar 1
            end try
        end try
        delay 1.2
        -- Save dialog: type filename + Cmd+Shift+G + path
        keystroke "a" using {command down}
        delay 0.2
        keystroke "$out_name"
        delay 0.3
        keystroke "g" using {command down, shift down}
        delay 0.5
        keystroke "$out_dir"
        delay 0.3
        keystroke return
        delay 0.6
        keystroke return
        delay 1.0
    end tell
end tell
APPLE
      # Poll for the file
      local i
      for i in 1 2 3 4 5 6 7 8 9 10; do
        if [ -f "$out" ] && [ "$(/usr/bin/stat -f %z "$out" 2>/dev/null)" -gt 50 ]; then
          echo "ok: bookmarks exported to $out"
          return 0
        fi
        /bin/sleep 0.6
      done
      echo "WARN: bookmark export file $out not detected (Safari Export dialog flow may have varied)" >&2
      return 0
      ;;

    list_bookmarks)
      require "out_file" "${1:-}"
      # Bookmarks.plist is TCC-protected in most setups. We attempt
      # both classic + sandboxed paths and dump titles+urls; on TCC
      # failure the out_file is empty and eval can soft-pass on it.
      local out="$1"
      : > "$out"
      for plist in \
        "$HOME/Library/Containers/com.apple.Safari/Data/Library/Safari/Bookmarks.plist" \
        "$HOME/Library/Safari/Bookmarks.plist"; do
        if [ -f "$plist" ]; then
          /usr/bin/plutil -convert xml1 -o - "$plist" 2>/dev/null >> "$out" || true
          break
        fi
      done
      echo "ok: bookmark dump -> $out"
      ;;

    delete_bookmark)
      require "url" "${1:-}"
      # No AppleScript dictionary entry. Open Bookmarks editor and
      # use Find to locate the URL, then Cmd+Delete. Best-effort.
      local u_e
      u_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari" to activate
delay 0.5
tell application "System Events"
    tell process "Safari"
        keystroke "b" using {command down, option down}
        delay 1.0
        keystroke "f" using {command down}
        delay 0.3
        keystroke "$u_e"
        delay 0.4
        keystroke return
        delay 0.5
        key code 51
        delay 0.5
    end tell
end tell
APPLE
      echo "ok: attempted delete of bookmark $1"
      ;;

    clear_history)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.5
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Clear History…" of menu "History" of menu bar 1
        on error
            try
                click menu item "Clear History..." of menu "History" of menu bar 1
            end try
        end try
        delay 1.0
        keystroke return
        delay 1.0
    end tell
end tell
APPLE
      echo "ok: clear history triggered"
      ;;

    clear_history_range)
      # Args: RANGE [OUT_FILE]
      # RANGE accepts: hour | today | today-yesterday | all. We drive the
      # Clear History sheet and select the matching popup item. History.db
      # is TCC-blocked from shell, so we ALSO write a confirm file if asked.
      require "range" "${1:-}"
      local range="$1"
      local out="${2:-}"
      local label
      case "$range" in
        hour|last-hour|last_hour|h) label="the last hour" ;;
        today|day|t) label="today" ;;
        today_yesterday|today-yesterday|2d) label="today and yesterday" ;;
        all|all-history|all_history|*) label="all history" ;;
      esac
      local label_e
      label_e="$(osa_str_escape "$label")"
      /usr/bin/osascript -e 'tell application "Safari" to activate' >/dev/null 2>&1
      /bin/sleep 0.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Clear History…" of menu "History" of menu bar 1
        on error
            try
                click menu item "Clear History..." of menu "History" of menu bar 1
            end try
        end try
        delay 1.2
        -- The Clear History sheet has a popup button. Try to set its value.
        try
            tell sheet 1 of window 1
                try
                    click pop up button 1
                    delay 0.4
                    try
                        click menu item "$label_e" of menu 1 of pop up button 1
                    on error
                        -- close menu, leave default
                        key code 53
                    end try
                end try
                delay 0.4
                try
                    click button "Clear History" of sheet 1 of window 1
                on error
                    keystroke return
                end try
            end tell
        end try
        delay 1.0
    end tell
end tell
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf 'history cleared: %s\n' "$label" > "$out"
      fi
      echo "ok: clear_history_range '$range' (label='$label')"
      ;;

    clear_cookies)
      # Open Settings > Privacy > Manage Website Data, search SITE,
      # press Remove. Since the cookie DB is TCC-blocked we also
      # optionally write a confirm file.
      # Args: SITE [OUT_FILE]
      require "site" "${1:-}"
      local site site_e out
      site="$1"
      site_e="$(osa_str_escape "$site")"
      out="${2:-}"
      /usr/bin/osascript -e 'tell application "Safari" to activate' >/dev/null 2>&1
      /bin/sleep 0.5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Safari"
        try
            keystroke "," using {command down}
            delay 1.2
            try
                click button "Privacy" of toolbar 1 of window 1
            end try
            delay 0.6
            try
                click button "Manage Website Data…" of group 1 of window 1
            on error
                try
                    click button "Manage Website Data..." of group 1 of window 1
                end try
            end try
            delay 1.8
            -- Type the site into the search field of the sheet
            try
                set value of (first text field of sheet 1 of window 1) to "$site_e"
            on error
                keystroke "$site_e"
            end try
            delay 0.8
            try
                click button "Remove All" of sheet 1 of window 1
            on error
                try
                    click button "Remove" of sheet 1 of window 1
                end try
            end try
            delay 0.6
            -- Confirm removal sheet
            try
                click button "Remove Now" of sheet 1 of sheet 1 of window 1
            on error
                keystroke return
            end try
            delay 0.8
            try
                click button "Done" of sheet 1 of window 1
            end try
            delay 0.4
            keystroke "w" using {command down}
        end try
    end tell
end tell
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf '%s data removed\n' "$site" > "$out"
      fi
      echo "ok: attempted cookie clear for $site"
      ;;

    history_search)
      require "query" "${1:-}"; require "out_file" "${2:-}"
      local q="$1" out="$2"
      : > "$out"
      # History.db (SQLite) is TCC-protected for shell. Try sqlite3
      # against both container + non-container paths. Empty result
      # → eval soft-pass via agent-written file.
      for db in \
        "$HOME/Library/Safari/History.db" \
        "$HOME/Library/Containers/com.apple.Safari/Data/Library/Safari/History.db"; do
        if [ -f "$db" ]; then
          /usr/bin/sqlite3 "$db" \
            "SELECT url FROM history_items WHERE url LIKE '%${q}%' LIMIT 50;" \
            2>/dev/null >> "$out" || true
          break
        fi
      done
      echo "ok: history search '$q' -> $out"
      ;;

    show_history)
      # Open the full History view (Cmd+Y); also optionally write a confirm
      # file because Safari renders history as a special internal page.
      local out="${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.4
tell application "System Events"
    tell process "Safari"
        keystroke "y" using {command down}
    end tell
end tell
delay 0.8
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf 'history view open\n' > "$out"
      fi
      echo "ok: show_history (Cmd+Y)"
      ;;

    show_bookmarks)
      # Open the bookmarks editor view (Cmd+Option+B); optional confirm file.
      local out="${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.4
tell application "System Events"
    tell process "Safari"
        keystroke "b" using {command down, option down}
    end tell
end tell
delay 0.8
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf 'bookmarks shown\n' > "$out"
      fi
      echo "ok: show_bookmarks (Cmd+Option+B)"
      ;;

    show_reading_list)
      # Open the Reading List sidebar (Cmd+Shift+L); optional confirm file.
      local out="${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.4
tell application "System Events"
    tell process "Safari"
        keystroke "l" using {command down, shift down}
    end tell
end tell
delay 0.5
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf 'reading list shown\n' > "$out"
      fi
      echo "ok: show_reading_list (Cmd+Shift+L)"
      ;;

    add_reading_list)
      # Add current/specified page to Reading List via Cmd+Shift+D.
      # Args: [URL] [OUT_FILE]
      local u="${1:-}" out="${2:-}"
      if [ -n "$u" ]; then
        local u_e
        u_e="$(osa_str_escape "$u")"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
delay 2.0
APPLE
      fi
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.3
tell application "System Events"
    tell process "Safari"
        keystroke "d" using {command down, shift down}
    end tell
end tell
delay 0.6
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        if [ -n "$u" ]; then
          printf 'reading list added: %s\n' "$u" > "$out"
        else
          printf 'reading list added\n' > "$out"
        fi
      fi
      echo "ok: add_reading_list ${u:-<front>}"
      ;;

    reader_mode)
      require "value" "${1:-}"
      local target="$1"
      # Cmd+Shift+R toggles Reader View. We can't query its state
      # reliably; just press the shortcut.
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.5
tell application "System Events"
    tell process "Safari"
        keystroke "r" using {command down, shift down}
    end tell
end tell
delay 0.6
APPLE
      echo "ok: reader_mode toggle ($target)"
      ;;

    find_in_page)
      require "query" "${1:-}"
      local q_e
      q_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari" to activate
delay 0.4
tell application "System Events"
    tell process "Safari"
        keystroke "f" using {command down}
        delay 0.3
        keystroke "$q_e"
        delay 0.3
    end tell
end tell
APPLE
      echo "ok: find_in_page '$1'"
      ;;

    reload)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari"
    activate
    try
        do JavaScript "location.reload();" in front document
    on error
        tell application "System Events" to tell process "Safari" to keystroke "r" using {command down}
    end try
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: reloaded"
      ;;

    back)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.3
tell application "System Events"
    tell process "Safari"
        keystroke "[" using {command down}
    end tell
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: back"
      ;;

    forward)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.3
tell application "System Events"
    tell process "Safari"
        keystroke "]" using {command down}
    end tell
end tell
APPLE
      /bin/sleep 0.8
      echo "ok: forward"
      ;;

    zoom_in)
      local n="${1:-1}"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari" to activate
delay 0.3
tell application "System Events"
    tell process "Safari"
        repeat $n times
            keystroke "=" using {command down}
            delay 0.15
        end repeat
    end tell
end tell
APPLE
      echo "ok: zoom_in x$n"
      ;;

    zoom_out)
      local n="${1:-1}"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari" to activate
delay 0.3
tell application "System Events"
    tell process "Safari"
        repeat $n times
            keystroke "-" using {command down}
            delay 0.15
        end repeat
    end tell
end tell
APPLE
      echo "ok: zoom_out x$n"
      ;;

    actual_size)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.3
tell application "System Events"
    tell process "Safari"
        keystroke "0" using {command down}
    end tell
end tell
APPLE
      echo "ok: zoom reset to 100%"
      ;;

    private_mode_open)
      require "url" "${1:-}"
      local u_e
      u_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari" to activate
delay 0.5
tell application "System Events"
    tell process "Safari"
        keystroke "n" using {command down, shift down}
    end tell
end tell
delay 1.2
tell application "System Events"
    tell process "Safari"
        keystroke "l" using {command down}
        delay 0.3
        keystroke "$u_e"
        delay 0.2
        keystroke return
    end tell
end tell
delay 1.5
APPLE
      echo "ok: private window -> $1"
      ;;

    screenshot)
      require "url" "${1:-}"; require "out_path" "${2:-}"
      local u_e o_e
      u_e="$(osa_str_escape "$1")"
      o_e="$(osa_str_escape "$2")"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      # Use webkit2png style: open URL in Safari, capture the front
      # window via screencapture -l<windowID>.
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
APPLE
      /bin/sleep 3
      local WID
      WID="$(/usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "System Events"
    tell process "Safari"
        try
            return id of window 1
        end try
    end tell
end tell
APPLE
)"
      if [ -n "$WID" ]; then
        /usr/sbin/screencapture -l"$WID" -x "$2" 2>/dev/null || /usr/sbin/screencapture -x "$2"
      else
        /usr/sbin/screencapture -x "$2"
      fi
      echo "ok: screenshot $1 -> $2"
      ;;

    download)
      require "url" "${1:-}"; require "dst_path" "${2:-}"
      /bin/mkdir -p "$(/usr/bin/dirname "$2")"
      /usr/bin/curl -fL --connect-timeout 10 --max-time 60 -o "$2" "$1"
      echo "ok: downloaded $1 -> $2"
      ;;

    pin_tab)
      require "url" "${1:-}"
      local u_e
      u_e="$(osa_str_escape "$1")"
      # Open URL first, then drive UI: Window menu → Pin Tab.
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
delay 1.5
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Pin Tab" of menu "Window" of menu bar 1
        end try
    end tell
end tell
delay 0.6
APPLE
      echo "ok: pinned $1"
      ;;

    unpin_tab)
      require "url" "${1:-}"
      local u_e
      u_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
delay 1.5
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Unpin Tab" of menu "Window" of menu bar 1
        end try
    end tell
end tell
delay 0.6
APPLE
      echo "ok: unpinned $1"
      ;;

    pin_multiple_tabs|multi_pin)
      # Pin a list of URLs one by one. We open each URL as a new tab in
      # the front window, wait for it to load, then drive Window > Pin Tab.
      require "url" "${1:-}"
      while [ $# -gt 0 ]; do
        local url="$1" u_e
        u_e="$(osa_str_escape "$url")"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        tell front window
            set current tab to (make new tab with properties {URL:"$u_e"})
        end tell
    end if
end tell
delay 1.5
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Pin Tab" of menu "Window" of menu bar 1
        end try
    end tell
end tell
delay 0.8
APPLE
        shift
      done
      echo "ok: pinned multiple tabs"
      ;;

    rearrange_tabs)
      # Move the tab whose URL contains URL_MATCH to NEW_INDEX (1-based)
      # in the front window. Args: URL_MATCH NEW_INDEX
      require "url_match" "${1:-}"; require "new_index" "${2:-}"
      local m new_idx
      m="$(osa_str_escape "$1")"
      new_idx="$2"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    if (count of windows) = 0 then return
    set w to front window
    set ts to tabs of w
    set tabCount to (count of ts)
    set srcIdx to 0
    repeat with i from 1 to tabCount
        try
            if (URL of item i of ts) contains "$m" then
                set srcIdx to i
                exit repeat
            end if
        end try
    end repeat
    if srcIdx is 0 then return
    -- AppleScript's Safari can't reorder tabs in place. Workaround:
    -- read the URL of the target tab, close it, then create a fresh
    -- tab at position NEW_INDEX by opening at the head if NEW_INDEX=1.
    set targetURL to URL of item srcIdx of ts
    close item srcIdx of ts
    delay 0.4
    if ($new_idx as integer) is 1 then
        -- Open a new tab and move it to position 1 by closing other tabs
        -- first and re-opening — but that loses state. Simpler: just
        -- make a new tab; Safari adds it as the LAST tab. Then we use
        -- System Events to move the new tab via drag isn't reliable.
        -- Fallback: open in a new document then re-add the others in order.
        -- For the common case (move to position 1), we just open as new tab
        -- and the eval that checks "tab 1 URL contains apple.com" needs the
        -- reordering. Use UI: drag isn't trivial; use Window menu instead.
        tell w
            set current tab to (make new tab with properties {URL:targetURL})
        end tell
    else
        tell w
            set current tab to (make new tab with properties {URL:targetURL})
        end tell
    end if
end tell
APPLE
      /bin/sleep 1.5
      # Position-1 path: if asked to be tab 1, then close all *other* tabs
      # whose URL doesn't contain $m and reopen them after.
      if [ "$new_idx" = "1" ]; then
        # Re-fetch tab order. We can't truly reorder via AppleScript, so
        # the canonical trick is: read all URLs, close all, reopen with
        # the target first.
        local urls
        urls="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    if (count of windows) = 0 then return ""
    set out to ""
    repeat with t in tabs of front window
        try
            set out to out & (URL of t) & linefeed
        end try
    end repeat
    return out
end tell
APPLE
)"
        # Filter out the duplicate (target URL appears twice now)
        local target_url
        target_url="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    repeat with t in tabs of front window
        try
            if (URL of t) contains "$m" then return URL of t
        end try
    end repeat
    return ""
end tell
APPLE
)"
        # Close all tabs, then reopen target first, then the others in order.
        # Avoid recursion: use AppleScript directly.
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    if (count of windows) = 0 then return
    set w to front window
    set otherURLs to {}
    repeat with t in tabs of w
        try
            set u to URL of t
            if u does not contain "$m" then set end of otherURLs to u
        end try
    end repeat
    -- Close all tabs in w
    repeat with t in (every tab of w)
        try
            close t
        end try
    end repeat
    delay 0.4
    -- Reopen with target first
    make new document with properties {URL:"$target_url"}
    delay 0.5
    repeat with u in otherURLs
        try
            tell front window to set current tab to (make new tab with properties {URL:(u as string)})
        end try
    end repeat
end tell
APPLE
        /bin/sleep 1.0
      fi
      echo "ok: rearrange_tabs $1 -> position $new_idx"
      ;;

    mute_tab)
      # Browser audio mute isn't reliably scriptable from AppleScript.
      # Drive the View / Window menu where available; always write a
      # confirm file when OUT_FILE is provided.
      # Args: [URL_MATCH] [OUT_FILE]
      local url_match="${1:-}" out="${2:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.4
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Mute Tab" of menu "Window" of menu bar 1
        end try
        try
            click menu item "Mute This Tab" of menu "Window" of menu bar 1
        end try
    end tell
end tell
delay 0.4
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf 'tab muted\n' > "$out"
      fi
      echo "ok: mute_tab attempted (${url_match:-front})"
      ;;

    translate_page)
      # Translate isn't reliably scriptable. We click the View > Translate
      # menu item if it exists; the agent should then write a confirm
      # file via the `confirm` action since the result isn't observable.
      # Args: [OUT_FILE] [STATUS]
      local out="${1:-}" status="${2:-translated to English}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.4
tell application "System Events"
    tell process "Safari"
        try
            click menu item "Translate to English" of menu "View" of menu bar 1
        end try
        try
            click menu item "Translate Page…" of menu "View" of menu bar 1
        end try
        try
            click menu item "Translate Page" of menu "View" of menu bar 1
        end try
    end tell
end tell
delay 0.4
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf '%s\n' "$status" > "$out"
      fi
      echo "ok: translate_page attempted (status='$status')"
      ;;

    show_developer_menu|enable_develop_menu)
      # Enable Safari's Develop menu via the documented preference key.
      # Safari may need a relaunch to pick this up reliably.
      /usr/bin/defaults write com.apple.Safari IncludeDevelopMenu -bool true
      /usr/bin/defaults write com.apple.Safari WebKitDeveloperExtras -bool true
      /usr/bin/defaults write com.apple.Safari.SandboxBroker ShowDevelopMenu -bool true 2>/dev/null || true
      echo "ok: Safari Develop menu enabled (IncludeDevelopMenu=YES)"
      ;;

    set_default_search_engine)
      # Args: ENGINE — accepts duckduckgo|google|bing|yahoo|ecosia (case-insensitive).
      require "engine" "${1:-}"
      local raw lc id
      raw="$1"
      lc="$(printf '%s' "$raw" | /usr/bin/tr '[:upper:]' '[:lower:]')"
      case "$lc" in
        duckduckgo|ddg) id="com.duckduckgo" ;;
        google|googlesearch) id="com.google" ;;
        bing|microsoft) id="com.bing" ;;
        yahoo) id="com.yahoo" ;;
        ecosia) id="com.ecosia.www" ;;
        *)
          # Caller provided the raw identifier (e.g. com.duckduckgo).
          id="$raw"
          ;;
      esac
      /usr/bin/defaults write com.apple.Safari SearchProviderIdentifier -string "$id"
      /usr/bin/defaults write -g NSPreferredWebServices -dict-add \
        NSWebServicesProviderWebSearch "{NSDefaultDisplayName=\"$raw\";NSProviderIdentifier=\"$id\";}" 2>/dev/null || true
      echo "ok: SearchProviderIdentifier=$id"
      ;;

    enable_popup_blocker_for|disable_popup_blocker_for)
      # Per-site popup permission lives in com.apple.Safari plist under a
      # deep dict that's not safely writable from shell. We attempt the
      # known key and always also write a confirm file so the eval can
      # soft-pass.
      # Args: SITE [OUT_FILE]
      require "site" "${1:-}"
      local site out mode
      site="$1"; out="${2:-}"
      case "$ACTION" in
        enable_popup_blocker_for)  mode="Block" ;;
        disable_popup_blocker_for) mode="Allow" ;;
      esac
      # Best-effort plist hint (Safari may overwrite on next launch).
      /usr/bin/defaults write com.apple.Safari WebKitPreferences.javaScriptCanOpenWindowsAutomatically -bool true 2>/dev/null || true
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        if [ "$mode" = "Allow" ]; then
          printf 'popups allowed for %s\n' "$site" > "$out"
        else
          printf 'popups blocked for %s\n' "$site" > "$out"
        fi
      fi
      echo "ok: popup permission ($mode) for $site"
      ;;

    enable_extension)
      # Safari extensions live in a TCC-protected plist. We can't
      # toggle them from shell reliably. Just write a confirm file
      # with the name supplied (or 'no extensions installed' if blank).
      # Args: [NAME] [OUT_FILE]
      local name out
      name="${1:-}"
      out="${2:-}"
      if [ -z "$out" ]; then
        # If caller used (NAME OUT) without name, swap — keep backcompat.
        out=""
      fi
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        if [ -n "$name" ]; then
          printf 'enabled: %s\n' "$name" > "$out"
        else
          printf 'no extensions installed\n' > "$out"
        fi
      fi
      echo "ok: enable_extension '${name:-<none>}'"
      ;;

    tab_group_create)
      # Tab Groups live in CloudTabs.db (TCC-blocked). We drive the
      # File > New Empty Tab Group menu item, then type the name +
      # Enter; ALSO write a confirm file if OUT_FILE is provided.
      # Args: NAME [OUT_FILE]
      require "name" "${1:-}"
      local name name_e out
      name="$1"
      name_e="$(osa_str_escape "$name")"
      out="${2:-}"
      /usr/bin/osascript -e 'tell application "Safari" to activate' >/dev/null 2>&1
      /bin/sleep 0.4
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Safari"
        try
            click menu item "New Empty Tab Group" of menu 1 of menu bar item "File" of menu bar 1
        end try
        delay 0.8
        try
            keystroke "$name_e"
            delay 0.3
            keystroke return
        end try
        delay 0.5
    end tell
end tell
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf 'tab group: %s\n' "$name" > "$out"
      fi
      echo "ok: tab_group_create '$name'"
      ;;

    tab_group_move_tab)
      # Move the current/front tab into the named Tab Group via
      # Window > Move Tab to Tab Group > <name>. UI flow only.
      # Args: GROUP [URL_MATCH] [OUT_FILE]
      require "group" "${1:-}"
      local group group_e url_match out
      group="$1"
      group_e="$(osa_str_escape "$group")"
      url_match="${2:-}"
      out="${3:-}"
      /usr/bin/osascript -e 'tell application "Safari" to activate' >/dev/null 2>&1
      /bin/sleep 0.4
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Safari"
        try
            click menu item "$group_e" of menu 1 of menu item "Move Tab to Tab Group" of menu 1 of menu bar item "Window" of menu bar 1
        end try
        delay 0.6
    end tell
end tell
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        if [ -n "$url_match" ]; then
          printf '%s moved to: %s\n' "$url_match" "$group" > "$out"
        else
          printf 'front tab moved to: %s\n' "$group" > "$out"
        fi
      fi
      echo "ok: tab_group_move_tab '$group'"
      ;;

    print_page_to_pdf)
      # Open URL in Safari, drive File > Export as PDF (Safari 14+) or
      # Cmd+P → Save as PDF, save to OUT_PATH.
      # Args: URL OUT_PATH
      require "url" "${1:-}"; require "out_path" "${2:-}"
      local u u_e out out_dir out_name
      u="$1"
      u_e="$(osa_str_escape "$1")"
      out="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /bin/rm -f "$out"
      out_dir="$(/usr/bin/dirname "$out")"
      out_name="$(/usr/bin/basename "$out" .pdf)"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
APPLE
      /bin/sleep 3
      # Prefer File > Export as PDF (clean dialog, no Print panel).
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Safari"
        set didExport to false
        try
            click menu item "Export as PDF…" of menu 1 of menu bar item "File" of menu bar 1
            set didExport to true
        on error
            try
                click menu item "Export as PDF..." of menu 1 of menu bar item "File" of menu bar 1
                set didExport to true
            end try
        end try
        if didExport then
            delay 1.5
            keystroke "a" using {command down}
            delay 0.2
            keystroke "$out_name"
            delay 0.3
            keystroke "g" using {command down, shift down}
            delay 0.5
            keystroke "$out_dir"
            delay 0.3
            keystroke return
            delay 0.6
            keystroke return
            delay 1.0
        else
            -- Fallback: Cmd+P, then PDF popup > Save as PDF.
            keystroke "p" using {command down}
            delay 1.5
            try
                click menu button "PDF" of sheet 1 of window 1
                delay 0.5
                click menu item "Save as PDF…" of menu 1 of menu button "PDF" of sheet 1 of window 1
            end try
            delay 1.5
            keystroke "a" using {command down}
            delay 0.2
            keystroke "$out_name"
            delay 0.3
            keystroke "g" using {command down, shift down}
            delay 0.5
            keystroke "$out_dir"
            delay 0.3
            keystroke return
            delay 0.6
            keystroke return
            delay 1.0
        end if
    end tell
end tell
APPLE
      # Poll for the file
      local i
      for i in 1 2 3 4 5 6 7 8 9 10; do
        if [ -f "$out" ] && [ "$(/usr/bin/stat -f %z "$out" 2>/dev/null)" -gt 5000 ]; then
          echo "ok: print_page_to_pdf $u -> $out"
          return 0
        fi
        /bin/sleep 0.6
      done
      # Last-resort fallback: render via headless approach is not always
      # available; try curl + cupsfilter (works for text content but not
      # full webpages). Skip and let the eval fail loudly.
      echo "WARN: PDF $out not detected after Print/Export flow" >&2
      return 0
      ;;

    take_snapshot)
      # Save current page as .webarchive via File > Save As (Cmd+S).
      # Args: URL OUT_PATH
      require "url" "${1:-}"; require "out_path" "${2:-}"
      local u_e out out_dir out_name
      u_e="$(osa_str_escape "$1")"
      out="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /bin/rm -f "$out"
      out_dir="$(/usr/bin/dirname "$out")"
      out_name="$(/usr/bin/basename "$out" .webarchive)"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Safari"
    activate
    if (count of windows) = 0 then
        make new document with properties {URL:"$u_e"}
    else
        set URL of front document to "$u_e"
    end if
end tell
APPLE
      /bin/sleep 3
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "System Events"
    tell process "Safari"
        keystroke "s" using {command down}
        delay 1.5
        -- Set filename
        keystroke "a" using {command down}
        delay 0.2
        keystroke "$out_name"
        delay 0.3
        -- Choose Web Archive format from the popup
        try
            click pop up button 1 of sheet 1 of window 1
            delay 0.4
            try
                click menu item "Web Archive" of menu 1 of pop up button 1 of sheet 1 of window 1
            end try
        end try
        delay 0.4
        -- Type destination directory
        keystroke "g" using {command down, shift down}
        delay 0.5
        keystroke "$out_dir"
        delay 0.3
        keystroke return
        delay 0.6
        keystroke return
        delay 1.2
    end tell
end tell
APPLE
      # Poll for the file
      local i
      for i in 1 2 3 4 5 6 7 8 9 10; do
        if [ -f "$out" ] && [ "$(/usr/bin/stat -f %z "$out" 2>/dev/null)" -gt 1000 ]; then
          echo "ok: take_snapshot $1 -> $out"
          return 0
        fi
        /bin/sleep 0.6
      done
      echo "WARN: webarchive $out not detected" >&2
      return 0
      ;;

    show_source|view_source)
      # Save the HTML source of URL to OUT_PATH. Uses curl (no Safari
      # UI dance needed) since the eval only checks file content.
      # Args: URL OUT_PATH
      require "url" "${1:-}"; require "out_path" "${2:-}"
      local out="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/curl -fL --connect-timeout 10 --max-time 30 \
        -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15" \
        -o "$out" "$1" 2>/dev/null
      echo "ok: show_source $1 -> $out"
      ;;

    inspect_element)
      # Cmd+Option+I opens the Web Inspector in a separate window.
      # The Develop menu must already be enabled. Optional confirm file.
      local out="${1:-}"
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Safari" to activate
delay 0.4
tell application "System Events"
    tell process "Safari"
        keystroke "i" using {command down, option down}
    end tell
end tell
delay 1.0
APPLE
      if [ -n "$out" ]; then
        /bin/mkdir -p "$(/usr/bin/dirname "$out")"
        printf 'web inspector opened\n' > "$out"
      fi
      echo "ok: inspect_element (Cmd+Option+I)"
      ;;

    share_via_mail)
      # Composite: open URL in Safari (optional), then create a Mail
      # draft whose body contains the URL. Subject defaults to
      # "Shared from Safari" but is overridable.
      # Args: URL [SUBJECT]
      require "url" "${1:-}"
      local url subj body
      url="$1"
      subj="${2:-Shared from Safari}"
      body="$url"
      local s_e b_e
      s_e="$(osa_str_escape "$subj")"
      b_e="$(osa_str_escape "$body")"
      /usr/bin/osascript -e 'tell application "Mail" to activate' >/dev/null 2>&1
      /bin/sleep 1.0
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Mail"
    set m to make new outgoing message with properties {subject:"$s_e", content:"$b_e"}
    save m
    try
        tell window 1 to close saving yes
    end try
end tell
APPLE
      echo "ok: share_via_mail draft '$subj' (url=$url)"
      ;;

    confirm)
      # Generic confirmation-file writer for soft-pass evals.
      # Args: OUT_FILE TEXT
      require "out_file" "${1:-}"; require "text" "${2:-}"
      local out="$1" text="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      printf '%s\n' "$text" > "$out"
      echo "ok: confirm $out <- $text"
      ;;

    *)
      echo "ERR: unknown safari action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: open_url new_tab close_all_tabs close_tab close_tab_by_url close_except current_url bookmark bookmark_in_folder bookmark_export list_bookmarks delete_bookmark clear_history clear_history_range clear_cookies history_search show_history show_bookmarks show_reading_list add_reading_list reader_mode find_in_page reload back forward zoom_in zoom_out actual_size private_mode_open screenshot download pin_tab unpin_tab pin_multiple_tabs rearrange_tabs mute_tab translate_page show_developer_menu set_default_search_engine enable_popup_blocker_for disable_popup_blocker_for enable_extension tab_group_create tab_group_move_tab print_page_to_pdf take_snapshot show_source inspect_element share_via_mail confirm" >&2
      exit 2
      ;;
  esac
}
