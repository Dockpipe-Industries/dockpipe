#!/usr/bin/env bash
# Real dependency resolution and startup in clean runtime-only environments.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
out="$(realpath "${2:-$root/release/artifacts}")"
for distribution in 22.04 24.04; do
  docker run --rm --env DEBIAN_FRONTEND=noninteractive \
    --mount "type=bind,source=$out,target=/artifacts,readonly" \
    --mount "type=bind,source=$root/release/packaging/desktop,target=/checks,readonly" \
    --mount "type=bind,source=$root/release/packaging/tests/native-smoke.py,target=/native-smoke.py,readonly" \
    "ubuntu:$distribution" bash -c '
      set -euo pipefail
      version="$1"
      arch="$(dpkg --print-architecture)"
      apt-get update
      apt-get install -y --no-install-recommends python3 \
        "/artifacts/dockpipe_${version}_${arch}.deb" \
        "/artifacts/dockpipe-desktop_${version}_${arch}.deb"
      test -f /usr/share/applications/dockpipe-launcher.desktop
      test -f /usr/share/icons/hicolor/256x256/apps/dockpipe-launcher.png
      python3 /checks/smoke.py /usr/bin/dockpipe-launcher /usr/bin/dockpipe
      python3 /native-smoke.py /usr/bin/dockpipe
      apt-get remove -y dockpipe-desktop
      test ! -e /usr/bin/dockpipe-launcher
      /usr/bin/dockpipe --version
    ' bash "$version"
done
