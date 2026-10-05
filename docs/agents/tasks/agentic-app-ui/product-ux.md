## Launcher Integration

The app should launch from DockPipe using the same context-passing approach as Pipeon.

Expected launcher context:

- current repo/workspace
- selected workflow or package, when present
- active session/run identity, when present
- artifact root, operation-result stream, and event projection path, discoverable through
  `dockpipe get event_log`, `dockpipe get event_index`, workflow `dockpipe scope`, and
  `dockpipe session inspect --json`
- allowed scopes and access policy
- available resolver/model lanes
- MCP connector availability

The app should be able to start a new run or attach to an existing run from that context.

## Remote Access And Web App Mode

The same app/server should be able to run as a web-accessible control surface when the user opts in.
The desktop Qt app, Qt mobile app, and web app should speak to the same local/server API instead of
creating separate products.

Supported setup options should include:

- local-only desktop app
- local or remote Qt mobile app
- local web app bound to localhost
- Cloudflare Tunnel for remote access without directly opening inbound ports
- Let's Encrypt certificate setup when serving through a user-controlled host/domain
- free starter domain or subdomain flow where practical
- bring-your-own-domain setup with DNS guidance and verification

Remote access setup is security-sensitive and should be treated as a governed operation:

- show exactly what will be exposed before enabling it
- require explicit approval before creating tunnels, DNS records, certificates, or public endpoints
- emit operation-result events for tunnel creation, DNS verification, certificate issuance, server
  start, endpoint health, and shutdown
- support disable/teardown as a first-class operation
- persist remote-access config in YAML or package-owned config, never only in UI state
- keep secrets as references, not plaintext tokens in repo files

The remote web app should use the same approval, event, artifact, and operation-result contracts as
local CLI/app runs.

Mobile should follow the same rule. A Qt mobile app can provide a compact control and monitoring
surface for runs, approvals, artifacts, logs, and endpoint status, but durable workflow and connector
state should still come from YAML/package config through the shared API.

## Runtime UX

When a workflow runs, the app should show richer information over the same CLI/master protocol:

- live operation-result timeline
- rebuildable operation-event index summary from `dockpipe runs events --index`
- current stage, worker, and task graph state
- logs by operation/task
- artifact browser
- approval prompts and decisions
- model lane usage and budget state
- verifier findings and repair suggestions
- apply/publish status

The UI should render the same structured events emitted by CLI runs. It should not require a
different execution path.

## Authoring UX

Authoring should expose YAML contracts through purpose-built views:

- workflow graph and stage editor
- agent/task definition editor
- prompt and context editor with source/read/write policy visibility
- MCP connector editor
- model-lane and budget policy editor
- approval/apply/publish policy editor
- package workflow browser
- schema diagnostics and validation
- generated YAML preview before save

The app should make it easy to switch between guided UI and raw YAML for advanced users.

## Extension UX Scope

Until ForgePipe owns richer authoring, the Pipeon VS Code extension should:

- expose a provider/model selector for direct chat
- route Ollama chat through DorkPipe MCP and the local stack
- route Codex chat through host `codex exec` with Codex workspace sandboxing and default host
  config model selection, resuming the same Codex transcript for each Pipeon session
- route Claude chat only through a guarded DorkPipe sandbox gate; do not run raw Claude directly
- route complex or risky work to existing workflows or YAML handoff instead of making direct chat
  behave like a multi-agent reasoning system
- show provider auth readiness before starting guarded workers, with host auth repair actions kept
  separate from workflow/container execution
- keep run inspection over DorkPipe artifacts/events
- expose workflow YAML handoff/export when users need to inspect the routing contract
- avoid extension-local model-lane or workflow-authoring state that cannot round-trip through
  workflow/package-owned contracts

Future ForgePipe work can add rich model-lane and workflow authoring once it is backed by durable
YAML, package catalogs, validation, and CLI/MCP execution.

## Review UX

The app should support reviewing changes without becoming a full IDE:

- generated diff preview
- apply preview by target file
- conflict detection and merge-conflict guidance
- accept/reject/retry/repair controls for generated changes
- artifact-to-diff traceability
- checklist of required artifacts and verifier status
- final summary suitable for commit/PR notes

Git merge conflicts should be surfaced clearly with enough context to resolve or defer, but deep
code editing can remain in the user's normal editor.

## Agent Docs Map

The app should provide a map from agents/workflows to markdown guidance:

- `AGENTS.md`
- `docs/agents/index.yaml`
- routed docs under `docs/agents/`
- workflow/package README files
- task-pack docs
- TODO index and linked TODO markdown
- rendered skills and provider-facing guidance

This view should explain which guidance a run will load and where durable agent knowledge should be
updated after the run.

