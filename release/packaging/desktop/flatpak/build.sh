#!/usr/bin/env bash
# Build the desktop against its actual Flatpak SDK, then test with the Platform.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
version="${1:?numeric release version}"
cli="$(realpath "${2:?static native CLI}")"
store="$(realpath "${3:?verified core store directory}")"
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
# No implicit dependency installation or system changes in a release build.
flatpak info "org.kde.Sdk//$runtime_branch" >/dev/null
flatpak info "org.kde.Platform//$runtime_branch" >/dev/null
mkdir -p "$out"
out="$(realpath "$out")"
app="$out/app"
flatpak build-init "$app" "$app_id" org.kde.Sdk org.kde.Platform "$runtime_branch"
mkdir -p "$app/files/bin" "$app/files/libexec/dockpipe"
install -m755 "$cli" "$app/files/libexec/dockpipe/dockpipe"
for command in dockpipe dockpipe-launcher; do
  install -m755 "$root/release/packaging/desktop/flatpak/entrypoint.sh" "$app/files/bin/$command"
done
python3 "$root/release/packaging/desktop/flatpak/prepare-container-tools.py" "$goarch" "$app/files" \
  --cache "${DOCKPIPE_FLATPAK_DOWNLOAD_CACHE:-$out/downloads}"
install -m755 "$root/release/packaging/desktop/flatpak/docker.sh" "$app/files/bin/docker"
install -m644 "$root/release/packaging/desktop/flatpak/container-environment.sh" "$app/files/libexec/dockpipe/container-environment.sh"
install -m755 "$root/release/packaging/desktop/flatpak/host-command.sh" "$app/files/bin/dockpipe-host-command"
for command in op code cursor codex claude npm systemctl loginctl pkexec nvidia-smi nvidia-ctk qemu-img qemu-system-x86_64; do
  printf '#!/usr/bin/env bash\nexec /app/bin/dockpipe-host-command %s "$@"\n' "$command" > "$app/files/bin/$command"
  chmod 755 "$app/files/bin/$command"
done
flatpak build --filesystem="$root/release/packaging/desktop/flatpak:ro" "$app" \
  bash "$root/release/packaging/desktop/flatpak/stage-sdk-tools.sh"
python3 "$root/release/packaging/release-artifacts.py" stage-core "$store" \
  --destination "$app/files/share/dockpipe"

source_dir="$root/src/app/tooling/dockpipe-launcher"
flatpak build --filesystem="$source_dir:ro" --filesystem="$out" "$app" \
  cmake -S "$source_dir" -B "$out/qt-build" -DCMAKE_BUILD_TYPE=Release \
  "-DDOCKPIPE_RELEASE_VERSION=$version"
flatpak build --filesystem="$source_dir:ro" --filesystem="$out" "$app" \
  cmake --build "$out/qt-build" --parallel 2
flatpak build --filesystem="$out" --env=QT_QPA_PLATFORM=offscreen "$app" \
  ctest --test-dir "$out/qt-build" --output-on-failure
install -m755 "$out/qt-build/dockpipe-launcher" "$app/files/libexec/dockpipe/dockpipe-launcher"
mkdir -p "$app/files/share/applications" "$app/files/share/icons/hicolor/256x256/apps" \
  "$app/files/share/licenses/$app_id"
sed 's/^Icon=.*/Icon=com.dockpipe.Dockpipe/' \
  "$root/release/packaging/desktop/dockpipe-launcher.desktop" \
  > "$app/files/share/applications/$app_id.desktop"
install -m644 "$source_dir/resources/icons/hicolor/256x256/apps/dockpipe-launcher.png" \
  "$app/files/share/icons/hicolor/256x256/apps/$app_id.png"
install -m644 "$root/LICENSE" "$app/files/share/licenses/$app_id/LICENSE"
flatpak build-finish --command=dockpipe-launcher --share=network --share=ipc \
  --socket=wayland --socket=fallback-x11 --device=dri --filesystem=home \
  --filesystem=/run/docker.sock --filesystem=xdg-run/docker.sock --filesystem=xdg-run/podman/podman.sock \
  --talk-name=org.freedesktop.Flatpak \
  --env=DOCKPIPE_SYSTEM_ROOT=/app/share/dockpipe --env=DOCKPIPE_BIN=/app/bin/dockpipe "$app"

# Exercise runtime libraries, not the larger development SDK. No app installation.
python3 "$root/release/packaging/desktop/flatpak/smoke.py" "$app"
flatpak info --show-commit "org.kde.Sdk//$runtime_branch" > "$out/sdk-commit.txt"
flatpak info --show-commit "org.kde.Platform//$runtime_branch" > "$out/runtime-commit.txt"
flatpak build-export "$out/repo" "$app" staging
flatpak build-bundle --runtime-repo=https://flathub.org/repo/flathub.flatpakrepo \
  "$out/repo" "$out/dockpipe-desktop_${version}_linux_${goarch}.flatpak" "$app_id" staging
printf 'Built %s for linux-%s-flatpak-org.kde.Platform-%s\n' "$app_id" "$goarch" "$runtime_branch"
