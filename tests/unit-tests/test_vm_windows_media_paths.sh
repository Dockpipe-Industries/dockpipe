#!/usr/bin/env bash
set -euo pipefail

repository_root="${DOCKPIPE_VM_MEDIA_TEST_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
script_path="${DOCKPIPE_VM_RUNTIME_TEST_SCRIPT:-$repository_root/src/core/assets/scripts/vmimage-run.sh}"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
export DOCKPIPE_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:?select an actual pinned Dockpipe CLI}"
export DOCKPIPE_WORKDIR="$fixture" DOCKPIPE_VMIMAGE_SOURCE_ONLY=1
export XDG_STATE_HOME="$fixture/state" XDG_CACHE_HOME="$fixture/cache"
unset DOCKPIPE_PACKAGE_ROOT DOCKPIPE_PACKAGE_ID DOCKPIPE_PACKAGE_STATE_DIR DOCKPIPE_SDK_ROOT
source "$script_path"

fixture_windows=1
vmimage_is_windows_host() {
  [[ "$fixture_windows" == 1 ]]
}
# Replace the native conversion executable only; all production validation,
# namespace handling and command assembly remain under test.
cygpath() {
  [[ "$1" == -aw && "$2" == -- ]]
  local value="$3"
  case "$value" in
    /c/*) value="C:/${value#/c/}" ;;
    /*) value="C:$value" ;;
  esac
  printf '%s\n' "${value//\//\\}"
}
assert_path() {
  local input="$1" expected="$2" actual
  actual="$(vmimage_qemu_media_path "$input")"
  if [[ "$actual" != "$expected" ]]; then
    printf 'Unexpected native QEMU path for %s: %s\n' "$input" "$actual" >&2
    exit 1
  fi
}
assert_path 'C:\images\disk.raw' '\\?\C:\images\disk.raw'
assert_path 'C:/images/disk.raw' '\\?\C:\images\disk.raw'
assert_path '/c/images/disk.raw' '\\?\C:\images\disk.raw'
assert_path '\\server\share\disk.raw' '\\?\UNC\server\share\disk.raw'
assert_path '\\?\UNC\server\share\disk.raw' '\\?\UNC\server\share\disk.raw'
assert_path '\\?\C:\images\disk.raw' '\\?\C:\images\disk.raw'
for invalid in 'relative.raw' 'C:relative.raw' '\\server' '\\.\PhysicalDrive0' \
  '\\?\GLOBALROOT\Device\Harddisk0' 'C:\images\..\disk.raw' $'C:\\images\\bad\nfile'; do
  if (vmimage_qemu_media_path "$invalid") > "$fixture/out" 2> "$fixture/error"; then
    printf 'Invalid Windows QEMU media path accepted: %s\n' "$invalid" >&2
    exit 1
  fi
done
fixture_windows=0
assert_path '/tmp/image with spaces.raw' '/tmp/image with spaces.raw'
fixture_windows=1

cat > "$fixture/qemu-img" <<'BACKEND'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\0' "$@" > "$DOCKPIPE_QEMU_IMAGE_ARGUMENTS"
exit "${DOCKPIPE_TEST_QEMU_IMG_STATUS:-0}"
BACKEND
chmod +x "$fixture/qemu-img"
export DOCKPIPE_QEMU_IMAGE_ARGUMENTS="$fixture/arguments"
vmimage_qemu_img_bin() {
  printf '%s/qemu-img\n' "$fixture"
}
vmimage_state_dir() {
  printf '%s\n' "$fixture"
}
vmimage_base_format() {
  printf 'raw\n'
}
vmimage_confirm_prompts_enabled() {
  return 1
}
export DOCKPIPE_RUN_ID=media-path-test DOCKPIPE_VM_PERSISTENCE=ephemeral
backing='C:\images\'
for (( index = 0; index < 5; index++ )); do
  backing+='abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij\'
done
backing+='base.raw'
if (vmimage_prepare_disk "$backing") > "$fixture/out" 2> "$fixture/error"; then
  echo 'Unsupported long Windows ephemeral backing path accepted' >&2
  exit 1
fi
[[ ! -e "$fixture/arguments" ]]
grep -q '259-byte automatic backing limit' "$fixture/error"
backing='C:\images\base.raw'
prepared="$(vmimage_prepare_disk "$backing")"
[[ "$prepared" == "$fixture/overlay-media-path-test.qcow2|qcow2" ]]
export expected_backing="$(vmimage_qemu_media_path "$backing")"
export expected_overlay="$(vmimage_qemu_media_path "$fixture/overlay-media-path-test.qcow2")"
python3 - "$fixture/arguments" <<'PY'
import os
from pathlib import Path
import sys
arguments = Path(sys.argv[1]).read_bytes().rstrip(b'\0').decode().split('\0')
assert arguments == ['create', '-q', '-f', 'qcow2', '-F', 'raw', '-b',
                     os.environ['expected_backing'], os.environ['expected_overlay']], arguments
assert len(arguments[-2]) < 260
PY
rm -f "$fixture/arguments"
for length in 259 260; do
  printf -v component '%*s' "$(( length - 11 ))" ''
  boundary="C:\\${component// /x}.raw"
  [[ "${#boundary}" == "$(( length - 4 ))" ]]
  status=0
  (vmimage_prepare_disk "$boundary") > "$fixture/out" 2> "$fixture/error" || status=$?
  if [[ "$length" == 259 ]]; then
    [[ "$status" == 0 && -e "$fixture/arguments" ]]
  else
    [[ "$status" != 0 && ! -e "$fixture/arguments" ]]
  fi
  rm -f "$fixture/arguments"
done
unicode_backing='C:\'
for (( index = 0; index < 130; index++ )); do
  unicode_backing+='é'
done
unicode_backing+='.raw'
if (vmimage_prepare_disk "$unicode_backing") > "$fixture/out" 2> "$fixture/error"; then
  echo 'Multibyte backing filename exceeded byte limit without rejection' >&2
  exit 1
fi
[[ ! -e "$fixture/arguments" ]]
vmimage_ensure_disk_exists_for_install "$fixture/new disk.qcow2"
export expected_disk="$(vmimage_qemu_media_path "$fixture/new disk.qcow2")"
python3 - "$fixture/arguments" <<'PY'
import os
from pathlib import Path
import sys
arguments = Path(sys.argv[1]).read_bytes().rstrip(b'\0').decode().split('\0')
assert arguments == ['create', '-f', 'qcow2', os.environ['expected_disk'], '64G'], arguments
PY
rm -f "$fixture/arguments"
if vmimage_prepare_disk 'C:relative.raw' > "$fixture/out" 2> "$fixture/error"; then
  echo 'Invalid backing path reached qemu-img' >&2
  exit 1
fi
[[ ! -e "$fixture/arguments" ]]
export DOCKPIPE_TEST_QEMU_IMG_STATUS=23
status=0
vmimage_prepare_disk "$backing" > "$fixture/out" || status=$?
[[ "$status" == 23 && ! -s "$fixture/out" ]]
status=0
vmimage_ensure_disk_exists_for_install "$fixture/failed.qcow2" || status=$?
[[ "$status" == 23 ]]
unset DOCKPIPE_TEST_QEMU_IMG_STATUS
export DOCKPIPE_VM_PERSISTENCE=persistent
[[ "$(vmimage_prepare_disk "$unicode_backing")" == "$unicode_backing|raw" ]]
# Exercise the exact drive operand consumed by the bootstrap fallback.
fixture_windows=1
export actual_bootstrap_drive="$(vmimage_bootstrap_directory_drive '/c/runtime/bootstrap with spaces')"
python3 - <<'PYTEST'
import os
options = dict(item.split('=', 1) for item in os.environ['actual_bootstrap_drive'].split(','))
assert options == {'if': 'none', 'id': 'vbootstrap', 'format': 'vvfat',
                   'dir': r'\\?\C:\runtime\bootstrap with spaces', 'readonly': 'on'}, options
assert 'file' not in options
PYTEST
if vmimage_bootstrap_directory_drive 'relative/bootstrap' > "$fixture/out" 2> "$fixture/error"; then
  echo 'Relative Windows bootstrap directory accepted' >&2
  exit 1
fi
fixture_windows=0
[[ "$(vmimage_bootstrap_directory_drive '/tmp/bootstrap with spaces')" == \
  'if=none,id=vbootstrap,file=fat:/tmp/bootstrap with spaces,format=raw,readonly=on' ]]
printf 'Native Windows QEMU media-path contracts passed\n'

