#!/bin/sh
# DSM owns the user service and route; the app owns its session-bound task.
set -eu
umask 077
unit=pkguser-dockpipe-host-task-qualification.service
root=/var/packages/dockpipe-host-task-qualification
socket=$root/var/run/task.sock
action=$(basename "$0")
if [ "$action" = start-stop-status ]; then
    action=${1:-}
fi

ready() {
    test ! -L "$socket" || return 1
    test -S "$socket" || return 1
    code=$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 2 --noproxy '*' --proto '=https' --cacert "$root/var/endpoint-public/ca.pem" https://127.0.0.1:38142/health 2>/dev/null) || return 1
    [ "$code" = 204 ]
}

case "$action" in
    preinst)
        command -v synosystemctl >/dev/null 2>&1
        command -v curl >/dev/null 2>&1
        test -x /usr/syno/synoman/webman/modules/authenticate.cgi
        ;;
    postinst|postupgrade)
        chmod 755 "$root/target" "$root/target/bin" "$root/target/ui" "$root/target/ui/images"
        test -d "$root/var"
        chmod 755 "$root/var"
        test ! -L "$root/var/run"
        mkdir -p "$root/var/run"
        chmod 755 "$root/var/run"
        "$root/target/bin/task-host" endpoint-init
        ;;
    start)
        synosystemctl start "$unit"
        remaining=20
        until ready; do
            remaining=$((remaining - 1))
            if [ "$remaining" -eq 0 ]; then
                echo 'Task qualification service did not become ready.' >&2
                exit 1
            fi
            sleep 1
        done
        ;;
    status)
        if ready; then
            exit 0
        fi
        exit 3
        ;;
    stop)
        synosystemctl stop "$unit"
        ;;
    preuninst|preupgrade|postuninst)
        # DSM has already released its service worker here. No second stop and
        # no TaskScheduler deletion without the user's active DSM session.
        ;;
    *)
        echo 'Unsupported task qualification lifecycle action.' >&2
        exit 1
        ;;
esac
