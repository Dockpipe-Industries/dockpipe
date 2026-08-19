## DorkPipe Provider Pools

Provider pools are a DorkPipe orchestration feature. They keep a bounded number of provider workers
ready for low-latency top-level orchestration while preserving DockPipe's governed runtime boundary.

Core intent:

- DorkPipe owns provider identity, pool lifecycle, provider availability, session affinity, queueing,
  spend limits, auth state, worker health, and workflow escalation policy.
- DockPipe remains the generic spawn/run/act engine. Do not add Pipeon-, Claude-, Codex-, or
  Ollama-specific behavior to `src/lib/` or `src/cmd/`.
- Pipeon, CLI workflows, and future UI surfaces call the same DorkPipe pool contract instead of each
  inventing provider routing.
- Pools are for top-level low-latency orchestrators and reusable direct workers. YAML workflows remain
  the durable contract for fan-out, validation DAGs, isolated edits, approvals, and release/CI work.

### Warm Provider Lanes

Direct chat and direct CLI orchestrator prompts should not cold-start provider workers for every
prompt.

Desired steady-state model:

- `ollama` stays up as the local model service for cheap/local direct chat and workflow lanes.
- `codex` stays available as a direct host-provider lane. It can use Codex's own host sandbox/session
  model and should preserve the Pipeon session binding so follow-up prompts resume cheaply.
- `claude` should have a warm guarded lane owned by the Pipeon/DorkPipe stack. Direct Claude chat
  should send prompts to that already-running boundary instead of invoking
  `dockpipe --package agent --workflow claude ...` for every message.
- Agentic workflows remain separate. When a direct provider decides a task needs fan-out, validation
  DAGs, or isolated edits, it escalates into YAML-backed DorkPipe workflows; the top-level direct
  chat providers do not become the workflow engine.

Historical gap resolved by the stream-worker implementation recorded below:

- Claude direct chat previously routed through the guarded DockPipe workflow boundary per prompt. That
  preserved the resolver/auth/container boundary, but created a container, ran one prompt, returned,
  and tore down. It was correct for cold workflow execution but too slow for a top-level chat lane.

Implemented direction:

- Package-owned warm provider workers live under the DorkPipe package/provider layer, with Pipeon as
  one consumer.
- Workers receive the repo/workspace bind, resolver auth/config paths, and explicit provider,
  workspace, tool, bridge, escalation, and approval contract.
- The guarded request path uses the shared host MCP boundary without exposing raw unrestricted host
  execution.
- Pipeon provider availability reflects whether the corresponding host or stack lane is ready and
  authenticated.
- Direct-provider results include queue, status/readiness, worker, provider-turn, and total timing.
- Cold workflow execution remains available for agentic runs, fan-outs, isolated worktrees, and
  release/CI use cases.

### Pool Contract

Warm lanes should generalize into provider pools.

Pool model:

- `provider_pool.<name>.min_ready` keeps a small number of workers warm while the Pipeon stack is
  open. Example: one Claude worker for direct chat, one Ollama service, zero or one Codex worker
  depending on whether Codex exposes a worthwhile persistent protocol.
- `provider_pool.<name>.max_active` caps concurrent workers so fan-out cannot silently explode cloud
  spend or host resource use.
- `provider_pool.<name>.idle_ttl` drains unused workers after a quiet period, unless the lane is marked
  sticky for an active chat session.
- `provider_pool.<name>.session_affinity` controls whether follow-up prompts stay on the same worker
  for context/memory, or whether prompts can be routed to any ready worker.
- `provider_pool.<name>.role` separates direct chat workers from workflow fan-out workers. Direct chat
  should be low-latency and session-affine; workflow workers can be short-lived, pooled, or scaled by
  DAG demand.
- `provider_pool.<name>.budget` records cloud/provider spend limits, token caps, and halt behavior.

### Provider Stream Worker Contract

The provider-pool stream-worker milestone established a generic contract. The public contract stays
provider-neutral and DorkPipe-owned; Claude is the first implementation because its CLI exposes a
working stream JSON mode, not because the pool API should become Claude-specific.

Terminology:

- **Pool provider**: catalog entry such as `claude`, `codex`, or `ollama`.
- **Pool session**: DorkPipe-owned session identity selected by CLI/Pipeon/MCP. It carries provider,
  workdir, role, selected model, budget policy, approval mode, and optional UI chat identity.
- **Worker instance**: runtime-owned execution boundary for a provider session. It can be a guarded
  container, host process, local service, or future remote worker depending on catalog/runtime policy.
- **Stream process**: optional long-lived provider process inside a worker instance. It receives
  prompts on stdin or a local socket/protocol and emits structured events until reset or terminated.
- **Prompt turn**: one request/response exchange within a pool session. Turns must be metered,
  observable, cancelable, and attributable even when they share a stream process.

