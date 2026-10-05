# Conditional boolean selector returns in terminal leaves

## Approved objective

- Objective: `TASK-021-terminal-leaf-boolean-selectors`; state: `completed`.
- Founder selected A and separately said exact `approved` on 2026-09-08.
- Baseline: clean saved checkout `js/pipelang` at `6ce476f6f43df045b1109ad7cbe362185b03bd59`.
  All 28 v0.98 hashes, protected stashes and ignored inventory matched; completed proof is admitted.
- Scope: v0.99.0 admits `return (a ? b : c) ? x : y;` in any subset of leaves of
  inherited terminal if/else trees through statement depth three. The selector is
  exactly one flat boolean ternary; all five operands use inherited nonconditional
  expressions. Selector operands are bool and value arms have exactly matching types.
- Preserve finite explicitly typed immutable locals at root/intermediate/leaf scopes,
  including inherited depth-three initializers; eager once-only source order including
  unused reached locals; lazy branches, then a once, selected b/c, selected x/y;
  lexical scope and complete supported values/carriers without implicit propagation.
- Preserve all v0.98 forms, parser/typechecker -> typed HIR -> target-neutral Core ->
  evaluator/Core-only Go -> executable Application IR, public identities, frozen
  45-source compatibility, generic internal Core locals and engine/package boundaries.
- Exclude new arrow/initializer/statement-condition/argument/matching/propagation
  placements, further nesting, increased depth limits, inference, mutation, loops,
  effects, backends and performance research.
- Done when all 25 statement shapes and every leaf subset pass an explicitly bounded
  routing/selector matrix with independent expected values and ordered/lazy traces;
  mixed inherited returns, used/unused scoped locals, types/carriers, computed operands,
  lexical/malformed refusals and all 98 earlier source/Core contracts pass; deterministic
  artifacts, consumer, compatibility, fresh discovered compiler suite, affected CLI/vet/
  editor/docs checks and isolated scaling through 256 actual locals at root/intermediate/
  leaf scopes pass unchanged 128 MiB / 5-second direct warm compiler ceilings.
- Validation: cached offline Go 1.25.13, existing private caches, canonical contained
  launcher, 30-second units, 25-second test children, 700 MiB temporary reclaim, 1 GiB
  hard, 800 MiB proactive stop, zero swap, 128 pids, at most two concurrent units.
- Execution: `dorkpipe-objective-execution`, automatic in-scope checkpoints; user-requested
  handoff only. Commit, push, publication, cleanup, worktree, stash mutation, generated-store
  refresh, installation, credentials and external operations remain outside scope.

## Completion and reproduction

The approved v0.99.0 implementation and verification are complete. All 676 freshly
discovered compiler tests, fuzz seed functions and examples pass in 1,670 contained
units, including 149 units for fifteen new tests. Source bytes remained unchanged
throughout the final run. Receipts are retained under `/tmp/pipelang-v099-proof`;
this durable record owns acceptance if temporary files expire.

All 25 inherited statement shapes and every leaf subset pass: 1,444 subsets exhaust
92,416 independent routing/selector vectors. Outside each selected subset, ordinary
and inherited depth-three returns remain mixed. Four disjoint subset partitions per
statement shape preserve complete coverage under the unchanged unit deadline.
One hundred scope layouts cross one/three locals with used/unused final bindings at
every reached scope and exhaust 51,200 routing/selector/initializer vectors. Six
additional leaf-local layouts supply 384 vectors with zero/one/four preceding locals.
Evaluator and pristine generated Go values match independent expectations; instrumented
Go preserves ordered reached locals, selected statement branches and lazy selector arms.
Routing bits are shared by depth; initializer bits are shared across local sequences.
These are bounded supplied matrices, not arbitrary independent assignments at every node.

Twenty-four computed-condition vectors and 48 computed checked-Result vectors pass.
Twelve type families and nine carrier/host-value families pass with and without locals.
All 98 earlier source and Core contracts independently reject the new leaf placement.
Source refusals include lexical leaks, shadowing, duplicates, self/forward references,
forbidden placements and excessive depth. Malformed Core refuses invalid selectors,
types, locals, statement conditions and depth. Five operand-position probes preserve
valid generic internal Core locals while refusing their public placement.
Seventeen inherited fixtures preserve normalized HIR/Core/semantic and exact Go bytes;
repeated new leaf-local artifacts are deterministic. Public `pipelang.compiler.v1`,
`pipelang.semantic.v1` and `dockpipe.application.v1` identities and node shapes are unchanged.

