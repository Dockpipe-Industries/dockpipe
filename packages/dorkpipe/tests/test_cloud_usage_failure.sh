#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${script_dir}/../resolvers/dorkpipe/assets/scripts/orchestrate-common.sh"
test_root="$(mktemp -d)"
trap 'rm -rf "${test_root}"' EXIT
export DORKPIPE_ORCH_CLOUD_USAGE_JSON="${test_root}/cloud-usage.json"
export DORKPIPE_ORCH_HALT_JSON="${test_root}/halt.json"
printf 'damaged ledger\n' > "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}"
cp "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}" "${test_root}/original"
dorkpipe_orchestrate_read_usage_number() { return 9; }
dorkpipe_orchestrate_read_provider_usage_number() { return 9; }
# The lock invokes the function in an OR-list, where errexit cannot protect it.
if dorkpipe_orchestrate_record_cloud_usage codex 12 34 56; then
  echo 'Accounting swallowed a failed ledger read' >&2
  exit 1
fi
cmp "${test_root}/original" "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}"
if dorkpipe_orchestrate_halt_run codex 'failed accounting'; then
  echo 'Halt rewrite swallowed a failed ledger read' >&2
  exit 1
fi
cmp "${test_root}/original" "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}"
[[ ! -d "${DORKPIPE_ORCH_CLOUD_USAGE_JSON}.lock" ]]
echo 'Cloud usage failure propagation passed'
