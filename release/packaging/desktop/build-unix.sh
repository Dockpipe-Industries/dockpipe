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
elif [[ "$(uname -m)" == x86_64 ]]; then
  export DOCKPIPE_QT_ROOT="${DOCKPIPE_QT_ROOT:-/opt/Qt/6.10.3/gcc_64}"
  [[ "$("$DOCKPIPE_QT_ROOT/bin/qmake" -query QT_VERSION)" == 6.10.3 ]]
  args+=("-DCMAKE_PREFIX_PATH=$DOCKPIPE_QT_ROOT"
         "-DCMAKE_CXX_FLAGS_RELEASE=-O2 -g -DNDEBUG -ffile-prefix-map=$root=. -ffile-prefix-map=$build=.")
fi
if [[ "${DOCKPIPE_BUILD_COCKPIT:-0}" == 1 ]]; then
  [[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]]
  export DOCKPIPE_GIT_TEST_CLI="$root/src/bin/dockpipe"
  [[ -x "$DOCKPIPE_GIT_TEST_CLI" ]]
  args+=("-DDOCKPIPE_COCKPIT_SOURCE=$root/packages/cockpit"
         "-DForgePipe_DIR=${DOCKPIPE_FORGEPIPE_SDK:?Verified native SDK required}/lib/cmake/ForgePipe")
fi
cmake "${args[@]}"
cmake --build "$build" --parallel 2
ctest --test-dir "$build" --output-on-failure
if [[ "${DOCKPIPE_BUILD_COCKPIT:-0}" == 1 ]]; then
  bash "$root/packages/cockpit/assets/scripts/package-release.sh" "$version" "$build" "$out/cockpit-store"
fi
if [[ "$(uname -s)" == Darwin ]]; then
  bash "$root/release/packaging/desktop/macos/package.sh" "$version" "$build/DockPipe.app" "$out"
else
  bash "$root/release/packaging/desktop/build-deb.sh" "$version" "$build/dockpipe-launcher" "$out"
fi
