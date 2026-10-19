#!/usr/bin/env bash
# Source from package build/test entrypoints; prepare only verified SDK bytes.
cockpit_prepare_native() {
  local sdk_work="${1:?fresh SDK working directory}"
  local scripts_dir="${2:?package scripts directory}"
  local qt_root="${DOCKPIPE_QT_ROOT:?Set DOCKPIPE_QT_ROOT to the matching Qt 6.10.3 gcc_64 SDK}"
  local qmake="$qt_root/bin/qmake"
  local qt_version
  [[ -x "$qmake" ]] || { echo 'The selected Qt SDK has no executable bin/qmake.' >&2; return 1; }
  qt_version="$("$qmake" -query QT_VERSION)"
  [[ "$qt_version" == 6.10.3 ]] || { echo "Cockpit requires Qt 6.10.3 exactly; selected $qt_version." >&2; return 1; }
  DOCKPIPE_FORGEPIPE_SDK="$(python3 "$scripts_dir/prepare-forgepipe-sdk.py" \
    --lock "$scripts_dir/../forgepipe-sdk.lock.json" --output "$sdk_work/sdk")" || return
  export DOCKPIPE_FORGEPIPE_SDK
  export DOCKPIPE_QMAKE="$qmake"
  export CMAKE_PREFIX_PATH="$DOCKPIPE_FORGEPIPE_SDK;$qt_root"
  export LD_LIBRARY_PATH="$qt_root/lib:$DOCKPIPE_FORGEPIPE_SDK/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
}
