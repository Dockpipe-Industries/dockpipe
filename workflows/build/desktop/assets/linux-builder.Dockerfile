# amd64 uses Ubuntu 22.04 / glibc 2.35; arm64 uses Ubuntu 24.04 / glibc 2.39.
ARG UBUNTU_VERSION=22.04
FROM ubuntu:${UBUNTU_VERSION}
RUN apt-get update \
    && audio_package=libasound2 \
    && if [ "$(dpkg --print-architecture)" = arm64 ]; then audio_package=libasound2t64; fi \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        build-essential cmake git dpkg-dev python3-venv patchelf ca-certificates libgl1-mesa-dev \
        libsdl2-dev xvfb xauth \
        libx11-dev libxcomposite-dev libxrender-dev libxres-dev libxcb1-dev \
        libxkbcommon-x11-0 libxcb-cursor0 libxcb-icccm4 libxcb-image0 libxcb-keysyms1 \
        libxcb-render-util0 libxcb-xinerama0 libxcb-randr0 libxcb-shape0 libxcb-sync1 libxcb-xfixes0 \
        libnss3 libnspr4 "$audio_package" libopus0 libminizip1 libwebpmux3 libwebpdemux2 libsnappy1v5 \
        libxdamage1 libxrandr2 libxshmfence1 libxss1 libxtst6 \
        libx11-xcb1 libdrm2 libgbm1 libpulse0 libwayland-client0 libwayland-cursor0 \
        libwayland-egl1 libopengl0 libgtk-3-0 libssl3 \
    && rm -rf /var/lib/apt/lists/*
COPY install-linux-qt.sh /usr/local/bin/install-linux-qt
RUN bash /usr/local/bin/install-linux-qt /opt/Qt
