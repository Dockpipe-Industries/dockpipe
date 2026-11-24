#!/bin/sh
set -eu
umask 077

: "${PRODUCT_RUN_ROOT:?Preparation must emit the proof directory}"
: "${PRODUCT_RUN_ID:?Preparation must emit the proof identity}"
: "${PRODUCT_SOURCE_ROOT:?Preparation must resolve the source scope}"
: "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR:?Preparation must materialize the typed source SDK}"
: "${DOCKPIPE_SOURCE_SDK_SHA256:?Preparation must pin the source descriptor}"
: "${PRODUCT_DOCKPIPE_BIN:?Select the Dockpipe executable under test}"
: "${PIPELANG_SDK_PREFIX:?Select the installed PipeLang SDK}"
: "${PIPELANG_SDK_MANIFEST_SHA256:?Pin the SDK manifest}"
: "${PIPELANG_TESTING_LIBRARY_SHA256:?Pin the installed testing library}"
: "${PIPELANG_CONTAINED_BIN:?Select the qualified containment runner}"

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
run_root=$PRODUCT_RUN_ROOT
source_root=$PRODUCT_SOURCE_ROOT
sdk=$PIPELANG_SDK_PREFIX
runner=$PIPELANG_CONTAINED_BIN

test "$(cat "$run_root/.journey-owner")" = "$PRODUCT_RUN_ID"
if [ "${1:-}" != --inside ]; then
    : "${PIPELANG_CONTAINMENT_PROBE:?Select the qualified containment probe}"
    : "${PIPELANG_CONTAINMENT_ADMISSION:?Select the prior containment admission receipt}"
    : "${PIPELANG_CONTAINMENT_ADMISSION_SHA256:?Pin the containment admission receipt}"
    "$PIPELANG_CONTAINMENT_PROBE" --admit "$runner" \
        "$PIPELANG_CONTAINMENT_ADMISSION" "$PIPELANG_CONTAINMENT_ADMISSION_SHA256"
    exec "$runner" job --output "$run_root/job" --estate "$run_root" --timeout 3600 -- \
        /usr/bin/env PRODUCT_RUN_ROOT="$run_root" PRODUCT_RUN_ID="$PRODUCT_RUN_ID" \
        PRODUCT_SOURCE_ROOT="$source_root" PRODUCT_DOCKPIPE_BIN="$PRODUCT_DOCKPIPE_BIN" \
        DOCKPIPE_SOURCE_SDK_DESCRIPTOR="$DOCKPIPE_SOURCE_SDK_DESCRIPTOR" \
        DOCKPIPE_SOURCE_SDK_SHA256="$DOCKPIPE_SOURCE_SDK_SHA256" \
        PIPELANG_SDK_PREFIX="$sdk" PIPELANG_SDK_MANIFEST_SHA256="$PIPELANG_SDK_MANIFEST_SHA256" \
        PIPELANG_TESTING_LIBRARY_SHA256="$PIPELANG_TESTING_LIBRARY_SHA256" \
        PIPELANG_CONTAINED_BIN="$runner" PRODUCT_SANITIZERS="${PRODUCT_SANITIZERS:-OFF}" \
        PRODUCT_CMAKE="${PRODUCT_CMAKE:-/usr/bin/cmake}" PRODUCT_CXX="${PRODUCT_CXX:-/usr/bin/c++}" \
        PIPELANG_CXX_RESOURCE_PROFILE="${PIPELANG_CXX_RESOURCE_PROFILE-system}" \
        /bin/sh "$script_directory/prove.sh" --inside
fi
: "${CONTAINED_JOB_SLICE:?Build and execution require the qualified aggregate job}"

unit() {
    unit_name=$1
    shift
    prefix="$run_root/units/$unit_name"
    if "$runner" unit --output "$prefix" --estate "$run_root" --timeout 900 -- \
        "$@" > "$prefix.launcher.log" 2>&1; then
        printf '%s: passed\n' "$unit_name"
        return
    fi
    tail -c 6000 "$prefix.output" >&2 || true
    tail -c 3000 "$prefix.launcher.log" >&2 || true
    return 1
}

