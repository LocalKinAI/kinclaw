terminal_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    run)
      require "cmd" "${1:-}"
      # Whole remainder is the command — let /bin/bash -c eval it so the
      # caller's quoting controls splitting. We pass via -c so shell
      # builtins (cd, echo, redirects) work naturally.
      /bin/bash -c "$*"
      ;;

    run_to_file)
      require "cmd" "${1:-}"; require "out_file" "${2:-}"
      local cmd="$1" out="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /bin/bash -c "$cmd" > "$out"
      echo "ok: '$cmd' -> $out"
      ;;

    open_in_terminal_app)
      local cwd="${1:-$HOME}"
      /usr/bin/open -a Terminal "$cwd"
      /bin/sleep 0.8
      echo "ok: Terminal opened in $cwd"
      ;;

    open_new_window|new_window)
      # Open a fresh Terminal window, optionally cd'd into CWD.
      local cwd="${1:-}"
      if [ -n "$cwd" ]; then
        local cwd_e
        cwd_e="$(osa_str_escape "$cwd")"
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Terminal"
    activate
    do script "cd \"$cwd_e\""
end tell
APPLE
      else
        /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Terminal"
    activate
    do script ""
end tell
APPLE
      fi
      /bin/sleep 0.6
      echo "ok: new Terminal window${cwd:+ in $cwd}"
      ;;

    open_new_tab|new_tab)
      # Open a new tab in the front Terminal window.
      local cwd="${1:-}"
      if [ -n "$cwd" ]; then
        local cwd_e
        cwd_e="$(osa_str_escape "$cwd")"
        # Cmd+T to open a new tab, then cd
        /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Terminal"
    activate
end tell
tell application "System Events"
    tell process "Terminal"
        keystroke "t" using {command down}
    end tell
end tell
delay 0.5
tell application "Terminal"
    do script "cd \"$cwd_e\"" in front window
end tell
APPLE
      else
        /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Terminal"
    activate
end tell
tell application "System Events"
    tell process "Terminal"
        keystroke "t" using {command down}
    end tell
end tell
APPLE
      fi
      /bin/sleep 0.6
      echo "ok: new Terminal tab${cwd:+ in $cwd}"
      ;;

    set_title|rename_tab)
      require "title" "${1:-}"
      local t_e
      t_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Terminal"
    activate
    try
        set custom title of selected tab of front window to "$t_e"
    end try
end tell
APPLE
      /bin/sleep 0.4
      echo "ok: title='$1'"
      ;;

    set_tab_color)
      require "color" "${1:-}"
      # Set background color via Terminal's color profile. Colors mapped
      # to AppleScript RGB triples (0-65535 range). best-effort — some
      # Terminal versions ignore this when a profile is active.
      local r g b
      case "$1" in
        red)     r=58000; g=10000; b=10000 ;;
        orange)  r=58000; g=30000; b=10000 ;;
        yellow)  r=58000; g=58000; b=10000 ;;
        green)   r=10000; g=45000; b=10000 ;;
        blue)    r=10000; g=20000; b=58000 ;;
        purple)  r=38000; g=10000; b=50000 ;;
        gray|grey) r=25000; g=25000; b=25000 ;;
        *) echo "ERR: unknown color '$1'" >&2; exit 2 ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Terminal"
    activate
    try
        set background color of selected tab of front window to {$r, $g, $b}
    end try
end tell
APPLE
      echo "ok: tab color=$1 (soft-pass — Terminal may ignore if profile is active)"
      ;;

    edit_profile|set_profile)
      # Switch the front Terminal window to a named profile (settings set).
      # Built-ins: Basic, Pro, Homebrew, Grass, Man Page, Novel, Ocean, Red Sands, Silver Aerogel.
      require "profile" "${1:-}"
      local prof_e
      prof_e="$(osa_str_escape "$1")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Terminal"
    activate
    try
        set current settings of front window to settings set "$prof_e"
    end try
end tell
APPLE
      /bin/sleep 0.4
      echo "ok: profile='$1' (soft-pass — UI only when profile missing)"
      ;;

    get_history)
      # Read the user's shell history. Args: OUT_FILE [COUNT=50]
      # Note: original signature was (out_file, count). To accept both
      # (count, out_file) and (out_file, count), sniff which arg is numeric.
      require "arg1" "${1:-}"
      local a1="$1" a2="${2:-}" out count
      if [ -n "$a2" ]; then
        # Two-arg form: figure out which is which
        if [ "$a1" -eq "$a1" ] 2>/dev/null; then
          count="$a1"; out="$a2"
        else
          out="$a1"; count="$a2"
        fi
      else
        out="$a1"; count="50"
      fi
      local src=""
      if [ -f "$HOME/.zsh_history" ]; then
        src="$HOME/.zsh_history"
      elif [ -f "$HOME/.bash_history" ]; then
        src="$HOME/.bash_history"
      else
        echo "ERR: no history file found" >&2
        exit 1
      fi
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      # zsh history has timestamp prefix ": 1234567890:0;cmd" — strip it
      /usr/bin/tail -n "$count" "$src" | /usr/bin/sed 's/^: [0-9]*:[0-9]*;//' > "$out"
      echo "ok: last $count history entries -> $out"
      ;;

    split_pane)
      # Cmd-D splits the current Terminal window. UI-only flow.
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Terminal"
    activate
end tell
tell application "System Events"
    tell process "Terminal"
        keystroke "d" using {command down}
    end tell
end tell
APPLE
      /bin/sleep 0.6
      echo "ok: split pane (Cmd+D)"
      ;;

    clear_screen|clear)
      # Send Cmd+K to the front Terminal window (UI keystroke).
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Terminal"
    activate
end tell
tell application "System Events"
    tell process "Terminal"
        keystroke "k" using {command down}
    end tell
end tell
APPLE
      /bin/sleep 0.3
      echo "ok: cleared screen"
      ;;

    git_init)
      # Composite: mkdir PATH + git init inside it. Optional [user.name] [user.email]
      # written to local config. Useful for the 043-terminal-git-init task.
      require "path" "${1:-}"
      local path="$1" gn="${2:-}" ge="${3:-}"
      /bin/mkdir -p "$path"
      # Avoid `cd` so our shell state stays clean; use -C to scope git.
      /usr/bin/env git -C "$path" init >/dev/null 2>&1 || {
        echo "ERR: git init failed in $path" >&2
        exit 1
      }
      if [ -n "$gn" ]; then
        /usr/bin/env git -C "$path" config user.name "$gn" >/dev/null 2>&1 || true
      fi
      if [ -n "$ge" ]; then
        /usr/bin/env git -C "$path" config user.email "$ge" >/dev/null 2>&1 || true
      fi
      echo "ok: git_init $path"
      ;;

    *)
      echo "ERR: unknown terminal action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: run run_to_file open_in_terminal_app open_new_window open_new_tab set_title set_tab_color edit_profile get_history split_pane clear_screen git_init" >&2
      exit 2
      ;;
  esac
}
