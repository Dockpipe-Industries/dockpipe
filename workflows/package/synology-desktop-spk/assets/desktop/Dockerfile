FROM @WEBTOP_IMAGE@

USER root
COPY packages/cli.deb /tmp/dockpipe/cli.deb
COPY packages/desktop.deb /tmp/dockpipe/desktop.deb
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends libnss-wrapper /tmp/dockpipe/cli.deb /tmp/dockpipe/desktop.deb \
    && rm -rf /tmp/dockpipe /var/lib/apt/lists/*
COPY entrypoint.sh /usr/local/bin/dockpipe-desktop-start
COPY desktop.py /usr/local/lib/dockpipe-desktop/desktop.py
COPY session.sh /usr/local/bin/dockpipe-desktop-session
COPY openbox.xml /etc/dockpipe/openbox.xml
RUN chmod 755 /usr/local/bin/dockpipe-desktop-start /usr/local/bin/dockpipe-desktop-session
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/dockpipe-desktop-start"]
