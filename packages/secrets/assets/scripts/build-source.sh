#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_root="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$script_dir/../..}" && pwd)"
repo_root="$(cd "$package_root/../.." && pwd)"
# shellcheck source=/dev/null
source "${DOCKPIPE_SOURCE_BUILD_LIB:-$repo_root/src/core/assets/scripts/lib/package-source-build.sh}"
dockpipe_source_build_init "$repo_root"
tool="secret-environment$(go env GOEXE)"
dockpipe_source_build_tool "$tool" "$repo_root" "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/$tool" secrets ./packages/secrets/tools/cmd/secret-environment
dockpipe_cmd="${DOCKPIPE_BIN:-$repo_root/src/bin/dockpipe}"
staging_root="$("$dockpipe_cmd" __state package-runtime --workdir "$repo_root" --owner secrets --path resolver-build)"
mkdir -p "$staging_root"
staging_dir="$(mktemp -d "$staging_root/environments.XXXXXX")"
trap 'rm -rf "$staging_dir"' EXIT
mkdir -p "$staging_dir/resolvers"
for provider in onepassword aws-secretsmanager azure-keyvault infisical; do
  cp -R "$package_root/resolvers/$provider" "$staging_dir/resolvers/$provider"
  mkdir -p "$staging_dir/resolvers/$provider/assets/tooling/bin"
  cp "$DOCKPIPE_SOURCE_BUILD_OUT_DIR/$tool" "$staging_dir/resolvers/$provider/assets/tooling/bin/$tool"
done
"$dockpipe_cmd" package compile resolvers --workdir "$repo_root" --from "$staging_dir/resolvers" --force
