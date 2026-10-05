# Go v0.102 terminal Boolean-selector initializer layout fixtures

Completed bounded experiment, 2026-09-13, on saved `js/pipelang` at
`058c90f1b88e8e96e51519efc34a02d571f74572`. Accepted language remains v0.113.0.
Implementation is local and uncommitted.

Eligible v0.102 layout native bundles now use the existing exact binary Value/Trace
format and shared reader. Seven routing inputs come from the existing ordered path
table (`paths[vector/512]`). The nine initializer/return inputs use
`(vector%512)/8 | (vector%8)<<6`, because the original loops vary return masks fastest.
Every reconstructed flag is checked against the independent oracle's original row
before encoding. Ordinary execution retains the original JSON loader.
Source generation, independent expected values/traces, evaluator calls,
HIR/Core/semantic determinism, both native assertions and owning lifetimes remain
unchanged. The shared reader, batch engine, compiler and storage accountant are
unchanged.

## Matched sample

Seventeen of 200 owners: partitions 8/9/10, 32/33/34, 112/113/114 and 192–199.
All assigned layouts and all existing vectors remain: 52 layouts, 185,344 vectors
and 104 fresh native executions per pass. The sample includes every scope mask,
ordinary/depth-three/selector returns, unused tails and both five-local layouts.
This is sample proof, not a full-family or full-language measurement.

Both lanes use offline Go 1.25.13, unchanged accounting, identical native dependency
preparation and audit settings. The control adds only ordered input/value/trace
logging to admitted JSON construction. Each lane starts with an empty native
artifact store for cold population; three warm passes execute every case again.
Warm pass two reverses lane order. All eight passes match source multisets, ordered
oracle digests, path/layout/vector coverage and fresh executions. Warm misses are zero.

| Seventeen selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Executable identities | 17 | 17 |
| Executable bytes including normal debug support | 86,490,892 | 79,056,096 |
| Cold summed unit time | 55.806 s | 48.104 s |
| Mean summed warm unit time | 29.055 s | 24.606 s |
| Mean summed warm native child time | 5.650 s | 2.097 s |

The sample avoids **7,434,796 executable bytes (8.5960%)**.
Mean summed warm unit time is 15.3104% lower. Host load was uncontrolled;
this is not a general speedup claim. Unit sums exclude preparation, coordinator
accounting and focused checks; native child time includes fixture loading/checking.
Individual warm unit sums and per-owner observations remain in the receipt.

## Validation, recovery and containment

All 159 contained units pass: two builds, native preparation, twenty focused checks
and 136 measured owner executions. Focused proof covers exact binary strings/nil/
empty encoding, malformed fixtures, current independent oracles, fresh native
execution, source/support invalidation, sealed snapshots, special-harness refusal
and corrupt preparation. Actual v0.102 JSON fallback, v0.103 through v0.109 layout
controls, v0.109 independent roots and v0.111 subset/scope controls pass.
All 34 measured executables retain normal debug sections.

Structural review proves the audit-only control, unchanged source/oracle/evaluator
prefix, trace instrumentation, JSON loader, remaining functions and native assertions.
Independent acceptance rechecks source/artifact digests and reconstructs raw-output
summaries; recursive minimum-path enumeration checks the driver's mask-scan inventory.
Go, containment/accounting Python and SQLite support hashes remain stable.

The first attempt caught an incorrect input-index mapping before candidate native
execution. Its failed unit and 184.321-second job remain retained;
all resource limits held and its process tree was removed. The corrected attempt
uses entirely fresh control/candidate stores. No measurement is carried forward
from the failed attempt. Arithmetic review exhausts all 512 initializer/return
combinations, and candidate construction checks every original row's flags.

The successful validation job took **401.874 seconds**; both job attempts
total 586.196 seconds. These figures exclude experiment
preparation, review, documentation and final acceptance, and are not total
investigation time. Successful aggregate peak: 821,469,184 bytes;
coordinator peak: 100,962,304 bytes. Limit/OOM/swap events are zero,
and process-tree removal is verified. Maximum recorded storage-heartbeat age:
1.096 seconds (existing ceiling: five).
Coordinator reclaim recorded 0 requests /
0 requested bytes; this is not measured freed storage.

The unchanged limits are 96-GiB storage/8-GiB reserve, 2-GiB aggregate/1536-MiB high,
512-MiB coordinator, 1-GiB unit/700-MiB high, no swap and all child/compiler ceilings.
Both attempts used narrowly reviewed host execution after sandbox user-systemd denial.

## Retained cost and evidence

The declared estate closed at 41,350,883,710 logical /
43,384,512,512 allocated file bytes, with sampled peaks of
41,423,131,962 / 43,456,413,696.
Logical growth during the successful attempt was
428,967,461 bytes; its admission already includes
the failed attempt. The union includes both attempts, all nine preceding construction
experiments, shared Go cache, source/package inputs, module/toolchain support,
Python/readelf executables and SQLite runtime. Sampled peaks are not exact temporary
or physical peaks; hardlinks are accounted for and shared extents remain unidentified.

Both attempts retain approximately **614.1 MB** before final documentation
and acceptance; shared-cache growth is separate. **Zero existing bytes were freed.**
Later documentation and final acceptance are outside the closing storage sample.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a09bb6-b240-7a11-9bb2-afc1de69972e/go-v102-layout-fixtures/`

Successful proof lives in `E/retry-1/`: `validation-receipt.json` owns matched results,
sources and debug artifact digests; `validation-job.json` owns aggregate containment
and process-tree removal; `storage/` owns estate and sampled accounting;
`coordinator-memory.jsonl` owns coordinator counters and heartbeat observations;
`source-review.json` owns structural checks; `final-acceptance.json` binds final
postimages and evidence after independent reconciliation. The first attempt's
receipts remain directly under `E` and are bound by final acceptance.
The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v102-layout-fixtures.md)
owns authority and exclusions. Engine/package boundaries are preserved. No full
campaign, installs, cleanup, commit, push or publication occurred. No successor is selected.
