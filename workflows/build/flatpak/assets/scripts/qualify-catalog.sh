#!/usr/bin/env bash
set -euo pipefail
python3 workflows/shared/launch.py workflows/ci/workflow-contracts/config.yml -- --suite workflows/build/flatpak/tests
python3 workflows/build/flatpak/assets/scripts/prepare-package-tools.py "$RUNNER_TEMP/package-tools" \
  --cache "$RUNNER_TEMP/package-downloads" --app "$RUNNER_TEMP/flatpak/app"
python3 -B workflows/catalog/compose/assets/scripts/ingest-pinned-stores.py --finalize-cockpit \
  --base-store "$RUNNER_TEMP/native-store" \
  --cockpit-store "$RUNNER_TEMP/flatpak-cockpit-runtime/store" \
  --cockpit-lock workflows/qualify/cockpit/assets/cockpit.lock.json --version "$DOCKPIPE_FLATPAK_VERSION" \
  --out "$RUNNER_TEMP/native-store-with-cockpit"
python3 workflows/build/flatpak/assets/scripts/build-catalog.py "$RUNNER_TEMP/flatpak/app" \
  "$RUNNER_TEMP/native-store-with-cockpit" "$RUNNER_TEMP/package-tools" "$RUNNER_TEMP/flatpak-catalog"
