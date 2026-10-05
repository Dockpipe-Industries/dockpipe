# Nested terminal-leaf returns

## Approved objective

- Objective: `TASK-021-nested-terminal-leaf-returns`; state: `completed`.
- Founder selected A and separately said `approved` in this task.
- Baseline: clean saved checkout `/home/jamie/source/dockpipe`, `js/pipelang` at
  `a8df6cc5b96c455a4ead8cadbee92b035d8512a4`; completed v0.88 proof admitted.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within this scope;
  handoff only on user request.
- Scope: v0.89.0 permits complete depth-two ternary returns in any subset of leaves of
  existing terminal if/else trees through statement depth three. Every lexical scope retains
  finite explicitly typed immutable locals, including complete nonnested conditional initializers.
  Conditions are bool; all return arms exactly match the method return type. Reached locals
  execute eagerly once in order, including unused locals; selected conditions and arms are lazy.
- Preserve: lexical scope, complete supported values/carriers, inherited versioned forms,
  internal Core capabilities, HIR/Core node shapes, compiler/semantic/Application IR identities,
  frozen 45-source lane, protected checkout state and generic engine/package boundaries.
- Exclude: nested initializers, deeper statement/return choices, new condition/argument placements,
  new matching/propagation combinations, loops, mutation, effects, inference, new backends,
  commit, push, publication, worktree, stash mutation, cleanup, generated-store refresh,
  credentials, cloud operations, persistent settings, service restart and automatic successor.
- Done when: all 25 statement shapes, leaf subsets and four return shapes, source -> typed HIR ->
  Core -> evaluator/Core-only Go parity, independent source/Core refusal, lexical/order/lazy/unused
  traces, supported values/carriers, deterministic artifacts, executable Application IR,
  inheritance and frozen compatibility pass. Scale through 256 locals; run affected compiler,
  consumer, application/CLI, vet, editor/docs/format checks.
- Validation: cached Go 1.25.13 offline, private caches and verified temporary Linux cgroups
  per `tests/containedexec/README.md`: 1 GiB hard memory, zero swap, 128 tasks, 800 MiB proactive
  stop, 30-second generated-child deadline, independent service deadline and whole-tree cleanup.
  Warm direct compiler ceiling stays 128 MiB / 5 seconds. Temporary 700 MiB memory.high reclaim
  is permitted for bootstrap; no uncontained fallback or global inlining/GC workaround.

## Evidence

Baseline explicit v0.89 source admission failed as expected before production changes.
The sandbox could not reach the user manager and started no workload; the reviewed host
launcher verified temporary containment. Source/Core placement admission and zero-local
record/Optional/Result signature and HIR routing now support the approved form. HIR/Core
node shapes, evaluator and Go backend production code remain unchanged.

Focused supported-value/carrier, computed checked-Result, downgrade, lexical, malformed
source/Core and representation checks pass. The independent placement audit reproduced a
hidden local inside an ordinary sibling return operand in a newly admitted tree. Both
source and Core walkers now reject that embedding, while internal `ValidateFunction`
continues to accept the valid local expression.

The original combined trace fixture reached the proactive memory stop; its whole cgroup
was removed with zero swap. The same cases now use two-method generated modules. A scale
assertion initially assumed the earlier three-closure bound; branch-local sequences under
one statement wrapper plus two return choices have fixed depth four. The structural bound
now follows that source shape; local count does not deepen it and resource limits and
normal compiler inlining are unchanged. No backend workaround was required.

All 84 new isolated compiler measurements pass: peak 40.64453125 MiB RSS and maximum
0.04877586697693914 seconds, within 128 MiB / 5 seconds. All 84 fixtures exported after
the final placement fix exactly match the measured source, generated Go, generated tests,
module and compiler imports. Application IR, frozen compatibility, affected application,
CLI, vet and editor/docs/format checks pass. Final complete compiler batches pass
from a rebuilt binary with checked-Result inheritance extended through v0.89.

Temporary evidence is under `/tmp/pipelang-v089-proof`; source hashes bind the final
compiler binary. The preliminary scheduler was stopped after the placement reproduction;
its partial results are supplementary. Final verification uses two independent bounded
units at a time through the unchanged repository launcher.


## Completion and reproduction

The approved v0.89.0 objective is complete in the saved `js/pipelang` checkout at
baseline `a8df6cc5`. Changes remain uncommitted for review. No successor is selected;
completion grants no commit, push, publication or live-operation authority.

