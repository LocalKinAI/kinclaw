#!/bin/bash
# daily-traffic.sh — one-line morning check on the KinClaw universe.
#
# Shows views / unique visitors / clones / unique cloners for all five
# public repos, plus a rough "human vs bot" split for clones.
#
# Bots that clone every new Go module within hours of publication:
#   proxy.golang.org, deps.dev, pkg.go.dev, goproxy.cn, goproxy.io,
#   sum.golang.google.cn, osv-scanner, snyk, socket.dev, ghArchive.
# Rule of thumb on a fresh-public-day-1 repo: 5-15 of the first 25
# unique cloners are bots. After a week, bots taper to ~2/day.
#
# Requires: gh CLI authenticated, python3.

set -euo pipefail

REPOS=(sckit-go kinrec input-go kinax-go kinclaw)
ORG=LocalKinAI

printf "%-12s  %8s  %10s  %8s  %12s\n" "repo" "views" "uniq 👀" "clones" "uniq 📥"
printf "%-12s  %8s  %10s  %8s  %12s\n" "────" "─────" "───────" "──────" "───────"

for r in "${REPOS[@]}"; do
  v=$(gh api "repos/$ORG/$r/traffic/views" 2>/dev/null | python3 -c 'import json,sys;d=json.load(sys.stdin);print(d["count"], d["uniques"])')
  c=$(gh api "repos/$ORG/$r/traffic/clones" 2>/dev/null | python3 -c 'import json,sys;d=json.load(sys.stdin);print(d["count"], d["uniques"])')
  read -r views uniq_v <<< "$v"
  read -r clones uniq_c <<< "$c"
  printf "%-12s  %8s  %10s  %8s  %12s\n" "$r" "$views" "$uniq_v" "$clones" "$uniq_c"
done

echo ""
echo "── top referrers ──"
for r in "${REPOS[@]}"; do
  refs=$(gh api "repos/$ORG/$r/traffic/popular/referrers" 2>/dev/null | python3 -c '
import json, sys
d = json.load(sys.stdin)
if not d: sys.exit()
parts = [x["referrer"] + "(" + str(x["uniques"]) + ")" for x in d]
print(" ".join(parts))
')
  [[ -n "$refs" ]] && echo "  $r: $refs"
done
