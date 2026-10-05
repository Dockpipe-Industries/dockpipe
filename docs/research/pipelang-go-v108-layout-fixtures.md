# Go v0.108 inner-selector layout fixtures

Completed bounded experiment, 2026-09-13, on saved `js/pipelang`, admitted at
`32cae8e82d0574834a03106f6eed2e9440b779e9`. Accepted language remains v0.113.0.

## Construction and scope

Eligible v0.108 inner-selector layout native bundles now use the existing exact
length-prefixed Value/Trace fixtures and shared binary reader. The native checker
reconstructs the twelve Boolean inputs from the vector index with the same
`bits & (1 << bit) != 0` mapping as the independent oracle. Ordinary execution
retains its JSON fixtures and loader. Current expected values and ordered traces
are still computed afresh; native trace checks also assert returned values.

Source emission, evaluator calls, HIR/Core/semantic determinism, owning lifetimes,
all vectors and package/source/fixture limits are unchanged. The generated batch
and shared reader implementations are unchanged. No compiler or language change,
cross-owner pooling, validation-read optimization or cleanup is included.

The frozen sample crosses trees 0/4/14/24 with all three inner-selector families:
partitions 1/9/17, 97/105/113, 337/345/353 and 577/585/593. Each owner retains all
assigned subsets and 4,096 vectors per source layout. Each pass covers 81 layouts,
331,776 vectors and 162 fresh native package executions. This is 12/600 owners,
not full-family or full-language acceptance.

An audit-only overlay adds ordered input/value/trace digests to the admitted JSON
control. Both lanes use the same cached Go 1.25.13, native dependency preparation,
audit settings and containment. The candidate builds directly from retained source.
Cold population and three fresh warm passes match generated-source multisets,
ordered oracle digests, subset inventory and native execution counts. Warm pass two
reverses lane order. Every warm pass has zero misses.

## Measurements

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Distinct executable identities | 15 | 12 |
| Executable bytes including normal debug/runtime support | 87,074,736 | 67,569,638 |
| Source layouts / vectors per pass | 81 / 331,776 | 81 / 331,776 |
| Fresh native package executions per pass | 162 | 162 |
| Cold summed unit time | 64.978 s | 61.737 s |
| Mean summed warm unit time | 30.594 s | 23.509 s |
| Mean summed warm native child time | 6.479 s | 2.354 s |

The sample avoids **19,505,098 executable bytes (22.4004%)** and three identities.
Every sampled owner shrinks. Owners 577/585/593 each construct one executable
instead of two within the existing limits; the other nine retain one each.
This does not establish the remaining family's identity count or savings.

Warm unit sums are 32.890/30.452/28.441 seconds for control and
22.990/24.316/23.222 for candidate, a 23.1581% lower mean. All three comparisons
improve, including reversed order. Host load is uncontrolled and the control times
decline across passes; this is an instrumented sample, not a general speedup claim.
Native child time includes fixture loading/checking and generated execution, not
pure computation. Unit sums exclude bootstrap, dependency preparation, controller
storage accounting and focused checks. No full-language campaign ran.

## Verification and retained cost

Fourteen focused checks pass: exact strings/nil/empty encoding, malformed binary
fixtures, current-oracle refusal and freshness, source/support invalidation, sealed
snapshots, corrupt preparation, special harness refusal, ordinary fixture/reuse
checks, actual v0.108 JSON fallback and unchanged v0.109 layout/independent-root
and v0.111 subset/scope controls. All 27 measured executables retain normal debug
sections. Source review proves an audit-only control and unchanged source/oracle/
evaluator prefix and other functions; validation verifies stable source bytes.

The complete validation job took 443.886 seconds with a 1,026,756,608-byte aggregate
peak, zero OOM/max/swap events and verified process-tree removal. No contained
execution failed. The 2-GiB aggregate, 512-MiB coordinator and unit/compiler ceilings were
unchanged. The job ran through a narrowly reviewed host escalation after the
sandbox denied user-systemd access.

The declared estate closed at 36,678,113,874 logical / 38,587,424,768 allocated file
bytes; sampled peaks were 36,744,426,600 / 38,654,074,880 bytes. Its union includes
this task, the preceding three construction experiments, shared Go build cache,
source/package inputs, module/toolchain support and Python/readelf executables.
Logical estate growth from the storage controller's initial inventory was
556,498,303 bytes. About 397 MB remains in task artifacts, with additional shared
Go build-cache growth. **Zero existing bytes were freed.** These
are sampled file metadata totals, not exact simultaneous allocation or unique
physical-device usage. Later small documentation/acceptance records are outside
the closing job sample. The 96-GiB cap and 8-GiB reserve remain unchanged; proposed
16/32-GiB budgets and retention/cleanup policy remain separate.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a0998d-a291-7633-985a-944246ec9459/go-v108-layout-fixtures/`

`validate.py` freezes comparison and checks; `source-review.json` records the
structural review against `admitted-matrix.go` and the audit-only control overlay.
`validation-receipt.json` owns measurements, source hashes, debug evidence and
focused results. `validation-job.json` owns aggregate containment and process-tree
removal. `final-acceptance.json` binds final postimages and validation evidence.
The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v108-layout-fixtures.md)
owns authority and exclusions. Existing evidence and caches remain preserved.
