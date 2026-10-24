# shellcheck shell=bash
# Optional package-owned VM session reporting. The runtime supplies observed
# connection values; the installed provider owns its records and management UI.
vmimage_prepare_session_reporter() {
  local relative="${DOCKPIPE_RESOLVER_VM_SESSION_REPORTER:-}"
  [[ -n "$relative" ]] || return 0
  local root="${DOCKPIPE_RESOLVER_PACKAGE_ROOT:-}"
  [[ -n "$root" ]] || vmimage_die "VM session reporter requires its resolver package root"
  case "$relative" in
    /*|*\\*|*:*|../*|*/../*|*/..|.|..)
      vmimage_die "VM session reporter must be a contained relative executable"
      ;;
  esac
  root="$(vmimage_shell_path "$root")"
  local candidate="${root}/${relative}"
  if vmimage_is_windows_host && [[ ! -f "$candidate" && -f "${candidate}.exe" ]]; then
    candidate+=".exe"
  fi
  [[ -f "$candidate" && -x "$candidate" ]] || vmimage_die "Installed VM session reporter is missing or not executable"
  local ancestor="$candidate"
  while [[ "$ancestor" != / && "$ancestor" != . ]]; do
    [[ ! -L "$ancestor" ]] || vmimage_die "VM session reporter path must not contain links"
    ancestor="$(dirname "$ancestor")"
  done
  export DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE
  DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE="$(vmimage_native_host_path "$candidate")"
}

vmimage_session_reporter_environment_names() {
  printf '%s\n' \
    PATH HOME USERPROFILE LOCALAPPDATA APPDATA SYSTEMROOT WINDIR TEMP TMP \
    XDG_STATE_HOME XDG_CACHE_HOME XDG_CONFIG_HOME XDG_RUNTIME_DIR DOCKPIPE_BIN \
    DOCKPIPE_VM_EVENT_SCHEMA DOCKPIPE_VM_EVENT_PHASE DOCKPIPE_VM_EVENT_RUN_ID DOCKPIPE_VM_EVENT_LAUNCH_ID \
    DOCKPIPE_VM_EVENT_RESOURCE_ID DOCKPIPE_VM_EVENT_WORKDIR \
    DOCKPIPE_VM_EVENT_MACHINE_UUID DOCKPIPE_VM_EVENT_PID \
    DOCKPIPE_VM_EVENT_SSH_HOST DOCKPIPE_VM_EVENT_SSH_PORT \
    DOCKPIPE_VM_EVENT_SSH_USER DOCKPIPE_VM_EVENT_SSH_KEY_REFERENCE \
    DOCKPIPE_VM_EVENT_AGENT_PORT DOCKPIPE_VM_EVENT_EXEC_MODE
}

vmimage_report_session() {
  local phase="$1" process_id="${2:-0}"
  [[ -n "${DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE:-}" ]] || return 0
  export DOCKPIPE_VM_EVENT_PHASE="$phase" DOCKPIPE_VM_EVENT_PID="$process_id"
  if vmimage_is_windows_host; then
    local powershell helper
    powershell="$(vmimage_powershell_bin)"
    helper="$(vmimage_windows_qemu_helper)"
    "$powershell" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$helper" \
      -Action report -ReporterPhase "$phase" -ReporterPid "$process_id" ||
      vmimage_log "VM session reporting failed; management status is unavailable"
    return 0
  fi
  local timeout_executable
  timeout_executable="$(command -v timeout || command -v gtimeout || true)"
  if [[ -z "$timeout_executable" ]]; then
    vmimage_log "VM session reporting requires timeout; management status is unavailable"
    return 0
  fi
  local name
  local -a reporter_environment=()
  while IFS= read -r name; do
    if [[ -v "$name" ]]; then
      reporter_environment+=("${name}=${!name}")
    fi
  done < <(vmimage_session_reporter_environment_names)
  if ! "$timeout_executable" --kill-after=1s 10s env -i "${reporter_environment[@]}" \
    "$DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE" report; then
    vmimage_log "VM session reporting failed; management status is unavailable"
  fi
}

vmimage_prepare_session_event() {
  local ssh_port="$1" machine_uuid="$2"
  vmimage_prepare_session_reporter
  [[ -n "${DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE:-}" ]] || return 0
  export DOCKPIPE_VM_EVENT_SCHEMA=1
  export DOCKPIPE_VM_EVENT_RUN_ID="${DOCKPIPE_RUN_ID:-}"
  export DOCKPIPE_VM_EVENT_LAUNCH_ID="${DOCKPIPE_SURFACE_LAUNCH_ID:-}"
  export DOCKPIPE_VM_EVENT_RESOURCE_ID="dockpipe-vm-${DOCKPIPE_RUN_ID:-vm}"
  export DOCKPIPE_VM_EVENT_WORKDIR="$(vmimage_native_host_path "${DOCKPIPE_WORKDIR:-}")"
  export DOCKPIPE_VM_EVENT_MACHINE_UUID="$machine_uuid"
  export DOCKPIPE_VM_EVENT_SSH_HOST="${DOCKPIPE_VM_SSH_HOST:-127.0.0.1}"
  export DOCKPIPE_VM_EVENT_SSH_PORT="$ssh_port"
  export DOCKPIPE_VM_EVENT_SSH_USER="${DOCKPIPE_VM_SSH_USER:-}"
  export DOCKPIPE_VM_EVENT_SSH_KEY_REFERENCE="${DOCKPIPE_VM_WINDOWS_SSH_KEY:-}"
  if [[ -n "$DOCKPIPE_VM_EVENT_SSH_KEY_REFERENCE" ]]; then
    DOCKPIPE_VM_EVENT_SSH_KEY_REFERENCE="$(vmimage_native_host_path "$DOCKPIPE_VM_EVENT_SSH_KEY_REFERENCE")"
  fi
  export DOCKPIPE_VM_EVENT_AGENT_PORT="$(vmimage_agent_port)"
  export DOCKPIPE_VM_EVENT_EXEC_MODE="$(vmimage_env_or_resolver 'DOCKPIPE_VM_EXEC_MODE' 'DOCKPIPE_RESOLVER_VM_EXEC_MODE' 'raw')"
  vmimage_report_session prepared
}
