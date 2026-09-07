# Conformance execution performance

## Objective contract

- objective_id: `TASK-021-conformance-execution-performance`; state: `completed` for this validated performance pass; 30-second target remains unmet.
- execution_skill: `dorkpipe-objective-execution`; execution_authority: explicit user request to pursue approximately 30-second conformance without compromising design.
- Timing scope: user explicitly selected a complete rerun with verified compiled artifacts retained. Every current test and oracle still executes; fresh compiler resource measurements remain mandatory.
- Authorized outcome: remove measured repeated compilation, validation, evaluation and launch work while preserving every input vector, independent oracle, language contract, deterministic artifact and explicit resource proof. Thirty seconds is a target to investigate, not a promised measurement.
- Done when the largest justified improvements are implemented and validated, clean and complete-rerun timing are reported separately, and any remaining gap to the target is explained from measurements. Never substitute cached test outcomes for executing tests or conceal skipped compilation.
- Inherit generic package/engine boundaries, v0.96.0 and all public identities, complete carrier/argument validation, caller ownership and concurrent prepared-program safety. All compiler/test execution stays in canonical containment: cached offline Go 1.25.13, normal inlining, 1 GiB hard limit, zero swap, 128 tasks, proactive 800 MiB stopping, child/service deadlines. Keep warm direct-compiler ceilings 128 MiB/5 seconds independent from executable reuse.
- Automatic checkpoints within this objective; handoff only on user request; warn and continue under context pressure. Focused proofs before one complete affected terminal run after final material changes.
- Exclusions: commit, push, publication, language expansion, worktrees, stash mutation, destructive cleanup, generated-store refresh, credentials/cloud/live operations, service restarts, persistent machine changes and automatic successor tasks.

## Admission and investigation

