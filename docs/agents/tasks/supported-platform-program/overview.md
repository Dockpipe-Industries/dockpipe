# TASK-025 Dockpipe Supported-Platform Program And Qualification Matrix

## Goal

Maintain one truthful, versioned definition of where Dockpipe is supported while separate platform
tasks implement and qualify their materially different operating-system and distribution contracts.
Package publication, installation, compilation, normal operation, workload-specific qualification,
and production support are distinct claims.

## Owned Platform Tasks

| Task | Platform boundary |
| --- | --- |
| TASK-024 | Fedora and Debian |
| TASK-026 | RHEL, Rocky Linux, and AlmaLinux enterprise family |
| TASK-027 | Alpine Linux with musl, BusyBox, and apk |
| TASK-028 | Arch Linux rolling release and pacman |
| TASK-029 | Amazon Linux cloud hosts |
| TASK-030 | openSUSE Leap and Tumbleweed |
| TASK-031 | NixOS declarative hosts |
| TASK-032 | Windows |
| TASK-033 | macOS |

Ubuntu/Pop!_OS evidence remains represented in the existing implementation and relevant workload
tasks. A future dedicated task is needed only if the shared matrix identifies unowned compatibility
work beyond maintaining those existing baselines.

## Scope

- Define canonical support terms, maturity levels, release/architecture lifecycle, revocation, and
  evidence freshness.
- Maintain a matrix for artifact availability, install/upgrade, CLI, packages/resolvers/workflows,
  runtimes, diagnostics, security boundaries, and workload-specific consumers.
- Require exact releases or an explicit rolling-release policy; never infer support from package
  format, kernel family, `ID_LIKE`, cross-compilation, or container-only tests.
- Route compatibility fixes to the owning engine, workflow, package, resolver, installer, or docs
  surface and preserve Dockpipe's architecture boundaries.
- Keep public support documentation synchronized only with accepted evidence.

## Acceptance Criteria

- Every advertised platform maps to an open or completed owning task, exact baselines, claimed
  surfaces, evidence, freshness policy, and known exclusions.
- Each platform can advance, defer, or revoke independently without silently changing another.
- The matrix distinguishes native host, VM, container, cross-compile, and workload-specific proof.
- Unsupported or stale combinations fail closed and are not described as supported.
- Windows, macOS, Linux-family, cloud-vendor, rolling, musl, and declarative-host differences remain
  explicit rather than collapsed into lowest-common-denominator behavior.

## First Bounded Slice

Create the canonical matrix schema and inventory current claims/evidence only. Link each row to its
owning task and mark unknowns honestly. Do not implement compatibility fixes, add CI, run live hosts
or VMs, publish artifacts, or promote support claims in this slice.

## Experimental Synology artifacts (2026-11-02)

The user authorized DSM 7+ x64/arm64 CLI SPKs and the full GUI through a browser
desktop container, with NAS testing before any SynoCommunity submission. The
release workflow now stages both SPKs and matching GUI project bundles, requires
them during assembly, and includes them in the download catalog and checksums.
The GUI bundle reuses released Linux DEBs with a pinned multiarchitecture Ubuntu
Webtop base; it does not port Qt to DSM or change engine/package ownership.

Local proof includes cross-built CLI SPKs, extraction and a real x64 host workflow,
packaging/release contract tests, and an x64 container build from checksum-verified
0.6.35 staging DEBs. The container GUI's installation check and a real host workflow
pass. HTTPS rejects missing/incorrect credentials; after user sign-in, the streamed
Qt window, native menu interaction and embedded browser rendering were observed.
GUI-launched workflow jobs, native arm64 execution, DSM install/upgrade/removal,
NAS workload compatibility and hosted release execution remain separate evidence.
The initial CLI/portable-GUI artifacts were subsequently published as staging
`0.6.36-staging.41416259425.1.067d543e3597` from source `067d543e35974f4ce7da46a8be788533dc0b9857`.
All hosted build/assembly/publication jobs passed and four GitHub download
assets matched the published catalog and SHA-256 checksums. No community
submission was made.

