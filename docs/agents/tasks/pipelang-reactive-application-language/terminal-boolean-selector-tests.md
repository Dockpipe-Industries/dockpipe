# Flat boolean selectors in terminal tests

## Approved objective

- Objective: `TASK-021-terminal-boolean-selector-tests`; state: `completed`.
- Founder selected A and separately said exact `approved` for v0.106.0.
- Baseline: clean saved `js/pipelang` at `b29197d9e112df2e0301755f1f222cdbdf1e5357`.
  All 28 v0.105 source hashes and protected-state anchors match; completed proof is admitted.
- Admit `if ((a ? b : c) ? d : e)` in any subset of condition positions in public
  pure terminal `if/else` trees through statement depth three. Omitted visibility
  retains the public default. All five operands remain inherited nonconditional
  bool expressions, including supported pure calls and earlier lexical bindings.
- Evaluate a once, only selected b or c, then only selected d or e, followed by
  only the selected statement branch. Preserve eager once-only ordered locals,
  including unused bindings, and all lexical rules.
- Preserve all v0.105 forms, source -> typed HIR -> target-neutral Core -> evaluator/
  Core-only Go, executable Application IR, AST/HIR/Core nodes and public identities,
  frozen 45-source compatibility, compiler limits and package/engine boundaries.
- Exclude additional selector/value-arm nesting within this new form, deeper
  statements, hidden locals, new return/initializer/argument/matching/propagation
  placements, private methods, inference, mutation, loops, effects, new backends
  and performance research.
- Done when independent source/Core admission/refusal, evaluator/generated-Go/
  executable Application IR agreement, all 25 statement shapes and condition-position
  subsets, independent values/lazy traces, all 105 earlier contract refusals,
  inherited compatibility, fresh discovered compiler tests, affected integration,
  CLI, vet, editor and documentation checks pass.
- Scale across statement depths, used/unused locals and 0/1/8/16/24/32/64/128/256
  preceding locals. Retain normal inlining and execution GC, and 128 MiB / 5-second
  warm direct compiler ceilings.
- Use cached offline Go 1.25.13 and canonical containment: 30-second units,
  25-second children, 700 MiB reclaim, 1 GiB hard, 800 MiB proactive stop, zero swap,
  128 pids and at most two units. Harness preparation alone may use GOMEMLIMIT=600MiB.
  Python uses `-B`.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints;
  handoff only on user request. No commit, push, cleanup, worktree, stash mutation,
  generated-store refresh, installation, credential change or live operation is authorized.
- Preserve both protected stashes, existing receipts, caches and ignored/generated state.

## Evidence

Task-owned receipts: `/tmp/pipelang-v106-proof`. Implementation and bounded verification are complete.

## Completion and reproduction

All 767 freshly discovered compiler tests, fuzz seed functions and examples pass
in 2,962 contained units under `terminal-final`, with unchanged source. Fourteen
new v0.106 functions occupy 224 units: 200 placement partitions, twelve scaling
families and twelve admission/inheritance/refusal/type/carrier/computed checks.

The placement matrix covers all 25 statement shapes and all 722 subsets of their
actual condition positions, including 25 inherited zero-new-test controls. Each
layout exhausts 2,048 supplied vectors: five selector operands, three independent
statement-depth flips and three local/return operands. The 1,478,656 supplied
values agree between the evaluator and pristine generated Go; separately instrumented
Go agrees with an independent ordered/lazy trace oracle. No-local, one-local and
mixed eager/unused sequences and ordinary/depth-three/selector returns rotate
across layouts. The five selector bits are shared across statement nodes; this
is not every independent assignment across all nodes. Repeated HIR/Core/semantic/
Go artifacts are deterministic.

Computed/lexical cases exhaust five independent booleans across four input strings
and three statement depths: 384 cases with pure helper calls, boolean operations,
earlier bool/string bindings, mixed inherited depth-three tests and eager unused
branch locals. A further 48 checked-Result cases cover lazy result transport,
overflow and unused eager bindings. Shared independent type/carrier oracles cover
12 supported type families and nine carrier/host-value families through the new
condition placement. Twenty-four earlier source fixtures preserve normalized HIR/
Core/semantic and exact Go bytes; checked propagation and chain inheritance extend
through v0.106.

