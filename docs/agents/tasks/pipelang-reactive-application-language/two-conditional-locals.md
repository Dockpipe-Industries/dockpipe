# Two conditional locals in terminal trees

## Approved objective

- Objective: `TASK-021-next-compiler-slice-after-checked-chain-inheritance-repair`.
- State: `completed`; founder selected A and separately said `approved` in this task.
- Baseline: saved checkout `/home/jamie/source/dockpipe`, branch `js/pipelang`, clean at
  `776bfa9e5c2e3a08bda362d489d30d2f69379bae`. The checked-chain repair is committed here;
  its historical uncommitted wording is superseded by live read-back.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only when requested; context pressure means warn and continue.
- Scope: v0.83.0 permits at most two ternaries per method, each a complete typed immutable-local
  initializer within terminal trees through depth three. Count occurrences across the entire
  method, including exclusive branches. The second may depend on the first in lexical scope.
  Preserve exact types, source order including unused locals, and selected-arm execution.
- Preserve: source analysis -> typed HIR -> target-neutral Core -> evaluator/Core-only Go ->
  semantic/Application IR consumers; all accepted versioned forms and combinations; internal
  Core capabilities; public compiler/semantic/Application IR identities; frozen 45-source lane.
- Exclude: third/nested ternaries, new placements, deeper trees, new matching/propagation
  combinations, fallthrough, mutation, loops, effects, unrelated repairs, commit, push,
  publication, worktree, stash inspection/mutation, cleanup, generated-store refresh,
  credentials/live actions, and automatic successor or handoff.
- Done when: all 25 terminal-tree shapes and same/ancestor/sibling scope pairs, dependent
  choices, lazy arms, ordered/unused locals, exact supported types, malformed source/Core,
  downgrade/unknown-version refusal, deterministic evaluator/pristine-Go parity, inherited
  forms and executable Application IR consumption pass; focused then terminal compiler,
  consumer, compatibility, affected application/CLI, vet, formatting, editor and docs checks pass.

## Evidence

The baseline admission regression rejected v0.83.0 as unsupported before implementation.
The new version is admitted explicitly; source and Core separately count conditional locals,
while the v0.82 one-occurrence bound and depth-three limit remain unchanged. Parser/HIR
inheriting-version checks, supported identities, editor snippet/docs, and version diagnostics
are synchronized. No new HIR/Core node, public schema, evaluator, or backend behavior was added.

The typed arithmetic-Result matrix exposed the legacy arithmetic-body recognizer being invoked
for a conditional initializer with direct-reference arms. v0.83 local lowering now uses the
ordinary conditional path for those initializers, transporting the complete Result. Earlier
version dispatch remains unchanged; tests cover both evaluators and pristine generated Go.

Focused checks passed for source diagnostics and independent malformed-Core/backend/evaluator
refusal, different local types, all twelve supported value categories, absent/failed/malformed
carriers and host values, dependent conditions, and inherited artifact equality. Both consumer
fixtures exercise two choices, either in one scope or ancestor/descendant scopes; independent
helper outcomes and snapshot values agree in evaluation/pristine Go, canonical Application IR
bytes equal the original-body fixture at v0.83, and v0.82 rejects each at the one-choice bound.

The extended one-stage/multi-stage checked propagation and numeric-comparison matrices, Core
identity/feature admission, v0.82 regressions, and new non-shape tests passed in the focused run
(compiler 43.631 s, Application IR 2.654 s). The full placement matrix passed: all 25 shapes, 2,652 methods, 84,864 evaluator/pristine-Go
cases, and matching ordered traces, including unused second locals and both arms at every
scope pair (540.643 s). The generated test harness was then partitioned into per-method table
checks to reduce Go compile cost without changing scenarios; the largest shape passed in
29.679 s. Terminal verification uses that final harness.

Application integration (10.395 s), CLI (0.009 s), vet, editor JavaScript syntax and snippet
checks, Go formatting, task YAML/state/version/approval/routes, affected document links, JSON,
and diff whitespace checks pass. The terminal compiler/consumer/frozen-compatibility run passed (main compiler 552.792s). No successor is authorized by completion.


## Completion

The approved v0.83.0 objective is complete. Focused and terminal checks passed using cached
Go 1.25.13, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, writable temporary caches,
and `GOTMPDIR=/tmp`:

- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1 -timeout=20m`.
- `go test ./src/lib/application -run 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe' -count=1`.
- `go test ./src/cmd -count=1`.
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir ./src/lib/application ./src/cmd`.
- Editor snippet/README assertions and JavaScript syntax; changed Go formatting; task
  YAML/state/version/approval/routes and affected document links/JSON; `git diff --check`.

Changed areas: source/Core version admission and conditional-local bounds; typed arithmetic-Result
conditional-initializer lowering; compiler/inheritance/numeric regressions; executable Application IR
consumer proof; canonical language/editor documentation and snippet; TASK-021 scope/completion records.
The evaluator, Go backend, public identities/schemas, internal Core capabilities, and generic
engine/package boundaries are preserved. Existing goldens and the exact frozen 45-source lane are
unchanged. No generated store was refreshed.

Supplementary logs/checkers are under `/tmp/pipelang-v083-proof`; caches and pristine generated Go
modules are temporary, with generated modules removed by their test helpers. Repository-wide builds
and suites, interactive editor execution, sustained fuzzing, stack-exhaustion checks, and exhaustive
feature/version cross-products beyond the bounded matrices were not run.

Changes remain uncommitted for review on `js/pipelang` at baseline `776bfa9e`. No push, publication,
worktree, stash operation, cleanup, credentials/live action, successor selection, or automatic
handoff was performed or authorized.
