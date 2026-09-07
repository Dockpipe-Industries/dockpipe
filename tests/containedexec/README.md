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
