#!/usr/bin/env bash
# Self-contained Go tests for DorkPipe MCP (`dorkpipe.mcp`).
# From repo root: dockpipe package test --only dorkpipe.mcp
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
MCP_ROOT="$ROOT/packages/dorkpipe-mcp"
eval "$("$ROOT/src/bin/dockpipe" sdk --workdir "$ROOT")"
export TMPDIR="${DORKPIPE_MCP_PACKAGE_TEST_TMPDIR:-$ROOT/bin/.dockpipe/tmp/package-tests}"
mkdir -p "$TMPDIR"
mkdir -p "$(dockpipe_sdk path build go-cache)" "$(dockpipe_sdk path build go-tmp)"
export GOCACHE="${GOCACHE:-$(dockpipe_sdk path build go-cache)}"
export GOTMPDIR="${GOTMPDIR:-$(dockpipe_sdk path build go-tmp)}"
cd "$MCP_ROOT"
unset DOCKPIPE_WORKDIR
unset DOCKPIPE_REPO_ROOT
go test ./...
bash "$MCP_ROOT/tests/test_build_source_operation_results.sh"
