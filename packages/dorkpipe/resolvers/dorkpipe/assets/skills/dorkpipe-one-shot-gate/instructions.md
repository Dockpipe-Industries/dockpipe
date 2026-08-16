# DorkPipe One-Shot Gate

Execute one exact gate at most once, perform its bounded read-back, and return control to the active
objective. This skill is an authority-consumption boundary, not a general execution loop.

## Admit the gate

Require a sealed packet before invocation:

```text
Gate contract:
gate_id: <stable id>
objective_id: <active objective id>
state: ready_for_execution
execution_skill: dorkpipe-one-shot-gate
execution_authority: approved_gate_task_creation
authorized_action: <one exact invocation>
artifact_sha256: <exact sealed bytes or not_applicable>
anchors: <execution checkout and distinct source or promotion pins>
non_consuming_preflight: <exact current command and result or why impossible>
coverage: <every predicate before first mutation>
unverified_predicates: none | <exact list>
attempt_count: 0
authority_consumed: false
read_back: <bounded result verification>
return_mode: resume_objective
return_authority: approved_with_gate_task_creation
return_limit: 1
objective_resume_state: <same objective contract and next checkpoint>
terminal_conditions: completed | failed_verification_before_consumption | consumed_failed | blocked
```

Task creation authorizes only `authorized_action`, one bounded read-back, and one transport-only
return to the same objective. It does not authorize repair, retry, fallback, cleanup, another gate,
commit, push, or scope expansion.

Reject the packet before invocation when the action is an ordinary retryable objective checkpoint,
the objective identity is missing, approval is only carried prose, or the exact action and current
anchors are not sealed. Return it to objective execution without consuming authority.

## Re-prove readiness

Immediately before invocation:

1. Revalidate the exact gate artifact, current anchors, attempt count, and unconsumed authority.
2. Keep execution-checkout, promotion-source, and other checkpoint roles distinct; do not assume their commits must match.
3. Re-run the exact non-consuming preflight when one exists. It must cover every predicate before the first mutation without creating identity material or exposing secrets.
4. Treat compilation, schema validation, hashes, narrow diffs, predecessor success, and spot checks as integrity evidence only.
5. If any predicate is missing, stale, or fails, stop before invocation.

Use `failed_verification_before_consumption` when a required current preflight fails without starting
the action. Use `blocked` when authority, ownership, state, or proof is missing. In both cases keep
`authority_consumed: false` and return that fact explicitly.

## Invoke once

Mark `authority_consumed: true` and increment `attempt_count` as the exact invocation begins,
regardless of its eventual exit status. Invoke only `authorized_action` once.

Never retry under the same or reconstructed authority. Never repair and reinvoke, switch to a direct
provider path, consume a second controller, apply cleanup, or reinterpret a partial result as unused
authority. A crash, timeout, unknown result, or transport failure after invocation begins is
`consumed_failed` until read-back proves a more specific completed outcome.

## Read back

Perform only the bounded, non-mutating read-back named by the contract. Preserve sanitized public
evidence: identifiers, hashes, modes, classifications, timestamps, and presence or absence. Keep
credentials, private identity, and resolved secret values opaque.

Classify the result:

| Result | Gate state | Authority |
| --- | --- | --- |
| Invocation and required read-back prove the expected result | `completed` | consumed |
| Current preflight fails before invocation | `failed_verification_before_consumption` | unconsumed |
| Invocation begins but result fails or remains unknown | `consumed_failed` | consumed |
| Required authority, ownership, or current state is missing before invocation | `blocked` | unconsumed |

## Return to objective execution

If the objective is still active, `return_mode: resume_objective` is mandatory. Use
`dorkpipe-task-handoff` exactly once with the preauthorized transport-only return. Carry the gate
receipt, read-back, terminal state, attempt count, and spent or unconsumed authority together with
the unchanged objective contract and resume state.

The return handoff needs no new micro-slice approval because it grants no new execution authority.
It may only resume the same objective. If host task creation is unavailable, emit the exact
paste-ready `resume_objective` prompt and stop.

Never hand off directly to another gate. The objective controller must receive and classify the gate
result before any later gate can be proposed or approved.
