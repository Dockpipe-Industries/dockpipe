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
- Decide how much editing happens in-app versus handing off to the user's normal editor.
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
