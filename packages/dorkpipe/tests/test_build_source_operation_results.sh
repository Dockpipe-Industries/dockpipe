#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_build_source_operation_results failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

ROOT="$(git rev-parse --show-toplevel)"
SCRIPT="$ROOT/packages/dorkpipe/assets/scripts/build-source.sh"
# shellcheck source=tests/unit-tests/package-source-build-test-lib.sh
source "$ROOT/tests/unit-tests/package-source-build-test-lib.sh"

dockpipe_test_source_build_contract "$ROOT" "$SCRIPT" "dorkpipe" "dorkpipe" \
  "dorkpipe|lib|dorkpipe|./cmd/dorkpipe" \
  "skills-render|lib|skills-render.testexe|./cmd/skills-render" \
  "orchestrate-helper|lib|orchestrate-helper.testexe|./cmd/orchestrate-helper"

echo "test_build_source_operation_results OK"
