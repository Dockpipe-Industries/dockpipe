# DorkPipe Task Handoff

Move existing lifecycle state into one fresh task. Handoff is transport only: it preserves authority,
scope, evidence, and dirty-tree ownership but never chooses a new objective or consumes gate authority.

## Choose one mode

Every handoff declares exactly one mode:

| Mode | Source state | Fresh task skill |
| --- | --- | --- |
| `continue_objective` | Active objective needs a user-requested or context-saving continuation | `dorkpipe-objective-execution` |
| `enter_one_shot_gate` | Objective is `waiting_for_gate` and the exact gate task is approved | `dorkpipe-one-shot-gate` |
| `resume_objective` | Gate reached a terminal state and the same objective remains active | `dorkpipe-objective-execution` |

Do not hand off after each objective checkpoint. Do not use handoff to rerank a backlog, invent the
next slice, shrink `done_when`, broaden exclusions, or turn context into authority.

## Capture current state

Re-read the minimum live state needed by the fresh task:

- exact checkout, branch, HEAD, and staged, unstaged, and untracked ownership;
- lifecycle mode, stable objective id, current state, authority status, and next checkpoint;
- `authorized_objective`, `done_when`, invariants, exclusions, and terminal policy;
- completed proof, failed checks, generated artifacts, and protected bytes;
- exact gate id, action, hashes, pins, non-consuming preflight coverage, attempt count, and read-back when applicable;
- authority classification: drafted, granted-unconsumed, consumed, expired, or replaced.

Never infer cheap current state from conversation memory. Never copy secrets, private keys, access
tokens, or resolved credentials; carry opaque references and sanitized hashes only.

## Preserve authority

- Objective authority survives `continue_objective`, `enter_one_shot_gate`, and `resume_objective` until the objective terminates or the user cancels it.
- Gate authority exists only for the exact approved action and is consumed when invocation begins.
- Spent gate authority returns as evidence, never as a reusable capability.
- Creating an approved gate task preauthorizes one `resume_objective` handoff to the same objective because that transport grants no new execution authority.
- A return handoff cannot enter another gate. The objective controller must classify any later gate and obtain its separate approval.
- Handoff never grants commit, push, cleanup, publication, cost, credential, retry, or external resource authority.

## Write the continuation prompt

Use the common envelope:

```text
Continue directly in the saved checkout: <absolute cwd>. Do not create a worktree unless explicitly requested.

Handoff contract:
mode: continue_objective | enter_one_shot_gate | resume_objective
transport_authority: <user_requested | objective_context_policy | approved_gate_task_creation | preauthorized_gate_return>
source_transport_limit: 1
source_transport_consumed: true
objective_id: <stable id>
objective_authority: <state>
objective_state: <state>
context_pressure_signals: <hard signal | at least two soft signals | not_applicable>
receiver_context_handoff_limit_per_task: 1 | <explicit override>

Objective contract:
<authorized objective, done_when, invariants, exclusions, verification and terminal policy>

Anchors and protected state:
<live checkout and ownership evidence>

Completed proof:
<durable facts only>

Pending boundary:
<next checkpoint or exact gate action; do not invent a successor>

Receiver budget:
<one first checkpoint; affected-anchor revalidation only; quiet focused proof; reassess before another seam or broad suite>

First action:
Invoke <specialized skill>, admit durable completed proof, revalidate only affected anchors, and execute the pending boundary first.

Hard stops:
<unrelated mutation and separate authority boundaries>
```

For `enter_one_shot_gate`, include the complete sealed gate contract after the objective contract.
For `resume_objective`, include the immutable gate receipt, terminal state, attempt count, read-back,
and consumed or unconsumed authority. Never compress `unverified` into `verified_current`.

Apply `dorkpipe-token-optimization` when available. Keep one canonical statement per fact and make
the prompt self-contained without replaying chronology. Target 500-900 words unless an exact sealed
gate contract or protected-state inventory genuinely requires more. Prefer counts plus a digest and
only boundary-relevant paths over full inventories or per-file hashes.

## Set the receiver contract

The source task consumes its one creation authority when it creates the fresh task. That consumption
does not consume the fresh task's own per-task context-handoff allowance. Carry
`receiver_context_handoff_limit_per_task: 1` when the objective policy is per-task, and omit any hard
stop such as "no second handoff." Add an objective-wide chain limit only when the user or objective
contract explicitly supplied one.

Require the fresh task to execute the pending boundary before expanding. After that checkpoint it
must update durable state and reassess context pressure before another seam, materially different
guidance set, or broad terminal suite. Completed proof is admitted, not replayed; only drift, new
failure evidence, or a direct dependency of the pending checkpoint justifies reopening it.

## Create and stop

- When the user explicitly requests handoff or continuation, create one fresh task without a second confirmation.
- For `continue_objective` with `context_handoff_policy: automatic_at_safe_boundary`, create one fresh task without another confirmation only after the objective skill records a hard signal or at least two soft signals and reaches a safe boundary.
- For `continue_objective` with `context_handoff_policy: ask_before_transport`, ask once before task creation.
- For `enter_one_shot_gate`, require exact gate approval before creation. Task creation authorizes only the sealed gate contract.
- For `resume_objective`, use the single return authority included with gate task creation; do not ask for another micro-slice approval.
- Use the host-native task capability, the same project and saved checkout, and no worktree unless requested.
- After successful creation, report the new task and stop the old task.
- If task creation is unavailable, return the exact paste-ready prompt and state that it was not created.

Never invoke the fresh task's execution skill in the old task. Never create more than one task from
one handoff authority. Never encode a consumed source transport as a consumed receiver transport.
