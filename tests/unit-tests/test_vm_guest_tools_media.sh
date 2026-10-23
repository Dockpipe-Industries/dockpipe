#!/usr/bin/env bash
set -euo pipefail
repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
source "$repository_root/src/core/assets/scripts/lib/vm-guest-tools-media.sh"
vmimage_shell_path() { printf '%s\n' "$1"; }
vmimage_die() { printf '%s\n' "$*" >&2; exit 1; }
vmimage_copy_tree_contents() { cp -R "$1/." "$2/"; }
export DOCKPIPE_RESOLVER_PACKAGE_ROOT="$fixture/resolver"
export DOCKPIPE_RESOLVER_VM_GUEST_TOOLS_BUNDLE=assets/guest-tools/windows
bundle="$DOCKPIPE_RESOLVER_PACKAGE_ROOT/$DOCKPIPE_RESOLVER_VM_GUEST_TOOLS_BUNDLE"
mkdir -p "$bundle" "$fixture/media"
printf 'native application fixture\n' > "$bundle/dockpipe-guest-tools.exe"
printf 'package installer fixture\n' > "$bundle/install-guest-tools.ps1"
vmimage_stage_resolver_guest_tools "$fixture/media"
cmp "$bundle/dockpipe-guest-tools.exe" "$fixture/media/GuestTools/dockpipe-guest-tools.exe"
[[ "$(cat "$fixture/media/Install-Dockpipe-Guest-Tools.cmd")" == *'GuestTools\dockpipe-guest-tools.exe'* ]]
if (DOCKPIPE_RESOLVER_VM_GUEST_TOOLS_BUNDLE=../outside vmimage_stage_resolver_guest_tools "$fixture/media") >/dev/null 2>&1; then
  echo 'Traversal accepted' >&2
  exit 1
fi
ln -s install-guest-tools.ps1 "$bundle/linked.ps1"
if (vmimage_stage_resolver_guest_tools "$fixture/media") >/dev/null 2>&1; then
  echo 'Linked bundle accepted' >&2
  exit 1
fi
rm "$bundle/linked.ps1"
truncate -s 536870913 "$bundle/oversized.bin"
if (vmimage_stage_resolver_guest_tools "$fixture/media") >/dev/null 2>&1; then
  echo 'Oversized bundle accepted' >&2
  exit 1
fi
printf 'Guest Tools media contract passed\n'
