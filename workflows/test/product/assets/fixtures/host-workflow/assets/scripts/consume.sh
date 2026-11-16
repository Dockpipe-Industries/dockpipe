#!/bin/sh
set -eu
: "${MERGED_CORRELATION:?The producer output must reach this step}"
: "${JOURNEY_MODE:?An explicit fixture mode is required}"
printf 'consumer:%s\n' "$MERGED_CORRELATION"
case "$JOURNEY_MODE" in
    fail)
        exit 23
        ;;
    success)
        exit 0
        ;;
    hold)
        sleep 30
        ;;
    *)
        exit 24
        ;;
esac
