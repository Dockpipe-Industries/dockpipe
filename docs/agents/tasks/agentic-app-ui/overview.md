# TASK-008 ForgePipe Agentic App UI

## Goal

Design and build ForgePipe, a standalone DockPipe-launched agentic app for creating, editing, running, and
inspecting DockPipe/DorkPipe workflows through a clean modern interface.

The app should make the YAML contracts approachable without replacing them. Workflow, agent, MCP,
model-lane, approval, and package contracts should still derive from durable YAML and package-owned
catalogs.

## Current Decisions

- Build ForgePipe as a standalone app launched by DockPipe, using the same launcher-context model as
  Pipeon.
- Treat the app as a control and inspection surface over DockPipe, not a second runtime.
- Keep YAML and package-owned catalogs as the durable source of truth.
- Use one shared local/server API for desktop Qt, Qt mobile, and web clients.
- Use the CLI/master protocol and operation-result stream for execution, approvals, logs, artifacts,
  and run state.
- Keep provider execution close to the DorkPipe orchestration stack: Codex and Claude lanes should
  run through resolver-backed `exec`/CLI workers and MCP/operation-result events, not through
  extension-local provider configuration.
- Keep Pipeon chat as a direct chatbot surface with explicit provider/model selection.
- Codex direct chat should use the normal host Codex CLI path (`codex exec`) so behavior matches
  other Codex surfaces, with Codex workspace sandboxing enabled and the host Codex config model
  used by default. Do not hardcode stale model aliases in the Pipeon layer.
- Pipeon chat sessions should map to durable Codex transcript IDs and use `codex exec resume`
  after the first Codex turn so direct chat keeps context across messages.
- Claude direct chat must not run raw unrestricted host Claude. It needs a guarded DorkPipe gate
  first: MCP/workflow authorization, bounded workspace access, then a Claude worker inside the
  controlled boundary.
- Provider authentication should be surfaced as explicit status/repair state, not discovered through
  failed chat runs. Pipeon should call `dorkpipe.provider_auth_status` before provider use and
  `dorkpipe.provider_auth_repair` for direct host login flows such as `claude auth login`.
- Escalate tougher work from chat/direct agents into YAML-backed DorkPipe workflows when the task
  needs planning, verification, multiple workers, provider comparison, approvals, cost/risk
  boundaries, or durable artifacts.
- Support remote access only as an explicit governed setup with approval, teardown, audit, and
  secret-reference handling.
- Provide diff, conflict, artifact, and repair review without becoming a full IDE.
- The Pipeon VS Code extension should remain a thin chat/run-inspection shell for now. Defer rich
  template and model-lane authoring to ForgePipe, with workflow YAML as the bridge until then.

## Product Shape

ForgePipe is a standalone application invoked through the same DockPipe launcher model as Pipeon. It
inherits execution context from the launcher: selected repo, workflow/package context, environment,
scopes, and runtime/session identity.

ForgePipe should be agentic, but not a full IDE. It should focus on governed workflow creation,
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

