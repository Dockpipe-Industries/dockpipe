#!/usr/bin/env bash
set -euo pipefail
qualification_args=()
if [[ "$QUALIFY_PACKAGE_DISPLAY" == true ]]; then
  qualification_args+=(--enable-qualification)
fi
bash workflows/build/desktop/assets/scripts/build-linux.sh "${qualification_args[@]}" "${DOCKPIPE_ADAPTER_0}" release/artifacts
