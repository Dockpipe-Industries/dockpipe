#!/usr/bin/env bash
set -euo pipefail
python3 workflows/publish/r2/assets/scripts/publish-r2.py --artifacts release/artifacts --apt release/apt --version "${DOCKPIPE_ADAPTER_0}"
