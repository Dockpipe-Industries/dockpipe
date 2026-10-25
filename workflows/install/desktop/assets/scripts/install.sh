#!/usr/bin/env bash
# Stage a repo-built desktop, with an explicit optional per-user installation.
set -euo pipefail
root="${DOCKPIPE_SOURCE_ROOT:?Workflow source scope required}"
stage="${DOCKPIPE_ARTIFACT_ROOT:?Workflow artifact scope required}/desktop"
build="$root/src/dockpipe-app/build"
mkdir -p "$stage/bin" "$stage/share/icons/hicolor"
install -m 755 "$build/dockpipe-launcher" "$stage/bin/dockpipe-launcher"
cp -R "$root/src/dockpipe-app/resources/icons/hicolor/." "$stage/share/icons/hicolor/"
install -m 644 "$root/src/dockpipe-app/resources/images/dockpipe-launcher.png" "$stage/share/icons/dockpipe-launcher.png"
if [[ "${1:-}" != --global ]]; then
  printf 'Staged desktop at %s\n' "$stage"
  exit 0
fi
mkdir -p "$HOME/.local/share/dockpipe/bin" "$HOME/.local/share/dockpipe/icons" \
  "$HOME/.local/share/icons/hicolor" "$HOME/.local/share/applications"
install -m 755 "$stage/bin/dockpipe-launcher" "$HOME/.local/share/dockpipe/bin/dockpipe-launcher"
install -m 644 "$stage/share/icons/dockpipe-launcher.png" "$HOME/.local/share/dockpipe/icons/dockpipe-launcher.png"
cp -R "$stage/share/icons/hicolor/." "$HOME/.local/share/icons/hicolor/"
printf '%s\n' \
  '[Desktop Entry]' \
  'Type=Application' \
  'Name=Dockpipe' \
  'Exec=/usr/bin/env -u DESKTOP_STARTUP_ID -u XDG_ACTIVATION_TOKEN '"$HOME"'/.local/share/dockpipe/bin/dockpipe-launcher --start-home' \
  'Icon=dockpipe-launcher' \
  'Terminal=false' \
  'Categories=Development;' \
  'StartupNotify=false' \
  'X-GNOME-WMClass=dockpipe-launcher' \
  'StartupWMClass=dockpipe-launcher' \
  > "$HOME/.local/share/applications/dockpipe-launcher.desktop"
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$HOME/.local/share/applications" >/dev/null 2>&1 || true
fi
