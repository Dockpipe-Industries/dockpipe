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


## Cache compaction follow-up

- Objective: `TASK-021-conformance-cache-compaction`; state: `failed_verification`. Authority: explicit user request for the same rerun performance at 1/1000 of the retained cache size. No qualifying implementation was accepted.
- Baseline: clean saved checkout at `55e262e6ffbb66d9510fd7d0a90b0f0a43ec8705`, containing the previous performance pass.
- Done when retained storage is at most 1/1000 of admission with unchanged complete rerun performance, current oracles and invalidation still pass, and migration preserves proof. These conditions have not been met.
- Preserve exact executable bytes, all proof vectors and direct compiler resource controls. Compression is a storage representation change, not permission to omit tests or debugging content.
- Keep bounded containment and offline toolchain controls. No commit, push, worktree, stash mutation, unrelated cleanup or machine changes. Cache migration within the explicitly identified private cache is authorized by the user's reduction request.
- Evidence: `/tmp/pipelang-cache-compaction`; admit prior completed language proof and profile only the cache/harness changes.

### Strict target and measured experiments

The user superseded the initial roughly-half-size approach with **1/1000 size at the same
performance**. Admission measures 14,390,546,687 executable bytes across 3079 entries, or
14,413,168,640 allocated bytes including metadata. The executable-byte target is approximately
14.39 MB. Preserve the admitted 326.532-second complete rerun as the performance reference;
no new full-run performance claim is made.

| Experiment | Measured result | Acceptance |
| --- | --- | --- |
| Fixed-block deduplication plus fast zlib, 24 executable sample | 62.2–62.9 MB from 109.2 MB | Rejected: far above the strict storage target |
| Shared reference executable with installed zstd dictionary mode | 57.95 MB from 109.2 MB at level 3 | Rejected: insufficient reduction |
| Standard gzip storage prototype with legacy migration | Focused storage, corruption, invalidation and current-oracle checks passed in 13.984 s | Parked outside checkout after the user rejected approximately half size; no cache migration performed |
| All generated native function bodies, excluding shared runtime/debugging/other ELF data, compressed together with LZMA2 | 392,865,167 bytes reduced to 57,732,408 bytes | Still exceeds the target even before required executable data |
| Reversible direct-branch address normalization, function-body deduplication and LZMA2 | 42,549,440 payload bytes plus 380,252 index bytes | Still exceeds the target; not an executable cache |

The full corpus contains 257,974,257 bytes of native oracle/test-harness functions and
134,890,910 bytes of generated program functions. Normalization was reversed and compared
byte for byte for every extracted function. It leaves 136229 unique bodies totaling
382,755,649 bytes before compression. The experiment is specific to the current ELF64/x86-64
cache; it is not a portable runtime implementation or an information-theoretic lower bound.
These code-only experiments omit mandatory runtime, data and executable reconstruction state,
so their sizes must not be advertised as a usable cache size or an achieved reduction.

The evidence rules out the tested packing approaches for this requirement. A stronger shared
code representation or an architectural change to native oracle construction remains unproven;
neither the 1000-fold reduction nor unchanged rerun performance has been established. Do not
silently substitute a smaller target, cached outcomes, interpreted evaluator parity, omitted
compiler probes, or storage moved into another cache.

The gzip prototype is preserved in `/tmp/pipelang-cache-compaction/gzip-prototype` for inspection,
but all five affected harness source files were restored to the admitted commit before closing
this investigation. The retained cache is unchanged and the previous full proof remains the
accepted implementation. No new runtime dependency, migration, commit or push was performed.
Experiment input/metric manifests, scripts and containment receipts remain under the same
`/tmp` evidence directory; disposable compressed experiment corpora were removed after recording
sizes. Initial sample filename collision and unsupported zstd patch-mode failures remain as
supplementary receipts, with corrected supported-mode runs passing. Production and generic
engine/package boundaries are unchanged. The strict optimization objective remains unfulfilled.

Final read-only audit passed: all 3079 retained executable digests match their manifests,
executable bytes remain 14,390,546,687, source/test harness matches HEAD, and protected Git/ignored
state is unchanged. The audit used canonical containment (42.247 s, zero swap, unit removed).
`git diff --check` passes. Only the five task/routing documentation files remain modified.


## Shared native bundle prototype

