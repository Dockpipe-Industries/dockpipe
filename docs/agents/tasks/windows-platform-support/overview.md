# TASK-032 Windows Dockpipe Platform Support And Qualification

## Goal

Maintain an explicit Dockpipe-wide Windows support contract across installation, CLI, packages,
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

## Qualification Checkpoint

- A disposable Windows 11 amd64 VM passed the installed CLI happy-path case and verified that
  the installed GUI Files editor persisted an exact edit.
- Managed Terminal rejected the inherited Windows `=::` entry. With only that entry omitted,
  the same installed app started PowerShell, returned interactive output, and reported normal
  shell exit. Restoring the entry reproduced the launch error.
- A focused Qt 6.10.3 build on Windows 11 amd64 reproduced the validation error before the fix
  and passed the same regression with the fix. Physical Windows and Windows arm64
  qualification remain open.
- Microsoft Build Tools and the Windows SDK installed from a verified local layout in the
  disposable amd64 VM. The native release gate passed 89 of 95 CTest cases with six skips
  and zero failures, including the native Terminal provider and desktop-window embedding.
  WiX produced the full private 0.6.40 MSI from that build.
- The Windows Execution Explorer fixture now isolates Qt settings. Workspace file listings
  retain entries with denied or locked metadata as unavailable. The native locked-file
  regression and the three dashboard Files cases passed against the revised CLI.
- Remote Files tabs and dashboard previews preserve slash-rooted workspace paths on Windows.
  Both passed the full diagnostic run after the path fix and dashboard fixture log isolation.
- The Windows worker publishes Files grants using its native private-state lock. Native
  tests verified publication and independent lock lifetime. The Files catalog only queries
  Terminal recovery stores on platforms with a Terminal backend.
  The private 0.6.43 MSI worker passed the installed Remote Files journey with a
  Pop!_OS viewer. The disposable Windows 11 x64 run covered exact-revision saves,
  independent grants, stale edits, receipt recovery and fixture cleanup.
- The private 0.6.44 MSI Windows worker passed the installed Remote Git journey
  using owner-selected host-reboot recovery. Native tests verified receipt
  publication before process start and same-boot rejection. An actual Windows
  reboot showed the boot environment GUID stayed unchanged. A follow-up build
  used the advancing kernel boot sequence and reconciled a seeded uncertain Git
  receipt with the real prior sequence. The installed GUI journey with a live
  interrupted mutation remains untested.
- Workflow session startup now runs on the event loop after launch admission. The native
  Windows lifecycle tests verified immediate cancellation without spawning, asynchronous failed
  startup, and a retry from the failure callback.
- The remote-access monitor fixture now waits for the cancelled child to be released. The
  focused native Windows test exited successfully twice after reproducing a CTest timeout.
- The translation catalogs now include the 40 PipeLang editor and language-client messages
  identified by Qt extraction. Linux catalog, launcher translation and settings checks passed.
- Browser tests use an invocation-owned configuration directory. The Git review fixture
  checks text, selection and scrolling so unrelated Qt formatting revisions do not affect it.
- The private 0.6.40 MSI upgraded the disposable 0.6.39 installation and preserved the
  existing workspace marker. The installed CLI and Start Menu GUI each ran a native
  PowerShell workflow. Closing the installed Terminal tab released the observed process
  tree, including the synthetic child.

## First Bounded Slice

Build the Windows support inventory and reconcile existing TASK-011/TASK-013 evidence into proposed
matrix rows. Do not run Windows VMs, install software, add CI, modify source, or expand support claims.
