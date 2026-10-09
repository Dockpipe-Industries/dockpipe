#!/usr/bin/env bash
# Explicit package-owned bridges use the host environment and host credentials.
set -euo pipefail
command="${1:?host integration command}"
shift
case "$command" in
  op|code|cursor|codex|claude|npm|systemctl|loginctl|pkexec|nvidia-smi|nvidia-ctk|qemu-img|qemu-system-x86_64)
    ;;
  *)
    printf 'Unsupported host integration: %s\n' "$command" >&2
    exit 2
    ;;
esac
exec /usr/bin/flatpak-spawn --host --watch-bus --directory="$PWD" \
  --unset-env=FLATPAK_ID --unset-env=LD_LIBRARY_PATH --unset-env=LD_PRELOAD \
  --unset-env=DOCKPIPE_BIN --unset-env=DOCKPIPE_SYSTEM_ROOT "$command" "$@"
