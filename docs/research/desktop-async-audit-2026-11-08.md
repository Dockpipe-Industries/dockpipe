# Desktop asynchronous execution audit — 2026-11-08

This review covers the application-owned Qt desktop sources in `src/dockpipe-app`:
shell/navigation, execution, dashboards, Files/Git, graph views, packages, remote
access, observability, terminal, browser integration and native desktop adapters.
It follows a reproduced input-capture defect where an interrupted drag consumed
clicks intended for navigation or the auto-hide dock. That defect was distinct
from the blocking operations below.

The review traced synchronous process waits and worker/future call sites, checked
their callers, and examined cancellation, result identity, coalescing and teardown.
A synchronous call inside an existing worker was not counted as a GUI stall.
Ordinary modal dialogs continue to process events; their expected input blocking
is not proof that a process or worker has frozen the application.

## Repairs in this working tree

| Area | Previous behavior | Repair |
| --- | --- | --- |
| Workflow sessions | Start waited up to 15 seconds; stop waited for graceful termination and escalation; stdin submission waited up to 3 seconds on the GUI thread. | `SessionManager` uses process signals and per-attempt deadlines. Launch returns admission, followed by start/failure signals. Stop and input return without waiting. Startup cancellation, retries, runtime failures and process-group/tree escalation retain explicit handling. |
| Workflow relaunch | A fixed 400 ms delay relaunched whatever was selected when the timer fired. | Await the captured session's terminal signal and relaunch the captured context. |
| Repository-wide stop | Git root discovery ran synchronously for each context, including duplicates. | Discover roots in one worker, coalesce repeated requests and cache roots within the operation. Revalidate context/workdir identity before applying results. |
| Catalog completion | An unused label calculation invoked synchronous Git discovery after the background catalog completed. | Remove the unused query. |
| Files graph | The force solver ran up to 56 iterations over 500 visible nodes on the GUI thread. | Solve value snapshots in a worker; keep one active and one latest pending request. Reject stale results after navigation, projection changes, dragging or destruction. Preserve the public widget's object layout using private QObject-owned state. |
| Remote cancellation | A delayed kill from an earlier cancelled command could kill a later cancelled command sharing the transport. Cancellation during startup could miss the eventual child. | Scope the deadline to its command attempt and reapply cancellation when startup completes. |

The changes stay in the desktop application. They do not change workflow YAML,
engine scheduling, package execution policy, external browser code or release state.

## Remaining findings

These are open findings, not claims that the application is fully asynchronous.

| Priority | Owning source / trigger | Remaining work |
| --- | --- | --- |
| P2 | `shell/presentation/MainWindow.cpp`, `findWorkflowMeta`: a workflow from another repository can call `WorkflowCatalog::discoverAll` synchronously. Catalog discovery has a 30-second deadline per candidate. | Add repository-scoped asynchronous metadata discovery/cache and resume the intended action only after its metadata is available. Do not bypass input/configuration validation on a cache miss. |
| P2 | `files/presentation/ProjectFilesWidget.cpp`, file loading and saving: bounded file reads, metadata checks and atomic save commits execute on the GUI thread. | Move storage I/O behind an owned asynchronous operation with document revision/root identity checks. Preserve dirty-buffer, disk-conflict and atomic-save semantics. A byte limit does not bound slow disk or network filesystem latency. |
| P2 | `remote/infrastructure/RemoteCommand.cpp` destructor; `packages/infrastructure/PackageCommand.cpp` destructor: closing an active preview can wait up to 2 seconds; deleting two package transports can wait up to 6 seconds in aggregate. | Introduce owned asynchronous cancellation/reaping before widget deletion, suppress callbacks after view destruction and preserve command lifetime until the child is reaped. Merely removing the explicit waits leaves QProcess's blocking destructor. |
| P2 | `native-desktop/infrastructure/DesktopAppWindow.cpp`: X11 discovery and frame acknowledgements perform synchronous server round trips on the GUI connection. | Use an independently owned discovery connection / asynchronous reply handling. Do not move Qt's shared X connection into arbitrary pool threads. Qualify embedding/input and capture ownership on native platforms. |
| P2 | `execution/infrastructure/SessionManager.cpp`, output framing: callers with the default zero line-capacity setting can retain an arbitrarily long unterminated line. Parsing and log flushing also run in the GUI callback. | Define a default framing/storage contract and incremental output delivery budget while preserving prompt framing and complete durable logs. Test sustained output and slow log storage. |
| P3 | `browser/integration/WebMermaidRenderer.cpp`: a JavaScript callback decodes bounded but potentially large base64/PNG data on the GUI thread. | Decode value data in a worker and gate completion by render identity. |
| P3 | `workflows/presentation/WorkflowEditorWidget.cpp` and `packages/infrastructure/WorkspacePackageManager.cpp`: teardown can wait approximately one second, plus package transport teardown. | Share an asynchronous subprocess shutdown owner with the transport repair above. |
| P3 | `shared-ui/presentation/Theme.cpp`: Linux fallback theme detection executes two bounded `gsettings` probes synchronously. | Use the native Qt scheme when available; asynchronously resolve/cache fallback probes without delaying theme changes. |

Session log writes and durable settings/store commits also remain synchronous
filesystem boundaries. They require slow-storage qualification; this pass did not
establish a latency bound for their underlying storage.

## Paths checked without a new GUI-wait finding

- Docker inventory/detail/action subprocesses run in worker tasks; their local
  blocking waits do not execute on the GUI thread.
- Main workspace and full-view catalog discovery already use futures with root
  identity checks. The foreign-repository lookup above is a separate path.
- Dashboard observers use asynchronous subprocess clients, source-level busy
  guards and generation checks. Filesystem browsing uses QFileSystemModel.
- Terminal shell discovery and process lifecycle already use asynchronous events
  and owned process reaping. Platform-native terminal APIs still need native
  stress testing; source inspection is not cross-platform proof.
- macOS capture uses latest-frame/coalesced delivery in the inspected adapter.
- The synchronous `--check-installation` subprocess path is a headless bootstrap
  check, not an ordinary interactive startup path.
- Browser implementation belongs to the pinned SpatialWeb SDK. Reviewing local
  forwarding/integration code does not qualify that external implementation.

## Verification

Focused process regressions exercise asynchronous admission, startup cancellation,
failed-start retry, runtime crashes, nonreading stdin, repeated stop, and a GUI
heartbeat during graceful shutdown. A two-attempt cancellation regression proves
an old remote command deadline cannot interrupt its replacement. Graph tests
exercise navigation/projection changes, request coalescing and destruction while
layout is in flight.
The three focused process suites passed. The eight selected integration suites
also passed: main-window launch, package surfaces, Files graph, dashboard
workspace/page, canvas tools, full workflow view and document/Git previews. Package
tests were updated to await the start signal instead of assuming synchronous
startup, then rebuilt and passed. The five source-layout checks and whitespace
validation passed.

Linux local tests are distinct from Windows/macOS acceptance, slow network-disk
qualification and interactive native-desktop stress tests. No exhaustive absence
of races, latency spikes or external SDK defects is claimed.
