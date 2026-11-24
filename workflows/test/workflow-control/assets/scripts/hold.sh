#!/bin/sh
set -eu
: "${TRACE:?Select a task-owned trace file}"
trap 'exit 24' INT TERM
printf 'ready\n' >> "$TRACE"
while :; do
    sleep 30 &
    wait $! || true
done
