#!/bin/sh
# The DSM worker may create the project before postinst stores wizard settings.
# Never start the web desktop until complete private settings are available.
set -eu

settings=/run/dockpipe-package/desktop-settings
if [ ! -d "$settings" ] || [ -L "$settings" ]; then
    echo 'Dockpipe setup is incomplete. Complete installation in Package Center.' >&2
    exit 1
fi
for name in username password uid gid; do
    if [ ! -f "$settings/$name" ] || [ -L "$settings/$name" ]; then
        echo 'Dockpipe setup is incomplete. Complete installation in Package Center.' >&2
        exit 1
    fi
done

username=$(cat "$settings/username")
username_lines=$(wc -l < "$settings/username" | tr -d ' ')
password_length=$(wc -c < "$settings/password" | tr -d ' ')
password_lines=$(wc -l < "$settings/password" | tr -d ' ')
PUID=$(cat "$settings/uid")
PGID=$(cat "$settings/gid")
for identity in "$PUID" "$PGID"; do
    case "$identity" in
        ''|*[!0-9]*)
            echo 'Dockpipe package identity is invalid.' >&2
            exit 1
            ;;
    esac
done
if [ "$PUID" -eq 0 ] || [ "$PGID" -eq 0 ]; then
    echo 'Dockpipe requires an unprivileged package identity.' >&2
    exit 1
fi
if [ "$username_lines" -ne 0 ] || [ "$password_lines" -ne 0 ] \
    || ! printf '%s' "$username" | LC_ALL=C grep -Eq '^[A-Za-z][A-Za-z0-9_.-]{0,31}$' \
    || ! LC_ALL=C grep -Eq '^[ -~]{16,128}$' "$settings/password" \
    || [ "$password_length" -lt 16 ] || [ "$password_length" -gt 128 ]; then
    echo 'Dockpipe sign-in settings are invalid.' >&2
    exit 1
fi

# LinuxServer reads these references before starting its authenticated HTTPS service.
export PUID PGID
export FILE__CUSTOM_USER="$settings/username"
export FILE__PASSWORD="$settings/password"
exec /init "$@"
