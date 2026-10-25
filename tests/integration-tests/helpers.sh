#!/usr/bin/env bash

integration_repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd
}

integration_use_repo_checkout() {
  local repo_root variable
  repo_root="${1:-$(integration_repo_root)}"

  # Each product invocation selects its own project and run context. Preserve
  # intentional tools, Docker settings and isolated package roots only.
  while IFS= read -r variable; do
    case "$variable" in
      DOCKPIPE_TEST_* | DOCKPIPE_BOOTSTRAP_* | DOCKPIPE_GO_SDK_* | \
      DOCKPIPE_DOCKER_* | DOCKPIPE_HOST_BASH_BIN | DOCKPIPE_FORCE_ROOT_CONTAINER | \
      DOCKPIPE_WINDOWS_BRIDGE | DOCKPIPE_GLOBAL_ROOT | DOCKPIPE_SYSTEM_ROOT)
        ;;
      DOCKPIPE_*)
        unset "$variable"
        ;;
    esac
  done < <(compgen -e)

  export DOCKPIPE_BIN="$repo_root/src/bin/dockpipe"
}

# Individual fixtures and the full runner use the same current product binary
# without forcing a private fixture's project to the maintainer checkout.
integration_use_repo_checkout
