#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
node "$ROOT/packages/ide/tests/devcontainer-lifecycle.test.js"
bash "$ROOT/packages/ide/tests/test_ide_state_ownership.sh"
