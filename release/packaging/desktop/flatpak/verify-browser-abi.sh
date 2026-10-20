#!/usr/bin/env bash
# Run inside the selected KDE SDK before compiling a consumer of the binary SDK.
set -euo pipefail
browser_sdk="${1:?Verified Browser SDK prefix}"
[[ "$(qmake6 -query QT_VERSION)" == 6.10.3 ]] || {
  echo 'KDE SDK Qt patch differs from Browser SDK 6.10.3. Qualify a matching SDK in Spatial-Web; do not embed its source here.' >&2
  exit 1
}
# BaseApp ships CMake packages; it need not expose WebEngine pkg-config files.
# Require the actual installed packages with the same exact patch contract as
# the application. The temporary project contains no Spatial-Web source.
probe="$(mktemp -d)"
trap 'rm -rf "$probe"' EXIT
cat > "$probe/CMakeLists.txt" <<'CMAKE'
cmake_minimum_required(VERSION 3.22)
project(BrowserSDKABI LANGUAGES CXX)
find_package(Qt6Core 6.10.3 EXACT REQUIRED CONFIG)
find_package(Qt6WebEngineCore 6.10.3 EXACT REQUIRED CONFIG)
message(STATUS "KDE/BaseApp Qt Core ${Qt6Core_VERSION}; WebEngine Core ${Qt6WebEngineCore_VERSION}")
CMAKE
cmake -S "$probe" -B "$probe/build" -DCMAKE_PREFIX_PATH=/app
browser="$browser_sdk/lib/libSpatialWebBrowser.so.0.1.0"
[[ -f "$browser" ]] || { echo 'Missing Browser SDK shared library' >&2; exit 1; }
dependencies="$(ldd "$browser")"
if [[ "$dependencies" == *'not found'* ]]; then
  echo 'Browser SDK cannot resolve its dependencies in this KDE SDK/BaseApp' >&2
  exit 1
fi
# Resolve both the SDK and runtime Qt ABI rather than assuming the 6.10 branch is a patch version.
printf '%s\n' "$dependencies" | grep -F 'libQt6Core.so.6'
printf '%s\n' "$dependencies" | grep -F 'libQt6WebEngineCore.so.6'
printf 'Qt 6.10.3 Browser binary dependency preflight passed; compiler: '
g++ -dumpfullversion
