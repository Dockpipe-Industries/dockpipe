#!/usr/bin/env bash
# Exact native Qt kit shared by the desktop and the matching Browser SDK.
set -euo pipefail
destination="${1:?Qt installation directory}"
[[ "$(uname -s)" == Linux ]] || { echo 'Expected Linux Qt toolchain host' >&2; exit 1; }
case "$(uname -m)" in
  x86_64) qt_host=linux; qt_arch=linux_gcc_64; qt_kit=gcc_64 ;;
  aarch64|arm64) qt_host=linux_arm64; qt_arch=linux_gcc_arm64; qt_kit=gcc_arm64 ;;
  *) echo 'Unsupported Linux Qt architecture' >&2; exit 1 ;;
esac
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
qt_root="$destination/6.10.3/$qt_kit"
if [[ ! -x "$qt_root/bin/qmake" ]]; then
  python3 -m venv "$destination/aqt"
  "$destination/aqt/bin/python" -m pip install --disable-pip-version-check aqtinstall==3.3.0
  "$destination/aqt/bin/python" -m aqt install-qt "$qt_host" desktop 6.10.3 "$qt_arch" \
    --outputdir "$destination" --modules qtwebengine qtwebchannel qtpositioning qtimageformats qtshadertools qtmultimedia
fi
[[ "$("$qt_root/bin/qmake" -query QT_VERSION)" == 6.10.3 ]]
[[ -x "$qt_root/libexec/QtWebEngineProcess" ]]
[[ -f "$qt_root/resources/qtwebengine_resources.pak" ]]
printf '%s\n' "$qt_root"
