# Conditional returns in terminal-tree leaves

## Approved objective

- Objective: `TASK-021-terminal-leaf-conditional-returns`; state: `completed`.
- Founder selected A and separately said `approved` in this task.
- Baseline: clean saved checkout `/home/jamie/source/dockpipe`, `js/pipelang` at
  `9db7ed28f8e6de782921bd5d272614fef47cf883`; completed v0.86 proof admitted.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only on user request.
- Scope: v0.87.0 permits complete nonnested ternary returns in any subset of leaves of
  existing terminal if/else trees through depth three. Each scope retains zero or more
  explicitly typed immutable locals, including complete conditional initializers.
  Conditions are bool; return arms exactly match the method return type. Reached initializers
  execute eagerly once in source order, including unused locals; branches and arms are lazy.
  Preserve lexical visibility and complete supported value/carrier transport.
- Preserve: inherited versioned forms, internal Core capabilities, compiler/semantic/Application
  IR identities, frozen 45-source lane, protected checkout state and engine/package boundaries.
- Exclude: deeper statement trees, nested ternaries, new argument/condition placements,
  new matching/propagation combinations, loops, mutation, effects, inference, new backends,
  commit, push, publication, worktree, stash mutation, generated-store refresh, credentials,
  cloud operations, persistent settings, service restart and automatic successor.
- Done when: all 25 tree shapes and leaf subsets, source -> typed HIR -> Core -> evaluator/
  Core-only Go parity, independent refusal, lexical/order/lazy/unused traces, supported
  values/carriers, deterministic artifacts, executable Application IR, inheritance and frozen
  compatibility pass. Scale through 256 locals and run bounded compiler/consumer,
  application/CLI, vet, editor/docs/format checks.
- Validation: cached Go 1.25.13 offline with private caches and verified temporary Linux
  cgroups per `tests/containedexec/README.md`: 1 GiB hard memory, zero swap, 128 tasks,
  800 MiB proactive stop, 30-second generated-child deadline, independent service deadline
  and whole-tree cleanup. Warm direct compiler ceiling stays 128 MiB / 5 seconds.
  Temporary 700 MiB memory.high reclaim is permitted for bootstrap; no uncontained fallback.

## Evidence

Baseline source admission rejected explicit v0.87.0 with PL3011. The sandbox could not
connect to the user manager and started no workload; the reviewed host launcher reproduced
the baseline rejection with the same fixed containment. Source and Core now independently
admit leaf-return choices, falling back to exact v0.86 contracts for inherited forms.
HIR/Core nodes, evaluator and Go backend production code remain unchanged.

Focused types/carriers, malformed source/Core, version boundaries and executable Application
IR checks pass. Initial test-only corrections used the correct Core parameter-reference field
and ensured the consumer fixture replacement actually introduced terminal-leaf choices.
No production behavior change was required for those test mistakes.

All 84 new memory regression cases pass through 256 locals. Fresh isolated direct compiler
measurements with normal inlining also pass: peak 40.516 MiB RSS and 0.0662 seconds, within
the unchanged 128 MiB / 5-second ceiling. These twelve families cross zero/one/two/three/all
conditional initializers, unused final choices and root/branch placement. Each case separately
executes both return outcomes in reached branches. The largest eight-leaf tree passes all
256 leaf subsets and 4,096 evaluator/pristine-Go outcomes plus ordered local-layout traces.

All 25 new shapes pass 1,444 leaf subsets (including ordinary-only inherited trees)
and 23,104 evaluator/pristine-Go outcomes with no local prerequisite. The independent
return condition in the ordered-local matrix covers both arms in every reached branch:
620 methods and 98,560 outcomes/traces. Type and carrier matrices, dependent conditions,
explicit HIR/Core statement-versus-value markers, source/Core malformed placement/type/
scope/depth refusal and normalized inherited artifact identity pass.

One original 25-test batch timed out at 170 seconds after completing the 245-case memory
matrix, while finishing an inherited v0.82 exhaustive shape test. Aggregate memory stayed
below 354 MiB; the unit was removed with unchanged OOM/swap counters. The batch was rerun
as three groups under the same limits: the complete memory test (91.53 s), the v0.82 shape
test (78.94 s), and the remaining 23 tests (12.11 s), all passing. The recovery orchestration
wrapper returned exit status 143 after the first successful report; no later recovery unit
was recorded then. The remaining groups were invoked directly after read-back. Original
and recovery receipts are retained. This is a scheduling/deadline recovery, not a language
repair or a waiver of verification.

