#!/usr/bin/env bash
# Build the desktop against its actual Flatpak SDK, then test with the Platform.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
version="${1:?numeric release version}"
cli="$(realpath "${2:?static native CLI}")"
store="${3:--}" # Explicit optional catalog store, or - for the standalone host.
out="${4:?fresh output directory}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Expected numeric release version' >&2; exit 1; }
[[ ! -e "$out" ]] || { echo 'Flatpak output directory must be new' >&2; exit 1; }
[[ "$("$cli" --version)" == "$version" ]] || { echo 'CLI version does not match release' >&2; exit 1; }
app_id=com.dockpipe.Dockpipe
runtime_branch=6.10
arch="$(flatpak --default-arch)"
case "$arch" in
  x86_64) goarch=amd64 ;;
  aarch64) goarch=arm64 ;;
  *) echo "Unsupported Flatpak architecture: $arch" >&2; exit 1 ;;
esac
browser_sdk="$(realpath "${DOCKPIPE_FLATPAK_BROWSER_SDK:?Prepare the exact qualified Flatpak Browser SDK first}")"
# Real optional-package fixtures are selected only by their integration lane.
# Generic package ABI/loader fixtures remain in the ordinary CTest suite.
cockpit_mounts=()
cockpit_options=()
if [[ -n "${DOCKPIPE_FLATPAK_COCKPIT_ASSETS:-}" ]]; then
  cockpit_assets="$(realpath "$DOCKPIPE_FLATPAK_COCKPIT_ASSETS")"
  if [[ ! -d "$cockpit_assets" ]]; then
    echo 'Selected Cockpit compatibility assets directory is missing' >&2
    exit 1
  fi
  cockpit_mounts+=("--filesystem=$cockpit_assets:ro")
  cockpit_options+=("-DDOCKPIPE_COCKPIT_ASSETS=$cockpit_assets")
fi
browser_platform="linux-$goarch-flatpak-kde-6.10"
browser_lock="$root/workflows/build/flatpak/assets/spatialweb-flatpak-browser.lock.json"
python3 -B "$root/workflows/build/flatpak/assets/scripts/prepare-flatpak-browser-sdk.py" --lock "$browser_lock" \
  --platform "$browser_platform" --verify-installed-refs
python3 -B "$root/workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py" --sdk "$browser_sdk" \
  --lock "$browser_lock" --profile flatpak --platform "$browser_platform"
# No implicit dependency installation or system changes in a release build.
flatpak info "org.kde.Sdk//$runtime_branch" >/dev/null
flatpak info "org.kde.Platform//$runtime_branch" >/dev/null
flatpak info "io.qt.qtwebengine.BaseApp//$runtime_branch" >/dev/null
mkdir -p "$out"
out="$(realpath "$out")"
app="$out/app"
flatpak build-init --base=io.qt.qtwebengine.BaseApp --base-version="$runtime_branch" \
  "$app" "$app_id" org.kde.Sdk org.kde.Platform "$runtime_branch"
flatpak build --filesystem="$browser_sdk:ro" --filesystem="$root/workflows/build/flatpak/assets:ro" "$app" \
  bash "$root/workflows/build/flatpak/assets/scripts/verify-browser-abi.sh" "$browser_sdk" > "$out/browser-abi.txt"
mkdir -p "$app/files/bin" "$app/files/libexec/dockpipe"
python3 "$root/workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py" --sdk "$browser_sdk" --lock "$browser_lock" --profile flatpak --platform "$browser_platform" \
  --runtime "$app/files/lib" --licenses "$app/files/share/licenses/SpatialWebBrowser"
install -m755 "$cli" "$app/files/libexec/dockpipe/dockpipe"
for command in dockpipe dockpipe-launcher; do
  install -m755 "$root/workflows/build/flatpak/assets/scripts/entrypoint.sh" "$app/files/bin/$command"
done
python3 "$root/workflows/build/flatpak/assets/scripts/prepare-container-tools.py" "$goarch" "$app/files" \
  --cache "${DOCKPIPE_FLATPAK_DOWNLOAD_CACHE:-$out/downloads}"
