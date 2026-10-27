#!/usr/bin/env bash
# Build Alpine (.apk), Fedora/RHEL (.rpm), and Arch Linux (.pkg.tar.zst) packages via nfpm.
# Run from repo root:
#   ./workflows/package/linux-cli/assets/scripts/build-nfpm.sh [version] [dest-dir]
# dest-dir defaults to "release/artifacts" (same as CI release artifacts).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
cd "$REPO_ROOT"

_default_ver="$(tr -d ' \t\r\n' < "${REPO_ROOT}/VERSION" 2>/dev/null || true)"
[[ -z "${_default_ver}" ]] && _default_ver="0.0.0"
VERSION="${1:-${_default_ver}}"
DEST="${2:-release/artifacts}"
if [[ "${DEST}" == /* ]]; then
  OUT_DIR="${DEST}"
else
  OUT_DIR="${REPO_ROOT}/${DEST}"
fi
mkdir -p "${OUT_DIR}"

STAGE="$(mktemp -d "$OUT_DIR/.nfpm-stage.XXXXXX")"
trap 'rm -rf -- "$STAGE"' EXIT
binary="${DOCKPIPE_RELEASE_BINARY:?An explicit built CLI is required}"
[[ -f "$binary" && ! -L "$binary" ]] || { echo 'CLI input must be a regular file' >&2; exit 1; }
# Pin nfpm for reproducible CI (bump when upgrading).
NFPM_VER="${NFPM_VERSION:-v2.41.0}"
NFPM=(go run "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VER}")
if [[ -n "${NFPM_BIN:-}" ]]; then
  NFPM=("$NFPM_BIN")
fi

architecture="${3:?An explicit architecture is required}"
[[ "$architecture" == amd64 || "$architecture" == arm64 ]] || exit 1
architectures=("$architecture")
for goarch in "${architectures[@]}"; do
  BIN="${STAGE}/dockpipe-linux-${goarch}"
  cp "$binary" "$BIN"

  sed -e "s|__VERSION__|${VERSION}|g" -e "s|__GOARCH__|${goarch}|g" -e "s|__BINARY__|${BIN}|g" \
    -e "s|__POLKIT_POLICY__|${REPO_ROOT}/workflows/package/linux-cli/assets/org.dockpipe.remote.policy|g" \
    "${REPO_ROOT}/workflows/package/linux-cli/assets/nfpm.yaml.in" > "${STAGE}/nfpm.yaml"

  if [[ -n "${DOCKPIPE_RELEASE_STORE:-}" ]]; then
    optional_stage="${STAGE}/optional-${goarch}"
    python3 "${REPO_ROOT}/workflows/release/metadata/assets/scripts/release-artifacts.py" stage-store "$DOCKPIPE_RELEASE_STORE" \
      --destination "$optional_stage"
    if [[ -d "$optional_stage/packages" ]]; then
      printf '  - src: "%s/packages/"\n    dst: /usr/share/dockpipe/packages/\n    type: tree\n' "$optional_stage" >> "${STAGE}/nfpm.yaml"
    fi
  fi

  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p apk -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.apk"
  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p rpm -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.rpm"
  "${NFPM[@]}" package -f "${STAGE}/nfpm.yaml" -p archlinux -t "${OUT_DIR}/dockpipe_${VERSION}_linux_${goarch}.pkg.tar.zst"
done

rm -rf "${STAGE}"
echo "Built nfpm packages into ${OUT_DIR}/ (apk, rpm, Arch)"
