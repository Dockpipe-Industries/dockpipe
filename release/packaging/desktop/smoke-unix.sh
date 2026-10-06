#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
out="$root/release/artifacts"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
if [[ "$(uname -s)" == Darwin ]]; then
  [[ "${GITHUB_ACTIONS:-}" == true ]] || { echo 'Installer smoke requires a disposable CI runner' >&2; exit 1; }
  case "$(uname -m)" in arm64) arch=arm64 ;; x86_64) arch=amd64 ;; esac
  mkdir "$stage/mount"
  trap 'hdiutil detach "$stage/mount" >/dev/null 2>&1 || true; rm -rf "$stage"' EXIT
  hdiutil attach "$out/dockpipe-desktop_${version}_darwin_${arch}.dmg" -nobrowse -mountpoint "$stage/mount"
  # Native hosted runners are disposable. Exercise Apple's real Installer and CLI link.
  sudo installer -pkg "$stage/mount/Install DockPipe.pkg" -target /
  python3 "$root/release/packaging/tests/native-smoke.py" /usr/local/bin/dockpipe
  python3 "$root/release/packaging/desktop/smoke.py" /Applications/DockPipe.app/Contents/MacOS/DockPipe /Applications/DockPipe.app/Contents/MacOS/dockpipe-cli
  test "$(readlink /usr/local/bin/dockpipe)" = /Applications/DockPipe.app/Contents/MacOS/dockpipe-cli
  # Verify managed updates are accepted too.
  sudo installer -pkg "$stage/mount/Install DockPipe.pkg" -target /
  sudo rm /usr/local/bin/dockpipe
  sudo rm -rf /Applications/DockPipe.app
  sudo pkgutil --forget com.dockpipe.desktop
else
  arch="$(dpkg --print-architecture)"
  dpkg-deb -x "$out/dockpipe-desktop_${version}_${arch}.deb" "$stage"
  dpkg-deb -x "$out/dockpipe_${version}_${arch}.deb" "$stage"
  test -f "$stage/usr/share/applications/dockpipe-launcher.desktop"
  test -f "$stage/usr/share/icons/hicolor/256x256/apps/dockpipe-launcher.png"
  python3 "$root/release/packaging/desktop/smoke.py" "$stage/usr/bin/dockpipe-launcher" "$stage/usr/bin/dockpipe"
fi
