# TASK-021 PipeLang Deterministic Self-Hosting Managed Language Foundation

## Decision Status

The `vNext` foundation decision packet in this record is **accepted for implementation planning** as
of 2026-08-16. Acceptance fixes semantics, compiler boundaries, bootstrap stages, compatibility,
and implementation order; examples and fixtures remain non-normative and accept no production
syntax. The first production slice still requires its own implementation review.

## Goal

Evolve PipeLang from its current optional typed configuration/model layer into a general-purpose,
target-neutral managed language for deterministic programs, compiler implementation, AI-assisted
change analysis, automated verification, declarative applications, and transport-neutral services.
PipeLang must eventually compile its own compiler without becoming C#-compatible, target-shaped, or
an unsafe/manual-memory systems language.

PipeLang should own types, validation, reactive state, pure computed values, typed state actions,
stable semantic identities, explicit effects and authority, contracts/invariants, deterministic
replay metadata, governed effect declarations, test-generation metadata, and safe binding
expressions. It compiles those semantics through one generic executable pipeline and publishes
versioned semantic projections consumed by tooling. The Application IR in
[TASK-020](declarative-application-surfaces-and-target-builders.md) and Service IR in
[TASK-022](go-first-pipelang-backend-services.md) are specialized projections over that shared
semantic/Core foundation, not independent language models.

This is a backlog/design record only. It does not authorize parser, lexer, AST, typechecker,
evaluator, compiler, CLI, schema, catalog, editor-extension, generated-artifact, or runtime changes.

## Priority And Dependency

This is the first implementation dependency for TASK-020. Define and prove the language semantics
before implementing Qt/web application adapters or accepting a public application YAML shape.

TASK-021 owns the language and compiler contract. TASK-020 owns semantic application components,
layout/styling, Application IR integration, target adapters, and artifact manifests. Qt, HTML, CSS,
QML, C++, CMake, and WebAssembly remain absent from PipeLang semantics.

[TASK-022](go-first-pipelang-backend-services.md) consumes the stable IDs, type, contract,
effect/authority, determinism, replay, and semantic-graph foundations defined here to describe
transport-neutral services. TASK-022 owns Service IR, Go backend and Qt client resolvers, schemas,
service tests, packaging, and deployment artifacts; backend concerns do not widen PipeLang core by
implication.

The initial delivery priority is:

1. source files, UTF-8 decoding, source spans, and structured diagnostics;
2. structured `TypeRef`, symbol ownership, modules/imports, and stable semantic identities;
3. one binding/type-checking path into typed HIR and backend-neutral Core IR;
4. explicit effects/authority plus contracts, replay, and first-class tests; and
5. the deterministic Go seed/backend needed to prove executable output and later self-hosting.

Broader reactive-state and application-language features build on those foundations rather than
landing as one uncontrolled rewrite.

## Language Personality

PipeLang remains C#-familiar without claiming C# source, binary, library, runtime, reflection, or
tooling compatibility:

- familiar braces, declarations, attributes, expressions, generics, and type spelling;
- strong static typing and ordinary code readable by C# developers;
- minimal punctuation and ceremony with clear source-located compiler diagnostics;
- deterministic semantics and explicit authority;
- no YAML-like indentation/nesting inside `.pipe` files;
- no Lisp-like, academic, or unnecessarily exotic surface syntax;
- no magical AI syntax embedded through normal programs; and
- one language contract across CLI, editor, compiler, semantic graph, tests, and backends.

Advanced semantics should make ordinary code safer and easier to inspect without making ordinary
code noisy. Attributes are appropriate for stable metadata and restrained architectural intent;
core control flow and contracts should use readable language constructs when attributes would hide
semantics.

The goal is not to bolt a model onto the language. Normal PipeLang remains fully useful without AI.
The language instead exposes enough stable, typed, deterministic structure for humans, agents,
compilers, IDEs, and verification tools to reason about the same program.

## Current Baseline

Canonical current behavior is documented in `docs/concepts/pipelang.md` as PipeLang `v0.0.0.1`.

| Current capability | Current boundary |
| --- | --- |
| Primitive types | `string`, `int`, `bool`, and `float` |
| Declarations | `Interface` and `Class`, visibility, annotations, fields, defaults, and structural conformance |
| Composite shapes | object/interface fields and `List<T>` type shapes |
| Modules | declarations merged from sibling `.pipe` files under one detected module tree |
| Methods | expression-bodied, statically typed, side-effect-free methods with CLI-only invocation |
| Expressions | literals, identifiers, unary operators, and bounded binary operators |
| Compilation | deterministic workflow YAML, bindings JSON, and bindings env artifacts |
| Integration | workflow `types:`, catalog/tooling metadata, materialization, CLI compile/invoke, and VS Code/Cursor language support |

The current contract explicitly rejects side effects, runtime/resolver execution through methods,
hidden compile-time execution, and general-purpose scripting. YAML remains first-class and DockPipe
workflow execution does not parse PipeLang directly.

## Existing Ownership Map

| Area | Current owner |
| --- | --- |
| Canonical language behavior | `docs/concepts/pipelang.md` |
| AST and type representation | `src/lib/pipelang/ast.go` |
| Lexing and parsing | `src/lib/pipelang/lexer.go`, `parser.go` |
| Static semantics | `src/lib/pipelang/typecheck.go` |
| Pure method evaluation | `src/lib/pipelang/eval.go` |
| Deterministic compilation | `src/lib/pipelang/compile.go` |
| CLI compile/invoke/materialize | `src/lib/application/pipelang_cmd.go`, `pipelang_materialize.go` |
| Workflow/catalog integration | `src/lib/domain/workflow.go` plus existing catalog/application projections |
| Editor language support | `src/app/tooling/vscode-extensions/dockpipe-language-support/` |
| Tests and compatibility fixtures | `src/lib/pipelang/*_test.go`, application PipeLang tests, and compiler golden tests |

Any public language change must update all affected owners together. Generated syntax support may
not lead or silently define the language.

## Hard Language Boundary

PipeLang is a general-purpose managed language and compiler foundation, not a second DockPipe
execution engine and not an unsafe/manual-memory systems language.

