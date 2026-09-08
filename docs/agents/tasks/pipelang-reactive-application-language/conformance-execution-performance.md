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


## Research/native-code continuation and requested handoff (2026-09-07)

The user explicitly reopened this objective: "keep pushing", research compression breakthroughs
and unconventional approaches, including whether a measured bottleneck could benefit from native
machine code in the Go-based workflow. The user expects autonomous useful work while away for a
couple of hours and explicitly requested `dorkpipe-task-handoff`. State: **executing**. Transport
one continuation to this saved checkout; do not create a worktree or automatically hand off again.

Public web research and bounded reversible profiling/implementation experiments are authorized.
The historical offline-only research restriction is superseded; native builds/tests retain the
pinned offline toolchain and established containment. Research primary papers, official codec/Go
sources and reproducible benchmarks; distinguish promising ideas from demonstrated improvements.
Go already produces native code, so investigate assembly/C/SIMD or a custom reconstruction path
only against a measured bottleneck, including call-boundary costs, portability and debug support.
This permission covers isolated experiments and appropriate harness/codec optimizations, not a
production language/backend redesign, loss of generated-Go coverage or weaker correctness proof.

Pending checkpoint: admit completed evidence, identify the remaining time/storage components,
research relevant compression/native-execution techniques, then select and measure the strongest
in-scope candidate on a small representative probe. Continue productive checkpoints automatically;
a research list alone is insufficient. Preserve the complete v0.91 family checkpoint before any
promotion or broader-family expansion. Keep the original whole-suite 30-second and 14,391,156-byte
assessment targets, independent current oracles, every vector/compiler probe, exact invalidation,
fresh processes/fixtures, state isolation, debugging, and complete support/storage accounting.
Do not treat a failed speculative probe as proof that the objective is impossible.

Live handoff admission is now clean commit `0f07ab8de053503613b846d3397c9868a6eba950` on
`js/pipelang`: the previous experiment edits have been committed since the last response.
Comparison with `family-current/source-hashes.json` finds no proof-relevant source drift, so admit
the recorded proof rather than replay it. Only the three task-record files changed to reopen this
objective are uncommitted handoff-owned changes. Both protected stashes and the exact ignored
inventory remain unchanged. Evidence and current anchors are local files under
`/tmp/pipelang-shared-runtime-followup`; the receiver must verify access, not assume ephemeral
browser or process state survived. No units remain active and disposable experiment caches are gone.
No commit, push, protected-cache cleanup, paid service, credential action, external publication,
installed-toolchain modification or persistent host change is authorized by this continuation.

### Receiver checkpoint: exact DWARF precompression

Receiver admission verifies HEAD/branch, both stashes, the ignored inventory and all 198
proof-source hashes. The accepted complete suite remains admitted. The existing warm CPU
profile uses AVX2 SHA-256 already; handwritten hashing assembly is not supported by that
evidence. Primary codec research and local Go linker inspection instead support exposing
zlib-compressed DWARF before outer compression, then reproducing the original compressed
sections with the pinned Go writer. All eight size-quantile samples round-trip exactly.
For 55,144,886 original bytes, raw XZ occupies 13,506,176 bytes and exposed XZ 5,543,928;
exposed Zstd level 12 occupies 8,785,152. Original-ELF reconstruction adds 0.530 seconds
across eight helper processes. This is a micro-probe, not a native replay or target result.
Evidence and primary-source links: `/tmp/pipelang-precompression-probe/research.md`,
`recode.go`, `probe.py`, `sample.json`, `build.json`, and `small.json`. Both units exited
with zero swap/OOM; small-probe aggregate peak was 711,487,488 bytes. State remains
executing: proceed to the complete 200-bundle checkpoint with exact reconstruction,
current oracles, matched replay and full support/storage accounting before reassessment.

### Complete precompression result: smaller representation, slower native replay

State: **failed_verification against the original targets**. The complete 200-bundle
prototype is correct and reviewable, but is not promoted. The whole-suite 30-second rerun
and 14,391,156-byte retained-storage goals remain unmet. This is a measured limitation of
these representations, not proof that a different approach cannot succeed.

