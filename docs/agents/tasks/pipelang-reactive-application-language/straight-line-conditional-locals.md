# Straight-line conditional-local sequences

## Approved objective

- Objective: `TASK-021-straight-line-conditional-locals`; state: `completed`.
- Founder selected A and separately said `approved` in this task.
- Baseline: saved checkout `/home/jamie/source/dockpipe`, clean `js/pipelang` at
  `a69a288bfbc7dd2e09c7c17ad3dbf87787015367`; completed compiler memory repair admitted.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only on user request; warn and continue under context pressure.
- Scope: v0.85.0 additionally admits finite explicitly typed immutable-local sequences with
  complete ternary initializers followed by an ordinary terminal return, without requiring
  a terminal if/else. Later initializers may depend on earlier locals. Conditions are bool;
  arms exactly match the declared local type. Initializers execute eagerly once in order,
  including unused locals; ternary arms are lazy. Complete supported carriers are transported.
- Preserve: existing accepted versioned forms, internal Core capabilities, compiler/semantic/
  Application IR identities, exact frozen 45-source lane, and generic engine/package boundaries.
- Exclude: nested ternaries, new return/condition/argument placements, new matching/propagation
  combinations, deeper terminal trees, fallthrough, loops, mutation, effects, inference, new
  backend, commit, push, publication, worktree, stash mutation, cleanup, generated-store refresh,
  credentials/cloud operations, persistent settings, service restart, and automatic successor.
- Done when: source -> typed HIR -> target-neutral Core -> evaluator/Core-only Go parity,
  independent malformed source/Core refusal, lexical dependencies and unused/order/lazy traces,
  supported types/carriers, deterministic projections, executable Application IR consumption,
  inherited contracts, compatibility, and scaling through 256 locals pass. Focused checks precede
  bounded terminal compiler/consumer/compatibility, application/CLI, vet, editor/docs/format checks.
- Validation: cached Go 1.25.13 offline with private caches and verified temporary Linux cgroups
  per `tests/containedexec/README.md`: 1 GiB hard memory, zero swap, 128 tasks, 800 MiB proactive
  stop, 30-second generated child deadline, independent service deadline, whole-tree cleanup.
  Warm direct compiler ceiling remains 128 MiB / 5 seconds. No uncontained fallback.

## Evidence

Baseline admission rejected v0.85.0 with PL3011 as unsupported. The implemented
source gate and independent Core placement validators admit complete conditional locals
in straight-line sequences, falling back to the existing contracts for inherited forms.
Explicit parser/typechecker/HIR and Core version admission includes v0.85; inherited
arithmetic-Result conditional lowering retains complete-carrier transport. No HIR/Core
node, evaluator or Go backend implementation changed.

Focused source/Core, type/carrier, order/laziness, version and executable Application IR
checks passed (15.332 s including builds). New layout tests exercise one, two, three and
five choices, ordinary locals before/between/after them, dependent values, unused final
choices, every independent choice mask, pristine generated execution and instrumented
ordered traces. Twelve supported type categories and nine carrier/host-value categories
cover exact transport, absent/failed/malformed carriers, UTF-8 and numeric edge cases.
Malformed source and independently altered Core cover placement, missing operands, type,
position, self/forward references, duplicate names and nested/terminal choices. Both Core
program evaluation and backend generation refuse malformed Core. Prior/unknown versions
reject the new form. The accepted internal Core representation remains valid.

The snapshot consumer executes a straight-line five-choice helper; both evaluators and
pristine Go agree on independently asserted helper outcomes and snapshot values. Its
canonical Application IR bytes equal the original-body fixture at v0.85, while v0.84
rejects the new body. Existing all-shape inherited HIR/Core/semantic/Go comparisons,
checked-propagation/chain and numeric-comparison lanes now include v0.85.

