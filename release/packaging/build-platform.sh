#!/usr/bin/env bash
# Build on the target host: resolver artifacts include native executables.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
version="${1:?release version}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Expected numeric release version' >&2; exit 1; }
# Core compilation reads the workspace VERSION. Keep every generated payload on
# the selected release version while preserving the authored release-line baseline.
authored_version="$(mktemp)"
wrapper_work="$(mktemp -d)"
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
