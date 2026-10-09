#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
resolver_root="$(cd "$script_dir/../.." && pwd)"
if [[ -d "$resolver_root/assets/tooling/bin" ]]; then
  export PATH="$resolver_root/assets/tooling/bin:$PATH"
fi
edge_binary="${DOCKPIPE_CLOUDFLARE_EDGE_BIN:-$resolver_root/assets/tooling/bin/cloudflare-edge}"

if [[ ! -x "$edge_binary" ]]; then
  dockpipe_cmd="${DOCKPIPE_BIN:-dockpipe}"
  eval "$("$dockpipe_cmd" sdk)"
  edge_binary="$(dockpipe_sdk require tooling-bin cloudflare-edge || true)"
fi

if [[ ! -x "$edge_binary" ]]; then
  printf '%s\n' 'The Cloudflare resolver helper is not built. Run dockpipe package build source --only remote, which builds and compiles the resolver.' >&2
  exit 1
fi

exec "$edge_binary" "$@"
