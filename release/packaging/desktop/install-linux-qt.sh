#!/usr/bin/env bash
# Shared native amd64 toolchain for the launcher and the first ForgePipe SDK.
set -euo pipefail
destination="${1:?Qt installation directory}"
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || {
  echo 'The qualified Qt 6.10.3 native SDK toolchain is Linux amd64 only.' >&2
  exit 1
}
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
qt_root="$destination/6.10.3/gcc_64"
if [[ ! -x "$qt_root/bin/qmake" ]]; then
  python3 -m venv "$destination/aqt"
  "$destination/aqt/bin/python" -m pip install --disable-pip-version-check aqtinstall==3.3.0
  "$destination/aqt/bin/python" -m aqt install-qt linux desktop 6.10.3 linux_gcc_64 \
    --outputdir "$destination" --modules qtwebengine qtwebchannel qtpositioning qtimageformats qtshadertools qtmultimedia
fi
[[ "$("$qt_root/bin/qmake" -query QT_VERSION)" == 6.10.3 ]]
[[ -x "$qt_root/libexec/QtWebEngineProcess" ]]
[[ -f "$qt_root/resources/qtwebengine_resources.pak" ]]
printf '%s\n' "$qt_root"
