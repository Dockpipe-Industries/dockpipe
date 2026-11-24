#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
app="${1:?built DockPipe.app}"
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
    echo "macOS bundle preparation evidence retained at $work" >&2
  fi
}
trap cleanup EXIT

# Assemble and sign the app without creating, mounting or installing a disk image.
# The distribution workflow reuses this same step before producing its outputs.
browser_sdk="${DOCKPIPE_BROWSER_SDK:?Verified Browser SDK prefix required}"
resources="$app/Contents/Resources"
store="$resources/share/dockpipe"
terminal_provider="$app/Contents/Helpers/dockpipe-terminal-host"
mkdir -p "$app/Contents/Helpers" "$resources" "$work/DockPipe.iconset"
cp -R "$root/workflows/build/desktop/assets/licenses" "$resources/"
cp "$root/LICENSE" "$resources/licenses/DockPipe-LICENSE.txt"
install -m 755 "$(dirname "$app")/dockpipe-workspace-reader" "$app/Contents/MacOS/dockpipe-workspace-reader"
# Core discovers the installed terminal provider beside its own executable.
install -m 755 "$(dirname "$app")/dockpipe-terminal-host" "$terminal_provider"
python3 "$root/workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py" --sdk "$browser_sdk" --platform "darwin-$arch" \
  --runtime "$app/Contents/Frameworks" --licenses "$resources/licenses/SpatialWebBrowser"
install -m 755 "$root/src/bin/dockpipe" "$app/Contents/Helpers/dockpipe"
clang -fobjc-arc -fblocks -O2 -Wall -Wextra \
  -framework Foundation -framework LocalAuthentication -framework CoreGraphics -framework IOKit -framework Security \
  "$root/src/lib/infrastructure/osauth/native/darwin/system_auth.m" \
  -o "$app/Contents/Helpers/dockpipe-system-auth"
install -m 755 "$root/workflows/package/macos/assets/dockpipe" "$app/Contents/MacOS/dockpipe-cli"
# The wrapper name must differ from Dockpipe even on case-insensitive APFS.
otool -hv "$app/Contents/MacOS/DockPipe" >/dev/null
if [[ -n "${DOCKPIPE_RELEASE_STORE:-}" ]]; then
  python3 "$root/workflows/release/metadata/assets/scripts/release-artifacts.py" stage-store "$DOCKPIPE_RELEASE_STORE" --destination "$store"
fi
icon="$root/src/dockpipe-app/resources/images/dockpipe-launcher.png"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$icon" --out "$work/DockPipe.iconset/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z "$double" "$double" "$icon" --out "$work/DockPipe.iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$work/DockPipe.iconset" -o "$resources/DockPipe.icns"
DYLD_LIBRARY_PATH="$browser_sdk/lib${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}" \
  macdeployqt "$app" -always-overwrite "-executable=$terminal_provider"
webengine="$app/Contents/Frameworks/QtWebEngineCore.framework"
webengine_helper="$webengine/Helpers/QtWebEngineProcess.app"
for required in \
  Helpers/QtWebEngineProcess.app/Contents/MacOS/QtWebEngineProcess \
  Helpers/QtWebEngineProcess.app/Contents/Resources/QtWebEngineProcess.entitlements \
  Resources/icudtl.dat Resources/qtwebengine_resources.pak \
  Resources/qtwebengine_resources_100p.pak Resources/qtwebengine_resources_200p.pak \
  Resources/qtwebengine_locales/en-US.pak; do
  [[ -f "$webengine/$required" ]] || { echo "Incomplete WebEngine runtime: $required" >&2; exit 1; }
done
# macdeployqt does not reliably replace custom owned-image build rpaths.
# Normalize only our images before symbol protection and final signing.
python3 "$root/workflows/package/macos/assets/scripts/normalize-owned-rpaths.py" \
  "$app" "$resources/dockpipe-native-rpath-audit.json"
# Verify our final native payload explicitly; macdeployqt only logs strip failures.
python3 "$root/workflows/package/macos/assets/scripts/protect-owned-binaries.py" \
  "$app" "$resources/dockpipe-native-symbol-audit.json"
# A Developer ID identity can be supplied from an already provisioned keychain.
# Ad-hoc signing makes local/staging builds runnable on Apple Silicon, not trusted by Gatekeeper.
# Local network privacy reliably tracks upgrades only with an Apple-issued signing identity.
identity="${DOCKPIPE_MAC_APP_IDENTITY:--}"
sign_args=(--force --sign "$identity")
if [[ "$identity" != - ]]; then
  sign_args+=(--options runtime --timestamp)
fi
codesign "${sign_args[@]}" --identifier com.dockpipe.cli "$app/Contents/Helpers/dockpipe"
codesign "${sign_args[@]}" --identifier com.dockpipe.system-auth "$app/Contents/Helpers/dockpipe-system-auth"
codesign "${sign_args[@]}" --identifier com.dockpipe.workspace-reader "$app/Contents/MacOS/dockpipe-workspace-reader"
codesign "${sign_args[@]}" --identifier com.dockpipe.terminal-host "$terminal_provider"
codesign "${sign_args[@]}" \
  --entitlements "$webengine_helper/Contents/Resources/QtWebEngineProcess.entitlements" \
  "$webengine_helper"
codesign "${sign_args[@]}" --deep --preserve-metadata=identifier,entitlements "$app"
codesign --verify --deep --strict "$app"
