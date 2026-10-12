# Go v0.104 nested terminal conditional-test layout fixtures

Completed bounded experiment with an authorized storage-accounting memory repair,
2026-09-13, on saved `js/pipelang`, admitted at
`7a4ae4184e13bec67574a77a44b7fe8c2518970e`. Accepted language remains v0.113.0.
Implementation is uncommitted. Fresh acceptance supersedes the failed initial run;
its evidence remains preserved.

Eligible v0.104 layout native bundles use the existing exact binary Value/Trace
format and shared reader. The native checker reconstructs thirteen Boolean inputs
from each vector index using the independent oracle's `bits & (1 << bit) != 0`
mapping. Ordinary execution retains JSON fixtures. Source generation, current
independent expected values/traces, evaluator calls, HIR/Core/semantic determinism,
both native assertions and owning lifetimes remain unchanged. The shared reader,
batch engine, compiler and language production code are unchanged.

## Fresh sample measurements

Twelve owners cover partitions 8/9/10, 32/33/34, 112/113/114 and 192/193/194:
trees 1/4/14/24 crossed with residues 0/1/2, covering rotating 0/1/3 local sequences.
Every assigned subset and all 8,192 vectors remain. This is 12/200 owners, with no
full-family extrapolation or full-language campaign.

Both lanes use Go 1.25.13, the same repaired accountant, matched native dependency
preparation and audit settings. The JSON control adds only ordered input/value/
trace audit logging to the admitted source. Separate empty native artifact stores
supply fresh cold population; three subsequent warm passes execute every case
again. Warm pass two reverses lane order. All eight passes match generated-source
multisets, ordered oracle digests, subsets, 81 layouts, 663,552 vectors and 162 fresh
native executions per pass. All warm misses are zero.

| Twelve selected owners | JSON control | Binary fixtures |
| --- | ---: | ---: |
| Executable identities | 20 | 15 |
| Executable bytes including normal debug support | 108,652,252 | 78,459,054 |
| Cold summed unit time | 72.985 s | 65.900 s |
| Mean summed warm unit time | 44.088 s | 32.630 s |
| Mean summed warm native child time | 9.894 s | 3.286 s |

The sample avoids **30,193,198 executable bytes (27.7888%)**.
Mean summed warm unit time is 25.9890% lower. Warm unit sums are
44.887/42.639/44.737 seconds for control and 32.689/32.926/32.275 for candidate.
Host load was uncontrolled; this is sample evidence, not a general speedup claim.
Native child time includes fixture loading/checking, not just computation. Unit
sums exclude preparation, coordinator storage accounting and focused checks.
These are fresh-run measurements, not timings spliced from the failed attempt.

## Exact accounting repair and proof

The initial comparison passed semantic/artifact checks and all 117 individual units,
but the aggregate job recorded 2,032 memory.max events. A separate contained
diagnostic reproduced 113 events while initializing the storage inventory, before
Go execution or harness hashing. The coordinator reached its unchanged 512-MiB cap
while tracking 566,850 files and 13,849 directory watches. Both jobs removed their
process trees, with zero OOM/swap events. Their timings remain historical unaccepted
observations in `failure-report.md` and the original receipts.

The user selected a bounded shared-accountant repair and fresh acceptance.
`DiskBudget` now stores exact names as filesystem bytes and counters in an in-memory
SQLite index, keyed by directory identity. Ordinary unsigned counters use packed
records; exceptional integers and hardlink identities use exact JSON records.
The database creates no file. Directory deletion and sub-root totals query only
matching directory entries, avoiding reconstruction of every full inventory path.
Original arithmetic, deduplication, symlink/root checks, watches, overflow refusal,
population locks, storage ceilings and reserve policy match the admitted source
after removing the representation adapter and memory hook. All content validation
and current-source hashing remain in place.

The first compact-index retry passed all 117 units and resource checks but took
1,040.537 seconds because localized deletion reconstructed all paths. The user
approved stopping it; it had already completed and removed its tree, so no stop
was issued. A directory-scoped Python index still hit 107 memory.max events during
a fresh inventory; an in-memory SQLite index subsequently hit one event. Neither
failed inventory reached Go execution. All of these receipts remain historical;
none supplies timings for the accepted comparison.

