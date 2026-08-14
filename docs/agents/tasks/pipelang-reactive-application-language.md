# TASK-021 PipeLang Deterministic Semantic And Application Language Foundation

## Goal

Evolve PipeLang from its current optional typed configuration/model layer into a bounded,
target-neutral language foundation for deterministic programs, AI-assisted change analysis,
automated verification, and declarative DockPipe applications.

PipeLang should own types, validation, reactive state, pure computed values, typed state actions,
stable semantic identities, explicit effects and authority, contracts/invariants, deterministic
replay metadata, governed effect declarations, test-generation metadata, and safe binding
expressions. It compiles those semantics into a versioned representation consumed by tooling and by
the Application IR and target builders owned by
[TASK-020](declarative-application-surfaces-and-target-builders.md).

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

1. stable semantic identities;
2. explicit effects and authority;
3. first-class constraints, contracts, and invariants; and
4. foundations for generated property tests and deterministic replay.

Broader reactive-state and application-language features build on those foundations rather than
landing as one uncontrolled rewrite.

## Language Personality

PipeLang remains a focused C# descendant:

- familiar braces, declarations, attributes, expressions, generics, and type spelling;
- strong static typing and ordinary code readable by C# developers;
- minimal punctuation and ceremony with clear source-located compiler diagnostics;
- deterministic semantics and explicit authority;
- no YAML-like indentation/nesting inside `.pipe` files;
- no Lisp-like, academic, or unnecessarily exotic surface syntax; and
- no magical AI syntax embedded through normal programs.

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

PipeLang is a typed authoring and application compiler, not a second DockPipe execution engine or a
general-purpose systems language.

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

## Required Language Surface

### Stable semantic identities

Externally referenced declarations need optional explicit identities that survive file moves and
symbol renames. A C#-style attribute is the leading direction because PipeLang already supports
annotations:

```csharp
[Id("deploy.production")]
public Action Deploy(Release release);
```

The design must define:

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

Actions are explicit, deterministic application-state transitions. The design must settle:

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

Each effect declaration should eventually bind:

- stable capability/workflow/package identity;
- typed input, output, and closed error/result contracts;
- required authorization/capability metadata;
- cancellation, progress, retry, and idempotency posture where the underlying operation supports it;
- explicit loading/success/error state transitions; and
- source information sufficient for policy and user-facing diagnostics.

The first design must decide whether effects are declarations invoked by actions, generated action
forms, or a separate application binding. No syntax is accepted until it proves that catalog/editor
analysis remains inert and execution still crosses normal DockPipe authority boundaries.

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

The design must specify:

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

The design must define:

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
contracts, and transition metadata rather than a separate unrelated DSL. The semantic model should
eventually represent:

- legal, guarded, terminal, and prohibited transitions;
- state and transition invariants;
- unreachable states and contradictory guards;
- transition semantic IDs and effects/authority;
- deterministic transition traces and replay; and
- generated positive, negative, and path-coverage tests.

The design must determine when an ordinary action becomes an explicitly modeled transition and how
the compiler avoids state-space explosion in generated tests.

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

PipeLang should provide typed semantics consumed by YAML/Application IR without absorbing layout or
target rendering. It needs:

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
`SelectDeployment`, and `Refresh`, but the view compiler must consume the typed projection rather
than reimplementing PipeLang parsing.

## Decisions Required Before Parser Work

| Decision | Required outcome |
| --- | --- |
| Language versioning | explicit version compatibility and artifact schema policy beyond `v0.0.0.1` |
| Semantic IDs | eligible/required declarations, syntax, uniqueness, compatibility, aliases, and metadata projection |
| Nullability | one explicit optional/absence model with no implicit null widening |
| Value versus state identity | records/value equality and mutable state-owner rules |
| Mutation | deterministic assignment/collection update and notification semantics |
| Computed graph | dependency discovery, cycle rejection, evaluation order, and caching contract |
| Collections | minimum constructors, operations, lambda restrictions, ordering, and equality |
| Errors/results | closed typed result model that maps consistently to all targets |
| Actions | bounded body semantics and purity enforcement |
| Effects | inert declaration, authority boundary, async lifecycle, and result application |
| Effect composition | inference, explicit public declarations, call-graph propagation, abstraction, and widening diagnostics |
| Contracts | requires/ensures/invariants/refinements, `old`/result scopes, enforcement, composition, and compatibility |
| Determinism/replay | purity boundary, injected nondeterminism, replay record, seeds, sensitive data, and version identity |
| Property tests | generator, invalid/boundary cases, shrinking, deterministic replay, and contract-derived tests |
| State machines | transition declaration/model, guards, terminal/prohibited states, invariants, and bounded path generation |
| Intent metadata | optional vocabulary, query semantics, non-authority, and compatibility expectations |
| Semantic graph | node/edge schemas, stable IDs, source locations, incremental emission, and impact queries |
| Change manifests | external schema, independent verification, claim statuses, and effect/contract/test comparison |
| Model invocation | explicit typed `ExternalModel` effect, resolver ownership, validation, authority, and replay posture |
| Modules | imports/namespaces, duplicate identities, visibility, and deterministic resolution |
| Compatibility | behavior of existing `.pipe` files, generated artifacts, CLI invoke, and materialization |
| Diagnostics | stable source spans and error categories shared by CLI, catalog, and editor tooling |

