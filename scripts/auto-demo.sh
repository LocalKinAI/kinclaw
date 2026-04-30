#!/usr/bin/env bash
# auto-demo.sh — drive a marketing task through the marketer soul.
#
# Pipeline:
#   1. Read scripts/marketing-tasks.yaml (or path via $TASKS_YAML)
#   2. Look up the task by id
#   3. Construct a one-shot -exec prompt with the task fields
#   4. Spawn `kinclaw -soul souls/marketer.soul.md -exec <prompt>`
#   5. Marketer drives the 5 claws, records, voices over, saves MP4
#
# Usage:
#   ./scripts/auto-demo.sh <task_id>
#   TASKS_YAML=~/.localkin/marketing.yaml ./scripts/auto-demo.sh meta_recursion_seed
#
# Exit codes:
#   0   marketer reported success (✓ in output)
#   1   marketer reported failure (✗ in output)
#   2   bad invocation / task not found
#   3   missing dependencies (yq / kinclaw binary)

set -euo pipefail

TASK_ID="${1:-}"
if [[ -z "$TASK_ID" ]]; then
    cat <<EOF >&2
Usage: $0 <task_id>

Available tasks (Tier 1):
  meta_recursion_seed   — KinClaw records itself recording itself (30s)
  vs_operator_native    — Side-by-side with OpenAI Operator (90s)
  skill_forge_live      — Live skill forging demo (90s)
  soul_clone_10x        — 10 parallel clones (60s)
  5_models_debate       — Multi-lab routing demo (90s)

See scripts/marketing-tasks.yaml for the full pool (30 tasks across 6 tiers).
EOF
    exit 2
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TASKS_YAML="${TASKS_YAML:-$REPO_ROOT/scripts/marketing-tasks.yaml}"
KINCLAW_BIN="${KINCLAW_BIN:-$REPO_ROOT/kinclaw}"
SOUL_PATH="${SOUL_PATH:-$REPO_ROOT/souls/marketer.soul.md}"
OUTPUT_DIR="${OUTPUT_DIR:-$REPO_ROOT/output/demos}"

# Tools sanity
if ! command -v yq >/dev/null 2>&1; then
    echo "auto-demo: yq not installed. brew install yq" >&2
    exit 3
fi
if [[ ! -x "$KINCLAW_BIN" ]]; then
    echo "auto-demo: kinclaw binary not found at $KINCLAW_BIN (run: go build -o kinclaw ./cmd/kinclaw)" >&2
    exit 3
fi
if [[ ! -f "$TASKS_YAML" ]]; then
    echo "auto-demo: tasks YAML not found at $TASKS_YAML" >&2
    exit 3
fi
if [[ ! -f "$SOUL_PATH" ]]; then
    echo "auto-demo: marketer soul not found at $SOUL_PATH" >&2
    exit 3
fi

mkdir -p "$OUTPUT_DIR"

# yq lookup: walk all tier_*.tasks[] arrays, find the one with id == TASK_ID.
# Output one yaml document with the fields the marketer prompt needs.
TASK_YAML="$(
    yq -o=yaml ".. | select(has(\"id\") and .id == \"$TASK_ID\")" "$TASKS_YAML" 2>/dev/null \
    | head -200
)"
if [[ -z "$TASK_YAML" ]]; then
    echo "auto-demo: task id $TASK_ID not found in $TASKS_YAML" >&2
    exit 2
fi

# Extract fields. yq's default null-renders "null"; replace with "" for cleanliness.
field() {
    local val
    val="$(echo "$TASK_YAML" | yq -r ".$1 // \"\"" 2>/dev/null || true)"
    [[ "$val" == "null" ]] && val=""
    echo "$val"
}

TITLE_ZH="$(field title_zh)"
TITLE_EN="$(field title_en)"
NARRATIVE_ZH="$(field narrative_zh)"
NARRATIVE_EN="$(field narrative_en)"
DURATION_SEC="$(field duration_sec)"
CAP="$(echo "$TASK_YAML" | yq -o=json '.capability_showcased // []' 2>/dev/null || echo '[]')"
STEPS="$(echo "$TASK_YAML" | yq -r '.task_steps[]? | "  - " + .' 2>/dev/null || true)"

TS="$(date +%Y%m%d-%H%M%S)"
OUT_PATH="$OUTPUT_DIR/${TASK_ID}-${TS}.mp4"

# Construct the marketer prompt. Heredoc for clarity; the marketer soul body
# already explains the pipeline shape, so the prompt just hands over the task.
PROMPT="$(cat <<EOF
你正在跑营销 task: $TASK_ID

## Task metadata
title_zh: $TITLE_ZH
title_en: $TITLE_EN
narrative_zh: $NARRATIVE_ZH
narrative_en: $NARRATIVE_EN
duration_sec: $DURATION_SEC
capability_showcased: $CAP

## task_steps (自然语言指引,你 LLM 解读后用 5 爪执行)
$STEPS

## 输出
最终 MP4 落到: $OUT_PATH

按你 soul 里的流程模板做：clean desktop → record start → 执行 task_steps
（关键 click 前后插 screen 截图 + tts 解说）→ record stop → mv 到上面那个
最终 path → 输出 ✓ task=$TASK_ID 的回报。
EOF
)"

echo "── auto-demo running task: $TASK_ID"
echo "   soul:    $SOUL_PATH"
echo "   binary:  $KINCLAW_BIN"
echo "   output:  $OUT_PATH"
echo "   ─── handing to marketer ───"
echo

# Spawn marketer. Marketer's stderr (kinclaw banner) and stdout (final
# response) go straight to console — caller (cron / human) sees both.
"$KINCLAW_BIN" -soul "$SOUL_PATH" -exec "$PROMPT"
RC=$?

echo
if [[ $RC -ne 0 ]]; then
    echo "── auto-demo: marketer exited with status $RC"
    exit $RC
fi
echo "── auto-demo: marketer finished. Verify $OUT_PATH"
