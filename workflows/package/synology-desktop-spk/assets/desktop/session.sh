#!/bin/sh
# Selkies owns the session and its process group. Keep one Dockpipe window alive;
# closing it reopens it, with a one-second pause to bound a failed launch loop.
set -u
stopping=false
application=
pause=
openbox --config-file /etc/dockpipe/openbox.xml &
window_manager=$!

shutdown() {
    stopping=true
    for child in "$application" "$pause" "$window_manager"; do
        if [ -n "$child" ]; then
            kill "$child" 2>/dev/null || true
        fi
    done
}
trap shutdown HUP INT TERM

while [ "$stopping" = false ]; do
    dockpipe-launcher --start-home &
    application=$!
    if [ "$stopping" = true ]; then
        kill "$application" 2>/dev/null || true
    fi
    wait "$application" || true
    application=
    if [ "$stopping" = true ]; then
        break
    fi
    sleep 1 &
    pause=$!
    wait "$pause" || true
    pause=
done
shutdown
wait "$window_manager" 2>/dev/null || true
