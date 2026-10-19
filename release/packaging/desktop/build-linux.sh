#!/usr/bin/env bash
# Keep the DEB ABI baseline independent of the CI host's Ubuntu version.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
out="${2:?output directory}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
debug_out="${DOCKPIPE_PRIVATE_DEBUG_DIR:-${out}.private-debug}"
mkdir -p "$debug_out"
debug_out="$(cd "$debug_out" && pwd)"
[[ "$debug_out/" != "$out/"* ]] || { echo 'Private debug output must be outside release artifacts.' >&2; exit 1; }
image="dockpipe-desktop-builder:ubuntu22.04-qt6.10.3"
docker build --file "$root/release/packaging/desktop/linux-builder.Dockerfile" \
  --tag "$image" "$root/release/packaging/desktop"
package_args=()
if [[ "${DOCKPIPE_BUILD_COCKPIT:-0}" == 1 ]]; then
  [[ "$(uname -m)" == x86_64 ]] || { echo 'Native Cockpit SDK packaging is qualified on Linux amd64 only.' >&2; exit 1; }
  sdk="$(realpath "${DOCKPIPE_FORGEPIPE_SDK:?Prepare and attest the native ForgePipe SDK before packaging}")"
  [[ "${DOCKPIPE_RELEASE_SOURCE_SHA:-}" =~ ^[0-9a-f]{40}$ ]] || {
    echo 'All-inclusive releases require the exact 40-character source commit SHA.' >&2
    exit 1
  }
  [[ -f "$sdk/lib/cmake/ForgePipe/ForgePipeConfig.cmake" ]]
  [[ -x "$root/src/bin/dockpipe" ]]
  package_args+=(--env DOCKPIPE_BUILD_COCKPIT=1 --env DOCKPIPE_FORGEPIPE_SDK=/forgepipe-sdk
    --env "DOCKPIPE_RELEASE_SOURCE_SHA=$DOCKPIPE_RELEASE_SOURCE_SHA"
    --mount "type=bind,source=$sdk,target=/forgepipe-sdk,readonly")
fi
docker run --rm --user "$(id -u):$(id -g)" \
  --env HOME=/tmp --env QT_QPA_PLATFORM=offscreen \
  --env QTWEBENGINE_DISABLE_SANDBOX=1 \
  --env DOCKPIPE_PRIVATE_DEBUG_DIR=/private-debug \
  --mount "type=bind,source=$root,target=/source,readonly" \
  --mount "type=bind,source=$out,target=/artifacts" \
  --mount "type=bind,source=$debug_out,target=/private-debug" \
  "${package_args[@]}" \
  "$image" bash /source/release/packaging/desktop/build-unix.sh "$version" /artifacts
