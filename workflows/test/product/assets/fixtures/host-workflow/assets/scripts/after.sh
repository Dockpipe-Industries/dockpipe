#!/bin/sh
set -eu
: "${JOURNEY_CORRELATION:?An explicit invocation correlation is required}"
printf '%s\n' "$JOURNEY_CORRELATION" > after.receipt
printf 'after:%s\n' "$JOURNEY_CORRELATION"
