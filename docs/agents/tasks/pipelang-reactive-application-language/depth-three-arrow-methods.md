# Depth-three expression-bodied methods

## Approved objective

- Objective: `TASK-021-depth-three-arrow-methods`; state: `completed`.
- Founder selected A and separately said `approved` on 2026-09-06.
- Baseline: clean saved `<checkout>`, `js/pipelang` at
  `ecb035e6f80eef7d137a1ba9afb78d6c8e26b1e8`. Completed v0.94 proof is admitted.
- Scope: v0.95.0 admits complete ternary arrow bodies through depth three in public
  pure methods, with either or both arms nesting and at most three decisions per path.
  Initializers retain depth two; statement trees and block returns retain depth three.
- Preserve bool conditions, exact types, complete supported values/carriers, lazy selected
  execution, inherited lexical/order semantics and internal Core capabilities. Preserve
  parser/typechecker -> typed HIR -> target-neutral Core -> evaluator/Core-only Go,
  public compiler/semantic/Application IR identities, frozen 45-source compatibility
  and generic engine/package boundaries. Core erases arrow/block spelling.
- Done when all 25 choice shapes, independent condition vectors, arrow/block normalized
  HIR and exact Core/Go equivalence, lazy traces, supported types/carriers, source/Core
  refusal, inherited artifacts and executable Application IR pass. Verify contained
  fixed-helper caller scaling through 256 locals and affected compiler/consumer,
  compatibility, application/CLI, vet, editor/docs/format checks.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within scope;
  handoff only on user request. Normal session; commit and push require separate authority.
- Validation: cached Go 1.25.13 offline, private caches, canonical temporary containment
  per `tests/containedexec/README.md`; retain memory/swap/task/deadline controls, normal
  inlining and 128 MiB / 5 second warm compiler ceilings. No uncontained fallback.
- Exclude depth-four choices/statement trees, deeper initializers, new condition/argument
  or matching/propagation placements, inference, mutation, loops, effects, new backends,
  worktree, stash mutation, cleanup, commit, push, publication, generated-store refresh,
  credentials, cloud/live operations, persistent settings, service restart and successor.

## Verification

Implementation and terminal verification pass. Preserved protected stash hashes
`26ea507907550d2449dc6f9c81b9942bd52d8629` and
`e3afeea1dad94ca0c63dac434f0873548875bfc5`, and the 108,166-path ignored inventory
with SHA-256 `ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`.
Inventory hashing does not prove ignored file contents.


## Completion and reproduction

The approved v0.95.0 scope is complete with no unresolved validation failures.
The baseline rejected v0.95 metadata before implementation. Production changes are exact
version inheritance and source admission plus a version-specific diagnostic. HIR/Core
node shapes, evaluator and Go backend production code are unchanged. A normalization
audit confirms the six affected source/validation files preserve their older-version
code after removing only the v0.95 extensions and new diagnostic.

All 25 arrow shapes exhaust 128 independent condition vectors: 3,200 evaluator/pristine-Go
outcomes plus selected-condition/arm traces. Normalized arrow/block HIR and exact Core/Go
agree. Twelve supported types, nine carrier/host-value families and computed checked
Results cover success/failure and unused eager caller initializers. Thirteen inherited
fixtures retain normalized HIR/Core/semantic bytes and exact Go. All 94 earlier source
versions reject a representative depth-three arrow; v0.93/v0.94 block Core remains valid.
Independent source/Core checks retain depth, placement, type, scope and hidden-local refusal.
Executable Application IR preserves canonical projection bytes and independently checks
helper results across six strings and eight condition vectors.

All 614 discovered compiler tests, fuzz seed functions and examples pass across 706
validation units. The 623 inherited memory cases across eleven versions pass. All 64
new fixed-arrow caller cases through 256 locals pass in fresh normal-inlining compiler
units. Peak direct compiler RSS is 22.3671875 MiB; maximum elapsed time is
0.02210723899770528 seconds, below unchanged 128 MiB / 5 second ceilings. Closure
depths are [2, 3], equal to each shape's zero-local baseline across caller
sizes. These four fixed helper shapes share three condition bits; the proof does not
claim arbitrary trees or arbitrary independent choices at each call.

Core/evaluator/backend/HIR, Application IR, frozen 45-source compatibility, affected
application tests, complete CLI, vet, editor assertions/syntax, formatting, authored
JSON/YAML, task routes and local documentation links pass. All 778
recorded temporary validation cgroups are removed; memory max/OOM and swap event counters
are unchanged and observed swap usage is zero. The sandbox preflight could not access
the user manager and started no compiler workload; reviewed host runs used the canonical
launcher. No persistent machine settings or services were changed.

Reproduce with cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the final compiler test binary using `run.py`; discover every test/seed/example
  and apply all documented splits, including `TestV950DepthThreeArrowMethodsLayouts=25`.
  Keep eleven inherited memory versions and each v0.92-v0.95 fixed-helper scale test in
  separate units. No compiler Go source/test drift occurred after the terminal build.
- Export `TestV950DepthThreeArrowMethodsMemory` with `PIPELANG_MEMORY_FIXTURES`; run
  `matrix.py` for all 64 fresh isolated direct compiler measurements.
- Run Core/evaluator/backend/HIR, Application IR, frozen compatibility; application tests
  selected by `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`; complete CLI;
  compiler/consumer/application/CLI vet; editor, format, authored-data and route checks.

Temporary fixtures, binaries, caches and reports are under `/tmp/pipelang-v095-proof`.
Evidence includes `final-evidence.json`, `final-source-hashes.json`,
`terminal-source-hashes.json`, `terminal-inventory.json`, `terminal-suite.json`,
`terminal-summary.json`, `isolated/matrix.json`, `integration.json` and `docs-checks.json`.
Temporary files may expire; this durable record owns completion.

The saved checkout remains on `js/pipelang` at `ecb035e6f80eef7d137a1ba9afb78d6c8e26b1e8`.
Owned changes are unstaged and uncommitted. Protected stashes and the ignored inventory
match the baseline; generated repository stores were not refreshed. Generic engine/package
boundaries remain preserved. No full repository build/CI/all-library suite, sustained
fuzzing, interactive editor, non-Linux containment or live operations were run.
Commit, push, publication and successor selection remain separate.
