# TASK-024 Fedora And Debian DockPipe Support And Qualification Parity

## Goal

Make DockPipe work as a supported platform on Fedora and Debian, then establish explicit,
evidence-backed qualification alongside the currently qualified Pop!_OS host and Ubuntu 24.04 guest
paths. This task owns both the compatibility implementation and its proof. Treat release packaging,
successful installation, normal DockPipe operation, and workload-specific host qualification as
separate claims that must each be proven before the corresponding distribution is advertised as
supported.

## Current State

- DockPipe publishes `.deb` and `.rpm` release packages, and the install documentation includes
  Debian-family and Fedora/RHEL-family installation paths.
- The package-owned SQLite durability qualification currently accepts valid Pop!_OS identities and
  exact Ubuntu `VERSION_ID=24.04`; Debian, Fedora, and unsupported Ubuntu releases fail closed.
- Focused offline tests cover the current distribution predicate and preserve the corrected direct
  and nested whole-device ext4 mount contract.
- No repository-level Fedora or Debian qualification matrix currently proves parity across install,
  CLI, package/workflow execution, app-server consumers, or VM durability evidence.

Package availability is not by itself a platform qualification claim.

## Scope

- Select and document the exact Fedora and Debian release/architecture baselines to qualify.
- Inventory and implement the engine, CLI, package, resolver, workflow, installer, and diagnostic
  changes required for normal DockPipe use on those baselines. Route each change through its focused
  architecture guidance and keep distribution-specific behavior out of generic engine code unless
  a genuinely general platform primitive is required.
- Define a versioned Linux distribution support matrix covering:
  - release artifact installation and upgrade;
  - DockPipe CLI initialization, package compilation, workflow execution, and diagnostics;
  - package-owned DorkPipe/App Server consumers that enforce host identity or filesystem contracts;
  - VM and native-host evidence where a workload makes platform-specific durability or isolation
    claims.
- Replace workload-local distribution allowlists only when the selected distro satisfies that
  workload's complete kernel, filesystem, mount, virtualization, toolchain, and security contract.
- Add deterministic positive and fail-closed negative tests for parsed `os-release` identities,
  including the exact supported releases and nearby unsupported releases.
- Add CI or maintained qualification workflows only after their runtime, cost, image provenance,
  and evidence-retention boundaries are reviewed.
- Update canonical installation/support documentation when evidence justifies a support claim.

The task is not complete if it only produces a matrix or identifies incompatibilities. Every
accepted Fedora and Debian baseline must either work across the claimed DockPipe surfaces or retain
an explicit unresolved blocker without being advertised as supported.

## Non-Goals

- Treating every Debian-derived or RHEL-compatible distribution as supported through `ID_LIKE`.
- Weakening ext4, whole-device mount, kernel, virtualization, sandbox, or durability predicates to
  obtain nominal distribution parity.
- Making Fedora or Debian qualification a prerequisite for unrelated TASK-013 Codex App Server
  work that runs within an already qualified platform contract.
- Running live VM, Gate, cloud, promotion, publication, or destructive tests without a separately
  approved bounded slice.
- Claiming support from parser fixtures alone.

## Required Decisions

1. Choose exact initial releases, architectures, and lifecycle policy. Prefer one maintained stable
   Fedora release and one maintained Debian stable release rather than floating aliases.
2. Decide which claims require native-host evidence, VM evidence, container evidence, or all three.
3. Define whether the SQLite durability/App Server qualification is required on each distribution
   and, if so, preserve the exact filesystem and mount contract rather than testing only identity.
4. Select CI images or builders with pinned provenance and a documented refresh cadence.
5. Define promotion criteria and the evidence needed to add each distro/release to public support
   documentation.

## Acceptance Criteria

- A canonical support matrix names exact Fedora and Debian releases and architectures, distinguishes
  packaging from qualified operation, and identifies unsupported combinations explicitly.
- All compatibility gaps on the claimed DockPipe surfaces are implemented and validated, with
  package-specific behavior kept package-owned and generic engine changes limited to general
  cross-platform primitives.
- Install/upgrade, CLI, package compilation, representative workflow, and diagnostic checks pass on
  every claimed baseline with reproducible commands and retained public evidence.
- Every workload-specific allowlist change has positive tests for exact supported identities and
  negative tests for missing fields, unsupported distributions, and unsupported releases.
- Where SQLite durability or App Server storage evidence is claimed, the existing direct/nested
  whole-device ext4 mount contract and all other platform predicates pass unchanged or are revised
  only through an independently reviewed contract decision.
- Failures in distribution detection, prerequisites, or evidence collection remain fail closed and
  do not fall back silently to a less-qualified runtime.
- Canonical docs, package metadata, CI/qualification workflows, and focused agent routing stay in
  sync with the final support claims.
- Fedora and Debian can be enabled, deferred, or revoked independently without changing the existing
  Pop!_OS and Ubuntu 24.04 qualification behavior.

## Dependencies And Adjacency

- TASK-025 owns the shared platform-support vocabulary, matrix, and lifecycle. TASK-024 owns the
  Fedora and Debian implementation and evidence rows within that program.
- TASK-013 owns Codex App Server adapter behavior; TASK-024 owns additional Linux-distribution
  qualification. TASK-013 may consume proven results but need not wait for them when its selected
  host/guest contract is already qualified.
- TASK-009 owns deterministic sandbox toolchain discovery and provenance where that affects the
  cross-distribution matrix.
- TASK-010 owns declarative dependency installation UX; TASK-024 supplies Fedora/Debian platform
  certification evidence rather than redesigning dependency declarations.
- TASK-023 owns the future host-sandbox runtime and must qualify its own distribution-specific
  enforcement separately before consuming broad Linux parity claims.

## First Bounded Slice

Produce the exact Fedora/Debian qualification matrix and evidence plan offline. Inventory existing
release packages, installation docs, CI coverage, distribution predicates, filesystem assumptions,
and app-server/VM consumers. Select proposed exact releases and architectures, identify the smallest
representative checks, and record unresolved decisions. Do not change allowlists, run live VMs or
Gates, install operating systems, add CI jobs, publish artifacts, or claim support in this slice.
