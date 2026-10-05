# TASK-030 OpenSUSE Leap And Tumbleweed Support

## Goal

Make DockPipe work on openSUSE with separate stable Leap and rolling Tumbleweed contracts rather
than treating SUSE-family hosts as generic RPM systems.

## Scope

- Select exact Leap releases/architectures and define a distinct Tumbleweed snapshot/freshness policy.
- Implement and validate zypper/RPM install and upgrade, CLI, packages/resolvers/workflows, services,
  Btrfs/snapshot and filesystem assumptions, AppArmor, diagnostics, runtimes, and workload consumers.
- Keep SUSE Linux Enterprise claims out of scope until separately licensed and qualified evidence is
  available; openSUSE evidence alone does not automatically establish SLE support.

## Acceptance Criteria

- Leap and Tumbleweed can advance or revoke independently and pass reproducible claimed-surface proof.
- AppArmor, Btrfs, service, packaging, and rolling-versus-stable differences are tested explicitly.
- TASK-025 and canonical docs identify exact accepted releases or snapshots and exclusions.

## First Bounded Slice

Inventory RPM assumptions, zypper packaging, AppArmor/Btrfs interactions, available images, and
stable-versus-rolling evidence needs. Do not install, add CI, modify source, or claim support.
