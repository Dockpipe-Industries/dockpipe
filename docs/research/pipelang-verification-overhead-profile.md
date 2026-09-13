# Current paired verification overhead

Approved bounded profiling objective, 2026-09-13. Completed with independent acceptance.
The [objective record](../agents/tasks/pipelang-reactive-application-language/verification-overhead-profile.md)
owns authorization, invariants and exclusions. No optimization is implemented.

## Frozen design

The existing 48-case selection covers eight cases each from v0.109 layouts,
v0.111 subsets, v0.108 layouts and v0.107 layouts, plus four each from v0.91
layouts, v0.105 heavy layouts, v0.109 memory and artifact/special harness tests.
Case names and comparison order were frozen before current measurement.

Existing opt-in Go identity timings and task-local Python timing wrappers measure
unchanged validation calls. Instrumented Python spans report inclusive and exclusive
wall time; exclusive excludes instrumented descendants in the same thread. These
are summed observations, not CPU time or additional elapsed time. The driver captures
complete subprocess time and aggregate CPU/pressure counters in both lanes. File
identity byte counts are logical repeated reads, not physical device I/O.

Each lane learns its own warm singleton profile under current source, host, worker
and policy identities. Only observations eligible for pairing in both lanes enter
the comparison profiles; no observed values or identities are rewritten. All 48 cases
remain selected, including ineligible singletons. Four balanced warm comparisons run
off/on, on/off, off/on, on/off with matching grouping. Retained read-buffer reuse,
two workers, ordinary native bundles, independent oracles, fresh native execution,
all content checks and every existing deadline/resource ceiling remain unchanged.

## Preparation and failed evidence

The initial private-store population passed 47 of 48 cases. The special current-oracle
harness hit its unchanged 25-second outer deadline during fresh temporary standard-library
preparation. The failed job took 170.286 seconds, peaked at 1,334,472,704 aggregate bytes,
recorded no hard-limit/OOM/swap event, and removed its process tree. Its evidence and
populated stores remain. The replacement uses fresh output generations and revalidates
those stores while executing all 48 cases again. No cold-cache or cold-device claim
applies to the replacement. Preparation and failed attempts are excluded from warm
comparisons and included in investigation cost.

## Accepted measurements

All **11 fresh passes** preserve **48 cases, 294 ordered source/fixture audits and
294 fresh audited native executions per pass**. Current source generation, independent
Value/Trace oracles, selected vectors and case order remain exact. The warm comparisons
use **18 pairs and 12 singletons**; independently learned profile identities and recorded
observations are unchanged. Every post-population pass has zero retained-artifact misses.
This is representative-sample evidence, not full-family or full-language proof.

| Round | Order | Profiling off, complete seconds | Profiling on, complete seconds | Observed overhead |
| --- | --- | ---: | ---: | ---: |
| 1 | off/on | 48.403 | 46.426 | -4.085% |
| 2 | on/off | 45.317 | 45.559 | +0.533% |
| 3 | off/on | 45.795 | 45.535 | -0.567% |
| 4 | on/off | 45.525 | 48.129 | +5.722% |

Mean complete-path time is **46.260 s off / 46.412 s on**
(+0.330%). The directions are mixed and the host was uncontrolled;
no consistent instrumentation penalty or negative overhead is resolved. These lanes
compare profiling, not an optimization. Both retain audit and general performance
logging; the measured toggle adds identity spans and Python wrappers.
Mean aggregate CPU is 108.785 / 107.312 s; summed unit time is
65.980 / 65.561 s. Neither is elapsed time.

Mean **exclusive Go identity leaf wall seconds**, summed across measured workers:

| Operation | Seconds |
| --- | ---: |
| Toolchain contents: read/hash | 10.3637 |
| Toolchain enumeration/metadata | 1.2920 |
| Cached record and binary checks | 1.7274 |
| Binary copy/hash/seal | 0.4461 |
| Lock acquisition | 0.0526 |
| Generated source keys | 0.0216 |

Toolchain visits represent **4,299,618,550 logical bytes and 234,150 file visits**
per instrumented paired pass. This is repeated logical read/hash work, not device I/O.
The separate enclosing artifact-identity span averages 12.277 s; never add it to its
leaf timings. Instrumentation does not cover every internal action, including output
captured inside special-harness probes, so leaf totals need not equal enclosing spans.
Native child time averages 3.202 s and overlaps the native-execution span.

