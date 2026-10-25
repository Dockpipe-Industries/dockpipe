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
wrapper_source="$GITHUB_WORKSPACE/workflows/ci/dockpipe-repo-quality"
wrapper_work="$RUNNER_TEMP/main-owned"
wrapper_target="$wrapper_work/workflows/dockpipe-repo-quality"
mkdir -p "$wrapper_target/assets/flatpak" "$wrapper_target/assets/scripts"
cp "$wrapper_source/config.yml" "$wrapper_source/package.yml" "$wrapper_target/"
cp "$wrapper_source/assets/source-lock.json" "$wrapper_target/assets/"
cp "$wrapper_source/assets/flatpak/package.json" "$wrapper_target/assets/flatpak/"
cp "$wrapper_source/assets/scripts/repo-quality-pinned.py" "$wrapper_target/assets/scripts/"
env -u DOCKPIPE_PACKAGES_ROOT "$RUNNER_TEMP/dockpipe" package compile workflows --workdir "$wrapper_work" --from "$wrapper_work/workflows" --force
env -u DOCKPIPE_PACKAGES_ROOT "$RUNNER_TEMP/dockpipe" package build store --workdir "$wrapper_work" --only workflows --out "$RUNNER_TEMP/main-store" --version "$version"
python3 workflows/catalog/compose/assets/scripts/ingest-pinned-stores.py \
  --lock workflows/catalog/compose/assets/pinned-stores.lock.json \
  --platform linux-amd64 \
  --main-store "$RUNNER_TEMP/main-store" \
  --out "$RUNNER_TEMP/native-store"
