# Contained generated-Go validation

## Durable campaigns

`verification_campaign.py` runs the complete compiler suite, the 1,944 v109, 192 v110, 576 v111 and 192 v112
isolated compiler fixtures, the nine integration checks and the editor tests in
sequence under one `job.py` budget. Its default data root is
`~/.cache/pipelang-verification`, separated into `campaigns/<name>`, `builds`,
`executables` and toolchain-bound Go build caches. Supply an absolute cached
Go 1.25.13 executable, the installed Node executable and the admitted baseline
receipt directory explicitly. Integration fingerprints the selected Node bytes.

```sh
python3 -B tests/containedexec/job.py --output /absolute/proof/job --timeout 21600 -- \
  python3 -B tests/containedexec/verification_campaign.py \
  --root /absolute/private/proof --campaign terminal --go /absolute/go --node /absolute/node \
  --baseline /absolute/private/accepted-baseline
```

Use a new job receipt path and `--mode resume` after interruption. Fresh mode
requires a new campaign name and executes all semantic checks; persistent verified
build objects may still be reused. Resumption validates every required retained
artifact and current dependency before admitting a completed receipt. Old receipts
without the versioned campaign schema remain baseline evidence only.

After the stage job exits, run the same driver with `--accept-job /absolute/proof/job.json`
inside a new small `job.py` invocation. This independently reconciles atomic receipts,
baseline inventory, compiler ceilings, integration/editor checks and the completed
aggregate job's cleanup. `accepted-verification.json` is published only then.

Individual `pipelang_suite.py`, `matrix.py` and `integration.py` entry points also
accept `--mode fresh|resume`. Matrix accepts either `--fixtures DIR` or
`--fixture-manifest FILE` containing absolute measurement-file paths. The suite
supports explicit `--selection-file` logical-case samples, always reported as
partial proof, and `--build-store` / `--native-build-cache` persistent paths.

Campaign schema `pipelang-campaign-v1` uses checksummed manifests and receipts,
fsync/rename publication, immutable manifest revisions, exclusive kernel writer
locks and linked attempts. A successful receipt requires complete case selection,
zero exit, unchanged inputs, verified artifacts, unchanged resource limits and
positive cleanup proof. Recovery retains incomplete and failed attempts; the
append-only event log and compact indexes never authorize proof. There is no cache
or proof deletion command. All referenced objects are pinned by preservation.

Module discovery runs offline inside containment. Content fingerprints cover dirty
and untracked local inputs, transitive module sources, embeds, toolchain bytes,
settings and policy. Linux input watches reject writes, restored bytes, renames,
new files and lost watches during execution; every new process hashes again.
Boot ID is provenance only. Host/kernel and resource-policy identity remain relevant.

Native executables retain current Value/Trace oracles and fresh children/fixtures.
Main test binaries are verified before reuse. Standard-library preparation validates
its cache-specific archive digests; missing or corrupt preparation is rebuilt under
the existing deadline. Only build preparation uses `GOMEMLIMIT=600MiB`.

`--disk-budget-gib` defaults to 96 GiB across selected caches and suite evidence.
An initial inventory plus incremental kernel notifications protects disk headroom;
new executable publication also reserves exact bytes across workers. Capacity
exhaustion stops population and preserves evidence. Build-store objects have a
separate 4 GiB budget. No garbage collection is authorized or performed by the
campaign controller.

Each suite writes a reusable `schedule-profile.json` from actual warm singleton
observations. The default schedule keeps singleton units. `--schedule-profile` admits only
current-input, host, worker and policy matched warm measurements with verified
executable digests: compatible numeric shapes may pair when their
predicted total is below 10 seconds with memory headroom. Unknown/heavy shapes,
memory families and special harnesses stay separate. A failed group retries as
linked singleton attempts under unchanged deadlines. Parallel shapes remain off.

Detailed logs stay in attempt directories, capped at 64 MiB per unit. `suite.json`,
`matrix.json`, `checks.json` and compact summaries remain available to legacy
readers and are regenerated at stage end. `timing.json` reports family quantiles,
workload occupancy, cache use, artifact audits and bounded phase timing. Process
monotonic clocks have separate domains; missing timing is unknown and nested
spans must not be added to workload time. See the
[architecture](../../docs/runtime/pipelang-verification.md) for scope and adoption evidence.

Generated PipeLang and Application IR tests require Linux cgroup v2 and a user
systemd manager. There is no uncontained fallback. Other platforms currently
refuse generated compilation; implementing and verifying an equivalent process-tree
strategy is required before claiming resource proof there.

Wrap suite and matrix coordinators with `job.py`. It creates a unique temporary
slice shared by the coordinator and every `run.py` child: 2 GiB hard maximum,
1536 MiB reclaim threshold, zero swap, 384 tasks and a proactive stop at 1800 MiB.
The coordinator separately has 512 MiB and 64 tasks; child limits remain unchanged.
Both coordinator and child verify their actual ancestor limits before starting work.
Each child binds its lifetime to the coordinator. An independent coordinator service
deadline and an outside supervisor stop the whole slice, including sibling units.
The supervisor retains only counters; test log inventory is streamed. Suite/matrix
entrypoints refuse execution outside this verified job. No persistent unit files or
machine settings are changed. Use durable private output/cache paths when receipts
must survive reboot; `/tmp` does not provide that guarantee.

```sh
python3 -B tests/containedexec/job.py --output /absolute/private/proof/job --timeout 21600 -- \
  python3 -B tests/containedexec/pipelang_suite.py --go /absolute/go \
  --output /absolute/private/proof/suite --cache /absolute/private/proof/cache --workers 1
```

Run `python3 -B tests/containedexec/test_job.py` with the user systemd manager before
compiler work to verify nested containment, escaped-process cleanup, independent
job deadline cancellation and refusal of an uncontained coordinator. These are
small synthetic probes; `test_run.py` additionally exercises per-unit limits.

Run from the repository root, using an absolute Go toolchain path and a private
cache/output directory. The launcher verifies `memory.max=1073741824`,
`memory.swap.max=0`, `pids.max=128`, `KillMode=control-group`, and a finite service
deadline before starting a workload. It samples aggregate memory every 5 ms and
terminates the whole unit near 800 MiB. The kernel hard limit covers sampling
overshoot. Tests independently recheck containment before each generated child,
apply a 30-second child deadline, and cancel the whole unit on loss of accounting,
timeout, or memory pressure. An outer `go test -timeout` alone is insufficient.

```sh
python3 tests/containedexec/run.py \
  --output /tmp/compiler-proof/suite --cache /tmp/compiler-proof/cache --timeout 900 \
  -- /absolute/go test -p 1 ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1 -timeout=13m
```