Clean saved checkout `js/pipelang` at `87c1df6e59d3668e8dd73fdc31a8b7ba9b88fe75`.
All eight previous manifest paths match; both protected stashes and the 108166-path ignored
inventory are unchanged. The externally created commit supersedes earlier uncommitted wording.
Admit the completed [previous proof](conformance-performance.md#follow-up-final-evidence):
628 discovered functions, 733 passing units, 35.6-minute overall wall time, 278 preserved
layout inventories and 64 byte-identical compiler fixtures. No whole-suite baseline replay.
Temporary work and receipts: `/tmp/pipelang-execution-performance`.

The target requires approximately 71 times less wall time. The completed receipts identify
v0.91 layouts (828 summed seconds), v0.94 subsets (666), v0.89 subsets (516), v0.96 layouts
(308), compiler memory sequences (234) and v0.94 layouts (232) as the largest groups. Existing
profiles attribute most evaluator allocation to copying full argument slices for lexical
bindings. Investigate invocation-owned reusable lexical frames, preserving push/pop scope,
caller-owned spare capacity, error paths, carriers and fresh frames for calls/predicates.
Separately investigate exact compiled-artifact reuse that still executes every current oracle;
measure it distinctly from a clean build. Do not relax fresh compiler resource measurements or
containment to reach a timing target. Current available memory does not justify increasing the
two-worker proof concurrency.

## Investigation checkpoints (superseded by final evidence below)

- Invocation-owned frames: implemented lazy detachment from caller storage, scoped push/pop,
  and independent frames for calls/predicates. Focused Core/admission/argument/ownership tests
  pass, including spare-capacity sentinels and scope restoration after nested expressions/errors.
  Phase measurements: prepared small/large 19.25/32.38 microseconds, 14744/19656 B per call,
  versus prior 27.05/44.72 microseconds and 54168/95560 B. Retain pending full proof.
- Positive vector matrices: move immutable preparation outside fixed-program input loops in
  v0.89/v0.94 subsets and v0.90/v0.93/v0.94/v0.96 layouts. Preserve malformed-Core tests,
  per-invocation carrier validation, independent oracles and all vectors. Pending validation.
- Shared linking: opt-in `PIPELANG_GENERATED_BATCH=1` queues exact copied source/check bytes
  into separate packages and registers cleanup before a test can pass. A bounded driver links
  once, then executes each original module in a separate fresh child with its own fixture cwd.
  No cached test outcomes. Packages with initialization, directives, unsupported imports,
  special test entrypoints or observable test names use the original synchronous path.
  Initial 16-package candidate passes representative cases (v0.91 3.08 s, v0.89 7.02 s), but
  v0.89 memory rises to 238 MiB; reduce the batch limit to four and remeasure before retaining.
  A first build failed on Go parser/token import-name collisions; corrected aliases compile.

- Retained artifacts: opt-in private content-addressed executable cache binds source/check/module/
  driver bytes, explicit settings and pinned toolchain content; verifies binary digests before use.
  Current runtime fixture bytes are always consumed afresh. Fault checks pass for changed fixture
  failure on a cache hit, source invalidation and corrupted-binary rebuild. Corrected the test's
  temporary directory permissions after its first focused run was rejected as non-private.
- Warm representative measurements before final toolchain-key hardening: v0.91/164 1.752 s,
  v0.89/24 2.594 s, v0.94 subsets/99 2.554 s, all artifacts hit and all current oracles ran.
  Initial population respectively 5.122/8.980/17.487 s; these are individual cases, not a whole
  rerun claim. Final full population and grouped complete rerun remain pending.
- Reproducible runner: `tests/containedexec/pipelang_suite.py` rebuilds current tests and records
  complete dynamic inventory, source digests, receipts and separate build/execution wall times.
  Warm grouping amortizes startup/toolchain verification while preserving direct compiler probes.
- Terminal population checkpoint: 632 discovered functions (four new regressions), 733 units.
  The v0.91 compiler-memory version reached the unchanged proactive memory stop while retaining
  many executables; no max/OOM/swap events increased and the whole unit was removed. Preserve
  this failed receipt. Split only that version by its five existing choice categories, keeping
  every family/count and fresh default compiler controls. Continue unaffected population units,
  then repair this partition before timing the complete warm rerun. The runner-only repair changes
  the initial population's broad source snapshot; final warm proof must use the repaired runner.
- Final harness review: nonempty Go flags retain the original synchronous harness, with a
  regression for explicit coverage settings. Added per-artifact kernel locking to close a
  stale-miss/publication race between workers. Locks cover lookup, corruption repair, publication
  and execution, release on process exit, and have a bounded wait; the cache regression verifies
  serialization. Final focused checks and the complete warm rerun validate these additions.
- First grouped warm attempt: 433.316 s, 101 units, all inherited language/oracle outcomes pass;
  the new corruption regression encounters transient ETXTBSY while overwriting a just-executed
  inode. Replace the injected corrupted file atomically and preserve the failure receipt. The
  repaired focused check passes. This attempt is not a successful full-rerun claim.
- Remaining measured build cost: compile-only compatibility checks had no test functions and
  therefore fell back to repeated Go invocations. Admit their inert packages into verified
  artifact reuse, execute a fresh driver, and prove invalid changed source still fails compilation.
  Focused positive/negative checks pass; populate only newly covered artifacts before final timing.
- CPU sample of a retained v0.91/164 case: 27.84% flat SHA-256 AVX2, 14.20% compiler-generated
  copying, and 20.45% cumulative recursive evaluation (1.76 s CPU sample). This single-case sample
  identifies existing native hot paths, not a whole-suite attribution or justification for assembly.
- Accepted complete rerun checkpoint: 422.705 s overall, 421.684 s execution, 101 passing units,
  632 discovered functions. All 628 inherited functions/outcomes remain; 278 prior split inventory
  messages aggregate into 206 grouped messages with identical per-matrix method/vector totals.
  All 64 fixtures remain byte-identical and pass independent direct compilation (106.824 MiB,
  0.618 s maxima). 3465 artifact hits, zero misses; protected state and resource controls verified.
- Last measured repetition: the v0.82 full shape matrix still spends 41.4 s repeatedly validating
  one fixed program; v0.83 scope-pair vectors similarly revalidate a fixed dependency closure.
  Move preparation outside these two positive input loops, preserving every vector and all
  malformed-Core/admission/standalone API checks. Focused worst shapes before a final complete
  timing; generated source/oracle artifacts are unaffected, so no cache repopulation is needed.
- The older-matrix change yields a fully accepted 396.425 s rerun (632 functions, 101 units,
  zero artifact misses, inherited outcomes/totals and fixtures preserved). v0.82 falls to 6.006 s.
- Bounded concurrency trial: four independent shapes share one unchanged 1 GiB unit; generated
  compilation is serialized, each shape holds its slot through generated-check cleanup, and
  shared inventory totals are merged safely. The first trial exposed an outer wrapper that kept
  layouts serial; moving scheduling to the actual independent layouts gives 10.040 -> 4.126 s
  for the same eight v0.91 cases, with 85.3 -> 174.6 MiB peaks. Actual concurrent race proof passes.
  Apply the same bounded scheduling to the independent v0.89/v0.94 subset and v0.93/v0.94/v0.96
  layout callbacks, whose counters and oracle builders are local to each shape. Keep resource
  matrices sequential and the two unit-worker ceiling. Broader race/heavy checks precede full proof.

## Final evidence

The complete retained-artifact rerun passes in **326.532 seconds (5 minutes 27 seconds)**,
including rebuilding the current root test binary. This is **6.54 times faster (84.7% less
wall time)** than the admitted 2136.733-second baseline. The requested 30 seconds remains
unmet; this result is a measured improvement, not a minimum possible runtime claim.

| Configuration | Overall elapsed | Result |
| --- | ---: | --- |
| Admitted previous complete proof | 2136.733 s | 733 units passed |
| Initial executable-cache population | 1813.787 s | One proactive stop; repaired with all five existing choice families in fresh units |
| Final sequential retained-artifact rerun | 396.425 s | 101 units passed |
| Final bounded parallel retained-artifact rerun | 326.532 s | 101 units passed |

Population used an already warm Go build cache and underwent subsequent harness refinements;
it is not a clean compiler-from-scratch benchmark or a single successful terminal proof.
The failed population receipt and repaired partition remain available. Final rerun source
hashes match the checkout throughout execution and at audit time.

The retained implementation combines invocation-owned lexical frames, immutable preparation
outside positive vector loops, exact generated executable reuse, bounded shared linking and
optional four-shape scheduling within each of two unchanged contained units. Generated Go
build/link commands remain serialized within a unit. No production language contract, public
API or generic engine/package boundary changed. Handwritten assembly was not introduced:
the sampled hashing hotspot already uses Go's AVX2 implementation, and the measured gains
came from avoiding repeated work and safely scheduling independent cases.

Validation and audit:

- All 632 discovered root functions execute: all 628 inherited functions plus four regressions.
  Every inherited named outcome and per-matrix method/vector total matches the admitted proof.
  Grouping reduces repeated parent inventory messages from 278 to 206 without reducing coverage.
- 3465 verified artifact hits, zero misses. Every current oracle executes in a fresh native
  process with current fixtures; no test results are cached. Cache regressions cover changed
  runtime fixtures, source/settings invalidation, corruption repair, locking and special-harness
  fallback, including explicit Go flags and invalid compile-only source.
- All 64 compiler fixtures are byte-identical. Independent direct compilation passes with
  maximum 106.824 MiB and 0.618 seconds, within 128 MiB/5 seconds. The complete rerun also
  executes the original direct compiler resource probes, including 623 local-sequence readings.
- Core/evaluator/backend/HIR, Application IR consumers, frozen compatibility, application and
  CLI checks, vet, planner coverage and focused race proofs pass. The final concurrent race
  lanes cover actual v0.91 scheduling and v0.94/v0.96 scheduling; heavy parallel samples pass.
- The audit verifies 914 primary containment receipts, preserved limits, zero swap, no increased
  max/OOM events and removal of every unit, with only the documented repaired population stop.
  Final full-run aggregate peak per unit is 700.492 MiB. HEAD, protected stashes and the ignored
  inventory match admission. Final source formatting and whitespace checks pass.

The remaining elapsed time includes fresh direct compiler measurements, source/Core validation,
current evaluator/oracle execution, binary/toolchain verification and process/containment work.
Local-sequence compiler probes alone consume 122.462 summed unit-seconds, and v0.91 layouts
consume 111.935 summed unit-seconds. These are overlapping unit durations, not additive wall
time or a CPU profile. The two-worker ceiling and original compiler proof were retained;
this pass provides no evidence that another 10.9-fold reduction to 30 seconds is achieved.

Receipts, audit and generated fixtures/profiles are under `/tmp/pipelang-execution-performance`:
`parallel-rerun/summary.json`, `final-evidence.json`, `isolated/matrix.json`, `integration/`,
and the focused parallel race receipts. The retained `complete-artifacts` cache contains
3079 executable entries and uses approximately 14 GiB; deleting it requires repopulation.
It remains outside the checkout, alongside the existing private Go build cache.

Reproduce the complete warm rerun from the saved checkout with a fresh output directory:

```bash
python3 tests/containedexec/pipelang_suite.py \
  --go /home/jamie/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin/go \
  --output /tmp/pipelang-execution-performance/next-rerun \
  --cache /tmp/pipelang-performance-proof/cache \
  --compiled-cache /tmp/pipelang-execution-performance/complete-artifacts \
  --shape-batch-size 25 --parallel-shapes
```

The optional cache/parallel lane requires the documented contained Linux environment; ordinary
test helpers retain their synchronous fallback. No clean-from-scratch terminal timing is claimed.
Changes remain uncommitted. No push, publication, worktree, stash mutation, cloud operation or
persistent machine change occurred. This closes the validated pass without selecting a successor.
