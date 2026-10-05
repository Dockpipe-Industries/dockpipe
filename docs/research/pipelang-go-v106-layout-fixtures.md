# Go v0.106 terminal Boolean-selector layout fixtures

Completed bounded experiment, 2026-09-13, on saved `js/pipelang`, admitted at
`a0766a9293edb6391482c9137ed255c18cf2916b`. Accepted language remains v0.113.0.

Eligible v0.106 layout native bundles use the existing exact length-prefixed
Value/Trace fixtures and shared binary reader. The native checker reconstructs
eleven Boolean inputs from each vector index using the independent oracle's
`bits & (1 << bit) != 0` mapping. Ordinary execution retains JSON fixtures.
Current expected values/traces, source emission, HIR/Core/semantic determinism,
evaluator calls, both native assertions and owning lifetimes are preserved.
The shared reader, batch engine, compiler and language contracts are unchanged.

The frozen sample crosses trees 1/4/14/24 with partition residues 0/1/2:
partitions 8/9/10, 32/33/34, 112/113/114 and 192/193/194. Every owner retains all
assigned subsets and all 2,048 vectors. The sample covers rotating 0/1/3 local
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
| Distinct executable identities | 12 | 12 |
| Executable bytes including normal debug/runtime support | 71,101,460 | 65,737,942 |
| Source layouts / vectors per pass | 81 / 165,888 | 81 / 165,888 |
| Fresh native package executions per pass | 162 | 162 |
| Cold summed unit time | 41.621 s | 38.266 s |
| Mean summed warm unit time | 17.135 s | 14.729 s |
| Mean summed warm native child time | 3.038 s | 1.411 s |

The sample avoids **5,363,518 executable bytes (7.5435%)**.
Mean warm unit time is 14.0429% lower. Warm unit sums are
16.922/17.068/17.415 seconds for control and 14.867/14.656/14.663 for candidate.
Host load was uncontrolled; this is an instrumented sample, not a general speedup
claim. Native child time includes fixture loading/checking and generated execution,
not pure computation. Unit sums exclude bootstrap, dependency preparation,
controller storage accounting and focused checks. No full-language campaign ran.

## Verification and retained cost

Sixteen focused checks pass: exact strings/nil/empty encoding, malformed binary
fixtures, current-oracle refusal and freshness, source/support invalidation, sealed
snapshots, corrupt preparation, special harness refusal, ordinary fixture/reuse
checks, actual v0.106 JSON fallback, v0.107/v0.108/v0.109 layout controls, v0.109
independent roots and v0.111 subset/scope controls. All
24 measured binaries retain normal debug sections.
Structural review proves an audit-only control and unchanged source/oracle/evaluator
prefix and other functions. Validation verifies stable Go source bytes; batch and
shared-reader implementations remain unchanged.

The validation job took 313.330 seconds with a
965,799,936-byte aggregate peak, zero OOM/max/swap events and
verified process-tree removal. All 115 contained units passed. The 2-GiB aggregate,
512-MiB coordinator and unit/child/compiler ceilings were unchanged. The job ran
through narrowly reviewed host execution after sandbox user-systemd denial.

The declared estate closed at 37,765,663,728 logical /
39,703,080,960 allocated file bytes; sampled peaks were
37,803,154,474 / 39,740,948,480
bytes. The union includes this task, all five preceding construction experiments,
the shared Go build cache, source/package inputs, module/toolchain support and
Python/readelf executables. Logical growth from the controller's initial inventory
was 525,572,965 bytes. Task artifacts
occupy about 377.8 MB before final documentation/acceptance records;
additional shared Go build-cache growth is separate. **Zero existing bytes were freed.**
These are sampled file metadata totals, not exact simultaneous allocation or unique
physical-device usage; hardlinks are deduplicated and shared extents are unidentified.
Later documentation/acceptance records are outside the closing job sample. The
96-GiB cap and 8-GiB reserve remain unchanged; proposed numerical targets and
retention/cleanup policy remain separate.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a099b5-6dda-7260-b950-d7d298c5a29b/go-v106-layout-fixtures/`

`validate.py` freezes comparisons/checks; `review.py` independently rechecks the
structural transformations in `source-review.json` against `admitted-matrix.go` and
the audit-only control. `validation-receipt.json` owns measurements, source hashes,
debug evidence and focused results. `validation-job.json` owns aggregate containment
and process-tree removal; `storage/storage-budget.json` owns the closing sample.
`final-acceptance.json` binds final postimages and validation evidence after
`accept.py` rechecks all pass inventories, current sources/binaries and protected
checkout state. The [objective record](../agents/tasks/pipelang-reactive-application-language/go-v106-layout-fixtures.md)
owns authority and exclusions. Existing evidence/caches remain preserved.
