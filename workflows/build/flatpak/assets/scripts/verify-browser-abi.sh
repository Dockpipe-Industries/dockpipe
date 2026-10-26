#!/usr/bin/env bash
# Run inside the selected KDE SDK before compiling a consumer of the binary SDK.
set -euo pipefail
browser_sdk="${1:?Verified Browser SDK prefix}"
[[ "$(qmake6 -query QT_VERSION)" == 6.10.3 ]] || {
  echo 'KDE Core differs from the separately qualified Flatpak Browser SDK 6.10.3 contract.' >&2
  exit 1
}
# BaseApp ships CMake packages; it need not expose WebEngine pkg-config files.
# Require the exact per-component contract qualified by Spatial-Web. The temporary project contains no Spatial-Web source.
probe="$(mktemp -d)"
trap 'rm -rf "$probe"' EXIT
cat > "$probe/CMakeLists.txt" <<'CMAKE'
cmake_minimum_required(VERSION 3.22)
project(BrowserSDKABI LANGUAGES CXX)
foreach(component IN ITEMS Core Gui Widgets Network PrintSupport)
    find_package(Qt6${component} 6.10.3 EXACT REQUIRED CONFIG)
endforeach()
foreach(component IN ITEMS WebEngineCore WebEngineWidgets)
    find_package(Qt6${component} 6.10.2 EXACT REQUIRED CONFIG)
endforeach()
if(NOT CMAKE_CXX_COMPILER_ID STREQUAL "GNU" OR NOT CMAKE_CXX_COMPILER_VERSION STREQUAL "15.2.0")
    message(FATAL_ERROR "Flatpak Browser SDK requires GNU 15.2.0")
endif()
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
printf 'Flatpak Core 6.10.3 / WebEngine 6.10.2 exact ABI preflight passed; compiler: '
g++ -dumpfullversion
