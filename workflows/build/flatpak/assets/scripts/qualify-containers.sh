#!/usr/bin/env bash
set -euo pipefail
docker pull debian:12
python3 workflows/build/flatpak/assets/scripts/container-smoke.py "$RUNNER_TEMP/flatpak/app"
