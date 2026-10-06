#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
version="${1:?version}"
app="${2:?built DockPipe.app}"
out="${3:?output directory}"
case "$(uname -m)" in
  arm64) arch=arm64 ;;
  x86_64) arch=amd64 ;;
  *) echo 'Unsupported macOS architecture' >&2; exit 1 ;;
esac
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
resources="$app/Contents/Resources"
store="$resources/share/dockpipe"
mkdir -p "$app/Contents/Helpers" "$store/packages/"{core,workflows,resolvers} "$work/DockPipe.iconset"
cp -R "$root/release/packaging/desktop/licenses" "$resources/"
cp "$root/LICENSE" "$resources/licenses/DockPipe-LICENSE.txt"
install -m 755 "$root/src/bin/dockpipe" "$app/Contents/Helpers/dockpipe"
install -m 755 "$root/release/packaging/desktop/macos/dockpipe" "$app/Contents/MacOS/dockpipe"
python3 "$root/release/packaging/release-artifacts.py" verify-store "$out/stores/darwin-$arch"
cp "$out/stores/darwin-$arch/packages-store-manifest.json" "$store/"
for kind in core workflow resolver; do
  category="$kind"
  [[ "$kind" == core ]] || category="${kind}s"
  cp "$out/stores/darwin-$arch/dockpipe-$kind-"*.tar.gz "$store/packages/$category/"
done
icon="$root/src/app/tooling/dockpipe-launcher/resources/images/dockpipe-launcher.png"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$icon" --out "$work/DockPipe.iconset/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z "$double" "$double" "$icon" --out "$work/DockPipe.iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$work/DockPipe.iconset" -o "$resources/DockPipe.icns"
macdeployqt "$app" -always-overwrite
# A Developer ID identity can be supplied from an already provisioned keychain.
# Ad-hoc signing makes local/staging builds runnable on Apple Silicon, not trusted by Gatekeeper.
identity="${DOCKPIPE_MAC_APP_IDENTITY:--}"
sign_args=(--force --sign "$identity")
if [[ "$identity" != - ]]; then
  sign_args+=(--options runtime --timestamp)
fi
codesign "${sign_args[@]}" "$app/Contents/Helpers/dockpipe"
codesign "${sign_args[@]}" --deep "$app"
codesign --verify --deep --strict "$app"
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
ln -s /Applications/DockPipe.app/Contents/MacOS/dockpipe "$work/payload/usr/local/bin/dockpipe"
install -m 755 "$root/release/packaging/desktop/macos/scripts/preinstall" "$work/scripts/preinstall"
pkgbuild --analyze --root "$work/payload" "$work/components.plist"
# Keep updates at /Applications; do not relocate a managed app to an old user copy.
/usr/libexec/PlistBuddy -c 'Set :0:BundleIsRelocatable false' "$work/components.plist"
pkg_args=(--root "$work/payload" --component-plist "$work/components.plist" --scripts "$work/scripts"
          --identifier com.dockpipe.desktop --version "$version" --install-location /)
if [[ -n "${DOCKPIPE_MAC_INSTALLER_IDENTITY:-}" ]]; then
  pkg_args+=(--sign "$DOCKPIPE_MAC_INSTALLER_IDENTITY")
fi
pkgbuild "${pkg_args[@]}" "$work/dmg/Install DockPipe.pkg"
if [[ -n "${DOCKPIPE_MAC_NOTARY_PROFILE:-}" ]]; then
  xcrun notarytool submit "$work/dmg/Install DockPipe.pkg" --keychain-profile "$DOCKPIPE_MAC_NOTARY_PROFILE" --wait
  xcrun stapler staple "$work/dmg/Install DockPipe.pkg"
fi
printf '%s\n' 'Open Install DockPipe.pkg to install DockPipe in Applications and the dockpipe terminal command.' \
  'Use one installation method: this installer or Homebrew.' > "$work/dmg/Read Me.txt"
dmg="$out/dockpipe-desktop_${version}_darwin_${arch}.dmg"
hdiutil create -volname DockPipe -srcfolder "$work/dmg" -format UDZO -ov "$dmg"
if [[ -n "${DOCKPIPE_MAC_NOTARY_PROFILE:-}" ]]; then
  codesign --sign "$identity" --timestamp "$dmg"
  xcrun notarytool submit "$dmg" --keychain-profile "$DOCKPIPE_MAC_NOTARY_PROFILE" --wait
  xcrun stapler staple "$dmg"
fi
