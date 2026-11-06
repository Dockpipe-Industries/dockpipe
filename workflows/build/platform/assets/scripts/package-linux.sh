#!/usr/bin/env bash
# Package the explicit platform CLI on supported Linux builders only.
set -euo pipefail
[[ "$PLATFORM_SYSTEM" == linux ]] || exit 0
export DOCKPIPE_RELEASE_BINARY="$PLATFORM_BINARY"
bash workflows/package/linux-cli/assets/scripts/build-deb.sh "$PLATFORM_VERSION" "$PLATFORM_ARCH" "$PLATFORM_OUTPUT"
bash workflows/package/linux-cli/assets/scripts/build-nfpm.sh "$PLATFORM_VERSION" "$PLATFORM_OUTPUT" "$PLATFORM_ARCH"
python3 workflows/package/synology/assets/scripts/build.py "$PLATFORM_VERSION" "$PLATFORM_ARCH" \
  --binary "$PLATFORM_BINARY" --output "$PLATFORM_OUTPUT"
