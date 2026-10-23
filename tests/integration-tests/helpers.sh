#!/usr/bin/env bash

integration_use_repo_checkout() {
  local repo_root
  repo_root="${1:-$(integration_repo_root)}"
  export DOCKPIPE_REPO_ROOT="$repo_root"
  export DOCKPIPE_BIN="$repo_root/src/bin/dockpipe"
}

integration_repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd
}

# These integration tests exercise the repo checkout binary and assets, not the
# installed bundled cache. Export the checkout root when the helper is sourced
# so individual test files behave the same way as the full harness.
integration_use_repo_checkout
