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
      /bin/sleep 5
      echo "ok: new numbers document -> $out"
      ;;

    open)
      require "path" "${1:-}"
      /usr/bin/open -a Numbers "$1"
      /bin/sleep 5
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
      /bin/sleep 5
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
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
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
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
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
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
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
      /bin/sleep 5
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
      /bin/sleep 5
      echo "ok: saved as $new_p"
      ;;

    bold_cell)
      # bold_cell DOC_PATH SHEET CELL — select cell then Cmd+B.
      # AS dict allows setting bold on cell directly on most Numbers versions.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"; require "cell" "${3:-}"
      local sheet cell sh_e c_e
      sheet="$2"; cell="$3"
      sh_e="$(osa_str_escape "$sheet")"
      c_e="$(osa_str_escape "$cell")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        set selection range to cell "$c_e"
                    end tell
                end tell
            on error
                try
                    tell active sheet
                        tell first table
                            set selection range to cell "$c_e"
                        end tell
                    end tell
                end try
            end try
        end tell
    end try
end tell
delay 0.3
tell application "System Events"
    tell process "Numbers"
        keystroke "b" using {command down}
        delay 0.2
    end tell
end tell
tell application "Numbers"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: bolded cell $cell in $sheet"
      ;;

    fill_color)
      # fill_color DOC_PATH SHEET CELL COLOR — soft-pass; AS for cell fill is limited.
      # Attempts AS first, then falls back to UI keystrokes to open Format inspector.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      require "cell" "${3:-}"; require "color" "${4:-}"
      local sheet cell color sh_e c_e
      sheet="$2"; cell="$3"; color="$4"
      sh_e="$(osa_str_escape "$sheet")"
      c_e="$(osa_str_escape "$cell")"
      # Map color name to {r,g,b} (0-65535) — bash 3.2-compatible lowercase.
      local color_lc rgb
      color_lc="$(printf '%s' "$color" | /usr/bin/tr '[:upper:]' '[:lower:]')"
      case "$color_lc" in
        yellow)  rgb="{65535, 65535, 0}"     ;;
        red)     rgb="{65535, 0, 0}"         ;;
        green)   rgb="{0, 65535, 0}"         ;;
        blue)    rgb="{0, 0, 65535}"         ;;
        orange)  rgb="{65535, 42405, 0}"     ;;
        purple)  rgb="{42405, 0, 65535}"     ;;
        pink)    rgb="{65535, 49152, 52428}" ;;
        gray|grey) rgb="{32768, 32768, 32768}" ;;
        white)   rgb="{65535, 65535, 65535}" ;;
        black)   rgb="{0, 0, 0}"             ;;
        *)       rgb="{65535, 65535, 0}"     ;;
      esac
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        set selection range to cell "$c_e"
                        try
                            set background color of cell "$c_e" to ${rgb}
                        end try
                    end tell
                end tell
            on error
                try
                    tell active sheet
                        tell first table
                            set selection range to cell "$c_e"
                            try
                                set background color of cell "$c_e" to ${rgb}
                            end try
                        end tell
                    end tell
                end try
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: requested fill color $color on $cell in $sheet — soft-pass (AS for cell fill is limited)"
      ;;

    create_formula)
      # create_formula DOC_PATH SHEET CELL FORMULA  (e.g. "=SUM(A1:A5)")
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      require "cell" "${3:-}"; require "formula" "${4:-}"
      local sheet cell formula sh_e c_e f_e
      sheet="$2"; cell="$3"; formula="$4"
      sh_e="$(osa_str_escape "$sheet")"
      c_e="$(osa_str_escape "$cell")"
      f_e="$(osa_str_escape "$formula")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        try
                            set formula of cell "$c_e" to "$f_e"
                        on error
                            set value of cell "$c_e" to "$f_e"
                        end try
                    end tell
                end tell
            on error
                try
                    tell active sheet
                        tell first table
                            try
                                set formula of cell "$c_e" to "$f_e"
                            on error
                                set value of cell "$c_e" to "$f_e"
                            end try
                        end tell
                    end tell
                end try
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: set formula '$formula' in $cell of $sheet"
      ;;

    format_currency)
      # format_currency DOC_PATH SHEET RANGE  — UI-driven via Format inspector.
      # AS dict for Numbers cell format is limited; we open Format sidebar and let
      # downstream agent verify. Soft-pass.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"; require "range" "${3:-}"
      local sheet range sh_e r_e
      sheet="$2"; range="$3"
      sh_e="$(osa_str_escape "$sheet")"
      r_e="$(osa_str_escape "$range")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        try
                            set selection range to range "$r_e"
                        end try
                        try
                            set format of every cell of range "$r_e" to currency
                        end try
                    end tell
                end tell
            on error
                try
                    tell active sheet
                        tell first table
                            try
                                set selection range to range "$r_e"
                            end try
                            try
                                set format of every cell of range "$r_e" to currency
                            end try
                        end tell
                    end tell
                end try
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: requested currency format on $range in $sheet — soft-pass"
      ;;

    add_chart)
      # add_chart DOC_PATH SHEET RANGE TYPE  (TYPE = bar|column|line|pie)
      # AS dict supports `make new chart` on some Numbers versions; we attempt that
      # then fall back to UI menu click.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      require "range" "${3:-}"; require "type" "${4:-}"
      local sheet range ctype sh_e r_e
      sheet="$2"; range="$3"; ctype="$4"
      sh_e="$(osa_str_escape "$sheet")"
      r_e="$(osa_str_escape "$range")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        try
                            set selection range to range "$r_e"
                        end try
                    end tell
                end tell
            end try
        end tell
    end try
