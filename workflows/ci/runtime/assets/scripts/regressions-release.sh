#!/usr/bin/env bash
set -euo pipefail
sudo apt-get update
sudo apt-get install -y --no-install-recommends ripgrep
bash workflows/ci/runtime/assets/scripts/test-runtime.sh
bash tests/run_tests.sh
