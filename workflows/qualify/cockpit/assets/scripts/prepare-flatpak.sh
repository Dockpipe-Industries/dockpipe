#!/usr/bin/env bash
set -euo pipefail
python3 -B workflows/qualify/cockpit/assets/scripts/prepare-cockpit.py prepare \
  --lock workflows/qualify/cockpit/assets/cockpit.lock.json --output "$RUNNER_TEMP/flatpak-cockpit-runtime"
echo "DOCKPIPE_FLATPAK_COCKPIT_ASSETS=$RUNNER_TEMP/flatpak-cockpit-runtime/package/resolvers/cockpit/assets" >> "$GITHUB_ENV"
