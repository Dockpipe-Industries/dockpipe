# TASK-014 Native Dev Container Discovery And Lifecycle

## Goal

Let Pipeon recognize a repository-owned `.devcontainer` definition and offer a governed way to
prepare, start, attach to, inspect, and stop that environment. The same lifecycle must be available
through a CLI/MCP contract so Pipeon is a consumer of the capability, not a second Dev Container
runtime.

## Current Context

The package-owned `packages/ide/resolvers/devcontainer` resolver now discovers standard, legacy, and
direct root repository definitions; fails closed on multi-definition selection; reports normalized
status from captured adapters; verifies live `read-configuration` through pinned Dev Container CLI
`0.87.0`; and defines an approved managed `up` contract through fixtures.

Pipeon already owns a separate local stack and provider-pool lifecycle. Native Dev Container support
must compose with that stack without treating the Dev Container as Pipeon's private state or silently
replacing a user's existing container session.

No live `up`, lifecycle hooks, `exec`, attach/editor action, stop/remove, reconciliation, Pipeon
consumer, or provider-pool/runtime integration exists yet. The managed `up` slice proves only the
approval, adapter-result, ownership-record, and event contract.

## Remaining Questions

- What is the bounded cross-platform live `up` adapter and reconciliation contract after the current
  pinned read-only CLI verification and fixture-only managed result?
- How do Docker Compose-based definitions, features, mounts, `remoteUser`, forwarded ports, and
  rebuild requirements map to bounded DockPipe operation-result events?
- How should Pipeon expose readiness, build progress, logs, container identity, attach targets, and
  repair actions while preserving the CLI as the execution authority?
- Can the DorkPipe provider pool safely use a ready Dev Container as a declared execution location,
  or must provider workers and the Dev Container remain separate until an explicit resolver contract
  exists?

## Product Shape

1. Discovery and status are package-owned, read-only, deterministic, and available through the
   package workflow and generic MCP execution path. Finding a definition never starts it, and
   multiple definitions require explicit selection.
2. The managed `up` contract requires explicit intent, risk-bound approval, a pinned adapter result,
   and exact ownership evidence. Live execution, rebuilding, stopping, or attaching remains a future
   governed action using the same operation-result contract.
3. The CLI/MCP surface remains package-owned, provider-neutral, and lifecycle-oriented. Do not add a
   `dockpipe devcontainer` engine subcommand or guess among multiple definitions.
4. Pipeon will consume that contract. The first UX only offers availability, selection, and status;
   `Use Dev Container`, logs, attach/open, rebuild, and stop arrive only with matching CLI/MCP
   operations. It stores only UI selections and drafts locally; the repository's `.devcontainer`
   files remain the durable source of truth.
5. The lifecycle operation returns an opaque environment/session reference plus normalized state and
   artifact/log pointers. It does not expose raw Docker or Dev Container command payloads to other
   app layers.

## Safety And Boundary Rules

- Keep Dev Container-specific resolution, CLI integration, and Docker behavior package/resolver
  owned unless research identifies a genuinely generic DockPipe primitive.
- Never auto-run a discovered configuration. Builds, pulls, feature installation, Compose changes,
  stop/remove, rebuild, and host-editor launch require explicit intent and applicable approval.
- Respect the user's existing containers and labels. Do not stop, remove, or rebuild a container
  not proven to belong to the selected definition and requested DockPipe session.
- Do not copy repository contents into a Pipeon volume when the Dev Container contract already owns
  workspace mounting. Do not infer editor attachment state from unsupported host process heuristics.
- Treat secrets only as existing Dev Container references or governed secret references; never read
  or serialize resolved secret values into Pipeon state, artifacts, or events.
- Keep Pipeon UI, CLI, and MCP on one structured event/approval contract. No extension-only
  lifecycle implementation or durable Pipeon-specific Dev Container configuration.

## First Research Deliverables

- Compatibility matrix for Dev Container CLI, Docker Desktop, Docker Compose, and host editor
  attachment across supported host platforms.
- Inventory of the existing `packages/ide/resolvers/` flows, Pipeon stack lifecycle, and their
  overlap/conflicts with repository-owned `.devcontainer` definitions.
- Proposed normalized lifecycle state machine, operation-result schema, approval classes, ownership
  labels, and cleanup/recovery rules.
- CLI/MCP contract proposal with multi-definition selection and non-interactive fail-closed behavior.
- Pipeon UX wireflow showing discovery, explicit start, progress, attach, error/repair, and teardown.
- A minimal vertical-slice recommendation with tests that use fixture Dev Container definitions and
  no live image pull by default.

## Open Decisions

- The first live `up` adapter and reconciliation design. Read-only configuration currently uses the
  pinned installed CLI; direct Docker remains limited to captured status evidence and must not become
  an alternate lifecycle implementation.
- Whether a started Dev Container becomes an eligible generic workflow runtime/resolver target or is
  initially limited to Pipeon/editor attachment and explicit CLI exec.
- Recovery for a live start that creates resources before its managed record is persisted. The
  decided managed-stop policy retains its record; stop implementation, remove/down, and automatic
  recovery still require separate authorization.
- Which editor attachments are supported first: VS Code, Cursor, Pipeon code-server, or a
  container-only status/exec surface.

