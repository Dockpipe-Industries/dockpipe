#!/bin/sh
# DSM's Docker Project worker owns the project lifecycle. These scripts manage
# only package-private settings and readiness; they never access the Docker socket.
set -eu
umask 077

package_home=${SYNOPKG_PKGHOME:-/var/packages/dockpipe-desktop/home}
package_var=${SYNOPKG_PKGVAR:-/var/packages/dockpipe-desktop/var}
settings=$package_home/desktop-settings

fail() {
    echo "$1" >&2
    exit 1
}

valid_username() {
    lines=$(printf '%s' "$1" | wc -l | tr -d ' ')
    [ "$lines" -eq 0 ] \
        && printf '%s' "$1" | LC_ALL=C grep -Eq '^[A-Za-z][A-Za-z0-9_.-]{0,31}$'
}

valid_password() {
    length=$(printf '%s' "$1" | wc -c | tr -d ' ')
    lines=$(printf '%s' "$1" | wc -l | tr -d ' ')
    [ "$lines" -eq 0 ] && [ "$length" -ge 16 ] && [ "$length" -le 128 ] \
        && printf '%s' "$1" | LC_ALL=C grep -Eq '^[ -~]{16,128}$'
}

validate_saved_settings() {
    [ -d "$settings" ] && [ ! -L "$settings" ] || fail 'Dockpipe private settings are unavailable.'
    for name in username password uid gid; do
        [ -f "$settings/$name" ] && [ ! -L "$settings/$name" ] \
            || fail 'Dockpipe private settings are incomplete.'
    done
    for name in username password; do
        lines=$(wc -l < "$settings/$name" | tr -d ' ')
        [ "$lines" -eq 0 ] || fail 'Dockpipe private settings contain an invalid line break.'
    done
    valid_username "$(cat "$settings/username")" || fail 'Dockpipe username is invalid.'
    valid_password "$(cat "$settings/password")" || fail 'Dockpipe password is invalid.'
    for name in uid gid; do
        identity=$(cat "$settings/$name")
        case "$identity" in
            ''|*[!0-9]*)
                fail 'Dockpipe package identity is invalid.'
                ;;
        esac
        [ "$identity" -ne 0 ] || fail 'Dockpipe requires an unprivileged package identity.'
    done
}

validate_wizard() {
    valid_username "${wizard_username:-}" || fail 'Choose a valid Dockpipe username in the installation wizard.'
    valid_password "${wizard_password:-}" || fail 'Choose a 16–128 character Dockpipe password in the installation wizard.'
    [ "${wizard_password:-}" = "${wizard_password_confirm:-}" ] || fail 'The Dockpipe passwords do not match.'
}

install_settings() {
    if [ -e "$settings" ] || [ -L "$settings" ]; then
        validate_saved_settings
        return
    fi
    validate_wizard
    [ -d "$package_home" ] && [ ! -L "$package_home/desktop-settings" ] \
        || fail 'DSM has not prepared the private package home.'
    package_uid=$(id -u)
    package_gid=$(id -g)
    [ "$package_uid" -ne 0 ] && [ "$package_gid" -ne 0 ] \
        || fail 'Dockpipe setup must run as its package account.'
    temporary=$(mktemp -d "$package_home/.desktop-settings.XXXXXX")
    cleanup_settings() {
        if [ -n "${temporary:-}" ]; then
            rm -f "$temporary/username" "$temporary/password" "$temporary/uid" "$temporary/gid"
            rmdir "$temporary"
        fi
    }
    trap cleanup_settings EXIT
    trap 'exit 1' HUP INT TERM
    printf '%s' "$wizard_username" > "$temporary/username"
    printf '%s' "$wizard_password" > "$temporary/password"
    printf '%s' "$package_uid" > "$temporary/uid"
    printf '%s' "$package_gid" > "$temporary/gid"
    chmod 600 "$temporary/username" "$temporary/password" "$temporary/uid" "$temporary/gid"
    mv "$temporary" "$settings"
    temporary=
    trap - EXIT HUP INT TERM
    unset wizard_password
}

is_ready() {
    code=$(curl --silent --insecure --output /dev/null --write-out '%{http_code}' \
        --connect-timeout 2 --max-time 3 https://127.0.0.1:38139/) || return 1
    # This read-only loopback probe accepts only the authentication gate.
    [ "$code" = 401 ]
}

action=$(basename "$0")
if [ "$action" = start-stop-status ]; then
    action=${1:-}
fi
case "$action" in
    preinst)
        command -v curl >/dev/null 2>&1 || fail 'DSM HTTPS readiness tools are unavailable.'
        if [ -e "$settings" ] || [ -L "$settings" ]; then
            validate_saved_settings
        else
            validate_wizard
        fi
        ;;
    postinst)
        install_settings
        [ -d "$package_var" ] || fail 'DSM has not prepared the package data directory.'
        chmod 700 "$package_var"
        ;;
    preupgrade|postupgrade|prestart)
        validate_saved_settings
        ;;
    start)
        validate_saved_settings
        # Image construction happens during installation. Startup gets two minutes
        # for the desktop's HTTPS gate, with individually bounded local probes.
        deadline=$(( $(date +%s) + 120 ))
        until is_ready; do
            [ "$(date +%s)" -lt "$deadline" ] || fail 'Dockpipe did not become ready. Check Package Center logs.'
            sleep 1
        done
        ;;
    status)
        if is_ready; then
            exit 0
        fi
        exit 3
        ;;
    stop|prestop|preuninst|postuninst)
        # The worker stops/removes the project. Retain private settings and data.
        ;;
    *)
        fail 'Unsupported Dockpipe package lifecycle action.'
        ;;
esac
