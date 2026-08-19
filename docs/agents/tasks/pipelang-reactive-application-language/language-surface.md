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

### Source-level debugging contract

PipeLang is agent-friendly because ordinary deterministic programs are compact, explicit, typed,
and machine-inspectable. Agents remain external language/tooling clients; no model, prompt, or
agent runtime is built into ordinary language semantics.

Source-level debugging requires more than backend symbols. The compiler/tooling contract must
eventually provide:

- stable generated symbol names derived from semantic identities;
- complete source maps from `.pipe` spans through typed HIR, Core, specialized IR, and generated
  target locations;
- resolver-produced native debug information plus a reversible symbol/location manifest;
- LSP navigation, diagnostics, completion, rename, and type inspection through compiler queries;
- DAP breakpoint, stepping, stack, watch, and exception mapping at PipeLang source locations;
- typed value projection that presents PipeLang values rather than target implementation layouts;
- semantic breakpoints resilient to generated line movement;
- deterministic, sanitized failure bundles with relevant source hashes/spans, semantic identities,
  module/dependency evidence, stack/value snapshots, resolver/toolchain identities, and redacted
  capability results;
- read-only structured queries for frames, values, provenance, generated locations, and semantic
  dependencies; and
- differential Core/backend evidence that localizes semantic, resolver, or target-runtime drift.

TASK-021 owns target-neutral spans, identities, value projection, Core evidence, and debug metadata.
TASK-020/TASK-022 and target resolvers own Application/Service/native mappings. IDE and structured
agent clients consume the same versioned evidence; neither scrapes console text as the canonical
debug contract. Replay and effect traces remain separately gated later slices and must not be
inferred by this debugging direction.

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

