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
| `environments` | Saved definitions, the asynchronous CLI reader, and passive runtime observations for the shared canvas and list. |
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

## Native package worker host

The explicit macOS `--native-worker-host --request <private-file>` mode dispatches
before QApplication and GUI single-instance handling. It runs one admitted native
package entrypoint on the main thread inside the existing Dockpipe app identity;
it does not create a separate provider app or start a remote session implicitly.

The C-compatible Native Worker Package ABI v1 is separate from the Qt Display
and Workspace SDKs. The package owns its backend, native run loop and opaque local
profile; the host owns private descriptor/hash/signature admission, stop signaling
and module lifetime. Production preserves Developer ID/team and library-validation
boundaries. Explicit hash-pinned ad-hoc review is not production signing. Unknown
cleanup is never success, and code remains mapped until the dedicated process
ends. The CLI reference must select the app's bundled CLI. Existing services and
paired state remain independent in this first slice.

See [Native Worker Package ABI v1](../../docs/packages/native-worker-sdk.md) for the
bounded descriptor and lifecycle contract. Published Display SDK1.0 is unchanged.

## Optional display pre-close

`packages/infrastructure/NativeDisplaySession` coordinates an optional QObject
pre-close operation exposed by a package surface. It captures the reference and
bounded completion budget, ignores foreign or late results, and supports explicit
cancellation. `PackageDisplayWidget` retains the surface and module until that
operation finishes, then calls the unchanged synchronous SDK close method before
deletion. Immediate completion is deferred out of the package's signal stack.

The remote desktop page owns its Closing state and validates completion against
the selected session context. Hiding the page starts asynchronous closure by
default. The visible, persisted "Keep connected across tabs" option defaults off;
opting in retains the surface and session while hidden and clears input focus.
Explicit disconnect still closes the session. Destruction and legacy providers
retain bounded synchronous cancellation without
claiming remote cleanup proof. Protocol, media and native input cleanup stay in
the package. No asynchronous semantics are added to the published SDK 1.0 virtual
close contract.

Remote desktop preferences are app-owned presentation and connection options.
The shell selects an installed resolver by the `remote.desktop` capability and
the catalog's provider identity/version. It launches that resolver's native display
with the provider-independent `dockpipe.remote.desktop-display/v1` options envelope:
`cli`, `state_directory`, the exact `catalog`, `mode`, and the `capture`, `display`
and `decoder` preference hints. These hints do not select a transport or grant
capture/input authority. The provider validates its supported settings and reports
unsupported requests; it owns protocol negotiation and platform behavior. See the
[native display contract](../../docs/packages/package-model.md#package-owned-native-display-widgets).
Quality and frame rate are passed to the package at explicit connection time;
the package owns authenticated media negotiation and the native capture limits.
The optional virtual-desktop choice uses those same preset sizes and is frozen
with the capture options. The package owns OS display creation, target selection
and teardown; the GUI does not create displays or change physical monitor modes.
The local decoder preference is also frozen per connection. Software preserves
the existing default; Automatic may fall back with actual-backend reporting,
while Hardware only must not silently select software. Remote hardware encoding
remains package-owned and independent of this viewer preference.
Fit/actual-size and cursor preferences can update an existing surface through the
optional QObject `setPresentationOptions(QJsonObject)` hook. The generic display
adapter validates the existing JSON bounds and, when exposed, checks the optional
`presentationOptions` readback property. This does not change SDK 1.0 virtuals.
Fullscreen temporarily preserves the same surface while reparenting it and
restores the user's tab-retention preference when it returns. Dashboard tiles
reuse this remote-desktop page in compact presentation with independent sessions.

The "Share text clipboard" preference is frozen into control connections as the
package-owned `capture.clipboard_text` option. It defaults on for control and is
omitted for view-only sessions or when disabled. The shell does not read remote
clipboard contents or add clipboard logic to the generic broker. The provider
owns negotiation, focused-session text transfer, acknowledgment ordering and OS
access. Clipboard changes apply on the next connection, like capture settings.

Image and regular-file sharing use independent opt-in connection preferences,
`capture.clipboard_images` and `capture.clipboard_files`. Both default off and
are omitted for view-only connections. An explicit rich paste shortcut can wait
for transfer confirmation and finish that original paste once its physical keys
are released. File drops transfer copies and then paste into the active remote
application; they do not claim native drag into a destination under the pointer.
The package binds one pending gesture to the session, clipboard generation,
focus epoch and modifier policy. Intervening input, focus loss or changed content
cancels it; ordinary clipboard changes never initiate a paste. Preparation,
acknowledged byte progress and final publication are shown by the package.
Image validation, streaming, private staging, quotas and clipboard publication
remain package-owned. These extensions remain under qualification.

The optional `display.swap_control_command` presentation preference defaults off.
When enabled, the viewer maps both physical Control keys to Command and both
Windows/Super keys to Control. Alt/Option is unchanged. A live change releases
already-sent keys before applying the new map. The physical emergency release
chord remains Ctrl+Alt+Shift+Escape. This is local viewer behavior and changes no
Mac keyboard preference or generic input protocol.
