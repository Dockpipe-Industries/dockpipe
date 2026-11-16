#!/bin/sh
set -eu
: "${JOURNEY_CORRELATION:?An explicit invocation correlation is required}"
rm -f -- resource.marker
printf '%s\n' "$JOURNEY_CORRELATION" > cleanup.receipt
printf 'cleanup:%s\n' "$JOURNEY_CORRELATION"
