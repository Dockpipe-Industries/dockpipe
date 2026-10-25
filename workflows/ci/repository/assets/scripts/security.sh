#!/usr/bin/env bash
set -euo pipefail
set -euo pipefail
CI_RAW_DIR="$(./src/bin/dockpipe scope artifacts ci-raw --workdir "$DOCKPIPE_WORKDIR")"
export DOCKPIPE_CI_RAW_DIR="$CI_RAW_DIR"
rm -rf "$CI_RAW_DIR"
mkdir -p "$CI_RAW_DIR"
GOBIN="$(go env GOPATH)/bin"
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0
set +e
"$GOBIN/govulncheck" -format json ./... > "$CI_RAW_DIR/govulncheck.json"
VC=$?
if [[ $VC -eq 0 ]]; then
  # JSON output reports findings without a failing exit status. Convert
  # the same scan to text to enforce reachable vulnerabilities as a gate.
  "$GOBIN/govulncheck" -mode convert < "$CI_RAW_DIR/govulncheck.json"
  VC=$?
fi
"$GOBIN/gosec" -conf .gosec.json -fmt json -out="$CI_RAW_DIR/gosec.json" -exclude-dir=.gomodcache -exclude-dir=bin/.dockpipe -exclude-dir=testdata ./...
GC=$?
set -e
echo "::group::gosec stats (JSON)"
jq '.Stats // empty' "$CI_RAW_DIR/gosec.json" || true
echo "::endgroup::"
if [[ $VC -ne 0 ]]; then
  echo "::error::govulncheck failed (exit $VC)"
  exit "$VC"
fi
if [[ $GC -ne 0 ]]; then
  echo "::error::gosec failed (exit $GC)"
  exit "$GC"
fi
