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
    package_etc=${SYNOPKG_PKGDEST_VOL%/}/@appconf/dockpipe-desktop
else
    package_home=${SYNOPKG_PKGHOME:-/var/packages/dockpipe-desktop/home}
    package_var=${SYNOPKG_PKGVAR:-/var/packages/dockpipe-desktop/var}
    package_etc=${SYNOPKG_PKGETC:-/var/packages/dockpipe-desktop/etc}
fi
settings=$package_home/desktop-settings
package_target=${SYNOPKG_PKGDEST:-/var/packages/dockpipe-desktop/target}
native_binary=$package_target/bin/dockpipe-dsm-host
native_unit=pkguser-dockpipe-desktop-native-host.service
native_root=$package_etc/native-host
worker_socket=$native_root/native-worker.sock
client_socket=$package_var/native-client/client.sock

fail() {
    echo "$1" >&2
    exit 1
}

validate_restore_selection() {
    restore_intent=${wizard_restore_intent:-}
    case "$restore_intent" in
        '')
            # Existing command-line callers still use the saved-settings check.
            return 0
            ;;
        restore|fresh|upgrade)
            ;;
        *)
            fail 'Dockpipe restore selection is invalid. Reopen the installation wizard.'
            ;;
    esac

    restore_destination=${SYNOPKG_PKGDEST_VOL:-}
    case "$restore_destination" in
        ''|/|[!/]*)
            fail 'DSM has not identified the installation volume for Dockpipe.'
            ;;
    esac
    restore_destination=${restore_destination%/}
    restore_selected_volume=${wizard_restore_volume:-}
    restore_selected_volume=${restore_selected_volume%/}
    if [ "$restore_intent" = upgrade ] && [ -z "$restore_selected_volume" ]; then
        # Repair's upgrade wizard has no volume selector. Only DSM's native
        # upgrade lifecycle may supply the destination without a browser hint.
        [ "${SYNOPKG_PKG_STATUS:-}" = UPGRADE ] \
            || fail 'DSM has not confirmed an existing Dockpipe upgrade volume.'
    else
        [ "$restore_selected_volume" = "$restore_destination" ] \
            || fail 'Dockpipe restore selection does not match the installation volume. Reopen the wizard with the intended volume.'
    fi

    case "$restore_intent" in
        restore|upgrade)
            [ -d "$settings" ] && [ ! -L "$settings" ] \
                || fail 'Saved Dockpipe sign-in settings are unavailable. Keep the retained data and select its original installation volume.'
            ;;
        fresh)
            for retained_path in "$settings" "$package_etc/install-state" \
                "$package_etc/native-host/endpoint-public/ca.pem"; do
                if [ -e "$retained_path" ] || [ -L "$retained_path" ]; then
                    fail 'Saved Dockpipe data already exists on this volume. Reopen the wizard to restore its sign-in and workspace.'
                fi
            done
            ;;
    esac
}

publish_restore_state() {
    restore_marker=$package_etc/install-state
    if [ -L "$restore_marker" ] || { [ -e "$restore_marker" ] && [ ! -f "$restore_marker" ]; }; then
        fail 'Dockpipe restore metadata must be a regular file.'
    fi
    restore_temporary=$(mktemp "$package_etc/.install-state.XXXXXX") \
        || fail 'Cannot prepare Dockpipe restore metadata.'
    if ! printf 'dockpipe.saved-settings.v1\n' > "$restore_temporary" \
        || ! chmod 644 "$restore_temporary" \
        || ! mv -f "$restore_temporary" "$restore_marker"; then
        rm -f "$restore_temporary"
        fail 'Cannot publish Dockpipe restore metadata.'
    fi
}

