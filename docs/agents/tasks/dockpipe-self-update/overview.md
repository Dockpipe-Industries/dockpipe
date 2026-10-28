# TASK-054 Coordinated Dockpipe self-updates

## Status, priority and dependencies

**P1; proposed, decision ready, unclaimed.** Added 2026-10-27 to
[TASK-049's security and reliability initiative](../dependency-lifecycle/overview.md).
This plans supported automatic and manual updates of Dockpipe itself, including
Windows. It does not change the current read-only lifecycle implementation.
P0 applies only to a verified immediately exploitable installed release needing
remediation, rather than the general self-update framework.

Depends on TASK-049 lifecycle/plan contracts, TASK-010 trusted installer and pin
ownership, TASK-036/039 OS authorization and remote service boundaries, and
TASK-051 advisory evidence. TASK-052 owns failure/platform qualification;
TASK-025/032/033 retain their existing platform and distribution responsibilities.
No second scheduler, resolver registry, installer framework or identity system.

## Current architecture findings and problem

| Existing area | Evidence and resulting gap |
| --- | --- |
| `workflows/package/windows-msi/README.md`, `assets/dockpipe.wxs` | WiX MSI supports per-user installation, required CLI and optional GUI payload/features, stable upgrade identity and downgrade rejection. Portable ZIP is a separate installation owner. These are packaging/upgrade primitives, not an automatic update coordinator. |
| `docs/install.md` | Documents native Windows MSI/ZIP, macOS and Linux installation paths. Dockpipe is not currently listed in the default winget catalog; do not assume winget or Microsoft Store delivery exists. |
| `src/lib/application/internal/remotecmd/service{,_definition,_storage}.go` | Explicit service replacement binds executable path/digest and retains interrupted activation state. A new installed executable does not by itself prove a running broker was replaced. Existing service registration/activation is Linux/macOS-specific; Windows broker/service and native OS-auth support remain prerequisite gaps. |
| `src/dockpipe-app/src/catalog/infrastructure/OperatorDockpipeBinary.cpp` | GUI operator operations select a trusted CLI independently of project binaries. An app update must preserve that selection and verify the selected CLI is compatible. |
| `workflows/package/{linux-cli,macos,windows-msi}/`, `docs/runtime/dependency-lifecycle.md` | Existing packaging and lifecycle contracts are reuse points. Current lifecycle commands only inventory, assess and emit non-executable plans; no self-update operation or background updater is implemented. |

Updating only the downloaded CLI, GUI files or service registration can leave
older processes active, incompatible protocols/SDKs loaded, or a broker pointing
at obsolete bytes. Success must describe the coherent installed and running
component set, including intentional CLI-only installations.

## Proposed design and ownership

1. Add a Dockpipe distribution provider to the shared lifecycle contracts. Core
   owns policy, immutable plans, serialization, audit and recovery orchestration;
   platform/distribution adapters own detection, verified artifact preparation,
   installation and OS-specific activation. Keep distribution mechanics out of
   generic lifecycle policy. Inventory installation owner, scope, channel, OS,
   architecture, paths/digests and installed versus running component versions.
2. Treat the selected release as a compatible component set: CLI, GUI, broker,
   worker when installed, native authentication helpers, SDK/runtime payload and
   protocol/state-schema compatibility. Validate signed release metadata and
   exact artifact hashes using the installation owner's trust mechanism. Record
   intended absent components; never silently add the GUI or a service to a
   CLI-only installation.
3. Respect the owning installation channel. Windows MSI uses the existing MSI
   identity/features and qualified installer behavior; portable ZIP needs an
   explicitly supported replacement path or manual guidance. Evaluate Windows
   file locking, Restart Manager or a minimal signed out-of-process activator,
   UAC/elevation and reboot-required outcomes. Such an activator only applies the
   already approved exact plan; it is not a privileged command execution service.
   Store/winget channels are future rows until independently available and tested.
4. macOS uses the owning signed application/pkg/Homebrew channel with bundle,
   helper, signing and privacy identity preserved. Linux uses its owning package
   manager, Flatpak or qualified native distribution path. Never overwrite files
   owned by another manager, write into read-only app stores, or silently switch
   channel, architecture, scope or publisher trust.
5. Bind one plan to the exact component set, observed preimages, owner/channel,
   target artifacts, compatibility/migration steps, privileges, restart window,
   active-work policy and recovery. Revalidate before mutation. All five lifecycle
   policies and version/channel pins apply; security-only never bypasses a pin,
   restart restriction or scope approval. Reuse existing scheduling for checks,
   notifications and explicitly authorized unattended updates.
6. Journal verified preparation before activation. Coordinate a bounded update
   handoff so the updater does not destroy its only executor/recovery path while
   replacing itself. Define installation and activation ordering from actual
   compatibility constraints, including intermediate versions; do not claim
   atomic multi-component replacement where the platform cannot provide it.
7. Preserve unsaved GUI work and active CLI/workflow jobs. Drain, defer or require
   an explicit maintenance decision before disruptive activation. Replace/reload
   service registration through the existing service lifecycle, then verify the
   actual running broker process/version/digest and selected CLI/GUI, not merely
   installed file versions. Broker restart must not turn uncertain jobs into
   replayed execution. Remote/offline nodes remain individually pending until
   their own authorized target-side upgrade and compatibility checks complete.
8. Preserve settings, package/version locks, machine identities, hardware-key
   registrations, pairing state, credentials, permissions and audit history.
   An update or restart cannot carry an old OS unlock lease into the new process,
   silently re-pair a machine, or weaken authentication/scoped authorization.
   Existing tunnel/connectivity disruption needs explicit maintenance/recovery
   policy; a successful local update does not imply a remote worker was updated.
9. Verify post-update CLI execution, GUI startup/CLI selection, native helper
   loading, broker health, authenticated operator access, paired-node reconnect
   and admitted job/result behavior. Report staged, installed, restart-required,
   active, degraded and failed/recovery states separately. Do not report success
   while an old broker remains active or required components are incompatible.
10. On interruption or failed health checks, reconcile the durable journal rather
    than repeat installation blindly. Restore a verified compatible component set
    only when state migration permits it, otherwise perform approved forward
    recovery. Define backward-compatible migration/backup boundaries and retain
    private recovery data. Vulnerable downgrade requires separate emergency
    authority and unresolved exposure reporting.

## Components and implementation phases

Candidate areas: lifecycle domain/application/CLI contracts; installation-owner
and release metadata inventory; existing platform package workflows and manifests;
remote service registration/activation; native authentication helpers; desktop
trusted CLI selection and update presentation; existing operationrecord and
recovery state helpers. Public command names and authored settings require a
separate implementation design, docs and schema changes where applicable.

1. Inventory-only provider and supported-method matrix. Detect mixed versions,
   stale running processes and missing prerequisites without mutation.
2. CLI-first exact plan, verified preparation and operator-approved update/recovery
   for one qualified distribution row, with native Windows MSI as an explicit
   priority. Establish broker/service and OS-auth support before enabling the
   Windows broker row; CLI/GUI-only Windows installs can qualify independently.
3. Qualify coordinated activation, rollback/forward recovery and packaged GUI
   consumption of the same CLI contracts. Expand macOS/Linux owner-specific rows.
4. Enable scheduled notify, then opt-in automatic updates per qualified platform,
   channel and component set through the existing scheduler/policy owner.

## Acceptance criteria and tests required

- [ ] Discover installed and running CLI, GUI and broker versions/digests, plus
  installed workers/helpers, owner/channel and supported compatibility constraints.
- [ ] Windows MSI CLI-only and CLI+GUI upgrades preserve feature selection and
  identity; portable, unsupported and reboot-required rows are explicit.
- [ ] Locked Windows executables/DLLs, UAC cancellation/denial, installer failure,
  reboot, offline operation and interrupted activation have tested safe outcomes.
- [ ] An old broker process, stale service executable or mixed incompatible set
  prevents a complete-success result, even when installation returned success.
- [ ] Bad signatures/hashes, stale/substituted plans, wrong architecture/channel,
  pin conflicts and expired/revoked authority fail before mutation.
- [ ] Unsaved work and active jobs follow the approved drain/defer policy; restart,
  reconnect and result retry do not duplicate work or renew authorization.
- [ ] Pairing/credentials/security-key registrations, permissions, settings and
  locks survive updates. New processes start locked; OS-auth unavailable fails closed.
- [ ] Crash/power loss at each phase, partial component updates, migration failure,
  failed restart/health checks and failed rollback reconcile without blind replay.
- [ ] CLI owns checks/plan/update/recovery/status; GUI/MCP consume those contracts.
  Native qualification proves installed and running outcomes, beyond cross-builds.
- [ ] Automatic updates remain disabled for unqualified channels/component sets;
  audit records identify before/after components, approvals, restart and recovery.

Planning only. No updater implementation, installation, service restart, dependency
change, release publication or hosted pipeline is authorized by this backlog item.
