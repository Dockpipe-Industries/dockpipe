#!/usr/bin/env bash
set -euo pipefail
python3 workflows/qualify/cockpit/assets/scripts/prepare-cockpit.py prepare \
  --lock workflows/qualify/cockpit/assets/cockpit.lock.json --output "$RUNNER_TEMP/cockpit-runtime"
cmake -S src/app/tooling/dockpipe-launcher -B "$RUNNER_TEMP/cockpit-host" \
  -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=ON \
  -DCMAKE_PREFIX_PATH="$DOCKPIPE_QT_ROOT;$RUNNER_TEMP/browser-sdk" \
  -DDOCKPIPE_COCKPIT_ASSETS="$RUNNER_TEMP/cockpit-runtime/package/resolvers/cockpit/assets"
python3 workflows/qualify/cockpit/assets/scripts/prepare-cockpit.py verify-compiler \
  --lock workflows/qualify/cockpit/assets/cockpit.lock.json --store "$RUNNER_TEMP/cockpit-runtime/store" \
  --build "$RUNNER_TEMP/cockpit-host"
cmake --build "$RUNNER_TEMP/cockpit-host" --target cockpit-package-rendering-test --parallel 2
LD_LIBRARY_PATH="$DOCKPIPE_QT_ROOT/lib:$RUNNER_TEMP/browser-sdk/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" \
  ctest --test-dir "$RUNNER_TEMP/cockpit-host" --output-on-failure -R '^cockpit-package-rendering$' --no-tests=error

