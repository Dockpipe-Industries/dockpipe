# Nested straight-line returns

## Approved objective

- Objective: `TASK-021-nested-straight-line-returns`; state: `completed`.
- Founder selected A and separately said `approved` in this task.
- Baseline: clean saved checkout `/home/jamie/source/dockpipe`, `js/pipelang` at
  `ff9fa2961b0fd30b91524f755e7a11cc650efda4`; completed v0.87 proof admitted.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only on user request.
- Scope: v0.88.0 adds complete ternary returns in straight-line block-bodied methods with
  zero or more existing explicitly typed immutable locals. Either or both return arms may
  contain another ternary, with at most two ternary decisions per path. Conditions are bool;
  all result arms exactly match the declared return type. Reached conditions and arms are lazy;
  locals execute eagerly once in order, including unused locals. Preserve lexical scope and
  complete supported value/carrier transport.
- Preserve: inherited versioned forms, internal Core capabilities, public compiler/semantic/
  Application IR identities, frozen 45-source lane, protected checkout state and engine/package
  boundaries.
- Exclude: nested initializers, nested choices inside statement trees, nested choices in
  expression-bodied methods, new condition/argument placements, deeper statement trees,
  new matching/propagation combinations, loops, mutation, effects, inference, new backends,
  commit, push, publication, worktree, stash mutation, cleanup, generated-store refresh,
  credentials, cloud operations, persistent settings, service restart and automatic successor.
- Done when: all four bounded ternary shapes and paths, source -> typed HIR -> Core ->
  evaluator/Core-only Go parity, independent source/Core refusal, lexical/order/lazy/unused
  traces, supported values/carriers, deterministic artifacts, executable Application IR,
  inheritance and frozen compatibility pass. Scale through 256 locals; run affected compiler,
  consumer, application/CLI, vet, editor/docs/format checks.
- Validation: cached Go 1.25.13 offline, private caches and verified temporary Linux cgroups
  per `tests/containedexec/README.md`: 1 GiB hard memory, zero swap, 128 tasks, 800 MiB proactive
  stop, 30-second generated-child deadline, independent service deadline and whole-tree cleanup.
  Warm direct compiler ceiling stays 128 MiB / 5 seconds. Temporary 700 MiB memory.high reclaim
  is permitted for bootstrap; no uncontained fallback or global inlining/GC workaround.

## Evidence

Baseline admission rejected explicit v0.88.0 with PL3011 before production changes. The
sandbox could not reach the user manager and started no workload; the reviewed host launcher
verified containment and reproduced that rejection. Source/Core admission and zero-local parser
support now pass. Focused matrices exposed zero-local record/Optional/Result signature gates and
HIR body routing that still assumed a direct transport form; the v0.88 placement checks now
route those bodies through ordinary expression typing/lowering. Earlier contracts retain their
existing dispatch.

All 144 layout methods pass 11,584 evaluator/pristine-Go outcomes and ordered/lazy traces.
Twelve supported types and nine carrier/host-value families pass with and without locals,
along with dependent local types, malformed source/Core, all earlier-version refusal, inherited
HIR/Core/semantic/Go identity and executable Application IR. All 42 new memory cases through
256 locals pass under unchanged limits. Final terminal verification is recorded below.

The independent Core placement audit reproduced acceptance of hidden immutable-local expressions
inside a new return arm, outer/inner condition or initializer arm. The v0.88 source/Core
validators now exclude embedded locals while preserving the leading straight-line sequence.
Four malformed-public-Core cases pass refusal; the same expressions remain valid internal Core
under `ValidateFunction`. The first probe had a test-only reference-kind typo, corrected before
the reproduction. Computed checked-Result return calls pass, including overflow in an unused
local without accidental propagation. No evaluator or Go backend production change was needed.

The full preliminary compiler binary was already running when this last placement check was
added. Its receipts remain supplementary; final verification uses a rebuilt binary.

Final Core/evaluator/backend/HIR, Application IR and frozen compatibility suites pass. Affected
application integration, the complete CLI suite, vet and editor assertions/syntax checks pass.
Temporary kernel reclaim at 700 MiB was used for file-heavy integration/bootstrap without raising
hard/proactive limits. Final full-compiler batches and isolated direct compiler measurements pass; completed receipts
remain in `/tmp/pipelang-v088-proof`.

## Completion and reproduction

