# Dockpipe GUI + CLI SPK

The SSH-based host-connection candidate is superseded. Native DSM host integration
is being qualified through the separate [read-only native probe](../../qualify/synology-host/README.md)
before host terminal and Docker controls are connected to this GUI. Existing DSM
sign-in must establish the authenticated operator; it does not by itself establish
a host process UID. Host networking and the loopback-only desktop listener remain.

This producer adds the combined DSM edition alongside the lightweight CLI SPK.
It packages a native CLI and a browser desktop project into one manually
installable `dockpipe-desktop_VERSION_synology_x64.spk` or
`dockpipe-desktop_VERSION_synology_arm64.spk`.

DSM 7.2.2+ and Container Manager 20.10.23-1437+ are required on a supported model.
The [Docker Project worker](https://help.synology.com/developer-guide/resource_acquisition/docker-project.html)
prepares the project during installation, starts/stops it with Package Center and
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

The candidate DSM launcher is a module-backed AppInstance/AppWindow, containing
one fitted iframe. It opens the existing Qt stream inside DSM. A package-scoped
`web-config` location proxies only `/webman/3rdparty/dockpipe-desktop/desktop/` to
the fixed loopback desktop listener. Selkies serves that subfolder and keeps
`frame-ancestors 'self'` and same-origin WebSocket checks. Browser TLS uses DSM's
existing HTTPS connection; the package-generated backend certificate stays on
loopback. DSM session cookies are stripped before proxying; Dockpipe still
requires its separate sign-in. No global framing/browser security settings change.
The proxy worker and declared nginx reloads own route install/start/stop/removal.
Direct HTTPS DSM access needs no operator-configured reverse proxy. If DSM already
sits behind another proxy, that hop must forward WebSocket upgrades too; the
package cannot configure an operator-owned frontend. An x64 NAS test reached a
working embedded desktop after enabling WebSocket forwarding on that existing hop.

Port 38139 binds only to loopback in candidate Compose files. Start waits up to
two minutes for HTTP 401 at the prefixed authentication gate, using bounded local
probes without credentials. The port-config resource reserves the service and
disables automatic router forwarding. Direct browser access to that port is no
longer the launcher; use HTTPS DSM. Full hardware lifecycle/focus/resize acceptance
remains separate from the working embedded desktop observation.

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

The historical diagnostic build-on-NAS producer accepts matching release inputs
(the release pipeline uses `prebuild.yml` below):

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

## Prebuilt image candidates

`prebuild.yml` builds the matching architecture's full desktop image in CI from
admitted release DEBs. The image gets version/source labels. Its OCI manifest
and config digests bind these two editions to the same image:

- Standard: the existing SPK filename, a native CLI plus immutable GHCR image
  reference. Container Manager downloads the image; it does not compile it.
- Offline: `_offline.spk`, adding a Docker-loadable archive. The official
  `docker-project.preload-image` worker imports it before creating the project.
  Compose selects a unique tag derived from the full config hash, with `pull_policy: never`. Registry manifest
  digests are deliberately not used for offline lookup because image import need
  not retain them. Config-ID lookup also varies by Docker image-store implementation;
  the portable tag is admitted against the exact config bytes in the archive.

Both keep the same package ID, account, settings and workspace. They are delivery
alternatives, not separately installable applications. Both require an already
installed, compatible Container Manager. Offline describes image installation;
workflows may still need network access depending on their contents.

The candidate uses `build: false`, `force_pull: false` and `force_recreate: false`
worker parameters. Immutable image references let Compose detect image upgrades
without replacing an unchanged container. This does not establish the cause of
any DSM worker failure; native lifecycle acceptance is still required. No
package script invokes Docker, mounts its socket, or requests root. Archive
admission checks byte/member/metadata limits, safe paths, exact config identity,
Linux architecture, labels, unprivileged user and layer references. The capacity
contract permits an 8 GiB expanded image archive and requires the final offline
SPK to fit GitHub's 2 GiB asset limit. The `.image.json` receipt binds the DEBs,
image identities and archive checksum; release checksums also cover this receipt.

CI retains OCI artifacts separately from public package artifacts. Registry writes
happen only in the existing staging/production publication jobs after release
verification. Manual dry runs build images/SPKs without registry writes. Publication
uses `skopeo copy --preserve-digests` and anonymous digest readback before exposing
standard SPKs. The GHCR `dockpipe-desktop` package must have Public visibility;
a new private package fails that gate with setup instructions. Credentials stay
in a temporary registry auth file, never in image metadata or packages.

Reference: [Synology Docker Project worker](https://help.synology.com/developer-guide/resource_acquisition/docker-project.html).
Its preload support is documented; actual import and startup on the target NAS
remain a qualification gate. Local Docker import cannot establish DSM worker behavior.

The CI prebuilder imports the normalized archive and starts the actual desktop
under the runner's non-root identity, with pulling disabled, all capabilities
dropped and networking disabled. It requires an unauthenticated 401, authenticated
HTML with the expected framing policy, a live Qt launcher process, no resolved
password in persisted container environment/logs, and clean shutdown before
packaging. This service proof complements native DSM preload/window acceptance.

## Installer host folder mapping

The install/update wizard uses DSM 7.2.2 custom Vue components for a repeatable
list of up to 16 host folders at `/host/<name>`. Names default to source basenames
until overridden; each row selects read-only (default) or read/write. Names are
unique single components limited to 128 UTF-8 bytes, with a 64 KiB aggregate JSON
budget. Keep reuses the stored list, edit replaces it, and remove-all drops only
the mappings and never deletes host contents. Legacy r10 settings migrate to the
v2 list while preserving `/host/source` on Keep. Updating
restarts the desktop through DSM's existing Docker Project resource lifecycle;
there is no runtime redeploy API or Compose editor added to the app.

`preinst` runs the candidate helper from
DSM's staging directory, recognizing the helper at `bin/dockpipe-dsm-host` or
`package/bin/dockpipe-dsm-host` and rejecting missing or ambiguous candidates.
The native helper binds its edited payload to the executable actually launched.
It validates input and
package-account access, then renders `desktop/compose.yml` from the bundled
`compose.base.yml` and stages `desktop/host-folder.json`. It changes no persistent
package settings in this phase. `postinst` stores the desired selection at
`etc/host-folder.json`, outside the GUI data mount. Both source/prebuilt editions
must carry identical base/live templates; private cached-image qualification
packages must patch both templates to the exact cached image.

DSM documents the staging environment variable and worker timing, but its exact
writable `package/` staging layout and new repeatable wizard controls still require NAS
acceptance. The test package fails rather than editing an old installed project
if staging is unavailable. Existing folder ACLs must already grant the package
account access; no recursive permission changes are performed. Nested mounts
are rejected because older kernels cannot make them recursively read-only; see
[Docker's recursive bind mount behavior](https://docs.docker.com/engine/storage/bind-mounts/#recursive-mounts).


Each wizard row has a native DSM **Browse…** directory chooser. Selection uses
`fullpath` (the physical host path), never the share-relative display `path`.
The picker cannot create folders, excludes remote/tiering selections, and closes
when its wizard component unmounts. Choosing a directory follows the same default
name/override rules as typing a path. Manual path entry remains available if the
vendor picker cannot be opened; no DOM token or credentials are copied into it.


The custom wizard's `getValues()` must return a flat map of string environment
values. Actual DSM `getCustData()` merges the result using `Ext.apply`; an array
becomes a numeric `0` key containing an object, which is not an installer script
environment. The regression fixture checks this aggregation contract, not only
the component's local field values. r11/r12 incorrectly returned an array; the
repair candidate returns the flat map without changing mount or privilege logic.

The form measures DSM's allocated `.mcontentwrapper` body and gives the folder
content one scroll area, rather than sizing it against the browser viewport.
On Next, the candidate wizard applies the selected package-account access through
the authenticated DSM ACL API and verifies it before advancing. Parent rules grant
traversal on that folder only; target rules grant the chosen access with folder/file
inheritance. It preserves existing explicit rules, owner IDs, inheritance settings
and denials, checks for intervening changes before writes, and does not change
ownership. Keep and remove-all do not write ACLs.

No separate Check access, Review permissions or native Save action is required.
The operation is bounded and sequential; failure/cancellation cannot undo writes
already acknowledged by DSM, and automatic rollback could overwrite later edits.
The native installer remains authoritative for actual kernel access. Fresh-install
account timing, automatic ACL writes, mount installation and ACL cleanup across
uninstall still require DSM qualification before general release.

Restore discovery is shared shell code in `assets/scripts/restore-discovery.sh`,
inlined into the generated install wizard. It prefers valid DSM volume/home hints
and otherwise checks at most 64 standard volume roots for this package's FHS
metadata. It reads no private credentials. A successful postinst atomically writes
public `@appconf/dockpipe-desktop/install-state` metadata; known older native-host
public certificate metadata supports retained-state discovery during migration.
Opaque or conflicting state never produces new-credential fields. The wizard's
restore intent and volume are checked against the selected DSM volume before ACL
work and again against the authoritative destination in preinst. Registered
upgrades use a static upgrade-mode component; no dynamic upgrade script is added.
DSM repair omits the upgrade wizard's volume selector. In that case the wizard
sends upgrade intent with an empty volume hint. Preinst accepts this only in DSM's
native UPGRADE lifecycle, validates its authoritative destination, and requires
complete retained settings. Fresh/restore flows and explicit mismatched volume
hints remain blocked; no volume is inferred from the selected host folders.
