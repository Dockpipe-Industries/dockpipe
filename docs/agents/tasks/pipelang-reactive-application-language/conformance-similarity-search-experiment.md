# Similarity search and exact-delta reconstruction experiment

State: **completed research experiment; unpromoted**. Date: 2026-09-07.

Latest follow-up: a third round tested original DEFLATE decision transcripts on
12 fixed samples plus all 16 references. Their encoded total fell **45.37 MB to
27.37 MB**, with exact bytes and a measured reconstruction cost. This is sample
component evidence; the latest full 200-object native replay candidate remains
**307.47 MB** encoded code and **454.82 MB** complete representation. See the
research sections below; earlier measurements remain historical evidence.

The user requested broader research into searching algorithms and an integrated
experiment, without assuming fuzzy matching or stronger compression would succeed.
This continues TASK-021's bounded performance objective. It does not select a new
language contract or change production compiler/cache behavior.

## Result

The combined prototype stores the same 200 v0.91 native binaries in **326,456,238
bytes**, including all 16 reference binaries. Independent Zstandard level 12 compression
uses **573,756,821 bytes**; raw executables occupy **1,302,426,120 bytes**. The candidate
saves **43.1% versus the same-level compressed control**, or **74.9% versus raw code**.
These are encoded-code totals, not complete dependency-inclusive representation sizes.

Every original ELF byte, including DWARF, reconstructs exactly. Current expected values
and ordered traces execute in fresh native processes with fresh fixture directories;
no pass outcome or mutable program state is reused.

| Final matched family replay | First | Second |
| --- | ---: | ---: |
| Original retained executables, grouped schedule | 13.416 s | 12.915 s |
| Exact deltas, resident codec, same grouped schedule | 14.072 s | 13.216 s |

Order is raw/delta/delta/raw. The observed additional wall time is **0.30–0.66 seconds**,
about **3.6% on the paired means**. This is a storage/performance tradeoff, not proven
zero-cost reconstruction or a whole-suite speedup. Shared-host noise remains material;
two mirrored runs do not establish a universal penalty or statistical equivalence.
No comparison against older runs with different validation establishes a new speed record.

## Research and implementation choices

