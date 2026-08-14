#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$SCRIPT_DIR/../..}" && pwd)"
REPO_ROOT="$(cd "$PACKAGE_ROOT/../.." && pwd)"

OUT_DIR="$REPO_ROOT/bin/.dockpipe/tooling/bin"
BUILD_DIR="$REPO_ROOT/bin/.dockpipe/build"
VERSION_FILE="$REPO_ROOT/VERSION"

version="0.0.0"
if [[ -f "$VERSION_FILE" ]]; then
  version="$(tr -d ' \t\r\n' < "$VERSION_FILE")"
fi
ldflags="-s -w -X main.Version=${version}"

mkdir -p "$OUT_DIR" "$BUILD_DIR/go-cache" "$BUILD_DIR/go-tmp"
export GOCACHE="${GOCACHE:-$BUILD_DIR/go-cache}"
export GOTMPDIR="${GOTMPDIR:-$BUILD_DIR/go-tmp}"

now_ms() {
  date +%s%3N
}

emit_result() {
  local status="${1:?status}"
  local duration_ms="${2:-}"
  local error="${3:-}"
  local dockpipe_bin="${DOCKPIPE_BIN:-dockpipe}"
  local args=("result" "--unit" "package.source.tool" "--status" "$status")
  if [[ -n "$duration_ms" && "$status" != "start" ]]; then
    args+=("--duration-ms" "$duration_ms")
  fi
  args+=("--id" "tool=mcpd" "--id" "output=$OUT_DIR/mcpd" "--id" "package=dorkpipe.mcp")
  if [[ -n "$error" ]]; then
    args+=("--error" "$error")
  fi
  if command -v "$dockpipe_bin" >/dev/null 2>&1 && "$dockpipe_bin" "${args[@]}"; then
    return 0
  fi
  printf '[dockpipe] unit=package.source.tool status=%s' "$status" >&2
  if [[ -n "$duration_ms" && "$status" != "start" ]]; then
    printf ' duration_ms=%s' "$duration_ms" >&2
  fi
  printf ' tool=mcpd output=%s package=dorkpipe.mcp' "$OUT_DIR/mcpd" >&2
  if [[ -n "$error" ]]; then
    printf ' error=%s' "$error" >&2
  fi
  printf '\n' >&2
}

emit_result start ""
started_ms="$(now_ms)"
set +e
go build -C "$PACKAGE_ROOT" -trimpath -ldflags "$ldflags" -o "$OUT_DIR/mcpd" ./cmd/mcpd
rc=$?
set -e
duration_ms="$(( $(now_ms) - started_ms ))"
if [[ "$rc" -ne 0 ]]; then
  emit_result fail "$duration_ms" "go build exited $rc"
  exit "$rc"
fi
emit_result done "$duration_ms"