- Objective: `TASK-021-conformance-native-bundle`; state: `completed`; authority: user accepted the proposed substantial-family prototype and said progress is progress.
- Done when a v0.91 layout-family prototype shares native checking code and links bounded bundles, retains every oracle/vector and fresh-process execution, and has measured retained footprint and matched rerun timing. The 1/1000 full-cache target remains an aspiration, not an acceptance claim for a partial-family prototype.
- Preserve independent expectations, source/toolchain invalidation, caller isolation and fresh direct compiler probes. No production language/backend changes, commit, push, worktree, stash changes, persistent machine settings or migration of the old cache.
- Evidence: `/tmp/pipelang-native-bundle`. Admission: `55e262e6`, five task-document changes from the preceding investigation; source/test harness clean.
- Initial implementation: a shared typed oracle package, bounded bundles for v0.91 layouts, and sealed executable snapshots so a large bundle is verified once without repeated whole-binary hashing for each fresh child. All native execution remains inside canonical containment.


## Shared native bundle final evidence

This section records prototype completion before the full-suite promotion below.

The bounded prototype passes the complete v0.91 layout family. It is opt-in and does not
claim a 1000-fold reduction or a new whole-suite runtime. The original full executable cache
remains available for comparison; no full-cache migration was performed.

| Matched family measure | Original retained executables | Shared native bundles |
| --- | ---: | ---: |
| Referenced artifacts | 560 | 200 |
| Retained bytes, including manifests | 3,001,822,088 | 1,302,465,584 before the controlled rebuild |
| Median execution time, three runs | 39.671 s | 34.154 s |
| Individual execution times | 48.262 / 39.671 / 35.259 s | 40.943 / 34.154 / 32.246 s |

This is **56.6% less retained native storage and 13.9% less median execution time** for the
same family. All three matched comparisons favor the bundle; the downward timing trend means
these measurements are not a cold filesystem benchmark. Both lanes use two contained workers,
four shapes per unit, normal compiler settings, the same inputs and explicit audit/profiling.
Initial bundle population took 276.530 seconds with a pre-existing Go compiler cache; it is
not a clean-from-scratch timing. The subsequent controlled cold-path check validates disposable
compiler storage separately.

Every compared run preserves 200 layouts, 3820 methods, 2,859,840 vectors and ordered traces,
and 2064 fresh native processes. Generated-source digests, current-fixture digests, original
native test names, every named parent/subtest outcome and all inventory totals match exactly.
The common oracle only handles fixture loading and comparison. Actual results still come from
native generated Go; expected results still come from the independent tree model.

The implementation uses a shared typed oracle package and at most 32 packages per bundle,
with the documented source/fixture thresholds. Each cached binary is checked against its
manifest while copying into a bounded memory file. Verified kernel seals prohibit writes,
resizing and seal changes. Children execute that immutable descriptor in fresh processes and
fixture directories. The normal helper keeps its previous behavior outside the opted-in
v0.91 family, including special-harness and explicit-Go-flag fallback.

Compiler storage is accounted for explicitly. Initial population/validation grew the shared
Go cache by 2,231,251,459 bytes. The provenance audit identified 5328 new files (2,178,853,392
bytes) whose embedded source-directory identities belonged to the 200 prototype bundles,
plus their new action records. After verifying hashes and absence from the pre-family inventory,
those files were removed. Pre-existing compiler-cache entries were preserved; the remaining
measured growth includes validation/root-harness compilation and is not counted as a native
bundle saving.

The runner now creates a fresh private build-cache directory beneath each run's output and
removes it only after every contained unit has exited and its cgroup is gone. Bundle misses
compile into this disposable cache; hits need no compiler intermediates. A controlled single
miss rebuilt one artifact, produced 65,622,405 temporary build-cache bytes, passed all 200
layouts, and removed that cache automatically. The subsequent warm run retained zero build-cache
bytes and passed with zero misses. Its overall time was 32.670 seconds including rebuilding
and listing the root test harness. The final explicitly audited run passed at 32.520 seconds
overall / 31.565 seconds execution, also with zero misses and zero retained build-cache bytes.

Validation:

- All family population and repeated comparison units pass under unchanged 1 GiB hard,
  zero-swap, 128-task and proactive-memory controls; native compilation remains serialized
  per unit. The initial population maximum was 700.617 MiB.
- Focused regressions cover current-fixture failure on a hit, source/shared-oracle invalidation,
  process-state isolation, immutable snapshots, cache-path replacement and incorrect digests.
  Original artifact corruption, build-key and special-harness regressions also pass.