The default case deadline is 30 seconds. A separately labeled cold bootstrap or
suite can request up to 1800 seconds. Every unit has its own independent deadline
(case timeout plus 10 seconds). The outside runner stops the exact temporary
unit on exit and checks cgroup removal. Reports distinguish sampled compiler RSS,
waited-child maximum RSS, aggregate peak, elapsed time, cache path, flags,
exit/stop state, swap usage and memory/swap events. Compiler-only resource claims
require a direct compiler command: suite/build child RSS can include the test
harness, standard library, linker, or executed program. Sampling can miss very
short compiler processes; direct waited-child RSS retains their high-water mark.

The source-admitted regression matrix independently crosses 8/16/24/32/64/128/256
locals with zero/one/two/three/all choices where admitted, v0.40/v0.72/v0.83/v0.84,
branch scopes, dependent values and unused locals. v0.85 additionally crosses straight-line
zero/one/two/three/all choices and unused final choices through 256 locals. Export names
and isolated matrix family keys distinguish branch, straight-line and unused dimensions.
v0.86 crosses the same six straight-line families with a complete ternary return,
including ordinary-only locals, dependent choices and unused final choices; both return outcomes
are exercised separately in each case. Versioned family identity keeps these measurements separate from v0.85.
v0.87 adds twelve root/branch families with zero/one/two/three/all conditional initializers
and unused final choices before terminal-leaf ternary returns. Independent return conditions
exercise both arms in reached branches through 256 locals.
v0.88 adds six straight-line families with depth-two return choices in both arms, crossing
zero/one/two/three/all conditional initializers and unused final choices through 256 locals.
All return paths execute separately.
It asserts closure depth,
evaluator/executable-Go parity, 128 MiB RSS and 5 seconds per warm direct compiler
invocation. Export fixture modules with the command below, then run `matrix.py`
for a fresh cgroup and aggregate peak per compiler case:

```sh
python3 tests/containedexec/run.py \
  --output /tmp/compiler-proof/regression --cache /tmp/compiler-proof/cache --timeout 180 \
  -- env PIPELANG_MEMORY_FIXTURES=/tmp/compiler-proof/fixtures /absolute/go test \
  -p 1 ./src/lib/pipelang -run TestCompilerMemoryLocalSequences -count=1 -v
python3 -B tests/containedexec/job.py --output /tmp/compiler-proof/matrix-job -- \
  python3 -B tests/containedexec/matrix.py \
  --fixtures /tmp/compiler-proof/fixtures --output /tmp/compiler-proof/isolated \
  --cache /tmp/compiler-proof/cache --compiler /absolute/goroot/pkg/tool/linux_amd64/compile
```

Each family stops at its first resource crossing. Normal Go inlining stays on.
No module downloads, custom GC flags, machine setting changes or service restarts
are involved. On hosts where the sandbox cannot access the user manager, use a
reviewed narrow host invocation of this launcher.

For a large suite whose aggregate file-cache charges accumulate across hundreds
of generated builds, compile its test binary with `run.py`, then use `suite.py`
from the package directory. It discovers every top-level test, fuzz seed function
and example, assigns each to exactly one batch, and runs each batch in a fresh
unit. This does not raise the memory cap or change compiler flags. A failure
retains the active test name in the batch log; all batch outcomes are recorded.

Run `python3 tests/containedexec/test_run.py` outside a containment unit to verify
success cleanup, cancellation of an escaped session, proactive memory stopping,
refusal before launching an uncontained child, and the independent service timer.
These synthetic probes use temporary units and never invoke the compiler.

The v0.83 exhaustive scope-pair test also needs a fresh unit per numbered shape:

```sh
# Run from src/lib/pipelang after building the test binary with run.py.
python3 /absolute/repo/tests/containedexec/suite.py \
  --binary /tmp/compiler-proof/pipelang.test --output /tmp/compiler-proof/batches \
  --cache /tmp/compiler-proof/cache \
  --split-test TestV830TwoConditionalLocalsAllShapesScopePairs=25
```

`--only NAME` can select a discovered top-level test for a focused repair rerun.
Numbered split counts must match the test's own asserted inventory. Splitting a
large module into at most 24 sample methods preserves every outcome and trace;
fresh shape units additionally prevent cumulative file charges from ending a long run.

For bootstrap or integration phases dominated by file-cache charges, add
`--memory-high-mib 700`. This requests temporary kernel reclaim before the
unchanged 800 MiB proactive stop; it retains the 1 GiB hard cap and zero swap.
The effective `memory.high` value is verified and reported. Its `high` event
counter may increase normally during reclaim; distinguish that from `max`,
`oom`, `oom_kill`, and swap events. Warm compiler-only measurements should keep
the default controls so results remain comparable. This option does not alter
Go's garbage collector, compiler flags, or persistent machine configuration.

The v0.87 leaf-subset and ordered-layout tests use 25 asserted shapes each. Include
`--split-test TestV870TerminalLeafConditionalReturnsSubsets=25` and
`--split-test TestV870TerminalLeafConditionalReturnsLayouts=25` in their `suite.py`
runs to retain fresh per-shape accounting. Subset modules contain at most 16 methods.

Also split `TestV820ConditionalLocalAllShapesScopesAndPaths=25` in full runs so its
exhaustive generated modules cannot share a deadline with the expanded memory matrix.
If a previously grouped batch times out, retain its receipt and rerun only its tests in
smaller fresh groups; keep the memory limits and service/child deadlines unchanged.

The v0.88 layout test enumerates four numbered ternary shapes. Use
`--split-test TestV880NestedStraightLineReturnsLayouts=4` for fresh per-shape accounting.

The v0.89 nested-leaf subset test enumerates 100 numbered cases: all 25 statement-tree
shapes crossed with four rotations of the bounded return shapes across leaves. Use
`--split-test TestV890NestedTerminalLeafReturnsSubsets=100` and
`--split-test TestV890NestedTerminalLeafReturnsLayouts=25`. Every leaf subset is exercised
with six independent condition bits; the layout matrix separately checks eager ordered,
unused, lexical and lazy traces with independent nested conditions. Its twelve root/branch
scale families cross zero/one/two/three/all conditional initializers and unused final locals
through 256 locals. Export `TestCompilerMemoryLocalSequences/v0.89.0` then use `matrix.py`
for 84 fresh isolated normal-inlining compiler measurements under unchanged limits.

The v0.90 initializer layout matrix enumerates 25 numbered cases: five initializer
layouts (four uniform shapes and a mixed rotation) crossed with an ordinary return or
one of four depth-two return shapes. Use
`--split-test TestV900NestedStraightLineInitializersLayouts=25`. It covers local subsets,
ordered/unused locals and lazy conditions/arms in two-method generated modules. Its
12 scale families cross zero/one/two/three/all nested initializers and unused final
locals with ordinary/nested returns through 256 locals. Export
`TestCompilerMemoryLocalSequences/v0.90.0` for 84 fresh isolated measurements. Normal
inlining and the existing 128 MiB / 5 second warm compiler ceilings remain unchanged.

