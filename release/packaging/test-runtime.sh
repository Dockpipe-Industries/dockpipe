#!/usr/bin/env bash
# Dockpipe 0.6 runtime/package qualification. PipeLang compiler campaigns use
# tests/containedexec/ and are qualified separately for the language release.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
package_list="$(go list ./...)"
packages=()
while IFS= read -r package; do
  case "$package" in
    dockpipe/src/lib/pipelang|dockpipe/src/lib/pipelang/*|dockpipe/src/lib/applicationir|dockpipe/tests/pipelangcompat|dockpipe/tests/containedexec|dockpipe/tests/containedexec/*)
      printf 'Separate PipeLang containment qualification: %s\n' "$package"
      ;;
    "") ;;
    *) packages+=("$package") ;;
  esac
done <<< "$package_list"
[[ ${#packages[@]} -gt 0 ]] || { echo 'No runtime packages found' >&2; exit 1; }
go test "${packages[@]}"
