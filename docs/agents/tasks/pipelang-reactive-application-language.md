# TASK-021 PipeLang Deterministic Self-Hosting Managed Language Foundation

## Decision Status

The `vNext` foundation decision packet in this record is **accepted** as of 2026-08-16. Acceptance
fixes semantics, compiler boundaries, bootstrap stages, compatibility, and implementation order;
examples and fixtures remain non-normative and accept no production syntax. Separately authorized
bounded objectives have completed implementation-order steps 1 through 6. This record does not by
itself authorize step 7 or any later language slice.

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

This is the durable design and bounded-progress record. It does not authorize additional parser,
lexer, AST, typechecker, evaluator, compiler, CLI, schema, catalog, editor-extension,
generated-artifact, or runtime changes beyond an explicitly granted implementation objective.

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

Founder review on 2026-08-16 superseded the earlier per-declaration `[Id(...)]` direction. Public
identity is initially derived from an explicit package identity, explicit namespace, declaration
ownership, and the declaration's source name. Ordinary declarations do not repeat an authored ID.
Once a public projection crosses an explicit compatibility baseline, centralized source-controlled
migration metadata preserves its established identity across renames or moves; the compiler never
guesses continuity from similar names or paths. Local implementation declarations remain ID-free.

The accepted identity contract is:

- package ids, namespaces, and expanded semantic paths use lowercase dotted ASCII segments;
- identity-bearing source names begin with an ASCII letter, continue with ASCII letters or digits,
  and convert uppercase ASCII directly to lowercase without punctuation stripping, Unicode
  normalization, locale, numbering, or collision repair;
- initial type/member paths derive deterministically from namespace, stable owner identity, and
  source name; new members of a renamed owner continue beneath that owner's preserved identity;
- public declarations carry compatibility-stable identities; internal declarations carry
  deterministic package-and-namespace-scoped tooling identities without a public compatibility
  promise; private/local declarations use analysis-local identities only;
- `internal` access means the same package identity and namespace, never matching namespace text in
  another package; public cross-package access still requires an explicit import and locked
  dependency;
- full cross-package identity keeps `package_id` and semantic path as separate structured fields;
  package version and content digest remain dependency-lock facts rather than identity components;
- callable identity is structured as method-group path plus ordered resolved parameter types and
  resolved return type. Parameter names are not identity. This reserves deterministic overload
  identity, including return-context overloads, without encoding signatures into dotted strings or
  enabling overload syntax/resolution in step 5;
- before a projection is explicitly accepted as a compatibility baseline, draft identities and
  migration history may be corrected freely. After baselining, public renames/moves require an
  explicit migration record, former public names remain deprecated aliases until an explicitly
  breaking release, and published identity replacement preserves the old identity as an alias;
- baselined removals, visibility narrowing, public field/type/signature changes, parameter reorder,
  required interface additions, identity reassignment, and promised-alias removal are breaking;
  additions, private/internal refactors, deprecation, and identity-preserving rename/move are
  compatible;
- migration and compatibility inputs are complete offline compiler inputs, are hashed into the
  lock, reject cycles/conflicts/missing targets, and project canonical current identity, former
  names, aliases, deprecations, removals, and source relationships for authorized consumers; and
- diagnostics and semantic projections carry structured identities. A compiler may calculate an
  internal canonical lookup hash, but no hash or analysis-local symbol number becomes public
  identity.

