#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:?DOCKPIPE_TEST_DOCKPIPE_BIN is required}"
FIXTURE="$(mktemp -d)"
trap 'rm -rf "$FIXTURE"' EXIT

make_project() {
  local path="$1"
  mkdir -p "$path"
  printf '%s\n' '{"schema":1,"compile":{"workflows":["workflows"]}}' > "$path/dockpipe.config.json"
  mkdir -p "$path/workflows"
}

first="$FIXTURE/first"
second="$FIXTURE/second"
make_project "$first"
make_project "$second"
export HOME="$FIXTURE/home"
export XDG_STATE_HOME="$FIXTURE/durable"
export DOCKPIPE_BIN="$DOCKPIPE_TEST_BIN"
export DOCKPIPE_WORKDIR="$first"
export DOCKPIPE_PACKAGE_ID="Example.Package"
unset DOCKPIPE_PACKAGE_STATE_DIR DOCKPIPE_SDK_ROOT

# shellcheck source=/dev/null
source "$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"

[[ "${dockpipe[package_state_dir]}" == "$XDG_STATE_HOME/dockpipe/"* ]]
[[ "${dockpipe[package_state_dir]}" != "$first/bin/.dockpipe/"* ]]
[[ "$(dockpipe_sdk path package '' settings.json)" == "${dockpipe[package_state_dir]}/settings.json" ]]
if dockpipe_sdk path package '' ../escape >/dev/null 2>&1; then
  echo "shell SDK accepted package-state traversal" >&2
  exit 1
fi
runtime_path="$(dockpipe_sdk path package-runtime Example.Package cache/build)"
[[ "$runtime_path" == "$first/bin/.dockpipe/packages-runtime/"*"/cache/build" ]]

first_state="${dockpipe[package_state_dir]}"
export DOCKPIPE_PACKAGE_STATE_DIR="$first_state"
dockpipe_sdk_refresh "$second"
[[ "${dockpipe[package_state_dir]}" == "$XDG_STATE_HOME/dockpipe/"* ]]
[[ "${dockpipe[package_state_dir]}" != "$first_state" ]]
[[ "${dockpipe[package_state_dir]}" != "$second/bin/.dockpipe/"* ]]

override="$FIXTURE/owner-override"
mkdir -m 700 "$override"
export DOCKPIPE_PACKAGE_STATE_DIR="$override"
unset DOCKPIPE_SDK_ROOT
dockpipe_sdk_refresh "$second"
[[ "${dockpipe[package_state_dir]}" == "$override" ]]

unsafe="$second/unsafe-state"
mkdir -m 700 "$unsafe"
export DOCKPIPE_PACKAGE_STATE_DIR="$unsafe"
if dockpipe_sdk_refresh "$second" >/dev/null 2>&1; then
  echo "shell SDK accepted a checkout-local package-state override" >&2
  exit 1
fi

export DOCKPIPE_PACKAGE_STATE_DIR="$override"
export DOCKPIPE_SDK_ROOT="$second"
unset DOCKPIPE_PACKAGE_ID DOCKPIPE_PACKAGE_ROOT
dockpipe_sdk_refresh "$first"
[[ -z "${DOCKPIPE_PACKAGE_STATE_DIR:-}" ]]
[[ -z "${dockpipe[package_state_dir]}" ]]

echo "durable package-state shell SDK tests passed"
