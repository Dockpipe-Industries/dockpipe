ARG WEBTOP_IMAGE=@WEBTOP_IMAGE@
FROM ${WEBTOP_IMAGE}

USER root
COPY packages/cli.deb /tmp/dockpipe/cli.deb
COPY packages/desktop.deb /tmp/dockpipe/desktop.deb
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends /tmp/dockpipe/cli.deb /tmp/dockpipe/desktop.deb \
    && rm -rf /tmp/dockpipe /var/lib/apt/lists/*

# Keep the upstream Webtop entrypoint, user setup and desktop services.
