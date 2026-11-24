#!/usr/bin/env bash
# Run project shell/unit tests. Exit 0 if all pass. From repo root: bash tests/run_tests.sh
# Integration tests (Docker): bash tests/integration-tests/run.sh
# Dockpipe-owned package/workflow hooks live behind `dockpipe test`.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UNIT="$(cd "${ROOT}/tests/unit-tests" && pwd)"
CLI="${ROOT}/src/bin/dockpipe"
export DOCKPIPE_TEST_DOCKPIPE_BIN="$CLI"
failed=0

for f in test_cli.sh test_repo_root.sh test_repo_layout.sh test_clone_worktree_include.sh; do
  if [[ -f "$UNIT/$f" ]]; then
    echo "--- $f ---"
    bash "$UNIT/$f" || failed=1
  fi
done

test_package_build() (
  build_root="$(mktemp -d)" || exit 1
  trap 'rm -rf -- "$build_root"' EXIT

  cp -a "$ROOT/dockpipe.config.json" "$ROOT/VERSION" "$ROOT/workflows" "$build_root/" || exit 1

  # The two native controller qualifications require a separately selected
  # PipeLang SDK and an explicit build-control step before they can be packaged.
  for workflow in sdk-pipelang workflow-control; do
    directory="$build_root/workflows/test/$workflow"
    mv "$directory/config.yml" "$directory/qualification.yml" || exit 1
  done

  echo "--- dockpipe build --no-images ---"
  "$CLI" build --workdir "$build_root" --no-images || exit 1

  echo "--- dockpipe test ---"
  "$CLI" test --workdir "$build_root"
)

test_package_build || failed=1

echo "--- smoke.sh (needs Docker) ---"
bash "$UNIT/smoke.sh" || failed=1

echo "--- test_deb_install.sh (needs Docker + .deb) ---"
_deb="$(echo "$ROOT/release/artifacts"/dockpipe_*_amd64.deb 2>/dev/null)"
_can_docker=0
if command -v docker &>/dev/null && docker run --rm debian:bookworm-slim true &>/dev/null; then
  _can_docker=1
fi
if [[ $_can_docker -eq 1 ]] && [[ -n "${_deb}" ]] && [[ -f "${_deb}" ]]; then
  bash "$UNIT/test_deb_install.sh" || failed=1
else
  echo "  (Docker runnable + .deb required; run python3 workflows/shared/launch.py workflows/package/linux-cli/config.yml -- VERSION amd64 --output release/artifacts and ensure 'docker run' works to run this test)"
fi

if [[ $failed -eq 0 ]]; then
  echo "All tests passed."
else
  echo "Some tests failed."
  exit 1
fi
