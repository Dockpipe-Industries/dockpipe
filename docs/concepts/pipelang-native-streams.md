# Experimental native streaming SDK calls

Status: explicitly requested native-stream integration, separate from the accepted
pure language contract and the ongoing general-block work. Profiles are
`pipelang.native-stream.v1` (direct calls) and explicitly selected
`pipelang.native-stream.v2` (result composition), plus explicitly selected
`pipelang.native-stream.v3` (incremental sessions and buffers). These do not promote
the complete C++ backend.

## Source API

An SDK supplies a small binding manifest. With that manifest explicitly selected,
a `.pipe` method can directly call its native operation:

```text
public Class Transfer {
    public StreamResult Encode(ReadStream input, WriteStream output, int maxBytes)
        => Codec.compressStream(input, output, maxBytes);
}
```

`Codec.compressStream` is a manifest alias, not a compiler builtin. Nucleon's own
SDK declares `Nucleon.compressStream` and `Nucleon.decompressStream`. Its manifest,
`.pipe` facade, native adapter and compiled codec live in the private Nucleon repo.
DockPipe contains only the generic compiler/type/runtime boundary.

The first profile admits public, stateless classes whose methods return one direct
native call. Each method takes exactly one `ReadStream`, one `WriteStream`, and one
`int` limit, in any declaration order; call arguments must have read/write/limit
order. There are no conversions, nested calls, local variables, branches, loops,
general FFI pointers, stream construction, or result matching in this profile yet.
An embedding native host receives the typed result. These restrictions are explicit
compiler diagnostics, not silent fallbacks to another backend.

## Ownership and effects

Stream handles are borrowed capabilities supplied by the host for one synchronous
call. Source cannot create a stream from an integer/path, store it, close it, retain
it asynchronously or change its access direction. The host owns opening, closing,
thread scheduling, callback lifetimes and any filesystem/network authority.

The selected native adapter runs in-process and is trusted code. It must enforce
the declared operation identity and bounds and must not retain borrowed handles.
The binding manifest alone grants no runtime authority. There is no dynamic library
loading, package fetching or I/O during parsing, binding, validation or generation.

`StreamResult` preserves a status plus unsigned 64-bit input/output/chunk progress.
Statuses are success, invalid argument, invalid stream, unsupported, limit exceeded,
I/O error, host failure and denied. The host receives partial progress on failure;
already-written output is not rolled back. Negative limits and missing callbacks
refuse before the host call. Unexpected native exceptions become host failure;
their external side effects/progress may be unknown. Both native-stream profiles
preserve these distinctions.

## Compiler and SDK build

`CompileNativeStreams` accepts source bytes and explicitly pinned manifest bytes.
Each manifest has profile, package, ABI and qualified-operation identities. The
source checker produces typed stream IR; `streamcpp.Generate` independently
validates that IR before emitting C++17. Pure Core/evaluator/Go behavior stays pure.
The stream profile is selected through its own compiler entrypoint, not inferred
from a `.pipe` filename or an unknown function name.

```sh
go build -o /absolute/tools/pipelang-streamc ./src/cmd/pipelang-streamc
/absolute/tools/pipelang-streamc \
  --source Transfer.pipe --manifest sdk/manifest.json \
  --manifest-sha256 <pinned-manifest-sha256> \
  --out Transfer.generated.hpp --bindings-out Transfer.bindings.json
```

Use returned function identity/name mappings when linking the generated native
entrypoints. The source and manifest hashes are retained in generated artifacts;
binary/toolchain identity is a separate build/distribution responsibility. A lock
digest is an integrity pin, not a signature or permission grant. The compiler does
not verify a codec implementation or infer its provenance from this metadata.

The shell command reads named files and writes generated artifacts; the compiler
library itself is inert. `--runtime-out` exports the generic C++ boundary header
for an SDK package. Nucleon's CMake package consumes this compiler and packages its
own native SDK. There is no Python subprocess in the application's codec calls.

Resource admission: 64 KiB source, 8,192 tokens, delimiter depth 64, 16 supplied
manifests of at most 64 KiB each, and 64 total bindings/functions. This initial
profile is a bounded native integration capability, not completion of foundation
F03/F18/F21 or P15/P16/P17/P20.

## Validation scope

