#!/usr/bin/env bash
set -euo pipefail
cp workflows/install/distribution/assets/scripts/install.sh release/artifacts/install.sh
cp workflows/install/windows/assets/scripts/install.ps1 release/artifacts/install.ps1
python3 workflows/release/metadata/assets/scripts/release-artifacts.py prepare release/artifacts --version "${DOCKPIPE_ADAPTER_0}" --candidate "${DOCKPIPE_ADAPTER_1}" --source-sha "${DOCKPIPE_ADAPTER_2}"
