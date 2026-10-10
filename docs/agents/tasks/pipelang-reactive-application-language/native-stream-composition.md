# Native stream result composition

Objective: `pipelang-native-stream-composition-20260915`
State: complete (bounded native-stream v2 composition)
Execution skill: dorkpipe-objective-execution
Authority: user requested committing Nucleon, then continuing the unfinished
PipeLang integration. Nucleon checkpoint: `2a3a6821296c25038d04144df8c65cf665a9d888`.

Outcome: a versioned native-stream composition profile with immutable typed
results, status/counter inspection, conditional control flow and acyclic helper
methods; prove real multi-stage Nucleon streaming and failure short-circuiting.
Done when source/IR rejection, independent evaluator versus native Value/Trace,
installed-SDK integration and focused compatibility checks pass under existing limits.

Invariants: preserve v1 and ordinary pure-language contracts, existing dirty
general-block work, caller-owned stream lifetime, explicit host authority,
left-to-right effects and lazy branches. Keep the codec private and unchanged.
Exclusions: full-language promotion, async scheduling/nonblocking I/O, arbitrary
FFI/pointers or file-opening syntax, publishing, and Dockpipe commits.
Checkpoint policy: automatic within scope. Handoff: user requested only.

The prior synchronous v1 SDK proof remains complete. This continuation does not
relabel that proof as full language/effect acceptance.

## Outcome and evidence

The generic source compiler, independently validated IR, C++17 generator and
reference evaluator implement immutable values, result/status/counter inspection,
branches, lazy expressions and acyclic helpers. V1 remains the explicit default.
The CLI emits a public-entry header; v2 symbols include source and SDK pin identity.
Nucleon owns the installed CMake helper and real multi-stage `.pipe` example.

Validation: 17 expected Value/EffectTrace cases, 16 source negatives, 8 forged-IR
negatives, v1 tests, six inherited general-block tests, two backend import boundary
checks and CLI entry tests. Nucleon SDK CTest passed 5/5; installed composed example
passed Release and ASan/UBSan, with exact 4 MiB + 37 byte reconstruction, limit
short-circuit and corrupt archive refusal. Its shared library is byte-identical to
the prior streaming SDK. The five internal artifact-store lifecycle tests passed.
One compatibility invocation used the repository root instead of the package
working directory; the two affected import checks passed on corrected rerun.
The failed receipt remains preserved. No resource limits were raised.

Durable proof: `<local-evidence>/2026/09/15/01a0a309-d1b8-7021-bd8e-96fc2c309059/nucleon-composition-20260915/`.

## Both SDK consumers

The owner clarified that the ecosystem should benefit internally and developers
should also call the SDK in apps. Both consume the same private binary boundary.
The existing internal executable-artifact pilot was reverified; its opt-in behavior
is preserved. Production-wide internal cache/package/transport adoption is planned
separately in Nucleon task 009. It is not implied by this bounded profile completion.
Async I/O and full-language acceptance remain open. Dockpipe changes stay uncommitted;
no remote publication is part of this continuation.

Nucleon integration commit: `85041db4c2c8ce17c502cf53a8fef307b5e59060`. Private checkout clean, two local commits ahead; no push.
