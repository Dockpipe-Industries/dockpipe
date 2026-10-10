# Conditional boolean selectors in straight-line returns

## Approved objective

- Objective: `TASK-021-conditional-boolean-selectors`; state: `completed`.
- Founder selected A and separately said exact `approved` on 2026-09-08.
- Baseline: clean saved `<checkout>`, `js/pipelang` at
  `0c8e3bd1ef202126699a53ff63498a3ad486b5ae`. All 34 v0.97 source hashes,
  both protected stashes and ignored inventory match; completed proof is admitted.
- Scope: v0.98.0 accepts `return (a ? b : c) ? x : y;` only in straight-line
  block methods, optionally after finite inherited explicitly typed immutable locals.
  The outer return condition is exactly one flat boolean ternary; its three operands
  and both outer value arms are inherited nonconditional expressions. All selector
  operands are bool; both outer arms have the exact same supported type/carrier.
- Preserve eager once-only ordered locals, including unused locals; evaluate `a`
  once, then only selected `b` or `c`, then only selected `x` or `y`. Preserve
  lexical scope, parser/typechecker -> typed HIR -> target-neutral Core -> evaluator/
  Core-only Go -> executable Application IR, all v0.97 forms, public identities,
  generic internal Core locals, frozen 45-source compatibility and engine boundaries.
- Exclude further nesting inside the new form, new argument/initializer/arrow/terminal
  condition or leaf-return placements, matching/propagation placements, inference,
  mutation, loops, effects, backends and performance research.
- Done when all eight independent selector vectors, computed conditions, types/carriers,
  ordered/lazy traces, source/Core refusals including all 97 earlier versions,
  deterministic HIR/Core/semantic/Go, fresh discovered compiler suite, affected
  integration/CLI/vet/editor/docs checks and scaling through 256 preceding locals pass.
- Validation: cached offline Go 1.25.13, existing private caches, canonical contained
  launcher; 30-second units, 25-second test children, 700 MiB temporary reclaim,
  1 GiB hard, 800 MiB proactive stop, zero swap, 128 pids, at most two units.
  Direct warm compiler ceilings remain 128 MiB / 5 seconds with normal inlining.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints;
  handoff only on user request. Implementation/docs/verification are approved.
  Commit, push, publication, worktree, stash mutation, cleanup, generated-store refresh,
  installation, credentials and external operations remain outside scope.

## Evidence

The approved v0.98.0 slice is complete. Receipts are retained under
`/tmp/pipelang-v098-proof`; this durable record owns acceptance if temporary files expire.

## Completion and reproduction

All 661 discovered compiler tests, fuzz seed functions and examples pass in 1,521
final contained units, including twelve new v0.98 tests. Production/test source remained
unchanged throughout this final run. Source and independent Core admission use existing
conditional nodes; evaluator and Go backend production code are unchanged. Removing only
v0.98 admission and the bounded selector gates restores all nine changed production files
exactly to the committed baseline, as recorded in `production-inheritance-audit.json`.

Six supplied layouts cross 0/1/4 preceding locals with used/unused final bindings.
Each exhausts eight selector vectors and eight shared initializer vectors: 384 evaluator/
pristine-Go values and instrumented-Go ordered/lazy trace checks. Mixed ordinary and
complete depth-three initializers reuse earlier locals. Repeated typed HIR, Core, semantic
projection and exact Go bytes are deterministic. Twenty-four computed-condition vectors
exercise calls, boolean operators, comparisons and earlier bool/string bindings. Forty-eight
computed checked-Result vectors preserve success/overflow behavior and unused eager locals.
Twelve type families and nine carrier/host-value families pass with and without locals.
This bounded matrix does not claim every independent initializer assignment at arbitrary
sequence length or an arbitrary Cartesian product of all supported expression families.

All 97 earlier source and Core versions independently reject the new placement. Source
refusals cover arrow/initializer/argument/terminal placements, recursive selector or value
arm choices, malformed bool/value types, lexical references and matching/propagation.
Malformed Core is independently rejected by validation, evaluator and backend. Valid
internal Core locals in all five operand positions pass generic `ValidateFunction` while
public `ValidateProgram` rejects their placement. Sixteen inherited source fixtures retain
normalized HIR/Core/semantic and exact Go bytes. Public `pipelang.compiler.v1`,
`pipelang.semantic.v1` and `dockpipe.application.v1` identities and node shapes are unchanged.