It must not expose unrestricted:

- filesystem, network, process, shell, environment, or secret access;
- pointers, manual allocation, unsafe memory, threads, locks, or target runtime handles;
- Qt, QML, browser DOM, JavaScript, C++, CMake, WebAssembly, or resolver-specific APIs;
- hidden work during parsing, type-checking, compilation, catalog projection, or editor analysis; or
- direct runtime/resolver execution that bypasses workflow/package and policy ownership.

Pure language evaluation is deterministic and offline. External work is represented by a typed
effect declaration and remains governed by the existing DockPipe model:

```text
PipeLang effect declaration
          |
          v
typed capability request
          |
          v
workflow/package -> runtime -> resolver -> optional strategy
```

Compiling, cataloging, rendering, hovering, or completing a PipeLang file never invokes the effect.

## Accepted Generic Compiler Contract

There is one compiler contract and one parser/binder implementation. CLI commands, editors,
catalogs, tests, semantic tooling, Application IR, Service IR, and native backends must call that
contract or consume its versioned outputs; they must not interpret PipeLang independently.

```text
UTF-8 source set + dependency lock + language version
                         |
                         v
              lossless syntax trees
                         |
                         v
         module/import resolution and binding
                         |
                         v
       typed HIR + effects + contracts + ownership
                         |
                         v
        target-neutral normalized Core IR
                         |
            +------------+-------------+
            |                          |
            v                          v
 versioned semantic projection   executable backends
            |                    (Go seed first)
      +-----+-----+
      |           |
      v           v
Application IR  Service IR
      |           |
      +-----+-----+
            v
 package-owned target resolvers
```

Compilation is a pure function of the complete source bytes, dependency lock, language/compiler
contract version, selected target profile/capability set, and explicit build options. Ambient or
absolute host paths, timestamps, locale, host map order, environment, network, and tool installation
are not semantic inputs. Artifact containers may record provenance outside hashed semantic payloads.

### Representation and projection boundaries

| Layer | Stability and owner | Allowed consumers | Forbidden coupling |
| --- | --- | --- | --- |
| Lossless syntax tree | Compiler-internal and version-private; preserves tokens, trivia, and full spans | Parser, formatter, IDE adapter through compiler queries | Backends or tools persisting node layouts |
| Bound tree | Compiler-internal; every name resolves to one owning module/symbol | Binder and diagnostic engine | Source-name lookup in backends |
| Typed HIR | Compiler-internal; explicit `TypeRef`, conversions, ownership, effects, contracts, and control flow | Type checker, analyses, Core lowering | Public schema stability promises |
| Core IR | Backend-neutral compiler contract; normalized types, functions, control flow, memory/value operations, effects, and source maps | Executable backends and specialized projection builders | Go, Qt, HTTP, MCU, or OS constructs redefining semantics |
| Semantic projection | Versioned public read model with semantic IDs, type/effect/contract summaries, source spans, graph edges, diagnostics, and compatibility metadata | CLI, editor, catalog, tests, impact/AI tooling, Application IR, Service IR | Reconstructing compiler internals or evaluating source |
| Application IR | TASK-020-owned versioned specialization over semantic/Core identities | Application target resolvers | Parsing `.pipe`, service semantics, or Qt/web behavior in PipeLang |
| Service IR | TASK-022-owned versioned specialization over semantic/Core identities | Service/client/schema target resolvers | Parsing `.pipe`, application layout, or Go/HTTP behavior in PipeLang |

Internal representations may change with the compiler. Public projections use independent schema
versions and compatibility rules. Application IR and Service IR reference stable semantic IDs,
`TypeRef` identities, source spans, effects, contracts, and selected Core units; neither copies or
forks language semantics.

### Deterministic Go seed and eventual self-hosting

Go is the first native executable backend and the bootstrap seed. Generated Go is deterministic,
readable, formatted, dependency-light, and checked against the same Core IR conformance suite as
future backends. Go implementation details never become PipeLang semantics.

Self-hosting proceeds through pinned, reproducible stages:

| Stage | Compiler | Required result |
| --- | --- | --- |
| Stage 0 | Reviewed Go seed built with a pinned Go toolchain | Compiles the accepted self-hosting subset and the PipeLang compiler source into a stage-1 compiler. |
| Stage 1 | PipeLang compiler produced by stage 0 | Recompiles the identical locked compiler source and library into stage 2. |
| Stage 2 | PipeLang compiler produced by stage 1 | When run on the same locked inputs, must match stage 1's normalized semantic projection, Core IR, diagnostics, public API, and target output digests after declared non-semantic provenance normalization. |

Stage equality requires byte identity for normalized artifacts and behavioral identity for the
compiler conformance corpus. Every stage records source, dependency-lock, compiler, standard-library,
backend, target-profile, and toolchain digests. A mismatch fails bootstrap; no stage may bless or
rewrite its own expected results. The Go seed remains a supported recovery artifact until a
separately accepted reproducible-bootstrap policy retires it.

### Target profiles and backend conformance

The source language has no target conditionals, target keywords, Qt/Go/HTTP types, pointer escape,
or MCU dialect. A build selects one profile and resolver-owned backend capability set:

| Profile | Semantic contract |
| --- | --- |
| `full` | Complete managed language and standard-library surface accepted for desktop/server targets. |
| `constrained` | Same semantics with declared static limits for heap, recursion, tasks, code size, or library capabilities; unsupported requirements fail before generation. |
| `mcu` | Same value/control/effect semantics with an explicitly bounded memory plan and resolver-provided device effects; dynamic allocation may be forbidden after initialization, but source meaning cannot change. |

Backends publish a machine-readable capability manifest. Selection fails on any required missing
capability, representation limit, numeric behavior, Unicode contract, or effect bridge. A backend
may optimize only when the observable language contract is preserved; it cannot silently truncate,
substitute, emulate with weaker guarantees, or drop a required feature.

## Accepted Core Semantic Decisions

Surface spelling remains deliberately unaccepted, but the following semantics are fixed before
grammar expansion.

### Values, numbers, text, equality, and ordering

