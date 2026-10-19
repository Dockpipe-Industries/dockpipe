# Build against the oldest supported Ubuntu Qt/glibc ABI on each native architecture.
FROM ubuntu:22.04
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        build-essential cmake dpkg-dev qt6-base-dev libqt6opengl6-dev qt6-qpa-plugins libgl1-mesa-dev \
        qt6-webengine-dev qt6-webengine-dev-tools libqt6webenginecore6-bin \
        libx11-dev libxcomposite-dev libxrender-dev libxres-dev libxcb1-dev \
    && rm -rf /var/lib/apt/lists/*
