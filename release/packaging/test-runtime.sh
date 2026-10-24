#!/usr/bin/env bash
# Qualify every Go package owned by the standalone Dockpipe runtime repository.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
go test ./...
