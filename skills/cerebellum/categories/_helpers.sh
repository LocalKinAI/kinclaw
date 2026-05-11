# Shared helpers used by all category dispatchers.

require() {
  local n="$1"; local got="$2"
  if [ -z "$got" ]; then
    echo "ERR: missing argument: $n" >&2
    exit 2
  fi
}

osa_str_escape() {
  printf '%s' "$1" | /usr/bin/sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

notes_focus_body() {
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
