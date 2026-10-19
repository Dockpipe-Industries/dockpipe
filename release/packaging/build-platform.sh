#!/usr/bin/env bash
# Build on the target host: resolver artifacts include native executables.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
if [[ "${1:-}" == --finalize-cockpit ]]; then
  version="${2:?release version}"
  cockpit_store="${3:?qualified Cockpit resolver store}"
  [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Expected numeric release version' >&2; exit 1; }
  [[ "$(go env GOHOSTOS)-$(go env GOHOSTARCH)" == linux-amd64 ]] || { echo 'Cockpit finalization supports Linux amd64 only.' >&2; exit 1; }
  out="$root/release/artifacts"
  store="$out/stores/linux-amd64"
  archive="$out/dockpipe-packages_${version}_linux-amd64.tar.gz"
  [[ -d "$store" && ! -L "$store" && -f "$archive" && ! -L "$archive" ]] || { echo 'Initial release store and archive are required.' >&2; exit 1; }
  work="$(mktemp -d "$out/.cockpit-finalization.XXXXXX")"
  finalized=0
  cleanup_finalization() {
    if [[ "$finalized" != 1 ]]; then
      if [[ -d "$work/previous-store" ]]; then
        rm -rf -- "$store"
        mv "$work/previous-store" "$store" || return
      fi
      if [[ -f "$work/previous-archive.tar.gz" ]]; then
        mv "$work/previous-archive.tar.gz" "$archive" || return
      fi
    fi
    rm -rf -- "$work"
  }
  trap cleanup_finalization EXIT
  python3 release/packaging/ingest-pinned-stores.py --finalize-cockpit \
    --base-store "$store" --cockpit-store "$cockpit_store" --version "$version" --out "$work/store"
  python3 release/packaging/release-artifacts.py verify-store "$work/store"
  tar -C "$work/store" -czf "$work/packages.tar.gz" .
  mv "$store" "$work/previous-store"
  mv "$archive" "$work/previous-archive.tar.gz"
  mv "$work/store" "$store"
  mv "$work/packages.tar.gz" "$archive"
  finalized=1
  echo "Finalized Linux amd64 release catalog with qualified Cockpit 0.1.1."
  exit 0
fi
version="${1:?release version}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Expected numeric release version' >&2; exit 1; }
# Core compilation reads the workspace VERSION. Keep every generated payload on
# the selected release version while preserving the authored release-line baseline.
authored_version="$(mktemp)"
wrapper_work="$(mktemp -d)"
# macOS exposes its temporary directory through /var, a system symlink. Use
# the physical path for this owned workspace so pinned inputs remain link-free.
wrapper_work="$(cd "$wrapper_work" && pwd -P)"
cp VERSION "$authored_version"
cleanup() {
  cp "$authored_version" VERSION
  rm -f "$authored_version"
  rm -rf "$wrapper_work"
}
trap cleanup EXIT
printf '%s\n' "$version" > VERSION
platform="$(go env GOHOSTOS)-$(go env GOHOSTARCH)"
exe="$(go env GOEXE)"
out="$root/release/artifacts"
store="$out/stores/$platform"
core_store="$out/core-stores/$platform"
mkdir -p "$out" "$root/src/bin"
[[ ! -e "$store" && ! -L "$store" ]] || { echo 'Final store must be fresh' >&2; exit 1; }
[[ ! -e "$core_store" && ! -L "$core_store" ]] || { echo 'Core store must be fresh' >&2; exit 1; }
export DOCKPIPE_BIN="$root/src/bin/dockpipe$exe"
export CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -X main.Version=$version" -o "$DOCKPIPE_BIN" ./src/cmd
"$DOCKPIPE_BIN" --version
"$DOCKPIPE_BIN" package compile core --workdir "$root" --from "$root/src/core" --force
"$DOCKPIPE_BIN" package build store --workdir "$root" --only core --out "$core_store" --version "$version"
wrapper_source="$root/workflows/ci/dockpipe-repo-quality"
wrapper_target="$wrapper_work/workflows/dockpipe-repo-quality"
wrapper_store="$wrapper_work/store"
mkdir -p "$wrapper_target/assets/flatpak" "$wrapper_target/assets/scripts"
cp "$wrapper_source/config.yml" "$wrapper_source/package.yml" "$wrapper_target/"
cp "$wrapper_source/assets/source-lock.json" "$wrapper_target/assets/"
cp "$wrapper_source/assets/flatpak/package.json" "$wrapper_target/assets/flatpak/"
cp "$wrapper_source/assets/scripts/repo-quality-pinned.py" "$wrapper_target/assets/scripts/"
"$DOCKPIPE_BIN" package compile workflows --workdir "$wrapper_work" --from "$wrapper_work/workflows" --force
"$DOCKPIPE_BIN" package build store --workdir "$wrapper_work" --only workflows --out "$wrapper_store" --version "$version"
python3 release/packaging/ingest-pinned-stores.py \
  --lock release/packaging/pinned-stores.lock.json \
  --platform "$platform" --core-store "$core_store" --main-store "$wrapper_store" --out "$store"
python3 release/packaging/release-artifacts.py verify-store "$store"
tar -C "$store" -czf "$out/dockpipe-packages_${version}_${platform}.tar.gz" .

archive_platform="${platform/-/_}"
if [[ "$platform" == windows-* ]]; then
  python3 - "$DOCKPIPE_BIN" "$out/dockpipe_${version}_${archive_platform}.zip" <<'PY'
import sys
import zipfile
with zipfile.ZipFile(sys.argv[2], "w", zipfile.ZIP_DEFLATED) as archive:
    archive.write(sys.argv[1], "dockpipe.exe")
PY
else
  tar -C "$root/src/bin" -czf "$out/dockpipe_${version}_${archive_platform}.tar.gz" dockpipe
fi

if [[ "$platform" == linux-* ]]; then
  export DOCKPIPE_RELEASE_BINARY="$DOCKPIPE_BIN"
  bash release/packaging/build-deb.sh "$version" "$(go env GOHOSTARCH)"
  cp "release/packaging/build/dockpipe_${version}_$(go env GOHOSTARCH).deb" "$out/"
  bash release/packaging/build-nfpm.sh "$version" "$out" "$(go env GOHOSTARCH)"
fi
# The platform-neutral core remains a direct release asset for existing installers.
if [[ "$platform" == linux-amd64 ]]; then
  cp "$store"/dockpipe-core-*.tar.gz "$out/"
  "$DOCKPIPE_BIN" package build core --repo-root "$root" --out "$out" --version "$version"
fi
