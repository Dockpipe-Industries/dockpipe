#!/usr/bin/env bash
set -euo pipefail

PACKAGE_ROOT="${DOCKPIPE_PACKAGE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
REPO_ROOT="$(cd "${PACKAGE_ROOT}/../.." && pwd)"
TEST_ROOT="$(mktemp -d)"
trap 'find "$TEST_ROOT" -mindepth 1 -delete; rmdir "$TEST_ROOT"' EXIT

WORKDIR="${TEST_ROOT}/checkout"
LEGACY_PACKAGE_ROOT="${WORKDIR}/bin/.dockpipe/packages/vm"
LEGACY_VM_ROOT="${LEGACY_PACKAGE_ROOT}/vmimage"
mkdir -p "${LEGACY_VM_ROOT}/identity" "${LEGACY_VM_ROOT}/tpm-old" "${TEST_ROOT}/durable" "${TEST_ROOT}/runtime"
printf 'legacy-tpm\n' > "${LEGACY_VM_ROOT}/tpm-old/tpm2-00.permall"
printf 'transient\n' > "${LEGACY_VM_ROOT}/tpm-old/swtpm.log"

STATE_HELPER="${TEST_ROOT}/state-helper.sh"
cat > "$STATE_HELPER" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$DOCKPIPE_VM_TEST_HELPER_LOG"
instance=""
run_id=""
while (( $# > 0 )); do
  case "$1" in
    --instance) instance="$2"; shift 2 ;;
    --run) run_id="$2"; shift 2 ;;
    *) shift ;;
  esac
done
[[ -n "$instance" && -n "$run_id" ]]
if [[ "${DOCKPIPE_VM_TEST_MALFORMED_HELPER:-}" == "1" ]]; then
  printf 'durable\truntime\tfalse\tfalse\textra\n'
  exit 0
fi
instance_digest="$(printf '%s' "$instance" | sha256sum | cut -d' ' -f1)"
run_digest="$(printf '%s' "$run_id" | sha256sum | cut -d' ' -f1)"
durable="${DOCKPIPE_VM_TEST_DURABLE_ROOT}/${instance_digest}"
runtime="${DOCKPIPE_VM_TEST_RUNTIME_ROOT}/${instance_digest}/${run_digest}"
mkdir -p "$durable" "$runtime"
chmod 700 "$durable" "$runtime"
printf '%s\t%s\tfalse\tfalse\n' "$durable" "$runtime"
EOF
chmod 700 "$STATE_HELPER"

export DOCKPIPE_WORKDIR="$WORKDIR"
export DOCKPIPE_PACKAGE_STATE_DIR="$LEGACY_PACKAGE_ROOT"
export DOCKPIPE_VM_DISK="${WORKDIR}/images/windows.qcow2"
export DOCKPIPE_RUN_ID="run-one"
export DOCKPIPE_VM_BACKEND=qemu-kvm
export DOCKPIPE_VMIMAGE_SOURCE_ONLY=1
export DOCKPIPE_VM_STATE_HELPER="$STATE_HELPER"
export DOCKPIPE_VM_TEST_HELPER_LOG="${TEST_ROOT}/helper.log"
export DOCKPIPE_VM_TEST_DURABLE_ROOT="${TEST_ROOT}/durable"
export DOCKPIPE_VM_TEST_RUNTIME_ROOT="${TEST_ROOT}/runtime"

# shellcheck source=/dev/null
source "${REPO_ROOT}/src/core/assets/scripts/vmimage-run.sh"

GENERATION_COUNT="${TEST_ROOT}/generation-count"
printf '0\n' > "$GENERATION_COUNT"
vmimage_test_generate() {
  local label="$1" count
  count="$(cat "$GENERATION_COUNT")"
  count=$((count + 1))
  printf '%s\n' "$count" >| "$GENERATION_COUNT"
  printf '%s-%s\n' "$label" "$count"
}
vmimage_uuid_generate() { vmimage_test_generate uuid; }
vmimage_mac_generate() { vmimage_test_generate mac; }
vmimage_serial_generate() { vmimage_test_generate serial; }

