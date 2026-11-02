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
