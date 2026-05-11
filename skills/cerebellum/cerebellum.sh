#!/bin/bash
# kinclaw cerebellum — fast macOS operation dispatcher (v2, modular)
#
# Categories live in skills/cerebellum/categories/*.sh — each one
# defines its own <cat>_dispatch() function. Main file just routes.
set -uo pipefail

CEREB_DIR="$(/usr/bin/dirname "${BASH_SOURCE[0]}")"

# Load shared helpers + every category dispatcher
. "$CEREB_DIR/categories/_helpers.sh"
for f in "$CEREB_DIR/categories"/*.sh; do
  case "$(/usr/bin/basename "$f")" in
    _helpers.sh) continue ;;  # already sourced
    *) . "$f" ;;
  esac
done

print_help() {
  /bin/cat <<'EOF'
USAGE: cerebellum "<category> <action> [args...]"

CATEGORIES (run "cerebellum '<cat>'" with no action for that category's full list):

  finder    file ops, sort/view settings, tags, search
  notes     create / edit / format / export Notes app entries
  mail      Mail draft creation (no send) + bulk operations
  calendar  events: create / delete / list / search
  reminders create / complete / delete / search reminders + lists
  settings  open System Settings panes, dark mode, volume, brightness
  safari    open url, new tab, bookmark, history, find, fill
  music     play, pause, next, prev, playlist ops
  photos    import, search, album ops
  maps      search location, route, share
  terminal  run command, run script
  multi     cross-app composites

For per-category action lists run e.g.:
  cerebellum "calendar"
  cerebellum "settings"

QUICK REFERENCE — most-used actions:

  cerebellum "finder rename /a /b"
  cerebellum "finder set_view list"
  cerebellum "notes create 'Title' 'body'"
  cerebellum "notes export_pdf 'Note' /path.pdf"
  cerebellum "mail draft 'subject' 'body' /path/attach.pdf"
  cerebellum "calendar create_event 'Home' 'Meeting' '2026-05-12 14:00' '2026-05-12 15:00'"
  cerebellum "reminders create 'Reminders' 'Buy milk' '2026-05-15 10:00'"
  cerebellum "settings open dark-mode"
  cerebellum "safari open_url https://localkin.dev"
  cerebellum "music play"
  cerebellum "terminal run 'echo hello'"
EOF
}

CMD="${1:-}"
[ -z "$CMD" ] && { print_help; exit 0; }

# eval set -- respects shell quoting on values with spaces
eval set -- "$CMD"
CAT="${1:-}"; ACTION="${2:-}"
shift 2 2>/dev/null || true

case "$CAT" in
  ""|help|"-h"|"--help") print_help ;;
  *)
    # Dynamic dispatch: look for a function named <cat>_dispatch
    if declare -F "${CAT}_dispatch" >/dev/null 2>&1; then
      "${CAT}_dispatch" "$ACTION" "$@"
    else
      echo "ERR: category '$CAT' not implemented yet (no ${CAT}_dispatch in skills/cerebellum/categories/)" >&2
      exit 1
    fi
    ;;
esac