SSH_GENERATION_COUNT="${TEST_ROOT}/ssh-generation-count"
printf '0\n' > "$SSH_GENERATION_COUNT"
ssh-keygen() {
  local output="" index
  if [[ "${1:-}" == "-y" ]]; then
    printf 'ssh-ed25519 durable-public-key\n'
    return 0
  fi
  for ((index = 1; index <= $#; index++)); do
    if [[ "${!index}" == "-f" ]]; then
      index=$((index + 1))
      output="${!index}"
      break
    fi
  done
  [[ -n "$output" ]]
  printf 'durable-private-key\n' > "$output"
  printf 'ssh-ed25519 durable-public-key\n' > "${output}.pub"
  local count
  count="$(cat "$SSH_GENERATION_COUNT")"
  printf '%s\n' "$((count + 1))" > "$SSH_GENERATION_COUNT"
}

vmimage_prepare_state_split
first_durable="$(vmimage_durable_state_dir)"
first_runtime="$(vmimage_state_dir)"
first_uuid="$(vmimage_machine_uuid)"
first_mac="$(vmimage_net_mac)"
first_serial="$(vmimage_disk_serial)"
vmimage_windows_ensure_admin_password
first_password_hash="$(printf '%s' "$DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD" | sha256sum | cut -d' ' -f1)"
vmimage_windows_ensure_ssh_key
first_ssh_hash="$(sha256sum "$DOCKPIPE_VM_WINDOWS_SSH_KEY" | cut -d' ' -f1)"

[[ "$(vmimage_secure_boot_vars_copy_path)" == "${first_durable}/firmware/vars.fd" ]]
[[ "$(vmimage_windows_unattend_dir)" == "${first_runtime}/windows-unattend-windows.qcow2" ]]
[[ "$(vmimage_windows_bootstrap_dir)" == "${first_runtime}/windows-bootstrap-windows.qcow2-run-one" ]]
[[ "$(vmimage_windows_sync_archive_local_path)" == "${first_runtime}/sync-run-one.zip" ]]
grep -Fq -- '--file identity/windows.qcow2.uuid=identity/guest.uuid' "$DOCKPIPE_VM_TEST_HELPER_LOG"
grep -Fq -- '--file ovmf-vars-windows.qcow2.fd=firmware/vars.fd' "$DOCKPIPE_VM_TEST_HELPER_LOG"
grep -Fq -- '--file windows-admin-password-windows.qcow2.txt=credentials/windows-admin-password.txt' "$DOCKPIPE_VM_TEST_HELPER_LOG"
grep -Fq -- '--tree tpm-old=tpm --ignore tpm-old/swtpm.sock --ignore tpm-old/swtpm.log' "$DOCKPIPE_VM_TEST_HELPER_LOG"

unset DOCKPIPE_VM_DURABLE_STATE_DIR DOCKPIPE_VM_RUNTIME_STATE_DIR DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD DOCKPIPE_VM_WINDOWS_SSH_KEY DOCKPIPE_VM_WINDOWS_SSH_PUBKEY
export DOCKPIPE_RUN_ID="run-two"
vmimage_prepare_state_split
second_durable="$(vmimage_durable_state_dir)"
second_runtime="$(vmimage_state_dir)"
[[ "$second_durable" == "$first_durable" ]]
[[ "$second_runtime" != "$first_runtime" ]]
[[ "$(vmimage_machine_uuid)" == "$first_uuid" ]]
[[ "$(vmimage_net_mac)" == "$first_mac" ]]
[[ "$(vmimage_disk_serial)" == "$first_serial" ]]
[[ "$(cat "$GENERATION_COUNT")" == 3 ]]
vmimage_windows_ensure_admin_password
[[ "$(printf '%s' "$DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD" | sha256sum | cut -d' ' -f1)" == "$first_password_hash" ]]
vmimage_windows_ensure_ssh_key
[[ "$(sha256sum "$DOCKPIPE_VM_WINDOWS_SSH_KEY" | cut -d' ' -f1)" == "$first_ssh_hash" ]]
[[ "$(cat "$SSH_GENERATION_COUNT")" == 1 ]]

rm "${DOCKPIPE_VM_WINDOWS_SSH_KEY}.pub"
unset DOCKPIPE_VM_WINDOWS_SSH_KEY DOCKPIPE_VM_WINDOWS_SSH_PUBKEY
vmimage_windows_ensure_ssh_key
[[ "$(cat "$SSH_GENERATION_COUNT")" == 1 ]]
[[ "$DOCKPIPE_VM_WINDOWS_SSH_PUBKEY" == 'ssh-ed25519 durable-public-key' ]]

uuid_path="${first_durable}/identity/guest.uuid"
uuid_value="$(cat "$uuid_path")"
: > "$uuid_path"
if (vmimage_machine_uuid >/dev/null 2>&1); then
  echo 'empty durable VM identity must fail closed' >&2
  exit 1
fi
printf '%s\n' "$uuid_value" > "$uuid_path"

if (DOCKPIPE_VM_TEST_MALFORMED_HELPER=1 DOCKPIPE_VM_DURABLE_STATE_DIR=untrusted DOCKPIPE_VM_RUNTIME_STATE_DIR=untrusted vmimage_prepare_state_split >/dev/null 2>&1); then
  echo 'malformed helper output and caller-provided state roots must fail closed' >&2
  exit 1
fi

if find "$first_durable" -type f ! -perm 0600 -print -quit | grep -q .; then
  echo 'durable VM identity files must be owner-only' >&2
  exit 1
fi
if find "$first_durable" -type d ! -perm 0700 -print -quit | grep -q .; then
  echo 'durable VM identity directories must be owner-only' >&2
  exit 1
fi

unset DOCKPIPE_VM_DURABLE_STATE_DIR DOCKPIPE_VM_RUNTIME_STATE_DIR
export DOCKPIPE_VM_DISK="${WORKDIR}/other/windows.qcow2"
vmimage_prepare_state_split
[[ "$(vmimage_durable_state_dir)" != "$first_durable" ]]

mkdir -p "${LEGACY_VM_ROOT}/tpm-second"
unset DOCKPIPE_VM_DURABLE_STATE_DIR DOCKPIPE_VM_RUNTIME_STATE_DIR
if (vmimage_prepare_state_split >/dev/null 2>&1); then
  echo 'ambiguous legacy TPM directories must fail closed' >&2
  exit 1
fi

# shellcheck disable=SC2016
grep -Fq 'tpm_dir="$(vmimage_durable_state_dir)/tpm"' "${REPO_ROOT}/src/core/assets/scripts/vmimage-run.sh"
# shellcheck disable=SC2016
grep -Fq 'sock="${state_dir}/swtpm.sock"' "${REPO_ROOT}/src/core/assets/scripts/vmimage-run.sh"
# shellcheck disable=SC2016
grep -Fq 'log_path="${state_dir}/swtpm.log"' "${REPO_ROOT}/src/core/assets/scripts/vmimage-run.sh"
# shellcheck disable=SC2016
grep -Fq 'overlay="${state_dir}/overlay-${DOCKPIPE_RUN_ID:-vm}.qcow2"' "${REPO_ROOT}/src/core/assets/scripts/vmimage-run.sh"
# shellcheck disable=SC2016
grep -Fq 'pidfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.pid"' "${REPO_ROOT}/src/core/assets/scripts/vmimage-run.sh"

echo '[vm] durable identity/runtime state split tests passed'
