# Conformance performance

## Objective contract

- objective_id: `TASK-021-conformance-performance`; state: `completed`.
- execution_skill: `dorkpipe-objective-execution`; execution_authority: explicit user authorization.
- Authorized outcome: shorten conformance testing without weakening proof, in three independently documented slices below. Automatic checkpoints within this objective; handoff only on user request. Normal session.
- Done when coverage/outcomes remain unchanged, performance improvement is reproducible, and resource limits remain satisfied. Document an unhelpful optimization instead of forcing it.
- Preserve every language contract, public compiler/semantic/Application IR identity, frozen 45-source compatibility and generic engine/package boundaries. No language bump.
- Validation: focused checks followed by one complete affected terminal validation after final code changes. Cached Go 1.25.13 offline, private caches, normal inlining, canonical containment including all memory/swap/task/deadline controls; 128 MiB / 5 second warm direct compiler ceilings. No uncontained fallback.
- Exclude commit, push, publication, language expansion, worktree, stash mutation, destructive cleanup, generated-store refresh, credentials, cloud/live operations, service restart, persistent machine changes and automatic successor tasks.

## Admission

Clean saved checkout `js/pipelang` at `35680d50e749ff19f20fd1a5ef7d9a883dd00632` on 2026-09-06; all 33 v0.96 manifest paths match. This commit supersedes historical uncommitted wording in the v0.96 record. Both protected stashes remain unchanged. Ignored inventory: 108166 paths, SHA256 `ba9e796198c62a673750896380d10484150d1fad2e496b3b690` (inventory only).

Admit completed [v0.96 proof](depth-three-straight-line-initializers.md): 623 discovered tests/seeds/examples, 733 passing units, frozen compatibility and affected integrations passed, 64 direct-initializer compiler measurements passed, no unresolved failures. Temporary receipts: `/tmp/pipelang-v096-proof`. Summed unit elapsed time is 7595.409254191443 seconds, not overall wall time; two workers ran concurrently. No whole-suite baseline replay.

## Slice 1: performance baseline

State: completed. Separate source analysis/lowering, Core validation, evaluator execution, generated compilation and launcher overhead. Record representative small/large workload inventories, CPU/allocation and memory evidence. New artifacts belong under `/tmp/pipelang-performance-proof`.

## Slice 2: test harness overhead

State: completed; batching rejected on resource evidence. Investigate modest batching of nested finite-local layouts. Preserve each case/vector/result/trace, deterministic artifacts and rejection checks, including separate pristine and instrumented executable proof. Keep isolated compiler measurements independent.

## Slice 3: evaluator preparation

State: completed, justified by slice 1. Consider validated immutable preparation only if measured repetition justifies it. Preserve existing EvaluateProgram behavior, invocation argument/carrier validation, malformed-Core refusal and evaluation order. No mutable-identity cache or validation bypass; caller mutation must not invalidate prepared validation. Measure setup and retained memory as well as repeated calls.

### Slice 1 evidence

Completed receipts attribute 3859.285 s to v0.91 layouts (50.8% of summed suite units) and 530.628 s to v0.89 layouts. Overall wall time is unavailable from completed receipts; file modification spans are not claimed as exact runtime.

The opt-in `TestConformancePerformanceProfile` uses one and five nested locals in the deepest statement shape with nested returns, three functions, respectively 512 and 8192 condition vectors. Source sizes are 2137/2720 bytes; Core JSON sizes 60966/78647 bytes. Baseline phase timings (small/large, microseconds): analysis 423.7/557.4, HIR lowering 4847.4/8456.9, Core lowering 611.0/903.3, Core validation 354.3/483.4, EvaluateProgram 407.5/547.4, Go generation 2044.7/2800.8. Lowering APIs include their existing internal checks; these are API-boundary timings, not additive exclusive phases. ValidateProgram alone accounts for about 87–88% of repeated EvaluateProgram time. Validation allocates 155152/263696 bytes and 2712/3600 objects per call; full evaluation allocates 212136/362073 bytes and 2767/3677 objects. This justifies measuring immutable preparation in slice 3.

The profile unit passed at 21.6 MiB aggregate peak with zero swap/events. Full before/after layout runs separately include generated-Go build/run and launcher timings, CPU and allocation profiles. The opt-in profile is skipped during ordinary conformance runs and imposes no timing threshold.

### Slice 2 decision