install -m755 "$root/workflows/build/flatpak/assets/scripts/docker.sh" "$app/files/bin/docker"
install -m644 "$root/workflows/build/flatpak/assets/scripts/container-environment.sh" "$app/files/libexec/dockpipe/container-environment.sh"
install -m755 "$root/workflows/build/flatpak/assets/scripts/host-command.sh" "$app/files/bin/dockpipe-host-command"
for command in op code cursor codex claude npm systemctl loginctl pkexec nvidia-smi nvidia-ctk qemu-img qemu-system-x86_64; do
  printf '#!/usr/bin/env bash\nexec /app/bin/dockpipe-host-command %s "$@"\n' "$command" > "$app/files/bin/$command"
  chmod 755 "$app/files/bin/$command"
done
flatpak build --filesystem="$root/workflows/build/flatpak/assets:ro" "$app" \
  bash "$root/workflows/build/flatpak/assets/scripts/stage-sdk-tools.sh"
if [[ "$store" != - ]]; then
  python3 "$root/workflows/release/metadata/assets/scripts/release-artifacts.py" stage-store "$store" \
    --destination "$app/files/share/dockpipe"
fi

source_dir="$root/src/app/tooling/dockpipe-launcher"
# The SDK omits libXres. Bundle it so clients without _NET_WM_PID can be
# identified by the same X11 ownership protocol as native desktop builds.
xres_archive="$out/libXres-1.2.3.tar.xz"
curl --fail --location --retry 3 --connect-timeout 15 --max-time 120 --output "$xres_archive" \
  https://xorg.freedesktop.org/archive/individual/lib/libXres-1.2.3.tar.xz
printf '%s  %s\n' d2de8f5401d6c86a8992791654547eb8def585dfdc0c08cc16e24ef6aeeb69dc "$xres_archive" | sha256sum --check
tar -xJf "$xres_archive" -C "$out"
flatpak build --filesystem="$out" "$app" \
  meson setup "$out/xres-build" "$out/libXres-1.2.3" --prefix=/app --libdir=lib -Ddefault_library=shared
flatpak build --filesystem="$out" "$app" meson compile -C "$out/xres-build" -j 2
flatpak build --filesystem="$out" "$app" meson install -C "$out/xres-build"
mkdir -p "$app/files/share/licenses/libXRes"
install -m644 "$out/libXres-1.2.3/COPYING" "$app/files/share/licenses/libXRes/COPYING"
flatpak build --filesystem="$source_dir:ro" --filesystem="$browser_sdk:ro" "${cockpit_mounts[@]}" --filesystem="$out" "$app" \
  cmake -S "$source_dir" -B "$out/qt-build" -DCMAKE_BUILD_TYPE=Release \
  "-DCMAKE_PREFIX_PATH=$browser_sdk;/app" \
  -DDOCKPIPE_BROWSER_ABI_PROFILE=flatpak-kde-6.10 \
  "${cockpit_options[@]}" \
  "-DDOCKPIPE_RELEASE_VERSION=$version"
flatpak build --filesystem="$source_dir:ro" --filesystem="$browser_sdk:ro" "${cockpit_mounts[@]}" --filesystem="$out" "$app" \
  cmake --build "$out/qt-build" --parallel 2
flatpak build --filesystem="$source_dir:ro" --filesystem="$out" --filesystem="$browser_sdk:ro" --env=QT_QPA_PLATFORM=offscreen \
  --env=QTWEBENGINEPROCESS_PATH=/app/bin/QtWebEngineProcess \
  --env=QT_QUICK_BACKEND=software --env=QTWEBENGINE_CHROMIUM_FLAGS=--disable-gpu \
  --env=DOCKPIPE_TEST_FLATPAK_HOST_NATIVE=0 \
  --env=QTWEBENGINE_DISABLE_SANDBOX=1 "$app" \
  ctest --test-dir "$out/qt-build" --output-on-failure --label-exclude installed-platform