If the complete memory matrix exceeds a batch deadline, run its ten language-version
subtests in separate fresh units using the final test binary and patterns such as
`-test.run '^TestCompilerMemoryLocalSequences$/^v0\.90\.0$'`.
The exact version inventory is v0.40.0, v0.72.0, v0.83.0, v0.84.0, v0.85.0, v0.86.0,
v0.87.0, v0.88.0, v0.89.0 and v0.90.0. Retain the failed receipt and run other tests
from its interrupted batch separately. Do not increase deadlines or memory limits.

The v0.91 nested terminal initializer layout matrix crosses all 25 statement shapes
with ordinary and depth-two leaf returns. Use
`--split-test TestV910NestedTerminalInitializersLayouts=200`: each shape/return pair
has four disjoint sample partitions, retaining the same cases under fresh accounting.
Root/intermediate/leaf sequences cover zero/one/two/three choices, all subsets of the
three root/left/right scope slots, selected five-local mixed layouts, and unused locals.
Three-local sequences rotate the true-only, false-only and both-arm nested shapes;
five-local sequences also contain a nonnested initializer. Each supplied condition
vector is exhausted: statement, outer initializer, inner initializer and return bits
are separate; the two inner initializer bits are shared across initializers. This is
a bounded matrix, not every independent condition/shape assignment at arbitrary length.

Its 24 scale families cross root/branch locals, ordinary/nested returns,
zero/one/two/three/all nested choices and unused final locals through 256 locals.
Export `TestCompilerMemoryLocalSequences/v0.91.0` and run `matrix.py` for 168 isolated
normal-inlining compiler cases. Keep the 128 MiB / 5 second warm ceilings. Full runs
now have eleven memory-version units (the ten above plus v0.91.0); run those units
separately from the start rather than accumulating them in a single batch.

The v0.92 arrow-method scale test is `TestV920NestedArrowMethodsMemory`. It crosses
all four fixed arrow shapes, used/unused final caller locals, and 0/1/8/16/32/64/128/256
locals: 64 cases. Every condition vector is evaluated and executed in pristine Go.
Export with `PIPELANG_MEMORY_FIXTURES`, then use `matrix.py` for 64 fresh isolated
normal-inlining compiler measurements with unchanged 128 MiB / 5 second ceilings.
The arrow has no locals; these fixtures prove scaling of inherited caller sequences
that repeatedly invoke it. The eleven-version inherited memory matrix remains separate.

In full compiler runs, additionally use `--split-test TestV920NestedArrowMethodsTypes=4`,
`--split-test TestV920NestedArrowMethodsCarriers=4` and
`--split-test TestV920NestedArrowMethodsLayouts=4`. Each index identifies one of the four
arrow shapes. Keep `TestV920NestedArrowMethodsMemory` in its own fresh unit.

The v0.93 return layout matrix enumerates 25 numbered shapes. Use
`--split-test TestV930DepthThreeReturnsLayouts=25` for fresh per-shape accounting.
Each shape crosses 0/1/3 preceding locals, used/unused final bindings, ordinary and
nested conditional initializers, seven independent return-condition bits and two
initializer-condition bits shared across locals. This proves the supplied bounded
layouts, not every independently assigned initializer vector at arbitrary length.

`TestV930DepthThreeReturnsMemory` separately measures 64 fixed-helper caller cases
through 256 locals. Export with `PIPELANG_MEMORY_FIXTURES`, then use `matrix.py`
for fresh normal-inlining measurements under the unchanged 128 MiB / 5 second
warm ceilings. The eleven-version inherited local-sequence matrix remains separate.

The v0.94 terminal-leaf return tests use
`--split-test TestV940DepthThreeTerminalLeafReturnsLayouts=25` and
`--split-test TestV940DepthThreeTerminalLeafReturnsSubsets=100`.
Layouts pair the 25 statement shapes with the 25 return shapes, placing zero/one/three
locals in each leaf and exhausting seven independent return bits and two shared
initializer bits. Statement conditions reuse the outer initializer bit in this matrix.
The separate subset matrix exercises every leaf subset of every statement shape with
three independent statement-depth bits and three shared return bits in four rotations.
The duplicated outer choice reaches depth three for nested rotations. These supplied
matrices do not claim every independent statement/return/initializer assignment.
`TestV940DepthThreeTerminalLeafReturnsMemory` exports 64 fixed terminal-helper caller
cases through 256 locals for `matrix.py` with unchanged normal-inlining compiler ceilings.
Keep it separate from the eleven inherited memory versions and the v0.92/v0.93 scale tests.

The v0.95 arrow layout matrix uses
`--split-test TestV950DepthThreeArrowMethodsLayouts=25`. Each shape exhausts all
128 independent condition vectors and checks normalized arrow/block HIR, exact Core/Go,
evaluator/pristine-Go results and selected-condition/arm traces.
`TestV950DepthThreeArrowMethodsMemory` exports 64 fixed-arrow caller cases through
256 locals for `matrix.py`, checking closure depth against each zero-local baseline.
Keep it in its own fresh unit, separate from inherited scale tests; normal inlining
and the 128 MiB / 5 second warm ceilings remain unchanged. The scale matrix shares
three condition bits and proves fixed-helper caller growth, not arbitrary trees.

The v0.96 straight-line initializer layout matrix uses
`--split-test TestV960DepthThreeStraightLineInitializersLayouts=25`. Each shape
crosses all subsets of three dependent local slots and used/unused final bindings.
Every case exhausts seven independent initializer bits plus an independent return bit;
initializer bits are shared across locals. Even subset masks use ordinary returns;
odd masks use depth-three returns. This bounded matrix does not claim every independent
condition or shape assignment across arbitrary-length sequences.
`TestV960DepthThreeStraightLineInitializersMemory` exports 64 cases through 256 locals
whose initializers contain the new choices directly, without a helper-call boundary.
Four fixed shapes cross used/unused final locals and 1/8/16/24/32/64/128/256 locals;
three condition bits are shared. Closure depth must not exceed each one-local baseline; multi-local statement emission
may reduce it. The last layout shape also covers five-local mixed-shape masks 21/31.
Run `matrix.py` for fresh normal-inlining compiler measurements with the unchanged
128 MiB / 5 second warm ceilings. Keep this test in its own unit, separate from
inherited memory and fixed-helper scale tests.

## Identity measurement and read-buffer reuse

Generated executable identity still hashes every current toolchain file and its
metadata once per process. The default reuses one 32-KiB read buffer during that
walk; it does not persist or skip any digest. `--no-toolchain-read-buffer` on
`pipelang_suite.py` selects the allocation-per-file control. Direct test processes
can select that control with `PIPELANG_TOOLCHAIN_READ_BUFFER=0`.

`--identity-profile` adds exclusive wall measurements for toolchain enumeration/
metadata and content reads/hashing, generated source keys, lock acquisition,
cached-binary checks and sealed copying. These leaf measurements are separate
from enclosing spans and must not be added to them. Profiling is opt-in; ordinary
native/raw execution, current oracle bytes, invalidation and sealing are unchanged.

