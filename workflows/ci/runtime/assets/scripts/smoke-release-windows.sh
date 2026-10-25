#!/usr/bin/env bash
set -euo pipefail
python3 workflows/ci/runtime/assets/scripts/native-smoke.py src/bin/dockpipe.exe
