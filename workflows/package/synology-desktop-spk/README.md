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
files at project creation time. Before invoking the upstream init, the entrypoint
requires complete, valid private settings. It passes file references to upstream
Webtop authentication and refuses to start without them. Passwords never enter
Compose, a release archive, image build arguments, or Docker's persisted environment.

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
whose framework-managed paths survive upgrades. Scripts do not delete retained
state on uninstall. The package requests 1 GiB shared memory, exposes only its
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
upgrades and reinstall with retained state. Initial TLS uses the upstream
self-signed certificate; DSM launcher integration does not supply DSM SSO or copy
DSM certificate keys. A trusted certificate/reverse proxy is a separate operator
configuration. Do not advertise native DSM qualification before those tests pass.