The approved v0.88.0 objective is complete on saved `js/pipelang` at baseline `ff9fa296`.
Changes remain uncommitted for review. Completion grants no commit, push, publication,
new feature, live operation or automatic successor authority.

All **535 discovered compiler tests pass in 141 final batches**, with the five exhaustive
shape tests split into their declared inventories. The preliminary binary's 534 tests also
passed in 140 batches; the final binary adds the embedded-local refusal regression and includes
the final source/Core placement fix. All 21 changed Go source/test hashes match the final build
snapshot. The initial source-admission and embedded-local reproductions failed as expected before
the corresponding fixes. No unresolved verification failure remains.

The full **287-case old/new memory matrix** passes. All **42 fresh isolated v0.88 compiler
measurements** pass with normal inlining: peak **34.953125 MiB RSS**, maximum **0.065432351 seconds**,
within the unchanged 128 MiB / 5-second ceiling. Closure depth stays at two through 256 locals.
Maximum aggregate memory across final compiler batches was **535.25 MiB**.

The new layout matrix covers **144 methods and 11,584 outcomes/traces**, all four return shapes,
zero/one/two/three/five local layouts, conditional-local subsets, dependent and unused locals,
and independent outer/inner conditions. Twelve supported value types and nine carrier/host-value
families pass with and without locals. Computed checked-Result calls retain success/failure,
including an unused failed Result initializer without accidental propagation. Source diagnostics,
independent malformed-Core/evaluator/backend refusal, HIR/Core value markers, all earlier-version
refusal, normalized inherited artifact identity and executable Application IR pass.

Core/evaluator/backend/HIR, Application IR, frozen 45-source compatibility, affected application
integration, complete CLI, vet, editor assertions/syntax, Go formatting, authored JSON/YAML,
task routing, relative documentation links and whitespace checks pass. All **339 recorded
validation cgroups** were removed, with unchanged memory max/OOM and swap counters and zero
swap use. Temporary 700 MiB memory.high reclaim was used for file-heavy bootstrap/integration;
hard and proactive limits remained unchanged. The failed sandbox preflight started no cgroup.

Both protected stash objects remain intact (`26ea507907550d2449dc6f9c81b9942bd52d8629` and
`e3afeea1dad94ca0c63dac434f0873548875bfc5`). The 108,166 ignored paths retain raw NUL-delimited
inventory SHA-256 `ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`;
this proves path inventory, not file contents. HEAD and branch are unchanged, with no staged files.

Reproduce with cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build `./src/lib/pipelang` with contained `go test -c`, then run `suite.py` from that package
  with `--batch-size 15`. Split `TestV820ConditionalLocalAllShapesScopesAndPaths`,
  `TestV830TwoConditionalLocalsAllShapesScopePairs`, `TestV870TerminalLeafConditionalReturnsSubsets`
  and `TestV870TerminalLeafConditionalReturnsLayouts` into 25 numbered shapes each; split
  `TestV880NestedStraightLineReturnsLayouts` into four. The test enumerates that inventory.
- Run `TestCompilerMemoryLocalSequences/v0.88.0` with `PIPELANG_MEMORY_FIXTURES` set to a private
  directory; use `matrix.py` for fresh direct compiler measurements of all 42 exported fixtures.
- Run contained `go test -p 1` for `./src/lib/pipelang/coreir`, `coreeval`, `gobackend`, `hir`,
  `./src/lib/applicationir` and `./tests/pipelangcompat`; select affected application tests with
  `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe` and run the complete `./src/cmd` suite.
- Run contained vet for `./src/lib/pipelang/...`, `./src/lib/applicationir`, `./src/lib/application`
  and `./src/cmd`, then check editor tests/syntax, Go formatting, task state/routes, authored
  JSON/YAML, documentation links and whitespace.

Detailed temporary evidence: `/tmp/pipelang-v088-proof/verification-summary.json`,
`final-evidence.json`, `final-source-hashes.json`, `final-batches/suite.json` and
`isolated/matrix.json`. Temporary caches, binaries and generated modules stay under that directory;
no repository-generated store was refreshed. Version/parser/source/Core admission, zero-local
HIR routing, tests, editor documentation and TASK-021 records changed. HIR/Core node shapes,
evaluator and Go backend production code, public identities and generic engine/package boundaries
remain unchanged.

Verification is bounded: finite matrices and scaling through 256 locals do not exhaust arbitrary
source lengths or every feature/version combination. Full repository build/CI/all-library suites,
sustained fuzzing, interactive editor use, non-Linux containment and live operations were not run.
