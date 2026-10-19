#!/usr/bin/env bash
# Real dependency resolution and startup in clean runtime-only environments.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
out="$(realpath "${2:-$root/release/artifacts}")"
probe="$(mktemp -d)"
trap 'rm -rf "$probe"' EXIT
if [[ "$(uname -m)" == x86_64 ]]; then
  # This private qualification executable is never placed in the release directory.
  docker run --rm --user "$(id -u):$(id -g)" \
    --mount "type=bind,source=$root/release/packaging/desktop,target=/checks,readonly" \
    --mount "type=bind,source=$probe,target=/probe" \
    dockpipe-desktop-builder:ubuntu22.04-qt6.10.3 bash -c '
      set -euo pipefail
      cd /probe
      printf "%s\n" "QT += webenginewidgets" "CONFIG += c++17 release" \
        "SOURCES = /checks/linux-browser-smoke.cpp" "TARGET = browser-smoke" \
        "QMAKE_RPATHDIR = /usr/lib/dockpipe/lib" > browser-smoke.pro
      /opt/Qt/6.10.3/gcc_64/bin/qmake browser-smoke.pro
      make -j2
    '
fi
for distribution in 22.04 24.04; do
  docker run --rm --env DEBIAN_FRONTEND=noninteractive \
    --mount "type=bind,source=$out,target=/artifacts,readonly" \
    --mount "type=bind,source=$root/release/packaging/desktop,target=/checks,readonly" \
    --mount "type=bind,source=$root/release/packaging/tests/native-smoke.py,target=/native-smoke.py,readonly" \
    --mount "type=bind,source=$probe,target=/probe,readonly" \
    "ubuntu:$distribution" bash -c '
      set -euo pipefail
      version="$1"
      arch="$(dpkg --print-architecture)"
      # Minimal container images discard documentation by default. Real package
      # installations retain these notices; this test must inspect them too.
      sed -i "\\|^path-exclude=/usr/share/doc/|d" /etc/dpkg/dpkg.cfg.d/*
      apt-get update
      apt-get install -y --no-install-recommends python3 binutils \
        "/artifacts/dockpipe_${version}_${arch}.deb" \
        "/artifacts/dockpipe-desktop_${version}_${arch}.deb"
      test -f /usr/share/applications/dockpipe-launcher.desktop
      test -f /usr/share/icons/hicolor/256x256/apps/dockpipe-launcher.png
      if [[ "$arch" == amd64 ]]; then
        python3 /checks/check-linux-runtime.py /usr/lib/dockpipe
        ! dpkg-query -W -f="\${Depends}" dockpipe-desktop | grep -E "libqt6|qt6-qpa"
        test -f /usr/share/doc/dockpipe-desktop/qt/Qt-LGPL-3.txt
        test -f /usr/share/doc/dockpipe-desktop/qt/qtwebengine-6.10.3.spdx.json
        QT_QPA_PLATFORM=offscreen QT_PLUGIN_PATH=/usr/lib/dockpipe/plugins \
          QTWEBENGINEPROCESS_PATH=/usr/lib/dockpipe/libexec/QtWebEngineProcess \
          QTWEBENGINE_RESOURCES_PATH=/usr/lib/dockpipe/resources \
          QTWEBENGINE_LOCALES_PATH=/usr/lib/dockpipe/translations/qtwebengine_locales \
          QTWEBENGINE_CHROMIUM_FLAGS="--disable-gpu --disable-dev-shm-usage" \
          QTWEBENGINE_DISABLE_SANDBOX=1 /probe/browser-smoke
      fi
      python3 /checks/smoke.py /usr/bin/dockpipe-launcher /usr/bin/dockpipe
      python3 /native-smoke.py /usr/bin/dockpipe
      apt-get remove -y dockpipe-desktop
      test ! -e /usr/bin/dockpipe-launcher
      /usr/bin/dockpipe --version
    ' bash "$version"
done
