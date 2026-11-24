#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
version="${1:?version}"
app="${2:?built DockPipe.app}"
out="${3:?output directory}"
case "$(uname -m)" in
  arm64) arch=arm64 ;;
  x86_64) arch=amd64 ;;
  *) echo 'Unsupported macOS architecture' >&2; exit 1 ;;
esac
work="$(mktemp -d)"
cleanup() {
  status=$?
  if (( status == 0 )); then
    rm -rf "$work"
  else
    echo "macOS packaging evidence retained at $work" >&2
  fi
}
trap cleanup EXIT
bash "$(dirname "$0")/prepare-app-bundle.sh" "$app"
identity="${DOCKPIPE_MAC_APP_IDENTITY:--}"
if [[ -n "${DOCKPIPE_MAC_NOTARY_PROFILE:-}" ]]; then
  [[ "$identity" != - && -n "${DOCKPIPE_MAC_INSTALLER_IDENTITY:-}" ]] || { echo 'Notarization requires both Developer ID identities' >&2; exit 1; }
  ditto -c -k --keepParent "$app" "$work/notarize.zip"
  xcrun notarytool submit "$work/notarize.zip" --keychain-profile "$DOCKPIPE_MAC_NOTARY_PROFILE" --wait
  xcrun stapler staple "$app"
fi
# Homebrew owns the app and uses its formula for the terminal command.
ditto -c -k --keepParent "$app" "$out/dockpipe-desktop_${version}_darwin_${arch}.zip"
# The direct DMG installs both through Apple's Installer, including the CLI link.
mkdir -p "$work/payload/Applications" "$work/payload/usr/local/bin" "$work/scripts" "$work/dmg"
ditto "$app" "$work/payload/Applications/DockPipe.app"
ln -s /Applications/DockPipe.app/Contents/MacOS/dockpipe-cli "$work/payload/usr/local/bin/dockpipe"
install -m 755 "$root/workflows/package/macos/assets/scripts/preinstall" "$work/scripts/preinstall"
pkgbuild --analyze --root "$work/payload" "$work/components.plist"
# Keep updates at /Applications; do not relocate a managed app to an old user copy.
python3 "$root/workflows/package/macos/assets/scripts/configure-components.py" "$work/components.plist"
pkg_args=(--root "$work/payload" --component-plist "$work/components.plist" --scripts "$work/scripts"
          --identifier com.dockpipe.desktop --version "$version" --install-location /)
if [[ -n "${DOCKPIPE_MAC_INSTALLER_IDENTITY:-}" ]]; then
  pkg_args+=(--sign "$DOCKPIPE_MAC_INSTALLER_IDENTITY")
fi
pkgbuild "${pkg_args[@]}" "$work/dmg/Install Dockpipe.pkg"
if [[ -n "${DOCKPIPE_MAC_NOTARY_PROFILE:-}" ]]; then
  xcrun notarytool submit "$work/dmg/Install Dockpipe.pkg" --keychain-profile "$DOCKPIPE_MAC_NOTARY_PROFILE" --wait
  xcrun stapler staple "$work/dmg/Install Dockpipe.pkg"
fi
printf '%s\n' 'Open Install Dockpipe.pkg to install Dockpipe in Applications and the dockpipe terminal command.' \
  'Use one installation method: this installer or Homebrew.' > "$work/dmg/Read Me.txt"
dmg="$out/dockpipe-desktop_${version}_darwin_${arch}.dmg"
bash "$root/workflows/package/macos/assets/scripts/create-dmg.sh" "$work/dmg" "$dmg"
if [[ -n "${DOCKPIPE_MAC_NOTARY_PROFILE:-}" ]]; then
  codesign --sign "$identity" --timestamp "$dmg"
  xcrun notarytool submit "$dmg" --keychain-profile "$DOCKPIPE_MAC_NOTARY_PROFILE" --wait
  xcrun stapler staple "$dmg"
fi