All 105 earlier source and Core contracts reject the new selector shape at each
statement depth (315 version/depth pairs). Source refusals cover each of the five
operand positions, additional selector/value-arm nesting, conditional arguments
and boolean operands, types, lexical violations, private methods, deeper statements
and unchanged initializer/return boundaries. Malformed Core checks cover the outer
conditional and its selector independently at all three statement depths. Hidden
local checks cover all seven expression positions at all three depths: generic
internal Core locals remain valid while public admission, evaluation, preparation
and backend reject hidden locals in public tests.

All 216 regression and fresh isolated scaling cases pass: three statement depths,
four initializer-arm families, used/unused tails and 0/1/8/16/24/32/64/128/256
preceding locals. Every case contains the new test, including 24 zero-local controls.
The three supplied bits are shared across initializers and routing; this is not every
independent initializer assignment. Local count never grows closure depth beyond the
family's one-local baseline. Normal inlining, normal execution GC and the 128 MiB /
5-second warm direct compiler ceilings remain fixed. Fresh isolated peaks are
71.61328125 MiB and 0.12805068399757147 seconds. All 1,296 exported fixture-file
hashes remain unchanged during isolated measurement.

Complete Core/HIR/evaluator/backend, executable Application IR, frozen 45-source
compatibility, affected application, full CLI and compiler/consumer/application/CLI
vet checks pass. The Application IR consumer preserves its canonical projection
and checks helper values across six strings and eight boolean vectors, with the
new selector tests at all three statement depths. Editor assertions/syntax,
authored JSON/YAML/Python, new documentation links, formatting and planner checks
pass. Planner checks preserve all 224 new labels with disjoint group widths 1/4/25.

The eight changed production files contain only v0.106 admission and version
inheritance additions; removing those additions restores HEAD exactly. Evaluator
and Go backend production code, AST/HIR/Core nodes, public identities and generic
engine/package boundaries are unchanged.

All 3,199 recorded temporary cgroups are removed. There are no max/OOM or swap-event
increases and zero observed swap. Harness preparation alone used GOMEMLIMIT=600MiB;
execution and isolated compiler measurements retained normal GC settings.

Reproduction uses `python3 -B tests/containedexec/pipelang_suite.py` with cached
Go 1.25.13, `/tmp/pipelang-performance-proof/cache`,
`/tmp/pipelang-execution-performance/complete-artifacts`, `--shape-batch-size 1`,
`--workers 2`, `--audit-generated` and a fresh output directory. Measure its
`fixtures-v106` using `python3 -B tests/containedexec/matrix.py`, a fresh output,
the same cache and the cached direct compiler. Preserve receipts, caches, normal
execution settings and containment limits.

Key receipts under `/tmp/pipelang-v106-proof`: `terminal-final/summary.json`,
`terminal-final/suite.json`, `terminal-final/source-hashes.json`,
`terminal-final/fixtures-v106`, `isolated/matrix.json`, `fixture-hashes.json`,
`production-inheritance-audit.json`, `integration-complete.json`, `docs-checks.json`,
`planner.json`, `placement-summary.json`, `protected-state-check.json`,
`final-evidence.json` and `final-source-hashes.json`. This durable record owns
acceptance if temporary receipts expire.

## Retained corrections and checkout evidence

The initial sandbox launcher could not reach the user systemd manager; a reviewed
host invocation supplied canonical containment. No uncontained compiler fallback ran.
The first new editor assertion referenced the wrong existing snippet variable; it
was corrected and the full assertion/syntax checks pass. The first new Application
IR assertion expected newer rejection wording. It was corrected to the inherited
predecessor diagnostic, and the complete consumer suite passed. Neither correction
required a production change. Earlier receipts remain retained.

The saved checkout remains `js/pipelang` at
`b29197d9e112df2e0301755f1f222cdbdf1e5357`. Changes are unstaged and uncommitted.
Both protected stashes, the 108,167-path ignored inventory and its digest, and the
existing containedexec bytecode hash are unchanged. Temporary fixtures, build outputs,
native/compiler cache entries and receipts were created; no repository generated
store was refreshed. Full repository build/CI/all-library tests, sustained fuzzing,
interactive editor, non-Linux containment and live operations were outside scope.
No successor is selected. Commit, push, publication and cleanup remain separate decisions.
