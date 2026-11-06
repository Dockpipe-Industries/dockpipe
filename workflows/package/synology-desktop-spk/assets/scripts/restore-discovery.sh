# shellcheck shell=sh
# Discover retained Dockpipe state from standard DSM FHS metadata only.
# The caller supplies the filesystem root; packaged callers always use /.

restore_valid_volume() {
    case "$1" in
        /volume[1-9]*)
            volume_number=${1#/volume}
            case "$volume_number" in
                *[!0-9]*) return 1 ;;
                *) return 0 ;;
            esac
            ;;
        *) return 1 ;;
    esac
}

restore_append_volume() {
    case ",$restore_checked_volumes," in
        *",$1,"*) return 0 ;;
    esac
    if [ -n "$restore_checked_volumes" ]; then
        restore_checked_volumes=$restore_checked_volumes,$1
    else
        restore_checked_volumes=$1
    fi
}

restore_record_result() {
    restore_append_volume "$1"
    case "$2" in
        retained)
            if [ -n "$restore_retained_volumes" ]; then
                restore_retained_volumes=$restore_retained_volumes,$1
            else
                restore_retained_volumes=$1
            fi
            ;;
        unknown)
            if [ -n "$restore_unknown_volumes" ]; then
                restore_unknown_volumes=$restore_unknown_volumes,$1
            else
                restore_unknown_volumes=$1
            fi
            ;;
    esac
}

restore_public_marker() {
    marker_root=$1
    for directory in "$marker_root" "$marker_root/@appconf" "$marker_root/@appconf/dockpipe-desktop"; do
        [ -d "$directory" ] && [ ! -L "$directory" ] && [ -x "$directory" ] || return 1
    done
    marker=$marker_root/@appconf/dockpipe-desktop/install-state
    if restore_explicit_marker "$marker" "$marker_root/@appconf/dockpipe-desktop"; then
        return 0
    fi
    # Compatibility with successful older installations: endpoint-init ran
    # after private settings were installed. Never read certificate contents.
    for directory in "$marker_root/@appconf/dockpipe-desktop/native-host" \
        "$marker_root/@appconf/dockpipe-desktop/native-host/endpoint-public"; do
        [ -d "$directory" ] && [ ! -L "$directory" ] && [ -x "$directory" ] || return 1
    done
    marker=$marker_root/@appconf/dockpipe-desktop/native-host/endpoint-public/ca.pem
    [ -f "$marker" ] && [ ! -L "$marker" ] && [ -r "$marker" ] && [ -s "$marker" ]
}

