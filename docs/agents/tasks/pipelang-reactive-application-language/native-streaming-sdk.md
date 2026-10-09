# Native streaming SDK integration

Objective: `pipelang-native-streaming-sdk-20260915`
State: completed (bounded synchronous native-stream profile)
Execution skill: dorkpipe-objective-execution

Authority: the user requested streaming in Nucleon and a Nucleon-owned native
PipeLang SDK, then explicitly selected extending PipeLang for direct native calls.
This authorizes the bounded synchronous native-stream compiler profile and proof.
It does not authorize publication, licensing changes or full-language promotion.

Done when: Nucleon streaming and malformed/I/O/limit checks pass; real generated
PipeLang calls roundtrip independently checked data through the installed native
SDK; Release/sanitizer and focused compiler/compatibility checks pass; limits and
remaining foundation work are documented.

Contract: [native stream profile](../../../concepts/pipelang-native-streams.md).
Invariants: generic engine; private codec/SDK ownership stays in Nucleon; compiler
analysis is inert; source handles are borrowed and typed; pure language contracts
and ongoing general-block state are preserved; no resource ceilings are raised.
Checkpoint policy: automatic within this objective. Handoff: user requested only.

Owned areas: new native_stream compiler admission, streamir and streamcpp,
explicit compiler shell, a profile-gated qualified-call parser branch, focused tests
and these docs. The pre-existing Nucleon SDK pilot and dirty compiler work are
retained; they are not newly accepted by this objective.

## Final proof

- 13 focused compiler/parser/native-runtime tests passed in the verified contained
  runner; source/IR refusals include SDK namespace shadowing. Final execution took
  0.228 seconds with 46,272,512-byte aggregate peak, zero swap and verified cleanup.
- Cold test-package compilation first reached the proactive memory stop. Using
  prescribed preparation-only `GOMEMLIMIT=600MiB` succeeded under the same limits.
  Test execution retained normal runtime GC. Failed and successful receipts remain.
- Nucleon Release and ASan/UBSan CTest passed 5/5 each. Real generated PipeLang
  calls roundtripped 65 MiB + 7 bytes in Release. Sanitized native integration used
  three chunks; separate C-stream sanitizer proof also decoded beyond 64 MiB.
- Separate installed consumers passed in Release and with sanitizer-instrumented
  adapters. The final packaged compiler produced byte-identical generated code
  and binding metadata to the native code exercised by the SDK tests.
- Codec/model/import identities are unchanged. No private implementation or binary
  entered Dockpipe, and no commit/push/publication was performed.

Durable detailed proof (receipts, failure logs, source/install hashes, test output):
`/home/jamie/.codex/visualizations/2026/09/15/01a0a309-d1b8-7021-bd8e-96fc2c309059/nucleon-streaming-20260915/`.
Nucleon owns the report at
`research/experiments/2026-09-15-streaming-sdk/README.md` in its private checkout.

Full conformance, general effect/result composition, nonblocking/async streaming,
broader fuzzing and stable external distribution remain separate work. The earlier
artifact-storage pilot is unchanged; these results make no new performance claim.
