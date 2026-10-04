#!/usr/bin/env sh
# Install dockpipe on Linux or macOS from checksum-verified GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/Dockpipe-Industries/dockpipe/master/release/packaging/linux/install.sh | sh
#
# Optional env:
#   DOCKPIPE_VERSION=1.2.3   Pin tag (default: latest release)
#   DOCKPIPE_REPO=owner/repo  Fork releases
set -eu

REPO="${DOCKPIPE_REPO:-Dockpipe-Industries/dockpipe}"
VERSION="${DOCKPIPE_VERSION:-}"
if [ -n "${DOCKPIPE_DOWNLOAD_BASE:-}" ] && [ -z "$VERSION" ]; then
  echo "DOCKPIPE_DOWNLOAD_BASE requires DOCKPIPE_VERSION" >&2
  exit 1
fi
TMP="$(mktemp -d "${TMPDIR:-/tmp}/dockpipe-install.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO- "$1"; }
else
  echo "Need curl or wget" >&2
  exit 1
fi

if [ -z "$VERSION" ]; then
  VERSION="$(fetch "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -1)"
fi
case "$VERSION" in
  *[!0-9.]* | .* | *..*) echo "Invalid release version" >&2; exit 1 ;;
esac
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
  echo "Could not determine release version" >&2
  exit 1
fi

arch="$(uname -m)"
case "$arch" in
  x86_64) goarch=amd64 ;;
  aarch64 | arm64) goarch=arm64 ;;
  *) echo "Unsupported architecture: $arch (need x86_64 or aarch64)" >&2; exit 1 ;;
esac

base="${DOCKPIPE_DOWNLOAD_BASE:-https://github.com/${REPO}/releases/download/v${VERSION}}"
fetch "${base}/SHA256SUMS.txt" > "$TMP/SHA256SUMS.txt"
download() {
  file="$1"
  fetch "${base}/${file}" > "$TMP/$file"
  expected="$(awk -v name="$file" '$2 == name {print $1}' "$TMP/SHA256SUMS.txt")"
  [ "${#expected}" = 64 ] || { echo "Missing checksum: $file" >&2; exit 1; }
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$TMP/$file" | cut -d ' ' -f1)"
  else
    actual="$(shasum -a 256 "$TMP/$file" | cut -d ' ' -f1)"
  fi
  [ "$actual" = "$expected" ] || { echo "Checksum mismatch: $file" >&2; exit 1; }
}
os="$(uname -s)"
case "$os" in
  Linux) goos=linux ;;
  Darwin) goos=darwin ;;
  *) echo "Use the Windows installer on Windows" >&2; exit 1 ;;
esac
id=
if [ "$goos" = linux ] && [ -f /etc/os-release ]; then
  id="$(
    # shellcheck source=/dev/null
    . /etc/os-release
    printf '%s' "${ID:-}"
  )"
fi
if [ "${DOCKPIPE_INSTALL_MODE:-}" = portable ] || [ "$goos" = darwin ]; then
  id=portable
fi

run_root() {
  if [ "$(id -u)" = 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

case "$id" in
  alpine)
    pkg="dockpipe_${VERSION}_linux_${goarch}.apk"
    download "$pkg"
    run_root apk add --allow-untrusted "$TMP/$pkg"
    ;;
  fedora | rhel | centos | rocky | almalinux)
    pkg="dockpipe_${VERSION}_linux_${goarch}.rpm"
    download "$pkg"
    if command -v dnf >/dev/null 2>&1; then
      run_root dnf install -y "$TMP/$pkg"
    elif command -v yum >/dev/null 2>&1; then
      run_root yum install -y "$TMP/$pkg"
    else
      run_root rpm -Uvh "$TMP/$pkg"
    fi
    ;;
  arch | archlinux | endeavouros | manjaro)
    pkg="dockpipe_${VERSION}_linux_${goarch}.pkg.tar.zst"
    download "$pkg"
    run_root pacman -U --noconfirm "$TMP/$pkg"
    ;;
  debian | ubuntu | pop | linuxmint | zorin)
    pkg="dockpipe_${VERSION}_${goarch}.deb"
    download "$pkg"
    run_root apt-get install -y "$TMP/$pkg"
    ;;
  *)
    echo "Installing portable tarball for $goos/$goarch"
    tgz="dockpipe_${VERSION}_${goos}_${goarch}.tar.gz"
    core_tgz="dockpipe-core-${VERSION}.tar.gz"
    download "$tgz"
    download "$core_tgz"
    install_dir="${DOCKPIPE_INSTALL_DIR:-${HOME}/.local/bin}"
    if [ -n "${DOCKPIPE_GLOBAL_ROOT:-}" ]; then
      data_root="$DOCKPIPE_GLOBAL_ROOT"
    elif [ "$goos" = darwin ]; then
      data_root="$HOME/Library/Application Support/dockpipe"
    else
      data_root="${XDG_DATA_HOME:-$HOME/.local/share}/dockpipe"
    fi
    core_dir="${DOCKPIPE_CORE_DIR:-$data_root/packages/core}"
    mkdir -p "$install_dir" "$core_dir"
    tar -xzf "$TMP/$tgz" -C "$TMP" dockpipe
    install -m 755 "$TMP/dockpipe" "$install_dir/dockpipe"
    cp "$TMP/$core_tgz" "$core_dir/$core_tgz"
    shell="${SHELL:-/bin/sh}"
    case "$shell" in
      */bash) rc="${HOME}/.bashrc" ;;
      */zsh) rc="${HOME}/.zshrc" ;;
      *) rc="${HOME}/.profile" ;;
    esac
    echo "Add $install_dir to PATH in $rc if needed."
    ;;
esac

echo "Installed dockpipe ${VERSION}. Ensure Docker and bash are available: dockpipe doctor"
