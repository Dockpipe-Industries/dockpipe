#!/usr/bin/env bash
# Lightweight guardrail: fail if docs/templates reintroduce pre-assets paths or removed legacy dirs.
# Run from repo root: bash workflows/ci/repository-layout/assets/scripts/check-paths.sh
set -euo pipefail

root="$(cd "$(dirname "$0")/../../../../.." && pwd)"
cd "$root"

# Regexes only — do not spell obsolete paths verbatim in comments or this file will self-match.
OBSOLETE_SCRIPTS_IMAGES='(src/core|src/templates|templates)/core/(scripts|images)/'
OBSOLETE_LEGACY_DIRS='(src/core|src/templates|templates)/core/workflows/|(src/templates|templates)/run-worktree/'
GENERATED_STATE_ROOT='bin/[.]dockpipe'
DORKPIPE_CI_STATE_PATHS="${GENERATED_STATE_ROOT}/(workflows/[^/]+/(dorkpipe|ci-raw|ci-analysis)|packages/dorkpipe/ci)"
DORKPIPE_CI_WORKFLOW_SDK='dockpipe_sdk[[:space:]]+path[[:space:]]+workflow[[:space:]]+ci[[:space:]]+dorkpipe'
DORKPIPE_CI_PACKAGE_SDK='dockpipe_sdk[[:space:]]+path[[:space:]]+package[[:space:]]+dorkpipe[[:space:]]+ci'

fail() {
  echo "check-templates-core-paths: FAIL — $1" >&2
  exit 1
}

source_paths=()
for path in docs README.md AGENTS.md CONTRIBUTING.md src/lib src/cmd src/core templates; do
  [[ -e "$path" ]] && source_paths+=("$path")
done

check_source_paths() {
  local pattern="$1"
  local message="$2"
  local matches status

  if ((${#source_paths[@]} == 0)); then
    return
  fi

  if matches="$(grep -rEnH --include='*.md' --include='*.go' "$pattern" "${source_paths[@]}")"; then
    printf '%s\n' "$matches" >&2
    fail "$message"
  else
    status=$?
    if ((status != 1)); then
      fail 'could not scan repository source paths'
    fi
  fi
}

check_source_paths "$OBSOLETE_SCRIPTS_IMAGES" \
  'found pre-assets templates/core scripts or images path; use templates/core/assets/…'
check_source_paths "$OBSOLETE_LEGACY_DIRS" \
  'found obsolete workflows/ or run-worktree/ under templates; see docs and workflow_dirs.go'

package_hits="$(
  grep -rInE "${DORKPIPE_CI_STATE_PATHS}|${DORKPIPE_CI_WORKFLOW_SDK}" \
    --include='*.go' \
    --include='*.sh' \
    --include='*.md' \
    --include='*.json' \
    --include='*.yml' \
    --include='*.yaml' \
    --include='*.ts' \
    --include='*.js' \
    packages 2>/dev/null || true
)"
package_sdk_hits="$(
  grep -rInE "${DORKPIPE_CI_PACKAGE_SDK}" \
    --include='*.sh' \
    --include='*.md' \
    packages 2>/dev/null || true
)"
if [[ -n "$package_sdk_hits" ]]; then
  package_hits="${package_hits}${package_hits:+$'\n'}${package_sdk_hits}"
fi
if [[ -n "$package_hits" ]]; then
  printf '%s\n' "$package_hits" >&2
  fail 'package-owned files must not hardcode DorkPipe CI generated-state paths; use dockpipe_sdk ci raw/analysis, Go statepath helpers, or explicit DOCKPIPE_CI_* user inputs'
fi

exit 0
