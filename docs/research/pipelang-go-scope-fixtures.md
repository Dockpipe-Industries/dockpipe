# PipeLang Go scope-layout fixture reduction

Bounded v0.111 scope-layout construction experiment, 2026-09-13, on saved
`js/pipelang`. This follows the [subset fixture experiment](pipelang-go-artifact-reduction.md).
The compiler, evaluator, Core-only Go backend and accepted v0.113.0 are unchanged.

## Construction and comparison

The scope-layout native-bundle path now uses the existing exact length-prefixed
Value/Trace fixture reader instead of expanded literal Go assertions. The ordinary
comparison path remains literal. Independent expected results and traces, current
evaluator calls, input mapping, generated programs and owning lifetimes are unchanged.

The existing 512-KiB source threshold can flush a scope subtest more than once.
Compact assertions may avoid those flushes without pooling across subtests or
raising any source, fixture or package-count ceiling. This is a measured property
of the selected corpus, not a promise about all verification families.

The frozen sample selects shapes 0/4/14/24 plus each 25-shape arm offset: all
three arm forms, four tree sizes, local counts 1/3 and unused false/true. Per pass
it checks 48 source layouts, 12,288 independent vectors and 96 fresh generated
native package executions. It covers twelve of 75 owning scope subtests.

The control is the admitted source with identical optional oracle audit logging
added through a Go overlay. A structural check proves that the overlay changes
only that logging. Both lanes record exact ordered input/value/trace digests per
layout and generated-source multisets. The candidate binary builds directly from
the retained source. Cold population and three fresh warm replays are compared;
the second warm replay reverses lane order. All runs use the same toolchain,
shared prepared native build cache, instrumentation and unchanged resource limits.

## Evidence and acceptance

The matched construction passes completed with these results:

| Twelve selected owning subtests | Literal control | Compact fixtures |
| --- | ---: | ---: |
| Distinct executable identities | 18 | 12 |
| Executable bytes including normal debug/runtime support | 100,032,017 | 66,181,484 |
| Source layouts / independent oracle vectors per pass | 48 / 12,288 | 48 / 12,288 |
| Fresh generated native package executions per pass | 96 | 96 |
| Warm misses in each of three passes | 0 | 0 |
| Mean summed warm unit time | 8.315 s | 7.712 s |

The sample avoids **33,850,533 executable bytes (33.8397%)** and six identities
(33.3333%). Every sampled shape shrinks. The six larger tree cases each move from
two binaries to one under the unchanged source threshold. The other six retain
one binary. This is construction within each existing owner, with no cross-owner
pooling. It does not establish savings over all 75 shapes or the whole language.

Warm unit sums were 8.617/8.096/8.233 seconds for control and 7.687/7.653/7.796
for the candidate: 7.26% lower mean time in this instrumented sample. These are
summed contained workloads, including validation, not complete campaign or pure
computation times. Host load is uncontrolled. Cold sums were 37.908 versus 32.966
seconds; test-harness bootstrap, native dependency preparation and controller
storage accounting are outside those sums. No full-campaign speedup is claimed.

Fresh native child execution averaged 0.479 s for control and 0.488 s for the
candidate. The smaller artifacts therefore do not establish faster computation.

All twelve focused checks passed: exact binary encoding, malformed fixtures,
current values/traces on cache hits, fresh process state, source/support keys,
sealed snapshots, corrupt preparation, special harness refusal, ordinary fixture
support and actual literal scope fallback, plus unchanged subset/v0.109 controls.
All 30 measured executable identities retain normal debug sections. The complete
validation job took 251.732 seconds with a 1,058,230,272-byte aggregate peak,
zero OOM/swap/max events and its cgroup tree removed. No failed attempt occurred.

The closing declared estate was 35,721,311,894 logical / 37,606,289,408 allocated
file bytes. Sampled logical/allocated peaks were 35,747,917,667 / 37,634,818,048
bytes, under the unchanged 96-GiB cap and 8-GiB free-space reserve. The estate
includes current task outputs, previous experiment evidence, shared Go build cache,
source/package inputs and module/toolchain support. It grew about 569 MB during
validation; about 378 MB is retained in this task's artifacts, with additional
shared cache growth. **Zero existing bytes were freed.** Later small documentation
and final acceptance records are outside the closing job sample.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a0996c-60eb-7e62-9263-8552b3556735/go-scope-fixtures/`

`validate.py` freezes invocation and comparison logic; `source-review.json`
records control-overlay and unchanged-function checks. `validation-receipt.json`
owns matched measurements and focused checks, `validation-job.json` owns aggregate
containment and tree removal, and `final-acceptance.json` records current source,
documentation and protected-state admission. The
[objective record](../agents/tasks/pipelang-reactive-application-language/go-scope-fixtures.md)
owns scope, authority and completion state.

No full-language campaign, full-family extrapolation, production behavior change,
cleanup, installation, limit increase or proposed-budget adoption is included.
Existing caches and evidence remain preserved; executable bytes avoided in newly
constructed samples must not be reported as existing bytes freed. Declared estate
peaks are sampled metadata rather than allocator-exact or physical-device proof.
