## Accepted First Application Consumer (2026-08-18)

The existing DockPipe Launcher is the first full application target for PipeLang, Application IR,
and the standard Qt resolver. Its checked-in Qt/C++ implementation is the behavioral and
presentation oracle. The end state is a PipeLang-authored native Qt launcher with one-to-one
observable parity; this direction does not authorize a redesign, feature removal, or production
language batch.

Parity includes:

| Surface | Required retained behavior |
| --- | --- |
| Native shell | Desktop-native startup, single-instance behavior, tray integration, show/hide, theme handling, and local/offline operation. |
| Basic mode | Project selection, recent projects, app-workflow catalog, icon/list presentation, refresh, configure, and launch state. |
| Advanced mode | Saved contexts, workflow/resolver/runtime/strategy settings, search, worktree discovery, launch/relaunch/stop, stop-all-for-repo, logs, and folder access. |
| Docker observability | Existing container, network, volume, detail, status, log, automatic/manual refresh, inspect, start, and stop behavior. |
| Workflow interaction | Catalog-derived typed inputs and views, prompt/file-picker bridge, subprocess status/output, and current failure reporting. |
| Supporting dialogs | Launcher settings, package management, workflow launch/configuration, context editing, log viewing, and current disclaimers/about behavior. |

### Frozen Docker-observability parity baseline

The checked-in `DockerObservabilityWidget` is the exact oracle for the first launcher slice. This
inventory freezes observable behavior; it does not require generated UI code to retain the current
Qt Widgets structure or invoke Docker directly.

The read-only snapshot has three independently successful or failed sections:

| Section | Stable key and projected fields | Current discovery operation |
| --- | --- | --- |
| Containers | Container ID; name, normalized state, status text, image, ports, and relative creation text | `docker container ls --all --format {{json .}}` |
| Networks | Network ID; name, driver, and scope | `docker network ls --format {{json .}}` |
| Volumes | Volume name; driver and mountpoint | `docker volume ls --format {{json .}}`, followed by `docker volume inspect <name>` for the mountpoint |

Container selection loads pretty-printed `docker inspect <id>` output and the last 200 lines from
`docker logs --tail 200 <id>`. Network and volume selections load pretty-printed
`docker network inspect <id>` and `docker volume inspect <name>` output. Detail requests are
asynchronous and a late response for a formerly selected object must not replace the current
selection. Invalid or non-object discovery lines are ignored, while command failures remain visible
for their own section instead of discarding successful sections.

The containers table retains Name, State, Image, Ports, and Created columns, single-row selection,
status text/tooltips, and the existing state badge categories: `healthy`, `running`, `paused`,
`restarting`, `exited`, `created`, and `other`. Search trims and Unicode-case-folds its input and
matches the joined visible container columns. Networks retain Name, Driver, and Scope; volumes
retain Name, Driver, and Mountpoint. All three retain count summaries, loading, empty, partial-error,
and successful-refresh states.

The view opens cold. Activation triggers the first refresh; while active and visible it performs a
quiet refresh every four seconds, and manual refresh remains available. Only one snapshot refresh
runs at a time; a requested refresh while one is running is coalesced, with an explicit request
taking precedence over a quiet one. Applying a successful section updates rows by stable key,
removes absent rows, preserves the selected object and scroll position when possible, reapplies the
container filter, and refreshes details for a preserved selection.

The current context menu exposes Inspect, Start, Stop, and Refresh. Start is unavailable for
`healthy` or `running`; Stop is available for `healthy`, `running`, `paused`, or `restarting`.
Current mutation parity is `docker start <id>` and `docker stop <id>` followed by refresh and detail
reload, with command failures shown in status/log output. These operations are frozen as later
observable parity only: the first replacement slice is read-only, and subsequent generated UI must
request them through a DockPipe-owned capability adapter rather than embedding process authority.

Parity proof must use deterministic adapter fixtures for complete, empty, partial-failure, stale
detail, refresh-coalescing, selection-preservation, filtering, and state-action cases. A live Docker
engine is useful integration evidence but cannot be the only acceptance oracle.

The implementation order is vertical:

1. freeze an executable/read-only parity inventory for the current launcher;
2. reproduce Docker snapshots, details, and logs through typed records, optionals, deterministic
   collections, and failures, without adding mutations;
3. add refresh/start/stop through an explicit DockPipe capability adapter and operation-result
   events rather than backend commands embedded in generated UI code;
4. reproduce Pipeon discovery, configuration, launch, prompt, output, and stop behavior;
5. reproduce VM workflows, settings, contexts, packages, and the remaining Basic/Advanced surfaces;
6. qualify native desktop parity before making the generated launcher the default; and
7. retain the current implementation as the fallback until the accepted parity matrix passes.

The first milestone is native desktop Qt. A browser, PWA, remote service, account, or network
connection must not be required to inspect or control the local Docker engine. After native parity,
the same generic Application IR and resolver contracts may prove Qt WebAssembly and semantic-web
outputs without making either output the local launcher runtime.

Authored DockPipe YAML remains the durable workflow contract and read-only input to the initial
replacement. The launcher consumes the normalized catalog/projection and stores only the same
launcher/session preferences, drafts, and selections it owns today. It does not rewrite authored
YAML, scan package trees, or duplicate workflow execution semantics.

This accepted consumer is dependency evidence for TASK-021 and this task, not permission to batch
records, optionals, collections, actions, effects, Application IR, Qt generation, and launcher
migration into one change. Each prerequisite remains an explicit versioned vertical slice.

