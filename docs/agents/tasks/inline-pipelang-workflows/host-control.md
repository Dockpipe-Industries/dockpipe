# TASK-055 Host control profile

Status: **first host-control slice qualified on Linux amd64; native platform qualification continues**. The user
authorized real embedded PipeLang orchestration and a small existing workflow
conversion. The older [pure-expression proposal](overview.md#status-scope-and-ownership)
remains deferred separately; its WASM sandbox, pure exports, typed outputs and DAG
work are outside this implementation.

## Contract and ownership

Use the canonical [workflow control reference](../../../workflows/workflow-yaml.md#inline-pipelang-host-control)
and [explicit build command](../../../cli-reference.md#workflow-control-build).
Dockpipe owns YAML, generated typed input/step bindings, build admission, ordinary
step dispatch, callback budgets, cancellation and independent finally cleanup.
The pinned installed PipeLang SDK owns actual source semantics and native
compilation. This is trusted host workflow code, with explicit immutable inputs;
it is not the proposed pure sandbox. No sibling compiler source or alternate
expression evaluator belongs in Dockpipe.

## Qualified first host-control slice

The first slice is implemented and locally qualified on Linux amd64. The
machine-readable proof is retained in the active task's private artifacts; no
private machine paths or runtime input values are public defaults.

| Area | Verified result |
| --- | --- |
| Domain/schema/editor | Strict additive YAML declarations, typed/bounded fields, schema mirrors and basic completion pass domain, validator and editor tests. |
| Build/run split | Real installed-SDK compilation passes. Invalid typed calls report the YAML file and line. Verified prebuilt execution succeeds without compiler/build tools in PATH; stale/tampered artifacts are refused. |
| Dispatch/protocol | Real branches, typed loops, identifier-hygiene bindings, fail-fast behavior, readiness-before-interruption and independent core finally pass. Normal and full ASan/UBSan/leak-enabled native runs pass. |
| Artifact lifecycle | Captured native authorization binds the controller artifact; bounded nonblocking metadata admission rejects FIFOs; original cache roots survive workspace projection; callback error identity survives intervening cleanup calls. |
| Useful conversion | The SDK qualification workflow passes its optional PipeLang normal/sanitizer loop with four controlled prepare/prove invocations. Existing single-profile mode, scripts, scopes and outputs remain supported. |
| Package distribution | Explicit package compilation stages the selected verified prebuilt controller, preserving executable mode. A relocated package runs in a fresh project without the original cache or compiler. No native compilation occurs during package staging or run. |
| Platforms | Linux amd64 has normal/full-sanitizer proof. Windows VM original PowerShell 5 control journeys and compiler-free relocation pass; physical Windows 11, macOS and other targets remain separate. |

The affected domain, infrastructure, application, package compiler and archive
packages pass their full Go suites. Focused control/finally race tests and editor
tests pass. The SDK owns a fixed 65,536-step synchronous evaluation budget;
protocol compute deadlines exclude effects already owned by the ordinary step
runner. This is trusted host workflow code, not a security sandbox.

Deferred work remains separate: the pure-expression/WASM profile below, typed
step-output dataflow/DAG scheduling, external `.pipe` control bodies, concurrent
control calls and full embedded-language editor diagnostics. No YAML parser
rewrite, publication or default SDK switch was performed. Native Windows proof
below uses the retained test VM; physical-machine reset/install remains user-owned.

## Native platform qualification

Use the canonical [host execution contract](../../../workflows/workflow-yaml.md#inline-pipelang-host-control)
and [fixture qualification](../../../../workflows/test/workflow-control/README.md).

- Linux amd64 has real installed-SDK, Ninja Multi-Config and full ASan/UBSan/leak
  proof for branches, loops, failure, cancellation, finally and relocation. The
  latest callback-context change passes full affected Go suites and focused race
  checks. The current CLI also passes all five shared Source journeys normally
  and with the controller, SDK bridge and consumer under ASan/UBSan/leak checks;
  every receipt closes unforced with independent cleanup and descendant removal.
- The retained Windows VM passes native compilation and five unchanged Source
  cases: default, skip, failure/cleanup, readiness-before-interruption/cleanup and
  fresh-project prebuilt-package relocation. They use original PowerShell 5,
  compiler-free runtime PATH, default Source evaluation and 8000ms body/5000ms
  independent cleanup. Every receipt closes unforced and containment confirms
  descendant removal. Physical Windows 11 and macOS remain unqualified.
- Windows CMake scratch stays in the project cache with a shorter prefix after
  an actual nested-path failure. MinGW statically links the controller's C++
  runtime after actual loader failure `0xc0000135`; PE imports and native startup
  now pass without compiler runtime DLLs.
- A signal-context propagation gap left PowerShell in its debugger. Controller
  callbacks now use call-local context-aware host adapters; independent finally
  retains the parent context. Sourced pre-scripts now use the same call-local
  cancellation and private output ownership. Linux actual Bash readiness,
  post-root inherited writers, independent cleanup and failure paths pass full
  affected Go suites and focused race checks. Two native Windows sourcing tests
  skip because Bash is absent; pre-cancellation and generic Windows custody
  regressions pass. Non-group Unix cancellation reports incomplete descendant
  custody; the legacy unbounded source capture contract remains unresolved.
  The Windows host owner keeps custody through root wait/output EOF,
  then releases ordinary redirected children to preserve GUI early return.
  Retained-handle regressions pass normal exits 0/17, genuine EOF, active
  cancellation and cancellation after root exit. Existing bounded process policy
  and default Unix signal/terminal behavior are preserved.
- Same-identity VM recovery preserves disk, firmware, TPM, UUID and tool pins;
  no reinstall, credentials/services repair or system/default switch occurred.
  Keep completed VM proof separate from user-owned bare-metal reset/install/
  pairing prerequisites. Earlier failed/incomplete proofs remain private evidence.

## Integration harness continuation

The user explicitly selected this implementation order:

1. Format authored `.pipe` files with blank lines between logical steps, preserving
   tokens and behavior.
2. Implement general static methods in the owning PipeLang compiler and expose
   static testing assertions with the existing explicit result semantics.
3. Migrate applicable assertion callers. Preserve unrelated stateful types and
   instance-method tests even when their type is also named `Assertions`.
4. Carry integration cases, orchestration, assertions and results in the PipeLang
   testing framework across platforms. Shell and PowerShell remain valid effect
   adapters called through explicit native integration; they must not become a
   second test-policy framework.

The older `workflows/qualify/product` entrypoint still invokes an active Python
qualification framework. The source-owned `workflows/test/product` harness uses
PipeLang, with native transport qualified only on Linux. Neither the
legacy framework migration nor a shared Windows/macOS harness is complete.
Preserve existing assertions, case identities, evidence admission, resource
ownership and cleanup guarantees during migration. Platform adapter availability
and actual native qualification remain separate claims.

The package lifecycle lane still dispatches to
`workflows/qualify/product/drivers/package_lifecycle.py`. Its replacement
belongs in the source-owned PipeLang product harness. Keep serving the
disposable loopback TLS catalog and running the installed CLI in native
adapters. Preserve the checksum rejection before the archive download, the
private-store checks, and proof that uninstalling one package leaves the other
package installed and runnable. Switch the registry route only after the
replacement produces the same case assertions and admissible receipts.

Native Windows SDK compilation, installation and inventory verification pass.
Further contract runs exposed pinned-toolchain fixture assumptions, a Clang
numeric-header emission warning and an incorrect floating-control test baseline.
The generator repair passes the existing Linux numeric consumers. Native Windows
acceptance remains in progress; retained failed runs are not an accepted baseline.
The targeted Windows static/member, contextual inference, intelligence, formatting,
composition/resource and static/inference object-runtime checks now pass. Normal
JSON source, native fault and maximal-input checks also pass; one allocation
fixture was corrected to accept eager or lazy container allocation while retaining
the same memory refusal and cleanup. Broad SDK and real product acceptance remain
separate, unfinished claims.

New feature changes remain uncommitted. Existing unrelated checkout work is
preserved. TASK-055 stays active for the explicitly deferred profiles and native
platform qualification; this bounded host-control slice is complete.

The user inserted exact static local `var` inference and shared compiler-backed
editor tooling before the remaining harness migration. The combined owning
PipeLang compiler now passes static/member, contextual inference, semantic graph
and source-formatting oracles. A discovered multiline-string formatting defect
was repaired with literal-preservation checks. Real Linux stdio LSP integration,
an isolated VSCode extension host and reproducible standalone archive/extracted
server checks pass. PipeLang owns those implementations and their capability
documentation; this does not qualify Dockpipe's native editor, other IDEs or
native Windows/macOS startup. Fresh selected-SDK qualification, all twelve
static/inference runtime modes, both testing libraries and the Dockpipe SDK/product
consumers pass on Linux, including full sanitizer/leak checks. SDK manifest
admission was corrected to accept exact static-method metadata only in supported
profiles and declaration categories. No compiler implementation, default SDK
change or publication is part of this Dockpipe checkpoint.

The active policy migration needs bounded JSON queries beyond source string/list
limits. A general owning PipeLang JSON reader is implemented with immutable
document/node handles. Its Linux generated ABI, 42 authored cases, native lifetime
and resource checks, and maximal 4 MiB input pass in normal and full sanitizer/leak
modes. Native reader sources are manifest-bound SDK artifacts for instrumentation;
they do not depend on a sibling checkout. Windows compiler/SDK qualification and
the generic Windows transport remain in progress. Product registry, receipt,
coverage and resume policy remains source-owned migration work. The existing
Python entrypoint has not been retired or replaced.

The production Windows SDK broker passes ten Source-owned cases at default
budgets: exact arguments, BREAK/custody, root exit with live writers, 2 MiB
complete overflow drainage, separate-group exclusion, lost offer, partial
adoption, actual late offer, the selected 32,766-character UTF-16 command boundary
and exact child cwd/environment. Eleven raw receipts close and 62 caller
observations match, including final native-client release. Unknown-root failures
retain PID 0/unknown exit facts and original errors after explicit forced closure.
The private handoff ceiling is 5000ms; Scope retains one overall 5000ms maximum.

The unchanged shared real-CLI consumer's nine operations and codec controls now
pass with original PowerShell 5 and the current context/GUI-compatible owner.
An earlier explicit portable PowerShell 7 profile also passed. Earlier debugger
stalls remain negative evidence, not the accepted baseline. Remaining protocol/
nested-job edges, broad SDK/product, actual GUI, physical Windows and macOS proof
remain separate from these bounded results.

An inactive source registry slice contains indexed selection and authored
fixtures. Its ten fixtures pass at default budgets, and the frozen inventory
passes with an explicit 1,048,576-step profile. This checks counts, not exact-row
parity or full protocol equivalence. PipeLang's general segment-budget option
preserves defaults and passes all six native/generated forms, both linked-runtime
source forms, and focused default initialization, bounded-call, finally and parallel
regressions normally and under sanitizers. PipeLang's optional document-scoped
string index and all existing reader regressions pass normally and with the reader
fully sanitizer-instrumented. It preserves ordinary source collection limits;
4,097 independent source ABI calls alone do not prove one authored traversal.
Registry adoption now passes its original ten fixtures and frozen inventory in
normal and fully instrumented modes. Separate source cases also pass one-call
traversal of 4,097 entries and rejection of an escaped duplicate, including index
release. The exact frozen rows/assertions and selector matrix for all 17 profile
data records pass normally and with the JSON reader and generated policy fully
instrumented by ASan/UBSan with leak checking. A smaller
explicit 4 KiB arena also passes that fixed corpus and avoids repeated oversized
reservations; this is not a default or a universal capacity promise. Maximal
index/input shapes remain pending.
An additive maintained SHA256 streaming ABI and independent optional
`PipeLang.Crypto` Source package are implemented in PipeLang, with existing ABI1
and dependency pins preserved. Rust tests, ABI1/ABI2 native boundaries and sixteen
Source-owned vector, partition, capacity, lifetime and genuine pre/post-consumption
cancellation cases pass. The Source cases also pass with the generated code,
adapter and native caller under ASan/UBSan/leak checks; Rust internals are not
instrumented. Both 512 MiB partitions match an independent maintained OpenSSL
answer. The unchanged optional package also passes installed-only relocated
generation/runtime and twelve explicit identity/install-alias refusals. Native
Windows crypto and publication notices remain pending; core SDK roots, defaults
and active consumer pins are unchanged.
The optional Linux x86-64 FileStreams SDK now supplies descriptor-rooted file
capture, bounded fragment reads and streamed evidence digests. The 31 Source
cases pass normally, with C++ ASan/UBSan/leak checks, and from relocated
installed SDKs. They cover the exact 4 MiB capture and 512 MiB digest, observed
replacement during reads, cancellation and independent resource cleanup.
The adapter returns immutable accepted bytes and observes file and ancestor
identity at defined points. Concurrent changes can escape those observations.
The focused Source evidence policy admits up to 64 receipt entries against
actual confined files and verifies each digest. Its 21 cases pass normally and
with the JSON reader, generated Source and native file adapter instrumented.
The Source lane receipt policy passes 30 normal and sanitizer cases, including
all 8,192 case positions in one lane and an exact 4 MiB receipt captured from
disk. It checks captured identities, requested cases and assertions, actual
evidence, and process exit status. The installed JSON SDK compares borrowed
decoded assertion IDs across the request and receipt documents. All documents
close. The file proof covers canonical UTF-8 lane output.
Source accounting passes 11 normal and sanitizer cases for required and critical
denominators, statuses, layers, the 90 percent threshold, and selected and full
platform coverage.
The optional exclusive writer passes 16 normal and sanitizer cases for exact
4 MiB output, duplicate refusal, cancellation, partial writes, and retained
terminal effects. Source input custody passes eight cases in both modes,
including exact copies after source mutation and resume refusal when a
retained plan or registry is missing or has a different digest.
The optional atomic replacement SDK passed eight normal and sanitizer cases,
including a 32 MiB report-sized write, visibility to older open readers and
uncertain custody after a successful rename followed by a failure. The optional
existing-file append SDK passed nine cases in both modes, including a real
partial write and stale-pin refusal. The six Source event-log cases also pass
in both modes for fresh logs, evidenced resume, and newline boundaries.
Driver source admission and revalidation pass thirteen cases in both modes
against actual product and repository roots, including a 64-dependency driver
and a same-size file change.
The driver claim index passes nine cases in both modes, including disjoint
filters, overlapping claims, unclaimed cases and all 8,192 positions.
The joined Source preflight passes five cases in both modes. It validates the
plan and registry before admitting dependency pins and nonoverlapping driver
claims across all selected targets. Applicable cases stay in the denominator
when capabilities are missing. Passing preflight creates no output or processes.
PipeLang's optional borrowed-value renderer passes native and generated Source
checks with and without C++ ASan/UBSan/leak instrumentation. It writes bounded
JSON fragments in decoded-key order for report and event projection.
Its structured document writer passes both modes for typed scalars, nested
containers, borrowed values, exact byte limits and failure custody. Generated
Source writes an actual file and verifies its exact serialized bytes.
Both output effects also pass all four native and generated contracts from a
fresh installed SDK in both modes. The Source lane request fixture passes
three cases in both modes and matches the established request bytes, including
exact captured input digests, registry ordering and nested configuration.
Unavailable-only and cancelled selections create no request.
The native SDK's explicitly selected tool mode preserves argument boundaries,
raw streams, bounded overflow receipts and owned process disposal. Its Linux
checks also exercise an actual workflow in the existing Dockpipe mode.
Retained lane verification passes nine cases in both modes. It compares the
exact request bytes and admits terminal process metadata, with refusals for
missing, malformed, changed or aliased evidence.
The optional JSON reader detects valid UTF-8 BOM, UTF-16 and UTF-32 inputs
while preserving original bytes. Normal and sanitizer checks pass for the
new selection and the existing strict defaults.
The Source report projection matches its normal and sanitizer fixture bytes
for coverage totals, absent profiles, failed and missing cases, and extensions
on admitted rows. The registry supplies criticality and the invocation supplies
the evidence root.
Five driver runtime cases pass in both modes on Linux. The generated Source
binds the owned native process and writes terminal metadata after both private
raw logs close. The cases cover nonzero exit, graceful timeout, exact 4 MiB
overflow retention, pre-cancellation and cancellation after readiness.
The installed PipeLang compiler now resolves statically known qualified generic
calls before clone-based instance discovery. The full fresh-lane project
compiles, the existing project clone limit still rejects its over-budget
fixture, and the legacy compiler protocol still accepts that fixture.
The Linux fresh-lane test runs the existing driver protocol fixture through
its selected interpreter in normal and sanitizer builds. It verifies exact
request custody, private logs and process metadata, admits the successful disk
evidence, and rejects a passing receipt from a child that exits 7.
The combined lane passes both modes for two fresh executions and five retained
checks. Retained request bytes, terminal process metadata and disk evidence
must agree. The lane refuses altered or incomplete metadata and enforces the
shared receipt allowance. The cases also cover UTF-8 BOM, UTF-16 and UTF-32
inputs, with hashes taken over the original wire bytes.
The run input contracts pass both modes for exact private input copies,
independently verified executable hashes, read-only resume, cancellation and a
changed runner manifest. The compiled catalog matches the legacy listing for
all 17 profiles, 178 cases and 546 assertions. The lane builds without the
Testing library, and the test entry still produces the original 18-case plan
report. Source-owned target observations match coverage from the previous
policy, including unavailable and unclaimed cases. The event publisher passes
both modes for the nine event kinds, resume append and warning-sink failure.
The report commits atomically after checking the runner identity again, and
both modes cover exit codes 0, 1, 2 and 130 and preserve an existing report on
cancellation or interpreter drift. The joined lane fixture passes six cases in
each mode. It publishes the start event after committing the request and
before launching the child, and retained lanes preserve terminal exit status
without launching a child. The active Python runner still handles qualification orchestration,
including dispatch across targets, events and the final report.

Plan schema admission now passes eighteen Source-owned cases normally and with
the JSON reader, generated policy and native caller under ASan/UBSan/leak checks
on Linux. The complete 68-variant suite runs in one Source invocation with the
existing explicit 33,554,432-step predicate-validation caller profile; default and
cleanup remain 65,536. Default-budget failures are retained. All documents and
indexes close, and the active Python files, registry fixtures, cases and assertions
are preserved. This proves the finite authored corpus, not maximal-schema or
Windows/macOS plan acceptance.

The owning Testing SDK supplies method-scoped probe-token composition after the
compiler correctly refused a token stored in model state. Its nine additional
Source contracts and original synchronous/asynchronous suites pass normally and
with ASan/UBSan/leak checks on Linux; the nine probe and five original synchronous
contracts also pass on the Windows VM. No compiler workaround, active SDK pin or
global budget change was introduced. Receipt/evidence, coverage/resume and the
active runner migration remain unfinished.