`job.py` and `run.py` capture cumulative cgroup CPU, I/O and pressure counters at
workload boundaries. Missing files are reported as null. Difference matching
before/after counters within one cgroup; do not sum descendant counters into their
parent. Cgroup CPU excludes the outside job supervisor, and these counters do not
provide exclusive per-operation CPU attribution. Matched results and the rejected
streaming fixture prototype are recorded in
[the performance report](../../docs/research/pipelang-performance-compression.md).

## Opt-in conformance profiling

`PIPELANG_PERFORMANCE_PROFILE=1` enables `TestConformancePerformanceProfile` and logs
per-child generated build/run time and waited-child RSS in the shared generated-Go helper.
The profile uses the existing nested-layout source builders, measures analysis, HIR/Core
lowering, Core validation, ordinary/prepared evaluation, preparation and Go generation with
allocations, and reports retained preparation memory. Lowering APIs include their existing
validation; these API-boundary timings are not additive exclusive phases. The ordinary test
inventory discovers this opt-in test but skips it unless explicitly enabled. Timing is never
an acceptance assertion in ordinary tests.

Run the compiled test binary through `run.py`, for example:

```sh
python3 tests/containedexec/run.py \
  --output /tmp/compiler-proof/phases --cache /tmp/compiler-proof/cache --timeout 180 \
  -- env PIPELANG_PERFORMANCE_PROFILE=1 /tmp/compiler-proof/pipelang.test \
  -test.run '^TestConformancePerformanceProfile$' -test.v -test.timeout=170s
```

For end-to-end profiles, select an existing numbered layout and add `-test.cpuprofile` and
`-test.memprofile` with absolute temporary paths. The CPU/heap profiles cover the harness,
not compiler children. Generated build/run measurements include the Go driver, compilation,
linking and execution. Use the independent `matrix.py` direct-compiler lane for compiler-only
resource claims. Compare repeated warm runs with identical containment and private cache;
report aggregate peaks separately from allocations and retained heap. Never add concurrent
unit times and label that sum overall wall time. Nested layouts retain one sample per module:
two-method batching was rejected after measuring increased aggregate memory.

Finite conditional-local layout tests store independent result and ordered-trace oracles
in temporary per-method JSON fixtures. Generated tests verify the exact vector count and
compare every value or ordered trace. Nested layouts use at most four methods per module;
pristine and instrumented generated modules still compile and run separately. Numbered
shape/partition units, normal compiler flags, direct-compiler resource matrices and all
containment controls remain unchanged. `TestFiniteConditionalOracleTransport` checks
empty values/traces, escaping, Unicode and repeated events. Opt-in generated build/run
measurements report fixture bytes separately from compiled source and test bytes.

## Complete reruns with retained executables

`pipelang_suite.py` builds the current root test harness, discovers all tests,
fuzz seed functions and examples, and executes the full current PipeLang inventory
with at most two contained workers. Use a fresh receipt directory per run. The
first pass populates a private executable cache; the second groups up to 25
numbered shapes per unit to amortize process startup and toolchain verification:

```sh
python3 -B tests/containedexec/job.py --output /tmp/compiler-proof/populate-job -- \
  python3 -B tests/containedexec/pipelang_suite.py \
  --go /absolute/go --output /tmp/compiler-proof/populate \
  --cache /tmp/compiler-proof/cache --compiled-cache /tmp/compiler-proof/executables
python3 -B tests/containedexec/job.py --output /tmp/compiler-proof/rerun-job -- \
  python3 -B tests/containedexec/pipelang_suite.py \
  --go /absolute/go --output /tmp/compiler-proof/rerun \
  --cache /tmp/compiler-proof/cache --compiled-cache /tmp/compiler-proof/executables \
  --shape-batch-size 25 --parallel-shapes
```

This is executable reuse, never test-result reuse. The opt-in test-helper settings
are `GOENV=off`, `PIPELANG_GENERATED_BATCH=1`, and an absolute
`PIPELANG_COMPILED_CACHE` directory with mode 0700. Ordinary test invocations retain
the synchronous generated-Go path. Nonempty `GOFLAGS` retains the original synchronous harness (including coverage
and other explicit build modes).
The key binds exact generated source, oracle Go code, module/driver code, explicit
build settings, and a content fingerprint of the pinned toolchain binaries and
library source. Binary digests are checked on lookup. Linux retained batches with
multiple cases, and shared-oracle bundles, then hash and kernel-seal one immutable
executable snapshot; every fresh child executes that verified descriptor. Ordinary
single-case and other-platform batches recheck the digest before each path execution.
Invalid entries are quarantined inside the private cache and rebuilt. Per-artifact
kernel locks serialize lookup, repair, publication and execution across workers;
locks release automatically when a worker exits. Current
runtime oracle fixtures are always written and consumed afresh; fixture outcomes
are never stored in the cache. Do not edit toolchain files during a run.

Compile-only compatibility checks may also retain their compiled package; the
driver still runs and changed invalid source must fail compilation.

Eligible inert generated modules share one link in bounded batches (four packages
in the comparison path, up to 32 with native bundles; also bounded by source/fixture size). Source and check bytes stay intact
in separate packages; each original module's checks execute in a fresh native
process with a fresh cwd and current original files. A registered test cleanup
flushes queued checks before their owning test can pass. Initialization,
compiler directives, special testing entrypoints, unsupported imports and
observable testing/package names retain the original synchronous harness.

Every direct compiler resource probe still compiles and measures its current
input with normal inlining and the existing 128 MiB/5-second ceilings. Those
resource cases retain separate units. Grouped execution uses the documented
700 MiB temporary reclaim threshold under unchanged hard/proactive/swap/task
limits. The runner records build time, complete execution time, every unit,
source digests and the discovered/selected inventory; any failure or source drift
makes the run fail. Report population and complete rerun timings separately.
The executable cache and all receipt/build/fixture files are temporary artifacts,
not source-controlled package state.

The optional `--parallel-shapes` mode schedules at most four independent heavy
shape cases per unit (`PIPELANG_PARALLEL_SHAPES=1`). Shape slots remain held until
all generated-check cleanups finish. The common finite-layout helper merges
per-shape method/vector totals after completion; independent subset/layout
callbacks keep their own counters. Generated Go build/link commands are limited
to one per unit, while retained executables and reference evaluations may run
concurrently. Direct compiler resource matrices retain their sequential case
execution and the existing two-unit worker limit. Race checks and identical
named-case/vector inventories are required before accepting this mode.

### Shared native bundles

`pipelang_suite.py` uses larger native bundles across compatible generated-test
families by default when `--compiled-cache` is supplied on Linux. Finite-layout
families also share typed oracle code (see the shared framework section below).
`--no-native-bundle` selects the original path for comparisons; `--native-bundle`
explicitly selects bundles. Bundles require empty `GOFLAGS` and Linux executable
sealing. Direct compiler resource probes retain their existing paths.

