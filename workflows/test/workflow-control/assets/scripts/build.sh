#!/bin/sh
set -eu
: "${TRACE:?Select a task-owned trace file}"
: "${TARGET:?The source controller must supply a target}"
printf 'build:%s\n' "$TARGET" >> "$TRACE"
