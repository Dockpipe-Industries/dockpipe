# Checked-propagation inheritance repair

## Approved objective

- Objective: `TASK-021-next-compiler-slice-after-numeric-comparison-repair`; state: `completed`.
- Authority: founder selected A and separately said `approved` on 2026-09-04.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only when requested; context pressure means warn and continue.
- Baseline: saved checkout `<checkout>`, branch `js/pipelang`, clean at
  `fb2f480af497ec34d134105eba0eadcdb0ab9a74` (numeric-comparison repair committed).
- Scope: reproduce and repair inheritance of the exact v0.56 two-parameter direct checked
  propagation form through v0.82, for integer addition/subtraction/multiplication and float
  division. No new source syntax, language version, propagation placement, signature, chain,
  or branching combination.
- Preserve: source analysis -> typed HIR -> target-neutral Core -> evaluator/Core-only Go;
  accepted versioned forms and combinations, internal Core capabilities, all three public
  compiler/semantic/Application IR identities, and the exact frozen 45-source compatibility lane.
- Exclude: unrelated repair, commit, push, publication, worktree, destructive cleanup, stash
  inspection/mutation, generated-store refresh, credentials, Docker/cloud/live operations,
  and automatic successor selection or handoff.
- Done when: the baseline regression fails and the repaired integer/float inheritance matrix
  passes; success, incoming failure, overflow/division failure, source/Core rejection boundaries,
  evaluator/generated-Go agreement, and semantic/Application IR stability are proven; focused
  then terminal compiler/consumer/compatibility, application/CLI, vet, formatting, and applicable
  editor/document checks pass.

## Evidence

The baseline admission regression passed at v0.56-v0.73 and failed with `PL3028` for all four
operations at v0.74-v0.82 (36 failures). `inferMethodBodyType` bypassed checked-propagation
inference by comparing raw versions; its continuation operator check also used raw versions.
Both now use the existing `inheritedLanguageContract` mapping. Production edits are confined to
`src/lib/pipelang/typecheck.go`; parser, lowering, Core validation, evaluator, backend, version
constants, public schemas, and editor are unchanged.

Focused verification passes:

- `checked_propagation_inheritance_test.go` covers all 27 versions from v0.56 through v0.82
  and all four operations (108 compiler pipelines). HIR, Core, semantic, and generated Go bytes
  agree with v0.56 after normalizing only language metadata; repeated semantic and Go outputs
  are deterministic. Pristine generated Go executes each distinct operation artifact.
- The matrix checks 729 success/failure outcomes through both standalone and program evaluation,
  including integer extrema, overflow, incoming failure precedence, division by both zero signs,
  negative zero, infinity, NaN, and subnormal transport. Malformed carriers fail in both evaluator
  entrypoints and pristine Go. Source restrictions, all four Core operand-placement restrictions,
  backend refusal, v0.55 downgrade rejection, and unknown-version rejection remain enforced.
- Existing v0.54 helper and v0.55 sole-carrier forms retain identical Go under v0.82.
- `applicationir/checked_propagation_inheritance_test.go` makes the existing snapshot entrypoint
  depend on the repaired two-parameter helper. Success and incoming-overflow paths preserve
  snapshot values in evaluation and pristine Go; repeated semantic/Core/Go outputs are stable,
  and canonical Application IR bytes equal the unchanged-body fixture at v0.82. All three
  public identities remain unchanged. Existing composite-signature and operand restrictions
  remain enforced; the fixture uses admitted helpers and explicit primitive locals.

Focused command:
`go test ./src/lib/pipelang ./src/lib/applicationir -run '^Test(CheckedPropagationInheritance|V560)' -count=1`.

## Completion — 2026-09-04

Focused and terminal verification passed using cached Go 1.25.13 with `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, `GOCACHE=/tmp/dockpipe-go-cache-pipelang`, and `GOTMPDIR=/tmp`:

- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`
  (PipeLang 207.767 s; all packages passed, including the exact frozen 45-source lane).
- `go test ./src/lib/application -run 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe' -count=1`.
- `go test ./src/cmd -count=1`.
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir ./src/lib/application ./src/cmd`.
- Changed Go files are gofmt-clean; editor extension JavaScript syntax checking, task YAML,
  state/version agreement, document routes/links, and `git diff --check` passed.

Changed areas: two inherited-contract checks in body-type inference, compiler and Application IR
regressions, canonical language documentation, and TASK-021 scope/completion records. The compiler
pipeline, generic engine/package boundaries, public identities, language version, and tracked
source/HIR/Core/semantic/Go/Application IR goldens remain unchanged.

Logs, temporary overlays, and their baseline source copies remain under
`/tmp/pipelang-checked-inheritance-proof`; caches and generated Go modules are temporary. Generated
modules are removed by test helpers. No generated store was refreshed or tracked golden updated.
Repository-wide builds/suites, interactive editor execution, sustained fuzzing, stack-exhaustion
checks, and exhaustive combinations beyond this bounded matrix were not run.

The objective is complete. Changes remain uncommitted for review on `js/pipelang` at baseline
HEAD `fb2f480a`. No push, publication, worktree, stash operation, credential or live action,
successor selection, or automatic handoff was performed.

## Deferred findings

An additional related-form probe finds the existing v0.57/v0.58 multi-stage checked fixtures
rejected at v0.82. Their intermediate arithmetic carrier initializer follows a separate raw-version
check in `inferCheckedArithmeticPropagationBlock`. This approved two-parameter repair does not
change that check or admit new chains. Temporary Go overlays reproduce `PL3009` at the intermediate arithmetic initializer with both
the committed baseline and repaired source (`deferred-baseline.log` and `deferred-current.log`).
The repaired diagnostic uses the inherited v0.73 label; the original uses v0.82. No tracked test
locks in that defect, no production chain repair was made, and no successor is selected.
