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

GOEXE="$(go env GOEXE)"
dockpipe_source_build_init "$REPO_ROOT"

dockpipe_source_build_tool "dorkpipe" "$PACKAGE_ROOT/lib" "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/dorkpipe" "dorkpipe" "./cmd/dorkpipe"
dockpipe_source_build_tool "skills-render" "$PACKAGE_ROOT/lib" "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/skills-render$GOEXE" "dorkpipe" "./cmd/skills-render"
dockpipe_source_build_tool "orchestrate-helper" "$PACKAGE_ROOT/lib" "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/orchestrate-helper$GOEXE" "dorkpipe" "./cmd/orchestrate-helper"
