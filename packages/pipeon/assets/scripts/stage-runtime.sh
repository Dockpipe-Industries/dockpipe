#!/usr/bin/env bash
# Build release payloads once, so installed Pipeon never needs a source checkout.
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_root="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$script_dir/../..}" && pwd)"
repo_root="$(cd "$package_root/../.." && pwd)"
dockpipe_bin="${DOCKPIPE_BIN:-$repo_root/src/bin/dockpipe}"
context="${1:?prepared code-server build context}"
source "$repo_root/src/core/assets/scripts/lib/package-source-build.sh"
dockpipe_source_build_init "$repo_root"
stage_root="$("$dockpipe_bin" __state package-runtime --workdir "$repo_root" --owner pipeon --path resolver-build)"
mkdir -p "$stage_root"
stage="$(mktemp -d "$stage_root/packaged.XXXXXX")"
trap 'rm -rf -- "$stage"' EXIT
mkdir -p "$stage/resolvers"
cp "$package_root/package.yml" "$stage/package.yml"
cp -R "$package_root/resolvers/pipeon-dev-stack" "$stage/resolvers/pipeon-dev-stack"
assets="$stage/resolvers/pipeon-dev-stack/assets"
mkdir -p "$assets/tooling/bin" "$assets/tooling/bin/linux"
cp -R "$context" "$assets/code-server-context"
cp -R "$repo_root/packages/dorkpipe/resolvers/dorkpipe/assets/provider-pools" "$assets/provider-pools"
mkdir -p "$assets/pipeon"
cp -R "$package_root/resolvers/pipeon/assets" "$assets/pipeon/assets"

cat > "$assets/tooling/bin/pipeon" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
bin_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export DOCKPIPE_SCRIPT_DIR="$(cd "$bin_dir/../../pipeon/assets/scripts" && pwd)"
export DOCKPIPE_WORKDIR="${DOCKPIPE_WORKDIR:-$PWD}"
exec "${DOCKPIPE_HOST_BASH_BIN:-bash}" "$DOCKPIPE_SCRIPT_DIR/pipeon.sh" "$@"
SCRIPT
chmod 755 "$assets/tooling/bin/pipeon"

host_os="$(go env GOOS)"
host_arch="$(go env GOARCH)"
suffix=""
[[ "$host_os" != windows ]] || suffix=.exe
for tool in dockpipe dorkpipe mcpd; do
  case "$tool" in
    dockpipe) module="$repo_root"; entry=./src/cmd ;;
    dorkpipe) module="$repo_root/packages/dorkpipe/lib"; entry=./cmd/dorkpipe ;;
    mcpd) module="$repo_root/packages/dorkpipe-mcp"; entry=./cmd/mcpd ;;
  esac
  GOOS=linux GOARCH="$host_arch" CGO_ENABLED=0 \
    dockpipe_source_build_tool "$tool" "$module" "$assets/tooling/bin/linux/$tool" pipeon "$entry"
  if [[ "$host_os" == linux ]]; then
    cp "$assets/tooling/bin/linux/$tool" "$assets/tooling/bin/$tool"
  else
    CGO_ENABLED=0 dockpipe_source_build_tool "$tool" "$module" "$assets/tooling/bin/$tool$suffix" pipeon "$entry"
  fi
done
printf '%s\n' "$host_arch" > "$assets/tooling/bin/linux/architecture"
"$dockpipe_bin" package compile resolvers --workdir "$repo_root" --from "$stage" --force
"$dockpipe_bin" package compile workflows --workdir "$repo_root" --from "$stage" --force
# Replacing the stack invalidates its status/stop consumers. Restore those
# package-owned workflows without recompiling the prepared stack payload.
"$dockpipe_bin" package compile workflows --workdir "$repo_root" --from "$package_root/workflows" --force