The source spelling for namespaces, imports, package metadata, and migration records remains a later
grammar/package-contract decision. The reviewed direction is a single namespace declaration before
imports with centralized package-level migration history, not declaration annotations. Step 5 uses
only structured compiler input and adds no production syntax.

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

    public Action SelectDeployment(string id)
        requires Deployments.Any(item => item.Id == id)
    {
        SelectedDeploymentId = id;
    }

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
| Semantic IDs | Initial public identity derives deterministically from separate package id, explicit namespace, stable owner identity, and ASCII source name; ordinary declarations do not author duplicate IDs. Baselined public changes require centralized alias/deprecation/migration records. Callable identity adds ordered resolved parameter and return types. Internal identities are package-and-namespace-scoped tooling identities; private/local symbols remain non-public compiler identities. |
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
| Types | Structured unresolved/resolved `TypeRef` for existing primitive, named, and nested `List<T>` spellings; named refs carry analysis-local symbol identity | Optional/result/function types, generic constraints, capabilities, and profile validation on the same structured graph |
| Symbols/modules | One deterministic frozen-legacy sibling-source-set symbol table with one owner and declaration span per interface/class | Explicit module/import graph, module-aware ownership/visibility, ambiguity diagnostics, and dependency lock |
| Spans on AST | No AST node stores a source span | Spans on declarations, names, types, members, expressions, statements, contracts, effects, and lowered source maps |
| Expressions | Literal, identifier, unary, binary, parentheses | Member/call/index/conditional/interpolation, typed match, bounded closures, collection construction/operations, conversions, and optional handling |
| Statements/control flow | Expression-bodied methods only | Blocks, locals, assignment, branches, loops needed for self-hosting, return/match, and explicit action transition boundaries |
| Runtime values | `Value` stores only string/int64/float64/bool | Full target-independent value model, fixed numeric families, bytes/scalars, records/unions/options/results, collections, and managed identities |
| Type checking | One symbol table and resolved type graph drive legacy conformance plus primitive expression inference | Module-aware bound-symbol checking, ownership/flow/nullability, generic constraints, effects/contracts, exhaustiveness, and backend/profile capability checks |
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

The accepted public identities are `PipeLang` for display, `pipelang` for machine-readable use,
`v0.1.0` for the first explicit post-legacy language contract, `pipelang.compiler.v1` for the
compiler contract, and `pipelang.semantic.v1` for the public semantic projection. Language,
compiler, and projection versions advance independently. `v0.0.0.1` remains frozen and no package
selects `v0.1.0` implicitly.

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

## Completed Bounded Production Slices (2026-08-16)

The separately authorized steps 1-4 slices are implemented. Steps 1-2 froze the exact legacy inventory and
introduced source-set/file identities, strict UTF-8 decoding, durable spans, and structured ordered
diagnostics across the existing lexer/parser/compiler entrypoint and direct CLI, catalog,
materialize, package-compile, and editor consumers. Step 3 replaced string-only static type plumbing
with structured unresolved/resolved references and one declaration symbol table. Step 4 added the
syntax-independent explicit module/import/dependency-lock binder while leaving the frozen parser and
every production consumer on `v0.0.0.1`. These slices added no public module/import syntax, public
semantic IDs, new types/declarations/expressions, HIR/Core IR, effects, or backends.

### Step 1 compatibility checkpoint (2026-08-16)

The separately authorized production slice established the frozen `v0.0.0.1` baseline before
source/span plumbing. `tests/pipelangcompat/legacy_compat_test.go` proves that the fixture is the
exact 45-file authored inventory, every file retains its parsed declaration/member/default
projection, every public class retains its workflow YAML and bindings JSON/env artifacts, and the
existing expression-bodied method retains its pure evaluation result. Existing focused compiler,
application CLI, materialize, catalog, and editor fixtures remain the direct consumer coverage.
The checkpoint is intentionally repository-level so the generic `src/lib` compiler does not learn
checkout-only package or workflow paths.

### Step 2 source/span/diagnostic checkpoint (2026-08-16)

`src/lib/pipelang` now admits deterministic path-identified source sets, rejects invalid UTF-8,
retains half-open file-aware byte spans on every token and existing AST node, resolves Unicode and
UTF-16 editor positions, and returns ordered diagnostics with stable code/category/severity plus
primary and optional related spans. `ParseFiles` preserves parse-only consumers while
`AnalyzeFiles` is the shared parse/type contract for compilation and diagnostics. `dockpipe
pipelang check` renders that contract as text or schema-1 JSON without evaluation or artifact
emission, and the VS Code extension consumes the JSON contract for unsaved buffers through stdin.

### Step 3 structured type/symbol checkpoint (2026-08-16)

The completed step-3 slice replaced parser-created type strings with structured unresolved
primitive, named, and applied `List<T>` references, including nested applications and exact source
spans. One deterministic symbol-table contract now owns both legacy interfaces and classes under an
explicit frozen sibling-source-set owner; every declared symbol has one analysis-local identity and
declaration span. Focused checks prove named resolution, cross-kind duplicate detection, unknown-type
locations, and conformance primary/related type spans. Type checking and invocation use resolved type
graphs; compiler, evaluator, CLI, catalog, materialize, package-compile, and editor projections retain
the frozen spellings and artifacts. The exact 45-source compatibility golden and affected offline
consumer/editor checks pass. Analysis-local symbol IDs are deliberately non-semantic and are not
persisted into legacy artifacts.

