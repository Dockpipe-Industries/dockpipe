# Unified MCP — CLI-First Management and Desktop Integration

## Architectural Requirement

Added 2026-10-17; applies to TASK-038, TASK-039, TASK-040, TASK-041 and TASK-042.
This refines backlog implementation requirements; no runtime or UI implementation is claimed.

**The CLI and core runtime are the source of truth. The desktop UI is a frontend over those
capabilities, not a separate implementation.**

**Implementation hierarchy: Core runtime → CLI/API → Desktop UI → MCP gateway exposure.**

Use this hierarchy to establish core capabilities and their public interfaces before frontend
integration. It does not make the desktop a runtime dependency of the MCP gateway: headless
installations must expose the same essential capabilities without installing or starting the UI.
Both frontend and gateway use the established core/CLI/API boundary; neither owns a duplicate
execution path, authorization implementation or configuration store.

Inspect the actual repository layering before implementation. Reuse existing application patterns,
components, state management, command interfaces and backend services. Do not introduce new
frameworks, service layers or abstractions where an existing owner already provides the capability.
Any shared-service proposals elsewhere in this initiative mean extending existing owners first,
not creating a parallel stack. Keep Terraform and provider-specific logic in their packages.

## Inspected Reuse Points

Bounded source inspection found these current patterns; revalidate them against the working tree
before implementation because desktop/package work is already in progress.

| Source | Observed pattern | Implementation direction |
| --- | --- | --- |
| `src/lib/application/package_cmd.go` | Existing catalog/install/uninstall/list command routing over domain/infrastructure operations | Extend existing package commands and core services for MCP package contributions |
| `src/app/tooling/dockpipe-launcher/src/PackageCommand.cpp` | Starts the selected CLI with arguments, bounds output/time, handles cancellation/failure and decodes JSON results | Reuse established command/result handling rather than implement package operations in UI code |
| `src/app/tooling/dockpipe-launcher/src/PackageManagerDialog.h` and `.cpp` | Existing installed/catalog UI, filtering, refresh and install/uninstall actions | Integrate MCP packages and management through existing desktop patterns |
| `src/app/tooling/dockpipe-launcher/src/LauncherSettings.cpp` | Launcher-local preferences persisted in `launcher.json` | Do not make launcher preferences a second authoritative MCP configuration store |
| `packages/dorkpipe-mcp/mcpbridge/` | Existing protocol transport, discovery, access checks and dispatch | Expose established capabilities through the gateway without making it a second engine |

These are reuse candidates, not a prescription to relocate every capability into a new class or
service. Identify actual configuration ownership, scope rules, lifecycle supervision, approval APIs,
status events and diagnostic contracts before selecting implementation seams or command names.
Do not claim invented CLI commands already exist.

## Required CLI and Core Surface

All essential management functionality must be available without the desktop:

- Install, update, inspect and remove MCP packages through the existing package manager and package
  commands. No parallel package registry or installation path.
- Configure native capabilities and imported servers; enable/disable them; query status, active
  processes, exposed tools, capability profiles and sanitized diagnostics.
- Configure authentication, OIDC issuer trust and claim mapping, access policies and credential
  references through existing core configuration conventions and authorized management operations.
- Request, inspect, approve/deny and revoke permissions through the established authorization
  boundary with authenticated human approval where required. CLI access is not approval authority.
- Manage local stdio child startup, MCP initialization, discovery, monitoring, bounded restart and
  cleanup through core lifecycle ownership. Define stop/disable/removal behavior, crash recovery,
  cancellation and orphan cleanup. Restart must not duplicate an in-flight privileged operation.
- Manage supported remote imports and expose their tools beside native tools through the unified
  gateway. Keep caller/import credentials, token audiences and permission scopes separated.
- Return structured status, results and errors suitable for CLI automation, desktop rendering and
  gateway use, preserving cancellation, failure and audit contracts.

Core configuration is authoritative. CLI and UI changes must use the same validation, persistence,
scope resolution and authorization. Reflect CLI changes in the UI and UI changes in CLI status;
use established refresh/event mechanisms, expose conflicting/stale edits and avoid silent overwrites.
No UI-only configuration or independently implemented business logic for MCP is allowed.

## Desktop Management Surface

Use the existing DockPipe desktop application to manage installed MCP packages, imported servers,
active processes, exposed tools, capability profiles, authentication, approvals and diagnostics.
Reuse existing components and state/command patterns. UI actions call the same core CLI/API
operations available headlessly, and display their authoritative status and failure outcomes.

UI-local state may describe presentation, but must not become a source of MCP policy or lifecycle
truth. Closing the desktop must not terminate a headless-managed server merely because the UI was
viewing it. Respect explicitly core-owned process lifetime and cleanup policy.

## Ownership Across the Backlog

| Task | Required application of this contract |
| --- | --- |
| TASK-039 gateway | Own CLI/API management, shared authoritative configuration and desktop integration of registry, profiles, status and diagnostics |
| TASK-040 imports | Core-owned stdio startup/initialize/discover/monitor/restart/cleanup and remote-import management, visible through the same CLI/API and UI |
| TASK-038 Terraform | Expose domain execution/status/approval operations through reusable package/core contracts; UI and MCP do not reimplement plan or permission logic |
| TASK-042 federation | Core-owned trust configuration, identity mapping, validation and diagnostics; CLI/UI use the same authorized configuration path |
| TASK-041 Cloud approvals | Optional notification/decision transport over the existing approval contract, retaining CLI/headless local authorization |

## Acceptance Gates

1. On a headless installation, use CLI operations to install/configure/enable a native MCP package
   and a local stdio import; initialize/discover tools, inspect processes/status/diagnostics, restart,
   disable and remove them. Verify bounded cleanup after normal exit, failed startup and crashes.
2. Expose both capabilities through one unified gateway without starting the desktop. Existing
   policy, namespace and cross-import privilege-isolation tests must still hold.
3. Perform equivalent management operations in the existing desktop UI. Confirm they invoke the
   same core/CLI/API contracts and receive the same validation, authorization and lifecycle outcomes.
4. Change settings through the CLI and observe them in the UI; change through the UI and read back
   through the CLI. Repeat across restart and stale/concurrent edits. Verify one authoritative store
   with no UI-only MCP configuration or duplicated policy/execution logic.
5. Exercise authenticated approvals and OIDC trust management headlessly and through the UI;
   ensure neither interface bypasses human identity, expiry, revocation or capability checks.
6. Verify the UI renders package/import/process/tool/profile/authentication/approval/diagnostic
   state and errors from the core. Closing/reopening it must not silently alter managed lifecycles.
7. Review source ownership: reuse the existing package manager, UI components and command/backend
   patterns. Any necessary new primitive must have a documented gap and remain appropriately generic.

Keep these gates in the implementation verification plan. Passing a CLI-only prototype does not
complete required desktop integration; a working UI does not complete headless or core parity.
