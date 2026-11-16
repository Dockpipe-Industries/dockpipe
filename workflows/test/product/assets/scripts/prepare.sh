#!/bin/sh
set -eu
umask 077

: "${DOCKPIPE_BIN:?Run this script through the Dockpipe workflow}"
: "${PRODUCT_DOCKPIPE_BIN:?Select the exact absolute Dockpipe CLI under test}"
: "${PIPELANG_SDK_PREFIX:?Select the exact installed PipeLang SDK prefix}"
: "${PIPELANG_SDK_MANIFEST_SHA256:?Pin the installed SDK manifest SHA256}"
: "${PIPELANG_TESTING_LIBRARY_SHA256:?Pin the installed testing library descriptor SHA256}"
: "${PIPELANG_CONTAINED_BIN:?Select the qualified installed pipelang-contained executable}"
: "${PIPELANG_CONTAINMENT_PROBE:?Select its matching installed containment admission probe}"
: "${PIPELANG_CONTAINMENT_ADMISSION:?Select the prior qualified containment acceptance receipt}"
: "${PIPELANG_CONTAINMENT_ADMISSION_SHA256:?Pin the containment acceptance receipt SHA256}"

for absolute_path in "$PRODUCT_DOCKPIPE_BIN" "$PIPELANG_SDK_PREFIX" \
    "$PIPELANG_CONTAINED_BIN" "$PIPELANG_CONTAINMENT_PROBE" "$PIPELANG_CONTAINMENT_ADMISSION"; do
    case "$absolute_path" in
        /*)
            ;;
        *)
            echo 'All selected tool and receipt paths must be absolute' >&2
            exit 1
            ;;
    esac
done
test -x "$PRODUCT_DOCKPIPE_BIN"
test -x "$PIPELANG_CONTAINED_BIN"
test -x "$PIPELANG_CONTAINMENT_PROBE"
test -x "$PIPELANG_SDK_PREFIX/bin/pipelang-sdk-tool"
test -f "$PIPELANG_CONTAINMENT_ADMISSION"

case "${PRODUCT_SANITIZERS:-OFF}" in
    OFF|ON)
        ;;
    *)
        echo 'PRODUCT_SANITIZERS must be OFF or ON' >&2
        exit 1
        ;;
esac

artifact_root=$("$DOCKPIPE_BIN" scope artifacts)
source_root=$("$DOCKPIPE_BIN" scope source)
case "$artifact_root:$source_root" in
    /*:/*)
        ;;
    *)
        echo 'Scope helpers must return absolute source and artifact paths' >&2
        exit 1
        ;;
esac
run_root=$(mktemp -d "$artifact_root/product-host.XXXXXXXX")
run_id=$(basename "$run_root")
mkdir "$run_root/recovery-project" "$run_root/cancellation-project" \
    "$run_root/xdg-state" "$run_root/xdg-cache" "$run_root/units"
printf '%s\n' "$run_id" > "$run_root/.journey-owner"
printf '%s\n' "$run_id" > "$run_root/recovery-project/.journey-owner"
printf '%s\n' "$run_id" > "$run_root/cancellation-project/.journey-owner"
ln -s .journey-owner "$run_root/recovery-project/.journey-marker-link"
ln -s .journey-owner "$run_root/cancellation-project/.journey-marker-link"

# Dotenv values are literal paths. Refuse unsupported line-oriented values.
carriage_return=$(printf '\r')
for emitted_value in "$run_root" "$run_id" "$source_root"; do
    case "$emitted_value" in
        *'
'*|*"$carriage_return"*)
            echo 'Proof and source paths must not contain line delimiters' >&2
            exit 1
            ;;
    esac
done
outputs=$("$DOCKPIPE_BIN" scope artifacts product.env)
{
    printf 'PRODUCT_RUN_ROOT=%s\n' "$run_root"
    printf 'PRODUCT_RUN_ID=%s\n' "$run_id"
    printf 'PRODUCT_SOURCE_ROOT=%s\n' "$source_root"
} > "$outputs"
printf 'Prepared fresh product proof %s\n' "$run_id"
