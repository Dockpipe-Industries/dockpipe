#!/usr/bin/env bash
set -euo pipefail
python3 -m unittest discover -s workflows/build/desktop/tests -p test_native_symbol_boundaries.py -v
set --
if [[ "$QUALIFY_PACKAGE_DISPLAY" == true ]]; then
  set -- "$@" --enable-qualification
fi
if [[ "$DIAGNOSE_LAYOUT" == true ]]; then
  set -- "$@" --diagnose-layout "$PRODUCT_SOURCE"
fi
bash workflows/build/desktop/assets/scripts/build-unix.sh "$@" "${DOCKPIPE_ADAPTER_0}" release/artifacts
