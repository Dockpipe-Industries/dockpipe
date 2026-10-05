# Incremental native streaming

Authority: user requested committing the verified Nucleon buffer API and updating
PipeLang to use it fully. Nucleon base commit: `815c3b49a771e1477b5eec8cf959b74c2e011114`.
State: complete for the bounded incremental native-stream v3 profile.

Scope: explicit native-stream v3 with borrowed host-owned sessions and buffers,
per-call consumption/production, cumulative counters, final-input signaling and
typed backpressure/completion. Preserve v1/v2 and the current pure-language work.
Nucleon owns the adapter, session lifetime/options/cancellation, operation/manifest
identity, CMake packaging and runnable example. DockPipe owns only generic types,
source/IR admission, independent evaluation, C++ generation and the compiler CLI.

The application resubmits unconsumed input and schedules further calls; no hidden
whole-stream loop sits in the adapter. The source cannot construct or retain raw
buffers/session handles. Codec CPU work remains synchronous. General async language
semantics, internal artifact format/default migration and throughput claims are not
part of this capability. No DockPipe commit or remote publication was requested.

Canonical contract: [native streams](../../../concepts/pipelang-native-streams.md).

## Validation

Focused generic v1/v2/v3 checks passed, including ten independent expected incremental
outcomes against the reference evaluator and generated native C++, lazy effects,
status/counter predicates, source/forged-IR refusals, v2 class-name compatibility,
CLI entry headers, accepted v114 general blocks and two backend import boundaries.

The initial contained cold `go test` compilation reached the existing proactive
memory threshold. A documented 600 MiB Go build-preparation setting and 700 MiB
reclaim threshold produced the test binary under unchanged hard limits; the tests
then ran separately without that Go memory override. The failed receipt is retained.
The first SDK example build caught an invalid inheritance from a final adapter;
the example now delegates to the adapter, and its failed build log is retained.

Final validation passed:

- 13 focused PipeLang test functions (native v1/v2/v3, general blocks and two
  import-boundary checks), plus seven compiler CLI cases.
- Nucleon Release and ASan/UBSan CTest: 7/7 each, including existing callback and
  buffer behavior and the generated incremental application.
- Isolated installed v3 applications: both configurations passed 303 resumable
  calls, independent byte reconstruction, backpressure, EOF, counters, limits,
  CRC/truncation/trailing refusals, identity checks and cancellation.
- Installed v2 applications: both configurations passed 4 MiB + 37 byte roundtrip,
  typed limit short-circuit and corrupt archive rejection.
- Nucleon import verification and final whitespace checks passed. No resource
  ceiling was increased. No full pure-language acceptance or throughput claim.

Generic receipts and source/compiler identities are retained at
`/home/jamie/.codex/visualizations/2026/09/16/01a0ac95-5a43-7013-a865-a6c1941137a4/pipelang-incremental-20260918/`.
SDK logs/installations are in Nucleon's
`build/20260918-pipelang-incremental-{release,asan}/` directories. Initial failure
receipts remain. No new generated artifact is added to DockPipe source control.

The base SDK commit above is complete. Integration follow-up changes in both repos
remain uncommitted; nothing was pushed. The unrelated mutable-locals verification
state and user-owned changes are preserved.
