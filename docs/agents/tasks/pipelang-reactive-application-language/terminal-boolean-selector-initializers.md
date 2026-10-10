# Boolean-selector initializers throughout terminal trees

## Approved objective

- Objective: `TASK-021-terminal-boolean-selector-initializers`; state: `completed`.
- Founder selected A and separately said exact `approved` in the receiving task.
- Baseline: clean saved `js/pipelang` at `bce96f06498413700934aa38d0aecc4815af8056`.
  All 40 v0.101 source hashes, protected stashes and ignored inventory matched.
- Scope: v0.102.0 admits `T value = (a ? b : c) ? x : y;` in any subset of finite
  explicitly typed immutable-local sequences at root, intermediate and leaf scopes
  of inherited terminal if/else trees through statement depth three. Public pure
  block methods include the inherited omitted-visibility public default.
- Selector operands are bool; arms exactly match the declared supported type. All
  five operands use inherited nonconditional expressions and earlier in-scope names.
  Complete primitive, record, list, Optional and Result values travel intact, without
  implicit propagation. Reached locals execute eagerly once in source order, including
  unused locals; evaluate a, selected b/c, selected x/y. Unreached branches do nothing.
- Bindings enter scope after initialization. Later locals, descendant conditions and
  inherited ordinary/depth-three/flat-selector returns may reuse them. Self/forward
  references, duplicates, shadowing and sibling/escaping references remain invalid.
- Exclude new statement-condition, argument, matching and propagation placements,
  further expression nesting, increased depth limits, private-method admission,
  inference, mutation, loops, effects, new backends and performance research.
- Preserve all v0.101 forms, lexical rules, generic internal Core locals, existing
  AST/HIR/Core nodes, compiler/semantic/Application IR identities, frozen 45-source
  compatibility and generic engine/package boundaries.
- Done when parser/typechecker -> typed HIR -> target-neutral Core -> evaluator/
  Core-only Go -> executable Application IR agree. Require all 25 terminal shapes,
  bounded placement/mixed/dependent/unused matrices, independent value and ordered/
  lazy trace oracles, supported type/carrier families, computed operands, determinism,
  malformed and lexical/placement refusals, all 101 earlier source/Core gates, fresh
  discovered compiler proof, affected integration/CLI/vet/editor/docs checks and
  isolated scaling through 256 actual new initializers across scope classes.
- Direct warm compiler ceilings stay 128 MiB / 5 seconds, normal inlining. Distinguish
  independent small-case vectors from shared-input scaling and bounded layout matrices.
- Validation: cached offline Go 1.25.13, retained private caches, canonical temporary
  containment, 30-second units, 25-second children, 700 MiB reclaim, 1 GiB hard,
  800 MiB proactive stop, zero swap, 128 pids, at most two units. Python uses `-B`.
- Execution: `dorkpipe-objective-execution`; automatic in-scope checkpoints and
  user-requested handoff only. Commit, push, publication, cleanup, worktree creation,
  stash mutation, generated-store refresh, installation, credentials and live operations
  remain outside scope. Preserve prior receipts and ignored/generated/cache state.

## Evidence

Task-owned receipts: `/tmp/pipelang-v102-proof`. v0.101 acceptance is admitted without
replaying its historical scripts. The predecessor bytecode deviation remains historical;
preserve current SHA256 `e631831e38539041b7982cb3553e227155f1b607b1970b04f98747534a07f26c`.
Implementation and bounded verification are complete. The completion below owns acceptance.

## Retained preparation and correction evidence

The enlarged Go test-harness build reached the unchanged 30-second deadline while
reclaiming heavily at 700 MiB. Command tracing identified the combined 139-file test
package compile. A process-local `GOMEMLIMIT=600MiB` for harness compilation resolved
that preparation issue within the same unit limits. It is not exported to test
execution or isolated direct compiler measurements; their GC/compiler settings and
128 MiB / 5-second acceptance ceilings remain unchanged.

An initial generated test edit failed to compile and was corrected. A scaling fixture
used `trim(raw)+"B"`, outside the inherited operand surface; it now uses inherited
`trim(raw+"B")`. The Application IR predecessor refusal used the wrong expected
diagnostic text; the assertion now matches the unchanged predecessor diagnostic.

Review identified missing hidden-local refusal in new terminal statement conditions.
The focused pre-fix Core regression proved the admission defect. Both source and Core
placement gates now require local-free statement conditions, with generic internal
Core validity preserved and regressions at all three statement depths. The first
complete-suite attempt was stopped before this correction: 1,372 recorded rows and
1,374 unit reports passed, and every reported unit was removed. Those receipts remain
partial evidence, not terminal acceptance. The corrected source passes the fresh complete run recorded below.


## Completion and reproduction

All 714 freshly discovered compiler tests, fuzz seed functions and examples pass in
1,947 contained units in `terminal2`, with no source drift. Thirteen new tests occupy
223 units, including 200 layout partitions and twelve scaling partitions. The prior
interrupted run remains separate. Twenty inherited fixtures preserve normalized
HIR/Core/semantic artifacts and exact Go; checked propagation and chain inheritance
explicitly extend through v0.102.

