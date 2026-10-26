#!/usr/bin/env bash
set -euo pipefail
set -euo pipefail
export GNUPGHOME="$(mktemp -d)"
trap 'rm -rf "$GNUPGHOME"' EXIT
chmod 700 "$GNUPGHOME"
printf '%s' "$APT_SIGNING_KEY" | gpg --batch --import
bash workflows/publish/apt/assets/scripts/build-apt.sh release/artifacts release/apt
