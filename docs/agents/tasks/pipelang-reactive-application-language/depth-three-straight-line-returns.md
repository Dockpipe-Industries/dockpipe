# Depth-three straight-line returns

## Approved objective

- Objective: `TASK-021-depth-three-straight-line-returns`; state: `completed`.
- Founder selected A and separately said `approved` on 2026-09-06.
- Baseline: clean saved `/home/jamie/source/dockpipe`, `js/pipelang` at
  `ab59a59e4199b64c8793cb6342c6625269a7b4ec`; completed v0.92 proof admitted.
  All 31 validated paths, both protected stashes and the ignored inventory match.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within scope;
  handoff only on user request.
- Scope: v0.93.0 admits complete ternary returns through depth three in public pure
  straight-line block methods after zero or more existing explicitly typed immutable locals.
  Either or both arms may nest; at most three decisions per path. Initializers, arrow
  bodies and terminal-tree leaf returns retain their existing depth-two limits.
- Preserve: bool conditions, exact result types, complete supported values/carriers,
  eager once-only ordered locals including unused bindings, lazy selected conditions/arms,
  lexical scope, inherited versions and internal Core capabilities, parser/typechecker ->
  typed HIR -> target-neutral Core -> evaluator/Core-only Go, public compiler/semantic/
  Application IR identities, frozen 45-source compatibility and generic engine boundaries.
- Exclude: depth-four returns, deeper statement trees, new condition/argument or matching/
  propagation placements, inference, mutation, loops, effects, new backends, worktree,
  stash mutation, cleanup, commit, push, publication, generated-store refresh, credentials,
  cloud operations, persistent settings, service restart and automatic successor.
- Done when: all 25 choice shapes and independent conditions, supported values/carriers,
  ordered/unused/lazy traces, independent source/Core refusal, typed HIR/Core representation,
  evaluator/pristine-Go parity, inherited identities and executable Application IR pass.
  Prove contained scaling through 256 caller locals and affected compiler/consumer,
  compatibility, application/CLI, vet, editor/docs/format checks.
- Validation: cached Go 1.25.13 offline, private caches, canonical temporary Linux cgroups
  per `tests/containedexec/README.md`; 1 GiB hard cap, zero swap, 128 tasks, 800 MiB
  proactive stop, child/service deadlines and whole-tree cleanup. Warm compiler ceilings
  remain 128 MiB / 5 seconds; temporary 700 MiB bootstrap reclaim is allowed.

## Evidence

The baseline regression rejected v0.93 metadata before implementation. Exact v0.93
admission now passes. Separate source and Core validators admit the complete depth-three
return while retaining depth-two initializers and terminal leaves. The parser retains
the arrow exclusion because Core erases block/arrow spelling. HIR/Core node shapes,
evaluator and Go backend production code are unchanged.

Focused proof passes all 25 nonempty return shapes, crossed with 0/1/3 preceding locals,
used/unused final bindings, four initializer vectors and all 128 independent return-condition
vectors: 150 methods and 76,800 evaluator/pristine-Go outcomes plus ordered/lazy traces.
The initializer bits are shared across locals; this bounded matrix does not claim every
independent initializer assignment or arbitrary source size. Typed transport covers 12 types
and nine carrier/host-value families with and without locals. Eleven inherited source fixtures
retain normalized HIR/Core/semantic bytes and exact deterministic Go. Source/Core rejection
covers depth, placement, malformed types/bindings, unknown identities and prior versions.
Computed checked Results preserve success/failure, including unused overflowing initializers.

The executable Application IR consumer retains canonical projection bytes and independently
checks helper results across six strings and eight condition vectors. Core/backend, Application
IR, frozen 45-source compatibility, affected application tests, complete CLI and vet pass.
Editor assertions/syntax, authored JSON/YAML, routed paths, local Markdown links and formatting
checks pass. All 593 discovered compiler tests pass across 549 terminal validation units.

