### Follow-up: Workflow Provider-Pool Leasing

Current state:

- Pipeon direct chat uses the host MCP provider-pool path and benefits from warm workers through
  `dorkpipe.provider_pool_chat`.
- CLI orchestrator prompts use the same DorkPipe provider-pool prompt contract.
- Workflow fan-out and sub-container agent nodes can now explicitly lease provider-pool workers when
  inherited workflow/agent/task `model_policy.execution_mode` is `provider_pool`. Workflows without
  that explicit policy still use the normal workflow resolver/runtime path.

Goal:

Let selected workflow/subagent turns use provider-pool workers without making provider pools the
workflow engine and without giving arbitrary containers direct access to host provider credentials.
This should be an explicit workflow/model-policy capability, not extension-local routing and not a
provider-specific shortcut.

Proposed authored surface:

```yaml
model_policy:
  execution_mode: provider_pool
  provider: codex
  model: default
  role: workflow
  session_scope: workflow
  max_active: 2
  queue_timeout_seconds: 60
```

Equivalent step/node-level overrides may be allowed when only one worker in a DAG needs the pool:

```yaml
steps:
  - id: review
    uses: agent.prompt
    model_policy:
      execution_mode: provider_pool
      provider: claude
      role: workflow
      session_scope: node
```

Field intent:

- `execution_mode`: provider-neutral mode selector. Initial values should include the existing
  default workflow execution mode and `provider_pool`. Implemented values are `workflow` and
  `provider_pool`.
- `provider`: catalog provider id such as `codex`, `claude`, or `ollama`; no provider-specific schema
  fields. This remains supplied through existing agent model/lane selection rather than duplicating
  provider/model fields inside `model_policy`.
- `role`: `direct`, `workflow`, or future role names. Workflow nodes should not reuse direct-chat
  workers by default.
- `session_scope`: `node`, `workflow`, or future `explicit`. `node` isolates each agent turn,
  `workflow` allows reuse within one workflow run, and `explicit` would require a caller-provided
  session id. Implemented values are `node` and `workflow`.
- `max_active`: cap for this workflow's active pool leases. The effective cap is the minimum of
  workflow policy, provider catalog policy, and global pool policy.
- `queue_timeout_seconds`: how long a workflow node may wait for a pooled worker before failing with
  an observable queued/timeout state.

Runtime contract:

- The workflow runner asks DorkPipe for a lease using generic fields:
  `provider`, `role`, `session_id`, `workflow_run_id`, `node_id`, `model`, `workdir`,
  `approval_mode`, `budget`, and `capabilities`.
- DorkPipe returns a worker/turn endpoint or a queued/auth/disabled/failed state through the existing
  provider-pool contract.
- The provider worker receives an explicit DorkPipe contract: workspace, allowed tools, budget,
  approval mode, escalation rules, and optional MCP bridge endpoint. It must not discover host
  authority implicitly from bind mounts.
- The workflow node sends one prompt turn through the provider-pool prompt operation and records the
  normal workflow artifacts plus provider-pool timing metadata.
- Lease release happens when the node completes, fails, is canceled, or the workflow run is torn down.

Session isolation rules:

- Direct Pipeon chat sessions and workflow sessions are separate by default.
- Workflow-scoped sessions may be reused across nodes only when the workflow declares that scope and
  the provider catalog says the worker mode supports it safely.
- Provider auth stays on the host/provider-pool control plane or inside the guarded provider worker.
  Do not mount host Codex/Claude credentials into arbitrary workflow containers.
- A sub-container agent may call the pool only through an explicit DorkPipe/MCP control-plane
  contract granted to that workflow run.

Implementation status on 2026-07-09:

- Authored workflow/model policy support now includes `execution_mode: provider_pool`, `role`,
  `session_scope`, `max_active`, and `queue_timeout_seconds` in the generic workflow domain, JSON
  schema, workflow docs, and VS Code language support. No Claude- or Codex-specific YAML fields were
  added.
- DorkPipe orchestration task execution now reads inherited workflow/agent/task `model_policy` and,
  when `execution_mode: provider_pool` is selected, routes the node through
  `dorkpipe provider-pool prompt --json` instead of the legacy direct provider CLI branch.
- Workflow provider-pool sessions are isolated from Pipeon direct chat by default with generated
  session ids under `workflow:<run>:node:<task>:role:<role>` or
  `workflow:<run>:role:<role>` for workflow-scoped sessions.
- Provider-pool prompt metadata now carries provider-neutral workflow attribution:
  `role`, `workflow_run_id`, `node_id`, `session_id`, `worker_id`, `prompt_turn_id`, queue timing,
  requested max active, and selected model.
- Lease acquisition now counts provider lease files up to the effective max-active cap, can wait up
  to `queue_timeout_seconds`, returns an observable `queued` state on timeout, and releases leases via
  normal prompt teardown/cancellation paths.
- Provider prompt errors that Codex reports with process exit code 0 are now converted to
  `state: failed` / `exit_code: 1`, so workflow nodes do not treat provider API/model errors as
  successful worker output.
