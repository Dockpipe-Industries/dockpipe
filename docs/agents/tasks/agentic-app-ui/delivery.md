## Ownership Correction — 2026-10-17

Repository ownership note: package paths in this record describe external source. Current catalog packages live in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages); previously extracted [DorkPipe/MCP](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages) and [Pipeon](https://github.com/Dockpipe-Industries/pipeon/tree/master/packages/pipeon) remain in their own repositories. Historical paths and receipts do not recreate a local package tree.

DorkPipe owns the AI features and their delivery. The earlier separate DockPipe AI
package/app plan below is superseded wherever it conflicts with the
[active ownership decision](overview.md#product-naming-and-ownership).
Implement package-contributed views in the existing DockPipe app, shared CLI/API
operations and tools through TASK-039. No separate manifest identity, launcher app
or mandatory local container/service stack is required. Preserve existing generic
launcher work and historical proof; remote/mobile surfaces are optional future scope.

## Prioritized Delivery (not implemented)

1. Audit and deliver shared core foundations first: TASK-043 effective networking,
   ingress/egress/proxy configuration and verified backend guarantees, alongside
   exact-version package selection/lifecycle needed by all consumers.
2. Establish TASK-039's unified MCP and smallest generic contribution seams for
   package commands, existing-app views and tools. Preserve headless use.
3. Deliver DorkPipe DORK-001 knowledge ingestion/search, graphs/DAGs and context
   through those contracts, including MCP-routed retrieval from a minimal AGENTS
   bootstrap. Full-text search and explicit graph operations need no inference.
4. Keep skills management (DORK-002), chat, Models UI, richer AI authoring, editor consolidation and remote/mobile
   presentation deferred. They do not gate the core/MCP/knowledge release path.

Build in bounded slices after the direction audit. Core-first does not require every
future networking backend to be implemented before local MCP/knowledge qualification;
qualify the guarantees needed by each target deployment without silent fallback.

DORK-001 remains the authoritative knowledge backlog in the extracted DorkPipe repository;
this sequence coordinates integration and does not duplicate its implementation scope.

## Preserved Pipeon Interim Direct-Agent Slice (paused)

Pipeon is hidden and paused. The following previous design is retained for reference;
it does not authorize more Pipeon product work or block DorkPipe integration:

- Chat: provider/model-selectable direct chatbot for lightweight questions, local commands, and
  handoff.
- Codex chat: host `codex exec` with workspace sandboxing, host config model selection by
  default, and per-Pipeon-session transcript resume.
- Claude chat: guarded DorkPipe sandbox gate only; do not run raw Claude directly from chat.
- Workflow: `/workflow <name>` or generated workflow YAML for agentic work, including model lanes,
  multiple workers, approvals, verification, artifacts, and escalation.

Direct agents should not pretend to be multi-reasoning. They can recommend or launch workflows when
the task crosses the value bar for orchestration.

## Still Open

- Keep [optional DorkPipe browser assistance](browser-integration.md) deferred:
  define reusable contribution and authorization APIs before AI page control,
  summaries, annotated requests or explicitly granted history access. Ordinary
  browser settings and extension-based password/autofill remain independent.
- Deliver AI contributions from the existing DorkPipe repository and package pipeline;
  coordinate generic registration and selected-version lifecycle with TASK-039.
- Define the launcher context payload shared with Pipeon-style app launches.
- Decide which YAML contracts exist for MCP connectors and agent/task packs before building rich
  editors.
- Extend the initial `dockpipe.operation_event.v1` JSONL stream into the full DockPipe AI run inspector
  feed, including logs, artifact references, approvals, and task graph state.
- Add a future graph visualizer for repo guidance and task contracts so DockPipe AI can render
  relationships across `AGENTS.md`, `docs/agents/*.yaml`, linked task docs, workflow/package docs,
  and other durable agent-routing sources instead of depending on markdown-link-only tooling.
- Extend the built-in editor where needed; the launcher owns text editing rather than launching the system default editor.
- Design conflict preview and repair flows without turning the app into a full IDE.
- Decide where remote-access YAML lives and how it maps to Cloudflare Tunnel, Let's Encrypt,
  free-domain/subdomain, and bring-your-own-domain flows.
- Define the threat model for exposing the app remotely, including auth, allowed operations,
  approval prompts, audit logs, and teardown.
- Decide the Qt mobile packaging path, authentication model, offline behavior, and which actions are
  safe to expose from mobile.

## Launcher workspace continuation (2026-10-09)

The user authorized a workflow-focused launcher shell during remote onboarding.
The Qt launcher now exposes Apps, Workflows, Machines, Activity and Docker, using existing
catalog/category metadata and CLI execution. Machines offers provider-neutral
broker setup, online pairing with explicit code comparison, and user-service
startup. Remote runs choose an enrolled node, preview the selected source snapshot,
and submit its exact digest. Activity reads broker job state and downloads results.
This is launcher execution/onboarding UX, not the DockPipe AI workflow editor or an
AI planner. All seven Qt checks pass, including the new remote UI fixture test and visual
inspection of captured screens. This local validation does not qualify native macOS/Flatpak UI or a
live cross-machine workflow; these remain distinct test boundaries.

The follow-up layout pass removes the nested Applications/Docker navigation, shares a
workspace selector across all pages, and introduces app cards, compact rows, and an
empty-library package action. Docker has one page and one refresh owner. Fixture checks
cover navigation, app action identity, empty-library package access, and Docker discovery;
rendered dark/light and desktop-width captures are inspected without live provider access.

The Machines page now starts with a bounded machine list and human-readable access
states. Pairing and worker connection are separate guided pages; broker setup is secondary.
Details are collapsed, approval/removal controls require a selection, and pending requests
refresh only on the visible pairing page. Fixture checks cover the invited-state explanation,
tall-window layout, navigation, consent gating, code display, and cancellation.

Provider-aware Machines setup now consumes resolver metadata from the public catalog and
shows active provider identity/address separately from the setup choice. Local-edge and
hosted-broker forms follow generic package hooks. Actual Dockpipe Cloud account login and
managed backend remain future work; see TASK-036 and the canonical remote resolver contract.

## Launcher workflow studio first slice

The launcher now has a Qt Widgets canvas with zoom-aware grid lines over ordinary workflow YAML: ordered
command/script/package cards, async groups, an inspector, a YAML tab, validation,
atomic save with external-change detection, and run routing through existing
launcher sessions. The generic `workflow document` CLI bridge owns bounded YAML
syntax-tree edits and preserves unedited fields/comments. Layout stays in local
preferences. This does not introduce new execution semantics or implement the full
DockPipe AI agent/MCP authoring scope. Live canvas status, arbitrary wiring, richer
catalog selectors, and native macOS/Windows qualification remain open.

The same pass refines shared launcher buttons, tab states, surface radii, focus styles,
status badges, and inspector spacing in both themes. It uses Railway as a visual
reference while preserving Dockpipe navigation and execution behavior.

The editor now occupies the Workflows page rather than a modal dialog. Each launcher
starts with a blank canvas; an on-demand searchable workflow picker opens existing
sources. Properties and the floating run console are collapsible. Runs reuse the
existing SessionManager and remote submission flow; local output stays beneath the
canvas. Workspace/file switches and quit guard unsaved drafts. Fixture integration
covers embedded layout, picker loading, keyboard output toggle, run routing, and
cancelled draft switches. This changes launcher presentation, not workflow semantics.

The canvas layout now extends beneath floating controls: workspace/workflow selection
at the upper left, Add/Save/Run at the upper right, compact view controls at the lower
left, and output/run-target controls at the lower right. Full-width workspace,
authoring, tab and footer strips are absent on Workflows. Properties and output are
bounded overlays; fixture checks assert they do not resize the canvas or overlap
one another at the supported desktop size. Other launcher pages keep their shared
workspace header.

Properties and Output share a movable, resizable canvas-window implementation.
Header dragging, edge resizing, click-to-front, persisted placement, viewport
clamping, and Reset panel layout keep these tools under user control. The canvas
uses scalable stroke icons, refined card hierarchy, theme-aware surfaces, and
visible zoom percentage. Linux integration checks exercise panel movement,
resizing, reopen placement, and reachability after resizing the launcher; rendered
dark and light previews are reviewed separately from native platform qualification.

Shared launcher window chrome replaces separate OS title strips on Linux/Windows
with controls in the menu row. Application dialogs, including standard confirmation
messages, share an in-app heading and close control while preserving QDialog and
QMessageBox result handling. File pickers retain native/portal behavior; macOS keeps
native main-window controls. Fixture checks cover header dragging, maximize/restore,
and cancellation through the confirmation close button. Native macOS/Windows
qualification remains outstanding.


The workflow picker now supports preview and direct Run alongside Edit on canvas,
with recent runs and remembered targets. Direct runs preserve unsaved canvas drafts.
Workflows includes a movable Activity panel filtering local sessions to the workspace;
the global Activity page remains a list and remote jobs remain broker-wide.
Docker and Machines share a resource canvas with movable details panels preserving
the existing controls and inventories. Docker separates declared Compose dependencies
from network/volume attachments; Machines reports access relationships, not unverified
online health. Graph refresh preserves positioning and selection. Graph limits and
partial discovery are visible, with full inventories in Details & actions. These pages
also hide the shared workspace header; Apps retains its card library.

Docker canvas layer toggles now persist visibility for containers, networks, volumes,
inactive containers, and dependency edges without removing inventory or rearranging
cards. Lifecycle status drives explicit labels and muted inactive cards; health is
interpreted only for running containers. Fixture coverage includes filters surviving
refresh, preserved layout, visible edge removal, and selectable stopped containers.
The proposed unified execution-observability specification is a roadmap; this slice
does not add execution telemetry collectors or a system process explorer.

Docker now uses a selected-resource inspector instead of repeating the inventory on
the canvas. Container logs are the default, include both output streams, and preserve
scroll position across bounded refreshes. Related-resource links focus graph cards;
readable metadata and raw inspect remain available. The complete inventory is a
separate on-demand dialog. Start/stop actions are asynchronous, and Exec launches an
explicitly chosen shell through the existing terminal helper extended for host commands.
Local fixture tests verify navigation, stopped-container exec gating, stderr logs,
terminal argument delivery, and action failure recovery without entering live containers.
Native macOS/Windows terminal validation remains outstanding.

### Unified execution observability

The implementation and support boundaries are documented in
[`docs/runtime/execution-observability.md`](../../../runtime/execution-observability.md).
The completed local objective and verification record is
[execution-observability.md](execution-observability.md).
Executions adds a Managed canvas and bounded System process table; it reuses the
runtime registry, existing process supervision, CLI transport and ResourceCanvas.
It introduces no independent orchestrator, remote process-control authority or AI assistant.

The System extension adds aggregate resources and a bounded CPU history. System
and Docker reuse paired-machine selection, backed by worker-local opt-in and
operator-authenticated outbound snapshots. Remote Docker is inventory/topology
only; local controls remain local. Raw logs and remote jobs are now under Executions.
See the same observability record for the extension's proof and limits.

### Navigation consistency and sidebar visibility

The View menu and sidebar share the same page actions and selection
state. A persistent top-left toggle, View checkbox and Ctrl+B hide/show the
sidebar while preserving the selected page. Visibility uses the existing local
launcher preferences. The launcher rebuilt and all 15 CTests passed, including
menu/sidebar synchronization, gained workspace width, hidden-sidebar navigation,
and saved preference readback. The menu and collapsed layout were rendered and
visually checked; receipt: `/tmp/dockpipe-navigation-tests.log`. No engine,
workflow contract, package or remote authority changed.

### System table sorting and Containers naming

Every System process column supports ascending and descending sorting. Numeric
metrics use their raw values, unavailable values remain last, and sorting applies
before the visible row limit. Refresh preserves the sort and process selection.
The Docker page is named Containers in the sidebar, shared View menu and canvas
heading. The launcher rebuilt and both affected CTests passed, including all six
sort columns, refresh selection, numeric ordering and navigation labels; receipt:
`/tmp/dockpipe-sort-tests.log`. These UI changes preserve engine/package boundaries.

### Activity merged into Executions

Executions now contains Processes & resources and Runs & logs. The standalone
Activity page is removed from both sidebar and View menu. Existing local logs/stop
and remote cancel/result-download controls are reused inside Executions, and remote
submission opens Runs & logs with Remote jobs selected. The workspace-scoped workflow panel
is also labelled Runs & logs. Monitoring pauses when its subview is hidden.
All 15 launcher CTests passed, including navigation synchronization, preserved
workflow run history, embedded run controls and remote cancellation arguments.
Local and remote run views were rendered for visual inspection. Receipts and
screenshots are under `/tmp/dockpipe-executions-merge*`; the launcher build directory
was updated. No engine, package or remote authority changed.

### Runs and logs visual refinement

Runs & logs now uses the canvas palette, flat section navigation, a compact location
selector, run counts and explicit empty states. Actions sit above the list and are
enabled for a valid selection; local Stop also requires a running launcher session.
Successful remote responses no longer expose JSON in the page; failed commands
retain bounded diagnostic output. The sidebar toggle has a padded menu-corner
container with verified separation from File at 1180 and 1910 pixel widths.
The full 15-test launcher suite passed, followed by focused failure/recovery and
layout checks. Dark/light fixture screenshots and test receipts are under
`/tmp/dockpipe-runs-ux*`. Runtime ownership and package/engine boundaries are unchanged.

### Project files, workflow files and project Git (2026-10-14)

The launcher now separates repository-level Files/Git from the workflow's Files
view. Both file surfaces use a built-in UTF-8 editor with a lazy folder tree,
line numbers, syntax hints, find, CRUD, atomic saves, dirty-draft prompts and disk
conflict detection. Workflow Files is rooted beside the workflow YAML; its active
YAML document is shared with the existing canvas and validation/save pipeline.
Project Files covers the selected project directory. New workflows establish their
file root on first save. Editing is limited to regular UTF-8 files up to 1 MiB;
linked paths and Git metadata are excluded.

Project Git reviews saved files, commits only checked paths, preserves unselected
staged changes, pulls only by fast-forward with a clean tree, and pushes the current
branch to origin without force. The generic runtime owns these human-requested
actions through `dockpipe git --request`; managed-agent session authority is
unchanged. Review fingerprints reject stale writes. Cancellation, bounded output
and failed-commit recovery are explicit. Branch management and a merge-conflict
editor remain outside this initial slice.

Issue #137 repairs make vault setup opt-in for new projects, resolve OS state-base
aliases without allowing links inside Dockpipe-owned private state, and avoid
persisting separate Flatpak document grants in script commands. Existing user
secret configuration is preserved. Issue #136 (host-wide Flatpak process visibility)
remains open; this slice does not widen sandbox permissions.

Local proof includes the affected Go packages, the launcher suite with real CLI
file/Git round trips, the package validation fixture, linked-home execution and
an AddressSanitizer workflow-editor run. Visuals were rendered offscreen. The broad
Go library sweep encounters existing loopback sandbox and compiler-containment
requirements; it is not a full-suite qualification. Native Bazzite/Flatpak,
macOS/Windows and authenticated remote Git still need their host-specific checks.
Documentation and release notes were updated; this work is not yet published.

### Shared editor and source-control refinement (2026-10-14)

The project and workflow file surfaces now share a spaced explorer with native
vector icons, a focused file header and actions menu, empty state, on-demand find,
encoding/language/cursor status, indentation, current-line emphasis and zoom.
Workflow Files hides duplicate canvas Add/Save controls. KSyntaxHighlighting's MIT
engine is built offline from a checksum-pinned source archive, with separate
compressed grammar data and notices. All 14 portable SDK languages are covered,
including a PipeLang grammar aligned with the existing language extension. The
editor provides syntax highlighting, not language-server or compiler services.

Git uses a dedicated commit panel and one selected file's review. A generic,
read-only `diff` request validates the snapshot and exact changed path; it does not
change managed-agent Git authority or introduce package knowledge into the engine.

Validation: all 19 launcher CTests passed with real CLI workflow/Git fixtures;
focused light-theme checks also passed. New Go coverage verifies exact-path,
read-only and stale-review behavior. AddressSanitizer passed file CRUD and the
real-CLI workflow editor. Dark/light fixture captures include an 820-pixel editor.
A local Debian artifact was assembled and its installed grammar-resource path
verified using the language fixture. Native Windows, macOS and Flatpak packaging
remain unqualified on this Linux host. Local receipts are `/tmp/dockpipe-ux-*`;
build output and a preview app were updated. No commit or publication occurred.

### Optional filesystem graph (2026-10-14)

Files now lazily creates a Workspace graph for its existing project/workflow root.
The graph shares the editor containment policy and the existing native canvas
pan/zoom/grid surface. One transient launcher helper streams immediate directory
entries into bounded lexical pages; it never recursively inventories a repository.
The visible/retained limit is 500 nodes, with pages up to 150 and no collapsed-node
cache. Cancellation, a single global reader slot, response/path budgets, changed
page detection and deadlines bound failures and navigation churn. File activation
and reveal route through ProjectFilesWidget. No engine/package contracts changed.

This delivers the filesystem-navigation slice only. The earlier guidance/task
relationship visualizer and future dependency overlays remain open. No parsers,
background indexers, Git scans, persistent services or additional dependencies were
introduced. A 2D/3D toggle projects the same bounded node set onto a native
orthographic grid. Mouse orbit and camera controls repaint only on input; there
is no OpenGL context or frame loop. Switching back restores manually positioned
2D nodes without rereading directories.

All 20 launcher CTests pass, including real CLI integration. Graph fixtures cover
5,000 immediate files, the 500-visible-node cap, rapid navigation, timeout,
cancel/close, symlinks, permission failures, stale/deleted paths, mouse orbit and
editor routing. Dark/light 1180- and 900-pixel captures were inspected. Local
Linux measurements: roughly 18–19 ms to open the small graph, 12–13 ms to expand,
28–30 ms for the first 150 entries of the 5,000-file fixture, and about 3.4–3.6 ms to
project 500 nodes per camera change (excluding paint). Idle CPU measured about
0.000043–0.000045 seconds over one second. RSS remained stable across 12 open/close cycles;
Qt/allocator caches retain some high-water memory after scene objects are freed.
Initial graph startup comparison with the graph closed was 57,284 versus 57,456 KiB
RSS, with no measured CPU ticks over three idle seconds; this single-run comparison
predates the 3D extension and is not a zero-overhead claim.
The graph suite also passed AddressSanitizer with leak detection disabled;
normal-build memory stability is measured separately from ASan's quarantine.

Receipts are under `/tmp/dockpipe-graph/`. Native interactive frame-rate, network
mounts, million-entry directories and Windows/macOS/Flatpak remain unqualified.
The user reported a system failure while adding Biztraak in an older build. The
available journal search did not establish an OOM or GPU fault; no reproduction
or root cause is claimed. Existing synchronous workflow catalog fallback scanning
is a separate investigation candidate. Adding a project does not create a graph.
No commit, publication or replacement of the user's running preview occurred.

### Graph city styling and mouse orbit (2026-10-14)

3D now renders files as native chip geometry, folders as taller blocks, and
relationships as projected circuit traces. Amber distinguishes links/unavailable
entries. Middle-mouse drag rotates alongside Shift-left-drag; release ends orbit
and 2D mouse behavior remains intact. Small zoom levels omit illegible pin/label
detail. Camera trigonometry is computed once per angle change. No new dependencies,
timers, graphics contexts, readers or engine/package changes were added.

The launcher was rebuilt and graph fixtures passed in dark/light themes, including
middle-drag/release, 2D restoration, 500-node city rendering and repeated cleanup.
Native offscreen captures were inspected. Updated local timing/RSS receipts are
`/tmp/dockpipe-graph/test-city.log` and `test-city-light.log`; paint timings include
a complete zoomed-out viewport capture, not a native-display frame-rate guarantee.
The running preview remains untouched and requires a restart to load this build.

File chip height now derives from the logical byte size in the helper's cached
per-page file metadata. Decimal byte counts avoid JSON precision loss. Empty files
remain visible; logarithmic height is capped, folders stay fixed, and links have
no target size lookup. Labels show size and tooltips expose exact bytes. Refreshing
a branch updates the snapshot; no contents, directory totals or size polling are
introduced. Graph tests cover empty/increasing sizes, the maximum-size cap, helper
metadata and the existing lifecycle/performance suite. Receipts and the inspected
native dark capture are `/tmp/dockpipe-graph/test-size.log` and `size-dark.png.3d.png`.

### Selected labels, GPU rendering and drone flight (2026-10-14)

Selected city labels now paint in the view foreground at a readable screen size,
above other geometry without changing chip depth order. Pixel assertions verify
that a higher-z occluder cannot cover the selected label and that deselection
restores ordinary ordering. Labels remain inside the viewport edges.

3D now tries a Qt OpenGLWidgets viewport on explicit entry, probes renderer support,
and falls back to raster painting for unsupported/offscreen displays, failed
contexts or detected software GL. Options exposes the acceleration switch and a
GPU/Software indicator explains the renderer. `DOCKPIPE_GRAPH_SOFTWARE=1` defaults
to raster. A one-shot viewport initialization check replaces failed GL widgets;
there is no frame loop. Linux CI/container build dependencies now explicitly include
the OpenGL development component, and Windows packaging checks its deployed DLLs.

Options also exposes an optional drone-flight Easter egg over a bounded snapshot
of already-loaded nodes. The first-person city has a static night environment,
cockpit, perspective clipping, simple building collision and bounded altitude.
WASD/QE, right-drag/arrows, Shift and Escape control movement/look/boost/exit. A
33 ms movement tick runs only while keys are held, stops on focus loss/hiding/exit,
and never scans files. Returning restores the graph and renderer preference.

All 20 desktop CTests passed with real CLI integration; the light-theme graph
fixture passed separately. Native disposable windows verified the actual viewport
renderer as NVIDIA GeForce RTX 3080, software/GPU switching, drone entry/exit and
visible rendered output. Native GPU captures were inspected (window readback is
required because QGraphicsView owns OpenGL viewport painting). Receipts are under
`/tmp/dockpipe-graph/`: `test-final-flight.log`, `test-flight-light.log`,
`gpu-smoke.log`, `gpu-viewport.png` and `gpu-viewport.png.flight.png`. Other native
platforms and packaged releases remain unqualified. The active user preview was
not replaced, and nothing was committed or published.


### Cyberpunk hangars and navigation drone (2026-10-14)

Flight now replaces its initial snapshot with one immediate directory page (150
structures maximum). Procedural beveled/stepped towers, emissive facade details,
hangar signs, runway strips and proximity shutters share the existing painter
renderer. Details are culled beyond 1,800 simulation units. Screen-space polygon
and line clipping prevents near-plane projections exceeding Qt's coordinate range.

Crossing a valid directory threshold requests one page through WorkspaceGraphReader;
there is no recursive prefetch or ancestor-world cache. Read failure retains the
previous world and leaves the camera outside the door. Parent, next-page and retry
controls reuse root containment, timestamp checks and cancellation. Links cannot
be entered. Hiding/exiting cancels the helper and invalidates the request generation.
The preserved normal graph remains separately bounded to 500 nodes.

Normal 3D selection has a 360 ms drone approach, followed by a static hover. File
activation lands the drone; directory activation opens the hangar and zooms into
the children in 650 ms. Explicit branch expansion remains available. Navigation
animations can be disabled and are canceled on hiding, mode change or a new
selection. All changes stay in the native launcher; engine/package contracts and
file-write authority are unchanged.

Validation: all 20 desktop CTests passed with real CLI integration, including
physical portal crossing, empty directories, parent navigation, 150-entry page
replacement, failed/out-of-root reads, cancelled reads, delayed file opening and
cancelled landing. The light-theme graph fixture also passed. Native GPU smoke verified the RTX 3080 viewport and visually
inspected hangar output; the new run emitted no coordinate-overflow warnings.
Receipts: `/tmp/dockpipe-graph/test-hangars-suite.log`, `hangars-gpu.log`,
`hangars-gpu.png.flight.png` and `test-hangars-final.log`. Builds and screenshots
are local generated artifacts. Packaged Windows/macOS/Flatpak qualification and
publication were not performed.


### File targeting and shared cockpit editor (2026-10-14)

The flight crosshair ray tests the bounded visible building set, including folders
and links as occluders. Left-click/Space produces one 140 ms targeting pulse;
regular-file hits request the existing ProjectFilesWidget editor. No shot saves,
executes or deletes content, and linked/unavailable structures cannot be edited.

The translucent HUD temporarily reparents the original editor pane and status
widget above the flight viewport. It reuses the same documents, undo stacks,
shortcuts, workflow document, bounded reads, atomic saves and disk-conflict checks;
there is no duplicate buffer or filesystem model. Escape returns to flight with
drafts preserved. Flight exit, graph closure, project navigation and widget
teardown restore the pane before deleting the graph. Movement and pending shots
stop while editing, and hiding or losing focus cancels a pending targeting pulse.
The overlay resize follows the viewport without a render timer or image copies.

All changes are confined to the native launcher and its docs/tests. Engine,
workflow execution and package boundaries remain unchanged.

Validation: all 20 desktop CTests passed with real CLI integration. The extended
graph test covers ray hits, folder/link occlusion, misses, hidden-shot cancellation,
shared editor identity, paused controls, save/readback, external disk conflicts,
dirty-switch cancellation, oversized/binary rejection, draft/undo retention,
flight/graph exit and workflow-document sharing. The light-theme graph run passed.
Native RTX 3080 tests verified translucent composition at wide/compact sizes,
Ctrl+S save and Escape resume; the captured windows were visually inspected.
Receipts are `/tmp/dockpipe-graph/test-hud-suite.log`, `test-hud-light.log`,
`hud-gpu.log`, `hud-gpu.png.hud-compact.png` and `hud-light.png.hud.png`.
The local launcher was rebuilt; no release or packaged-platform qualification was
performed. Build products and screenshots remain local generated artifacts.

The visual pass also fixed transparent-editor syntax contrast in SourceEditor:
theme selection follows foreground contrast when a transparent stylesheet leaves
Qt's Base role stale. SourceEditorTest now verifies both transparent themes.
The GPU HUD test also switches raster/OpenGL while the editor is open and verifies
that the original pane stays above the replacement viewport.

### Workflow file actions and app billboards (2026-10-14)

Files and the shared flight HUD offer **Open as workflow** for YAML. The action
preserves the existing dirty-file prompt, opens Workflow Studio, and leaves
execution behind its normal validate/save/Run flow and launcher input/session
controls. No shell-script execution action was introduced.

Drone flight has an off-by-default **App billboards · Experimental** option with
an explicit picker for up to three app windows. The first backend supports native
Linux X11 with XComposite/XRender; other platforms and Flatpak report unavailable.
Selected app previews are scaled server-side to 1280 × 720, captured at two frames
per second total and retained only by the visible flight. A failed stream stops
after three attempts. Shooting opens a front-facing native app panel inside
Dockpipe, with genuine native mouse/keyboard input rather than desktop-wide input
injection. Return, hide, cancellation and destruction restore the window.

The compositor must acknowledge ICCCM withdrawal before embedding. That handoff
uses a cancellable 25 ms timer and a two-second deadline; the GUI never waits in a
nested event loop. Native embedding uses Qt's connection for X save-set protection
so a host crash preserves the external app. Preview redirection is released while
controlling; the paused backdrop uses raster rendering and restores the requested
GPU setting on return. An ownership difference in Qt 6.2 required QPointer guards
around the foreign-window wrapper. Pop!_OS native visual checks caught the blank
surface produced by embedding before window-manager withdrawal completed.

Validation: all 20 regular desktop CTests pass with real CLI integration; the new
native-only test skips offscreen and passes separately on X11. The disposable app
fixture verifies preview pixels/bounds, foreign parentage, visibility, native
focus and text input, detach, closed-app handling, and survival after forcibly
terminating its embedding host. The GPU flight fixture verifies shooting, actual
app pixels in the panel, renderer restoration, resizing, and hiding during a
pending handoff with capture/readback of the restored window. Native captures
were visually inspected; no user apps were selected or captured.

Receipts: `/tmp/dockpipe-graph/test-billboards-final-suite.log`,
`native-window-final.log`, `billboard-native-final.log`, and
`billboards-verified.control.png`. Qt 6.2 prints a BadAccess diagnostic while
setting the foreign window's event mask; the verified native input, visibility
and restoration paths pass despite that diagnostic. Arbitrary third-party apps,
popups, packaged platforms and non-X11 desktops are not qualified by this fixture.

The native builder now includes the optional X11 development dependencies;
Flatpak explicitly builds the unavailable stub without broader permissions.
The no-backend source syntax and Flatpak script syntax checks pass. No package
build or release publication was performed. The local launcher was rebuilt;
engine and package execution contracts are unchanged. Build products and test
images are local generated artifacts only.

### Billboard clarity and fullscreen (2026-10-14)

Previews now use bounded 1280 × 720 server-side scaling, smooth perspective
sampling and labels below the app content. The three-image retained pixel
budget is 10.6 MiB; capture remains two frames per second total and pauses during
app/editor use. Flight offers F11 and a visible fullscreen button. Connected
apps have their own fullscreen/exit control. Fullscreen hides surrounding
workspace chrome in place and restores the previous window state and visible
panels, preserving native app parentage throughout the transition. Escape while
flying exits fullscreen before exiting flight. Engine/package boundaries stay
unchanged.

Native disposable-app verification passed fullscreen entry/exit, actual visible
app pixels, retained embedding, chrome restoration, capture bounds, input and
crash recovery. Receipts: `/tmp/dockpipe-graph/fullscreen-native.log`,
`fullscreen-native-window.log`, and `fullscreen-hd.fullscreen.png`.

### Standalone cockpit and holographic selection (2026-10-14)

The launcher now has a standalone Cockpit destination. Seven bounded 3D planet
meshes navigate to Filesystem, Apps, Workflows, Containers, Settings, Machines
and Executions. Atmospheric entry loads one destination city and animates surface
descent. The cabin is projected 3D geometry with middle-drag head movement separate
from ship steering. Navigation and Ship controls toggle their HUD panels.
The existing OpenGL-backed painter remains optional; this is a bounded workspace
scene, not a streaming game engine. Editors and native controls stay front-facing.

Selected files and open windows raise a camera-facing sprite above their building.
A second selection opens the shared editor/native app panel. App hangars still
enter native fullscreen control. File excerpts read at most 8 KiB / 18 lines in
a cancellable helper with a 2.5-second deadline and reuse editor syntax definitions.
Binary/non-UTF-8 files receive an explanation; links and Git metadata stay excluded.
One 768 x 432 file image is retained. App capture keeps its existing opt-in,
two-captures-per-second total and three-stream limits.

Operational screens are borrowed and restored at the same stack index. Restoration
on hide is queued after Qt's stack transition; synchronous restoration deleted
Qt's pending target and was caught by ASAN. HUD position/size is stored locally.
Native apps launch through GIO desktop entries, with bounded, unambiguous
window-class matching. Optional local push-to-talk uses a fixed PocketSphinx grammar
and bounded audio lifetime. Optional SDL2 controller input is gated by focus and
visibility. Neither feature downloads dependencies or changes workflow execution.

Engine and package boundaries remain unchanged. Physical controller/microphone,
non-X11 app embedding and packaged-platform qualification remain outstanding.
Local build products, diagnostic logs and screenshots live under the ignored
launcher build directory and `/tmp/dockpipe-graph`; no publication is part of this
iteration.

Verification: the complete launcher build succeeds with two compile workers.
All 21 offscreen-capable CTests pass, including real CLI Git integration; the
native-only desktop app test skips there. The separate native fixture passes
hologram selection, app-building previews, real native control, fullscreen,
renderer restoration and window cleanup. Native screenshots were inspected and
used to repair short-window clipping and cabin field of view. Planet arrival and
cancellation, bounded text/binary previews, preserved drafts and HUD bounds pass.
The virtual SDL controller fixture passes mapped axes/buttons, focus loss and
disconnection; PocketSphinx's 12-command grammar/silence self-test passes without
opening the microphone. In the restricted test sandbox SDL udev discovery must
be disabled for the virtual fixture; native SDL init/quit passes without that
fixture-only setting. This does not qualify physical input hardware.

Receipts: `/tmp/dockpipe-graph/cockpit-release-build.log`,
`cockpit-release-suite.log`, `hologram-release-app.log`,
`cockpit-release-native.log`, `cockpit-final-controller.log`, and the corresponding
native PNG captures. Qt 6.2's existing XCB BadAccess diagnostic remains observable
during embedding, with the native visibility/input/restoration checks passing.

### Flight performance and black-space palette (2026-10-14)

Native 3840 x 2160 profiling on the RTX 3080 established that the city bottleneck
was CPU geometry/projection work despite the hardware viewport being active.
Building silhouettes now cache both detailed and distant meshes for the current
bounded world only. Conservative sphere/frustum rejection skips invisible
buildings, outward normals reject hidden solid faces, and fully onscreen polygons
bypass clipping. Moving hangar shutters retain their dynamic geometry and entry
behavior. Movement/arrival timers are precise 16 ms, active only during input or
a finite transition. Engine/package execution contracts are unchanged.

The same native fixture measured these CPU draw costs (milliseconds per paint):

| Scene | Before median / p95 | After median / p95 |
| --- | --- | --- |
| 150 buildings | 14.52 / 16.41 | 4.72 / 5.30 |
| 500 buildings | 27.72 / 31.16 | 7.59 / 8.69 |
| Seven planets | 13.63 / 17.53 | 8.48 / 11.92 |

The fixture measured 53.4 moving paints/second at 4K and zero idle repaints over
its settled idle window. These are local fixture results, not a universal FPS
guarantee. Its separate presentation sample intentionally requests two paints;
33 ms for that sample is not a 30 FPS render measurement. Native receipt:
`/tmp/dockpipe-graph/perf-before-native.log` and `perf-cadence-native.log`.
The graph regression also passes lazy paging, scope/link boundaries, cancelled
reads, hangar navigation, editor routing and repeated-cycle cleanup.

Orbit now uses a black background with seven cached atmosphere sprites. Their
1.75 MiB retained image budget avoids full-resolution blur, extra simulation or
an idle animation loop. The file/app surface palette remains readable.

Final validation: the rebuilt launcher passes all 21 offscreen-capable tests with
real CLI integration; the native-only window test skips there. The separate
disposable-app fixture passes holograms, embedded control, fullscreen, GPU
restoration and cleanup. Receipts: `/tmp/dockpipe-graph/perf-final-build.log`,
`perf-final-suite.log`, `perf-native-app.log`, and `black-space-native.log`.
The existing Qt 6.2 XCB diagnostic remains, with native behavior checks passing.
No workflow/engine semantics, package dependencies or live user app state were
changed by this iteration. Generated profiling logs and captures remain local.

### Cockpit pointer selection and return portals (2026-10-14)

Mouse selection now casts from the clicked viewport position rather than always
using the center reticle. Space/controller selection retain the center ray. First
selection raises the bounded camera-facing file/app hologram; a second opens the
existing editor/control surface. Selection hints remain visible with HUD panels
closed, and app selection reveals the experimental capture opt-in when disabled.

Back button, Backspace and controller Back share one-level navigation. Folder
worlds expose a return gate in the clear arrival apron; planet surfaces expose a
sky gate inside the smallest world's flight bounds. The gate opening and its
complete label are clickable. Segment/plane crossing handles physical traversal
without tunnelling at boost speed. Returning cancels travel, shots and preview
reads, keeps file drafts, and loads only the parent world. Each portal has a fixed
32-segment rim; no new idle timer, capture stream or retained world was introduced.

Validation receipts are local under `/tmp/dockpipe-graph/return-*`: native GPU
cockpit captures, disposable-app capture/control/restoration, the full launcher
CTest suite, and focused checks for off-center file selection, nested/empty
folder returns, button/keyboard/portal paths, sky-gate flight and retained drafts.
Engine, workflow and package execution semantics remain unchanged. No publication
or user application capture is part of these fixtures.

### Native app handoff recovery (2026-10-14)

A reported ChatGPT control failure was a blank HUD with the app returned to the
desktop; both processes survived. The existing XCB BadAccess diagnostic also occurs
in successful native fixtures and is not sufficient to identify the failure cause.
A disposable Chromium app window successfully embedded and survived restoration.
ChatGPT-specific successful control is still unverified after this repair.

The launcher now confirms native attachment through a bounded parent-chain check
before announcing connection, with a two-second connection deadline. The existing
500 ms app-health tick detects later detachment. Failures restore the desktop
window, resume flight, retain a preview error separately from capture status, and
log handoff phases without window titles or content. Capture limits and explicit
app opt-in remain unchanged. Renderer preference survives hidden-control cleanup;
GL viewport recreation is deferred until the view is shown when necessary.

Native fixture coverage includes deliberate desktop reclamation of an embedded
app, preserved process/window, removal of the blank HUD, retained hologram and GPU
restoration. Backend tests also cover input, app closure and host-crash save-set
recovery. Receipts and disposable screenshots use `/tmp/dockpipe-graph/app-recovery-*`.
No live ChatGPT window was moved or captured during these tests. Engine/package
execution boundaries remain unchanged.

### Cockpit HUD confinement and native previews (2026-10-14)

Native app attachment now also synchronizes the foreign window's position and
size to its actual HUD container, using host-relative coordinates and device
pixel ratio without forcing another native widget. Panel moves/resizes and the
existing 500 ms health check correct restored desktop geometry. This addresses
the case where host ancestry alone passes while the app draws outside the HUD.
ChatGPT-specific control still requires the user's retry; no live user app was
moved during verification.

Containers, Machines, Executions, Workflow studio and Settings now preview a
single bounded 768 by 432 snapshot on first selection, with no page reparenting
or monitor activation. The second selection opens the original page in the HUD.
Workflow buildings preview bounded YAML before opening the studio, and installed
apps show a launch card before launching. Cockpit preferences reuse SettingsDialog
as an embedded widget, including queued completion and normal settings persistence.

Proof: full build and 21 offscreen CTests passed, with the native-only backend
test skipped there and passed separately on X11. Native backend coverage forces
incorrect geometry and verifies corrected size/origin, input and restoration.
The native HUD fixture passed fullscreen, resize, detachment and renderer recovery.
The real MainWindow preferences preview/open/cancel lifecycle passed separately.
Native desktop cockpit runs encountered an input-sensitive return-portal assertion;
the complete offscreen cockpit fixture passed, including all new preview cases.
The first native HUD attempt also stopped at first-selection state; its completed
build rerun passed. Receipts and inspected snapshots are under
`/tmp/dockpipe-graph/hud-preview*`. Engine/package execution remains unchanged.

### Persistent custom spatial workspaces (2026-10-14)

The existing cockpit now owns a bounded `SpatialWorkspace` configuration, with
atomic, conflict-checked version 1 persistence through `ContextStore::configDir`.
`SpatialWorkspaceEditor` owns explicit draft editing, `CockpitWorkspaces` coordinates
navigation and catalogue resolution, and `SpatialInteriorGeometry` plus
`WorkspaceFlightInterior` reuse the existing projection/input/capture infrastructure.
Model data contains no commands, native handles, captured contents or automatic
launch policy. Presentation themes and room geometry are independent of application
references; no plugin SDK or website interpreter was added.

The phase sequence implemented custom planet management/themes, positioned building
styles and entrances, rectangular first-person interiors, application wall placement,
and lifecycle/persistence polish. Normal navigation has a direct workspace selector;
manual flight and skippable approach remain available. Native app interaction reuses
`DesktopAppWindow`/`FlightAppBillboards`; repeated references share up to three streams,
visibility gates the existing capture rate, and exit does not own the app process.
Renderer recovery raises navigation controls above the replacement viewport and
returns focus after releasing native control.

Focused model and offscreen UI tests pass. Native disposable fixture proof includes
HUD keyboard input, shared captures, stopping capture while facing away, exit during
control, preserved process, missing-app handling, separate-process layout readback
without capture/launch, hidden-scene inactivity, and inspected native screenshots.
The local voice grammar self-test passes with 14 commands. Native runs needed an
explicitly reviewed host operation because the sandbox cannot connect to X11.
One native attempt exposed fixture focus loss; another prompted the renderer/focus
recovery repair. GPU per-process graphics memory was unavailable from the host query.
Physical controller qualification and ChatGPT-specific control remain unproven.

Receipts: `/tmp/dockpipe-graph/spatial-*`. Product usage, storage, capacity and
platform limits are documented in the launcher README. Full-suite terminal proof
is recorded with the completion receipt. Engine/package/workflow execution semantics
and protected checkout state remain unchanged; no commit or publication is authorized.

Terminal proof: full launcher build at `-j 2` passed. CTest reported 22 passed,
zero failed and two default skips; the Git test passed with its explicit fixture
CLI. The native spatial fixture and existing HUD regression passed separately;
unchanged native backend proof is admitted from the preceding delivery. At the
configured scene capacities, native p95 frame time measured 18.2 ms for 24 displays,
18.7 ms for 64 buildings and 27.7 ms for 31 planets. The complete fixture peaked
at 166108 KiB RSS; hidden scenes recorded zero paints and captures. These are host
fixture measurements, not incremental memory costs or universal frame-rate claims.
Detailed commands, scope and limitations: `/tmp/dockpipe-graph/spatial-validation.md`.

### Filesystem flight pacing and folder autopilot (2026-10-14)

Filesystem hangars now accept double-click travel through a bounded overhead route,
reusing the existing directory reader on arrival. Space skips travel; Escape,
focus loss and hiding cancel it. Failed arrival preserves the current world.
Flight uses sub-millisecond movement timing, convex face drawing and coalesced
mouse-look repaint requests. No engine, package or filesystem write behavior changed.

The launcher and focused test targets built successfully; graph, cockpit and spatial
cockpit suites passed. A separate native GPU folder-travel fixture passed arrival,
cancellation, missing-directory failure and hide cleanup. At 3840 x 2160 on this
host, the finished synthetic straight-flight fixture measured 60.1 FPS, 18.55 ms p95
and 19.03 ms maximum frame intervals, with zero gaps over 25 ms and zero idle paints.
This is bounded fixture evidence; the user's reported in-app stutter still needs
interactive confirmation. The broader native graph suite stopped at its existing
label-pixel assertion before folder tests; that assertion was not changed.
Receipts: `/tmp/dockpipe-graph/filesystem-flight-*.log`.

### Starship cockpit and orbital solar system (2026-10-14)

The cockpit now has a structural canopy, angled instrument deck with live radar,
navigation and flight-vector readings, and a responsive destination command deck.
Planets follow bounded circular orbits around a central sun; outer destinations
have longer periods. Lighting follows the sun direction. Orbital motion is a
persisted preference, pauses for travel/editing, and stops while hidden. Autopilot
turns into its approach and takes an elevated route when the sun blocks it.

`FlightSolarSystem` owns the orbit model; `FlightPlanetArt` owns fixed shared
procedural art; separate orbit and cockpit painters share `FlightProjection`.
The image cache is bounded to 6.1875 MiB, independent of world count and flight
duration. Orbital paths have 64 segments per destination. There are no downloads,
per-frame image allocations, or new engine/package execution semantics.

The full launcher build and CTest run passed (22 passed, zero failed, two default
skips: project-git and desktop-app-window). Orbit period/radius, sun avoidance, timer lifecycle,
command deck and navigation checks passed. Native GPU spatial fixtures measured
5.479 ms p95 CPU draw and 17.440 ms p95 frame interval for 31 planets, with zero
hidden paints/captures. A separate native 4K straight-flight fixture measured
59.9 FPS, 18.72 ms p95 and 18.98 ms maximum intervals, with zero gaps over 25 ms.
These are synthetic host measurements; interactive confirmation of the reported
stutter remains necessary. Offscreen orbit, surface, compact and controls previews
were inspected; they do not prove native desktop capture behavior. Receipt:
`/tmp/dockpipe-graph/bridge-validation.md`. No commit or publication was performed.

### Native control return and room previews (2026-10-14)

App control now uses a native HUD attached directly to the top-level host, bounded
to the flight viewport. It retains the flight's existing GPU widget rather than
replacing it, and avoids turning the nested cockpit and its ancestors into native
windows. Fullscreen preserves the app overlay. Release retries preview capture
within the existing failure budget and returns focus to the unchanged scene.

With App control enabled, room entry connects up to three assigned apps for
previews when each has exactly one matching window; selection is still required
for input control. Ambiguous windows remain disconnected. No app launches on room
entry. Apps-planet open windows occupy the front rows with up to three wall
previews. Out-of-view capture pauses; hiding, leaving or disabling control releases
streams. Showing the enabled scene reconnects its previews. Limits remain three
1280 x 720 frames and two captures per second total.

The new native handoff fixture uses a disposable GPU app and a nested launcher
layout. It verifies previews before control, repeated Apps/room return frames,
stable GPU viewport identity, non-native cockpit ancestors, fullscreen, hide/show,
ambiguous-match suppression and closed-app recovery. Native fixture screenshots
show the room and its preview both before and after control. The original simple
fixture did not reproduce the user's black screen; actual ChatGPT compatibility
still requires interactive retesting. Final evidence is recorded in
`/tmp/dockpipe-graph/handoff-validation.md`. No engine/package contracts changed.

Validation completed: full build, 22 passing CTest checks (three default skips),
native GPU handoff suite, native spatial/input/lifecycle suite and native backend
preview/input/detach/crash-recovery suite. Hidden native scenes had zero paints
and captures. No commit or publication was performed.


### Optional Cockpit mode settings (2026-10-14)

Settings now has a Cockpit mode group with a master switch that defaults off for
new and existing installations. App control, orbital motion, controller, local
voice, spaceship visibility and hardware acceleration are nested and persisted
in the launcher's existing JSON settings. Child controls are disabled while the
master is off and their preferences survive disabling/re-enabling. Settings
commit atomically; cancel and failed saves leave active preferences unchanged.

The sidebar/View entry and the Files graph's flight entry are unavailable while
disabled. The launcher constructs the cockpit only when its enabled entry is
opened. Disabling it leaves the scene, releases app control/capture, stops voice
and controller input, and preserves file drafts and spatial/HUD layouts. Ship
controls retains interaction actions and links to the same application settings.
Normal 2D/3D graph navigation remains available independently.

Validation: full build; 23 passing CTest checks with three default skips
(project-git, desktop-app-window, native-handoff); the native GPU handoff fixture
also passed separately, including disabling the master during control, zero
capture while disabled and preview/control recovery after re-enabling. Settings
fixtures cover default-off, malformed boolean input, nesting, persistence and
cancel; launcher coverage checks lazy construction, hidden navigation and disabling
from embedded Settings. Graph coverage proves default denial and draft preservation
on disable. Disabled/enabled Settings screenshots were inspected. Receipt:
`/tmp/dockpipe-graph/cockpit-settings-validation.md`. Actual ChatGPT compatibility
and physical microphone/controller qualification remain interactive checks.
No engine/package contracts changed; no commit or publication was performed.

## Experimental spatial web worlds inside Cockpit

The cockpit-only Web destination now owns an optional Qt WebEngine browser and
an ephemeral interpretation of the current rendered document. It uses real page
loads, JavaScript and browser history; it does not import another browser's
session. The existing default-off Cockpit setting gates creation and teardown.
Web occupies the main cockpit view with an explicit return to flight. A native
start screen provides a real prefilled URL and an Explore action. Page loading
runs behind the start screen or previous world; geometry and signs publish before
image sprites stream in, without resetting the user's position on completion.

The first interpreter maps products, repository/folder/file links and generic
content links to market stalls facing a central aisle, and video pages to a seated
cinema. Stalls keep images at eye level behind counters, below their canopies.
Facing and approaching an entrance navigates its link; aisle movement, looking
in place and backing away do not activate it. The current world remains during
loading, followed by a bounded 420 ms phase animation when the next world is ready.
Failed or empty destinations preserve the current world. These are
procedural volumes with collision, occlusion and real navigation destinations.
Native signs contain extracted text; browser captures are limited to actual image
elements, not entire page/article panels. Playback, authentication and forms still
use the original Live page. This is an initial bounded interpretation, not a
universal semantic parser or generated reconstruction of merchandise.

The active world holds twelve destinations, twelve shared 512 × 320 textures
(7.5 MiB), and at most 96 primitives / 768 solid faces plus bounded polygon meshes. Geometry is built once per page;
movement projects and sorts visible faces, and idle worlds do not animate or
capture. The background browser is muted and freezes after image streaming. Navigation,
Live page and rebuilding reactivate it. Extraction visits at most 5,000 DOM
nodes, admits 96 KiB of metadata, has a 15-second conversion deadline and
invalidates callbacks on navigation, hide and teardown.

Local evidence is recorded in `/tmp/dockpipe-spatial-web-validation.md` after the
native fixtures and launcher regression checks. The fixture covers JavaScript
content, procedural layouts, walk-through navigation, back/forward, prices,
freezing, idle frames, capacity, collision/occlusion, cancellation, missing renderer
and master-disable teardown. A public GitHub topics page also produces repository
stalls. Amazon/YouTube sessions, codecs/DRM, physical controllers and
cross-platform WebEngine packaging remain unqualified. No engine or package
contracts change; the web worlds are not persisted into spatial workspace JSON.

The stall/approach follow-up is recorded in `/tmp/dockpipe-market-validation.md`.
Native checks cover eye-level placement, both entrance orientations, phase
cancellation, empty-page recovery and live GitHub repository entry. One offscreen
spatial-cockpit run segfaulted; a standalone rerun and three bounded repeats
passed. The intermittent failure is unresolved, not a confirmed fix.

The construction follow-up separates DOM observations, a typed content graph,
composition parameters, construction proposals and validation before rendering.
The same graph supports branching, loop, spine and layered flight arrangements;
geometry, theme and locomotion are independent. Primitive parts use hierarchical
translation/yaw/scale, collision and bound content surfaces. Route, occlusion and
budget validation reject bad proposals and retry a minimal spine. The shared-screen
cinema remains a specialized compatibility recipe. Real browser video is captured
into its centre screen; provider-specific playback remains a separate qualification.
See `packages/cockpit/docs/spatial-world-construction.md` for current
contracts, examples and explicit limits. Validation is recorded in `/tmp/dockpipe-composition-validation.md`: build and
JavaScript syntax checks pass, full CTest reports 24 passed / 4 skipped, the final
Cockpit/spatial/model checks pass, and the native browser fixture passes all four
arrangements, history, approach entry, video pause/resume, idle freeze and teardown.
A separate public YouTube video advances on the centre screen after forwarding
an explicit play gesture. This is one verified provider case; Amazon sessions,
DRM, physical controllers and cross-platform packaging remain unqualified.

The vehicle follow-up adds content-bound physical cars to ground web worlds.
Model-specific automotive cards and Vehicle/Car markup provide body type and
optional year; utility/signup links are excluded. The generic construction
compiler validates up to twelve vehicles against bounds, other cars, reserved
routes and source bindings. Five cached procedural body shapes are explicitly
approximate; manufacturer GLB/glTF import and exact model-year geometry are not
implemented. Enter/exit, parking, driver view, independent looking/steering,
full-footprint collision, braking, blocked exits and hide/reset ownership live
in the launcher scene, with no engine/package or persistence contract changes.

Validation is recorded in `/tmp/dockpipe-vehicle-validation.md`: warning-free
launcher/test builds; four focused Cockpit/spatial/model/vehicle tests passed;
final model/vehicle checks and native browser vehicle fixture passed. Native
Ford Canada homepage and F-150 page checks extracted listing identities/years,
entered using E, drove, parked and resumed after hiding. The native fixture
measured 67 frames in 1106 ms with scene draw p95 0.963 ms on this Linux host.
Physical gamepad input, other manufacturers and cross-platform rendering are
not qualified by these checks. Vehicles are transient and reset on world rebuild.

### Optional Cockpit package and page-driven composition (2026-10-14)

Moved the complete Cockpit implementation, renderer, filesystem reader, browser,
voice assets, construction rules and feature tests into `packages/cockpit`.
The desktop retains settings, a versioned native surface interface, an asynchronous
package manager and shared editor services. Its normal build has no Cockpit,
WebEngine or OpenGLWidgets link dependency. General source builds skip the native
bundle unless opted in; a profile template prevents publishing an empty resolver.

Settings first resolves an installed package, otherwise delegates HTTPS catalog
lookup and checksum-pinned installation to the CLI. Enablement waits for successful
module loading; failures leave it off with retry, and cancellation stops the child
operation. Disabling destroys active surfaces; native Qt/Chromium code remains
mapped until exit. The sole engine change is the generic resolver `assets-dir`
scope field. Linux runtime packaging includes browser libraries, resources, locales
and the child renderer. Public catalog publication and other platform bundles are
still separate, unperformed work.

Page categories now influence branch/aisle/loop composition, page path and semantic
structure seed variation, artwork proportions affect exhibit widths, and admitted
site colours influence accents. Bounded capture also recognizes SVG and decorative
CSS artwork. These extend the existing vocabulary, not arbitrary model generation.

Validation: package test hook passed (8 tests passed, 2 native-platform skips);
core-only build and its two package/settings tests passed; launcher integration,
world/vehicle model and filesystem cancellation tests passed. A native browser
vehicle fixture passed with the bundled renderer, about 60 FPS and 1.14 ms draw p95.
The core-only DEB built without a WebEngine dependency. A local HTTPS fixture drove
the actual Settings -> CLI download -> checksum/install -> resolver extraction ->
native surface create/destroy path successfully. That test exposed and fixed a
controller destructor emitting into an already-destroyed flight view; a dedicated
regression assertion now checks silent controller teardown. Generic scope and
catalog installation tests passed. No public package or release was published.
Receipts: `/tmp/cockpit-package-test.log`, `/tmp/cockpit-lifecycle-final.log`,
`/tmp/cockpit-graph-test.log`, `/tmp/cockpit-native-web.log`,
`/tmp/cockpit-install-smoke-final.log`, `/tmp/cockpit-deb-test.log`.

### Optional local semantic enrichment (2026-10-14)

Cockpit's package now owns a separately installed local ONNX helper: semantic
page/item matching combines construction features, and foreground extraction turns
suitable product images into cutouts. The existing compiler retains all destination,
geometry and route checks. Worlds appear before inference; late designs require an
entrance action after the user has moved. Images update without resetting the view.
The native renderer also gains cached contact shadows, directional shading and
outdoor haze. No engine or launcher dependency was added for this slice.

The CPU lane is the smaller default; CUDA is optional and reports its actual
backend. Installation pins model hashes, supports cancellation/retry, and is owned
by the package. Requests, queues, caches, geometry and child-process lifetimes are
bounded. Private page content stays local and inference caches stay in RAM.
See `packages/cockpit/README.md` for installation and model/licensing details, and
`packages/cockpit/docs/spatial-world-construction.md` for contracts and limitations.
This is semantic composition with a small part vocabulary, not unrestricted neural
world synthesis; depth reconstruction and generated/imported meshes remain open.

### Cockpit late-load compositor repair (2026-10-15)

The real launcher could stop displaying frames when Cockpit was first opened on
Linux/Qt 6.2. Its host backing store existed before the optional package loaded
Qt OpenGLWidgets, which registers the compositor factory. Linked Cockpit tests
missed this ordering. Linux builds using Qt before 6.4 now retain the small Qt
OpenGLWidgets support dependency in the shared host UI at process startup. This
is a compatibility exception to the earlier no-OpenGLWidgets boundary; Cockpit,
WebEngine and neural dependencies remain package-owned and lazily loaded.
Single-configuration builds now default to optimized Release unless the caller
selects another build type explicitly.

A native regression links only the launcher UI, displays its host before loading
the real package, and checks composition, movement and hide/show. Before the fix
it captured a blank window and 94 compositor failures; after it rendered the
solar system with zero failures. Local construction fell from 558 to 376 ms.
Straight flight measured 60 FPS, no frame gaps over 25 ms and zero idle paints;
the GPU neural Web fixture completed 12 cutouts while rendering 74 frames in
1215 ms (draw p95 1.14 ms). Six focused CTests passed; the native regression skips
under offscreen CTest and passed separately on the real desktop. This qualifies
the local Linux/Qt 6.2/NVIDIA path, not other platforms or every live website.
Evidence: `/tmp/cockpit-render-before.log`, `/tmp/cockpit-render-fixed.log`,
`/tmp/cockpit-render-fixed.png`, `/tmp/cockpit-flight-performance-fixed.log`,
`/tmp/cockpit-web-performance-fixed.log`, `/tmp/cockpit-render-regression.log`.

### Browser and theatre return lifecycle (2026-10-15)

Returning from Web could leave orbit visible but unresponsive: `openWeb()` set
the flight view's editor flag, although Web uses a stacked page rather than an
editor panel. Removing that redundant flag leaves flight suspension with its
existing hide/show lifecycle. Separately, hiding the browser parent did not mark
the live page child explicitly hidden, so returning to a stopped theatre could
wake Chromium. The browser now hides that child on exit and explicitly shows it
only when resuming Live page mode.

The native browser fixture reproduced both failures before repair. It now passes
two playing-theatre -> orbit -> theatre cycles, including fullscreen exit,
keyboard flight, orbital repainting, stopped media capture, mute/frozen state,
camera preservation and deliberate playback restart. Live page exit/reentry and
the existing navigation, cancellation and teardown checks also pass. All changes
remain inside the Cockpit package. The playback proof uses a local generated video,
not qualification of every streaming website or its DRM. Evidence:
`/tmp/cockpit-return-before.log`, `/tmp/cockpit-return-after.log`,
`/tmp/cockpit-return-fixed.log`.

### Spatial operations districts and machine laboratories (2026-10-15)

Cockpit now consumes the existing bounded ResourceCanvas snapshots through the
versioned launcher workspace interface (2.0). Containers become factories,
networks switchyards, and named volumes storage depots. Cables only follow real
network -> container attachments and container -> volume mounts. Dependency
edges are excluded from that representation. State pulses are explicitly not
traffic/throughput measurements. Executions use managed workflow/task/process
relationships in a mission campus. The Settings destination is removed; normal
application settings remain available.

Machines become paginated planets with enterable laboratory portals, a procedural
android, and an existing-machine-controls console. Android nickname, five finishes
and three chassis styles are local QSettings preferences keyed by hashed machine
identity, capped at 256 profiles. Pairing/access labels are not online checks.
The package owns construction, pagination, input and animation; host changes only
export generic cached resource data and route selection/lifecycle to existing
controls. No engine changes or new discovery process is introduced by this slice.

Sectors contain at most 12 primary plus 8 connected resources, 64 cables, 96 solids,
24 displays and 12 sign images. All source resources remain reachable by paging;
omitted connection counts and discovery notices are visible. Polling is 1 Hz,
pauses while hidden, and defers scene reconstruction during movement or open
controls. Status refresh preserves camera position unless newly blocked. Leaving
operations releases monitoring ownership; opening its controls retains it.

Validation: ten focused CTests passed (cockpit, spatial-operations, spatial-cockpit,
spatial-intelligence, spatial-web-model, spatial-vehicles, workspace-graph,
spatial-workspace, workspace-package, execution-explorer), plus remote-launcher
integration. Final spatial-operations passed after extending coverage to walking
through the lab entrance. Fixtures cover multiple network attachments, rejected
missing/dependency edges, storage links, oversized snapshots and sector bounds,
controls return, movement during updates, hidden/exit polling shutdown, and saved
android appearance. Native operations passed and its district/lab/android captures
were visually inspected. An initial widget/framebuffer capture omitted Qt's GL
viewport; the isolated native fixture now captures its topmost window. The blank
frame check allows the deliberately small flat-shaded indoor palette. The native
browser fixture passed playing-theatre/orbit/input return twice. Native late module
loading reported zero compositor failures (31 ms load, 244 ms construction,
18.71 ms frame p95 for its orbit/flight fixture). These are local fixture results,
not live Docker/remote-machine or every streaming-provider qualification.

Evidence: `/tmp/cockpit-operations-regression.log`,
`/tmp/cockpit-operations-host.log`, `/tmp/cockpit-operations-final-model.log`,
`/tmp/cockpit-operations-native.log`, `/tmp/cockpit-operations-factories.png`,
`/tmp/cockpit-operations-lab.png`, `/tmp/cockpit-operations-android.png`,
`/tmp/cockpit-operations-browser-regression.log`,
`/tmp/cockpit-operations-package-rendering.log`.

### Return from Live page without rebuilding (2026-10-15)

The spatial browser's Live page button now becomes Return to space whenever an
existing scene can be resumed. The return changes visible surfaces only; it does
not re-extract content, publish geometry, reset the camera, or reload the page.
Navigation and history within Live page suppress automatic projection until an
explicit Build space or spatial navigation action. The retained world remains
available even when the website has navigated elsewhere. Spatial-only controls
are disabled while using the original page.

Theatre handoff stops capture without pausing or seeking the website's video.
Returning attaches capture to the current page and observes its actual playback
state without forced autoplay. The existing bounded timeout handles unavailable
video. This preserves website pauses and current time, while the existing exit
lifecycle still stops capture and freezes the page when returning to Cockpit.
All implementation and tests remain package-owned; no launcher/engine contract
changes were needed.

Validation: rebuilt cockpit-test/native package; cockpit, spatial-web-model and
spatial-vehicles passed. The native browser suite passed repeated Live page/space
handoffs with unchanged DOM marker, camera, URL and publication count; continuous
media time; a website-controlled pause; navigation to a different live page with
no automatic rebuild; and a subsequent explicit rebuild. Existing theatre ->
orbit -> flight input -> theatre checks passed twice. Tests use local browser/video
fixtures, not live YouTube/DRM qualification. Evidence:
`/tmp/cockpit-space-toggle-build.log`, `/tmp/cockpit-space-toggle-regression.log`,
`/tmp/cockpit-space-toggle-native.log`. The running user window was not restarted.


### Arcade flight modes and physical canopy (2026-10-15)

Added package-owned ArcadeFlightDynamics, independent of scene drawing and head
look. Explorer provides soft acceleration, bank/level assistance and hover
settling. Pilot provides higher speed, coasting, local quaternion pitch/yaw/roll,
boost and brakes. F or the visible flight-mode button switches modes; Z/C rolls
in Pilot and X brakes. Controller Y switches, X brakes and D-pad left/right
rolls, preserving the existing stick/trigger/boost bindings. Focus loss, hiding,
editing and controller disconnect halt dynamics. Walking and car handling remain
separate. Mode selection is per view and starts in Explorer.

Projection, ray picking and the surface horizon now follow full craft attitude;
head look is a separate relative rotation. Movement samples at most eight world
units apart prevent high-speed tunnelling through the existing clearance-expanded
colliders. Integration is bounded to six substeps and 50 ms per update; idle
Explorer views stop scheduling flight frames after settling. The hull has a
closer beveled canopy with blue trim, recessed consoles, amber local radar and
actual velocity/mode/heading/altitude instrumentation. No engine or launcher SDK
contract changes were needed.

Validation: build succeeded, nine focused CTests passed across flight dynamics,
Cockpit, operations, spatial worlds, intelligence, Web model, vehicles, graph and
workspace state. Desktop-app-window skipped offscreen; that native handoff was
not requalified in this slice. New tests cover frame-rate-independent travel,
coasting, braking, boost bounds, invalid elapsed time, inverted flight, assisted
leveling, thin-wall collision, mode controls, rolled display picking, independent
head look, focus/hide cancellation and idle rendering. Existing immediate-stop
fixtures were updated for intentional damping; portal navigation now supplies a
normalized local thrust direction.

The isolated native late-package-loading fixture passed with zero compositor
failures: 29 ms package load, 242 ms construction, 89 movement frames over the
1.5-second sample and 18.73 ms p95 frame interval. The native local-browser suite
passed Live page/space state retention and playing theatre -> orbit -> flight ->
theatre twice. These are fixture results, not live website or physical controller
qualification. Raster cockpit captures were inspected at 1440x900 and 900x650,
including independent head look. Evidence: `/tmp/cockpit-arcade-build.log`,
`/tmp/cockpit-arcade-tests.log`, `/tmp/cockpit-arcade-retest.log`,
`/tmp/cockpit-arcade-native-rendering.log`, `/tmp/cockpit-arcade-native-web.log`,
`/tmp/cockpit-arcade-{orbit,compact,headlook}.png`. The initial regression log
contains the two old-input-contract failures; the retest records their passing
replacements. The user's running preview was preserved; restart to load the new
package. No commit or publication was performed.

### Cockpit Git Planet local review (2026-10-15)

The optional Cockpit package now has a Working Changes review route using the
existing spatial composition/compiler and native source editor. Runtime-owned
project Git exposes explicit staged, unstaged and combined comparisons, streamed
content identities, file staging/unstaging, confirmed discard and exact-index
commits. Conventional Project Git shares the request client and keeps its saved-file
commit semantics. No cockpit world logic or heavyweight dependencies entered the
engine. [Capabilities and bounds](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/cockpit/docs/git-review.md)
document this local vertical slice; PR providers and branch/commit gauntlets remain
unimplemented. Validation uses disposable repositories, including the fifteen-file
acceptance fixture, rather than modifying the active user checkout.

Local validation: 14 `TestWorkspaceGit*` cases passed; eight focused Qt CTests
passed after updating both built-in/custom-planet count expectations. The browser
CTest intentionally skips offscreen; its native GPU local HTTP/theatre fixture
passed, including playback/live-page state preservation and orbit return twice.
The Git review GPU fixture also passed and produced inspected spatial/focus
captures under `/tmp/cockpit-git-native-*`. Current receipts are
`/tmp/cockpit-git-go.log`, `/tmp/cockpit-git-ctest.log`,
`/tmp/cockpit-git-final-ctest.log`, `/tmp/cockpit-git-spatial-final.log` and
`/tmp/cockpit-git-web-native.log`; early CTest logs retain the superseded count
failures. CLI, launcher and package built successfully. No live PR provider,
physical controller, non-Linux host or full repository suite was qualified.

### Cockpit furnishing layer (2026-10-15)

The optional Cockpit package now owns a separate furnishing overlay: 55 catalog
items, surface targeting, green/red placement ghosts, keyboard/mouse and controller
commands, move/rotate/duplicate/delete, and atomic per-environment persistence.
Custom rooms, spatial websites, operations campuses/labs and Git review rooms have
durable layout keys. Generated content remains separate from user layouts.

Placeable screen variants share source bindings. Current providers offer text,
local file/image previews, and aliases to existing scene producers; independent
browser, terminal and new application bindings remain future providers. Existing
app/theatre/Git displays are adapted for editing while retaining their content and
activation identities. The Add display action is replaced by Furnish room; legacy
application-reference editing remains for compatibility. Details and limits are in
[the furnishing guide](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/cockpit/docs/furnishing.md).

Launcher and package built successfully. Seven focused CTests passed, including
the new furnishing fixture, Git review, flight dynamics, operations and spatial
model/room regressions. Native GPU furnishing and local browser/theatre fixtures
passed, including live-page/playback retention and repeated orbit returns. The
isolated native app fixture passed control/release, capture, adapted source reuse,
permission and restart checks; its XCB BadAccess warnings did not fail the checks.
Hidden scenes recorded zero paints/captures. Physical controller, arbitrary live
websites, full repository tests and non-Linux hosts remain unqualified.

Evidence: `/tmp/cockpit-furnishing-build5.log`,
`/tmp/cockpit-furnishing-ctest.log`,
`/tmp/cockpit-furnishing-native-{furnishing,spatial,app,web}.log`, and inspected
`/tmp/cockpit-furnishing-{raster,native}-{placement,input}.png` captures. The user's
preview was preserved. No commit, catalog publication, engine dependency or new
heavyweight runtime was introduced by this furnishing slice.

### Site-wide Spatial Internet (2026-10-15)

The optional Cockpit package now owns bounded anonymous site discovery, semantic
source classification and persistent site worlds. Robots rules, nested XML
sitemaps, HTML navigation, canonical references, JSON-LD and breadcrumbs feed a
bounded graph of sections and sources. Relevance decisions, duplicates, utility
and version-navigation exclusions are inspectable and overridable; access rules
cannot be overridden. Restricted/noindex/noarchive records retain their decision
and URL without retaining source excerpts. Excluded sources cannot be entered
through the discovery inspector.

The existing document/world compiler renders section entrances, source exhibits,
attributed excerpts and related links. Stable IDs, saved placement slots and a
fixed entrance preserve locations across updates. Twelve destinations per area,
progressive same-origin images, atomic bounded caches and cached local neural
features reuse the existing rendering and inference infrastructure. One bounded
background interpreter handles HTML; cancellation discards stale results and
shuts down owned work. A large fixture demonstrated UI event processing during
interpretation rather than blocking the UI thread.

World overview, searchable Map / discovery, relevance/page/depth controls,
source/section overrides, refresh, update-area and clear-cache actions live in the
existing Web surface. Source pages retain the existing single-page interactions;
Live page / Return to space preserves the current world and camera. No renderer,
heavy dependency, engine behavior or package-acquisition contract was added.
Details, supported hierarchy and intentional limitations are in
[the site-world guide](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/cockpit/docs/site-worlds.md).

Launcher/package builds passed. The package test runner passed five Python tests
and fourteen CTests; three desktop-only checks skipped under offscreen Qt.
Subsequent focused site-world, web-model and intelligence tests passed after the
final policy/UI corrections. Native GPU fixtures passed site overview → section
→ source → overview, a locally served live-page round trip, hide cancellation,
background interpretation and excluded-source enforcement. The existing native
browser/theatre fixture passed playback/camera retention and repeated orbit
returns. Public HTTPS transport was checked against example.org, and a bounded
three-page docs.python.org pass produced a persisted semantic graph; live-site
coverage is intentionally narrow, not arbitrary-site qualification.

Evidence: `/tmp/cockpit-site-build-qualified.log`,
`/tmp/cockpit-site-package-final.log`, `/tmp/cockpit-site-qualified.log`,
`/tmp/cockpit-site-native-qualified.log`, `/tmp/cockpit-site-native-web.log`,
`/tmp/cockpit-site-public-fetch.log`, `/tmp/cockpit-site-live-docs-final.log`, and
inspected raster/native captures under `/tmp/cockpit-site-*`. Test caches and
package test state used temporary directories. Full repository, physical
controller and non-Linux qualification were not run. No commit, publication or
user-preview relaunch was performed.

### 2026-10-15 — Civic Spatial Web spaces

Cockpit's optional package now owns a furnished Dockpipe Industries HQ starting
hub, a grand cinema with a raised balcony/lifts and usable seats, and a connected
street with a drone boarding pad. Cinema posters still control the existing
shared screen. Website/live-page toggling retains the current world. Search
results can form a bounded city of storefront entrances using the existing
passage loader. Google/Bing/DuckDuckGo/Yahoo recognition is deterministic; it
does not bypass website consent or access restrictions.

The HQ logo is the unmodified company-site source asset; its provenance and hash
are recorded in `packages/cockpit/resources/branding/README.md`. Reused catalogue
furnishings and cached window meshes stay inside existing scene limits. A small
transient action model supplies seats, lifts, drone boarding and navigation;
raised floor support preserves walking below the balcony. No renderer, runtime
dependency or generic engine capability was added.

Validation: eight focused suites passed, including site worlds, furnishing,
input, flight, spatial cockpit, civic scenes, web models and vehicles. The native
browser fixture passed search interpretation, slow article corridors, fullscreen
video, live-page/world toggling and theatre/orbit reentry. Civic coverage includes
bounds, clear lobby paths, seat immobility/standing, screen framing, wall
occlusion, balcony support/lifts, drone return and city-to-cinema passages.
Evidence: `/tmp/cockpit-civic-tests.log`, `/tmp/cockpit-civic-final-tests.log`,
`/tmp/cockpit-civic-browser.log`, `/tmp/cockpit-civic-browser-final.log`,
`/tmp/cockpit-civic-startup.log`, `/tmp/cockpit-civic-build.log` and inspected
raster captures in `/tmp/cockpit-civic-captures/`. Scene-draw timings in the native
fixture are not end-to-end frame-rate or arbitrary-site guarantees. Physical
controller and non-Linux testing were not performed. The new work was not
committed, published or relaunched into the user's preview.

### 2026-10-15 — Shared Spatial Internet media recovery

The Hawaii reproduction established a browser-runtime failure: Qt 6.2.4 /
Chromium 90 could not parse the site's class static initializers, leaving its
logo and photographs as one-pixel placeholders even in Live page. Cockpit now
requires a matching Qt 6.8+ launcher SDK; a separate ignored local build at
`src/app/tooling/dockpipe-launcher/build/modern-web` uses Qt 6.10.3. The SDK and
XCB cursor dependency are local to the build, without replacing system Qt.
The package bundler selects the configured SDK's qmake and rejects mismatches.

The shared image pipeline rejects one-pixel placeholders, rotates through pending
images within one ten-second budget, activates selected lazy/responsive sources,
and rebinds uniquely identified images replaced during hydration. Images update
existing exhibits without moving the camera. Site-wide discovery now preserves
CDN image query strings and reads JSON-LD, Open Graph and lazy/responsive sources.
Only images referenced by accepted content may cross origins; page crawling stays
site-scoped, with existing DNS pinning, private-network restrictions, cancellation
and aggregate budgets retained. Redirects preserve the original image identity.

The newer Qt also exposed reentrant OpenGL viewport installation on show events;
setup is now deferred and guarded. Robots percent-encoding normalization is
compatible with both old and new Qt behavior. All changes remain in the optional
Cockpit package apart from this delivery record; no generic engine changes.

Validation: eight focused suites passed in `/tmp/cockpit-media-final-tests.log`.
The full native browser regression passed in
`/tmp/cockpit-modern-browser-final.log`, including fullscreen video, live/world
state retention, theatre/orbit transitions and slow corridor cancellation.
The real Hawaii cold-load probe captured three source photographs into eight
spatial exhibits; Live page loaded twenty of twenty-three image elements.
Evidence is `/tmp/cockpit-hawaii-modern-cold.log`, the inspected live-page capture
`/tmp/cockpit-hawaii-modern.png`, and its inspected `.exhibit.png` source texture.
The dedicated image fixture passed in `/tmp/cockpit-image-fixture-final.log`.
Late package loading into an existing host passed with zero compositor failures
in `/tmp/cockpit-modern-package-rendering.log` (89 movement frames in 1.5 seconds;
fixture timings are not an arbitrary-site frame-rate guarantee).
This is Linux/native and controlled-fixture proof, not universal site or codec
compatibility. The newer build needs a fresh launch; the user's existing preview
was preserved. No commit or publication was performed.


### 2026-10-15 — Native city directory and civic refinement

The HQ reception, street directory and Search city button now open a native
Cockpit search terminal over the resident scene. Query submission prepares the
result street in the background; Escape cancels and returns to the world, and
fullscreen stays active. Empty queries, loading and unavailable results have
native feedback. The ordinary address bar remains compatible.

Live Google investigation found two parser failures: article-only readiness
checks waited for an H1, and opaque /goto result links were rejected as search
chrome. Search readiness now accepts result headings, committed query identity
allows provider session parameters, and opaque navigation URLs retain the visible
source citation without inventing destination URLs. Pasted search URLs bypass
site crawling. Controls and links remain isolated in the existing WebEngine.

Civic façades now have larger wings, glazing, cornices and illuminated marquees;
the cinema adds ceiling ribs, bright coves and sixteen usable seats. These remain
stylized cached geometry, not dynamic lights. Cinema: 95 solids / 488 vertices;
maximum city: 69 solids / 480 vertices. Arrival faces the first result instead of
an empty alley. Package and renderer boundaries are preserved; no new dependency.

Validation: focused site-world, civic and model suites passed (site-world in
/tmp/cockpit-city-focused.log, model in /tmp/cockpit-city-model-final.log, final
civic in /tmp/cockpit-civic-reviewed.log). The full native controlled browser
regression passed in /tmp/cockpit-city-browser-final.log, including opaque search
fixtures, playback/fullscreen, history, streaming, timeout and cancellation.
The final live native-terminal test in /tmp/cockpit-search-walkthrough.log created
eight rainforest search storefronts and walked through a result corridor into the
Rainforest Alliance article's twelve-exhibit park. Fullscreen terminal and city
captures were visually inspected at /tmp/cockpit-search-fullscreen.png and
/tmp/cockpit-search-city-native.png; civic captures are in
/tmp/cockpit-civic-reviewed/. This is bounded Linux/native and representative
live-provider evidence, not universal search-provider compatibility. Build outputs
remain in the ignored modern-web directory. No commit, publication or user-preview
restart was performed.

### 2026-10-15 — Article prose versus gallery notices

The reported Brown Bear exhibit came from the site reader's empty-description
fallback flattening a leading gallery into source text. Package-only extraction
now prefers distinct prose paragraphs and excludes semantic gallery controls,
captions, credit containers and hidden duplicates. The live DOM reader applies
the same selection with bounded text traversal; ordinary prose about copyright
remains eligible. Live pages and crawl restrictions are unchanged. Cached text
from the previous extractor refreshes without losing stable placements or user
overrides.

The modern Qt launcher/package build passed in
`/tmp/cockpit-article-extraction-build.log`. Site-world and spatial-web-model tests
passed in `/tmp/cockpit-article-tests.log`, including malformed gallery markup,
duplicate prose and legacy-cache refresh. The public Brown Bear HTML downloaded
to `/tmp/dockpipe-brown-bear-source.html` produced article prose in the source
probe instead of gallery notices. Native browser regression passed in
`/tmp/cockpit-article-browser.log`, including gallery/prose selection, retained
illustrations, fullscreen playback, navigation, bounded loading and teardown.
JavaScript syntax and diff whitespace checks passed. This verifies the reported
markup and controlled browser cases, not every site's gallery structure. No
engine changes, commit, publication or user-preview restart were performed.

### 2026-10-15 — Restore browser desktop hints after screen control

Reproduced the extra desktop frame with a disposable frameless native client:
`_MOTIF_WM_HINTS` decorations changed from zero to fourteen after release
(`/tmp/cockpit-native-restore-repro.log`). Cockpit's DesktopAppWindow now snapshots
six bounded ICCCM/EWMH properties before withdrawal and restores them, including
original absence, after Qt wrapper teardown and before mapping the desktop client.
The restoration includes sizing, decorations, type, transient owner, desktop and
window state. Package and engine boundaries are unchanged.

The modern launcher/package build and deterministic native test passed; evidence
is in `/tmp/cockpit-native-restore-build.log` and
`/tmp/cockpit-restore-fixture-final.log`. Regression coverage includes frameless
decoration/sizing equality, repeated releases, fullscreen, input, escaped clients,
host-crash survival and asynchronous app closure. Native room/app handoff checks
passed in `/tmp/cockpit-restore-room.log`; a returned room frame was inspected at
`/tmp/cockpit-restore-room-room-return-1.png`.

An isolated Brave profile with local animated video passed decoration and sizing
constraint restoration in `/tmp/cockpit-brave-restore-passed.log`. Earlier probe
attempts exposed fixture window-class/color assumptions and Chromium's changing
preferred coordinates; those were corrected without weakening decoration checks.
The temporary Brave profile cleanup raced browser writes after shutdown (recorded
in that log); the native assertions passed. This was a separate test profile,
not the user's signed-in session or the exact Qalzar room. No user app was moved,
restarted or closed, and no commit or publication was performed.

### 2026-10-15 — Arcade cabinets, usable furnishings and interior occlusion

Added the package-owned arcade cabinet with independent main-screen app/scene
bindings and static image/text marquee artwork. Both bindings survive save,
reload and duplication; the cabinet consumes two of the existing 24 screen slots.
Action-button entry fits the existing native app handoff inside the projected
cabinet screen at a fixed first-person pose. Leave arcade/controller Back restores
the standing position and desktop client. Native HUD placement now coalesces
resize events until the flight viewport has its final dimensions. Spawned seating
families use existing bounded Seat actions, with rotated sit/stand and removal
coverage. The 32-action capacity is enforced during placement and regeneration.

Interior painting now caches a bounded BSP rather than sorting whole faces only
by centre depth. This prevents long walls and balcony surfaces painting over
nearer furniture at oblique angles. Split faces preserve their original image and
text coordinates. Static geometry edits invalidate the cache; moving placement
previews and vehicles traverse it without rebuilding it. Depth, fragment, build
work and moving-split limits retain complete fallback leaf content on exhaustion.
This extends the current Qt painter renderer; no engine changes or dependencies.

Build evidence: `/tmp/cockpit-arcade-build-final.log` (with the preceding package
compile in `/tmp/cockpit-arcade-build6.log`). Twelve offscreen regression suites
passed in `/tmp/cockpit-arcade-regressions.log`; the four affected suites passed
again after final edits in `/tmp/cockpit-arcade-complete-regressions.log`.
Native RTX 3080 furniture, cinema, action-button arcade handoff and local browser
fixtures passed in `/tmp/cockpit-arcade-complete-{furnishing,spatial-civic,native-handoff,spatial-web}.log`.
The native tests check projected-screen pixels after resize, repeated release,
closed-app recovery, fullscreen video, and return-to-world behavior. Fixture
captures now use their own compositor pixmaps: desktop-overlap captures and direct
QGraphicsView viewport framebuffer grabs proved unreliable. The final cabinet
and cinema-angle images were visually inspected under `/tmp/cockpit-arcade-complete*`
and `/tmp/cockpit-cinema-order-complete/`.

The first native Git invocation lacked DOCKPIPE_BIN and skipped. With the required
repo-local CLI supplied, `/tmp/cockpit-arcade-complete-git-profile.log` passed all
15 changed-file fixtures and fullscreen/focus transitions: 400 walking frames,
4.117 ms draw p95 and 18 ms frame-interval p95 (100 ms maximum observed interval).
Physical controller feel and arbitrary third-party app compatibility still need
hands-on verification. No user application was selected for control, no preview
was restarted, and no commit or publication was performed. Generated build output
remains in the existing modern-web build directory; test artifacts are in `/tmp`.


## RetroArch window discovery (2026-10-15)

The native window picker now falls back to bounded ICCCM `WM_NAME` metadata
when a client does not publish `_NET_WM_NAME`. RetroArch's running Steam window
exhibited exactly that case; no emulator-specific matching or engine changes
were needed. Modern UTF-8 titles retain precedence and both reads stay bounded
to 1 KiB. Native fixture checks cover legacy discovery, precedence, absent and
oversized titles, capture, input handoff, repeated detach and crash recovery.
The initial native run failed its existing keyboard-input assertion; the bounded
retry passed the full suite (`/tmp/cockpit-retroarch-discovery-retest.log`).
The optional package and tests built in modern-web. A read-only capture of the
user-selected RetroArch window produced its actual menu at 1280×720
(`/tmp/cockpit-retroarch-live.png`). A temporary modern-title property enables
discovery in the already-running preview without restarting either app.
DockPipe fullscreen behavior remains under investigation separately.

Fullscreen follow-up: the native arcade fixture now checks actual screen pixels
when connecting in fullscreen and when entering fullscreen during control, plus
return to the saved standing position. The isolated arcade run passed all four
windowed/fullscreen cycles and closed-app cleanup in
`/tmp/cockpit-retroarch-arcade-only.log`; its fullscreen image was inspected.
Two broader native-handoff runs failed in older room restore/attach checks before
reaching the arcade, so they are not reported as passing. The room-preview assertion
now waits up to two seconds for the existing reduced-rate preview timer. The
`DOCKPIPE_NATIVE_ARCADE_ONLY` test switch isolates cabinet checks without weakening
the default test path. RetroArch-specific DockPipe-fullscreen behavior remains
unresolved pending a live reproduction; no running preview was restarted.


## Application window replacement recovery (2026-10-15)

Cockpit app streams now retain their stream index and cabinet playing pose while
an application recreates its native window. Replacement lookup uses the existing
bounded X11 inventory at the 500 ms stream cadence, with a five-second deadline.
Only one new normal window with the same application class, PID and Linux process
start time is eligible. Pre-existing windows, other processes and ambiguous matches
are excluded. The existing native attachment path then fits the new window to the
cabinet; failure releases control with a reason. Visible passive previews also
recover. Manual reconnection reuses a dead display's stream slot instead of
exhausting the three-stream budget. All implementation remains in packages/cockpit.

The modern-web package and native test executables built. The initial RTX 3080
fixture passed window replacement and fullscreen screen-pixel checks in
`/tmp/cockpit-window-replacement-native.log`. Later shared-desktop adverse tests
were unstable while the desktop was actively used, so final verification moved
into an unmapped Xephyr display with a temporary Openbox extracted under /tmp;
no system package was installed and no game process was controlled. The isolated
software-rendered X11 suite passed fullscreen replacement, preview replacement,
five-second exhaustion, ambiguous matches, different-process rejection, existing
sibling rejection, stream-slot reuse and closed-app cleanup:
`/tmp/dockpipe-reconnect-test-tools/replacement.log`. The full low-level native
window regression passed in that display (`window.log` in the same directory).
The offscreen furnishing regression passed in
`/tmp/cockpit-window-replacement-furnishing.log`; whitespace checks passed.
The isolated display and window manager were shut down after validation.

The running user preview was not restarted. Actual RetroArch game transitions
with this new build still need a user test after restart. No commit or publication
was performed. Build output remains in modern-web and diagnostic assets in /tmp.

## 2026-10-15 — Visual asset library and modular plots

Launched and verified the requested replacement-window build in the existing
`dockpipe-preview-newest` user service before starting this slice. RetroArch was
left running independently.

Cockpit now provides a searchable, categorized thumbnail grid in furnishing mode,
with the existing keyboard/controller selection and placement flow. Added 120
Kenney Furniture Kit CC0 meshes, preserving original diffuse material colors and
license/provenance. The package-owned converter is reproducible from the official
archive; generated model JSON and its resource manifest are intentional shipped
assets. No renderer, engine dependency, or runtime network fetch was introduced.
`SpatialAssetLibrary` returns existing `SpatialMesh` data for reuse by spatial-web
generation; existing web layouts are not automatically reskinned in this slice.

Custom workspaces can create saved empty outdoor plots. Habitat, sandstone and
timber kits add 21 floor/wall/door/window/roof/pillar/railing pieces, 100-unit snap,
90-degree rotations, repeated placement, and fixed-height roof placement with
keyboard/controller presets. Doorway collision leaves the opening traversable.
This is single-level construction, without stairs or multi-floor traversal.
Imported furniture uses conservative collision boxes and existing seat actions.
Per-environment budgets are 192 records, 768 collision parts, 8,192 furnishing
faces, and the existing 24 surfaces/32 actions; parsed models have an 8 MiB cache.

Modern-web build passed. Five focused CTests passed (furnishing, workspace storage,
spatial cockpit, civic worlds, input controls), including source-mesh validation,
empty searches, plot persistence, doorway traversal, adjacent tiles, scene-budget
exhaustion, repeated placement, and controller/keyboard roof-height changes.
Evidence: `/tmp/cockpit-builder-validation.log`. The furnishing suite also passed
in an unmapped Xephyr/Openbox display; no host-app input was sent. Inspected native
catalog and furnished-base captures under `/tmp/cockpit-builder-native-*`; fixed
light-theme contrast and removed distracting triangle/BSP outlines. This is
isolated software-rendered X11 proof, not a new hardware performance measurement.

The builder update is compiled for the next launch. The session opened earlier
was left intact while it was being used. Changes remain uncommitted; package/core
boundaries were preserved. Build output is under modern-web and test artifacts
are under `/tmp`.

## 2026-10-15 — Experimental voice opt-in

Moved local voice commands into Settings → Cockpit mode → Experimental. The
persisted gate is now `cockpit.experimental.voiceCommands`, false by default;
the former flat voice preference is deliberately not imported, requiring a fresh
opt-in. Talk controls are hidden and disabled while gated off, active capture is
cancelled, controller talk state is cleared, and delayed recognized commands
cannot navigate after disablement. Removed unconditional talk guidance from the
flight tooltip. Existing app-control preferences remain unchanged.

The modern-web launcher, settings fixture and Cockpit fixture built. Settings and
Cockpit regression tests passed (`/tmp/cockpit-voice-gate-tests.log`), covering
legacy-enabled reset, nested UI, explicit opt-in persistence, hidden controls and
late recognition rejection. Inspected the settings capture under
`/tmp/cockpit-experimental-settings-enabled.png`. No microphone capture or live
session restart was performed. Launcher preference/UI changes use the existing
Cockpit settings bridge; voice execution remains package-owned. No commit.

## 2026-10-15 — Dirt bike and Mesa Moto

Added package-owned `DirtBikeDynamics`, a fixed 120 Hz arcade bike with two
spring/damper contacts, traction, automatic lean, slope forces, gravity, rider
pitch, braking and recoverable crashes. Each update executes at most six steps;
focus loss/menu suspension cannot accumulate simulation catch-up. Bike control
uses the existing keyboard/controller routes, first-person view, interaction,
collision and recovery menu. Cars retain their existing drive path.

Mesa Moto is an outdoor loop with two outbound tabletops, a larger return jump,
three rolling jumps, pit pavilion, markers and scenery. It is reachable through
Web's Moto track button and an HQ entrance; the pit returns to HQ. The bike is
visible directly ahead at arrival. A queued browser-startup HQ initialization
cannot overwrite an explicitly selected local world.

`SpatialRideSurface` supplies matching ramp geometry, wheel contacts, walking
height and picking occlusion. It is bounded to 16 surfaces with eight profile
knots each. Construction plans validate bounds, slope, arrival/vehicle clearance
and budgets; streaming transforms contacts with geometry. Spatial Web's vehicle
vocabulary/extractor can admit explicit dirt-bike/motocross/enduro listings.
Automatic track layout inference is not introduced. No external physics engine,
new dependency, downloaded model or generic engine change was required. Details
and controls: `packages/cockpit/docs/dirt-bike.md`.

Validation: modern-web launcher and Cockpit fixture built. Seven focused CTests
passed (bike, cars, flight, civic scenes, web model, controller inputs, furnishing)
in `/tmp/cockpit-bike-tests-verified.log`; bike/car checks passed again after the
local-world startup guard and exit-status cleanup in
`/tmp/cockpit-bike-entry-tests.log`. Bike coverage includes both track straights,
launch/landing, timestep independence, braking, finite/bounded time input,
collision, occlusion, recovery, cleanup and generated-plan validation. An isolated
hidden X11/ software-rendered run passed including toolbar/HQ entry, with captures
under `/tmp/cockpit-bike-native-*.png` and log
`/tmp/dockpipe-reconnect-test-tools/bike.log`. Pit, rider, bike and overview images
were inspected. This is not physical-controller feel, GPU performance or
cross-platform qualification. Test-only capture/build artifacts remain outside
authored package assets. Existing live DockPipe/RetroArch sessions were preserved;
the new build is ready for a requested relaunch. No commit or publication.

### Mesa Moto presentation, sound and trigger controls — 2026-10-15

Upgraded the package-owned bike into a stylized motocross presentation: cached
spoked wheels, frame/engine/fork/body detail, a helmeted rider, steering and wheel
animation, suspension movement, bounded dust and a ground shadow. A smoothed,
obstruction-checked chase camera is the default; V / controller Y selects helmet
view. The continuous track ribbon includes wheel ruts, broad jump profiles,
landing runout, cliffs, pines and flags within existing ordinary scene budgets.
Bike/rider models have a 640-face cap; car models retain their former size.

RT and LT are independently normalized analog accelerator/service brake inputs;
X / Space applies a rear brake with reduced lateral grip. Widget routing retains
menu neutral guards for both triggers independently and clears inputs on loss of
focus. Tests caught a bottomed-out landing tunnelling through the floor; the
fixed-step solver now resolves the finite suspension stroke as a contact impulse.
Combined brakes stop at zero rather than reversing the vehicle.

Original local synthesis supplies engine, wind, tyre/gravel and landing sounds.
Optional SDL2 playback uses one bounded mono queue, pauses on suspension/exit,
supports M or the controller menu for mute, and degrades to silent riding when
no device opens. No sampled assets, new rendering framework, network dependency
or generic engine change. Source and tests remain in packages/cockpit.

Modern-web launcher and cockpit-test built with two jobs. Final seven focused
CTests passed in /tmp/cockpit-moto-tests-final.log, including bike, input-controls,
flight, cars, civic scenes, web model and furnishing. Audio tests cover finite
samples, missing devices and stop/restart using SDL's silent dummy driver. One
flight collision exact-velocity assertion failed during an earlier simultaneous
native/offscreen run; its isolated recheck and the final sequential suite passed.
The assertion was not weakened. Native X11 captures are generated by the hidden
nested-display harness under /tmp/cockpit-bike-native-*.png. No physical controller,
speaker listening test, GPU benchmark or other-platform qualification was performed.
The detailed controls and bounds are in packages/cockpit/docs/dirt-bike.md.
The existing user application was not relaunched, and no commit/publication occurred.
Final isolated native run also passed, including browser Moto track/HQ transitions;
the final chase-view capture was inspected. Its log is
/tmp/dockpipe-reconnect-test-tools/bike.log. Temporary screenshots and build/test
logs were generated outside authored assets.

### Controller visual spawn catalogue — 2026-10-15

The unified command menu's Build / spawn section now uses a large centered
thumbnail grid, backed by the existing FurnishingCatalog/FurnishingPreview data
and cache. D-pad and two-axis stick navigation move through the grid; LB/RB
cycle type filters, A selects for placement, B returns, Start closes. Keyboard
arrows/Q/E/Enter/Esc share the actions. Normal command sections remain lists.
The catalogue preserves category/model selection and fits the current viewport.
Fixed tile geometry prevents lazy image arrival from overlapping neighboring
cards. Only visible thumbnails are generated, six per event turn; failed previews
are marked attempted so they cannot create a timer loop. Existing input suspension
and placement-neutral guards remain in use. All implementation stays in Cockpit.

Validation: modern-web launcher and Cockpit test targets built; furnishing and
input-controls CTests passed (/tmp/cockpit-controller-catalogue-tests.log).
Coverage includes centered size, nonoverlapping tiles, horizontal/vertical
navigation, category filtering, lazy previews after scrolling, last-row bounds,
keyboard parity, resize containment, returning to the menu and normal placement.
The isolated native X11 input run also passed, and its final screenshot was
visually inspected (/tmp/cockpit-controller-native-catalogue.png). Physical
controller ergonomics remain for user testing. No live app restart or commit.

### Bike grip, load transfer and recoverable wipeouts — 2026-10-15

Refined the package-owned fixed-step solver with smoothed steering, approximate
front/rear load transfer, acceleration/braking pitch, and tyre friction budgets
shared between longitudinal and cornering forces. Only supported wheels provide
grip. Rear locking allows slides; sustained high-speed over-demand can wash out.
Clean jumps and ordinary low-speed bumps retain assisted arcade handling.

Typed crash causes distinguish fast obstacle collisions, excessive relative
landing speed, off-balance landing angle and sustained grip loss. A crash cuts
engine synthesis, preserves a bounded impact/slide sound, tips/slides the bike and
animates the rider falling beside it. The chase view temporarily shows the fall;
a cause/recovery banner appears. The wreck stops after 2.4 simulated seconds and
cannot respond to riding input. R / Start → Recover bike clears crash state and
returns to the original boarding point and selected camera. Pausing during a
crash stops both sound and animation; returning input resumes only the unfinished
fall. The rider fall is procedural, not an imported ragdoll physics runtime.

Implementation is limited to packages/cockpit physics, vehicle presentation,
input guards, synthesis and tests/docs; no engine or dependency changes. Native
build uses two jobs. Detailed tuning limits and controls are documented in
packages/cockpit/docs/dirt-bike.md. No running user app restart, commit or publish.

Validation: modern-web launcher and fixture builds passed. Controller, flight,
bike and car CTests passed (/tmp/cockpit-bike-crashes-tests.log); bike/car checks
passed again after wheel-contact grip refinement
(/tmp/cockpit-bike-crashes-contact-tests.log). Fixtures exercise every crash cause,
low-speed impacts, clean authored jump straights, weight transfer, bounded settling,
engine cut, ignored post-crash input, pause/resume and recovery camera restoration.
Native isolated X11 regression also passed and the rendered fall/recovery banner
was inspected; captures are /tmp/cockpit-bike-native-crash.png and sibling views.
The crash camera pulls back to frame both rider and bike. No physical-controller
feel or speaker test, GPU benchmark, or cross-platform qualification was performed.
Build logs and captures are temporary validation artifacts outside authored assets.

### Motocross materials, sculpted jumps and powertrain — 2026-10-15

Expanded the existing Cockpit renderer with bounded built-in surface materials,
world-aligned perspective texture projection, screen-footprint mip selection,
shared tint/brush caches and consistent UV/LOD across BSP fragments. Bundled two
verified CC0 Poly Haven photographs (1.6 MB total); provenance/checksums live in
packages/cockpit/resources/materials/README.md. Other patterns and the articulated
bike/rider remain original procedural geometry. No engine/core changes, new
rendering framework, external physics library or runtime asset download.

Mesa Moto now uses textured ground, rock slopes and shaped jump shoulders. Four
monotone profile samples per knot span produce the same triangle surfaces for
rendering and wheel/walking contact. The ordinary scenery stays within 512
vertices / 256 faces (254 faces used). Sculpted surfaces have a separate bounded
2,240-face worst case across sixteen maximum-size profiles. The bike/rider uses
1,039 faces under a 1,280-face vehicle limit; cars keep their former geometry.
Rounded tyre profiles, raised tread, tank/body panels and rounded rider geometry
replace several block-shaped parts. Closed model faces cull before ordering.

Handling adds six automatic gears with hysteresis/torque interruption, a throttle
response/torque curve, engine braking, progressive spring stiffness, separate
compression/rebound damping, rider weight/load effects and damped lean momentum.
Audio and HUD use the same simulated powertrain RPM/gear. The existing RT/LT/X,
keyboard, camera, crash and recovery controls remain applicable. This is still
an assisted model, not calibrated MX Bikes parity; no deforming ground or full
rider ragdoll is claimed. Details: packages/cockpit/docs/dirt-bike.md.

Profiling caught excessive texture variants, oversized tiny-face tiles and GL
brush overhead. Shader/context availability alone was insufficient performance
proof. Textured terrain now composes into one reusable raster layer for the
existing OpenGL viewport, at most 1280 x 800 / approximately 4 MiB. This bounds
full-screen compositing at the cost of some sharpness on larger displays; the
layer releases on world replacement/exit. Other scenes retain their direct path.
One warmed RTX 3080 framebuffer overview draw fell from approximately 90 ms in
the direct textured GL prototype to 18.59 ms with the compositor. These are
controlled draw measurements, not a mapped-window gameplay FPS guarantee.

Validation: modern-web launcher and cockpit-test builds passed with two jobs.
All eight final CTests passed (bike, cars, input-controls, flight, furnishing,
civic, spatial cockpit and web model): /tmp/cockpit-moto-fidelity-tests-final.log.
Coverage adds texture resource/mip validation, nondegenerate model faces, exact
sculpted contact at face centres, gear/RPM stability, engine braking and rider
weight. Existing clean jumps, all crash causes, recovery, pause/neutral controls,
audio lifecycle and single-page/web vehicle checks remain passing.

The isolated native X11 interactive fixture passed during the rendering pass.
Final host GPU visual qualification also passed with hidden, non-activating
windows and explicit GPU framebuffers; /tmp/cockpit-moto-gpu-test.log records the
RTX 3080 context, and /tmp/cockpit-bike-gpu-{pit,rider,overview}.png were generated.
The rider view was visually inspected. Ordinary QWidget grabs from unmapped GL
windows were blank, so the GPU fixture explicitly renders into an FBO and checks
nonblank pixels; it deliberately does not assert interactive wall-clock timing
for an unexposed window. Physical controller ergonomics, speaker listening,
long-session gameplay FPS and other platforms remain unqualified. The user's
running preview was preserved; no commit, push or release was performed.

### Moto camera look and optional terrain detail — 2026-10-15

Cockpit now separates bike view input from rider balance: right stick or
right-drag looks around in chase and helmet views, holds for two simulated
seconds after input stops, then smoothly recenters. Left-stick vertical carries
rider weight / airborne pitch; keyboard Up/Down retains that action. Camera
limits, return timing and pause lifecycle are package-owned. Recovery, boarding,
camera switching and world replacement reset the look; crashes retain their
existing camera. The parked-bike tick stays alive until the return finishes.

Start/F1 exposes Smooth/Sharp terrain detail while walking or riding on textured
terrain. Smooth keeps the original 1280 × 800 default. Sharp uses physical pixels
including HiDPI up to 2560 × 1600 (16 MiB). A forced resolution increase was
rejected after GPU measurements showed excessive cost. Final hidden RTX 3080
framebuffer checks measured 54.34 ms sharp versus 20.62 ms smooth overview draw
submission at a 2560 × 1600 viewport, not gameplay FPS. The detail selection lasts
for the current view session; changing it releases the cached frame layer.

Build targets cockpit-test and dockpipe-launcher passed with -j2. Four focused
CTests passed: dirt-bike, input-controls, flight-dynamics and spatial-vehicles
(23.36 seconds). Coverage includes hold/return timing, frame-rate independence,
invalid camera input, parked-bike return, menu suspension, steering isolation,
and controller selection of terrain detail. Hidden GPU checks passed in both
modes; captures are /tmp/cockpit-bike-sharp-{pit,rider,overview,smooth}.png, with
logs /tmp/cockpit-bike-camera-tests.log and /tmp/cockpit-bike-sharp-gpu.log.
The sharp rider capture was visually inspected. Physical controller feel,
long-session FPS and cross-platform rendering remain user/native follow-ups.
The running preview was preserved; all code remains inside packages/cockpit.

### Correct the low-resolution default — 2026-10-15

The user rejected the blurred Moto view: keeping reduced resolution as the
default and capping Sharp at 2560 × 1600 did not address their larger display.
This supersedes the preceding default/detail policy. Native is now the default
and paints directly into the existing viewport framebuffer, with no intermediate
image or 1280/2560-pixel cap. Start/F1 explicitly labels the optional fallback
"Resolution: Native → Reduced" and explains the softer 1280 × 800 output.
Existing camera, physics and source asset detail are unchanged.

The shared interior painter now uses drawConvexPolygon for triangles and
verified convex quads. Concave quads and larger polygons keep the general path.
This avoids unnecessary general polygon work without changing geometry/order.
A hidden RTX 3080 3840 × 2160 fixture measured native overview draw submission
at 98.12 ms before and 30.31 ms after this optimization; reduced mode measured
20.70 ms. These are controlled draw timings, not gameplay FPS. Native baseline
versus optimized image comparison had mean channel differences below 0.066/255
for the rider and 0.013/255 for the overview. The optimized rider capture was
visually inspected. Artifacts: /tmp/cockpit-bike-native4k-*.png and
/tmp/cockpit-bike-convex4k-*.png; corresponding *-gpu.log files record results.

cockpit-test and dockpipe-launcher built with -j2. All eight focused CTests passed
in 38.11 seconds (dirt-bike, spatial-vehicles, input-controls, flight-dynamics,
furnishing, spatial-civic, spatial-cockpit and spatial-web-model). Controller
coverage now asserts Native is selected by default and toggles both ways.
The GPU test asserts that default before capture and exercises a 4K viewport.
Validation log: /tmp/cockpit-native-resolution-tests.log. Physical-controller
feel, sustained gameplay FPS and other platforms are not newly qualified.
Changes remain package-owned; the current preview was not restarted or moved.

### Moto tyre steering, articulation and scenery — 2026-10-16

Steering now derives yaw torque from separate front/rear contact slip and grip
budgets. The front tyre initiates the corner and the rear follows; stationary
bar input does not rotate the chassis. Lean follows actual speed and yaw rate.
The fork, front wheel, fender and bars articulate around the raked steering stem;
rider arms follow the bars and upper-body lateral movement now reinforces the
inward lean. Existing fixed-step bounds, braking, crashes and recovery remain.

The package adds an original generated juniper cutout and a credited CC0
PolyHaven Sparse Grass diffuse image. Twenty-four shared foliage cards and
approximate flat alpha shadows replace cone trees. Admission inspects at most
32 placements; the shared foliage/shadow images use about 12 MiB. Photographic
surface tiles retain RGB variation within the existing cache. Helmet, torso
and arms gain detail, bringing the ridden model to 1,299 faces under a 2,048
per-vehicle cap. Source provenance/checksums are recorded in materials/README.md.
Native framebuffer resolution remains the default. This is an incremental
visual pass, not AAA parity: the rider remains procedural, foliage is billboard
art, and shadows are static approximations rather than terrain-conforming maps.

cockpit-test and dockpipe-launcher built with -j2. Eight relevant CTests passed
in 38.19 seconds; see /tmp/cockpit-moto-art-turn-final-tests.log. Coverage adds
stationary steering/settling, mirrored corners, inward roll and the fork pivot.
The final hidden RTX 3080 3840 x 2160 GPU run passed with a heading-change
assertion before capturing the moving corner. Rider and corner captures were
visually inspected: /tmp/cockpit-moto-art-{rider,corner}.png. Build/GPU logs are
/tmp/cockpit-moto-turn-visual-build.log, /tmp/cockpit-moto-corner-check-build.log
and /tmp/cockpit-moto-art-gpu.log. Draw-submission readings varied substantially
across runs (about 2–41 ms), so these are visual qualification, not a reliable
gameplay performance result. Sustained FPS, physical-controller feel and other
platforms remain unqualified. Generated build/test images stay outside authored
source; the generated foliage is an intentionally bundled package asset.
No engine changes, app restart, commit, push or release were performed.

### Collision-aware foliage placement and turning render cost — 2026-10-16

The user reported junipers intersecting signs and hills, visible brown collider
posts, and lag while riding/turning. SpatialFoliagePlacement now builds bounded
projected occupancy from admitted solids, mesh faces, ramps, displays, actions,
vehicles and spawn/exit routes. It reserves each tree's complete rotating canopy
plus 50 cm clearance and five metres around labelled landmarks. It admits only
flat-ground placements within room bounds/height. Mesa Moto plants last, after
structures exist, trying at most 65 deterministic nearby candidates per requested
tree; exhausted locations are skipped instead of forced. Scene entry repeats
the same admission. Furnishing placement respects the canopy envelope.

Tree trunk collision is derived from accepted foliage for walking and vehicle
queries, without submitting a visible box or leaving orphan colliders after
rejection. Every tree still shares the same original image and shadow image;
no asset duplication, extra tree textures or resolution reduction was added.
The renderer rejects fully off-screen moving polygons before BSP traversal and
static fragments before projection, preserving near-plane/edge crossings.
Depth sorting computes each centroid depth once instead of per comparison.

cockpit-test and dockpipe-launcher builds passed with -j2. Eight related suites
passed (38.27 s): dirt-bike, furnishing, spatial-vehicles, spatial-cockpit,
spatial-civic, spatial-web-model, input-controls and flight-dynamics. The final
sign-clearance change passed dirt-bike again (12.86 s). Tests cover the reported
sign/hill positions, deterministic relocation, canopy conflicts, rotated signs,
flat-ground/height/bounds rejection, invalid sizes, admission capacity, walking
and bike trunk collision, furnishing exclusion and frustum crossing cases.

Hidden native 3840 x 2160 RTX 3080 GPU checks passed; the final sign close-up
/tmp/cockpit-placement-clearance-placement.png was inspected and is clear of
trees/posts. Logs: /tmp/cockpit-placement-culling-tests.log,
/tmp/cockpit-placement-clearance-tests.log, /tmp/cockpit-sign-clearance-build.log,
/tmp/cockpit-placement-clearance-gpu.log. Images/logs are generated test artifacts
outside source. A 36-frame synchronized GPU turning sample measured median/p95
6.54/7.49 ms before the culling/sort pass and 6.26/7.42 ms after; another run was
6.12/7.28 ms. This is only a modest controlled improvement, not proof that live
lag is resolved. A corrected camera reset exposes the whole track in the
overview: about 27 ms draw submission remains a heavier view. Earlier overview
measurements could inherit camera yaw and point away from the course; those
are not comparable. Native gameplay/frame pacing still needs live retesting.
All changes remain in the Cockpit package and delivery docs. The running preview
was preserved; no restart, commit, push or release was performed.

### Moto grip, completed flips and spin-out rendering — 2026-10-16

The user reported excessive sliding, lag during spin-outs, and completed backflips
being classified as crashes. DirtBikeDynamics now allocates longitudinal/lateral
tyre forces together instead of letting engine force consume the rear tyre's
entire cornering budget. Steering retains full low-speed lock but asks for wider
turns at speed. Loss-of-grip crashes require sustained actual sideslip, not an
unmet force request. The controlled full-throttle/half-steering case fell from
1600.07 cm/s peak sideways velocity to 5.27 cm/s while retaining forward speed.
Deliberately holding the rear brake through a turn can still cause LostGrip.

Pitch is normalized each step and landing attitude is measured relative to the
slope modulo a full turn. A real full backflip over the authored tabletop now
lands without crashing; the same jump with insufficient rotation still crashes.
Hard impacts, inverted contact, obstacle crashes and recovery remain covered.
This is assisted motocross handling, not a calibrated MX Bikes-equivalent model.

Hidden native 3840 x 2160 RTX 3080 profiling separated bike updates from rendering.
Updates stayed near 0.5 ms p95. The material working set was 46.15 MiB against a
32 MiB cache: 618 misses included 447 repeated creations and about 491 ms of tile
creation across the qualification run. A shared hard 64 MiB limit retained the
same 171 unique tiles with zero repeated creations (about 205 ms initial work).
Tests verify image identity survives a 48 MiB working set and eviction still
occurs beyond the bound. No viewport/texture resolution reduction was introduced.
Vector clips replace per-frame scanline regions, and un-antialiased filled model
faces omit redundant same-colour outlines. Temporary detailed profiling was
removed; bounded last-bike-update timing and the opt-in GPU spin benchmark remain.

Builds of cockpit-test and dockpipe-launcher passed with -j2. Eight relevant
CTest suites passed in 38.43 s: furnishing, input-controls, flight-dynamics,
spatial-cockpit, spatial-civic, spatial-web-model, dirt-bike and spatial-vehicles.
Final hidden GPU qualification passed. Spin render median/p95/max changed from
11.51/17.14/19.29 ms before rendering changes to 10.26/13.76/15.01 ms in the final
build, with 0.49 ms p95 bike updates. An intermediate resident-cache run measured
10.21/13.57/14.04 ms. These are synchronized FBO render samples, not displayed
frame pacing or proof that all live stutter is eliminated. The full-track
stationary overview still costs roughly 25 ms draw submission.

Evidence: /tmp/cockpit-grip-final-build.log, /tmp/cockpit-grip-final-tests.log,
/tmp/cockpit-spin-grip.log, /tmp/cockpit-spin-cache.log,
/tmp/cockpit-spin-resident.log and /tmp/cockpit-spin-final.log. Native images are
/tmp/cockpit-spin-final-*.png; the final spin image and earlier corner image were
inspected. Generated logs/captures remain outside source. Package code/tests/docs
and this delivery note are the only authored areas for this slice; engine/package
boundaries and unrelated checkout work were preserved. No preview restart,
commit, push or release was performed. Live riding/frame-pacing retest remains.
