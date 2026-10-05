# Nested value arms in terminal conditional tests

## Approved objective

- Objective: `TASK-021-nested-terminal-conditional-tests`; state: `completed`.
- Founder selected A and separately said exact `approved` for this v0.104.0 contract.
- Baseline: clean saved `js/pipelang` at `7d9849a60101d4601f8b8d0b83a478627cf8efba`.
  All 42 v0.103 source hashes match; completed predecessor proof is admitted.
- Admit `if (a ? (b ? c : d) : e)`, its false-arm counterpart, and nesting in
  both value arms at any subset of condition positions in public pure terminal
  trees through statement depth three. Omitted visibility retains the public default.
- Test expression depth is at most two. Every ternary condition and final arm is an
  inherited nonconditional bool expression, including supported pure calls and
  earlier lexical bindings. Ordinary and flat conditional tests remain available.
- Evaluate each reached condition once, only its selected value arm, then only the
  selected statement branch. Reached locals remain eager, once-only and ordered,
  including unused bindings. Preserve all v0.103 forms and lexical rules.
- Exclude selector nesting, deeper expressions or statement trees, hidden locals
  inside tests, new argument/return/initializer/matching/propagation placements,
  private methods, inference, mutation, loops, effects, backends and performance work.
- Preserve AST/HIR/Core nodes, public identities, target-neutral Core, Core-only Go,
  executable Application IR, frozen 45-source compatibility and engine/package boundaries.
- Done when independent source/Core admission and refusal, evaluator/generated Go/
  executable Application IR, all 25 statement shapes with bounded placement and lazy
  trace matrices, computed/lexical cases, all 103 earlier contract refusals, inherited
  forms, fresh discovered compiler and affected integration/CLI/vet/editor/docs checks pass.
- Scaling crosses zero controls and 1/8/16/24/32/64/128/256 preceding eager locals,
  used/unused bindings and all three test depths. Retain normal inlining and execution
  GC and the 128 MiB / 5-second warm direct compiler ceilings.
- Use cached offline Go 1.25.13 and canonical containment: 30-second units, 25-second
  children, 700 MiB reclaim, 1 GiB hard, 800 MiB proactive stop, zero swap, 128 pids,
  at most two units. Harness compilation alone may use `GOMEMLIMIT=600MiB` preparation.
  Python uses `-B`.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints;
  handoff only on user request. No commit, push, cleanup, worktree, stash mutation,
  generated-store refresh, installation, credentials or live operations are authorized.
- Preserve existing receipts, caches and ignored/generated state.

## Evidence

Task-owned receipts: `/tmp/pipelang-v104-proof`. Implementation and bounded verification are complete.


## Completion and reproduction

All 739 freshly discovered compiler tests, fuzz seed functions and examples pass
in 2,416 contained units in `terminal`, with unchanged source. Thirteen new test
functions occupy 247 units: 200 layout partitions, 36 scaling partitions and eleven
other admission, inheritance, type/carrier, computed-expression and refusal tests.

All 25 terminal statement shapes and all 722 subsets of their actual condition
positions pass, including 25 inherited zero-new-test controls. Each layout supplies
8,192 vectors: seven nested-test operand bits shared across nodes, three independent
statement-depth result flips, and three independent local/return bits. This produces
5,914,624 supplied evaluator/Go values and ordered/lazy Go traces. True-only,
false-only and both-arm nesting rotate by node and layout; non-new positions mix
ordinary and v0.103 flat tests. These are bounded matrices, not all independent
assignments or nesting-form combinations at every node. Repeated HIR/Core/semantic/
Go artifacts are deterministic. No-local, one-local and mixed eager-local sequences,
unused tails, and ordinary/depth-three/flat-selector returns rotate across layouts.

Thirty-two computed/dependent vectors across four strings verify prior bool/string
bindings, boolean operations, lazy pure calls and eagerly executed unused locals.
Forty-eight computed checked-Result vectors include overflow and whole-carrier
transport. Independent inherited type/carrier oracles exercise twelve supported type
families and nine carrier/host-value families through the new placement. Twenty-two
inherited source fixtures retain normalized HIR/Core/semantic artifacts and exact Go;
checked propagation and chain inheritance extend through v0.104.

