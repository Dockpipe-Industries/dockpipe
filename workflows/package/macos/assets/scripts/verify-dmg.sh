#!/usr/bin/env bash
set -euo pipefail

image="${1:?existing DMG to verify}"
if [[ ! -f "$image" || ! -s "$image" || -L "$image" ]]; then
  echo 'DMG verification requires a nonempty regular image' >&2
  exit 1
fi

# DiskImages can still report EAGAIN immediately after a successful create.
# Retry only that measured transient failure, never a checksum or format error.
work="$(mktemp -d "${TMPDIR:-/tmp}/dockpipe-dmg-verify.XXXXXX")"
cleanup() {
  status=$?
  if (( status == 0 )); then
    rm -rf "$work"
  else
    echo "DMG verification evidence retained at $work" >&2
  fi
}
trap cleanup EXIT

for attempt in 1 2 3; do
  log="$work/attempt-$attempt.log"
  if hdiutil verify "$image" >"$log" 2>&1; then
    cat "$log"
    exit 0
  else
    status=$?
    cat "$log" >&2
    if (( status != 1 || attempt == 3 )) \
      || ! grep -Fxq 'hdiutil: verify failed - Resource temporarily unavailable' "$log"; then
      exit "$status"
    fi
    delay=$((attempt * 2))
    echo "hdiutil verification reported Resource temporarily unavailable; retrying in ${delay}s (attempt $((attempt + 1))/3)" >&2
    sleep "$delay"
  fi
done