Three repetitions of unrestricted two-method batching reduced median v0.89/24 time from 33.707 s to 26.535 s and v0.91/164 from 24.876 s to 20.980 s, but raised warm aggregate peaks (v0.89 about 207 to 260 MiB; v0.91 about 246 to 398 MiB). A vector-budgeted candidate capped combined constants at the existing five-local fixture inventory and kept larger samples isolated, but still reached about 398 MiB. Combining it with preparation did not remove that regression (about 385 MiB). Neither candidate crossed containment limits, but the stronger no-resource-regression acceptance condition is not met. Reject both batching changes and retain `batchSize=1`, every pristine/instrumented compilation and all existing case boundaries. No fault-injection proof is claimed for rejected, unshipped batching. The user explicitly permits documenting an unjustified optimization instead of forcing it.

Profiles support concentrating the retained change in slice 3: the v0.91/164 baseline consumed 9.98 sampled harness CPU seconds in 24.86 wall seconds; 7.48 CPU seconds were under EvaluateProgram and 6.92 under ValidateProgram. Allocation profile total was 4078 MiB, with 2611 MiB attributed cumulatively to Core validation. Profiles exclude compiler child CPU; generated build/run timing is measured separately. No compiler generation or language admission code changes are needed.

### Retained slice 3 design and focused proof

`coreeval.PreparedProgram` owns the complete copied Core graph and private lookup map.
Preparation validates its owned copy once. A private structural copier traverses all fields,
including inactive expression members, carrier/type pointers and semantic metadata; unsupported
future field kinds and cyclic graphs fail closed. Returned outcomes are independently copied,
including type metadata, so neither original input mutation nor returned-value mutation can alter
later calls. Shared preparations are safe for concurrent evaluation; preparation itself must not
race with caller mutation. There is no global cache, mutable-object key or unsafe bypass.

`EvaluateProgram` still validates each supplied program, builds lookups and invokes the same
argument/carrier checks and evaluator. Only the finite-local test helper opts into preparation
outside its vector loop. Its dependency closure, case/vector/trace oracles, deterministic artifact
assertions and pristine/instrumented generated compilation are unchanged. Other callers keep
ordinary evaluation. The shared generated-Go helper exposes opt-in measurements through the
existing contained `Measure` API; this uses the same child containment as `CombinedOutput`.

Focused canonical argument tests now additionally run prepared evaluation for supported string,
record, list, Optional and Result carriers, including unused arguments and lazy branches.
Independent malformed-Core admission assertions compare preparation errors against Core admission.
New ownership tests erase every reachable original Core field and every returned outcome field,
then repeat evaluation against independent expected outcomes/errors; include nested helpers,
carrier payloads, literal numeric metadata, missing identity/arguments and nil/zero preparations.
Eight concurrent callers pass focused race checks. Snapshot checks preserve NaN payload bits,
negative zero, infinity, arbitrary string bytes and nil-versus-empty collections; reject pointer
and slice cycles and unsupported future fields. Focused checks and race detection pass in
canonical containment; race/build phases use documented temporary memory.high=700 MiB reclaim.

The baseline no-op launcher median was 0.100 s outer elapsed versus 0.00559 s inside the unit.
Instrumented original v0.89/24 and v0.91/164 retain 72 and 38 generated child invocations;
generated Go build/link/run totals are 25.131 s and 17.331 s in 35.299 s and 24.962 s units.
These are generated build/run measurements, not direct compiler-only claims. Compiler children
remain independently contained, and isolated compiler measurements stay separate.

### Final repeatable measurements

Same cached offline Go 1.25.13, private cache and normal containment; three repetitions per
layout, with CPU/allocation profiling on repetition zero of both before and after. The host
is shared, so report observed distributions rather than treating percentages as universal.

| Workload | Before seconds (three runs) | After seconds (three runs) | Median reduction |
| --- | --- | --- | --- |
| v0.89 layout 24, 36 methods, 72 generated children | 35.235, 33.015, 33.707 | 24.798, 24.751, 24.695 | 26.6% |
| v0.91 layout 164, 19 methods, 23808 outcomes/traces, 38 generated children | 24.876, 23.896, 26.596 | 17.520, 17.320, 17.302 | 30.4% |

The final opt-in phase benchmark reports ordinary/prepared call costs of 364.177/27.053 μs
(small) and 519.211/44.721 μs (large): 13.5×/11.6× faster repeated calls. Preparation costs
1.236/1.718 ms, breaking even at approximately four repeated calls for these workloads.
Per-call allocations fall from 210264 bytes/2765 objects to 54168 bytes/52 objects (small)
and from 360201 bytes/3675 objects to 95560 bytes/74 objects (large). Preparation itself
allocates 621018/864476 bytes and 5469/7145 objects, retaining approximately 193698/245807
bytes per program across 32 retained snapshots after GC. These retained-heap estimates
include runtime noise; preparation is not a free or zero-memory operation. Final phase-unit
aggregate peak is 27.7 MiB. No speculative whole-suite speedup is extrapolated.