The coordinator now checks memory during inventory and event processing. At
384 MiB it requests reclaim toward 256 MiB, acting only on its verified temporary
coordinator cgroup. It verifies the fixed job/coordinator limits first, refuses
insufficient read-back headroom, and never changes hard limits or deletes files.
[Linux memory.reclaim](https://docs.kernel.org/admin-guide/cgroup-v2.html#memory-interface-files)
provides scoped reclamation of reclaimable charges; requested bytes are not a
measurement of bytes actually freed. The fresh run recorded
0 requests totaling 0 requested bytes.
A separate one-byte kernel capability probe succeeded with unchanged limits/events.
The reclaim helper stays inactive outside the verified job coordinator.

Nineteen accounting/verification tests pass, including exact integer and filesystem
name boundaries; independent filesystem totals through hardlinks, sparse files,
symlinks, writes, replacements, directory moves and deletion; local-lookup
regression; population locks and monitor failures; and scoped reclaim identity,
fixed-limit verification, denied writes and insufficient/partial reclaim read-back.
A 50,000-entry differential against a reference dictionary passes under a 256-MiB
address-space ceiling and confirms that the database has no file. The existing
SQLite module, extension and library join the declared estate and fingerprints;
no installation occurred.

The final full-estate inventory/launcher probe passed with a 309,485,568-byte
coordinator peak and no limit events. Filesystem-cache and kernel charges varied
across runs, so these unmatched totals do not prove a general memory percentage
reduction. The fresh complete comparison is the terminal proof: coordinator peak
115,789,824 bytes, zero limit/OOM events and maximum observed storage-heartbeat age
0.561 seconds (existing ceiling: five).

All 117 fresh contained units and eighteen fixture-focused checks pass: exact
strings/nil/empty encoding, malformed fixtures, current-oracle refusal, freshness,
source/support invalidation, sealed snapshots, corrupt preparation, special
harness refusal, ordinary fixture/reuse checks, actual v0.104 JSON fallback,
v0.105/v0.106/v0.107/v0.108/v0.109 layouts, v0.109 independent roots and v0.111
subset/scope controls. All 35 measured binaries retain normal debug sections.
Structural review proves the audit-only control, unchanged source/oracle/evaluator
prefix and other functions, and the accepted v0.105 construction block with only
version identifiers changed. Go, containment/accounting Python and SQLite support
hashes stayed stable throughout the fresh run.

The fresh validation job took 492.961 seconds, with a
1,152,729,088-byte aggregate peak, zero memory-limit/OOM/swap events,
and verified process-tree removal. The 2-GiB aggregate, 512-MiB coordinator,
96-GiB storage cap, 8-GiB reserve and all unit/child/compiler limits are unchanged.
Jobs used narrowly reviewed host execution after sandbox user-systemd denial.
The fresh job time excludes the failed 512.795-second run, diagnostic jobs and
implementation/review time; it is not total investigation time.

## Retained cost and acceptance

The declared estate closed at 40,045,728,134 logical /
42,042,273,792 allocated file bytes; sampled peaks were
40,117,428,569 / 42,113,929,216.
Logical growth during the fresh job was 537,080,965 bytes.
The union includes this entire task (failed run, repair diagnostics and fresh retry),
all seven preceding construction experiments, shared Go cache, source/package
inputs, module/toolchain support, Python/readelf executables and SQLite runtime.
Task artifacts occupy about 1313.1 MB before final docs/acceptance; shared-cache growth is
separate. **Zero existing bytes were freed.** These are metadata samples, not exact
physical peaks; hardlinks are accounted for, shared extents are unidentified, and
later final documentation/acceptance records are outside the closing sample.

`E=/home/jamie/.codex/visualizations/2026/09/13/01a099d5-a3bc-74e3-8ac3-2540173fd02c/go-v104-layout-fixtures/`

`guarded/validation-receipt.json` owns fresh comparisons and unit results;
`guarded/validation-job.json` owns aggregate containment and cleanup;
`guarded/coordinator-memory.jsonl` records per-check coordinator counters.
`guarded/final-acceptance.json` binds final postimages, current binaries, passed
storage tests, repaired-accountant source, preserved failure evidence and protected
checkout state. `storage-memory-repair/` contains admitted sources, differential
tests, structural review and contained diagnostics. Existing failed receipts and
`failure-report.md` remain immutable. The
[objective record](../agents/tasks/pipelang-reactive-application-language/go-v104-layout-fixtures.md)
owns authority and exclusions.

Other fixture families, content-validation reuse, C++, cleanup, installs, raised
limits, Git mutation and publication remain outside this completed objective.
No further performance scope is selected; engine/package boundaries are preserved.
