# Conditional result arms in straight-line selector returns

## Approved objective

- Objective: `TASK-021-straight-line-selector-value-arms`; state: `completed`.
- Founder selected A and separately said exact `approved` in this task.
- Baseline: clean saved `js/pipelang` at `605b249953ef5fed616b96324ef013f4b2d4491a`.
  Both protected stashes and all 108169 ignored paths match. Admit completed v0.109
  and both completed verification optimization objectives without baseline reruns.
- v0.110.0 admits `return (a ? b : c) ? (d ? x : y) : (e ? z : w);` with a flat
  ternary in either or both outer result arms (three forms), only in public pure
  straight-line block methods. Omitted visibility retains the public default.
  Zero or more finite inherited explicitly typed immutable locals may precede it.
- Selector operands are bool; result leaves have exactly matching supported types,
  including intact record, list, Optional and Result values without implicit propagation.
  Named operands are inherited nonconditional expressions, including supported pure
  calls and earlier lexical bindings. Reached selector operands and result arms execute
  lazily once; preceding locals are eager, once-only and source-ordered, including unused locals.
- Preserve every v0.109 form, lexical rules, AST/HIR/Core shapes, generic internal
  Core locals, public compiler/semantic/Application IR identities, executable Application IR,
  frozen 45-source compatibility and generic engine/package boundaries.
- Exclude new terminal-leaf/arrow/initializer/argument/matching/propagation placements,
  further nesting, private methods, inference, mutation, loops, effects and new backends.
- Done when independent source/Core admission and refusal, all three forms with exhaustive
  independent selector inputs, type/carrier coverage, independent value and lazy/order
  oracles versus evaluator and pristine/instrumented generated Go, computed operands,
  determinism, lexical checks, all 109 earlier-contract refusals, fresh complete compiler
  campaign, affected integration/editor/docs checks and fresh isolated scaling through
  256 preceding locals pass. Report bounded matrix coverage without Cartesian claims.
- Retain normal inlining/execution GC and 128 MiB / 5-second warm direct compiler ceilings.
  Use cached offline Go 1.25.13, durable receipts, existing aggregate and child containment;
  preparation alone may use GOMEMLIMIT=600MiB. Python uses `-B`.
- Execution skill: `dorkpipe-objective-execution`; implementation and verification are
  authorized, with automatic in-scope checkpoints and user-requested handoff only.
  No commit, push, cleanup, worktree, stash mutation, generated-store refresh, installation,
  credentials, machine configuration or external mutation is authorized.

## Evidence

Durable task root:
`<local-evidence>/2026/09/10/01a08c7e-4d58-71e0-a405-9addfc7d35ae`.
Baseline source identities are in `baseline.json`. Implementation and verification are complete; the completion section below owns terminal acceptance.

## Focused proof

`focused-job-5.json` and `focused-5/` record 52 passing logical units across fourteen
new compiler tests. The complete inventory discovers 825 functions. Eighteen layouts
cross three arm forms, 0/1/4 preceding locals and used/unused tails. Each exhausts
five independent selector bits plus three initializer bits: 4608 value and ordered/lazy
trace vectors. Initializer inputs are shared across preceding locals; this is not an
arbitrary Cartesian product of all operand families. All three forms also cross the
inherited typed/carrier matrices, with and without locals. Computed expressions,
checked success/overflow, deterministic HIR/Core/semantic/Go, all 109 earlier source
and Core contracts, malformed result arms and eleven hidden-local positions pass.

The twelve scaling families cross three arm forms and four preceding initializer
depths, used/unused tails and 0/1/8/16/32/64/128/256 locals. All 192 in-suite direct
compiler checks pass: maximum 78020 KiB RSS / 0.232611 seconds. Their three input bits
are shared across initializers and selector/result-arm conditions. Larger local counts
must not exceed each family's one-local closure baseline. Fresh isolated measurements
remain pending the terminal campaign.

The focused capped job completed in 69.49 seconds with 1200627712 bytes aggregate peak,
zero OOM/swap and complete cgroup removal. Five planner checks, five verification checks,
editor assertions and 251 local documentation links pass. Production evaluator and Go
backend implementations, AST/HIR/Core node shapes and public wire identities are unchanged.

Earlier focused attempts remain retained. They exposed a fixture-name error and missing
v110 source/HIR inheritance gates, now corrected. One negative Core mutation was an
already-valid v109 terminal tree; it is now a positive inheritance check. The initial
sandbox capability probe could not reach the user systemd bus; subsequent reviewed host
runs use the existing capped harness. No uncontained compiler lane was used.

`integration-job-1.json` records all nine integration checks and the editor check passing,
including the executable v110 consumer. It completed in 84.27 seconds with zero OOM/swap
and complete cgroup removal. The fresh complete campaign is running under `terminal-job`
with singleton scheduling; changed inputs do not inherit the preceding pairing profile.
It includes all 1944 inherited v109 isolated fixtures plus 192 new fixtures. After the
stage job completes, independent acceptance and final protected-state read-back remain.
`terminal-source-postimages.json` records the final source/test/editor identities.

## Completion

The fresh full campaign passed all 825 discovered functions in 6240 logical units,
including fourteen new functions in 52 units. Every logical case executed fresh with
zero retries; source and toolchain inputs remained unchanged. Singleton scheduling
retained the changed-input policy. Full-suite wall time was 5013.964 seconds; the
complete suite/matrix/integration job took 5530.145 seconds. No performance comparison
or cold-cache claim is made.

All 2136 fresh isolated compiler probes pass: 1944 inherited v109 cases plus 192
new v110 cases. Maximum waited compiler RSS was 76.953 MiB and maximum elapsed time
was 1.511375 seconds, below the unchanged 128 MiB / 5-second ceilings with normal
inlining and execution GC. All nine integration checks and the editor check passed
again in the terminal campaign. Executable Application IR remains canonical and
independently checks helper results, preserving frozen 45-source compatibility.

`accepted-verification.json` under `verification-data/campaigns/terminal/` records
acceptance. `terminal-comparison.json` independently confirms exact inherited order:
43725 source/fixture audits and 43725 fresh native children match the accepted
v109 proof. The independent audit completed in 183.350 seconds and verified every
owned terminal source postimage. The workload and audit job trees were removed;
both recorded zero OOM and swap events. Aggregate peaks were 1612759040 and
536920064 bytes respectively, under their unchanged caps.

`protected-state-final.json` confirms the original HEAD/branch, empty staging area,
both protected stash identities and all 108169 ignored paths with the same digest.
All compiler, test, editor and documentation changes remain uncommitted. The eight
production changes are admission/version inheritance only; evaluator and Go backend
implementations, node shapes, public identities and engine/package boundaries are
preserved. Durable receipts, native caches, compiler fixture exports and earlier
failed attempts are retained. No cleanup, commit, push, worktree, external mutation
or successor selection occurred. No required work remains in this objective.