### Step 4 module/import/dependency-lock checkpoint (2026-08-16)

The completed step-4 slice introduced `ModuleSetInput` and `AnalyzeModuleSet` as the explicit,
syntax-independent compiler seam for a non-legacy language contract, root module, complete source
bytes, structured module/symbol imports, and a complete dependency lock. Locked module records carry
direct dependency identities and a deterministic SHA-256 digest over normalized source identities
plus exact bytes; missing or changed inputs diagnose and compilation never fetches. One symbol table
now supports both the frozen legacy owner and deterministic module-aware owners, allowing equal local
names in different modules while retaining one analysis-local symbol identity, visibility, owner,
and declaration span per declaration. Explicit symbol imports resolve unqualified names; explicit
module imports authorize qualified structured type references; no wildcard, relative, ambient, or
implicit cross-module lookup exists.

Focused tests cover input-order-independent symbol/lock queries, cross-module conformance, module and
symbol import resolution, lock drift, duplicate module owners, undeclared dependencies, unknown and
private imports, ambiguous symbol imports, import cycles, durable primary/related spans, and rejection
of selecting the frozen legacy contract through the module lane. The exact 45-source compatibility
golden plus affected compiler, CLI, catalog, materialize, package-compile, and editor checks retain
their prior behavior and artifacts. This slice deliberately chooses no public post-legacy language
name/release, module/import keyword or casing, manifest/YAML/schema shape, CLI selector, or editor
grammar; those unresolved founder/product spellings remain unguessed.

### Completed step 5 semantic identity/projection checkpoint (2026-08-16)

The first syntax-independent implementation checkpoint established a distinct `SemanticID`,
analysis-local `SymbolID`, semantic lock digest, diagnostic propagation, and deterministic semantic
projection. It used explicit caller-supplied ID-to-span assignments and caller-supplied contract
identities. Founder review subsequently accepted the public contract identities and superseded that
assignment model with the derived-and-baselined identity contract above. The initial code and tests
are preserved as predecessor work but are not the final step-5 contract.

The completed checkpoint replaced missing-explicit-ID behavior with deterministic
package/namespace/name derivation, reserved structured callable identity over ordered resolved
parameter and return `TypeRef`s, represented centralized compatibility/migration input without
production syntax, fixed the public contract constants, and retained deterministic structured
diagnostics and projection bytes. Migration records retain every promised former name, canonicalize
caller ordering into the semantic lock, reject missing/conflicting targets and identity cycles, and
make former public top-level names resolve as deprecated aliases through the one symbol table.

The public `pipelang.semantic.v1` projection is canonical deterministic JSON with separate
`package_id` and semantic path fields, structured named/applied/callable identities, declaration
source ranges, source and semantic lock digests, fixed compiler/language contracts, and explicit
public/workspace views. The public view excludes private declarations and rejects absolute source
paths; the workspace view may retain local source details. Private declarations remain
analysis-local and identity-optional. No timestamp, host path, package version, dependency content
digest, or analysis-local `SymbolID` becomes semantic identity.

Focused proof passed with cached Go 1.25.13 and `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
GOWORK=off` plus writable `/tmp` caches:

- `go test ./src/lib/pipelang ./tests/pipelangcompat`;
- exact equality of the frozen 45-path inventory and current authored `.pipe` inventory, including
  unchanged source, language-projection, emitted-artifact, and evaluation goldens;
- affected PipeLang application/CLI/catalog/materialize/package-compile tests using only the
  already-cached temporary `golang.org/x/sys v0.46.0` substitute because the checkout-pinned v0.28
  source is absent from the read-only module cache; no checkout dependency file changed;
- `go vet` over PipeLang and the affected internal consumers; and
- the existing VS Code extension validation.

The broad `src/lib/application` suite remains non-authoritative for this slice because its unrelated
`TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap` fixture names
`/path/to/your/project`; the focused affected application suite passes. The frozen legacy lane,
emitted artifacts, generated stores, dependency files, and protected ignored bytes did not change.

### Completed step 6 typed HIR/Core/Go checkpoint (2026-08-16)

