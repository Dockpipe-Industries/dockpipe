# Go v0.89 nested terminal-leaf return layout fixtures

Completed bounded experiment, 2026-09-13, in the saved `js/pipelang` checkout at
`cea1674ee7118aece732797bc4adc917c844fc4a`. Accepted language remains v0.113.0.
Implementation is local and uncommitted.

Eligible v0.89 nested terminal-leaf return bundles now select the existing exact
binary Value/Trace reader. The original mask already identifies input flags.
Ordinary execution and `PIPELANG_SHARED_EXPORT` retain JSON fixtures. Existing
v0.91 binary behavior and audit format remain unchanged. Source generation,
original expected values/traces, evaluator calls, four-method batches, both
native assertions and native lifetimes are unchanged. Shared readers, compiler
and containment/accounting code are untouched.

## Matched sample

Twelve of 25 shape owners: **0-3, 10-13 and 21-24**. Every assigned case remains:
**296 methods, 188,416 vectors and 148 fresh native executions per pass**. The
sample includes simple and full depth-three statement trees, three-local scopes,
distributed local layouts and five-local layouts, with used/unused locals.
It does not cover all statement trees and is not full-family or full-language proof.

Both lanes use offline Go 1.25.13, identical preparation, audit settings and
inclusive accounting. The control is admitted source plus v0.89 ordered audit
logging only. Each lane starts with an empty artifact store. One cold and three
warm passes execute every selected case freshly; warm pass two reverses lane
order. All eight passes match source multisets, ordered input/value/trace digests,
methods, vectors and native execution counts. Warm misses are zero.

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Executable identities | 13 | 13 |
| Executable bytes including normal debug support | 102,391,545 | 96,796,916 |
| Cold summed unit time | 63.997 s | 90.740 s |
| Warm one summed unit time | 20.678 s | 18.793 s |
| Warm two summed unit time, candidate first | 19.243 s | 20.406 s |
| Warm three summed unit time | 18.977 s | 17.731 s |
| Mean summed warm unit time | 19.633 s | 18.976 s |
| Mean summed warm native child time | 2.827 s | 1.551 s |

The sample avoids **5,594,629 executable bytes (5.4640%)**, without reducing
identities. Mean warm unit time is 3.3428% lower, but the reversed-order warm pass
is slower and cold unit time is **41.7871% higher**. Host load was uncontrolled;
these observations do not establish a reliable overall speedup. Retention meets
the approved executable-byte and correctness/resource acceptance criteria.
Unit sums exclude preparation, coordinator accounting and focused checks; native
child time includes fixture loading/checking. All per-owner observations remain.

## Validation and containment

All **122 contained units** pass: two builds, native preparation, nine focused
reader/cache checks, twelve matched ordinary/adjacent checks, two shared-export
checks and 96 measured owner executions. Focused proof covers exact encoding,
malformed fixtures, current independent oracles, source/support invalidation,
sealing, special-harness refusal and corrupt preparation.

Ordinary v0.89 JSON and selected v0.84/v0.87/v0.90/v0.91/v0.97 owners retain
matching source/artifact identities and native execution counts. The selected
shared-export owner preserves every exported file byte, including JSON fixtures
and reader source; this is a focused check, not a full export campaign.
All 26 measured executables retain normal debug sections.

Structural review reconstructs the admitted source by reversing only additional
v0.89 audit logging and the eligibility change. All source generation, original
oracles, evaluator calls, mask mapping, batching and native assertions remain.
Independent acceptance uses base-five shape decoding and inverse method numbering,
then reconciles raw outputs, unit receipts and current executable hashes.
All compiler Go, containment/accounting Python and SQLite support hashes remained
stable during validation.

The validation job took **427.537 seconds**, excluding preparation, review,
documentation and independent acceptance. It is not total investigation time.
Aggregate peak was 1,596,198,912 bytes; coordinator peak was 402,984,960 bytes.
Hard-limit, OOM and swap events were zero; process-tree removal is verified.
The control/candidate harness builds recorded 188/25 soft `memory.high` events;
the aggregate hierarchical counter recorded their sum, 213. Measured owner units
recorded no such events. Soft reclaim remains allowed by the existing
`campaign.resource_accepted` policy. The copied task-local checker was corrected
to follow that policy, with its original and correction record preserved; no
repository acceptance predicate or limit changed. No benchmark was rerun.
Scoped coordinator reclaim requested 537,706,496 bytes over four requests; this
is not measured freed storage.

Unchanged limits: 96-GiB storage/8-GiB reserve, 2-GiB aggregate/1536-MiB high,
512-MiB coordinator, 1-GiB unit/700-MiB high, no swap and all child/compiler
ceilings. Native validation and final storage inventory used separately reviewed
host containment operations after sandbox user-systemd bus denial.

## Retained cost and evidence

The validation estate closed at 43,560,253,427 logical /
45,636,976,640 allocated file bytes; sampled peaks were 43,614,965,803 /
45,690,855,424 bytes. It includes this experiment, twelve preceding construction
experiments, shared Go cache, source/package inputs, module/toolchain support,
Python, readelf and SQLite runtime. Hardlinks are deduplicated; shared extents
remain unidentified. Sampled peaks are not exact temporary or physical-device
peaks. Final documentation and acceptance occur after the closing sample.

Task artifacts occupy approximately **828.5 MB** before final documentation and
acceptance; shared-cache growth is separate. **Zero existing bytes were freed.**
The separate post-run inventory passed at 43,560,265,966 logical /
45,637,001,216 allocated bytes under unchanged limits. Its capped job took
27.686 seconds and passed process-tree removal with no OOM/swap events.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a09c19-13f3-7841-a3b3-251c5d8bbce9/go-v089-layout-fixtures/`

`validation-receipt.json` owns matched measurements and source/debug hashes;
`validation-job.json` owns aggregate containment; `storage/` owns inclusive
accounting; `post-run-storage.json` and `storage-accept-job.json` own the final
capped inventory. `source-review.json` owns structural proof and
`acceptance-review.json` explains soft-event acceptance. `final-acceptance.json`
binds final postimages and independently reconciled evidence. The
[objective record](../agents/tasks/pipelang-reactive-application-language/go-v089-layout-fixtures.md)
owns authorization and exclusions. Engine/package boundaries are preserved.
No full campaign, install, cleanup, commit, push or publication occurred.
