#!/usr/bin/env bash
# Package the Cockpit targets from the already qualified matching desktop build.
set -euo pipefail
version="${1:?numeric Dockpipe release version}"
build="$(realpath "${2:?matching desktop build directory}")"
out="${3:?fresh resolver store directory}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_root="$(cd "$script_dir/../.." && pwd)"
repo_root="$(cd "$package_root/../.." && pwd)"
[[ "$(uname -s)-$(uname -m)" == Linux-x86_64 ]] || { echo 'Cockpit release supports Linux amd64 only.' >&2; exit 1; }
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Expected numeric release version.' >&2; exit 1; }
[[ ! -e "$out" && ! -L "$out" ]] || { echo 'Cockpit store output must be fresh.' >&2; exit 1; }
qt_root="${DOCKPIPE_QT_ROOT:?Matching Qt SDK is required}"
sdk="${DOCKPIPE_FORGEPIPE_SDK:?Verified ForgePipe SDK is required}"
debug="${DOCKPIPE_PRIVATE_DEBUG_DIR:?Private diagnostic directory is required}"
[[ "${DOCKPIPE_RELEASE_SOURCE_SHA:-}" =~ ^[0-9a-f]{40}$ ]] || { echo 'An exact Dockpipe release source SHA is required.' >&2; exit 1; }
dockpipe_bin="${DOCKPIPE_BIN:-$repo_root/src/bin/dockpipe}"
export LD_LIBRARY_PATH="$qt_root/lib:$sdk/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export DOCKPIPE_QMAKE="$qt_root/bin/qmake"
[[ "$("$DOCKPIPE_QMAKE" -query QT_VERSION)" == 6.10.3 ]]
timeout 90 xvfb-run -a env QT_QPA_PLATFORM=xcb LIBGL_ALWAYS_SOFTWARE=1 "$build/spatial-gpu-test"
timeout 90 xvfb-run -a env QT_QPA_PLATFORM=xcb LIBGL_ALWAYS_SOFTWARE=1 SDL_AUDIODRIVER=dummy \
  "DOCKPIPE_WORKSPACE_PACKAGE=$build/cockpit-package/assets" "$build/cockpit-package-rendering-test"
work="$(mktemp -d "$build/cockpit-release.XXXXXX")"
trap 'rm -rf -- "$work"' EXIT
resolver="$work/source/resolvers/cockpit"
mkdir -p "$resolver"
cp "$package_root/resolvers/cockpit/package.yml" "$resolver/"
cp "$package_root/resolvers/cockpit/profile.in" "$resolver/profile"
python3 "$script_dir/bundle-linux.py" "$build" "$resolver/assets"
python3 "$script_dir/protect-runtime.py" \
  --assets "$resolver/assets" --sdk "$sdk" --qt "$qt_root" \
  --private-debug "$debug/cockpit-$version-amd64" --repo-root "$repo_root"
python3 "$script_dir/protect-runtime.py" --verify-runtime "$resolver/assets" \
  --build "$build" --qt "$qt_root" --sdk "$sdk" --repo-root "$repo_root"
# The launcher owns QPA and XCB GL-integration plugins. Select that matching
# host plugin catalog while keeping the package's library closure isolated.
timeout 90 xvfb-run -a env QT_QPA_PLATFORM=xcb LIBGL_ALWAYS_SOFTWARE=1 SDL_AUDIODRIVER=dummy \
  "QT_PLUGIN_PATH=$qt_root/plugins" "LD_LIBRARY_PATH=$resolver/assets/lib:$build" \
  "DOCKPIPE_WORKSPACE_PACKAGE=$resolver/assets" \
  "$build/cockpit-package-rendering-test"
export DOCKPIPE_PACKAGES_ROOT="$work/compiled"
"$dockpipe_bin" package compile resolvers --workdir "$repo_root" --from "$work/source" --force
"$dockpipe_bin" package build store --workdir "$repo_root" --only resolvers --out "$out" --version "$version"
python3 "$script_dir/protect-runtime.py" --write-receipt "$out" --sdk "$sdk" --repo-root "$repo_root"
