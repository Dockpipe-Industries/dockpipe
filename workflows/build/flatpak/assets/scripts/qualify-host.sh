#!/usr/bin/env bash
set -euo pipefail
python3 workflows/build/flatpak/assets/scripts/verify-installed-host.py \
  --bundle "$RUNNER_TEMP/flatpak/dockpipe-desktop_${DOCKPIPE_FLATPAK_VERSION}_linux_amd64.flatpak" \
  --build-dir "$RUNNER_TEMP/flatpak/qt-build" \
  --out "$RUNNER_TEMP/flatpak-host-qualification"
