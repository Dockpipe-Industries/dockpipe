# PipeLang performance and compression

Execution follow-up completed: bounded toolchain-buffer reuse is retained, streaming
fixtures are rejected, and complete fresh verification plus independent acceptance
passed in 112.87 minutes. The approved implementation results are recorded below.

## Recommendation

Prioritize repeated artifact and toolchain verification, then bounded scheduling, before investing in a new native compression format. The accepted v0.113.0 proof took **119.43 minutes** including independent acceptance. Its suite recorded **4,088 summed worker-seconds in artifact identity and verification**, compared with **11 seconds compiling and linking new cached bundles**. Faster compilation cannot substantially shorten this particular warm workflow. The larger opportunity is performing the same trustworthy verification with less repeated work.[^1]

Keep ordinary native execution as the default. Previous complete compressed execution was 63.61% slower while its selected representation was 81.89% smaller than the represented executable set. Those are different outcomes. Original executables remained, so actual freed storage was zero. The prior implementation is useful as an optional storage mechanism, but the evidence does not support making it the normal performance path.[^3]

For broader compression research, favor an adaptive system that combines exact deduplication, bounded reference deltas, and structure-aware coding, with an ordinary raw fallback. There is no lossless algorithm that makes every possible input smaller. There is substantial room to exploit recurring structure across real workloads, provided reference data, decoding work, memory, and random-access costs are counted. A new contained screen of 128 verification records supports this distinction: 16-record compressed blocks were 88.2% smaller including their index, but slower to read than the original records.[^2]

The recommended next implementation target is **instrumented, safely amortized identity verification**, beginning with the existing measured-pair mechanism and finer phase attribution. Cross-process immutable identity reuse is a second, more demanding experiment. Compression should initially target inactive artifacts or immutable evidence archives; it should enter the execution path only after a matched end-to-end win.

## Current verification economics

### Accepted scope and timing

The baseline is the saved `js/pipelang` checkout at `5173d4c1c2fe3560f9b8998313326d15c7ded862`. All 33 accepted source postimages were rehashed with zero drift. The completed proof was admitted without rerunning it. The campaign executed all 871 test functions and 8,859 logical suite cases, 2,922 fresh isolated compiler cases, nine integration checks, and one editor suite.[^1]

The **49,465 inherited ordered audits and native children** match the preceding proof. The current campaign contains **49,526 total native children**, including the new coverage. These counts must not be substituted for each other.

| Sequential component | Seconds | Minutes | Share of final proof |
| --- | ---: | ---: | ---: |
| Suite stage, including preparation and final reporting | 6,213.40 | 103.56 | 86.71% |
| Isolated compiler matrix | 633.92 | 10.57 | 8.85% |
| Integration and editor stage | 81.20 | 1.35 | 1.13% |
| Other outer-job time | 4.81 | 0.08 | 0.07% |
| Independent acceptance | 232.42 | 3.87 | 3.24% |
| **Total** | **7,165.77** | **119.43** | **100%** |

Stage intervals come from consecutive monotonic controller events on one boot. The suite summary separately reports 6,213.22 seconds; its measurement boundary differs slightly. The outer job reports 6,933.34 seconds. These measurements reconcile without adding concurrent worker times to elapsed wall time.[^1]

The suite's timing window is 6,186.32 seconds, containing 9,818.52 summed workload seconds across two workers. Workload occupancy is 79.36%. Summed per-unit outer elapsed time is 11,063.44 seconds, leaving 1,244.92 worker-seconds between measured workload and unit outer time. A further 1,309.21 worker-slot seconds lie outside those unit intervals. These are accounting residuals containing launch, coordination, reconciliation, waiting, and window-boundary effects; they are not measured idle CPU or entirely removable overhead.

### Measured phases

| Phase | Summed worker-seconds | Observations | Interpretation |
| --- | ---: | ---: | --- |
| Artifact identity and verification | 4,088.26 | 16,375 | Leading measured target; includes several operations |
| Generated evaluator | 1,373.11 | 22,235 | Repeated semantic work; profile before optimizing |
| Native execution | 1,257.48 | 49,526 | Fresh children are required |
| Parser/typechecker/HIR/Core | 412.63 | 52,169 | Smaller than verification overhead in this run |
| Generated build/run wrapper | 205.10 | 527 | Nested wrapper; do not add to child phases |
| Independent oracle | 151.57 | 6,507 | Required independent expected results |
| Source/fixture materialization | 120.78 | 16,375 | Fresh fixtures remain mandatory |
| Fixture serialization | 35.96 | 6,507 | Small measured component |
| Evaluator preparation | 34.07 | 22,235 | Existing preparation optimization remains useful |
| Compilation and linking | 11.06 | 21 | Only new retained-bundle misses, not all compiler work |
| Artifact publication | 0.37 | 21 | Not a meaningful current bottleneck |

These are instrumented wall spans summed across workers, **not CPU seconds**. Some spans nest or duplicate each other. In particular, `generated_native_run` closely duplicates `native_execution`, and `generated_shared_link` duplicates compilation/linking. Adding every phase would double-count work. The admitted current receipts do not expose aggregate `cpu.stat` totals or a complete CPU profile; total CPU consumption is **unknown**, not zero.[^1]

The largest measured families are v0.109 layouts (2,633.07 workload-seconds), v0.111 subsets (1,222.66), v0.108 layouts (823.13), and v0.107 layouts (808.54). Together they account for about 55.9% of suite workload time. A representative next profile must cover these families as well as compiler-memory and special-harness cases.

### Identity work and locality

The suite had 16,354 verified cache hits and 21 misses: **99.87% hits**. Its 15,142 referenced executable identities contain 70,973,989,613 logical binary bytes, or 66.10 GiB. Binary records raise that to 70,976,987,729 bytes. The configured pinned-cache accounting scope held 104,899,071,378 bytes against a 112-GiB limit, approximately 15.36 GB of logical budget headroom. This is one defined cache scope, not an inventory of the entire machine.[^1]

The current helper computes a toolchain digest once **per test process**. It walks `bin`, `pkg/tool`, `src`, `go.env`, and `VERSION`, excluding `_test.go`, and hashes file contents plus identity metadata. Live metadata inspection found 9,366 participating files totaling 171,984,742 bytes. The current singleton profile has 7,536 artifact-using processes. The product is **1,296,077,015,712 logical content bytes**, approximately 1.30 TB, plus roughly 70.6 million file visits. This is a source-derived repetition estimate, not a measured physical-I/O count.[^4]

The identity timing bucket also contains source-key construction, locking, cache reads, and executable hashing. It therefore does not prove that all 4,088 seconds are toolchain hashing. Cache-hit binaries are checked and subsequently copied, hashed, and sealed for safe descriptor execution. Existing sealing already amortizes the executable across fresh children within a bundle. Recommending another generic “hash once” cache without identifying its lifetime would repeat work already implemented.

A receipt-only application of the unchanged two-case planner produces **3,160 pairs and 2,539 singletons**, reducing outer groups from 8,859 to 5,699. This is a planning result, not a freshly admitted executable profile or a measured speedup. Exact source, host, settings, and artifact admission still apply. No widened grouping rule is implied.[^1][^4]

### Memory and comparison limits

The outer job peaked at 1,614,200,832 bytes, with zero swap and OOM events. Its hierarchical counters recorded 206,895 `high` and 213,242 `max` events. The independent acceptance job peaked at 536,920,064 bytes, with 166,952 `max` events and no OOM or swap. The maximum suite-unit aggregate peak was 296,443,904 bytes. Isolated compiler maxima were 88.34 MiB RSS and 1.577 seconds, within the unchanged 128-MiB/5-second ceilings.[^1]

Linux documents `memory.high` as a reclaim/throttling boundary. Hierarchical event counts include descendant activity and do not measure time stalled or prove that the shared hard cap was breached. The evidence justifies collecting pressure and CPU/I/O counters next; it does not justify increasing the limits or attributing the whole slowdown to reclaim.[^5]

