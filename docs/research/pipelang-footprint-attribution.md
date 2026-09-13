# PipeLang footprint attribution and corrective plan

Planning-only investigation, 2026-09-13 UTC, on saved `js/pipelang` at
`616aab5dcc4ad5c31a55a4a2878c2a360b0cc78a`. Accepted language remains v0.113.0.
No backend/harness implementation, cleanup, install, campaign, limit increase or
Git mutation occurred. **Zero bytes freed.**

## Findings

The executable increase is predominantly more active generated test artifacts,
not larger average binaries or an obsolete cache that can simply be discarded.
The broader footprint also contains a concrete packaging defect: a retained
application integration test binary embeds **1,262,790,433 distinct payload bytes
of ignored Tauri build output**. Fix that input boundary and inclusive budget
enforcement first; evaluate native output as a separate, bounded second objective.

Evidence directory `E`:
`/home/jamie/.codex/visualizations/2026/09/13/01a098cd-9038-7c62-81d8-7e9140a7ceef/footprint`.
`inventory.py`, `elf_accounting.py` and `embed_accounting.py` retain the analysis
methods. Existing JSON identities were read; file sizes/inodes/allocated blocks
were inspected. ELF accounting requested 31,086,526 bytes of headers/tables across
15,142 binaries; embedded-file accounting requested another 1,340,382 bytes.
Neither operation read or rehashed executable payloads. These are attribution
measurements, not refreshed semantic, content-integrity or physical-I/O proof.

## Comparable scopes

GB = 1,000,000,000 bytes; GiB = 1,073,741,824 bytes. The historical phrase
“approximately 14 GiB” actually refers to **14.391 GB / 13.402 GiB** of executable
files. Its allocated figure, including metadata, was 14,413,168,640 bytes.

| Scope | Executable identities | Logical executable bytes | Meaning |
| --- | ---: | ---: | --- |
| Early `complete-artifacts` | 3,079 | 14,390,546,687 | Historical cache inventory; old `/tmp` inputs unavailable now |
| Accepted retained v0.109 inventory | 12,032 | 56,788,504,699 | Existing `accepted-artifacts.json` in the durable cache |
| v0.112 terminal inventory | 15,121 | 70,884,194,413 | All 12,032 prior keys survive with unchanged recorded digests |
| v0.113 terminal inventory and profile references | 15,142 | 70,973,989,613 (66.100 GiB) | All v0.112 keys survive; 21 additional keys, 89,795,200 bytes |
| Live same executable directory | 15,174 | 71,139,346,441 | 32 keys / 165,356,828 bytes beyond that v0.113 profile |

Sources: [early accounting](../agents/tasks/pipelang-reactive-application-language/conformance-execution-performance.md),
`E/generation-comparison.json`, `E/inventory.json`; historical v0.113 evidence
`/home/jamie/.codex/visualizations/2026/09/12/01a09387-ddcf-71d1-94b7-9395966a4500/verification-data/campaigns/terminal-2/suite/{artifacts,schedule-profile,summary}.json`.

Count increased 4.918 times from the early snapshot; mean file size increased
only 0.288%. This is an arithmetic decomposition, not a matched compiler experiment.
Different language/corpus generations prevent an exact old-case/new-case causal
join. v0.109 to v0.112 adds 3,089 keys / 14,095,689,714 bytes, with no removed keys
or changed recorded digests on common keys. All 15,142 v0.113 keys remain present
live with matching sizes. No duplicate recorded binary digest occurs among the
15,174 live entries. Digest records were not freshly validated. The 32 extra keys
are unreferenced by this historical profile, **not proven stale or deletable**.

### Active coverage and executable contents

| v0.113 family | Identities | Exclusive binary bytes |
| --- | ---: | ---: |
| v0.109 combined selector layouts | 1,682 | 9,026,469,208 |
| v0.111 terminal-leaf selector subsets | 1,904 | 8,889,709,544 |
| v0.109 compiler-memory cases | 1,755 | 7,528,655,949 |
| v0.108 inner-selector layouts | 566 | 3,000,046,200 |
| v0.107 selector layouts | 566 | 2,986,592,048 |

