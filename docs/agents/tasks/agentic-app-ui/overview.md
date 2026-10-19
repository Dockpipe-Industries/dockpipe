# TASK-008 DorkPipe AI Features in DockPipe

## Goal

Backlog: DorkPipe contributes AI workflow creation, editing, execution and inspection
features to the existing DockPipe application, CLI and unified MCP endpoint. DorkPipe
owns AI domain logic and package-owned views; DockPipe owns generic integration contracts.

The app should make the YAML contracts approachable without replacing them. Workflow, agent, MCP,
model-lane, approval, and package contracts should still derive from durable YAML and package-owned
catalogs.

## Delivery Priority and Models Direction

Latest user priority, 2026-10-17: work core-up. Deliver TASK-043 shared networking
and package/version lifecycle foundations, then TASK-039 unified MCP contributions,
then DorkPipe DORK-001 graph, knowledge and context features. Defer richer model
management, chat and AI workflow authoring. Those later features must not become
prerequisites for the first useful MCP and knowledge slices.

DorkPipe may contribute top-level navigation such as Models inside DockPipe using
reviewed generic contribution contracts; the exact labels and schema remain design
work. Model configuration and adapters belong to DorkPipe. Users choose supported
local, remote or hosted providers and execution targets through DockPipe runtime,
credential-reference, permission, resource and lifecycle contracts. Do not force one
provider, require a new model runtime, or start a container merely to show model features.
Knowledge indexing/full-text search and explicit graph/DAG operations remain useful
without model inference; optional semantic retrieval can add a selected model later.

## Chat and Editor Direction — Review Required

DorkPipe contributes a chat interface inside DockPipe for DockPipe-related AI work,
using the selected project, package versions, knowledge/context and user-selected
supported model/provider. CLI/API owns session operations, streaming, cancellation,
resume and results; desktop and editor are consumers. Context access is explicit;
chat does not grant tool execution or privileged approval automatically.

A DockPipe-branded VS Code integration should cover DockPipe and PipeLang, with
optional AI chat using those same DorkPipe operations. Pipeon retirement and companion
extension consolidation are proposed, pending compatibility and data-retention audit.
Do not resume Pipeon feature work or remove its published packages/extensions now.

These are backlog directions, not simultaneous implementation requests. Core networking/package foundations precede MCP and
knowledge. Chat, rich Models management and AI authoring remain deferred. [Review the direction and open decisions](direction-review.md)
with the user before implementation, migration, source retirement or release operations
under this revised plan. Existing release preparation is paused at its current checkpoint
until that review resolves any effect on its scope.

## Agent Bootstrap and Context

