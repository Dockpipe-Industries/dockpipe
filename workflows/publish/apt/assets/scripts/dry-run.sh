#!/usr/bin/env bash
set -euo pipefail
set -euo pipefail
export GNUPGHOME="$(mktemp -d)"
trap 'rm -rf "$GNUPGHOME"' EXIT
chmod 700 "$GNUPGHOME"
gpg --batch --passphrase '' --quick-generate-key 'Dockpipe dry-run (not for publication)' rsa3072 sign 1d
export APT_SIGNING_FINGERPRINT="$(gpg --with-colons --list-secret-keys | awk -F: '$1 == "fpr" {print $10; exit}')"
bash workflows/publish/apt/assets/scripts/build-apt.sh release/artifacts release/apt