The preceding v0.112 suite had 8,837 cases, 49,465 children, 8,400.76 workload-seconds, and a 5,316.54-second timing window. The latest window is 16.36% longer despite a 0.25% case-count increase. These are different language versions, binaries, cache states, and uncontrolled host periods. They expose a profiling question, not a causal enum-performance regression. Neither this baseline nor the compression sample is a controlled cold-cache measurement.

## Prior experiment dispositions

Historical results below were checked against the repository's durable experiment journals. Their workloads predate v0.113, and some underlying scratch paths may no longer be available. They are historical measurements, not new benchmarks on the current corpus.[^3][^6]

| Experiment | Measured outcome | Disposition for this objective |
| --- | --- | --- |
| Immutable `PreparedProgram` evaluation | Repeated evaluation 11.6–13.5× faster in two samples; preparation broke even after about four calls; representative layouts improved 26.6–30.4% | Retain; include copying, preparation, and retained heap in any new extension |
| Immutable-local environment copying | Matched profiled complete suite 307.97 → 273.53 seconds; cgroup CPU 1,068.20 → 948.64 seconds | Retain; the later 234.38-second result includes sealing and semantic-key improvements too |
| Unrestricted/vector-budgeted early batching | Sample time improved, but some peaks rose to approximately 398 MiB | Rejected under that experiment's no-memory-regression condition; not evidence for arbitrary batching |
| Shared ordinary native bundles | Matched warm execution 388.01 → 377.16 seconds; selected bytes 12.69 → 11.01 GB | Retain bounded framework; fresh children and independent fixtures remain |
| Durable campaigns/reporting/cache reuse | Suite 10,802.49 → 4,297.08 seconds; logical serialization about 81.14 GB → 153.40 MB | Accepted workflow result; warm-cache advantage prevents attributing all gain to the ledger |
| Measured pairs, performance round two | Full job plus acceptance saved 282.56 seconds, 5.76%; small warm samples averaged 1.34% slower | Explicit option for matching warm full campaigns; singleton fallback remains |
| Full ordinary versus packed framework | 381.43 → 624.06 seconds execution; selected representation 2.299 GB versus 12.691 GB executable bytes | Packing was 63.61% slower and included 151 first preparations; retain opt-in only; zero freed originals |
| Exact ELF/DWARF transcript reconstruction | Exact bytes and rejection checks passed; several transcript forms decoded more slowly than ordinary deltas | Exactness is feasible; exactness alone is not performance evidence |
| Hex/Base64/separated-digit/reversible layouts | No winners in 336 comparisons; hex increased compressed raw bytes about 19.2% | Do not repeat without a new structural hypothesis |
| Reference shortlist and extra sections | More sections reduced payload but raised CPU/reconstruction cost; single-reference approach lost sample size and preparation | Keep bounded selection; no universal best reference |
| Two versus four reconstructors | Two improved sampled full-family wall time about 5.17%; four added no benefit and used more CPU | Do not equate parallel codec work with a full-suite improvement |
| Zstandard version/level and scaffold/token tuning | Installed patch-12 sharply improved preparation; newer high compression spent much longer and added support cost | Reuse the established disposition; version upgrades are not automatically wins |

Several figures are easy to misuse. The earlier packed result offers approximately 10.39 GB of **potential replacement** savings for that executable set, but it retained both representations. The durable-ledger serialization reduction measures logical serialized output, not physical writes. A speed gain on 200 bundles is not proof for 15,142 current artifacts. A cache-sensitive warm replay is not an inclusive cold preparation result.

## Compression limits and useful generality

Lossless compression must encode enough information to reconstruct every original byte. For fixed input length `n`, there are `2^n` possible bitstrings, but only `2^n − 1` bitstrings of length less than `n`. An injective encoding cannot map every input to a shorter output. Some inputs must remain as large or grow. Recompressing arbitrary outputs forever cannot escape the same counting argument; headers, dictionaries, and reconstruction programs carry information too.

Shannon's source-coding result concerns average code length for a source distribution. Better modeling of a real distribution can approach its entropy more closely. This is compatible with large gains on repetitive binaries or structured logs and does not imply universal shrinking of arbitrary bytes.[^7]

Three different claims should remain distinct:

1. **Corpus-specific:** a fixed reference set makes known binaries smaller. Useful locally, but weak evidence for future versions.
2. **Domain-general:** a content-derived planner handles unseen binaries or record schemas, with invalidation and fallback. This is a realistic framework objective.
3. **Broad-purpose:** the system selects among general algorithms across unrelated formats. Benefits vary by data; no guaranteed reduction on all inputs.

A reversible transform, such as byte shuffling or hex conversion, preserves information but changes what a particular compressor can easily discover. Good transforms expose structure; poor ones add representation overhead. A hash is an identifier, not a replacement for unknown original bytes unless an external store retains those bytes. Source-plus-compiler is a reconstruction recipe only if all required output bytes are reproducible, and the source, toolchain, settings, dependencies, and regeneration costs are included.

For this project, replacing exact ELF/DWARF with a semantically equivalent rebuild is not admissible wherever exact native/debug bytes are required. Likewise, summary statistics, approximate traces, reduced test vectors, or semantically similar model outputs are not lossless substitutes for the accepted proof.

## Alternatives supported by research

### Fast independent frames and shared dictionaries

Zstandard offers a broad speed/ratio range; LZ4 emphasizes rapid decoding. Their official benchmarks use specific corpora and machines, so published MB/s figures should not be transferred to this host's filesystem, process-launch, and sealing workload. Compare whole read/decode/verify/execute cost, with the same input transport and cache state.[^8][^9]

A trained Zstandard dictionary helps correlated small records by providing useful context at the beginning of each frame. The official API guidance places its strongest use in many similar small files and warns that gains diminish for larger inputs. Train on an earlier campaign and hold out later families; count the dictionary even when it is already cached. This is a better next dictionary target than treating every several-megabyte ELF as an independently tiny message.[^10]

The Zstandard format itself does not provide random access inside a stream. Use independently decodable bounded frames plus an authenticated index when records must be fetched separately. The 2025 Shared Brotli RFC also formalizes external shared dictionaries and requires encoder and decoder to use identical context; it is a credible comparison for small correlated resources, not evidence of a local win.[^11][^12]

Recommendation: use ordinary Zstandard and LZ4 as mandatory controls for any custom format. For evidence archives, test a library API and independent frames before designing a new compressor. Keep active atomic receipts in their existing form until crash consistency and resume semantics are proven through a real archive reader.

### Exact deduplication, content-defined chunks, and deltas

Whole-file deduplication captures identical objects; chunk deduplication can also reuse unchanged regions between related binaries. Fixed-size chunks are simple but lose alignment after insertions. FastCDC chooses boundaries from content to improve shift tolerance while reducing chunking overhead. Its paper measures the chunking/deduplication problem; that does not establish faster executable reconstruction under PipeLang's limits.[^13]

Reference deltas encode a target relative to a retained source. VCDIFF explicitly models this relationship. A practical system must charge the reference, choose it cheaply, bound dependency depth, and verify the reconstructed target. A small patch against a large uncounted reference is not a small standalone representation.[^14]

DeepSketch studies learned similarity search for choosing delta references. Its headline reduction gain is accompanied by a substantial throughput tradeoff: the paper's overhead analysis reports average throughput around 44.6% of its Finesse comparison for DeepSketch. The lesson is to benchmark reference search and restore, not only delta size. Existing PipeLang shortlist experiments already show that more aggressive matching can lose on execution economics.[^15]

Recommendation: investigate a **cold-object store with bounded independent chunk groups**, not a solid archive that requires scanning much of the corpus for one executable. Compare fixed-size chunks, FastCDC, and the existing bounded reference planner on a future-version holdout. Start with exact identity duplication and stored-byte accounting; only progress to a native replacement candidate if inclusive retained bytes justify its decoding and support complexity.

