#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_build_source_operation_results failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

ROOT="$(git rev-parse --show-toplevel)"
SCRIPT="$ROOT/packages/dorkpipe-mcp/assets/scripts/build-source.sh"

mkdir -p "$ROOT/bin/.dockpipe/tmp/package-tests"
tmp="$(mktemp -d "$ROOT/bin/.dockpipe/tmp/package-tests/dorkpipe-mcp-build-source.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

fake_bin="$tmp/bin"
fake_repo="$tmp/repo"
fake_package_root="$fake_repo/packages/dorkpipe-mcp"
mkdir -p "$fake_bin" "$fake_package_root"
printf '0.0.0-test\n' > "$fake_repo/VERSION"
operation_log="$tmp/operation.log"
go_log="$tmp/go.log"

cat >"$fake_bin/dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ "${1:-}" == "result" ]] || exit 1
printf '%s\n' "$*" >> "${FAKE_OPERATION_LOG:?}"
SH
chmod +x "$fake_bin/dockpipe"

cat >"$fake_bin/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ "${1:-}" == "build" ]] || exit 1
args=("$@")
out=""
for ((i = 0; i < ${#args[@]}; i++)); do
  if [[ "${args[$i]}" == "-o" ]]; then
    out="${args[$((i + 1))]:-}"
    break
  fi
done
[[ -n "$out" ]] || exit 1
mkdir -p "$(dirname "$out")"
printf 'fake binary\n' > "$out"
printf '%s\n' "$*" >> "${FAKE_GO_LOG:?}"
SH
chmod +x "$fake_bin/go"

export PATH="$fake_bin:$PATH"
export FAKE_OPERATION_LOG="$operation_log"
export FAKE_GO_LOG="$go_log"
export DOCKPIPE_BIN="$fake_bin/dockpipe"
export DOCKPIPE_PACKAGE_ROOT="$fake_package_root"

bash "$SCRIPT"

[[ "$(wc -l < "$operation_log")" -eq 2 ]]
grep -Fq -- "--status start" "$operation_log"
grep -Fq -- "--status done" "$operation_log"
grep -Fq -- "--id tool=mcpd" "$operation_log"
grep -Fq -- "--id package=dorkpipe.mcp" "$operation_log"
grep -Fq -- "./cmd/mcpd" "$go_log"

echo "dorkpipe-mcp test_build_source_operation_results OK"