| Research | Idea used or assessed |
| --- | --- |
| [Broder: resemblance sketches](https://www.cs.princeton.edu/courses/archive/spring13/cos598C/broder97resemblance.pdf) | Approximate set resemblance using compact fingerprints; approximate search never authorizes approximate execution bytes. |
| [Douglis/Iyengar: resemblance-selected deltas](https://www.usenix.org/conference/2003-usenix-annual-technical-conference/application-specific-delta-encoding-resemblance) | Select a reference by measured resemblance instead of assuming filename relationships. |
| [Finesse](https://www.usenix.org/conference/fast19/presentation/zhang) | Local-region features as a cheaper resemblance signal; prototype is inspired by locality, not a reproduction of its full superfeature algorithm. |
| [FastCDC](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia) | Content-defined matching tolerates shifts; deduplication/chunking throughput alone does not establish compression or execution gains. |
| [rsync](https://rsync.samba.org/tech_report/) | Rolling fingerprints can discover shifted matches, with exact verification kept separate. |
| [bsdiff](https://www.daemonology.net/bsdiff/) | Suffix sorting and bytewise residuals address executable differences; reviewed, not implemented or benchmarked here. |
| [HDiffPatch](https://github.com/sisong/HDiffPatch) | Block/memory matching provides alternative delta tradeoffs; reviewed, not downloaded or benchmarked here. |
| [Zstandard patching](https://github.com/facebook/zstd/wiki/Zstandard-as-a-patching-engine) | Exact reference-based dictionary coding. The initial measurement used raw dictionary mode with a 16 MiB window. The later codec round verified that installed 1.4.8 also supports `--patch-from`, correcting the initial capability assumption. |
| [Pinned Zstandard API](https://github.com/facebook/zstd/blob/v1.4.8/lib/zstd.h) | Immutable prepared dictionaries with a private decoding context per operation, loaded from a verified sealed library descriptor. |

The prototype combines a 64-byte rolling polynomial fingerprint, deterministic mixing,
content-based 1/1024 sampling, 256 bottom hashes, an inverted feature index, and four
candidate rankings: closest size, whole-file overlap, rarity-weighted `.text` overlap,
and 12 local-region minima. These are bounded research adaptations, not claimed paper
reproductions or production nearest-neighbor infrastructure.

Sixteen anchors are selected at size quantiles before measuring outcomes. Every target
has at most one reference hop. Twelve stratified targets are tested against all 16
anchors, first at level 3 and then level 12. No feature/hash similarity can replace the
expected original executable SHA256. The measured full-family prototype tries closest
size and weighted-text's top references at level 12 and retains the smaller actual patch,
with standalone compression as a fallback.

## What each strategy contributed

| Level 12 sample reference selection | Patch bytes, excluding anchors | Exact best-reference hits |
| --- | ---: | ---: |
| Closest size | 18,292,465 | 9/12 |
| Whole-file fingerprints | 18,358,246 | 9/12 |
| Weighted code fingerprints | 18,378,100 | 9/12 |
| Local-region fingerprints | 18,636,565 | 5/12 |
| Best actual patch among all predictors' top two | 18,261,685 | 11/12 |
| Exhaustive best of 16 anchors | 18,241,417 | 12/12 |

The exhaustive result is an oracle within the chosen anchor set, not a global optimal
compression bound. The union selection remains a sample comparison; the full candidate
uses only the two explicitly named top references above.

On the complete family, the fingerprint alternative beats closest size for 38 targets,
saving **1,372,112 bytes**, or 0.42% of the size-only representation. Most space savings
come from reference compression, not sophisticated retrieval. Building fingerprints took
**21.292 s**, searching **0.303 s**, and candidate population/roundtrip verification
**107.485 s** with at most two units. The persisted index has 8,384 postings;184 target
queries visit 681,967 postings across the three feature families. Exploration JSON
includes 3,030,441 bytes of sketches and 1,759,435 bytes of rankings/index, larger raw
than the marginal fingerprint saving; both are compressed and counted in support.
A production design would need to justify this complexity against the cheap control.

At fixed sample references, level 19 reduces delta bytes 18,955,584 → 18,403,350
(2.91%) but raises summed encoding wall time 5.984 → 25.324 s (4.23x). It was not
re-optimized over references at level 19 and was not expanded to all 200 objects.
This does not prove that heavier compression is useless; it identifies the measured
cost at those references. Separate encoder CPU utilization was not instrumented.

The CLI decoder required approximately 7.8 summed worker-seconds for decode/hash/seal;
the resident-library variant reduced that work to about 4.9–5.2. References were then
grouped into four 50-object units balanced using case counts, reducing reference loads
**59 →19** and summed reference setup **about 1.8 →0.68 s**. All units retain four native
workers and at most two concurrent units. Raw controls always use the same schedule
as their corresponding candidate; no fewer test processes or validation steps explain
the final result.

All earlier CLI/resident timings are retained. In particular the final CLI baseline
rose to 16.674 s from 13.827 s, including higher native-process time. That noisy pair is
not used to claim reconstruction is faster than uncompressed execution.

## Correctness, resource proof and accounting

- A fresh complete v0.91 family invocation passed in 39.458 s: 636 discovered top-level
  tests, one complete selected family, eight execution units plus build/list. This is
  explicitly partial-suite proof. Its 200 regenerated source/fixture sets match the
  earlier independently exported corpus byte-for-byte.
- All 14 replay inventories are identical: **2,064 package/case executions and 9,704
  named native tests per replay**, including subtests. Exact expected values/traces,
  original binary hashes, debug bytes, fresh native processes and fixture directories
  remain intact. Reconstructed executables are kernel sealed before execution.
- Eleven fault checks reject corrupted deltas/anchors, wrong reference/original hash,
  sealed-write/truncate attempts, changed current value/trace expectations, source,
  settings and toolchain. Resident decoding also passes 16 concurrent exact decodes
  and rejects three malformed/reference cases.
- **130 successful contained units** completed and their cgroups are removed.
  Peak per-unit aggregate memory is **735,817,728 bytes**. Limits remain 1 GiB hard,
  zero swap, 128 tasks, 800 MiB proactive stop, 700 MiB memory-high, native timeout 25 s,
  and bounded codec/workload execution. No hard-limit/OOM/OOM-kill or swap event
  increased. There were 1523 allowed memory-high reclaim events.
- Final grouped-delta peak was 333,090,816 bytes in the first run and 293,740,544 in
  the second. Conservative sums of the two largest per-unit peaks are 646,328,320
  and 565,587,968; these are bounds, not sampled simultaneous peaks.
- The initial sandbox launch was denied access to the systemd user bus. The narrowly
  reviewed canonical launcher then ran the work; no uncontained compiler fallback.
- The audit initially expected a full-suite artifact receipt from a partial-family
  run. It was corrected to respect the planner's deliberate omission: compare the
  retained 2719-entry metadata inventory, and claim fresh byte/execution verification
  only for the 200 objects actually exercised. No extra suite rerun was invented.

| Complete representation, conservative shared support included | Bytes |
| --- | ---: |
| Independently compressed code | 573,756,821 |
| Delta code, including all reference binaries | 326,456,238 |
| Common inherited input/Go/C/Python support | 136,704,685 |
| New verified support archive, including source, codec/library, search data and scripts | 7,159,548 |
| Candidate identity/support manifests | 1,547,492 |
| **Complete standalone comparison** | **719,168,546** |
| **Complete candidate** | **471,867,963** |

The complete candidate is 34.4% smaller than its matched complete representation.
This includes conservative inherited support overcapture, not a minimal support bound.
The support archive's 440 file inventory is reconstructed and hash-verified; exact
current counts and hashes are in `support.json`. Coexisting original exports, raw cache,
comparison encodings and experiment receipts are additionally counted in `storage-closure.json`.
No savings are claimed for retained experimental alternatives or the general Go cache.
The original 30-second/14,391,156-byte goals remain unmet, and no full-suite compressed
replay result is claimed. The older XZ precompression candidate stored less but had
higher historical replay costs; its differently instrumented timings are not a matched
comparison against this experiment.

## Scope and retained evidence

Prototype, research, corpora, encodings, schedules, scripts and receipts:
`/tmp/pipelang-similarity-probe/`. Main receipts: `final-evidence.json`, `results.json`,
`search-analysis.json`, `search12-analysis.json`, `manifest.json`, `identity-grouped.json`,
`support.json`, `storage-closure.json`, plus all population/replay/unit logs.

The admitted checkout HEAD is `8c7c070c4eb2141c74335875017ef6ddce4f942f` on `js/pipelang`. Source hashes match the
accepted uncompressed full-suite implementation. That 636-test proof, direct compiler
resource measurements and focused race/vet evidence remain admitted; no production
source changed here, so no whole-suite, repository-wide CI, non-Linux, cold-host or
new sustained-fuzzing result is claimed.

Repository edits are this report and the existing task records only. Generic engine/
package boundaries remain intact. No worktree, stash mutation, commit, push, toolchain
installation, external publication or protected/broad cache cleanup was performed.
The completed experiment supports a real storage/performance tradeoff and identifies
which layers contribute. It does not promote the candidate or select a successor
language contract, and does not establish that more advanced search cannot help.

Closure storage snapshot: **53,447,485,346 logical bytes** across all nonoverlapping
cache/toolchain/experiment roots, up **2,287,778,755 bytes** from the prior closure.
The new experiment root accounts for **2,287,738,792 bytes**, including
1,342,070,294 bytes of fresh source/fixture exports, both comparison encodings,
support archives, test binary and receipts. The native cache remains 12,691,799,961
bytes; the Go cache increases 38,401 bytes. No cache was cleaned or relocated.
The snapshot excludes its own subsequently written receipt and later tiny audit receipts.


## Further reconstruction research: second completed round

The user explicitly asked to research further and push the boundaries. This round
keeps the previous candidate/proof intact and evaluates three additional mechanisms:
structure-aligned residuals, bounded reference hierarchies and decoding directly into
final executable storage. State: **completed research experiment, unpromoted**.

New primary sources include [Courgette's reversible executable representation](https://www.chromium.org/developers/design-documents/software-updates-courgette/),
[Zucchini's typed-reference design](https://chromium.googlesource.com/chromium/src.git/+/62.0.3178.1/chrome/installer/zucchini),
[Hierarchical Relative Lempel-Ziv](https://arxiv.org/abs/2208.11371),
[RLZAP adaptive pointers](https://arxiv.org/abs/1605.04421),
[ReLZ](https://arxiv.org/abs/1903.01909),
[DeepSketch](https://www.usenix.org/conference/fast22/presentation/park),
[MedFS](https://www.usenix.org/system/files/fast25-wu.pdf), and the
[pinned Zstandard output-buffer API](https://github.com/facebook/zstd/blob/v1.4.8/lib/zstd.h).
Research notes distinguish implemented mechanisms from reviewed alternatives;
no paper's headline results are extrapolated to this corpus.

### What worked and what did not

**Structural residuals did not help.** The narrow prototype aligns ELF functions
by symbol name and other uncompressed sections by name, subtracts reference bytes,
and compresses the residual plus exact reconstruction spans. It reconstructs every
sample exactly, but all 12 samples are larger: **29,654,260 bytes versus 18,264,988**
for the existing dictionary deltas. This rejects this particular transform; it does
not reproduce or disprove full Courgette/Zucchini relocation-aware matching.
Compressed DWARF remains untouched, preserving exact original bytes.

**A reference hierarchy saves more space.** Measure all 240 directed pairs among
16 anchors at level 12, then choose a root by total encoded anchor cost. The root
is stored independently, other anchors reference it, and the 184 existing leaf
choices remain unchanged. Reconstruction depth is explicitly at most two. Anchor
storage falls **46,081,715 → 27,100,287 bytes**; total encoded corpus falls
**326,456,238 → 307,474,810 bytes**, a further **5.8%** reduction. No original
executable or debug byte is removed. The root matrix took 69.787 seconds.

The worst tested root produces 27,223,290 anchor bytes—only **123,003 bytes** above
the best. Exploiting reference redundancy matters much more here than fine-tuning
which root wins. This is the optimum within the measured two-level star family,
not the optimum among arbitrary references, graphs, codecs or corpus representations.
Learned search was researched but not trained; its model/training/storage cost is
not justified by the small measured selection gap within this anchor set.

**Direct mapped output reduces reconstruction work.** Each decode gets a private
codec context and bounded writable memfd mapping. The decoder writes into that
mapping; SHA256 is verified there, all writable views are released, and kernel
seals are applied before native execution. This avoids intermediate decoded-buffer
copies and a separate write into executable storage. It does not eliminate all
copies in the pipeline. Summed per-object decode/hash/seal time falls from about
**4.99 to 4.61 seconds** in the matched buffered/mapped comparison, around 7.8%.
This component saving alone does not establish a family wall-time win.

### Matched native proof and timing uncertainty

The initial order is raw/buffered/mapped/hierarchy/hierarchy/mapped/buffered/raw.
A confirmation reverses the selected comparison: hierarchy/raw/raw/hierarchy.
No compilation or archive capture overlaps these replay runs. All methods use the
same grouped schedule, exact source/toolchain/settings and encoded-blob validation,
50 objects per unit, at most two units and four native workers per unit.

| Method | First wall time | Second wall time | Mean cgroup CPU, summed across units |
| --- | ---: | ---: | ---: |
| Raw, initial comparison | 14.684 s | 13.826 s | 59.459 s |
| Previous buffered delta | 14.328 s | 13.927 s | 61.046 s |
| Direct-mapped delta | 14.581 s | 13.777 s | 60.836 s |
| Direct-mapped hierarchy | 13.673 s | 13.822 s | 60.098 s |
| Raw, confirmation | 13.928 s | 14.579 s | 59.500 s |
| Hierarchy, confirmation | 14.729 s | 14.780 s | 60.718 s |

Across all four runs of each selected method, mean wall times are **14.254 seconds
raw versus 14.251 seconds hierarchy**. The difference is far smaller than observed
variation and is not a speedup/equivalence claim. Mean cgroup CPU is **59.480 versus
60.408 seconds**, about **1.6% more CPU** for hierarchy. CPU sums are not wall times.
The favorable first sequence and unfavorable confirmation are both retained. This
supports near-raw observed replay timing with a modest CPU cost, not zero-cost
reconstruction, cold-host performance or a whole-suite speedup.

A fresh complete v0.91 invocation passed in **36.225 seconds**, regenerating all
200 current source/fixture sets identically. It discovers 636 top-level tests but
runs one complete selected family, explicitly partial-suite proof. Every one of
**12 native replay inventories** is identical to the previous experiment: **2,064
case executions and 9,704 named native tests per run**, retaining independent
current expected values and ordered traces, fresh processes/fixture directories,
original ELF/DWARF hashes and sealed code. Existing full-suite/compiler resource
proof remains admitted because production source hashes are unchanged.

Mapped decoding passes 16 concurrent exact reconstructions, 48 seal mutation checks,
and five malformed/bounded-output/descriptor-cleanup cases. Hierarchy tests reject
cycles, missing or excessive-depth references, corrupt roots and corrupt anchors.
Two-level reconstructed native binaries also fail when current expected values or
ordered traces are deliberately changed. No outcome is reused.

All **106 new contained units** completed and their cgroups are removed.
Peak aggregate memory per unit was **735,035,392 bytes**. The 1 GiB hard limit,
zero swap, 128 tasks, 800 MiB proactive stop, 700 MiB memory-high, bounded codec
workloads and native timeout 25 seconds remain. No hard-limit/OOM/OOM-kill or swap
event increased; 469 memory-high reclaim events are recorded separately.
Hierarchy replay peaks ranged from 198,119,424 to 425,234,432 bytes; conservative
sums of the two largest unit peaks reached 794,238,976 bytes. These are bounds,
not sampled simultaneous peaks or a universal memory-reduction claim.

### Complete storage and retained scope

| Candidate representation component | Bytes |
| --- | ---: |
| Selected leaf/root/anchor encodings | 307,474,810 |
| Inherited input/Go/C/Python reconstruction support | 136,704,685 |
| New roundtrip-verified support archive, 457 files | 9,110,756 |
| Identity/support/representation manifests | 1,532,641 |
| **Complete candidate representation** | **454,822,892** |

The previous complete candidate was 471,867,963 bytes; this round saves another
17,045,071 bytes after accounting for new support. Accounting retains conservative
inherited support overcapture and all search/reconstruction helpers. It is not a
minimal representation bound or a packaged portable-cache product. The benchmark's
comparison-only plain/older-delta blobs are checked symmetrically; all coexisting
encodings, rejected transform files, fresh exports and receipts are additionally
counted in the physical retained-root inventory.

The closure snapshot totals **55,050,874,217 logical bytes** across
all nonoverlapping cache/toolchain/experiment roots. This round adds **1,603,388,871
bytes**, with **1,603,317,484 bytes** in the new experiment root. The native
cache remains 12,691,799,961 bytes and its 2,719-entry metadata inventory is unchanged;
fresh byte/execution verification applies to the 200 exercised objects. No cache was
cleaned, relocated or silently excluded. The snapshot excludes its own subsequent
receipt and later tiny audit receipts.

Evidence: `/tmp/pipelang-reconstruction-frontier/`, especially `research.md`,
`results.json`, `hierarchy-plan.json`, `manifest.json`, `identity.json`,
`mapped-check.json`, `faults.json`, `support.json`, `final-evidence.json`,
`storage-closure.json`, and every root-matrix/population/replay/unit receipt.

Repository changes remain documentation/task records only. Engine/package boundaries,
HEAD `8c7c070c4eb2141c74335875017ef6ddce4f942f`, protected stashes and ignored inventory are preserved. Edits remain
uncommitted. No new language contract, installed toolchain change, publication,
worktree or production promotion occurred. The original 30-second/14,391,156-byte
goals remain unmet; larger claims require further evidence rather than extrapolation.


## Original compression decisions instead of recompression (2026-09-07)

The user requested continued outside-the-box research. This completed round changes
representation: expose DEFLATE symbols and retain the original decisions, instead
of treating compressed DWARF as opaque bytes or rerunning the compressor on plaintext.

[Microsoft preflate-rs](https://github.com/microsoft/preflate-rs) explains exact
reconstruction using predicted compression decisions plus corrections. The local
prototype independently implements a simpler approach from
[RFC 1951](https://www.rfc-editor.org/info/rfc1951/): record literal/length symbols,
distance symbols, extra bits, original Huffman headers, stored blocks, padding and
zlib trailer. Replaying the recorded codes avoids LZ match search. No external
library was installed or downloaded, and Microsoft's compressor/size claims are
not treated as evidence about this Go corpus.

The prototype replaces only compressed ELF section payloads with transcripts;
all other ELF bytes remain verbatim. It applies Zstd level 12 using the previously
selected references. Original and buffered bit writers are retained separately.
Independent corpus SHA-256 checks verify the entire reconstructed ELF, including
DWARF. Choosing an approximate reference is compatible with exact reconstruction
because explicit transcript/residual data is retained; similarity fingerprints or
hashes alone cannot recover discarded information.

### Fixed sample and reference cost

The same 12 size-stratified objects and their previous references were used, with
no sample reselection. The hierarchy uses the previous root and charges all 16
reference objects, including five immediate references not needed by these leaves.

| Encoded component | Existing representation | Decision transcript |
| --- | ---: | ---: |
| 12 leaf patches | 18,264,988 bytes | 8,782,059 bytes |
| Full 16-object reference pool | 27,100,287 bytes | 18,585,806 bytes |
| **Sample plus references** | **45,365,275 bytes** | **27,367,865 bytes** |

All 12 leaf patches shrink: **51.9%** in aggregate. Charging the reference pool
reduces the gain to **39.7%**. The leaves' original ELF total is 81,265,628 bytes.
Transcripts compressed independently occupy 38,859,255 bytes; reference sharing
is still essential. Exposing plaintext with the same references produces smaller
leaf patches, 7,047,864 bytes, but needs the slower exact recompression path.

These figures do not extrapolate to the 200-object corpus. The sample's storage
model also charges the full inherited 145,815,441-byte input/toolchain/reconstruction
support, 1,532,641 bytes of inherited metadata, a new roundtrip-verified support
archive of 4,640,684 bytes and 17,965 bytes of new metadata. The resulting conservative
sample representation is **179,374,596 bytes**, versus **192,713,357 bytes** using
the existing sample representation and inherited support. This deliberately
captures more support than a minimal sample needs; it is not a portable cache
product or a replacement for the earlier complete-corpus accounting.

### Matched reconstruction measurements

Six alternating-order rounds per sample compare the existing delta path, buffered
transcript reconstruction and plaintext followed by exact Go recompression. Each
measurement includes codec processes, required intermediate file writes, final
reconstruction/write/read and an independently expected original hash. References
are already prepared; their cold preparation is not included in this timing table.

| Path | Sum of 12 per-object median times |
| --- | ---: |
| Existing delta decode | 0.4164 s |
| Buffered transcript decode and reconstruction | 1.1572 s |
| Plaintext delta decode and exact Go recompression | 1.4381 s |

The transcript path is **19.5% faster than plaintext/recompression**, but **2.78x
as slow as existing delta decoding** in these warmed component measurements.
The separately reported reconstruction-helper medians sum to 0.5544 versus
0.8126 seconds. These sums are not whole-family wall times or CPU measurements.
Leaf encoding itself also costs more: 12.47 summed seconds for transcript Zstd
encoding versus 7.19 for the original-byte patches, excluding transcript analysis
and reference preparation. Both the earlier bit writer and all its measurements
remain available; no cross-phase timing is presented as a controlled bit-writer
speedup.

This establishes a middle ground between storage size and reconstruction work.
It does not establish performance without loss, a full-suite speedup, cold-host
performance, or the original 30-second/14,391,156-byte targets.

### Proof, retained artifacts and remaining integration

All **28 distinct original binaries** roundtrip exactly, including 12 root-to-anchor-
to-leaf chains reconstructed solely from encoded reference recipes. Two implementations
pass **42 synthetic roundtrips** spanning stored, fixed and dynamic blocks, flushes,
empty/incompressible/repetitive input and nonzero padding. **15 negative cases**
reject changed/truncated/oversized recipes, corrupt root/leaf encodings and a wrong
reference. The 360 timed sample reconstructions verify exact original hashes.

The round uses **42 contained units: 41 successful and one retained audit-script
failure**. The first audit inspected its own preliminary runner receipt before
`unit_exit` existed. The corrected audit explicitly excludes its active receipt
and preserves the failure, first support archive and corrected support archive.
Every cgroup was removed. Peak aggregate memory was **735,485,952 bytes**, with
36 memory-high reclaim events and no hard-limit/OOM/OOM-kill/swap-event increase.
The inherited 1 GiB hard limit, zero swap, 128 tasks, 800 MiB proactive stop,
700 MiB memory-high and maximum two concurrent units remain unchanged.

All nonoverlapping retained cache, toolchain and experiment roots total
**57,239,192,715 logical bytes**. This round's root contains **2,187,851,416 bytes**;
the increase from the previous closure is 2,188,318,498 bytes. All intermediate
transcripts, expanded comparisons, restored originals, failed audit evidence and
both support archives are counted. No cleanup occurred. The snapshot excludes
its own subsequent receipt and later small audit receipts; allocated file sizes
are recorded separately and are not unique-device storage usage.

Production source hashes, HEAD, protected stashes, ignored inventory and all
2,719 retained native-cache metadata entries remain unchanged. Byte verification
this round covers 28 exercised objects; no fresh native process execution or
whole-suite timing is claimed. Existing full-suite/oracle/trace proof is preserved,
not rerun or weakened. Repository edits are documentation/task records only;
engine/package boundaries remain intact and no candidate is promoted.

The next integration question is whether resident transcript reconstruction and
bounded preparation can retain this space saving during full native replay.
[LoopDelta](https://www.usenix.org/conference/atc23/presentation/zhang-yucheng) and
[ALACC](https://www.usenix.org/conference/fast18/presentation/cao) motivate measuring
reference locality and look-ahead within fixed memory. They do not establish that
our additional CPU or validation work can be hidden. Full-corpus encoding, fresh
current fixtures, resident-path negative tests and matched complete native-family
runs are still required before promoting this sample result.

Evidence: `/tmp/pipelang-representation-lab/`, especially `research.md`, both Go
implementations, `sample-*.json`, `compare-*.json`, `anchors-*.json`, `faults.json`,
`manifest.json`, `support-v2.json`, `results.json`, `final-evidence.json`,
`storage-closure.json` and all contained-unit receipts. This research round is
complete; the previous 200-object hierarchy remains the latest candidate with
complete native replay measurements.

## Resident decision-transcript reconstruction: measured rejection (2026-09-07)

The approved continuation is **completed, unpromoted**. It profiled the fixed
12 leaves, tested resident reconstruction and bounded reference preparation, and
rejected expansion on measured evidence. Evidence root:
`/tmp/pipelang-resident-transcript/`. Production and v0.96.0 are unchanged.

### Profile and tested candidates

The initial Go CPU profile covers 300 exact resident reconstructions plus separate
copy/hash controls. Its combined 66.65% hash fraction includes those controls.
The later resident-only mapped profile attributes 46.06% to SHA-256, 13.78% to
`unpackZ` exclusive work, 11.81% to bit writing, 11.02% to symbol writing and 8.66%
to copying. Thus removing process startup alone cannot remove the observed work.
Ordinary delta's 300 measured component repetitions sum per-object median decode,
hash, copy and allocation times to 0.0227, 0.1389, 0.0150 and 0.00785 seconds.
Codec startup median is 0.000870 seconds. These are component profiles, not native
test timing or measurements of durable filesystem writes.

The retained alternatives are the unchanged buffered algorithm in a resident pipe
service, a pointer receiver that avoids copying tree headers per symbol, and a
shared-mapping service that reconstructs directly into the final executable buffer.
Each request uses fresh descriptors; input transcripts and completed outputs are
sealed, all original section/ELF/transcript/encoding hash checks remain, and the
service releases its mappings before the client seals executable output. No codec,
assembly, installed toolchain or production compiler was patched.

[preflate-rs](https://github.com/microsoft/preflate-rs) supports the exact-decisions
approach. [LoopDelta](https://www.usenix.org/conference/atc23/presentation/zhang-yucheng)
and [ALACC](https://www.usenix.org/conference/fast18/presentation/cao) motivate testing
reference locality within fixed memory; their backup-storage gains do not establish
local reconstruction gains. [PivCo-Huffman](https://arxiv.org/abs/2606.05765) uses a
pivot-coded representation. It was not adopted as a replacement for the original
RFC1951 bitstream writer, and its reported throughput was not transferred here.
No external implementation was downloaded or installed.

### Matched unprofiled confirmation

Both paths use resident Zstd, independent original hashes and sealed output.
The warm comparison has six alternating-order repetitions per fixed leaf.
Preparation-inclusive measurements have four counterbalanced rounds per path/order,
retain only the root dictionary plus one current reference dictionary, and include
all 12 required preparations. Grouping did not reduce that preparation count.
Storage still charges the full 16-reference pool, and all 16 roundtrip exactly.

| Measurement | Ordinary delta | Mapped transcript |
| --- | ---: | ---: |
| Warm sum of per-object median wall times | 0.203 s | 0.924 s |
| Warm sum of per-object median cgroup CPU | 0.218 s | 0.991 s |
| Grouped preparation-inclusive median wall | 0.532 s | 2.210 s |
| Grouped preparation-inclusive median cgroup CPU | 0.561 s | 2.357 s |
| Reference preparation within grouped wall | 0.292 s | 1.245 s |
| Grouped per-unit aggregate memory range | 83,591,168–83,947,520 B | 108,179,456–110,018,560 B |

The candidate is **4.56x slower** in the warm component comparison and **4.16x
slower**, with **4.20x CPU**, including grouped bounded preparation. Shared mappings
improve the earlier pipe candidate but leave a large measured gap. Runs use warm
local files; some units overlap within the maximum-two-unit allowance. Raw rounds
and ranges remain in the receipts. These results justify rejecting full-corpus
expansion of this candidate. They do not establish native workload wall time,
rule out other algorithms, or support a zero-overhead claim.

### Exactness, accounting and scope

All 28 distinct original sample/reference ELFs, including DWARF, reconstruct exactly.
The two new helper builds pass 42 synthetic roundtrips. Twenty rejection scenarios,
45 seal mutation attempts and descriptor recovery checks pass, covering corrupt
recipes/encodings, wrong references, unavailable descriptors and output size bounds.
No cached pass outcome substitutes for a reconstruction check.

Encoded sample/reference storage remains **27,367,865 bytes**, versus 45,365,275
for the ordinary representation. Conservatively retaining all inherited support
and adding the 5,840,072-byte roundtrip-verified new support archive plus 27,470
bytes of new representation metadata gives **185,242,138 bytes**, versus the prior
ordinary sample's 192,713,357 bytes. This smaller sample representation is slower;
it is not a complete 200-object representation or a portable cache product.

All **49 contained units pass and are removed**. Peak aggregate memory, including
the inventory audit, is **438,755,328 bytes**; the conservative sum of the two
largest unit peaks is **766,971,904 bytes**, not a sampled simultaneous peak.
There are no memory-high, hard-limit/OOM/OOM-kill or swap-event increases. All
inherited limits remain: 1 GiB hard memory, zero swap, 128 tasks, 800 MiB proactive
stop, 700 MiB memory-high, GOMAXPROCS=4 and bounded serial codec/compiler children.

The nonoverlapping retained-root snapshot totals **57,292,836,767 logical bytes**,
including **52,436,635 bytes** in the new root; growth is **53,644,052 bytes**.
All old alternatives, caches, toolchains and new intermediates are counted; nothing
was cleaned. Logical bytes are not unique device allocation. The snapshot excludes
its subsequent receipt, final audit receipts and later small documentation edits.

All 200 production source hashes, 2,719 native-cache metadata entries, HEAD,
protected stashes and ignored inventory match admission. Prior 636-test whole-suite
acceptance remains admitted. Because the candidate was rejected before native
integration, fresh full-family fixtures/native replay and independent compiler
resource probes were not rerun. No native-performance or promotion claim is made.
Repository changes remain the four owned documentation/task records; engine/package
boundaries are intact. No commit, push, worktree, stash change, installation,
persistent host change, new language contract or automatic successor occurred.

Key receipts: `results.json`, `profile-mapped.txt`, `control-profile.json`,
`final-warm.json`, `final-*-*-*.json`, `faults.json`, `boundaries.json`,
`support.json`, `final-evidence.json` and `storage-closure.json`. Earlier prototypes
and measurements are preserved. The previous hierarchy experiment remains the
latest complete-corpus candidate with native replay proof.

## Selective transcripts and bounded preparation: useful tradeoff (2026-09-07)

The user requested more research after the resident rejection. This round is
**completed research, unpromoted**. It retains a full-corpus candidate with a
substantial storage reduction and a small measured native wall-time increase.
Evidence root: `/tmp/pipelang-selective-transcript/`.

### Representation selected from the profile

The fixed 12-leaf sample contains about 17 MB of compressed debug sections, expanding
to about 51 MB of full transcripts. Section-specific measurements show that
`.debug_info` consumes 0.0865 summed seconds of replay work for 2.08 MB of delta
savings, while `.debug_line`, `.debug_loclists` and `.debug_rnglists` together save
7.50 MB with 0.1047 summed seconds. Those three sections capture about 76% of the
9.82 MB measured section-level savings. The original 12 leaves and full 16-reference
pool were preserved; no favorable samples or references were substituted.

The selected representation reconstructs most ELF bytes through ordinary delta
into an incomplete scaffold, then fills three bounded ranges with exact section
transcripts. This avoids treating all unchanged ELF bytes as expanded transcript
data. Every stored encoding is authenticated, section transcripts retain their
expected hashes, ZT01 retains original compressed-section checks, and the completed
ELF is checked against its independent original hash before sealing. The scaffold
is never executable or a cached pass. Original ELF/DWARF bytes remain exact;
source/toolchain/settings and current value/ordered-trace checks remain independent.
The earlier ET01/ZT01 implementations and their evidence are preserved.

[Application-specific delta encoding](https://www.usenix.org/legacy/event/usenix03/tech/full_papers/douglis/douglis_html/paper.html)
motivates measuring where structural treatment pays. The section selection is an
inference tested locally, not a transferred performance claim from that paper.
The bounded preparation experiment also follows the locality/look-ahead questions
raised in the earlier LoopDelta/ALACC research. No external codec was installed.

### Full native measurements

The initial full native comparison was 14.537 seconds ordinary versus 15.450 seconds
splice, averaged over four runs each. The final scheduling experiment applies a
single serial reconstruction producer and four native consumers to **both**
representations. It admits at most eight submitted workload binaries: four executing
and four queued. Only root/current reference objects are retained, with temporary
reference-validation buffers separately charged. All workload binaries are fully
verified and sealed before entering the queue; each case still receives a fresh
process and fixture directory. This overlaps reconstruction/reference preparation
with useful native execution without reusing test outcomes.

| Four-run matched comparison | Ordinary delta | Selective splice |
| --- | ---: | ---: |
| Mean native-family wall | 12.527 s | 13.068 s |
| Mean cgroup CPU summed across four shards | 57.941 s | 63.780 s |
| Whole-run wall range | 11.920–13.021 s | 12.877–13.135 s |
| Per-unit aggregate memory range | 130,400,256–378,531,840 B | 122,023,936–213,192,704 B |
| Encoded 200-object corpus, including references | 307,474,810 B | 191,507,484 B |
| Complete conservatively charged representation | 454,822,892 B | 364,910,236 B |

The complete-storage baseline is the previously accounted hierarchy representation;
new experimental support is conservatively charged to the candidate. Native timing
uses the same final scheduling implementation for both paths.

The final candidate uses **37.72% fewer encoded bytes** and **19.77% fewer complete
representation bytes**, with **4.32% more native wall time** and **10.08% more CPU**
than the matched ordinary-delta schedule. Scheduling helps both paths; comparing
only the new schedule against an old schedule would overstate the representation's
benefit. The observed memory ranges are not a universal memory-reduction claim.
Two raw controls average 14.924 seconds under the earlier grouped schedule; they
are diagnostic, not substitutes for the matched final control.

This is a useful measured storage/wall/CPU tradeoff, not zero-overhead reconstruction,
a whole-suite speedup or attainment of the original 30-second/14,391,156-byte goals.

### Fresh proof and retained failures

The canonical `tests/containedexec/pipelang_suite.py` produced all 200 fresh current
v0.91 family exports and passed its build, listing and eight family units. The thin
local adapter tightens every unit to 30 seconds, uses memory-high 700 MiB and retains
the newly created empty native-build cache. The canonical driver's overall exit is
**1 solely because its housekeeping condition requires that cache to be removed**.
That return and the explicit qualified acceptance are preserved in
`family-acceptance.json`; test success is not conflated with housekeeping success.
All 200 cache uses were fresh verified hits, with zero misses and no reused outcomes.

All **18 complete native replays** retain **2,064 cases and 9,704 named tests each**,
with identical ordered inventories, original executable/DWARF hashes and current
independent expected values/traces. Deliberately changed current values and traces
fail native execution. Forty-two synthetic roundtrips cover both ordinary and direct
mapped section writers. Twenty-four rejection checks cover corrupted root/anchor/leaf
encodings, wrong references, malformed spans and sizes, seals, current source/settings
invalidation and current native value/trace faults; descriptor recovery passes.

Three initial native-launch units failed before executing binaries because a
manifest-building glob included unit receipts instead of only population results.
The corrected explicit 50-part manifest and fresh replay labels preserve those
failures. The complete round has **169 contained units: 166 successful and three
retained startup failures**. Every cgroup is removed. Peak aggregate memory was
**736,079,872 bytes**. The conservative sum of the two largest unit peaks is
**1,472,036,864 bytes**, not a sampled simultaneous peak. There were **618 memory-high reclaim events** and no
hard-limit/OOM/OOM-kill or swap-event increase. The 1 GiB hard limit, zero swap,
128 tasks, 800 MiB proactive stop, GOMAXPROCS=4, at most two units, four native
workers/unit and serial bounded codec/compiler children remain unchanged.

### Complete accounting and final scope

| Candidate component | Bytes |
| --- | ---: |
| All selected scaffold/transcript encodings, including references | 191,507,484 |
| Conservatively retained inherited reconstruction/toolchain support | 156,296,197 |
| New roundtrip-verified support archive | 13,924,844 |
| Inherited metadata | 1,578,076 |
| New recipe/support/identity metadata | 1,603,635 |
| **Complete representation** | **364,910,236** |

All 12,568 fresh input-file hashes match the inherited full-corpus input manifest.
The current generator binary/source closure is captured again; raw fresh fixtures
remain separately charged in physical retained storage. All intermediate section
matrices, rejected layouts, full scaffolds, fresh fixtures, failure receipts and
previous experiments are retained. The new root contains **3,499,793,381 bytes**;
the nonoverlapping retained-root snapshot totals **60,794,150,002 logical bytes**,
a **3,501,313,235-byte** increase. Logical size is not unique device allocation.
The snapshot excludes its subsequent receipt, final audit receipts and later small
documentation edits. No cache or experimental root was cleaned.

All 200 production source hashes, 2,719 native-cache metadata entries, HEAD,
protected stashes and ignored inventory remain unchanged. Byte/execution proof
covers all 200 affected corpus objects; the unaffected cache entries were checked
through metadata. Prior 636-test whole-suite acceptance and independent compiler
resource proof remain admitted; the full suite was not rerun for this temporary
representation experiment. No production implementation, language contract,
engine/package boundary, commit, push, installation, persistent machine setting or
publication changed. Repository edits remain the four owned documentation/task
records, unstaged and uncommitted. No automatic successor was created.

Key evidence: `sections.json`, `matrix-*.json`, `full-manifest.json`,
`family-acceptance.json`, `replay-*-result.json`, `pipeline-*-result.json`,
`faults.json`, `results.json`, `support.json`, `final-evidence.json` and
`storage-closure.json`. This is now the latest complete-corpus representation with
fresh native measurements; production adoption remains a separate decision.

## Adaptive section and reference research (2026-09-07)

Renewed explicit user research authority reopens the same objective. State: executing.
Evidence root: `/tmp/pipelang-adaptive-transcript/`. The prior completed proof is
admitted; v0.96.0, exact ELF/DWARF, source/toolchain/settings invalidation, current
Value and ordered Trace oracle, sealed execution and contained limits remain fixed.
Automatic checkpoints: profile the matched complete pipeline; screen section choices
on the fixed 12 leaves and 16 references; screen representation-aware references;
advance promising candidates through matched full affected-family native proof;
close inclusive storage and retain failures. Production remains unpromoted.

### Checkpoint 1: complete-pipeline profile

Two matched counterbalanced rounds per lane passed all 200 binaries, 2,064 cases
and identical ordered 9,704 test names per replay. Detailed stage timings are in
`profile-summary.json`; stage sums overlap and must not be added as wall time.
Validation is a large serial cost, while native processes dominate cumulative
worker time. Splice reconstruction costs more than ordinary delta, with reference
preparation and service fill contributing. Keep validation intact. Next screen
section choices and representation-aware references on the unchanged 28-object
sample, retaining the fixed-three-section control and all reference candidates.

### Checkpoint 2: section and reference screening

The fixed 12-leaf/16-anchor screen tested 192 reference pairs, 36 matched-section
and 36 mixed-section pairs, plus 576 exact sealed reconstruction checks. The
fixed-three sample uses 10,788,685 encoded bytes and 0.359 s warm reconstruction;
five sections use 8,732,808 bytes and 0.517 s; four use 8,888,439 bytes and 0.500 s.
Line-only uses 15,017,082 bytes and 0.278 s. These are diagnostic warm sums, not
full-pipeline timings. Mixing four/five leaf sections with fixed-three references
worsens both size and replay and is rejected. A per-object choice needs consistent
reference treatment and cannot assume additional reference variants are free.

Exhaustive reference selection on the fixed sample changes two choices and saves
231,014 bytes (2.14%), with approximately unchanged warm reconstruction. Advance
the five-section storage tradeoff and a bounded reference-only shortlist: test the
incumbent plus both discovered references for every non-reference leaf, retaining
all 16 anchors. This is explicitly a shortlist heuristic, not a full-corpus optimum.
Run matched full-family native proof before assessing either candidate.

### Checkpoint 3: full-corpus candidates

All 200 five-section objects encode to 159,572,585 bytes, including the complete
16-reference hierarchy, versus 191,507,484 bytes for fixed three sections. The
reference shortlist produces 186,439,519 bytes and changes 31 reference choices.
The full search is bounded to the incumbent plus two references; no claim of a
global optimum is made. All stream roundtrips passed. Final native comparisons
use four counterbalanced rounds across ordinary delta, fixed three, fixed five,
and the reference candidate; each lane validates the same union of relevant
representation inventories. Thus terminal timings are compared only within this
new schedule, not against earlier rounds with cheaper inventory validation.

### Terminal proof and adoption assessment

The renewed round is **completed research, unpromoted**. Four counterbalanced
full-family native runs per lane used the same serial producer, four native workers,
at most eight submitted binaries and identical inventory validation. Each shard
retains only root/current references, with temporary reference buffers charged.

| Matched four-run lane | Encoded corpus | Mean wall | Mean cgroup CPU | Wall range |
| --- | ---: | ---: | ---: | ---: |
| Ordinary delta | 307,474,810 B | 12.983 s | 52.966 s | 11.500–14.216 s |
| Fixed three sections | 191,507,484 B | 13.124 s | 57.426 s | 12.304–14.062 s |
| Fixed five sections | 159,572,585 B | 14.742 s | 62.381 s | 12.762–15.618 s |
| Reference shortlist | 186,439,519 B | 13.203 s | 58.490 s | 12.288–14.461 s |

Relative to the matched fixed-three control, the reference candidate saves **2.65%
encoded bytes**, with **0.61% more mean wall** and
**1.85% more CPU**. Five sections save **16.68% encoded bytes**,
with **12.33% more mean wall** and **8.63% more CPU**.
The wall ranges overlap; four runs do not establish statistical equivalence or a
universal performance bound. No comparison uses a different scheduling round.

A verified 35-file support bundle adds **2,170,020 bytes**. Conservatively retaining
the prior complete overhead of 173,402,752 bytes gives the following closure:

| Complete accounted representation | Bytes |
| --- | ---: |
| Prior three-section representation, prior support | 364,910,236 |
| Three-section control, also charged all new support | 367,080,256 |
| Reference candidate, all new support | 362,012,291 |
| Five-section candidate, all new support | 335,145,357 |

Against the previously retained complete representation, reference selection saves
2,897,945 bytes (0.79%); five sections save 29,764,879 bytes (8.16%). Against the
control charged the same new support, the reductions are 1.38% and 8.70%. Reference
selection is a modest improvement worth retaining as research evidence. Five
sections are a storage/CPU tradeoff and are rejected as a universal default. Mixed
per-binary treatments are rejected on the sample; line-only is faster in the warm
microbenchmark but materially larger. No candidate is promoted to production.

Preparation is not free: sample reference search took 66.354 s wall / 137.079 s
systemd CPU; full shortlisted search took 156.824 s / 315.331 s CPU. Five-section
form generation took 16.702 s / 30.565 s CPU and full encoding plus stream
verification took 90.730 s / 186.906 s systemd CPU. The narrower codec-only
measurement was 182.393 s CPU; it excludes unit startup. `results.json` records
all phase costs. These are offline preparation costs, separate from warm replay.

The round passed **20 complete native replays**: four profiling replays plus 16
terminal comparisons, each preserving **200 original ELF/DWARF binaries, 2,064
cases and 9,704 identical ordered test names**. The terminal candidates separately
passed **48 rejection/lifecycle checks** and **42 synthetic roundtrips**. The sample
also passed 576 sealed reconstructions. All 351 newly tested full-corpus reference
pairs and all 200 five-section stream pairs roundtripped exactly. Current expected
Value and ordered Trace checks remain independent of representation and fail when
deliberately altered. Original source/settings/toolchain validation and sealing
remain intact. The prior fresh family compiler/export proof, complete 636-test
whole-suite proof and independent 128 MiB/5 s warm compiler proof are admitted;
no source or direct dependency drift justified rerunning them. The earlier
canonical driver's intentional housekeeping exit 1 remains a qualified result.

There were **354 successful contained units**, all removed, plus one retained
sandbox bus preflight failure before a workload started. Narrow reviewed host
execution then used the required contained runner; no uncontained fallback ran.
Peak per-unit aggregate memory was **689,770,496 bytes**. The conservative sum of
the two largest peaks was **1,369,931,776 bytes**, not a sampled simultaneous peak.
There were zero memory-high, hard-limit, OOM, OOM-kill or swap-event increases.
Limits remained 1 GiB hard, zero swap, 128 tasks, 800 MiB proactive stop, 700 MiB
high, GOMAXPROCS=4, at most two units and serialized codec/compiler children.

All retained roots total **64,616,693,550 logical bytes**, including
**3,822,209,187 bytes** in the new research root. This includes rejected encodings,
raw forms, fixtures, old alternatives, caches and toolchains; it is not a claim of
unique physical allocation. The closure excludes its own final small receipts and
subsequent documentation edits. Every earlier root remains in place. All 200
production source hashes, 2,719 protected native-cache entries, HEAD, branch,
stashes, empty index and ignored inventory were reverified. Only the four admitted
task documentation paths changed; package/engine boundaries are preserved.

Authoritative receipts: `final-evidence.json`, `results.json`, `screen-summary.json`,
`profile-summary.json`, `full-selection.json`, `support.json`, `audit.json` and
`storage-closure.json` under `/tmp/pipelang-adaptive-transcript/`.

[Zstandard's dictionary-based patching description](https://github.com/facebook/zstd/wiki/Zstandard-as-a-patching-engine)
and [application-specific resemblance research](https://www.usenix.org/legacy/event/usenix03/tech/full_papers/douglis/douglis_html/paper.html)
motivated the local selection experiments. Their published performance is not
transferred to this corpus. No packages or codecs were installed.

Remaining ideas—validation-cost work preserving invalidation, exhaustive full-corpus
reference search and joint section/reference optimization—are deferred. No
automatic further round, handoff, commit, push, cleanup, language expansion, cloud
operation or production promotion is authorized by these results. Strict original
30-second whole-suite and 14,391,156-byte storage goals remain unmet.

## Further representation research authorized (2026-09-07)

After the completed adaptive round, the user explicitly requested another research
round and a fresh-task handoff, suggesting converting binary to hex and related
ideas. The same objective is reopened as `ready_for_execution`; the previous
acceptance remains intact. First checkpoint: screen binary-to-hex and a bounded
set of related reversible byte representations against unchanged raw-binary and
current selective-transcript controls on the fixed sample. Charge expansion,
metadata, transforms, codec work and reference preparation; verify exact inverse
bytes. Advance only evidence-led candidates to matched full affected-family native
proof. Negative results are valid; no gain is promised. Public primary-source
research and temporary local prototypes remain authorized. All inherited resource,
oracle, sealing, invalidation, coverage and authority boundaries remain in force.
Production remains unpromoted. The requested handoff creates one fresh task in the
same saved checkout; it does not authorize automatic future rounds or handoffs.

## Hex and reversible layout round (2026-09-07)

State: executing under the renewed authority above. Evidence root:
`/tmp/pipelang-hex-round/`. Admission reverified 200 source hashes, the
108166-entry ignored inventory, protected Git anchors and prior acceptance receipts.
The fixed 12 leaves plus 16 references are unchanged. Screen raw and fixed-three
selective components with identity, ordinary hex, Base64, separated hex digit
planes and four-byte column layouts, using the existing Zstandard level 12
reference graph. Charge transform, reference preparation, codec, inverse, hash,
metadata and retained artifacts. Select only after this checkpoint is recorded;
production remains unpromoted.

### Representation checkpoint: reject all screened transforms

The matched file-input screen reproduced every fixed-three control encoded hash.
All four transforms increased size for every sampled raw/scaffold/token component:
zero storage wins across 336 transformed component comparisons. No hybrid or
full-family candidate is selected. The initial stdin diagnostic did not reproduce
the historical file-input control and is excluded from selection; all its evidence
is retained. Full affected-family native proof is therefore not reopened: there is
no promising candidate or source/runtime change. Terminal closure checks the exact
sealed reconstructions, accounting and preserved anchors. Evidence:
`/tmp/pipelang-hex-round/selection.json` and `screen-summary.json`.

### Terminal representation assessment

State: **completed research; every new transform rejected; unpromoted**.

The selection sample is 28 distinct original ELFs (12 fixed leaves plus all 16
references), totalling **188,162,524 bytes**. Every lane uses the unchanged
fixed-three reference graph and file-input Zstandard `--single-thread --long=24
-12`. All 56 selective identity components reproduce the prior encoded hashes;
the raw identity total also reproduces the prior 45,365,275-byte sample control.

| Form | Representation | Compressed bytes | Change vs identity | Preparation + verification wall s | Aggregate CPU s | Decode + inverse/hash s |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| raw | Identity control | 45,365,275 | +0.00% | 13.622 | 14.500 | 1.071 |
| raw | Ordinary hex | 54,054,729 | +19.15% | 36.274 | 38.496 | 1.937 |
| raw | Base64 | 63,951,362 | +40.97% | 25.929 | 27.541 | 2.055 |
| raw | Separated hex digits | 55,485,406 | +22.31% | 37.414 | 39.562 | 2.313 |
| raw | Four-byte columns | 47,498,120 | +4.70% | 15.364 | 16.384 | 1.324 |
| selective | Identity control | 31,004,014 | +0.00% | 19.857 | 21.041 | 1.431 |
| selective | Ordinary hex | 40,868,634 | +31.82% | 31.762 | 33.571 | 2.355 |
| selective | Base64 | 65,608,841 | +111.61% | 39.837 | 42.265 | 2.795 |
| selective | Separated hex digits | 47,625,200 | +53.61% | 34.600 | 36.581 | 2.731 |
| selective | Four-byte columns | 36,871,597 | +18.93% | 19.325 | 20.448 | 1.710 |

Wall and CPU columns sum per-object screen measurements, including input read/hash,
forward conversion, reference read/conversion/write, encoding, inverse and output
verification; they are **not full native replay or independent statistical trials**.
The matched dispatch took 149.854 seconds over 140 fresh contained units, with at
most two running together. The initial stdin diagnostic took 116.239 seconds and
is excluded from selection. Hex doubles bytes before compression; Base64 expands
by approximately 4/3, separated hex digits by 2, and columns preserve byte length.
Compressed text ratios are never used as the original-byte denominator.

The phase profile attributes most preparation time to the codec, not hex
conversion. Raw hex encode time was 32.170 seconds versus 11.158 for identity;
forward read/hash/conversion was 0.619 versus 0.514, and reference preparation
0.695 versus 0.389. Selective columns had slightly lower summed preparation wall
(19.325 versus 19.857), but added 18.93% encoded storage and increased measured
decode/inverse/hash time (1.709 versus 1.431 seconds). This single screening run
does not establish a speed improvement or equivalence. No assembly/C/SIMD work is
justified by this profile.

Every transformed raw/scaffold/token component was larger than its respective
identity control: zero wins in 336 component comparisons. All four transforms
are rejected on both forms; there is no component hybrid to advance. This is
a result for this corpus, reference graph, codec and settings, not an impossibility
claim about text encodings or other compressors. Other codecs, word widths,
bitplanes and full-corpus searches are deferred; this round converges here.

#### Inclusive accounting

Each encoded total already includes all 16 references, not just leaf deltas.
`accounting.json` conservatively charges every sample lane the full inherited
173,402,752 bytes of support and metadata (156,296,197 inherited support;
13,924,844 selective support; 1,578,076 inherited metadata; 1,603,635 selective
metadata), plus all new root-level research support uncompressed. That inherited
charge includes the codecs, fixtures/oracle, validation and reconstruction support.
This deliberately retains full-family support when reporting a sample. Reference
reconstruction during actual replay would add work; it is not measured by the
component screen and no full native latency claim is made.

At the accounting checkpoint, new uncompressed support was 1,685,879 bytes.
The conservative raw identity/hex sample totals were
220,453,906 / 229,143,360 bytes; selective identity/hex
were 206,092,645 / 215,957,265 bytes.
These are sample charges, not extrapolated 200-object representations. The final
`terminal-storage.json` inventories all retained roots including later receipts.

The main retained-storage snapshot totals **74,956,067,936 logical bytes**,
including **10,339,330,050 bytes** in the new root, before the closing audit/report
receipts. Both diagnostic and matched rounds retain all encoded alternatives,
expanded input files, copied dictionaries, scripts, logs and receipts. Scratch
copies are charged to this retained inventory rather than hidden in a claimed
minimal representation. No cleanup or relocation occurred.

#### Correctness, resources and admitted proof

All 287 contained workload units passed and their unit trees were removed. One
sandbox user-bus preflight failed before workload execution and is retained.
The initial stdin control mismatch is a retained methodology failure, not a
lossless-correctness pass substituted for matched evidence. An early summary read
preceded the final receipt and failed with FileNotFoundError; it was rerun after
completion and changed no workload or artifact.

All 280 screen reconstructions (28 objects times 10 lanes) passed exact original
ELF/DWARF hashes and immutable sealing. The verification passed 1,680 synthetic
roundtrips (240 unique kind/input combinations repeated in seven units) and 140
encoded-corruption/original-hash rejection checks. Source/toolchain/settings and
fresh fixture identities were checked before reconstruction. The current Value
and ordered Trace fixtures remain unchanged. No transformed executable is
adopted; these 280 objects were sealed, **not newly executed as a full native
family**. The previously completed 200-object/2,064-case/9,704-ordered-name native
proof, 636-test whole-suite proof, and independent warm-compiler 128MiB/5s proof
remain admitted without rerun. The older canonical fresh-family driver remains
qualified: exit 1 solely from intentional empty-cache housekeeping retention.

The matched screen peaked at 446,111,744 bytes; final reconstruction verification
peaked at **734,756,864 bytes**. Three verification units accumulated **317
memory.high events** (31/78/208); do not claim pressure-free verification. There
were zero hard-limit, OOM or swap-event increases and zero swap use. The
conservative sum of the two largest unit peaks is 1,469,501,440 bytes; it is an
upper bound, not a simultaneous sample. All units retained 1GiB hard memory,
zero swap, 128 tasks, 700MiB soft high, 800MiB proactive stop, GOMAXPROCS=4,
30-second measured limits and bounded serial codec children. The native splice
service remained sealed and its operations bounded by 25 seconds.

Final audit reverified 200 source hashes, the unchanged 108,166-entry ignored
inventory, branch/HEAD, empty index, both protected stashes, exact four-path
ownership and prior evidence/support hashes. It also checked the 2,719 protected
cache records, recorded binary hashes, sizes and key inventory; it did not
rehash every unrelated cached binary. Python syntax, both task-index YAML files
and `git diff --check` passed. Engine/package boundaries and v0.96.0 are unchanged.
Only the four admitted task documentation paths changed; prototypes and evidence
are confined to the new temporary root. No commit, push, worktree, stash mutation,
installation, service/cloud change or production promotion occurred.

Primary-source motivation: [RFC 4648](https://www.rfc-editor.org/info/rfc4648/)
defines the textual encodings; [Python base64](https://docs.python.org/3/library/base64.html)
documents standard-library conversion; [h5py shuffle](https://docs.h5py.org/en/stable/high/dataset.html#shuffle-filter)
motivates the reversible column layout. Published performance was not transferred
to this corpus. The [Zstandard CLI documentation](https://github.com/facebook/zstd/blob/dev/programs/zstd.1.md)
is the reference for dictionary/long-mode options. Detailed research notes are
`/tmp/pipelang-hex-round/research.md`.

Final evidence: `/tmp/pipelang-hex-round/final-evidence.json`, with hashed screen,
selection, verification, audit and accounting receipts. Strict original
30-second whole-suite and 14,391,156-byte storage goals remain unmet. Negative
results complete this authorized round; further research is deferred.

## Parallel reconstruction round (2026-09-07)

The user asked to try parallel worker reconstruction. This authorizes one bounded
local prototype round under the same objective; state: executing. The request
permits concurrent reconstruction workers for this experiment, replacing the prior
serial-codec scheduling restriction only as necessary to test it. All memory,
CPU-process, oracle, invalidation, exact-byte, sealing and authority boundaries
remain fixed. No compiler parallelism or subagents. Evidence root:
`/tmp/pipelang-parallel-reconstruction/`.

Profile evidence shows about 1.765 seconds summed reconstruction per 50-object
shard against 5.024 seconds elapsed, already overlapping native work. Screen one,
two and four reconstructors on the reference-shortlist representation; retain
four native workers, eight total binaries in flight, root/current reference
retention, per-worker decoding contexts and independent sealed splice services.
Advance through matched full-family native checks if bounded screening passes.
Do not promote or assume linear speedup.

### Parallel checkpoint

All three 50-object screens passed identical native names, exact reconstruction
and bounds; measured reconstruction concurrency reached 1/2/4. Screening elapsed
7.410/6.065/4.711 seconds is confounded by validation cache warming
(5.023/3.984/2.612 seconds). It is not a speedup claim. Advance the unchanged
prototype to six permutations of the three worker counts, four 50-object shards
each, at most two contained units concurrently; record startup-inclusive family
wall, cgroup CPU, post-validation work, memory and ordered native coverage.

### Parallel reconstruction terminal assessment

State: **completed research, unpromoted**. Retain **two reconstructors** as the
preferred prototype configuration; four provides no additional measured benefit.
Compressed content is unchanged at 186,439,519 bytes, preserving the existing
reference-shortlist representation. This is a scheduling experiment, not a new
compression format.

All six permutations of 1/2/4 worker counts were measured, each on the full
200-object family in four 50-object shards and at most two contained units at once.
The one-worker lane retains the previous serial-producer/four-native-consumer
schedule. All lanes use the same identity validation, reference graph, native
workers, eight-job cap and new per-service bounded pipe transport.

| Reconstruction workers | Mean family wall | Wall range | Mean cgroup CPU | Mean summed post-validation shard work |
| --- | ---: | ---: | ---: | ---: |
| 1 | 13.747 s | 12.308–17.240 s | 59.834 s | 11.676 s |
| 2 | 13.037 s | 11.964–16.677 s | 59.545 s | 10.378 s |
| 4 | 13.056 s | 12.107–15.984 s | 60.861 s | 10.623 s |

Two workers reduce mean family wall by **5.17% (0.710 seconds)** and measured
CPU by 0.48%. Four reduce wall 5.03% but increase CPU 1.72% versus one. Two
beat the same-order control in all six rounds, by 0.143–2.362 seconds, but
cache warming remains visible: the first control took 17.240 seconds, while
later controls were around 12–13 seconds. As a labelled post-hoc sensitivity
check, the last four rounds average 12.581/12.247/12.345 seconds for 1/2/4
workers: two is 2.66% faster there. Use **a modest roughly 3–5% observed gain**,
not linear scaling, statistical equivalence, a universal bound or a whole-suite
speedup. Ranges overlap. No measured work overlapped unrelated task-owned codec
preparation or checks. Shared host/cache conditions were not made deterministic.

The profile is consistent with limited benefit: validation and native execution
remain, and the original pipeline already overlaps reconstruction with native
workers. Summed post-validation shard work drops from 11.676 to 10.379 seconds
with two workers (11.1%); those overlapping shard/worker sums are not family wall.
Four worker contexts and native splice services add concurrency/CPU without
further wall improvement. Wider pools, other representations (including the
slower XZ path), validation optimization and production promotion are deferred.

#### Concurrency and correctness

Each reconstructor owns a separate decoder/library handle and sealed splice
service. Decompression contexts are fresh per call. Root/current digested
references are immutable and released only after all reconstruction readers
finish, including failure paths. Native consumers own only final sealed FDs.
The eight-job cap covers queued, reconstructing, ready and executing targets
across both pools; native concurrency remains four. Instrumentation reached
1/2/4 concurrent reconstructors without exceeding the cap.

All lanes replace the old process-wide alarm with independent nonblocking
pipe/select deadlines. The real hanging-service test rejected after 25 seconds
within its 30-second contained unit. A global alarm would not independently
govern concurrent workers. The [Zstandard C API](https://raw.githubusercontent.com/facebook/zstd/dev/lib/zstd.h)
specifies separate decompression contexts for parallel threads; the prototype
uses that model and const dictionary inputs. Detailed rationale: `research.md`.

All **18 full native replays** preserve 200 exact ELF/DWARF binaries, **2,064
cases and 9,704 identical ordered test names** each, matched against prior
reference-lane receipts. Three 50-object screens also passed. The new runtime
passed 24 existing corruption, span, sealing, source/settings and independent
current Value/Trace rejection checks, 21 synthetic roundtrips, and an additional
32 concurrent exact decodes plus 32 concurrent hash rejections with descriptor
recovery. Current source/toolchain/settings identities are checked before work;
all support imports and manifests are hash-bound in `identity.json`.

#### Resources and storage

All **77 contained workload units passed and their unit trees were removed**.
One sandbox-bus preflight failed before workload execution; narrow reviewed
host operations then used the same contained runner. A generated-import syntax
error was corrected by AST checking before any workload ran. No uncontained
compiler/native/codec checks occurred.

Peak per-unit aggregate memory was **735,113,216 bytes**;
soft-high events totalled **3,979**. There were zero hard-limit, OOM
or swap-event increases and zero swap use. Do not describe these measurements
as pressure-free. The conservative sum of the two largest peaks was
1,470,164,992 bytes, not a simultaneous sample.

All worker counts retain 1GiB hard memory, zero swap, 128 tasks, 700MiB soft-high,
800MiB proactive stop, GOMAXPROCS=4 and 30-second measured units. Native tests
and each splice-service request retain 25-second deadlines. Parallel
reconstruction changes only the previously serial reconstruction scheduling
constraint under the latest user request; no compiler parallelism was added.

No compressed payload, dictionary or reference manifest was regenerated. The
prior support-inclusive reference candidate remains 362,012,291 bytes before
charging this round. `terminal-storage.json` charges all new scripts, logs and
receipts uncompressed to each configuration equally, reports the small runtime
source subset separately, and inventories every retained prior root plus this
new root. Research receipts are retained; no cleanup or relocation occurred.

Final audit reverified all 200 production source hashes, the unchanged ignored
inventory, branch/HEAD, empty index, two protected stashes and four-path task
ownership. The 2,719 protected cache records, recorded binary hashes, sizes and
key inventory match; unrelated binaries were not all rehashed. Prior 636-test
whole-suite and independent warm-compiler 128MiB/5s proof remain admitted. The
previous fresh-family driver retains its intentional housekeeping-exit
qualification. No source drift required a whole-suite rerun.

Only the four already-owned task documentation paths changed; all prototypes
and receipts are in `/tmp/pipelang-parallel-reconstruction/`. Python AST, both
task-index YAML files and Git whitespace checks passed. Engine/package
boundaries and v0.96.0 are unchanged. No commit, push, worktree/stash mutation,
installation, service/cloud/persistent host change or production promotion.
The original 30-second whole-suite and 14,391,156-byte goals remain unmet.

Evidence: `results.json`, `screen-summary.json`, `faults-reference.json`,
`deadline.json`, `audit.json`, `final-evidence.json` and `terminal-storage.json`
in the new root. This bounded round is complete; further expansion is deferred.

## Current codec and worker reuse research (2026-09-07)

Renewed user request: another research round using the latest work. State: completed, unpromoted.
Evidence root: `/tmp/pipelang-modern-codec-round/`. Recent primary-source screening
covers OpenZL v0.2 (May 2026), DirectStorage 1.4 (March 2026), nvCOMP 5.3 (July
2026), Brevis (August 2026 preprint), and current stable Zstandard v1.5.7. Select
a bounded current-Zstandard decoder and per-worker DCtx-reuse experiment against
the unchanged two-worker reference pipeline. Keep all encoded bytes, dictionaries,
source/toolchain/settings checks, Value/Trace oracle, sealing and coverage fixed.
Temporary source builds are prototypes, with no system installation or promotion.
All compiler/native/codec work stays contained under the inherited limits.

### Modern codec checkpoint

All 448 sealed sample reconstructions passed. Old-fresh/old-reuse/new-fresh/
new-reuse summed decode wall was 3.986/4.013/4.018/4.060 seconds; no runtime
candidate advances. Current release evidence is strongest for compression and
reference patching, so this round admits one further bounded 28-object compression
screen before convergence. Preserve old decode as control, exact originals,
preparation costs and complete support accounting. No system installation.

### Compression checkpoint

All 28 objects and references roundtripped through the existing decoder. Old
dictionary-12/current dictionary-12/current patch-12/current patch-19 payloads
were 30,773,000/30,706,048/31,014,908/28,882,963 bytes. Encode sums were
19.980/21.226/10.964/77.360 seconds. Reject the negligible dictionary upgrade;
advance patch-12 (45.1% less encoding time, 0.79% more bytes) and patch-19
(6.14% fewer bytes, 3.87x encoding time) as distinct tradeoffs. Build full
200-object candidates with matched old control, then matched native replay
using the unchanged old decoder and two reconstructors. Charge full support
before deciding adoption; no gain or promotion is implied by sample selection.

An attribution check is added before terminal candidate selection: upstream
v1.4.8 source confirms that the installed CLI already supports patch mode.
Screen old-patch12 on the same 28 objects after current preparation completes.
This distinguishes a configuration gain from an upgrade-dependent gain and may
avoid new runtime/compiler-support cost. No measured run overlaps preparation.

Installed patch12 produced 31,004,825 sample bytes in 11.197 summed encode
seconds, essentially matching new patch12 without its new codec dependency.
Advance installed patch12 to full preparation as the final candidate. Freeze
the set at old-D12, old-patch12, new-patch12 and new-patch19; use four balanced
orders for terminal native proof. No further representation or runtime seams.

### Current-codec terminal measurements

The latest-source screen found [OpenZL v0.2.0](https://github.com/facebook/openzl/releases)
(May 2026), [DirectStorage 1.4 preview](https://devblogs.microsoft.com/directx/directstorage-1-4-release-adds-support-for-zstandard/)
(March 2026), [nvCOMP 5.3](https://developer.nvidia.com/nvcomp-archive)
(July 2026), and the August [Brevis preprint](https://arxiv.org/abs/2608.02162).
Their typed-stream or GPU/tensor results do not establish performance for this
CPU ELF/DWARF workload. They remain possible separate experiments. The checked
[stable Zstandard release is v1.5.7](https://github.com/facebook/zstd/releases/tag/v1.5.7);
a development changelog heading for 1.6.0 is not a released-version claim.
`research.md` records the source dates, applicability and deferred boundaries.

The final four configurations use identical three-section forms, reference
selection and original binaries. Full preparation includes 200 objects, with
every scaffold/token component decoded and compared to the original. The old
dictionary control also reproduces every prior compressed hash. Preparation
dispatch has at most two contained units; these are actual sequential-wave
configuration/build measurements, not an isolated causal codec-version study.

| Configuration | Encoded bytes | Summed encoding seconds | Preparation dispatch seconds | Mean native replay seconds | Replay CPU seconds |
| --- | ---: | ---: | ---: | ---: | ---: |
| Installed 1.4.8 dictionary-12 | 186,439,519 | 160.767 | 102.921 | 15.814 | 70.973 |
| Installed 1.4.8 patch-12 | 187,811,638 | 69.141 | 56.524 | 15.475 | 71.531 |
| Local 1.5.7 patch-12 | 188,258,319 | 88.303 | 66.154 | 15.393 | 72.472 |
| Local 1.5.7 patch-19 | 176,004,173 | 551.793 | 299.494 | 15.093 | 71.835 |

Installed patch-12 is the preferred preparation prototype: **45.08% less
dispatch wall, 56.99% less encoding time, and 0.736% more encoded bytes**.
The installed CLI already supports this mode, so this gain does not require a
codec upgrade. New patch-12 gives no reason to upgrade on these measurements.
New patch-19 reduces payload by 5.60%, but preparation takes 2.91 times the
control dispatch wall. Its payload reduction alone is not a complete-storage win.

All native replays keep the installed decoder, two reconstructors, four native
workers and eight jobs in flight. Four balanced orders give 16 full-family
runs, each with 200 byte-identical sealed binaries, 2,064 current cases and
9,704 identically ordered test names. Mean wall ranges by configuration are
14.729–18.002, 14.938–16.793, 15.242–15.597 and 14.988–15.150 seconds.
These overlap. Summed post-validation wall is 12.412/12.564/12.764/12.704
seconds; summed reconstruction-worker time is 8.583/8.719/8.789/8.821 seconds.
The apparent mean wall reductions mainly follow validation variation, so no
replay-speed gain is established. All lanes validate the same enlarged support
closure; their absolute times are not comparable to the earlier smaller-closure
parallel round as a new regression or whole-suite claim.

The 2x2 decoder/version/context screen was negative and stays rejected. No
production implementation, codec installation or language-contract change is
made. Exact ELF/DWARF reconstruction, independent current Value/Trace results,
source/toolchain/settings invalidation and immutable final sealing are preserved.


Terminal verification passed: 658 successful contained units, 448 screen sealed
reconstructions, 280 sample component roundtrips, 1,600 full-preparation component
roundtrips, 16 full native replays, 72 standard rejection checks, 96 concurrent
hash rejections, 96 concurrent exact decodes and 63 synthetic roundtrips.
Descriptor recovery and reader/dictionary lifetimes passed. The unchanged
parallel service's independent 25-second deadline proof is admitted from the
prior round. Successful units have 1 GiB hard memory, zero swap, 128 tasks,
700 MiB soft-high, 800 MiB proactive stop and 30-second measured deadlines;
at most two run together. Peak per-unit aggregate memory is 734,998,528 bytes;
the conservative sum of the two largest peaks is 1,469,599,744 bytes, not a
simultaneous measurement. There were 670 soft-high events and no hard/OOM/swap
event increases. All unit trees were removed.

Two support archive attempts timed out cleanly at 30 seconds; both artifacts
and failed receipts are retained. A single-pass gzip archive roundtrip passed
in 2.335 seconds. The archive contains snapshots of its own in-flight unit
receipts, so their final versions are additionally charged uncompressed. The
initial terminal audit detected these changing closing receipts; the corrected
terminal audit admits only those exact receipt snapshots. All archived source
inputs remain unchanged. `accounting-recovery.json` preserves this correction.
One sandbox-bus preflight and one sandbox-DNS download failure are also retained;
the reviewed source download and contained host execution subsequently succeeded.

The archive is 23,152,858 bytes and includes all new research support plus the
explicit C source/header/compiler closure, even for the installed-codec lane
that does not need an upgrade. Every candidate additionally retains the prior
175,572,772-byte complete overhead and all closing receipts. Raw and alternative
payloads, source archive, objects and failed archive outputs remain in the full
retained-root inventory. This conservative research-inclusive accounting is not
a minimal deployment-footprint estimate. The denser payload does not beat the
prior complete representation after this new support is charged.

Final complete costs are 425,639,942 bytes for installed dictionary-12,
427,012,061 for installed patch-12, 427,458,742 for new patch-12, and
415,204,596 for new patch-19. These each include 40,474,793 bytes of closing
receipts and retained failed archives in addition to the verified support
archive. The new root retains 980,637,994 bytes; all non-overlapping retained
roots total 75,945,026,006 bytes. These self-inclusive values come from
`terminal-storage.json`; unsuccessful accounting archives are research cost,
not a required codec runtime dependency.

All 200 protected source hashes, the 2,719 native-cache key/record/size entries,
HEAD, branch, empty index, ignored inventory and both protected stashes match
admission. Python AST, both task-index YAML files and Git whitespace checks pass.
The admitted 636-test whole-suite and independent warm compiler 128 MiB/5-second
proof were not rerun without source drift. The earlier fresh-family driver exit
1 still reflects intentional empty-cache housekeeping retention, not a clean
canonical driver pass. No new full-suite/cold-build acceptance is claimed.

Only the four task-owned documentation/index paths changed in the checkout;
all prototypes and generated evidence remain under
`/tmp/pipelang-modern-codec-round/`. Engine/package boundaries and v0.96.0 remain
unchanged. No commit, push, worktree, installation or production promotion.
The original 30-second whole-suite and 14,391,156-byte goals remain unmet.
This round is complete; further codec/format/GPU expansion is deferred.
Evidence: `full-compression-summary.json`, `results.json`, `audit.json`,
`support.json`, `final-evidence.json` and self-inclusive `terminal-storage.json`.


## Two final strategy iterations before implementation (2026-09-07)

User requests a couple more research iterations before returning to implementation.
State: completed, unpromoted. Evidence root: `/tmp/pipelang-next-two-rounds/`. Use the
objective-execution skill and saved checkout; checkpoints are automatic.
Iteration 1 screens installed patch levels 6/9/12/15 on the fixed 28-object
sample, with level 12 hash-matched to the prior winner. A fixed per-component
level rule may advance only if the sample improves preparation for at most 2%
additional payload; test the frozen rule on all 200 rather than selecting each
object after exhaustive encoding. Iteration 2 tests a single-root reference
topology to eliminate intermediate dictionary preparation, preserving every
original object and exact raw component. Advance only a bounded storage/time
tradeoff, with full native proof for any retained candidate. Two iterations,
no new codec/build, production changes, worktree, commit, push or installation.
Keep two reconstructors/four native workers/eight jobs, at most two contained
units, 1 GiB hard memory, zero swap, 700 MiB high, 800 MiB proactive stop, 128
tasks and 30-second measured units/25-second children. Prior protected sources,
state and proof are admitted after hash verification. Complete accounting must
separate required representation support from the inventory of all experiments,
while retaining conservative research-inclusive totals; no original target claim.


Iteration1 checkpoint: uniform6/9/12/15 sample payload is
34,009,206/31,904,934/31,004,825/30,759,700 bytes, encoding
4.191/5.949/10.451/38.243 seconds. Freeze scaffold 9/token 12: sample
31,228,152 bytes (+0.720%) and 8.472 summed encoding seconds (-18.943%).
Iteration2 checkpoint: single-root topology costs 34,743,403 bytes (+11.257%
against the mixed shortlist) and 9.626 encoding seconds (+13.630%). Reject at
screen; possible anchor-setup savings are not established as a native-speed win.
The final set is baseline12 and fixed scaffold 9/token 12 with existing references.
Run full-family preparation in both orders, compare deterministic payload hashes,
then four balanced native orders and rejection checks. No third iteration.


### Final two-iteration preparation findings

The full 200 results confirm the frozen component rule, including 172 objects
outside the selection sample. Two preparation orders yield identical output
hashes for all 800 compared components. Each execution independently verifies
original scaffold/token bytes; no per-object best-of-search cost is hidden.

| Patch mode | Payload bytes | Mean summed encoding seconds | Mean preparation dispatch seconds | Mean unit CPU seconds |
| --- | ---: | ---: | ---: | ---: |
| Level12 for both components | 187,811,638 | 74.081 | 56.765 | 94.574 |
| Scaffold9 / tokens12 | 189,350,098 | 59.240 | 47.937 | 79.120 |

Mixed levels use 0.819% more payload (1,538,460 bytes), with 20.034% less summed
encoding time and 15.551% less mean preparation dispatch wall. The first order
was 64.789 versus 50.593 seconds; reverse order was 48.740 versus 45.281 seconds.
The reduction is positive in both orders (21.91% and 7.10%) but has substantial
cache/load variation; this two-order mean is not a precise universal guarantee.
Do not multiply this gain by a different round's percentage as if it were a
single matched measurement. Native replay is assessed separately below.


### Final two-iteration native and resource proof

Four balanced native orders passed for both baseline12 and scaffold9/token12.
Each full replay has 200 byte-identical sealed binaries, 2,064 current cases
and 9,704 identically ordered names. Mean wall is 14.328 versus 13.304 seconds;
mean CPU is 66.756 versus 64.881 seconds. Ranges overlap: 12.968–17.342 and
12.963–14.170 seconds. Excluding the first order as an explicit cache-warming
sensitivity check gives 13.323 versus 13.015 seconds (about 2.31% lower).
The all-order mean difference is about 7.15%, heavily affected by the first
baseline run. This is encouraging local replay evidence, not a universal speed
claim or a reason to present the outlier-sensitive number as the primary gain.
Summed reconstruction-worker time is 8.404 versus 8.169 seconds; summed
post-validation wall is 12.173 versus 11.719 seconds. The stronger retained
result remains the preparation improvement.

All 573 contained workload units passed, with one retained sandbox-bus preflight
failure followed by reviewed contained host execution. There are 280 sample
component roundtrips, 1,600 full-preparation component roundtrips, eight full
native replays, 24 standard rejection checks, 32 concurrent hash rejections,
32 concurrent exact decodes and 21 synthetic roundtrips. Corruption, output seals,
current-source/settings invalidation, independent altered Value/Trace fixtures,
reader/dictionary lifetimes and descriptor recovery all pass. The unchanged
service's independent 25-second deadline proof is admitted from the parallel
round. No current production or compiler source changed.

Peak per-unit aggregate memory is 735,121,408 bytes; the conservative sum of the
two largest peaks is 1,469,919,232 bytes, not a simultaneous measurement. All
2,042 soft-high events occur in baseline native replay, with none in the mixed
candidate. There are zero hard/OOM/swap increases and all unit trees are removed.
Native maximum aggregate peaks are 735,121,408 bytes baseline and 373,551,104
bytes mixed; those observed peaks include cache/load effects and do not establish
a universal memory reduction. Full-preparation maxima are 135,761,920 and
124,899,328 bytes, respectively. The inherited contained limits remain unchanged.

Terminal audit verifies all 200 protected source hashes, 2,719 native-cache
key/record/size entries, previous acceptance hashes, branch/HEAD, empty index,
ignored inventory, both stashes and exactly four owned documentation/index paths.
Python AST, both task-index YAML files and Git whitespace checks pass. Prior
636-test whole-suite and independent warm compiler 128 MiB/5-second proof are
admitted without rerun because protected sources match. The earlier fresh-family
driver's intentional empty-cache housekeeping exit1 remains qualified; no new
whole-suite/cold-build acceptance is claimed.

Complete candidate accounting retains the prior parallel representation's
183,674,131-byte non-payload overhead and adds every new research script, result,
manifest and closing receipt uncompressed, plus the two explicit old-codec
preparation inputs. Each candidate charges its full 200-object payload once.
All sample/repeated/alternative payloads and earlier rejected codec-build research
remain in the separate non-overlapping retained-root inventory. The upgraded
codec is not a new dependency of either current candidate. This is conservative
research-inclusive accounting, not a minimal deployment estimate; exact closing
values are in `terminal-storage.json`.

Recommendation after these two iterations: retain two reconstruction workers,
the existing reference shortlist and fixed scaffold9/token12 patch compression
as the next implementation candidate. Single-root topology and broader codec
search are deferred. Both requested research iterations are complete, with no
production promotion, codec installation, worktree, commit or push. v0.96.0 and
engine/package boundaries remain unchanged. Original whole-suite speed and
14,391,156-byte goals are still unmet.

Evidence in `/tmp/pipelang-next-two-rounds/`: `iteration1-selection.json`,
`iteration2-selection.json`, `preparation-results.json`, `results.json`,
`replay-sensitivity.json`, `resource-summary.json`, `audit.json`,
`final-evidence.json` and self-inclusive `terminal-storage.json`.


## Selected implementation completed (2026-09-08)

The explicit implementation go-ahead has been executed in the saved checkout.
The exact scaffold9/token12 representation and frozen selected reference
assignments now have repository-owned preparation and two-worker replay support
under `tests/containedexec`, with no embedded research paths. All 559 contained
units passed; nine full native replays each preserve 200 exact debug-bearing
binaries, 2,064 current cases and 9,704 ordered names. Final comparison
preparations reproduce 1,600 prototype/control payload hashes.

Matched preparation dispatch improves 5.82% versus patch12, with 0.819% more
payload. Packed native dispatch averages 12.562 s versus raw 11.886 s, a 5.69%
cost. Parent inventory/accounting and the qualified empty-cache family-driver
exit are reported separately. No new whole-suite or strict target claim.
See [the implementation acceptance](conformance-execution-performance.md#repository-owned-transcript-implementation-accepted-2026-09-08)
for source ownership, rejection/resource proof, inclusive storage and limitations.
Temporary acceptance evidence is `/tmp/pipelang-transcript-integration/`.
State: completed; no further research, cache cleanup, commit or publication.


### Reusable-framework correction authorized (2026-09-08)

The user explicitly authorized correcting the fixed-corpus integration and
requested one handoff. State: ready_for_execution; implementation approved.
The prior exact-replay evidence remains admitted for its components and corpus.
It does not satisfy reusable framework integration. See “Corrective generalization
authorized” in `conformance-execution-performance.md` for the corrected objective,
normal-path integration, new/changed input handling, fresh full-suite requirement,
remaining boundaries and first checkpoint.
