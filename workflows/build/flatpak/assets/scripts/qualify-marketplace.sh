#!/usr/bin/env bash
set -euo pipefail
python3 -B workflows/build/flatpak/assets/scripts/verify-marketplace-cockpit.py \
  --catalog "$RUNNER_TEMP/flatpak-catalog/store" --cockpit-lock workflows/qualify/cockpit/assets/cockpit.lock.json \
  --test-build-dir "$RUNNER_TEMP/flatpak/qt-build" \
  --output "$RUNNER_TEMP/flatpak-marketplace-cockpit"
