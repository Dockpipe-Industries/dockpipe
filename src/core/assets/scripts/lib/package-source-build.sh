#!/usr/bin/env bash

# Neutral source-checkout build plumbing for package-owned Go tool lists.

dockpipe_source_build_init() {
  local repo_root="${1:?repo root}"
  local version_file="$repo_root/VERSION"
  local version="0.0.0"

  if [[ -f "$version_file" ]]; then
    version="$(tr -d ' \t\r\n' < "$version_file")"
  fi

  DOCKPIPE_SOURCE_BUILD_OUT_DIR="$repo_root/bin/.dockpipe/tooling/bin"
  DOCKPIPE_SOURCE_BUILD_DIR="$repo_root/bin/.dockpipe/build"
  DOCKPIPE_SOURCE_BUILD_LDFLAGS="-s -w -X main.Version=${version}"

  mkdir -p "$DOCKPIPE_SOURCE_BUILD_OUT_DIR"
  mkdir -p "$DOCKPIPE_SOURCE_BUILD_DIR/go-cache" "$DOCKPIPE_SOURCE_BUILD_DIR/go-tmp"
  export GOCACHE="${GOCACHE:-$DOCKPIPE_SOURCE_BUILD_DIR/go-cache}"
  export GOTMPDIR="${GOTMPDIR:-$DOCKPIPE_SOURCE_BUILD_DIR/go-tmp}"
}

dockpipe_source_build_now_ms() {
  date +%s%3N
}

dockpipe_source_build_emit_result() {
  local unit="${1:?unit}"
  local status="${2:?status}"
  local duration_ms="${3:-}"
  shift 3 || true
  local dockpipe_bin="${DOCKPIPE_BIN:-dockpipe}"
  local args=("result" "--unit" "$unit" "--status" "$status")
  local field key value

  if [[ -n "$duration_ms" && "$status" != "start" ]]; then
    args+=("--duration-ms" "$duration_ms")
  fi
  for field in "$@"; do
    [[ -n "$field" ]] || continue
    if [[ "$field" == *=* ]]; then
      key="${field%%=*}"
      value="${field#*=}"
      if [[ "$key" == "error" ]]; then
        args+=("--error" "$value")
      else
        args+=("--id" "$field")
      fi
    fi
  done
  if command -v "$dockpipe_bin" >/dev/null 2>&1 && "$dockpipe_bin" "${args[@]}"; then
    return 0
  fi

  printf '[dockpipe] unit=%s status=%s' "$unit" "$status" >&2
  if [[ -n "$duration_ms" && "$status" != "start" ]]; then
    printf ' duration_ms=%s' "$duration_ms" >&2
  fi
  for field in "$@"; do
    [[ -n "$field" ]] && printf ' %s' "$field" >&2
  done
  printf '\n' >&2
}

dockpipe_source_build_tool() {
  local tool="${1:?tool}"
  local module_dir="${2:?module dir}"
  local output="${3:?output}"
  local package_id="${4:?package id}"
  local go_package="${5:?go package}"
  local started_ms duration_ms rc
  local had_errexit=0
  local fields=("tool=$tool" "output=$output" "package=$package_id")

  dockpipe_source_build_emit_result "package.source.tool" "start" "" "${fields[@]}"
  started_ms="$(dockpipe_source_build_now_ms)"
  [[ $- == *e* ]] && had_errexit=1
  set +e
  go build -C "$module_dir" -trimpath -ldflags "$DOCKPIPE_SOURCE_BUILD_LDFLAGS" -o "$output" "$go_package"
  rc=$?
  if [[ "$had_errexit" -eq 1 ]]; then
    set -e
  fi
  duration_ms="$(( $(dockpipe_source_build_now_ms) - started_ms ))"
  if [[ "$rc" -ne 0 ]]; then
    dockpipe_source_build_emit_result "package.source.tool" "fail" "$duration_ms" "${fields[@]}" "error=go build exited $rc"
    return "$rc"
  fi
  dockpipe_source_build_emit_result "package.source.tool" "done" "$duration_ms" "${fields[@]}"
}
