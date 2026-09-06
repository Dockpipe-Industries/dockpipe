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
