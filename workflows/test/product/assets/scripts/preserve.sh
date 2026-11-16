#!/bin/sh
set -eu

if [ -z "${PRODUCT_RUN_ROOT:-}" ] || [ -z "${PRODUCT_RUN_ID:-}" ]; then
    printf '%s\n' 'Preparation did not expose an owned proof directory; no preservation receipt written.'
    exit 0
fi
test "$(cat "$PRODUCT_RUN_ROOT/.journey-owner")" = "$PRODUCT_RUN_ID"
printf '%s\n' "$PRODUCT_RUN_ID" > "$PRODUCT_RUN_ROOT/workflow-finally.receipt"
printf 'Preserved product proof %s\n' "$PRODUCT_RUN_ROOT"
