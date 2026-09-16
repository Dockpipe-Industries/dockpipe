# PipeLang language foundation completion plan

## Direction and status

Founder alignment on 2026-09-11 requires a coherent general-purpose managed language before
pivoting to production launcher/application/service generation. The foundation includes enums,
structs/value types, classes, polymorphism, generics, general control flow and loops, managed
mutation, asynchronous tasks, parallel execution, atomic variables, lock objects and semaphores.
These are requirements to design and implement, not optional omissions justified by older scope.

This direction supersedes the earlier foundation-wide exclusion of threads/locks/shared-memory
concurrency and the deferral of broader polymorphism. It does not introduce raw OS thread handles,
unsafe memory, ungoverned host access, or backend-specific language behavior. The precise managed
concurrency and inheritance models remain design decisions; C# familiarity does not imply C#
compatibility or acceptance of every C# feature.

This page is the canonical capability inventory and planning direction. The live
[TASK-021 index](../agents/tasks/pipelang-reactive-application-language/index.yaml) owns current
status and approval. The accepted executable baseline is v0.114.0; P01 nominal enums and
[P04.a lexical blocks, joins and early returns](../agents/tasks/pipelang-reactive-application-language/general-blocks.md)
are complete. No successor capability is selected.
The authorized foundation specification/dependency planning objective is now complete. Its
[contract proposals](pipelang-foundation-contracts.md) and
[delivery ledger, slices and verification economics](pipelang-foundation-delivery.md) are reviewable
proposals, not accepted executable semantics. P01 nominal enums were subsequently selected and
approved; the [completed objective](../agents/tasks/pipelang-reactive-application-language/nominal-enums.md)
owns its concrete scope and accepted verification.

The user-requested [native backend and footprint strategy](pipelang-native-backends.md)
proposes retaining the Go compiler while adding C++, C and assembly/native output
through Core IR. It includes explicit speed/storage regression gates and an artifact
lifecycle investigation. This is a separate planning proposal; it adds no implemented
backend, changes no active objective, and does not include new backends in the existing
foundation delivery estimate.

The inventory is based on source, existing tests and contract inspection, not a new verification
run. In-progress code, parser recognition, internal IR representation, design fixtures and working
Go runtime mechanisms do not by themselves establish a completed executable language feature.

## Capability inventory

**Partial** means a restricted implementation exists. **Missing** means the required executable
language capability remains to be implemented. **Design revision** identifies an old exclusion or
an unresolved model that must be reconciled before implementation. Rows are capabilities, not
implementation slices; completion and scope must be evidenced independently.

| ID | Capability | Inventory status and remaining work |
| --- | --- | --- |
| F01 | Compiler pipeline | Established for the accepted subset: source, typed HIR, target-neutral Core, evaluator and Core-only Go. Extend all layers for every new capability. |
| F02 | Primitive and numeric values | Partial: bool/int/float/string and bounded checked arithmetic exist; numeric widths, signedness, explicit conversions, exact decimal and complete operations remain. |
| F03 | Text and bytes | Partial: Unicode-aware text operations exist; complete byte/scalar/grapheme APIs, slicing, parsing and encoding library remain. |
| F04 | Enums | Partial: v0.113.0 completes P01 public nominal declarations, stable tags, typed transport/equality and exhaustive matching. Serialization, payload unions and container/field integration remain later P09/P32/P03 work. |
| F05 | Structs and records | Partial: Struct spelling shares class parsing; distinct immutable primitive-field Records have bounded value behavior. Define complete value/copy, nested-field, member, construction and equality semantics. |
| F06 | Classes and objects | Partial: configuration declarations/defaults exist; general construction, reference identity, mutable fields, encapsulation and initialization remain. |
| F07 | Interfaces and polymorphism | Partial: declaration/conformance checks exist; executable interface dispatch and substitutability remain. Broader abstract/virtual/override/inheritance support is required design work, superseding blanket deferral. |
| F08 | Generics | Missing user-defined generic types/functions and constraints. Built-in applied List/Optional/Result shapes are not general generics. |
| F09 | Tagged unions and failure values | Partial: bounded Optional/Result construction, matching and propagation exist; user unions, general composition and exhaustive matching remain. |
| F10 | Functions | Partial: public same-class pure calls exist; cross-class/module calls, access rules, overload resolution and callable composition remain. |
| F11 | Callable values | Missing: lambdas, delegates/function values, captures, lifetime rules and interactions with tasks. |
| F12 | Local variables and control flow | Partial: v0.114.0 completes P04.a nested lexical blocks, initialized immutable locals, sequential branches/joins and early returns with independent Core validation. Mutable slots, reassignment and delayed-initialization definite assignment remain P04.b work. |
| F13 | Loops and recursion | Missing: iteration, accumulation, break/continue, nested control flow, recursive-call policy and enforceable resource/termination rules. |
| F14 | Lists | Partial: bounded construction, append, count, indexing, selection, filtering and sorting exist; general element types and composition remain. |
| F15 | Maps, sets and builders | Missing: deterministic collections, stable hashing, equality/order capabilities, scoped builders and mutation-during-iteration rules. |
| F16 | Managed memory and resources | Partial for current values: specify complete aliasing, copying, reference lifetime, captured/shared values, deterministic resource release and profile requirements. |
| F17 | Modules and visibility | Partial: structured locked module binding and identity/migration groundwork exist; authored module/import syntax, full callable use, access and distribution integration remain. |
| F18 | Entrypoints and effects | Missing executable language contract for typed host operations, authority, outcomes and resource lifecycle; compile/catalog/editor analysis remain inert. |
| F19 | Async tasks | Missing: task creation, awaiting, result/error types, parent-child ownership, suspension and completion. Exact syntax is undecided. |
| F20 | Structured parallel execution | Missing: bounded fan-out, join/result ordering, failure aggregation and pure/effectful execution rules. Workflow async groups do not implement PipeLang tasks. |
| F21 | Cancellation and deadlines | Missing: propagation, completion races, cleanup, progress and operation-specific retry/idempotency semantics. |
| F22 | Shared-memory model | Design revision: replace the former exclusion with specified sharing, visibility, ordering and enforceable race-safety rules. |
| F23 | Atomic variables | Design revision/missing: supported types, load/store, exchange, compare-and-swap, read-modify-write and memory-order guarantees. |
| F24 | Lock/mutex objects | Design revision/missing: ownership, scoped acquisition/release, reentrancy policy, lock ordering and suspension/cancellation behavior. |
| F25 | Semaphores | Required new design/missing: permits, ownership/accounting, waiting, cancellation/timeouts, release guarantees and fairness policy. |
| F26 | Concurrent communication | Undecided: determine required channels/queues/concurrent collections, ordering and backpressure; do not silently promise every abstraction. |
| F27 | Reactive state | Missing: state ownership, computed dependencies, invalidation, cycles, serialization and observer ordering. |
| F28 | Typed actions | Missing: validated atomic state transitions, typed events and result application. Atomic multi-field transitions are distinct from atomic variables. |
| F29 | Contracts and refinements | Missing: executable preconditions/postconditions/invariants, constrained values, static/runtime checks and compatibility rules. |
| F30 | State machines | Missing: guarded transitions, legal/terminal states, traces and bounded verification. |
| F31 | Language testing and replay | Partial: compiler conformance infrastructure exists; authored tests, contract-derived generators/shrinkers, effect replay and concurrency verification remain. |
| F32 | Semantic tooling and debugging | Partial: source spans, diagnostics, semantic identities and projections exist; complete graphs, source stepping, stacks/watches and feature-complete debug metadata remain. |
| F33 | Self-hosting library and bootstrap | Missing: complete compiler-required library, PipeLang compiler implementation and reproducible stage-0/1/2 proof. |
| F34 | Target profiles | Planned: enforce full/constrained/MCU capability and resource contracts without silently weakening semantics. |

