#!/bin/sh
set -eu
# DSM reads the generated wizard from this framework-owned temporary file.
if [ -d "${SYNOPKG_PKGHOME:-/var/packages/dockpipe-desktop/home}/desktop-settings" ]; then
    cat > "$SYNOPKG_TEMP_LOGFILE" <<'JSON'
[{"step_title":"Restore Dockpipe","items":[{"desc":"Your existing Dockpipe sign-in and workspace will be retained. After installation, open Dockpipe from DSM and use your previous sign-in."}]}]
JSON
else
    cat > "$SYNOPKG_TEMP_LOGFILE" <<'JSON'
@INSTALL_WIZARD@
JSON
fi