| Concern | Accepted decision |
| --- | --- |
| Boolean and integer families | Booleans are distinct from numbers. Integers are explicit fixed-width signed or unsigned values with two's-complement representation; default arithmetic is checked and overflow is a typed failure. Wrapping/saturating operations are explicit library operations. |
| Floating point | Binary32/binary64 follow pinned IEEE-754 operations and rounding. `NaN` is unordered and unequal, signed zero compares equal, and deterministic total ordering is an explicit operation rather than an accidental target sort. |
| Decimal/exact quantities | Exact decimal is a versioned standard-library value type with checked precision/scale rules, not a backend-native alias. Money requires an explicit currency/domain type. |
| Conversions | No implicit narrowing, signedness change, float/integer conversion, or text parsing. Lossless widening within one numeric family may be implicit only when specified by the language contract. |
| Source and text | Source is strict UTF-8. `String` is an immutable sequence of Unicode scalar values; invalid encodings are diagnostics. Byte, scalar, and grapheme operations are distinct. Grapheme/case/normalization data pins a Unicode version in the standard-library contract. |
| Text identity | String equality, hashing, and ordering are ordinal over preserved scalar sequences. The language never silently normalizes or applies host locale; normalization and culture-aware comparison are explicit. |
| Equality | Enums and immutable values use structural equality. Mutable state owners and managed reference objects use identity unless they expose an explicit value projection. Effects/resources are not equatable by default. |
| Hashing | Hashability is an explicit type capability consistent with equality. The stable hash algorithm and seed are language-versioned and target-independent; randomized host/runtime hashes are not observable language results. |
| Ordering | Ordering is an explicit total- or partial-order capability. Collection sorting requires a proven default total order or an explicit comparer; backends cannot invent target ordering. |

### Value/reference, nullability, failure, mutation, collections, and memory

| Concern | Accepted decision |
| --- | --- |
| Value/reference model | Scalars, enums, records, tagged unions, optionals, results, and immutable collections are values. Classes that own mutable application state and explicit managed objects have reference identity. Type declarations expose the category; backends may not change it. |
| Nullability | Ordinary references and values are non-null. Absence is `Optional<T>` and must be handled explicitly. There is no implicit null widening, null default, or target null leakage. |
| Expected failure | Recoverable domain, validation, arithmetic, parsing, and effect outcomes use closed `Result<T,E>`-style values. Cancellation is a distinct declared outcome. Uncaught target exceptions/panics are infrastructure failures, never guessed into domain results. |
| Mutation | Immutable values produce new values. Local mutable variables and compiler-owned mutable buffers are lexically scoped and cannot escape as shared aliases. State-owner actions commit one validated transition atomically; observers see the before or after state, never a partial update. |
| Collections | `List<T>` is ordered; `Map<K,V>` and `Set<T>` have deterministic insertion iteration plus stable equality independent of target hash-table layout. Bounds are checked. Mutation uses scoped builders or explicit state updates; mutation during iteration is rejected. Canonical serialization orders map/set entries by the declared stable key order. |
| Memory | Memory is automatic and managed. There are no raw pointers, address arithmetic, manual allocation/free, user-visible object addresses, finalizer semantics, unsafe casts, data races, threads, or locks. Resource lifetimes cross explicit effect/host interfaces and use deterministic close/use protocols. |
| Concurrency | Pure computations are observationally deterministic. Structured tasks may be added only over typed effects with deterministic join/result ordering; backend scheduling is not observable. Shared-memory concurrency is outside the accepted foundation. |

### Modules, imports, distribution, and compatibility

- Every `vNext` file belongs to one explicitly identified module. Imports name modules or exported
  symbols; wildcard/relative/ambient imports are not part of the initial contract.
- Binding uses a locked dependency graph and canonical module IDs. Duplicate owners, ambiguous
  imports, undeclared dependencies, and import cycles fail with structured diagnostics.
- Visibility and symbol ownership are module-aware. Every bound symbol has exactly one owning
  module and declaration span before type checking.
- Packages carry language contract, semantic-projection schema, standard-library, dependency, and
  public semantic-ID versions plus content hashes. Compilation never fetches dependencies.
- Source packages are canonical for bootstrap. A package may additionally distribute verified Core
  IR/native artifacts, but those bind to exact source/lock/compiler/profile digests and are never
  treated as a newer source contract.
- Compatibility is checked on public semantic IDs, type shapes, effects, contracts, serialization,
  entrypoints, and projection schemas. Removing/renaming an ID, widening effects, strengthening an
  input precondition, weakening an output guarantee, or changing serialized meaning is breaking
  unless covered by an explicit versioned migration/alias policy.
- `v0.0.0.1` keeps its implicit sibling-module behavior only inside the frozen legacy compiler
  contract. Migration to `vNext` is explicit and produces a reviewable source/artifact diff.

### Executable entry points and host effects

An executable has one manifest-selected, explicitly exported entrypoint with typed arguments,
result/exit status, declared effects, and target-profile requirements. There is no top-level code,
module-load execution, static initializer I/O, or hidden environment access. Libraries have no
entrypoint and are inert when compiled or inspected.

The compiler core itself accepts source/dependency bytes and options as values and returns artifacts
and diagnostics as values. Its CLI shell performs declared file/process effects through governed
host capabilities. Ordinary external work follows the DockPipe chain
workflow/package -> runtime -> resolver -> optional strategy. Device I/O, HTTP, Go runtime services,
Qt events, files, clocks, randomness, secrets, processes, and models are resolver-owned effect
implementations, never syntax-owned privileges.

### Minimum self-hosting library

The self-hosting subset includes only what the compiler demonstrably needs:

- strict UTF-8 decode/encode, scalar-aware immutable strings, immutable bytes, and source slicing;
- fixed-width integers, booleans, optionals, results, records, enums, tagged unions, and bounded
  generics;
- deterministic lists, maps, sets, scoped builders, sorting/comparers, and stable hashing;
- modules/imports, functions, local variables, branches, loops, pattern matching, and explicit
  checked conversions;
- spans, structured diagnostics, canonical serialization, semantic IDs, and graph construction;
- pure parser/binder/type/HIR/Core APIs plus host-supplied file/artifact interfaces; and
- assertions, table tests, golden data, deterministic property seeds, and replay records.

