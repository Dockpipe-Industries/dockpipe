#!/usr/bin/env bash
# Keep the DEB ABI baseline independent of the CI host's Ubuntu version.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
out="${2:?output directory}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
image="dockpipe-desktop-builder:ubuntu22.04"
docker build --file "$root/release/packaging/desktop/linux-builder.Dockerfile" \
  --tag "$image" "$root/release/packaging/desktop"
docker run --rm --user "$(id -u):$(id -g)" \
  --env HOME=/tmp --env QT_QPA_PLATFORM=offscreen \
  --mount "type=bind,source=$root,target=/source,readonly" \
  --mount "type=bind,source=$out,target=/artifacts" \
  "$image" bash /source/release/packaging/desktop/build-unix.sh "$version" /artifacts
