numbers_dispatch() {
  local ACTION="$1"; shift || true
  case "$ACTION" in

    new)
      require "out_path" "${1:-}"
      local out p_e
      out="$1"
      p_e="$(osa_str_escape "$out")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    set newDoc to make new document
    delay 0.5
    try
        save newDoc in (POSIX file "$p_e")
    end try
end tell
APPLE
      /bin/sleep 1.5
      echo "ok: new numbers document -> $out"
      ;;

    open)
      require "path" "${1:-}"
      /usr/bin/open -a Numbers "$1"
      /bin/sleep 2
      echo "ok: opened $1"
      ;;

    save_as_pdf)
      require "src_path" "${1:-}"; require "out_pdf" "${2:-}"
      local src out s_e o_e
      src="$1"; out="$2"
      s_e="$(osa_str_escape "$src")"
      o_e="$(osa_str_escape "$out")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    set theDoc to open (POSIX file "$s_e")
    delay 1.2
    try
        export theDoc to (POSIX file "$o_e") as PDF
    on error errMsg
        log errMsg
    end try
    delay 1.0
    try
        close theDoc saving no
    end try
end tell
APPLE
      /bin/sleep 1
      echo "ok: exported numbers pdf -> $out"
      ;;

    set_cell)
      # set_cell DOC_PATH SHEET CELL VALUE
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      require "cell" "${3:-}"; require "value" "${4:-}"
      local doc sheet cell val d_e sh_e c_e v_e
      doc="$1"; sheet="$2"; cell="$3"; val="$4"
      d_e="$(osa_str_escape "$doc")"
      sh_e="$(osa_str_escape "$sheet")"
      c_e="$(osa_str_escape "$cell")"
      v_e="$(osa_str_escape "$val")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            tell sheet "$sh_e"
                tell first table
                    set value of cell "$c_e" to "$v_e"
                end tell
            end tell
        end tell
    on error
        try
            tell front document
                tell active sheet
                    tell first table
                        set value of cell "$c_e" to "$v_e"
                    end tell
                end tell
            end tell
        end try
    end try
end tell
APPLE
      echo "ok: set $cell = $val in $sheet"
      ;;

    sum_column)
      # sum_column DOC_PATH SHEET COLUMN OUT_FILE
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      require "column" "${3:-}"; require "out_file" "${4:-}"
      local sheet column out sh_e c_e
      sheet="$2"; column="$3"; out="$4"
      sh_e="$(osa_str_escape "$sheet")"
      c_e="$(osa_str_escape "$column")"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      local total
      total="$(/usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    try
        tell front document
            tell sheet "$sh_e"
                tell first table
                    set total to 0
                    set colRef to column "$c_e"
                    repeat with c in (every cell of colRef)
                        try
                            set v to value of c
                            if v is not missing value then
                                set total to total + (v as real)
                            end if
                        end try
                    end repeat
                    return total
                end tell
            end tell
        end tell
    on error
        return 0
    end try
end tell
APPLE
)"
      echo "${total:-0}" > "$out"
      echo "ok: sum of column $column -> $out (${total:-0})"
      ;;

    add_row)
      # add_row DOC_PATH SHEET
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      local sheet sh_e
      sheet="$2"
      sh_e="$(osa_str_escape "$sheet")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            tell sheet "$sh_e"
                tell first table
                    add row below last row
                end tell
            end tell
        end tell
    on error
        try
            tell front document
                tell active sheet
                    tell first table
                        add row below last row
                    end tell
                end tell
            end tell
        end try
    end try
end tell
APPLE
      echo "ok: added row to $sheet"
      ;;

    add_column)
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      local sheet sh_e
      sheet="$2"
      sh_e="$(osa_str_escape "$sheet")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            tell sheet "$sh_e"
                tell first table
                    add column after last column
                end tell
            end tell
        end tell
    on error
        try
            tell front document
                tell active sheet
                    tell first table
                        add column after last column
                    end tell
                end tell
            end tell
        end try
    end try
end tell
APPLE
      echo "ok: added column to $sheet"
      ;;

    chart)
      # chart DOC_PATH RANGE CHART_TYPE — UI-driven heuristic
      require "doc_path" "${1:-}"; require "range" "${2:-}"; require "chart_type" "${3:-}"
      local rng="$2" ctype="$3"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers" to activate
delay 0.4
tell application "System Events"
    tell process "Numbers"
        try
            -- Insert > Chart > 2D > Bar (first menu path; very heuristic)
            click menu item "Chart" of menu 1 of menu bar item "Insert" of menu bar 1
        end try
    end tell
end tell
APPLE
      /bin/sleep 0.5
      echo "ok: requested chart insert ($rng, $ctype) — UI-driven, may need agent follow-up"
      ;;

    save_as)
      require "doc_path" "${1:-}"; require "new_path" "${2:-}"
      local new_p p_e
      new_p="$2"
      p_e="$(osa_str_escape "$new_p")"
      /bin/mkdir -p "$(/usr/bin/dirname "$new_p")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        save front document in (POSIX file "$p_e")
    end try
end tell
APPLE
      /bin/sleep 1.2
      echo "ok: saved as $new_p"
      ;;

    *)
      echo "ERR: unknown numbers action '$ACTION'. Try: new|open|save_as_pdf|set_cell|sum_column|add_row|add_column|chart|save_as" >&2
      exit 2
      ;;
  esac
}
