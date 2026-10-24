#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
export DOCKPIPE_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:?DOCKPIPE_TEST_DOCKPIPE_BIN is required}"
export XDG_STATE_HOME="$fixture/durable"
export XDG_CACHE_HOME="$fixture/cache"
export DOCKPIPE_WORKDIR="$fixture"
export DOCKPIPE_VMIMAGE_SOURCE_ONLY=1
unset DOCKPIPE_PACKAGE_ROOT DOCKPIPE_PACKAGE_ID DOCKPIPE_PACKAGE_STATE_DIR DOCKPIPE_SDK_ROOT
source "$repository_root/src/core/assets/scripts/vmimage-run.sh"

# Exercise the real argument assembler, replacing only host/guest I/O boundaries.
for operation in require_host_dependencies ensure_prompted_inputs prepare_state_split \
  apply_windows_installer_compat_defaults apply_windows_image_compat_defaults \
  ensure_secure_boot_firmware confirm_user_supplied_media_rights \
  confirm_host_network_exposure validate_pci_passthrough confirm_persistent_disk_use \
  prompt_replace_missing_file prepare_session_event report_session; do
  eval "vmimage_${operation}() {
    :
  }"
done
vmimage_boot_source() {
  printf 'image\n'
}
vmimage_windows_should_unattend() {
  return 1
}
vmimage_prepare_bootstrap_media() {
  :
}
vmimage_attach_bootstrap_media_dir() {
  :
}
vmimage_bootstrap_attachment_mode() {
  printf 'disk\n'
}
vmimage_qemu_bin() {
  printf '/unused-fixture-qemu\n'
}
vmimage_resolve_path() {
  printf '%s\n' "$1"
}
vmimage_prepare_disk() {
  printf '%s|raw\n' "$1"
}
vmimage_disk_bus() {
  printf 'sata\n'
}
vmimage_net_device() {
  printf 'e1000e\n'
}
vmimage_machine_uuid() {
  printf '4e03054d-4449-4564-b903-38a385071a08\n'
}
vmimage_net_mac() {
  printf '52:54:00:12:34:56\n'
}
vmimage_disk_serial() {
  printf 'fixture-disk\n'
}
vmimage_ssh_base() {
  printf '2244\n'
}
vmimage_state_dir() {
  printf '%s\n' "$fixture"
}
vmimage_prepare_socket_dir() {
  export DOCKPIPE_VM_SOCKET_DIR="$fixture"
}
vmimage_clipboard_mode() {
  printf 'off\n'
}
vmimage_agent_enabled() {
  return 1
}
vmimage_secure_boot_mode() {
  printf 'off\n'
}
vmimage_tpm_mode() {
  printf 'off\n'
}
vmimage_default_accel() {
  printf 'tcg\n'
}
vmimage_default_cpu_model() {
  printf 'max\n'
}
vmimage_has_guest_automation() {
  return 1
}
vmimage_interactive_ssh_enabled() {
  return 1
}
vmimage_installer_display_mode() {
  printf 'none\n'
}
vmimage_log() {
  :
}
vmimage_run_installer_session() {
  shift
  printf '%s\0' "$@" > "$fixture/arguments"
}

export DOCKPIPE_VM_DISK="$fixture/disk.raw"
: > "$DOCKPIPE_VM_DISK"
unset DOCKPIPE_VM_SSH_USER DOCKPIPE_VM_FIRMWARE_CODE DOCKPIPE_VM_FIRMWARE_VARS
unset DOCKPIPE_VM_BIOS DOCKPIPE_VM_CDROM DOCKPIPE_VM_VIRTIO_ISO DOCKPIPE_VM_WINDOWS_UNATTEND_DIR
unset DOCKPIPE_VM_HOSTFWD DOCKPIPE_VM_BOOT_DISPLAY_WIDTH DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT
unset DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_WIDTH DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_HEIGHT
unset DOCKPIPE_RESOLVER_VM_PCI_DEVICES DOCKPIPE_RESOLVER_VM_GPU_PRIMARY

assert_arguments() {
  python3 - "$fixture/arguments" "$1" "$2" <<'PY'
import sys
from pathlib import Path

arguments = Path(sys.argv[1]).read_bytes().rstrip(b'\0').decode().split('\0')
devices = [arguments[index + 1] for index, value in enumerate(arguments[:-1]) if value == '-device']
standard = [device for device in devices if device.split(',')[0] == 'VGA']
expected = int(sys.argv[2])
assert len(standard) == expected, (standard, devices)
if expected:
    assert standard == ['VGA,xres=1920,yres=1080'], standard
assert not any(value.startswith(('VGA.xres=', 'VGA.yres=')) for value in arguments), arguments
passthrough = [device for device in devices if device.startswith('vfio-pci,')]
assert passthrough == ([sys.argv[3]] if sys.argv[3] else []), passthrough
PY
}

export DOCKPIPE_VM_PCI_DEVICES='' DOCKPIPE_VM_GPU_PRIMARY=off
vmimage_run_qemu_common qemu-kvm
assert_arguments 0 ''

export DOCKPIPE_VM_BOOT_DISPLAY_WIDTH=1920 DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT=1080
vmimage_run_qemu_common qemu-kvm
assert_arguments 1 ''

# Ordinary non-primary PCI passthrough retains the requested virtual display.
export DOCKPIPE_VM_PCI_DEVICES=0000:01:00.0
vmimage_run_qemu_common qemu-kvm
assert_arguments 1 'vfio-pci,host=0000:01:00.0'

# The existing primary-GPU selection remains authoritative; add no second GPU.
export DOCKPIPE_VM_GPU_PRIMARY=on
vmimage_run_qemu_common qemu-kvm
assert_arguments 0 'vfio-pci,host=0000:01:00.0,x-vga=on'

# A primary flag with no selected PCI device is not a passthrough selection.
export DOCKPIPE_VM_PCI_DEVICES=''
vmimage_run_qemu_common qemu-kvm
assert_arguments 1 ''
printf '%s\n' 'VM boot display assembled-argument cases passed'