All **548 discovered compiler tests pass in 267 final batches**. Every discovered test
is assigned once, with the seven exhaustive tests split into their declared inventories.
All 21 changed Go source/test hashes match the final binary snapshot. The full **371-case
old/new memory matrix** passes. All **84 isolated v0.89 compiler measurements** pass with
normal inlining: peak **40.64453125 MiB RSS**, maximum **0.04877586697693914 seconds**.
Generated closure depth remains two for root-local fixtures and four for branch-local
fixtures through 256 locals; it does not grow with local count. Final compiler batches
peak at **533.17578125 MiB aggregate memory**.

The new subset matrix passes all **100 statement-shape/return-shape rotations**, all leaf
subsets, **5,776 methods and 369,664 evaluator/pristine-Go outcomes**. Mixed leaf return
shapes rotate across all 25 statement trees; six independent conditions exercise every
selected path. The separate 25-shape layout matrix passes **620 methods and 394,240
outcomes/traces**, including same-scope and descendant dependencies, independent nested
conditions, eager ordered/unused locals, and unselected branch/condition/arm exclusion.
This does not exhaust every independent assignment of four return shapes to eight leaves;
all shapes appear at every leaf across the rotations, with mixed layouts included.

Twelve supported value types and nine carrier/host-value families pass with and without
locals. Computed checked-Result calls retain success/failure, including overflow in an
unused initializer without propagation. Source diagnostics, independent malformed-Core
and backend/evaluator refusal, statement/value representation markers, all earlier-version
refusal, inherited HIR/Core/semantic/Go artifact identity and executable Application IR
pass. Core/backend/HIR, frozen 45-source compatibility, affected application integration,
complete CLI, vet, editor assertions/syntax, Go formatting, JSON/YAML, task routes,
documentation links and whitespace checks pass. No unresolved verification failure remains.

All **400 recorded validation cgroups** are removed, with unchanged max/OOM and swap
counters and zero swap use. Temporary 700 MiB reclaim was used during bootstrap/integration
without raising the hard/proactive limits. The stopped preliminary scheduler and failed
reproductions remain supplementary evidence; the sandbox preflight started no workload.
HEAD, branch, both protected stashes and the 108,166-path ignored inventory remain unchanged.
The ignored inventory SHA-256 is
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`;
this proves path inventory, not ignored file contents. No repository-generated store was refreshed.

Reproduce using cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build `./src/lib/pipelang` with contained `go test -c`, then run `suite.py` from that
  package directory with `--batch-size 15`. Split v0.82 scopes/paths, v0.83 scope pairs,
  both v0.87 layout/subset tests, and v0.89 layouts into 25 numbered cases each; split
  v0.88 layouts into four and v0.89 subsets into 100. Exact test names are in the README.
  Final verification used the same jobs and unchanged containment launcher with two
  independent units at a time via temporary `suite_parallel.py`; sequential reproduction
  retains the same inventory and per-unit limits.
- Export `TestCompilerMemoryLocalSequences/v0.89.0` with `PIPELANG_MEMORY_FIXTURES` set to
  a private directory; run `matrix.py` for 84 fresh direct compiler measurements. Final
  fixture source/generated-code/test/module/import files match the isolated measured files.
- Run contained tests for `coreir`, `coreeval`, `gobackend`, `hir`, `applicationir` and
  `tests/pipelangcompat`; select application tests with
  `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe` and run the complete CLI suite.
- Run contained vet for `./src/lib/pipelang/...`, `./src/lib/applicationir`,
  `./src/lib/application` and `./src/cmd`; verify editor, formatting, authored data and docs.

Temporary evidence: `/tmp/pipelang-v089-proof/verification-summary.json`,
`final-source-hashes.json`, `terminal-batches/suite.json`, `isolated/matrix.json`,
`fixture-identity.json` and `final-evidence.json`. Temporary files may disappear; this
record owns the durable proof. Version/parser/source/Core admission, zero-local HIR
routing, tests, editor documentation and TASK-021 records changed. HIR/Core node shapes,
evaluator and Go backend production code, public identities and generic engine/package
boundaries remain unchanged.

Verification is bounded: finite matrices and scaling through 256 locals do not exhaust
arbitrary source lengths or every feature/version combination. Full repository build/CI/
all-library suites, sustained fuzzing, interactive editor use, non-Linux containment and
live operations were not run.