Implementation evidence: [parser](../../src/lib/pipelang/parser.go),
[AST](../../src/lib/pipelang/ast.go), [type references](../../src/lib/pipelang/type_ref.go),
[module tests](../../src/lib/pipelang/module_test.go),
[Core operations](../../src/lib/pipelang/coreir/core.go), and the
[versioned language reference](pipelang.md). The live objective owns newer completion evidence.

## Specification and dependency plan

The [contract proposals](pipelang-foundation-contracts.md) specify object/type, value/reference,
control-flow, memory/sharing, tasks, synchronization, reactive transitions, determinism/replay and
profiles, with positive and rejection examples. All proposed source spelling is illustrative and
unaccepted. D1–D6 identify the unresolved founder choices; existing accepted numeric/text/failure,
compatibility and authority contracts are preserved.

The [delivery plan](pipelang-foundation-delivery.md) maps every F01–F34 row to inspected evidence,
remaining deliverables, prerequisites and proposed completion disposition. Its 33 dependency
packages contain **52–84 proposed foundation slices**, including the minimum self-hosting library
and an explicit M-app acceptance gate. This is a decomposition range with stated assumptions and
split triggers, not a statistical forecast, date or final version. It replaces the unvalidated
conversational 120–200 estimate; the earlier 50–80 estimate remains withdrawn.

Recommend first app work after M-app, using the maintained Go seed. Compiler port/bootstrap remains
required F33 work at a separate milestone estimated at **10–19 additional slices**, giving
**62–103 foundation-plus-bootstrap slices**. Whether bootstrap must precede the first app is an
explicit founder decision. Production Qt/launcher/service/deployment and physical target certification
remain separate downstream work requiring their own estimates and approval. F33 is not declared fully
complete at M-app. F26 queue scope and F34 profile/physical-target boundary also require ratification.

The dependency plan offered three implementation choices: nominal enums, general blocks/mutable
locals, or nested value semantics. The founder selected and approved P01 nominal enums. Foundation planning was
authorized by the user's request to plan and start; it does not transfer the completed v0.112.0
approval to new source changes. The [planning objective record](../agents/tasks/pipelang-reactive-application-language/foundation-specification-plan.md)
owns completion and documentation validation. No broad compiler rerun was needed for this docs-only
checkpoint; verification estimates use the existing accepted receipts.

## Completion evidence and maintenance

For each implemented capability, link its accepted semantics and source spelling, typed HIR/Core
admission and refusal, evaluator/generated-backend agreement, source diagnostics, compatibility,
tooling and applicable resource/concurrency proof. Define whether a capability is complete for the
full foundation or only an explicit subset. Do not mark completion from syntax support alone.

Update this inventory when a family materially advances; leave per-version receipts and chronology
in the owning task records. Maintain the TASK-021 foundation-planning route and downstream
TASK-020/TASK-022 dependency links. Preserve the frozen compatibility lane, generic engine/package
boundaries, current verification limits and active objective scope.
