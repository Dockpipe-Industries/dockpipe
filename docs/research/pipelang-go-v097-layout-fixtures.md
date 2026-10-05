# Go v0.97 terminal initializer layout fixtures

Completed bounded experiment, 2026-09-13, in saved `js/pipelang` at
`62b13d068ba576c12c8917afb0ef609464a56147`. Accepted language remains v0.113.0.
Implementation is local and uncommitted.

Eligible v0.97 layout native bundles now use the existing exact binary Value/Trace
format and shared reader. Seven routing flags use the existing ordered path table
(`paths[vector/256]`). Seven initializer flags and the finish flag use
`(vector%256)/2 | (vector%2)<<7`, because finish varies fastest. Every reconstructed
flag is checked against the original independent oracle row before encoding.
Ordinary execution retains JSON fallback. Source generation, expected values and
traces, evaluator calls, HIR/Core/semantic determinism, both native assertions and
owning lifetimes remain unchanged. Shared reader, batch engine, compiler and
accounting remain unchanged.

## Matched sample

Twelve of 100 owners: partitions 0-3, 48-51 and 96-99. All assigned layouts and
vectors remain: **66 layouts, 102,400 vectors and 132 fresh native executions per
pass**, including every scope mask and both five-local layouts. This is sample
proof, not a full-family or full-language measurement.

Both lanes use offline Go 1.25.13, unchanged accounting, identical native dependency
preparation and audit settings. Control differs from the admitted source only by
ordered input/value/trace logging. Each lane starts with an empty native artifact
store for cold population; three warm passes execute all selected cases again.
Warm pass two reverses lane order. All eight passes match source multisets, ordered
oracle digests, paths, layouts, vectors and fresh execution counts. Warm misses are zero.

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Executable identities | 12 | 12 |
| Executable bytes including normal debug support | 67,586,330 | 62,279,514 |
| Cold summed unit time | 48.106 s | 50.431 s |
| Mean summed warm unit time | 19.692 s | 17.843 s |
| Mean summed warm native child time | 3.111 s | 1.397 s |

The sample avoids **5,306,816 executable bytes (7.8519%)**.
Mean summed warm unit time is 9.3901% lower. Host load was uncontrolled;
these are sample observations, not a general speedup claim. Unit sums exclude
preparation, coordinator accounting and focused checks; native child time includes
fixture loading/checking. Individual pass and owner observations remain in the receipt.

## Validation and containment

All **120 contained units** pass: two builds, native preparation, 21 focused checks
and 96 measured owner executions. Focused proof covers exact binary strings/nil/
empty encoding, malformed fixtures, current independent oracles, fresh native
execution, source/support invalidation, sealed snapshots, special-harness refusal
and corrupt preparation. Actual v0.97 JSON fallback, v0.102 through v0.109 layout
controls, v0.109 independent roots and v0.111 subset/scope controls pass.
All 24 measured executables retain normal debug sections.

Structural review proves the audit-only control, preserved source/oracle/evaluator
prefix, original trace instrumentation and JSON loader, untouched remaining functions
and both native assertions. Arithmetic review exhausts all 256 initializer/finish
combinations, and candidate construction checks all 15 original flags on every row.
Independent acceptance reconciles raw output with receipts, uses recursive minimum-path
enumeration independently of the driver's mask scan, and checks current artifact hashes.
Go source, containment/accounting Python and SQLite support hashes remain stable.
A stale v0.102 literal in the preparation review script was corrected before any
native validation; no benchmark failed or measurement was carried forward from it.

The validation job took **353.048 seconds**. This excludes preparation,
review, documentation and final acceptance, and is not total investigation time.
Aggregate peak: 848,326,656 bytes; coordinator peak:
101,855,232 bytes. Limit/OOM/swap events are zero and process-tree
removal is verified. Maximum recorded storage-heartbeat age:
0.664 seconds (five-second ceiling).
Coordinator reclaim recorded 0 requests /
0 requested bytes; this is not measured freed storage.

Unchanged limits: 96-GiB storage/8-GiB reserve, 2-GiB aggregate/1536-MiB high,
512-MiB coordinator, 1-GiB unit/700-MiB high, no swap and all child/compiler ceilings.
Validation used a narrowly reviewed host operation after sandbox user-systemd denial.

## Retained cost and evidence

The declared estate closed at 41,819,531,292 logical /
43,866,853,376 allocated file bytes, with sampled peaks of
41,867,736,496 / 43,915,223,040.
Logical growth during validation was 468,426,218 bytes.
The union includes this experiment, all ten preceding construction experiments
(including v0.102's failed attempt), shared Go cache, source/package inputs,
module/toolchain support, Python/readelf executables and SQLite runtime. Hardlinks
are deduplicated; shared extents remain unidentified. Sampled peaks are not exact
temporary or physical-device peaks. Documentation and final acceptance occur after
the closing storage sample.

This task retains approximately **359.7 MB** before final documentation
and acceptance; shared-cache growth is separate. **Zero existing bytes were freed.**

`E=/home/jamie/.codex/visualizations/2026/09/13/01a09bce-198f-7f70-b71d-ba99eca633bb/go-v097-layout-fixtures/`

`validation-receipt.json` owns matched measurements, sources and debug artifact hashes;
`validation-job.json` owns aggregate containment and process-tree removal; `storage/`
owns estate accounting; `coordinator-memory.jsonl` owns coordinator counters and
heartbeat observations; `source-review.json` owns structural checks; and
`final-acceptance.json` binds final postimages and independently reconciled evidence.
The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v097-layout-fixtures.md)
owns authority and exclusions. Engine/package boundaries are preserved. No full
campaign, installs, cleanup, commit, push or publication occurred. No successor is selected.
