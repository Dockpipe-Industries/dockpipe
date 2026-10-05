# Go v0.105 depth-three terminal conditional-test layout fixtures

Completed bounded experiment, 2026-09-13, on saved `js/pipelang`, admitted at
`793c5c3a5595ab1e8b3678545bea963dfa923ce7`. Accepted language remains v0.113.0.

Eligible v0.105 layout native bundles use the existing exact length-prefixed
Value/Trace fixtures and shared binary reader. The native checker reconstructs
thirteen Boolean inputs from each vector index using the independent oracle's
`bits & (1 << bit) != 0` mapping. Ordinary execution retains JSON fixtures.
Current expected values/traces, source emission, HIR/Core/semantic determinism,
evaluator calls, both native assertions and owning lifetimes are preserved.
The shared reader, batch engine, compiler and language contracts are unchanged.

The frozen sample crosses trees 1/4/14/24 with partition residues 0/1/2:
partitions 8/9/10, 32/33/34, 112/113/114 and 192/193/194. Every owner retains all
assigned subsets and all 8,192 vectors. The sample covers rotating 0/1/3 local
sequences and differing tree sizes. This is 12/200 owners, with no full-family
extrapolation or full-language campaign.

An audit-only overlay augments the admitted JSON control with ordered input/value/
trace digests. Both lanes use existing Go 1.25.13, matched native dependency
preparation, audit settings and containment. Verification compares cold population
and three fresh warm passes, reversing lane order on warm pass two. All eight
passes match generated-source multisets, ordered oracle digests, subsets and fresh
native executions. Every warm pass has zero misses.

## Measurements

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Distinct executable identities | 21 | 15 |
| Executable bytes including normal debug/runtime support | 115,290,272 | 80,384,174 |
| Source layouts / vectors per pass | 81 / 663,552 | 81 / 663,552 |
| Fresh native package executions per pass | 162 | 162 |
| Cold summed unit time | 83.446 s | 71.832 s |
| Mean summed warm unit time | 46.828 s | 37.904 s |
| Mean summed warm native child time | 10.427 s | 3.542 s |

The sample avoids **34,906,098 executable bytes (30.2767%)**.
Mean warm unit time is 19.0561% lower. Warm unit sums are
45.448/47.164/47.872 seconds for control and 38.942/38.817/35.954 for candidate.
Host load was uncontrolled; this is an instrumented sample, not a general speedup
claim. Native child time includes fixture loading/checking and generated execution,
not pure computation. Unit sums exclude bootstrap, dependency preparation,
controller storage accounting and focused checks. No full-language campaign ran.

## Verification and retained cost

Eighteen focused checks pass: exact strings/nil/empty encoding, malformed binary
fixtures, current-oracle refusal and freshness, source/support invalidation, sealed
snapshots, corrupt preparation, special harness refusal, ordinary fixture/reuse
checks, actual v0.105 JSON fallback, v0.106/v0.107/v0.108/v0.109 layout controls,
v0.109 independent roots, v0.111 subset/scope controls and the unchanged v0.105
expression-shape family. All
36 measured binaries retain normal debug sections.
Structural review proves an audit-only control and unchanged source/oracle/evaluator
prefix and other functions. Validation verifies stable Go source bytes; batch and
shared-reader implementations remain unchanged.

The validation job took 551.849 seconds with a
1,092,399,104-byte aggregate peak, zero OOM/max/swap events and
verified process-tree removal. All 117 contained units passed. The 2-GiB aggregate,
512-MiB coordinator and unit/child/compiler ceilings were unchanged. The job ran
through narrowly reviewed host execution after sandbox user-systemd denial.

The declared estate closed at 38,383,462,447 logical /
40,335,581,184 allocated file bytes; sampled peaks were
38,456,084,275 / 40,408,322,048
bytes. The union includes this task, all six preceding construction experiments,
the shared Go build cache, source/package inputs, module/toolchain support and
Python/readelf executables. Logical growth from the controller's initial inventory
was 617,652,169 bytes. Task artifacts
occupy about 457.6 MB before final documentation/acceptance records;
additional shared Go build-cache growth is separate. **Zero existing bytes were freed.**
These are sampled file metadata totals, not exact simultaneous allocation or unique
physical-device usage; hardlinks are deduplicated and shared extents are unidentified.
Later documentation/acceptance records are outside the closing job sample. The
96-GiB cap and 8-GiB reserve remain unchanged; proposed numerical targets and
retention/cleanup policy remain separate.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a099c1-826a-7420-bae9-9f8c84acf922/go-v105-layout-fixtures/`

`validate.py` freezes comparisons/checks; `review.py` independently rechecks the
structural transformations in `source-review.json` against `admitted-matrix.go` and
the audit-only control. `validation-receipt.json` owns measurements, source hashes,
debug evidence and focused results. `validation-job.json` owns aggregate containment
and process-tree removal; `storage/storage-budget.json` owns the closing sample.
`final-acceptance.json` binds final postimages and validation evidence after
`accept.py` rechecks all pass inventories, current sources/binaries and protected
checkout state. The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v105-layout-fixtures.md)
owns authority and exclusions. Existing evidence/caches remain preserved.