Each bundle contains at most 32 generated packages, with a 512 KiB source and
32 MiB source/fixture queue threshold. A single item may cross a threshold before
the queue flushes, as with the original four-package helper. Compilation remains
serialized per unit. Generated source stays in separate packages; the common
oracle package loads current fixtures and compares actual native results with
the independent expected values and ordered traces. Its source participates in
the artifact key. Each original package still runs in a fresh child process.

The bundle is copied into a bounded memory file while checking the manifest's
binary digest. Kernel write/grow/shrink/seal locks then make that exact snapshot
immutable. Each child executes the inherited sealed descriptor, allowing reuse
without hashing a larger disk executable for every child. Failure to create or
verify the sealed snapshot fails the run; it does not weaken verification.
Transient snapshots count against the same cgroup memory ceiling and disappear
when their descriptors close. No additional persistent runtime cache is created.

The runner creates a fresh private `native-build-cache` beneath the receipt output
for bundle compiler intermediates. It records its byte count and retains the
directory for accounting; this runner does not authorize cache cleanup.
Shared-oracle bundle misses use `PIPELANG_BUNDLE_BUILD_CACHE`; ordinary bundled
checks retain the supplied Go cache. Hits do not need
that directory. Direct test-harness invocations must supply an absolute private
disposable path themselves and clean it only after their units have exited.
Existing Go compiler caches are not pruned by the runner.

For a full suite using shared native bundles:

```sh
python3 -B tests/containedexec/job.py --output /tmp/compiler-proof/bundle-rerun-job -- \
  python3 -B tests/containedexec/pipelang_suite.py \
  --go /absolute/go --output /tmp/compiler-proof/bundle-rerun \
  --cache /tmp/compiler-proof/cache --compiled-cache /tmp/compiler-proof/executables \
  --shape-batch-size 25 --parallel-shapes
```

Use a separate executable cache for baseline/candidate storage comparisons. Include
all referenced artifacts and manifests, and report compiler-cache growth separately.
`--audit-generated` forwards `PIPELANG_BUNDLE_AUDIT=1` and profiling into each
contained workload, logging generated-source and current-fixture digests plus the
original test names for exact baseline/candidate coverage comparisons. A full successful retained run writes `artifacts.json`, containing only executed keys,
manifest digests, bytes, hits and misses. Partial or failed runs do not produce a live-set
receipt. The runner never prunes retained artifacts; migration must revalidate the receipt
and current artifact bytes before removing obsolete entries. This does not establish a
1000-fold reduction.

To run only the complete 200-layout family, add
`--test-family TestV910NestedTerminalInitializersLayouts` to the command above.
The summary explicitly marks this as partial-suite proof and retains the full
discovery count separately from the selected test count. A family-only result
must not be reported as a new whole-suite runtime.

### Exact native artifact representations

`pipelang_suite.py --compiled-cache ...` uses ordinary native execution by default.
On Linux x86-64 with the installed Zstandard tool and selected library, add
`--native-representation` to opt in to compressed preparation and replay.
`--no-native-representation` explicitly selects the ordinary path. Supplying
`--representation-cache` alone does not enable compression.
`--representation-cache` chooses a private store; the default is a sibling of the
compiled cache. No test-family manifest or export is needed. Builds and discovered
tests run through `run.py` with 30-second units, 25-second children and the same
memory/process limits. Use shape batches of one for cold workloads; bounded groups
can reduce warm launch costs. Compiler memory families are partitioned by their
existing independent choices without skipping counts or changing assertions.
Cold preparation also partitions `TestV840FiniteConditionalLocalsAllShapes=25`,
and `TestV810TerminalTreeAllShapesAndPaths=25` (whose subtest prefix is `shape`),
matching their existing asserted shape inventories.

`native_artifacts.py` snapshots current cache identities and sizes and derives up
to 16 size-stratified references deterministically. Selection examines each input
once plus sorting; it performs no codec search. The Go helper computes current
source/toolchain/settings keys, then requests that key and its expected executable
digest through a private local socket. Missing entries prepare on demand using
scaffold9/token12. Newly generated keys absent from the initial snapshot prepare
without a dictionary and can participate in the next run's plan. Existing recipes
retain their immutable reference DAG, so adding a test does not invalidate every
unrelated payload. Preparation publishes complete entries under per-key locks.

Two service slots reconstruct at most two objects concurrently per contained unit.
The normal Go helper checks the received descriptor's seals and final SHA256,
then runs its current fixtures in fresh per-case processes. No expected value,
trace, pass result or generated-source whitelist is stored by this layer.
Unsupported ELF/DEFLATE forms retain the ordinary executable path. Corrupt payloads,
changed records, bad seals or digest mismatches fail; they do not silently pass by
falling back. Packed replay requires cache identity records and representation
support but does not require original executable files or exports. Source, build
settings and toolchain changes create new keys; reconstruction source/support
changes create a new preparation namespace. Old data is retained and charged.

`native-*.json` receipts distinguish preparation from reconstruction. Preparation
adds codec work and storage; a warm run avoids this preparation. Include service
startup, dictionary reconstruction, raw originals, support, cache growth, fixtures,
receipts and coexisting representations when reporting costs. No storage is freed
by this runner. `test_native_artifacts.py` covers new eligible keys, unsupported
forms, packed-only replay, corruption and invalidation; Go transport checks cover
unsealed and incorrect descriptors. All workload tests must run through `run.py`.

### Exported transcript comparison tool

`transcript_suite.py` also prepares and replays current exported bundles for
component comparisons. It derives a bounded plan from those current inputs and
reports partial-suite proof. `transcript_plan.json` is retained historical research
data and is not an admission list or an input to either execution path. Regenerate
exports with the current evaluator before an exported replay. The normal framework
integration above directly runs the current test helpers and needs no export.

The helper under `transcript/` preserves every ELF/DWARF byte. It replaces only
the compressed `.debug_line`, `.debug_loclists`, and `.debug_rnglists` bodies
with an exact DEFLATE symbol/extra-bit transcript, preserving Huffman headers,
block boundaries, padding and checksums. The remaining scaffold uses Zstandard
1.4.8 patch level 9; tokens use patch level 12, both single-threaded with a
24-bit window. No codec installation or research-directory code is required.
Python, the installed codec and an explicit offline Go toolchain are preparation
inputs. The helper and decoder library are retained with the representation.

For an existing fresh private export directory:

```sh
python3 -B tests/containedexec/transcript_suite.py prepare \
  --go /absolute/go --directory /absolute/new-representation \
  --exports /absolute/current-exports --output /absolute/new-preparation-receipts \
  --cache /absolute/private-go-cache
python3 -B tests/containedexec/transcript_suite.py run \
  --directory /absolute/new-representation --exports /absolute/current-exports \
  --output /absolute/new-replay-receipts --cache /absolute/private-go-cache
```