The Python coordinator's file-identity calls average **8.308 summed seconds**
across 52,919 calls and approximately 2.00 GB of logical bytes per instrumented pass.
This function includes path resolution, stat/mutation checks and hashing; do not call
it pure hashing or CPU time. Two explicit coordinator toolchain identities average
**3.143 inclusive seconds**, and receipt reconciliation averages **0.945 inclusive
seconds**. These include file-identity work above and must not be added again.
Other wrapper exclusive costs and all raw counters remain in the evidence.
`io.stat` was unavailable; physical I/O and exclusive per-operation CPU are unknown.

## Recommendation and boundary

**Next recommendation: one bounded experiment in sharing a verified immutable toolchain
identity across the existing worker processes.** Worker toolchain content reads dominate
the measured identity leaves even after pairing and buffer reuse. Their repetition grows
with worker-group count. Coordinator startup has additional opportunities, but its two
explicit toolchain walks are fixed per suite; the 48-case share must not be extrapolated
as a full-campaign saving. The coordinator already watches the same toolchain roots in
`dependency_guard`; that is a design input, not authority to skip worker validation.

The proposed follow-up should first establish an immutable byte/lifetime contract that
workers can independently admit, then compare total preparation, execution, reconciliation,
RAM and retained support against the unchanged current path. It needs adversarial proof
for same-path mutation/restoration, replacement, new/missing files, settings/toolchain
changes, corrupt identities, lost watches or owner death, concurrency and restart.
Every new campaign must admit current bytes. No path/mtime digest cache, blind parent
attestation, receipt-only execution or skipped content validation is acceptable.
Retain a candidate only with consistent complete-path gains and exact semantic/resource
proof; snapshot construction or support cost could erase the benefit. Savings are unknown.
No cross-process reuse is implemented or approved by this profiling result.

## Acceptance, resources and retained cost

Independent acceptance reconciles **417 contained units**, each raw sealed receipt,
7,907 required artifact references / 5,430,796,169 logical hash bytes, ordered audits,
current source/support hashes and both protected stashes. Every selected case executes
freshly; the failed initial population is separate. The successful measurement job took
**627.495 seconds**; independent acceptance and final storage took **37.943
seconds**. Including the failed 170.286-second job, recorded job time is
835.725 seconds. This excludes preparation of scripts,
review and documentation and is not total investigation time.

The successful measurement job peaks at **962,134,016 aggregate bytes**.
Hard-limit, soft memory.high, OOM and swap events are zero in that job and its accepted
units; all measurement/acceptance process trees are removed. The unchanged storage
owner made three scoped coordinator reclaim requests totaling 403,791,872 requested
bytes; these are memory requests, not freed storage. The existing coordinator ceiling
remains 512 MiB and all original job/unit/child/compiler limits remain unchanged.

The post-run declared estate is **37,343,949,254 logical / 39,320,592,384
allocated file bytes**, with measured campaign metadata peaks of
37,430,889,157 / 39,409,991,680 bytes.
It includes the task's failed and successful generations, shared Go build cache, current
source/package inputs, offline module/toolchain support, Python and SQLite support.
It is not an inventory of all historical unrelated campaigns. Hardlinks are deduplicated;
shared extents and exact temporary/device peaks remain unknown. The 96-GiB cap and
8-GiB reserve pass. Final report/binding documents follow this metadata snapshot.

Task evidence/private stores retain approximately **867.2 MB logical /
908.5 MB allocated** before final reporting; shared-cache growth is separate.
**Zero existing bytes were freed.** The initial timeout disappeared in the fresh
replacement using admitted populated stores; reduced competing build work is a plausible
explanation, not isolated causal proof. No benchmark deadline or acceptance predicate changed.

## Evidence

`E=/home/jamie/.codex/visualizations/2026/09/13/01a09c2e-b445-7322-8a02-1d2328313bec/verification-overhead/`

The original `profile-job.json` and `recovery.json` retain failure and recovery facts.
`selection.json` freezes case selection. `attempt-2/frozen-design.json` binds current
source/support and comparison order. Task-local scripts and raw per-unit output,
sealed receipts, storage accounting and coordinator timing remain alongside results.
No source/compiler/containment implementation changes, full campaign, deletion,
commit, push, publication, install or external configuration mutation occurred.

`attempt-2/independent-acceptance.json`, `accept-job.json` and `post-run-storage.json`
own terminal acceptance. `task-storage.json` reports task-only retained cost.
Engine/package boundaries are preserved; only research/tracking documents changed
in the checkout. All measurement support and generated artifacts remain task-local.
