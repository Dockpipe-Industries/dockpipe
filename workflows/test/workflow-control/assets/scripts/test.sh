#!/bin/sh
set -eu
: "${TRACE:?Select a task-owned trace file}"
printf 'test\n' >> "$TRACE"
if [ "${FAIL_TEST:-false}" = true ]; then
    exit 23
fi
