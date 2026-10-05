# DorkPipe Objective Execution

Own one bounded objective until it is complete, genuinely blocked, cancelled, or fails required
verification. Advance ordinary in-scope checkpoints automatically. A checkpoint is progress inside
the objective, not a successor that needs a new approval or task.

## Admit the objective

Read the source-controlled objective or task record, applicable `AGENTS.md`, focused routing docs,
and live checkout state. Prefer a cheap read-only check over conversation memory.

Require or establish this contract before mutation:

```text
Objective contract:
objective_id: <stable id>
state: ready_for_execution | executing | waiting_for_user | completed | blocked | failed_verification | cancelled
execution_skill: dorkpipe-objective-execution
execution_authority: approved_objective_creation
authorized_objective: <bounded outcome, larger than one mechanical edit>
done_when: <observable completion proof>
inherited_invariants: <facts that must remain true>
explicit_exclusions: <forbidden or separately authorized work>
checkpoint_policy: automatic_within_objective
verification_policy: <focused checks plus terminal proof>
handoff_policy: user_requested_only
context_pressure_policy: warn_and_continue
checkpoint_output_policy: quiet_success_bounded_failure
terminal_conditions: completed | blocked | failed_verification | cancelled
```

Objective creation or an explicit user request to begin the named objective authorizes ordinary,
reversible implementation and validation needed to reach `done_when`. Destructive cleanup, commit,
push, publication, cost, credential refresh or profile mutation, and external resource mutation
remain outside scope unless the user or repository contract explicitly authorizes them. Once an
operation is explicitly authorized, execute and recover it inside this objective instead of moving
it to a gate task or manufacturing a single-use authority contract.

Inventory branch, HEAD, staged, unstaged, and untracked state. Identify user-owned changes and the
paths the objective owns. Preserve unrelated bytes. Stop `blocked` when ownership overlaps cannot
be resolved safely.

## Receive a continuation

For `continue_objective`, treat the handoff's durable completed proof as the
admitted baseline. Revalidate affected live anchors, but do not reconstruct completed chronology,
rerun passed proof, or reopen completed implementation unless drift, a new failure, or the pending
checkpoint directly requires it.

The fresh task starts with this receiver budget:

- own the single `Pending boundary` as the first checkpoint;
- admit supporting work only when it is strictly required to complete that checkpoint;
- after that checkpoint, update durable state before selecting another materially different seam;
- continue in the receiving task until completion or until the user requests another handoff.

## Respect boundaries without fragmenting execution

Before mutation, confirm that the next action is inside `authorized_objective` and outside
`explicit_exclusions`. Read-only diagnostics, source edits, preflight checks, credential-backed
readiness, and evidence collection stay in this objective.

For an external, destructive, costly, credential-refreshing, publishing, or otherwise consequential
action, require explicit authority from the user or repository contract. Do not infer that authority
from source-edit, review, or diagnostic scope. Once the exact action is authorized, keep its
preparation, invocation, read-back, repair, and evidence-supported retry in this same task and
objective. Do not create a fresh task merely because the action uses credentials or mutates an
external system.

Do not invent approval seals, reuse policies, attempt budgets, or no-retry rules. If a repository or
external system provides a real nonce, concurrency token, idempotency key, or attempt limit, respect
that actual mechanism and record only the evidence needed to use it safely.

## Advance checkpoints

Set `state: executing`, then repeat until `done_when` is proven or a terminal condition is reached:

1. Select the next necessary checkpoint from the objective record and current evidence.
2. Revalidate only anchors that could have changed and matter to that checkpoint.
3. Implement the checkpoint while preserving invariants and unrelated work.
4. Run the smallest check that proves the checkpoint and record durable evidence.
5. Update the objective record and continue without asking for approval or creating a fresh task.

Discovery may refine checkpoint order or add necessary checkpoints already implied by
`authorized_objective`. It must not widen the outcome, erase exclusions, add speculative cleanup,
or turn a backlog item into authority.