Terminal validation passed after final Go edits. New source is limited to the generic
offline Core evaluator and test/measurement code; no parser, language admission, lowering,
Go generation, serialized artifacts, Application IR, workflows or package semantics changed.

Final sampled harness CPU falls from 13.12 to 2.83 s for v0.89 and from 9.98 to 1.88 s
for v0.91. Sampled total allocation falls from 4.83 to 1.69 GB (as displayed by pprof)
and from 4078.09 to 1404.57 MiB respectively; these are allocated totals, not resident peaks.
Corrected final aggregate ranges are 199.75–206.20 MiB (v0.89) and 227.60–238.39 MiB
(v0.91), compared with 201.63–259.72 and 232.18–246.37 MiB before. Both final maximums
are below their sampled baseline maximums. No claim of universally identical peak memory is
made: generated-child compiler peaks vary despite unchanged generated inputs. The retained
preparation graph is explicitly accounted for above. All repetitions remain far below the
unchanged 800 MiB proactive / 1 GiB hard containment budget, with zero swap and no max/OOM
events; independent warm compiler ceilings are verified separately.

### Terminal review correction

During the first terminal run, review found that replacing the original pairwise entrypoint
selection with a concatenated map-key lookup could accept a different package/path pair when
Core identities contain an embedded NUL. Core admission does not prohibit that data. The
correction retains map lookup but also checks both selected identity fields exactly, preserving
original EvaluateProgram behavior without narrowing Core admission. A regression test exercises
the accepted exact identity and a colliding-but-different pair in both evaluator APIs.

The initial terminal attempt was stopped after 234 completed units; its two active canonical
launchers removed their exact temporary cgroups. Receipts are retained under
`/tmp/pipelang-performance-proof/initial-terminal`, with `initial-cancellation.json` recording
cleanup. That partial run is not terminal acceptance. A fresh complete terminal run follows the
final correction; the evidence-based rerun does not change scope, limits or test inventories.

The complete 200-part v0.91 layout matrix passes on the corrected final binary. Its summed
unit time decreases from 3859.285 to 3009.039 seconds (22.0%). This is measured matrix
work under two concurrent contained workers, not inferred overall wall time.

## Terminal proof

The corrected final source passes the complete compiler inventory: 627 discovered test/seed/
example functions in 733 isolated units, with no failures. All 623 inherited functions and
all inherited named pass/skip outcomes are unchanged. Three new ownership/identity tests pass;
the opt-in profile is skipped in the ordinary inventory and passes separately. All 278 logged
finite-layout inventories are identical to the baseline, preserving every counted method,
evaluator/pristine-Go result and ordered trace. No original test is removed or newly skipped.
The generic Core snapshot unit and focused race checks also pass.

Summed unit time is 6721.473 seconds versus 7595.409 seconds in admitted completed receipts,
an 11.5% decrease. Actual final elapsed time including build/discovery is 3421.903 seconds
(57.0 minutes); the prior receipts do not provide a reliable corresponding overall wall time.
Do not label summed concurrent unit times as overall wall time. The superseded partial attempt,
profiling experiments and focused checks are excluded from these terminal timings.

All Core/evaluator/backend/HIR, Application IR, frozen 45-source compatibility, affected
application tests, full CLI and compiler/consumer/application/CLI vet checks pass. All inherited
memory cases run in the complete inventory. All 64 v0.96 generated direct-compiler fixture
sources are byte-identical to the admitted fixtures and pass fresh isolated normal-inlining
measurement. Peak waited direct compiler RSS is 107.4375 MiB and maximum elapsed is
0.622269804 seconds, below unchanged 128 MiB / 5 second ceilings. The baseline direct peak
was 107.2461 MiB and maximum elapsed 0.701264687 seconds; this small RSS variation does not
change compiler resource acceptance. No lowering or Go-emission source changed.

The highest whole-unit peak is 746.6055 MiB in v0.91 layout 103, versus 733.4766 MiB for the
same baseline unit. Both remain below the unchanged 800 MiB proactive stop and 1 GiB hard cap.
Paired worst-case follow-up measurements distinguish this small peak variation from the large
systematic increase that caused batching to be rejected. Completed receipts report zero swap
and no max/OOM/swap event increases; temporary memory.high reclaim during labeled builds,
race and integration phases retains all hard controls.

