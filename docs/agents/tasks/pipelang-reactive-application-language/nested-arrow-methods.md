# Nested expression-bodied methods

## Approved objective

- Objective: `TASK-021-nested-arrow-methods`; state: `completed`.
- Founder selected A and separately said `approved` in this task on 2026-09-06.
- Baseline: clean saved `<checkout>`, `js/pipelang` at
  `b1fbc31b36cad52add2efc87d5f479810326602b`; completed v0.91 proof admitted.
  All 38 validated paths, both protected stashes and the ignored inventory match.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within scope;
  handoff only on user request.
- Scope: v0.92.0 admits complete depth-two ternary bodies in public pure expression-bodied
  methods. Either or both arms may contain another ternary, with at most two decisions per
  path. Existing conditions and operand forms retain bool conditions and exact result types.
- Preserve: lazy selected conditions/arms, complete supported values/carriers, inherited
  contracts, internal Core capabilities, parser/typechecker -> typed HIR -> target-neutral
  Core -> evaluator/Core-only Go, public compiler/semantic/Application IR identities,
  frozen 45-source compatibility and generic engine/package boundaries.
- Exclude: deeper choices or statement trees, new condition/argument or matching/propagation
  placements, inference, mutation, loops, effects, new backends, worktree, stash mutation,
  cleanup, commit, push, publication, generated-store refresh, credentials, cloud operations,
  persistent settings, service restart and automatic successor.
- Done when: all four depth-two choice shapes and independent condition combinations,
  supported values/carriers, arrow/block representation and behavior equivalence, lazy traces,
  source/Core refusal, inherited identities and executable Application IR pass. Verify contained
  scaling and affected compiler/consumer, compatibility, application/CLI, vet, editor/docs/format.
- Validation: cached Go 1.25.13 offline, private caches, canonical temporary Linux cgroups
  per `tests/containedexec/README.md`; retain 1 GiB hard cap, zero swap, 128 tasks, 800 MiB
  proactive stop, child/service deadlines and whole-tree cleanup. Warm compiler ceilings stay
  128 MiB / 5 seconds; temporary 700 MiB bootstrap reclaim is allowed. No uncontained fallback.

## Evidence

The baseline regression failed on unsupported v0.92 metadata before implementation.
Production changes add exact version admission and inherit existing source/Core validators,
signature checks and HIR routing. Only v0.92 lifts the parser's nested-arrow restriction.
HIR/Core node shapes, evaluator and Go backend production code are unchanged.

Focused verification passes all four choice shapes and eight independent condition vectors,
12 supported types, nine carrier/host-value families, normalized arrow/block HIR and exact Core/Go
equivalence, selected-condition/arm Go traces, malformed source/Core refusal, prior source-version
rejection and ten inherited artifact fixtures. Core retains v0.88-v0.91 depth-two block expressions:
arrow spelling is erased and must not retroactively narrow those valid artifacts.
Computed checked Results retain success/failure, including overflow in an unused initializer.
The executable Application IR consumer preserves canonical projection bytes and separately checks
its arrow helper's results across six input strings and eight condition vectors.

All 64 scale fixtures pass, crossing four helper shapes, used/unused final caller locals and
0/1/8/16/32/64/128/256 locals. All eight condition vectors agree between evaluator and pristine
Go. Fresh isolated normal-inlining measurements pass with peak 22.01953125 MiB compiler RSS and
maximum 0.02327150700148195 seconds, within the 128 MiB / 5 second warm ceilings. Closure depth
is at most two and does not grow with local count. These are bounded caller-scaling vectors,
not proof of every finite source size or arbitrary independent conditions at each call.

Core/backend, Application IR and frozen 45-source compatibility checks pass. All 584 discovered compiler tests pass across 521 terminal validation units.
All 623 inherited memory cases pass across eleven separate version units. Affected
application tests, the complete CLI suite and vet pass. Editor assertions/syntax and authored
JSON/YAML, routed paths, local documentation links and whitespace checks pass.

Two test-fixture issues were corrected without changing compiler behavior: a Go import name
collided with the parser token type; zero-local ordinary block returns were excluded by the
inherited grammar, so those callers use the existing arrow-call spelling. Failed receipts are
supplementary evidence. Sandbox preflight could not reach the user manager and started no
workload; reviewed host invocations use the canonical temporary containment launcher.
Temporary evidence is owned by `/tmp/pipelang-v092-proof`; this document will own final proof.


## Completion and reproduction

The approved v0.92.0 scope is complete. No unresolved verification failure remains.
All terminal compiler inputs match their recorded hashes; no production or test code
changed during the terminal run. The acceptance inventory binds every discovered compiler
test, fuzz seed function and example to its completed unit(s), and confirms that every
requested test actually ran. The four new shape/type and shape/carrier partitions and
four arrow layout partitions have complete coverage of their supplied vectors.

All 597 recorded validation cgroups are removed. Every recorded unit retained unchanged
max/OOM and swap-event counters and used zero swap. Bootstrap/integration used temporary
700 MiB reclaim without raising hard/proactive limits. All 64 isolated measurements used
normal compiler inlining. Baseline and corrected test-fixture failures remain supplementary;
all required final verification passes.

Reproduce with cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the compiler test binary inside `run.py`. Run from `src/lib/pipelang`, using fresh
  units, batch size 15 and every declared numbered split inventory. Split the eleven
  `TestCompilerMemoryLocalSequences` version subtests into fresh units from the start.
  This run used a temporary `terminal.py` scheduler with two independent units at a time;
  `terminal-inventory.json` owns its exact 584-test / 521-unit assignment.
- Export `TestV920NestedArrowMethodsMemory` with `PIPELANG_MEMORY_FIXTURES` and use `matrix.py`
  for 64 fresh isolated measurements. The unchanged fixture-builder/compiler inputs bind
  those exports to the terminal binary; closure depth remains at most two through 256 locals.
- Run Core/evaluator/backend/HIR, Application IR and frozen compatibility; affected application
  tests selected by `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`; complete CLI; vet
  for compiler, Application IR, application and CLI. Run editor assertions/syntax, Go
  formatting, authored JSON/YAML, task routes, local documentation links and whitespace checks.

Temporary evidence under `/tmp/pipelang-v092-proof` includes `final-evidence.json`,
`terminal-source-hashes.json`, `final-source-hashes.json`, `terminal-inventory.json`,
`terminal-suite.json`, `terminal-summary.json`, `isolated/matrix.json` and `integration.json`.
Temporary files may expire; this record owns durable completion proof. Generated fixtures,
Go binaries, private caches and validation reports live only under `/tmp`.

The saved checkout remains on `js/pipelang` at `b1fbc31b36cad52add2efc87d5f479810326602b`.
Changes are uncommitted for review; nothing is staged. Both protected stashes and the
108,166-path ignored inventory remain unchanged (SHA-256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`). This proves path inventory,
not ignored contents. No repository-generated store was refreshed.

Source/Core version admission, inherited signature/HIR routing, compiler/consumer tests,
editor guidance and task documentation changed. HIR/Core shapes, evaluator/backend production
code, public identities and generic engine/package boundaries remain preserved. Full repository
build/CI/all-library suites, sustained fuzzing, interactive editor use, non-Linux containment
and live operations were not run. No commit, push, publication or successor is authorized
by completion.