# BaseApp cannot spawn nested Flatpak instances during a build. The environment
# above applies only to tests; installed browser processes retain sandboxing.
flatpak build --env=FLATPAK_DEST=/app --env="FLATPAK_ARCH=$arch" "$app" \
  bash /app/cleanup-BaseApp.sh
install -m755 "$out/qt-build/dockpipe-launcher" "$app/files/libexec/dockpipe/dockpipe-launcher"
cp -a "$out/qt-build"/libdockpipe-launcher-ui.so* "$app/files/libexec/dockpipe/"
install -m755 "$out/qt-build/dockpipe-workspace-reader" "$app/files/libexec/dockpipe/dockpipe-workspace-reader"
# Release compilation can still inherit private symbol/debug sections from
# linked objects. Enforce the public payload contract after deployment.
# The attested Browser SDK is dependency-owned and must remain byte-identical.
# Its immutable inventory is checked again after installation.
# Variables in this script belong to the Flatpak build environment.
# shellcheck disable=SC2016
flatpak build "$app" bash -c '
  set -euo pipefail
  for binary in /app/libexec/dockpipe/dockpipe-launcher /app/libexec/dockpipe/dockpipe-workspace-reader /app/libexec/dockpipe/libdockpipe-launcher-ui.so* /app/lib/libXRes.so*; do
    strip --strip-unneeded "$binary"
    if readelf --wide --sections "$binary" | grep -E "\\.(z?debug[_.]|symtab[[:space:]]|gnu_debugdata[[:space:]]|gdb_index[[:space:]])" >/dev/null; then
      echo "Private symbol sections remain in public Flatpak binary: $binary" >&2
      exit 1
    fi
  done
'
install -m644 "$out/qt-build/dockpipe-syntax.rcc" "$app/files/libexec/dockpipe/dockpipe-syntax.rcc"
cp -R "$out/qt-build/syntax-licenses" "$app/files/libexec/dockpipe/"
mkdir -p "$app/files/share/applications" "$app/files/share/icons/hicolor/256x256/apps" \
  "$app/files/share/licenses/$app_id"
sed 's/^Icon=.*/Icon=com.dockpipe.Dockpipe/' \
  "$root/workflows/build/desktop/assets/dockpipe-launcher.desktop" \
  > "$app/files/share/applications/$app_id.desktop"
install -m644 "$source_dir/resources/icons/hicolor/256x256/apps/dockpipe-launcher.png" \
  "$app/files/share/icons/hicolor/256x256/apps/$app_id.png"
install -m644 "$root/LICENSE" "$app/files/share/licenses/$app_id/LICENSE"
flatpak build-finish --command=dockpipe-launcher --share=network --share=ipc \
  --socket=wayland --socket=fallback-x11 --device=dri --filesystem=home \
  --filesystem=/run/docker.sock --filesystem=xdg-run/docker.sock --filesystem=xdg-run/podman/podman.sock \
  --talk-name=org.freedesktop.Flatpak \
  --talk-name=org.kde.StatusNotifierWatcher \
  --env=DOCKPIPE_SYSTEM_ROOT=/app/share/dockpipe --env=DOCKPIPE_BIN=/app/bin/dockpipe "$app"

# Exercise runtime libraries, not the larger development SDK. No app installation.
python3 "$root/workflows/build/flatpak/assets/scripts/smoke.py" "$app"
flatpak info --show-commit "org.kde.Sdk//$runtime_branch" > "$out/sdk-commit.txt"
flatpak info --show-commit "org.kde.Platform//$runtime_branch" > "$out/runtime-commit.txt"
flatpak build-export "$out/repo" "$app" staging
flatpak build-bundle --runtime-repo=https://flathub.org/repo/flathub.flatpakrepo \
  "$out/repo" "$out/dockpipe-desktop_${version}_linux_${goarch}.flatpak" "$app_id" staging
printf 'Built %s for linux-%s-flatpak-org.kde.Platform-%s\n' "$app_id" "$goarch" "$runtime_branch"
