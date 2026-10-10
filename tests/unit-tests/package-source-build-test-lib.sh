#!/usr/bin/env bash

# Neutral contract fixture for package-owned source-build tool specifications.

dockpipe_test_source_build_contract() (
  local repo_root="${1:?repo root}"
  local script="${2:?build script}"
  local package_dir="${3:?package dir}"
  local package_id="${4:?package id}"
  shift 4
  local specs=("$@")
  local tmp_root="$repo_root/bin/.dockpipe/tmp/package-tests"
  local tmp fake_bin fake_bin_unix fake_repo fake_package_root
  local operation_log go_log stdout_log stderr_log
  local spec tool module_rel output_name go_package expected_output expected_module_dir
  local first_spec first_go_package rc

  mkdir -p "$tmp_root"
  tmp="$(mktemp -d "$tmp_root/package-source-build.XXXXXX")"
  trap 'rm -rf "$tmp"' EXIT
  fake_bin="$tmp/bin"
  fake_repo="$tmp/repo"
  fake_package_root="$fake_repo/packages/$package_dir"
  operation_log="$tmp/operation.log"
  go_log="$tmp/go.log"
  stdout_log="$tmp/stdout.log"
  stderr_log="$tmp/stderr.log"
  mkdir -p "$fake_bin" "$fake_package_root"
  printf '0.0.0-test\n' > "$fake_repo/VERSION"
  : > "$operation_log"
  : > "$go_log"

  for spec in "${specs[@]}"; do
    IFS='|' read -r tool module_rel output_name go_package <<< "$spec"
    mkdir -p "$fake_package_root/$module_rel"
  done

  if command -v cygpath >/dev/null 2>&1; then
    fake_bin_unix="$(cygpath -u "$fake_bin")"
  else
    fake_bin_unix="$fake_bin"
  fi

  cat > "$fake_bin/dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ "${1:-}" == "result" ]] || exit 1
shift
unit=""
status=""
duration_ms=""
fields=()
while (($#)); do
  case "${1:-}" in
    --unit) unit="${2:-}"; shift 2 ;;
    --status) status="${2:-}"; shift 2 ;;
    --duration-ms) duration_ms="${2:-}"; shift 2 ;;
    --id) fields+=("${2:-}"); shift 2 ;;
    --error) fields+=("error=${2:-}"); shift 2 ;;
    *) exit 1 ;;
  esac
done
{
  printf 'unit=%s status=%s' "$unit" "$status"
  if [[ -n "$duration_ms" ]]; then
    printf ' duration_ms=%s' "$duration_ms"
  fi
  for field in "${fields[@]}"; do
    printf ' %s' "$field"
  done
  printf '\n'
} >> "${FAKE_OPERATION_LOG:?}"
SH
  chmod +x "$fake_bin/dockpipe"

  cat > "$fake_bin/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "env" && "${2:-}" == "GOEXE" ]]; then
  printf '.testexe\n'
  exit 0
fi
[[ "${1:-}" == "build" ]] || exit 1
printf 'args=%s|gocache=%s|gotmpdir=%s\n' "$*" "${GOCACHE:-}" "${GOTMPDIR:-}" >> "${FAKE_GO_LOG:?}"
if [[ -n "${FAKE_GO_FAIL_PACKAGE:-}" && "${*: -1}" == "$FAKE_GO_FAIL_PACKAGE" ]]; then
  exit "${FAKE_GO_FAIL_RC:-1}"
