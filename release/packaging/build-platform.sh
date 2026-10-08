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
cp VERSION "$authored_version"
embedded_prepared=false
cleanup() {
  cp "$authored_version" VERSION
  rm -f "$authored_version"
  if [[ "$embedded_prepared" == true ]]; then
    bash release/packaging/prepare-embedded-dorkpipe-assets.sh clean
  fi
}
trap cleanup EXIT
printf '%s\n' "$version" > VERSION
platform="$(go env GOHOSTOS)-$(go env GOHOSTARCH)"
exe="$(go env GOEXE)"
out="$root/release/artifacts"
store="$out/stores/$platform"
mkdir -p "$store" "$root/src/bin"
export DOCKPIPE_BIN="$root/src/bin/dockpipe$exe"
# Release packages contain the Pipeon resolver/extension, not its optional desktop app.
export PIPEON_BUILD_DESKTOP=0
export CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -X main.Version=$version" -o "$DOCKPIPE_BIN" ./src/cmd
embedded_prepared=true
bash release/packaging/prepare-embedded-dorkpipe-assets.sh prepare
go build -trimpath -ldflags "-s -w -X main.Version=$version" -o "$DOCKPIPE_BIN" ./src/cmd
"$DOCKPIPE_BIN" --version
"$DOCKPIPE_BIN" build --workdir "$root" --no-images
"$DOCKPIPE_BIN" package build store --workdir "$root" --out "$store" --version "$version"
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
