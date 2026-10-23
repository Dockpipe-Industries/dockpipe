#!/usr/bin/env bash
set -euo pipefail
repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "$repository_root/src/core/assets/scripts/lib/vm-boot-display.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
unset DOCKPIPE_VM_BOOT_DISPLAY_WIDTH DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT
unset DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_WIDTH DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_HEIGHT
[[ -z "$(vmimage_boot_display_geometry)" ]]
[[ "$(DOCKPIPE_VM_BOOT_DISPLAY_WIDTH=1920 DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT=1080 vmimage_boot_display_geometry)" == 1920x1080 ]]
[[ "$(DOCKPIPE_VM_BOOT_DISPLAY_WIDTH=2560 DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT=1600 vmimage_boot_display_geometry)" == 2560x1600 ]]
[[ "$(DOCKPIPE_VM_BOOT_DISPLAY_WIDTH=4096 DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT=1024 vmimage_boot_display_geometry)" == 4096x1024 ]]
export DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_WIDTH=1280
export DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_HEIGHT=800
[[ "$(vmimage_boot_display_geometry)" == 1280x800 ]]
[[ "$(DOCKPIPE_VM_BOOT_DISPLAY_WIDTH=1920 DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT=1080 vmimage_boot_display_geometry)" == 1920x1080 ]]
unset DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_WIDTH DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_HEIGHT
for dimensions in '1920|' '|1080' '639|480' '640|479' '4097|480' '640|2161' \
  '3840|2160' '999999999999999999999|1080' '01920|1080' '1920.0|1080' '+1920|1080' \
  '1920,edid=off|1080' '-1920|1080' '1920 |1080' '0|0'; do
  IFS='|' read -r width height <<< "$dimensions"
  if (DOCKPIPE_VM_BOOT_DISPLAY_WIDTH="$width" DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT="$height" vmimage_boot_display_geometry) > "$fixture/output" 2> "$fixture/error"; then
    printf 'Invalid geometry accepted: %s\n' "$dimensions" >&2
    exit 1
  fi
  [[ ! -s "$fixture/output" && -s "$fixture/error" ]]
done
payload='$(touch '"$fixture"'/executed)'
if (DOCKPIPE_VM_BOOT_DISPLAY_WIDTH="$payload" DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT=1080 vmimage_boot_display_geometry) >/dev/null 2>&1; then
  printf '%s\n' 'Shell syntax accepted as a dimension.' >&2
  exit 1
fi
[[ ! -e "$fixture/executed" ]]
printf '%s\n' 'VM boot display geometry contract passed'
