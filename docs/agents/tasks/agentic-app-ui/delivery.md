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
