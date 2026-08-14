# Serial Task Execution Lifecycle — Closed

Completed: 2026-08-09

## Shipped

- Added package-owned `dorkpipe-task-execution` as the execution counterpart to `dorkpipe-task-handoff`.
- Defined the gated serial lifecycle `approved task creation` to `ready_for_execution` to `executing`
  to exactly one of `completed`, `blocked`, or `failed_verification`.
- Made every fresh handoff explicitly invoke execution there while preserving the old session's stop
  boundary and a single top-level successor ceiling.
- Required completed work to propose one exact successor and wait for user approval before handoff.
  That approval authorizes one task creation and execution of only that slice; the successor does not
  ask again. Any later slice requires another end-of-slice approval.
- Added convergence rules that require changed implementation, new failure evidence, or an unresolved
  blocking invariant before repeating an audit or verification pass.
- Kept task artifacts as the system of record and preserved dirty-tree, verification, authority,
  source-control, and package/engine boundaries.

## Closure Evidence

The package test harness covers ten lifecycle scenarios: successful execution with and without a
successor, blocking invariants, failed verification, non-blocking audit repetition, unrelated dirty
work, insufficient authority, recursion prevention, the one-successor ceiling, and the old-session
stop rule. The existing skills renderer validates the source metadata and Codex/Claude/generic
adapters without a new engine primitive.

## Boundaries Preserved

- No engine, schema, external tracker, parallel top-level scheduler, commit, push, or live-resource
  behavior was added.
- A skill cannot itself guarantee that every host exposes task creation or automatically starts a
  fresh thread; unavailable host automation still returns the exact paste-ready handoff.
