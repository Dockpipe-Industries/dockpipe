#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
launch="$root/packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/launch.sh"
# Load the real retry function without starting the stack's main program.
definition="$(sed -n '/^retry_with_backoff() {/,/^}/p' "$launch")"
[[ -n "$definition" ]]
eval "$definition"

calls=0
always_fails() {
  calls=$((calls + 1))
  return 23
}
status=0
retry_with_backoff fixture 3 0 always_fails >/dev/null 2>&1 || status=$?
[[ "$status" == 23 && "$calls" == 3 ]]

calls=0
succeeds_on_second_attempt() {
  calls=$((calls + 1))
  [[ "$calls" -ge 2 ]]
}
retry_with_backoff fixture 3 0 succeeds_on_second_attempt >/dev/null 2>&1
[[ "$calls" == 2 ]]
printf 'Pipeon stack retry preserves failure and stops after success\n'
