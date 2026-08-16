#!/usr/bin/env bash
# shellcheck disable=SC2030,SC2031
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:-$ROOT/src/bin/dockpipe}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home"
export XDG_STATE_HOME="$tmp/durable"
export DOCKPIPE_PACKAGE_ROOT="$ROOT/packages/ide"
export DOCKPIPE_PACKAGE_MANIFEST="$DOCKPIPE_PACKAGE_ROOT/package.yml"
mkdir -p "$HOME"

dockpipe() {
  "$DOCKPIPE_TEST_BIN" "$@"
}

assert_file() {
  [[ -f "$1" ]] || { echo "missing expected file: $1" >&2; exit 1; }
}

assert_absent() {
  [[ ! -e "$1" && ! -L "$1" ]] || { echo "unexpected path: $1" >&2; exit 1; }
}

exercise_cursor() (
  set -euo pipefail
  # shellcheck source=/dev/null
  source "$ROOT/packages/ide/resolvers/cursor-dev/assets/scripts/cursor-dev-common.sh"
  W="$tmp/cursor-workspace"
  export W DOCKPIPE_WORKDIR="$W"
  mkdir -p "$W/bin/.dockpipe"
  legacy="$W/bin/.dockpipe/packages/cursor-dev"
  mkdir -p "$legacy/home/.cursor-server/bin" "$legacy/home/.cursor-server/extensions/user.extension" \
    "$legacy/home/.cursor-server/data/User" "$legacy/home/.cursor-server/data/Machine" \
    "$legacy/xdg-config" "$legacy/xdg-data" "$legacy/xdg-cache" "$legacy/dotnet"
  printf 'user-home\n' > "$legacy/home/.gitconfig"
  printf 'server-product\n' > "$legacy/home/.cursor-server/bin/server"
  printf 'extension\n' > "$legacy/home/.cursor-server/extensions/user.extension/package.json"
  printf 'user-settings\n' > "$legacy/home/.cursor-server/data/User/settings.json"
  printf 'machine-settings\n' > "$legacy/home/.cursor-server/data/Machine/settings.json"
  printf 'config\n' > "$legacy/xdg-config/settings"
  printf 'data\n' > "$legacy/xdg-data/state"
  printf 'cache\n' > "$legacy/xdg-cache/cache"
  printf 'dotnet-cache\n' > "$legacy/dotnet/cache"
  source_hash="$(find "$legacy" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum)"

  cursor_dev_prepare_state
  [[ "$CURSOR_DEV_RUNTIME_ROOT" == "$W/bin/.dockpipe/packages-runtime/"* ]]
  [[ "$CURSOR_DEV_DURABLE_HOME" != "$W/"* ]]
  assert_file "$CURSOR_DEV_DURABLE_HOME/.gitconfig"
  assert_file "$CURSOR_DEV_DURABLE_EXTENSIONS/user.extension/package.json"
  assert_file "$CURSOR_DEV_DURABLE_USER/settings.json"
  assert_file "$CURSOR_DEV_DURABLE_MACHINE/settings.json"
  assert_file "$CURSOR_DEV_DURABLE_CONFIG/settings"
  assert_file "$CURSOR_DEV_DURABLE_DATA/state"
  assert_absent "$CURSOR_DEV_DURABLE_HOME/.cursor-server"
  assert_absent "$CURSOR_DEV_DURABLE_USER_ROOT/bin/server"
  assert_absent "$CURSOR_DEV_DURABLE_USER_ROOT/xdg-cache"
  assert_absent "$CURSOR_DEV_DURABLE_USER_ROOT/dotnet"
  [[ "$(stat -c '%a' "$CURSOR_DEV_DURABLE_HOME")" == "700" ]]
  [[ "$source_hash" == "$(find "$legacy" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum)" ]]

  printf 'durable-wins\n' > "$CURSOR_DEV_DURABLE_USER/settings.json"
  chmod 600 "$CURSOR_DEV_DURABLE_USER/settings.json"
  cursor_dev_prepare_state
  [[ "$(cat "$CURSOR_DEV_DURABLE_USER/settings.json")" == "durable-wins" ]]
)

