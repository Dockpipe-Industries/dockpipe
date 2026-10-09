#!/usr/bin/env bash
# Package-owned recipes build dependencies in the SDK and test in the Platform.
set -euo pipefail
app="$(realpath "${1:?Flatpak app build directory}")"
source_dir="$(realpath "${2:?package source directory}")"
out="${3:?fresh output directory}"
for recipe in build.sh test.sh; do
  [[ -f "$source_dir/assets/flatpak/$recipe" ]] || { echo "Missing package recipe assets/flatpak/$recipe" >&2; exit 1; }
done
[[ ! -e "$out" ]] || { echo 'Package output directory must be new' >&2; exit 1; }
mkdir -p "$out"
out="$(realpath "$out")"
mkdir -p "$out/compiled" "$out/home" "$out/state" "$out/cache" "$out/config" "$out/data"
options=(--unshare=network --nofilesystem=host --nofilesystem=home
  --nofilesystem=/run/docker.sock --nofilesystem=xdg-run/docker.sock --nofilesystem=xdg-run/podman/podman.sock
  --no-talk-name=org.freedesktop.Flatpak
  "--filesystem=$source_dir:ro" "--filesystem=$out" "--build-dir=$out"
  "--env=HOME=$out/home" "--env=XDG_STATE_HOME=$out/state" "--env=XDG_CACHE_HOME=$out/cache"
  "--env=XDG_CONFIG_HOME=$out/config" "--env=XDG_DATA_HOME=$out/data"
  "--env=DOCKPIPE_PACKAGES_ROOT=$out/compiled" --env=DOCKPIPE_BIN=/app/bin/dockpipe)
# Sources for bundled dependencies must be supplied and verified before this
# offline build. Recipes own their compiled package tree and library lookup paths.
flatpak build "${options[@]}" "$app" bash "$source_dir/assets/flatpak/build.sh" "$out/compiled"
flatpak build --runtime "${options[@]}" "$app" bash "$source_dir/assets/flatpak/test.sh" "$out/compiled"
# Label only after the package test passes without development SDK libraries.
flatpak build --runtime "${options[@]}" "$app" /app/bin/dockpipe package build store \
  --workdir "$out" --out "$out/store"
