#!/bin/sh
set -eu
: "${TRACE:?Select a task-owned trace file}"
printf 'hygiene:%s:%s:%s\n' "$ARG_A" "$ARG_B" "$ARG_C" >> "$TRACE"
