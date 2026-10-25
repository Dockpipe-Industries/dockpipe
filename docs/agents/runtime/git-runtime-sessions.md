# Git Runtime Sessions

Read when changing Git lifecycle, workspace ownership, autonomous edit sessions, checkpointing,
managed workspaces, bind-mount policy, or recovery/publish behavior.

## Canonical Design

Use [Git runtime sessions](../../runtime/git-runtime-sessions.md) for the implemented
local interface and the separately labelled design. Use [operation results](../../runtime/operation-results.md)
for the shared result/logging contract. Design goals are not evidence of current support.

## Hard Rules

- AI agents must not run arbitrary Git commands for session lifecycle.
- Git operations are runtime lifecycle actions, not resolver or package-script behavior.
- Every autonomous edit session should run on a runtime-owned session branch.
- Checkpoint commits are recovery/collaboration artifacts, not final human-approved history.
- Human review remains the final authority before merge.
- Workflow authors describe workspace intent; runtimes decide clone, worktree, volume, bind mount,
  branch, checkpoint, and cleanup mechanics.
- For `workspace.storage: volume`, treat `/work` as the worker editing surface and keep Git
  lifecycle actions in runtime-owned helper tools.
- Provider/auth mount rules live in `docs/agents/runtime/git-runtime-auth.md`.

## Routing

| Work type | Read with |
| --- | --- |
| Runtime/session primitives | `docs/agents/core/engine-boundary.md`, `docs/agents/core/architecture.md` |
| Provider detection or auth mounts | `docs/agents/runtime/git-runtime-auth.md`, `docs/agents/core/engine-boundary.md` |
| Workspace storage or path movement | `docs/agents/core/path-scopes.md`, `docs/agents/core/core-package-model.md` |
| Workflow YAML fields | `docs/agents/workflows/yaml-workflows.md`, `src/lib/infrastructure/schema/workflow.schema.json` |
| Agentic orchestration behavior | `docs/agents/workflows/model-escalation.md`, `docs/agents/workflows/docs-generation.md` |
| Generated state, recovery logs, MCP/session metadata | `docs/agents/runtime/artifacts-and-mcp.md` |

## Implementation Guidance

- Keep workspace/session identities, leases, lifecycle requests and events provider-neutral.
  Branch logic belongs to runtime infrastructure, not provider-specific engine branches.
- Reuse the shared operation-result contract for lifecycle work. Preserve failure and cancellation
  results instead of inventing per-path status vocabularies.
- Current local sessions require an existing Git checkout. Managed sessions default to a worktree;
  `storage: volume` supplies a Docker editing surface with a managed worktree for checkpointing.
  `storage: clone` is reserved and rejected. `workspace.repo` does not clone remote URLs.
- Keep `mode: bind` explicit. Do not treat a design preference for volume-first storage as authority
  to change existing defaults or the user's selected checkout.
- Keep session metadata and audit logs under paths returned by the session helpers. Workers edit
  `/work`; runtime-owned helpers own Git lifecycle. Auth mount rules remain in the linked guide.
- Serialize shared write ownership before adding parallel workers/branches. A lease or checkpoint
  is not publication authority.

## Approval Boundaries

- Controlled checkpoints use `dockpipe session checkpoint <id> --request <path> --receipt <path>`.
  Preserve exact session/workspace, branch/parent, authorization/request fingerprints, sorted paths,
  postimages and bounded message validation. Reject staged state, extra/unsupported changes,
  stale or linked/non-regular postimages, and ambiguous recovery before committing.
- Revalidate controlled receipts and exact-commit recovery instead of creating another commit.
  Checkpoint approval grants no push, publication, sync, merge or branch-switch authority.
- Policy-driven `CheckpointSession` remains the broader existing finalizer. `auto`/`step` YAML
  policy is not a substitute for exact-path approval in approval-sensitive package flows.
- Plain `session publish` may checkpoint dirty work before pushing the session branch. Strict
  request-driven publication binds an already approved checkpoint and pushes only the approved
  commit/ref; it must not create a checkpoint, rewrite a branch, force, delete or publish tags.
  Keep these paths distinct; see the canonical request/receipt contracts before changing either.
- `session switch` prints a managed worktree path; it cannot change the parent shell directory.
- Provider skills overlays and DorkPipe worker prompt preparation are harness responsibilities,
  not generic Git-session behavior. Implement them in the owning package and preserve the user’s
  existing provider skills when adding curated overlays.

## Validation

- For docs-only routing changes: `git diff --check`.
- For workflow surface changes: update schema, docs, language support, and run workflow validation.
- For runtime/session code: use the [validation workflows](../core/validation-commands.md#core), starting with affected session/checkpoint/publication tests. Build or broaden only when required by the change.
