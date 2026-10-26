#!/usr/bin/env bash
# Guardrails for repo layout and extracted package boundaries.
# Run from repo root: bash tests/unit-tests/test_repo_layout.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

fail() {
	echo "test_repo_layout: FAIL — $1" >&2
	exit 1
}

command -v rg >/dev/null 2>&1 || fail "ripgrep (rg) is required"

for resource in \
 src/lib/infrastructure/runtime_defaults.go \
 src/lib/runtimeassets/containers.go \
 src/lib/runtimeassets/git.go \
 src/lib/sdkassets/assets.go \
 src/lib/sdk/repotools/repotools.go \
 workflows/shared/bootstrap.lock.json; do
 test -f "$ROOT/$resource" || fail "missing library/runtime ownership resource: $resource"
done
test -d "$ROOT/workflows" || fail "missing owning maintainer workflows/"

if rg -n '^replace[[:space:]]|=>[[:space:]]*\.\.?/' go.mod; then
	fail "source checkout dependency overrides are forbidden in the released module"
fi

rg -q '^[[:space:]]*github\.com/Dockpipe-Industries/pipelang[[:space:]]+v[0-9]' go.mod ||
	fail "PipeLang must be consumed through an exact external module version"

echo "test_repo_layout OK"
