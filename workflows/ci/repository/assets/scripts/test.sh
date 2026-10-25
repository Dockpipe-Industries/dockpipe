#!/usr/bin/env bash
set -euo pipefail
python3 workflows/ci/core/assets/scripts/test.py
./src/bin/dockpipe test --workdir .
