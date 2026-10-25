#!/usr/bin/env bash
set -euo pipefail
python3 workflows/dependencies/browser-sdk/assets/scripts/prepare-browser-sdk.py --lock workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json \
  --platform "${DOCKPIPE_ADAPTER_0}" --output "$RUNNER_TEMP/browser-sdk"
echo "DOCKPIPE_BROWSER_SDK=$RUNNER_TEMP/browser-sdk" >> "$GITHUB_ENV"