Both entry points dispatch exclusively through `run.py`: at most two units,
30-second workloads, 700 MiB soft reclaim, 1 GiB hard memory, zero swap,
128 tasks and proactive stop at 800 MiB. Every compiler/codec/native child has
a 25-second deadline. Preparation builds the helper once, then encodes two
objects per unit and verifies both component roundtrips. Per-object scratch
files are temporary; retained native or compiler caches are never pruned.
A failed unit stops new dispatch and retains its receipts.

Replay uses two reconstructors, four native consumers and eight total queued or
running objects per unit. Immutable dictionaries outlive every reader, including
failure paths. The service uses nonblocking request/response pipes with an
independent deadline. Scaffold, token and final executable digests are checked;
only kernel-sealed final descriptors are executed. Each original case runs in a
fresh process and fixture directory using current independent Value and ordered
Trace expectations. Changes to source, toolchain, build settings or harness
support invalidate reuse. Fixture changes reach the native oracle directly.

`--raw` runs the matched uncompressed baseline with the same current-input
validation and native case path; it requires the original executables.
`prepare --scaffold-level 12` is the previous patch12 preparation control.
Level 9 is the selected default. Timing receipts separate contained dispatch,
component encoding, native execution and accounting. Dispatch excludes the
parent pre-run cache inventory and closing report generation; measure the outer
command separately for those costs. These are not whole-suite speedups. The normal bundle path remains available for unsupported
inputs and comparisons.

`storage.json` records all representation files, support, current exports,
harness files, external toolchain/Python support, any retained original
executables and cache records, compiler-cache before/after sizes, and closing
receipt sizes. Categories are inventories: deduplicate overlapping physical
paths when comparing complete retention. Preparation/control runs and old
research roots remain separate retained costs. Original binaries are optional
for packed replay; no command deletes them. Compiled-cache migration and any
strict storage/time target require their own evidence.

Run `test_transcript.py` through `run.py`. Set `PIPELANG_TRANSCRIPT_DIRECTORY`
and `PIPELANG_TRANSCRIPT_EXPORTS` to include the prepared integration checks;
without them only the plan/source/deadline tests run. Integration checks cover
exact debug-bearing bytes, sealing, malformed spans and lengths, corrupt
root/reference/leaf payloads, source/settings/support invalidation, two-reader
lifetime and descriptor recovery, and independent current Value/Trace failures.

### Bounded shared-library experiment

`--shared-export /absolute/fresh-private-directory` exports the complete v0.91
bundle family's current generated source, checks and fixtures while executing its
normal retained baseline. It requires `--test-family
TestV910NestedTerminalInitializersLayouts` and native bundles. The destination
must already exist, be empty and have mode 0700. An existing export is never
overwritten. The flag supplies transcript preparation and experimental sharing probes; it does not change the normal suite lane.

`shared_runtime_probe.py` compares those exact bundles against Go's
`-linkshared` facilities in Linux GOPATH mode. Each original package still runs
in its own fresh process and fixture directory. Shared libraries and executables
are hash-verified into sealed memory files; the system loader resolves library
names through private links to inherited sealed descriptors. Source and build
settings are checked separately from current runtime fixtures. No pass result
is retained. `test_shared_runtime_probe.py` exercises invalidation and sealing.

The prototype expects a private evidence root containing `settings.json`,
`compiler-closure.json`, `std-pkg`, and `gopath/src/pipelang-generated-check/oracle`.
Prepare the oracle from an exported `oracle/oracle.go`. With the pinned offline
toolchain, private `GOPATH`/`GOBIN`, `GO111MODULE=off`, and `GOENV=off`, build
`go install -p=1 -buildmode=shared -pkgdir=<root>/std-pkg std`, then
`go install -p=1 -linkshared -buildmode=shared -pkgdir=<root>/std-pkg ./oracle`
from the private generated-check directory. Run both through `run.py`, using the
existing bootstrap allowance only for the first command. Module-mode sharing
does not support this prototype. No source under the installed toolchain is edited.

`shared_runtime_settings.py --root <root> --go <absolute-go> --output
<root>/settings.json` records exact Go and selected GCC support bytes after
preparation. `compiler-closure.json` has the same `toolchain` map shape for any
additional audited compiler inputs (an empty map when the settings snapshot is
complete). Build with `shared_runtime_probe.py build --root <root> --exports
<exports> --go <absolute-go> --start N --stop M --receipt <fresh-json>`, serially
in small fresh contained units. Replay with `run`, `--lane baseline|candidate`
and a freshly regenerated export. Both lanes check the same source identity.
Use at most two units, each with at most four shape workers. Every native child
retains the 25-second test and 30-second process deadline.

This measures the native replay phase of the full family; evaluator/oracle
generation, root harness build and compressed reconstruction are separate costs.
Count all program binaries, both libraries, manifests, source/fixtures, package
archives and compiler/reconstruction support, including coexistence with any
compressed representation. Go 1.25.13 forces `-w` with `-linkshared`; the explicit `-w=false` probe fails
with `dwarf: missing type (no data): type:unsafe.Pointer`. The prototype therefore
cannot replace the debug-bearing baseline binaries, which must remain retained
and counted. Shared-library support and compact storage are experimental results,
not whole-suite target claims. The TASK-021 performance
record links the measured evidence and limitations.

### Shared native test framework

Retained-cache Linux runs use `--native-bundle` by default for every compatible
`compileAndRunGeneratedGoFiles[WithFixtures]` family. The framework links up to
32 inert generated packages into one executable and runs each original package
in a fresh contained child with its own current fixture directory. It flushes at
512 KiB accumulated source/check bytes or 32 MiB combined source/fixture bytes;
a single large case retains its original checks. Queues stay within their owning
Go test lifetime, so independent compiler-resource probes keep their boundaries.
`--no-native-bundle` retains the four-package ordinary comparison path (8 MiB
combined threshold). It does not disable retained executable reuse.

Finite conditional-layout families v0.84, v0.85, v0.86, v0.87, v0.89 and v0.91
also share the existing fixture loader and Value/Trace comparison package.
Independent tree models still produce the expected data on every run. Other
families retain their existing inline assertions. Neither path caches outcomes.
Executable keys bind current generated source, checks, shared support, toolchain
and settings; changed runtime fixtures run against cached binaries immediately.
Debug information and per-case generated package namespaces are retained.

Standalone exceptions remain intentional: explicit Go flags, custom init or
TestMain, non-inert global initialization, compiler directives, examples/fuzzing,
unsupported imports, extra Go fixtures, and checks that observe testing names or
package identity. Tests with distinct owning subtests and compiler-resource
probes are not pooled across their lifetimes. Larger inline-check families can
hit the source threshold before reaching 32 packages. These boundaries preserve
existing behavior and compiler evidence; they do not remove any tests. All
framework code lives in compiler-owned test helpers; engine behavior is unchanged.