### Structure-aware compression

OpenZL provides a framework for composing reversible transforms and codecs around a data description, while retaining a common decoder. Its design groups homogeneous streams so operations such as integer deltas and field coding can expose redundancy that generic byte compression misses. This is the strongest broadly reusable direction for structured receipts, numeric vectors, and compiler metadata.[^16]

Current official limitations materially affect this project: the documentation warns about inputs larger than 500 MB, describes memory use that can be about ten times input size, and says streaming is not currently supported. These are reasons to use small independent chunks, not to relax containment. The prior research already identified OpenZL as a future seam; no local OpenZL benchmark or installation was performed here.[^17]

An exact JSON archive must restore whitespace, key order, number spelling, and every other byte if identity is defined on the original file. Parsing to values and emitting a normalized JSON object would preserve some semantics but fail that identity requirement. A format-specific residual stream or a byte-preserving parser is therefore part of the design and its cost. Already compressed DWARF requires the same care: recovering equivalent debug information is weaker than reproducing the original compressed section.

Recommendation: begin with naturally typed new metadata streams or a separately versioned immutable export. Do not rewrite existing accepted receipts just to obtain a better compression ratio. Freeze any learned format selection on training data before evaluating a later campaign.

### Learned lossless compression

Predictive models can be coupled to arithmetic coding to produce lossless compression. *Language Modeling Is Compression* explicitly distinguishes raw coded size from adjusted size including model weights; large models can lose badly once the model is charged. Its discussion also notes the high cost of online training during both encoding and decoding.[^18]

The practical decision is a two-part-code budget. If a model occupies `M` bytes and improves compressed fraction by `delta` relative to a conventional compressor, the storage-only break-even input size is `M / delta`, before runtime support. A hypothetical 2-GB model improving by 0.10 bytes per input byte needs more than 20 GB of input to cover its own size. This arithmetic is a scenario, not a measured prediction for PipeLang.

More recent work reinforces the importance of matching the domain. Mao, Pirk, and Xue study models compressing LLM-generated text using next-token predictability. A June 2026 preprint by Tan and colleagues studies lossless weight compression with decoding fused into GPU matrix operations; its hardware evaluation uses A100/H200 systems. Neither establishes a fast CPU ELF compressor or a verification-harness improvement. The latter is an example of compression helping computation when decoding is integrated at the point of consumption, rather than through an extra full-buffer reconstruction.[^19][^20]

Recommendation: defer neural byte compression for the initial performance objective. It would need a small deterministic decoder, bit-exact probability agreement, model-inclusive storage, and competitive bounded CPU/memory. Do not substitute statistically preserved model quality or lossy quantization for exact bytes. Keep learned **reference selection** as a later experiment only if cheaper shortlist methods leave enough measured value unclaimed.

### Immutable identity and computation on compressed data

An immutable snapshot can amortize verified identity across processes without relying on mutable path/mtime metadata. Linux fs-verity makes enabled file contents read-only, verifies data through the page cache, and exposes a constant-time measurement ioctl. Its digest is a Merkle-based identity, different from ordinary full-file SHA-256, so a migration must bind the two identities rather than compare them interchangeably. Filesystem support must be established, and mutable directory/path replacement remains a separate concern.[^21]

This is an architectural candidate, not an authorized host configuration change. Existing sealed descriptors are the nearer precedent. A campaign-scoped bounded service or snapshot could distribute already verified immutable objects, but it must handle descriptor lifetime, crashes, mutation of original paths, fresh children, and the shared memory budget. A read-only flag, stored digest, timestamp, or signature over writable content is insufficient protection by itself.

Go's PGO is a separate later option: it uses representative CPU profiles and may improve compiler decisions. Official guidance warns that narrow microbenchmarks are often poor profile sources. Profile provenance must become part of build identity, with explicit settings and separate acceptance; silently dropping a `default.pgo` into the checkout could invalidate the normal-build baseline.[^22]

The general opportunity is to avoid materializing redundant intermediate data, not merely compress it after creation. For example, a streaming acceptance reducer can consume bounded decoded records while retaining only required identities. Such a design still validates all receipts and independent proof. It cannot replace fresh semantic execution or omit vectors to gain speed.

## Compiler and framework consumption of compressed data

### Avoiding full reconstruction

Compression can be part of the consumer's representation rather than an archive
layer placed in front of it. This merits a separate experiment from native-file
packing. It has two useful forms:

| Approach | What disappears | What remains |
| --- | --- | --- |
| Bounded streaming decode into a consumer | Full expanded file, large temporary buffer, and potentially a second parse/copy pass | Decoding each consumed value; compressed-block buffering |
| Operations over encoded data | Expansion of values that the operation can handle in encoded form | Representation-aware algorithms, indexes/dictionaries, fallback for unsupported operations |
| Demand-decoded native pages | Eager restoration of the entire executable | Decoding requested instruction/data pages before native use |

For example, a dictionary representation might store one copy of `"approved"`
and an array of small identifiers. Equality against that value can compare
identifiers if their dictionary identity is established; there is no need to
allocate a new string for every entry. A run represented as `(value=7,count=1000)`
can support a count, or a mathematically valid sum, without producing a thousand
values. Such shortcuts require the operation's semantics to permit them: they
must not erase effects, overflow behavior, test invocations, or ordered traces.

This is established systems research. Abadi, Madden, and Ferreira studied
integrating compression with database execution, selecting encodings according
to both data and operations. DuckDB's execution format provides constant,
dictionary, and sequence vectors that avoid flattening every value into a full
array at every step. Their results support the architectural possibility, not
a speed prediction for PipeLang.[^24][^25]

### Concrete PipeLang entry points

The v0.109 generated fixture loader currently calls `os.ReadFile("oracle.json")`
and `json.Unmarshal` into a complete `[]row`, then loops through its flags,
expected values, and ordered traces. Similar loaders exist in several other
selector families. The batching helper also clones fixture byte slices and
writes current fixture files for fresh child execution. These are concrete
materialization boundaries to profile, not hypothetical missing APIs.[^27]

The first candidate should be an exact-byte compressed fixture stream feeding
a bounded row iterator directly into the existing native checks. It preserves
every row, value comparison, trace comparison, native child, fresh fixture
directory, and original ordered audit identity. Hash the original logical bytes
while streaming when required. That avoids retaining a full expanded fixture,
but hashing every original byte still requires producing every byte; a hash of
the compressed container is not a substitute. Require end-of-stream, exact row
count, and corruption checks before admitting successful proof.

The second candidate is a versioned typed fixture representation: bit-packed
flags, dictionary-backed expected strings and trace labels, offsets for trace
sequences, and bounded independent blocks. Native checks consume views of these
structures, materializing only an individual value or trace when an existing
comparison needs it. Dictionary equality requires compatible dictionary
identities; numerical identifier order does not imply string sort order.
Expected values must still come from the independent oracle, never from the
compiler output being checked. Repeated expected values do not authorize running
fewer test vectors.

A typed representation changes the serialized fixture format. Treat it as a
separate candidate with explicit compatibility and audit mapping, retaining the
current format as the control. Do not silently replace exact accepted JSON bytes
with a semantically similar serialization. Both formats should remain comparable
through complete ordered logical rows and the existing byte-level controls.

At the compiler level, analogous techniques could use packed internal tables,
interned immutable strings/types, or indexed shared immutable nodes instead of
expanding everything into separate heap objects. Consumers must traverse that
representation directly for the saving to survive. Ownership, source locations,
mutation isolation, malformed-input rejection, and debug behavior remain part
of correctness. This would be a new representation design; the existing
`PreparedProgram` already copies and validates an owned Core snapshot, so it is
not evidence that the compiler currently runs on compressed Core.[^27]

### Native execution boundary