The approach follows [reversible precompression](https://github.com/schnaader/precomp-cpp/blob/master/README.md):
expose nested compressed data before outer compression. The local pinned Go linker uses
`zlib.BestSpeed`; the small native Go helper reproduces every original compressed section
and executable hash. The original 1,302,426,120 executable bytes include 277,348,331 bytes
of compressed DWARF. Exposing those sections produces 1,875,560,785 bytes before packing.
Twenty ten-bundle shards were populated in 242.164 seconds, with serial helper/codec
commands inside each of at most two contained units. Every codec child stayed below its
30-second deadline. This population timing is not a rerun measurement.

| Complete family representation | XZ/LZMA2 | Zstandard level 12 |
| --- | ---: | ---: |
| Exact reconstructed-code archives | 123,964,208 B | 205,640,582 B |
| Common input/Go/C/Python support and inherited manifests | 136,704,685 B | 136,704,685 B |
| Additional helper, codec, source and validation support archive | 4,471,620 B | 4,471,620 B |
| Candidate identities and shard manifests | 209,173 B | 209,173 B |
| Complete representation | **265,349,686 B** | **347,026,060 B** |

The additional support archive contains 418 explicitly hashed paths, including the native
helper and its source, installed codec executables/dependencies, all current proof-source
files and source-validation records. The inherited support deliberately retains the earlier
Go/C/Python overcapture; this is conservative accounting, not a minimal-support lower bound.
Including every root JSON receipt and the fresh source-identity record raises the retained
totals to 270,574,786 B and 352,251,160 B, respectively, before the final storage receipt
itself. Coexisting raw/alternative experimental files are counted separately below.

Compared with the previous exact-byte packing, XZ archive bytes fall from 303,033,116 to
123,964,208 (59.1%); the complete representation falls from 439,737,801 to 265,349,686
(39.7%). The latter still exceeds the entire whole-suite storage target by 18.4 times.
These family measurements cannot be extrapolated to whole-suite success.

| Matched native replay, four 50-bundle units, two units/four shape workers | First | Second |
| --- | ---: | ---: |
| Original retained executables | 13.330 s | 12.724 s |
| Exposed DWARF + XZ + exact native reconstruction | 21.090 s | 21.186 s |
| Exposed DWARF + Zstandard + exact native reconstruction | 20.338 s | 20.252 s |

Order: baseline/XZ/Zstd/Zstd/XZ/baseline. No build, compression or unrelated checksum pass
overlapped these measurements. Both lanes perform identical current-source, archive,
helper, build-settings and toolchain identity checks. These checks differ from the previous
experiment, so only the new matched controls establish the slowdown. Reconstruction uses
a sealed helper, verifies the exact original executable hash and seals native code before
execution. Every original package runs in a fresh process and fixture directory; mutable
state is never carried between cases. All six inventories match: **2,064 packages, 9,704
named native tests including 7,640 subtests**, with the original DWARF bytes intact.

Exact reconstruction consumes 16.88–17.48 summed worker-seconds per candidate replay;
this overlaps decoding/native execution and must not be added to wall time. Actual native
process durations remain similar across controls/candidates. Faster Zstandard decoding
does not erase the reconstruction and data-movement cost. The earlier AVX2 hashing profile
and researched FastCDC/VectorCDC results do not justify claiming an assembly breakthrough
for this workload; source links and the candidate selection rationale are in `research.md`.

The fresh end-to-end family invocation passed in 45.479 seconds, with all 2,064 independently
regenerated source/fixture audit records identical to the admitted exports and zero native
cache misses. The accepted 634-test, 101-unit, 623-direct-compiler whole-suite proof remains
admitted at approximately 272 seconds. No original test/vector/resource probe was removed,
no pass outcome was reused, and no new whole-suite, race, vet or cold-host result is claimed.

Nine fault checks pass: current source, archive, reconstructed body, toolchain and settings
mutations reject; wrong current value and ordered trace cause actual native failures without
rebuild; sealed helper writes and truncation reject. All **59 contained units** completed,
their cgroups were removed, swap stayed zero, and no max/OOM/OOM-kill event increased.
Maximum aggregate peak was 736,182,272 bytes under the unchanged 1 GiB hard/800 MiB
proactive controls. Audit and codec checks remain Linux ELF64 little-endian evidence only.

Terminal cleanup removed exactly 3,930,794,723 bytes from the new disposable helper cache,
expanded packing staging and redundant sample streams after the audit passed. The cleanup
receipt names all 23 targets. The new evidence root retains 1,852,543,335 file bytes before
its final storage receipt; previous experiment roots remain 3,787,205,641 and 1,713,830,465
bytes. The general Go cache remains 29,293,264,619 bytes and the protected native cache
remains 2,719 entries / 12,691,799,961 bytes. No shared/protected cache was pruned.

Reviewable prototype and receipts: `/tmp/pipelang-precompression-probe/{recode.go,replay.py,
populate.py,compare.py,faults.py,research.md,final-evidence.json,identity.json,storage-final.json,
cleanup.json}`, plus both complete packed corpora and `family-current/`. Repository changes
are confined to the three task records; production language, backend, harness defaults and
package/engine boundaries are unchanged. HEAD, branch, both protected stashes, all 198 proof
source hashes and the 108,166-entry ignored inventory match admission. No commit, push,
promotion, broader-family run, worktree or persistent host/toolchain change occurred.

## Approved bounded whole-suite pass and requested handoff (2026-09-07)

After the precompression result, the user accepted the recommendation to prioritize full-suite
execution costs, agreed that a bounded foundation pass should precede more language slices,
then explicitly approved proceeding and invoked `dorkpipe-task-handoff`. The same objective
is **executing** with this user-approved next phase. The prior compression candidate's failed
target verification remains historical evidence; no compression format is promoted.

Authorized objective: build a complete time breakdown of the approximately 272-second suite
(direct compiler probes, current source/Core/oracle work, binary/toolchain validation, native
execution, process/containment startup and scheduling). Admit existing proof and profiles;
collect only the missing measurements needed to distinguish computation from waiting and
identify the largest avoidable cost. Implement the strongest bounded improvement if supported
by evidence, then verify the complete suite once after the final material change. Preserve all
tests/vectors, current independent values and ordered traces, generated-Go execution, exact
invalidation, fresh processes/fixtures, mutable-state isolation, debugging and compiler-resource
measurements. Keep aggregate-memory and complete retained-storage accounting in the comparison.

Done when the breakdown is recorded and either one supported optimization has complete
correctness/resource and matched full-suite performance proof, or measurements establish why
the examined dominant costs offer no justified bounded change. Report that result and return
to language-slice planning under existing selection/approval gates. Do not turn this into
indefinite codec research, start a new language contract, or make 30 seconds / 1/1000 storage a
prerequisite for resuming language progress. Those original targets remain visible and unproven;
the user explicitly accepted a bounded useful pass rather than waiting for both to be reached.

First receiver checkpoint: analyze durable suite receipts/profiles, define and collect the
missing component timings, then select the strongest evidenced optimization. No implementation
seam is preselected. A new instrumented full run is justified only by a concrete measurement
gap; do not replay unchanged correctness proof for admission. Keep successful output quiet and
failures bounded. After focused proof, run the final complete suite with all original probes;
if no code changes are justified, preserve the admitted correctness proof and document the
measured limitation without a ceremonial repeat.

Transport exactly one fresh task into `/home/jamie/source/dockpipe`, with no worktree. Admission
still matches HEAD `0f07ab8de053503613b846d3397c9868a6eba950`, branch `js/pipelang`, both
protected stashes, all 198 proof-source hashes and the 108,166-entry ignored inventory. The
three task-record files are the only unstaged handoff-owned changes; nothing is staged or
untracked. All 59 new contained units are removed. Existing evidence remains accessible under
`/tmp/pipelang-{execution-performance,bundle-promotion,shared-runtime-experiment,shared-runtime-followup,
precompression-probe}`. The receiver must reclaim these files directly, not assume live process
or browser state survives. Public primary-source research remains authorized; native validation
stays pinned, offline and contained. No commit, push, worktree/stash mutation, broad/protected
cache cleanup, paid service, credential/publication action, installed-toolchain patch or
persistent host change is authorized. Another handoff requires another user request.

### Whole-suite receiver: measurement gap and selected candidate

Admission matches the handoff's HEAD, branch, stashes, 108,166 ignored entries and all
198 current proof-source hashes. Existing full-suite correctness remains admitted. Its
binary predates three later export/logging harness edits, so a current-source timed
baseline is required for a matched implementation comparison, not admission replay.

Existing logs contain 623 direct compiler measurements totaling 12.992 summed seconds,
49.596 seconds of direct generated build/run calls and 72.450 seconds of grouped native
calls (overlapping worker time, not additive suite wall time). The opt-in performance
benchmark itself occupies 22.83 seconds. Targeted CPU profiles of units 97, 10 and 70
separate v0.83 source/HIR generation, compiler-memory source work and prepared v0.91
oracle evaluation. In unit 10, `inferExprType` accounts for 7.91/19.44 sampled CPU
seconds (40.7%), with scope-map construction and GC prominent. Source inspection finds
one growing environment copy per immutable local, repeated during HIR tail inference.

Selected bounded candidate: clone the input type environment once per consecutive
immutable-local sequence, then extend only that private copy. Keep initializer order,
shadow/type diagnostics, recursive branch isolation and every compiler probe. No
cross-call memoization, result cache, language contract or backend redesign is proposed.
First collect a current-source full baseline with per-unit CPU profiles, cgroup CPU,
subprocess and outer-launch timing. This fills the whole-suite computation/waiting gap;
final verification will use the identical instrumentation and limits. Evidence lives
under `/tmp/pipelang-wholesuite-pass`; initial profiles used the admitted test binary
and all six profile/analysis units exited with unchanged limits, zero swap and removed
cgroups. CPU samples and summed worker intervals must not be presented as wall time.

The fresh instrumented baseline passes 634 tests in 101 execution units plus build/list:
307.972 seconds overall, 306.169 execution, zero cache misses and unchanged source.
Disjoint harness CPU samples total 812.36 seconds: source/HIR/Core 251.02, background
GC 138.99, evaluator 106.76, executable validation 100.03, toolchain validation 51.90,
Go generation 39.04, native harness/fixtures 27.01, separate Core validation 9.28,
other runtime/harness 88.33. Classification uses profile stack membership and reports
one category per sample; cumulative function timings overlap these categories.
`inferExprType` totals 111.78 cumulative sampled seconds across the complete suite.
The cgroups used 1,068.20 CPU seconds; 1,021.98 child CPU seconds include external
compiler/native/manager children absent from the Go harness samples. Sampling and
launcher processes account for part of the difference, not merely idle waiting.

Logged intervals cover 7,743 fresh native calls (82.046 summed seconds), 263 direct
build/run calls (59.470), and 623 logged direct compiler probes (14.006). Later
memory fixtures also retain their independent compiler checks; these logged durations
are not a claim that every external child has a separate timing line. Total outer
unit intervals are 611.426 seconds, with 596.697 seconds inside measured workloads
and 14.729 seconds in outer startup/report/cleanup. Two workers overlap these totals.
The 0.912-second difference between twice execution wall and summed outer intervals
bounds the lane imbalance/gaps for this run. CPU profiles add shutdown/measurement
cost, so compare only with equally instrumented final proof, not the old 272 seconds.

The candidate changes only generic inference of consecutive immutable locals, plus
an isolation test. Focused source-rejection/dependency tests, the new success/error/
shadow/nil/shared-map checks under race detection, Core/evaluator/Go-backend checks,
Application IR and frozen compatibility all pass. The HIR package has no standalone
tests. Source, expected values, generated bytes, argument checks and direct compiler
commands remain unchanged. Terminal complete-suite verification is next after vet.


### Bounded whole-suite pass complete: private inference scopes

State: **completed** for the user-approved bounded performance phase. The historical
compression candidate remains unpromoted and failed against the original targets.
No next language contract is selected or implemented. Resume language-slice planning
with separate founder selection/implementation approval; depth-three terminal-tree
initializers are a natural candidate because v0.96 still limits that placement to
depth two. This observation is not a new scope decision.

`typecheck.go` now copies the caller's environment once for each consecutive immutable
local sequence. Each initializer still sees exactly its preceding bindings; type and
shadow checks execute in their original order. Recursive branches get private scopes,
and neither success nor failure changes the caller map. `typecheck_scope_test.go`
checks sibling names, an error after validated locals, shadowing, nil input scopes and
eight concurrent callers sharing one read-only environment. No persistent cache,
invalidation rule, source contract, generated code or native execution path changes.
Generic compiler/package boundaries are preserved.

| Matched warm full-suite measurement | Before | After |
| --- | ---: | ---: |
| Overall wall time, CPU profiling enabled | 307.972 s | **273.530 s** |
| Execution wall time | 306.169 s | 271.918 s |
| Summed cgroup CPU | 1,068.197 s | 948.638 s |
| Sampled harness CPU | 812.360 s | 685.120 s |
| Cumulative type-inference CPU samples | 111.780 s | 25.850 s |
| Source/HIR/Core CPU category | 251.020 s | 171.680 s |
| Background GC CPU category | 138.990 s | 84.600 s |
| Summed memory-test unit wall intervals | 184.853 s | 96.718 s |
| Maximum per-unit aggregate memory peak | 734,728,192 B | 734,617,600 B |
| Conservative simultaneous two-unit peak upper bound | 1,469,255,680 B | 1,469,140,992 B |
| Original / total discovered tests | 634 / 634 | 634 / 635 |
| Execution units, plus build/list | 101 + 2 | 102 + 2 |

The observed matched wall-time reduction is **34.441 seconds (11.2%)**. Type-inference
CPU falls 76.9%; the v0.87 memory unit falls from 15.000 to 6.868 seconds and its
aggregate peak from 425,025,536 to 398,766,080 bytes. The new test creates one extra
planner unit. This is one before/after pair on a shared host with the same pinned
Go, caches, profiling, audit flags, two units/four shape workers and all original
probes. It is not a cold-host benchmark or a promised universal speedup. Do not
compare the profiled 273.530 seconds directly with the historical unprofiled 272.063.

The remaining final CPU categories are evaluator 112.31, executable validation 103.31,
toolchain validation 53.54, Go generation 41.00, native harness/fixtures 28.39,
separate Core validation 9.88 and other runtime/harness 80.41 sampled seconds.
Summed fresh native intervals rise from 82.046 to 90.859 seconds and direct build/run
intervals from 59.470 to 65.303; logged compiler intervals remain 14.006/13.943.
These overlap workers and include child startup/containment checks. The remaining
external compiler/native CPU is included in cgroup accounting; it is not silently
assigned to the harness CPU categories. No evidence supports removing current oracle
work, toolchain/binary validation or fresh processes. Further optimization is deferred
under the user's bounded-pass decision, rather than claiming these costs irreducible.

Terminal verification ran once after the last material source change. It passes all
635 tests, with all 634 inherited named outcomes/statuses, vector inventory, generated
source/fixture audit hashes and native artifact keys identical. All 623 logged direct
compiler measurements remain, as do the later memory fixtures' original probes; all
64 exported fixture sets retain identical nonmeasurement bytes. Both complete runs
have 3,105 native cache hits, zero misses and exactly the same 2,719 live entries.
Fresh processes, fixture directories, independent expected values and ordered traces,
per-case state isolation, exact invalidation and DWARF debugging remain unchanged.
Focused checks, race detection, Core/evaluator/backend, Application IR, frozen
compatibility and compiler vet pass. No broad repository CI/build, sustained fuzzing,
new non-Linux, interactive editor, deployment or production operations were run.

Every new recorded unit (targeted profiles, profile summaries, focused/race/downstream/
vet and both full runs) completed and removed its cgroup. Hard memory stays 1 GiB,
swap zero, task count 128, proactive stop 800 MiB, grouped/build high 700 MiB, and
GOMAXPROCS=4. No max/OOM/OOM-kill/swap events increased. Two-unit figures above sum
the two largest individual peaks: they are conservative bounds, not sampled simultaneous
peaks. Native 25-second and measurement 30-second deadlines and bootstrap allowances
remain intact. No compiler or profiling workload ran uncontained.

Storage is measured over nonoverlapping retained roots, including all older execution,
bundle, sharing and compression evidence, the general Go cache, new proof files and
the complete pinned Go toolchain. Logical retained file bytes rise from
50,225,466,806 to **50,390,680,020** before the final receipt/documentation writes.
The native cache remains exactly **12,691,799,961 bytes**. Go build/race cache storage
rises 136,240,376 bytes to 29,503,182,882; new experiment evidence rises 28,972,838
bytes to 59,185,537. Earlier experiment roots are unchanged. These are retained logical
bytes, not deduplicated physical-disk blocks or a minimal deployable representation.
No storage was moved elsewhere or pruned; the 30-second and 14,391,156-byte goals
remain unproven, as accepted for resuming language work.

Receipts and reproduction: `/tmp/pipelang-wholesuite-pass/{full_profile.py,measure.py,
analyze.py,compare.py,before-analysis.json,after-analysis.json,comparison.json,
storage-before.json,storage-after.json}`, plus per-unit logs/CPU profiles and
focused/race/downstream/vet receipts. `full_profile.py <fresh-output-name>` runs the
canonical launcher with identical instrumentation; every retained profile is freshly
collected and no pass outcome is reused. The profile reader partitions stack samples
into disjoint categories; cumulative function timings are explicitly separate.

Final anchors remain HEAD `0f07ab8de053503613b846d3397c9868a6eba950`, branch
`js/pipelang`, both protected stashes and the exact 108,166-entry ignored inventory.
Only `typecheck.go` differs among the 198 admitted proof-source files; the new test
and three task records are the remaining owned changes. Nothing is staged. No commit,
push, worktree/stash mutation, cache cleanup, installed-toolchain or persistent host
change occurred. User-requested handoff transport was used to receive this objective;
no further task was created.


### Latest user direction: keep iterating (2026-09-07)

After the inference pass was fully recorded and its 217 units audited, Jamie's side
conversation relayed “just keep iterating.” The same objective is **executing** again
under that explicit direction: continue measured, bounded deterministic improvements
with correctness, complete-suite timing, aggregate memory and complete retained-storage
proof. Existing exclusions and all language/native-test invariants remain. No particular
compression/ML idea is selected. The completed inference result above remains admitted.

Next measured candidate: final profiles assign 103.31 CPU seconds to executable
validation. Ordinary four-case batches currently hash the same cached binary on lookup
and again before each of their fresh children; the existing v0.91 shared-oracle lane
instead executes an immutable, hash-verified kernel-sealed snapshot. Evaluate extending
that existing mechanism only to Linux retained batches with more than one case. Preserve
all source/toolchain/settings identities, lookup corruption repair, fresh processes and
fixture directories; measure the added transient snapshot memory. The largest admitted
native executable is 12,643,426 bytes, below the existing 64 MiB sealing bound. Use the
completed 635-test profiled run as the matched baseline. Do not repeat it for admission.

The ordinary-batch candidate passes focused checks: actual multi-case native children
observe the sealed memfd executable, repeated current fixtures execute afresh, wrong
oracles fail, changed generated source misses the cache, and corruption is quarantined
and rebuilt. Existing source/settings-key checks, special-harness rejection and sealed
snapshot write/grow/shrink/path-replacement/wrong-digest faults pass. No retained native
cache format or generated program changes. The compiler's completed inference/downstream/
race proof remains admitted; the new change is test harness execution only. Next run
complete verification with the previous 273.530-second profiled run as the matched
baseline, then audit every original outcome, fixture/key and memory/storage receipt.

### Ordinary-batch sealing checkpoint verified

The complete 635-test / 102-unit run passes at **257.545 seconds**, versus 273.530
for the admitted inference baseline: 15.986 seconds (5.8%) faster in this matched pair.
Sampled executable-validation CPU falls from 103.31 to 68.96 seconds; total harness
CPU from 685.12 to 637.69 and cgroup CPU from 948.638 to 890.330. Maximum per-unit
aggregate peak is 632,209,408 bytes, with conservative two-unit upper bound
1,219,260,416 bytes; no increased memory/swap/OOM event or remaining cgroup occurs.
Do not attribute every shared-host memory/timing difference to this change.

All 635 named outcomes, vector/source/fixture identities, 623 logged direct compiler
probes, 64 exported fixture sets and all native keys remain identical. Both runs have
7,743 fresh batched native calls, 263 direct build/run calls, 3,105 retained-cache hits
and zero misses; no native cache bytes were added. Source/settings/corruption/current-
oracle fault checks and vet pass. The final full run follows the last material change.
The cumulative audit now verifies 323 contained units across both checkpoints.

Nonoverlapping retained storage is 50,514,079,014 logical bytes before subsequent
receipts. Native bytes remain 12,691,799,961; Go build/race cache is 29,597,702,400;
new proof root is 88,065,013. All earlier experiment roots stay unchanged. The
123,398,994-byte increase over the preceding storage snapshot is recorded support/build
and proof data, not moved or hidden native storage. No cleanup occurred. Evidence:
`sealing-analysis.json`, `comparison-sealing.json`, `storage-sealing.json`,
`final-evidence-sealing.json` and `sealing/` under `/tmp/pipelang-wholesuite-pass`.

Under “keep iterating,” the next bounded candidate is exact semantic-key formatting.
The latest complete profile still attributes 77.97 sampled CPU seconds cumulatively
to `semanticIdentityKey`, including 63.21 in `writeCanonicalKeyField`. Replace general
`fmt` formatting of nonnegative lengths with decimal integer conversion and direct
builder writes. Preserve the historical byte-length framing and every key byte;
verify recursive type/callable encodings against the historical reference before a
matched full run. No identity schema, lookup policy, memoization or language change
is selected. The completed sealing proof is admitted before changing this seam.

Exact-key candidate focused proof passes the historical formatter oracle across nested
identities, zero/one and 9/10/99/100/128 parameter counts, byte-length boundaries,
Unicode, NUL and invalid UTF-8. Semantic identity/dependency checks, Core/evaluator/
backend, Application IR, frozen compatibility and vet pass. Only integer formatting
inside the existing encoding changes; the key schema and all bytes remain identical.
Proceed to one complete profiled verification after these final source changes,
matched against the admitted 257.545-second sealing run.

### Exact-key first result: CPU gain, wall-time result unresolved

The candidate passes 636 tests in 102 units and every inherited outcome, native key,
fixture and compiler-probe audit. Key-construction CPU falls 77.97 → 26.28 sampled
seconds; source/HIR/Core CPU 167.58 → 121.43 and total harness CPU 637.69 → 607.73.
However complete wall time is 260.512 versus 257.545 seconds, so no full-suite speedup
is claimed. Cgroup CPU is 878.513 seconds; native/direct build-run intervals also rise.
The new encoding reference test takes only 0.13 seconds. Maximum per-unit aggregate
peak is 734,691,328 bytes, still within the unchanged controls, and all 430 cumulative
units are removed with no adverse memory/swap/OOM events. Retained logical storage is
50,655,912,135 bytes; native cache unchanged, Go cache 29,710,698,291 and proof root
116,902,243. Receipts are `keys-analysis.json`, `comparison-keys.json`,
`storage-keys.json` and `final-evidence-keys.json`.

One confirmation pair is justified by the unresolved end-to-end performance result,
not by a need to replay passed correctness. Both runs retain the same current 636-test
inventory. The control builds with Go's explicit source overlay replacing only
`semantic_id.go` with bytes whose SHA256 matches the admitted sealing source hash;
the candidate uses current source. Every other test/helper, generated oracle, source
contract, cache, profile flag and limit is identical. The read-only control replacement
and overlay are retained under `/tmp/pipelang-wholesuite-pass/semantic_id.control.go`,
`keys-control-overlay.json` and `keys-control-identity.json`; the checkout is unchanged.
This isolates the key formatter without weakening source provenance or inventing a
wall-time claim from CPU samples. Record the pair, then decide from its evidence.

### Exact-key confirmation accepted

Both confirmation runs pass the identical **636 tests / 102 execution units** and
all outcome/vector/source/fixture/native-key/probe comparisons. The historical-format
control takes 251.810 seconds overall and 246.631 in suite execution; current code
234.384 overall and **233.035 execution**. The control's one-time overlay build/list
cost is 5.179 seconds versus 1.349 for the warm candidate, so the defensible matched
execution improvement is **13.595 seconds (5.5%)**, not all 17.426 overall seconds.

Key-construction CPU again falls 76.09 → 25.06 sampled seconds, total harness CPU
618.68 → 567.88 and cgroup CPU 863.377 → 810.051. Source/HIR/Core CPU falls
163.41 → 112.77; executable validation stays 67.50/67.49 and evaluator 106.90/108.09.
This confirms a repeatable formatter reduction without attributing unrelated child
variation to it. The earlier 260.512-second result remains recorded; that run had
304 permitted memory-high events in one v0.96 layout unit, unlike the original
sealing baseline. No hard-limit/OOM/swap failures occurred in either pair.

Control/candidate maximum aggregate peaks are 734,593,024 / 734,756,864 bytes and
conservative two-unit bounds 1,469,100,032 / 1,469,251,584. All 638 cumulative units
completed and removed their cgroups. Native retained bytes remain 12,691,799,961.
Complete retained logical storage is 50,880,783,353 bytes before later receipts,
including Go cache 29,878,018,650 and new evidence 174,453,102; earlier roots remain
unchanged. The confirmation pair's extra build artifacts and profiles are included.
The exact formatter change is retained; no identity/key bytes or language contracts
changed. Proof: `keys_control/`, `keys_confirm/`, their analysis receipts,
`comparison-keys_confirm.json`, `final-evidence-keys_confirm.json` and
`storage-keys_confirm.json` under `/tmp/pipelang-wholesuite-pass`.

### Next measured checkpoint: validate while creating the sealed snapshot

The confirmed profile still spends 67.49 sampled CPU seconds in executable validation.
Source inspection finds that sealed retained batches first hash the cache file on
lookup, then read/hash the same file again while constructing the immutable snapshot.
The next bounded candidate combines those checks: validate cache identity/metadata,
then hash the snapshot bytes once and execute only that sealed descriptor. A digest
mismatch must still quarantine/rebuild the corrupt entry; snapshot/containment resource
failures must fail closed without mislabeling a sound cache entry as corrupt. Keep
single-case/non-Linux path validation unchanged. No source/toolchain/settings check,
pass outcome, fresh child/fixture, mutable-state isolation or debugging byte may be
removed. Admit the completed key proof before implementation and measure the result
against `keys_confirm/` with the same 636 tests and profiling settings.

The combined snapshot candidate passes current-oracle/source/settings/corruption checks
for both ordinary batches and shared-oracle bundles, existing sealing faults and vet.
An added assertion in the existing snapshot test proves that exceeding the snapshot
size budget returns an error rather than a repairable miss and leaves its cache entry
intact. Research-export invocations retain the admitted lookup/path-export/seal order;
normal runs use the combined read. No test inventory or native key changes are expected.
Proceed to complete matched verification against `keys_confirm/` after these final edits.

### Combined snapshot-read candidate not retained

Correctness/resource verification passes all 636 tests and all inherited inventories,
with 745 cumulative units removed and no hard-limit/OOM/swap failures. However full
wall time is **240.482 seconds versus 234.384**, execution 239.138 versus 233.035.
Executable-validation CPU falls 67.49 → 54.34 sampled seconds, but total harness CPU
567.88 → 562.80 and cgroup CPU **810.051 → 810.893** are effectively unchanged.
Peak per-unit aggregate memory is 606,048,256 bytes. This pair does not establish an
end-to-end benefit sufficient to retain additional cache failure-classification code.
The candidate is deferred, not claimed impossible, and no repeated full timing is
required merely to chase a favorable result.

The five candidate-owned postimages were retained in
`/tmp/pipelang-wholesuite-pass/rejected-snapshot-source/`, then restored byte-for-byte
to `keys_confirm/source-hashes.json`. All current source hashes again match that
verified baseline, including the ordinary-batch sealing and exact-key improvements.
The added snapshot-budget assertion was prototype proof, not an original conformance
vector; it remains in the preserved candidate source/receipt. No user-owned bytes were
reverted. `snapshot-reversion.json` records exact restoration, so admit the completed
636-test key-confirmation proof without a ceremonial rerun.

Retained logical storage for this experiment is 51,022,130,852 bytes before reversion
receipts and preserved source copies: Go cache 29,990,625,310, new proof root
203,193,941, native cache unchanged at 12,691,799,961. No caches were pruned or moved.
`comparison-snapshot.json`, `snapshot-analysis.json`, `final-evidence-snapshot.json`
and `storage-snapshot.json` preserve the successful correctness and unsuccessful
performance evidence. The objective remains executing under the user's iteration
direction; the current accepted source is the confirmed key-formatting baseline.

### Next bounded probe: read-only evaluator expression traversal

The accepted key-confirmation profiles attribute 67.15 sampled CPU seconds to recursive
`evalExprWithProgram`; 26.94 are `duffcopy`/`memmove` samples under it. Line-level
accounting (`evaluator-lines.json`) shows that these include both expression argument
copies and Outcome/Value copies. Do not claim all copying is removable: carrier/value
ownership and independent results must remain unchanged.

Probe only passing already validated, read-only expression nodes by reference inside
the private recursive evaluator. Retain the existing entry wrapper, public APIs,
argument/carrier validation, frame ownership, returned-value cloning and prepared
snapshot isolation. First collect a focused accepted-baseline phase benchmark, then
compare the candidate before any complete run. If focused measurements do not justify
this bounded change, restore the accepted source and defer it. This is not evaluator
bytecode compilation, memoization or a language/backend redesign.

The focused accepted/candidate phase comparison is positive for prepared evaluation:
small 16,383 → 15,277 ns/op (6.7%), large 24,489 → 22,625 ns/op (7.6%). Allocation
bytes/counts remain 14,744/43 and 19,656/61. Ordinary evaluation, which revalidates the
whole Core graph, is 349,707/361,311 and 485,736/494,332 ns/op; no ordinary-call gain
is claimed. All evaluator tests under race detection and prepared ownership/metadata/
exact-identity checks pass. The change is one private read-only recursive helper with
the old value-entry wrapper retained; it does not eliminate Outcome/Value cloning.
`eval-phase-comparison.json` records this small-probe evidence. It justifies a complete
run to test suite-scale value, not a prediction of the final speedup.

### Evaluator traversal candidate not retained

The candidate passes all 636 tests and every source/fixture/key/probe inventory, but
full wall time is **251.453 seconds versus 234.384** for the accepted baseline.
Execution is 249.463 versus 233.035; total sampled harness CPU 584.44 versus 567.88,
cgroup CPU 841.799 versus 810.051. Evaluator-category CPU decreases only 108.09 →
103.71, insufficient to improve the complete result. Maximum per-unit aggregate peak
is 734,515,200 bytes; conservative two-unit bound 1,468,968,960. Focused race,
ownership, downstream and vet checks all pass, but the private-recursion refactor is
not retained on a microbenchmark result alone. No further full rerun is justified
merely to seek a favorable timing.

The candidate is preserved in `rejected-evaluator-source/evaluate.go`, and production
`coreeval/evaluate.go` is restored exactly to the accepted `keys_confirm/` source hash.
All execution-source hashes match that completed 636-test proof. The only difference
in its source-snapshot set is a README prose clarification that shared-oracle bundles
also use sealing; the historical prose preimage hash and every planner split declaration
are verified unchanged. `evaluator-reversion.json` records restoration and this docs-only
exception. The accepted evaluator implementation and its ownership semantics remain intact.

`evaluator-analysis.json`, `comparison-evaluator.json`, `final-evidence-evaluator.json`
and `storage-evaluator.json` preserve the experiment. All **854** temporary units in
this task have completed and their cgroups are removed. Hard memory, task, swap,
proactive, compiler/native timeouts and concurrency limits remained unchanged; no
max/OOM/OOM-kill or swap event increased. Temporary memory-high reclaim is separately
recorded and is not mislabeled as an OOM or hard-limit failure.

### Iteration batch completed; resume language planning

Under the user's “keep iterating” direction, five bounded implementations were measured.
Three are retained: one private inference scope per consecutive local sequence,
sealed execution for ordinary retained Linux batches, and exact decimal semantic-key
formatting. Two are rejected at suite scale: combined snapshot lookup and read-only
recursive expression references. Their source, successful correctness proof, timing
limitations and storage remain reviewable; neither is silently promoted or erased.

The retained complete profiled run is **234.384 seconds**, versus 307.972 for the
current-source starting control (23.9% observed reduction with the same profiling,
cache and resource settings, while adding two tests). The extra same-inventory key
confirmation proves 5.5% execution improvement for that step; the control's additional
build cost is separately disclosed above. These are observed warm shared-host runs,
not a universal/cold-host claim or a comparison against the historical unprofiled
272.063-second run. All original 634 tests plus two regressions pass, retaining every
native process, current oracle/ordered trace, compiler probe and artifact identity.

The remaining measured costs include current evaluator work (108.09 sampled CPU
seconds in the accepted run), source/HIR/Core (112.77), exact toolchain validation
(49.20), executable validation (67.49), GC (78.88) and other harness/runtime work.
Repeated validation removal, result reuse, shared mutable execution or broad runtime/
backend restructuring is not authorized by these results. The two subsequent probes
show why small local savings cannot be assumed to improve the complete suite. Defer
those unproven paths and larger allocation/runtime work rather than keep making
changes without demonstrated suite-scale value. This concludes the current bounded
iteration batch; it does not assert a lower bound or that further progress is impossible.

Complete retained logical storage after the final experiment is 51,159,651,908 bytes
before its final restoration/closure receipts: Go build/race cache 30,099,243,733,
new proof root 232,096,574, unchanged native cache **12,691,799,961**. All older
experiment roots and the pinned toolchain are included and unchanged. Relative to the
first current-source baseline inventory, retained bytes increased 934,185,102; this is
reported build/proof storage, not hidden support or a storage optimization. No cache
cleanup or relocation occurred. The original **30-second / 14,391,156-byte** goals
remain unmet; the user accepted that they need not block language progress.

State: **completed** for this iteration batch. The next language contract remains
unselected and requires the existing founder selection/implementation approval. A
natural planning candidate remains depth-three terminal-tree initializers after v0.96;
no successor was implemented or created. Final owned code areas are `typecheck.go`,
`semantic_id.go`, ordinary batch execution and its cache-oracle regression test, plus
the two new scope/key regression files. README and the three task records document
behavior/proof. Generic engine/package boundaries are intact. All edits remain
unstaged and uncommitted in the saved checkout; no worktree, stash mutation, commit,
push, publication, installed-toolchain patch or persistent host change occurred.

Final closure audit: `closure-evidence.json` verifies all 854 removed units, exact
accepted execution-source hashes, the prose-only README exception, unchanged HEAD,
protected stashes and ignored inventory, empty staging, valid task YAML and clean
`git diff --check`. `storage-closure.json` accounts for **51,159,706,591 logical bytes**
across all nonoverlapping retained roots, including 232,151,257 in the new proof root
and unchanged native/cache/toolchain roots. Increase from the initial inventory is
934,239,785 bytes. The snapshot excludes its own subsequently written receipt.

### Reopened: resemblance search plus exact reconstruction

The user explicitly requested research into other search algorithms and an integrated
experiment. State: **executing**. The bounded outcome is a measured comparison of
similarity-selected references, exact deltas and stronger compression on the existing
200-bundle family, including reference-selection controls, exact reconstruction,
native replay if feasible, memory and complete storage accounting. Evidence and
primary-source research: `/tmp/pipelang-similarity-probe/research.md`. Preserve all
existing invariants and prior full-suite proof; no production promotion, new language
contract, commit, push, installation or cache cleanup is implied. The new clean HEAD
is recorded in `admission.json`; accepted execution-source hashes still match.

### Similarity-search experiment completed

The integrated research and experiment are complete; see
[the canonical similarity-search report](conformance-similarity-search-experiment.md).
Four resemblance strategies, exhaustive sampled reference controls, compression
levels 3/12/19, a resident codec and reference-local scheduling were investigated.
The 200-bundle delta representation is 326,456,238 bytes including references, versus
573,756,821 independently compressed at the same level. Complete dependency-inclusive
candidate:471,867,963 bytes. Final mirrored replay is 14.072/13.216s versus
13.416/12.915s raw controls: an observed 0.30–0.66s cost, not zero-cost reconstruction.
All 14 replay inventories retain 2064 native cases/9704 named tests, exact original
ELF/DWARF bytes, fresh current independent expected values/traces and process isolation.
The 130 contained units, negative checks, all alternatives and complete storage are
recorded. No production implementation changed or candidate was promoted. State:
**completed research experiment**; no successor language contract selected.

### Further reconstruction research authorized

The user explicitly requested further research and boundary-pushing experiments.
State: **executing**. Preserve the complete prior similarity experiment and native
proof. Investigate structure-aware residuals, bounded reference hierarchies and
decode directly into the verified executable buffer. Compare actual sizes, exact
reconstruction, fresh native execution, wall/CPU time, memory and full support
storage before judging candidates. Evidence: `/tmp/pipelang-reconstruction-frontier/`.
No production promotion, new language contract, installation or cache cleanup.

### Further reconstruction batch completed

[The second research round](conformance-similarity-search-experiment.md) measured
structure-aligned residuals, every root in a bounded two-level reference hierarchy,
and direct decoding into verified/sealed executable storage. Structural residuals
lost on all 12 samples and were not expanded. The hierarchy saves another 18,981,428
encoded bytes: **307,474,810 bytes**, or **454,822,892** with complete support/manifests.
Four raw/hierarchy runs each have nearly identical mean wall time (14.254/14.251s),
but hierarchy uses about 1.6% more cgroup CPU. No zero-overhead or whole-suite
speedup claim is supported. All 12 native replays retain 2064 cases/9704 named tests,
exact ELF/DWARF bytes, fresh current oracles/processes and unchanged limits. The
106 new units are removed; all prototype alternatives and storage are accounted.
State: **completed research experiment, unpromoted**. Production source and the
accepted whole-suite implementation are unchanged; no next language slice selected.

### Further representation research admitted (2026-09-07)

The user requests continued outside-the-box research. The existing TASK-021
conformance shared-runtime experiment admits a bounded 12-object sample of
DEFLATE decision transcripts, preserving original ELF bytes without repeating
LZ match search. Research, implementation, exact roundtrips, matched component
measurements, negative cases and inclusive retained storage close this round.
The sample is not whole-family/native performance proof. Existing production,
source invalidation, native oracle, containment and exclusion rules remain.
Evidence root: `/tmp/pipelang-representation-lab/`.

### Representation research closed (2026-09-07)

The bounded decision-transcript round completed with exact reconstruction of 12
fixed samples and all 16 references. Sample-plus-reference code falls from
45,365,275 to 27,367,865 bytes; the conservatively charged sample representation
is 179,374,596 bytes. Component reconstruction is faster than the old exact
recompression path but remains slower than ordinary delta decoding. These are
sample component results, not fresh native-family execution or full-corpus proof.
See `conformance-similarity-search-experiment.md` for source citations, matched
measurements, support/storage accounting, 42 contained units (one corrected and
retained audit failure), unchanged-source/protected-state checks and remaining
integration work. Evidence: `/tmp/pipelang-representation-lab/final-evidence.json`.
No production changes or promotion; prior full-suite/native proof remains admitted.

### Next reconstruction round approved for continuation (2026-09-07)

The user accepted the proposed next round with “lessa go” and explicitly requested
`dorkpipe-task-handoff`. Continue `TASK-021-conformance-shared-runtime-experiment`
in the saved checkout; state: `ready_for_execution`. This is execution authority
for the discussed bounded research round, separately from handoff transport.

Authorized objective: profile decision-transcript reconstruction to distinguish
symbol work, copying, I/O and startup; use the evidence to test resident decoding
and bounded reference preparation, then measure the complete affected native
workload with exact original bytes, CPU, aggregate memory and inclusive storage.
Research relevant primary sources without assuming their gains transfer here.
First checkpoint: reproducible contained profiling of the current sample and
existing decoder, before choosing an optimization. Subsequent in-scope checkpoints
are automatic; reassess before a materially different seam or broad suite.

Done when the strongest justified candidate has matched affected-workload timing,
CPU/memory, complete dependency/storage accounting and exactness/current native
oracle proof, or measured rejection establishes why the candidate should not be
expanded. Report negative evidence honestly; no improvement or zero-overhead
claim is required for completion. Preserve all existing vectors, ordered traces,
source/toolchain/settings invalidation, fresh per-case process/fixture isolation,
DWARF and independent compiler resource probes. All prior containment, cleanup,
Git, publication, language-contract and machine-state exclusions remain.

The handoff transports exactly one task in this saved checkout, with no worktree.
Live anchors and protected-state digest: `/tmp/pipelang-representation-lab/handoff-live.json`.
The receiver invokes `dorkpipe-objective-execution`; the old task performs no new
execution work after creation. Prior completed research and failed receipts remain
admitted evidence, reopened only for drift, a new failure or an affected dependency.

### Resident round profiling checkpoint (2026-09-07)

State: `executing`, evidence `/tmp/pipelang-resident-transcript/`. Admission matches
handoff HEAD, dirty hashes, stashes and ignored inventory. The fixed 12-object CPU
profile includes 300 exact reconstructions and separate copy/hash controls; hashing
and symbol/bit work dominate. No installed toolchain or production source changed.
Resident pipes, pointer-receiver copying reduction and shared mappings preserve all
ET01/ZT01 checks. Profiled mapped reconstruction remains materially slower than the
mapped ordinary-delta control (0.895/0.199 seconds summed per-object medians).
Root-plus-one-reference preparation is bounded to two dictionaries; grouping gives
2.110/0.526 seconds median with all required preparation included. These are warm
local reconstruction components, not native execution. The measured gap supports
rejecting corpus expansion; pending terminal checkpoint is unprofiled confirmation,
resident negative tests, exact reference proof and inclusive retained storage.

### Resident reconstruction round completed with rejection (2026-09-07)

State: **completed research experiment, unpromoted**. The strongest tested resident
candidate decodes through shared mappings into sealed executable storage, with
root-plus-one-reference preparation and all exactness checks preserved. Unprofiled
confirmation is 0.924/0.203 seconds versus ordinary delta for summed warm medians;
including grouped reference preparation gives 2.210/0.532 seconds wall and
2.357/0.561 seconds cgroup CPU. This measured loss rejects full-corpus expansion.
No fresh native execution or native-performance claim is made for this sample.

All 28 original sample/reference ELFs reconstruct exactly; 42 synthetic roundtrips,
20 rejection scenarios, 45 seal mutation attempts and all 49 contained units pass.
The sample representation is 185,242,138 bytes with support; full retained storage
is 57,292,836,767 logical bytes. All 200 production source hashes and protected
checkout/cache state remain unchanged. Prior whole-suite/native proof is admitted;
no production promotion or next language contract was selected. Details and limits:
[resident reconstruction report](conformance-similarity-search-experiment.md).
Evidence: `/tmp/pipelang-resident-transcript/final-evidence.json`.

### Further reconstruction research reopened (2026-09-07)

The user explicitly requested more research after the resident rejection. State:
`executing`, same bounded research objective and inherited exclusions. Investigate
whether repeated equivalent integrity checks can share computation without losing
any rejection guarantee, and whether selective ELF-section transcripts retain most
space savings with less expanded data, hashing and symbol work. Profile the fixed
sample by section first, retain prior prototypes, and measure the justified candidate
with CPU, memory, inclusive storage and exactness. Expand to fresh native proof only
if component evidence supports it; measured rejection is a valid result. No production
promotion or next language contract is implied. New evidence root:
`/tmp/pipelang-selective-transcript/`.

### Selective-splice sample checkpoint (2026-09-07)

Section profiling selected `.debug_line`, `.debug_loclists` and `.debug_rnglists`:
7.50 MB of the 9.82 MB section-delta savings, avoiding the expensive `.debug_info`
replay. The new recipe reconstructs a scaffold with ordinary delta, fills only those
bounded sections, verifies exact original ELF bytes and seals the completed output.
Warm fixed-sample medians sum to 0.406/0.231 seconds versus ordinary delta; encoded
sample plus all 16 references totals 31,004,014 bytes versus 45,365,275. This is a
promising component tradeoff, not native proof. Automatic next checkpoint: complete
200-object representation and fresh current v0.91 family export/native comparison,
with all existing limits and no production promotion. Evidence remains in
`/tmp/pipelang-selective-transcript/`; old transcript formats/proofs are preserved.

### Selective transcript round completed with full native proof (2026-09-07)

State: **completed research experiment, unpromoted**. Section-specific profiling
selected three debug-section transcripts plus ordinary-delta ELF scaffolds. Bounded
preparation overlaps verified reconstruction with four native workers for both paths.
The 200-object representation falls from 307,474,810 to **191,507,484 encoded bytes**;
complete support-inclusive storage is **364,910,236 bytes** versus 454,822,892.
Four matched runs average **13.068/12.527 seconds** native wall and **63.780/57.941
CPU seconds** versus ordinary delta: +4.32% wall and +10.08% CPU, not zero overhead.

All 18 full native replays preserve 2064 cases/9704 names, exact original ELF/DWARF,
current independent values/traces and fresh isolation. Fresh canonical family tests
pass; the driver's separate cache-removal housekeeping return remains recorded under
the no-cleanup scope. The round retains three pre-execution startup failures and 166
successful contained units, all removed. Forty-two synthetic roundtrips and 24
rejection checks pass. Source, protected checkout/cache state and v0.96.0 are unchanged.
No production promotion or whole-suite speed claim. See
[the selective-transcript report](conformance-similarity-search-experiment.md) and
`/tmp/pipelang-selective-transcript/final-evidence.json` for accounting and limits.

## Renewed adaptive-transcript research completed (2026-09-07)

The user-authorized profiling, section-selection and reference-selection round is
complete and unpromoted. The detailed acceptance is in
[Adaptive section and reference research](conformance-similarity-search-experiment.md#adaptive-section-and-reference-research-2026-09-07);
receipts are under `/tmp/pipelang-adaptive-transcript/`.

Matched four-run native means: fixed three sections 13.124 s wall / 57.426 s CPU;
reference shortlist 13.203 s / 58.490 s; five sections 14.742 s / 62.381 s. Encoded
bytes are respectively 191,507,484, 186,439,519 and 159,572,585. After charging all
new support, complete candidate sizes are 362,012,291 and 335,145,357 bytes versus
the previously retained 364,910,236-byte representation. Reference selection is a
modest improvement; five sections are a storage/CPU tradeoff, rejected as a universal
default. Mixed per-binary treatment loses on the unchanged sample. Timing ranges
overlap; no universal speed or statistical-equivalence claim is made.

All 20 new full native replays preserved 200 exact binaries, 2,064 cases and 9,704
ordered test names apiece; 48 candidate rejection checks, 42 synthetic roundtrips
and 576 sample sealed decodes passed. All 354 contained units passed; one sandbox
bus preflight failed before starting a workload and is retained. Peak aggregate
memory was 689,770,496 bytes, with no high/hard/OOM/swap-event increases. The source,
protected caches, stashes, ignored inventory and v0.96.0 remain unchanged. The
prior fresh family/whole-suite/warm-compiler proofs remain admitted, including the
previous canonical driver's qualified housekeeping exit. Complete retained storage
is 64,616,693,550 logical bytes; no cleanup occurred. Further research is deferred;
strict original size/time goals remain unmet.

The user subsequently requested another bounded research round via task handoff,
including binary-to-hex and related reversible representations. This renews
research authority; prior completed proof remains admitted. The pending screen and
unchanged boundaries are recorded in `conformance-similarity-search-experiment.md`
under “Further representation research authorized”.

### Hex and reversible-layout round complete (2026-09-07)

The renewed bounded research round is complete and unpromoted. On the fixed
28-object sample, ordinary hex increased compressed storage 19.15% for raw
binaries and 31.82% for selective transcripts and increased preparation and
decode/inverse time. Base64, separated hex digits and four-byte columns also
increased total storage; no transformed component beat its control. The matched
file-input screen reproduced the existing control hashes. All candidates were
rejected, so no full-family runtime/source change or new native replay was needed.

All 287 contained workload units passed, including 280 exact original sealed
reconstructions and 140 rejection checks. One sandbox-bus startup failure and an
initial mismatched stdin methodology are retained. Peak aggregate memory was
734,756,864 bytes with 317 soft-high events and no hard/OOM/swap increases. The
new root retained 10,339,330,050 logical bytes at the pre-final-receipt snapshot;
all roots totalled 74,956,067,936 bytes. Final receipt-inclusive storage is in
`/tmp/pipelang-hex-round/terminal-storage.json`. See the canonical similarity report
under “Terminal representation assessment” for matched costs, inclusive support,
resource qualifications and admitted prior proof. v0.96.0 and production remain
unchanged; strict original size/time goals remain unmet.

### Parallel reconstruction round complete (2026-09-07)

The user-requested parallel reconstruction experiment is complete and unpromoted.
Six balanced full-family orders compare 1/2/4 reconstructors with unchanged
reference-shortlist bytes, four native consumers, eight total jobs and identical
contained limits. Mean wall is **13.747/13.037/13.056 seconds**; CPU is
59.834/59.545/60.861 seconds. Two workers are the preferred prototype: 5.17%
mean wall reduction, with a 2.66% reduction in the last-four-round cache-warming
sensitivity check. Ranges overlap; no whole-suite or universal speed claim.
Four workers add no benefit and use more CPU.

All 18 full native replays preserve 200 exact binaries, 2,064 cases and 9,704
ordered names. Existing rejection/oracle checks, concurrent dictionary-lifetime
and descriptor-recovery checks, and the independent 25-second service deadline
passed. All 77 contained units passed; one sandbox-bus preflight is retained.
Peak aggregate memory was 735,113,216 bytes, with 3,979 soft-high events and
no hard/OOM/swap increases. Compressed content stays 186,439,519 bytes;
new support and retained-root costs are in the new terminal storage receipt.
See “Parallel reconstruction terminal assessment” in the similarity report and
`/tmp/pipelang-parallel-reconstruction/final-evidence.json`. Production and
v0.96.0 remain unchanged; further research is deferred.


### Latest codec research round complete (2026-09-07)

The latest-source research and contained prototypes are complete and unpromoted.
Installed Zstandard 1.4.8 patch-12 is the preparation winner: full-family dispatch
102.921 → 56.524 seconds (45.08% less), summed encoding 160.767 → 69.141 seconds,
with payload 186,439,519 → 187,811,638 bytes (+0.736%). This gain requires no
codec upgrade. New 1.5.7 patch-12 does not improve that tradeoff; new patch-19
reduces payload 5.60% but preparation takes 299.494 seconds and added support
erases the storage gain against the prior complete representation.

Four balanced orders preserve 200 exact binaries, 2,064 cases and 9,704 ordered
names across all 16 native replays. Installed dictionary/installed patch12/new
patch12/new patch19 mean wall is 15.814/15.475/15.393/15.093 seconds, with
overlapping ranges and slightly higher post-validation work for patch modes.
No replay-speed gain is established. The separate version/context decoder
screen is negative. OpenZL, DirectStorage, nvCOMP and tensor-program preprint
results remain future seams, without importing their published speed claims.

All 658 successful contained units and candidate rejection checks pass; two
support archive timeouts are retained, followed by successful sequential archive
verification. Peak per-unit aggregate memory is 734,998,528 bytes, 670 soft-high
events, zero hard/OOM/swap increases. Protected sources and state are unchanged;
prior whole-suite and independent compiler proofs are admitted, not rerun.
The new support archive is 23,152,858 bytes; complete candidate costs include
inherited overhead and every closing receipt, with retained alternatives counted
separately. See “Current-codec terminal measurements” in the similarity report
and `/tmp/pipelang-modern-codec-round/terminal-storage.json` for all tradeoffs,
resource qualifications, accounting recovery and source links. Production and
v0.96.0 remain unchanged; strict original goals remain unmet.


### Two additional strategy iterations complete (2026-09-07)

Two user-requested research iterations before implementation are complete and
unpromoted. A fixed scaffold9/token12 patch rule improves on installed patch12:
full-family mean preparation dispatch 56.765 → 47.937 seconds (15.55% less),
encoding 74.081 → 59.240 seconds (20.03% less), and payload 187,811,638 →
189,350,098 bytes (+0.819%). Both preparation orders improve, with substantial
cache/load variation; the selection rule was frozen on 28 objects before the
remaining 172 were validated. Single-root references cost 11.26% more sample
bytes and more encoding time than the mixed shortlist and were rejected.

All eight full native replays preserve 200 exact binaries, 2,064 current cases
and 9,704 ordered names. Mean wall is 14.328 versus 13.304 seconds, with
about 2.31% lower wall when excluding the first order as a cache-warming
sensitivity check. Ranges overlap; preparation is the stronger measured gain.
All 573 contained workload units and rejection checks pass; one sandbox-bus
preflight is retained. Peak per-unit aggregate memory is 735,121,408 bytes,
2,042 soft-high events in the baseline only, zero hard/OOM/swap increases.
Protected source and state checks, Python AST, YAML and whitespace checks pass;
prior whole-suite and independent compiler proof remain admitted, not rerun.

The next implementation candidate is two reconstructors, existing reference
shortlist and scaffold9/token12 patch compression. Only task-owned docs/indexes
changed; temporary prototypes and complete accounting are under
`/tmp/pipelang-next-two-rounds/`. See “Final two-iteration native and resource
proof” in the similarity report for timing qualifications, inclusive storage and
retained artifacts. Production, v0.96.0 and package/engine boundaries remain
unchanged; no original strict target is claimed.


### Implementation continuation authorized (2026-09-07)

After the two final strategy iterations, the user said “please go ahead” and
explicitly invoked `dorkpipe-task-handoff`. The selected implementation is two
reconstruction workers, the existing reference shortlist and fixed scaffold9 /
token12 patch compression. Continue the same objective in exactly one fresh task,
directly in this saved checkout. State: ready_for_execution. The user go-ahead
supplies implementation authority; handoff itself only transports it.

Pending boundary: admit the completed experiment receipts, inspect the canonical
conformance harness and integrate the selected representation/preparation/replay
path into repository-owned source with no `/tmp` runtime dependencies. Preserve
exact coverage, ELF/DWARF bytes, current Value/Trace oracle, invalidation, sealing,
deadlines and resource limits. Finish focused correctness/rejection checks and
matched preparation/replay measurements; revalidate broader compiler/native proof
only when affected implementation or source drift makes it necessary. Update
canonical task docs with the actual implementation and limitations. No new
language/codec/research seam, commit, push, worktree, cleanup, installation,
release/publication or persistent external change is authorized by this handoff.


### Repository integration checkpoint (2026-09-08)

State: executing under the implementation authority above. Integration is owned
by `tests/containedexec`: the exact transcript helper, sealed decoder/service,
bounded native pipeline, source-bound selected reference plan, and contained
preparation/replay driver now have repository-owned paths. There are no embedded
research paths. The canonical harness README describes the opt-in family lane
and its unsupported-input behavior; v0.96.0 and generic engine source are unchanged.

The first development preparation passed 102 contained units and reproduced all
400 selected prototype payload hashes. Its nine focused tests passed, including
independent Value/Trace rejection and dictionary/descriptor lifetime checks.
Complete support/receipt accounting was then extended before the final matched
acceptance. Development evidence remains under
`/tmp/pipelang-transcript-integration/`; it is not the terminal performance claim.
The existing whole-suite/compiler proofs remain admitted because their source is
unchanged. Final matched preparation/native/resource acceptance is in progress.


### Repository-owned transcript implementation accepted (2026-09-08)

State: **completed** for `TASK-021-conformance-shared-runtime-experiment` under
the explicit implementation go-ahead. The selected representation, preparation
and replay path is now owned by `tests/containedexec/transcript_suite.py`,
`transcript_runtime.py`, `transcript_replay.py`, `transcript_plan.json`, and the
three Go helper files under `transcript/`. `test_transcript.py` owns rejection
and lifetime checks. The canonical harness README documents invocation, costs,
limits and unsupported inputs; `pipelang_suite.py` identifies transcript exports
and includes nested helper sources in its drift snapshot. No generic engine,
compiler/evaluator, authored language surface or v0.96.0 contract changed.

The plan freezes the previously selected reference assignments for the exact
200-bundle source corpus. It retains 16 references, one root and the existing
four-shard schedule. It does not rerun the research search. Source changes fail
closed and use the ordinary bundle harness until another plan is deliberately
prepared; no automatic generalized reference selection is claimed. Preparation
uses installed Zstandard 1.4.8 patch mode (scaffold 9 / tokens 12). Runtime code
has no research-directory dependencies. The helper/library are retained with the
representation; toolchain, Python and native support bytes are recorded and
validated. Original binaries are optional for packed replay but required for
preparation and raw comparisons. No cache migration or deletion occurred.

All **559 contained units passed**, including **15 focused tests** and Go vet.
Five complete preparations include the development pass and two balanced
candidate/control orders; all 1,600 final comparison payload hashes match the
selected prototype/control exactly. Helper self-tests cover 21 synthetic zlib
roundtrips per build. Nine complete native replays (eight in four alternating
raw/packed comparison pairs) each preserve **200 exact ELF/DWARF binaries,
2,064 cases and 9,704 ordered names**. Rejection checks cover corrupt
root/reference/leaf payloads, wrong dictionaries, malformed spans/lengths,
source/settings/support changes, immutable sealing, two concurrent readers,
error-path descriptor recovery, independent blocked read/write deadlines, and
separate current Value and Trace failures.

The current canonical family was regenerated before final native acceptance:
all ten build/list/family units passed; all 12,368 current export input files
match the admitted deterministic source/oracle output. Its outer driver exits 1
solely because the validation adapter intentionally retains its empty private
native-build cache. This is qualified family proof, not a clean driver exit or a
new whole-suite pass. The prior 636-test whole-suite and independent warm
128 MiB / 5-second compiler proofs remain admitted because those implementations
are unchanged. Of 200 admission source hashes, 198 remain unchanged and exactly
two authorized paths changed (harness README and suite driver). All 2,719 cache
records/sizes and all 200 affected native hashes match; branch, HEAD, empty index,
protected stashes and the 108,166-entry ignored inventory remain unchanged.

| Matched measure | Control | Selected |
| --- | ---: | ---: |
| Mean preparation dispatch, two orders | 104.490 s | 98.409 s |
| Summed component encoding, mean | 64.538 s | 50.962 s |
| Compressed payload | 187,811,638 B | 189,350,098 B |
| Mean native dispatch, four orders (raw / packed) | 11.886 s | 12.562 s |
| Native dispatch range | 11.766–11.976 s | 12.430–12.632 s |
| Peak native aggregate memory | 189,878,272 B | 216,256,512 B |

The preparation control is patch12, while the native control is retained raw
execution. Selected preparation dispatch improves **5.82%** and summed encoding
**21.04%**, for 0.819% more payload; both preparation orders improve. Packed
native dispatch costs **5.69%** more than raw. These measurements do not
compound earlier research percentages. Dispatch excludes parent cache inventory
and accounting; final outer native commands took about 29.3–30.7 seconds,
including roughly 12 seconds of accounting and the pre-dispatch cache inventory.
No whole-suite, CPU-time or universal speed claim is made.

Peak per-unit aggregate memory across acceptance is **734,785,536 bytes**;
there are 900 soft-high events and zero hard/OOM/swap increases.
The same 1 GiB hard limit, zero swap, 128 tasks, 800 MiB proactive stop,
700 MiB soft reclaim, 30-second unit / 25-second child deadlines, two contained
units, two reconstructors, four native workers and eight total jobs remain.
All cgroups were removed. The reconstructed dictionaries outlive every reader.

The selected retained representation is 197,661,769 bytes, including payload,
helper/library/encoder copies, plan, identities, recipes and manifest. This is
not complete storage by itself: current exports, all Go/Python/native support,
harness source, receipts, retained originals, compiler-cache growth and all
coexisting controls/development artifacts are charged separately in each
`storage.json` and the terminal receipt. Closing receipt sizes include themselves.
See `/tmp/pipelang-transcript-integration/final-evidence.json`,
`outer-replay-timings.json`, `retained-notes.json` and `terminal-storage.json`.
Strict original size/time targets remain unmet. The opt-in bounded implementation
is complete; broader language/codec/generalized-compression work remains deferred.


### Corrective generalization authorized (2026-09-08)

The user challenged whether the implementation benefits the framework rather
than only hardcoded tests. Inspection confirmed that `transcript_suite.py`
requires 200 artifacts, 16 fixed references and four groups of 50, with
`transcript_plan.json` freezing their keys and source fingerprints. Its only
connection to the normal suite is an export interface. The earlier completion
therefore describes the bounded corpus implementation, **not completion of the
intended reusable conformance-framework outcome**.

After this was stated plainly, the user said: “Yeah we need to do the otherwise
it’s a cheap hack please work on doing it properly” and invoked
`dorkpipe-task-handoff`. This directly authorizes corrective implementation and
one same-checkout continuation. It is not a new candidate-selection gate. The
same objective is reopened as ready_for_execution; implementation is approved.

Objective contract:

- objective_id: `TASK-021-conformance-shared-runtime-experiment`
- execution_skill: `dorkpipe-objective-execution`
- authorized_objective: replace the corpus-bound wrapper with reusable native
  artifact preparation/replay in the normal PipeLang conformance framework.
- done_when: derive a deterministic bounded reference plan from current artifacts
  instead of committed artifact ids, counts, reference ids or source whitelists;
  support changed/new eligible tests through proper invalidation and preparation;
  integrate ordinary retained-artifact execution so no hand-maintained family
  manifest is required; preserve a valid ordinary path for unsupported forms;
  retain independent current Value/Trace checks and exact debug-bearing bytes;
  prove focused adversarial and lifecycle behavior plus a **fresh complete
  discovered suite**, matched before/after execution/preparation and inclusive
  storage; document actual framework benefit and limitations. Prior whole-suite
  proof cannot substitute for this explicitly required fresh terminal proof.
- inherited_invariants: exact ELF/DWARF; current independent oracle; source,
  toolchain and settings invalidation; immutable final sealing; reader-owned
  dictionary lifetime; unchanged compiler resource contracts; at most two
  reconstructors, four native consumers and eight jobs per contained unit, and
  at most two concurrent units; all compiler/native/codec work through `run.py`
  with 30-second units, 25-second children, 700 MiB high, 1 GiB hard, zero swap,
  128 tasks and proactive 800 MiB stop. Keep v0.96.0 unchanged.
- ownership: conformance/artifact helpers and their actual source owners, not
  generic DockPipe engine special cases. Apply focused core-review guidance if
  integration touches compiler-owned test helpers under `src/lib/pipelang`.
- checkpoints: automatic within this correction; user-requested handoff only.
- exclusions: no new language/codec research seam, broad product redesign,
  commit/push, stash/worktree mutation, installation, cache cleanup, publication,
  deployment, credentials or persistent external changes. Reviewed bounded host
  validation remains available when the sandbox cannot reach the systemd bus.
- terminal_policy: complete only with reusable integration and the required fresh
  acceptance; otherwise record the actual blocker or failed verification.

Pending boundary: inspect the current native cache/bundle helper interfaces and
normal suite lifecycle, map reusable representation/preparation ownership, then
begin replacing the fixed plan/admission and connecting the normal execution
path. Do not merely generate another static 200-case JSON, hide the same fixed
corpus behind a flag, or mark an export-only wrapper as framework integration.
Reuse the proven scaffold9/token12 mechanics and bounded reference approach;
perform necessary engineering, not a replay of completed search experiments.

Admitted evidence: `/tmp/pipelang-transcript-integration/final-evidence.json`
and `terminal-storage.json`, with 559 passing units, 15 focused tests, Go vet,
nine exact 200-bundle replays, independent oracle/rejection/sealing proof and
qualified fresh family generation. These validate components and the old corpus
only. Preparation averaged 104.490 -> 98.409 s versus patch12; packed native
12.562 s versus raw 11.886 s. The 197,661,769-byte representation replaces
1,302,426,120 raw bytes in principle; no originals were deleted, and the work
retained roughly 2.44 GB more data. Toolchain/fixture/support/receipt costs are
separate and counted. No whole-project speed or 85% total-framework storage claim.

The source of truth for the receiving checkout is the fresh handoff receipt at
`/tmp/pipelang-transcript-generalization-handoff/anchors.json`. All existing
implementation and research documents are owned and must be preserved/adapted.
No operation is in flight. Revalidate affected anchors and execute the pending
boundary first, then converge on the authorized framework outcome.

Corrective execution started: handoff anchors verified (14 owned paths, HEAD/index,
protected stashes and component receipts). Native cache identity and batch sealing
are the integration seam. Reference planning now derives from current artifact
sizes and keys with bounded deterministic selection; no language changes.

### Reusable cache integration checkpoint (2026-09-08)

State: executing; fresh complete acceptance is in progress, not yet claimed.
`native_artifacts.py` now owns normal cache-derived bounded planning, lazy
scaffold9/token12 preparation and exact sealed descriptor delivery. The Go
compiler-owned batch helper requests bytes using its independently computed
current source/toolchain/settings key; current fixtures and per-case processes
remain owned by that helper. There are no language-version or family-name
branches in representation admission. Unsupported ELF/DEFLATE forms use the
ordinary path. New keys prepare on demand. The export CLI now derives its plan
from current exports; the old checked-in JSON is historical research data only.
Two reconstruction slots run inside each test unit, with at most four existing
native consumers. Reference ownership is synchronous and local to each slot.

Affected-anchor admission passed. Initial focused Go artifact/bundle tests passed
in 21.982 s; dynamic planner tests passed; five new lifecycle tests prove new
eligible preparation, unsupported fallback, replay with no raw executables or
exports, record/digest invalidation and corruption/cycle rejection. Go descriptor
transport tests reject missing seals and wrong bytes. First ordinary v0.91 shape
passed through three automatically prepared objects in 2.109 s, peak aggregate
130,646,016 bytes. Evidence lives in `/tmp/pipelang-generalized/`.

The required fresh suite at `full-packed/` discovers 637 tests, scheduled into
768 units with shape batches of five and one ordinary top-level test per unit.
All build/list/compiler/native/preparation workloads use 30-second units,
25-second children, 700 MiB soft-high and the inherited hard limits. Disposable
native build-cache contents are retained under the explicit no-cleanup scope;
retention itself is no longer reported as a failed validation adapter. Matched
raw execution and inclusive storage accounting remain pending.

### Corrective framework acceptance complete (2026-09-08)

The reopened objective is **completed**. Native representation ownership is now
in the ordinary conformance cache path, not a fixed family/export wrapper.
`native_artifacts.py` deterministically selects at most 16 references from current
cache keys and sizes, prepares missing identities on demand, atomically publishes
supported representations under locks, and serves verified sealed descriptors.
The compiler-owned `generated_batch_test.go` uses its current source/toolchain/
settings key to request the bytes and retains fresh per-case fixture directories,
independent oracles and processes. `generated_transport_*_test.go` validates the
received SHA256 and kernel seals. No version/family admission branch, expected
Value/Trace cache, or committed source/key whitelist exists in that transport.
Unsupported forms use the ordinary path. New keys absent from a run's initial
reference snapshot prepare without a dictionary; existing immutable recipes keep
their valid DAG. A later run can select those new artifacts as references.

The export comparison driver also derives its plan dynamically. Its old
`transcript_plan.json` is preserved historical research data, not execution
admission. Reconstruction/encoding mechanics and v0.96.0 are unchanged. The
changes under `src/lib/pipelang` are compiler-owned test helpers only; generic
DockPipe engine/package boundaries are preserved.

Fresh acceptance (all evidence under `/tmp/pipelang-generalized/`):

- `full-raw/` and `full-accepted/` each passed **637 discovered tests in 812
  contained units**, with zero failed units and source unchanged during each run.
  Both runs independently built/listed the current suite and ran **623 direct
  compiler resource probes**. Coverage is identical: **7,743 ordered generated
  source/fixture/test-name audit vectors**, **13,059 named invocations** including
  multiplicity, and **2,719 exact native artifacts**. Eight existing map-iterated
  assertion tests list their subtests in different orders; each unit's subtest
  multiset still matches. Ordered generated vectors and current trace bytes match.
- The 30-second-unit cold exploration initially passed 758/768 units and exposed
  ten preparation-time deadline overruns. The corrected scheduler partitions the
  existing compiler-memory choice/branch groups, v0.92-v0.96 memory choices, and
  the asserted v0.81/v0.84 shape inventories. No cases, sizes, limits or assertions
  were removed. The failed receipts remain; they are not acceptance. The final
  packed run completed the remaining **151** first-time preparations successfully.
- New planner/lifecycle checks prove deterministic plans for varying inventories,
  new eligible preparation, unsupported fallback, packed-only execution without
  original executables/exports, changed records/digests, corrupt payloads, cycles,
  immutable seals and descriptor recovery. Go transport checks reject unsealed
  descriptors and wrong bytes. Four integrated normal-helper probes show initial
  preparation, warm reuse, changed-current-oracle rejection on a hit, and changed
  source producing a new key and preparation. Existing current Value/Trace and
  codec component proof remains admitted; the new whole-suite proof runs current
  independent fixtures through the normal helpers. Focused Go vet passed.
- Final inventory reporting now supports packed-only retention as well as execution.
  Its regression counts absent originals as zero retained bytes and rejects a
  mismatched recipe. This post-run accounting change and its Python unit test are
  the only two source-file differences between the ordinary and packed phases;
  Go binaries, workload source/fixtures and the execution schedule are identical.
  All four planner/accounting tests passed; Python syntax and `git diff --check`
  passed.

Measured costs, without extrapolated target claims:

| Measure | Ordinary | Packed |
| --- | ---: | ---: |
| Complete outer run | 382.960 s | 625.632 s |
| Complete execution phase | 381.431 s | 624.055 s |
| Summed contained test-unit elapsed time | 670.733 s | 1,159.421 s |
| Original executable bytes represented | 12,691,261,599 | 12,691,261,599 |
| Selected retained representation, including local support/recipes/locks | — | 2,298,795,073 bytes |

The complete packed execution phase is **63.61% slower** in this comparison and
includes 151 new preparations; it is not a pure warm-replay or speedup claim.
Across cold exploration and final acceptance, all **2,719** objects prepared,
with **936.545 summed object-preparation seconds** (wall time summed across
objects, not CPU time or outer dispatch). This preparation includes component
roundtrip checks. The ordinary control adds no representation-encoding work.
The logical representation is **81.89% smaller than this executable set**, but
this is not whole-framework storage savings. Every original remains: **zero
bytes freed**. The selected namespace has 10,881 retained files. The task also
retains its exploratory namespace, receipt logs, test binaries, fixtures and
focused-probe artifacts. Native compiler-intermediate directories were empty and
retained, consistent with the no-cleanup instruction.

`storage.json` counts current nonoverlapping research/control roots, compiler
cache, original binaries and records, repository inputs, toolchain/Python/native
support and this task's artifacts. Closing totals include the receipts themselves;
logical file accounting does not infer shared physical blocks. The old family
representation and earlier research controls continue to coexist and are charged.
No claim is made that retaining both raw and packed data reduces actual disk use.
Dictionary reuse, service-startup overhead and any later cache migration remain
possible future work, not unapproved additions to this completed correction.
The strict earlier time/storage targets are still unmet.

Peak aggregate memory across the recorded work is **734,773,248 bytes**, with
167 soft-high events and zero hard-memory/OOM/swap event increases. Every workload
used the required contained runner, offline Go 1.25.13, 30-second unit and
25-second child limits, 700 MiB soft-high, 1 GiB hard, zero swap, 128 tasks and
800 MiB proactive stop. No more than two units, two reconstruction slots per unit,
four native workers or eight jobs were admitted. All test cgroups were removed.

Final protected-state readback preserves branch `js/pipelang`, HEAD
`8c7c070c4eb2141c74335875017ef6ddce4f942f`, empty index and both stashes.
All 2,719 current cache records/sizes equal the pre-handoff live-set receipt,
and every original executable SHA256 was freshly checked. The 108,166-entry
ignored inventory retains SHA256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`.
Earlier codec/replay bytes, static research plan and research narrative are
preserved. No commit, push, worktree/stash mutation, cache cleanup, installation,
publication, credentials or persistent external operation occurred. No successor
task was created.

Packed-only end-to-end closeout also passed: `packed-only.json` records a
1.054-second contained invocation of the normal Go helper with both original
paths detached in the task's private probe cache. The helper hit the prepared
representation, executed the current oracle successfully, and final inventory
reported zero original executable bytes. Both probe binaries were restored;
the protected conformance cache was untouched. See `packed-only-inventory.json`.

### Ordinary execution restored as default (2026-09-08)

The user accepted prioritizing day-to-day test feedback and keeping compression
as an explicit option. `pipelang_suite.py` now defaults `native_representation`
to false and no longer enables it from codec availability or a retained cache.
`--native-representation` enables the existing packed path;
`--no-native-representation` selects ordinary execution explicitly. A
`--representation-cache` path alone does not enable it. Existing shared native
bundles keep their separate prior behavior. The implementation, codecs, cache
contents, resource limits, test selection and v0.96.0 are unchanged.

Validation for this default-selection-only change: four parser-selection checks
(default, explicit opt-in, explicit opt-out and store-path-only), the four existing
suite planner/accounting tests, CLI help and `git diff --check`. The earlier fresh
637-test ordinary and packed acceptance runs remain evidence for their unchanged
execution paths; no new whole-suite timing or compiler run is claimed. No further
compression search, cache cleanup, commit, push or successor work was started.
