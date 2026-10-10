# Nested straight-line initializers

## Approved objective

- Objective: `TASK-021-nested-straight-line-initializers`; state: `completed`.
- Founder selected A and separately said `approved` in this task on 2026-09-06.
- Baseline: clean saved `<checkout>`, `js/pipelang` at
  `2560ee1b4b1045efdc8045ed358ab872a7c6b207`; completed v0.89 proof admitted.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within scope;
  handoff only on user request.
- Scope: v0.90.0 allows any subset of a finite sequence of explicitly typed immutable
  locals in a straight-line block method to use complete ternary initializers through
  depth two. Either or both arms may contain one further ternary. Later locals and
  existing ordinary or depth-two returns may use earlier selected values.
- Preserve: bool conditions, exact types, lexical scope, eager once-only source order
  including unused locals, lazy selected conditions/arms, complete values/carriers,
  inherited contracts, internal Core capabilities, HIR/Core shapes, public compiler/
  semantic/Application IR identities, frozen 45-source lane and engine/package boundaries.
- Exclude: nested initializers in statement trees, nested arrow methods, depth-three
  choices, new condition/argument and matching/propagation placements, mutation, inference,
  loops, effects, new backends, worktree, stash mutation, cleanup, commit, push, publication,
  generated-store refresh, credentials, cloud operations, persistent settings, service
  restart and automatic successor.
- Done when: all initializer shapes, finite subsets and mixed sequences, ordinary and
  nested returns, ordered/unused/lazy traces, supported values/carriers, independent
  source/Core rejection, typed HIR/Core structure, evaluator/Core-only Go parity,
  inherited artifact identity and executable Application IR pass. Prove contained scaling
  through 256 locals and affected compiler/consumer/compatibility, application/CLI, vet,
  editor/docs/format checks.
- Validation: cached Go 1.25.13 offline with private caches and verified temporary Linux
  cgroups per `tests/containedexec/README.md`: 1 GiB hard cap, zero swap, 128 tasks,
  800 MiB proactive stop, 30-second generated-child and independent service deadlines,
  whole-tree cleanup. Warm direct compiler ceiling 128 MiB / 5 seconds. Temporary 700 MiB
  reclaim permitted for bootstrap; no uncontained fallback or global compiler/GC workaround.

## Evidence

Source and Core independently admit the bounded straight-line initializer form. Existing
versioned placement checks, signature validation and HIR routing include v0.90.0. HIR/Core
node shapes, evaluator and Go backend production code are unchanged.

Focused source/Core rejection, initializer and return representation markers, hidden-local
refusal, exact types/carriers, computed checked Results (including unused failed carriers),
all earlier-version refusal and inherited artifact identity pass. Generic internal Core local
expressions remain valid. Application IR, Core/evaluator/backend/HIR, frozen compatibility,
affected application, complete CLI, vet, editor syntax/assertions, formatting, authored data,
task routes and documentation links pass.

All 25 initializer/return layout cases pass: 850 methods and 72,000 evaluator/pristine-Go
outcomes and ordered/lazy traces. The matrix crosses four uniform initializer shapes and a
mixed rotation with ordinary returns and all four return shapes; it enumerates every local
subset through three locals and selected five-local layouts, with used/unused final locals.
Each supplied condition vector is exhausted; inner initializer and return conditions share
some test parameters. This does not claim every independent condition assignment for arbitrary
source lengths or every independent assignment of shapes to every local.

The complete 455-case old/new compiler-memory matrix passes in ten fresh version units.
All 84 isolated v0.90 compiler cases pass with normal inlining: peak 70.953125 MiB RSS,
maximum 0.20955820102244616 seconds, within 128 MiB / 5 seconds. Generated closure depth is
zero or two, independent of local count through 256. All 84 final exported fixture sources,
generated code, tests, module files and imports exactly match the measured fixtures.

Two validation-harness issues were resolved without changing production code. An ordinary
return with no choices did not reference the Check helper; the layout fixture now explicitly
includes both trace helpers in its requested method graph. The expanded memory matrix exceeded
a 170-second batch deadline; all ten versions now pass separately at unchanged limits, and
the other tests from the interrupted batch pass separately. Failed receipts are supplementary.
The initial sandbox preflight started no workload; reviewed host invocations used the canonical
containment launcher. The initial v0.90 source-admission failure was expected before implementation.

## Completion and reproduction

All 560 discovered compiler tests have passing evidence in 303 accepted validation units.
The original 293-batch run had six failed batches: one timeout and five layout-fixture
failures. The final manifest replaces that interrupted batch with ten version runs and its
remaining tests, and replaces all 25 new layouts with the repaired fixture runs. All other
compiler tests retain their unchanged passing evidence. No unresolved verification failure
remains. Final source/test hashes bind the binaries and the sole later test-only repair.

All 428 recorded validation cgroups are removed. Every recorded unit used zero swap and
retained unchanged max/OOM and swap-event counters. Temporary 700 MiB reclaim was used
for bootstrap/integration without raising the hard or proactive limits. HEAD remains
`2560ee1b4b1045efdc8045ed358ab872a7c6b207` on `js/pipelang`; the work is uncommitted for review.
Both protected stashes remain unchanged. The 108,166-path ignored inventory retains SHA-256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`;
this proves path inventory, not ignored file contents. No repository-generated store was refreshed.

Reproduce with cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the compiler test binary inside `run.py`; run `suite.py` from `src/lib/pipelang`
  with batch size 15 and all eight declared numbered split inventories. This run used a
  temporary copy with two independent units at a time. If the combined memory matrix
  reaches its deadline, retain the receipt and run all ten declared version subtests and
  the interrupted batch's other tests in fresh units; do not change the limits.
- Export `TestCompilerMemoryLocalSequences/v0.90.0` and run `matrix.py` for 84 fresh
  isolated compiler measurements. Compare source, generated code, tests, module files and
  import configuration with the final exported fixtures.
- Run Core/evaluator/backend/HIR, Application IR and `tests/pipelangcompat`; select application
  tests with `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe` and run the complete CLI suite.
- Run vet for `./src/lib/pipelang/...`, `./src/lib/applicationir`, `./src/lib/application`
  and `./src/cmd`, then editor assertions/syntax, Go formatting, YAML/JSON, task routes,
  documentation links and whitespace checks.

Temporary evidence: `/tmp/pipelang-v090-proof/verification-summary.json`,
`accepted-compiler-manifest.json`, `final-source-hashes.json`, `initial-terminal-source-hashes.json`,
`recheck/summary.json`, `isolated/matrix.json` and `fixture-identity.json`. Temporary files may
expire; this record owns durable proof. Source/Core admission, version/signature/HIR routing,
tests, editor guidance and task documentation changed. HIR/Core shapes, evaluator and Go
backend production code, public identities and generic engine/package boundaries are preserved.

Full repository build/CI/all-library suites, sustained fuzzing, interactive editor use,
non-Linux containment and live operations were not run. No commit, push, publication or
successor is selected or authorized by this completion.
