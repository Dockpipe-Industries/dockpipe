# Objective Execution And Session Handoffs

Use `dorkpipe-objective-execution` for a bounded outcome that requires multiple checkpoints. Keep
one objective in the current task until its observable `done_when` passes, it is genuinely blocked,
required verification fails, or the user cancels it. Do not ask for a new approval or create a task
after each edit, test, audit, deployment step, credential check, failure, or retry.

## Authority and execution

Objective authority covers the ordinary reversible implementation and verification needed to reach
`done_when`. Respect `AGENTS.md`, live repository state, dirty-tree ownership, and explicit
exclusions throughout.

External, destructive, publishing, costly, credential-refreshing, or otherwise consequential
actions require explicit user or repository authority. Once the exact action is authorized, keep
its preparation, invocation, read-back, repair, and evidence-supported retry inside the same
objective and task. Do not manufacture approval seals, reuse policies, attempt budgets, or a
DorkPipe-wide no-retry rule.

Classify failures from evidence:

- no external effect: repair readiness and retry while the action remains in scope;
- idempotent or reconcilable effect: use the documented idempotency or reconciliation mechanism;
- partial or unknown effect: perform bounded read-back first, then retry only when evidence shows it
  is safe;
- real external attempt limit or consumed capability: respect that actual system constraint.

A failed preflight with no external effect may be corrected and rerun. Credential refresh still
requires explicit authority when it changes a profile, but it does not require a new task.

## Context pressure

The objective skill does not claim an exact remaining-token meter. It watches for compaction,
repeatedly reopened constraints, tool output dominating useful context, materially different
upcoming seams, and corrections caused by buried guidance.

When context is becoming wasteful, tell the user briefly that a handoff would make continuation
cleaner. Keep working in the current task unless the user requests the handoff. Never create a task
automatically because the conversation is long or because an external action is next.

## User-requested handoff

Use `dorkpipe-task-handoff` only after the user requests a fresh task or continuation. It has one
mode: `continue_objective`.

The handoff transports the same objective id, authority, `done_when`, invariants, exclusions,
dirty-tree ownership, completed proof, effect and retry evidence, and next checkpoint. It grants no
new execution scope. Create exactly one fresh task for that user request, use the same saved checkout
without a worktree unless requested, then stop the old task.

A continuation receiver admits durable completed proof, revalidates only affected live anchors, and
executes the single pending boundary first. It does not replay chronology or rerun passed proof
without drift, new failure evidence, or a direct dependency.

## Handoff boundaries

- Re-read the minimum live checkout, ownership, objective, and effect state needed for transport.
- Never transfer secrets or resolved credentials; carry opaque references and sanitized evidence.
- Never claim ephemeral browser, UI, agent-session, or temporary-token state survives transport.
- Never turn handoff context into commit, push, cleanup, publication, cost, credential, retry, or
  external-resource authority.
- Do not interrupt a running mutation or incomplete read-back. Reach a safe boundary first.
- After successful task creation, stop the old task.
- Do not create a worktree unless explicitly requested.

## Normal completion

When the objective completes, report its owned scope, checks, risks, generated artifacts, deferred
findings, and terminal state. Ask before commit in a normal session. Optional follow-up work is not
part of the completed objective and is not automatically authorized.

## Autonomous Master Exception

An explicitly designated master-orchestrator session may select one bounded objective whose required
product decisions are already recorded, then stage only its exact changed files and commit the
validated objective on the current branch. It must preserve unrelated worktree changes and never
push, open a PR, rebase, reset, stash, delete state, or change repository policy incidentally.

Stop and ask the user only for an architecture gate: a missing decision or ambiguous scope; a new
generic primitive or `src/lib` / `src/cmd` edit; a public CLI/MCP/schema contract; a package/runtime
ownership boundary; live provider/Docker/auth/network work; destructive cleanup or secrets;
validation uncertainty; or overlap/conflict with user changes. All other in-objective implementation,
validation, task-documentation, and permitted commit decisions are autonomous.

## Compact continuation prompt

Carry a self-contained lifecycle record, not a status sentence or transcript replay. State:

- `mode: continue_objective`, the stable objective id, authority, state, and execution skill;
- bounded objective, observable `done_when`, pending checkpoint, exclusions, and terminal rules;
- live checkout anchors, dirty-tree ownership, protected state, and completed proof;
- authorized external actions, readiness, observed effects, safe retry evidence, and read-back;
- explicit hard stops and the first receiver action.

Keep it compact enough for a fresh agent to execute without reopening the previous conversation.
Target 500-900 words for an ordinary continuation, using counts and digests instead of full
inventories unless exact boundary proof requires them.