Post-v0.96 named families exclusively reference **59,701,615,957 bytes**;
v0.96/earlier and unversioned families exclusively reference 10,578,962,599 bytes.
Another 693,411,057 bytes are shared across family references, counted once in the
total. Family identity counts can overlap and must not be summed as unique objects.
The newer family names explain where active bytes reside; they do not prove every
byte was necessary or attribute all old-to-new changes to language semantics.

`E/elf-accounting.json` accounts for the complete 70,973,989,613 bytes:

| Stored section class | Bytes | Share |
| --- | ---: | ---: |
| Machine code `.text` | 22,124,534,950 | 31.17% |
| DWARF debug sections, already-compressed sizes | 17,668,939,281 | 24.89% |
| Go PC/line metadata `.gopclntab` | 16,388,643,320 | 23.09% |
| Symbol/string tables | 3,748,155,079 | 5.28% |
| Other sections | 10,887,793,182 | 15.34% |
| Headers, alignment and other non-section bytes | 155,923,801 | 0.22% |

These are repeated categories, not measured byte-identical duplicates. Code and
metadata mix generated programs, Go runtime, independent checking code and the Go
test framework. A complete runtime-versus-program symbol attribution is unavailable.
None of the debug/support total is automatically removable; preserve exact debug
and executable proof. Whole-file deduplication alone has no measured opportunity
inside this inventory. Shared runtime/check construction remains a design question,
especially where independent compiler probes and source/lifetime limits prohibit
ordinary bundling.

### Broader storage and retained generations

The historical **104,899,071,378-byte** figure is the suite gate's four-root scope:
shared Go cache, executable cache, native build cache and that suite output.
It is neither a unique physical-byte measurement nor the complete evidence estate.
Live comparable roots total **105,989,847,050 bytes (98.711 GiB)**:

| Live component | Logical bytes |
| --- | ---: |
| Shared Go build cache | 31,346,940,673 |
| Executable cache including records/locks | 71,142,350,893 |
| Native build cache | 3,022,050,563 |
| Historical v0.113 suite output retained today | 478,504,921 |

Current components cannot be substituted as the exact historical component split.
The historical difference between binary bytes and the broader gate was
33,925,081,765 bytes; live metadata confirms build caches dominate that kind of
residual. Cache contents were not attributed to individual past builds or declared
safe to evict.

The entire durable v0.109 cache root now holds **108,902,706,060 logical bytes**
and 110,911,336,448 inode-deduplicated allocated bytes. It also retains integration
runs, isolated compiler outputs and old suite evidence. Additional inspected,
nonoverlapping roots are v0.113 `verification-data` (2,617,053,239 bytes), identity
research (3,033,397,810), pairing research (2,963,826,268), recovery work (339,811,051),
default Go cache (270,807,784), and pinned Go toolchain (189,412,951).
Their union is **118,317,015,163 logical bytes**. This is a named partial estate;
older campaign roots, other toolchains, source checkout and other research are
outside it. Do not sum this union with its nested suite/cache components above.
Allocated blocks deduplicate hardlinks within each root; shared/reflink extents
and cross-root inode overlap are not a measured unique-device total.

For one v0.113 terminal campaign, `E/evidence-categories.json` records:

- Suite: 147,823,395 fixture-input bytes, 237,710,940 JSON-record bytes,
  75,687,111 log/output bytes and 17,283,344 executable bytes.
- Matrix: 743,171,480 bytes, predominantly retained compiler outputs such as
  `target.a` (the 676,734,099-byte “other” category is not all proven duplicates).
- Integration: 1,321,735,970 bytes, of which 1,318,247,501 are executables.
- Failed `terminal` and successful `terminal-2` evidence coexist. Recovery's
  `candidate-full` and `candidate-interrupted` retain 161,674,817 and 133,273,555
  bytes. These preserved generations are outside the active executable-cache total.

Historical temporary peak storage is unknown: end-of-run sizes do not measure
compiler/link scratch or simultaneous copies. No current inventory can recover it.

### Confirmed generated-input amplification

The v0.113 integration receipt identifies the 1,310,717,217-byte binary as
`go test -c ./src/lib/application`. Its `.rodata` occupies 1,295,166,105 bytes.
The root [embed declaration](../../embed.go) recursively includes `packages`;
application imports infrastructure, which imports this embedded filesystem.

