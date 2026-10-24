#!/usr/bin/env bash
set -euo pipefail

repository_root="${DOCKPIPE_VM_CLEANUP_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
script_path="${DOCKPIPE_VM_RUNTIME_TEST_SCRIPT:-$repository_root/src/core/assets/scripts/vmimage-run.sh}"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
export DOCKPIPE_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:?select an actual pinned Dockpipe CLI}"
export DOCKPIPE_WORKDIR="$fixture" DOCKPIPE_VMIMAGE_SOURCE_ONLY=1 DOCKPIPE_STEP_CMD=true
export XDG_STATE_HOME="$fixture/state" XDG_CACHE_HOME="$fixture/cache"
export cleanup_script="$script_path" cleanup_fixture="$fixture"
unset DOCKPIPE_PACKAGE_ROOT DOCKPIPE_PACKAGE_ID DOCKPIPE_PACKAGE_STATE_DIR DOCKPIPE_SDK_ROOT

cat > "$fixture/backend" <<'BACKEND'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$case_mode" == stubborn ]]; then
  trap '' TERM
fi
printf '%s\n' "$$" > "$case_directory/backend.pid"
case "$case_mode" in
  completed) exit 0 ;;
  backend-error) exit 37 ;;
  *) exec sleep 60 ;;
esac
BACKEND
chmod +x "$fixture/backend"

cat > "$fixture/driver" <<'DRIVER'
#!/usr/bin/env bash
set -euo pipefail
source "$cleanup_script"
vmimage_state_dir() {
  printf '%s\n' "$case_directory"
}
vmimage_log() {
  :
}
vmimage_write_pid_sidecar() {
  printf '%s\n' "$1" > "$case_directory/owned.pid"
}
vmimage_stop_swtpm() {
  printf 'stopped\n' >> "$case_directory/tpm"
}
vmimage_require() {
  :
}
vmimage_wait_for_guest() {
  if [[ "$case_mode" == readiness-error ]]; then
    return 31
  fi
}
vmimage_maybe_start_clipboard_bridge() {
  :
}
vmimage_sync_host_to_guest() {
  :
}
vmimage_interactive_ssh_enabled() {
  return 1
}
vmimage_remote_run_internal() {
  :
}
vmimage_fetch_outputs() {
  :
}
vmimage_keepalive_wait() {
  :
}
kill() {
  printf '%s\n' "$*" >> "$case_directory/signals"
  builtin kill "$@"
}
trap 'printf "caller-exit\n" > "$case_directory/caller-trap"' EXIT
"$session_function" "$cleanup_fixture/backend"
# A later failure must not retain cleanup for a PID already reaped by the session.
printf 'returned\n' > "$case_directory/returned"
exit 43
DRIVER

run_case() {
  local function_name="$1" mode="$2" expected="$3"
  local directory="$fixture/$function_name-$mode"
  mkdir "$directory"
  export case_directory="$directory" case_mode="$mode" session_function="$function_name"
  local status=0
  bash "$fixture/driver" > "$directory/stdout" 2> "$directory/stderr" || status=$?
  if [[ "$status" != "$expected" ]]; then
    cat "$directory/stderr" >&2
    printf 'Unexpected %s/%s status: %s, wanted %s\n' "$function_name" "$mode" "$status" "$expected" >&2
    exit 1
  fi
  [[ "$(cat "$directory/tpm")" == stopped ]]
  ! grep -q 'unbound variable' "$directory/stderr"
  if [[ "$mode" == completed || "$mode" == backend-error ]]; then
    [[ ! -e "$directory/signals" ]]
  else
    local pid
    pid="$(cat "$directory/owned.pid")"
    if builtin kill -0 "$pid" 2>/dev/null; then
      echo 'Owned child survived failed session cleanup' >&2
      exit 1
    fi
  fi
  if [[ "$mode" == completed ]]; then
    [[ -f "$directory/returned" && -f "$directory/caller-trap" ]]
  fi
}

