#!/usr/bin/env bash
set -euo pipefail

source_directory="${1:?installer source directory}"
output="${2:?new output DMG}"
[[ -d "$source_directory" ]] || { echo 'DMG source directory is missing' >&2; exit 1; }
[[ ! -e "$output" && ! -L "$output" ]] || { echo 'DMG output already exists' >&2; exit 1; }

# Keep attempts beside the final output so an exclusive hard link can publish it
# without replacing an existing file. Never detach or modify unrelated volumes.
work="$(mktemp -d "${output}.work.XXXXXX")"
cleanup() {
  status=$?
  if (( status == 0 )); then
    rm -rf "$work"
  else
    echo "DMG creation evidence retained at $work" >&2
  fi
}
trap cleanup EXIT

for attempt in 1 2 3; do
  candidate="$work/attempt-$attempt.dmg"
  log="$work/attempt-$attempt.log"
  if hdiutil create -volname Dockpipe -srcfolder "$source_directory" -format UDZO "$candidate" >"$log" 2>&1; then
    cat "$log"
    [[ -f "$candidate" && -s "$candidate" && ! -L "$candidate" ]] || { echo 'hdiutil produced no regular DMG' >&2; exit 1; }
    python3 - "$candidate" "$output" <<'PY'
import os
import sys

os.link(sys.argv[1], sys.argv[2])
PY
    exit 0
  else
    status=$?
    cat "$log" >&2
    if (( status != 1 || attempt == 3 )) \
      || ! grep -Fxq 'hdiutil: create failed - Resource busy' "$log"; then
      exit "$status"
    fi
    delay=$((attempt * 2))
    echo "hdiutil reported Resource busy; retrying with a new image in ${delay}s (attempt $((attempt + 1))/3)" >&2
    sleep "$delay"
  fi
done