The next user-directed slice adds a combined GUI + CLI SPK with a DSM launcher,
credential wizard and Package Center lifecycle through the official Container
Manager Docker Project worker. It retains the CLI-only edition and portable GUI
bundle. This combined installer remains subject to DSM 7.2.1+ hardware acceptance;
container execution and simulated package scripts do not qualify that boundary.
Local combined-package proof passed for archive admission/reproducibility, wizard
and retained-state lifecycle scripts, fail-closed missing/root identities and
authenticated HTTPS. On x64, the packaged session opened Dockpipe directly,
ran a workflow through the GUI and retained working authentication after a
container restart. ARM64 archive production passed; native ARM64 execution and
actual DSM lifecycle acceptance remain pending.
Hardware feedback confirmed CLI installation, but combined 0.6.37-1 failed to
start: DSM's Docker Project worker forced the unprivileged package UID, while
Webtop init required root to configure nginx. The packaging repair uses the
pinned streamer's native unprivileged session/HTTPS/authentication path and
process-local UID mapping. A revision-2 upgrade candidate is being qualified
against that worker constraint before further NAS acceptance. Local repair proof
passed 26 producer/lifecycle/runtime checks, authenticated page/asset/API/WebSocket
checks and a real GUI-launched workflow under an arbitrary unprivileged UID with
capabilities dropped except NET_BIND_SERVICE and no-new-privileges enabled.
See the [combined producer](../../../../workflows/package/synology-desktop-spk/README.md)
and the current
[Synology installation and acceptance guide](../../../install-synology.md),
[SPK producer](../../../../workflows/package/synology/README.md) and
[browser desktop producer](../../../../workflows/package/synology-desktop/README.md).

## 0.6 release implementation progress

The user authorized a cross-platform release pipeline. It now builds native Linux amd64/arm64, macOS Intel/Apple Silicon, and Windows amd64 artifacts plus separate native package stores. Linux formats are DEB/RPM/APK/Arch and tarball; Windows ZIP/MSI; macOS tarball. Signed APT and versioned R2 publication are implemented, with a credential-free dry-run lane. Host-workflow smoke checks run per target; local Linux package generation and APT tests are distinct from the still-pending hosted matrix and downstream distro/hardware qualification. The initial pipeline excluded Flatpak; the 2026-10-08 scope below supersedes that decision.

Local validation on 2026-10-03: Linux amd64 release build produced a checksum-verified 59-package store (42 workflows, 16 resolvers, core), CLI archive, DEB/RPM/APK/Arch packages. Native CLI and all four packaged secret-environment adapters passed outside-checkout smoke tests using fake provider processes. Seven release-tool tests passed, including real GnuPG signing and isolated APT index consumption. The runtime/package Go gate, focused race tests, Go vet, workflow validation, shell lint, and Actions syntax checks passed. Mac/Windows secret helpers cross-built only; hosted native runs remain pending.

A broad Go run failed existing PipeLang generated-compilation containment checks and timed out after ten minutes. The explicitly scoped 0.6 runtime gate excludes the compiler campaign packages; this does not establish PipeLang conformance. Language qualification remains with `tests/containedexec/` for the deferred release. Build outputs are ignored under `bin/.dockpipe`, `src/bin`, and `release/artifacts`; isolated tests/cross-builds are under `/tmp`.

Production setup on 2026-10-03: the signing key saved in **Dockpipe Packages - Production** passed an unattended GnuPG signing check with fingerprint `71B23431AB2FD718629B2696A27C6DAE150CD947`. The release setup workflow copied the two scoped R2 credentials and signing key into GitHub's `release` environment, plus the four public settings. GitHub secret names/update timestamps and public values were read back; required-reviewer protection remains enabled. The Cloudflare dashboard shows `packages.dockpipe.com` active with access enabled. A public object download, hosted native matrix, and live release publication remain unverified; no release was started by setup.

Portability cleanup on 2026-10-03 removed machine-specific defaults from VM provisioning, QEMU build scripts/specification, and package examples. QEMU now requires validated explicit source/build/final directories and passes its selected loader root into the isolated builder. The old machine-specific JSON evidence is preserved byte-for-byte under `docs/research/vm-toolchain-2026-08-07/`, outside embedded package inputs. Store exports omit the builder's private directory, and generated core/workflow/resolver descriptions no longer include absolute source paths. Full VM package tests, focused core/export/embed tests, path-admission tests, shell lint, and fresh VM workflow/resolver compilation passed. The updated QEMU recipe has not been rebuilt or live-qualified; prior hashes do not qualify new paths.


## Flatpak qualification accepted 2026-10-08

The user accepted a launcher-plus-CLI Flatpak for atomic Linux desktops, initially
motivated by Bazzite and Pipeon. Selection follows the running environment: native
Dockpipe uses native stores; Flatpak Dockpipe uses its architecture/runtime store.
No OS heuristic, toggle, automatic native fallback, or bundled Marketplace catalog.

Implemented source: shared process-platform detection, platform-labelled store
exports, catalog/pinned-install checks, a KDE 6.10 desktop builder, offline
package-owned dependency build/runtime-test recipes, and a reusable release CI
workflow. See the canonical [Flatpak build contract](../../../../workflows/build/flatpak/README.md).

