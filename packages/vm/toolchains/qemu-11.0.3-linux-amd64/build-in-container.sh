#!/bin/bash
set -euo pipefail

if (($# != 0)); then
  echo "this purpose-specific builder accepts no arguments" >&2
  exit 2
fi

readonly SOURCE_ARCHIVE=/input/qemu-11.0.3.tar.xz
readonly SOURCE_SHA256=da5fcffc32762820568b828ed430a728864d34d50b6d2f30358597760cbb0523
readonly FINAL_ROOT=/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1
readonly SOURCE_DATE_EPOCH=1784926308

test "$(/usr/bin/sha256sum "$SOURCE_ARCHIVE" | /usr/bin/cut -d' ' -f1)" = "$SOURCE_SHA256"
test ! -e /build/source
test ! -e /build/build
test ! -e /record/output
test ! -e /record/stage

/bin/mkdir -p "$FINAL_ROOT/lib"
test ! -e "$FINAL_ROOT/lib/ld-musl-x86_64.so.1"
test ! -e "$FINAL_ROOT/lib/libc.musl-x86_64.so.1"
/bin/cp -L /lib/ld-musl-x86_64.so.1 "$FINAL_ROOT/lib/ld-musl-x86_64.so.1"
/bin/cp -L /lib/ld-musl-x86_64.so.1 "$FINAL_ROOT/lib/libc.musl-x86_64.so.1"
/bin/chmod 0500 "$FINAL_ROOT/lib/ld-musl-x86_64.so.1" "$FINAL_ROOT/lib/libc.musl-x86_64.so.1"

/bin/mkdir -p /build/source /build/build /build/home /record/output/bin /record/output/lib /record/output/share/qemu /record/stage
/bin/tar -xJf "$SOURCE_ARCHIVE" --strip-components=1 --no-same-owner --no-same-permissions -C /build/source
/usr/bin/find /build/source -depth -exec /bin/touch -h -d "@$SOURCE_DATE_EPOCH" {} +
/bin/chmod -R u+rwX,go-rwx /build/source

# The ELF token in the linker flags must remain literal.
# shellcheck disable=SC2016
configure_arguments=(
  "--prefix=$FINAL_ROOT"
  "--bindir=$FINAL_ROOT/bin"
  "--datadir=$FINAL_ROOT/share"
  "--libdir=$FINAL_ROOT/lib"
  "--target-list=x86_64-softmmu"
  "--cc=/usr/bin/gcc"
  "--host-cc=/usr/bin/gcc"
  "--python=/usr/bin/python3"
  "--ninja=/usr/bin/ninja"
  "--without-default-features"
  "--enable-kvm"
  "--disable-tcg"
  "--enable-tools"
  "--disable-pixman"
  "--disable-relocatable"
  "--disable-docs"
  "--disable-guest-agent"
  "--disable-modules"
  "--disable-plugins"
  "--disable-download"
  "--disable-containers"
  "--disable-strip"
  "--disable-werror"
  "--extra-cflags=-O2 -g0 -fdebug-prefix-map=/build=/usr/src/qemu -ffile-prefix-map=/build=/usr/src/qemu -fmacro-prefix-map=/build=/usr/src/qemu"
  '--extra-ldflags=-Wl,--build-id=none -Wl,--disable-new-dtags -Wl,-rpath,$ORIGIN/../lib -Wl,-z,nodefaultlib -Wl,--dynamic-linker=/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1/lib/ld-musl-x86_64.so.1'
)

cd /build/build
/build/source/configure "${configure_arguments[@]}" 2>&1 | /usr/bin/tee /record/configure.log
/usr/bin/ninja -j1 qemu-img qemu-system-x86_64 2>&1 | /usr/bin/tee /record/build.log
DESTDIR=/record/stage /usr/bin/ninja -j1 install 2>&1 | /usr/bin/tee /record/install.log

readonly staged_root="/record/stage$FINAL_ROOT"
test -f "$staged_root/bin/qemu-img"
test -f "$staged_root/bin/qemu-system-x86_64"
test -d "$staged_root/share/qemu"

/bin/cp "$staged_root/bin/qemu-img" /record/output/bin/qemu-img
/bin/cp "$staged_root/bin/qemu-system-x86_64" /record/output/bin/qemu-system-x86_64
/usr/bin/strip --strip-unneeded /record/output/bin/qemu-img /record/output/bin/qemu-system-x86_64

while IFS= read -r -d '' source_file; do
  relative=${source_file#"$staged_root/share/qemu/"}
  destination="/record/output/share/qemu/$relative"
  /bin/mkdir -p "$(/usr/bin/dirname "$destination")"
  /bin/cp -L "$source_file" "$destination"
done < <(/usr/bin/find "$staged_root/share/qemu" \( -type f -o -type l \) -print0 | /usr/bin/sort -z)

declare -A copied_libraries=()
library_queue=()
enqueue_needed() {
  local elf=$1 needed name
  needed=$(/usr/bin/scanelf -q -n -F '%n' "$elf" | /usr/bin/awk '{print $1}')
  IFS=',' read -r -a names <<<"$needed"
  for name in "${names[@]}"; do
    [[ -n "$name" ]] && library_queue+=("$name")
  done
}

enqueue_needed /record/output/bin/qemu-img
enqueue_needed /record/output/bin/qemu-system-x86_64
while ((${#library_queue[@]})); do
  name=${library_queue[0]}
  library_queue=("${library_queue[@]:1}")
  [[ ${copied_libraries[$name]+yes} ]] && continue
  source_path=
  for candidate in "/lib/$name" "/usr/lib/$name"; do
    if [[ -e "$candidate" ]]; then
      source_path=$(/usr/bin/readlink -f "$candidate")
      break
    fi
  done
  if [[ -z "$source_path" || ! -f "$source_path" ]]; then
    echo "unresolved runtime library: $name" >&2
    exit 1
  fi
  /bin/cp -L "$source_path" "/record/output/lib/$name"
  copied_libraries[$name]=$source_path
  printf '%s\t%s\n' "$name" "$source_path" >>/record/runtime-library-sources.tsv
  enqueue_needed "/record/output/lib/$name"
done

/bin/cp -L /lib/ld-musl-x86_64.so.1 /record/output/lib/ld-musl-x86_64.so.1
printf '%s\t%s\n' ld-musl-x86_64.so.1 "$(/usr/bin/readlink -f /lib/ld-musl-x86_64.so.1)" >>/record/runtime-library-sources.tsv
/usr/bin/sort -o /record/runtime-library-sources.tsv /record/runtime-library-sources.tsv

if /usr/bin/find /record/output -type l -print -quit | /bin/grep -q .; then
  echo "output contains a symlink" >&2
  exit 1
fi
if /bin/grep -R -a -E -l '/build/(source|build)(/|$)' /record/output > /record/embedded-build-paths.txt; then
  echo "output embeds the container build path" >&2
  exit 1
fi

/usr/bin/readelf -l /record/output/bin/qemu-img > /record/qemu-img.program-headers.txt
/usr/bin/readelf -d /record/output/bin/qemu-img > /record/qemu-img.dynamic.txt
/usr/bin/readelf -l /record/output/bin/qemu-system-x86_64 > /record/qemu-system-x86_64.program-headers.txt
/usr/bin/readelf -d /record/output/bin/qemu-system-x86_64 > /record/qemu-system-x86_64.dynamic.txt
/bin/grep -F "[Requesting program interpreter: $FINAL_ROOT/lib/ld-musl-x86_64.so.1]" /record/qemu-img.program-headers.txt
/bin/grep -F "[Requesting program interpreter: $FINAL_ROOT/lib/ld-musl-x86_64.so.1]" /record/qemu-system-x86_64.program-headers.txt
# The readelf output contains the literal ELF token.
# shellcheck disable=SC2016
/bin/grep -F 'Library rpath: [$ORIGIN/../lib]' /record/qemu-img.dynamic.txt
# The readelf output contains the literal ELF token.
# shellcheck disable=SC2016
/bin/grep -F 'Library rpath: [$ORIGIN/../lib]' /record/qemu-system-x86_64.dynamic.txt
/bin/grep -F 'NODEFLIB' /record/qemu-img.dynamic.txt
/bin/grep -F 'NODEFLIB' /record/qemu-system-x86_64.dynamic.txt

while IFS= read -r -d '' directory; do /bin/chmod 0500 "$directory"; done < <(/usr/bin/find /record/output -type d -print0)
/bin/chmod 0500 /record/output/bin/qemu-img /record/output/bin/qemu-system-x86_64 /record/output/lib/ld-musl-x86_64.so.1
while IFS= read -r -d '' file; do
  case "$file" in
    /record/output/bin/qemu-img|/record/output/bin/qemu-system-x86_64|/record/output/lib/ld-musl-x86_64.so.1) ;;
    *) /bin/chmod 0400 "$file" ;;
  esac
  /bin/touch -h -d "@$SOURCE_DATE_EPOCH" "$file"
done < <(/usr/bin/find /record/output -type f -print0)
while IFS= read -r -d '' directory; do /bin/touch -h -d "@$SOURCE_DATE_EPOCH" "$directory"; done < <(/usr/bin/find /record/output -depth -type d -print0)

(
  cd /record/output
  while IFS= read -r -d '' file; do
    relative=${file#./}
    printf '%04o\t%s\t%s\n' "$((8#$(/bin/stat -c '%a' "$file")))" "$(/usr/bin/sha256sum "$file" | /usr/bin/cut -d' ' -f1)" "$relative"
  done < <(/usr/bin/find . -type f -print0 | /usr/bin/sort -z)
) > /record/output-files.tsv
/usr/bin/sha256sum /record/output-files.tsv | /usr/bin/cut -d' ' -f1 > /record/output-files.sha256
printf '%s\n' "${configure_arguments[@]}" > /record/configure-argv.txt
/usr/bin/sha256sum /record/configure-argv.txt | /usr/bin/cut -d' ' -f1 > /record/configure-argv.sha256
/usr/bin/env | /usr/bin/sort > /record/build-environment.txt
/usr/bin/sha256sum /record/build-environment.txt | /usr/bin/cut -d' ' -f1 > /record/build-environment.sha256