Reproduction: use the cached Go path, `tests/containedexec/README.md`, final test discovery and
all documented splits. Run the eleven inherited memory versions and each v0.92-v0.96 scale
test separately. Run Core/evaluator/backend/HIR, Application IR and frozen compatibility;
application selection `PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe`; full CLI and
corresponding vet. Export v0.96 memory fixtures and run `matrix.py` for the independent 64-case
warm direct compiler lane. Use the opt-in phase test and numbered v0.89/24, v0.91/164 layouts
for repeated timing; v0.91/103 is the worst aggregate-memory comparison. Preserve normal
inlining, offline flags, private caches, two suite workers and every containment control.

Temporary binaries, fixture modules, compiler outputs, private caches, CPU/allocation profiles
and receipts are under `/tmp/pipelang-performance-proof`. `terminal-inventory.json`,
`terminal-suite.json`, `terminal-summary.json`, `corrected-measurements.json`,
`phases-final-v3.output`, `peak-pairs.json`, `isolated/matrix.json`, `integration.json` and
`final-evidence.json` carry detailed evidence. Temporary artifacts may expire; this record owns
the durable result. Initial attempt and intentional failing-regression receipts remain retained.

Generic engine/package boundaries are preserved. No language version, public serialized
identity, schema, authored workflow semantics or backend changed. Generated repository stores
were not refreshed.
No commit, push, publication, worktree, stash mutation, credentials, cloud/live operation,
service restart, persistent setting change or successor task occurred. Full repository build/CI,
all-library tests, sustained fuzzing, interactive editor and non-Linux containment were not run;
the affected validation and editor/static documentation checks are the intended scope.

### Final resource decision and completion

Three additional paired v0.91/103 runs used identical warm private-cache and containment settings.
Baseline aggregate peaks were 688.305, 712.191 and 646.324 MiB; final peaks were 698.832,
684.238 and 691.195 MiB. The ranges overlap, final maximum is lower, and median difference
is 2.891 MiB (0.42%). This supports ordinary run-to-run variation rather than a systematic
resource regression. All six pass with no max/OOM/swap events. The owned graph's measured
retention remains an explicit cost; no zero-allocation or universally identical-RSS claim is made.

All three authorized slices are complete: baseline established; batching investigated and rejected
on memory evidence; immutable evaluator preparation implemented and verified. The source change
shortens repeated conformance evaluation while preserving all original coverage, outcomes and
resource ceilings. No further implementation approval or successor task is pending.

The final audit confirms removal of 897 successful temporary units, all 238 units from the
superseded initial attempt (including both cancelled units), and the intentional failing
identity-regression unit. All original named outcomes and all 278 logged layout inventories
match the baseline. Documentation, authored YAML, task routes, local links, formatting and
editor assertions/syntax checks pass. Branch `js/pipelang`, HEAD `35680d50`, both protected
stashes and the 108166-path ignored inventory remain unchanged; the inventory does not prove
ignored file contents. Changes remain unstaged and uncommitted.

Exact toolchain used: `<go-module-cache>/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin/go`.
The private cache is `/tmp/pipelang-performance-proof/cache`. Source and final-evidence manifests
are retained alongside the receipts for review, with no generated artifacts added to the repository.

## Further performance pass

- objective_id: `TASK-021-conformance-performance-followup`; state: `completed`.
- execution_skill: `dorkpipe-objective-execution`; execution_authority: explicit user request to try further, including native code where bottlenecks justify it.
- Authorized outcome: further reduce conformance time without losing cases, independent result/trace oracles, deterministic artifacts, pristine/instrumented executable checks or resource proof.
- Done when a measured improvement survives focused correctness/resource checks and one complete affected terminal validation; reject candidates that do not justify their cost.
- Inherit all exclusions, language/identity invariants, containment and Git boundaries above. Automatic checkpoints; handoff only on user request; context-pressure policy warn and continue.
- Admission: clean saved checkout `js/pipelang` at `04c07808221a4002ce08db7282d102aec7d97f92`; all 16 previous manifest paths match. Both protected stashes and the 108166-path ignored inventory are unchanged. This externally created commit supersedes earlier uncommitted wording.
- Previous completed proof is the baseline, not a new run. Further artifacts: `/tmp/pipelang-performance-followup`.

The corrected profiles show generated build/link/run dominating representative wall time, while the Go evaluator is already compiled to native machine code. First investigate moving bulky independent oracle tables out of Go literals into runtime fixture data, preserving exact values, trace order, vector counts and separate generated executions. Assembly is justified only by a measured remaining computation hotspot; it cannot remove repeated compiler/linker work.