All 64 isolated caller-scale cases pass: four fixed helper layouts, used/unused final locals
and 0/1/8/16/32/64/128/256 caller locals. One layout retains depth two as an inherited control;
the other three reach depth three. All eight supplied condition vectors agree between evaluator
and pristine Go. Peak compiler RSS is 22.34765625 MiB; maximum direct compile time is
0.02207197801908478 seconds; closure depth is at most three and independent of caller size.
These are bounded caller-scaling measurements with normal inlining, not proof of every source
size or independent conditions at every call. Generated fixtures, binaries, private caches and
reports remain under `/tmp/pipelang-v093-proof`.

The first focused run exposed a missing zero-local record/Optional/Result signature admission
path. The corrected source predicate passes all corresponding type/carrier cases. The v0.92
unknown-version fixture now uses v0.94, preserving its refusal purpose. Failed baseline and
pre-correction receipts are supplementary evidence; the final terminal run passes. Sandbox preflight could not reach the user manager and started no workload;
reviewed host invocations use the canonical temporary containment launcher.


The first terminal scheduler exited with signal 143 after recording 209 rows.
Read-back found 211 successful unit receipts with matching commands and removed cgroups.
The unchanged final test binary resumes only unfinished units in segments of at most 80;
passed tests are retained. This scheduler interruption is not a compiler-test failure.


## Completion and reproduction

The approved v0.93.0 scope is complete. No unresolved verification failure remains.
The final acceptance inventory proves that every discovered compiler test, fuzz seed
function and example ran in its assigned unit(s). All 623 inherited memory cases pass
across eleven language-version units. All 624 recorded validation cgroups are removed;
max/OOM and swap-event counters remain unchanged, and recorded swap usage is zero.
Bootstrap/integration reclaim retained the original hard and proactive memory limits.
No production or test source changed during terminal verification.

Reproduce with cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the compiler test binary inside `run.py`. Discover every test, seed function and
  example, and use fresh units with the documented numbered split inventories. Split
  `TestV930DepthThreeReturnsLayouts=25`; keep the v0.92 and v0.93 memory tests separate.
  Run all eleven `TestCompilerMemoryLocalSequences` versions in separate units.
  This run used batches of at most 15 ordinary tests and two concurrent contained units.
- Export `TestV930DepthThreeReturnsMemory` with `PIPELANG_MEMORY_FIXTURES`, then use
  `matrix.py` for 64 fresh direct compiler measurements with normal inlining.
- Run Core/evaluator/backend/HIR, Application IR and frozen compatibility; affected
  application tests selected by `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`;
  complete CLI; vet for compiler, Application IR, application and CLI. Verify editor
  assertions/syntax, Go formatting, authored JSON/YAML, task routes, local links and whitespace.

Temporary evidence under `/tmp/pipelang-v093-proof` includes `final-evidence.json`,
`final-source-hashes.json`, `terminal-source-hashes.json`, `terminal-inventory.json`,
`terminal-suite.json`, `terminal-summary.json`, `scheduler-interruption.json`,
`isolated/matrix.json` and `integration.json`. Temporary files may expire; this durable
record owns completion. The scheduler interruption was reconciled without replaying
passed units or changing the test binary, source inputs or containment limits.

The saved checkout remains on `js/pipelang` at
`ab59a59e4199b64c8793cb6342c6625269a7b4ec`. Changes are uncommitted for review; nothing
is staged. Both protected stashes and the 108,166-path ignored inventory remain unchanged
(SHA-256 `ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`).
This verifies path inventory, not ignored contents. No generated repository store was refreshed.

Source/Core version and topology admission, inherited signature/HIR routing, compiler/consumer
regressions, editor guidance and documentation changed. HIR/Core node shapes, evaluator/backend
production code, public identities and generic engine/package boundaries remain preserved.
Full repository build/CI/all-library suites, sustained fuzzing, interactive editor use,
non-Linux containment and live operations were not run. Completion does not authorize
commit, push, publication or a successor.
