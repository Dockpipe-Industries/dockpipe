# PipeLang C++, assembly and Qt pilot

Date: 2026-09-13. Objective: `pipelang-cpp-qt-pilot-20260913`.
Disposition: retain an experimental integration backend; no default-backend or
full-language promotion. The accepted executable language remains v0.113.0 with Go.

## Result and implementation

A Go-implemented, Core-only C++17 emitter now produces native code for the bounded
pilot. The same compiler emits assembly, which assembles, links and returns the same
values. A real Qt 6.2.4 Widgets window invokes generated PipeLang arithmetic through
a QObject bridge. Button events, a property, queued result signals, overflow and
invalid-input presentation, the visible result `42`, and ownership cleanup pass on
the actual xcb desktop. This demonstrates a useful native Qt integration path.

The emitter and its embedded support are in
[`cppbackend`](../../src/lib/pipelang/cppbackend/generate.go). The independent corpus
export is [`cpp_pilot_test.go`](../../src/lib/pipelang/cpp_pilot_test.go); the
[contained native harness and Qt adapter](../../tests/pipelang-native-pilot/) remain
outside Core. No parser, typechecker, HIR, Core, existing Go backend, engine command,
package semantics or public CLI behavior changed. No direct machine-code emitter,
compiler rewrite, Qt generator, Application IR port or supported public C++ ABI is
claimed. The Qt adapter is handwritten experimental host code.

Backend capability `pipelang.cpp.pilot.v1` validates the bounded graph before Core
validation, refuses unsupported contracts/types/operations and missing/duplicate
function identities, and uses deterministic names from semantic identities. Values
use owned C++ strings/vectors/records; checked integer operations avoid signed
arithmetic overflow. UTF-8 is validated, text comparison uses unsigned bytes,
Result/Optional carriers require canonical inactive fields, and statement
temporaries preserve evaluation order and lazy branches. The support uses GCC
checked-overflow builtins: this is tested Linux/GCC output, not cross-platform proof.

## Exact coverage

Eight programs contain 20 independently specified vectors each. Expected integer
arithmetic uses an independent big-integer oracle. Every valid vector passes the
current frontend, HIR, Core and evaluator before native comparison. Test-only call
instrumentation supplies independent order/laziness checks in Go and C++; ordinary
application variants contain no trace hooks.

| Cases | Scope | Actual source/Core language contract |
| --- | --- | --- |
| N1, N2 | Checked signed-64 add, subtract, multiply, negate; helper calls and boundaries | v0.113 |
| C1, C2 | Lazy Boolean control, immutable selection, conditional checked calls | v0.113 |
| A1 | Flat record construction, projection/equality and text order | v0.12 |
| A2 | Owned record-list construction, append and count | v0.17 |
| A3 | Indexed lookup and Optional record results | v0.20 |
| A4 | Record-list filtering | v0.22 |

Allocating vectors include lengths 0/1/16/256, empty and long Unicode strings,
embedded NUL, combining sequences and supplementary scalars. Older contracts are
preserved exactly; these are not eight programs claimed to compose under v0.113.
The broader emitter capability is experimental. Operations outside these cases
have no comprehensive backend acceptance from this sample.

The planning proposal's integer division is not accepted source behavior: accepted
division is binary64, which the pilot excludes. Integer division is a refusal case.
An earlier zero-argument record/list source exposed an existing frontend panic at
`typecheck.go`'s `resolvedParameters[1:]`. The failed preparation output is retained;
this objective did not repair the frontend or widen the language contract. That
finding belongs in subsequent frontend work.

## Proof and recovery

The frozen export passes 160 vectors and 16 malformed/unsupported Core cases. Native
runs include ordinary Go and C++ applications, instrumented Go and C++ verification
applications, and an application assembled from compiler-generated assembly. Each
campaign checks 8,000 value outputs (400 fresh children, 20 vectors each), with exact
traces in the two verification variants. Sixteen malformed host fixtures produce
80 refusals across the five variants, including noncanonical Boolean values, counts,
truncation, trailing bytes and invalid UTF-8.

