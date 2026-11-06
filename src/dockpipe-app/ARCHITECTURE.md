# Dockpipe GUI shell

`src/dockpipe-app` owns Dockpipe's Qt GUI shell and native host integration SDKs.
The engine and CLI remain in `src/lib` and `src/cmd`. The app calls their existing
catalog, package, workflow, Git and remote contracts; it does not implement a
second execution engine. Editor extensions belong to DorkPipe, as described in
[editor ownership](../../docs/packages/editor-extensions.md).

## Source ownership

Source is organized by feature context, then responsibility. Keep a class with
its owning context; add a layer only when there is real code for that role.

| Area | Responsibility |
| --- | --- |
| `bootstrap/composition` | Process entrypoints and Qt startup wiring. |
| `shell` | Main window composition, settings persistence, diagnostics and navigation. |
| `execution` | Saved context values, discovery, process lifecycle, activity and launch UI. |
| `catalog` | Workflow/input metadata values, CLI discovery client and selection UI. |
| `workflows` | Script-path policy and workflow editor/canvas presentation. |
| `dashboards`, `workspace-graph` | Layout state, persistence, graph reading and views. |
| `files`, `git`, `remote` | CLI/filesystem adapters and their respective views. |
| `packages`, `host-sdk` | Package integration, native loading and published host contracts. |
| `native-desktop` | OS capture, permissions and desktop-window adapters and controls. |
| `browser`, `terminal` | External browser SDK integration and shared terminal services/views. |
| `observability`, `shared-ui` | Resource views and reusable Qt presentation primitives. |
| `qualification` | Explicit test composition; unavailable in ordinary builds. |

`domain` holds values and pure decisions. QtCore value types are supported; domain
code must not depend on widgets, process execution, network clients or stores.
`application` coordinates a feature's use cases. `infrastructure` owns processes,
filesystem persistence, CLI transport and platform APIs. `presentation` owns Qt
widgets and interaction. `integration` adapts separately released components.

Native DSM authentication and deployment live in workflow assets. Qualification
under `workflows/qualify/synology-host` established real DSM session and kernel
process identity. The production gateway under
`workflows/package/synology-desktop-spk/assets/native-host` owns DSM validation,
operator binding and browser leases; shared DSM helpers live in
`workflows/internal/synology`. The generic worker/client transport and native
machine snapshot contract remain engine-owned. DSM session handling does not
enter the GUI adapter or generic engine.

The catalog value types and their display decisions live in
`catalog/domain/WorkflowCatalogData.*`; `WorkflowCatalog` owns CLI discovery.
The compatibility client header includes the model header, while DTO-only views
include the model directly. `ContextStore` owns the existing JSON representation
and commits saves atomically. `SessionManager` owns process lifecycle and emits
one failure for each failed start, after clearing that attempt's state.

Some existing classes still combine responsibilities: session coordination and
QProcess management, settings values and their store, and large dashboard/main
window controllers. They are placed according to their actual effects, not
labelled pure domain services. Split those further when a concrete change needs
the seam; do not add pass-through interfaces or move external implementations in.

## Build and compatibility

`cmake/AppSourceLayout.cmake` explicitly lists private source include directories.
Platform source selection and AUTOMOC remain in the existing CMake targets.
`tests/test_source_layout.py` checks source paths, unique local header names and
the domain dependency boundary. No recursive source glob selects native adapters.

Public branding is **Dockpipe**. These existing machine identities are retained
to preserve installed applications, saved data and package ABI compatibility:

- `dockpipe-launcher` executable, desktop/icon IDs and Qt application identity;
- existing IPC keys, `launcher.json`, configuration/cache directories and schemas;
- `DockPipeLauncherSDK`, `DockPipe::LauncherUI`, `DockPipe::HostBrowser`,
  `DockPipe::LauncherHeaders`, installed header names and native plugin IIDs;
- release component names, SDK version/pins and resource names used by consumers.

Their legacy spelling is a compatibility contract, not the product name or source
location. Source headers moved, but SDK installation keeps the flat
`include/dockpipe-launcher` closure. Browser forwarding headers still consume the
pinned SpatialWeb SDK; this app does not own that browser implementation.

The remote hardware-key ceremony uses a normal private dialog in
`remote/presentation`. The CLI owns native FIDO2 device access and verification;
`RemoteCommand` validates its bounded interaction frames and sends a single
response through the child process's private stdin. The dialog never stores PINs
or credentials and closes when the owning command finishes. Credential interaction
uses `DockpipeChoices::operatorDockpipeBinary`: an explicit absolute host override,
an installed sibling CLI, or an absolute PATH directory. Selected-project binaries
and relative/current-directory PATH entries cannot become implicit PIN endpoints.

Remote access security controls are also presentation-only CLI clients. The
Settings dialog's Security tab reads global policy and current lock status from
`dockpipe remote security-status`; lock, native-authenticated unlock and policy
changes use the same trusted operator CLI selection. They do not store duplicate
policy values in `LauncherSettings`. Lock and unlock actions remain reachable if
status reporting is unavailable; the CLI reports whether the host can perform
them. Platform capability gaps are shown explicitly.

An optional `DOCKPIPE_NATIVE_HOST_CONNECTION` reference selects an authenticated
native host for terminal and system/Docker monitoring. MainWindow reads this
absolute file reference once and shares one `NativeHostConnection` with those
views and the dashboard terminal hub. Only the trusted operator CLI's bounded
`host status` response supplies connected host, operator and generation.
Connection/key contents remain CLI-owned and are never read or saved by Qt.

The native terminal launches `host terminal` through the existing local PTY with
no container working directory forwarded. The native worker owns the actual host
PTY, default home and revocation. Status loss or an identity/generation change
closes native terminal entries and removes their replay buffers. Monitoring
clears previous data and retains the unavailable host selection. Container
shells and monitoring remain explicit choices labeled Application container;
paired monitoring retains its existing read-only contract. Deployment-owned
authentication and exclusive stream ownership remain outside this adapter.
Native Docker start/stop actions use the CLI's typed host operation and require a
matching action/resource success receipt. Failed or cancelled mutations trigger
inventory reconciliation without retrying the mutation. Read-only native inspection and the last 200 container log lines use the typed,
bounded `host docker-query` contract. Selection, pause, host identity and generation
changes cancel pending queries and discard stale responses. Raw inspect is local
to the authenticated operator view. Container exec remains in the native host
terminal; paired-node controls remain unavailable.
Terminal cleanup covers the owned PTY/session. Intentionally detached or
privilege-changing processes follow normal host terminal semantics and may
outlive terminal closure.

## Verification

Use `workflows/build/desktop-development/config.yml` with `--configure-only` for
configuration admission, or repeated `--target` for a focused native build. Run
selected tests through `workflows/ci/desktop-native/config.yml --build ... --regex ...`.
Use task-owned output directories; do not reuse a CMake cache that names the old
source directory. A successful configure, focused test, native GUI run and
cross-platform build are separate evidence. The source cleanup does not itself
qualify Windows/macOS behavior, packaging, native embedding or GPU presentation.