Focused candidate evidence: JSON fixtures alone reduced three-run median v0.89/24 to
21.049 s and v0.91/164 to 10.206 s. Four-method batches reduced those medians further to
10.589 s and 4.883 s. Host load varied; these are observations, not universal timings.
Representative batching peaks remain below the original literal-table implementation.
Retain the bounded four-method candidate; no production compiler or evaluator changes.
The new transport test and both small v0.85/v0.86 layout cases pass. Seven deliberate
fixture faults fail visibly: missing file, invalid JSON, missing vector, wrong value,
changed trace order, removed repeated event and nil replacing an empty trace. Each
executes in a fresh contained unit; expected failure receipts are labeled separately.
Complete affected terminal validation passed after final Go edits.

### Follow-up final evidence

The retained implementation changes only two compiler test files and their task/test guidance.
Independent oracle values and ordered traces become per-method JSON fixtures, loaded once by
each generated test with an exact vector-count assertion. Nested layouts use at most four methods
per module. Every method/vector remains present, and pristine and trace-instrumented generated
modules still compile and execute separately. `-count=1` prevents test-result cache reuse. The
source generator, independent oracle model, production compiler and evaluator are unchanged.

Final alternating before/after measurements use the admitted previous binary and the exact final
binary, cached offline Go 1.25.13, the same private cache, three repetitions, normal inlining,
and unchanged containment. Repetition zero includes CPU/allocation profiles on both sides.

| Workload | Before seconds | After seconds | Median reduction | Before/after aggregate peak range |
| --- | --- | --- | --- | --- |
| v0.89 layout 24, 36 methods, 21504 vectors/traces | 24.898, 25.567, 24.689 | 11.102, 9.420, 9.466 | 62.0% | 200–216 / 165–169 MiB |
| v0.91 layout 164, 19 methods, 23808 vectors/traces | 17.538, 17.296, 17.196 | 4.382, 4.441, 4.355 | 74.7% | 229–254 / 111–126 MiB |

Generated child invocations fall from 72 to 18 and from 38 to 10 respectively, while every
method retains both executable checks. Generated build/link/run still takes about 7.8 seconds
of the median 9.47-second v0.89 run and 3.39 seconds of the median 4.38-second v0.91 run.
These measurements support reducing compiler/linker work; no measured kernel justifies adding
handwritten assembly or an architecture-specific implementation in this pass. Go execution is
already native. No language contract, compiler flags, machine settings or resource ceilings change.

The complete final compiler run discovers 628 functions (all 627 inherited plus the transport
regression), and all 733 fresh units pass. Actual overall wall time is 2136.733 seconds (35.6
minutes), versus the previous 3421.903 seconds (57.0 minutes): 37.6% lower on this host.
Summed concurrent unit time falls from 6721.473 to 4149.942 seconds (38.3%); do not present
summed time as wall time. All 200 v0.91 layout units pass in 828.293 summed seconds versus
3009.039 previously (72.5% lower). Shared-host timing variation remains a measurement limitation.

All inherited named pass/skip outcomes and all 278 finite-layout inventory messages match.
All 64 exported compiler fixture sources are byte-identical to the admitted baseline. Existing
isolated direct-compiler, consumer/Application IR, frozen-45 compatibility, CLI and evaluator race
proof is retained because production source and fixture bytes are unchanged; those unaffected
suites/matrices were not replayed. The complete compiler suite, focused transport/small layouts,
seven deliberate fixture-fault checks and Go vet passed freshly. The normal opt-in profiling test
continues to skip outside its explicit profiling invocation, as in the baseline.

The final audit accepts 772 temporary units, including the seven expected test failures: all
units are removed, memory hard-limit/OOM and swap counters remain unchanged, and zero swap is
used. Compiler-only limits remain 128 MiB/5 seconds; build/test/vet aggregate measurements are
separate. Both protected stashes, HEAD and the ignored-path inventory are unchanged. No generated
artifacts are added to the checkout. Temporary binaries, profiles, fixture exports and receipts
remain under `/tmp/pipelang-performance-followup`; per-execution generated fixture directories
are removed by the existing test helper. Generic package/engine boundaries are preserved.

Evidence: `final-evidence.json`, `performance-summary.json`, `terminal-summary.json`,
`terminal-suite.json`, `final-measurements.json`, `faults.json` and `vet.json` in that temporary
proof directory. Documentation routes, YAML parsing, Go formatting and diff checks pass.
The follow-up is complete and uncommitted; no successor implementation is selected.