The current Go native executable cannot execute arbitrary Zstandard/XZ bytes as
machine instructions. A loader can decode portions as needed, but that moves
decoding rather than eliminating it. Squashfs illustrates this distinction:
compressed file data is decoded into the normal page cache. PipeLang's current
whole-file verification and sealed-memfd snapshot would still force a complete
pass before execution, defeating much of a demand-loading design unless that
architecture were deliberately changed and re-proven.[^26][^4]

A compact bytecode interpreter could consume a purpose-designed instruction
format without reconstructing an ELF. That is an alternative execution engine,
with dispatch, portability, compiler, and debugging tradeoffs. It would not
replace the required Core-only Go/native acceptance path merely because it is
smaller. Keep it outside the first experiment.

### Hybrid native and compact execution

A single framework can support both native execution and compact representations.
The native-instruction boundary does not require choosing one execution mode for
the whole program. There are two distinct combinations:

1. **Native code over encoded data.** Keep generated Go/native functions and let
   them consume packed flags, dictionary identifiers, and streamed records through
   representation-aware readers. The CPU runs ordinary instructions; those
   instructions operate on compact data. This is compatible with the current
   backend architecture and is the nearer-term experiment.
2. **Native and interpreted functions in one runtime.** Lower validated Core to
   a compact instruction representation for less frequently executed functions,
   while compiling selected functions to native code. Both routes use a defined
   call boundary, value layout, error behavior, and debug/source identity. A small
   native interpreter consumes compact instructions; it need not reconstruct a
   complete native executable for each interpreted function.

V8 demonstrates the second architectural pattern: its interpreter and native
compilers cooperate, and Sparkplug documents compatible stack frames that support
debugging and transitions between execution tiers. These are precedents for
feasibility, not evidence that their implementation complexity or speed transfers
to PipeLang.[^28][^29]

For PipeLang, the proposed common owner would be validated target-neutral Core.
An explicit build profile could select compact or native function implementations
ahead of time; a JIT is not necessary for an initial hybrid. A future adaptive
runtime could compile frequently executed functions and bound its native-code
cache, but then compilation time, eviction, and transition behavior become part
of the cost. Bytecode decoding and interpreter dispatch still occur; what is
avoided is full ELF reconstruction for the interpreted portion. The existing
tree evaluator is not automatically a compact-bytecode runtime.

The hybrid cost must include the interpreter, compact instructions, selected native
functions, metadata, adapters, and any retained duplicate representation. Function
boundaries also need correct GC roots, ownership, calls, errors, traces, and source
mapping. Deciding at function entry is simpler than switching an active stack in
the middle of a loop. Keeping a native code cache can amortize repeated work within
a lifetime, but each required fresh validation child remains fresh.

Support for native/compact execution is therefore a coherent long-term design
candidate, while native code consuming encoded fixtures can be tested first
without adding another backend. New interpreter agreement would be additional
proof, not a replacement for the current complete Core-only Go/native checks.
No shared-library or plugin shortcut is selected, and no whole-program speed or
storage improvement is claimed until all retained components and transitions are
measured.

### Experiment decision

Add **direct fixture consumption** as the first experiment in the
compression-for-execution track, alongside the separate identity-performance
track. Compare three paths on the same frozen fixtures: existing raw JSON
materialization, exact compressed streaming into rows, and a typed encoded view.
Measure fixture preparation, total fresh-child time, CPU, allocations, peak
memory, read bytes, stored bytes including support, and cold/warm behavior.
Do not compare decoder throughput alone.

The current native-execution spans include child startup, fixture loading, and
the actual checks; they do not isolate fixture decoding. The existing 35.96-second
fixture-serialization figure measures producer work and cannot predict the
consumer benefit. First instrument the load/parse/check boundary. This proposal
is source-grounded design analysis only: no streaming reader, typed fixture
format, compiler representation, or performance gain has been implemented or
measured here.

## New bounded record screen

The sample was frozen before execution: 128 evenly spaced `unit.json` files across the accepted suite schedule, including endpoints. Each file is 3,670–4,378 bytes; the sample totals 485,477 bytes. These are resource/unit reports, not the full semantic receipt corpus. Sampling avoids tuning to one favorable family but is not a random estimate of all verification artifacts.[^2]

The screen used installed Zstandard 1.4.8, level 3, one codec thread, file input, three alternating orders, and either one record per frame or 16 length-prefixed records per frame. The index includes record length, original SHA-256, and ordinal. Every decoded buffer matched its original bytes, and all records parsed successfully: **768 exact record roundtrips**. The screen does not implement production archive admission or corruption recovery.

| Sample measure | Raw records | One record/frame | 16 records/frame |
| --- | ---: | ---: | ---: |
| Original bytes represented | 485,477 | 485,477 | 485,477 |
| Compressed payload | — | 227,463 | 43,590 |
| Index bytes | — | 17,427 | 13,707 |
| Payload + index | 485,477 | 244,890 | 57,297 |
| Potential reduction before codec support | — | 49.56% | 88.20% |
| Median encode wall | — | 156.14 ms | 9.70 ms |
| Median encode CPU | — | 89.44 ms | 7.06 ms |
| Median read/decode/check/parse wall | 6.81 ms | 210.20 ms | 16.21 ms |
| Median read/decode/check/parse CPU | 6.81 ms | 89.50 ms | 8.91 ms |

CPU includes the probe and waited codec children. CLI process startup is included. The raw control reopens original files, verifies SHA-256, and parses JSON; the compressed path compares reconstructed bytes against retained originals and parses. These checks both establish sample identity, but this is not a matched production-reader benchmark. Raw controls ran after compressed trials; no cache flush, storage synchronization, or background-load isolation occurred. Timings measure read/codec/check behavior, not durable archive publication.

Sixteen-record blocks were much faster than 128 separate CLI decoders but still about 2.38× the raw control's read time. A single-record lookup would also have to decode its containing block; up to 16 records of reconstruction is the design bound, not a measured random-access latency. An in-process decoder might reduce startup cost and needs its own measurement.

The sample's 428,180-byte potential replacement saving does **not** include codec support. The installed Zstandard executable and resolved shared-library closure total 3,773,424 bytes; the probe helper adds 4,236 bytes. Charging this entire codec closure to this small sample gives **3,834,957 bytes** including its 16-record representation, already larger than the raw sample. Existing Python/harness runtime is common to both alternatives and is excluded from this incremental codec charge. A standalone distributable archive must include that common runtime too. At the observed per-record saving, roughly 1,130 similarly sized records would amortize the charged codec support; this is a size-only extrapolation.

The accepted contained job took 1.516 seconds, with 36,843,520-byte aggregate peak. Its workload unit took 1.254 seconds, peaking at 26,763,264 bytes. Both cgroup trees were removed, with zero high/max/OOM/swap event increases. The initial sandbox bus preflight failed before workload launch; its receipt remains separate from the successful reviewed host invocation. No native executable, compiler test, production reader, or existing receipt was changed.[^2]

**Disposition:** retain this as evidence for a future immutable archival format. Do not adopt it for active verification speed. Actual freed original bytes: **zero**. The sample ratio is not a prediction for the 70.97-GB executable set.

## End-to-end cost model

For any candidate, report at least:

`retained bytes = payload + references/dictionaries/models + indexes/manifests + decoder/support + retained originals + retained alternatives`

Separate common support from candidate-specific support explicitly, and count each once. Also report peak temporary reconstruction bytes and filesystem allocated bytes where measured. Logical file length does not establish physical allocation, deduplication, or bytes actually returned to the filesystem.

For time, use:

`T = preparation + admission + scheduling + fresh semantic work + reconstruction + verification + reporting + independent acceptance`

Terms must be exclusive when summed. Report total CPU separately, including children and preparation. A cold candidate against a warm control, or a replay-only candidate against a control including compilation, is not a valid speed comparison.

For a read-once uncompressed object of size `U`, compression fraction `r`, effective storage throughput `B`, and decoder throughput `D` measured in uncompressed bytes/second, a simplified serialized model favors compression when:

`U/B > rU/B + U/D + extra overhead`