Source and forged-IR negatives cover direction/type/borrow errors, undeclared calls,
stale manifests, identity ambiguity, unsupported syntax and backend injection.
A generated native mock proves argument ordering, exactly one host invocation,
typed partial progress and exception handling independently of any codec.
Nucleon's own native test compiles its `.pipe` facade and validates streaming
against independent generated bytes, including a Release input larger than 64 MiB.
The objective record tracks final receipts and limits; no full-language performance
or acceptance claim follows from this integration.

## Version 2: compose results in PipeLang

Select `--profile pipelang.native-stream.v2` (or
`CompileNativeStreamsWithProfile`) explicitly. Version 1 remains the default and
keeps its direct-call restrictions. The SDK manifest and native ABI remain v1;
the language profile version and SDK ABI version are separate contracts.

```text
public Class Transfer {
    private bool Succeeded(StreamResult result) => result.status == StreamStatus.ok;
    public StreamResult Run(ReadStream input, WriteStream packed,
                            ReadStream stored, WriteStream output, int limit) {
        StreamResult compressed = Codec.compressStream(input, packed, limit);
        if (!Succeeded(compressed)) { return compressed; }
        return Codec.decompressStream(stored, output, limit);
    }
}
```

Both `Codec` operations must be declared by the selected SDK manifest. The host
supplies all four borrowed capabilities and owns the transition from archive
writing to archive reading. PipeLang sequences the operations and chooses whether
to continue. There is no implicit seek, file opening or rollback.

V2 admits immutable typed locals, nested lexical blocks, `if`/`else`, early returns,
lazy `&&`/`||`/conditional expressions, and acyclic public/private helper methods.
Arguments and comparison operands evaluate left to right, exactly once; untaken
branches have no native effects. Private helpers are callable within their class.
Supported value types are `bool`, signed 64-bit `int`, `uint64`, `StreamResult` and
`StreamStatus`. Borrowed streams may be passed as parameters but cannot become
locals, return values, fields or constructed values. There are no loops, mutable
locals, arbitrary arithmetic, general pointers or asynchronous operations.

Results expose `.ok`, `.status`, `.inputBytes`, `.outputBytes` and `.chunks`.
Counter fields retain all 64 unsigned bits; comparisons with nonnegative integer
literals are supported without narrowing. Status constants are `StreamStatus.ok`,
`.invalidArgument`, `.invalidStream`, `.unsupported`, `.limitExceeded`, `.ioError`,
`.hostFailure` and `.denied`. Unknown host status tags normalize to host failure.
Source cannot fabricate a result constructor. Every method must return on all paths.

Additional admission limits: 16 parameters per method, 256 total local/parameter
slots per method, 256 statements per block, expression/block depth 64 and 8,192
IR nodes. Helper nesting is bounded to 16 edges and expanded call cost to 8,192
nodes, including both sides of branches. The independent IR validator enforces
these limits before either reference evaluation or C++ generation.

The compiler can produce a convenient typed C++ entry header:

```sh
# Add to the earlier compilation command:
--profile pipelang.native-stream.v2 --entry Transfer.Run \
--entry-header entry.hpp --entry-namespace pipelang_entry_Transfer
```

Place `entry.hpp` beside the generated source header. Include it and invoke
`pipelang_entry_Transfer::invoke(host, ...)`; the compiler resolves the public
binding instead of requiring a consumer to guess its generated symbol.
Nucleon's installed CMake helper wraps this process as `nucleon_add_pipelang`.
The private SDK owns that helper, adapters, packaged examples and implementation.

Validation includes 17 hand-authored expected result/effect traces compared against
both the reference evaluator and compiled native execution, source and forged-IR
refusals, v1 compatibility, and a real installed-SDK application that compresses,
branches and decompresses entirely through its `.pipe` method. This is synchronous
native effect composition, not full language/backend or asynchronous acceptance.

## Version 3: application-controlled incremental buffers

Select `--profile pipelang.native-stream.v3` with a separately pinned v3 SDK
manifest. The default remains v1. V3 retains v2's bounded result composition and
adds borrowed `StreamSession`, `InputBuffer`, and `OutputBuffer` parameters and
an immutable `StreamStep` value. Each declared native operation has the signature
`(StreamSession, InputBuffer, OutputBuffer, bool) -> StreamStep`; the Boolean marks
the final input span. Manifest/profile mismatches are rejected before generation.

