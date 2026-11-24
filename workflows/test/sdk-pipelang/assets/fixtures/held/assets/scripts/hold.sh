#!/bin/sh
set -eu
trap 'exit 0' INT TERM
printf 'SDK_READY\n'
while :; do
    sleep 1 &
    wait $! || true
done
