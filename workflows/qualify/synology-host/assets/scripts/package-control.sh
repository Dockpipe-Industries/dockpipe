#!/bin/sh
# Diagnostic package only. DSM owns the native user service and its removal.
set -eu
umask 077

unit=pkguser-dockpipe-host-qualification.service
root=/var/packages/dockpipe-host-qualification
socket=$root/var/run/probe.sock
action=$(basename "$0")
if [ "$action" = start-stop-status ]; then
    action=${1:-}
fi

case "$action" in
    preinst)
        command -v synosystemctl >/dev/null 2>&1
        test -x /usr/syno/synoman/webman/modules/authenticate.cgi
        ;;
    postinst)
        # DSM's FHS var link already exists and is owned by the package user.
        test -d "$root/var"
        test ! -L "$root/var/run"
        mkdir -p "$root/var/run"
        chmod 700 "$root/var/run"
        ;;
    start)
        synosystemctl start "$unit"
        remaining=20
        until "$root/target/bin/host-probe" status --socket "$socket" >/dev/null 2>&1; do
            remaining=$((remaining - 1))
            if [ "$remaining" -eq 0 ]; then
                echo 'Native qualification service did not become ready.' >&2
                exit 1
            fi
            sleep 1
        done
        ;;
    status)
        if "$root/target/bin/host-probe" status --socket "$socket" >/dev/null 2>&1; then
            exit 0
        fi
        exit 3
        ;;
    stop|preuninst)
        synosystemctl stop "$unit"
        ;;
    preupgrade|postupgrade|postuninst)
        ;;
    *)
        echo 'Unsupported qualification lifecycle action.' >&2
        exit 1
        ;;
esac