### v0.97 terminal initializer placement

Split `TestV970DepthThreeTerminalInitializersLayouts=100` (four disjoint sample
partitions per statement shape, preserving every case). All 25 statement shapes
rotate all 25 depth-three initializer shapes across finite mixed/dependent locals.
Every local-slot subset crosses used/unused final bindings; the final shape adds
all root/intermediate/leaf scope subsets and two five-local mixed layouts. Exhaust
128 initializer vectors and both return bits for one representative of each
reachable statement path. The seven statement bits, seven shared initializer bits
and return bit are independent; this is not every independent initializer shape
and condition assignment at arbitrary sequence length. Current value/trace fixtures
are regenerated from the independent tree model and run in pristine/instrumented Go.

The new memory family has twelve `choices` partitions: four direct initializer
shapes at root, intermediate and depth-three leaf placements, each crossing used/
unused final locals and 1/8/16/24/32/64/128/256 locals (192 fixtures). Export
`TestV970DepthThreeTerminalInitializersMemory` with `PIPELANG_MEMORY_FIXTURES`;
run `matrix.py` for independent fresh compiler units. Retain normal inlining and
128 MiB / 5 second warm compiler ceilings. Other statement branches return the
original input; the scale count describes the one growing sequence.

The v0.98 conditional boolean selector suite verifies the eight independent selector
vectors, mixed ordinary/depth-three preceding locals, used/unused bindings, computed
conditions, types/carriers, malformed Core, source refusals and all earlier versions.
Its memory test has four initializer families (ordinary and depth one/two/three),
used/unused tails and 0/1/8/16/32/64/128/256 actual preceding locals. The return itself
contains the new selector; no helper hides its placement. The normal suite partitions
four `choices` subtests and exports 64 fixtures to `fixtures-v098`. Fresh `matrix.py`
measurements retain normal inlining and 128 MiB / 5-second direct compiler ceilings.
The bounded matrix uses three shared input bits; it does not claim all independent
initializer assignments at arbitrary sequence lengths.

The v0.99 terminal-leaf boolean selector matrices enumerate all 25 statement shapes.
Use `--split-test TestV990TerminalLeafBooleanSelectorsSubsets=100` and
`--split-test TestV990TerminalLeafBooleanSelectorsScopeLayouts=25`. Subset shapes use four disjoint subset partitions each. Modules
contain at most eight methods; every leaf subset exhausts 64 routing/selector vectors.
Scope layouts cross one/three locals and used/unused final bindings at each scope,
with 512 routing/selector/initializer vectors. Bits are independent between these
three groups but shared by statement depth and across local initializers.
`TestV990TerminalLeafBooleanSelectorsMemory` crosses root/intermediate/leaf placement,
four initializer depths, used/unused tails and 0/1/8/16/32/64/128/256 actual locals
(192 cases). Export `fixtures-v099`, then use `matrix.py` for fresh isolated compiler
proof under unchanged normal-inlining 128 MiB / 5-second ceilings.

The v0.100 arrow-selector memory family has four `choices` partitions. Each
crosses used/unused final bindings and 0/1/8/16/32/64/128/256 actual caller locals
(64 cases). The fixed arrow helper uses one flat boolean selector and inherited
nonconditional value arms. Every case exercises all eight selector vectors;
caller invocations share those input bits. Export `fixtures-v100`, then run
`matrix.py` for fresh isolated compiler proof at the unchanged normal-inlining
128 MiB / 5-second ceilings. The generic internal Core representation and already
admitted v0.98/v0.99 block Core must remain valid when old source rejects arrow syntax.

The v0.101 straight-line selector-initializer layouts use 24 asserted cases.
Use `--split-test TestV1010StraightLineBooleanSelectorInitializersLayouts=24`.
Each layout crosses 512 independent vectors for two initializer selector triples
and the return/depth-three triple, with all nonempty two-slot subsets, mixed four-local
sequences and used/unused tails. Scaling exports actual selector initializers to
`fixtures-v101`; each family shares input bits across the scaled local sequence.
Run `matrix.py` for fresh isolated compiler evidence at unchanged limits.

### v0.102 selector initializers in terminal scopes

Split `TestV1020TerminalBooleanSelectorInitializersLayouts=200`: eight disjoint
layout partitions per each of the 25 terminal statement shapes. The bounded matrix
crosses all three local-slot subsets with used/unused tails, rotates ordinary,
depth-three and flat-selector returns, and adds scope-class subsets and five-local
mixed sequences. Each supplied layout covers every reachable statement path,
64 independently supplied two-initializer-triple vectors and eight return vectors.
Selector triples alternate across locals and are shared between scopes. Mixed inherited
depth-three initializers share this supplied input group; this is not arbitrary
independent assignments at every initializer and tree node.

`TestV1020TerminalBooleanSelectorInitializersMemory` has twelve `choices` partitions:
four nonconditional arm families at root/intermediate/depth-three leaf placement,
used/unused tails and 0/1/8/16/24/32/64/128/256 locals (216 fixtures). Zero-local cases
are inherited controls; 192 cases contain actual new initializers. Routing and selector
bits are shared only in this bounded scaling matrix. Export `fixtures-v102`, then
run `matrix.py` for independent fresh compiler measurements with normal inlining and
unchanged 128 MiB / 5-second ceilings. Preserve all older receipts and caches.


The v0.103 direct terminal conditional-test layout inventory is partitioned with
`TestV1030TerminalConditionalTestsLayouts=200`. It covers all 25 statement shapes
and every subset of their test positions, including zero-new-test inherited controls.
Each layout supplies three independent selector triples (one per statement depth,
shared between siblings) and a fourth triple shared by local and return expressions.
It does not claim arbitrary independent values at every node. Scaling adds twelve
families at three conditional-test depths through 256 eager preceding locals, with
used/unused tails and four initializer-arm families. Every scaling case contains a
new test, including zero-local controls. Limits and normal compiler flags are unchanged.


The v0.104 nested terminal test-arm matrix uses
`TestV1040NestedTerminalConditionalTestsLayouts=200`. All 25 statement shapes and
all 722 subsets of condition positions are supplied. True-only, false-only and
both-arm nesting rotate by node and layout. Each layout crosses 128 shared operand
vectors, eight independent statement-depth result flips, and eight independent
local/return vectors (8192 supplied rows). Nodes share operand inputs; this is not
all independent assignments at every node. Non-new positions mix ordinary and flat
v0.103 tests. The independent model checks values and lazy ordered Go traces.
Scaling has 36 choices partitions: three nesting forms, three test depths and four
initializer families, with used/unused tails and 0/1/8/16/24/32/64/128/256 locals.
Export `fixtures-v104`, then use `matrix.py` for fresh isolated compiler measurements
with unchanged normal inlining, execution GC and 128 MiB / 5-second ceilings.


