#!/bin/sh
set -eu
# DSM reads the generated wizard from this framework-owned temporary file.
if [ -n "${SYNOPKG_PKGDEST_VOL:-}" ] && [ "$SYNOPKG_PKGDEST_VOL" != / ]; then
    package_home=${SYNOPKG_PKGDEST_VOL%/}/@apphome/dockpipe-desktop
else
    package_home=${SYNOPKG_PKGHOME:-/var/packages/dockpipe-desktop/home}
fi
if [ -d "$package_home/desktop-settings" ]; then
    cat > "$SYNOPKG_TEMP_LOGFILE" <<'JSON'
[{"step_title":"Restore Dockpipe","items":[{"desc":"Your existing Dockpipe sign-in and workspace will be retained. After installation, open Dockpipe from DSM and use your previous sign-in."}]}]
JSON
else
    cat > "$SYNOPKG_TEMP_LOGFILE" <<'JSON'
@INSTALL_WIZARD@
JSON
fi
