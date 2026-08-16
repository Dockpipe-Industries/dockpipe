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
state: ready_for_execution | executing | waiting_for_gate | completed | blocked | failed_verification | cancelled
execution_skill: dorkpipe-objective-execution
execution_authority: approved_objective_creation
authorized_objective: <bounded outcome, larger than one mechanical edit>
done_when: <observable completion proof>
inherited_invariants: <facts that must remain true>
explicit_exclusions: <forbidden or independently gated work>
checkpoint_policy: automatic_within_objective
verification_policy: <focused checks plus terminal proof>
handoff_policy: transport_only
context_handoff_policy: automatic_at_safe_boundary | ask_before_transport
context_handoff_limit_per_task: 1
checkpoint_output_policy: quiet_success_bounded_failure
terminal_conditions: completed | blocked | failed_verification | cancelled
```

Objective creation or an explicit user request to begin the named objective authorizes ordinary,
reversible implementation and validation needed to reach `done_when`. It does not authorize a
one-shot gate, destructive cleanup, commit, push, publication, cost, credential use, or external
resource mutation unless that authority is separately explicit.

Inventory branch, HEAD, staged, unstaged, and untracked state. Identify user-owned changes and the
paths the objective owns. Preserve unrelated bytes. Stop `blocked` when ownership overlaps cannot
be resolved safely.

## Receive a continuation

For `continue_objective` or `resume_objective`, treat the handoff's durable completed proof as the
admitted baseline. Revalidate affected live anchors, but do not reconstruct completed chronology,
rerun passed proof, or reopen completed implementation unless drift, a new failure, or the pending
checkpoint directly requires it.

The fresh task starts with this receiver budget:

- own the single `Pending boundary` as the first checkpoint;
- admit supporting work only when it is strictly required to complete that checkpoint;
- after that checkpoint, update durable state and reassess context pressure before selecting another
  seam, loading materially different guidance, or starting broad terminal verification;
- treat the source task's transport as consumed there, not as the receiver's transport. Unless an
  explicit objective-wide chain limit says otherwise, `context_handoff_limit_per_task: 1` gives
  each fresh task one outgoing context-saving handoff of its own.

Never turn `transport_limit: 1`, `source_transport_consumed: true`, or prose such as "second
handoff" into an objective-wide ban. If the handoff packet is ambiguous, preserve the per-task
meaning and do not manufacture a chain limit.

## Classify each action

Before mutation, classify the next action as an objective checkpoint or a one-shot gate.

An **objective checkpoint** is all of the following:

- inside `authorized_objective` and outside `explicit_exclusions`;
- ordinary offline or read-only work, or a reversible local mutation;
- does not consume a separate approval, nonce, capability, promotion, or attempt;
- can be retried safely under the objective authority.

A **one-shot gate** is any action that meets one or more of these conditions:

- explicitly single-use, no-retry, or attempt-limited;
- consumes a separate approval, nonce, capability, promotion, or publication authority;
- mutates external state with material cost, destructive effect, difficult rollback, or credential-bearing authority;
- the governing contract says invocation consumes authority regardless of result.

Read-only diagnostics and ordinary source edits do not become gates merely because credentials or
sensitive code exist nearby. If retry safety or authority consumption is genuinely uncertain,
classify the action as a gate and stop before invoking it.

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

At a safe boundary, use `dorkpipe-task-handoff` with mode `continue_objective` when any hard signal
or at least two soft signals are present, the continuation packet can preserve all authority and
protected state, and meaningful work remains. Do not hand off during a mutation, running command,
or incomplete read-back. Do not hand off when the objective is close enough to finish safely in the
current task: "close enough" means only bounded terminal proof remains. Open-ended seam discovery,
another broad audit, or a materially different implementation checkpoint is not close enough. Do
not hand off when the user prohibited task creation or merely because the conversation feels long.

In a receiving task, reassess these signals immediately after the carried first checkpoint and
before broad terminal verification. When the threshold is met, handoff takes priority over selecting
another seam or running a broad suite.

For a user-approved objective, default `context_handoff_policy` to
`automatic_at_safe_boundary` unless the user requires confirmation. This policy authorizes one
transport-only context handoff per task; it grants no new execution scope. Record the triggering
signals in the handoff and stop the old task immediately after successful creation.

If safe continuation requires handoff but an explicit objective-wide chain limit is exhausted,
checkpoint durable state and stop `blocked` on missing transport authority. Do not force the entire
remaining objective through an overloaded task.

## Enter a one-shot gate

The objective controller never invokes a gate. When the next necessary action is a gate:

1. Complete all safe prerequisite checkpoints that remain inside the objective.
2. Seal the exact gate artifact and current anchors.
3. Run only authorized non-consuming readiness checks.
4. Record `state: waiting_for_gate` and a gate packet for `dorkpipe-one-shot-gate`.
5. Request the exact missing gate approval if it has not already been granted.
6. Use `dorkpipe-task-handoff` with mode `enter_one_shot_gate` after approval, then stop this task.

The gate packet must preserve the active `objective_id`, objective authority, resume state, exact
action, artifact hashes, preflight coverage, attempt count, and return contract. Gate creation
preauthorizes exactly one transport-only return handoff to the same active objective; it does not
authorize another gate.

## Resume after a gate

For handoff mode `resume_objective`, verify the gate receipt, read-back, terminal classification,
and that its authority is spent or explicitly unconsumed. Restore the same objective contract and
continue from its recorded resume state. The return is not a new objective or a request to choose a
successor slice.

If the gate failed after consumption, decide from the existing objective contract whether remaining
local diagnosis is authorized. Never retry the gate, invent fallback authority, or perform cleanup
under objective authority.

## Hand off without fragmenting work

Use `dorkpipe-task-handoff` only when the user requests a fresh task, context pressure makes a safe
continuation materially clearer, a one-shot gate must run separately, or a genuine blocker requires
another owner. Ordinary checkpoint completion is never a handoff trigger.

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
whether transport or gate authority was used. Follow repository Git policy; never infer commit or
synchronization approval.
