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

    new_window)
      /usr/bin/osascript <<'APPLE' 2>/dev/null
tell application "Terminal"
    activate
    do script ""
end tell
APPLE
      /bin/sleep 0.6
      echo "ok: new Terminal window"
      ;;

    new_tab)
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
      /bin/sleep 0.6
      echo "ok: new Terminal tab"
      ;;

    set_title)
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
      # to AppleScript RGB triples (0-65535 range). best-effort.
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
      echo "ok: tab color=$1"
      ;;

    get_history)
      require "out_file" "${1:-}"
      local out="$1" count="${2:-50}"
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
      # Cmd-D splits the current Terminal window
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

    clear_screen)
      # Send Cmd+K to the front Terminal window
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

    *)
      echo "ERR: unknown terminal action '$ACTION'. Run 'cerebellum' for menu." >&2
      echo "Actions: run run_to_file open_in_terminal_app new_window new_tab set_title set_tab_color get_history split_pane clear_screen" >&2
      exit 2
      ;;
  esac
}
