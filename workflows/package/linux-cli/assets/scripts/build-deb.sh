#!/usr/bin/env bash
# Build a .deb package. Run from repo root:
#   ./workflows/package/linux-cli/assets/scripts/build-deb.sh [version] [deb-arch]
# deb-arch: amd64 (x86_64 Linux) or arm64 (aarch64 Linux). Default amd64.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
cd "$REPO_ROOT"

PKG_ROOT="${REPO_ROOT}/workflows/package/linux-cli/assets"

source "${REPO_ROOT}/workflows/shared/lib/status.sh"

_default_ver="$(tr -d ' \t\r\n' < "${REPO_ROOT}/VERSION" 2>/dev/null || true)"
[[ -z "${_default_ver}" ]] && _default_ver="0.6.0"
VERSION="${1:-${_default_ver}}"
DEB_ARCH="${2:-amd64}"
case "${DEB_ARCH}" in
  amd64 | arm64) ;;
  *)
    echo "usage: $0 [version] [amd64|arm64]" >&2
    exit 1
    ;;
esac
# Go uses the same GOARCH names for these Debian ports.
GOARCH="${DEB_ARCH}"
PACKAGE="dockpipe_${VERSION}_${DEB_ARCH}"
OUT_DIR="${3:-${REPO_ROOT}/release/artifacts}"
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"
BUILD_DIR="$(mktemp -d "$OUT_DIR/.deb-stage.XXXXXX")"
trap 'rm -rf -- "$BUILD_DIR"' EXIT
binary="${DOCKPIPE_RELEASE_BINARY:?An explicit built CLI is required}"
[[ -f "$binary" && ! -L "$binary" ]] || { echo 'CLI input must be a regular file' >&2; exit 1; }
mkdir -p "${BUILD_DIR}/usr/bin"
cp "$binary" "${BUILD_DIR}/usr/bin/dockpipe"
chmod 755 "${BUILD_DIR}/usr/bin/dockpipe"

if [[ -n "${DOCKPIPE_RELEASE_STORE:-}" ]]; then
  python3 "${REPO_ROOT}/workflows/release/metadata/assets/scripts/release-artifacts.py" stage-store "$DOCKPIPE_RELEASE_STORE" \
    --destination "${BUILD_DIR}/usr/share/dockpipe"
fi

# Doc
mkdir -p "${BUILD_DIR}/usr/share/doc/dockpipe"
cp README.md LICENSE CONTRIBUTING.md "${BUILD_DIR}/usr/share/doc/dockpipe/"
cp -r docs "${BUILD_DIR}/usr/share/doc/dockpipe/" 2>/dev/null || true

# Debian control (version substituted)
mkdir -p "${BUILD_DIR}/DEBIAN"
sed -e "s/^Version: .*/Version: ${VERSION}/" -e "s/^Architecture: .*/Architecture: ${DEB_ARCH}/" "${PKG_ROOT}/control" > "${BUILD_DIR}/DEBIAN/control"

run_with_elapsed_status "Building Debian package ${PACKAGE}.deb" \
  dpkg-deb --root-owner-group --build "${BUILD_DIR}" "${OUT_DIR}/${PACKAGE}.deb"
echo "Built: ${OUT_DIR}/${PACKAGE}.deb"
rm -rf "${BUILD_DIR}"
