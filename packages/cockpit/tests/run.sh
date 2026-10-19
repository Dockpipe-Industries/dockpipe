#!/usr/bin/env bash
set -euo pipefail
package_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$package_root/../.." && pwd)"
dockpipe_bin="${DOCKPIPE_BIN:-$repo_root/src/bin/dockpipe}"
python3 -B -m unittest discover -s "$package_root/tests" -p 'test_*.py'
if [[ "${DOCKPIPE_COCKPIT_NATIVE_TEST:-0}" != 1 ]]; then
  echo 'PASS: Cockpit portable contracts. Native rendering/build proof belongs to the required cockpit-native CI job.'
  exit 0
fi
build_root="${DOCKPIPE_COCKPIT_TEST_BUILD:-$("$dockpipe_bin" __state package-runtime --workdir "$repo_root" --owner cockpit --path native-tests)}"
mkdir -p "$build_root"
sdk_work="$(mktemp -d "$build_root/forgepipe.XXXXXX")"
trap 'rm -rf -- "$sdk_work"' EXIT
# shellcheck source=../assets/scripts/native-environment.sh
source "$package_root/assets/scripts/native-environment.sh"
cockpit_prepare_native "$sdk_work" "$package_root/assets/scripts"
cmake -S "$repo_root/src/app/tooling/dockpipe-launcher" -B "$build_root" \
  -DBUILD_TESTING=ON "-DDOCKPIPE_COCKPIT_SOURCE=$package_root" \
  "-DCMAKE_PREFIX_PATH=$CMAKE_PREFIX_PATH" "-DQt6_DIR=$DOCKPIPE_QT_ROOT/lib/cmake/Qt6" \
  "-DForgePipe_DIR=$DOCKPIPE_FORGEPIPE_SDK/lib/cmake/ForgePipe"
cmake --build "$build_root" --target \
  cockpit-test cockpit-package-rendering-test workspace-graph-test \
  spatial-workspace-test desktop-app-window-test spatial-gpu-test \
  launcher-settings-test workspace-package-test \
  --parallel "${DOCKPIPE_BUILD_JOBS:-2}"
ctest --test-dir "$build_root" --output-on-failure \
  --no-tests=error \
  -R '^(site-world|furnishing|input-controls|git-review|flight-dynamics|cockpit|cockpit-package-rendering|spatial-cockpit|spatial-operations|spatial-intelligence|spatial-civic|spatial-web-model|spatial-web-browser|spatial-web-images|spatial-vehicles|spatial-terrain|return-portals|native-handoff|dirt-bike|workspace-graph|spatial-workspace|desktop-app-window|spatial-gpu|launcher-settings|workspace-package)$'
# CTest's offscreen GPU skip is not graphics proof. This required native pass
# fails on exit 77 and uses the same executable under a real X11 GL context.
timeout 90 xvfb-run -a env QT_QPA_PLATFORM=xcb LIBGL_ALWAYS_SOFTWARE=1 "$build_root/spatial-gpu-test"
timeout 90 xvfb-run -a env QT_QPA_PLATFORM=xcb LIBGL_ALWAYS_SOFTWARE=1 SDL_AUDIODRIVER=dummy \
  "DOCKPIPE_WORKSPACE_PACKAGE=$build_root/cockpit-package/assets" "$build_root/cockpit-package-rendering-test"
echo 'PASS: Cockpit matching-ABI native build, package tests and X11 graphics qualification.'
