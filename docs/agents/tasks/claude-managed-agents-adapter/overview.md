# TASK-034 Claude Managed Agents Remote Provider Adapter

## Goal

Determine whether Anthropic-hosted Managed Agents should become an optional remote DorkPipe provider
mode while preserving DockPipe's local-first behavior, provider-neutral contracts, explicit budgets,
and guarded execution boundaries.

## Upstream Baseline (2026-08-12)

- [Managed Agents sessions](https://platform.claude.com/docs/en/managed-agents/sessions) persist
  conversation history and run an agent in an Anthropic-managed environment.
- [Session event streams](https://platform.claude.com/docs/en/managed-agents/events-and-streaming)
  provide bidirectional events, streamed output, steering/interruption, tool confirmations, custom
  tool results, history reconciliation, and idle-session resume.
- [Anthropic's migration guide](https://platform.claude.com/docs/en/managed-agents/migration)
  distinguishes the models clearly: the Agent SDK runs in a process DorkPipe operates, while Managed
  Agents runs in Anthropic infrastructure and replaces local paths with uploaded or mounted resources.
- The API is beta and requires the documented Managed Agents beta header. Its contracts must not be
  treated as stable until revalidated during implementation.

## Ownership Split

- TASK-012 owns the existing guarded local Claude provider pool and the local Agent SDK comparison.
- TASK-013 owns Codex App Server behavior and remains independent.
- TASK-034 owns only an explicit Anthropic-hosted provider mode and its remote data, auth, sandbox,
  retention, recovery, cost, and availability policy.

## Work Packages

1. **Capability and policy review:** verify current API availability, authentication, pricing,
   regions, retention, sandbox lifetime, network controls, rate limits, and beta stability from
   first-party documentation.
2. **Neutral mapping:** map agents, environments, sessions, threads, events, approvals, interrupts,
   usage, and terminal states into the existing DorkPipe provider/session/turn vocabulary without
   exposing Claude-native fields in public DockPipe or MCP contracts.
3. **Resource boundary:** define an explicit allowlisted upload/mount model. Never infer authority to
   send a checkout, credentials, private payloads, or host paths to Anthropic infrastructure.
4. **Safety and economics:** require visible remote-execution selection, network/data disclosure,
   spend and token caps, cancellation behavior, audit evidence, and fail-closed handling for unknown
   or disconnected outcomes.
5. **Prototype:** only after the contract is reviewed, build a package-owned, non-default prototype
   behind the generic provider-pool boundary. Do not add Claude-specific behavior to `src/lib/` or
   `src/cmd/`.

## Acceptance Criteria

- Local Claude remains available and is the default where local policy selects it; enabling Managed
  Agents is explicit and session-visible.
- Remote resources contain only reviewed allowlisted inputs, with no implicit host credential or
  filesystem access.
- Approvals, user input, interruption, reconnect/history reconciliation, idle resume, usage, and
  terminal outcomes map deterministically and fail closed.
- Beta API or capability drift disables the adapter safely instead of silently falling back,
  replaying a possibly active turn, or switching execution location.
- Focused package tests prove provider-neutral behavior and preserve engine/package boundaries.

## Explicit Non-Goals

- Replacing TASK-012's guarded local Claude lane.
- Making Managed Agents a prerequisite for TASK-013, Pipeon startup, or platform qualification.
- Treating the Claude Code background supervisor as a public RPC API.
- Uploading local workspaces or protected data without a separately reviewed execution policy.

## Status

Backlog only. Research is useful after the current TASK-013 App Server lane and the TASK-012 local
Agent SDK comparison; no prototype, remote call, account setup, or beta dependency is authorized.
