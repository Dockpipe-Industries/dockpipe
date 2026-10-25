#!/usr/bin/env bash
# Smoke test: run a trivial command in the default image (builds if needed).
# Requires Docker. Skip if docker not available.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="${DOCKPIPE_TEST_DOCKPIPE_BIN:-${REPO_ROOT}/src/bin/dockpipe}"

if ! command -v docker &>/dev/null; then
  echo "smoke: Docker not found, skip"
  exit 0
fi

if ! docker info &>/dev/null; then
  echo "smoke: Docker daemon not reachable, skip"
  exit 0
fi

# The repository is the mounted workspace; the default image context is library-owned.
cd "$REPO_ROOT"
if out=$("$CLI" -- ls -la /work 2>&1); then
  status=0
else
  status=$?
fi
if [[ "$status" -ne 0 ]]; then
  printf 'smoke FAIL: default container command exited %s\n%s\n' "$status" "$out" >&2
  exit "$status"
fi
if echo "$out" | grep -q "Dockerfile\|entrypoint\|README"; then
  echo "smoke OK (ls /work showed repo contents)"
else
  echo "smoke FAIL: unexpected output"
  echo "$out"
  exit 1
fi