Core/evaluator/backend/HIR, Application IR, frozen compatibility, affected application,
CLI, vet and editor checks pass. All remaining inherited compiler batches pass.
Temporary evidence is under
`/tmp/pipelang-v087-proof`; no generated repository store was refreshed.


## Completion and reproduction

The approved v0.87.0 objective is complete on `js/pipelang` at baseline `9db7ed28`.
Changes remain uncommitted for review; completion grants no commit, push, publication,
new feature, live operation or automatic successor authority.

All 524 discovered compiler tests have passing coverage. The original 97 batches had
one deadline failure, whose exact 25 tests passed in three smaller groups: 99 effective
passed groups in total. All 245 old/new memory cases and all 84 fresh isolated v0.87
normal-inlining measurements pass. Maximum passed compiler-batch aggregate memory was
527.156 MiB. Core/evaluator/backend/HIR, Application IR, frozen 45-source compatibility,
application integration, CLI, vet, editor assertions/syntax, Go formatting, authored JSON/YAML,
task routes, relative documentation links and whitespace checks pass.

The evidence audit verifies all 197 recorded temporary cgroups were removed and memory
max/OOM and swap counters stayed unchanged, with zero swap use. Temporary 700 MiB
memory.high reclaim was used for bootstrap/integration/vet without raising hard/proactive
limits. Both protected stash objects remain intact (`26ea507907550d2449dc6f9c81b9942bd52d8629`,
`e3afeea1dad94ca0c63dac434f0873548875bfc5`). The 108,166 ignored paths retain raw NUL-delimited
inventory SHA-256 `ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`;
this is path-inventory evidence, not file-content proof.

Changed areas are version/source/Core placement admission, inherited parser/typechecker/HIR
predicates, compiler/conformance/carrier/resource tests, Application IR consumer tests,
canonical language/editor documentation and TASK-021 records. Generic engine/package boundaries,
internal Core capabilities, public identities and frozen compatibility are preserved. No
HIR/Core nodes, evaluator or Go backend production code changed.

Reproduce with cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the compiler test binary with contained `go test -c`, then use `suite.py` from
  `src/lib/pipelang`. Split `TestV820ConditionalLocalAllShapesScopesAndPaths`,
  `TestV830TwoConditionalLocalsAllShapesScopePairs`,
  `TestV870TerminalLeafConditionalReturnsLayouts` and
  `TestV870TerminalLeafConditionalReturnsSubsets` into their 25 numbered shapes each.
- Run `TestCompilerMemoryLocalSequences/v0.87.0` with `PIPELANG_MEMORY_FIXTURES` pointing
  to a private temporary directory, then use `matrix.py` for fresh direct compiler cases.
- Run contained `go test -p 1` for `./src/lib/pipelang/coreir`, `./src/lib/pipelang/coreeval`,
  `./src/lib/pipelang/gobackend`, `./src/lib/pipelang/hir`, `./src/lib/applicationir` and
  `./tests/pipelangcompat`; run affected application tests selected by
  `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe` and the complete `./src/cmd` package.
- Run contained vet for `./src/lib/pipelang/...`, `./src/lib/applicationir`,
  `./src/lib/application` and `./src/cmd`; verify editor tests/syntax, Go formatting,
  task routes/state, authored JSON/YAML, documentation links and `git diff --check`.

`/tmp/pipelang-v087-proof/final-evidence.json`, `verification-summary.json`, `batches/suite.json`,
`batch2-recovery/suite.json` and `isolated/matrix.json` retain detailed proof. Temporary caches,
binaries and generated modules remain under that directory; they are not release artifacts.
No repository-generated store was refreshed. Verification is bounded, not exhaustive over
arbitrary local lengths or every feature/version combination. Full repository build/CI/all-library
suites, sustained fuzzing, interactive editor use, non-Linux containment and live operations
were not run. No unresolved verification failure remains.
