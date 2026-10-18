#!/usr/bin/env bash
set -euo pipefail
package_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$package_root/../.." && pwd)"
dockpipe_bin="${DOCKPIPE_BIN:-$repo_root/src/bin/dockpipe}"
build_root="${DOCKPIPE_COCKPIT_TEST_BUILD:-$("$dockpipe_bin" __state package-runtime --workdir "$repo_root" --owner cockpit --path native-tests)}"
python3 -m unittest discover -s "$package_root/tests" -p test_neural.py
cmake -S "$repo_root/src/app/tooling/dockpipe-launcher" -B "$build_root" \
  -DBUILD_TESTING=ON "-DDOCKPIPE_COCKPIT_SOURCE=$package_root"
cmake --build "$build_root" --target \
  cockpit-test cockpit-package-rendering-test workspace-graph-test \
  spatial-workspace-test desktop-app-window-test spatial-gpu-test \
  launcher-settings-test workspace-package-test \
  --parallel "${DOCKPIPE_BUILD_JOBS:-2}"
ctest --test-dir "$build_root" --output-on-failure \
  -R '^(site-world|furnishing|input-controls|git-review|flight-dynamics|cockpit|cockpit-package-rendering|spatial-cockpit|spatial-operations|spatial-intelligence|spatial-web-model|spatial-web-browser|spatial-vehicles|dirt-bike|workspace-graph|spatial-workspace|desktop-app-window|spatial-gpu|launcher-settings|workspace-package)$'