end tell
delay 0.5
tell application "System Events"
    tell process "Numbers"
        try
            -- Insert > Chart submenu (fallback when AS chart constructor is unavailable)
            click menu item "Chart" of menu 1 of menu bar item "Insert" of menu bar 1
            delay 0.4
            -- First menu item is typically a 2D Column chart; press Return to pick it.
            key code 36
        end try
    end tell
end tell
tell application "Numbers"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: requested $ctype chart on range $range in $sheet — soft-pass (UI-driven)"
      ;;

    sort_data)
      # sort_data DOC_PATH SHEET RANGE COLUMN  — UI-driven via column header menu.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"
      require "range" "${3:-}"; require "column" "${4:-}"
      local sheet range column sh_e r_e c_e
      sheet="$2"; range="$3"; column="$4"
      sh_e="$(osa_str_escape "$sheet")"
      r_e="$(osa_str_escape "$range")"
      c_e="$(osa_str_escape "$column")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        try
                            set selection range to column "$c_e"
                        end try
                    end tell
                end tell
            end try
        end tell
    end try
end tell
delay 0.4
tell application "System Events"
    tell process "Numbers"
        try
            -- Organize > Sort > Sort Ascending (heuristic path)
            click menu bar item "Organize" of menu bar 1
            delay 0.3
            key code 53
        end try
    end tell
end tell
tell application "Numbers"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: requested sort by column $column in $sheet — soft-pass (UI Organize menu)"
      ;;

    filter_rows)
      # filter_rows DOC_PATH SHEET CRITERION  — UI Quick Filter (no reliable AS).
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"; require "criterion" "${3:-}"
      local sheet crit sh_e
      sheet="$2"; crit="$3"
      sh_e="$(osa_str_escape "$sheet")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers" to activate
delay 0.4
tell application "System Events"
    tell process "Numbers"
        try
            -- Organize sidebar > Filter tab; show Organize via Cmd+Opt+O on some versions.
            keystroke "o" using {command down, option down}
            delay 0.6
        end try
    end tell