- Final concurrent race checks and PipeLang vet pass. Python planner and disposable-cache
  lifecycle tests pass, including refusal to remove storage while a unit remains active or
  removal is unproven. Formatting and `git diff --check` pass.
- Final audit verifies original source/fixture/test identities and all outcomes after removing
  the intermediates. Snapshot hashes match the final harness and runner. HEAD, protected stashes
  and ignored inventory remain unchanged. No production Go files, language contract, generator,
  dependency versions or package/engine boundaries changed.

Evidence is under `/tmp/pipelang-native-bundle`: `family-evidence.json`,
`terminal-evidence.json`, `intermediate-removal.json`, `audited-rerun/summary.json`, and the
focused/race/vet receipts. Two diagnostic-only issues were repaired: the provenance scanner
initially treated a Go cache directory as a file, and the first post-cleanup auditor expected
flags set only in the outer shell to reach systemd workloads. The runner now explicitly forwards
`--audit-generated`; the final recorded source/fixture comparison passes. These did not represent
native test failures.

Reproduce the final family lane with a fresh output directory. The command uses the main
cache, where the prototype artifacts were subsequently consolidated:

```sh
python3 tests/containedexec/pipelang_suite.py \
  --native-bundle --audit-generated \
  --test-family TestV910NestedTerminalInitializersLayouts \
  --go /home/jamie/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin/go \
  --output /tmp/pipelang-native-bundle/next-family-run \
  --cache /tmp/pipelang-performance-proof/cache \
  --compiled-cache /tmp/pipelang-execution-performance/complete-artifacts \
  --shape-batch-size 25 --parallel-shapes
```

Changes remain uncommitted. This completes the authorized substantial-family prototype;
expansion to other test families, full-cache migration and the 1/1000 target remain unproven.


## Full-suite bundle promotion

The user authorized implementation after the completed prototype. Promote the validated
v0.91 bundle path as the Linux retained-run default, preserve the legacy comparison option,
and run the complete current inventory. Reuse the verified prototype artifacts, then remove
only obsolete entries from the task-owned executable cache after complete passing proof and
manifest revalidation. Preserve every oracle, fresh process and resource probe. No production
language changes, commit, push or persistent host settings. Evidence lives under
`/tmp/pipelang-bundle-promotion`. Completion requires a full warm rerun after migration, exact
inherited coverage and fixture parity, and measured whole-cache bytes and runtime.

### Implementation and migration completed

The retained-suite runner now selects the validated v0.91 bundle path by default on Linux.
`--no-native-bundle` preserves the original comparison lane. The runner emits `artifacts.json`
only for a complete, successful, source-unchanged retained run with verified unit cleanup.
It records executed keys, manifest digests, native bytes, hits and misses. It does not prune
retained caches automatically. Overall timing includes producing this receipt.

| Whole-suite measure | Accepted previous implementation | Implemented bundles |
| --- | ---: | ---: |
| Required native entries | 3079 | 2719 |
| Executables plus manifests | 14,391,156,329 bytes | 12,691,799,961 bytes |
| Complete rerun, including root build/list | 326.532 s | 271.910 s before pruning; 272.063 s after pruning |
| Discovered test functions | 632 | 634 |
| Contained execution units | 101 | 101 |
| Artifact misses | 0 | 0 in both runs |
| Retained bundle compiler intermediates | not applicable | 0 bytes in both runs |

The whole executable cache is **1,699,356,368 bytes (11.8 percent) smaller**. The affected
family retains its approximately 56.6 percent saving; this does not make the entire cache
56.6 percent smaller. The two new complete runs show no measured runtime regression. They
are warm retained-artifact runs on this host, not cold-build results or a controlled claim
that bundling alone explains the entire timing difference from the earlier baseline.

Migration first verified all 3079 old and 200 replacement binary digests. It reused the
replacements through hard links, without duplicating their physical contents. After the first
complete passing run and independent coverage audit, the live-key set proved exactly 560
old v0.91 entries obsolete; none were required by another family. The migration revalidated
source snapshots, receipts, manifests and binary hashes, acquired the existing per-key locks,
and removed only those entries. The prototype links were then consolidated into the main
cache. All 2719 retained binaries were verified again. The final complete rerun used precisely
that reduced set, with 3105 hits and zero misses. No deleted binary was moved to another cache.
Zero-byte lock files remain to preserve lock inode identity.

