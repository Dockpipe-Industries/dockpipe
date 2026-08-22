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

Step 8a is complete for `v0.31.0` same-class named pure record predicates consumed by exact direct
`filter(List<R>, PredicateName, P1, ...) -> List<R>`. Step 8b is complete for `v0.36.0` public
same-class pure method calls with exact signatures and a closed acyclic call graph. Step 8c is
complete for `v0.37.0` composition of those calls throughout already admitted eager pure
expressions, with direct match and propagation carriers preserved. Step 8d is complete for one
exactly typed lazy `condition ? whenTrue : whenFalse` expression per method under `v0.38.0`. These
seams do not complete or authorize cross-class/module calls, overloads, generics, function values,
lambdas, recursion, blocks, locals, branch statements, loops, effects, entrypoints, or the rest of
step 8.

Each slice is independently reviewable and keeps syntax, semantics, diagnostics, projection,
editor, tests, and any enabled backend synchronized. No permissive parser, target-owned semantics,
or syntax-first feature batch may skip the earlier foundations.

## Future Mixed-Mode Native-Module Checkpoint

Prepare a module-by-module strangler transition from evaluated Core to native machine-code
artifacts without turning native execution into a second language contract. This is backlog
preparation only: it authorizes no ABI, loader, backend, code generation, runtime, source, or
migration change.

Open this checkpoint only after step 10 has produced the shared semantic/Core projections and at
least one representative TASK-020 Qt application or TASK-022 PipeServe service works end to end
through the evaluator or current deterministic Go backend. The consumer must provide module-level
timing/allocation evidence or a concrete standalone-deployment requirement; anticipated performance
alone is not an implementation trigger.

The first action is a source-backed founder decision packet with two or three mutually exclusive
execution-boundary options, such as an isolated native process/capability, an in-process stable C
ABI shared library, or selected-module AOT through an enabled backend. Do not preselect the boundary
from this record. Each option must define isolation, call and error transport, canonical value
marshalling, allocation/ownership, module and semantic identity, source/lock/compiler/profile/
toolchain digest binding, capability authority, debug/source mapping, platform packaging, fallback,
and version/migration behavior.

Any accepted first implementation is limited to one pure, measured, low-dependency leaf module. The
same unchanged PipeLang source and module identity must resolve to either evaluated or native
execution; the evaluator remains the semantic oracle. Differential conformance must cover all
inputs, failures, validation, observable ordering, and storage ownership, while benchmarks record
build cost, startup, call overhead, throughput, allocations, and artifact size. Native selection
must be reversible without changing source or weakening workflow/package/runtime/resolver authority.

Exclude whole-runtime conversion, self-hosting by implication, multiple native modules, JIT and AOT
as one batch, raw Go or C++ object-layout exposure, target-specific language semantics, unsafe/manual
memory features, shared mutable aliases, callbacks, asynchronous effects, a general plugin ecosystem,
and retirement of the Go seed or evaluator. Expansion remains one measured module at a time through
separately accepted slices.

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
- source-map/symbol/value-projection, semantic-breakpoint, sanitized debug-bundle determinism and
  redaction, and Core/backend differential-debug cases for each enabled executable backend;
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
- Enabled executable backends retain reversible PipeLang source/symbol/value mappings and emit
  deterministic sanitized debug evidence consumable by both IDE and structured tooling clients.
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


## Checkpoint v0.33.0 complete contract

The v0.32.0 per-key ordinal sorting direction is the exact paired `sort_by_ordinal(values, R.Field, ascending|descending, ...)` contract recorded in `next-boundary.md`; it is one bounded pure ordering slice and preserves all earlier nodes and source contracts.


## Checkpoint v0.34.0 complete contract

Bounded propagation uses only `some(propagate(p))` and bounded `ok<T,E>(propagate(p))` over one direct identical carrier parameter. It has explicit HIR/Core control flow, deterministic evaluator and Core-only Go behavior, unchanged semantic identities, `PL3032` misuse diagnostics, and no exceptions, effects, matching, blocks, arbitrary Results, or target behavior. This is one coherent version boundary because syntax, carrier typing, early failure/absence, IR, execution, editor support, and compatibility are reviewed together.

## Checkpoint v0.35.0 complete contract

Exhaustive bounded matching is the exact direct-carrier `match` contract recorded in `next-boundary.md`. It adds explicit arm-local bindings, exact arm-type equality, deterministic PL3029–PL3031 diagnostics, evaluator/Core-only Go parity, unchanged identities, and no guards, destructuring, blocks, effects, or target behavior.

## Checkpoint v0.36.0 complete contract

Same-class pure calls use only `Method(expression, ...)` from a public expression-bodied method to
one uniquely named public method on the same class. Arguments and return types match the target's
declared ordered signature exactly; nested calls are accepted; self-recursion and every indirect
cycle fail as `PL3033`. Typed HIR and target-neutral Core carry the resolved callable semantic
identity and ordered operands, Core validates a closed same-owner acyclic call graph, the evaluator
uses isolated copied call frames, and the Core-only Go backend emits only validated calls.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain unchanged.
Cross-class/module calls, overloads, generics, private callers or targets, lambdas, function values,
effects, blocks, locals, branches, entrypoints, and target semantics remain excluded.

## Checkpoint v0.37.0 complete contract

General pure-call composition reuses the v0.36.0 resolved call identity, exact ordered signature,
same-class ownership, participant closure, and acyclic dependency graph throughout already
admitted eager pure expressions and match-arm bodies. Match carriers and propagation operands stay
direct references. HIR/Core retain the same call node, Core validates placement recursively, the
evaluator uses isolated copied frames, and the Go backend emits from Core only. The consumer proves
an Optional record match arm calling a normalization helper. Public schema shapes and the exact
45-source legacy inventory remain unchanged.

## Checkpoint v0.38.0 complete contract

One method may contain one `condition ? whenTrue : whenFalse` expression. The condition is exactly
`bool`; both branches are statically checked and have the same admitted type; only the selected
branch executes. Conditional operands may use existing eager pure expressions, including resolved
v0.37.0 calls, but may not contain another conditional, match, or propagation node. Typed HIR and
target-neutral Core carry the three typed operands explicitly, Core independently validates the
bounded shape and types, the evaluator selects one branch, and the Core-only Go backend emits the
same lazy choice without inference. Public schema shapes and the exact 45-source legacy inventory
remain unchanged; only language-contract metadata advances.

## Application IR checkpoint complete contract

`dockpipe.application.v1` consumes the canonical public semantic projection plus the matching Core
program and an explicit source-located stable-identity spec. It projects typed snapshot, section
Result, row/key/column, optional selection/details, filtering, ordering, and contract metadata into
deterministic JSON. Validation rejects missing identities, mismatched contracts, duplicate
sections, empty columns, and invalid directions. It changes no PipeLang language, HIR, Core,
evaluator, backend, or stable identity; it adds no parsing, inference, runtime, target, Docker,
refresh, action, launcher, or CLI behavior.
