#!/usr/bin/env bash
set -euo pipefail
owner="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script_path="${DOCKPIPE_VM_RUNTIME_TEST_SCRIPT:-$owner/src/core/assets/scripts/vmimage-run.sh}"
fixture="$(mktemp -d /tmp/dockpipe-vm-test.XXXXXXXX)"
fixture="$(cd "$fixture" && pwd -P)"
fixture_owner_subshell="$BASH_SUBSHELL"
trap '
  if (( BASH_SUBSHELL == fixture_owner_subshell )); then
    rm -rf "$fixture"
  fi
' EXIT
mkdir -p "$fixture/scripts/lib" "$fixture/state"
cp "$script_path" "$fixture/scripts/"
cp "$owner"/src/core/assets/scripts/lib/*.sh "$fixture/scripts/lib/"
export DOCKPIPE_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:?select an actual pinned Dockpipe CLI}"
export DOCKPIPE_WORKDIR="$fixture" DOCKPIPE_VMIMAGE_SOURCE_ONLY=1
export XDG_STATE_HOME="$fixture/xdg-state" XDG_CACHE_HOME="$fixture/xdg-cache"
unset DOCKPIPE_PACKAGE_ROOT DOCKPIPE_PACKAGE_ID DOCKPIPE_PACKAGE_STATE_DIR DOCKPIPE_SDK_ROOT
# Execute the production pre-SDK admission prelude independently. The sentinel
# represents SDK sourcing and must remain unreachable for system Bash 3.2.
sed '/^SCRIPT_DIR=/,$d' "$fixture/scripts/vmimage-run.sh" > "$fixture/admission.sh"
source "$fixture/admission.sh"
for version in '3 2' '4 2'; do
  read -r major minor <<< "$version"
  if (
    vmimage_require_bash_version "$major" "$minor"
    touch "$fixture/sdk-reached"
  ) >"$fixture/out" 2>"$fixture/error"; then
    echo 'Unsupported Bash reached the SDK boundary' >&2
    exit 1
  fi
  [[ ! -e "$fixture/sdk-reached" ]]
  grep -q 'Bash 4.3 or newer' "$fixture/error"
done
vmimage_require_bash_version 4 3
vmimage_require_bash_version 5 0
source "$fixture/scripts/vmimage-run.sh"
actual_host="$(uname -s)"
if [[ "$actual_host" == Darwin ]] && /bin/bash -c '(( BASH_VERSINFO[0] < 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] < 3) ))'; then
  if /bin/bash "$fixture/scripts/vmimage-run.sh" >"$fixture/out" 2>"$fixture/error"; then
    echo 'Native system Bash unexpectedly reached SDK/runtime execution' >&2
    exit 1
  fi
  grep -q 'Bash 4.3 or newer' "$fixture/error"
fi
uname() {
  printf '%s\n' "$fixture_host"
}
fixture_host=Linux
[[ "$(vmimage_default_backend)" == qemu-kvm ]]
[[ "$(vmimage_default_accel)" == kvm ]]
[[ "$(vmimage_default_cpu_model)" == host ]]
mkdir -m 700 "$fixture/private-sockets"
(DOCKPIPE_VM_SOCKET_DIR="$fixture/private-sockets" vmimage_prepare_socket_dir)
chmod 755 "$fixture/private-sockets"
if (DOCKPIPE_VM_SOCKET_DIR="$fixture/private-sockets" vmimage_prepare_socket_dir) >"$fixture/out" 2>"$fixture/error"; then
  echo 'Public socket directory was accepted' >&2
  exit 1
fi
ln -s "$fixture/private-sockets" "$fixture/socket-alias"
if (DOCKPIPE_VM_SOCKET_DIR="$fixture/socket-alias" vmimage_prepare_socket_dir) >"$fixture/out" 2>"$fixture/error"; then
  echo 'Socket directory alias was accepted' >&2
  exit 1
fi
if (( EUID != 0 )); then
  if (DOCKPIPE_VM_SOCKET_DIR=/ vmimage_prepare_socket_dir) >"$fixture/out" 2>"$fixture/error"; then
    echo 'Foreign-owned socket directory was accepted' >&2
    exit 1
  fi
  grep -q 'ordinary and owned' "$fixture/error"
fi
fixture_host=Darwin
# A directory fits the 107-byte Linux bound but its full /monitor.sock endpoint
# exceeds Darwin's 103-byte pathname bound. Test actual allocated path bytes.
endpoint_directory="$fixture/$(printf '%*s' "$((92 - ${#fixture} - 1))" '' | tr ' ' x)"
mkdir -m 700 "$endpoint_directory"
if (DOCKPIPE_VM_SOCKET_DIR="$endpoint_directory" vmimage_prepare_socket_dir) >"$fixture/out" 2>"$fixture/error"; then
  echo 'Overlong Darwin endpoint was accepted' >&2
  exit 1
fi
grep -q '103-byte limit' "$fixture/error"
fixture_host=Linux
(DOCKPIPE_VM_SOCKET_DIR="$endpoint_directory" vmimage_prepare_socket_dir)
fixture_host=Darwin
[[ "$(vmimage_host_os)" == darwin ]]
[[ "$(vmimage_backend)" == qemu-darwin ]]
[[ "$(vmimage_default_accel)" == tcg ]]
[[ "$(vmimage_default_cpu_model)" == max ]]
[[ "$(vmimage_installer_display_mode)" == cocoa ]]
[[ "$(DOCKPIPE_VM_DISPLAY=none vmimage_installer_display_mode)" == none ]]
vmimage_should_prepare_builtin_bootstrap
vmimage_run_qemu_common() {
  printf '%s\n' "$1"
}
[[ "$(vmimage_run_qemu_darwin)" == qemu-darwin ]]
if (DOCKPIPE_VM_ACCEL=hvf vmimage_run_qemu_darwin) >"$fixture/out" 2>"$fixture/error"; then
  echo 'Unqualified HVF was accepted' >&2
  exit 1
fi
grep -q 'requires TCG' "$fixture/error"
if (DOCKPIPE_VM_PCI_DEVICES=0000:01:00.0 vmimage_run_qemu_darwin) >"$fixture/out" 2>"$fixture/error"; then
  echo 'Darwin PCI passthrough was accepted' >&2
  exit 1
fi
grep -q 'does not support host PCI' "$fixture/error"
fixture_host=Linux
if (vmimage_run_qemu_darwin) >"$fixture/out" 2>"$fixture/error"; then
  echo 'Darwin backend ran on a Linux host' >&2
  exit 1
fi
# Missing bounded reporter tooling must fail in dependency admission, before
# state preparation or QEMU launch. Supply only the two unrelated tool answers.
fixture_host=Darwin
mkdir "$fixture/no-tools"
if (
  vmimage_qemu_bin() {
    printf 'fixture-qemu\n'
  }
  vmimage_qemu_img_bin() {
    printf 'fixture-qemu-img\n'
  }
  export DOCKPIPE_VM_TPM=off DOCKPIPE_RESOLVER_VM_SESSION_REPORTER=fixture-reporter
  export PATH="$fixture/no-tools"
  vmimage_require_host_dependencies
  touch "$fixture/launch-reached"
) >"$fixture/out" 2>"$fixture/error"; then
  echo 'Missing timeout tooling reached launch' >&2
  exit 1
fi
[[ ! -e "$fixture/launch-reached" ]]
grep -q 'GNU timeout or gtimeout' "$fixture/error"
# Exercise the macOS command spelling with only gtimeout available. This runs
# a local reporter fixture; it neither launches a VM nor opens a connection.
mkdir "$fixture/reporter-tools"
printf '#!/bin/sh\n[ "$1" = --kill-after=1s ] && [ "$2" = 10s ] || exit 99\nshift 2\nexec "$@"\n' > "$fixture/reporter-tools/gtimeout"
chmod 700 "$fixture/reporter-tools/gtimeout"
ln -s /usr/bin/env "$fixture/reporter-tools/env"
printf '#!/bin/sh\nprintf reported > "%s/reported"\n' "$fixture" > "$fixture/reporter"
chmod 700 "$fixture/reporter"
vmimage_session_reporter_environment_names() {
  :
}
(
  export PATH="$fixture/reporter-tools"
  export DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE="$fixture/reporter"
  vmimage_report_session started 1234
)
[[ "$(cat "$fixture/reported")" == reported ]]
# Reload the real common orchestration, then replace external boundaries only.
source "$fixture/scripts/vmimage-run.sh"
fixture_host=Darwin
for operation in require_host_dependencies ensure_prompted_inputs prepare_state_split ensure_secure_boot_firmware confirm_user_supplied_media_rights confirm_host_network_exposure confirm_persistent_disk_use prompt_replace_missing_file prepare_session_event wait_for_guest maybe_start_clipboard_bridge sync_host_to_guest fetch_outputs keepalive_wait stop_swtpm report_session; do
  eval "vmimage_${operation}() { :; }"
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
  :
}
vmimage_qemu_bin() {
  printf '%s\n' "$fixture/qemu"
}
vmimage_prepare_disk() {
  printf '%s|qcow2\n' "$1"
}
vmimage_state_dir() {
  printf '%s\n' "$fixture/state"
}
vmimage_prepare_socket_dir() {
  export DOCKPIPE_VM_SOCKET_DIR="$fixture"
}
vmimage_machine_uuid() {
  printf 'test-uuid\n'
}
vmimage_net_mac() {
  printf '52:54:00:12:34:56\n'
}
vmimage_disk_serial() {
  printf 'test-serial\n'
}
vmimage_ssh_base() {
  printf '2474\n'
}
vmimage_secure_boot_mode() {
  printf 'off\n'
}
vmimage_tpm_mode() {
  printf 'off\n'
}
vmimage_agent_enabled() {
  return 1
}
vmimage_clipboard_mode() {
  printf 'off\n'
}
vmimage_has_guest_automation() {
  return 0
}
vmimage_interactive_ssh_enabled() {
  return 1
}
vmimage_guest_display_mode() {
  printf 'none\n'
}
vmimage_remote_run_internal() {
  printf 'guest-command\n' >> "$fixture/steps"
}
vmimage_write_pid_sidecar() {
  printf '%s\n' "$1" > "$fixture/owned-pid"
}
printf '#!/usr/bin/env bash\nprintf "%%s\\n" "$@" > "%s/args"\nexit 23\n' "$fixture" > "$fixture/qemu"
chmod 700 "$fixture/qemu"
touch "$fixture/disk.qcow2"
export DOCKPIPE_VM_DISK="$fixture/disk.qcow2" DOCKPIPE_STEP_CMD=true DOCKPIPE_RUN_ID=fixture
set +e
(
  set -e
  vmimage_run_qemu_darwin
) > "$fixture/stdout" 2> "$fixture/stderr"
result=$?
set -e
[[ "$result" == 23 ]]
grep -qx 'q35,accel=tcg' "$fixture/args"
grep -qx 'max' "$fixture/args"
grep -qx 'guest-command' "$fixture/steps"
if grep -qx -- '-daemonize' "$fixture/args"; then
  echo 'Darwin foreground lifecycle unexpectedly daemonized' >&2
  exit 1
fi
[[ -s "$fixture/owned-pid" ]]
printf 'PASS: Linux defaults, Darwin selection/admission, shared foreground image automation and native exit 23 (%s test host).\n' "$actual_host"
