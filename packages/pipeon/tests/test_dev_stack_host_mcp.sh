#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:-$ROOT/src/bin/dockpipe}"
launch="$ROOT/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/launch.sh"
common="$ROOT/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/common.sh"
desktop="$ROOT/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/desktop.sh"

if ! grep -q 'DOCKPIPE_MCP_ALLOWED_TOOLS= \\' "$launch"; then
  echo "pipeon-dev-stack host bridge must clear inherited DOCKPIPE_MCP_ALLOWED_TOOLS so new bridge tools are not denied by stale parent env" >&2
  exit 1
fi
if ! grep -q 'DOCKPIPE_MCP_IGNORE_ALLOWED_TOOLS=1 \\' "$launch"; then
  echo "pipeon-dev-stack host bridge must ignore inherited allowed-tool filters on Windows" >&2
  exit 1
fi
if ! grep -q 'Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort' "$launch"; then
  echo "pipeon-dev-stack host bridge cleanup must stop stale Windows port owners, not only pid-file pids" >&2
  exit 1
fi

for tool in \
  dorkpipe.provider_pool_catalog \
  dorkpipe.provider_pool_status \
  dorkpipe.provider_pool_chat \
  dorkpipe.host_codex_chat \
  dorkpipe.host_claude_chat \
  dorkpipe.host_claude_auth \
  dorkpipe.provider_auth_status \
  dorkpipe.provider_auth_repair
do
  if ! grep -q "\"$tool\"" "$launch"; then
    echo "missing host MCP bridge tool in pipeon-dev-stack reuse probe: $tool" >&2
    exit 1
  fi
done

if ! grep -q 'pipeon_stack_powershell_hidden()' "$common"; then
  echo "missing hidden PowerShell helper for non-interactive Pipeon host calls" >&2
  exit 1
fi

if ! grep -q -- '-WindowStyle Hidden' "$common"; then
  echo "hidden PowerShell helper does not set -WindowStyle Hidden on Windows" >&2
  exit 1
fi

if grep -q '"\$powershell_bin" -NoProfile -Command' "$desktop"; then
  echo "desktop launch still invokes non-interactive PowerShell without hidden helper" >&2
  exit 1
fi

if ! grep -q 'ollama_model_available()' "$launch"; then
  echo "missing cached Ollama model check before launch-time pull" >&2
  exit 1
fi

if ! grep -q 'Ollama model .* is already present; skipping pull' "$launch"; then
  echo "missing launch-time Ollama pull skip message" >&2
  exit 1
fi

if ! grep -q 'provider-pool warm --workdir "\$WORKDIR"' "$launch"; then
  echo "missing Pipeon startup provider-pool warm-up through shared DorkPipe contract" >&2
  exit 1
fi

if ! grep -q 'provider-pool status --workdir "\$WORKDIR" --json' "$launch"; then
  echo "missing Pipeon startup provider-pool status snapshot" >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin/.dockpipe"
export DOCKPIPE_WORKDIR="$tmp"
export HOME="$tmp/fabricated-home"
export XDG_STATE_HOME="$tmp/fabricated-durable"
mkdir -p "$HOME"
dockpipe() {
  "$DOCKPIPE_TEST_BIN" "$@"
}
# shellcheck source=/dev/null
source "$common"

legacy_root="$(dockpipe scope --package pipeon-dev-stack . --workdir "$tmp")"
mkdir -p "$legacy_root/code-server-home/.cache" \
  "$legacy_root/code-server-home/.local/share/code-server/User" \
  "$legacy_root/code-server-home/.local/share/code-server/Machine" \
  "$legacy_root/code-server-home/.local/share/code-server/extensions/user.extension"
printf 'config\n' > "$legacy_root/code-server-home/.gitconfig"
printf 'cache\n' > "$legacy_root/code-server-home/.cache/cache"
printf 'settings\n' > "$legacy_root/code-server-home/.local/share/code-server/User/settings.json"
printf 'machine\n' > "$legacy_root/code-server-home/.local/share/code-server/Machine/settings.json"
printf 'extension\n' > "$legacy_root/code-server-home/.local/share/code-server/extensions/user.extension/package.json"
legacy_hash="$(find "$legacy_root/code-server-home" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum)"
pipeon_stack_prepare_code_server_state
runtime_root="$(pipeon_stack_state_dir)"
if [[ "$runtime_root" != "$tmp/bin/.dockpipe/packages-runtime/"* ]]; then
  echo "pipeon-dev-stack disposable state did not resolve through PackageRuntimeDir: $runtime_root" >&2
  exit 1
fi
if [[ "$(pipeon_stack_code_server_home)" == "$legacy_root/code-server-home" || "$(pipeon_stack_code_server_home)" == "$tmp/bin/.dockpipe/"* ]]; then
  echo "pipeon-dev-stack durable code-server home stayed under legacy/runtime state" >&2
  exit 1
fi
if [[ ! -f "$(pipeon_stack_code_server_home)/.gitconfig" \
  || ! -f "$(pipeon_stack_code_server_user_dir)/settings.json" \
  || ! -f "$(pipeon_stack_code_server_machine_dir)/settings.json" \
  || ! -f "$(pipeon_stack_code_server_extensions_dir)/user.extension/package.json" ]]; then
  echo "pipeon-dev-stack durable user configuration or extensions were not imported" >&2
  exit 1
fi
if [[ -e "$(pipeon_stack_code_server_home)/.cache" \
  || -e "$(pipeon_stack_code_server_runtime_user_data)/User/settings.json" ]]; then
  echo "pipeon-dev-stack cache or runtime server product fell back to durable/user authority" >&2
  exit 1
fi
if [[ "$legacy_hash" != "$(find "$legacy_root/code-server-home" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum)" ]]; then
  echo "pipeon-dev-stack legacy code-server home was mutated" >&2
  exit 1
fi

key_file="$(pipeon_stack_api_key_file)"
printf 'existing-secret\n' > "$key_file"
chmod 644 "$key_file"
ensure_pipeon_stack_api_key
if [[ "$(stat -c '%a' "$key_file")" != "600" ]]; then
  echo "pipeon-dev-stack API key is not owner-only" >&2
  exit 1
fi

link_target="$tmp/link-target"
printf 'do-not-touch\n' > "$link_target"
ln -s "$link_target" "$(pipeon_stack_host_mcp_api_key_file)"
if ensure_pipeon_stack_host_mcp_api_key >/dev/null 2>&1; then
  echo "pipeon-dev-stack accepted a linked API-key path" >&2
  exit 1
fi

echo "pipeon-dev-stack host MCP, hidden PowerShell, and model-cache checks ok"