valid_installer_payload() {
    payload_root=$1
    [ -d "$payload_root" ] && [ ! -L "$payload_root" ] \
        && [ -d "$payload_root/bin" ] && [ ! -L "$payload_root/bin" ] \
        && [ -d "$payload_root/desktop" ] && [ ! -L "$payload_root/desktop" ] \
        && [ -f "$payload_root/bin/dockpipe-dsm-host" ] && [ ! -L "$payload_root/bin/dockpipe-dsm-host" ] \
        && [ -x "$payload_root/bin/dockpipe-dsm-host" ] \
        && [ -f "$payload_root/desktop/compose.base.yml" ] && [ ! -L "$payload_root/desktop/compose.base.yml" ] \
        && [ -r "$payload_root/desktop/compose.base.yml" ]
}

describe_installer_candidate() {
    candidate_root=$1
    helper_regular=no
    helper_executable=no
    base_regular=no
    base_readable=no
    if [ -f "$candidate_root/bin/dockpipe-dsm-host" ] && [ ! -L "$candidate_root/bin/dockpipe-dsm-host" ]; then
        helper_regular=yes
    fi
    if [ -x "$candidate_root/bin/dockpipe-dsm-host" ]; then
        helper_executable=yes
    fi
    if [ -f "$candidate_root/desktop/compose.base.yml" ] && [ ! -L "$candidate_root/desktop/compose.base.yml" ]; then
        base_regular=yes
    fi
    if [ -r "$candidate_root/desktop/compose.base.yml" ]; then
        base_readable=yes
    fi
    printf 'Candidate %s: helper_regular=%s helper_executable=%s base_regular=%s base_readable=%s\n' \
        "$candidate_root" "$helper_regular" "$helper_executable" "$base_regular" "$base_readable" >&2
}

select_installer_helper() {
    installer_temporary=${SYNOPKG_PKGINST_TEMP_DIR:-}
    case "$installer_temporary" in
        ''|/|[!/]*)
            fail 'DSM installer staging must be an absolute temporary directory.'
            ;;
    esac
    temporary_lines=$(printf '%s' "$installer_temporary" | wc -l | tr -d ' ')
    if [ "$temporary_lines" -ne 0 ] || printf '%s' "$installer_temporary" | LC_ALL=C grep -q '[[:cntrl:]]'; then
        fail 'DSM installer staging contains invalid path characters.'
    fi
    installer_temporary=${installer_temporary%/}
    selected_payload=
    candidate_count=0
    # DSM may identify the extracted payload itself or its enclosing SPK
    # directory. Never search elsewhere or select an installed old helper.
    for candidate in "$installer_temporary" "$installer_temporary/package"; do
        if valid_installer_payload "$candidate"; then
            selected_payload=$candidate
            candidate_count=$((candidate_count + 1))
        fi
    done
    if [ "$candidate_count" -ne 1 ]; then
        printf 'Dockpipe installer staging: %s; matching payloads: %s\n' "$installer_temporary" "$candidate_count" >&2
        describe_installer_candidate "$installer_temporary"
        describe_installer_candidate "$installer_temporary/package"
        fail 'Dockpipe installer helper is unavailable or ambiguous.'
    fi
    candidate_native=$selected_payload/bin/dockpipe-dsm-host
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
    for port in 38139 38143; do
        peer=$(curl --silent --insecure --output /dev/null --write-out '%{remote_ip}' \
            --noproxy '*' --connect-timeout 2 --max-time 3 "https://127.0.0.1:$port/" || true)
        [ -z "$peer" ] || fail "Dockpipe port $port is still active. Saved data was retained; complete service cleanup first."
    done
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
    # Validate every root before deleting anything. Follow no package-root symlink.
    for directory in "$package_home" "$package_var" "$package_etc"; do
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
            "$volume/@apphome/dockpipe-desktop"|"$volume/@appdata/dockpipe-desktop"|"$volume/@appconf/dockpipe-desktop")
                ;;
            *)
                fail 'Refusing to remove data outside Dockpipe package storage.'
                ;;
        esac
    done
    for directory in "$package_home" "$package_var" "$package_etc"; do
        if [ -d "$directory" ]; then
            # Preserve DSM's root directory. Do not follow symlinks or cross
            # filesystem-device boundaries beneath the package's storage.
            find "$directory" -xdev -depth -mindepth 1 -delete \
                || fail 'Some Dockpipe data could not be removed. Check package logs.'
        fi
    done
}

