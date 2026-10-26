# Artifacts and external MCP

Read when using generated context or package-owned MCP tools. Human reference:
[maintainer artifacts](../../runtime/artifacts.md).

## Artifact rules

- Treat `bin/.dockpipe/` and `.dorkpipe/` reports as read-only grounding unless the
  user asks to refresh them. Identify stale or missing evidence rather than
  silently regenerating it.
- Compare the recorded source identity with the current work when it affects a
  conclusion. An old handoff, approval or successful run is not fresh authority.
- Use `cwd: artifacts` or `dockpipe scope artifacts ...` for workflow outputs.
  Durable continuity state belongs to its package owner; disposable caches and
  reproducible products belong in artifact or package-runtime scope.
- Keep generated outputs, credentials and resolved configuration out of commits.

## MCP ownership

The existing MCP bridge, tiers and analysis integrations are DorkPipe-owned.
Read [its MCP guidance](https://github.com/Dockpipe-Industries/dorkpipe/blob/master/docs/agents/runtime/mcp-and-analysis.md)
and inspect actual tool capabilities before relying on them. Access to a tool
is not permission to perform every operation it supports.

The proposed [unified core gateway](../tasks/unified-mcp-gateway/index.yaml)
is open backlog; do not describe it as replacing the existing package bridge.