`native-proof-1` completed all 49 stages freshly. The recovery campaign deliberately
exited with status 19 after 42 stages; its process tree was removed. Resumption
regenerated the corpus from current source, checked exact producer output against
the frozen corpus, revalidated source/toolchain/fixture/settings and saved artifacts,
reused 42 stages, and executed seven remaining stages. All 49 were accepted.

Execution hashes current complete executable bytes, copies them into a private
Linux memfd, checks write/grow/shrink/seal restrictions, executes that descriptor,
then closes it. Live input watches complement content hashes so mutation followed
by restoration cannot silently reuse evidence. Independent acceptance reads raw
receipt envelopes, rehashes retained artifacts, compares raw outputs and fixtures,
and checks resource cleanup separately from worker assertions.

## Measurements and their limits

Pinned toolchain: Go 1.25.13, GCC 11.4, C++17, Linux x86-64 SysV baseline,
`-O2 -g -march=x86-64 -mtune=generic`; no stripping, LTO or PGO. Go and C++ both build
plain application mains. These are not Go test executables compared against C++
application mains. Go's test adapter uses reflection while the C++ adapter is typed;
formatting, input decoding and adapter costs remain part of these measurements.

Each original application/verification variant ran ten times in alternating order.
The warm compile/link observations are one build per variant, not a repeated clean
compiler benchmark. The broader proposal's three private-cache clean repetitions
and five incremental pairs were not performed. They remain prerequisites to a
performance adoption claim; Qt integration value is the reason to retain this pilot.

| Metric, original eight-program sample | Go application | C++ application |
| --- | --- | --- |
| Executable including debug information, per program | 2.388–2.399 MB | 0.107–0.302 MB |
| Executable plus linked runtime, per program | 2.388–2.399 MB | 5.895–6.089 MB |
| Observed warm compile/link, per program | 0.164–0.214 s | 0.414–0.865 s |
| Median admission + fresh execution, 20 vectors | 12.37–15.16 ms | 2.09–3.26 ms |
| Median sampled process max RSS, per program | 1,824–5,184 KiB | 3,560 KiB |

The short-process wall timer includes input preparation, executable hashing,
InputGuard setup, copying to a sealed descriptor, process startup and result I/O.
Smaller binaries reduce that admission cost. The timing difference is **not** a
measurement of pure generated-code speed. `/usr/bin/time` process CPU readings are
also too coarsely resolved to support that claim for these tiny programs. Larger
batches repeat the same 20 vectors 50 times without changing generated code; those
measurements still include decoding, formatting, admission and startup.

C++ fails the proposed standalone inclusive-size gate, and the observed build
samples are slower. Several cases also use more process RSS. Sharing C++ runtime
libraries across the eight applications gives a 7,350,136-byte union versus
19,142,018 bytes for Go, excluding source/object/support output. Those retained
extras are 193,384 bytes for C++ applications and 164,281 bytes for Go; the assembly
lane adds 10,801,622 bytes of retained supporting artifacts. The union result does
not undo the standalone result. Assembly output demonstrates inspectability and
semantic equivalence; it adds no demonstrated independent speed benefit.

The sixteen source-only compiler probes (eight per backend) all passed 5 s and
128 MiB. Maximum observed wall/RSS were 0.0157 s / 20,184 KiB for Go and
0.2141 s / 60,392 KiB for C++. These compile emitted source separately from the
host adapter; full linking, test-exporter building and Qt have their own unit
receipts and must not be described as 128-MiB compiler probes.

The 1,000-vector batches ran ten fresh Go/C++ pairs per program, checking another
160,000 exact values. Median admission-plus-batch time ranged 14.45–140.53 ms for
Go and 2.51–17.70 ms for C++. This is an application-adapter observation with the
same limitations above, not a compute-kernel speedup. Wrong executable digest,
attempted sealed write, descriptor cleanup, mutation/restoration, wrong-value and
wrong-trace controls all passed.

