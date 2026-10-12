# Conditional result arms in terminal selector tests

## Approved objective

- Objective: `TASK-021-terminal-selector-value-arms`; state: `completed`.
- Founder selected A and separately said exact `approved` for v0.107.0.
- Baseline: clean saved `js/pipelang` at `986dc0d3c21c268afbdacb91807bef4baff04df3`.
  All 28 v0.106 source hashes, both stashes and ignored inventory match; completed proof is admitted.
- Admit `if ((a ? b : c) ? (d ? e : f) : (g ? h : i))`, with a flat ternary in
  either or both outer result arms, at any subset of test positions in public pure
  terminal trees through statement depth three. Omitted visibility remains public.
  Every named operand remains an inherited nonconditional bool expression, including
  supported pure calls and earlier lexical bindings.
- Reached operands evaluate lazily once; only the selected statement branch runs.
  Reached locals remain eager, once-only and source-ordered under inherited lexical rules.
- Preserve every v0.106 form, source -> typed HIR -> target-neutral Core -> evaluator/
  Core-only Go, executable Application IR, all nodes and public identities, frozen
  45-source compatibility, compiler resource ceilings and package/engine boundaries.
- Exclude conditional arms inside the inner selector (option B), further nesting in
  the new form, deeper statements, new return/initializer/argument/matching/propagation
  placements, hidden locals, private methods, inference, mutation, loops, effects,
  new backends and performance research.
- Done when independent source/Core admission/refusal, all 25 statement shapes and
  applicable position subsets, value/lazy-trace evaluator/generated-Go agreement,
  executable Application IR, all 106 earlier-contract refusals, fresh discovered
  compiler tests, scaling, inherited compatibility and affected integration/editor/docs pass.
- Retain 128 MiB / 5-second warm direct compiler ceilings, normal inlining and execution GC.
  Use cached offline Go 1.25.13 and canonical containment: 30-second units, 25-second
  children, 700 MiB reclaim, 1 GiB hard, 800 MiB proactive stop, zero swap, 128 pids,
  at most two units. Harness preparation alone may use GOMEMLIMIT=600MiB. Python uses `-B`.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints;
  handoff only on user request. No commit, push, cleanup, worktree, stash mutation,
  generated-store refresh, installation, credential change or live operation is authorized.
- Preserve receipts, caches, both protected stashes and existing ignored/generated state.

## Evidence

Task-owned receipts: `/tmp/pipelang-v107-proof`. Implementation and bounded verification are complete.


## Completion and reproduction

All 781 freshly discovered compiler tests, fuzz seed functions and examples pass
in 3,610 contained units under `terminal-complete`, with unchanged source. Fourteen
new functions occupy 648 units: 600 placement partitions, 36 scaling families and
12 admission/inheritance/refusal/type/carrier/computed checks.

The placement matrix covers all 25 statement shapes and all 722 subsets of their
actual condition positions for each of three arm families: true only, false only,
and both. All 2,166 layouts, including 75 inherited zero-new-test controls, exhaust
4,096 supplied vectors: nine independent selector/result-arm operands and three
independent statement-depth flips. The 8,871,936 supplied values agree between the
evaluator and pristine generated Go; instrumented Go agrees with an independent
ordered/lazy trace oracle. Local/return choices reuse some of those bits, and the
selector bits are shared across statement nodes. This is not every independent
assignment across nodes or locals. No-local, one-local and mixed eager/unused
sequences and inherited return forms rotate across layouts. Repeated HIR/Core/
semantic/Go artifacts are deterministic.

Computed/lexical cases cover 384 combinations of five independent inputs, four
strings and three statement depths, with pure calls, boolean operations, earlier
bindings, mixed inherited deep tests and eager unused locals. Another 48 checked-
Result cases cover lazy carrier transport, overflow and unused eager bindings.
Shared independent oracles cover 12 supported type families and nine carrier/host
families through the new placement. Twenty-five earlier fixtures retain normalized
HIR/Core/semantic and exact Go bytes. Checked propagation and chain inheritance
extend through v0.107.