Local amd64 proof: six Qt tests, actual Platform launcher/CLI/core-only inventory,
real sandbox workflow, a package with a compiled shared-library dependency, and
native/Flatpak catalog rejection tests passed. Runtime regression and 47 release
packaging tests passed. A local unsigned staging bundle was exported under `/tmp`;
no hosted dispatch, public release, installation, or user data migration occurred.

Host integration was explicitly approved on 2026-10-08. The source now includes
shared Docker/Compose/Buildx socket selection, explicit host bridges, host-compatible
remote service commands, and Pipeon release payloads that carry their container
helpers/context. First-party package profiles and pinned dependency bundles cover
all 61 optional authored artifacts, including the core-owned dotenv resolver. The
offline Platform dependency/preflight pass completed. Pipeon started containers,
MCP endpoints and its browser UI twice from the same isolated project and saved
state. The test used an existing code-server image and no provider credentials.
Fixes include user-owned nested mount targets, a private code-server umask, packaged
provider catalogs, and retry failure propagation. A compiler regression now retains
declared workflow identities when folder names differ, including during pruning.
The final local export contains 62 entries: required core plus 15 resolvers and
46 workflows. Every optional exported archive matches its qualification checksum.

The reusable Flatpak CI job is wired into release assembly, preserving the existing
publication gates and a distinct runtime-specific Marketplace store. Local Docker
and Compose bind-mount checks passed. Hosted qualification and staging publication
provide additional evidence beyond this local proof.

