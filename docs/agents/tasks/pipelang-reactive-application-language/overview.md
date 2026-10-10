# TASK-021 PipeLang foundation

## Current status

P04.a / v0.114.0 is the accepted language baseline. P04.b mutable locals and
definite assignment is implemented with focused checks, but proposed v0.115.0
remains **unaccepted**: coordinator headroom stops prevented complete terminal
verification and independent acceptance. No verification job remains running at
the recorded closeout. [Mutable locals](mutable-locals.md) owns the exact scope,
limits, failed receipts and remaining proof. Recheck current checkout state before
resuming; this backlog record does not create fresh operational authority.

The foundation specification and dependency plan are complete. The older
resumable-verification, Go fixture and native pilot objectives are completed
records, not a queue of pending implementation requests. The 30-second conformance
and strict storage targets remain unproven; completion of a bounded experiment
does not establish those targets.

## Scope and ownership

PipeLang owns the target-neutral language, typed HIR, Core IR, deterministic
execution and compiler contracts. TASK-020 owns application projections and target
adapters; TASK-022 owns service projections, backend/client generation and delivery.
Frameworks and platform-specific behavior stay outside language semantics.

The frozen v0.0.0.1 configuration lane and the separately versioned
`dockpipe.application.v1` projection are distinct from the evolving language
contract. See the canonical references for their exact boundaries:

- [Foundation requirements](../../../concepts/pipelang-foundation.md)
- [Foundation contracts](../../../concepts/pipelang-foundation-contracts.md)
- [Delivery plan and evidence ledger](../../../concepts/pipelang-foundation-delivery.md)
- [Compiler contract](compiler-contract.md)
- [Verification contract](../../../runtime/pipelang-verification.md)

## Completed evidence

[Progress history](history-overview.md) preserves earlier milestones and their
links. [Historical state](history-state.yaml) preserves the former router's
completed-objective metadata. Use the task-local [index](index.yaml) to select a
specific evidence branch; do not load all completed history for orientation.
