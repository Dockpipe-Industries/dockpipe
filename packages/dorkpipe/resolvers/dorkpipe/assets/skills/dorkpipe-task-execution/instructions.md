# DorkPipe Task Execution

Execute one authorized top-level slice to `completed`, `blocked`, or `failed_verification`. Fresh-task creation authorizes execution of that exact slice; approval of what comes next happens only after this slice terminates. Keep one authoritative session and converge without scope drift.

## Admit the task

Read the handoff or source-controlled task artifact, applicable `AGENTS.md`, focused routing docs, and live checkout state. Prefer a cheap read-only check over conversation memory.

Require or write this concise contract before mutation:

```text
Lifecycle contract:
state: ready_for_execution
execution_authority: approved_task_creation
authorized_slice: <one bounded result>
inherited_invariants: <must remain true>
explicit_exclusions: <out of scope and forbidden actions>
acceptance_criteria: <proof required for completion>
terminal_conditions: completed | blocked | failed_verification
completion_policy: require_user_approval_before_successor | stop
top_level_successor_limit: 1
```

For any one-shot action, also require:

```text
One-shot readiness:
required: true
status: verified_current | unverified
artifact_sha256: <exact hash>
preflight: <exact non-consuming command and result or why unavailable>
coverage: <all predicates before first mutation>
checkpoint_roles: <execution checkout, promotion/source, and other distinct owners>
unverified_predicates: none | <exact list>
```

For ordinary work, record `required: false` and `status: not_applicable` when the distinction matters.

If the fresh task lacks this exact slice or its creation was not approved, end `blocked` before mutation. Explicit creation approval authorizes ordinary implementation and validation only within `authorized_slice`; it does not consume or replace separately required live-action, retry, cleanup, commit, push, cost, credential, or resource authority. A carried approval, old capability, broad backlog, or suggested next step remains context only.

Inventory branch, HEAD, staged, unstaged, and untracked state. Identify user-owned changes and the files this slice owns. Do not revert, stash, overwrite, reformat, stage, commit, or publish unrelated work. Stop `blocked` when ownership overlap cannot be resolved safely.

## Gate one-shot consumption

Treat a single-use, no-retry, live, costly, destructive, or credential-bearing invocation as
unready until its `One-shot readiness` record is `verified_current` for the exact artifact bytes and
current state.

- Distinguish checkpoint roles. Validate the execution checkout against its pin and immutable
  promotion/source evidence against its own pin; never assume those commits must be equal.
- Re-run the exact non-consuming preflight immediately before invocation. It must exercise every
  predicate before the first mutation and must not create identity material, consume authority, or
  expose secrets.
- Treat compilation, schema validation, hashes, a narrow diff, predecessor success, or spot checks
  as integrity evidence only. They cannot replace complete readiness proof.
- When repairing a fail-fast artifact, validate the remaining pre-mutation predicates as well as the
  observed failure before sealing or proposing direct invocation.
- If the handoff lacks complete proof and the slice does not authorize readiness repair, stop
  `blocked` without consuming authority. If the exact non-consuming preflight fails, preserve its
  evidence and stop `failed_verification` without invoking the one-shot action.

Once the one-shot action itself begins, its authority is consumed regardless of exit status. Never
retry it under the same authority.

## Execute the slice

Set `state: executing` in the working contract or report, then:

1. Implement only `authorized_slice` while preserving inherited invariants and exclusions.
2. Keep source-controlled task artifacts current according to repository conventions; they are the system of record. Do not invent an external tracker.
3. Classify every finding as `blocking`, `required_for_acceptance`, or `non_blocking_deferred`.
4. Resolve blocking and required findings only within the authorized slice. Record deferred findings without implementing them.
5. Run the smallest sufficient validation for every acceptance criterion and preserve exact evidence. Never claim a check was run when it was not.

Internal bounded delegation or parallel investigation may be used when allowed, but reconcile it into this one authoritative top-level execution. Do not create competing sessions, branches of authority, or parallel backlog work.

## Force bounded convergence

Every repeated audit, review, hardening, or verification pass requires at least one material trigger:

- changed implementation relevant to the pass;
- new failure evidence; or
- an explicitly unresolved blocking invariant.

Before repeating a pass, state its trigger and the exact question it can resolve. A different phrasing of the same concern is not new evidence. If a pass produces no materially new blocking or acceptance-required evidence, stop repeating it, classify remaining findings as `non_blocking_deferred`, and decide the terminal state. Do not expand acceptance criteria through audit findings, perform speculative hardening, polish, unrelated cleanup, or continue because theoretical improvements remain.

Use judgment rather than a fixed pass count. Repeated evidence about the same unresolved condition without a new trigger means `blocked` or `failed_verification`, not another loop. Finish as soon as the authorized criteria and invariants are proven.

## Route the terminal state

| Scenario | Terminal state | Required action |
| --- | --- | --- |
| Acceptance passes; one exact successor is warranted | `completed` | Propose it and wait for `Approve next slice: <exact short slice label>`; only after that response invoke handoff once, then stop. |
| Acceptance passes; no successor exists | `completed` | Record proof and stop; do not invoke handoff. |
| A genuine invariant, authority, ownership, or human decision blocks safe progress | `blocked` | Record the exact blocker and required input; do not invoke handoff automatically. |
| Required one-shot readiness proof is missing, partial, or stale | `blocked` | Do not invoke; identify the exact proof or repair slice required. |
| Exact non-consuming preflight fails before invocation | `failed_verification` | Preserve evidence and stop without consuming the one-shot authority. |
| Required verification fails after the authorized implementation | `failed_verification` | Preserve failure evidence and stop; do not claim completion or invoke handoff automatically. |
| Repeated audit yields only non-blocking findings | Based on acceptance proof | Defer findings and terminate the audit loop. |
| Dirty tree contains unrelated user-owned changes | Based on owned slice | Preserve them and validate only the owned diff; block on unsafe overlap. |
| Handoff lacks authority to implement | `blocked` | Perform no mutation and request the missing exact authority. |
| Either skill would run again in the same session | Unchanged | Refuse recursion; execution runs only in the fresh successor and handoff runs at most once after completion. |
| More than one top-level successor is proposed | Unchanged | Create none until reduced to exactly one authorized successor. |
| Handoff succeeds | `completed` | Report the new task and stop the old session immediately. |

## Ask once, hand off once, or stop

After `completed`, identify at most one exact successor from the source-controlled task record. If none exists, stop. If one exists, report its bounded result, exclusions, acceptance criteria, and any separate approvals it will still require, then request exactly `Approve next slice: <exact short slice label>`.

That response authorizes exactly one invocation of `dorkpipe-task-handoff`, creation of exactly one fresh successor, and execution there of the named slice. It authorizes neither multiple successors nor any separately gated action. The handoff creates the fresh task with `state: ready_for_execution` and `execution_authority: approved_task_creation`; the fresh task invokes `dorkpipe-task-execution` without asking again. Never execute the successor slice in the completed session.

Before that exact response, do not invoke handoff or create the task. Multiple possible successors, `blocked`, `failed_verification`, or a human judgment requirement must present the decision and wait. After successful handoff, stop this session immediately.

Finish with the owned files, validation evidence, generated artifacts, deferred findings, terminal state, and whether a successor was created. Follow repository Git policy; do not infer commit or synchronization approval.
