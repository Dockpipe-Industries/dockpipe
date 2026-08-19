# TASK-032 Windows DockPipe Platform Support And Qualification

## Goal

Maintain an explicit DockPipe-wide Windows support contract across installation, CLI, packages,
workflows, runtimes, filesystem/process semantics, applications, and diagnostics.

## Scope

- Select supported Windows client/server releases and amd64/arm64 claims with exact shell,
  filesystem, path, permissions, process, service, and container/VM boundaries.
- Inventory and implement compatibility across installer/upgrade, PowerShell and native execution,
  Git/workspaces, CLI, package compilation, workflows, resolvers, runtimes, Pipeon, and diagnostics.
- Consolidate evidence references without absorbing specialized ownership: TASK-011 retains local
  Windows CI VM/guest tooling, TASK-013 retains App Server storage/session evidence, and TASK-023
  retains future host-sandbox enforcement.
- Keep native Windows, WSL, Windows containers, and Windows VMs as distinct claims.

## Acceptance Criteria

- Each advertised Windows release/architecture/runtime combination passes reproducible claimed-surface
  evidence, including path, ACL, process-tree, cancellation, and filesystem semantics.
- Cross-compilation and WSL evidence are never substituted for native Windows proof.
- Specialized task evidence is linked into TASK-025 without duplicating or weakening its contract.

## First Bounded Slice

Build the Windows support inventory and reconcile existing TASK-011/TASK-013 evidence into proposed
matrix rows. Do not run Windows VMs, install software, add CI, modify source, or expand support claims.
