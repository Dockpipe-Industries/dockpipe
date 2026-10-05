#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
args=()
if [[ "${RELEASE_SETUP_APPLY:-0}" == 1 ]]; then
  args+=(--apply)
fi
exec python3 "$script_dir/configure-release.py" "${args[@]}"
