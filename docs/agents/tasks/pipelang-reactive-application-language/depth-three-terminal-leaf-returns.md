# Depth-three terminal-leaf returns

## Approved objective

- Objective: `TASK-021-depth-three-terminal-leaf-returns`; state: `completed`.
- Founder said `approved`, then explicitly clarified option A on 2026-09-06.
- Baseline: clean saved checkout on `js/pipelang` at
  `0a54e858302cd361264ef646b996a6712fa24b24`. Completed v0.93 proof is admitted;
  its historical uncommitted wording is superseded by this observed commit.
- Scope: v0.94.0 admits complete ternary returns through depth three in any subset
  of leaves of existing terminal if/else trees through statement depth three.
  Every lexical scope retains finite explicitly typed immutable locals, including
  depth-two initializers. Arrow bodies retain depth two; straight-line returns retain three.
- Preserve bool conditions, exact types, lexical scope, complete supported values/carriers,
  eager once-only ordered reached locals including unused bindings, lazy selected conditions
  and arms, inherited contracts and internal Core capabilities. Preserve parser/typechecker
  -> typed HIR -> target-neutral Core -> evaluator/Core-only Go, executable Application IR,
  public identities, frozen 45-source compatibility and generic engine/package boundaries.
- Done when all 25 statement/return shapes, leaf placements/subsets, independent reached
  conditions, ordered/unused/lazy traces, supported types/carriers, source/Core refusal,
  inherited artifacts and executable Application IR pass. Prove contained scaling through
  256 caller locals and affected compiler/consumer, compatibility, application/CLI, vet,
  editor/documentation/format checks. Bound supplied matrices explicitly.
- Use `dorkpipe-objective-execution`, automatic checkpoints within scope, user-requested
  handoff only. Validation uses cached Go 1.25.13 offline, private caches and canonical
  `tests/containedexec/README.md` temporary containment with unchanged memory/swap/task/
  deadline controls and normal inlining. Warm compiler ceilings remain 128 MiB / 5 seconds.
- Exclude depth-four choices/statement trees, new argument/condition or matching/propagation
  placements, inference, mutation, loops, effects, backends, worktree, stash mutation,
  cleanup, commit, push, publication, generated-store refresh, credentials/settings,
  service restart, cloud/live operations and automatic successor.

## Verification

Source and Core independently admit the new leaf placement. Version admission, inherited
signature/HIR routing, compiler/consumer tests, editor guidance and canonical documentation
are updated. HIR/Core node shapes and evaluator/backend production code are unchanged.

Focused admission, 12 types, nine carrier/host-value families, computed checked Result success/
failure, malformed source/Core, lexical scope and hidden-local refusal pass. The new root/
intermediate/leaf trace fixture checks eager unused locals and lazy selected conditions/arms.
Twelve inherited source fixtures preserve normalized HIR/Core/semantic bytes and exact Go.
Executable Application IR preserves canonical projection bytes and independently checks helper
results. Core/evaluator/backend/HIR, frozen compatibility, affected application, complete CLI,
vet and editor/docs checks pass.

All 25 layout partitions pass: 150 methods and 76,800 evaluator/pristine-Go outcomes plus
ordered/lazy traces. They pair the 25 statement shapes with 25 return shapes, putting 0/1/3
locals in leaves. Seven return-condition bits are independent; initializer bits are shared,
and the statement tree reuses the outer initializer bit. The separate subset matrix covers
all statement shapes and leaf subsets with four rotated return layouts and six supplied bits;
these are bounded matrices, not all independent choices at every node or arbitrary source sizes.

All 64 fixed-helper caller-scale cases through 256 locals pass. Fresh isolated normal-inlining
measurements peak at 23.30078125 MiB RSS and 0.021928373025730252 seconds. Closure depth is three
for the inherited control and four for the depth-three return layouts, constant across caller
sizes. The initial test copied the prior straight-line closure bound of three; the terminal
wrapper adds one fixed closure. The corrected test checks both the four-level bound and equality
with each zero-local baseline. Memory/time ceilings and containment limits are unchanged.

The first consumer run rejected the prior source contract correctly but expected different
wording; the test expectation was corrected. Its structural assertion was updated to inspect
both terminal branches and their depth-three return trees. These test corrections pass.
The complete terminal compiler inventory passes: 605 discovered tests, fuzz seed functions
and examples across 678 validation units. Unit 73's old scale assertion is superseded by
`scale-fixed.json`, which executes all 64 cases with the corrected constant-depth assertion.
The original failure remains recorded. Only that test file differs from the terminal binary's
source manifest; all other test and production inputs are unchanged. Passed unrelated units
were retained. Final compiler vet passes after the correction.
Temporary generated fixtures, binaries, private caches and reports are under
`/tmp/pipelang-v094-proof`.


## Completion and reproduction

The approved v0.94.0 scope is complete. No unresolved verification failure remains.
All 100 new leaf-subset partitions pass 369,664 evaluator/pristine-Go
outcomes. The 623 inherited memory cases across eleven versions pass. All
756 recorded validation cgroups are removed; max/OOM and swap-event counters are
unchanged and recorded swap usage is zero. Sandbox preflight could not reach the user manager
and started no workload; reviewed host invocations used the canonical temporary launcher.

Reproduce using cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the compiler test binary under `run.py`, discover every top-level test/seed/example,
  and retain fresh numbered units for every documented split inventory. New splits are
  `TestV940DepthThreeTerminalLeafReturnsLayouts=25` and
  `TestV940DepthThreeTerminalLeafReturnsSubsets=100`. Keep each of the eleven inherited
  memory versions and each v0.92/v0.93/v0.94 fixed-helper memory test in its own unit.
- Export `TestV940DepthThreeTerminalLeafReturnsMemory` with `PIPELANG_MEMORY_FIXTURES`,
  then run `matrix.py` for 64 isolated warm compiler measurements with normal inlining.
- Run Core/evaluator/backend/HIR, Application IR, frozen compatibility; affected application
  tests selected by `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`; complete CLI;
  compiler/consumer/application/CLI vet; editor assertions/syntax, formatting, authored
  JSON/YAML, routed paths and local documentation links.

Temporary evidence includes `final-evidence.json`, `final-source-hashes.json`,
`terminal-source-hashes.json`, `terminal-inventory.json`, `terminal-suite.json`,
`terminal-summary.json`, `scale-fixed.json`, `isolated/matrix.json`, `integration.json`,
`consumers.json`, `vet-final.json` and `docs-checks.json` under `/tmp/pipelang-v094-proof`.
Temporary files may expire; this durable record owns completion. The raw terminal summary
retains the superseded unit; the final evidence manifest records the reconciled pass.

The saved checkout remains on `js/pipelang` at `0a54e858302cd361264ef646b996a6712fa24b24`.
Changes are uncommitted and unstaged for review. Both protected stashes and the 108,166-path
ignored inventory remain unchanged (SHA-256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`). This proves inventory,
not ignored file contents. No generated repository store was refreshed.

Full repository build/CI/all-library suites, sustained fuzzing, interactive editor use,
non-Linux containment and live operations were not run. No commit, push, publication or
successor is authorized by completion. Generic engine/package boundaries remain preserved.
