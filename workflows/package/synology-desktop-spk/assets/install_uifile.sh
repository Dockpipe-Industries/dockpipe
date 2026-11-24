#!/bin/sh
set -eu
# DSM reads the generated wizard from this framework-owned temporary file.
@RESTORE_DISCOVERY_HELPERS@

render_install_wizard() {
    restore_discover "$1" "${SYNOPKG_PKGDEST_VOL:-}" "${SYNOPKG_PKGHOME:-}"
    if [ "$restore_mode" = fresh ]; then
        restore_render_wizard > "$SYNOPKG_TEMP_LOGFILE" <<'JSON'
@INSTALL_WIZARD@
JSON
    else
        restore_render_wizard > "$SYNOPKG_TEMP_LOGFILE" <<'JSON'
[
  {
    "step_title": "Restore Dockpipe",
    "items": [
      {
        "desc": "Saved Dockpipe settings are checked before continuing. Your existing sign-in and workspace will be retained; no replacement credentials are requested."
      }
    ]
  },
@HOST_FOLDER_WIZARD@
]
JSON
    fi
}

render_install_wizard /
