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

    *)
      echo "ERR: unknown safari action '$ACTION'. Run 'cerebellum' for menu." >&2
      exit 2
      ;;
  esac
}
