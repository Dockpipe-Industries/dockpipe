# Depth-three value arms in terminal conditional tests

## Approved objective

- Objective: `TASK-021-depth-three-terminal-conditional-tests`; state: `completed`.
- Founder selected A and separately said exact `approved` for v0.105.0.
- Baseline: clean saved `js/pipelang` at `dcdceefb461c6cb51ba681214825f95013b3de6a`.
  All 28 v0.104 source hashes match; completed predecessor proof is admitted.
- Admit ternary value-arm nesting through expression depth three in either or both
  arms at any subset of terminal statement conditions through statement depth three.
  Every selector and final arm stays an inherited nonconditional bool expression,
  including supported pure calls and earlier lexical bindings. Public pure methods
  retain omitted visibility as the public default.
- Preserve once-only reached conditions, lazy selected value arms and statement
  branches, eager once-only ordered locals including unused bindings, and lexical rules.
- Preserve all v0.104 forms, source -> typed HIR -> target-neutral Core -> evaluator/
  Core-only Go, executable Application IR, AST/HIR/Core nodes and public identities,
  frozen 45-source compatibility, compiler limits and package/engine boundaries.
- Exclude selector nesting, depth-four tests, deeper statements, hidden locals in tests,
  new argument/return/initializer/matching/propagation placements, private methods,
  inference, mutation, loops, effects, backends and performance research.
- Done when independent source/Core admission and refusal, evaluator/generated Go/
  executable Application IR agreement, all 25 statement shapes with explicitly bounded
  expression-shape/placement/lazy-trace matrices, computed/lexical cases, all 104 earlier
  version refusals, inherited forms, freshly discovered compiler suite and affected
  integration/CLI/vet/editor/docs checks pass.
- Scaling crosses all three statement depths, used/unused locals and
  0/1/8/16/24/32/64/128/256 preceding locals. Retain normal inlining and execution GC,
  and 128 MiB / 5-second warm direct compiler ceilings.
- Use cached offline Go 1.25.13 with canonical containment: 30-second units,
  25-second children, 700 MiB reclaim, 1 GiB hard, 800 MiB proactive stop, zero swap,
  128 pids, at most two units. Harness preparation alone may use GOMEMLIMIT=600MiB.
  Python uses `-B`.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints;
  handoff only on user request. No commit, push, cleanup, worktree, stash mutation,
  generated-store refresh, installation, credentials or live operations are authorized.
- Preserve existing receipts, caches and ignored/generated state.

## Evidence

Task-owned receipts: `/tmp/pipelang-v105-proof`. Implementation and bounded verification are complete.


## Completion and reproduction

All 753 freshly discovered compiler tests, fuzz seed functions and examples pass
in 2,738 contained units under `terminal-final`, with unchanged source. Fourteen
new v0.105 test functions occupy 322 units: 200 placement partitions, 75 expression
partitions, 36 scaling families and eleven other admission/inheritance/refusal/
computed/type/carrier checks.

The placement matrix covers all 25 statement shapes and all 722 subsets of their
actual condition positions, including 25 inherited zero-new-test controls. The 21
expression shapes of exactly depth three rotate across new positions; ordinary,
flat and depth-two tests fill inherited positions. Each layout supplies 8,192
vectors: seven shared test-operand bits, three independent statement-depth flips
and three independent local/return bits. This yields 5,914,624 supplied evaluator/
pristine-Go values and ordered/lazy Go traces. No-local, one-local and mixed eager/
unused sequences and ordinary/depth-three/selector returns rotate across layouts.
This does not claim every independent expression shape or operand assignment at
all statement nodes. Repeated HIR/Core/semantic/Go artifacts are deterministic.

The separate expression matrix crosses all 25 expression shapes through depth three
with all three statement depths. Each case exhausts 32,768 assignments of fifteen
independent possible operand positions, checking evaluator and pristine-Go values
and instrumented-Go lazy traces. The 75 cases supply 2,457,600 vectors; positions
absent from a smaller shape are unused. Source emission and iterative route/trace
oracles are separate. Preceding ordinary statement conditions reach the new test.

Computed/dependent checks include supported pure calls, boolean operations, prior
bool/string bindings, nested selectors reached once and unused eager locals.
Thirty-two string/boolean vectors and 48 checked-Result vectors cover lazy results,
overflow and whole-carrier transport. Independent type/carrier oracles exercise
12 supported type families and nine carrier/host-value families through the new
placement. Twenty-three inherited source fixtures preserve normalized HIR/Core/
semantic and exact Go bytes; checked propagation and chain inheritance extend
through v0.105.

