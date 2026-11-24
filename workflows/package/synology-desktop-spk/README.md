# Dockpipe GUI + CLI SPK

This producer adds the combined DSM edition alongside the lightweight CLI SPK.
It packages a native CLI and a browser desktop project into one manually
installable `dockpipe-desktop_VERSION_synology_x64.spk` or
`dockpipe-desktop_VERSION_synology_arm64.spk`.

DSM 7.2.1+ and Container Manager 20.10.23-1437+ are required on a supported model.
The [Docker Project worker](https://help.synology.com/developer-guide/resource_acquisition/docker-project.html)
builds the project during installation, starts/stops it with Package Center and
removes it on uninstall. No package script runs Docker, accesses its socket or
requests an elevated identity. The full minimum dependency version follows the
first archived stable Container Manager build above the worker's documented
build-1432 floor; a bare `1432` is not a product version.

The install wizard collects a separate Dockpipe username and confirmed password.
Passwords must contain 16–128 printable ASCII characters. Preinst validates inputs
without changing the system. Postinst atomically stores credentials and the
package account's numeric UID/GID in its private home, using a 0700 directory and
0600 files. Upgrade/reinstall retains existing sign-in and workspace data; the
reinstall wizard explains that rather than asking for unused new credentials.

Resource preparation precedes postinst. Compose therefore references the existing
DSM package home directory, without requiring password interpolation or secret
files at project creation time. The entrypoint requires complete, valid private
settings and the actual unprivileged UID/GID must match the package account.
DSM's Docker Project worker adds that identity to its generated Compose file,
overriding the image's `USER`. The first combined SPK (0.6.37-1) failed this hardware
boundary because LinuxServer init expected root to configure nginx.

The repaired image calls the pinned Selkies session launcher directly, using its
native HTTPS and authentication instead of the root-only init/nginx path. A
process-local NSS account lets the arbitrary DSM UID run the display, audio and
Qt application without changing system accounts. Runtime files stay under private
package data; credentials are loaded from private files only into the server's
runtime environment. Passwords never enter argv, Compose, a release archive,
image build arguments, or Docker's persisted environment. The streamer's public
health/status endpoints expose liveness/mode metadata; desktop assets, control APIs
and WebSocket connections require authentication.

The DSM launcher opens HTTPS on port 38139. Package start waits up to two minutes
for the authentication gate, using bounded loopback probes. Status reports stopped
when that gate is unavailable. The local readiness probe accepts only HTTP 401;
it does not send credentials. Container Manager owns actual project start/stop.
The DSM port-config resource registers the named service for firewall management,
with automatic router port forwarding disabled.

The desktop session launches Dockpipe directly under a minimal window manager,
maximizes normal application windows and retains normal dialogs. It does not
present the XFCE applications panel or require users to open a terminal. The app
and CLI remain their existing implementations; no GUI/runtime boundaries change.

GUI state is bound to DSM's persistent package `var`; private settings are in
`home`. Both follow the [DSM FHS](https://help.synology.com/developer-guide/integrate_dsm/fhs.html),
whose framework-managed paths survive upgrades. Uninstall retains that state by
default; the explicit full-removal choice deletes it. The package requests 1 GiB shared memory, exposes only its
authenticated HTTPS port and mounts no arbitrary NAS shares or Docker socket.
It is one shared desktop per installation, not a separate session for every DSM
user. DSM administrator launcher visibility and Dockpipe sign-in are separate.

Build with matching release inputs:

```sh
python3 workflows/shared/launch.py workflows/package/synology-desktop-spk/config.yml -- \
  0.6.36 amd64 --binary /absolute/native/dockpipe \
  --cli-deb /absolute/dockpipe_0.6.36_amd64.deb \
  --desktop-deb /absolute/dockpipe-desktop_0.6.36_amd64.deb --output /tmp/synology
```

The build uses shared ELF/DEB admission, existing product icons and a pinned
multiarchitecture Webtop base. Archives are reproducible and atomically replaced;
DSM image installation downloads the base and current Ubuntu dependencies, so the
resulting image is not claimed bit reproducible. Native CPU archive, lifecycle and
container checks are distinct from actual DSM acceptance.

NAS acceptance must cover first install, installer validation, dependency handling,
build progress, Open/icon navigation, authenticated GUI, package start/stop/boot,
upgrades and reinstall with retained state. Initial TLS uses a package-private
self-signed certificate; DSM launcher integration does not supply DSM SSO or copy
DSM certificate keys. A trusted certificate/reverse proxy is a separate operator
configuration. Do not advertise native DSM qualification before those tests pass.

For a packaging-only NAS repair candidate, `--package-revision 2` produces DSM
version `VERSION-2` from the same matching product DEBs, with `_r2.spk` in the
filename. The normal release default remains revision 1. Install that candidate
as an upgrade to retain settings; do not overwrite an existing published asset.

Non-root qualification must reproduce an arbitrary package UID/GID, no added
privileges, protected assets/APIs/WebSocket handshakes, real GUI workflow execution
and clean stop/restart. A root-started local demo does not qualify the DSM worker.

Upgrade scripts and the restore wizard resolve persistent volume storage through
`SYNOPKG_PKGDEST_VOL`, following DSM FHS. The `/var/packages` links may be absent
between removal of the old version and new preinst. An upgrade with missing saved
state fails with an explicit recovery error rather than asking for unavailable
installation-wizard fields. Revision 3 includes this repair; revision 2 only fixed
the container UID startup issue. Actual DSM port cleanup remains to be qualified.

Uninstall now presents a saved-data choice, defaulting to keep. Both choices rely
on DSM's project worker to remove runtime resources. Only explicit `delete` with
`SYNOPKG_PKG_STATUS=UNINSTALL` removes package-owned home/data contents after the
worker runs; upgrade callbacks and absent choices retain everything. Removal
refuses unexpected/symbolic-link roots or a still-active desktop port, follows no
symlinks, and stays on each root filesystem device. DSM owns the empty FHS root folders.
The application asks for no Docker socket or elevated package identity to do this.
Native DSM wizard rendering, managed-project removal and full removal still need
hardware acceptance. An orphan project must be recovered through DSM's control
plane, not by deleting Container Manager's internal database.

When reusing retained settings, postinst refreshes the recorded UID/GID to the
current unprivileged package account without changing credentials. This supports
a reinstall where DSM assigns a new identity; runtime still checks for an exact
identity match. This behavior also requires actual NAS reinstall acceptance.