The separately authorized step-6 implementation checkpoint adds three cohesive generic compiler
packages: target-independent typed HIR, normalized target-independent Core IR, and the first Go
backend. `LowerSemanticMethodToHIR` accepts only a successful `v0.1.0` semantic module analysis and
one public method semantic identity. The resulting HIR keeps the owning module and analysis-local
class symbol, stable owner and callable semantic identities, resolved types, parameter bindings,
and durable declaration/type/expression spans. No target spelling or Go representation enters HIR.

`LowerHIRToCore` removes source spans and analysis-local symbols while preserving the callable
identity, ordered typed signature, parameter positions, typed literals/references, and normalized
existing unary/binary operators. The Go backend imports Core IR only, validates the fixed language
and compiler contracts, orders functions deterministically, and emits formatted dependency-free Go
for the primitive capability slice. It rejects missing identities, duplicate target names, named or
applied types, division, and logical operators rather than silently choosing Go behavior before
step 7 fixes the relevant failure and evaluation-order semantics.

The first vertical fixture uses only existing expression-bodied method syntax:
`Ready(int count) => count > 0`. HIR, Core, and generated-Go goldens prove the representation
boundaries. The generated Go compiles and executes only under `/tmp`; its result matches the frozen
pure evaluator for the same source and input. A source-architecture test rejects any Go-backend
import of the PipeLang parser, AST/compiler root, HIR, `go/parser`, or `go/ast`, so a direct
parser/AST-to-Go path fails the focused suite. HIR and Core lowering failures remain ordered
PipeLang diagnostics with durable spans and semantic identities; unsupported Go capabilities use
stable backend errors.

Terminal proof passed with the cached Go 1.25.13 toolchain, the required offline environment, and
writable `/tmp` caches:

- focused HIR/Core/backend/diagnostic goldens plus exact
  `go test ./src/lib/pipelang/... ./tests/pipelangcompat`;
- compile and execution of generated Go only in a temporary `/tmp` module, with the same result as
  the existing pure evaluator;
- affected application PipeLang compile/check/invoke, catalog, materialize, workflow/package compile,
  and CLI package checks using only a temporary `/tmp` modfile pinned to the already-cached
  `golang.org/x/sys v0.46.0` source; checkout dependencies did not change; and
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages.

The exact 45-source inventory and its source, language-projection, artifact, and evaluation goldens
did not drift. The semantic projection tests pass unchanged. This checkpoint changes no production
syntax, CLI/YAML/schema/editor surface, generated store, runtime, or external state.

### Completed step 7a fixed numeric comparison/equality checkpoint (2026-08-16)

The first separately bounded step-7 slice makes executable numeric representation explicit without
adding source syntax or changing stable semantic identities. Existing source `int` lowers to a
target-independent signed two's-complement 64-bit HIR/Core type; existing source `float` lowers to
an IEEE-754 binary64 HIR/Core type. The Go backend consumes those Core shapes directly and no longer
infers width, signedness, or a mixed numeric conversion from source primitive names.

The `v0.1.0` semantic module lane accepts the existing comparison and equality operators only when
both numeric operands have the same resolved type. Mixed integer/float comparison or equality fails
with deterministic structured diagnostic `PL3028`; the frozen legacy source-set lane retains its
existing behavior. Generated binary64 comparison/equality is compiled and executed under `/tmp` and
matches the existing pure evaluator for ordinary values, unordered and unequal `NaN`, and equal
positive/negative zero. Malformed mixed-type Core operands fail in the backend instead of being
coerced.

Numeric addition, subtraction, multiplication, division, and negation remain fail-closed in both
the `v0.1.0` checker and Go backend. Enabling them would require the accepted checked-overflow and
recoverable-failure contract, so neither layer substitutes unchecked target arithmetic or a panic
before typed `Result` failure semantics exist. This slice changes no syntax, semantic projection,
legacy artifact, public YAML/schema/editor surface, generated store, runtime, or dependency.

### Completed step 7b compiler-internal arithmetic Result checkpoint (2026-08-16)

The next bounded step-7 foundation adds structural `Result<Success, ArithmeticError>` types to typed
HIR and Core without selecting production source spelling. Core owns one target-independent checked
arithmetic signature and validation contract: signed 64-bit addition, subtraction, multiplication,
and negation produce an integer result or `overflow`; binary64 division produces a binary64 result
or `division_by_zero` for positive or negative zero. Other operand widths, signedness, implicit
numeric conversions, nested fallible expressions, and error families fail closed.