Use a full rebaseline only after a branch or HEAD change, unexpected dirty path, protected-state
change, ownership overlap, or mutation that invalidates terminal acceptance evidence. Do not repeat
the full baseline after every mechanical edit.

Every repeated audit or verification pass needs changed relevant implementation, new failure
evidence, or an unresolved blocking invariant. Without one of those triggers, classify remaining
ideas as deferred and converge.

Keep verification output proportional to the proof:

- run focused checks before package-wide or repository-wide suites;
- keep successful output quiet and record command, exit status, and a compact result;
- send predictably noisy output to a task-owned temporary log and surface only the failure excerpt;
- run broad terminal verification once after the last material change, unless a failure or later
  change invalidates it;
- do not print full status inventories, hashes, or logs when counts, affected paths, and digests
  preserve the same evidence.

## Detect context pressure

Do not claim access to an exact remaining-token meter. Infer context pressure at safe checkpoint
boundaries from observable signals.

**Hard signals:**

- the host has compacted or replaced earlier conversation with a summary;
- required authority, invariants, ownership, or evidence can no longer be kept reliably available
  without reconstructing earlier context.

**Soft signals:**

- old constraints or evidence must be repeatedly reopened or restated;
- accumulated tool output, anchors, and policy dominate the context needed for the next checkpoint;
- the next checkpoint needs a materially different file, documentation, or tool context;
- a recent omission or correction indicates that a still-applicable constraint was buried;
- the durable objective state can now be represented more clearly in a compact continuation packet.

When a hard signal or at least two soft signals are present, tell the user briefly that the current
conversation is wasting context and that a handoff would make continuation cleaner. Keep working in
the current task unless the user asks for the handoff. Never create a task automatically because the
conversation is long, a checkpoint completed, credentials are involved, or an operation failed.

If the user requests handoff, finish the current atomic mutation or read-back, update durable state,
then invoke `dorkpipe-task-handoff`. Do not abandon an in-flight action or claim that task-local
state will survive transport.

## Recover and retry from evidence

Failure is not an automatic loss of authority and does not justify a new task. Classify the observed
effect before retrying:

- **No external effect:** repair readiness and retry in this task while the action remains in scope.
- **Idempotent or reconcilable effect:** use the documented idempotency or reconciliation path, then
  retry or continue from verified state.
- **Partial or unknown effect:** perform bounded read-back first. Retry only when evidence establishes
  that doing so is safe; otherwise stop and ask for the missing decision or authority.
- **Actual external attempt limit or consumed capability:** respect the system-provided limit and
  report it. Do not generalize it into a DorkPipe-wide no-retry policy.

A failed preflight that produced no external effect may be fixed and rerun. Credential expiry,
missing expiration metadata, transient transport failure, and stale local readiness are ordinary
recoverable conditions unless the user or the actual external system says otherwise. Credential
refresh still requires explicit authority when it changes a profile, but it does not require a new
task.

## Hand off without fragmenting work

Use `dorkpipe-task-handoff` only when the user requests a fresh task. Context pressure is a reason to
warn and offer the option, not authority to create a task. Ordinary checkpoint completion, external
mutation, deployment, credential work, preflight failure, and retry are never handoff triggers.

For `continue_objective`, carry the same objective id, authority, remaining `done_when`, exclusions,
dirty-tree ownership, completed proof, and next checkpoint. Task creation continues the objective;
it does not approve a newly invented scope.

## Terminate

| Condition | State | Action |
| --- | --- | --- |
| `done_when` and terminal verification pass | `completed` | Report proof and stop. |
| Required authority, ownership, external state, or human decision is missing | `blocked` | Name the exact blocker and stop. |
| Required verification fails after authorized work | `failed_verification` | Preserve evidence; do not claim completion. |
| User cancels the objective | `cancelled` | Stop safely without cleanup unless separately authorized. |
| Only optional improvements remain | `completed` when `done_when` passes | Defer them; do not create micro-slices. |

Finish with owned files, validations, generated artifacts, deferred findings, terminal state, and
whether user-requested transport was used. Follow repository Git policy; never infer commit or
synchronization approval.