Both complete audits preserve all 632 inherited test functions and named outcomes, every
method/vector inventory, all 2064 v0.91 generated-source/current-fixture/original-test-name
records, 64 byte-identical compiler fixtures and 623 fresh compiler measurements. Audit mode
additionally enables the existing small/large profiler cases that the old full run skipped;
the auditor explicitly verifies both passes. The two added bundle regression functions pass.
All 103 receipts per run (build, list and 101 execution units) preserve the 1 GiB hard limit,
zero swap and 128-task limit, with no hard-memory/OOM events and verified cgroup removal.

Three Python regressions pass: complete inventory grouping, disposable-cache cleanup guards,
and executed-artifact inventory validation. All 185 Go files exactly match the already
validated prototype, so its focused, concurrent race and vet proof applies unchanged.
Formatting and `git diff --check` pass. HEAD, protected stashes and ignored inventory are
unchanged. Only test harness, runner and task-documentation areas changed; production Go,
language design and package/engine boundaries are preserved. Nothing was committed or pushed.

Storage accounting remains explicit: the 12.692 GB figure is executable data plus manifests,
with 12,710,539,264 allocated file bytes. The former family cache has zero file-content bytes
after consolidation. The separate pre-existing general Go build cache remains at
29,275,296,596 file bytes at completion and was not broadly pruned; no whole-workspace storage
claim is made. Root test binaries, logs, fixtures and migration receipts remain as local proof
under `/tmp/pipelang-bundle-promotion`. No generated artifact was added to tracked source.
The whole-suite 30-second and 1/1000-storage targets remain unmet.

Evidence: `full-before-prune-evidence.json`, `full-after-prune-evidence.json`, `migration.json`,
`cache-prune.json`, `cache-before.json`, `cache-after.json`, `storage-final.json`,
`go-proof-reuse.json`, and both full-run receipt directories beneath that evidence root.
The one-task migration script is retained there for review. Diagnostic scripts encountered
a Python-version helper mismatch and a receipt filename collision during staging; neither
caused native test failure or unverified deletion. They were resolved and the final migration
and coverage audits completed successfully.

## Shared runtime experiment (2026-09-07)

Objective `TASK-021-conformance-shared-runtime-experiment` is **completed with a demonstrated toolchain limitation**, authorized by
"lets go with that experiment" and the user-requested saved-checkout handoff. The previous
bundle-promotion proof remains admitted. First measure the pinned Go 1.25.13 shared-linking
facilities in private temporary storage; if feasible, compare the entire 200-layout v0.91
family against its retained bundle baseline before considering any other family.

Done when a reviewable representative-family prototype, or an evidenced technical limitation,
has native correctness, invalidation and isolation evidence, complete retained-storage accounting,
matched timings and an assessment against the 30-second / 14,391,156-byte whole-suite goals.
Preserve independent expected values and ordered traces, actual generated-Go execution, exact
source/toolchain/settings identity, every original test/vector and direct compiler resource
probe, fresh processes and fixture directories, and per-case mutable-state isolation. Never
reuse pass results. Count runtime, unique code, metadata, compression and reconstruction/compiler
support together; preserve debugging content. The main native and general Go caches are protected.
Keep the existing containment/deadlines and serialize native compiler commands. No production
language change, commit, push, worktree/stash mutation, broad cleanup or persistent host changes.
Evidence root: `/tmp/pipelang-shared-runtime-experiment`.

### Measured result and terminal boundary

The full representative family is complete: **200 bundles, 2,064 original generated
packages and 9,704 native named tests** pass in both candidate repetitions. Two independent
full-family exports have identical source, fixture and baseline identities; all 2,064
source/fixture/original-test audit records match. This is native replay of the complete
family, not a whole-suite or end-to-end conformance timing. Every current expected value
and ordered trace was regenerated by the existing independent model before retained replay.

| Matched native replay (two units, four shapes per unit) | First | Second |
| --- | ---: | ---: |
| Original retained bundles | 12.619 s | 12.416 s |
| Go shared runtime/checker | 19.971 s | 20.106 s |

