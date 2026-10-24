#!/usr/bin/env bash
set -euo pipefail
repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
source "$repository_root/src/core/assets/scripts/lib/vm-session-reporter.sh"
vmimage_is_windows_host() { return 1; }
vmimage_shell_path() { printf '%s\n' "$1"; }
vmimage_native_host_path() { printf '%s\n' "$1"; }
vmimage_die() { printf '%s\n' "$*" >&2; exit 1; }
vmimage_log() { printf '%s\n' "$*" >&2; }
vmimage_agent_port() { printf '47831\n'; }
vmimage_env_or_resolver() { printf 'powershell\n'; }
mkdir -p "$fixture/resolver/assets/tooling/bin"
cat > "$fixture/resolver/assets/tooling/bin/reporter" <<SCRIPT
#!/usr/bin/env bash
set -euo pipefail
[[ "\$1" == report ]]
[[ "\${DOCKPIPE_VM_EVENT_SCHEMA}" == 1 ]]
[[ "\${DOCKPIPE_VM_EVENT_LAUNCH_ID}" == launcher-fixture ]]
[[ "\${DOCKPIPE_VM_EVENT_SSH_PORT}" == 2474 ]]
[[ "\${DOCKPIPE_VM_EVENT_MACHINE_UUID}" == fixture-machine ]]
[[ ! -v UNRELATED_SECRET ]]
printf '%s:%s\\n' "\$DOCKPIPE_VM_EVENT_PHASE" "\$DOCKPIPE_VM_EVENT_PID" >> "$fixture/events"
SCRIPT
chmod +x "$fixture/resolver/assets/tooling/bin/reporter"
export DOCKPIPE_RESOLVER_PACKAGE_ROOT="$fixture/resolver"
export DOCKPIPE_RESOLVER_VM_SESSION_REPORTER=assets/tooling/bin/reporter
export DOCKPIPE_RUN_ID=fixture-run DOCKPIPE_WORKDIR="$fixture"
export DOCKPIPE_SURFACE_LAUNCH_ID=launcher-fixture
export DOCKPIPE_VM_SSH_USER=fixture-user UNRELATED_SECRET=must-not-inherit
vmimage_prepare_session_event 2474 fixture-machine
vmimage_report_session started 1234
vmimage_report_session exited 1234
expected=$'prepared:0\nstarted:1234\nexited:1234'
[[ "$(cat "$fixture/events")" == "$expected" ]]
for unsafe in ../reporter /tmp/reporter 'assets/../../reporter' 'C:/reporter'; do
  if (DOCKPIPE_RESOLVER_VM_SESSION_REPORTER="$unsafe" vmimage_prepare_session_reporter) >/dev/null 2>&1; then
    echo 'Unsafe reporter path accepted' >&2
    exit 1
  fi
done
ln -s reporter "$fixture/resolver/assets/tooling/bin/linked"
if (DOCKPIPE_RESOLVER_VM_SESSION_REPORTER=assets/tooling/bin/linked vmimage_prepare_session_reporter) >/dev/null 2>&1; then
  echo 'Linked reporter accepted' >&2
  exit 1
fi
printf 'VM session reporter contract passed\n'
