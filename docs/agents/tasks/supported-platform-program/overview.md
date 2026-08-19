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
