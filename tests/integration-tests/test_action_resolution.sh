#!/usr/bin/env bash
# Test explicit relative project actions and reject missing implicit library fallback.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE="$REPO_ROOT/src/bin/dockpipe"
source "$(dirname "${BASH_SOURCE[0]}")/helpers.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

git init -q -b dockpipe-it-action-resolution
git config user.email "test@dockpipe"
git config user.name "Test"
echo "initial" > initial.txt
git add initial.txt
git commit -q -m "initial"

# Missing project scripts must fail before running a command or committing.
initial_head=$(git rev-parse HEAD)
if "$DOCKPIPE" --workdir "$tmp" --action scripts/commit-worktree.sh \
  -- sh -c 'echo unexpected > rejected.txt' > missing-action.log 2>&1; then
  echo "Expected missing explicit action to be rejected"
  exit 1
fi
grep -q "action script not found" missing-action.log || { cat missing-action.log; exit 1; }
[[ ! -e rejected.txt ]] || { echo "Rejected action ran its command"; exit 1; }
[[ "$(git rev-parse HEAD)" == "$initial_head" ]] || { echo "Rejected action changed Git HEAD"; exit 1; }
rm missing-action.log

# A library helper is available only after explicit project materialization.
mkdir scripts
"$DOCKPIPE" action init "$tmp/scripts/commit-worktree.sh" --from commit-worktree
git add scripts/commit-worktree.sh
git commit -q -m "explicit project action"
"$DOCKPIPE" --workdir "$tmp" --action scripts/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=resolution test" \
  -- sh -c 'echo "resolved" > resolved.txt'

[[ -f "$tmp/resolved.txt" ]] || { echo "Expected resolved.txt"; exit 1; }
git -C "$tmp" log -1 --format=%s | grep -q "resolution test" || { echo "Expected commit message"; exit 1; }

echo "test_action_resolution OK"
