#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:-${DOCKPIPE_BIN:-$ROOT/src/bin/dockpipe}}"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/pipeon-package-runtime.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

runtime_root="$("$DOCKPIPE_TEST_BIN" __state package-runtime --workdir "$tmp" --owner pipeon)"
legacy_root="$("$DOCKPIPE_TEST_BIN" scope --package pipeon --workdir "$tmp")"
case "$runtime_root" in
  "$tmp/bin/.dockpipe/packages-runtime/"*) ;;
  *)
    echo "Pipeon runtime root did not use PackageRuntimeDir: $runtime_root" >&2
    exit 1
    ;;
esac
if [[ "$runtime_root" == "$legacy_root" ]]; then
  echo "Pipeon runtime root fell back to legacy package state" >&2
  exit 1
fi

build_script="$ROOT/packages/pipeon/assets/scripts/build.sh"
enable_script="$ROOT/packages/pipeon/resolvers/pipeon/assets/scripts/lib/enable.sh"
extension_source="$ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/src/extension.ts"
desktop_script="$ROOT/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/desktop.sh"
code_server_dockerfile="$ROOT/packages/pipeon/resolvers/pipeon/vscode-extension/Dockerfile.code-server"
if ! grep -Fq '__state package-runtime' "$build_script" ||
   ! grep -Fq '__state package-runtime' "$enable_script" ||
   ! grep -Fq '"__state", "package-runtime"' "$extension_source"; then
  echo "Pipeon maintained runtime consumers do not all select PackageRuntimeDir" >&2
  exit 1
fi

if grep -Fq -- '--extensions-dir /opt/pipeon/extensions' "$desktop_script" ||
   ! grep -Fq -- '--extensions-dir /home/coder/.local/share/code-server-extensions' "$desktop_script" ||
   ! grep -Fq 'GOMODCACHE=/home/coder/.cache/go-mod' "$desktop_script" ||
   ! grep -Fq 'NUGET_PACKAGES=/home/coder/.cache/nuget-packages' "$desktop_script" ||
   ! grep -Fq 'cp -a /opt/pipeon/extensions/. /usr/lib/code-server/lib/vscode/extensions/' "$code_server_dockerfile"; then
  echo "Pipeon code-server does not separate image-owned extensions from durable user extensions" >&2
  exit 1
fi
if grep -Fq 'bin", ".dockpipe", "packages", "pipeon", "pipeon-context.md' "$extension_source"; then
  echo "Pipeon extension still falls back to legacy context state" >&2
  exit 1
fi

echo "Pipeon package-runtime contract OK"