manifest="$sdk/share/pipelang-model/sdk-manifest.json"
unit sdk-verify-before "$sdk/bin/pipelang-sdk-tool" verify --prefix "$sdk" \
    --manifest "$manifest" --sha256 "$PIPELANG_SDK_MANIFEST_SHA256"
harness="$source_root/workflows/test/product/assets/harness"
fixture="$source_root/workflows/test/product/assets/fixtures/host-workflow/config.yml"
testing="$sdk/share/pipelang-model/libraries/testing/library.json"
test -f "$testing"
actual_testing_sha256=$(sha256sum "$testing" | cut -d' ' -f1)
test "$actual_testing_sha256" = "$PIPELANG_TESTING_LIBRARY_SHA256"
{
    sha256sum "$PRODUCT_DOCKPIPE_BIN" "$manifest" "$testing"
    find "$(dirname -- "$DOCKPIPE_SOURCE_SDK_DESCRIPTOR")" -type f -exec sha256sum '{}' ';'
    find "$harness" "$source_root/workflows/test/product/assets/fixtures" -type f \
        -exec sha256sum '{}' ';'
    sha256sum "$source_root/workflows/test/product/config.yml" "$script_directory/prepare.sh" \
        "$script_directory/prove.sh" "$script_directory/preserve.sh"
} > "$run_root/identities.sha256"

cmake=${PRODUCT_CMAKE:-/usr/bin/cmake}
cxx=${PRODUCT_CXX:-/usr/bin/c++}
case "$cmake:$cxx" in
    /*:/*)
        ;;
    *)
        echo 'Select absolute installed CMake and C++ compiler executables' >&2
        exit 1
        ;;
esac
unit configure "$cmake" -S "$harness" -B "$run_root/build" \
    -DPIPELANG_PRODUCT_SDK_PREFIX="$sdk" \
    -DPIPELANG_PRODUCT_SDK_SHA256="$PIPELANG_SDK_MANIFEST_SHA256" \
    -DPIPELANG_PRODUCT_TESTING_SHA256="$PIPELANG_TESTING_LIBRARY_SHA256" \
    -DDOCKPIPE_SOURCE_SDK_DESCRIPTOR="$DOCKPIPE_SOURCE_SDK_DESCRIPTOR" \
    -DDOCKPIPE_SOURCE_SDK_SHA256="$DOCKPIPE_SOURCE_SDK_SHA256" \
    -DPipeLangModel_CXX_RESOURCE_PROFILE="${PIPELANG_CXX_RESOURCE_PROFILE-system}" \
    -DCMAKE_CXX_COMPILER="$cxx" -DCMAKE_BUILD_TYPE=Release \
    '-DCMAKE_CXX_FLAGS_RELEASE=-O1 -DNDEBUG' \
    -DPIPELANG_PRODUCT_SANITIZERS="${PRODUCT_SANITIZERS:-OFF}"
unit build "$cmake" --build "$run_root/build" --target dockpipe_product_host --parallel 1

# The same source case family is executed once per fresh workflow proof. Select
# ON for a separate complete producer-and-consumer sanitizer proof.
unit journey /usr/bin/env ASAN_OPTIONS=detect_leaks=1:halt_on_error=1 \
    UBSAN_OPTIONS=halt_on_error=1:print_stacktrace=1 \
    "$run_root/build/dockpipe_product_host" \
    --cli "$PRODUCT_DOCKPIPE_BIN" --proof-root "$run_root" \
    --recovery-project "$run_root/recovery-project" \
    --cancellation-project "$run_root/cancellation-project" \
    --workflow "$fixture" --run-id "$PRODUCT_RUN_ID" \
    --outside-file /proc/version --outside-directory /proc \
    --xdg-state "$run_root/xdg-state" --xdg-cache "$run_root/xdg-cache" \
    --identity-file "$run_root/identities.sha256" --report "$run_root/report.json"
unit sdk-verify-after "$sdk/bin/pipelang-sdk-tool" verify --prefix "$sdk" \
    --manifest "$manifest" --sha256 "$PIPELANG_SDK_MANIFEST_SHA256"