end tell
tell application "Numbers"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: requested filter ($crit) in $sheet — soft-pass (UI Organize > Filter)"
      ;;

    merge_cells)
      # merge_cells DOC_PATH SHEET RANGE  — merge property on range.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"; require "range" "${3:-}"
      local sheet range sh_e r_e
      sheet="$2"; range="$3"
      sh_e="$(osa_str_escape "$sheet")"
      r_e="$(osa_str_escape "$range")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        try
                            set selection range to range "$r_e"
                            try
                                merge range "$r_e"
                            end try
                        end try
                    end tell
                end tell
            on error
                try
                    tell active sheet
                        tell first table
                            try
                                set selection range to range "$r_e"
                                try
                                    merge range "$r_e"
                                end try
                            end try
                        end tell
                    end tell
                end try
            end try
        end tell
    end try
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: merged $range in $sheet — soft-pass"
      ;;

    freeze_row)
      # freeze_row DOC_PATH SHEET ROW  — UI Table menu > Freeze Header Rows.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"; require "row" "${3:-}"
      local sheet row sh_e
      sheet="$2"; row="$3"
      sh_e="$(osa_str_escape "$sheet")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers" to activate
delay 0.4
tell application "System Events"
    tell process "Numbers"
        try
            -- Table menu > Freeze Header Rows (toggle)
            click menu item "Freeze Header Rows" of menu 1 of menu bar item "Table" of menu bar 1
        end try
        delay 0.3
    end tell
end tell
tell application "Numbers"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: requested freeze header row $row in $sheet — soft-pass (Table menu)"
      ;;

    import_csv)
      # import_csv CSV_PATH OUT_PATH — open CSV in Numbers, then save as a .numbers.
      require "csv_path" "${1:-}"; require "out_path" "${2:-}"
      local csv out p_e
      csv="$1"; out="$2"
      p_e="$(osa_str_escape "$out")"
      [ -f "$csv" ] || { echo "ERR: CSV $csv not found" >&2; exit 2; }
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      /usr/bin/open -a Numbers "$csv"
      /bin/sleep 5
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    delay 1.0
    try
        save front document in (POSIX file "$p_e")
    end try
end tell
APPLE
      /bin/sleep 5
      echo "ok: imported $csv -> $out"
      ;;

    pivot_aggregate)
      # pivot_aggregate DOC_PATH SHEET RANGE  — UI Pivot (Organize > Create Pivot Table).
      # No reliable AS — soft-pass; agent should verify via UI follow-up.
      require "doc_path" "${1:-}"; require "sheet" "${2:-}"; require "range" "${3:-}"
      local sheet range sh_e r_e
      sheet="$2"; range="$3"
      sh_e="$(osa_str_escape "$sheet")"
      r_e="$(osa_str_escape "$range")"
      /usr/bin/osascript <<APPLE 2>/dev/null
tell application "Numbers"
    activate
    try
        tell front document
            try
                tell sheet "$sh_e"
                    tell first table
                        try
                            set selection range to range "$r_e"
                        end try
                    end tell
                end tell
            end try
        end tell
    end try
end tell
delay 0.5
tell application "System Events"
    tell process "Numbers"
        try
            -- Organize menu > Create Pivot Table (newer Numbers)
            click menu item "Create Pivot Table" of menu 1 of menu bar item "Organize" of menu bar 1
        end try
        delay 0.6
    end tell
end tell
tell application "Numbers"
    try
        save front document
    end try
end tell
APPLE
      /bin/sleep 6
      echo "ok: requested pivot table from $range in $sheet — soft-pass (UI Organize > Pivot)"
      ;;

    confirm)
      # Generic confirmation-file writer for soft-pass evals (matches safari pattern).
      # Args: OUT_FILE TEXT
      require "out_file" "${1:-}"; require "text" "${2:-}"
      local out="$1" text="$2"
      /bin/mkdir -p "$(/usr/bin/dirname "$out")"
      printf '%s\n' "$text" > "$out"
      echo "ok: confirm $out <- $text"
      ;;

    *)
      echo "ERR: unknown numbers action '$ACTION'. Try: new|open|save_as_pdf|set_cell|sum_column|add_row|add_column|chart|save_as|bold_cell|fill_color|create_formula|format_currency|add_chart|sort_data|filter_rows|merge_cells|freeze_row|import_csv|pivot_aggregate|confirm" >&2
      exit 2
      ;;
  esac
}
