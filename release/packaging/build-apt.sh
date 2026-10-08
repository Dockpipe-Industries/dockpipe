#!/usr/bin/env bash
# Build a signed, by-hash APT repository. Caller owns the temporary GPG home.
set -euo pipefail
artifacts="${1:?artifact directory}"
destination="${2:?new repository directory}"
suite="${APT_SUITE:-stable}"
case "$suite" in
  stable|staging) ;;
  *) echo "APT_SUITE must be stable or staging" >&2; exit 1 ;;
esac
fingerprint="${APT_SIGNING_FINGERPRINT:?APT signing key fingerprint is required}"
[[ ! -e "$destination" ]] || { echo "APT destination already exists: $destination" >&2; exit 1; }
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
release_version=""
for arch in amd64 arm64; do
  pool="$destination/pool/main/d/dockpipe/$arch"
  index="$destination/dists/$suite/main/binary-$arch"
  mkdir -p "$pool" "$index/by-hash/SHA256"
  shopt -s nullglob
  packages=("$artifacts"/dockpipe_*_"$arch".deb)
  [[ ${#packages[@]} -eq 1 ]] || { echo "Expected one $arch DEB" >&2; exit 1; }
  [[ "$(dpkg-deb -f "${packages[0]}" Architecture)" == "$arch" ]]
  [[ "$(dpkg-deb -f "${packages[0]}" Package)" == dockpipe ]]
  version="$(dpkg-deb -f "${packages[0]}" Version)"
  [[ -z "$release_version" || "$version" == "$release_version" ]] || { echo 'APT architectures must share one release version' >&2; exit 1; }
  release_version="$version"
  [[ "$(basename "${packages[0]}")" == "dockpipe_${version}_${arch}.deb" ]] || { echo 'CLI DEB filename/version mismatch' >&2; exit 1; }
  cp "${packages[0]}" "$pool/"
  desktop=("$artifacts"/dockpipe-desktop_*_"$arch".deb)
  [[ ${#desktop[@]} -eq 1 ]] || { echo "Expected one $arch desktop DEB" >&2; exit 1; }
  [[ "$(dpkg-deb -f "${desktop[0]}" Architecture)" == "$arch" ]]
  [[ "$(dpkg-deb -f "${desktop[0]}" Package)" == dockpipe-desktop ]]
  [[ "$(dpkg-deb -f "${desktop[0]}" Version)" == "$(dpkg-deb -f "${packages[0]}" Version)" ]]
  [[ "$(basename "${desktop[0]}")" == "dockpipe-desktop_${version}_${arch}.deb" ]] || { echo 'Desktop DEB filename/version mismatch' >&2; exit 1; }
  cp "${desktop[0]}" "$pool/"
  (cd "$destination" && apt-ftparchive packages "pool/main/d/dockpipe/$arch") > "$index/Packages"
  gzip -n -9 -c "$index/Packages" > "$index/Packages.gz"
  for file in "$index/Packages" "$index/Packages.gz"; do
    hash="$(sha256sum "$file" | cut -d ' ' -f1)"
    cp "$file" "$index/by-hash/SHA256/$hash"
  done
done
release="$destination/dists/$suite/Release"
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin=DockPipe \
  -o APT::FTPArchive::Release::Label=DockPipe \
  -o "APT::FTPArchive::Release::Suite=$suite" \
  -o "APT::FTPArchive::Release::Codename=$suite" \
  -o 'APT::FTPArchive::Release::Architectures=amd64 arm64' \
  -o APT::FTPArchive::Release::Components=main \
  -o APT::FTPArchive::Release::Acquire-By-Hash=yes \
  release "$destination/dists/$suite" > "$destination/Release.tmp"
mv "$destination/Release.tmp" "$release"
gpg --batch --yes --local-user "$fingerprint" --digest-algo SHA256 --clearsign --output "${release%Release}InRelease" "$release"
gpg --batch --yes --local-user "$fingerprint" --digest-algo SHA256 --armor --detach-sign --output "$release.gpg" "$release"
gpg --batch --export "$fingerprint" > "$destination/dockpipe-archive-keyring.gpg"
gpgv --keyring "$destination/dockpipe-archive-keyring.gpg" "${release%Release}InRelease"
