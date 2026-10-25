#!/usr/bin/env bash
# Qualify every Go package owned by the standalone Dockpipe runtime repository.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
cd "$root"
python3 workflows/ci/core/assets/scripts/test.py
