# Nested initializers throughout terminal trees

## Approved objective

- Objective: `TASK-021-nested-terminal-initializers`; state: `completed`.
- Founder selected A and separately said `approved` on 2026-09-06.
- Baseline: clean saved `/home/jamie/source/dockpipe`, `js/pipelang` at
  `83cd8adc986b8abd5c70a67b1c701684c7c88539`; completed v0.90 proof admitted.
  This live commit supersedes historical uncommitted wording in the v0.90 record.
- Execution skill: `dorkpipe-objective-execution`; automatic checkpoints within scope;
  handoff only on user request.
- Scope: v0.91.0 allows complete depth-two ternary initializers in any subset of a finite
  explicitly typed immutable-local sequence at root, intermediate and leaf scopes of existing
  terminal if/else trees through statement depth three. Either or both initializer arms may
  contain another ternary. Existing ordinary or depth-two returns and later locals/descendant
  conditions may reuse earlier bindings within lexical scope.
- Preserve: bool conditions, exact types, complete supported values/carriers, eager once-only
  ordered execution including unused locals, lazy selected conditions/arms, lexical scope,
  inherited forms, internal Core capabilities, HIR/Core shapes, public compiler/semantic/
  Application IR identities, frozen 45-source lane and engine/package boundaries.
- Exclude: nested arrow methods, deeper statement/ternary choices, new condition/argument or
  matching/propagation placements, mutation, inference, loops, effects, new backends, worktree,
  stash mutation, cleanup, commit, push, publication, generated-store refresh, credentials,
  cloud operations, persistent settings, service restart and automatic successor.
- Done when: independent source/Core admission and rejection, typed HIR/Core representation,
  evaluator/Core-only Go parity, all 25 statement shapes with bounded initializer subsets and
  mixed placements, lexical/order/unused/lazy traces, supported types/carriers, executable
  Application IR, inherited artifacts and frozen compatibility pass. Prove contained scaling
  through 256 locals and affected compiler/consumer, application/CLI, vet, editor/docs/format.
- Validation: cached Go 1.25.13 offline, private caches and verified temporary Linux cgroups
  per `tests/containedexec/README.md`: 1 GiB hard memory, zero swap, 128 tasks, 800 MiB proactive
  stop, 30-second generated-child and independent service deadlines, whole-tree cleanup.
  Warm compiler ceiling stays 128 MiB / 5 seconds. Temporary 700 MiB reclaim is allowed for
  bootstrap; no uncontained fallback or global compiler/GC workaround.

## Evidence

Source and Core independently admit the new bounded tree placement. Version inheritance,
parser admission, signature checks and HIR routing include v0.91.0. HIR/Core shapes,
evaluator and Go backend production code, and public identities remain unchanged.

Focused type/carrier, source/Core refusal, hidden-local and depth checks, lexical references,
representation and dependent-condition checks pass. Executable Application IR retains its
canonical projection; affected application tests, complete CLI, vet and frozen compatibility
pass. All 200 new layout partitions pass: 3,820 methods and 2,859,840 evaluator/pristine-Go
outcomes, plus generated-Go ordered/lazy traces. The matrix crosses all 25 statement shapes
with ordinary and depth-two leaf returns, zero/one/two/three choices at every scope, all
subsets of root/left/right scope slots, selected five-local mixed layouts and unused locals.
Three-local sequences rotate true-only, false-only and both-arm nested choices; five-local
sequences also contain a nonnested choice. Statement bits, outer initializer bits, inner
initializer bits and return bits are separate; the two inner initializer bits are shared
across initializers. Every supplied vector is exhausted. This does not claim arbitrary
independent shape/condition assignments at every local or every finite sequence length.
A separate five-return-shape matrix covers 640 outcomes with independent statement,
initializer and return condition bits. All inherited compiler checks pass.

All 168 v0.91 scale fixtures through 256 locals pass both the regression run and fresh
isolated normal-inlining measurements. Peak compiler RSS is 82.11328125 MiB; maximum
elapsed time is 0.1993254639673978 seconds. Closure depths are zero, two or four, independent
of local count. All five fixture input files match the terminal-suite exports exactly.

Temporary evidence lives under `/tmp/pipelang-v091-proof`; this record will own the final
completion result. Failed probes are retained as supplementary evidence. A copied fixture
had a missing closing brace; its source was fixed. Consumer assertions were updated for the
prior-version diagnostic and the new root conditional dependency shape. The first combined
layout probe timed out, so the unchanged case inventory now uses 200 disjoint partitions
under fresh containment. Limits and compiler flags were not raised.


## Completion and reproduction

All 575 discovered compiler tests have passing evidence across 507 accepted validation
units: 506 terminal-suite units plus one final two-test type/carrier unit. The latter
strengthens a constant outer condition to a runtime boolean so both statement branches
execute. Only those two tests use that fixture; all other compiler/test inputs remain
identical to the terminal binary inputs. The accepted manifest records the replacements
and retains every other passing result. No production code changed during the full run.
The complete eleven-version memory matrix passes all 623 cases. No unresolved verification
failure remains.

All 694 recorded validation cgroups are removed. Every recorded unit retained unchanged
max/OOM and swap-event counters and used zero swap. Bootstrap/integration used temporary
700 MiB reclaim without raising the hard or proactive limits. The initial sandbox preflight
started no workload; reviewed host invocations used the canonical launcher. Both protected
stashes and the 108,166-path ignored inventory remain unchanged; the inventory SHA-256 is
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`.
This proves path inventory, not ignored contents. HEAD remains `83cd8adc` on `js/pipelang`;
all changes are uncommitted for review, with no staged changes or generated-store refresh.

Reproduce using cached Go 1.25.13 offline, private caches and `tests/containedexec/README.md`:

- Build the compiler test binary inside `run.py`. Run from `src/lib/pipelang`, using fresh
  units, batch size 15 and all nine numbered split inventories, including the 200 v0.91
  partitions. Run all eleven memory-version subtests separately. This run used a temporary
  `suite.py` copy with two independent units at a time and explicit memory-version splits.
- Export `TestCompilerMemoryLocalSequences/v0.91.0`; run `matrix.py` for 168 fresh isolated
  measurements. Compare source, generated Go, tests, module files and imports with final fixtures.
- Run Core/evaluator/backend/HIR, Application IR and frozen compatibility; affected application
  tests selected by `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`; complete CLI; vet
  for compiler, Application IR, application and CLI. Editor assertions/syntax, Go formatting,
  authored JSON/YAML, task routes, local documentation links and whitespace checks pass.

Temporary evidence includes `verification-summary.json`, `accepted-compiler-manifest.json`,
`source-hashes.json`, `final-evidence.json`, `fixture-identity.json`, `protected-state.json`,
`terminal-batches/suite.json`, `types-final.json` and `isolated/matrix.json` under
`/tmp/pipelang-v091-proof`. Temporary files may expire; this record owns durable proof.
Production changes are confined to generic source/Core admission and version/signature/HIR
routing. Tests, editor guidance and task documentation also changed. HIR/Core node shapes,
evaluator and Go backend production code, public identities and engine/package boundaries
remain preserved.

Full repository build/CI/all-library suites, sustained fuzzing, interactive editor use,
non-Linux containment and live operations were not run. No commit, push, publication,
successor selection or automatic handoff is authorized by this completion.