```text
public Class Transfer {
    public StreamStep Decode(StreamSession session, InputBuffer input,
                             OutputBuffer output, bool endOfInput) {
        StreamStep step = Codec.decodeStep(session, input, output, endOfInput);
        if (!step.ok) { return step; }
        return step;
    }
    public bool Complete(StreamStep step) => step.ok && step.done;
}
```

`Codec.decodeStep` is package metadata, not a builtin. Nucleon's private SDK declares
`Nucleon.compressStep` and `Nucleon.decompressStep` in its incremental manifest.
The generic compiler/runtime/evaluator contain no compression implementation.

`StreamStep` exposes:

| Member | Meaning |
| --- | --- |
| `.ok`, `.status` | Successful processing versus a typed error; needing buffers is success. |
| `.needInput`, `.needOutput`, `.done` | Success-only progress predicates; all false on error. |
| `.inputConsumed`, `.outputWritten` | Unsigned 64-bit byte counts for this call. |
| `.inputBytes`, `.outputBytes`, `.chunks` | Unsigned 64-bit cumulative session counters. |

The application creates a session through its SDK adapter, supplies fresh spans,
advances by the returned counts and invokes the generated entry again when input
or output space is available. Zero-length input without final input is temporary
starvation; zero output capacity is backpressure. For Nucleon, repeat the final
flag with the unconsumed suffix of a final span; once consumed, provide empty input
while draining output. Explicit final input, not a socket close, establishes the
message boundary. A completed result is required before accepting an entire stream.

Sessions persist in the embedding host between calls; source only borrows them.
Source cannot construct, copy into locals, return, store or close sessions/buffers,
nor access raw pointers. The host owns chunk/aggregate limits, cancellation,
transport framing, readiness scheduling and single-session serialization. Independent
sessions can run concurrently. Reusing a session with a different operation is an
adapter error; Nucleon denies mode, package, manifest and foreign-session mismatches.
Its RAII session releases the codec state on destruction and exposes explicit cancel.

The C++ boundary rejects null nonempty spans, overlapping spans and overflowing
address extents before invoking a host. Unknown statuses/progress tags, overreported
counts or counts exceeding cumulative totals become host failure. Exceptions become
host failure with unknown external effects; successful counts and typed codec errors
remain observable. The reference evaluator models borrowed capability identities and
span lengths; physical address overlap is checked at the native boundary.

CPU work within a step is synchronous. This is resumable buffer processing, not a
language event loop, general async support, zero-copy promise or performance claim.
A large offered span can cause multiple chunks to be processed before returning.
The application must yield on readiness instead of busy-looping on an unavailable
buffer. Nucleon validates each complete decoded chunk before releasing it; a later
error can leave an already-emitted valid prefix.

The installed Nucleon CMake helper selects its incremental manifest for
`PROFILE pipelang.native-stream.v3`, emits typed entry headers, and links its adapter.
Generated composition headers include a source/profile/SDK-specific guard so several
entries from the same application can coexist in one translation unit. V1/v2 callback
bindings retain their contracts. Internal artifact-store format/default adoption is
separate from these explicit application APIs.

## Internal ecosystem use and application use

Nucleon has two intended consumers of the same private SDK:

1. PipeLang/DockPipe internals: compressed executable artifacts, and future selected
   cache, package and transport paths. Developers should benefit without calling
   compression themselves when those integrations are enabled.
2. Developer applications: explicit native `.pipe` SDK calls with typed results and
   host-owned streams, usable independently of the internal artifact strategy.

The current internal integration is the optional compiled-executable artifact pilot
in `tests/containedexec`; it already stores and reconstructs real generated programs
through the binary SDK. Its lifecycle checks also pass with the streaming SDK build.
It is not yet a default production cache, package or transport backend. See
[the measured pilot](../research/pipelang-nucleon-pilot.md) for its actual scope.

Promotion should use a generic codec/provider boundary in the owning runtime or
package layer, with format/version identity, integrity checks, resource limits,
SDK availability and an ordinary representation for unsupported cases. Measure
whole-workflow costs before choosing defaults. Codec implementation/model sources
remain in Nucleon; enabling an internal integration must not copy them into DockPipe.