The alternating order was baseline/candidate/candidate/baseline, with no concurrent build
or compression work. All four runs executed every original package in a fresh process and
fixture directory. The candidate is about **60 percent slower** in this matched phase.
The first full baseline export took 37.740 s including root build/export (33.579 s execution,
two units); the independently regenerated export used one unit alongside population and is
coverage evidence, not another matched speed claim. The 199 remaining candidate builds took
1,167.291 s after the first successful build; population used warmed private intermediates
and is not a cold-toolchain benchmark. The previous complete 634-test / 623-direct-compiler
proof remains admitted at approximately 272 s; this experiment did not rerun or accelerate
that whole suite, and does not establish the 30-second goal.

Go shared linking works for these native values/traces only after using GOPATH mode and a
complete shared standard library. Module-mode custom-library linking panicked in `LoadSyms`;
the partial standard-library probe linked but rejected the valid `25s` test timeout. Explicit
complete dependency names also exceeded the generated shared-library filename limit. These
failed probes and the successful `std` grouping are retained, not generalized into a claim
that all Go shared-linking arrangements fail.

**Debugging is a terminal limitation.** All 200 shared executables lack Go `.debug_info`;
the oracle shared library also lacks it. The pinned Go source deliberately appends `-w`
when `-linkshared` is selected (`src/cmd/go/internal/work/init.go`, lines 325–331).
An explicit `-ldflags=-w=false` probe fails with
`dwarf: missing type (no data): type:unsafe.Pointer`. No agent-authored stripping flag was
used in population. The final audit caught the toolchain's implicit omission, so the raw
size reduction is **not an invariant-preserving replacement**. Every original debug-bearing
baseline binary remains byte-verified in the protected cache. No toolchain patch, custom
loader or production language/backend change was attempted.

### Complete storage accounting

All numbers below are file-content bytes; MB is decimal. Lossless XZ level 6 archives were
stream-reconstructed and checked against every original file digest. The archives are an
alternative representation, not a claim that their coexisting raw files disappeared.

| Representation | Uncompressed bytes | Compressed bytes |
| --- | ---: | ---: |
| Current generated source/checks/fixtures and export records | 1,342,070,294 | 7,014,936 |
| 200 candidate binaries, both shared libraries, package archives, identity and reconstruction scripts | 572,308,277 | 57,290,612 |
| Initial Go/C reconstruction-support capture | 343,984,541 | 70,369,896 |
| Additional actual GCC 11 helper closure and audit record | 118,831,627 | 35,607,696 |
| Captured reconstruction set | **2,377,194,739** | **170,283,140** |

Even before external compiler support or the required debug fallback, the first two rows
need **64,305,548 compressed bytes**, about 4.47 times the entire **14,391,156-byte** goal.
The support capture conservatively retains the initial GCC 12 support snapshot as well as
the actual GCC 11 helper closure; this overcapture is explicit and is not a lower bound.
The actual driver's additional files have timestamps predating bootstrap and are now checked
for same-path byte changes. This accounting issue does not rescue the first-two-row result.

Program binaries alone total 361,036,288 bytes; shared libraries add 48,539,824 bytes, versus
1,302,426,120 bytes for the baseline family's binaries. That apparent 68.6 percent native
saving excludes reconstruction support and loses DWARF. Keeping the already-retained baseline
binaries for debugging adds **1,302,426,120 referenced bytes** to the candidate representation:
the measured archive set plus these uncompressed debug-bearing binaries is **1,472,709,260
bytes**, before coexisting raw experimental files. These references are already inside the
main cache; they are not copied or counted twice in its physical inventory. No compressed
end-to-end rerun was claimed. Stream reconstruction/verification took 10.267 s for inputs,
4.545 s for candidate material, 5.518 s for the initial support archive and 2.181 s for the
supplemental archive; these are sequential archive measurements, not additive suite timings.

### Verification, retained evidence and exclusions

- Current-value and ordered-trace fault injection fails in native checking without rebuilding.
  Executable/library corruption is rejected; sealed snapshots survive path replacement and
  reject writes/truncation. Six focused Python identity/sealing/planner tests pass.
- A dedicated mutable shared-oracle/program-state probe passes in 16 fresh processes. Repeating
  the same test twice inside one process intentionally fails on leaked shared-oracle state.
- Source, explicit build settings, shared-library bytes, pinned Go inputs and the audited C
  helper closure are checked for invalidation. Current fixtures are consumed afresh and are
  not a compiler-cache key or a retained pass result.
- Every recorded contained unit is removed, uses zero swap, and has no new max/OOM/OOM-kill
  events. Compilation remains serial in each unit and every native child keeps the 25-second
  test / 30-second process deadline. The full family's 200 artifacts and original debug
  fallback hashes are verified. The main cache still has 2,719 entries / 12,691,799,961 bytes.
