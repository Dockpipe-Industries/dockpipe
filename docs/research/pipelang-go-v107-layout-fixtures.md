# Go v0.107 terminal selector value-arm layout fixtures

Completed bounded experiment, 2026-09-13, on saved `js/pipelang`, admitted at
`de7f5fd014682c5c66b4eb34c89e3bdf1387b3fc`. Accepted language remains v0.113.0.

Eligible v0.107 layout native bundles use the existing exact length-prefixed
Value/Trace fixtures and shared binary reader. The native checker reconstructs
twelve Boolean inputs from each vector index using the independent oracle's
`bits & (1 << bit) != 0` mapping. Ordinary execution retains JSON fixtures.
Current expected values/traces, source emission, HIR/Core/semantic determinism,
evaluator calls, both native assertions and owning lifetimes are preserved.
The shared reader, batch engine, compiler and language contracts are unchanged.

The frozen sample crosses trees 0/4/14/24 with all three true/false/both arm families:
partitions 1/9/17, 97/105/113, 337/345/353 and 577/585/593. Every owner retains all
assigned subsets and all 4,096 vectors. This is 12/600 owners, with no full-family
extrapolation or full-language campaign.

An audit-only overlay augments the admitted JSON control with ordered input/value/
trace digests. Both lanes use the existing Go 1.25.13 toolchain, matched native
dependency preparation, audit settings and containment. Verification compares cold
population and three fresh warm passes, reversing lane order on warm pass two.
All eight passes match generated-source multisets, ordered oracle digests, subset
inventory and fresh native execution counts. Every warm pass has zero misses.

## Measurements

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Distinct executable identities | 15 | 12 |
| Executable bytes including normal debug/runtime support | 86,531,176 | 67,017,270 |
| Source layouts / vectors per pass | 81 / 331,776 | 81 / 331,776 |
| Fresh native package executions per pass | 162 | 162 |
| Cold summed unit time | 59.971 s | 50.882 s |
| Mean summed warm unit time | 27.838 s | 22.614 s |
| Mean summed warm native child time | 5.779 s | 2.159 s |

The sample avoids **19,513,906 executable bytes (22.5513%)**.
Mean warm unit time is 18.7663% lower. Warm unit sums are
27.640/27.733/28.142 seconds for control and
22.642/22.405/22.795 for candidate.
Host load was uncontrolled; this is an instrumented sample, not a general speedup
claim. Native child time includes fixture loading/checking and generated execution,
not pure computation. Unit sums exclude bootstrap, dependency preparation,
controller storage accounting and focused checks. No full-language campaign ran.

## Verification and retained cost

Fifteen focused checks pass: exact strings/nil/empty encoding, malformed binary
fixtures, current-oracle refusal and freshness, source/support invalidation, sealed
snapshots, corrupt preparation, special harness refusal, ordinary fixture/reuse
checks, actual v0.107 JSON fallback, v0.108/v0.109 layout controls, v0.109 independent
roots and v0.111 subset/scope controls. All 27 measured binaries retain normal
debug sections. Structural source review proves an audit-only control and unchanged
source/oracle/evaluator prefix and other functions. Validation verifies stable Go
source bytes; batch and shared-reader implementations remain unchanged.

The complete validation job took 401.944 seconds with a
1,162,326,016-byte aggregate peak, zero OOM/max/swap events and verified
process-tree removal. All 114 contained units passed. The 2-GiB aggregate,
512-MiB coordinator and unit/child/compiler ceilings were unchanged. The job ran
through narrowly reviewed host execution after sandbox user-systemd denial.

The declared estate closed at 37,239,961,344 logical /
39,163,330,560 allocated file bytes; sampled peaks were
37,305,520,669 / 39,229,095,936 bytes. The union includes
this task, all four preceding construction experiments, the shared Go build cache,
source/package inputs, module/toolchain support and Python/readelf executables.
Logical growth from the controller's initial inventory was
552,833,723 bytes. About 397 MB remains in task artifacts,
with additional shared Go build-cache growth. **Zero existing bytes were freed.**
These are sampled file metadata totals, not exact simultaneous allocation or unique
physical-device usage; hardlinks are deduplicated and shared extents are unidentified.
Later documentation/acceptance records are outside the closing job sample. The
96-GiB cap and 8-GiB reserve remain unchanged; proposed numerical targets and
retention/cleanup policy remain separate.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a099a3-db29-75f0-8798-1c8ef79b6842/go-v107-layout-fixtures/`

`validate.py` freezes comparisons/checks; `source-review.json` records structural
review against `admitted-matrix.go` and the audit-only control overlay.
`validation-receipt.json` owns measurements, source hashes, debug evidence and
focused results. `validation-job.json` owns aggregate containment and process-tree
removal; `storage/storage-budget.json` owns the closing storage sample.
`final-acceptance.json` binds final postimages and validation evidence.
The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v107-layout-fixtures.md)
owns authority and exclusions. Existing evidence/caches remain preserved.
