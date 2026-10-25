#!/usr/bin/env bash
# Quick checks: Go tests + Dockpipe package/workflow tests + template path guard + bash unit tests (no Docker integration sweep).
# From repo root:  make -f tooling/Makefile test-quick
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
cd "$ROOT"
export DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$ROOT}"

UNIT="$(cd "${ROOT}/tests/unit-tests" && pwd)"

echo "=== go test ==="
python3 workflows/ci/core/assets/scripts/test.py

echo "=== dockpipe package test ==="
go run ./src/cmd package test --workdir "$ROOT"

echo "=== dockpipe workflow test ==="
go run ./src/cmd workflow test --workdir "$ROOT"

echo "=== check-templates-core-paths ==="
bash workflows/ci/repository-layout/assets/scripts/check-paths.sh

for f in test_cli.sh test_repo_root.sh test_repo_layout.sh test_clone_worktree_include.sh; do
	echo "=== $f ==="
	bash "${UNIT}/${f}"
done

echo ""
echo "test-quick: ok"