- The initial trace fault targeted the value-only package; the corrected trace package proves
  rejection. The first audit accidentally tested its own still-running cgroup for cleanup;
  that was corrected. Its next debug-preservation assertion exposed the real toolchain limit.
  The terminal audit records that limit instead of treating missing DWARF as success.
- The language, generator, independent oracle model and all direct compiler resource probes
  are unchanged. The accepted whole-suite/race/vet proof is not repeated. Changes are confined
  to an opt-in test exporter, experimental contained replay/settings tools, focused tests and
  documentation; package/engine boundaries remain intact. No commit, push, stash/worktree
  mutation, broader-family run, baseline migration or persistent host change occurred.

Reviewable source: `generated_shared_experiment_test.go`, the small export hook in
`generated_batch_test.go`, `tests/containedexec/shared_runtime_probe.py`,
`shared_runtime_settings.py`, `test_shared_runtime_probe.py`, and the opt-in suite flag.
Evidence root `/tmp/pipelang-shared-runtime-experiment` contains `final-evidence.json`,
`comparison.json`, `family-{export,current}`, `population.json`, `*-storage.json`,
`faults-corrected-result.json`, `state-probe/result.json`, `explicit-debug.output`,
`terminal-audit.json`, and exact source/fixture/compiled artifacts. Terminal cleanup removed exactly 2,496,915,607 bytes from the task-owned disposable Go
cache after all 75 recorded units were gone; none of that cache remains. `storage-final.json`
counts 3,787,205,641 coexisting task-owned file bytes, including raw exports, archives,
failed probes and retained diagnostics. The separate preserved general Go cache contains
29,293,264,444 bytes; it was not pruned. HEAD, branch, both protected stashes and the exact
108,166-entry ignored inventory digest still match admission. Formatting, Python parsing
and `git diff --check` pass. Failed probe receipts are retained for review.

**Stop here.** The experiment supplies complete family correctness/timing evidence and a
conclusive debugging limitation, while failing the measured storage and speed objectives.
Do not promote this path or expand to another family automatically. Reaching both goals needs
a different representation/execution approach that can preserve debugging and substantially
reduce complete-suite work; this result is not a lower-bound proof for such alternatives.

## Continued native-sharing investigation (2026-09-07)

The user explicitly requested "keep working on this seems promising" after reviewing the
storage improvement. The same objective is **executing** again; the previous stop boundary
is historical and superseded by this request. Retain the saved checkout, all existing proof,
source/toolchain/settings identities, independent current oracles, fresh processes/fixtures,
complete v0.91 comparison checkpoint, debugging/reconstruction content and containment limits.
No commit, push, worktree/stash mutation, installed-toolchain patch, language redesign, broad
cache cleanup or network operation is authorized by this continuation.

First evaluate Go's supported plugin loading as a debug-preserving native sharing mechanism,
and measure a matched lossless compressed baseline so compression savings are not confused
with raw-versus-compressed comparisons. Select the next full-family prototype from the small
probe evidence. Include reconstruction cost and both common and candidate-only dependencies
in explicit storage accounting. Evidence: `/tmp/pipelang-shared-runtime-followup`.


### Continuation result: exact compressed native replay

The continuation is `failed_verification` against the original acceptance targets. It produced
a debug-preserving storage prototype and complete family proof; it did not promote a new cache
format, reach 1/1000 storage, or establish a 30-second whole-suite rerun.

- **Plugin probe:** the pinned Go toolchain built a 6,568,608-byte host and an 8,705,464-byte
  plugin with Go DWARF, versus the first original 7,461,516-byte executable. All ten packages
  and 38 native subtests passed with a normal plugin pathname. Alternating native replay was
  0.2521/0.2427 seconds baseline and 0.2663/0.2636 seconds plugin. This is a small diagnostic,
  not family evidence. `plugin.Open` rejects the sealed memfd pathname with `realpath failed`;
  regular-file success does not satisfy the immutable-code invariant. Do not expand this
  candidate to 200 plugins or claim it fixes shared-library debugging/integrity.
