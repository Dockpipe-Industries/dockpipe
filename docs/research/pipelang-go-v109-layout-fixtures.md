# Go v0.109 layout fixture construction

Completed bounded experiment, 2026-09-13, on saved `js/pipelang`, admitted at
`6c79c45da6f382799a1ed23c06790c86cec47bc7`.

## Construction and scope

Eligible v0.109 layout native bundles now use the existing exact length-prefixed
Value/Trace fixtures and shared binary reader. Previously each generated package
linked a JSON fixture loader and carried serialized Boolean flags. The candidate
reconstructs flags with the same `bits & (1 << bit) != 0` mapping and ordered
4,096-vector inventory used by the independent oracle. Expected values and traces
are still computed afresh. Both native checks retain their assertions; the trace
check still compares the returned value as well as the ordered trace.

JSON remains the ordinary fallback and the independent-operands suite's format.
Source generation, independent expected-result logic, current evaluator calls,
HIR/Core determinism, generated programs, owner lifetimes, package/fixture/source
ceilings, compiler and accepted v0.113.0 are preserved. No cross-owner pooling,
validation-read optimization or language behavior change is included.

The frozen sample selects partitions 1/9/17/25/33/41/49/57/65 (all nine arm
combinations in the smallest tree), plus 289/1041/1793 (trees 4/14/24 with families
1/5/9). Each owner retains all assigned subsets and vectors: 35 source layouts,
143,360 independent vectors and 70 fresh native package executions per pass.
This is 12/1,800 layout owners, not a full-family or full-language campaign.

The control overlay differs from admitted source only by optional ordered
input/value/trace audit logging, verified structurally. The candidate builds from
retained source. Both lanes use the same cached Go 1.25.13, native dependency
preparation, instrumentation and resource limits. Cold population and three fresh
warm passes match exact generated-source multisets, ordered oracle digests,
subset inventories and native execution counts. Warm pass two reverses lane order.

## Measured result

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Distinct executable identities | 13 | 13 |
| Executable bytes including normal debug/runtime support | 67,651,272 | 61,975,186 |
| Source layouts / oracle vectors per pass | 35 / 143,360 | 35 / 143,360 |
| Fresh native package executions per pass | 70 | 70 |
| Misses in each of three warm passes | 0 | 0 |
| Mean summed warm unit time | 15.010 s | 13.274 s |
| Mean summed warm native child time | 2.511 s | 0.941 s |

Every sampled owner shrinks. The executable set avoids **5,676,086 bytes (8.3902%)**,
with unchanged identity count. Per-owner reductions range from 429,634 to 904,024
bytes. These are measured new-construction savings, not existing disk space freed.

Warm unit sums are 14.753/14.714/15.561 seconds for control and
12.909/14.031/12.881 for candidate: 11.5651% lower mean in this instrumented
sample. Native child time includes fixture loading/checks and generated execution;
it is not pure PipeLang computation. Host load is uncontrolled. Cold unit sums were
29.762 versus 34.679 seconds, a candidate regression; bootstrap, dependency
preparation, controller storage accounting and focused checks are outside those
unit sums. No full-campaign or general computation speedup is established.

## Verification, resources and evidence

All thirteen focused checks pass: exact binary encoding/malformed input, current
oracles and fresh state, support/source identity invalidation, sealed snapshots,
special harness refusal, corrupt preparation, ordinary bundles/artifact reuse,
actual JSON layout fallback, unchanged v0.111 subset/scope and independent-root
controls. All 26 measured executable identities retain normal debug sections.

The whole validation job took 284.579 seconds with a 959,561,728-byte aggregate
peak, zero OOM/max/swap events and verified removal of its cgroup tree. No failed
attempt occurred. Existing 2-GiB aggregate, 512-MiB coordinator and unit/compiler
ceilings were unchanged; the host launcher ran through narrow reviewed escalation.

The declared estate closed at 36,121,487,539 logical / 38,016,897,024 allocated
file bytes, with sampled peaks 36,150,824,310 / 38,047,772,672 bytes. It includes
this task, both preceding construction experiments, shared Go build cache,
source/package inputs, module/toolchain support and Python/readelf executables.
The estate grew 399,264,883 logical bytes during the job. About 304 MB remains in
this task's artifacts, with additional shared-cache growth. **Zero existing bytes
were freed.** These are sampled file metadata totals, not allocator-exact peaks
or unique physical-device storage. Later small documentation/acceptance records
fall outside the closing job sample. The 96-GiB cap and 8-GiB reserve were preserved;
16/32-GiB targets and retention/cleanup policy remain separate and unadopted.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a0997d-9f74-7301-891c-b485518f17d6/go-v109-layout-fixtures/`

`validate.py` freezes comparison and checks; `source-review.json` proves the
control is audit-only and source generation/oracle/evaluator prefixes and other
functions are unchanged. `validation-receipt.json` owns matched measurements,
source hashes, debug checks and focused results. `validation-job.json` owns
aggregate containment and cleanup; `final-acceptance.json` binds these records to
final source/documentation and protected state. The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v109-layout-fixtures.md)
owns authority and exclusions. Existing evidence and caches remain preserved.