An inert `coreeval` package consumes Core IR only and serves as the semantic conformance evaluator.
The Go backend consumes the same validated Core contract and emits explicit generic result values
with stable `overflow` and `division_by_zero` errors; it never converts a target panic into a domain
result. Exact HIR/Core/Go goldens cover checked addition. Boundary tests compare the centralized
integer implementation with exact `math/big` results and prove evaluator/generated-Go agreement for
success, every supported overflow family, negating the minimum integer, division by both signed
zeros, and IEEE `NaN` propagation. Architecture tests prohibit both `coreeval` and the Go backend
from importing parser, AST/compiler-root, HIR, or each other.

The existing `v0.1.0` source checker intentionally continues to reject numeric arithmetic with
structured `PL3028`: a source method declared to return `int` or `float` cannot silently become a
fallible method. The frozen legacy evaluator and artifacts remain unchanged. This checkpoint adds no
production `Result`/`ArithmeticError` spelling, implicit unwrap, branches or matching, public
semantic projection, YAML/schema/editor surface, runtime, generated store, or dependency.

### Completed step 7c public arithmetic Result contract (2026-08-16)

The founder explicitly accepted the first production-source arithmetic result contract as a
versioned migration from the fail-closed seed. The new language contract is `v0.2.0`; `v0.1.0`
remains supported and continues to reject numeric arithmetic with `PL3028`. No source file, module,
or package selects `v0.2.0` implicitly. The display and machine names remain `PipeLang` and
`pipelang`, and the independently versioned compiler and public projection contracts remain
`pipelang.compiler.v1` and `pipelang.semantic.v1`.

The accepted source spelling is
`Result<int, ArithmeticError> Add(int left, int right) => left + right;`. `Result` is the language
built-in type constructor with semantic identity `pipelang:result`; `ArithmeticError` is the closed
language built-in failure type with semantic identity `pipelang:arithmetic.error`. Callable identity
and semantic projection use the existing structured applied-type shape: the identified `Result`
constructor carries ordered success and failure arguments, and the failure argument carries the
identified `ArithmeticError` type. This is not authority for a general Result library or
user-defined generic declarations.

Checked arithmetic already has the declared Result type; there is no conversion or implicit
wrapping. In this bounded slice, the only admitted handling is returning one checked integer
addition as the complete body of an expression-bodied method whose declared return is exactly
`Result<int, ArithmeticError>`. No unwrap, propagation, nesting, extraction, matching, block,
branch, or use as `int` is admitted. The observable outcomes are an explicit success value or the
closed `overflow` error. Other arithmetic operators and the already-proven
`division_by_zero` identity remain compiler-internal until separately synchronized source slices.

The synchronized implementation gates the grammar on an explicitly selected `v0.2.0` module while
the legacy parser and `v0.1.0` semantic lane retain their prior behavior. The parser preserves full
type and argument spans; resolution fixes both built-in identities; callable identity and
`pipelang.semantic.v1` project the identified applied type without an analysis-local symbol. The
typechecker rejects ordinary integer returns, nesting, other arithmetic, other Result arguments,
Result fields/parameters/interfaces, bare arithmetic errors, and declarations that shadow the two
language-owned names. The accepted method lowers from checked source through typed HIR and Core,
evaluates through `coreeval`, and generates deterministic compilable Go with identical success and
overflow outcomes.

Terminal proof passed with cached Go 1.25.13, the required offline environment, and writable `/tmp`
caches:

- exact `go test ./src/lib/pipelang/... ./tests/pipelangcompat`, including parser/type/identity/
  projection/diagnostic coverage, source-derived HIR/Core/Go goldens, Core evaluation, and generated
  Go compilation/execution under `/tmp`;
- focused application PipeLang check/compile/invoke, catalog, materialize, representative workflow/
  package compile, internal materialize/package-compile, and `src/cmd` tests through only the
  temporary `/tmp` modfile pinned to cached `golang.org/x/sys v0.46.0`; checkout dependencies did
  not change;
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages; and
- VS Code extension diagnostics plus durable Result/ArithmeticError grammar and completion checks.

The broad application suite had only its two unrelated sandbox/topology failures: loopback listen
is prohibited for `TestCmdInstallCoreEmitsOperationResults`, and
`TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap` names nonexistent
`/path/to/your/project`. The focused affected suite is green. The exact 45-source compatibility
inventory, legacy artifacts/evaluation, dependencies, generated stores, and protected ignored bytes
did not change.