exercise_vscode() (
  set -euo pipefail
  # shellcheck source=/dev/null
  source "$ROOT/packages/ide/resolvers/vscode/assets/scripts/vscode-common.sh"
  W="$tmp/vscode-workspace"
  export W DOCKPIPE_WORKDIR="$W"
  mkdir -p "$W/bin/.dockpipe" "$W/.vscode-server"
  printf 'source-owned\n' > "$W/.vscode-server/source.txt"
  legacy="$W/bin/.dockpipe/packages/vscode"
  mkdir -p "$legacy/home/.vscode-server/cli/servers" "$legacy/home/.vscode-server/extensions/user.extension" \
    "$legacy/home/.vscode-server/data/User" "$legacy/home/.vscode-server/data/Machine" \
    "$legacy/xdg-config" "$legacy/xdg-data" "$legacy/xdg-cache" "$legacy/gocache"
  printf 'user-home\n' > "$legacy/home/.gitconfig"
  printf 'server-product\n' > "$legacy/home/.vscode-server/cli/servers/product"
  printf 'extension\n' > "$legacy/home/.vscode-server/extensions/user.extension/package.json"
  printf 'user-settings\n' > "$legacy/home/.vscode-server/data/User/settings.json"
  printf 'machine-settings\n' > "$legacy/home/.vscode-server/data/Machine/settings.json"
  printf 'config\n' > "$legacy/xdg-config/settings"
  printf 'data\n' > "$legacy/xdg-data/state"
  printf 'cache\n' > "$legacy/xdg-cache/cache"
  printf 'go-cache\n' > "$legacy/gocache/cache"
  source_hash="$(find "$legacy" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum)"

  vscode_prepare_state
  [[ "$VSCODE_RUNTIME_ROOT" == "$W/bin/.dockpipe/packages-runtime/"* ]]
  [[ "$VSCODE_DURABLE_HOME" != "$W/"* ]]
  assert_file "$VSCODE_DURABLE_HOME/.gitconfig"
  assert_file "$VSCODE_DURABLE_EXTENSIONS/user.extension/package.json"
  assert_file "$VSCODE_DURABLE_USER/settings.json"
  assert_file "$VSCODE_DURABLE_MACHINE/settings.json"
  assert_file "$VSCODE_DURABLE_CONFIG/settings"
  assert_file "$VSCODE_DURABLE_DATA/state"
  assert_absent "$VSCODE_DURABLE_HOME/.vscode-server"
  assert_absent "$VSCODE_DURABLE_USER_ROOT/cli/servers/product"
  [[ "$(cat "$W/.vscode-server/source.txt")" == "source-owned" ]]
  [[ "$source_hash" == "$(find "$legacy" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum)" ]]

  rm -rf "$legacy/home/.vscode-server/extensions"
  ln -s "$tmp/linked-extensions" "$legacy/home/.vscode-server/extensions"
  if vscode_prepare_state >/dev/null 2>&1; then
    echo "vscode state preparation accepted a linked legacy extension tree" >&2
    exit 1
  fi
)

exercise_isolation() (
  set -euo pipefail
  # shellcheck source=/dev/null
  source "$ROOT/packages/ide/resolvers/cursor-dev/assets/scripts/cursor-dev-common.sh"
  first="$tmp/isolation-one"
  second="$tmp/isolation-two"
  mkdir -p "$first/bin/.dockpipe" "$second/bin/.dockpipe"
  W="$first"; export W DOCKPIPE_WORKDIR="$W"; cursor_dev_prepare_state; first_runtime="$CURSOR_DEV_RUNTIME_ROOT"; first_durable="$CURSOR_DEV_DURABLE_HOME_ROOT"
  W="$second"; export W DOCKPIPE_WORKDIR="$W"; cursor_dev_prepare_state
  [[ "$first_runtime" != "$CURSOR_DEV_RUNTIME_ROOT" ]]
  [[ "$first_durable" != "$CURSOR_DEV_DURABLE_HOME_ROOT" ]]
)

exercise_cursor
exercise_vscode
exercise_isolation

if rg -n 'mv "\$src" "\$dst"' \
  "$ROOT/packages/ide/resolvers/cursor-dev/assets/scripts" \
  "$ROOT/packages/ide/resolvers/vscode/assets/scripts" >/dev/null; then
  echo "maintained IDE state still mutates source" >&2
  exit 1
fi
[[ "$(rg -c 'scope --package cursor-dev \.' "$ROOT/packages/ide/resolvers/cursor-dev/assets/scripts/cursor-dev-common.sh")" == "1" ]]
[[ "$(rg -c 'scope --package vscode \.' "$ROOT/packages/ide/resolvers/vscode/assets/scripts/vscode-common.sh")" == "1" ]]
for session_script in \
  "$ROOT/packages/ide/resolvers/cursor-dev/assets/scripts/cursor-dev-session.sh" \
  "$ROOT/packages/ide/resolvers/vscode/assets/scripts/vscode-session.sh"; do
  grep -Fq 'GOMODCACHE=${CONTAINER_STATE_ROOT}/gomodcache' "$session_script"
  grep -Fq 'NUGET_PACKAGES=${CONTAINER_STATE_ROOT}/nuget-packages' "$session_script"
  grep -Fq 'NPM_CONFIG_CACHE=${CONTAINER_STATE_ROOT}/xdg-cache/npm' "$session_script"
done

echo "IDE durable user state and disposable server/runtime split tests ok"
