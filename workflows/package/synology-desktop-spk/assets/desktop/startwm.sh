#!/bin/sh
# Use the upstream display/streaming services with a Dockpipe-only session.
exec dbus-launch --exit-with-session /usr/local/bin/dockpipe-desktop-session