Ignoring overhead gives `D > B/(1-r)`. With an illustrative 1-GB/s read path and `r = 0.2`, decoding must exceed 1.25 GB/s merely to break even; with a 10-GB/s effective memory path, it must exceed 12.5 GB/s. These are scenarios, not measurements of this host. Pipelines can overlap work, but decompression, copying, hashing, and sealing may compete for memory bandwidth and must be measured together.

For repeated runs, a one-time preparation cost `P` is amortized only if the candidate saves time per run: `runs_to_break_even > P / (Traw − Tcandidate)`. If `Tcandidate >= Traw`, additional repetitions never produce a time break-even under those conditions. A storage benefit may still justify a colder tier.

### Performance scenarios, not forecasts

If a fraction of the measured identity bucket could be removed and every saved worker-second translated ideally across two busy workers, the arithmetic would be:

| Identity-bucket reduction | Ideal wall saving | Hypothetical final-proof time |
| --- | ---: | ---: |
| 25% | 8.52 minutes | 110.91 minutes |
| 50% | 17.03 minutes | 102.40 minutes |
| 100%, unattainable zero-cost illustration | 34.07 minutes | 85.36 minutes |

This model excludes new admission/service cost, scheduling changes, cache effects, and fixed validation requirements. It is not an estimate that safe snapshot reuse will achieve 25% or 50%. It shows why splitting the identity bucket matters. Eliminating the 11.06-second new-bundle compilation bucket, under the same ideal conversion, saves only about 5.53 wall seconds.

The 2,554.13 worker-slot-second residual similarly corresponds to 21.28 minutes under an impossible perfect-occupancy interpretation. It includes useful containment, coordination, and reporting, not just avoidable idle time. Do not add this entire hypothetical saving to the hashing scenarios: mechanisms such as grouping affect both and can overlap.

## Prioritized experiments and implementation recommendations

### 1. Identity attribution and existing measured pairs

**Priority: highest; generality: high across repeated build/test workloads.** Split the identity bucket into toolchain enumeration/hash, generated-source hash, lock wait, cache binary verification, and sealed-copy/hash. Add aggregate CPU, I/O bytes, page faults, and pressure counters at existing cgroup boundaries; record their measurement overhead. Keep full hashing and identities intact.

Freeze a representative selection of 48 cases across the four dominant families, older layouts, heavy shapes, memory cases, and special harnesses. Use four balanced singleton/pair orders under the existing two-worker policy, with exact current profile admission. Compare admission plus execution plus reconciliation, not only group body time. Preserve flattened case order, current Value/Trace results, native-child counts, and fresh fixtures. Missing/cold/changed identities must fall back conservatively.

Select the next implementation seam only from the resulting exclusive costs. Existing measured pairs have full historical acceptance; the new 3,160-pair eligibility count is only planning evidence. Do not increase group size or worker count as part of this experiment. A consistent small gain with unchanged resources is preferable to a larger noisy gain that requires a different proof contract.

### 2. Safe reuse across immutable identity lifetimes

**Priority: high after attribution; generality: high; engineering risk: moderate.** Compare process-local grouping with a bounded immutable snapshot mechanism. Capture complete toolchain/build input identity once, ensure subsequent consumers cannot observe modified bytes, and keep native execution on verified sealed descriptors. A cross-process digest cache keyed by path/mtime is excluded.

Measure initial snapshot cost, retained support, descriptor count, memory, warm reuse, new-campaign admission, and restart recovery. Adversarial cases must cover same-path byte mutation, executable replacement, changed settings, corrupted records, wrong digests, missing seals, service death, and concurrent publication. Original paths may change without invalidating a genuinely immutable already-admitted snapshot, but a new campaign must discover current inputs correctly. Whether filesystem-backed verity is appropriate remains a separate platform decision.

### 3. Streaming acceptance and bounded metadata lifetime

**Priority: medium-high; generality: high.** Profile the 232.42-second independent acceptance path and the coordinator's reclaim counters. Existing compact receipt reconciliation is already implemented, so this is not a recommendation to repeat that change. Test one further bounded improvement only if measured parsing, retained objects, or repeated artifact reads justify it.

Potential changes are streaming reducers, deduplicated in-memory identities within a verified immutable lifetime, and bounded read buffers. Preserve validation of every sealed receipt, source/fixture identity, required artifact, resource report, and cleanup result. Compare cold-ish first reads, warm repeats, peak coordinator memory, CPU, pressure time, and total acceptance. Compression may reduce stored bytes while increasing decoded heap; it must be evaluated inside the fixed coordinator cap.

### 4. Hot ordinary execution and cold exact artifact storage

**Priority: medium for storage; generality: domain-general.** Build a read-only inventory of references and reuse frequencies first. Keep the frequently executed set ordinary; compare independent compressed objects, chunk deduplication, and bounded deltas for less frequently accessed objects. The current cache budget provides a reason to plan retention, but no deletion or migration is part of this research.

Freeze 64 representative training objects and 64 holdout objects from later families or builds, chosen by size and format before tuning. Charge all references and decoder support; avoid training/holdout leakage through reference selection. Use small contained batches and capped reconstruction sizes. Measure encoding, lookup, full exact reconstruction, hash/seal, and fresh native execution. Record p50/p95/max restore cost and failure behavior when an entire referenced chunk or dictionary is unavailable.

Only a candidate passing exact-byte, invalidation, sealing, independent-oracle, and resource checks should proceed to a complete fresh campaign. No sample licenses removal of protected originals. A migration proposal must separately name owned objects, references, retention policy, rollback space, and expected versus actually freed bytes.

### 5. Immutable record archives and structure-aware coding

**Priority: medium-low for immediate speed; generality: high for structured logs.** Compare an in-process Zstandard reader, trained small-record dictionaries, bounded 16-record frames, and a structure-aware prototype. Use earlier campaigns for training and later campaigns as holdout. The baseline is the existing exact-byte record format and current complete acceptance behavior.

Measure sequential acceptance and random single-case lookup separately. Require bounded decompression, frame/index corruption rejection, exact byte identity, interrupted archive publication, restart recovery, and coexistence with unarchived active receipts. Cap OpenZL experiments to small chunks well within documented memory limits. The current sample supports storage feasibility only; it neither selects OpenZL nor proves archive-reader speed.

### 6. Evaluator allocation, PGO, and learned methods

**Priority: conditional.** Profile remaining evaluator allocation and repeated preparation on the largest families. Retain immutable ownership and current argument/carrier validation. Consider a generic algorithmic improvement only if CPU/allocation evidence identifies one; the prior prepared evaluator already removed a major repeated-validation cost.

PGO belongs in an explicit isolated candidate build with profile identity, original build controls, and unchanged direct compiler measurements. Learned byte compressors, GPU decoding, shared-runtime relinking, and new ELF/DWARF formats are deferred until the cheaper experiments leave a quantified gap. None currently has evidence sufficient to replace the ordinary verification path.

## Combined roadmap for the foundation goals

Performance work should increase the rate at which complete, verified capabilities
can be delivered. The [foundation delivery plan](../concepts/pipelang-foundation-delivery.md)
separates minimum-library readiness, the managed application foundation, compiler
porting/bootstrap, and constrained-target work. These milestones need different
benefits; a smaller archive alone does not make compilation, runtime execution,
or verification faster.

The portfolio should retain as many compatible, demonstrated improvements as are
useful. More mechanisms also add code, proof, memory, and maintenance costs. The
selection criterion is their combined effect on the complete workflow, including
preparation and independent acceptance. Do not sum speedup percentages from
separate experiments or make a new codec/backend a prerequisite for unrelated
foundation delivery.