### Completed step 7d direct checked-subtraction contract (2026-08-17)

The founder selected the recommended new-version option for the next smallest public arithmetic
slice. `v0.3.0` is an explicit migration that preserves `v0.2.0` as add-only and additionally admits
exactly
`Result<int, ArithmeticError> Subtract(int left, int right) => left - right;`. `v0.1.0` remains
fail-closed with `PL3028`, and no source file, module, or package selects `v0.3.0` implicitly.

The subtraction is the complete body of one expression-bodied class method. Its two operands are
exactly `int`; its expression already has the declared `Result<int, ArithmeticError>` type; and its
only observable outcomes are an explicit integer success or the existing closed `overflow` error.
There is no conversion, wrapping, unwrapping, propagation, nesting, extraction, matching, block,
branch, or use as an ordinary integer. Multiplication, negation, binary64 division, and general
Result handling remain compiler-internal.

The source contract reuses the language-owned `pipelang:result` and
`pipelang:arithmetic.error` identities. Callable identity remains the structured ordered signature
`(int, int) -> Result<int, ArithmeticError>`, and the public compiler/projection contracts remain
`pipelang.compiler.v1` and `pipelang.semantic.v1`. Typed HIR and Core preserve the selected
subtraction operator and target-independent Result type; Core evaluation and the Go backend consume
that same contract rather than redefining failure behavior.

The synchronized implementation explicitly gates `v0.3.0` through parsing, type resolution and
checking, semantic callable identity and `pipelang.semantic.v1`, typed HIR, target-neutral Core,
`coreeval`, and the deterministic Go backend. Source-derived HIR/Core/Go goldens preserve the
subtraction operator and identified Result return. Core evaluation and compiled generated Go agree
for ordinary and negative successes plus both signed overflow directions. Negative diagnostics
prove `v0.1.0` and `v0.2.0` do not accept subtraction, `v0.2.0` remains add-only, and `v0.3.0`
rejects nesting, multiplication, negation, division, alternate Result arguments, and unsupported
Result placements.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` build and
temporary caches:

- exact `go test -count=1 ./src/lib/pipelang/... ./tests/pipelangcompat`;
- focused application PipeLang/check/compile/invoke, catalog, representative workflow/package
  compile, internal materialize/package-compile, and `src/cmd` tests through only the existing
  temporary modfile pinned to cached `golang.org/x/sys v0.46.0`;
- `go vet` across PipeLang, compatibility, and all affected application/internal/CLI packages;
- VS Code extension diagnostics/completion/grammar validation, including subtraction highlighting;
  and
- `gofmt`, `git diff --check`, exact 45-source inventory and compatibility goldens, dependency-diff,
  branch/stash, and protected ignored-byte proof.

The previously admitted unrelated broad-application loopback and nonexistent-fixture failures were
not reopened; the complete affected set is green. No generated store, dependency, runtime,
credential, external state, commit, or publication changed. No later step-7 operator or
Result-composition rule is authorized by this checkpoint.

## Exact Next Boundary

Step 7 of **Bounded Implementation Order** is underway through the completed fixed numeric,
compiler-internal checked-arithmetic Result, first production-source checked integer-add, and
accepted direct checked-subtraction slices. `v0.2.0` admits only the exact explicit Result-returning
addition recorded above; `v0.3.0` preserves that addition and additionally admits only the exact
direct Result-returning subtraction recorded above. All other numeric arithmetic remains
fail-closed from production source. Any next slice requires a new synchronized decision for its
exact source spelling, Result handling/composition rule, semantic projection, migration, and
bounded semantics before implementation.

No later step-7 slice is included here. In particular, this checkpoint does not add Unicode text,
value/reference, hashing, total ordering, optional, general result, record, union, or deterministic
collection production semantics; accept namespace, import,
migration, `internal`, overload, generic, or ID production syntax; implement overload resolution;
add new types/declarations/expressions/operators, blocks, locals, branches, or loops; add effects,
entrypoints, actions/state, contracts/replay, executable application/service semantics, Application
IR, Service IR, another backend, or self-hosting; mutate generated stores; or widen Go emission by
guessing step-7 semantics. Exact successor production spellings remain later synchronized language
slices.
