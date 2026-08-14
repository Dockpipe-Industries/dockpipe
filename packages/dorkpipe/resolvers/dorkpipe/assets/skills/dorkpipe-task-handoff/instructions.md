# DorkPipe Task Handoff

Create a high-fidelity handoff for one approved successor. Creating it authorizes execution of that exact slice in the fresh task; make the fresh task invoke `dorkpipe-task-execution` and stop the old task.

## Decide when to hand off

- Hand off immediately when the user explicitly requests a new task/chat or carry-over.
- Offer a handoff before starting a materially different next slice after a completed milestone, especially when the next action needs a new exact approval.
- Use conversation length only as a heuristic; do not claim access to an exact remaining-token meter.
- Do not interrupt a running command, live mutation, or incomplete read-back. Reach a safe boundary first.
- Do not hand off for a tiny follow-up that is safer and clearer to finish in place.

## Capture fresh state

Re-read the minimum live state needed for the next task:

- exact checkout, branch, HEAD, and worktree status;
- unrelated user-owned staged, unstaged, and untracked files;
- active task or decision record and the smallest routing docs;
- completed commits, validations, evidence, and generated-artifact paths;
- current blocker or next bounded outcome;
- exact hashes, IDs, paths, nonces, budgets, attempt counts, and stop rules;
- approval state: drafted, granted-but-unconsumed, consumed, expired, or replaced.

Never infer current state only from conversation memory when a cheap read-only check exists.

## Prove one-shot readiness

Before handing off an invocation that is single-use, no-retry, live, costly, destructive, or
credential-bearing:

1. Enumerate every predicate evaluated before the first mutation. A fail-fast repair must cover the
   remaining predicates, not only the first failure that was reached.
2. Require a repeatable non-consuming preflight from the exact hash-pinned artifact, or an exact
   checker proven to cover the same predicates. Compilation, schema validation, hashes, a narrow
   diff, and predecessor success prove integrity, not execution readiness.
3. Re-run that preflight against current state after the final artifact bytes are sealed. Record its
   command, artifact hash, result, covered boundary, and state anchors without exposing secrets.
4. Name checkpoint roles separately, such as `execution_checkout` and `promotion_source`. Never
   require them to be equal merely because both are Git commits.

If full non-consuming proof is unavailable, do not hand off direct invocation or describe the
artifact as execution-ready. Hand off a verification/repair slice instead and preserve the exact
unreached or unverified predicates.

## Compress without weakening

Apply `dorkpipe-token-optimization` when available. Summarize durable milestones instead of replaying the transcript.

- Aim for 500-900 tokens, excluding exact approval text or other protected payloads.
- Preserve hash-pinned approval text and exact required user responses verbatim.
- Never copy private keys, access tokens, credentials, or resolved secrets; carry references and hashes only.
- Preserve negative facts such as `not executed`, `not consumed`, `no retry`, and `cleanup not authorized`.
- Keep one canonical statement per fact; remove chronology that does not change the next action.
- Link focused repo docs instead of copying broad policy.
- Never compress away dirty-tree ownership, live-resource state, validation failures, or separate authority boundaries.

## Write the continuation prompt

Use this shape:

```text
Continue directly in the saved checkout: <absolute cwd>. Do not create a worktree unless explicitly requested.

Outcome:
<one bounded next result>

Lifecycle contract:
state: ready_for_execution
execution_skill: dorkpipe-task-execution
execution_authority: approved_task_creation
authorized_slice: <same bounded result>
inherited_invariants: <must remain true>
explicit_exclusions: <out of scope and forbidden actions>
acceptance_criteria: <proof required for completion>
terminal_conditions: completed | blocked | failed_verification
completion_policy: require_user_approval_before_successor | stop
top_level_successor_limit: 1

One-shot readiness:
required: true | false
status: verified_current | unverified | not_applicable
artifact_sha256: <exact hash or not_applicable>
preflight: <exact non-consuming command and result or why unavailable>
coverage: <all predicates before first mutation or not_applicable>
checkpoint_roles: <execution checkout, promotion/source, and other distinct owners>
unverified_predicates: none | <exact list>

Anchors and protected state:
<branch, HEAD, dirty-tree ownership, exact live or preserved resources>

Completed proof:
<durable milestones, commits, validations, evidence, and exact readiness-proof coverage>

Pending boundary:
<exact unresolved action, approval state, and why it has not run>

Exact required user response:
none — approving creation authorized this slice

First action:
Invoke `dorkpipe-task-execution`, validate the recorded readiness proof, then perform the authorized slice.

Hard stops:
<no retry, cleanup, unrelated mutation, authority transfer, or fallback>
```

Make the prompt self-contained and execution-ready. Do not make the fresh task reopen the full prior conversation.
Never turn `unverified` into `verified_current` through wording or compression.

The old task must receive `Approve next slice: <exact short slice label>` before this handoff is created. That approval authorizes one task creation and execution of only the named slice. It does not authorize any separately gated live action, retry, cleanup, commit, push, cost, credential, or resource operation. Never infer approval from a broad backlog or suggested follow-up.

## Preserve authority boundaries

- Treat carried approval text as context, never as authorization in the fresh task.
- State whether the old task's authorization was consumed.
- Require only separately gated live-action, retry, cleanup, commit, push, cost, credential, or resource approvals in the fresh task; do not re-ask for slice execution approval.
- Treat approved creation as execution authority only for the exact ordinary implementation and validation slice recorded in the lifecycle contract.
- Never widen it into live action, retry, cleanup, commit, push, cost, credential, or resource authority.
- If an approved action must finish before handoff, finish its exact read-back first or state that it remains unexecuted.

## Create the fresh task

- Treat explicit invocation of `dorkpipe-task-handoff` with one exact successor slice as approval to create and execute that slice. Do not ask for a second confirmation unless the request asks for prompt-only output or says not to create the task.
- Also treat an explicit `create/open/start the new task now` request naming one exact slice as approval to create the task and execute that slice there.
- Ask one Yes/No question that names the exact slice, `Create a fresh task and execute <slice>?`, only when proactively offering a handoff that the user did not invoke or request. A positive response is the creation and slice-execution approval.
- Put the recommended Yes option first when structured prompts are supported.
- On approval, use the host-native task/thread creation capability, target the same project and saved checkout, preload the continuation prompt, and do not create a worktree unless requested.
- After successful creation, report the new task and stop work in the old task.
- The fresh task, not this task, invokes `dorkpipe-task-execution`. Never invoke both skills as a same-session loop.
- If host task creation is unavailable, return the exact paste-ready prompt and say that creation could not be automated.

Do not continue the next slice in the old task after a successful handoff.
