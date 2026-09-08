# Contained generated-Go validation

Generated PipeLang and Application IR tests require Linux cgroup v2 and a user
systemd manager. There is no uncontained fallback. Other platforms currently
refuse generated compilation; implementing and verifying an equivalent process-tree
strategy is required before claiming resource proof there.

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
python3 tests/containedexec/matrix.py \
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
python3 tests/containedexec/pipelang_suite.py \
  --go /absolute/go --output /tmp/compiler-proof/populate \
  --cache /tmp/compiler-proof/cache --compiled-cache /tmp/compiler-proof/executables
python3 tests/containedexec/pipelang_suite.py \
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

Eligible inert generated modules share one link in batches of at most four
packages (also bounded by source/fixture size). Source and check bytes stay intact
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

`pipelang_suite.py` uses shared typed oracle code and larger native bundles for the
v0.91 finite-layout helper by default when `--compiled-cache` is supplied on Linux.
`--no-native-bundle` selects the original path for comparisons; `--native-bundle`
explicitly selects bundles. Bundles require empty `GOFLAGS` and Linux executable sealing. Other language
families and direct compiler resource probes retain their existing paths.

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
Bundle misses use `PIPELANG_BUNDLE_BUILD_CACHE` for compilation; hits do not need
that directory. Direct test-harness invocations must supply an absolute private
disposable path themselves and clean it only after their units have exited.
Existing Go compiler caches are not pruned by the runner.

For a full suite with this family using shared native bundles:

```sh
python3 tests/containedexec/pipelang_suite.py \
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
