#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:-${DOCKPIPE_BIN:-$ROOT/src/bin/dockpipe}}"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/dorkpipe-package-runtime.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

runtime_root="$("$DOCKPIPE_TEST_BIN" __state package-runtime --workdir "$tmp" --owner dorkpipe)"
collision_root="$("$DOCKPIPE_TEST_BIN" __state package-runtime --workdir "$tmp" --owner DorkPipe)"
legacy_root="$("$DOCKPIPE_TEST_BIN" scope --package dorkpipe --workdir "$tmp")"
case "$runtime_root" in
  "$tmp/bin/.dockpipe/packages-runtime/"*) ;;
  *)
    echo "DorkPipe runtime root did not use PackageRuntimeDir: $runtime_root" >&2
    exit 1
    ;;
esac
if [[ "$runtime_root" == "$collision_root" || "$runtime_root" == "$legacy_root" ]]; then
  echo "DorkPipe runtime ownership collided with another owner or legacy package state" >&2
  exit 1
fi

scripts=(
  aggregate-reasoning-context.sh
  dev-stack-lib.sh
  merge-paste-prompt.sh
  orchestrator-prompt.sh
  run-self-analysis.sh
  self-analysis-prep.sh
  self-analysis-signals.sh
)
script_root="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
for script in "${scripts[@]}"; do
  if grep -Fq 'scope --package dorkpipe' "$script_root/$script"; then
    echo "DorkPipe disposable script still uses public package state: $script" >&2
    exit 1
  fi
  if ! grep -Fq '__state package-runtime' "$script_root/$script"; then
    echo "DorkPipe disposable script does not select package runtime: $script" >&2
    exit 1
  fi
done

echo "DorkPipe disposable package-runtime contract OK"
