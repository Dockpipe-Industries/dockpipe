# Direct conditional tests in terminal trees

## Approved objective

- Objective: `TASK-021-terminal-conditional-tests`; state: `completed`.
- Founder selected A and separately said exact `approved` in this task.
- Baseline: clean saved `js/pipelang` at `bf65af2dfc6918bf169575b6011966337f7f6390`.
  All 43 predecessor source hashes match; v0.102 completion proof is admitted.
- Scope: v0.103.0 admits `if (a ? b : c) { ... } else { ... }` at any subset
  of condition positions in public pure terminal trees through statement depth three.
  Operands are inherited nonconditional bool expressions, including supported pure
  calls and earlier lexical bindings. Omitted visibility retains the public default.
- Evaluate a once, then only selected b/c, then only the selected statement branch.
  Reached locals remain eager, once-only and ordered, including unused bindings.
- Preserve every v0.102 form, lexical rules, AST/HIR/Core nodes, public identities,
  frozen 45-source compatibility, target-neutral Core and package/engine boundaries.
- Exclude nested ternaries within the new test, hidden local declarations within
  condition expressions, deeper trees, new return/initializer/argument/matching/
  propagation forms, private methods, inference, mutation, loops, effects, backends
  and performance research.
- Done when source and independent Core admission/refusal, evaluator/generated Go/
  executable Application IR, all 25 shapes with bounded placement and independent
  value/lazy-trace oracles, computed operands, lexical refusals, all 102 earlier
  source/Core contracts, fresh discovered compiler and affected integration/CLI/vet/
  editor/docs checks pass. Scaling retains 128 MiB / 5-second warm direct compiler
  ceilings, normal inlining and execution GC, through 256 preceding locals.
- Validation uses cached offline Go 1.25.13 and canonical containment: 30-second
  units, 25-second test children, 700 MiB reclaim, 1 GiB hard, 800 MiB proactive
  stop, zero swap, 128 pids, at most two units. Python uses `-B`.
  Harness compilation alone may use the admitted `GOMEMLIMIT=600MiB` preparation.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints,
  handoff only on user request. No commit, push, cleanup, worktree, stash mutation,
  generated-store refresh, installation, credentials or live operations are authorized.
- Preserve existing receipts, caches and ignored state, including current predecessor
  bytecode SHA256 `e631831e38539041b7982cb3553e227155f1b607b1970b04f98747534a07f26c`.

## Evidence

Task-owned receipts: `/tmp/pipelang-v103-proof`. Implementation and bounded verification are complete.


## Completion and reproduction

All 726 freshly discovered compiler tests, fuzz seed
functions and examples pass in 2169 contained units in `terminal2`,
with unchanged source. Twelve new test functions occupy 222 units,
including 200 layout partitions and twelve scaling partitions.

All 25 terminal statement shapes and all 722 subsets of their actual condition
positions pass, including 25 inherited zero-new-test controls. Each layout supplies
4,096 vectors: three independent boolean selector triples, one per statement depth,
and a fourth independent triple shared by local/return expressions. Nodes at the
same depth share their selector triple. Not every input is consumed in every layout.
The resulting 2,957,312 supplied evaluator/Go values and
ordered/lazy Go traces are bounded matrices, not arbitrary independent assignments
at every node. Layouts rotate no locals, one local, and mixed ordinary/selector locals
with an eagerly executed unused tail; ordinary, depth-three and flat-selector returns
also rotate. Repeated HIR/Core/semantic/Go artifacts are deterministic.

Thirty-two computed/dependent vectors across four strings prove earlier lexical bool
and string bindings, lazy pure calls, selected branches and eager unused locals.
Forty-eight computed checked-Result vectors include overflow and whole-carrier transport.
The inherited independent type/carrier oracles exercise twelve supported type families
and nine carrier/host-value families through the new statement-condition placement.
All 102 earlier source and Core versions reject the new form at all three depths.
Malformed types, operands, nesting, identities, references, branches and depths are
refused. Generic internal Core locals remain valid while public placement rejects
hidden locals around the condition or any of its operands at each statement depth;
validation, evaluation, preparation and backend refusal agree. Twenty-one inherited
source fixtures retain normalized HIR/Core/semantic artifacts and exact generated Go;
checked-propagation and chain inheritance also extend through v0.103.

