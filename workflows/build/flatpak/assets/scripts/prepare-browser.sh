#!/usr/bin/env bash
set -euo pipefail
python3 -B workflows/build/flatpak/assets/scripts/prepare-flatpak-browser-sdk.py --lock workflows/build/flatpak/assets/spatialweb-flatpak-browser.lock.json \
  --verify-installed-refs --output "$RUNNER_TEMP/browser-sdk"
echo "DOCKPIPE_FLATPAK_BROWSER_SDK=$RUNNER_TEMP/browser-sdk" >> "$GITHUB_ENV"
