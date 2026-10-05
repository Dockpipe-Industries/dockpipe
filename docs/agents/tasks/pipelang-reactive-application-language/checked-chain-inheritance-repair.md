# Multi-stage checked-chain inheritance repair

## Approved objective

- Objective: `TASK-021-next-compiler-slice-after-checked-propagation-inheritance-repair`;
  state: `completed`.
- Authority: founder selected A and separately said `approved` in this task.
- Baseline: saved checkout `/home/jamie/source/dockpipe`, branch `js/pipelang`, clean at
  `46ce299a385f58b41b6619454168c56429349995`. That commit contains the completed prior
  checked-propagation inheritance repair and supersedes its historical uncommitted wording.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only when requested; context pressure means warn and continue.
- Scope: restore inheritance of existing v0.57 two-stage and v0.58 generalized checked chains
  through v0.82. Integer stages independently retain `+`, `-`, `*`; float stages retain `/`.
  Preserve complete-carrier validation, exact parameter/local order, and failure short-circuiting.
- Preserve: source analysis -> typed HIR -> target-neutral Core -> evaluator/Core-only Go;
  accepted versioned forms and combinations, internal Core capabilities, all three public
  compiler/semantic/Application IR identities, and the exact frozen 45-source compatibility lane.
- Exclude: new syntax, language version, chain shape, branching/propagation combination,
  unrelated repair, commit, push, publication, worktree, cleanup, stash inspection/mutation,
  generated-store refresh, credentials, Docker/cloud/live operations, and automatic handoff.
- Done when: baseline regressions fail and the repaired inheritance matrix passes; mixed integer
  stages, longer chains, incoming/intermediate/terminal failures, numeric boundaries, malformed
  carriers, source/Core exclusions and backend refusal are covered; normalized HIR/Core/semantic/
  Go artifacts retain identity; evaluator/pristine-Go agree; the Application IR snapshot consumer
  executes the repaired chains while retaining canonical output; focused then terminal compiler,
  consumer, compatibility, affected application/CLI, vet, formatting, editor/document checks pass.

## Evidence

The baseline admission regression passed at each form's original version through v0.73 and
failed for all 40 tested chains at v0.74-v0.82 (360 `PL3009` failures). The separate raw-version
check in `inferCheckedArithmeticPropagationBlock` retained Result carriers in the environment
used to infer intermediate arithmetic locals. That selected the incompatible Result transport
rule. Inference now uses the existing `inheritedLanguageContract` mapping for this check and
arithmetic-Result recognition. Production edits are confined to `src/lib/pipelang/typecheck.go`;
parser, HIR/Core lowering, Core validation, evaluator, backend, version constants, editor, public
schemas, and tracked goldens are unchanged.

Focused verification passed:

- `checked_chain_inheritance_test.go` covers all nine two-stage and 27 three-stage integer
  operator combinations, corresponding float chains, and integer/float eight-stage chains
  under every inheriting version through v0.82 (1,010 compiler pipelines). HIR/Core/semantic/Go
  artifacts must equal their original-version baseline after normalizing only language metadata.
- An independent arbitrary-precision integer/native IEEE division oracle checks success,
  incoming/intermediate/terminal failures, integer extrema, both zero signs, infinities, NaNs,
  subnormals, and underflow. Both evaluator entrypoints and each distinct pristine generated Go
  artifact agree: 19,439 outcomes through each evaluator entrypoint and 40 distinct generated Go
  artifacts. Malformed carriers and input-carrier preservation are checked.
- Source exclusions and independent program admission/backend refusal retain parameter, operand,
  stage, binding, propagation, and type restrictions, plus downgrade/unknown-version rejection.
  Standalone Core retains its broader internal capabilities; public version gates belong to
  program admission.
- `applicationir/checked_chain_inheritance_test.go` makes the snapshot entrypoint execute either
  the two-stage or three-stage chain. Success, incoming failure, and failure at each arithmetic
  stage retain snapshot values; helper outcomes are independently asserted in evaluation and
  pristine Go. Repeated semantic/Core/Go outputs are deterministic and canonical Application IR
  bytes equal the unchanged-body fixture at v0.82. Both consumer fixtures fail under a temporary
  baseline typechecker overlay, so the repaired dependency is exercised.

Focused command (passed):
`go test ./src/lib/pipelang ./src/lib/applicationir -run '^Test(CheckedChainInheritance|CheckedPropagationInheritance|V570|V580)' -count=1`.
This includes the prior one-stage inheritance repair and original v0.57/v0.58 regressions.

## Completion — 2026-09-04

Focused and terminal verification passed using cached Go 1.25.13 with `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, `GOCACHE=/tmp/dockpipe-go-cache-pipelang`, and `GOTMPDIR=/tmp`:

- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`
  (PipeLang 167.045 s; all packages passed, including the exact frozen 45-source lane).
- `go test ./src/lib/application -run 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe' -count=1`.
- `go test ./src/cmd -count=1`.
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir ./src/lib/application ./src/cmd`.
- Changed Go files are gofmt-clean; editor extension JavaScript syntax, task YAML,
  objective/approval/version agreement, record state, all task routes/document paths, affected
  document links, and `git diff --check` passed.

Changed areas: inherited-contract inference for intermediate checked arithmetic locals, compiler
and executable Application IR regressions, canonical language documentation, and TASK-021
scope/completion records. Language metadata remains v0.82.0; source/Core restrictions, internal
Core capabilities, all three public identities, and generic engine/package boundaries are preserved.
No tracked source/HIR/Core/semantic/Go/Application IR golden or generated store was refreshed.

Logs, the baseline typechecker copy/overlay, and a document checker are supplementary artifacts
under `/tmp/pipelang-checked-chain-proof`; caches are temporary. Pristine generated Go modules
were created in test temporary directories and removed by their helpers. Repository-wide
builds/suites, interactive editor execution, sustained fuzzing, stack-exhaustion checks, and
exhaustive stage counts/feature-version combinations beyond the bounded matrix were not run.

The approved objective is complete. All changes remain uncommitted for review on `js/pipelang`
at baseline HEAD `46ce299a`. No commit, push, publication, worktree, cleanup, stash inspection or
mutation, credential/live action, successor selection, or automatic handoff was performed.
