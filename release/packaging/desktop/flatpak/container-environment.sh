#!/usr/bin/env bash
# Source in both desktop/CLI entrypoints and the Docker client wrapper, so the
# engine's API client and subprocess tools select the same host socket.
export DOCKER_CONFIG="${DOCKER_CONFIG:-${XDG_CONFIG_HOME:?}/dockpipe/docker}"
if [[ -z "${DOCKER_HOST:-}" && -z "${DOCKER_CONTEXT:-}" ]]; then
  for socket in /run/docker.sock "${XDG_RUNTIME_DIR:?}/docker.sock" "$XDG_RUNTIME_DIR/podman/podman.sock"; do
    if [[ -S "$socket" ]]; then
      export DOCKER_HOST="unix://$socket"
      break
    fi
  done
fi
