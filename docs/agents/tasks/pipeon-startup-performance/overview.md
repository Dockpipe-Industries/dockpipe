# TASK-012 Pipeon Startup, Provisioning, And Provider Pools

## Goal

Make Pipeon open quickly from a normal developer machine while preserving the current local-first,
offline-capable stack.

Generalize warm provider execution into a DorkPipe provider-pool capability that can be used by
Pipeon, CLI workflows, and future app surfaces. Pipeon should consume the pool; it should not own the
pooling model.

Pipeon startup should prefer already-materialized state: repo bind mounts, stable package state under
`bin/.dockpipe`, cached Go/npm/Cargo outputs, existing Docker images, existing containers when safe,
and already-pulled Ollama models.

## Current State Summary

- Immediate startup optimizations skip cached Ollama pulls, hide non-interactive Windows startup
  processes, reuse valid host bridges, and retain package-owned build caches and bind mounts.
- DorkPipe now owns the shared provider-pool catalog, lifecycle, status/chat CLI and MCP operations,
  package orchestrator workflow, and Pipeon consumer path.
- Claude direct prompts use a session/model-affine in-container stream worker with a one-shot fallback;
  measured repeated turns proved the intended low-latency reuse path.
- Claude's supported Agent SDK now offers a richer local integration candidate than the original
  CLI-only assumption. A bounded comparison against the current guarded stream worker is pending;
  no migration is implied.
- Workflow provider-pool leasing is implemented with normalized events, bounded queue/cancel behavior,
  worker reuse, and generic workflow consumption.
- Remaining work is release and product hardening: prebuilt image publication/selection, richer lease
  detail in status and Pipeon, deferred explicit session scope and live workflow smoke, measured
  Pipeon launch budgets, and the local Claude Agent SDK comparison below.

## Claude Local Adapter Refresh (2026-08-12)

Anthropic's supported local surfaces now materially exceed the earlier CLI `stream-json` baseline:

- [Agent SDK streaming input](https://code.claude.com/docs/en/agent-sdk/streaming-vs-single-mode)
  is the recommended persistent, interactive mode and supports long-lived input, interruption,
  permission requests, and session management.
- The SDK exposes structured streaming output,
  [runtime approvals and user questions](https://code.claude.com/docs/en/agent-sdk/user-input),
  permission modes, and [resumable sessions](https://code.claude.com/docs/en/agent-sdk/sessions).
  It therefore deserves comparison with DorkPipe's current direct JSONL parsing rather than being
  treated as a one-shot wrapper.
- [Claude Code agent view](https://code.claude.com/docs/en/agent-view) uses a per-user local supervisor
  with the same credentials as interactive Claude Code and supports attach, logs, stop, and respawn.
  Its documented management surface is CLI/UI-oriented, not a stable provider integration protocol.
  Treat its daemon files and private control channel as implementation details.
- Anthropic-hosted Managed Agents is a separate remote execution model, not a replacement for this
  guarded local/container lane. TASK-034 owns that optional path.

Bounded next slice for TASK-012:

1. Prototype `ClaudeSDKClient` inside the existing guarded Claude worker without changing the public
   provider-pool CLI, MCP, Pipeon, session, or event contracts.
2. Compare it with the current CLI stream worker for copied/allowlisted subscription authentication,
   startup and repeated-turn latency, typed event coverage, approvals and user questions,
   interruption, disk-backed resume, session affinity, health detection, and teardown.
3. Prove the same workspace, tool, MCP, budget, and approval boundaries remain fail closed. Do not
   adopt SDK defaults that widen the guarded worker's authority.
4. Keep the current stream worker as the production baseline until the SDK path passes focused
   offline tests and an explicitly approved guarded live smoke. Retain an explicit rollback path.

This comparison does not block TASK-013 Codex App Server work or the supported-platform program.

## Current Startup Cost Centers

- Branded `dockpipe-code-server:latest` image refresh: packaging VSIX files and rebuilding the image
  is correct but expensive when extension inputs changed.
- DorkPipe stack image refresh: Linux `dockpipe`, `dorkpipe`, and `mcpd` binaries are built for the
  container image and copied into a Docker build context.
- Docker image availability: first launch pays base image pulls and layer extraction.
- Ollama model provisioning: model pulls dominate startup when the model is absent or Docker volume
  state was reset.
- Signature checks: recursive source hashing is safer than mtime-only checks, but it is still a
  startup tax on large extension/source trees.
- Host bridge setup: MCP bridge restart is cheap, but stale allowlists must be detected before reuse.
- Code-server container setup: bind mounting is the right workspace model; copying the repo into a
  volume should not be part of normal Pipeon startup.

## Immediate Optimizations

- Skip `ollama pull <model>` when `ollama list` already shows the requested model.
- Keep non-interactive Windows PowerShell calls hidden so startup does not flash transient consoles.
- Reuse the host MCP bridge only when its tool catalog exposes the required tools for the current
  Pipeon build.
- Keep Go/npm/Cargo caches under package/build state, not global `.gocache` or `.gotemp` paths.
- Keep the workspace bind-mounted so repeated launches reuse the same repo and package state.