## Compatibility And Migration

- Existing `v0.0.0.1` files must continue compiling without semantic drift or require an explicit,
  test-covered version migration.
- Existing generated workflow YAML and bindings artifacts remain pinned by golden tests.
- `dockpipe pipelang invoke` remains pure and CLI-only unless a separately reviewed public contract
  replaces it.
- `dockpipe run` continues consuming compiled workflow/YAML behavior rather than parsing PipeLang.
- New application projections must be additive/versioned and must not silently change existing
  catalog or launcher fields.
- Parser acceptance, typechecker support, evaluator support, compiler emission, CLI help, canonical
  docs, schemas, and editor syntax/completion must advance together for each shipped feature.

## Implementation Sequence

1. Freeze current `v0.0.0.1` parser, typechecker, evaluator, compiler, CLI, materialization, and
   generated-artifact behavior with focused compatibility/golden fixtures.
2. Define the complete semantic-ID contract, then implement one end-to-end explicit-ID vertical
   slice across existing annotations, compiler validation, emitted metadata, diagnostics, tests, and
   editor support without changing execution behavior.
3. Define the effect/authority taxonomy and call-composition rules, then add inert declarations and
   prove compile/catalog/editor operations execute nothing.
4. Add first-class constrained values plus the smallest `requires`/`ensures`/invariant metadata and
   runtime-validation contract required by one fixture.
5. Define deterministic replay records and property-generator/shrinker interfaces; prove one seeded
   constrained primitive/record property test can reproduce and shrink the same failure.
6. Write the remaining language-version, nullability, value/state, mutation, computed-graph, action,
   module, compatibility, semantic-graph, and diagnostic decisions before expanding syntax further.
7. Add non-normative positive and negative fixtures for the smallest application vertical slice.
8. Define the versioned typed/semantic-graph projection consumed by TASK-020's Application IR and
   impact-analysis tooling.
9. Implement coherent type-system slices across AST, lexer/parser, typechecker, evaluator where
   applicable, compiler artifacts, docs, CLI diagnostics, and editor support.
10. Add pure computed properties and dependency/cycle validation.
11. Add bounded local state actions with deterministic transition tests.
12. Integrate safe view-expression type-checking with TASK-020 fixtures.
13. Prove the same typed projection drives `static-html` and the selected Qt backend before widening
    the language further.

Each implementation slice must be independently reviewable and preserve the engine boundary. Do not
land a permissive parser first with semantics deferred to target generators.

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
- semantic graph/impact query and independently verified change-manifest cases; and
- TASK-020 Application IR integration fixtures with no target-specific language symbols.

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
- Diagnostics carry stable source locations through the typed projection.
- The semantic graph produces dependency-kind-aware impact results rather than text-match claims.
- Change manifests are independently verified and never treated as trusted agent assertions.
- External model invocation is an explicit typed resolver-backed effect and is absent from ordinary
  programs unless declared.
- TASK-020 can consume the projection without parsing `.pipe` files independently.
- One minimal application fixture compiles through both semantic-web and Qt target fixtures with
  equivalent typed state, action, validation, and effect semantics.

## Next Bounded Design Slice

Produce a documentation-and-fixtures-only PipeLang `vNext` decision packet containing:

1. every decision in **Decisions Required Before Parser Work**, with stable IDs, effects/authority,
   contracts, determinism/replay, and property testing resolved first;
2. a formal current-versus-required grammar and AST gap table;
3. one minimal deployment-dashboard fixture covering stable IDs, enum, record, optional, constrained
   value, list value, computed property, local action, contract, inert effect declaration, explicit
   authority, and typed result/error;
4. positive and negative type-check examples with expected source-located diagnostics;
5. one deterministic seeded property-test/replay fixture and expected shrink result;
6. a versioned typed/semantic-graph projection fixture for TASK-020 and impact tooling;
7. a verifiable change-manifest fixture with verified, contradicted, and unverifiable claims; and
8. a compatibility matrix for current examples, generated artifacts, CLI invoke/materialize,
   catalog output, and editor tooling.

That slice must not change production PipeLang syntax or implementation, public YAML/schema,
catalog output, CLI behavior, launcher code, target adapters, or generated stores.

After that packet is accepted, the first production implementation slice is explicit semantic IDs
end to end. It must not include effects, contracts, property testing, state machines, impact tooling,
or model invocation until the ID slice is independently complete and verified.
