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
new execution scope. Follow the rendered skill's delivery instructions: create exactly one fresh
task when supported, or return one paste-ready prompt for the user to open a fresh chat. Preserve
the active checkout or managed worktree and its owned state, then stop the old task.

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
- Use a runtime-managed worktree when isolation protects existing edits or supports the bounded
  task. Reuse a suitable existing workspace first; preserve the original checkout and unrelated state.

## Normal completion

When the objective completes, report its owned scope, checks, risks, generated artifacts, deferred
findings, and terminal state. Keep implementation uncommitted until the user authorizes a commit
or explicitly requests moving the bounded task to staging. Optional follow-up work is not part
of the completed objective and is not automatically authorized.

## Bounded staging authorization

An explicit request to move a bounded task to staging authorizes the necessary commits, pushes,
PRs, dependency publications and staging promotion for that task after their required checks pass.
Carry that authority through the flow without asking again at each step. Keep implementation
uncommitted while the task is still being developed unless a validated intermediate source
publication is required by that staging dependency chain.

Preserve unrelated changes and use the runtime-owned Git operations. This authority does not
extend to production, unrelated work, destructive operations or unresolved scope decisions.
An automatic approval rejection still requires resolving its stated condition before retrying.

## Scoped orchestration authority

A master-orchestrator label, task dispatch flag or historical checkpoint does not
itself authorize staging or commits. Apply any explicit user-granted orchestration
scope to its named objective and checkout only, preserving unrelated work. Current
instructions to leave work unstaged or uncommitted take precedence. Ask about a
missing product decision or unresolved scope boundary; do not manufacture a new
approval gate for reversible work already authorized.

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
