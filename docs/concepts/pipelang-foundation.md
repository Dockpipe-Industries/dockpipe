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
implementation status and approval. At inventory time, v0.111.0 was the completed baseline and
v0.112.0 arrow-method selector result arms were executing. This documentation update does not
interrupt, expand, or approve a successor to that active objective. Historical version-specific
exclusions remain correct for those versions.

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
| F04 | Enums | Missing: user declarations, members, representation, exhaustive handling, equality and serialization. Built-in arithmetic errors are not general enums. |
| F05 | Structs and records | Partial: Struct spelling shares class parsing; distinct immutable primitive-field Records have bounded value behavior. Define complete value/copy, nested-field, member, construction and equality semantics. |
| F06 | Classes and objects | Partial: configuration declarations/defaults exist; general construction, reference identity, mutable fields, encapsulation and initialization remain. |
| F07 | Interfaces and polymorphism | Partial: declaration/conformance checks exist; executable interface dispatch and substitutability remain. Broader abstract/virtual/override/inheritance support is required design work, superseding blanket deferral. |
| F08 | Generics | Missing user-defined generic types/functions and constraints. Built-in applied List/Optional/Result shapes are not general generics. |
| F09 | Tagged unions and failure values | Partial: bounded Optional/Result construction, matching and propagation exist; user unions, general composition and exhaustive matching remain. |
| F10 | Functions | Partial: public same-class pure calls exist; cross-class/module calls, access rules, overload resolution and callable composition remain. |
| F11 | Callable values | Missing: lambdas, delegates/function values, captures, lifetime rules and interactions with tasks. |
| F12 | Local variables and control flow | Partial: typed immutable locals and bounded terminal branches exist; mutable locals, reassignment, definite assignment, general statement sequences and early returns remain. |
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

## Design decisions required before implementation

1. **Object/type model:** distinguish struct/record values from class references; define nesting,
   initialization, equality/hash/order, interface dispatch and generics. Specify abstract classes,
   virtual/override and inheritance rules, including whether multiple class inheritance is excluded.
2. **Control flow:** define mutable local scope, assignment, general returns, loops, break/continue,
   accumulation, recursion policy and resource enforcement. Bounded proof must not be confused with
   a claim that only literal-size programs can run.
3. **Memory and sharing:** define which values may cross task boundaries, copying versus sharing,
   visibility/happens-before, race prevention or diagnostics, and backend obligations. Managed memory
   does not by itself make concurrent access safe.
4. **Task lifecycle:** define structured ownership, joins, failure/cancellation precedence, deadlines,
   cleanup and limits. Separate pure parallel computation from effectful work without silently
   granting external authority.
5. **Synchronization:** specify atomic operations and ordering, lock ownership/reentrancy and
   suspension rules, semaphore permits/fairness/cancellation, misuse diagnostics and cleanup.
   Define whether waiting is blocking or suspending; target schedulers cannot invent semantics.
6. **Determinism and replay:** pure code remains deterministic. Synchronization can make observable
   results depend on execution order even without data races. Define the allowed boundary, trace
   requirements and replay guarantees; do not retain an unconditional claim that scheduling is
   unobservable, or promise deterministic shared-memory results without proof.
7. **Profiles and verification:** define supported features and resource ceilings per profile,
   cross-backend conformance, controlled schedule exploration and honest coverage limits. Missing
   capabilities must fail explicitly; Go locks/tasks are implementation tools, not the specification.

Raw pointers, manual free, unsafe casts, exposed OS thread handles and hidden host access remain
outside this managed-language direction. Exact exception syntax, variance, operator overloading,
reflection and other unrequested extensions are decisions to inventory, not implied promises.

## Next decision and implementation order

After the active approved objective reaches its own terminal boundary, the next recommended
checkpoint is a **foundation specification and dependency plan**, not another automatically chosen
conditional-expression placement and not a production application adapter. The founder has approved
recording this direction; the next bounded objective and its implementation still require their
normal selection/approval. No v0.113/v0.114 feature or implementation batch is selected here.

The design checkpoint should produce:

- a reviewed disposition for F01-F34, including any explicit deferral and the definition of
  foundation completion;
- the object, memory, task and synchronization decisions above, with worked examples, expected
  results and rejection cases (illustrative syntax must be marked unaccepted);
- a dependency graph and bounded implementation slices, each with compiler-layer ownership,
  acceptance proof and compatibility obligations; and
- a recommendation-first choice of 2-3 next implementation slices grounded in that graph.

Dependency direction: value/reference/type and module rules support general functions/control flow;
those support libraries and managed state. The sharing model and typed effects constrain tasks;
tasks plus sharing constrain atomics/locks/semaphores. Reactive actions, contracts, replay and
concurrency verification must agree with those decisions before target adapters depend on them.
This is not a requirement to implement each entire family before any useful cross-layer proof.

Production launcher/application/service generation follows the reviewed language-foundation
milestone. Existing read-only Application IR fixtures remain useful acceptance evidence and do not
constitute that pivot. Qt generation, launcher parity, service generation and deployment are separate
downstream work; the minimum self-hosting library is language work, while porting/bootstrap is its
own milestone to place explicitly in the dependency plan.

The earlier conversational v0.114 pivot and 50-80-slice estimate are withdrawn for this expanded
scope. F01-F34 are not 34 slices. Publish a new count only after dependency sizing; no completion
date or final language version is established by this inventory.

## Completion evidence and maintenance

For each implemented capability, link its accepted semantics and source spelling, typed HIR/Core
admission and refusal, evaluator/generated-backend agreement, source diagnostics, compatibility,
tooling and applicable resource/concurrency proof. Define whether a capability is complete for the
full foundation or only an explicit subset. Do not mark completion from syntax support alone.

Update this inventory when a family materially advances; leave per-version receipts and chronology
in the owning task records. Maintain the TASK-021 foundation-planning route and downstream
TASK-020/TASK-022 dependency links. Preserve the frozen compatibility lane, generic engine/package
boundaries, current verification limits and active objective scope.
