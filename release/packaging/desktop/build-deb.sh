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
syntax_dir="$(dirname "$launcher")"
mkdir -p "$stage/root/usr/lib/dockpipe"
install -D -m644 "$syntax_dir/dockpipe-syntax.rcc" "$stage/root/usr/share/dockpipe/launcher/dockpipe-syntax.rcc"
cp -R "$syntax_dir/syntax-licenses" "$stage/root/usr/share/dockpipe/launcher/"
cp "$root/release/packaging/desktop/dockpipe-launcher.desktop" "$stage/root/usr/share/applications/"
cp -R "$root/src/app/tooling/dockpipe-launcher/resources/icons/hicolor" "$stage/root/usr/share/icons/"
# Let the native distribution determine ABI constraints rather than guessing Qt names/versions.
printf 'Source: dockpipe-desktop\nMaintainer: dockpipe maintainers\n\nPackage: dockpipe-desktop\nArchitecture: any\n' > "$stage/debian/control"
if [[ "$arch" == amd64 ]]; then
  qt_root="${DOCKPIPE_QT_ROOT:?Set DOCKPIPE_QT_ROOT to the matching Qt 6.10.3 SDK}"
  debug_out="${DOCKPIPE_PRIVATE_DEBUG_DIR:-${out}.private-debug}"
  mkdir -p "$debug_out"
  debug_out="$(realpath "$debug_out")"
  [[ "$debug_out/" != "$out/"* ]] || { echo 'Private debug output must be outside release artifacts.' >&2; exit 1; }
  python3 "$root/release/packaging/desktop/bundle-linux-qt.py" \
    --launcher "$launcher" --qt "$qt_root" --stage "$stage/root" \
    --private-debug "$debug_out/$version-$arch" --version "$version" > "$stage/binaries"
  cp "$stage/root/DEBIAN/shlibs" "$stage/debian/shlibs.local"
  mapfile -t binaries < "$stage/binaries"
  dependencies="$(cd "$stage" && dpkg-shlibdeps -O -xdockpipe-desktop \
    -l"$stage/root/usr/lib/dockpipe/lib" "${binaries[@]}")"
  # Qt's OpenSSL backend and Cockpit's SDL audio/controller support load these
  # libraries at runtime; ldd cannot discover them.
  qt_dependencies=", libssl3, ca-certificates, libsdl2-2.0-0"
else
  install -m 755 "$launcher" "$stage/root/usr/bin/dockpipe-launcher"
  cp -a "$syntax_dir"/libdockpipe-launcher-ui.so* "$stage/root/usr/lib/dockpipe/"
  for binary in "$stage/root/usr/bin/dockpipe-launcher" "$stage/root/usr/lib/dockpipe"/libdockpipe-launcher-ui.so*; do
    strip --strip-unneeded "$binary"
    if readelf --wide --sections "$binary" | grep -E '\.(z?debug[_.]|symtab[[:space:]]|gnu_debugdata[[:space:]]|gdb_index[[:space:]])' >/dev/null; then
      echo "Private symbol sections remain in public desktop binary: $binary" >&2
      exit 1
    fi
  done
  printf 'libdockpipe-launcher-ui 1 dockpipe-desktop (= %s)\n' "$version" > "$stage/debian/shlibs.local"
  dependencies="$(cd "$stage" && dpkg-shlibdeps -O -xdockpipe-desktop -l"$stage/root/usr/lib/dockpipe" "$stage/root/usr/bin/dockpipe-launcher" "$stage/root/usr/lib/dockpipe/libdockpipe-launcher-ui.so.1")"
  qt_dependencies=", qt6-qpa-plugins, libqt6webenginecore6-bin"
fi
dependencies="${dependencies#shlibs:Depends=}"
cat > "$stage/root/DEBIAN/control" <<EOF
Package: dockpipe-desktop
Version: $version
Architecture: $arch
Maintainer: dockpipe maintainers
Section: devel
Priority: optional
Depends: dockpipe (= $version), $dependencies$qt_dependencies
Description: Dockpipe desktop launcher
 Desktop launcher and application menu entry for Dockpipe workflows.
EOF
dpkg-deb --root-owner-group --build "$stage/root" "$out/dockpipe-desktop_${version}_${arch}.deb"
