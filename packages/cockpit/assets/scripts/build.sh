#!/usr/bin/env bash
# Produce an ordinary resolver archive; the source tree never receives binaries.
set -euo pipefail
# General source builds stay headless; release builders opt into the desktop payload.
if [[ "${DOCKPIPE_SOURCE_BUILD:-0}" == 1 && "${DOCKPIPE_BUILD_COCKPIT:-0}" != 1 ]]; then
  echo 'Skipping optional Cockpit native bundle (set DOCKPIPE_BUILD_COCKPIT=1 to build it).' >&2
  exit 0
fi
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_root="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$script_dir/../..}" && pwd)"
repo_root="$(cd "$package_root/../.." && pwd)"
dockpipe_bin="${DOCKPIPE_BIN:-$repo_root/src/bin/dockpipe}"
[[ "$(uname -s)" == Linux ]] || { echo 'Cockpit runtime packaging currently supports Linux only.' >&2; exit 1; }
build_root="$("$dockpipe_bin" __state package-runtime --workdir "$repo_root" --owner cockpit --path native-build)"
cmake -S "$repo_root/src/app/tooling/dockpipe-launcher" -B "$build_root" \
  -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=OFF "-DDOCKPIPE_COCKPIT_SOURCE=$package_root"
cmake --build "$build_root" --target dockpipe-cockpit --parallel "${DOCKPIPE_BUILD_JOBS:-2}"
staging_root="$("$dockpipe_bin" __state package-runtime --workdir "$repo_root" --owner cockpit --path resolver-build)"
mkdir -p "$staging_root"
staging_dir="$(mktemp -d "$staging_root/cockpit.XXXXXX")"
trap 'rm -rf -- "$staging_dir"' EXIT
mkdir -p "$staging_dir/resolvers"
cp -R "$package_root/resolvers/cockpit" "$staging_dir/resolvers/cockpit"
mv "$staging_dir/resolvers/cockpit/profile.in" "$staging_dir/resolvers/cockpit/profile"
python3 "$script_dir/bundle-linux.py" "$build_root" "$staging_dir/resolvers/cockpit/assets"
"$dockpipe_bin" package compile resolvers --workdir "$repo_root" --from "$staging_dir" --force
