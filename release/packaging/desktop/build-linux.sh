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
case "$(uname -m)" in
  x86_64) ubuntu_version=22.04; browser_platform=linux-amd64 ;;
  aarch64|arm64) ubuntu_version=24.04; browser_platform=linux-arm64 ;;
  *) echo 'Unsupported Linux desktop architecture' >&2; exit 1 ;;
esac
browser_sdk="$(realpath "${DOCKPIPE_BROWSER_SDK:?Prepare the exact public Browser SDK first}")"
python3 "$root/release/packaging/desktop/browser-runtime.py" --sdk "$browser_sdk" --platform "$browser_platform"
image="dockpipe-desktop-builder:ubuntu${ubuntu_version}-qt6.10.3"
docker build --file "$root/release/packaging/desktop/linux-builder.Dockerfile" \
  --build-arg "UBUNTU_VERSION=$ubuntu_version" --tag "$image" "$root/release/packaging/desktop"
package_args=()
if [[ "${DOCKPIPE_QUALIFY_COCKPIT:-0}" == 1 ]]; then
  [[ "$(uname -m)" == x86_64 ]] || { echo 'Cockpit qualification requires Linux amd64.' >&2; exit 1; }
  prepared="$(realpath "${DOCKPIPE_COCKPIT_PREPARED:?Prepare the pinned public Cockpit runtime first}")"
  python3 "$root/release/packaging/prepare-cockpit.py" verify-store \
    --lock "$root/release/cockpit.lock.json" --store "$prepared/store"
  package_args+=(--env DOCKPIPE_QUALIFY_COCKPIT=1
    --env DOCKPIPE_COCKPIT_STORE=/cockpit-input/store
    --mount "type=bind,source=$prepared,target=/cockpit-input,readonly")
fi
docker run --rm --user "$(id -u):$(id -g)" \
  --env HOME=/tmp --env QT_QPA_PLATFORM=offscreen \
  --env QTWEBENGINE_DISABLE_SANDBOX=1 \
  --env DOCKPIPE_PRIVATE_DEBUG_DIR=/private-debug \
  --env DOCKPIPE_BROWSER_SDK=/browser-sdk \
  --mount "type=bind,source=$browser_sdk,target=/browser-sdk,readonly" \
  --mount "type=bind,source=$root,target=/source,readonly" \
  --mount "type=bind,source=$out,target=/artifacts" \
  --mount "type=bind,source=$debug_out,target=/private-debug" \
  "${package_args[@]}" \
  "$image" bash /source/release/packaging/desktop/build-unix.sh "$version" /artifacts