- Validation timings from this pass:
  - provider-pool CLI unit tests: about 3.5 seconds
  - orchestration helper unit tests with repo-local Go cache: about 12.7 seconds
  - `src/lib/domain` tests: cached pass
  - workflow infrastructure tests focused on workflow validation: about 11.0 seconds
  - source-backed workflow validation of a provider-pool model-policy smoke file: passed
  - DorkPipe package workflow compile: passed, about 292 seconds, regenerating workflow tarballs under
    `bin/.dockpipe/internal/packages/workflows`
- Live prompt findings:
  - provider status reported Codex ready, Ollama warming at `http://127.0.0.1:11434`, and Claude
    disabled because image `dockpipe-claude:latest` was missing.
  - initial direct provider-pool Codex smoke reached the provider-pool path, but PATH resolved
    `codex` to the stale Node shim at CLI `0.42.0`; that binary rejected the configured `gpt-5.5`
    model and newer catalog models.
  - the newer Codex app/VS Code bundled CLI `0.142.5` succeeded with the same `gpt-5.5` config, so
    the DorkPipe Codex provider-pool resolver now prefers `CODEX_CLI_PATH`, bundled Codex app/VS Code
    binaries, and only then PATH.
  - direct provider-pool Codex smoke now succeeds through DorkPipe:
    `state=ready`, `text=provider-pool-direct-smoke`, `provider_prompt_ms=12783`, `total_ms=12791`.
  - workflow-attributed provider-pool prompt smoke now succeeds with expected provider-neutral
    metadata: `state=ready`, `text=provider-pool-workflow-smoke`, `role=reviewer`,
    `workflow_run_id=provider-pool-smoke`, `node_id=review`,
    `session_id=workflow:provider-pool-smoke:node:review:role:reviewer-v2`,
    `queue_timeout_seconds=1`, `requested_max_active=1`, `provider_prompt_ms=12964`, and
    `total_ms=12973`.
- Pipeon needs no extension-local routing change for this follow-up. The changed path is the shared
  DorkPipe provider-pool CLI/MCP contract and DorkPipe workflow runner; no `packages/pipeon` files
  were touched.

Follow-up completion checklist:

- Done: schema/docs/language-support for provider-neutral `model_policy.execution_mode:
  provider_pool`, `role`, `session_scope`, `max_active`, and `queue_timeout_seconds`.
- Done: workflow-run/node identity in provider-pool prompt metadata without changing the public
  provider/session/worker/turn model.
- Done: package-owned workflow runner adapter for eligible agent prompt nodes through
  `dorkpipe provider-pool prompt --json`.
- Done: max-active and bounded queue timeout enforcement with visible queued/failed states.
- Done: Codex host-provider direct and workflow-attributed provider-pool smoke validation.
- Done: Pipeon extension-local routing check; no Pipeon changes needed for this follow-up.
- Partial: MCP/status visibility reports provider state and active lease counts through existing
  status concepts, but it does not yet list workflow lease role/run/node details.
- Deferred: `session_scope: explicit` and caller-provided workflow session ids.
- Deferred: Claude guarded stream-worker workflow lease smoke after the local
  `dockpipe-claude:latest` image is rebuilt/materialized.
- Deferred: Pipeon UI status validation of workflow lease metadata. Pipeon direct chat already uses
  the shared MCP path, but no workflow-lease-specific UI change was added.

Acceptance criteria:

- Done: A workflow with `execution_mode: provider_pool` can run a subagent turn through the shared
  provider-pool contract and record provider-pool timings in its artifacts.
- Done: Direct Pipeon chat continues to use its own session-affine provider lane and is not polluted by
  workflow/subagent context.
- Done: `max_active` and queue state are enforced and visible; workflow fan-out cannot silently create
  more cloud/provider work than policy allows.
- Done: Prompt cancellation/teardown releases leases. Provider-pool stop removes provider lease files
  as part of pool lifecycle cleanup.
- Done: Public API and MCP terminology remains provider-neutral: provider, session, worker, turn.

Non-goals for this follow-up:

- Do not make provider-pool execution the default for every workflow.
- Do not add Claude- or Codex-specific workflow YAML fields.
- Do not put provider routing logic in the Pipeon VS Code extension.
- Do not bypass YAML workflow artifacts, approvals, or validation just because a warm worker is used.

Claude-specific note:

- A current Claude worker transcript showed the CLI correctly denying any implicit host/MCP contract:
  the bind-mounted `/work` repo is file access, not host execution. The warm Claude lane must therefore
  be launched with an explicit DorkPipe/Pipeon contract and routed through a guarded bridge. It should
  not rely on the Claude Code session independently discovering `packages/dorkpipe-mcp`.

Non-goals:

- Do not move Claude-specific warm-worker behavior into `src/lib/` or `src/cmd/`.
- Do not replace workflow YAML with extension-local routing state.
- Do not keep hidden cloud workers alive when the stack is closed; warm lanes live and die with the
  Pipeon/DorkPipe stack.

