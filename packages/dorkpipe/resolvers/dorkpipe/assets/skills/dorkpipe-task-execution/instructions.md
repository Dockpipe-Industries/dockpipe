# DorkPipe Task Execution Compatibility Router

Translate a legacy `dorkpipe-task-execution` contract once, then invoke the specialized lifecycle
skill. Do not execute implementation, validation, or live action inside this router.

## Route once

1. Read the exact legacy contract and its authority state.
2. If `One-shot readiness.required: true`, or the authorized action is single-use, no-retry,
   authority-consuming, materially costly, destructive, externally mutating, or credential-bearing,
   translate it to a sealed gate packet and invoke `dorkpipe-one-shot-gate`.
3. Otherwise translate it to an objective contract and invoke `dorkpipe-objective-execution`.
4. Record that compatibility routing occurred and do not invoke this router again in the same chain.

An exact legacy slice becomes one objective checkpoint unless the legacy task or source-controlled
record already grants a broader bounded objective with observable `done_when`. Do not infer broad
authority from backlog context, suggested successors, or historical approvals.

Legacy task creation remains valid authority only for the exact work it originally authorized. This
router cannot upgrade ordinary authority into a gate approval, revive spent authority, add retry or
cleanup permission, or convert carried prose into execution authority.

## Prevent the old loop

- Do not require a fresh approval or task after an ordinary translated checkpoint when the admitted objective remains incomplete.
- Do not recreate `top_level_successor_limit: 1` as the objective lifecycle.
- Do not invoke `dorkpipe-task-handoff` merely because the translated checkpoint completed.
- Do not route a one-shot gate back through this compatibility skill; return it directly to `dorkpipe-objective-execution`.
- Do not execute both specialized skills for one action.
- Preserve a carried per-task context-handoff allowance as the fresh objective task's allowance;
  never reinterpret the source task's consumed transport as an objective-wide chain limit.

If classification is uncertain, stop before mutation and require the smallest missing authority fact.
After routing, the selected specialized skill owns all terminal reporting.
