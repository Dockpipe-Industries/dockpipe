# Objective Execution And Session Handoffs

Use `dorkpipe-objective-execution` for a bounded outcome that requires multiple ordinary checkpoints.
The objective remains active until its observable `done_when` passes, it is genuinely blocked, its
required verification fails, or the user cancels it. Do not ask for a new approval or task after
each source edit, test group, audit pass, or other retryable local checkpoint.

Use `dorkpipe-one-shot-gate` only for a separately approved single-use, no-retry,
authority-consuming, materially costly, destructive, externally mutating, or credential-bearing
action. The objective skill seals the gate packet but never invokes it. The gate skill invokes once,
performs bounded read-back, and returns to the same objective.

Use `dorkpipe-task-handoff` as transport only. It has three modes:

- `continue_objective`: preserve the active objective in a fresh task after a user request or when
  context pressure makes continuation materially clearer;
- `enter_one_shot_gate`: carry one approved sealed gate into its isolated execution task;
- `resume_objective`: carry the terminal gate receipt back to the unchanged active objective.

Objective authority survives all three modes. Gate authority is consumed when invocation begins.
Creating the approved gate task also preauthorizes one transport-only `resume_objective` handoff,
which prevents the gate task from stranding the objective or asking for another micro-slice. A later
gate still needs separate approval after the objective controller receives and classifies the first
gate result.

The objective skill infers context pressure without claiming an exact token meter. At a safe
checkpoint boundary it may automatically use `continue_objective` when the host has compacted the
conversation, required state needs repeated reconstruction, or at least two weaker pressure signals
show that a compact continuation will preserve the objective more reliably. It must finish safely
in place when only bounded terminal proof remains and must never hand off during mutation or
read-back. Another implementation seam, materially different guidance set, or broad exploratory
audit is not "nearly complete".

A continuation receiver admits durable completed proof, revalidates only affected live anchors, and
executes the single pending boundary first. After that checkpoint it updates durable state and
reassesses context pressure before opening another seam or broad terminal suite. Successful checks
stay quiet; noisy checks keep full output in a task-owned temporary log and surface only a bounded
failure excerpt.

The one-handoff limit is per task. The source task consumes its creation authority, while the fresh
task receives its own outgoing context-handoff allowance under the same objective policy. Do not
write "no second handoff" unless the user or objective contract explicitly defines an objective-wide
chain limit. If such a chain limit prevents safe continuation, checkpoint and stop for missing
transport authority instead of forcing the remaining objective through an overloaded task.

`dorkpipe-task-execution` is a compatibility router for legacy contracts. New work names the
specialized skill directly.

## Handoff boundaries

- Re-read live checkout, dirty-tree ownership, objective state, authority state, and gate attempt
  state before transport.
- Never transfer secrets or resolved credentials; carry opaque references and sanitized hashes.
- Never turn handoff context into commit, push, cleanup, publication, cost, credential, retry, or
  external-resource authority.
- Do not interrupt a running mutation or incomplete read-back. Reach a safe boundary first.
- After successful task creation, stop the old task. Do not run the fresh task's execution skill in
  the old task.
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

Carry a self-contained lifecycle record, not a status sentence or transcript replay. It must state:

- handoff mode, stable objective id, objective authority, current state, and specialized skill;
- bounded objective, observable `done_when`, current checkpoint, exclusions, and terminal rules;
- live checkout anchors, dirty-tree ownership, protected state, and completed proof;
- gate id, exact action, readiness coverage, attempt count, authority consumption, and read-back when applicable;
- explicit hard stops and whether task creation itself authorizes execution or only transport.

Keep it compact enough for a fresh agent to execute without reopening the previous conversation.
Target 500-900 words for an ordinary continuation, using counts and digests instead of full
inventories or per-file hashes unless exact boundary proof requires them.
