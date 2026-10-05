#!/usr/bin/env bash
# Shared validation for the host materializer and its isolated builder.

qemu_require_build_path() {
  local name="$1" value="${!1:-}"
  # These paths also enter Docker mount specifications and linker arguments.
  # Require clean absolute paths without their separator or quoting syntax.
  if [[ ! "$value" =~ ^/([a-zA-Z0-9._-]+/)+[a-zA-Z0-9._-]+$ ]] ||
     [[ "$value" == */./* || "$value" == */../* || "$value" == */. || "$value" == */.. ]]; then
    echo "$name must be an explicit clean absolute path with only letters, digits, dots, underscores, hyphens, and slashes" >&2
    return 2
  fi
}

qemu_require_separate_paths() {
  local first="$1" second="$2"
  if [[ "$first" == "$second" || "$first" == "$second/"* || "$second" == "$first/"* ]]; then
    echo 'QEMU source, build-record, recipe, and final toolchain directories must not overlap' >&2
    return 2
  fi
}