Reading that binary's `dockpipe.BundledFS.files` table, using the pinned Go
`src/embed/embed.go` layout, finds 2,638 embedded files. The ignored
`packages/pipeon/apps/pipeon-desktop/src-tauri/target/` subtree contributes
1,688,846,344 logical embedded-file bytes but **1,262,790,433 distinct payload-span
bytes** after compiler sharing. Other embedded files occupy 29,418,290 distinct
payload-span bytes. Rust `.rlib`, `.rmeta`, shared libraries and build executables
are present. This is direct artifact evidence, not an inference from `.rodata` alone.

The original target tree still holds 1,689,304,708 logical bytes / 1,273,720,832
inode-unique allocated bytes (`E/runtime-support.json`). Originals, embedding and
build-cache copies must all be charged. Potential avoided payload in a future
matching application binary is about 1.263 GB; actual replacement size and savings
need a build. Existing copies and source artifacts remain protected. Other complete
campaign integration roots have comparable sizes, but their embedding tables and
cross-generation content equality were not independently joined here.

## Budget provenance and why growth passed

`E/budget-history.json` and source history establish:

| Recorded run | Configured cap | Recorded retained bytes |
| --- | ---: | ---: |
| Verification round 2, 6,188 cases | 96 GiB | 84,433,423,618 |
| v0.111 repaired terminal, 8,791 cases | 104 GiB | 102,971,512,220 |
| v0.112 terminal, 8,837 cases | 112 GiB | 104,172,433,801 |
| v0.113 terminal, 8,859 cases | 112 GiB | 104,899,071,378 |

The checked-in default is still **96 GiB**, introduced with `DiskBudget` in
`3154030b`; history of the relevant source paths shows no default increase.
104 and 112 were invocation overrides. In session `01a08cf5-e6c6-7331-9956-f854d2c59619`,
the assistant announced 104 GiB to preserve the first campaign and cited about
293 GiB free (rollout line 1639; invocation line 1704). v0.112 commands used 112
from focused testing onward (session `01a09287-8762-7e02-a5ca-da7b5be9ddea`, lines
217 and 455); v0.113 and later commands inherited 112. User messages in these
examined sessions selected and approved language slices. **No explicit approval
of these numeric budget increases was found in the examined records.** This is
bounded provenance, not proof that no authorization exists elsewhere. Assistant
announcements, generic slice approval, successful host execution and available
capacity do not establish informed acceptance of a footprint regression.

The [current implementation](../../tests/containedexec/budget.py) watches file
sizes, checks an absolute configured cap and an 8-GiB free-space reserve, and
preserves files on exhaustion. Its admission headroom becomes the new-population
allowance. It has no approved-baseline identity, permitted growth, cap-change
authorization or product-footprint check. By v0.112, even initial scoped storage
exceeded the source-default 96 GiB; the override admitted it.

Scope gaps: the [suite](../../tests/containedexec/pipelang_suite.py) includes its
own output, not sibling matrix/integration/controller directories, earlier campaign
outputs, the separate test build store, checkout inputs or toolchain installation.
The [integration driver](../../tests/containedexec/integration.py) does not apply
that four-root disk gate. Thus passing it cannot certify total verification size.
Pinning all evidence preserves proof but has no bounded retention lifecycle.

## Verification reads and ranked actions

Subsequent bounded work: [package-input repair](../agents/tasks/pipelang-reactive-application-language/package-input-budget-repair.md)
completed, followed by a [v0.111 Go artifact construction experiment](pipelang-go-artifact-reduction.md).
The latter reduced sampled executable bytes by 8.7373% with unchanged identities;
it does not close the broader count/retention problem. The original ranking below
remains the attribution roadmap, not fresh implementation authority.

The subsequent [v0.111 scope-layout fixture experiment](pipelang-go-scope-fixtures.md)
reduced its twelve-owner sample from 18 binaries / 100,032,017 bytes to 12 /
66,181,484 bytes under unchanged lifetime/source limits. This is bounded sample
evidence; the remaining families and retention problem remain separate work.

