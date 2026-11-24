#!/bin/sh
set -eu
umask 077

: "${SDK_PROOF_ROOT:?Preparation must expose its proof directory}"
: "${SDK_PROOF_SOURCE:?Preparation must resolve the source scope}"
: "${SDK_PROOF_DESCRIPTOR:?Preparation must materialize the source SDK}"
: "${SDK_PROOF_DESCRIPTOR_SHA256:?Preparation must pin the source descriptor}"
: "${PRODUCT_DOCKPIPE_BIN:?Select the CLI under test}"
: "${PIPELANG_SDK_PREFIX:?Select the installed PipeLang SDK}"
: "${PIPELANG_SDK_MANIFEST_SHA256:?Pin the installed SDK}"
: "${PIPELANG_CONTAINED_BIN:?Select the qualified containment runner}"

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$SDK_PROOF_ROOT
sdk=$PIPELANG_SDK_PREFIX
runner=$PIPELANG_CONTAINED_BIN
if [ "${1:-}" != --inside ]; then
    "$PIPELANG_CONTAINMENT_PROBE" --admit "$runner" \
        "$PIPELANG_CONTAINMENT_ADMISSION" "$PIPELANG_CONTAINMENT_ADMISSION_SHA256"
    exec "$runner" job --output "$root/job" --estate "$root" --timeout 3600 -- \
        /usr/bin/env SDK_PROOF_ROOT="$root" SDK_PROOF_SOURCE="$SDK_PROOF_SOURCE" \
        SDK_PROOF_DESCRIPTOR="$SDK_PROOF_DESCRIPTOR" \
        SDK_PROOF_DESCRIPTOR_SHA256="$SDK_PROOF_DESCRIPTOR_SHA256" \
        PRODUCT_DOCKPIPE_BIN="$PRODUCT_DOCKPIPE_BIN" PIPELANG_SDK_PREFIX="$sdk" \
        PIPELANG_SDK_MANIFEST_SHA256="$PIPELANG_SDK_MANIFEST_SHA256" \
        PIPELANG_CONTAINED_BIN="$runner" PRODUCT_SANITIZERS="${PRODUCT_SANITIZERS:-OFF}" \
        PIPELANG_CXX_RESOURCE_PROFILE="${PIPELANG_CXX_RESOURCE_PROFILE:-system}" \
        /bin/sh "$script_directory/prove.sh" --inside
fi
: "${CONTAINED_JOB_SLICE:?Compilation requires the qualified aggregate job}"

unit() {
    name=$1
    shift
    prefix="$root/units/$name"
    if "$runner" unit --output "$prefix" --estate "$root" --timeout 900 -- \
        "$@" > "$prefix.launcher.log" 2>&1; then
        printf '%s: passed\n' "$name"
        return
    fi
    tail -c 6000 "$prefix.output" >&2 || true
    tail -c 2000 "$prefix.launcher.log" >&2 || true
    return 1
}

manifest="$sdk/share/pipelang-model/sdk-manifest.json"
unit sdk-verify-before "$sdk/bin/pipelang-sdk-tool" verify --prefix "$sdk" \
    --manifest "$manifest" --sha256 "$PIPELANG_SDK_MANIFEST_SHA256"
assets="$SDK_PROOF_SOURCE/workflows/test/sdk-pipelang/assets"
bundle=$(dirname -- "$SDK_PROOF_DESCRIPTOR")
{
    sha256sum "$PRODUCT_DOCKPIPE_BIN" "$manifest"
    find "$bundle" "$assets" -type f -exec sha256sum '{}' ';'
} > "$root/identities.sha256"
unit configure /usr/bin/cmake -S "$assets/harness" -B "$root/build" \
    -DPIPELANG_SDK_PREFIX="$sdk" -DPIPELANG_SDK_MANIFEST_SHA256="$PIPELANG_SDK_MANIFEST_SHA256" \
    -DDOCKPIPE_SOURCE_SDK_DESCRIPTOR="$SDK_PROOF_DESCRIPTOR" \
    -DDOCKPIPE_SOURCE_SDK_SHA256="$SDK_PROOF_DESCRIPTOR_SHA256" \
    -DPipeLangModel_CXX_RESOURCE_PROFILE="${PIPELANG_CXX_RESOURCE_PROFILE:-system}" \
    -DCMAKE_BUILD_TYPE=Release '-DCMAKE_CXX_FLAGS_RELEASE=-O2 -DNDEBUG' \
    -DSANITIZERS="${PRODUCT_SANITIZERS:-OFF}"
unit build /usr/bin/cmake --build "$root/build" --parallel 1
unit run /usr/bin/env ASAN_OPTIONS=detect_leaks=1:halt_on_error=1 \
    UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1 \
    "$root/build/sdk_probe" "$PRODUCT_DOCKPIPE_BIN" "$root" \
    "$assets/fixtures/failure/config.yml" "$assets/fixtures/held/config.yml"
unit sdk-verify-after "$sdk/bin/pipelang-sdk-tool" verify --prefix "$sdk" \
    --manifest "$manifest" --sha256 "$PIPELANG_SDK_MANIFEST_SHA256"
printf '%s\n' 'passed' > "$root/qualification.result"