stream_ready() {
    code=$(curl --silent --insecure --output /dev/null --write-out '%{http_code}' \
        --noproxy '*' --connect-timeout 2 --max-time 3 https://127.0.0.1:38139/webman/3rdparty/dockpipe-desktop/desktop/) || return 1
    # This read-only loopback probe accepts only the authentication gate.
    [ "$code" = 401 ]
}

native_ready() {
    for socket in "$worker_socket" "$client_socket"; do
        [ ! -L "$socket" ] && [ -S "$socket" ] || return 1
    done
    code=$(curl --silent --output /dev/null --write-out '%{http_code}' \
        --noproxy '*' --proto '=https' --cacert "$native_root/endpoint-public/ca.pem" \
        --resolve localhost:38143:127.0.0.1 --connect-timeout 2 --max-time 3 \
        https://localhost:38143/health) || return 1
    [ "$code" = 204 ]
}

action=$(basename "$0")
if [ "$action" = start-stop-status ]; then
    action=${1:-}
fi
case "$action" in
    preinst)
        command -v curl >/dev/null 2>&1 || fail 'DSM HTTPS readiness tools are unavailable.'
        command -v synosystemctl >/dev/null 2>&1 || fail 'DSM native service tools are unavailable.'
        validate_restore_selection
        if [ -e "$settings" ] || [ -L "$settings" ]; then
            validate_saved_settings
        elif [ "${SYNOPKG_PKG_STATUS:-}" = UPGRADE ]; then
            fail 'Saved Dockpipe settings are unavailable during upgrade. Keep the package data and check its persistent home.'
        else
            validate_wizard
        fi
        # DSM has extracted the candidate payload but has not acquired its
        # Docker Project resource yet. Change only that staging payload.
        select_installer_helper
        "$candidate_native" folder-prepare
        ;;
    postinst)
        install_settings
        [ -d "$package_var" ] || fail 'DSM has not prepared the package data directory.'
        chmod 700 "$package_var"
        [ -d "$package_etc" ] || fail 'DSM has not prepared the package configuration directory.'
        [ ! -L "$native_root" ] || fail 'Native host settings must not be a symbolic link.'
        mkdir -p "$native_root"
        chmod 755 "$package_etc" "$native_root" "$package_target" "$package_target/bin" \
            "$package_target/ui" "$package_target/ui/images"
        "$native_binary" folder-save
        "$native_binary" endpoint-init
        # Public, non-secret metadata is published only after private settings
        # and native initialization have both completed successfully.
        publish_restore_state
        ;;
    preupgrade|postupgrade|prestart)
        validate_saved_settings
        ;;
    start)
        validate_saved_settings
        # Image construction happens during installation. Startup gets two minutes
        # for the desktop's HTTPS gate, with individually bounded local probes.
        deadline=$(( $(date +%s) + 120 ))
        until stream_ready; do
            [ "$(date +%s)" -lt "$deadline" ] || fail 'Dockpipe did not become ready. Check Package Center logs.'
            sleep 1
        done
        synosystemctl start "$native_unit"
        deadline=$(( $(date +%s) + 30 ))
        until native_ready; do
            [ "$(date +%s)" -lt "$deadline" ] || fail 'Dockpipe native host service did not become ready. Check Package Center logs.'
            sleep 1
        done
        ;;
    status)
        if stream_ready && native_ready; then
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
    stop)
        synosystemctl stop "$native_unit"
        ;;
    prestop)
        # The owned stop action stops the unit; DSM then stops the project.
        ;;
    *)
        fail 'Unsupported Dockpipe package lifecycle action.'
        ;;
esac