Reflection, dynamic types, runtime code generation, macros, regex, networking, shells, threads,
locks, GUI APIs, HTTP, package download, and garbage-collector observability are not required for
self-hosting. A managed allocator/collector is backend runtime machinery, not a user language API.

## First-Class Verification Contract

- Tests are ordinary typed declarations in the semantic graph, not a second test-language parser.
  Unit, table, contract-derived, property, negative-compile, golden, differential-backend, and
  bootstrap tests share stable IDs and source spans.
- Replay records pin semantic IDs, source/dependency/compiler/library/profile digests, inputs,
  deterministic seed, ordered effect requests/results, redaction references, and expected outcome.
  Replaying cannot perform an unrecorded effect.
- The semantic graph is emitted from the bound/typed compiler state and distinguishes declaration,
  ownership, type, call, effect, contract, import, projection, backend, and test-coverage edges.
- AI tools receive the versioned projection, graph, diagnostics, change manifest, and bounded source
  spans through the same compiler contract. There is no AI-only syntax or trusted AI assertion.
- Change claims are independently classified as verified, contradicted, unverifiable, or
  out-of-scope by comparing semantic IDs, graph deltas, effects, contracts, artifacts, and tests.
  Model invocation is an explicit typed resolver-backed effect and is replayed/redacted like any
  other external result.

The non-production fixtures
[`compiler-contract.v1.json`](fixtures/pipelang-vnext/compiler-contract.v1.json),
[`bootstrap-reproducibility.v1.json`](fixtures/pipelang-vnext/bootstrap-reproducibility.v1.json), and
[`semantic-verification.v1.json`](fixtures/pipelang-vnext/semantic-verification.v1.json) make these
boundaries executable as future fixture assertions without accepting syntax or schemas.

## Required Language Surface

### Stable semantic identities

Externally referenced declarations require explicit identities that survive file moves and symbol
renames; local implementation declarations may remain ID-free. A C#-style attribute is the leading
non-normative spelling because PipeLang already supports annotations:

```csharp
[Id("deploy.production")]
public Action Deploy(Release release);
```

The accepted identity contract requires:

- which declarations may carry IDs: modules, types, fields/properties, methods, actions, effects,
  states, transitions, tests, and exported projections;
- which externally referenced declarations eventually require an ID and which local implementation
  details remain ID-free;
- normalization and allowed character rules without conflating an ID with a source symbol;
- uniqueness scope across a file, module, compiled package/workflow, and composed application;
- duplicate, missing-required, malformed, moved, renamed, and compatibility diagnostics;
- whether an explicit ID becomes part of the public compatibility contract;
- how IDs appear in compiler artifacts, diagnostics, catalog/Application IR metadata, test failures,
  impact graphs, and change manifests; and
- how aliases/deprecation work if a public semantic ID must change.

An explicit ID is stable because it is authored and validated, not because the compiler hashes a
path or symbol name. The compiler may derive non-public ephemeral identities for internal analysis,
but tools must never present those as rename-stable public contracts.

### Type system

Define the minimum coherent additions needed by application state and bindings:

- enums with closed member sets;
- records/value types distinct from mutable state-owning classes;
- `Optional<T>` with explicit null/absence semantics;
- typed list and map values, not only generic type shapes;
- bounded generic types required by standard collections and results;
- tagged unions/result types for closed state and error modeling;
- validation/refinement annotations with deterministic diagnostics;
- explicit serializable, persisted, session, and transient state classification; and
- module/import and namespace rules that retain deterministic multi-file compilation.

Do not add nominal or generic complexity without a fixture that requires it. Recursive types,
variance, inheritance beyond the existing interface boundary, user-defined operators, and open
dynamic values remain out of scope until separately justified.

### Pure expressions

Bindings and computed state need a safe expression language with:

- member access and optional chaining/explicit optional handling;
- boolean, comparison, arithmetic, and string operations;
- conditional expressions and closed pattern matching;
- deterministic string interpolation;
- list/map lookup and bounded standard collection operations;
- typed lambdas only where required for map/filter/count/selection fixtures; and
- source-located errors for unknown members, invalid operators, unsafe optional access, and
  incompatible branches.

Expression evaluation has no clock, randomness, environment, locale-dependent behavior, I/O, or
unbounded user recursion. Any future nondeterministic input is an explicit governed effect result.

### Reactive application state

Define target-neutral semantics for:

- observable state fields;
- pure computed properties with deterministic dependency graphs;
- invalidation and recomputation ordering;
- cycles and cycle diagnostics;
- persisted, session, and transient state scopes;
- initial/default state construction;
- immutable value updates versus mutable state-owner updates; and
- serialization compatibility across compiler versions.

Reactive behavior belongs to the language/IR contract. Qt signals/properties and web reactive stores
are backend implementations, not the semantics.

### Typed actions

Actions are explicit, deterministic application-state transitions. The accepted action contract
requires:

- typed parameters and return values;
- statement blocks versus expression/declarative transition bodies;
- assignment and collection-update semantics;
- atomicity and observable notification timing;
- validation before and after transition;
- typed failure without exceptions leaking from generated targets; and
- prevention of effect invocation from a supposedly pure action.

The minimum action implementation should cover selection, form editing, reset, local validation,
and applying an already returned effect result. It does not need loops, exceptions, arbitrary
objects, or a general standard library.

### Explicit effects and authority

Effects describe asynchronous or externally owned work without implementing that work in PipeLang.
Public effectful boundaries must declare machine-readable effects and approval requirements. The
compiler may infer effects through call graphs for validation, but inference must not silently widen
an exported contract.

Illustrative directions, not accepted syntax:

```csharp
[Id("deploy.production")]
[Effects(Effect.Network, Effect.FileSystemWrite, Effect.ProcessSpawn)]
[RequiresApproval("production-deploy")]
public Action Deploy(Release release);
```

or:

```csharp
public Action Deploy(Release release)
    effects Network, FileSystemWrite, ProcessSpawn
    requires approval "production-deploy";
```

The semantic effect taxonomy must cover at least:

- filesystem read and write;
- network and remote execution;
- process execution;
- environment/configuration access;
- clock and randomness;
- secret/private material access;
- external model invocation;
- workflow/package capability invocation; and
- separately governed approval requirements.

Effect composition rules must reject undeclared widening, pure-to-effectful calls, unresolved
effects, and target adapters that invent authority. Private implementation may infer a closed effect
set, while exported declarations must expose the validated set or an explicitly reviewed abstraction.

Each exported effect declaration binds:

- stable capability/workflow/package identity;
- typed input, output, and closed error/result contracts;
- required authorization/capability metadata;
- cancellation, progress, retry, and idempotency posture where the underlying operation supports it;
- explicit loading/success/error state transitions; and
- source information sufficient for policy and user-facing diagnostics.

Effects are inert declarations invoked only by an exported effectful entrypoint or application
binding through the governed host bridge. Actions cannot perform effects; they may apply a returned
typed effect result. Generated action forms may project this contract but cannot own it. No syntax is
accepted until it proves that catalog/editor analysis remains inert and execution still crosses
normal DockPipe authority boundaries.

External model invocation is only one explicit resolver-backed effect. Model input/output is typed
and validated like any other effect, carries `ExternalModel` authority, and cannot become an
implicit compiler, evaluator, completion, or runtime behavior.

### Contracts, invariants, and constrained values

Types and callable boundaries need reusable contracts that can drive compile-time checks, runtime
guards, generated forms/schemas/docs, resolver validation, and test generation from one definition.

Illustrative directions:

```csharp
public Action Transfer(Account source, Account destination, Money amount)
    requires amount > Money.Zero
    requires source.Balance >= amount
    ensures source.Balance == old(source.Balance) - amount
    ensures destination.Balance == old(destination.Balance) + amount;
```

```csharp
public type Percentage = decimal
    where value >= 0m && value <= 100m;
```

The accepted contract model requires:

- `requires`, `ensures`, type/state `invariant`, constrained aliases/refinements, `old(expression)`,
  and result references;
- which contracts are statically provable, emitted as runtime guards, or both;
- side-effect-free contract expressions and their available value scopes;
- contract inheritance/composition for interfaces, records, actions, effects, and state transitions;
- failure categories, stable semantic IDs, source locations, and generated diagnostics;
- compatibility rules when a public precondition strengthens or a postcondition weakens; and
- normalized metadata consumed by schemas, forms, resolvers, docs, tests, and impact analysis.

Contracts are executable/verifiable semantics, not optional prose annotations.

### Determinism and replay

Determinism is a mechanically enforced language property derived from the effect system and value
semantics. Pure code cannot silently observe clocks, randomness, environment, network, models,
secrets, locale, process state, or target runtime state.

The accepted determinism/replay contract requires:

- whether purity is inferred, explicitly declared at public boundaries, or both;
- diagnostics when deterministic code calls or captures effectful behavior;
- typed injection of clock, random source, environment, model, and remote-service results;
- stable serialization of inputs, effect results, state transitions, semantic IDs, language/compiler
  version, and relevant dependency identities;
- deterministic seeds and exact failure/execution replay; and
- the boundary between replay metadata and sensitive values that must remain references or redacted.

Illustrative intent:

```csharp
public pure decimal CalculateTax(Money subtotal, TaxRate rate)
{
    return subtotal.Amount * rate.Value;
}
```

The `pure` keyword is not accepted by this example; it is unnecessary if inference plus explicit
effect-free public contracts provide clearer semantics.

### Property-based and contract-generated testing

Testing is a first-class consumer of PipeLang types, contracts, semantic IDs, and deterministic
replay metadata. The language/compiler contract should support:

- generation of valid values from types and constraints;
- boundary values and deliberately invalid values;
- deterministic seeds and exact replay artifacts;
- shrinking while preserving constraints and semantic identity;
- derivation of baseline checks from `requires`, `ensures`, and invariants;
- association of tests/failures with declaration and contract IDs; and
- machine-readable test metadata for DockPipe tooling.

Illustrative C#-style directions:

```csharp
[Id("site.serialization.round-trip")]
[Test]
[ForAll]
public void SerializationRoundTrips(SiteConfig value)
{
    assert Decode(Encode(value)) == value;
}
```

The first slice should define generator/shrinker/replay contracts and one small built-in type
fixture, not prematurely build a complete property-testing framework.

### State-machine semantics

State machines should emerge from ordinary PipeLang enums/tagged unions, guarded actions,
contracts, and transition metadata rather than a separate unrelated DSL. The semantic model
represents:

- legal, guarded, terminal, and prohibited transitions;
- state and transition invariants;
- unreachable states and contradictory guards;
- transition semantic IDs and effects/authority;
- deterministic transition traces and replay; and
- generated positive, negative, and path-coverage tests.

An action is projected as a modeled transition only when it declares a source state-set and target
state-set or is referenced by an explicit state-machine projection. Bounded path generation records
maximum depth/state count and reports truncation rather than implying exhaustive coverage.

### Structured intent and rationale

Optional restrained attributes may carry queryable architectural context:

```csharp
[Intent("Prevent duplicate external side effects")]
[Protects("BillingConsistency")]
[Assumes("ProviderSupportsIdempotency")]
public Action RetryPayment(Payment payment);
```

This metadata is emitted into the semantic model for navigation and impact analysis. It does not
change runtime behavior, grant authority, satisfy a contract, or become trusted merely because an
agent authored it. Unknown or overused prose metadata must not pollute ordinary code.

### View-facing contract

PipeLang provides typed semantics consumed by YAML/Application IR without absorbing layout or
target rendering. The projection includes:

- stable field, computed-property, action, and effect identities;
- safe expression type-checking for visibility, enabled, text, selection, and collection bindings;
- typed event argument projection;
- conditional and repetition source contracts; and
- a versioned compiler projection with source locations and complete diagnostics.

Pages, component trees, responsive layout, theme rules, accessibility presentation, Qt, and semantic
HTML remain owned by TASK-020.

## Semantic Graph And Tooling Contracts