## Resources, artifacts and remaining work

All accepted compiler/native/Qt work uses the existing contained lane: aggregate
2 GiB, 1536 MiB high, coordinator 512 MiB, unit 1 GiB/700 MiB high, and zero swap.
The complete declared estate remains below 96 GiB with an 8 GiB reserve. Debug
sections, generated code, objects, runtime identities, fixtures, raw outputs,
failed preparations and all campaign generations are retained. Nothing was freed.
Installed tool/runtime dependencies are accounted separately from new pilot output.
Preparation also left two ignored Python bytecode files in
`tests/pipelang-native-pilot/__pycache__/`; they are retained and included in the
final support accounting extension.
The evidence tree snapshot retains about 412 MB logical / 428 MB allocated, well
inside the 2-GiB pilot output target; it includes unsuccessful attempts. Shared
caches are counted in the broader estate, not omitted from totals.
Metadata peaks are sampled file sizes, not exact temporary or physical-device peaks.

The first native admission attempt hit coordinator hard-limit pressure while
hashing unrelated installed libraries and was stopped. The retained next attempt
stopped before execution to introduce sealed descriptors. Neither is accepted
performance evidence. Dependency discovery now identifies the needed headers,
compiler/linker tools, startup files and Qt inputs, with directory watches for new
shadowing inputs. Existing resource limits were preserved.

The accepted original native job took 691.667 s end to end, while its 49 unit
workloads totaled 24.698 s. Much of the difference is estate/identity accounting;
it must not be attributed to C++ compilation or program execution. The Qt job took
67.779 s and peaked at 839,405,568 aggregate bytes. The original native job peaked
at 409,112,576 aggregate bytes. Both removed their process trees without hard-limit,
OOM or swap events.

The independent audit accepted 1,628 retained artifact hashes per native campaign
and 15,543 current source/toolchain/fixture identities. All 26 existing synthetic
receipt/verification tests passed. Their first invocation inherited the enclosing
job identity, causing two simulated-crash recovery refusals; the corrected fixture
invocation removes that metadata while retaining actual process containment.
Passed compiler, control and batch units were not rerun to hide that failed unit.

The Qt rerun observed 159 mapped/linked files totaling 227,485,428 bytes. This
includes graphics and desktop integration dependencies as well as Qt. It exposed
91 existing files (150,831,396 bytes) beyond the earlier compiler/linker estate.
`qt-accounting-extension.json` records that observation.
`final-footprint-extension.json` adds those files and authored harness/documentation
support outside the original compiler estate: 37,424,844,860 logical bytes at
that snapshot, below 96 GiB. The corresponding sampled logical peak plus the
extension is 37,441,663,401 bytes, before final documentation/receipt writes. This is an
observed-runtime accounting extension, not proof of a portable Qt deployment
manifest or exhaustive file accesses. Runtime libraries, plugins, fonts and
platform packaging still need a deployment-specific closure before distribution.

Recommended next use of these findings: define the Qt host boundary around owned
values, error conversion, object lifetime and GUI-thread delivery, then resume the
language/application foundation work. Keep compiler-generated assembly available
for diagnosis. A full C++ backend or performance-based adoption needs broader
language coverage, repeated clean/incremental comparisons and packaging proof.

Local evidence root:
`/home/jamie/.codex/visualizations/2026/09/13/01a09c2e-b445-7322-8a02-1d2328313bec/cpp-qt-pilot/`.
Primary records: `prepare-7`, `native-proof-1`, `recovery`, their job receipts,
`qt-proof-1/qt-pilot.png`, `qt-proof-1/acceptance.json`, `measurements.json`, and the
`supplement-1`, `independent-acceptance.json`, `acceptance-job-work`,
`qt-closure-1/closure.json`, `qt-accounting-extension.json`,
`final-footprint-extension.json` and
`final-acceptance.json`. Source changes remain local and
uncommitted. No install, worktree, cache deletion, commit, push or publication ran.
