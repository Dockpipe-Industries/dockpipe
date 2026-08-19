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
[`compiler-contract.v1.json`](../fixtures/pipelang-vnext/compiler-contract.v1.json),
[`bootstrap-reproducibility.v1.json`](../fixtures/pipelang-vnext/bootstrap-reproducibility.v1.json), and
[`semantic-verification.v1.json`](../fixtures/pipelang-vnext/semantic-verification.v1.json) make these
boundaries executable as future fixture assertions without accepting syntax or schemas.