| Workstream | Contribution to the goals | Dependency and first evidence |
| --- | --- | --- |
| Identity, scheduling, and acceptance | Shorter complete verification across every foundation capability | Exclusive phase/CPU attribution, current measured pairs, then immutable reuse where justified |
| Native consumers of encoded data | Smaller working sets and less loading/copying while keeping the Go/native path | Fixture load/parse/check measurements; raw, streaming, and typed-view controls |
| Compact compiler data structures | Lower allocation and memory costs for larger programs and eventual self-hosting | Allocation profiles; direct consumers of immutable indexed structures; ownership and diagnostic equivalence |
| Shared exact storage and cold archives | More retained versions, artifacts, and proof within a fixed storage budget | Reference/reuse inventory, inclusive support and scratch accounting, bounded exact restore |
| Mixed native and compact execution | Potential size/speed choices for application and constrained profiles | Stable common Core contract, explicit value/call/GC/debug boundary, then bounded backend prototype |
| Incremental computation and recoverable campaigns | Less repeated preparation and less lost work after interruption | Extend verified dependency identities only where measurements show misses; retain fresh semantic execution |

The first two workstreams can proceed as separate experimental designs because
one targets repeated identity work and the other targets data consumption. Shared
fixtures, smaller allocations, and bounded acceptance can later reinforce one
another. Conversely, compression can compete with hashing for CPU/memory bandwidth,
and a native-code cache can consume the storage saved by compact instructions.
Every successful component therefore needs a combined measurement before adoption.

### Delivery order

1. **Establish a current cost ledger.** Keep the accepted 119.43-minute terminal
   baseline and all proof counts. Add missing CPU/pressure and exclusive
   identity/fixture-consumer spans on a frozen representative selection. This
   establishes what can be improved without guessing from nested wall times.
2. **Prove the first paired candidates.** Evaluate existing safe grouping and
   direct fixture consumption independently. Implement only the strongest
   justified reversible changes, then measure their combination against the same
   raw singleton control. Preserve ordinary fallbacks.
3. **Extend ownership and storage lifetimes.** Use the measured repetition/reuse
   distribution to decide whether immutable cross-process identity, indexed
   compiler structures, or a cold store provides the next best return. Include
   initialization, invalidation, restart, support, and temporary-memory costs.
4. **Design the optional execution backend when its prerequisites are stable.**
   Keep native consumers useful regardless of whether a compact interpreter is
   adopted. Evaluate ahead-of-time selection before adding adaptive compilation;
   account for both representations and cross-mode calls.
5. **Revisit economics after coherent language capabilities.** Track proof cost
   per completed capability, not only per test or language version. New control
   flow and concurrency may change the dominant cost. Preserve existing coverage
   while designing future proof families deliberately.

This is an implementation recommendation sequence, not a new language selection,
an instruction to run simultaneous jobs, or a claim that a hybrid backend is
already approved or implemented. Bounded investigation and documentation remain
the completed work of this research task.

### Targets and return on effort

Use three separate scorecards: development feedback latency; complete proof time
and total CPU; and runtime/compiler memory plus inclusive retained storage. Record
regressions in any scorecard instead of hiding them behind a single compression
ratio. For a candidate to become the default, require a repeatable complete-path
benefit, unchanged acceptance, and acceptable measured resource costs. A storage
tradeoff can remain explicit and optional.

As an arithmetic illustration, a 20% improvement on the present terminal baseline
would save 23.89 minutes per proof, or 19.90 hours across 50 equally costly proofs.
Neither 20% nor 50 proofs is a forecast or a revised foundation slice estimate.
This also bounds engineering return: several days spent on a tiny proof-only
speedup may be less useful than a reusable improvement that benefits development,
compilation, runtime, and later self-hosting together.

The practical aim is a growing set of measured improvements with a shared
correctness model. No universal compression breakthrough is required for the
foundation plan to keep advancing.

## Acceptance and measurement protocol

Every future result should carry one of four labels: **hypothesis**, **sample**, **full affected-family proof**, or **complete framework proof**. A correct reconstruction sample does not establish framework integration, and a compression percentage does not establish execution speed or freed disk space.

Keep parser/typechecker → typed HIR → target-neutral Core → evaluator → Core-only Go, Application IR, frozen compatibility, adversarial checks, full semantic inventories, independent Value/Trace oracles, exact required native/debug bytes, fresh native children and directories, invalidation, and sealing. Compiler probes remain fresh and isolated at 128 MiB/5 seconds with normal GC/inlining and cached offline Go 1.25.13. Preparation-only `GOMEMLIMIT=600MiB` must remain separately labeled.[^23]

All compiler/test/native experiments use the canonical `job.py` enclosing `run.py`: shared 2-GiB hard/1,536-MiB high/1,800-MiB proactive boundary, zero swap, 384 tasks; 512-MiB/64-task coordinator; existing 1-GiB per-unit hard/700-MiB high/800-MiB proactive/128-task boundaries where applicable. Keep the direct matrix's distinct memory-high policy, two workload workers, current deadlines, independent finite supervisor deadline, and proven cgroup cleanup. Do not increase ceilings to make a candidate competitive.

Record elapsed wall, user/system CPU including children, peak aggregate memory, compiler RSS separately, logical and allocated retained bytes, reconstruction scratch, warm/cold state, cache hit/miss reasons, and complete support costs. Capture `cpu.stat`, `io.stat`, pressure totals, and memory events where supported; distinguish unavailable counters from zero. Do not subtract monotonic clocks across boots. Keep failed attempts and preparation costs visible without adding overlapping work into engineering elapsed time.

Use balanced run order and repeated measurements on a frozen selection. Disclose shared-host load and cache sensitivity. Evaluate cold population in fresh task-owned stores without deleting protected caches or flushing global machine caches. Demand a mechanism-consistent end-to-end gain before a full candidate campaign; run that campaign once after the final material change, then independently reconcile it. Reject regressions rather than weakening proof to meet an arbitrary speed target.

## Evidence and sources

Research and source review were completed on 2026-09-12. Current local measurements apply to the accepted v0.113.0 campaign and the explicitly bounded record sample. External algorithm results describe their authors' workloads; newer papers are identified as preprints where applicable. The report does not claim an exhaustive codec benchmark, controlled cold-cache performance, a production speedup, or a universal compression breakthrough.