The later [v0.109 layout fixture experiment](pipelang-go-v109-layout-fixtures.md)
reduced its twelve-owner executable sample by 8.3902%, keeping 13 identities.
Three warm comparisons improved, while cold unit sums regressed. This preserves
all sampled vectors, debug support and fresh execution; it does not close the
remaining identity-count, full-family or retention questions.

The subsequent [v0.108 inner-selector layout experiment](pipelang-go-v108-layout-fixtures.md)
reduced its twelve-owner sample from 15 executables / 87,074,736 bytes to 12 /
67,569,638 bytes (22.4004% fewer bytes). All matched cold/warm passes preserve
81 layouts, 331,776 vectors and 162 fresh native executions. Mean warm unit time
fell 23.1581%; all limits and debug support remain unchanged. This remains bounded
sample evidence, with no full-family extrapolation or existing storage freed.

The subsequent [v0.107 selector value-arm experiment](pipelang-go-v107-layout-fixtures.md)
reduced its twelve-owner sample from 15 executables / 86,531,176 bytes to
12 / 67,017,270 bytes (22.5513% fewer bytes). All eight matched passes
preserve 81 layouts, 331,776 vectors and 162 fresh native executions per pass.
Mean warm unit time fell 18.7663%; all limits and debug support remain unchanged.
This is bounded sample evidence; no full-family extrapolation or existing storage
freed is claimed.

The subsequent [v0.106 Boolean-selector layout experiment](pipelang-go-v106-layout-fixtures.md)
reduced its twelve-owner sample from 12 executables / 71,101,460 bytes to
12 / 65,737,942 bytes (7.5435% fewer bytes). All eight matched passes
preserve 81 layouts, 165,888 vectors and 162 fresh native executions per pass.
Mean warm unit time fell 14.0429%; all limits and debug support remain unchanged.
This remains bounded sample evidence; no full-family extrapolation or existing
storage freed is claimed.

The subsequent [v0.105 depth-three conditional-test layout experiment](pipelang-go-v105-layout-fixtures.md)
reduced its twelve-owner sample from 21 executables / 115,290,272 bytes to
15 / 80,384,174 bytes (30.2767% fewer bytes). All eight matched passes
preserve 81 layouts, 663,552 vectors and 162 fresh native executions per pass.
Mean warm unit time fell 19.0561%; all limits and debug support remain unchanged.
This remains bounded sample evidence; no full-family extrapolation or existing
storage freed is claimed.

The v0.113 profile contains 73,904,076,510 logical binary-reference bytes over
70,973,989,613 distinct binary bytes: **1.0413 times** for one per-case reference pass.
The recovery diagnostic's approximately 74.1 GB includes its own artifact scope;
these should not be forced equal. Every admission/consumption pass rechecks content
in [campaign.py](../../tests/containedexec/campaign.py), so repeated phases multiply
reads even when within-pass duplication is modest. Total recovery-pass physical I/O
and temporary sealed-file peak are unavailable.

The source-derived historical toolchain estimate is **1.296 TB logical reads**
(171,984,742 bytes × 7,536 singleton artifact-using processes), not current paired
physical traffic. In the prior matched 48-case attribution sample, toolchain
content read/hash took 19.51 summed worker-seconds; cache record/binary checks took
1.80 and copy/hash/seal 0.49. Full warm identity/verification was 4,088 seconds,
versus 11 seconds of retained-bundle miss compilation. The measured bucket also
contains metadata, locking and keys; do not call it all hashing or CPU time.
Current pairing/buffer reuse has already addressed part of this cost. Physical
`io.stat` was unavailable in the attribution receipt. See
[measured performance](pipelang-performance-compression.md).