The compiler should expose a versioned semantic graph keyed by stable identities. Nodes may include
modules, types, members, contracts, actions, effects, states/transitions, tests, schemas, generated
projections, and externally referenced capabilities. Edges should distinguish typed references,
calls, effects, bindings, transitions, contract dependencies, generated artifacts, and test
coverage.

This graph should let tooling answer narrow questions such as which views, renderers, tests, schemas,
and exports depend on one changed field without loading or reparsing the entire repository. It must
include source locations, compiler/language version, and enough dependency kind information to avoid
presenting mere text matches as semantic impact.

### Verifiable change manifests

Agents and other tools may submit a proposed change manifest containing intent, touched semantic
IDs, claimed preserved contracts, tests, and requested capabilities. The manifest belongs at the
compiler/tooling or DockPipe operation boundary, not necessarily inside PipeLang syntax.

Illustrative external shape:

```yaml
intent: add-site-routing
touchedSymbols:
  - site.routes
preservedContracts:
  - navigation.unique-routes
testsAdded:
  - routing.generates-known-pages
capabilitiesRequested:
  - FileSystemWrite
```

Claims are never inherently trusted. Tooling should independently compare the semantic graph,
source diff, emitted artifacts, effect/authority changes, contract results, and executed tests. It
reports verified, contradicted, unverifiable, and out-of-scope claims separately.

## Non-Normative Fixture

The following illustrates required semantics only. It does not accept spelling, casing, keywords,
annotations, collection syntax, block syntax, or effect syntax.

```csharp
public Enum DeploymentStatus
{
    Pending,
    Running,
    Succeeded,
    Failed
}

public Record Deployment
{
    public string Id;
    public string Name;
    public DeploymentStatus Status;
}

public Result DeploymentRefreshResult
{
    List<Deployment> Success;
    DeploymentRefreshError Error;
}

public Class DeploymentState
{
    [Transient]
    public bool IsLoading = false;

    [Session]
    public Optional<string> SelectedDeploymentId;

    public List<Deployment> Deployments = [];

    public computed int FailureCount =>
        Deployments.Count(item => item.Status == DeploymentStatus.Failed);

    public computed Optional<Deployment> SelectedDeployment =>
        Deployments.FirstOrNone(item => item.Id == SelectedDeploymentId);

    [Id("deployment.select")]
    public Action SelectDeployment(string id)
        requires Deployments.Any(item => item.Id == id)
    {
        SelectedDeploymentId = id;
    }

    [Id("deployment.refresh")]
    [Intent("Refresh visible deployment state from the governed provider")]
    public AsyncEffect Refresh()
        invokes workflow "deployments.list"
        effects Network, RemoteExecution
        returns DeploymentRefreshResult
        requires capability "deployment.read";
}
```

A corresponding view fixture may reference `FailureCount`, `SelectedDeployment`,
`SelectDeployment`, and `Refresh`, but the view compiler must consume the semantic projection rather
than reimplementing PipeLang parsing.

## Accepted Decisions Before Parser Expansion

| Decision | Accepted outcome |
| --- | --- |
| Language versioning | The source declares or is manifest-bound to a language contract. Internal IR versions follow the compiler; semantic, Application, Service, replay, and artifact projections version independently. `v0.0.0.1` never silently opts into `vNext`. |
| Semantic IDs | Public/exported declarations referenced outside their module require explicit IDs. IDs use a lowercase dotted ASCII contract, are package-wide unique, and are distinct from names/paths. Local symbols receive non-public compiler IDs. Public changes require an explicit alias/deprecation/migration record. |
| Source and diagnostics | Every token and syntax/HIR/Core/projection node that can diagnose carries a file ID plus UTF-8 byte start/end and line/column rendering information. Diagnostics have stable category/code, severity, primary span, related spans, semantic IDs, and deterministic ordering. |
| Types and ownership | `TypeRef` is a structured graph (primitive, named, applied generic, optional, result, function) resolved to an owning module/symbol; type meaning is never encoded only as a string. Every symbol has exactly one owner and declaration span. |
| Nullability and identity | Non-null by default with explicit `Optional<T>`; immutable structural values and explicit mutable/reference identities follow the accepted core table above. |
| Mutation and computed graph | Local/scoped mutation cannot escape as shared aliases. State actions commit atomically. Computed dependencies come from bound symbol references, evaluate in stable topological order, reject cycles, and may cache only by semantic input identity/version. |
| Collections and lambdas | Deterministic collection semantics follow the core table. Lambdas are typed closures over immutable values or lexically owned locals; captured mutable state/effects are rejected unless a later explicit action/effect contract permits them. |
| Failures/results | Expected failure is a closed typed value; infrastructure failure is separate. Backends must preserve every declared outcome and cannot translate by target exception text. |
| Actions | Actions are bounded typed state transitions. They may call pure functions and apply already-returned effect results; they cannot directly perform effects. Preconditions run before, invariants/postconditions before commit, and failure leaves state unchanged. |
| Effects and composition | Effect declarations are inert typed capability requests. Private call graphs infer a closed set; exported effectful boundaries declare an equal or reviewed abstract set. Pure-to-effectful calls, undeclared widening, and backend-invented authority fail. Results enter state only through an action. |
| Contracts | `requires`, `ensures`, invariants, refinements, old-value, and result-value references are pure typed expressions. Statically provable clauses compile away; remaining clauses emit normalized guards. Strengthened public preconditions and weakened guarantees are breaking. |
| Determinism/replay | Purity is inferred internally and explicit in exported projection metadata. All nondeterminism is a typed effect input/result. Replay uses the first-class verification contract and cannot reveal secret values; it stores references/redactions. |
| Property tests | Generators derive from types/constraints; invalid/boundary generation is explicit; shrinking preserves constraints and stable test identity. Seed plus replay record must reproduce the minimized failure. |
| State machines | States are closed enums/unions and transitions are actions with stable IDs, guards, contracts, and optional effects. Terminal/prohibited/unreachable states diagnose through bounded graph analysis; path generation has explicit depth/state limits. |
| Intent metadata | Optional controlled metadata is queryable but non-semantic: it grants no authority, proves no contract, and may be removed without changing executable meaning unless a projection explicitly declares it required. |
| Semantic graph | The compiler emits the versioned, typed edge kinds defined above from one bound program. Incremental results must equal a clean full compile for the same inputs. |
| Change manifests and AI | Manifests are external/versioned claims checked against graph, diagnostics, effect/contract diffs, artifacts, and tests. AI uses the same contract; model invocation is an explicit `ExternalModel` effect. |
| Modules/distribution | Explicit locked modules/imports and offline package resolution follow the accepted module contract. No ambient sibling merge exists in `vNext`. |
| Entry points | Manifest-selected exported entrypoint, no top-level execution, with explicit result, effects, and profile requirements. Host operations remain governed effect implementations. |
| Compatibility | Frozen legacy lane or explicit version migration only. All current files and artifacts retain their current behavior until deliberately migrated. |