Generic API shape:

- `provider-pool catalog`: returns provider capabilities including whether the provider supports
  `stream_worker`, `single_prompt`, `host_resume`, `service`, or another execution mode.
- `provider-pool status`: reports provider state plus worker/session details:
  `ready`, `warming`, `busy`, `queued`, `failed`, `auth-required`, `disabled`, selected mode,
  worker id, session id, prompt count, last turn timing, last error, and spend/budget state.
- `provider-pool warm`: materializes the minimum ready worker count for enabled providers. For
  stream-capable providers, warming starts both the worker boundary and the stream process when
  policy permits.
- `provider-pool prompt`: remains the stable direct prompt API for CLI, Pipeon, and MCP. It chooses
  a stream worker when available, falls back only according to explicit catalog policy, and returns
  the same response envelope as today with richer metadata.
- `provider-pool reset`: future operation to reset a pool session or worker when context,
  authentication, budget, or health requires it. Reset must be explicit and observable.

Existing MCP leverage:

- The stack already has the right MCP front door. `packages/dorkpipe-mcp` exposes
  `dorkpipe.provider_pool_catalog`, `dorkpipe.provider_pool_status`, and
  `dorkpipe.provider_pool_chat`; Pipeon already calls the host MCP bridge tool
  `dorkpipe.provider_pool_chat` for direct provider chat.
- The first stream-worker implementation should preserve those MCP tool names and schemas where
  possible. The `dorkpipe.provider_pool_chat` handler should keep invoking the generic
  `dorkpipe provider-pool prompt --json` contract, and the provider-pool implementation should choose
  the fast stream worker behind that CLI contract.
- New MCP tools are only needed for new lifecycle operations that do not exist today, such as
  `dorkpipe.provider_pool_warm`, `dorkpipe.provider_pool_reset`, or future streaming/event-subscribe
  support. Do not create a Claude-only MCP tool for this fast path.
- If richer streaming to the UI is needed, prefer a generic provider-pool event stream contract
  keyed by provider/session/turn id. Keep the current JSON response path as the compatibility
  baseline for CLI workflows and simple MCP clients.
- The Pipeon stack MCP proxy should remain the externally exposed control-plane surface. The
  upstream DorkPipe MCP service and any provider stream worker internals stay private to the stack or
  host bridge boundary according to existing MCP tier/auth policy.

Required response metadata for direct prompt turns:

- `provider_preset`, `selected_model`, `session_id`, `worker_id`, `worker_mode`
- timing fields:
  `queue_wait_ms`, `status_ms`, `worker_start_ms`, `stream_start_ms`, `stream_ready_ms`,
  `time_to_request_ms`, `time_to_first_token_ms`, `provider_api_ms`, `provider_turn_ms`,
  `total_ms`
- stream fields when available:
  `provider_session_id`, `provider_request_id`, `prompt_turn_id`, `prompt_count`,
  `stream_reused`, `stream_restart_reason`
- budget fields when available:
  `estimated_input_tokens`, `estimated_output_tokens`, `estimated_cost_usd`, `budget_remaining`,
  `budget_halt_reason`

Worker lifecycle rules:

- Session affinity is honored first. A Pipeon chat session or CLI `--session-id` should reuse the
  same stream worker while healthy and under budget.
- A stream worker may process many turns, but never more than one active turn unless the provider
  protocol explicitly supports multiplexing and the catalog declares it safe.
- `max_active` caps active workers or active turns according to the provider mode. Queueing must be
  visible instead of silently spawning more cloud/provider work.
- Idle workers drain after `idle_ttl_seconds` unless pinned by an active Pipeon session or explicit
  stack lifecycle policy.
- Failed stream processes restart inside the existing worker boundary when possible. Repeated
  failures mark the provider `failed` with the last error and restart count.
- Stack teardown owns provider-pool teardown. No hidden Claude/Codex/Ollama workers should outlive
  Pipeon unless the catalog exposes an explicit detached mode.

Boundary and ownership rules:

- Keep provider-specific protocol adapters in DorkPipe package-owned assets/code/catalogs, not
  DockPipe engine code. Core DockPipe remains spawn/run/act.
- The generic provider-pool contract is provider/session/worker oriented. It must not expose fields
  such as `claude_session_id` as public API; provider-native IDs belong under provider metadata.
- MCP is a front door and session router, not the only implementation. CLI workflows, Pipeon, and
  future app surfaces all call the same DorkPipe provider-pool operations.
- Pipeon must not own pool state in the VS Code extension. It should store UI chat identity and call
  the DorkPipe MCP/CLI provider-pool contract for catalog, status, prompt, warm, and reset.
