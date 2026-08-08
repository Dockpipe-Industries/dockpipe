#!/usr/bin/env bash
set -euo pipefail

if (($# != 0)); then
  echo "this purpose-specific materializer accepts no arguments" >&2
  exit 2
fi

RECIPE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly RECIPE_ROOT
readonly BUILDER='registry.gitlab.com/qemu-project/qemu/qemu/alpine@sha256:9108d3cbdacbaf442f8b8938a2e94a7cdf04c0b093953866726c5734cb478f2e'
readonly BUILDER_DIGEST='sha256:9108d3cbdacbaf442f8b8938a2e94a7cdf04c0b093953866726c5734cb478f2e'
readonly SOURCE_ARCHIVE=/tmp/qemu-11.0.3.tar.xz
readonly SOURCE_SIGNATURE=/tmp/qemu-11.0.3.tar.xz.sig
readonly SOURCE_KEY=/tmp/qemu-release-key-ubuntu.asc
readonly OFFICIAL_SOURCE_KEY=/tmp/qemu-release-key.asc
readonly SOURCE_SHA256=da5fcffc32762820568b828ed430a728864d34d50b6d2f30358597760cbb0523
readonly SIGNATURE_SHA256=719f32c491ee724629f7d5918a6ff04ddc115d92a597b504cc4f12191e4a5e77
readonly SOURCE_KEY_SHA256=e2673aabb4b1880be19325bf2b763705191c01a5c5e580a7532c3ad8b3582a6c
readonly OFFICIAL_SOURCE_KEY_SHA256=0ce28d0b02f2e36286be047e1c76558421c8b6324f729462a753cf6cb20fe368
readonly SIGNER=CEACC9E15534EBABB82D3FA03353C9CEF108B584
readonly BUILD_ROOT=/home/jamie/.cache/dockpipe/vm/toolchain-builds/qemu-11.0.3-linux-amd64.1-attempt-10
readonly FINAL_ROOT=/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1
readonly PUBLISH_ROOT=/home/jamie/.cache/dockpipe/vm/toolchains/.qemu-11.0.3-linux-amd64.1-attempt-10.partial
readonly BUILD_TIMEOUT_SECONDS=7200

for required in "$SOURCE_ARCHIVE" "$SOURCE_SIGNATURE" "$SOURCE_KEY" "$OFFICIAL_SOURCE_KEY"; do
  test -f "$required" || { echo "missing exact preverified input: $required" >&2; exit 1; }
done
test "$(sha256sum "$SOURCE_ARCHIVE" | cut -d' ' -f1)" = "$SOURCE_SHA256"
test "$(sha256sum "$SOURCE_SIGNATURE" | cut -d' ' -f1)" = "$SIGNATURE_SHA256"
test "$(sha256sum "$SOURCE_KEY" | cut -d' ' -f1)" = "$SOURCE_KEY_SHA256"
test "$(sha256sum "$OFFICIAL_SOURCE_KEY" | cut -d' ' -f1)" = "$OFFICIAL_SOURCE_KEY_SHA256"
test ! -e "$BUILD_ROOT" || { echo "build-record root already exists: $BUILD_ROOT" >&2; exit 1; }
test ! -e "$FINAL_ROOT" || { echo "final artifact root already exists: $FINAL_ROOT" >&2; exit 1; }
test ! -e "$PUBLISH_ROOT" || { echo "partial publication root already exists: $PUBLISH_ROOT" >&2; exit 1; }

image_json=$(docker image inspect "$BUILDER")
test "$(jq -r '.[0].Id' <<<"$image_json")" = "$BUILDER_DIGEST"
test "$(jq -r '.[0].Architecture + "/" + .[0].Os' <<<"$image_json")" = 'amd64/linux'

mkdir -p "$BUILD_ROOT/source" "$BUILD_ROOT/builder" "$BUILD_ROOT/build-1/work" "$BUILD_ROOT/build-1/record" "$BUILD_ROOT/build-2/work" "$BUILD_ROOT/build-2/record" "$BUILD_ROOT/final-staging"
chmod 0700 "$BUILD_ROOT" "$BUILD_ROOT/source" "$BUILD_ROOT/builder" "$BUILD_ROOT/build-1" "$BUILD_ROOT/build-1/work" "$BUILD_ROOT/build-1/record" "$BUILD_ROOT/build-2" "$BUILD_ROOT/build-2/work" "$BUILD_ROOT/build-2/record" "$BUILD_ROOT/final-staging"
cp "$SOURCE_ARCHIVE" "$BUILD_ROOT/source/qemu-11.0.3.tar.xz"
cp "$SOURCE_SIGNATURE" "$BUILD_ROOT/source/qemu-11.0.3.tar.xz.sig"
cp "$SOURCE_KEY" "$BUILD_ROOT/source/qemu-release-key-renewed.asc"
cp "$OFFICIAL_SOURCE_KEY" "$BUILD_ROOT/source/qemu-release-key-official.asc"
printf '%s\n' "$image_json" > "$BUILD_ROOT/builder/image-inspect.json"

gpg_home="$BUILD_ROOT/source/gnupg"
mkdir -m 0700 "$gpg_home"
gpg --homedir "$gpg_home" --batch --import "$BUILD_ROOT/source/qemu-release-key-renewed.asc" >"$BUILD_ROOT/source/key-import.stdout" 2>"$BUILD_ROOT/source/key-import.stderr"
gpg --homedir "$gpg_home" --batch --status-fd 1 --verify "$BUILD_ROOT/source/qemu-11.0.3.tar.xz.sig" "$BUILD_ROOT/source/qemu-11.0.3.tar.xz" >"$BUILD_ROOT/source/signature.status" 2>"$BUILD_ROOT/source/signature.stderr"
grep -F "[GNUPG:] VALIDSIG $SIGNER " "$BUILD_ROOT/source/signature.status"
grep -F '[GNUPG:] GOODSIG ' "$BUILD_ROOT/source/signature.status"
if grep -Eq 'EXPKEYSIG|REVKEYSIG|BADSIG|ERRSIG' "$BUILD_ROOT/source/signature.status"; then
  echo "QEMU signature did not satisfy the exact valid-key policy" >&2
  exit 1
fi

(
  cd "$RECIPE_ROOT"
  for recipe in build-spec.json build-in-container.sh generate-manifest.py materialize.sh; do
    printf '%s\t%s\n' "$(sha256sum "$recipe" | cut -d' ' -f1)" "$recipe"
  done
) > "$BUILD_ROOT/build-recipe-files.tsv"
sha256sum "$BUILD_ROOT/build-recipe-files.tsv" | cut -d' ' -f1 > "$BUILD_ROOT/build-recipe.sha256"

docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --pids-limit 512 \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m --entrypoint /bin/sh "$BUILDER" \
  -c 'apk info -vv | LC_ALL=C sort' > "$BUILD_ROOT/builder/installed-packages.txt"
test "$(sha256sum "$BUILD_ROOT/builder/installed-packages.txt" | cut -d' ' -f1)" = 21539b040e7e81bc44a54b8ed4ad5b077e0f6db7f116268c4f98ee9135969186
docker run --rm --network none --read-only --cap-drop ALL --cap-add DAC_READ_SEARCH --security-opt no-new-privileges --pids-limit 512 \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m --entrypoint /bin/bash "$BUILDER" -c '
    set -euo pipefail
    while IFS= read -r -d "" path; do
      type=$(stat -c %F "$path")
      mode=$(stat -c %a "$path")
      owner=$(stat -c %u:%g "$path")
      value=-
      if [[ -L "$path" ]]; then value=$(readlink "$path");
      elif [[ -f "$path" ]]; then value=$(sha256sum "$path" | cut -d" " -f1); fi
      printf "%s\t%s\t%s\t%s\t%s\n" "$type" "$mode" "$owner" "$value" "$path"
    done < <(find / -xdev \( -path /proc -o -path /sys -o -path /dev -o -path /tmp -o -path /run -o -path /etc/hosts -o -path /etc/hostname -o -path /etc/resolv.conf \) -prune -o -print0 | sort -z)
  ' > "$BUILD_ROOT/builder/filesystem-inventory.tsv" 2> "$BUILD_ROOT/builder/filesystem-inventory.stderr"
sha256sum "$BUILD_ROOT/builder/filesystem-inventory.tsv" | cut -d' ' -f1 > "$BUILD_ROOT/builder/filesystem-inventory.sha256"

run_build() {
  local number=$1 record="$BUILD_ROOT/build-$1/record" work="$BUILD_ROOT/build-$1/work"
  timeout --foreground --signal=TERM --kill-after=30s "$BUILD_TIMEOUT_SECONDS" \
    docker run --rm --name "dockpipe-qemu-11-0-3-build-$number" --network none --read-only --cap-drop ALL --security-opt no-new-privileges --pids-limit 2048 \
      --user "$(id -u):$(id -g)" --tmpfs /tmp:rw,nosuid,nodev,size=1g \
      --tmpfs "$FINAL_ROOT/lib:rw,exec,nosuid,nodev,size=16m" \
      --mount "type=bind,src=$BUILD_ROOT/source,dst=/input,readonly" \
      --mount "type=bind,src=$work,dst=/build" \
      --mount "type=bind,src=$record,dst=/record" \
      --mount "type=bind,src=$RECIPE_ROOT/build-in-container.sh,dst=/recipe/build-in-container.sh,readonly" \
      --entrypoint /usr/bin/env "$BUILDER" -i \
        AR=/usr/bin/ar AS=/usr/bin/as CC=/usr/bin/gcc CCACHE_DISABLE=1 HOME=/build/home HOST_CC=/usr/bin/gcc \
        LANG=C LC_ALL=C LD=/usr/bin/ld LOGNAME=builder MAKE=/usr/bin/make MAKEFLAGS=-j1 NINJA=/usr/bin/ninja \
        NM=/usr/bin/nm OBJCOPY=/usr/bin/objcopy OBJDUMP=/usr/bin/objdump PATH=/usr/bin:/bin PKG_CONFIG=/usr/bin/pkgconf \
        PYTHON=/usr/bin/python3 PYTHONHASHSEED=0 RANLIB=/usr/bin/ranlib READELF=/usr/bin/readelf \
        SOURCE_DATE_EPOCH=1784926308 STRIP=/usr/bin/strip TZ=UTC USER=builder ZERO_AR_DATE=1 \
        /recipe/build-in-container.sh
}

run_build 1
run_build 2
cmp "$BUILD_ROOT/build-1/record/output-files.tsv" "$BUILD_ROOT/build-2/record/output-files.tsv"
cmp "$BUILD_ROOT/build-1/record/output-files.sha256" "$BUILD_ROOT/build-2/record/output-files.sha256"
cmp "$BUILD_ROOT/build-1/record/configure-argv.txt" "$BUILD_ROOT/build-2/record/configure-argv.txt"
cmp "$BUILD_ROOT/build-1/record/build-environment.txt" "$BUILD_ROOT/build-2/record/build-environment.txt"

cp -a "$BUILD_ROOT/build-1/record/output/." "$BUILD_ROOT/final-staging/"
chmod 0700 "$BUILD_ROOT/final-staging"
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --pids-limit 128 \
  --user "$(id -u):$(id -g)" --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m \
  --mount "type=bind,src=$BUILD_ROOT/final-staging,dst=/bundle" \
  --mount "type=bind,src=$BUILD_ROOT,dst=/evidence,readonly" \
  --mount "type=bind,src=$RECIPE_ROOT/generate-manifest.py,dst=/recipe/generate-manifest.py,readonly" \
  --entrypoint /usr/bin/env "$BUILDER" -i LANG=C LC_ALL=C PATH=/usr/bin:/bin PYTHONHASHSEED=0 TZ=UTC /usr/bin/python3 /recipe/generate-manifest.py
chmod 0400 "$BUILD_ROOT/final-staging/toolchain.json"
touch -h -d @1784926308 "$BUILD_ROOT/final-staging/toolchain.json"
while IFS= read -r -d '' directory; do chmod 0500 "$directory"; done < <(find "$BUILD_ROOT/final-staging" -type d -print0)

mkdir -p "$(dirname "$FINAL_ROOT")"
chmod 0700 "$(dirname "$FINAL_ROOT")"
mkdir -m 0700 "$PUBLISH_ROOT"
cp -a "$BUILD_ROOT/final-staging/." "$PUBLISH_ROOT/"
chmod 0500 "$PUBLISH_ROOT"
mv "$PUBLISH_ROOT" "$FINAL_ROOT"
sha256sum "$FINAL_ROOT/toolchain.json" | cut -d' ' -f1 > "$BUILD_ROOT/toolchain.sha256"
printf 'materialized=%s\nmanifest_sha256=%s\n' "$FINAL_ROOT" "$(cat "$BUILD_ROOT/toolchain.sha256")"
