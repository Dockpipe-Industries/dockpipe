#!/usr/bin/env bash
set -euo pipefail
sudo apt-get update
sudo apt-get install -y --no-install-recommends ripgrep
python3 workflows/ci/core/assets/scripts/test.py
bash tests/run_tests.sh