- **Exact archive prototype:** four 50-bundle XZ/LZMA2 archives (preset 6, 32 MiB dictionary)
  preserve every original executable and record byte, including full DWARF. The complete
  200-bundle family shrinks from 1,302,465,720 to **303,033,116 bytes** before shared support.
  Each archive was fully reconstructed and hash checked. Summed reconstruction/hash time is
  12.625 seconds; streaming replay overlaps decoding with native work rather than adding that
  sum to the execution wall time. These are separate measurements, not a whole-suite result.
- **Complete accounting:** archived Python/stdlib/native reconstruction dependencies contribute
  15,567,024 bytes (758 files, 61,186,044 raw bytes; exact reconstruction verified). Include the
  retained input archive, existing Go/C source-identity/reconstruction support archives and
  8,145,133 bytes of manifests/aliases/settings: **439,737,801 bytes total**. Common support is
  counted explicitly; the Python capture intentionally overincludes the standard library.
  No extra original debug executable is needed to reconstruct this candidate because the
  archives preserve it exactly. Existing raw baseline files remain physically retained.
- **Current proof:** a fresh complete family invocation regenerated 200 exports and all 2064
  independent source/fixture audits, byte-identical to the admitted exports, with zero native
  cache misses. Its end-to-end family time was 42.724 seconds (41.401 execution), not a new
  whole-suite timing. All five subsequent native replays executed every original package in
  a fresh process and fixture directory, with 9704 named tests including 7640 subtests.
  Source, settings, compiler-support identity, archive/manifest hashes and reconstructed
  executable hashes are checked before use; execution uses sealed memfds. No outcome cache.

| Matched native replay, four 50-bundle units, two units / four shape workers per unit | Wall seconds |
| --- | ---: |
| Baseline A (fresh-export checksum work overlapped; keep as diagnostic) | 10.5554 |
| Packed A | 12.7615 |
| Packed B | 12.3162 |
| Baseline B | 10.1058 |
| Additional uncontended baseline C | 10.3560 |

Inventories match exactly in all five runs. The uncontended controls show approximately
1.96–2.66 seconds of additional native replay time for the tested packing path. The previous
shared-link experiment used eight 25-bundle units, so its absolute timings must not be treated
as a controlled comparison with this four-unit schedule. No compilation or compression ran
concurrently with the matched native measurements; the first baseline's checksum overlap is
explicitly excluded from the clean controls.

Changed source and corrupted archives are rejected; wrong current value and ordered-trace
fixtures produce real native failures without recompilation. Existing source/settings/toolchain
validation and sealing helpers remain unchanged. The archive reconstruction hashes prove
preservation of the original native bytes and debug information. No language semantics,
independent oracle generation, expected vectors, default harness behavior or compiler resource
probe was changed. The accepted 634-test whole-suite/race/vet proof is not repeated or replaced.

All 42 new contained units have exited, their cgroups are removed, swap stayed zero and no
memory max/OOM event occurred. The task-owned disposable plugin compiler cache was removed;
its pre-removal byte count was not durably recorded. The first anchor audit found one newly
created 8966-byte Python bytecode file: the outer environment flag was not forwarded by systemd.
That exact file was removed, restoring the complete admission ignored inventory. Future contained
Python commands should pass `python3 -B` inside the unit when importing repository helpers.
The initial VCS-stamping build failure, relative runner-path error, sealed-plugin rejection and
corrected audit are retained as diagnostic history, not successful checks.

The main executable cache is unchanged: 2719 entries / 12,691,799,961 bytes. The general Go cache
contains 29,293,264,619 bytes, 175 bytes above admission after the family invocation; no pruning.
The new evidence root holds approximately 1.714 GB of coexisting exports, archives, binaries,
receipts and diagnostics after disposable-cache cleanup, separate from the prior experiment's
3.787 GB. HEAD, branch, protected stashes and the exact 108166-entry ignored inventory match
admission. This continuation changes only these task records and private temporary experiments;
all previously uncommitted exporter/helper changes are preserved. No commit, push, cache
migration, broader-family run, production language/backend change or host configuration change.

Reviewable evidence: `/tmp/pipelang-shared-runtime-followup/final-evidence.json`,
`retained-metadata.json`, `terminal-anchors.json`, `packed_replay.py`, `compare_packed.py`,
`packed_faults.py`, `packed-faults.json`, `family-current/`, `plugin-path-diagnostic.json`,
`plugin-replay-error.log`, and the exact compressed corpora. Compression is useful here, but
these tested representations fail the strict objective. Stronger native sharing with preserved
DWARF and immutable loading remains unproven; this result is not a lower-bound proof.
