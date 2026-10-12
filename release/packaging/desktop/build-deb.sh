#!/usr/bin/env bash
# Desktop is an optional package; headless workers install only dockpipe.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
version="${1:?version}"
launcher="$(realpath "${2:?native launcher}")"
out="$(realpath "${3:?output directory}")"
arch="$(dpkg --print-architecture)"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/root/usr/bin" "$stage/root/usr/share/applications" "$stage/root/usr/share/icons" "$stage/root/DEBIAN" "$stage/debian"
mkdir -p "$stage/root/usr/share/doc/dockpipe-desktop"
cp "$root/LICENSE" "$stage/root/usr/share/doc/dockpipe-desktop/copyright"
install -m 755 "$launcher" "$stage/root/usr/bin/dockpipe-launcher"
cp "$root/release/packaging/desktop/dockpipe-launcher.desktop" "$stage/root/usr/share/applications/"
cp -R "$root/src/app/tooling/dockpipe-launcher/resources/icons/hicolor" "$stage/root/usr/share/icons/"
# Let the native distribution determine ABI constraints rather than guessing Qt names/versions.
printf 'Source: dockpipe-desktop\nMaintainer: dockpipe maintainers\n\nPackage: dockpipe-desktop\nArchitecture: any\n' > "$stage/debian/control"
dependencies="$(cd "$stage" && dpkg-shlibdeps -O "$stage/root/usr/bin/dockpipe-launcher")"
dependencies="${dependencies#shlibs:Depends=}"
cat > "$stage/root/DEBIAN/control" <<EOF
Package: dockpipe-desktop
Version: $version
Architecture: $arch
Maintainer: dockpipe maintainers
Section: devel
Priority: optional
Depends: dockpipe (= $version), $dependencies, qt6-qpa-plugins
Description: Dockpipe desktop launcher
 Desktop launcher and application menu entry for Dockpipe workflows.
EOF
dpkg-deb --root-owner-group --build "$stage/root" "$out/dockpipe-desktop_${version}_${arch}.deb"