Source admission and independent Core validation use existing conditional/local nodes.
Removing only v0.99 admission and the new bounded selector gates restores all eight
changed production files exactly to the committed baseline. Evaluator and Go backend
production code are unchanged; generic engine/package boundaries are preserved.

All 192 regression and fresh isolated scaling cases pass. They cross root/intermediate/
leaf placement, ordinary/depth-one/depth-two/depth-three initializer families, used/unused
tails and 0/1/8/16/32/64/128/256 actual locals. The selector return directly follows the
sequence; helper calls do not hide compiler growth. Fresh isolated maxima are
78.6875 MiB compiler RSS and 0.24943253298988566 seconds. Final-suite regression maxima
are 78.65234375 MiB and 0.224362073 seconds; the earlier focused regression peaked at
79.16796875 MiB and 0.235432974 seconds. All remain below unchanged 128 MiB / 5-second
normal-inlining compiler ceilings. Fixed closure depths range from two through seven;
no tested sequence grows beyond its one-local baseline. All 1,152 earlier exported
fixture hashes remain intact; all 960 nonmeasurement files regenerated by the final
suite match those measured in isolation.

Complete Application IR consumer, Core/evaluator/backend/HIR, frozen 45-source
compatibility, affected application, complete CLI and compiler/consumer/application/CLI
vet pass. Executable consumer results agree across six strings and eight boolean vectors,
with unchanged canonical Application IR projection. Editor assertions/syntax, authored
JSON/YAML, local documentation links, formatting and diff checks pass. Planner checks
prove disjoint complete partitioning at widths one, four and twenty-five.

All 1,908 recorded temporary cgroups are removed. No max/OOM or swap-event increases
occurred; observed swap is zero. Maximum recorded aggregate peak is 701.66015625 MiB.
Tests used cached offline Go 1.25.13, existing private caches and canonical containment:
30-second units, 25-second test children, 700 MiB temporary reclaim, 1 GiB hard memory,
800 MiB proactive stop, zero swap, 128 pids and at most two concurrent units. Fresh
isolated compiler measurements used the canonical default controls.

Reproduce the final suite with `tests/containedexec/pipelang_suite.py`, cached Go 1.25.13,
`--cache /tmp/pipelang-performance-proof/cache`,
`--compiled-cache /tmp/pipelang-execution-performance/complete-artifacts`,
`--shape-batch-size 1 --workers 2 --audit-generated` and a fresh output directory.
Then pass its `fixtures-v099` directory to `tests/containedexec/matrix.py` with the
cached toolchain compiler and same private cache. Use `python3 -B` for Python checks.
No resource-limit changes, custom compiler/GC flags, downloads or performance research
are involved. The sandbox user-manager preflight started no workload; reviewed host
invocations used the canonical temporary cgroup launcher.

Key receipts: `terminal/summary.json`, `terminal/suite.json`, `terminal/source-hashes.json`,
`terminal/fixtures-v099`, `isolated/matrix.json`, `scale-fixture-hashes.json`,
`production-inheritance-audit.json`, `integration.json`, `docs-checks.json`,
`final-evidence.json` and `final-source-hashes.json`.
Failed receipts are retained: the initial focused invocation omitted the retained-cache
requirement GOENV=off; the first scaling assertion undercounted fixed terminal wrappers;
the largest unsplit subset exceeded its 25-second deadline; and the consumer assertion
expected different predecessor rejection wording. Corrections preserve all cases and
resource limits. These failed attempts are not terminal acceptance.

The saved checkout remains `js/pipelang` at `6ce476f6f43df045b1109ad7cbe362185b03bd59`.
Owned changes are unstaged and uncommitted. Both protected stashes remain intact. Ignored
inventory remains 108,167 paths with SHA-256
`b39314777ac271ecd1ea58975c53c69983912f932dd95ead0d909b4f98ca2bc2`.
Native caches gained validated entries and task-owned temporary evidence/build/fixture
artifacts were created. Existing cache entries and receipts were not removed; no generated
repository store was refreshed.

**Preservation deviation:** a planner import refreshed the pre-existing ignored file
`tests/containedexec/__pycache__/pipelang_suite.cpython-310.pyc`. Its original bytes were
not saved and exact preservation cannot be claimed. The file remains present. Subsequent
Python checks use `-B` to prevent further bytecode writes. Inventory equality is not
proof that this ignored file's content was preserved.

Full repository build/CI/all-library tests, sustained fuzzing, interactive editor,
non-Linux containment and live operations were not run. No successor is selected or
approved. Commit, push, publication and any successor remain separate decisions.
