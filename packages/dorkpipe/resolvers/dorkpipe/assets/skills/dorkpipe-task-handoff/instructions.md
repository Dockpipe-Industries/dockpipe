# DorkPipe Task Handoff

Move an existing objective into one fresh task when the user explicitly requests it. Handoff is
transport only: it preserves authority, scope, evidence, and dirty-tree ownership but never chooses
a new objective or creates execution authority.

## Choose one mode

Every handoff declares this mode:

| Mode | Source state | Fresh task skill |
| --- | --- | --- |
| `continue_objective` | User requested continuation, including after a context-pressure warning | `dorkpipe-objective-execution` |

Do not hand off after each objective checkpoint. Do not use handoff to rerank a backlog, invent the
next slice, shrink `done_when`, broaden exclusions, or turn context into authority.

## Capture current state

Re-read the minimum live state needed by the fresh task:

- exact checkout, branch, HEAD, and staged, unstaged, and untracked ownership;
- lifecycle mode, stable objective id, current state, authority status, and next checkpoint;
- `authorized_objective`, `done_when`, invariants, exclusions, and terminal policy;
- completed proof, failed checks, generated artifacts, and protected bytes;
- authorized external actions, current readiness, observed effects, safe retry evidence, and read-back.

Never infer cheap current state from conversation memory. Never copy secrets, private keys, access
tokens, or resolved credentials; carry opaque references and sanitized hashes only.

## Preserve authority

- Objective authority survives `continue_objective` until the objective terminates or the user cancels it.
- Preserve exact explicit authority already granted for external, destructive, publishing, costly,
  credential-refreshing, or otherwise consequential actions; do not widen it during transport.
- Preserve effect and retry evidence. A failed preflight or proven zero-effect attempt remains
  recoverable in the objective; partial or unknown effects still require read-back before retry.
- Handoff never grants commit, push, cleanup, publication, cost, credential, retry, or external
  resource authority that the source objective did not already have.

## Write the continuation prompt

Use the common envelope:

```text
Continue directly in the saved checkout: <absolute cwd>. Do not create a worktree unless explicitly requested.

Handoff contract:
mode: continue_objective
transport_authority: user_requested
objective_id: <stable id>
objective_authority: <state>
objective_state: <state>
context_pressure_signals: <why handoff was offered, or not_applicable>

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

Apply `dorkpipe-token-optimization` when available. Keep one canonical statement per fact and make
the prompt self-contained without replaying chronology. Target 500-900 words unless a protected-state
inventory genuinely requires more. Prefer counts plus a digest and only boundary-relevant paths over
full inventories or per-file hashes.

## Set the receiver contract

The user request authorizes creation of exactly one fresh task for this handoff. It does not impose
a permanent transport limit on the objective or authorize automatic future handoffs. A later
handoff requires another user request.

Require the fresh task to execute the pending boundary before expanding. Completed proof is
admitted, not replayed; only drift, new failure evidence, or a direct dependency of the pending
checkpoint justifies reopening it.

## Create and stop

- When the user explicitly requests handoff or continuation, create one fresh task without a second confirmation.
- Use the host-native task capability, the same project and saved checkout, and no worktree unless requested.
- After successful creation, report the new task and stop the old task.
- If task creation is unavailable, return the exact paste-ready prompt and state that it was not created.

Never invoke the fresh task's execution skill in the old task. Never create more than one task from
one user request. Never claim that ephemeral browser or agent-local state will survive task
transport; record how the receiver can reclaim and verify any state it still needs.