## Current-Versus-Required Grammar And AST Gap

The table is an implementation inventory, not a syntax proposal.

| Area | Current `v0.0.0.1` implementation | Required foundation before broad syntax |
| --- | --- | --- |
| Source model | Parser receives one byte slice; tokens carry one integer byte offset | Source-set/file identities, strict UTF-8, durable spans, line mapping, related locations, and deterministic multi-file ordering |
| Diagnostics | Formatted error strings with byte offsets | Structured code/category/severity, primary/related spans, semantic IDs, deterministic sort, and CLI/editor renderers over one diagnostic value |
| Program/declarations | `Program` holds only `Interfaces` and `Classes`; `Struct` parses into `ClassDecl` | Distinct module, import, interface, class/reference, record/value, enum, union/result, function, test, action, and effect nodes |
| Types | `TypeName` is a string; primitive checks and `List<T>` parse from spelling | Structured resolved `TypeRef`, generic arguments, named symbol identity/owner, optional/result/function types, capabilities, and profile validation |
| Symbols/modules | One merged global interface/class namespace across sibling files | Explicit module/import graph, one symbol owner, visibility, deterministic binding, duplicate/ambiguity diagnostics, dependency lock |
| Spans on AST | No AST node stores a source span | Spans on declarations, names, types, members, expressions, statements, contracts, effects, and lowered source maps |
| Expressions | Literal, identifier, unary, binary, parentheses | Member/call/index/conditional/interpolation, typed match, bounded closures, collection construction/operations, conversions, and optional handling |
| Statements/control flow | Expression-bodied methods only | Blocks, locals, assignment, branches, loops needed for self-hosting, return/match, and explicit action transition boundaries |
| Runtime values | `Value` stores only string/int64/float64/bool | Full target-independent value model, fixed numeric families, bytes/scalars, records/unions/options/results, collections, and managed identities |
| Type checking | Separate maps for interfaces/classes; primitive expression inference | Bound-symbol type checking, ownership/flow/nullability, generic constraints, effects/contracts, exhaustiveness, and backend/profile capability checks |
| Evaluation | Primitive expression evaluator | Pure reference semantics for constant folding/tests only; executable lowering goes through typed HIR/Core IR rather than a competing evaluator language |
| Compilation | Direct workflow YAML, bindings JSON/env emission | Generic typed HIR -> Core IR pipeline, semantic projection, backend contract, reproducible manifests, then specialized Application/Service projections |
| Entry/effects | CLI selects class/method; methods must remain pure | Explicit exported entrypoint plus inert typed effects and governed host bridges; no compile/editor execution |
| Tooling | Go parser/compiler and regex-based editor interpretation can diverge | One compiler service/query contract for CLI, editor, catalog, graph, tests, and backends |
| Testing/bootstrap | Focused parser/typecheck/golden tests | Negative diagnostics, semantic projection/Core goldens, property/replay, differential backends, and stage-0/1/2 bootstrap proof |

## Compatibility And Migration

The exact 45-file inventory is frozen in
[`legacy-v0.0.0.1-inventory.txt`](fixtures/pipelang-vnext/legacy-v0.0.0.1-inventory.txt).
Every listed path has the same treatment: preserve exact `v0.0.0.1` parse/type/evaluation,
workflow-YAML, bindings JSON/env, catalog, materialize, CLI invoke, and editor expectations until an
explicit file/package migration selects a later language contract. The inventory currently contains
22 files with interface declarations, 30 with class declarations, one `Struct` spelling that is
currently collapsed into `ClassDecl`, and one expression-bodied method file; files may contain more
than one declaration, so those counts intentionally overlap.

- New compilers must retain a frozen legacy frontend/emission lane or provide an explicit,
  reviewable, test-covered migration with before/after artifacts. Filename, directory, or compiler
  upgrade alone never selects new semantics.
- Existing generated workflow YAML and bindings artifacts remain pinned by golden tests.
- `dockpipe pipelang invoke` remains pure and CLI-only in the legacy lane.
- `dockpipe run` continues consuming compiled workflow/YAML behavior rather than parsing PipeLang.
- New semantic/Application/Service projections are additive and independently versioned; they do
  not silently change current catalog or launcher fields.
- Parser, binder, typechecker, HIR/Core lowering, CLI, canonical docs, projection schemas, editor,
  backends, and compatibility fixtures advance together for each shipped feature.

## Explicitly Unresolved Founder/Product Choices

These choices do not block the accepted foundation and must not be guessed by an implementation
slice:

- the public name and release number for the first post-`v0.0.0.1` language contract;
- final keyword, casing, attribute, module/import, entrypoint, action/effect, contract, and test
  spellings after grammar prototypes are reviewed;
- whether official compiler distributions are source-only, source plus signed stage artifacts, or
  source plus reproducible toolchain images;
- the support/retirement lifetime of the Go stage-0 recovery compiler after self-hosting is proven;
- the first concrete constrained and MCU platforms, their numeric/memory limits, and qualification
  tier; and
- standard-library release cadence and which optional domains (decimal, time, localization) ship in
  the first full profile.

No resolution may introduce manual memory, hidden effects, C# compatibility claims, target syntax,
backend semantic drift, nondeterministic compilation, or implicit migration. A choice that would do
so reopens this accepted packet and requires a founder decision.

## Bounded Implementation Order

