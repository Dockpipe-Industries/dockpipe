# Go v0.91 nested terminal initializer layout fixtures

Completed bounded experiment, 2026-09-13, in the saved `js/pipelang` checkout at
`d55d83e67d18c78981b008438a7dc4cb59d39f88`. Accepted language remains v0.113.0.
Implementation is local and uncommitted.

Eligible v0.91 nested terminal initializer bundles now encode their existing
independent Value/Trace rows with the exact binary reader. The original mask
already identifies input flags, so no new input reconstruction is required.
Ordinary execution, other families using this harness, and `PIPELANG_SHARED_EXPORT`
retain their JSON fixtures and reader. Source generation, original expected
values/traces, evaluator calls, batch partitions, both native assertions and
native lifetimes remain unchanged. Shared readers, batch engine, compiler and
accounting are untouched. Optional audit logs bind original ordered inputs,
values and traces in both control and candidate.

## Matched sample

Twelve of 200 owners: **0-3, 96-99 and 196-199**. All assigned methods and vectors
remain: **258 methods, 166,528 vectors and 136 fresh native executions per pass**.
The sample includes simple and full depth-three statement trees, ordinary and
nested returns, all four owner partitions at each selected shape/return pair,
used/unused locals and zero/one/two/three/five-local layouts. It does not cover
all 25 statement trees or every shape/return pair.

Both lanes use offline Go 1.25.13, identical dependency preparation, audit settings
and resource/storage accounting. The control is admitted source plus ordered audit
logging only. Each lane starts with an empty artifact store; one cold and three
warm passes execute every selected case freshly. Warm pass two reverses lane order.
All eight passes match source multisets, ordered input/value/trace digests, method
inventory, vectors and execution counts. Warm misses are zero.

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Executable identities | 12 | 12 |
| Executable bytes including normal debug support | 94,775,640 | 89,595,580 |
| Cold summed unit time | 56.755 s | 55.280 s |
| Mean summed warm unit time | 17.645 s | 16.104 s |
| Mean summed warm native child time | 2.373 s | 1.287 s |

The sample avoids **5,180,060 executable bytes (5.4656%)**.
Mean summed warm unit time is 8.7339% lower; cold unit time is 2.5993% lower.
Host load was uncontrolled. These are sample observations, not full-family or
full-language improvements. Unit sums exclude preparation, coordinator accounting
and focused checks; native child time includes fixture loading/checking.
All repetitions and per-owner observations remain in the receipt.

## Validation and containment

All **122 contained units** pass: two builds, native preparation, nine focused
reader/cache checks, twelve matched ordinary/adjacent checks, two shared-export
checks and 96 measured owner executions. Focused proof covers exact binary
strings/nil/empty encoding, malformed fixtures, current independent oracles,
fresh native execution, source/support invalidation, sealing, special-harness
refusal and corrupt preparation.

Ordinary v0.91 JSON, v0.84/v0.87/v0.89/v0.90 shared-harness controls and v0.97
layout controls retain matching source/artifact identities and native execution
counts. The selected shared-export owner preserves every exported file byte,
including JSON fixtures and shared reader; this is a focused export check, not a
full export campaign. All 24 measured executables retain normal debug sections.

Structural review reconstructs the admitted source after removing exactly the
audit and encoding/loading changes. Source generation, oracle/evaluator work,
original mask mapping, batching, trace instrumentation, fallback and remaining
functions are unchanged. Independent acceptance uses inverse method numbering
and a separate scope enumeration, then reconciles raw outputs, unit receipts and
current executable hashes. All compiler Go, containment/accounting Python and
SQLite support hashes remain stable during validation.

The validation job took **356.712 seconds**, excluding preparation,
review, documentation and independent acceptance. It is not total investigation
time. Aggregate peak: 1,344,921,600 bytes; coordinator peak:
407,470,080 bytes. Limit/OOM/swap events are zero and process-tree removal is
verified. Scoped coordinator reclaim made two requests totaling 273,063,936 bytes;
this is not measured freed storage. No benchmark failed or reused failed timing.

Unchanged limits: 96-GiB storage/8-GiB reserve, 2-GiB aggregate/1536-MiB high,
512-MiB coordinator, 1-GiB unit/700-MiB high, no swap and all child/compiler
ceilings. Native validation and final storage re-inventory use narrowly reviewed
host containment after sandbox user-systemd denial.

## Retained cost and evidence

The validation estate closed at 42,673,099,648 logical /
44,734,734,336 allocated file bytes, with sampled peaks of
42,731,921,911 / 44,792,926,208 bytes.
It includes this experiment, all eleven preceding construction experiments,
shared Go cache, source/package inputs, module/toolchain support and Python,
readelf and SQLite runtime. Hardlinks are deduplicated; shared extents remain
unidentified. Sampled peaks are not exact temporary or physical-device peaks.
Final documentation and independent acceptance occur after the closing sample.

Task artifacts occupy approximately **795.4 MB** before final documentation and
acceptance; shared-cache growth is separate. **Zero existing bytes were freed.**
Post-cleanup storage acceptance separately re-inventories the declared estate
without rewriting population credits. Its separate capped job took 23.048 seconds
and also passed containment/cleanup, with zero OOM/swap events.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a09c04-20ed-7990-8f8b-e370305d21e7/go-v091-layout-fixtures/`

`validation-receipt.json` owns matched measurements, source and debug hashes;
`validation-job.json` owns aggregate containment and process-tree removal;
`storage/` owns inclusive accounting; `post-run-storage.json` and
`storage-accept-job.json` own the capped post-cleanup inventory; `source-review.json`
owns structural proof; `final-acceptance.json` binds final postimages and independently
reconciled evidence. The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v091-layout-fixtures.md)
owns authority and exclusions. Engine/package boundaries are preserved.
No full campaign, install, cleanup, commit, push or publication occurred.
