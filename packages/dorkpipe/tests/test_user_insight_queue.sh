#!/usr/bin/env bash
# Smoke test for the `dorkpipe insight ...` user-insight flow.
# Run from repo root: bash packages/dorkpipe/tests/test_user_insight_queue.sh
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:-$ROOT/src/bin/dockpipe}"
cd "$ROOT"
export PATH="$ROOT/src/bin${PATH:+:$PATH}"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_user_insight_queue"
dorkpipe_test_init_go_cache "$ROOT"

tmp="$(dorkpipe_test_mktemp_dir "$ROOT")"
trap 'rm -rf "$tmp"' EXIT

export DOCKPIPE_WORKDIR="$tmp"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
INSIGHTS_BY_CATEGORY="$("$DOCKPIPE_TEST_BIN" __state package-runtime --workdir "$tmp" --owner dorkpipe --path analysis/by-category)"
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-enqueue.sh" -m 'convention: use gofmt for Go.' >/dev/null
bash "$DOCKPIPE_SCRIPT_DIR/user-insight-enqueue.sh" -m 'SOC2 review will cover secret storage.' >/dev/null
process_output="$(bash "$DOCKPIPE_SCRIPT_DIR/user-insight-process.sh" 2>&1)"
printf '%s\n' "$process_output"
INSIGHTS_PATH="$(printf '%s\n' "$process_output" | sed -n 's/^user-insight-process: wrote \(.*\) (new normalized insights this run: [0-9][0-9]*)$/\1/p')"
if [[ -z "$INSIGHTS_PATH" || "$INSIGHTS_PATH" == "$tmp"/* ]]; then
	echo "test_user_insight_queue: process did not report an external durable insights path" >&2
	exit 1
fi

if ! dorkpipe_test_assert "$ROOT" insights-main "$INSIGHTS_PATH"; then
	echo "test_user_insight_queue: insights.json shape unexpected" >&2
	cat "$INSIGHTS_PATH" >&2 || true
	exit 1
fi

bash "$DOCKPIPE_SCRIPT_DIR/user-insight-export-by-category.sh"
if ! dorkpipe_test_assert "$ROOT" insights-category "$INSIGHTS_BY_CATEGORY/convention.json"; then
	echo "test_user_insight_queue: by-category export unexpected" >&2
	exit 1
fi

echo "test_user_insight_queue OK"
