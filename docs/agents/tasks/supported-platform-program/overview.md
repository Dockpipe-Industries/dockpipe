# TASK-025 DockPipe Supported-Platform Program And Qualification Matrix

## Goal

Maintain one truthful, versioned definition of where DockPipe is supported while separate platform
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
  surface and preserve DockPipe's architecture boundaries.
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

## 0.6 release implementation progress

The user authorized a cross-platform release pipeline. It now builds native Linux amd64/arm64, macOS Intel/Apple Silicon, and Windows amd64 artifacts plus separate native package stores. Linux formats are DEB/RPM/APK/Arch and tarball; Windows ZIP/MSI; macOS tarball. Signed APT and versioned R2 publication are implemented, with a credential-free dry-run lane. Host-workflow smoke checks run per target; local Linux package generation and APT tests are distinct from the still-pending hosted matrix and downstream distro/hardware qualification. Flatpak is excluded for this host CLI.

Local validation on 2026-10-03: Linux amd64 release build produced a checksum-verified 59-package store (42 workflows, 16 resolvers, core), CLI archive, DEB/RPM/APK/Arch packages. Native CLI and all four packaged secret-environment adapters passed outside-checkout smoke tests using fake provider processes. Seven release-tool tests passed, including real GnuPG signing and isolated APT index consumption. The runtime/package Go gate, focused race tests, Go vet, workflow validation, shell lint, and Actions syntax checks passed. Mac/Windows secret helpers cross-built only; hosted native runs remain pending.

A broad Go run failed existing PipeLang generated-compilation containment checks and timed out after ten minutes. The explicitly scoped 0.6 runtime gate excludes the compiler campaign packages; this does not establish PipeLang conformance. Language qualification remains with `tests/containedexec/` for the deferred release. Build outputs are ignored under `bin/.dockpipe`, `src/bin`, and `release/artifacts`; isolated tests/cross-builds are under `/tmp`.

Production setup on 2026-10-03: the signing key saved in **Dockpipe Packages - Production** passed an unattended GnuPG signing check with fingerprint `71B23431AB2FD718629B2696A27C6DAE150CD947`. The release setup workflow copied the two scoped R2 credentials and signing key into GitHub's `release` environment, plus the four public settings. GitHub secret names/update timestamps and public values were read back; required-reviewer protection remains enabled. The Cloudflare dashboard shows `packages.dockpipe.com` active with access enabled. A public object download, hosted native matrix, and live release publication remain unverified; no release was started by setup.

Portability cleanup on 2026-10-03 removed machine-specific defaults from VM provisioning, QEMU build scripts/specification, and package examples. QEMU now requires validated explicit source/build/final directories and passes its selected loader root into the isolated builder. The old machine-specific JSON evidence is preserved byte-for-byte under `docs/research/vm-toolchain-2026-08-07/`, outside embedded package inputs. Store exports omit the builder's private directory, and generated core/workflow/resolver descriptions no longer include absolute source paths. Full VM package tests, focused core/export/embed tests, path-admission tests, shell lint, and fresh VM workflow/resolver compilation passed. The updated QEMU recipe has not been rebuilt or live-qualified; prior hashes do not qualify new paths.