The v0.105 depth-three terminal test matrix uses
`--split-test TestV1050DepthThreeTerminalConditionalTestsLayouts=200` and
`--split-test TestV1050DepthThreeTerminalConditionalTestsExpressionShapes=75`.
The first covers all 25 statement shapes and all condition-position subsets with
21 depth-three test shapes rotated across nodes and seven shared operand bits,
three independent statement-depth flips and three local/return bits. The second
crosses all 25 expression shapes through depth three with all three statement depths,
using independent expression operand bits and separately checked lazy traces.
These are bounded matrices, not all independent assignments across statement nodes.
The 36 memory families cross three nesting forms, three statement depths and four
initializer families with used/unused locals and 0/1/8/16/24/32/64/128/256 bindings.
The suite exports `fixtures-v105` for fresh isolated `matrix.py` measurements.
Existing normal inlining, execution GC, compiler ceilings and containment remain fixed.

The v0.106 terminal boolean-selector placement matrix uses
`TestV1060TerminalBooleanSelectorTestsLayouts=200` disjoint partitions over all 25
statement shapes and 722 condition-position subsets. Each layout exhausts 2,048
vectors for five selector operands, three statement-depth flips and three local/
return operands. Operand bits are shared across statement nodes; this does not
claim every independent assignment across nodes. Separate computed/lexical checks
exhaust five independent booleans at all three statement depths. Twelve memory
families cross three depths and four initializer operand families with used/unused
locals and 0/1/8/16/24/32/64/128/256 preceding bindings, exporting 216 fixtures to
`fixtures-v106`. Keep normal inlining and the 128 MiB / 5-second compiler ceilings.

The v0.107 terminal selector value-arm matrix uses
`TestV1070TerminalSelectorValueArmsLayouts=600` disjoint partitions over 25 statement
shapes, 722 condition-position subsets and three arm families (true, false, both).
Each layout exhausts 4,096 supplied vectors: nine selector/result-arm operands and
three independent statement-depth flips. Local/return choices reuse some of those
bits; selector bits are shared across statement nodes. This does not claim every
independent assignment across nodes or locals. Separate computed/lexical checks
exercise eager ordered locals and lazy pure calls.
The 36 scaling families cover three statement depths, four initializer-arm families
and three new test-arm families, each with used/unused tails and nine local counts.
Export fresh isolated compiler fixtures to `fixtures-v107`. Preserve normal GC,
inlining, all contained limits and the 128 MiB / 5-second compiler ceilings.

The v0.108 terminal inner-selector arm matrix uses
`TestV1080TerminalInnerSelectorArmsLayouts=600` disjoint partitions over all 25
statement shapes and all 722 condition-position subsets for each of three arm
families. Every layout exhausts 4,096 supplied vectors (nine independent boolean
operands and three statement-depth flips); selector bits are shared across nodes,
and local/return choices reuse bits. This is not all independent cross-node or
local assignments. Inherited v0.107 outer-arm tests mix at unselected positions.
The memory family retains 36 groups and 648 cases; export `fixtures-v108` for
fresh isolated compiler measurement with unchanged 128 MiB / 5-second ceilings.

The v0.109 combined selector-arm proof partitions all nine arm pairs and all
statement shapes: `TestV1090TerminalCombinedSelectorArmsLayouts=1800`.
Scaling spans 108 families (nine arm pairs, three depths, four initializer families).
Layout vectors have nine independent boolean inputs and three independent depth
flips; four of thirteen operand positions reuse inputs, as do locals and returns.
A separate root-test matrix exhausts all thirteen operand inputs independently:
`TestV1090TerminalCombinedSelectorArmsIndependentOperands=9`.

The v0.110 straight-line selector result matrix uses
`--split-test TestV1100StraightLineSelectorValueArmsLayouts=18`.
Three arm forms cross 0/1/4 preceding locals and used/unused tails, exhausting five
independent selector bits and three independent initializer bits (shared across locals).
`TestV1100StraightLineSelectorValueArmsMemory` has twelve shape families: three arm
forms times four initializer depths, each with used/unused tails and 0/1/8/16/32/64/128/256
locals. Its 192 cases reuse three input bits and retain the 128 MiB / 5-second ceilings.

The v0.110 typed and carrier matrices use
`--split-test TestV1100StraightLineSelectorValueArmsTypes=6` and
`--split-test TestV1100StraightLineSelectorValueArmsCarriers=6`.
Each tests all three arm forms with and without preceding locals.

### v0.111 terminal-leaf selector result arms

TestV1110TerminalLeafSelectorValueArmsTypes=6
TestV1110TerminalLeafSelectorValueArmsCarriers=6
TestV1110TerminalLeafSelectorValueArmsLayouts=18
TestV1110TerminalLeafSelectorValueArmsSubsets=2400
TestV1110TerminalLeafSelectorValueArmsScopeLayouts=75

The subset matrix crosses all 25 terminal-tree shapes, every leaf subset, and
three result-arm forms with independent routing and selector/result bits (256
vectors per subset). Scope layouts use 1/3 locals and used/unused tails; initializer
bits share routing bits there. The separate 18 leaf-local layouts exhaust eight
independent selector/result/initializer bits. These are bounded matrices, not an
arbitrary Cartesian operand claim. Thirty-six scaling families cross three local
scope placements, three arm forms and four initializer depths, used/unused tails
and 0/1/8/16/32/64/128/256 locals: 576 fresh isolated compiler fixtures.

Full-suite reconciliation validates each complete sealed receipt and its artifacts
before retaining only case identity and supersession metadata. This avoids holding
a second full payload set beside the suite report under the unchanged coordinator
cap. Execution/resumption and compiler-fixture discovery still obtain full receipts;
compact results cannot substitute for those payloads.


v0.112 arrow selector result arms retain full inherited coverage and add independent
value/order checks with `TestV1120ArrowSelectorValueArmsLayouts=18`,
`TestV1120ArrowSelectorValueArmsTypes=3` and
`TestV1120ArrowSelectorValueArmsCarriers=3`. The 12 scaling families cross three
arm forms and four caller-initializer depths, used/unused tails and
0/1/8/16/32/64/128/256 locals: 192 additional fresh isolated compiler fixtures.
Arrow and equivalent block HIR/Core/semantic/Go are compared after removing only
source spans/fingerprints; source spelling refusal stays exact through v0.111.0.


v0.113 nominal enums add source/HIR/Core/evaluator/native ownership, stable tags,
exhaustive matching, lazy traces, invalid host/Core refusal and branch-local checks.
`TestV1130EnumInheritedTypes=3` and `TestV1130EnumInheritedCarriers=3` retain inherited
scalar/carrier forms under the new contract. `TestV1130EnumsMemory` partitions three
member counts (2/8/32), each crossing 0/1/8/32/128/256 locals: 18 additional fresh
isolated compiler fixtures, bringing the maintained terminal matrix to 2922.
Every member and both local-selector outcomes have independent evaluator/native
oracles. No direct compiler, process, GC, inlining or execution limit changes.