| Rank | Proposed action | Evidence and acceptance boundary |
| --- | --- | --- |
| 1 | Fix authored package embedding and add inclusive storage admission | Remove generated build output from future packaged inputs, preserving legitimate assets and all existing files. Add a regression fixture proving ignored generated output cannot enter a bundle; govern the complete campaign estate and cap changes. Directly targets the proven 1.263-GB payload amplification. |
| 2 | Reduce active artifact proliferation | Start with v0.109/v0.111 families; compare support-aware construction within existing source/lifetime limits. Preserve every vector, fresh execution and independent compiler probe. The 59.70-GB newer-family set is a target population, not promised savings. |
| 3 | Run the bounded C++ pilot | Test inclusive application and harness costs using the [pilot contract](../concepts/pipelang-native-backends.md#bounded-c-pilot-proposal). It may reduce overhead per identity; it does not fix count, retention or frontend toolchain hashing. |
| 4 | Safely reduce repeated validation reads | Profile current paired lifetimes; investigate validated immutable lifetime reuse with adversarial invalidation and bounded memory. No path/mtime trust, skipped consumption validation or receipt-only execution. |
| 5 | Govern build caches and historical retention | Inventory owners/references, prevent unbounded new generations, then separately seek archive/cleanup authority if useful. Do not prune now. Active compression remains opt-in; prior slowdown disqualifies making it the default. |

First implementation recommendation: **bounded package-input and inclusive-budget
repair**, with exact asset compatibility, generated-input refusal, preserved files,
source-scoped package behavior, and a matched application-build/storage receipt.
Do not turn this into generic engine package knowledge or a compiler rewrite.
Implementation and numerical-budget adoption are not authorized by this report.

## Concrete budget proposal for review

These numbers are proposed gates, not new configuration or an assertion of
feasibility. The existing workload fails the intended product targets; that should
be visible, rather than converted into a pass by silently widening a cap.

| Ledger | Proposed absolute gate | Growth rule |
| --- | --- | --- |
| Active executable set including required debug/runtime support | 16 GiB | Same corpus: no increase in bytes or identities. Changed corpus: explicit per-feature allocation and a new reviewed baseline; no automatic allowance per added test. |
| Active development/verification estate | 32 GiB total: 16 executable, 8 Go cache, 4 native intermediates, 2 evidence/fixtures, 2 compiler/tool/support | Every category and total must pass. Share dependencies once in the union; charge their full cost for a standalone deployment. |
| Frozen preserved historical estate | Exact metadata baseline of all declared roots; separate from the 32-GiB target | Zero unapproved growth. Existing oversized data stays intact and visibly over target; no cleanup or silent exclusions. Missing roots mean incomplete admission. |
| New bounded pilot experiment | At most 2 GiB retained and 3 GiB incremental simultaneous peak, across both backends, controls, repetitions and evidence | A byte of new cache/support anywhere counts. Preserve originals and add them to the combined estate; unused space is not authority. |
| Each pilot application | 8 MiB inclusive executable, debug, assets and transitive runtime closure | Must not exceed its matched Go application's inclusive bytes; report union amortization separately. |
| Verification logical reads | Per immutable lifetime, target ≤1.10 × distinct admitted artifact bytes plus enumerated protocol metadata | Each independent trust boundary is separately budgeted; additional full passes require a named necessity. This is a future optimization target, never permission to weaken current checks. |

Keep the existing 8-GiB free-space reserve as a separate emergency floor. For new
population, reserve estimated worst-case outputs plus intermediates atomically
before workers start; stop before exhaustion, retain evidence and report the
overage. Use logical bytes **and** allocated high-water measurements, canonical
nonoverlapping roots, hardlink/shared-extent caveats, and included dependency lists.
Do not treat sampled peak as an exact peak unless allocation accounting guarantees it.

An approved baseline should bind scope/corpus/coverage identities, toolchain,
backend/runtime, flags, artifact generation, absolute limits and exact authorized
growth. Reject a higher CLI cap without a separately recorded decision naming the
old/new amounts and reason. Reject stale baseline identities; do not replace them
with whichever cache is currently larger. On unchanged-corpus warm replay, require
zero unexpected executable misses and no identity-count growth; bound evidence
growth by its declared experiment allocation. No limit increase, narrowed corpus,
discarded debug data or hidden external storage may cure a failing gate.

## Completion and limits

Attribution, tool inventory, corrective ordering, budgets and pilot/comparison
design are delivered. Only planning docs changed. Existing recovery proof remains
completed; no full fresh campaign or new speed/size win is claimed. Old `/tmp`
evidence, cross-generation cache content equality, unique physical-device storage,
historical peak scratch and physical verification I/O remain explicit gaps.