restore_explicit_marker() {
    explicit_marker=$1
    configuration_root=$2
    [ -f "$explicit_marker" ] && [ ! -L "$explicit_marker" ] && [ -r "$explicit_marker" ] || return 1
    marker_metadata=$(LC_ALL=C stat -c '%a:%u:%s' "$explicit_marker" 2>/dev/null) || return 1
    marker_mode=${marker_metadata%%:*}
    marker_identity=${marker_metadata#*:}
    marker_owner=${marker_identity%%:*}
    marker_size=${marker_identity#*:}
    [ "$marker_mode" = 644 ] && [ "$marker_size" = 27 ] || return 1
    case "$marker_owner" in
        ''|0|*[!0-9]*) return 1 ;;
    esac
    configuration_owner=$(LC_ALL=C stat -c '%u' "$configuration_root" 2>/dev/null) || return 1
    [ "$marker_owner" = "$configuration_owner" ] || return 1
    # This marker is explicitly public and contains no credentials. Bound the
    # read even if it changes after stat; legacy certificate contents stay unread.
    marker_schema=$(LC_ALL=C head -c 28 "$explicit_marker" 2>/dev/null) || return 1
    [ "$marker_schema" = dockpipe.saved-settings.v1 ]
}

restore_inspect_volume() {
    inspected_volume=$1
    physical_volume=$restore_filesystem_root$inspected_volume
    restore_result=unknown
    if [ ! -d "$physical_volume" ] || [ -L "$physical_volume" ] || [ ! -x "$physical_volume" ]; then
        return 0
    fi
    if restore_public_marker "$physical_volume"; then
        restore_result=retained
        return 0
    fi
    for directory in "$physical_volume/@apphome" "$physical_volume/@apphome/dockpipe-desktop"; do
        if [ -L "$directory" ]; then
            return 0
        fi
        if [ ! -e "$directory" ]; then
            restore_result=fresh
            return 0
        fi
        [ -d "$directory" ] && [ -x "$directory" ] || return 0
    done
    private_settings=$physical_volume/@apphome/dockpipe-desktop/desktop-settings
    if [ -L "$private_settings" ]; then
        return 0
    fi
    if [ -d "$private_settings" ]; then
        restore_result=retained
    elif [ ! -e "$private_settings" ]; then
        restore_result=fresh
    fi
}

restore_discover() {
    restore_filesystem_root=${1%/}
    provided_volume=${2%/}
    provided_home=${3%/}
    restore_checked_volumes=
    restore_retained_volumes=
    restore_unknown_volumes=
    restore_mode=unknown
    preferred_volume=
    if restore_valid_volume "$provided_volume"; then
        preferred_volume=$provided_volume
    fi
    home_volume=
    if [ -n "$provided_home" ]; then
        case "$provided_home" in
            /volume*/@apphome/dockpipe-desktop)
                home_volume=${provided_home%/@apphome/dockpipe-desktop}
                restore_valid_volume "$home_volume" || home_volume=
                ;;
        esac
        physical_home=$restore_filesystem_root$provided_home
        if [ -z "$home_volume" ] && [ -d "$physical_home" ] && [ -x "$physical_home" ]; then
            resolved_home=$(CDPATH='' cd "$physical_home" 2>/dev/null && pwd -P) || resolved_home=
            case "$resolved_home" in
                "$restore_filesystem_root"/volume*/@apphome/dockpipe-desktop)
                    logical_home=${resolved_home#"$restore_filesystem_root"}
                    home_volume=${logical_home%/@apphome/dockpipe-desktop}
                    restore_valid_volume "$home_volume" || home_volume=
                    ;;
            esac
        fi
    fi
    if [ -n "$preferred_volume" ] && [ -n "$home_volume" ] && [ "$preferred_volume" != "$home_volume" ]; then
        return 0
    fi
    if [ -z "$preferred_volume" ]; then
        preferred_volume=$home_volume
    fi
    if [ -n "$preferred_volume" ]; then
        restore_inspect_volume "$preferred_volume"
        restore_record_result "$preferred_volume" "$restore_result"
    else
        [ -d "${restore_filesystem_root:-/}" ] && [ -r "${restore_filesystem_root:-/}" ] \
            && [ -x "${restore_filesystem_root:-/}" ] || return 0
        restore_volume_count=0
        for physical_candidate in "$restore_filesystem_root"/volume[1-9]*; do
            candidate_name=${physical_candidate##*/}
            candidate_volume=/$candidate_name
            restore_valid_volume "$candidate_volume" || continue
            restore_volume_count=$((restore_volume_count + 1))
            [ "$restore_volume_count" -le 64 ] || return 0
            restore_inspect_volume "$candidate_volume"
            restore_record_result "$candidate_volume" "$restore_result"
        done
    fi
    if [ -n "$restore_retained_volumes" ]; then
        restore_mode=restore
    elif [ -n "$restore_checked_volumes" ] && [ -z "$restore_unknown_volumes" ]; then
        restore_mode=fresh
    fi
}

restore_render_wizard() {
    # Every substituted value is an enum or a comma-separated list of the
    # validated /volumeN names above. None contains sed metacharacters, quotes,
    # backslashes or private settings. JSON and embedded JavaScript stay intact.
    sed -e "s|@DOCKPIPE_RESTORE_MODE@|$restore_mode|g" \
        -e "s|@DOCKPIPE_RESTORE_VOLUMES@|$restore_retained_volumes|g" \
        -e "s|@DOCKPIPE_CHECKED_VOLUMES@|$restore_checked_volumes|g" \
        -e "s|@DOCKPIPE_UNKNOWN_VOLUMES@|$restore_unknown_volumes|g"
}