AGENTS.md should become a minimal local bootstrap; unified DockPipe MCP routes
knowledge/context to DorkPipe. The [proposed bootstrap contract](../../../workflows/agentic-design-notes.md#proposed-agent-bootstrap-and-knowledge-routing)
defines authority, source permissions, provenance and offline behavior. Knowledge
and decision management belong to DORK-001, not a duplicate TASK-008 database.
Audit and qualify migration before replacing the existing Markdown routing tree.

## Deferred Skills Management

DorkPipe DORK-002 owns a future Skills manager inside DockPipe, reusing curated sources,
target-neutral IDs and existing rendering/provider adapters. CLI/API owns catalog,
source/version/scope selection and safe host rendering; UI/MCP consume those operations.
See [the deferred audit scope](direction-review.md#deferred-skills-manager). Skills do not
grant runtime authority and this work does not gate networking, MCP or knowledge delivery.

## Deferred Browser Assistance

DorkPipe may add optional page summaries, annotated requests and explicitly
approved browser control through shared DockPipe operations. History access and
site permissions require separate, visible scope. The
[browser integration backlog](browser-integration.md) records contribution API
gaps, headless authorization and disable/uninstall acceptance. No AI implementation
is requested now. Ordinary browser settings remain DockPipe work; password storage
and autofill use compatible user-selected extensions.

## Current Decisions

- DorkPipe is the optional, independently versioned package owner. Do not create a separate
  DockPipe AI package, standalone AI application or mandatory AI service stack.
- Installation and activation use DockPipe package lifecycle and settings. Core remains
  usable without DorkPipe; CLI, desktop and MCP share authoritative domain operations.
- Ordinary local use requires no extra container, persistent daemon or separately managed
  database. Bounded isolated workers and explicitly requested remote services remain valid
  through DockPipe lifecycle, cancellation and permission contracts.
- This is unbuilt backlog work. Existing generic launcher editing and Cockpit features
  do not establish that DorkPipe AI contributions have been implemented.
- Keep YAML and package-owned catalogs as the durable source of truth.
- Reuse authoritative core CLI/API operations for desktop contributions. Optional future
  remote/mobile clients consume the same operations without becoming a local prerequisite.
- Use the CLI/master protocol and operation-result stream for execution, approvals, logs, artifacts,
  and run state.
- Keep provider execution close to the DorkPipe orchestration stack: Codex and Claude lanes should
  run through resolver-backed `exec`/CLI workers and MCP/operation-result events, not through
  extension-local provider configuration.
- Preserve paused Pipeon provider/session behavior as migration evidence only. Evaluate
  reusable adapters in DorkPipe; the new chat interface is within DockPipe.
- Surface provider authentication as explicit readiness/repair state through package-owned
  adapters. Reuse supported guarded provider contracts without hardcoding model aliases
  or treating conceptual commands as shipped functionality.
- Escalate tougher work from chat/direct agents into YAML-backed DorkPipe workflows when the task
  needs planning, verification, multiple workers, provider comparison, approvals, cost/risk
  boundaries, or durable artifacts.
- Support remote access only as an explicit governed setup with approval, teardown, audit, and
  secret-reference handling.
- Provide diff, conflict, artifact, and repair review without becoming a full IDE.
- Consolidate editor direction around DockPipe/PipeLang with optional DorkPipe chat;
  qualify identity/settings/session migration before retiring the companion extension.

## Product Shape

DorkPipe contributes views within the existing DockPipe application. The views
inherit the current DockPipe execution context: selected repo, workflow/package context, environment,
scopes, and runtime/session identity.

The package focuses on agentic workflow work rather than becoming a full IDE. It should focus on governed workflow creation,
execution, review, and iteration.

Primary jobs:

- create and edit workflows
- create and edit agent/task definitions
- configure MCP connectors and capability declarations
- inspect model lanes, budgets, and escalation policy
- run workflows from the current launcher context
- review approvals, diffs, artifacts, logs, operation results, and follow-up tasks
- map agent-facing docs, markdown guidance, skills, and TODOs into a navigable view
- optionally expose the same app/server through a governed remote-access setup

## Contract Rule

YAML remains the source of truth.

The app may provide rich editors, forms, graph views, previews, and guided flows, but saved durable
state should round-trip through:

- workflow `config.yml`
- package-owned catalogs
- agent/task YAML
- MCP connector YAML
- model-lane policy YAML
- repo-owned task packs
- docs/agents indexes and markdown guidance

App-local state is acceptable for drafts, UI layout, selected views, cached metadata, and transient
chat context. It should not become the only durable definition of a workflow, agent, connector, or
approval policy.


## Product naming and ownership

ForgePipe is the native game/spatial engine in [TASK-037](../forgepipe/overview.md).
The former ForgePipe AI-app backlog remains TASK-008 as DorkPipe-owned AI authoring
and inspection scope. The earlier separate “DockPipe AI” package/application direction
is superseded by the user's 2026-10-17 decision. DorkPipe's extracted repository owns
implementation and delivery; this task retains scope and generic integration coordination.

[TASK-039](../unified-mcp-gateway/overview.md) owns generic CLI/UI/MCP contribution
contracts. DorkPipe's DORK-001 owns knowledge, graph and context functionality; these
are complementary feature scopes within the same package, not separate AI applications.
Selected package versions must bind command, view and tool contributions consistently.
Do not import DorkPipe source back into core or require sibling-source builds.

Pipeon is hidden and paused. Its interim features remain historical context, not a
prerequisite or instruction to resume development. Existing container-backed behavior
needs a compatible transition plan; this decision does not authorize deleting user data
or retiring current integrations before replacements are qualified.
