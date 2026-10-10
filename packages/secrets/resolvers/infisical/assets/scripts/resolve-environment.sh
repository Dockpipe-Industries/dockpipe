#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
resolver_root="$(cd "$script_dir/../.." && pwd)"
if [[ -d "$resolver_root/assets/tooling/bin" ]]; then
  export PATH="$resolver_root/assets/tooling/bin:$PATH"
fi
tool=secret-environment
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) tool=secret-environment.exe ;;
esac
helper="$resolver_root/assets/tooling/bin/$tool"
if [[ ! -x "$helper" ]]; then
  dockpipe_cmd="${DOCKPIPE_BIN:-dockpipe}"
  eval "$("$dockpipe_cmd" sdk)"
  helper="$(dockpipe_sdk require tooling-bin "$tool")"
fi
exec "$helper" infisical
