#!/usr/bin/env bash
set -euo pipefail
exec "${DOCKPIPE_BIN:?Select the pinned released Dockpipe CLI}" --workflow-file workflows/publish/r2/staging.yml --workdir "$PWD"
