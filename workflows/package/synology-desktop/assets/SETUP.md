# Dockpipe full desktop on Synology

This bundle runs the existing full Dockpipe Linux GUI inside a Webtop Ubuntu XFCE
container. The browser streams that desktop. It is separate from the host CLI SPK:
the container has its own Dockpipe installation, processes and persistent state.
It does not automatically control the SPK or the NAS Docker daemon.

Use a Synology NAS with Container Manager, the matching CPU architecture, and
enough memory for a full desktop and Qt WebEngine. Container Manager availability
depends on the NAS model. The pinned Webtop image must provide Ubuntu 24.04 or a
compatible newer userspace for the ARM64 desktop's glibc 2.39 requirement; use the
same qualified Ubuntu family on amd64. Do not substitute an Alpine image.

1. Extract the bundle into a dedicated directory on the NAS. Retain its files.
2. Copy `.env.example` to `.env` and restrict it to the operator (for example,
   `chmod 600 .env`). Set a unique `CUSTOM_USER`, strong `PASSWORD`, and the numeric
   `PUID`/`PGID` for the NAS account that owns the `config` directory. These values
   must be nonempty or Compose refuses to start. Keep `.env` private: Docker
   administrators can inspect container environment variables.
3. Build and start using `docker compose up --build -d` in this directory, or
   import its compose file as a Container Manager project. Building downloads the
   pinned base and Ubuntu dependencies. No elevated desktop mode is required.
4. The default listener is NAS loopback, HTTPS port 3001. Reach it through an
   authenticated HTTPS reverse proxy or an SSH tunnel, for example
   `ssh -L 3001:127.0.0.1:3001 operator@nas`, then open `https://localhost:3001`.
   Webtop supplies its own HTTPS certificate; trust it only after checking your
   intended endpoint. A proxy must support WebSockets. For explicit LAN access,
   set `BIND_ADDRESS` to the NAS LAN address and restrict access with the NAS
   firewall. Avoid forwarding the desktop port directly to the Internet.
5. Sign in with the configured credentials. Launch Dockpipe from the desktop
   application menu, or open a terminal and run `dockpipe-launcher`. Use
   `dockpipe --version` to verify the bundled CLI version.

`config/` persists the desktop home and Dockpipe state. Back it up privately.
Stop with `docker compose down`; it preserves `config/`. For upgrades, retain
`config/` and `.env`, replace the release files with a matching new bundle, and
rebuild. Test an upgrade with a copy of your state first. The bundle does not
mount arbitrary NAS files, use privileged mode, or expose the Docker socket.
Add data mounts only after explicitly selecting the intended NAS directories.

The archive is reproducible for identical DEBs, base-image reference and scripts.
The resulting image build is not bit reproducible: apt downloads current Ubuntu
dependencies. The image pin records the base identity, not DSM/GPU compatibility
or end-to-end qualification. Verify browser sign-in, GUI launch, workflow
execution, state persistence, restart and upgrade on each supported NAS target.
