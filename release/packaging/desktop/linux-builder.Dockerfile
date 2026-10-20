# Preserve Ubuntu 22.04's compiler/glibc baseline. amd64 ships its matching Qt;
# arm64 retains distribution Qt and does not qualify the native ForgePipe SDK.
FROM ubuntu:26.04
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        build-essential cmake git dpkg-dev python3-venv patchelf ca-certificates libgl1-mesa-dev \
        libsdl2-dev xvfb xauth \
        qt6-base-dev libqt6opengl6-dev qt6-qpa-plugins \
        qt6-webengine-dev qt6-webengine-dev-tools libqt6webenginecore6-bin \
        libx11-dev libxcomposite-dev libxrender-dev libxres-dev libxcb1-dev \
        libxkbcommon-x11-0 libxcb-cursor0 libxcb-icccm4 libxcb-image0 libxcb-keysyms1 \
        libxcb-render-util0 libxcb-xinerama0 libxcb-randr0 libxcb-shape0 libxcb-sync1 libxcb-xfixes0 \
        libnss3 libnspr4 libasound2 libxdamage1 libxrandr2 libxshmfence1 libxss1 libxtst6 \
        libx11-xcb1 libdrm2 libgbm1 libpulse0 libwayland-client0 libwayland-cursor0 \
        libwayland-egl1 libopengl0 libgtk-3-0 libssl3 \
    && rm -rf /var/lib/apt/lists/*
COPY install-linux-qt.sh /usr/local/bin/install-linux-qt
RUN if [ "$(dpkg --print-architecture)" = amd64 ]; then \
      bash /usr/local/bin/install-linux-qt /opt/Qt; \
    fi
