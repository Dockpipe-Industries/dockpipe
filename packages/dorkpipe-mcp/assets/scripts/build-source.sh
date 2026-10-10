#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$SCRIPT_DIR/../..}" && pwd)"
REPO_ROOT="$(cd "$PACKAGE_ROOT/../.." && pwd)"

SOURCE_BUILD_LIB="${DOCKPIPE_SOURCE_BUILD_LIB:-}"
if [[ -z "$SOURCE_BUILD_LIB" ]]; then
  if [[ -n "${DOCKPIPE_SDK_SH:-}" ]]; then
    SOURCE_BUILD_LIB="$(cd "$(dirname "$DOCKPIPE_SDK_SH")" && pwd)/package-source-build.sh"
  else
    SOURCE_BUILD_LIB="$REPO_ROOT/src/core/assets/scripts/lib/package-source-build.sh"
  fi
fi
# shellcheck source=src/core/assets/scripts/lib/package-source-build.sh
source "$SOURCE_BUILD_LIB"

dockpipe_source_build_init "$REPO_ROOT"
dockpipe_source_build_tool "mcpd" "$PACKAGE_ROOT" "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/mcpd" "dorkpipe.mcp" "./cmd/mcpd"
