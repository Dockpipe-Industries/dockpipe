#!/usr/bin/env bash
set -euo pipefail
version="${DOCKPIPE_FLATPAK_VERSION:-}"
if [[ -z "$version" ]]; then
  # Use the same numeric candidate identity as release dry runs.
  # VERSION is a release-line baseline, not the current engine version.
  tags="$RUNNER_TEMP/dockpipe-qualification-tags.txt"
  gh api --paginate "repos/$GITHUB_REPOSITORY/tags?per_page=100" --jq '.[].name' > "$tags"
  python3 workflows/release/metadata/assets/scripts/release-version.py --base "$(cat VERSION)" --tags "$tags" > "$RUNNER_TEMP/dockpipe-qualification-version.txt"
  version="$(sed -n 's/^version=//p' "$RUNNER_TEMP/dockpipe-qualification-version.txt")"
fi
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
printf 'DOCKPIPE_FLATPAK_VERSION=%s\n' "$version" >> "$GITHUB_ENV"
export CGO_ENABLED=0
export XDG_STATE_HOME="$RUNNER_TEMP/dockpipe-state"
python3 workflows/shared/launch.py workflows/build/cli/config.yml -- --version "$version" --output "$RUNNER_TEMP/dockpipe"
