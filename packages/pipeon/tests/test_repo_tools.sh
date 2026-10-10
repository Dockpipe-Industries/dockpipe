#!/usr/bin/env bash
# Pipeon helper resolution should prefer the repo-local dockpipe binary.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:-$ROOT/src/bin/dockpipe}"

normalize_path() {
  local raw="${1:-}"
  [[ -n "$raw" ]] || return 0
  case "$raw" in
    [A-Za-z]:\\*|[A-Za-z]:/*|\\\\*)
      if command -v cygpath >/dev/null 2>&1; then
        cygpath -u "$raw"
        return 0
      fi
      ;;
  esac
  printf '%s\n' "$raw"
}

ROOT_NORM="$(normalize_path "$ROOT")"

# shellcheck source=/dev/null
source "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"

actual="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk require dockpipe-bin' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
expected="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_resolve_dockpipe_bin "$2"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh" "$ROOT")"

if [[ "$actual" != "$expected" ]]; then
  echo "test_repo_tools: expected $expected, got $actual" >&2
  exit 1
fi

build_cache="$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk path build npm-cache' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
expected_build_cache="$ROOT_NORM/bin/.dockpipe/build/npm-cache"
if [[ "$build_cache" != "$expected_build_cache" ]]; then
  echo "test_repo_tools: expected build cache under root bin/.dockpipe, got $build_cache" >&2
  exit 1
fi

package_state="$(normalize_path "$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk scope --package dorkpipe dev-stack' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")")"
expected_package_state="$(normalize_path "$("$ROOT/src/bin/dockpipe" scope --package dorkpipe dev-stack --workdir "$ROOT")")"
if [[ "$package_state" != "$expected_package_state" ]]; then
  echo "test_repo_tools: expected package state $expected_package_state, got $package_state" >&2
  exit 1
fi

workflow_state="$(normalize_path "$(DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk scope workflow docs.orchestrate orchestrate' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")")"
expected_workflow_state="$(normalize_path "$("$ROOT/src/bin/dockpipe" scope workflow docs.orchestrate orchestrate --workdir "$ROOT")")"
if [[ "$workflow_state" != "$expected_workflow_state" ]]; then
  echo "test_repo_tools: expected workflow state $expected_workflow_state, got $workflow_state" >&2
  exit 1
fi

ci_default="$(normalize_path "$(env -u DOCKPIPE_WORKFLOW_NAME -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT DOCKPIPE_CI_ARTIFACT_SCOPE=package:pipeon DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk ci analysis findings.json' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")")"
state_root="$ROOT_NORM/bin/.dockpipe"
package_runtime="$(normalize_path "$("$DOCKPIPE_TEST_BIN" __state package-runtime --workdir "$ROOT" --owner pipeon)")"
expected_ci_default="$package_runtime/ci/analysis/findings.json"
if [[ "$ci_default" != "$expected_ci_default" ]]; then
  echo "test_repo_tools: expected explicitly bound CI artifacts under Pipeon package runtime, got $ci_default" >&2
  exit 1
fi

ci_default_injected="$(normalize_path "$(env -u DOCKPIPE_WORKFLOW_NAME -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT DOCKPIPE_CI_ARTIFACT_SCOPE=package:pipeon DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; printf "%s\n" "$DOCKPIPE_CI_ANALYSIS_DIR/findings.json"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")")"
if [[ "$ci_default_injected" != "$expected_ci_default" ]]; then
  echo "test_repo_tools: expected SDK refresh to inject explicitly bound CI artifacts, got $ci_default_injected" >&2
  exit 1
fi

ci_unbound="$(env -u DOCKPIPE_WORKFLOW_NAME -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT -u DOCKPIPE_CI_ARTIFACT_SCOPE DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; printf "%s|%s\n" "${DOCKPIPE_CI_RAW_DIR:-}" "${DOCKPIPE_CI_ANALYSIS_DIR:-}"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
if [[ "$ci_unbound" != "|" ]]; then
  echo "test_repo_tools: expected unbound generic SDK CI dirs to remain unset, got $ci_unbound" >&2
  exit 1
fi
if env -u DOCKPIPE_WORKFLOW_NAME -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_CI_ARTIFACT_SCOPE DOCKPIPE_WORKDIR="$ROOT" bash -lc 'source "$1"; dockpipe_sdk ci analysis >/dev/null 2>&1' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"; then
  echo "test_repo_tools: expected an unbound generic SDK CI lookup to fail" >&2
  exit 1
fi

ci_explicit="$(env -u DOCKPIPE_WORKFLOW_NAME -u DOCKPIPE_CI_ARTIFACT_SCOPE DOCKPIPE_WORKDIR="$ROOT" bash -lc 'DOCKPIPE_CI_RAW_DIR=/custom/raw; DOCKPIPE_CI_ANALYSIS_DIR=/custom/analysis; source "$1"; bash -c '\''printf "%s|%s\n" "$DOCKPIPE_CI_RAW_DIR" "$DOCKPIPE_CI_ANALYSIS_DIR"'\''' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh")"
if [[ "$ci_explicit" != "/custom/raw|/custom/analysis" ]]; then
  echo "test_repo_tools: expected explicit CI dirs to remain exported, got $ci_explicit" >&2
  exit 1
fi

ci_bound="$(env -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT -u DOCKPIPE_CI_ARTIFACT_SCOPE DOCKPIPE_WORKDIR="$ROOT" DOCKPIPE_WORKFLOW_NAME=ci bash -lc 'source "$1"; printf "%s\n" "$(dockpipe_sdk ci raw)" "$(dockpipe_sdk ci analysis)"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh" | while IFS= read -r line; do normalize_path "$line"; done)"
expected_ci_bound="$state_root/workflows/ci/artifacts/ci-raw
$state_root/workflows/ci/artifacts/ci-analysis"
if [[ "$ci_bound" != "$expected_ci_bound" ]]; then
  echo "test_repo_tools: expected workflow-bound CI artifacts, got $ci_bound" >&2
  exit 1
fi

ci_injected="$(env -u DOCKPIPE_CI_RAW_DIR -u DOCKPIPE_CI_ANALYSIS_DIR -u DOCKPIPE_ARTIFACT_ROOT -u DOCKPIPE_OUTPUT_ROOT -u DOCKPIPE_CI_ARTIFACT_SCOPE DOCKPIPE_WORKDIR="$ROOT" DOCKPIPE_WORKFLOW_NAME=ci bash -lc 'source "$1"; printf "%s\n" "$DOCKPIPE_CI_RAW_DIR" "$DOCKPIPE_CI_ANALYSIS_DIR"' _ "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh" | while IFS= read -r line; do normalize_path "$line"; done)"
if [[ "$ci_injected" != "$expected_ci_bound" ]]; then
  echo "test_repo_tools: expected SDK refresh to inject workflow-bound CI dirs, got $ci_injected" >&2
  exit 1
fi

echo "test_repo_tools OK"
