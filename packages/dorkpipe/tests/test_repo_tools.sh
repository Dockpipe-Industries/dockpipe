#!/usr/bin/env bash
# Shared DockPipe helper should prefer the repo-local dockpipe build, and the
# DorkPipe package helper should resolve the package-local dorkpipe tool.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"

test_dockpipe_bin="${DOCKPIPE_TEST_DOCKPIPE_BIN:-}"
unset DOCKPIPE_BIN DOCKPIPE_WORKDIR DOCKPIPE_SDK_ROOT
if [[ -n "$test_dockpipe_bin" ]]; then
  export DOCKPIPE_BIN="$test_dockpipe_bin"
fi

# shellcheck source=/dev/null
source "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"
# shellcheck source=/dev/null
source "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/lib/dorkpipe-cli.sh"

dockpipe_sdk refresh "$ROOT"

expected_dockpipe="$(dockpipe_resolve_dockpipe_bin "$ROOT")"
actual_dockpipe="$(dockpipe_sdk require dockpipe-bin)"
if [[ "$actual_dockpipe" != "$expected_dockpipe" ]]; then
  echo "test_repo_tools: expected dockpipe $expected_dockpipe, got $actual_dockpipe" >&2
  exit 1
fi

expected_dorkpipe="$(dorkpipe_script_resolve_bin "$ROOT")"
actual_dorkpipe="$(dorkpipe_script_resolve_bin "$ROOT")"
if [[ "$actual_dorkpipe" != "$expected_dorkpipe" ]]; then
  echo "test_repo_tools: expected dorkpipe $expected_dorkpipe, got $actual_dorkpipe" >&2
  exit 1
fi

actual_orch="$(dorkpipe_script_resolve_bin "$ROOT")"
if [[ "$actual_orch" != "$expected_dorkpipe" ]]; then
  echo "test_repo_tools: expected orchestrator dorkpipe $expected_dorkpipe, got $actual_orch" >&2
  exit 1
fi

ci_analysis="$(DOCKPIPE_WORKDIR="$ROOT" DOCKPIPE_CI_ARTIFACT_SCOPE=package:dorkpipe bash -lc 'source "$1"; dockpipe_sdk ci analysis findings.json' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
runtime_root="$("$expected_dockpipe" __state package-runtime --workdir "$ROOT" --owner dorkpipe)"
expected_ci_analysis="$runtime_root/ci/analysis/findings.json"
if [[ "$ci_analysis" != "$expected_ci_analysis" ]]; then
  echo "test_repo_tools: expected explicit DorkPipe CI binding at $expected_ci_analysis, got $ci_analysis" >&2
  exit 1
fi

echo "test_repo_tools OK"