1. Freeze all 45 current files and current emitted artifacts as the `v0.0.0.1` compatibility lane.
2. Introduce source-set/file identities, strict UTF-8, full spans, and structured diagnostics without
   adding syntax.
3. Replace string-only type plumbing with structured unresolved/resolved `TypeRef` and create one
   symbol table with explicit ownership and declaration spans, still preserving legacy behavior.
4. Add explicit module/import/dependency-lock binding and deterministic resolution before any broad
   declaration or expression expansion.
5. Add stable semantic IDs end to end through diagnostics and a versioned semantic projection;
   keep local implementation symbols ID-optional.
6. Establish typed HIR and target-neutral Core IR with one tiny pure executable function and the
   deterministic Go backend; prove no direct parser-to-Go emission.
7. Add fixed numeric/text/value/equality/hash/order semantics plus optionals, results, records,
   unions, and deterministic collections in coherent vertical slices.
8. Add blocks, locals, branches, bounded loops, matching, and functions sufficient for the compiler
   subset, with negative flow/ownership/profile tests.
9. Add inert effects, executable entrypoints, contracts, actions/state, replay, and first-class
   testing on the established compiler representations.
10. Publish the shared semantic/Core projection builders, then let TASK-020 and TASK-022 define
    separately versioned Application IR and Service IR specializations without reparsing.
11. Implement the compiler in the accepted PipeLang subset, complete the minimum self-hosting
    library, and prove stage-0/stage-1/stage-2 reproducibility.
12. Validate full, constrained, and MCU profiles with resolver capability manifests and explicit
    unsupported-feature diagnostics before widening libraries or adding another backend.

Each slice is independently reviewable and keeps syntax, semantics, diagnostics, projection,
editor, tests, and any enabled backend synchronized. No permissive parser, target-owned semantics,
or syntax-first feature batch may skip the earlier foundations.

## Validation Matrix

Every shipped language slice needs proportionate coverage across:

- lexer/parser positive, malformed, ambiguity, recovery, and source-span cases;
- semantic-ID malformed, duplicate, rename/move stability, compatibility, and metadata cases;
- effect inference/composition, undeclared widening, pure/effectful call, authority, and inertness
  cases;
- requires/ensures/invariant/refinement positive, violation, composition, and compatibility cases;
- deterministic replay identity, injected nondeterminism, sensitive-value redaction, and exact seed
  cases;
- property generation, invalid/boundary generation, shrinking, replay, and contract-derived cases;
- state-machine legal/prohibited/terminal/unreachable transition and bounded path-generation cases;
- typechecker positive and negative cases, including optionals, collections, branches, actions, and
  effect purity boundaries;
- deterministic evaluator and computed-dependency behavior;
- compiler golden artifacts and repeat-build identity;
- old-version compatibility fixtures;
- CLI compile/invoke/materialize behavior and help;
- catalog/workflow projection compatibility;
- VS Code/Cursor syntax, completion, hover, and diagnostics;
- semantic graph/impact query and independently verified change-manifest cases;
- TASK-020 Application IR and TASK-022 Service IR integration fixtures with no target-specific
  language symbols;
- Core IR conformance and differential results for each enabled backend/profile; and
- stage-0/stage-1/stage-2 normalized artifact, diagnostic, behavior, and digest equality.

Run at minimum the focused `src/lib/pipelang` and application PipeLang tests for implementation
slices, then the broader engine/package validation required by the touched public surfaces.

## Acceptance Criteria

- The language decisions above are explicit before production syntax is accepted.
- Existing PipeLang behavior and artifacts remain compatible or migrate through an explicit version.
- Stable semantic IDs survive file moves and symbol renames, reject duplicates, and appear
  consistently in diagnostics, metadata, tests, graphs, and compatibility analysis without being
  forced onto local implementation details.
- Public effect/authority contracts are machine-readable, compositional, enforceable, and cannot be
  silently widened through calls or target generation.
- Contracts/invariants/refinements have deterministic enforcement and reusable normalized metadata.
- Pure code cannot observe nondeterministic or external state without an explicit effect/input.
- Seeded property failures and deterministic executions can be replayed exactly within the declared
  compatibility and sensitive-data boundary.
- Enums, records, optionals, collection values, computed properties, reactive state, bounded actions,
  governed effects, and safe expressions form one coherent type system rather than isolated syntax.
- Pure compilation/evaluation remains deterministic, offline, and side-effect-free.
- Effect declarations cannot execute during compile, catalog, editor, render, or target build setup.
- Workflow/package/runtime/resolver/strategy authority remains the only external execution path.
- PipeLang contains no Qt, HTML, CSS, QML, JavaScript, C++, CMake, or WASM semantics.
- AST/parser/typechecker/evaluator/compiler/CLI/docs/editor support stay synchronized.
- Diagnostics carry stable source locations through the semantic projection.
- The semantic graph produces dependency-kind-aware impact results rather than text-match claims.
- Change manifests are independently verified and never treated as trusted agent assertions.
- External model invocation is an explicit typed resolver-backed effect and is absent from ordinary
  programs unless declared.
- TASK-020 can consume the projection without parsing `.pipe` files independently.
- One minimal application fixture compiles through both semantic-web and Qt target fixtures with
  equivalent typed state, action, validation, and effect semantics.
- PipeLang is capable of expressing its compiler and minimum library, and the staged Go-seeded
  bootstrap contract is reproducible.
- Numeric, Unicode text, equality, hashing, ordering, value/reference, nullability, failure,
  mutation, collection, memory, module, distribution, and entrypoint semantics are target-neutral
  and fail rather than degrade on unsupported profiles.
- Typed HIR, Core IR, the public semantic projection, Application IR, and Service IR remain distinct
  layers over one parser/binder/type contract.

## Next Bounded Production Slice

Implement only steps 1-2 of **Bounded Implementation Order**: freeze the exact legacy inventory and
introduce source-set/file identities, strict UTF-8 decoding, durable spans, and structured ordered
diagnostics across the existing lexer/parser entrypoint and its CLI/editor consumers. Do not add
modules, semantic IDs, new types/declarations/expressions, HIR/Core IR, effects, backends, or syntax
in that slice. It requires a separate implementation authorization; this accepted design record
does not grant it.
