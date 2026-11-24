#!/bin/sh
# The existing unprivileged host qualification package is never changed here.
set -eu
umask 077
unit=pkguser-dockpipe-host-identity-qualification.service
root=/var/packages/dockpipe-host-identity-qualification
action=$(basename "$0")
if [ "$action" = start-stop-status ]; then
    action=${1:-}
fi

ready() {
    code=$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 2 --noproxy '*' --proto '=https' --cacert "$root/var/endpoint-public/ca.pem" https://127.0.0.1:38141/health 2>/dev/null) || return 1
    [ "$code" = 204 ]
}

case "$action" in
    preinst)
        command -v synosystemctl >/dev/null 2>&1
        command -v curl >/dev/null 2>&1
        test -x /usr/syno/synoman/webman/modules/authenticate.cgi
        ;;
    postinst)
        # Explicit directory permissions avoid depending on installer umask.
        chmod 755 "$root/target" "$root/target/bin" "$root/target/ui" "$root/target/ui/images"
        chmod 755 "$root/var"
        "$root/target/bin/identity-endpoint-init"
        ;;
    start)
        synosystemctl start "$unit"
        remaining=20
        until ready; do
            remaining=$((remaining - 1))
            if [ "$remaining" -eq 0 ]; then
                echo 'Identity qualification capability admission did not become ready.' >&2
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
    preuninst|preupgrade|postupgrade|postuninst)
        ;;
    *)
        echo 'Unsupported identity qualification lifecycle action.' >&2
        exit 1
        ;;
esac
