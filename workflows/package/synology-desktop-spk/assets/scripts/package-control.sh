#!/bin/sh
# DSM's Docker Project worker owns the project lifecycle. These scripts manage
# only package-private settings and readiness; they never access the Docker socket.
set -eu
umask 077

# DSM removes /var/packages links between old-package removal and new preinst.
# Volume-backed persistent storage survives that interval (DSM FHS).
if [ -n "${SYNOPKG_PKGDEST_VOL:-}" ] && [ "$SYNOPKG_PKGDEST_VOL" != / ]; then
    package_home=${SYNOPKG_PKGDEST_VOL%/}/@apphome/dockpipe-desktop
    package_var=${SYNOPKG_PKGDEST_VOL%/}/@appdata/dockpipe-desktop
else
    package_home=${SYNOPKG_PKGHOME:-/var/packages/dockpipe-desktop/home}
    package_var=${SYNOPKG_PKGVAR:-/var/packages/dockpipe-desktop/var}
fi
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
    if [ ! -d "$settings" ] || [ -L "$settings" ]; then
        fail 'Dockpipe private settings are unavailable.'
    fi
    for name in username password uid gid; do
        if [ ! -f "$settings/$name" ] || [ -L "$settings/$name" ]; then
            fail 'Dockpipe private settings are incomplete.'
        fi
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

refresh_package_identity() {
    current_uid=$(id -u)
    current_gid=$(id -g)
    if [ "$current_uid" -eq 0 ] || [ "$current_gid" -eq 0 ]; then
        fail 'Dockpipe setup must run as its package account.'
    fi
    if [ "$(cat "$settings/uid")" = "$current_uid" ] && [ "$(cat "$settings/gid")" = "$current_gid" ]; then
        return
    fi
    identity_directory=$(mktemp -d "$package_home/.desktop-identity.XXXXXX")
    cleanup_identity() {
        rm -f "$identity_directory/uid" "$identity_directory/gid"
        rmdir "$identity_directory"
    }
    trap cleanup_identity EXIT
    trap 'exit 1' HUP INT TERM
    printf '%s' "$current_uid" > "$identity_directory/uid"
    printf '%s' "$current_gid" > "$identity_directory/gid"
    chmod 600 "$identity_directory/uid" "$identity_directory/gid"
    mv "$identity_directory/uid" "$settings/uid"
    mv "$identity_directory/gid" "$settings/gid"
    rmdir "$identity_directory"
    trap - EXIT HUP INT TERM
}

install_settings() {
    if [ -e "$settings" ] || [ -L "$settings" ]; then
        validate_saved_settings
        refresh_package_identity
        return
    fi
    validate_wizard
    if [ ! -d "$package_home" ] || [ -L "$package_home/desktop-settings" ]; then
        fail 'DSM has not prepared the private package home.'
    fi
    package_uid=$(id -u)
    package_gid=$(id -g)
    if [ "$package_uid" -eq 0 ] || [ "$package_gid" -eq 0 ]; then
        fail 'Dockpipe setup must run as its package account.'
    fi
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

validate_uninstall_choice() {
    case "${wizard_uninstall_data:-keep}" in
        keep|delete)
            ;;
        *)
            fail 'Choose whether to keep or delete saved Dockpipe data.'
            ;;
    esac
}

verify_desktop_stopped() {
    command -v curl >/dev/null 2>&1 || fail 'Cannot verify that Dockpipe stopped. Saved data was retained.'
    # The DSM project worker must already have stopped the published service.
    # Even a non-HTTP listener must prevent deleting data under a live service.
    peer=$(curl --silent --insecure --output /dev/null --write-out '%{remote_ip}' \
        --connect-timeout 2 --max-time 3 https://127.0.0.1:38139/ || true)
    [ -z "$peer" ] || fail 'Dockpipe port 38139 is still active. Saved data was retained; complete project cleanup first.'
}

check_removal_tools() {
    command -v find >/dev/null 2>&1 || fail 'Safe package-data removal tools are unavailable. Saved data was retained.'
    probe=$(mktemp -d "$package_home/.desktop-remove-check.XXXXXX") \
        || fail 'Cannot check package-data removal support. Saved data was retained.'
    if ! find "$probe" -xdev -depth -mindepth 1 -delete; then
        rmdir "$probe"
        fail 'DSM find does not support safe package-data removal. Saved data was retained.'
    fi
    rmdir "$probe"
}

remove_saved_data() {
    # Upgrades also invoke postuninst. Never treat them as a data-removal request.
    [ "${SYNOPKG_PKG_STATUS:-}" = UNINSTALL ] || return 0
    validate_uninstall_choice
    [ "${wizard_uninstall_data:-keep}" = delete ] || return 0
    command -v find >/dev/null 2>&1 || fail 'Safe package-data removal tools are unavailable. Saved data was retained.'

    case "${SYNOPKG_PKGDEST_VOL:-}" in
        ''|/|[!/]*)
            fail 'Cannot identify the persistent Dockpipe volume. Saved data was retained.'
            ;;
    esac
    volume=$(CDPATH='' cd "$SYNOPKG_PKGDEST_VOL" && pwd -P) \
        || fail 'Cannot resolve the Dockpipe volume. Saved data was retained.'
    # Validate both roots before deleting anything. Follow no package-root symlink.
    for directory in "$package_home" "$package_var"; do
        if [ -L "$directory" ]; then
            fail 'Refusing to remove saved data through a symbolic link.'
        fi
        if [ ! -e "$directory" ]; then
            continue
        fi
        [ -d "$directory" ] || fail 'A Dockpipe data directory is invalid.'
        physical=$(CDPATH='' cd "$directory" && pwd -P) \
            || fail 'Cannot resolve a Dockpipe data directory.'
        case "$physical" in
            "$volume/@apphome/dockpipe-desktop"|"$volume/@appdata/dockpipe-desktop")
                ;;
            *)
                fail 'Refusing to remove data outside Dockpipe package storage.'
                ;;
        esac
    done
    for directory in "$package_home" "$package_var"; do
        if [ -d "$directory" ]; then
            # Preserve DSM's root directory. Do not follow symlinks or cross
            # filesystem-device boundaries beneath the package's storage.
            find "$directory" -xdev -depth -mindepth 1 -delete \
                || fail 'Some Dockpipe data could not be removed. Check package logs.'
        fi
    done
}

is_ready() {
    code=$(curl --silent --insecure --output /dev/null --write-out '%{http_code}' \
        --connect-timeout 2 --max-time 3 https://127.0.0.1:38139/webman/3rdparty/dockpipe-desktop/desktop/) || return 1
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
        elif [ "${SYNOPKG_PKG_STATUS:-}" = UPGRADE ]; then
            fail 'Saved Dockpipe settings are unavailable during upgrade. Keep the package data and check its persistent home.'
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
    preuninst)
        if [ "${SYNOPKG_PKG_STATUS:-}" = UNINSTALL ]; then
            validate_uninstall_choice
            if [ "${wizard_uninstall_data:-keep}" = delete ]; then
                check_removal_tools
            fi
        fi
        ;;
    postuninst)
        if [ "${SYNOPKG_PKG_STATUS:-}" = UNINSTALL ]; then
            validate_uninstall_choice
            verify_desktop_stopped
            remove_saved_data
        fi
        ;;
    stop|prestop)
        # The DSM worker stops the project after this script returns.
        ;;
    *)
        fail 'Unsupported Dockpipe package lifecycle action.'
        ;;
esac