The 418 supplied layouts cover all 25 terminal statement shapes, all subsets of
three local slots, used/unused tails, scope-class subsets and five-local mixed
sequences, rotating ordinary, depth-three and flat-selector returns. Each covers a
representative of every reachable statement path, 64 independently supplied vectors
for two initializer selector triples, and eight vectors for an independent return
triple: 1,138,688 evaluator/Go values and generated-Go ordered/lazy traces. Selector
triples alternate across locals and are shared between scopes; mixed inherited
initializers share the supplied input group. These are bounded matrices, not arbitrary
independent assignments at every initializer and tree node. Not every supplied bit
is consumed in every layout. Repeated HIR/Core/semantic/Go artifacts are deterministic.

Dependent bool/string locals add 96 vectors over three strings, controlling descendant
branches and preserving sibling isolation. Computed checked-Result helpers add 48
vectors including overflow and complete failure transport. Twelve type families and
nine carrier/host-value families pass. All 101 earlier source and Core contracts
independently reject the new placement at root, intermediate and leaf scopes.
Malformed types, selectors, arguments, identities, scope, hidden locals, depths and
private forms are refused. The hidden-statement-local regression confirms valid
generic internal Core while independently rejecting public placement at every
statement depth; validation, evaluator and backend refusal agree.

All 216 regression and fresh isolated scaling cases pass. Twenty-four are zero-local
inherited controls; 192 contain 1/8/16/24/32/64/128/256 actual new initializers across
four nonconditional arm families, three scope classes and used/unused final bindings.
The 1,728 result vectors share routing/selector bits across scaled sequences. Peak
isolated compiler RSS is 80.51953125 MiB; maximum elapsed time is
0.16343395301373675 seconds, below unchanged 128 MiB / 5-second warm direct compiler
ceilings with normal inlining and normal GC settings. Controls have closure depth
three. Actual root sequences have depth two or four, intermediate sequences four,
and depth-three leaf sequences six; growth never exceeds each one-local baseline.
All 180 nonmeasurement exports from the focused extreme partitions match the final
fixtures exactly. All 1,296 final fixture files, including measurements, are hashed.

Complete Core/HIR/evaluator/backend, executable Application IR consumer, frozen
45-source compatibility, affected application, complete CLI and compiler/consumer/
application/CLI vet pass on corrected source. The consumer preserves canonical
Application IR across six strings and eight selector vectors. Editor assertions and
syntax, authored JSON/YAML/Python, new documentation links, formatting and planner
partitions pass. Removing only v0.102 additions restores all eight changed production
files exactly to baseline; evaluator and Go backend production code are unchanged.
Generic engine/package boundaries, AST/HIR/Core nodes and public identities remain intact.

All 3,597 recorded temporary cgroups are removed, including failed preparation and
partial-run units. There are no max/OOM or swap-event increases and zero observed
swap. Integration/suite limits stay 700 MiB reclaim, 1 GiB hard, 800 MiB proactive
stop, zero swap, 128 pids, 30-second units and 25-second children, with at most two
units. Isolated measurements use the canonical defaults. The harness-only 600 MiB
Go GC target described above is not applied to tests or resource measurements.

For the historical reproduction command, `GO_TOOLCHAIN` denotes the pinned Go
1.25.13 Linux/amd64 executable. Resolve that toolchain locally before using it.
Reproduce the full suite with `python3 -B tests/containedexec/pipelang_suite.py`,
`--go "$GO_TOOLCHAIN"`,
`--cache /tmp/pipelang-performance-proof/cache`,
`--compiled-cache /tmp/pipelang-execution-performance/complete-artifacts`,
`--shape-batch-size 1 --workers 2 --audit-generated` and a fresh `--output` directory.
For a cold enlarged test-harness build, first use canonical `run.py` with the same
limits and `env GOENV=off GOMEMLIMIT=600MiB <cached-go> test -p 1 -c -o <fresh-binary>
./src/lib/pipelang`; the normal suite reuses the resulting compiler cache. Keep that
environment assignment confined to harness compilation. Run `python3 -B
tests/containedexec/matrix.py` against the fresh suite's `fixtures-v102`, with a fresh
output, the same private cache and the cached toolchain's direct compiler. Preserve
all receipts and caches; do not run a cleanup or generated-store refresh.

Key receipts under `/tmp/pipelang-v102-proof`: `terminal2/summary.json`,
`terminal2/suite.json`, `terminal2/source-hashes.json`, `terminal2/fixtures-v102`,
`isolated/matrix.json`, `scale-fixture-hashes.json`, `production-inheritance-audit.json`,
`integration3.json`, `docs-checks.json`, `planner.json`, `build-environment.json`,
`containment-audit.json`, `final-evidence.json` and `final-source-hashes.json`.
The durable completion record owns acceptance if temporary receipts expire.
Task-owned temporary build files, fixtures and native/compiler cache entries were
created. No repository generated store was refreshed or migrated.

The saved checkout remains `js/pipelang` at
`bce96f06498413700934aa38d0aecc4815af8056`; owned changes are unstaged and uncommitted.
Both protected stashes, the 108,167-path ignored inventory and its digest, and the
current predecessor bytecode hash match. Full repository build/CI/all-library tests,
sustained fuzzing, interactive editor, non-Linux containment and live operations were
not run. No successor is selected. Commit, push, publication and cleanup remain
separate decisions.