All 42 new source-admitted scaling cases pass: six families (zero/one/two/three/all
choices, plus all choices with an unused final local), each at 8/16/24/32/64/128/256
locals. Every case passes evaluator/pristine-Go parity and the unchanged 128 MiB / 5-second
warm compiler ceiling. Fresh isolated normal-inlining compiler measurements peak at
34.801 MiB RSS / 0.059 s. Export names and matrix family keys now include straight-line
and unused-local dimensions so distinct evidence cannot overwrite or merge.

Core/backend, Application IR and frozen compatibility suites pass (8.194 s including
builds). Application integration passes (58.785 s including bootstrap, 10.230 s test time).
Editor assertions/syntax, affected JSON/YAML/routes/document links and whitespace pass.
The complete compiler inventory passes: 503 discovered top-level tests in 46 bounded
batches, including all 25 isolated v0.83 shapes. Maximum batch aggregate memory was
476.36 MiB. The full memory regression includes all 119 old/new scaling cases.
CLI passes (4.935 s including build), and vet passes (4.967 s). Final editor, formatting,
JSON/YAML/routes, affected document links and whitespace checks also pass.

Logs, private caches, test binaries, exported modules and isolated compiler measurements
are retained under `/tmp/pipelang-v085-proof`; they are temporary proof, not release
artifacts. Every completed run removed its temporary cgroup and used zero swap. The
sandbox's initial launch lacked user-manager access and started no workload; reviewed
host invocations use the same verified containment, without an uncontained fallback.


## Completion and reproduction

The approved v0.85.0 objective is complete. Changes are uncommitted for review on
`js/pipelang` at `a69a288b`; no commit, push, publication or successor was performed.
The existing 45-source compatibility fixtures and goldens are unchanged. Both protected
stash objects remain intact; 108,166 ignored paths retain SHA-256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`.
This is path-inventory evidence, not ignored-file content proof.

Changed areas: source/Core version admission and straight-line placement validation;
inherited parser/typechecker/HIR gates; compiler and Application IR regressions;
scaling fixture identity; canonical language/editor documentation and TASK-021 records.
Generic engine/package boundaries and public schema identities are preserved. No evaluator
or Go backend production code changed, and no generated repository store was refreshed.

Use the cached Go 1.25.13 binary and the launcher commands in
`tests/containedexec/README.md`. The completed command lanes were:

- `go test -p 1 ./src/lib/pipelang ./src/lib/applicationir -run '^TestV850' -count=1 -v`.
- `go test -c -o /tmp/pipelang-v085-proof/pipelang.test ./src/lib/pipelang`, then
  `suite.py --split-test TestV830TwoConditionalLocalsAllShapesScopePairs=25` from the compiler
  package directory, using that binary, private cache and fresh units for each batch.
- Export `TestCompilerMemoryLocalSequences/v0.85.0` fixtures using the final binary, then
  `matrix.py` with the cached toolchain's direct compiler for all 42 isolated cases.
- `go test -p 1 ./src/lib/pipelang/coreir ./src/lib/pipelang/coreeval
  ./src/lib/pipelang/gobackend ./src/lib/pipelang/hir ./src/lib/applicationir
  ./tests/pipelangcompat -count=1`.
- `go test -p 1 ./src/lib/application -run 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe' -count=1`;
  `go test -p 1 ./src/cmd -count=1`.
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir ./src/lib/application ./src/cmd`.
- Editor `extension.test.js`, JavaScript syntax, changed Go formatting, task/document/JSON/YAML
  checks and `git diff --check`.

All compiler execution used verified temporary containment; these Go commands are not
instructions to run uncontained. Bootstrap/application/vet lanes used the documented temporary
700 MiB soft reclaim threshold without changing hard/proactive limits. Soft high events are
expected; max/OOM/swap counters stayed unchanged. Every recorded cgroup was removed.
`/tmp/pipelang-v085-proof/final-evidence.json` and `isolated/matrix.json` retain detailed proof.

Proof is bounded, not exhaustive over arbitrary source lengths, nested expressions or every
feature/version combination. Repository-wide build/CI/all-library suites, sustained fuzzing,
interactive editor execution, non-Linux containment and live operations were not run.
The existing Linux-only containment boundary remains unchanged.
