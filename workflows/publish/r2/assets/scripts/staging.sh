#!/usr/bin/env bash
set -euo pipefail
python3 workflows/publish/r2/assets/scripts/publish-r2.py --artifacts release/artifacts --apt release/apt --version "${DOCKPIPE_ADAPTER_0}" --candidate "${DOCKPIPE_ADAPTER_1}"

  # Optional: dev.to — PUT main article (DEVTO_ARTICLE_ID) + optional one-time POST per release (DEVTO_ONE_TIME_POST).
  # See release/docs/devto.md
