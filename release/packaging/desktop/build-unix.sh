#!/usr/bin/env bash
# Native Qt desktop build. Output stays outside the authored source tree.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
out="${2:?output directory}"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
if [[ -n "${DOCKPIPE_PRIVATE_DEBUG_DIR:-}" ]]; then
  private_debug="$(python3 - "$DOCKPIPE_PRIVATE_DEBUG_DIR" <<'PY'
from pathlib import Path
import sys

print(Path(sys.argv[1]).resolve())
PY
  )"
  [[ "$private_debug/" != "$out/"* ]] || {
    echo 'Private build diagnostics must be outside release artifacts.' >&2
    exit 1
  }
  mkdir -p "$private_debug"
  build="$(mktemp -d "$private_debug/native-build.XXXXXX")"
else
  build="$(mktemp -d)"
fi
cleanup_build() {
  local status=$?
  if [[ "$status" == 0 || -z "${DOCKPIPE_PRIVATE_DEBUG_DIR:-}" ]]; then
    rm -rf "$build"
  else
    echo "Build failed; private diagnostics retained at $build" >&2
  fi
}
trap cleanup_build EXIT
browser_sdk="$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).resolve(strict=True))' "${DOCKPIPE_BROWSER_SDK:?Prepare the exact public Browser SDK first}")"
case "$(uname -s):$(uname -m)" in
  Linux:x86_64) browser_platform=linux-amd64; qt_kit=gcc_64 ;;
  Linux:aarch64|Linux:arm64) browser_platform=linux-arm64; qt_kit=gcc_arm64 ;;
  Darwin:x86_64) browser_platform=darwin-amd64 ;;
  Darwin:arm64) browser_platform=darwin-arm64 ;;
  *) echo 'Unsupported native Browser SDK platform' >&2; exit 1 ;;
esac
export DOCKPIPE_BROWSER_SDK="$browser_sdk"
python3 "$root/release/packaging/desktop/browser-runtime.py" --sdk "$browser_sdk" --platform "$browser_platform"
args=(-S "$root/src/app/tooling/dockpipe-launcher" -B "$build" -DCMAKE_BUILD_TYPE=Release
      "-DDOCKPIPE_RELEASE_VERSION=$version")
prefixes="$browser_sdk${CMAKE_PREFIX_PATH:+;$CMAKE_PREFIX_PATH}"
if [[ "$(uname -s)" == Darwin ]]; then
  args+=("-DCMAKE_OSX_ARCHITECTURES=$(uname -m)" -DCMAKE_OSX_DEPLOYMENT_TARGET=13.0)
else
  export DOCKPIPE_QT_ROOT="${DOCKPIPE_QT_ROOT:-/opt/Qt/6.10.3/$qt_kit}"
  [[ "$("$DOCKPIPE_QT_ROOT/bin/qmake" -query QT_VERSION)" == 6.10.3 ]]
  prefixes="$browser_sdk;$DOCKPIPE_QT_ROOT${CMAKE_PREFIX_PATH:+;$CMAKE_PREFIX_PATH}"
  args+=(
         "-DCMAKE_CXX_FLAGS_RELEASE=-O2 -g -DNDEBUG -ffile-prefix-map=$root=. -ffile-prefix-map=$build=.")
fi
args+=("-DCMAKE_PREFIX_PATH=$prefixes")
if [[ "${DOCKPIPE_QUALIFY_COCKPIT:-0}" == 1 ]]; then
  [[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]]
  export DOCKPIPE_GIT_TEST_CLI="$root/src/bin/dockpipe"
  [[ -x "$DOCKPIPE_GIT_TEST_CLI" ]]
  python3 "$root/release/packaging/prepare-cockpit.py" verify-store \
    --lock "$root/release/cockpit.lock.json" --store "${DOCKPIPE_COCKPIT_STORE:?Verified Cockpit store required}"
  assets="$(python3 "$root/release/packaging/prepare-cockpit.py" extract \
    --lock "$root/release/cockpit.lock.json" --store "$DOCKPIPE_COCKPIT_STORE" --output "$build/cockpit-runtime")"
  args+=("-DDOCKPIPE_COCKPIT_ASSETS=$assets")
fi
cmake "${args[@]}"
if [[ "${DOCKPIPE_QUALIFY_COCKPIT:-0}" == 1 ]]; then
  python3 "$root/release/packaging/prepare-cockpit.py" verify-compiler \
    --lock "$root/release/cockpit.lock.json" --store "$DOCKPIPE_COCKPIT_STORE" --build "$build"
fi
cmake --build "$build" --parallel 2
ctest --test-dir "$build" --output-on-failure
if [[ "${DOCKPIPE_QUALIFY_COCKPIT:-0}" == 1 ]]; then
  [[ ! -e "$out/cockpit-store" ]]
  cp -R "$DOCKPIPE_COCKPIT_STORE" "$out/cockpit-store"
fi
if [[ "$(uname -s)" == Darwin ]]; then
  bash "$root/release/packaging/desktop/macos/package.sh" "$version" "$build/DockPipe.app" "$out"
else
  bash "$root/release/packaging/desktop/build-deb.sh" "$version" "$build/dockpipe-launcher" "$out"
fi