All 103 earlier source and Core contracts reject all three nesting forms at all
three statement depths (927 version/form/depth pairs). Source refusals cover selector
nesting, depth-three test arms, ternaries hidden in calls/boolean operands, malformed
types, lexical violations, private methods and deeper statements. Core refusals cover
outer and nested payloads/operands/types, selector and arm nesting, identities,
references and branches. Hidden-local refusals cover ten positions at each statement
depth while generic internal Core locals remain valid; public validation, evaluation,
preparation and backend refusal agree.

All 648 scaling regression cases and fresh isolated measurements pass. The 36 families
cross all three nesting forms, all three statement depths and four initializer-arm
families, with used/unused tails and 0/1/8/16/24/32/64/128/256 preceding locals.
Every case contains the new test: 72 zero-local controls and 576 nonzero cases.
The bounded scaling input group is shared across locals and routing; it is not every
independent initializer assignment. Peak isolated compiler RSS is 71.48046875 MiB;
maximum elapsed time is 0.15198588802013546 seconds. Normal inlining, normal execution
GC and the 128 MiB / 5-second warm direct compiler ceilings remain unchanged.
Closure depth does not grow beyond each family's one-local baseline.

Complete Core/HIR/evaluator/backend, executable Application IR, frozen 45-source
compatibility, affected application, complete CLI and compiler/consumer/application/
CLI vet checks pass. The consumer checks canonical Application IR and helper outcomes
across six strings and eight selector vectors, with both-arm, true-arm and false-arm
tests placed through statement depth three. Editor assertions/syntax, authored
JSON/YAML/Python, new documentation links, formatting and planner checks pass.

The eight changed production files contain only v0.104 admission additions and
version inheritance; removing those additions restores the committed baseline
exactly. Evaluator and Go backend production code, AST/HIR/Core node shapes, public
identities and engine/package boundaries are unchanged.

All 3,086 recorded temporary cgroups are removed, including the earlier failed
consumer assertion. There are no max/OOM or swap-event increases and zero observed
swap. Harness builds alone used the admitted `GOMEMLIMIT=600MiB` preparation target;
tests and isolated compiler measurements retain normal GC settings.

Reproduction uses `python3 -B tests/containedexec/pipelang_suite.py` with the cached
Go 1.25.13 binary, `/tmp/pipelang-performance-proof/cache`,
`/tmp/pipelang-execution-performance/complete-artifacts`, `--shape-batch-size 1`,
`--workers 2`, `--audit-generated` and a fresh output directory. Use its
`fixtures-v104` with `python3 -B tests/containedexec/matrix.py`, a fresh output,
the same cache and the cached toolchain's direct compiler. Cold harness preparation
may first use canonical `run.py` with the harness-only GC target above. Preserve
existing receipts/caches and execution limits.

Key receipts under `/tmp/pipelang-v104-proof`: `terminal/summary.json`,
`terminal/suite.json`, `terminal/source-hashes.json`, `terminal/fixtures-v104`,
`isolated/matrix.json`, `production-inheritance-audit.json`, `integration-final.json`,
`docs-checks.json`, `planner.json`, `protected-state-check.json`, `final-evidence.json`
and `final-source-hashes.json`. This durable record owns acceptance if temporary
receipts expire.

## Retained correction and checkout evidence

The initial sandbox build could not connect to the user systemd manager; the reviewed
canonical host launcher supplied containment. The initial Application IR test expected
new wording for a predecessor rejection; its assertion was corrected to the unchanged
existing diagnostic. The final complete consumer suite passes after that correction.
No production change was needed for it. Earlier receipts remain retained separately.

The saved checkout remains `js/pipelang` at
`7d9849a60101d4601f8b8d0b83a478627cf8efba`. Changes are unstaged and uncommitted.
Both protected stashes, the 108,167-path ignored inventory and its digest, and the
predecessor bytecode hash remain unchanged. Temporary fixtures, build outputs and
native/compiler cache entries were created; no repository generated store was refreshed.
Full repository build/CI/all-library tests, sustained fuzzing, interactive editor,
non-Linux containment and live operations were skipped. No successor is selected.
Commit, push, publication and cleanup remain separate decisions.