Publication rechecked 2026-10-10: the staging pointer selects
`0.6.3-staging.37959504785.1.4da6948415b9`; its manifest includes the 62-entry Flatpak
store and the combined launcher/CLI bundle, whose package-host URL returns HTTP 200.
The matching hosted Flatpak run succeeded. See the canonical
[publication evidence](../../../../workflows/build/flatpak/README.md#release-integration-and-proof-limits)
and [installation guide](../../../install.md#flatpak-desktop-linux-amd64).

Still open: native Bazzite, Podman/SELinux, arm64, signed/updating Flatpak remote,
production Flatpak publication and authenticated/provider or VM end-to-end parity. Package
availability checks do not prove every workflow on every host. See the canonical
build contract for current test boundaries and host prerequisites.

Synology repair follow-up: hardware upgrade feedback exposed preinst falling back
to wizard inputs after DSM removed the old package links. Revision 3 resolves
persistent home/data paths from DSM's install volume and tests the missing-link,
no-wizard upgrade sequence. NAS upgrade/startup and port cleanup remain pending.

Synology uninstall repair adds an explicit keep/delete saved-data wizard, with
keep as default. Only real uninstall plus explicit deletion purges owned data;
upgrade/default callbacks retain it. Simulated destructive-boundary checks cover
symlink isolation, idempotence and retaining data when the service port is live.
DSM-managed orphan project recovery and on-NAS uninstall cleanup remain unproven.

### Prebuilt Synology image follow-up

The operator selected CI-prebuilt x64/arm64 images, standard digest-pinned SPKs
and offline bundled-image SPKs. Synology documents a `docker-project.preload-image`
resource, preserving the package-account privilege boundary. Candidate producers
and image/archive admission are implemented in a managed worktree. Native x64 DSM
preload imported the exact candidate image without extra package privileges, but
the large offline installation did not reliably complete. A cached-image recovery
installer allowed further testing. One startup failure was traced to a stale
Container Manager profile name; a guarded, backed-up identity repair restored
package startup. Clean offline completion and repeatable upgrade/removal remain
release gates; imported image presence alone does not qualify the installer.
The operator also selected the existing container Qt desktop inside a real DSM
application window. Modern AppWindow and a package-scoped same-origin proxy are
implemented. The user confirmed a working embedded desktop on x64 DSM after
enabling WebSocket forwarding on an existing frontend reverse proxy. The package
automatically installs the internal route for direct HTTPS DSM access; users do
not need to create an additional proxy. Full lifecycle, resize/focus and ARM64
hardware acceptance remain pending. NAS host-terminal access is newly requested;
the current PTY remains container-local and host networking does not change that.

The next unsubmitted candidate adds a generic SSH host connection for the native
NAS terminal, system/process view and read-only Docker inventory. Deployment
assets select that host explicitly and label the local environment GUI container.
NAS diagnostics isolated working DNS but blocked bridge TCP egress; the same
image returned HTTPS 200 under an unprivileged, read-only host-network probe.
The candidate therefore uses host networking with only a loopback stream
listener. Full host-session and new image hardware qualification remain pending;
this does not advance ARM64 or cold offline installer acceptance.

The SSH host-connection candidate was explicitly rejected as the DSM UX. Native
host integration now starts with a separate read-only qualification SPK, using
DSM's documented authenticate.cgi and systemd user-unit contracts. Existing DSM
login authentication is distinct from Linux user identity and terminal authority;
neither per-user execution nor native Docker access is accepted until proven on
hardware. The qualification package does not modify the working desktop install,
request extra capabilities, or expose terminal/Docker controls.

### Native host and installer folder qualification (2026-11-04)

The x64 r21 recovery is now running on DSM and the operator confirmed the selected
folder is accessible at `/host/source`. The installer UI timeout was independently
traced to a reverse-proxy read deadline; it was corrected through DSM's supported
per-rule setting. Native Docker Inspector logs/details/raw inspect are being
qualified with a typed read-only worker query, bounded output and stale-session
cancellation; local proof does not replace live Inspector acceptance.

The SSH approach was replaced by a native reverse worker launched through a
verified, fixed DSM one-shot task as the signed-in ordinary account. The operator
confirmed host terminal, executions/system monitoring, outbound web access and
Docker inventory on x64 DSM. A generic Docker inventory fix disables unintended
filesystem size scans. The embedded cursor mismatch was traced to outer-frame raw
pointer coordinates; using normal frame-relative events passed two live menu
hover checks. These observations do not qualify ARM64 or the complete lifecycle.

The next test SPK adds **Add Host Folder** to the normal install/update wizard,
not a live container redeploy feature. One existing shared-folder path maps to
`/host/source`, with read-only default, explicit read/write, keep, replace and
remove choices. The candidate installer renders its staged Compose before DSM
resource acquisition, then persists the desired mapping for later upgrades.
Folder contents and host ACLs are not modified by setup/removal. Validation uses
the package account, rejects system roots/symlinks/nested mounts, and preserves
the existing container/host identity boundaries. Focused local packaging,
lifecycle and native tests pass. Actual wizard rendering, writable staging layout,
bind mount access and keep/remove/reinstall behavior on DSM remain test gates.


The follow-up installer candidate replaces the one-folder control with DSM
7.2.2's repeatable custom wizard: up to 16 named mappings under `/host`, basename
defaults with editable overrides, independent access modes, and collision checks.
The v2 manifest migrates r10 Keep without renaming `/host/source`. Native and UI
fixtures cover the list/size/name boundaries; actual updated wizard and multiple
mount acceptance remain pending on the NAS.

The folder wizard now includes DSM's native directory picker per row. A read-only
live check opened it, displayed NAS shared folders and verified physical versus
share-display paths, then cancelled without file changes. The adapter uses only
the physical path, retains name overrides and handles cancel/unmount safely.
The updated SPK's complete install/mount acceptance remains pending.

The custom wizard field contract was corrected to return a flat map for DSM's
`getCustData` merge. The folder preflight inspects the package account's access
for each selected path and parent within its share. A subsequent layout fix fits
the scroll area to DSM's allocated body; mouse-wheel access to the bottom controls
was verified in the live wizard. The permission-review adapter proposes scoped
additive rules in DSM's native editor: traversal only on a blocking parent, or
the selected access on the target folder and children. Only the user's native
Save applies changes; the installer does not change ACLs itself. Full packaged
permission-review and mount acceptance on DSM remain pending.

The manual permission-review flow was rejected as unnecessary installer UX. Its
replacement applies the mapped access automatically on the normal Next action
through DSM's ACL API, with scoped parent traversal, preserved existing rules,
pre-write conflict detection and post-write verification. There are no separate
permission buttons/dialogs. The supported wizard getNext false/async transition
contract and ACL read/self-denial-check contracts were verified live without
changing NAS permissions. Automatic writes, fresh-install account timing and
uninstall ACL ownership/cleanup remain explicit hardware acceptance work.

The r17 NAS test passed automatic package-account ACL setup and reached package
installation. Preinst then failed to locate the helper; DSM had already removed
the r9 target, while volume-backed data/home/configuration remained intact. The
retained staging directories contain executable package-owned helpers, readable
as the package user. The follow-up resolves the two bounded extracted layouts
and binds preparation to the actual candidate executable, with real role-launch
tests and retained-data reinstall coverage. The latest failing environment value
was not captured, so full DSM install/mount acceptance remains pending.

The r18 recovery wizard unexpectedly rendered new credentials. Restore tests had
supplied a volume hint and did not cover the actual missing-context case. The
follow-up discovers retained FHS metadata without reading credentials, handles
opaque private homes through validated public state, and binds restore to DSM's
selected volume. Explicit fresh setup refuses retained settings; successful
postinst publishes non-secret restore metadata. Fixtures cover absent hints and
package links, restricted private state, multiple volumes and destination mismatch.
Live recovery-wizard and complete mount acceptance remain pending.
