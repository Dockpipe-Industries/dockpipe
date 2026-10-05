# TASK-028 Arch Linux Rolling-Release Support

## Goal

Make DockPipe work on Arch Linux with a truthful rolling-release support policy that qualifies
current repository state instead of pretending a monthly installer image is a long-lived baseline.

## Scope

- Define supported architectures, update cadence, evidence freshness, breakage response, and the
  meaning of supported on a rolling distribution.
- Implement and validate pacman package install/upgrade, current toolchains and kernels, CLI,
  packages/resolvers/workflows, runtimes, diagnostics, and relevant workload consumers.
- Audit latest-version drift, filesystem/service defaults, optional dependencies, packaging hooks,
  and scripts that assume Debian/RPM conventions.
- Keep Arch support separate from Arch-derived distributions unless they qualify independently.

## Acceptance Criteria

- The current supported snapshot passes reproducible claimed-surface evidence and has a defined
  maximum evidence age plus requalification trigger.
- `.pkg.tar.zst` availability, rolling runtime compatibility, and workload qualification remain
  distinct matrix claims.
- Breakage or stale evidence automatically removes or downgrades the support claim until refreshed.
- TASK-025 records the exact rolling policy and current accepted evidence.

## First Bounded Slice

Inventory packaging, current toolchain/kernel sensitivities, rolling CI options, and representative
checks. Propose freshness and revocation policy only; do not install Arch, add CI, modify source, or
claim support.
