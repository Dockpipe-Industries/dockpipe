#!/bin/sh
set -eu
umask 077

: "${DOCKPIPE_BIN:?Run through the qualification workflow}"
: "${PRODUCT_DOCKPIPE_BIN:?Select the absolute Dockpipe CLI under test}"
: "${PIPELANG_SDK_PREFIX:?Select the installed PipeLang SDK}"
: "${PIPELANG_SDK_MANIFEST_SHA256:?Pin the installed SDK manifest}"
: "${PIPELANG_CONTAINED_BIN:?Select the qualified containment runner}"
: "${PIPELANG_CONTAINMENT_PROBE:?Select the matching containment admission probe}"
: "${PIPELANG_CONTAINMENT_ADMISSION:?Select the independent containment receipt}"
: "${PIPELANG_CONTAINMENT_ADMISSION_SHA256:?Pin the containment receipt}"

for selected in "$PRODUCT_DOCKPIPE_BIN" "$PIPELANG_SDK_PREFIX" \
    "$PIPELANG_CONTAINED_BIN" "$PIPELANG_CONTAINMENT_PROBE" "$PIPELANG_CONTAINMENT_ADMISSION"; do
    case "$selected" in
        /*)
            ;;
        *)
            echo 'Select absolute tool and receipt paths' >&2
            exit 1
            ;;
    esac
done
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
run_root=$(mktemp -d "$artifact_root/sdk-pipelang.XXXXXXXX")
mkdir "$run_root/state" "$run_root/cache" "$run_root/units"
descriptor=$("$PRODUCT_DOCKPIPE_BIN" sdk path --language pipelang --workdir "$run_root")
test -f "$descriptor"
source_hash=$(sha256sum "$descriptor")
source_hash=${source_hash%% *}
carriage_return=$(printf '\r')
for emitted in "$run_root" "$source_root" "$descriptor"; do
    case "$emitted" in
        *'
'*|*"$carriage_return"*)
            echo 'Proof paths must not contain line delimiters' >&2
            exit 1
            ;;
    esac
done
outputs=$("$DOCKPIPE_BIN" scope artifacts sdk-proof.env)
{
    printf 'SDK_PROOF_ROOT=%s\n' "$run_root"
    printf 'SDK_PROOF_SOURCE=%s\n' "$source_root"
    printf 'SDK_PROOF_DESCRIPTOR=%s\n' "$descriptor"
    printf 'SDK_PROOF_DESCRIPTOR_SHA256=%s\n' "$source_hash"
} > "$outputs"
printf 'Prepared SDK proof %s\n' "$run_root"