Executable Application IR retains its canonical projection and checks helper results
independently across six strings and eight boolean vectors. Core, evaluator, backend, HIR,
Application IR, frozen 45-source compatibility, affected application, complete CLI and
compiler/consumer/application/CLI vet pass. Editor assertions/syntax, authored JSON/YAML,
task routes, local documentation links, formatting and diff checks pass. Planner checks
prove four disjoint memory partitions with group widths one, four and twenty-five.

All 64 scale cases pass regression and fresh isolated direct compiler measurements.
They cross ordinary/depth-one/depth-two/depth-three preceding initializer families,
used/unused tails and 0/1/8/16/32/64/128/256 locals; the return directly contains the new
selector. Only one sequence grows and its three input bits are shared across initializers.
Normal Go inlining and the 128 MiB / 5-second direct warm compiler ceilings remain fixed.
Peak isolated compiler RSS is 74.22265625 MiB; maximum elapsed is
0.267298138 seconds. Regression maxima are
76.62890625 MiB / 0.227259597 seconds.
Observed closure depths are [2, 3, 4]; every family stays within its one-local baseline.
All 384 exported fixture file hashes remain unchanged across isolated measurements.

All 1621 recorded temporary cgroups are removed. No max/OOM or swap-event
increases occurred, and observed swap is zero. Aggregate peak across recorded workloads
is 701.96484375 MiB. Integration/suite units used cached offline Go 1.25.13,
existing private caches, canonical `run.py`, 30-second units, 25-second test children,
700 MiB temporary reclaim, 1 GiB hard, 800 MiB proactive stop, zero swap and 128 pids.
At most two units ran concurrently. Isolated direct compiler units used default controls.

Reproduce the final suite with `tests/containedexec/pipelang_suite.py`, cached Go 1.25.13,
`--cache /tmp/pipelang-performance-proof/cache`,
`--compiled-cache /tmp/pipelang-execution-performance/complete-artifacts`,
`--shape-batch-size 1 --workers 2 --audit-generated`, and a fresh output directory.
Then pass its `fixtures-v098` directory to canonical `tests/containedexec/matrix.py`
with the cached toolchain compiler and the same private cache. No custom compiler flags,
GC settings, dependency downloads, resource-limit changes or performance work are involved.
Sandbox user-manager preflight started no workload; reviewed host invocations used the
canonical containment launcher. Commands and measurements are retained in each receipt.

Key receipts: `terminal/summary.json`, `terminal/suite.json`, `terminal/source-hashes.json`,
`terminal/fixtures-v098`, `isolated/matrix.json`, `scale-fixture-hashes.json`,
`production-inheritance-audit.json`, `integration.json`, `docs-checks.json`,
`planner-checks.json`, `final-evidence.json` and `final-source-hashes.json`.
Initial focused failures exposed missing zero-local Optional/Result signature admission;
those paths were corrected and both failed families passed focused reruns and terminal proof.
The first consumer attempt expected different predecessor rejection text; the assertion
was corrected and the complete consumer suite passed. Failed receipts are retained and are
not terminal acceptance. Passed unrelated integration checks were not replayed.

The saved checkout remains `js/pipelang` at `0c8e3bd1ef202126699a53ff63498a3ad486b5ae`;
owned changes are unstaged and uncommitted. Both protected stashes remain intact. Ignored
inventory remains 108,167 paths with SHA-256
`b39314777ac271ecd1ea58975c53c69983912f932dd95ead0d909b4f98ca2bc2`.
This is an inventory check, not a content hash of caches. Existing caches and receipts
are retained; validated cache entries and task-owned temporary evidence/build/fixture
artifacts were created. No generated repository store was refreshed or cache cleaned.
Generic engine/package boundaries are preserved.

Full repository build/CI/all-library tests, sustained fuzzing, interactive editor,
non-Linux containment and live operations were not run. No successor is selected or
approved. Commit, push, publication and any successor remain separate decisions.