- Provider workers must receive an explicit DorkPipe contract: workspace path, allowed tools, budget,
  escalation rules, approval mode, and optional MCP bridge endpoint. Do not rely on a provider CLI
  discovering host capabilities implicitly from a bind mount.

First implementation target: Claude stream worker.

- Preserve the guarded container boundary and copied/allowlisted subscription auth state already used
  by the Claude pool.
- Replace the sleeping warm container plus per-prompt `docker exec claude -p` path with a
  session-affine in-container worker manager that starts:

```bash
claude --dangerously-skip-permissions --model <model> -p \
  --input-format stream-json \
  --output-format stream-json \
  --include-partial-messages \
  --replay-user-messages \
  --verbose
```

- Send each prompt turn as one JSONL user message to the stream process and read structured events
  until a `result` event closes the turn.
- Extract response text from the `result.result` field or final assistant text events, preserving raw
  JSONL diagnostics in debug metadata/artifacts when requested.
- Reuse the same stream process for follow-up turns while session affinity, model, workdir, auth,
  budget, and approval mode are unchanged.
- If model/workdir/auth/policy changes, start a new stream process and report
  `stream_restart_reason`.
- Keep the existing single-prompt `docker exec claude -p` path as a fallback behind an explicit
  catalog mode or failure policy until the stream worker is stable.

Pipeon consumption rules:

- Pipeon provider selection continues to read the shared provider-pool catalog.
- Pipeon direct chat sends messages through the DorkPipe MCP `provider_pool.prompt` operation with a
  stable session id for the chat tab/conversation.
- Pipeon displays provider status from `provider-pool status`, including warming/queued/failed states
  and last-turn timing. It should not infer readiness from extension-local state.
- Pipeon stack startup may call `provider-pool warm`, but it should not eagerly start costly stream
  workers unless the provider is enabled by catalog/environment policy.
- Pipeon should benefit automatically from stream workers because the MCP bridge and CLI orchestrator
  call the same provider-pool prompt contract.

Default pool for Pipeon:

- `ollama`: `min_ready=1`, `max_active=1`, lifecycle tied to the existing Ollama compose service.
- `claude`: `min_ready=1`, `max_active=1`, guarded container worker, session-affine for direct chat.
- `codex`: `min_ready=1` only if a persistent Codex protocol/MCP lane is viable; otherwise direct chat
  keeps using host `codex exec resume` with session binding and no always-on container.
- Defaults must be configurable from package-owned config/environment. Expected knobs:
  `PIPEON_PROVIDER_POOL_CLAUDE`, `PIPEON_PROVIDER_POOL_CODEX`, `PIPEON_PROVIDER_POOL_OLLAMA` or an
  equivalent YAML/catalog-backed setting.

DorkPipe defaults should be independent of Pipeon and reusable from the CLI. Pipeon may override them
for its bundled stack profile, but the source of truth should be package-owned DorkPipe provider
catalog/config, not VS Code extension state.

Lifecycle:

- Pool lifecycle matches long-lived Pipeon stack services such as the project database/control
  services: start with the stack, stay warm while Pipeon is open, drain/stop with stack teardown.
- Workers should not be tied to one prompt. A worker can process many direct chat turns until it is
  idle, unhealthy, over budget, explicitly reset, or the stack closes.
- Direct chat workers are sticky to Pipeon chat sessions when context continuity matters.
- Workflow fan-out workers are leased from the pool by role/capability and released when the workflow
  node completes. They may use a different `max_active` than direct chat.
- If `max_active` is reached, new work queues with visible UI state instead of silently spawning more
  cloud workers.

Operational behavior:

- Startup warms only the configured minimum. It should not pre-spawn expensive cloud workers unless
  the user selected/enabled that provider.
- Direct chat first uses a session-affine warm worker. If none is ready, it can either queue briefly
  while one starts or fall back to a clear “warming provider” status.
- Agentic workflows request workers from the pool by provider/role/capability, but still execute under
  YAML policy with artifacts, approvals, and validation.
- Pool workers must be observable: ready/busy/failed/auth-required, current session binding, elapsed
  lifetime, prompt count, last error, and estimated spend.
- Pool teardown is tied to stack teardown. No orphan provider workers should outlive Pipeon unless an
  explicit detached mode exists.

Dispatch rules:

- Pipeon provider dropdown targets direct provider pools, not workflow fan-out by default.
- CLI provider selection targets the same direct provider pools by default.
- A top-level orchestrator prompt should be low-latency and use a ready direct worker.
- The direct worker can recommend or request escalation to an agentic workflow when the task needs
  parallelism, isolated edits, validation, artifacts, or higher spend.
- DorkPipe workflow YAML remains the durable contract for fan-out/subagents. Provider-native `/loop`
  or `/fanout` features can be used inside a bounded worker implementation, but they should not become
  the public workflow contract.