All 104 earlier source and Core contracts reject the 21 depth-three expression
shapes at all three statement depths: 6,552 version/shape/depth pairs. Source
refusals cover selector nesting, depth four, hidden conditional arguments/boolean
operands, types, lexical violations, private methods and deeper statements.
Malformed Core checks cover all six nested conditional nodes of a full depth-three
test at every statement depth. Hidden-local checks cover all 22 test-expression
positions at all three statement depths, preserving valid generic internal Core
locals while public validation, evaluation, preparation and backend reject them.

All 648 regression and fresh isolated scaling cases pass: three nesting forms,
three statement depths, four initializer-arm families, used/unused tails and
0/1/8/16/24/32/64/128/256 preceding locals. Every case contains the new test,
including 72 zero-local controls. The three supplied input bits are shared across
initializers and routing; this is not every independent initializer assignment.
Closure depth never grows beyond each family's one-local baseline. Normal inlining,
normal execution GC and 128 MiB / 5-second warm direct compiler ceilings remain fixed.
Final isolated peaks are 72.04296875 MiB and 0.128001970006153 seconds. All exported
fixture file hashes remain unchanged across isolated measurement.

Complete Core/HIR/evaluator/backend, executable Application IR, frozen 45-source
compatibility, affected application, full CLI and compiler/consumer/application/CLI
vet checks pass. The Application IR consumer preserves its canonical projection
and checks helper values across six strings and eight boolean vectors, with deep
value arms through all three statement depths. Editor assertions/syntax, authored
JSON/YAML/Python, new documentation links, formatting and planner checks pass.
Planner checks preserve all 322 new labels with disjoint shape group widths 1/4/25.

The eight changed production files contain only v0.105 admission and version
inheritance additions; removing those additions restores the committed baseline
exactly. Evaluator and Go backend production code, AST/HIR/Core nodes, public
identities and engine/package boundaries are unchanged.

All 3,409 recorded temporary cgroups are removed. There are no max/OOM or swap-event
increases and zero observed swap. Harness preparation alone used the admitted
GOMEMLIMIT=600MiB target; test execution and isolated compiler measurements retained
normal GC settings.

Reproduction uses `python3 -B tests/containedexec/pipelang_suite.py` with cached
Go 1.25.13, `/tmp/pipelang-performance-proof/cache`,
`/tmp/pipelang-execution-performance/complete-artifacts`, `--shape-batch-size 1`,
`--workers 2`, `--audit-generated` and a fresh output directory. Use its
`fixtures-v105` with `python3 -B tests/containedexec/matrix.py`, a fresh output,
the same cache and the cached direct compiler. Cold harness preparation may first
use canonical `run.py` with the harness-only GC target above. Preserve all receipts,
caches, normal execution settings and containment limits.

Key receipts under `/tmp/pipelang-v105-proof`: `terminal-final/summary.json`,
`terminal-final/suite.json`, `terminal-final/source-hashes.json`,
`terminal-final/fixtures-v105`, `isolated/matrix.json`, `fixture-hashes.json`,
`production-inheritance-audit.json`, `integration-complete.json`, `docs-checks.json`,
`planner.json`, `protected-state-check.json`, `final-evidence.json` and
`final-source-hashes.json`. This durable record owns acceptance if temporary receipts expire.

## Retained corrections and checkout evidence

The initial sandbox launcher could not connect to the user systemd manager; reviewed
host invocation supplied canonical containment. The first full-suite cold harness
build hit its 30-second deadline before any test ran, with no OOM/swap increases;
its cgroup was removed. Admitted harness-only preparation completed, then the full
suite passed in a fresh receipt directory under normal test settings.

The initial new Application IR assertion expected newer rejection wording. It was
corrected to the existing predecessor diagnostic, and the complete consumer suite
then passed. No production change was required. All earlier receipts are retained.

The saved checkout remains `js/pipelang` at
`dcdceefb461c6cb51ba681214825f95013b3de6a`. Changes are unstaged and uncommitted.
Both protected stashes, the 108,167-path ignored inventory and its digest, and the
existing containedexec bytecode hash remain unchanged. Temporary fixtures, build
outputs and native/compiler cache entries were created. No repository generated
store was refreshed. Full repository build/CI/all-library tests, sustained fuzzing,
interactive editor, non-Linux containment and live operations were outside scope.
No successor is selected. Commit, push, publication and cleanup remain separate decisions.
