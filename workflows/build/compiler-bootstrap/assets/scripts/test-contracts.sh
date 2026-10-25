#!/usr/bin/env bash
set -euo pipefail
python3 workflows/ci/core/assets/scripts/test.py ./src/lib/application/internal/packagecompile ./src/lib/infrastructure/packagebuild ./src/lib/infrastructure/packagecatalog
python3 workflows/ci/core/assets/scripts/test.py ./src/lib/application -run TestResolverAssetsScope -count=1
python3 workflows/ci/core/assets/scripts/test.py ./src/lib/infrastructure -run 'TestResolveResolverAssetsDir|TestFindLatestPackageTarball|TestInvalidateTarballExtractCache' -count=1
python3 workflows/shared/launch.py workflows/ci/workflow-contracts/config.yml -- --suite workflows/build/compiler-bootstrap/tests
