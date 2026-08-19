# TASK-029 Amazon Linux Cloud-Host Support

## Goal

Make DockPipe work and remain qualified on selected supported Amazon Linux releases for AWS-hosted
development, build, orchestration, and server workloads.

## Scope

- Select exact Amazon Linux releases and x86_64/arm64 claims with a lifecycle aligned to AWS support.
- Implement and validate RPM/DNF install, minimal and standard images, cloud-init, systemd, EC2
  metadata isolation, CLI, packages/resolvers/workflows, diagnostics, and applicable runtimes.
- Test Graviton separately from x86_64 and keep cloud-host proof separate from generic Fedora or
  enterprise-Linux evidence.
- Prevent ambient instance roles, metadata credentials, or AWS sockets from becoming implicit
  DockPipe worker authority.

## Acceptance Criteria

- Every claimed Amazon Linux image/architecture passes reproducible host evidence for its claimed
  surfaces without relying on ambient cloud credentials.
- Cloud identity, networking, metadata, storage, and architecture differences remain explicit.
- Unsupported images fail closed and TASK-025 records the exact accepted baselines.

## First Bounded Slice

Inventory current RPM compatibility, official images, architecture gaps, cloud credential exposure,
and offline-versus-AWS evidence requirements. Propose baselines only; do not launch EC2, incur cost,
install software, add CI, modify source, or claim support.