for session_function in vmimage_run_installer_session vmimage_run_automated_installer_session vmimage_run_headless_guest_session; do
  run_case "$session_function" completed 43
  run_case "$session_function" backend-error 37
  if [[ "$session_function" != vmimage_run_installer_session ]]; then
    run_case "$session_function" readiness-error 31
  fi
  for cancellation_mode in cancelled stubborn; do
    directory="$fixture/$session_function-$cancellation_mode"
    mkdir "$directory"
    export case_directory="$directory" case_mode="$cancellation_mode" session_function
    bash "$fixture/driver" > "$directory/stdout" 2> "$directory/stderr" &
    controller=$!
    for (( attempt = 0; attempt < 100; attempt++ )); do
      [[ -f "$directory/backend.pid" ]] && break
      sleep 0.02
    done
    [[ -f "$directory/backend.pid" ]]
    builtin kill -TERM "$controller"
    status=0
    wait "$controller" || status=$?
    [[ "$status" == 143 ]]
    [[ "$(cat "$directory/tpm")" == stopped ]]
    pid="$(cat "$directory/backend.pid")"
    ! builtin kill -0 "$pid" 2>/dev/null
    ! grep -q 'unbound variable' "$directory/stderr"
    if [[ "$cancellation_mode" == stubborn ]]; then
      grep -q -- '-KILL' "$directory/signals"
    fi
  done
done

# A stale numeric PID must not be signalled once the owned child was reaped.
mkdir "$fixture/post-reap"
export case_directory="$fixture/post-reap"
bash -c '
  set -euo pipefail
  source "$cleanup_script"
  vmimage_stop_swtpm() {
    printf "stopped\n" > "$case_directory/tpm"
  }
  kill() {
    printf "%s\n" "$*" > "$case_directory/signals";
    return 1
  }
  sleep 0.01 &
  owned_pid=$!
  wait "$owned_pid"
  vmimage_cleanup_posix_qemu "$owned_pid"
'
[[ ! -e "$case_directory/signals" && -f "$case_directory/tpm" ]]
# A child that completed without an explicit wait is also no longer live.
mkdir "$fixture/completed-unwaited"
export case_directory="$fixture/completed-unwaited"
bash -c '
  set -euo pipefail
  source "$cleanup_script"
  vmimage_stop_swtpm() {
    :
  }
  kill() {
    printf "%s\n" "$*" > "$case_directory/signals";
    return 1
  }
  sleep 0.01 &
  owned_pid=$!
  sleep 0.1
  vmimage_cleanup_posix_qemu "$owned_pid"
'
[[ ! -e "$case_directory/signals" ]]

# Failed termination must exhaust a bounded poll, report failure, and never wait
# on a still-running child. The fixture then explicitly reaps its own sleeper.
mkdir "$fixture/termination-failure"
export case_directory="$fixture/termination-failure"
bash -c '
  set -euo pipefail
  source "$cleanup_script"
  vmimage_log() {
    printf "%s\n" "$*" >> "$case_directory/diagnostic"
  }
  vmimage_stop_swtpm() {
    printf "stopped\n" > "$case_directory/tpm"
  }
  /bin/sleep 60 &
  owned_pid=$!
  kill() {
    :
  }
  sleep() {
    printf "poll\n" >> "$case_directory/polls"
  }
  wait() {
    printf "unexpected wait\n" > "$case_directory/wait";
    return 1
  }
  status=0
  vmimage_cleanup_posix_qemu "$owned_pid" || status=$?
  [[ "$status" == 1 ]]
  [[ ! -e "$case_directory/wait" ]]
  unset -f kill sleep wait
  builtin kill -KILL "$owned_pid"
  builtin wait "$owned_pid" 2>/dev/null || true
'
[[ "$(wc -l < "$case_directory/polls" | tr -d " ")" == 70 ]]
[[ -f "$case_directory/tpm" && -f "$case_directory/diagnostic" ]]
printf 'POSIX QEMU cleanup contracts passed (17 scenarios)\n'