All 216 scaling regression cases and fresh isolated measurements pass. Every case
contains a new conditional statement test: 24 zero-local controls and 192 cases with
1/8/16/24/32/64/128/256 preceding locals, crossing four initializer-arm families,
three test depths and used/unused final bindings. Routing and initializer inputs are
shared within each scaling case. Peak isolated compiler RSS is
71.2578125 MiB; maximum elapsed time is
0.13503029994899407 seconds. The 128 MiB / 5-second warm direct compiler
ceilings, normal inlining and normal execution GC settings are unchanged. Closure
nesting never grows beyond each family's one-local baseline.

Complete Core/HIR/evaluator/backend, executable Application IR, frozen 45-source
compatibility, affected application, complete CLI and compiler/consumer/application/
CLI vet pass. The consumer checks canonical Application IR and helper outcomes across
six strings and eight selector vectors. Editor assertions/syntax, authored JSON/YAML/
Python, documentation links, formatting and planner checks pass. The eight changed
production files contain only v0.103 admission additions and version inheritance;
removing those additions restores the committed baseline exactly. Evaluator and Go
backend production code, AST/HIR/Core node shapes, public identities and generic
engine/package boundaries remain unchanged.

All 2623 recorded temporary cgroups are removed, including
preparation failures and the partial suite. There are no max/OOM or swap-event
increases and zero observed swap. Existing containment limits are unchanged.
Harness builds alone used the admitted `GOMEMLIMIT=600MiB` preparation target;
tests and isolated compiler measurements use their normal GC settings.

Reproduction uses `python3 -B tests/containedexec/pipelang_suite.py` with the cached
Go 1.25.13 binary, `/tmp/pipelang-performance-proof/cache`,
`/tmp/pipelang-execution-performance/complete-artifacts`, `--shape-batch-size 1`,
`--workers 2`, `--audit-generated` and a fresh output directory. Use the suite's
`fixtures-v103` with `python3 -B tests/containedexec/matrix.py`, a fresh output,
the same cache and the cached toolchain's direct compiler. Cold enlarged test-harness
compilation may first use canonical `run.py` and the harness-only GC target above.
Do not change execution limits or remove prior receipts/caches to rerun the proof.

Key retained receipts under `/tmp/pipelang-v103-proof`: `terminal2/summary.json`,
`terminal2/suite.json`, `terminal2/source-hashes.json`, `terminal2/fixtures-v103`,
`isolated/matrix.json`, `production-inheritance-audit.json`, `integration-final.json`,
`docs-checks.json`, `planner.json`, `final-evidence.json` and
`final-source-hashes.json`. This durable completion record owns acceptance if
these temporary receipts expire.

## Retained preparation and correction evidence

The sandbox's initial launcher attempt could not reach the user systemd manager;
the reviewed canonical host invocation supplied containment. The first contained
build caught a misplaced visibility check in an interface-signature parser block;
that edit was corrected before semantic tests. The first complete-suite attempt
encountered historical unknown-version assertions naming newly supported v0.103.0.
Those test sentinels now use v0.999.0, retaining unknown-contract rejection. Its
partial receipts remain separate from `terminal2` and are not terminal acceptance.
The focused sentinel invocation initially used checkout cwd; rerunning from the
compiler package restored its testdata paths. The consumer's predecessor rejection
assertion was corrected to match its unchanged diagnostic wording. All corrections
are covered by the final checks above.

The saved checkout remains `js/pipelang` at
`bf65af2dfc6918bf169575b6011966337f7f6390`. Changes are unstaged and uncommitted.
Both protected stashes, the 108,167-path ignored inventory and its digest, and the
current predecessor bytecode hash match. Temporary fixtures, build outputs and
native/compiler cache entries were created; no repository generated store was refreshed.
Full repository build/CI/all-library tests, sustained fuzzing, interactive editor,
non-Linux containment and live operations were skipped. No successor is selected.
Commit, push, publication and cleanup remain separate decisions.