All 106 earlier source/Core contracts reject all three new arm forms at each
statement depth: 954 version/form/depth combinations. Source refusals cover all
nine operand positions, types, lexical violations, private methods, deeper trees,
additional selector/value-arm nesting and unchanged return/initializer/argument
boundaries. Malformed Core covers the outer test, inner selector and both result
arms independently. Hidden locals at all 13 expression positions and all three
statement depths remain valid in generic internal Core but are rejected by public
admission, evaluation, preparation and backend generation.

All 648 scaling regressions and fresh isolated cases pass: three statement depths,
four initializer-arm families and three new test-arm families, with used/unused
tails and 0/1/8/16/24/32/64/128/256 preceding locals. All include a new arm form,
including 72 zero-local controls. Scaling inputs are shared across initializers
and routing; these fixtures measure growth, not independent operand exhaustiveness.
Local count does not increase closure depth beyond the family's one-local baseline.
Normal inlining, execution GC and 128 MiB / 5-second warm direct compiler ceilings
remain fixed. Isolated peaks are 71.79296875 MiB and
0.15324825095012784 seconds. All 3,888 exported fixture-file hashes remain
unchanged through isolated measurement.

Complete Core/HIR/evaluator/backend, executable Application IR, frozen 45-source
compatibility, affected application, full CLI and compiler/consumer/application/CLI
vet checks pass. The new Application IR consumer checks both-arm nesting at the
root and single-arm nesting at the second and third statement depths, preserving
the canonical projection and inherited helper value oracle. Editor assertions and
syntax, authored JSON/YAML/Python, new documentation links and formatting pass.
Planner checks preserve all 648 labels under disjoint group widths 1/4/25.

All eight production files contain only v0.107 admission and version-inheritance
additions; removing those additions restores HEAD exactly. Evaluator/backend
production, node schemas, public identities and generic package/engine boundaries
are unchanged. All 4282 recorded temporary cgroups are removed;
there are no max/OOM or swap-event increases and no observed swap.

Reproduce with `python3 -B tests/containedexec/pipelang_suite.py`, cached offline
Go 1.25.13, `/tmp/pipelang-performance-proof/cache`,
`/tmp/pipelang-execution-performance/complete-artifacts`, `--shape-batch-size 1`,
`--workers 2`, `--audit-generated` and a fresh output directory. Measure exported
`fixtures-v107` with `python3 -B tests/containedexec/matrix.py`, the same cache,
cached direct compiler and fresh output. Preserve receipts, caches and containment.

Key receipts under `/tmp/pipelang-v107-proof`: `terminal-complete/summary.json`,
`terminal-complete/suite.json`, `terminal-complete/source-hashes.json`,
`terminal-complete/fixtures-v107`, `isolated/matrix.json`, `fixture-hashes.json`,
`placement-summary.json`, `production-inheritance-audit.json`,
`integration-complete.json`, `docs-checks.json`, `planner.json`,
`protected-state-check.json`, `final-evidence.json` and `final-source-hashes.json`.
This durable record owns acceptance if temporary receipts expire.

## Retained corrections and checkout evidence

The sandbox could not reach the user systemd bus; reviewed host invocations used
canonical containment. The first full-suite preparation build reached its unchanged
30-second deadline, without OOM. A preparation-only `GOMEMLIMIT=600MiB` build passed;
a fresh complete suite then passed with normal execution GC and unchanged source.
The first new consumer assertion expected different predecessor diagnostic text;
correcting that test expectation restored the inherited wording, and the complete
consumer suite passed. Production code needed no correction for either issue.
Earlier receipts remain retained.

The saved checkout remains `js/pipelang` at
`986dc0d3c21c268afbdacb91807bef4baff04df3`. Changes are unstaged and uncommitted.
Both protected stashes, the 108,167-path ignored inventory and digest, and existing
containedexec bytecode remain unchanged. Temporary proof, fixture and cache outputs
were created; no repository generated store was refreshed. Full repository build/CI/
all-library tests, sustained fuzzing, interactive editor, non-Linux and live checks
were outside scope. No successor is selected. Commit, push, publication and cleanup
remain separate decisions.
