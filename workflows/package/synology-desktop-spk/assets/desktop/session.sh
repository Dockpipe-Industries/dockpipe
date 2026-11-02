#!/bin/sh
# S6 owns this session. Forward shutdown to both children and retain the app's exit.
set -u
openbox --config-file /etc/dockpipe/openbox.xml &
window_manager=$!
dockpipe-launcher --start-home &
application=$!

shutdown() {
    kill "$application" "$window_manager" 2>/dev/null || true
}
trap shutdown HUP INT TERM
wait "$application"
result=$?
shutdown
wait "$window_manager" 2>/dev/null || true
exit "$result"
