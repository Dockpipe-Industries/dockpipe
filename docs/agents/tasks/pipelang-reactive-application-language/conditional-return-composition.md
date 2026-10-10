# Conditional return composition

## Approved objective

- Objective: `TASK-021-conditional-return-composition`; state: `completed`.
- Founder selected A and separately said `approved` in this task.
- Baseline: clean saved checkout `<checkout>`, `js/pipelang` at
  `f94fdafa31bdde269425ebf0d1558412af6aa6f8`; completed v0.85 proof admitted.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only on user request.
- Scope: v0.86.0 additionally permits finite explicitly typed immutable-local sequences,
  including complete conditional initializers, followed by a complete ternary return in
  straight-line methods. Conditions are bool; arms exactly match the declared return type.
  Locals execute eagerly once in source order, including unused bindings. Return arms are lazy.
  All existing supported values and carriers retain complete-value transport.
- Preserve: inherited versioned forms, internal Core capabilities, compiler/semantic/Application
  IR identities, frozen 45-source lane, protected checkout state, generic engine/package boundaries.
- Exclude: nested ternaries, new argument/condition placements, terminal-tree expansion, new
  matching/propagation combinations, loops, mutation, effects, inference, new backend, commit,
  push, publication, worktree, stash mutation, cleanup, generated-store refresh, credentials,
  cloud operations, persistent settings, service restart, and automatic successor.
- Done when: source -> typed HIR -> Core -> evaluator/Core-only Go parity; independent source/Core
  refusal, lexical/order/lazy/unused traces, supported values/carriers, deterministic artifacts,
  executable Application IR proof, inheritance and frozen compatibility pass. Scale through 256
  locals; then bounded compiler/consumer, application/CLI, vet, editor/docs/format checks.
- Validation: cached Go 1.25.13 offline with private caches and temporary verified Linux cgroups
  per `tests/containedexec/README.md`: 1 GiB hard memory, zero swap, 128 tasks, 800 MiB proactive
  stop, 30-second generated-child deadline, independent service deadline, whole-tree cleanup.
  Warm direct compiler ceiling remains 128 MiB / 5 seconds; no uncontained fallback.

## Evidence

Baseline source admission rejected v0.86.0 with PL3011 as unsupported. Source and Core now
independently admit the new complete return placement, with fallback to exact older contracts.
Explicit parser/typechecker/HIR/Core inheritance includes v0.86. Existing HIR/Core nodes,
evaluator and Go backend production code are unchanged.

Focused final checks pass: 10 layout methods, 752 evaluator/pristine-Go outcomes and ordered
traces; twelve supported types and nine carrier/host-value families; dependent return conditions;
independent malformed source/Core refusal with source spans; version rejection; inherited
HIR/Core/semantic/Go identity; explicit HIR/Core placement; and executable Application IR
consumption retaining canonical output. Unselected return arms remain lazy while all locals,
including unused final choices, execute eagerly once in source order. A Core terminal-statement
flag alone produces an inherited valid tree, so that representation is preserved rather than
misclassified as malformed.

Initial test-only corrections fixed a Core reference-field name, avoided double-counting the
return conditional, and retained already-admitted third locals in terminal-tree rejection
helpers. These failures required no production behavior change.

Terminal verification and scaling passed. Temporary proof is under
`/tmp/pipelang-v086-proof`; no generated repository store has been refreshed.


## Completion and reproduction

The approved v0.86.0 objective is complete in the saved checkout on `js/pipelang`, with changes
uncommitted for review at baseline `f94fdafa31bdde269425ebf0d1558412af6aa6f8`. No commit,
push, publication, worktree, stash mutation, cleanup, generated-store refresh or successor occurred.

All 513 discovered compiler tests pass in 46 bounded batches, including the 25 isolated v0.83
compatibility shapes. Maximum batch aggregate memory was 463.875 MiB. The complete memory
regression passes all 161 old/new cases. All 42 v0.86 cases (zero/one/two/three/all conditional
initializers and unused final choices at 8/16/24/32/64/128/256 locals) additionally pass fresh
isolated normal-inlining direct compiler measurements: peak 35.520 MiB RSS / 0.0592 seconds,
within the unchanged 128 MiB / 5-second ceiling. Both return outcomes are exercised separately in every scale case.

Core/backend, Application IR and frozen compatibility suites pass (5.523 s including builds).
Affected application integration passes (65.877 s including bootstrap); CLI passes (5.225 s),
and vet passes (4.589 s). Editor assertions/syntax, Go formatting, authored JSON/YAML, task
routes/state, relative documentation links and whitespace checks pass.

The full evidence audit verifies all 100 recorded temporary cgroups were removed, memory
max/OOM and swap event counters stayed unchanged, and swap use remained zero. Temporary
700 MiB `memory.high` reclaim was used for bootstrap/integration/vet without raising hard or
proactive limits. The initial sandbox launch failed to connect to the user manager and started
no workload; reviewed host launches used the same verified containment. Both protected stashes
remain intact. The 108,166 ignored paths retain SHA-256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`;
this is path-inventory evidence, not ignored-file content proof.

Changed areas: source/Core version admission and conditional-return placement; inherited
parser/typechecker/HIR predicates; compiler/type/carrier/order/inheritance/resource tests;
Application IR consumer tests; canonical language/editor documentation and TASK-021 records.
Generic engine/package boundaries, internal Core capabilities, public identities and the frozen
45-source lane are preserved. No evaluator or Go backend production code or HIR/Core node changed.

Reproduce with the cached Go 1.25.13 toolchain and `tests/containedexec/README.md` launchers,
using private caches and the fixed containment limits:

- Focused `go test -p 1 ./src/lib/pipelang ./src/lib/applicationir -run '^TestV860' -count=1 -v`.
- Build the compiler test binary with contained `go test -c`; run `suite.py` from
  `src/lib/pipelang` with `--split-test TestV830TwoConditionalLocalsAllShapesScopePairs=25`.
- Export `TestCompilerMemoryLocalSequences/v0.86.0` fixtures with `PIPELANG_MEMORY_FIXTURES`,
  then run `matrix.py` with the cached toolchain's direct compiler for fresh isolated cases.
- Contained `go test -p 1 ./src/lib/pipelang/coreir ./src/lib/pipelang/coreeval
  ./src/lib/pipelang/gobackend ./src/lib/pipelang/hir ./src/lib/applicationir
  ./tests/pipelangcompat -count=1`.
- Contained application tests selected by `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`,
  the full `./src/cmd` test package, and vet for `./src/lib/pipelang/...`,
  `./src/lib/applicationir`, `./src/lib/application`, and `./src/cmd`.
- Editor `extension.test.js`, JavaScript syntax, affected Go formatting, task route/state,
  JSON/YAML, documentation-link and `git diff --check` validation.

Temporary logs, caches, binaries and exported generated modules are under
`/tmp/pipelang-v086-proof`; `final-evidence.json`, `batches/suite.json`, and
`isolated/matrix.json` retain detailed proof. They are not release artifacts. Proof is bounded,
not exhaustive over arbitrary lengths or every feature/version combination. Repository-wide
build/CI/all-library suites, sustained fuzzing, interactive editor use, non-Linux containment,
and live operations were not run.