[^1]: Local accepted v0.113.0 campaign: [derived receipt analysis](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/receipt-analysis.json), [reproducible analysis script](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/analyze_receipts.py), [outer job](/home/jamie/.codex/visualizations/2026/09/12/01a09387-ddcf-71d1-94b7-9395966a4500/terminal-2-job.json), [acceptance job](/home/jamie/.codex/visualizations/2026/09/12/01a09387-ddcf-71d1-94b7-9395966a4500/terminal-accept-job.json), and [independent comparison](/home/jamie/.codex/visualizations/2026/09/12/01a09387-ddcf-71d1-94b7-9395966a4500/terminal-comparison.json). The analysis reads `suite/timing.json`, `schedule-profile.json`, `artifacts.json`, `summary.json`, `suite.json`, and controller events beneath the same campaign. Prior comparison: [v0.112 timing](/home/jamie/.codex/visualizations/2026/09/11/01a09287-8762-7e02-a5ca-da7b5be9ddea/verification-data/campaigns/terminal/suite/timing.json).
[^2]: Local sample: [selection](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/record-sample.json), [probe source](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/record_codec_probe.py), [all measurements](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/record-codec-results.json), [contained job](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/record-codec-host-job.json), [unit](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/record-codec-unit.json), and [support accounting](/home/jamie/.codex/visualizations/2026/09/12/01a09431-f799-75e1-a603-a139ea3af283/support-accounting.json). Three repetitions; 128 resource records; exact-byte sample only.
[^3]: DockPipe historical journals, September 2026: [execution performance](/home/jamie/source/dockpipe/docs/agents/tasks/pipelang-reactive-application-language/conformance-execution-performance.md), especially “Corrective framework acceptance complete,” “Ordinary execution restored as default,” and “Shared framework acceptance complete”; [similarity and representation research](/home/jamie/source/dockpipe/docs/agents/tasks/pipelang-reactive-application-language/conformance-similarity-search-experiment.md).
[^4]: Current source: [toolchain and artifact identity](/home/jamie/source/dockpipe/src/lib/pipelang/generated_cache_test.go:25), [combined verification span](/home/jamie/source/dockpipe/src/lib/pipelang/generated_batch_test.go:294), [sealed executable snapshot](/home/jamie/source/dockpipe/src/lib/pipelang/generated_seal_linux_test.go:17), and [conservative scheduler](/home/jamie/source/dockpipe/tests/containedexec/scheduling.py:5).
[^5]: Linux kernel contributors. [Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html), memory controller and pressure/counter documentation; live documentation accessed 2026-09-12.
[^6]: DockPipe. [Conformance performance](/home/jamie/source/dockpipe/docs/agents/tasks/pipelang-reactive-application-language/conformance-performance.md); [resumable verification results](/home/jamie/source/dockpipe/docs/runtime/pipelang-verification.md); [performance round two](/home/jamie/source/dockpipe/docs/agents/tasks/pipelang-reactive-application-language/conformance-verification-performance-round-2.md). Historical accepted results, September 2026.
[^7]: Claude E. Shannon. [A Mathematical Theory of Communication](https://people.math.harvard.edu/~ctm/home/text/others/shannon/entropy/entropy.pdf), Bell System Technical Journal, 1948, corrected reprint; section 9, Theorem 9. The finite-string counting argument above is stated independently.
[^8]: Meta/Zstandard project. [Zstandard](https://facebook.github.io/zstd/), official algorithm and benchmark description, accessed 2026-09-12. Published corpus benchmarks are not local measurements.
[^9]: Yann Collet and LZ4 contributors. [LZ4](https://lz4.org/), official project and format documentation, accessed 2026-09-12.
[^10]: Zstandard contributors. [Dictionary API guidance, `zdict.h`, v1.5.7](https://github.com/facebook/zstd/blob/v1.5.7/lib/zdict.h), pinned source version; dictionary use, training, and sample-size guidance.
[^11]: Yann Collet and Murray Kucherawy. [RFC 8878: Zstandard Compression and the application/zstd Media Type](https://www.rfc-editor.org/rfc/rfc8878.html), February 2021; format, bounded streaming, and random-access scope.
[^12]: Jyrki Alakuijala et al. [RFC 9841: Shared Brotli Compressed Data Format](https://www.rfc-editor.org/rfc/rfc9841.html), September 2025; shared context and dictionary dependencies.
[^13]: Wen Xia et al. [FastCDC: A Fast and Efficient Content-Defined Chunking Approach for Data Deduplication](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia), USENIX ATC 2016; [paper](https://www.usenix.org/system/files/conference/atc16/atc16-paper-xia.pdf).
[^14]: David Korn, Joshua MacDonald, Jeffrey Mogul, and Kiem-Phong Vo. [RFC 3284: The VCDIFF Generic Differencing and Compression Data Format](https://www.rfc-editor.org/info/rfc3284/), June 2002.
[^15]: Jisung Park et al. [DeepSketch: A New Machine Learning-Based Reference Search Technique for Post-Deduplication Delta Compression](https://www.usenix.org/conference/fast22/presentation/park), USENIX FAST 2022; [paper, section 5.6](https://www.usenix.org/system/files/fast22-park.pdf). Both data reduction and throughput tradeoffs considered.
[^16]: Meta/OpenZL contributors. [Introduction](https://openzl.org/getting-started/introduction/), official structure-aware compression design documentation, accessed 2026-09-12.
[^17]: Meta/OpenZL contributors. [Core Library Limitations](https://openzl.org/getting-started/library-limitations/), accessed 2026-09-12; current payload, memory, and streaming limitations.
[^18]: Grégoire Delétang et al. [Language Modeling Is Compression](https://arxiv.org/html/2309.10668v2), 2023, revised version; raw versus model-adjusted size and online-coding costs.
[^19]: Yu Mao, Holger Pirk, and Chun Jason Xue. [Lossless Compression of Large Language Model-Generated Text via Next-Token Prediction](https://arxiv.org/html/2505.06297v1), May 2025 preprint; domain-specific next-token compression.
[^20]: Hongshi Tan et al. [Approaching Shannon Bound with Lossless LLM Weight Compression](https://arxiv.org/html/2606.15789v1), June 2026 preprint; tiled ANS/GEMM decoding and A100/H200 evaluation. No local reproduction.
[^21]: Linux kernel contributors. [fs-verity: read-only file-based authenticity protection](https://cdn.kernel.org/doc/html/latest/filesystems/fsverity.html), accessed 2026-09-12; page-cache verification, measurement ioctl, and filesystem requirements.
[^22]: Go team. [Profile-guided optimization](https://go.dev/doc/pgo), official guidance, accessed 2026-09-12; representative profiles and explicit build inputs. No toolchain upgrade or PGO candidate was run.
[^23]: DockPipe. [Contained execution contract](/home/jamie/source/dockpipe/tests/containedexec/README.md), [verification architecture](/home/jamie/source/dockpipe/docs/runtime/pipelang-verification.md), and [foundation delivery plan](/home/jamie/source/dockpipe/docs/concepts/pipelang-foundation-delivery.md). Current local acceptance and scope requirements.
[^24]: Daniel J. Abadi, Samuel R. Madden, and Miguel C. Ferreira. [Integrating Compression and Execution in Column-Oriented Database Systems](https://www.cs.umd.edu/~abadi/papers/abadisigmod06.pdf), ACM SIGMOD 2006; operations and compression-aware execution.
[^25]: DuckDB contributors. [Execution Format](https://www.duckdb.org/docs/current/internals/vector), official documentation accessed 2026-09-12; constant, dictionary, sequence, and unified vector formats.
[^26]: Linux kernel contributors. [Squashfs 4.0 Filesystem](https://docs.kernel.org/filesystems/squashfs.html), accessed 2026-09-12; section 4.2 explains decompression into the normal page cache.
[^27]: Current local source: [v0.109 fixture loader and checks](/home/jamie/source/dockpipe/src/lib/pipelang/terminal_combined_selector_arms_test.go:402), [fixture cloning](/home/jamie/source/dockpipe/src/lib/pipelang/generated_batch_test.go:162), [fresh child execution](/home/jamie/source/dockpipe/src/lib/pipelang/generated_batch_test.go:397), and [owned prepared Core representation](/home/jamie/source/dockpipe/src/lib/pipelang/coreeval/prepared.go:17).
[^28]: V8 team. [Launching Ignition and TurboFan](https://v8.dev/blog/launching-ignition-and-turbofan), May 2017; combined bytecode interpreter and native compiler architecture.
[^29]: V8 team. [Sparkplug — a non-optimizing JavaScript compiler](https://v8.dev/blog/sparkplug), May 2021; interpreter-compatible frames, tier transitions, and dispatch tradeoffs.


## Approved identity/fixture implementation follow-up (2026-09-12)

The bounded [identity/fixture objective](../agents/tasks/pipelang-reactive-application-language/identity-fixture-performance.md)
completed attribution and candidate screening. The retained implementation is a
single 32-KiB read buffer per generated-test toolchain digest, opt-in exclusive
identity wall measurements, and boundary cgroup CPU/pressure counters. Every
current file and metadata field is still hashed; there is no persisted digest,
changed native representation, relaxed sealing, or production-language change.
Fresh full terminal verification and independent acceptance passed; see the terminal result below.

All evidence below lives under
`/home/jamie/.codex/visualizations/2026/09/12/01a0944d-24d3-78a2-8064-d1537381f4e7`.
`frozen-design.json` and `selection.json` were recorded before tuning: 48 cases
cover the four dominant families, older/heavy layouts, memory cases, and special
artifact harnesses. Corrected controls and candidates all retain exactly the
accepted baseline's 294 ordered source/fixture audits and fresh audited native
children. `compare_audits.py` and each `*-audit-comparison.json` retain this proof.

### Exclusive attribution and telemetry cost

Two instrumented warm controls average these logged leaf costs. They are observed
wall seconds summed across processes, not CPU time or additional elapsed time:

| Operation | Summed sample seconds |
| --- | ---: |
| Toolchain content read/hash | 19.508 |
| Toolchain enumeration/metadata | 2.623 |
| Cached-record/binary verification | approximately 1.83 |
| Binary copy/hash/seal | approximately 0.50 |
| Lock acquisition | approximately 0.053 |
| Generated source keys | approximately 0.022 |
| v109 native fixture load | 0.0208 |
| v109 native fixture parse | 1.207 |
| v109 native independent checks | 0.299 |

Toolchain enumeration is the enclosing walk less timed content reads/hashing;
source-key timing begins after toolchain identity. Native fixture measurements
are from the instrumented v109 consumers, not a claim about all native work.
Enclosing artifact/workload spans must not be added to these leaf totals.
Cgroup CPU includes the coordinator and its descendants, but excludes the outside
job supervisor. Exclusive CPU per operation was not collected. `io.stat` was
unavailable; CPU and pressure counters were available and stored as matching
before/after observations. Hierarchical counters must not be summed with children.

Warm instrumentation controls took 70.23 and 69.11 seconds enabled versus 71.80
disabled. This small sample did not resolve a telemetry penalty beyond timing
variation; it does not establish negative overhead. A 1,000-capture synthetic
boundary probe averaged 146 microseconds per counter capture, excluding report
serialization. The 106.71-second first control populated eight changed checking
bundles and is excluded from warm overhead comparisons.

### Identity candidate and compatible scheduling

The cached Go implementation routes `io.Copy` through `File.WriteTo` and its
fallback buffer allocation. Reusing a 32-KiB buffer requires hiding `WriteTo`
with a reader-only wrapper so `io.CopyBuffer` actually uses that buffer. The
retained change preserves hashing order, metadata, current bytes and exact keys.
The old allocation path remains available as `--no-toolchain-read-buffer`.

| Matched complete path | Control seconds | Buffered seconds |
| --- | ---: | ---: |
| Interleaved singleton sample, instrumentation variant | 67.52 | 65.91 |
| Interleaved measured pairs, instrumentation variant | 65.48 | 59.69 |
| Final raw-fixture singleton sample, profiling disabled | 64.888 | 62.515 |

The final comparison observed **3.66% less wall time** and cgroup CPU of
128.042 -> 125.769 seconds (1.78% lower), with 48 cases, 294 exact audits/children,
and zero native misses. Its earlier 80.67-second control included a main-test
binary rebuild and is excluded from this warm comparison. Earlier buffered
outliers (98.89 seconds singleton, 72.77 paired) remain in the evidence; their
variability prompted the interleaved controls rather than selective removal.

The existing paired planner was compared in four balanced singleton/pair orders,
with fresh input/host/settings/worker matched profiles. Pairing and buffer reuse
were tested together; their percentages are not additive. The sparse frozen
sample's current profile yields zero adjacent eligible pairs in the complete
inventory, so the terminal retains singleton scheduling. No stale full profile
is admitted and no grouping default or limit changes. The complete terminal will
produce a full current profile for the existing opt-in future pairing workflow.

### Native streaming fixture candidate: rejected

The prototype applied a common raw/streaming row consumer to v107/v108/v109 JSON
fixtures. It retained every original JSON byte and source/fixture audit mapping,
used the same independent Value/Trace callbacks, and checked row count, array
termination and trailing data. Both modes passed focused valid-input checks and
rejected malformed, wrong-count, trailing, wrong-value, wrong-trace and wrong-flag
fixtures. Duplicate-key/whitespace behavior was checked without canonicalizing
or re-emitting any fixture. No typed-view or encoded format was selected.

| Complete sample path | Raw seconds | Streamed seconds |
| --- | ---: | ---: |
| Warm singletons, ordinary identity | 61.42 | 61.40 / 60.71 |
| Warm singletons, buffered identity | 58.94 | 62.74 |
| Buffered identity plus measured pairs | 55.21 | 55.34 |

Raw parse work was 3.202 summed seconds versus 3.264/3.231 streamed. For the
136 measured children in these fixture families, native execution was
4.843 seconds raw versus 4.845/4.810 streamed; observed native RSS did not show a
material working-set benefit. The changed checking bundles cost 114.52 seconds
to populate with 24 misses, approximately 53 seconds above the warm raw path.
Streaming provides no consistent complete-path benefit to justify that cost and
support. Its source is removed from the checkout; `variants/fixture`, build
objects, focused checks and all eight comparison receipts remain preserved.
Original raw loaders are restored exactly.

### Retained evidence and terminal boundary

`experiment-accounting.json` accounts for 32 experiment-referenced retained native
objects absent from the accepted inventory: 165,363,164 logical bytes and
165,556,224 allocated bytes. Source-variant and main-build support adds 104,049,182
logical bytes; common Go caches and receipts are additional. Original bytes
actually freed: **zero**. Non-overlapping pre-terminal contained jobs total
2,056.39 seconds in that accounting snapshot; do not add their nested suite or
phase totals again. Subsequent terminal jobs are reported separately.

Failed setup attempts are retained: one discovery quoting error, an instrumentation
run that unintentionally selected standalone fallback, and a private-directory
permission error in the fixture probe. They are excluded from matched performance
comparisons; cleanup passed. Corrected raw/streaming adversarial checks and all
13 retained harness/planner regressions pass. Thirty-nine final source postimages
are frozen in `terminal-source-postimages.json`.

Terminal receipts are `verification-data/campaigns/terminal`, `terminal-job.json`,
and independent `terminal-accept-job.json` / `terminal-comparison.json`. Terminal
acceptance subsequently passed as recorded below. Compiler RSS/time ceilings,
normal GC/inlining, offline Go 1.25.13, two workers, all containment limits,
current independent oracles and generic engine/package boundaries are unchanged.

### Fresh terminal result: accepted

`final-performance-result.json` and `terminal-comparison.json` record independent
acceptance of 871 functions, 8,859 logical cases, 2,922 isolated compiler cases,
nine integration checks and the editor suite. All **49,526 ordered audits and
fresh audited native children** match the accepted v0.113.0 baseline exactly,
including the 49,465 inherited audits. All 39 frozen source postimages match;
both aggregate jobs exited zero and removed their complete cgroup trees.

| Complete proof timing | Prior accepted baseline | Retained implementation |
| --- | ---: | ---: |
| Fresh terminal job | 6,933.342 s | 6,543.042 s |
| Independent acceptance | 232.424 s | 229.155 s |
| Combined | **119.429 min** | **112.870 min** |

The new complete run was **6.559 minutes (5.49%) shorter** than the historical
warm baseline. This is an observed whole-path comparison, not an isolated causal
estimate of the buffer change; the final matched sample supports 3.66% wall
and 1.78% cgroup-CPU improvement. Current complete cgroup CPU was 12,598.712 seconds
(including descendants, excluding outside supervisors); the baseline lacks an
equivalent CPU total. No CPU saving is inferred for the full historical comparison.

All 16,375 native artifact lookups hit verified objects. Isolated compiler maxima
were 88.387 MiB RSS and 0.7485 seconds, comfortably within the unchanged 128 MiB
and 5-second acceptance ceilings. Shared peak was 1,538.203 MiB under the fixed
2-GiB maximum, with zero swap/OOM events. Kernel reclaim/max events occurred
inside the fixed limits; CPU/pressure counters and unavailable `io.stat` are
preserved in the receipts. No limit increase or machine change was needed.

The completed singleton campaign produces a current full profile with 3,172
potential pairs and 2,515 singletons under existing rules. This is eligibility,
not a measured full paired speedup; a future invocation still verifies all profile
and executable identities. The bounded objective is complete. No further roadmap
work, automatic successor, commit, push or protected cleanup is selected.
