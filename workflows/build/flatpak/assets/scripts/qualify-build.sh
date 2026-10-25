#!/usr/bin/env bash
set -euo pipefail
bash workflows/build/flatpak/assets/scripts/build.sh "$DOCKPIPE_FLATPAK_VERSION" "$RUNNER_TEMP/dockpipe" - "$RUNNER_TEMP/flatpak"
bash workflows/build/flatpak/assets/scripts/build-package.sh "$RUNNER_TEMP/flatpak/app" workflows/build/flatpak/tests/fixtures/flatpak-package "$RUNNER_TEMP/flatpak-package"
