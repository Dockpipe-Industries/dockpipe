#!/usr/bin/env bash
set -euo pipefail
python3 -B workflows/build/flatpak/assets/scripts/verify-installed-cockpit.py \
  --lock workflows/qualify/cockpit/assets/cockpit.lock.json --prepared "$RUNNER_TEMP/flatpak-cockpit-runtime" \
  --browser-sdk "$DOCKPIPE_FLATPAK_BROWSER_SDK" \
  --app-build-dir "$RUNNER_TEMP/flatpak/app" --test-build-dir "$RUNNER_TEMP/flatpak/qt-build" \
  --output "$RUNNER_TEMP/flatpak-cockpit-qualification"
