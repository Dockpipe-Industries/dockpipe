#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_root="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$script_dir/../..}" && pwd)"
repo_root="$(cd "$package_root/../.." && pwd)"
source_build_lib="${DOCKPIPE_SOURCE_BUILD_LIB:-$repo_root/src/core/assets/scripts/lib/package-source-build.sh}"
source "$source_build_lib"
dockpipe_source_build_init "$repo_root"
dockpipe_source_build_tool "cloudflare-edge" "$repo_root" "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/cloudflare-edge" "remote" "./packages/remote/tools/cmd/cloudflare-edge"

# Materialize a self-contained resolver through the existing package compiler.
# The authored resolver remains source-only; binaries stay in disposable state.
dockpipe_cmd="${DOCKPIPE_BIN:-$repo_root/src/bin/dockpipe}"
staging_root="$("$dockpipe_cmd" __state package-runtime --workdir "$repo_root" --owner remote --path resolver-build)"
mkdir -p "$staging_root"
staging_dir="$(mktemp -d "$staging_root/cloudflare.XXXXXX")"
mkdir -p "$staging_dir/resolvers"
cp -R "$package_root/resolvers/cloudflare" "$staging_dir/resolvers/cloudflare"
mkdir -p "$staging_dir/resolvers/cloudflare/assets/tooling/bin"
cp "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/cloudflare-edge" "$staging_dir/resolvers/cloudflare/assets/tooling/bin/cloudflare-edge"
"$dockpipe_cmd" package compile resolvers --workdir "$repo_root" --from "$staging_dir/resolvers" --force
