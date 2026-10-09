#!/usr/bin/env bash
set -euo pipefail
# Docker bind sources are interpreted by the host engine. Flatpak's private
# /tmp is unsuitable, so subprocesses use the app's persistent host-visible cache.
export TMPDIR="${XDG_CACHE_HOME:?}/dockpipe/tmp"
mkdir -p "$TMPDIR"
chmod 700 "$TMPDIR"
# shellcheck source=release/packaging/desktop/flatpak/container-environment.sh
source /app/libexec/dockpipe/container-environment.sh
case "$(basename "$0")" in
  dockpipe) exec /app/libexec/dockpipe/dockpipe "$@" ;;
  dockpipe-launcher) exec /app/libexec/dockpipe/dockpipe-launcher "$@" ;;
  *) echo 'Invalid Flatpak entrypoint' >&2; exit 2 ;;
esac
