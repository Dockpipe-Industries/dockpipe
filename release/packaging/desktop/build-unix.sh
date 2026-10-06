#!/usr/bin/env bash
# Native Qt desktop build. Output stays outside the authored source tree.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
out="${2:?output directory}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
build="$(mktemp -d)"
trap 'rm -rf "$build"' EXIT
args=(-S "$root/src/app/tooling/dockpipe-launcher" -B "$build" -DCMAKE_BUILD_TYPE=Release
      "-DDOCKPIPE_RELEASE_VERSION=$version")
if [[ "$(uname -s)" == Darwin ]]; then
  args+=("-DCMAKE_OSX_ARCHITECTURES=$(uname -m)" -DCMAKE_OSX_DEPLOYMENT_TARGET=13.0)
fi
cmake "${args[@]}"
cmake --build "$build" --parallel 2
if [[ "$(uname -s)" == Darwin ]]; then
  bash "$root/release/packaging/desktop/macos/package.sh" "$version" "$build/DockPipe.app" "$out"
else
  bash "$root/release/packaging/desktop/build-deb.sh" "$version" "$build/dockpipe-launcher" "$out"
fi
