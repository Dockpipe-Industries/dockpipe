# TASK-026 Enterprise Linux Support For RHEL, Rocky Linux, And AlmaLinux

## Goal

Make DockPipe work and remain qualified on selected long-lived enterprise-Linux baselines. Fedora
evidence alone does not establish RHEL-family compatibility.

## Scope

- Select exact supported RHEL-compatible major releases and architectures, with Rocky Linux and
  AlmaLinux as reproducible community qualification hosts and an explicit policy for the RHEL claim.
- Implement and validate RPM installation/upgrade, DNF repositories and dependencies, CLI,
  packages/resolvers/workflows, services, diagnostics, runtimes, and relevant App Server/VM consumers.
- Account for older enterprise kernels, SELinux, systemd, crypto policy, filesystem defaults, and
  long support lifecycles without weakening security or durability contracts.
- Keep each distribution's support row explicit; do not treat binary compatibility as complete
  behavioral qualification.

## Acceptance Criteria

- Every claimed enterprise baseline passes reproducible native or VM evidence for all claimed
  DockPipe surfaces, including SELinux-enforcing operation where applicable.
- Required compatibility fixes are implemented with generic primitives or package-owned behavior in
  the correct layer; unsupported releases fail closed.
- RPM availability is not presented as support until install, operation, and evidence gates pass.
- TASK-025 and canonical support/install documentation reflect the accepted exact baselines.

## First Bounded Slice

Inventory current RPM artifacts, Fedora assumptions, RHEL-family dependencies, kernels, SELinux
interactions, and available test images. Propose exact baselines and evidence; do not install hosts,
add CI, change source, or claim support.
