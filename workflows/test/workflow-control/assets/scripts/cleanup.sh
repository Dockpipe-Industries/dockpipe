#!/bin/sh
set -eu
: "${TRACE:?Select a task-owned trace file}"
printf 'cleanup\n' >> "$TRACE"
