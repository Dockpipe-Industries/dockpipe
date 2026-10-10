# Depth-three straight-line initializers

## Approved objective

- Objective: `TASK-021-depth-three-straight-line-initializers`; state: `completed`.
- Founder selected A and separately said `approved` on 2026-09-06.
- Baseline: clean saved `<checkout>`, `js/pipelang` at
  `25f8af71b0478af6255ab3a9c508cfba6f6c43e4`. Completed v0.95 proof is admitted.
- Scope: v0.96.0 admits complete ternary initializers through depth three in any
  subset of a finite explicitly typed immutable-local sequence in straight-line
  public pure block methods. Either or both arms may nest. Later locals and
  inherited ordinary or depth-three returns may reuse selected values.
- Preserve bool conditions, exact types, complete supported values/carriers,
  eager once-only ordered locals including unused bindings, lazy selected conditions
  and arms, lexical scope, inherited contracts and internal Core capabilities.
  Preserve parser/typechecker -> typed HIR -> target-neutral Core -> evaluator/Core-only
  Go, public compiler/semantic/Application IR identities, frozen 45-source compatibility
  and generic engine/package boundaries.
- Done when all 25 choice shapes and independent condition vectors, mixed/dependent
  local subsets, ordered/unused/lazy traces, types/carriers, independent source/Core
  refusal, inherited artifacts and executable Application IR pass. Verify contained
  scaling through 256 locals and affected compiler/consumer, compatibility,
  application/CLI, vet, editor/docs/format checks. State matrix bounds explicitly.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within scope;
  handoff only on user request. Normal session; commit and push require separate authority.
- Validation: cached Go 1.25.13 offline, private caches, canonical temporary containment
  per `tests/containedexec/README.md`; retain memory/swap/task/deadline controls, normal
  inlining and 128 MiB / 5 second warm compiler ceilings. No uncontained fallback.
- Exclude depth-three initializers before/inside terminal statement trees, depth-four
  choices/statement trees, new condition/argument or matching/propagation placements,
  inference, mutation, loops, effects, new backends, worktree, stash mutation, cleanup,
  commit, push, publication, generated-store refresh, credentials, cloud/live operations,
  persistent settings, service restart and successor selection.

## Completion and reproduction

The approved v0.96.0 scope is complete with no unresolved validation failures.
Baseline admission rejects v0.96 metadata before implementation. Source, typechecker,
HIR dispatch and independent Core admission now accept only the approved placement.
HIR/Core node shapes, evaluator and Go backend production code are unchanged. An audit
removing only the new version extensions, placement guard and diagnostic restores all
six affected source/validation files exactly after formatting.

All 25 choice shapes cross all subsets of three dependent local slots and used/unused
final bindings. Every case exhausts seven independent initializer bits and one independent
return bit; bits are shared across initializers. Even subset masks use ordinary returns,
odd masks use depth-three returns with distinct outcomes. The final shape also covers
five-local mixed-shape masks 21/31. The 404 supplied layouts produce 103,424 independent
expected-value comparisons in both evaluator and pristine generated Go, plus instrumented
selected-condition/arm and eager ordered/unused-local traces. Repeated typed HIR, Core,
semantic projection and exact Go agree. This is a bounded matrix, not every independent
shape/condition assignment across arbitrary-length sequences.

Twelve supported types, nine carrier/host-value families, and computed checked arithmetic
Results cover success/failure and unused eager initializers. Fourteen inherited fixtures
retain normalized HIR/Core/semantic bytes and exact Go. All 95 earlier source versions
reject a representative depth-three initializer. Independent source/Core checks reject
malformed types, depth, placement, scope, hidden locals, unknown identities and downgrades;
Core evaluator/backend refusal is verified. Executable Application IR preserves canonical
projection bytes and independently checks helper results across six strings/eight vectors.

All 623 discovered compiler tests, fuzz seed functions and
examples pass across 733 validation units. All 623 inherited local-sequence
memory cases across eleven versions and the inherited v0.92-v0.95 fixed-helper scale tests
pass. All 64 new direct-initializer scale cases through 256 locals pass in fresh isolated
normal-inlining compiler units. Peak direct compiler RSS is 107.24609375 MiB;
maximum elapsed time is 0.7012646869989112 seconds, below unchanged 128 MiB / 5 second
ceilings. Observed closure depths are [2, 3, 4]; no case exceeds its one-local
baseline. Existing statement emission reduces depth for multiple locals. Four fixed
initializer shapes share three bits and cross used/unused final locals at
1/8/16/24/32/64/128/256 locals; this is direct initializer growth, not a helper-call proxy.

Core/evaluator/backend/HIR, Application IR, frozen 45-source compatibility, affected
application tests, complete CLI, vet, editor assertions/syntax, formatting, authored
JSON/YAML, task routes and local documentation links pass. All 809
recorded temporary validation cgroups are removed; memory max/OOM and swap event counters
are unchanged and observed swap usage is zero. The sandbox cannot access the user manager
and starts no compiler workload; reviewed host runs use the canonical temporary launcher.

Test-only corrections retained in the failed receipts: the initial malformed-Core test
used the wrong reference-kind constant; ordinary-only layouts needed explicit trace-helper
retention; and scaling must prohibit increasing closure depth rather than require equality
when statement emission reduces it. Production behavior and resource ceilings were not
changed to address these harness failures.

Reproduce with cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the final compiler test binary with `run.py`, discover every test/seed/example,
  and apply all documented splits, including
  `TestV960DepthThreeStraightLineInitializersLayouts=25`. Keep the eleven inherited
  memory versions and each v0.92-v0.96 scale test in separate units. No Go source/test
  drift occurred after the terminal build.
- Export `TestV960DepthThreeStraightLineInitializersMemory` with `PIPELANG_MEMORY_FIXTURES`;
  run `matrix.py` for all 64 fresh isolated direct compiler measurements.
- Run Core/evaluator/backend/HIR, Application IR, frozen compatibility; application tests
  selected by `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`; complete CLI;
  compiler/consumer/application/CLI vet; editor, format, authored-data and route checks.

Temporary fixtures, binaries, caches and reports are under `/tmp/pipelang-v096-proof`.
Evidence includes `final-evidence.json`, `final-source-hashes.json`,
`terminal-source-hashes.json`, `terminal-inventory.json`, `terminal-suite.json`,
`terminal-summary.json`, `isolated/matrix.json`, `integration.json`, `docs-checks.json`
and `production-inheritance-audit.json`. Temporary files may expire; this durable record
owns completion.

The saved checkout remains on `js/pipelang` at `25f8af71b0478af6255ab3a9c508cfba6f6c43e4`.
Owned changes are unstaged and uncommitted. Protected stashes
`26ea507907550d2449dc6f9c81b9942bd52d8629` and
`e3afeea1dad94ca0c63dac434f0873548875bfc5` are unchanged. The ignored inventory remains
108,166 paths with SHA-256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`;
this proves inventory, not ignored file contents. Generated repository stores were not
refreshed. Generic engine/package boundaries remain preserved. No full repository
build/CI/all-library suite, sustained fuzzing, interactive editor, non-Linux containment
or live operations were run. Commit, push, publication and successor selection remain separate.
