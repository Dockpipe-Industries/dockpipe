#!/usr/bin/env bash
# The client runs in Flatpak; only its engine socket crosses the boundary.
set -euo pipefail
# shellcheck source=release/packaging/desktop/flatpak/container-environment.sh
source /app/libexec/dockpipe/container-environment.sh
mkdir -p "$DOCKER_CONFIG/cli-plugins"
for plugin in compose buildx; do
  destination="$DOCKER_CONFIG/cli-plugins/docker-$plugin"
  if [[ ! -e "$destination" && ! -L "$destination" ]]; then
    ln -s "/app/lib/docker/cli-plugins/docker-$plugin" "$destination"
  fi
done
exec /app/libexec/dockpipe/docker "$@"
