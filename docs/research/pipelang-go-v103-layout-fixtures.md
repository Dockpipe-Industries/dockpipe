# Go v0.103 terminal conditional-test layout fixtures

Completed bounded experiment, 2026-09-13, on saved `js/pipelang` at
`f74649308429a572cca484b96d665f2f4fe290da`. Accepted language remains v0.113.0.
Implementation is local and uncommitted.

Eligible v0.103 layout native bundles now use the existing exact binary Value/Trace
format and shared reader. The native checker reconstructs twelve Boolean inputs
from each vector index with the independent oracle's `bits & (1 << bit) != 0`
mapping. Ordinary execution retains JSON fixtures. Source generation, current
independent expected values/traces, evaluator calls, HIR/Core/semantic determinism,
both native assertions and owning lifetimes remain unchanged. The shared reader,
batch engine, compiler, language production code and storage accountant are unchanged.

## Matched sample

Twelve of 200 owners: partitions 8/9/10, 32/33/34, 112/113/114 and 192/193/194.
Trees 1/4/14/24 crossed with residues 0/1/2 cover rotating 0/1/3 local sequences.
All assigned subsets and all 4,096 vectors per layout remain. Each pass preserves
81 layouts, 331,776 vectors and 162 fresh native executions. This is sample proof,
not a full-family measurement or full-language acceptance campaign.

Both lanes use offline Go 1.25.13, unchanged accounting, identical native dependency
preparation and audit settings. The control adds only ordered input/value/trace
logging to admitted JSON construction. Each lane starts with an empty native
artifact store for cold population; three warm passes execute every case again.
Warm pass two reverses lane order. All eight passes match source multisets, ordered
oracle digests, owner/subset coverage and fresh executions. Warm misses are zero.

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Executable identities | 15 | 12 |
| Executable bytes including normal debug support | 84,121,760 | 64,613,750 |
| Cold summed unit time | 50.107 s | 46.574 s |
| Mean summed warm unit time | 24.065 s | 20.259 s |
| Mean summed warm native child time | 5.107 s | 1.956 s |

The sample avoids **19,508,010 executable bytes (23.1902%)**.
Mean summed warm unit time is 15.8191% lower. Host load was uncontrolled, so
this is not a general speedup claim. Unit sums exclude preparation, coordinator
accounting and focused checks; native child time includes fixture loading/checking.
Individual warm unit sums and per-owner observations remain in the receipt.

## Validation and containment

All 118 contained units pass: two builds, native preparation, nineteen focused
checks and 96 measured owner executions. Focused proof covers exact binary
strings/nil/empty encoding, malformed fixtures, current independent oracles,
fresh native execution, source/support invalidation, sealed snapshots, special
harness refusal and corrupt preparation. Actual v0.103 JSON fallback, v0.104
through v0.109 layout controls, v0.109 independent roots and v0.111 subset/scope
controls pass. All 27 measured executables retain normal debug sections.

Structural review proves the audit-only control, unchanged source/oracle/evaluator
prefix and other functions, and construction matching accepted v0.104 except the
version labels and 12-bit/4,096-vector input domain. Go, containment/accounting
Python and SQLite support hashes stayed stable throughout the run. Independent
acceptance reconstructs summaries from measured outputs and rechecks current
artifact digests, coverage, resource receipts, source and protected checkout state.

The complete validation job took **374.303 seconds**, with an aggregate
peak of 880,549,888 bytes and coordinator peak of
116,367,360 bytes. Limit/OOM/swap events were zero and the process
tree was removed. Maximum recorded storage-heartbeat age was
0.559 seconds (existing ceiling: five).
The unchanged coordinator reclaim hook recorded 0 requests and
0 requested bytes; this does not measure bytes freed.

The 96-GiB storage cap/8-GiB reserve, 2-GiB aggregate/1536-MiB high,
512-MiB coordinator, 1-GiB unit/700-MiB high, no swap and all child/compiler
ceilings remain unchanged. The job used narrowly reviewed host execution after
sandbox user-systemd denial. Job time excludes preparation of the experiment,
source review, documentation and final acceptance; it is not total investigation time.

## Retained cost and evidence

The declared estate closed at 40,585,160,412 logical /
42,596,261,888 allocated file bytes, with sampled peaks of
40,651,241,900 / 42,662,125,568.
Logical growth from admission was 539,262,770 bytes.
The union includes this task, all eight preceding construction experiments,
shared Go cache, source/package inputs, module/toolchain support, Python/readelf
executables and SQLite runtime. Sampled peaks are not exact temporary or physical
peaks; hardlinks are accounted for and shared extents remain unidentified.

This task retains approximately 397.5 MB before final documentation and
acceptance; shared-cache growth is separate. **Zero existing bytes were freed.**
Later documentation and final acceptance are outside the closing storage sample.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a09ba6-75b0-7401-af5e-f9b46c56e266/go-v103-layout-fixtures/`

`validation-receipt.json` owns matched results, sources and debug artifact digests;
`validation-job.json` owns aggregate containment and process-tree removal;
`storage/` owns the declared estate and sampled storage accounting;
`coordinator-memory.jsonl` owns coordinator counters and heartbeat observations;
`source-review.json` owns structural preservation checks. `final-acceptance.json`
binds current postimages and evidence after independent reconciliation.
The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v103-layout-fixtures.md)
owns authority and exclusions. No further performance scope is selected.
Engine/package boundaries are preserved; no installs, cleanup, Git mutation or
publication occurred.
