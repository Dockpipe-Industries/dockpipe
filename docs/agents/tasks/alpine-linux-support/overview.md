# TASK-027 Alpine Linux Musl And BusyBox Support

## Goal

Make DockPipe work and remain qualified on selected Alpine Linux baselines as a distinct musl- and
BusyBox-based platform, not merely as an `.apk` publication target or generic Linux container.

## Scope

- Select exact stable Alpine releases and architectures plus an explicit upgrade cadence.
- Implement and validate apk install/upgrade, musl compatibility, BusyBox versus GNU-tool behavior,
  shell/process/filesystem assumptions, CLI, package compilation, workflows, diagnostics, and
  appropriate container/native-host runtimes.
- Audit CGO, dynamic linking, resolver binaries, scripts, system services, certificates, user/group
  tools, and workload-specific predicates that assume glibc or systemd.
- Separate container qualification from native host or VM support.

## Acceptance Criteria

- All claimed Alpine surfaces pass on pinned stable baselines with reproducible evidence.
- No GNU, glibc, bash, or systemd dependency is ambient or undocumented; required dependencies are
  declared or behavior is implemented portably.
- Unsupported Alpine releases and unsupported workload contracts fail closed.
- `.apk` publication becomes a support claim only after TASK-025 records accepted evidence.

## First Bounded Slice

Inventory the existing `.apk`, scripts, linked binaries, shell/tool assumptions, container images,
and native-host gaps. Select proposed baselines and checks only; do not install, add CI, modify
compatibility code, or claim support.
