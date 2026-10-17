## First Slices

1. Define the app contract and launcher context payload.
2. Add a read-only run inspector over operation-result events, logs, and artifacts.
3. Add YAML-backed workflow/agent/MCP connector browsers with validation status.
4. Add diff/apply preview and approval rendering over the same CLI/master bridge protocol.
5. Add guided workflow/task-pack authoring that previews generated YAML before save.
6. Add the agent-docs map and TODO index view.
7. Add opt-in remote access setup for Cloudflare Tunnel, certificate/domain configuration, endpoint
   health, and teardown.
8. Add a Qt mobile client over the same app/server API for run monitoring, approvals, and lightweight
   workflow control.

## Pipeon Interim Direct-Agent Slice

Before ForgePipe owns the full control surface, Pipeon should expose a minimal, honest split:

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

- Decide where ForgePipe lives in the first-party package tree.
- Define the launcher context payload shared with Pipeon-style app launches.
- Decide which YAML contracts exist for MCP connectors and agent/task packs before building rich
  editors.
- Extend the initial `dockpipe.operation_event.v1` JSONL stream into the full ForgePipe run inspector
  feed, including logs, artifact references, approvals, and task graph state.
- Add a future graph visualizer for repo guidance and task contracts so ForgePipe can render
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
This is launcher execution/onboarding UX, not the ForgePipe workflow editor or an
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
ForgePipe agent/MCP authoring scope. Live canvas status, arbitrary wiring, richer
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
