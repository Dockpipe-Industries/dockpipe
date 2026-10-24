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

test -d "$ROOT/src/core/runtimes" || fail "missing src/core/runtimes"
test -f "$ROOT/src/core/workflows/run/config.yml" || fail "missing bundled workflow src/core/workflows/run/config.yml"
test -d "$ROOT/workflows" || fail "missing workflows/"
test -f "$ROOT/embed.go" || fail "missing repo-root embed.go"

test ! -L "$ROOT/src/scripts/pipeon" || fail "src/scripts/pipeon must not be a symlink (use paths.go + compile.workflows — see paths_test.go)"
test ! -e "$ROOT/src/scripts/review" || fail "src/scripts/review removed — keep review helpers out of src/scripts"
test ! -e "$ROOT/src/scripts/dockpipe" || fail "extracted DorkPipe scripts returned under src/scripts"
test ! -e "$ROOT/scripts/dockpipe" || fail "extracted DorkPipe scripts returned under repo-root scripts"
for retired in \
	packages \
	.staging/packages \
	src/lib/pipelang \
	src/lib/applicationir \
	src/cmd/pipelang-streamc \
	src/cmd/pipelang-native-cpp \
	src/app/tooling/vscode-extensions/dockpipe-language-support; do
	test ! -e "$ROOT/$retired" || fail "retired package source returned: $retired"
done

if rg -n '^replace[[:space:]]|=>[[:space:]]*\.\.?/' go.mod; then
	fail "source checkout dependency overrides are forbidden in the released module"
fi

rg -q '^[[:space:]]*github\.com/Dockpipe-Industries/pipelang[[:space:]]+v[0-9]' go.mod ||
	fail "PipeLang must be consumed through an exact external module version"

echo "test_repo_layout OK"
