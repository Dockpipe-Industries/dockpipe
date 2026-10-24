#!/usr/bin/env bash

# Optional standard-VGA EDID preference, expressed only as validated dimensions.
# Device selection and guest-driver capabilities remain with the caller/guest.
vmimage_boot_display_geometry() {
  local width="${DOCKPIPE_VM_BOOT_DISPLAY_WIDTH:-${DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_WIDTH:-}}"
  local height="${DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT:-${DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_HEIGHT:-}}"
  if [[ -z "$width" && -z "$height" ]]; then
    return 0
  fi
  if [[ -z "$width" || -z "$height" ]]; then
    printf '%s\n' 'Boot display width and height must be supplied together.' >&2
    return 2
  fi
  if [[ ! "$width" =~ ^[1-9][0-9]{2,3}$ || ! "$height" =~ ^[1-9][0-9]{2,3}$ ]]; then
    printf '%s\n' 'Boot display dimensions must be plain decimal integers.' >&2
    return 2
  fi
  if (( width < 640 || width > 4096 || height < 480 || height > 2160 )); then
    printf '%s\n' 'Boot display width must be 640..4096 and height 480..2160.' >&2
    return 2
  fi
  if (( width * height > 4194304 )); then
    printf '%s\n' 'Boot display geometry exceeds the standard VGA 16 MiB framebuffer budget.' >&2
    return 2
  fi
  printf '%sx%s\n' "$width" "$height"
}
