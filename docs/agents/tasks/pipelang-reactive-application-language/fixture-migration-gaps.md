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
[`legacy-v0.0.0.1-inventory.txt`](../fixtures/pipelang-vnext/legacy-v0.0.0.1-inventory.txt).
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