fi
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
SH
  chmod +x "$fake_bin/go"

  dockpipe_test_run_source_build() {
    local dockpipe_bin="${1:?dockpipe bin}"
    local fail_package="${2:-}"
    (
      unset GOCACHE GOTMPDIR
      export PATH="$fake_bin_unix:$PATH"
      export FAKE_OPERATION_LOG="$operation_log"
      export FAKE_GO_LOG="$go_log"
      export FAKE_GO_FAIL_PACKAGE="$fail_package"
      export FAKE_GO_FAIL_RC=23
      export DOCKPIPE_BIN="$dockpipe_bin"
      export DOCKPIPE_PACKAGE_ROOT="$fake_package_root"
      unset DOCKPIPE_SOURCE_BUILD_LIB
      export DOCKPIPE_SDK_SH="$repo_root/src/core/assets/scripts/lib/dockpipe-sdk.sh"
      bash "$script"
    ) > "$stdout_log" 2> "$stderr_log"
  }

  dockpipe_test_assert_builds() {
    local expected_count="${1:?expected count}"
    [[ "$(wc -l < "$go_log")" -eq "$expected_count" ]]
    for spec in "${specs[@]:0:expected_count}"; do
      IFS='|' read -r tool module_rel output_name go_package <<< "$spec"
      expected_output="$fake_repo/bin/.dockpipe/tooling/bin/$output_name"
      expected_module_dir="$fake_package_root"
      if [[ "$module_rel" != "." ]]; then
        expected_module_dir="$fake_package_root/$module_rel"
      fi
      grep -Fq -- "args=build -C $expected_module_dir -trimpath -ldflags -s -w -X main.Version=0.0.0-test -o $expected_output $go_package|gocache=$fake_repo/bin/.dockpipe/build/go-cache|gotmpdir=$fake_repo/bin/.dockpipe/build/go-tmp" "$go_log"
    done
  }

  dockpipe_test_assert_result_lines() {
    local log="${1:?log}"
    local start_status="${2:?start status}"
    local finish_status="${3:?finish status}"
    local expected_count="${4:?expected count}"
    for spec in "${specs[@]:0:expected_count}"; do
      IFS='|' read -r tool module_rel output_name go_package <<< "$spec"
      expected_output="$fake_repo/bin/.dockpipe/tooling/bin/$output_name"
      grep -Fq -- "unit=package.source.tool status=$start_status tool=$tool output=$expected_output package=$package_id" "$log"
      grep -Eq -- "unit=package.source.tool status=$finish_status duration_ms=[0-9]+ tool=$tool output=$expected_output package=$package_id" "$log"
    done
  }

  dockpipe_test_run_source_build "$fake_bin/dockpipe" ""
  [[ ! -s "$stderr_log" ]]
  [[ "$(wc -l < "$operation_log")" -eq "$(( ${#specs[@]} * 2 ))" ]]
  dockpipe_test_assert_result_lines "$operation_log" start "done" "${#specs[@]}"
  dockpipe_test_assert_builds "${#specs[@]}"
  for spec in "${specs[@]}"; do
    IFS='|' read -r tool module_rel output_name go_package <<< "$spec"
    [[ -f "$fake_repo/bin/.dockpipe/tooling/bin/$output_name" ]]
  done

  : > "$operation_log"
  : > "$go_log"
  dockpipe_test_run_source_build "$fake_bin/missing-dockpipe" ""
  [[ ! -s "$operation_log" ]]
  [[ "$(wc -l < "$stderr_log")" -eq "$(( ${#specs[@]} * 2 ))" ]]
  dockpipe_test_assert_result_lines "$stderr_log" start "done" "${#specs[@]}"
  dockpipe_test_assert_builds "${#specs[@]}"

  : > "$operation_log"
  : > "$go_log"
  first_spec="${specs[0]}"
  IFS='|' read -r tool module_rel output_name first_go_package <<< "$first_spec"
  set +e
  dockpipe_test_run_source_build "$fake_bin/dockpipe" "$first_go_package"
  rc=$?
  set -e
  [[ "$rc" -eq 23 ]]
  [[ "$(wc -l < "$operation_log")" -eq 2 ]]
  expected_output="$fake_repo/bin/.dockpipe/tooling/bin/$output_name"
  grep -Fq -- "unit=package.source.tool status=start tool=$tool output=$expected_output package=$package_id" "$operation_log"
  grep -Eq -- "unit=package.source.tool status=fail duration_ms=[0-9]+ tool=$tool output=$expected_output package=$package_id error=go build exited 23" "$operation_log"
  if grep -Fq -- "status=done" "$operation_log"; then
    return 1
  fi
  dockpipe_test_assert_builds 1

)
