#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
COMMON="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-common.sh"
# shellcheck source=/dev/null
source "$COMMON"

tmp="$(mktemp -d "${TMPDIR:-/tmp}/dorkpipe-training-metrics.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

export DORKPIPE_ORCH_TRAINING_METRICS_JSONL="$tmp/run/training/metrics.jsonl"
export DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS="$tmp/durable/training/metrics.jsonl"
export DORKPIPE_ORCH_TRAINING_MODE="observe"

dorkpipe_orchestrate_record_training_metric "task-1" "lane-a" "local" "completed" "0.8" 10 5 false false "start-1" "finish-1" 12
dorkpipe_orchestrate_record_training_metric "task-2" "lane-b" "codex" "completed" "0.9" 20 7 true false "start-2" "finish-2" 18

[[ "$(wc -l < "$DORKPIPE_ORCH_TRAINING_METRICS_JSONL" | tr -d ' ')" == "2" ]]
[[ "$(wc -l < "$DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS" | tr -d ' ')" == "2" ]]
grep -q '"task_id":"task-1"' "$DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS"
grep -q '"task_id":"task-2"' "$DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS"

if [[ "$(uname -s)" != MINGW* && "$(uname -s)" != MSYS* && "$(uname -s)" != CYGWIN* ]]; then
	mode="$(stat -c '%a' "$DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS" 2>/dev/null || stat -f '%Lp' "$DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS")"
	[[ "$mode" == "600" ]]
fi

echo "durable training metrics test OK"
