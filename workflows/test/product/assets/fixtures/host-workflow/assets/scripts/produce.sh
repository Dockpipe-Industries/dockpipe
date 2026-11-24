#!/bin/sh
set -eu
: "${JOURNEY_CORRELATION:?An explicit invocation correlation is required}"
printf 'producer:%s\n' "$JOURNEY_CORRELATION"
printf 'producer-error:%s\n' "$JOURNEY_CORRELATION" >&2
printf 'MERGED_CORRELATION=%s\n' "$JOURNEY_CORRELATION" > values.env
printf '%s\n' "$JOURNEY_CORRELATION" > resource.marker
