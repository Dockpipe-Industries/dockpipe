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
exactly typed lazy `condition ? whenTrue : whenFalse` expression per method under `v0.38.0`. Step 8e
is complete for one explicitly typed immutable local followed by one terminal return under
`v0.39.0`. Step 8f is complete for one or more source-ordered explicitly typed immutable locals
followed by one terminal return under `v0.40.0`. Step 8g is complete for exactly one
`T name = propagate(carrier);` first local under `v0.41.0`, where `carrier` is the sole direct
parameter, its bounded carrier type exactly equals the method return type, and `T` exactly equals
its payload type. Step 8h is complete for exactly one prior-local helper propagation under
`v0.42.0`: the first local is one resolved public same-class helper call over the method's sole
direct parameter, the second local propagates that first carrier local, and the bounded carrier
exactly equals the method return type. Step 8i is complete for one top-level
`match(Helper(input))` under `v0.43.0`: the one-parameter public pure caller returns `string`, the
resolved same-class public pure helper takes that direct `string` parameter and returns
`Result<string, string>`, and exact source-ordered bound `ok` then `err` arms select the copied
payload. Step 8j is complete for general bounded helper-carrier matching under `v0.44.0`. One
public pure caller with one or more parameters may use exactly one top-level
`match(Helper(...))`; every caller parameter is passed directly once in declaration order to one
resolved public pure same-class helper with the exact parameter signature. The helper carrier is
closed to admitted `Optional<T>`, `Result<List<R>, string>`, or `Result<string, string>`. Optional
arms are exact source-ordered `some(binding)` then binding-free `none`; Result arms are exact
source-ordered `ok(binding)` then `err(binding)`. Both arms exactly typecheck to the declared caller
return. Step 8k is complete for one v0.44-compatible helper-carrier match as the initializer of the
first explicitly typed immutable local under `v0.45.0`; later existing ordered locals and the
terminal return may consume that local. Step 8l is complete under `v0.46.0`: that exact first-local
composition additionally admits existing checked-arithmetic `Result<int, ArithmeticError>` and
`Result<float, ArithmeticError>` helper carriers without widening placement, arguments, or Result
construction. Step 8m is complete under `v0.47.0`: exactly one v0.46-compatible helper-carrier
match may initialize any explicitly typed immutable local after zero or more ordinary locals. All
earlier locals evaluate eagerly once in source order; the helper and selected local initialize
once; only the selected arm evaluates; and later locals plus the terminal return continue. Step 8n
is complete under `v0.48.0`: after zero or more ordinary locals, one explicitly typed local may
call that exact helper and the immediately adjacent typed local may use the single canonical
`match(carrier)`; the carrier local and selected local each initialize once. These
seams do not complete or authorize inference,
reassignment, shadowing, cross-class/module calls, overloads, generics, function values, lambdas,
recursion, general blocks, branch statements, loops, effects, entrypoints, additional or nested
propagation, arbitrary computed propagation operands, propagation from any other prior local,
arbitrary `Result` carriers, or the rest of step 8.

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

## Checkpoint v0.39.0 complete contract

One public pure method may use `{ T name = initializer; return expression; }` instead of an
expression body. `T` is explicit and must exactly match the eagerly evaluated initializer. The
local enters scope only after initialization, cannot shadow a field or parameter, and is immutable.
The terminal return may reference the local, parameters, and existing admitted eager pure
expressions. An explicit checked-arithmetic `Result` local supplies arithmetic result context;
contextual propagation remains excluded from the initializer and return because it retains its
established complete-method carrier shape.
Typed HIR and target-neutral Core carry an explicit `immutable_local` node, analysis-local binding,
initializer, and return. Core validates canonical scope and exact types; the evaluator and
Core-only Go backend evaluate the initializer once and return from the scoped expression. Public
identities and schema shapes remain unchanged; only language-contract metadata advances.

## Checkpoint v0.40.0 complete contract

One public pure method may use
`{ T1 first = expression1; T2 second = expression2; ... return expression; }` with one or more
source-ordered explicitly typed immutable locals. Initializers evaluate eagerly exactly once in
source order. Each local enters scope only after its initializer, so later initializers may use
earlier locals while self-reference, forward reference, duplicate names, and field/parameter
shadowing fail. Every initializer exactly matches its declared type and the terminal expression
exactly matches the method return type.

Typed HIR and target-neutral Core represent the sequence as canonically positioned right-nested
`immutable_local` nodes. Core proves that every local belongs to the one top-level sequence,
validates contiguous positions and parameter/prior-local shadowing independently, and rejects locals in initializers or
other expression positions. The evaluator and Core-only Go backend preserve source order,
once-only initialization, lexical scope, and copied value storage. Public compiler, semantic, and
Application IR schema identities and shapes remain unchanged; only language-contract metadata
advances. Inference, reassignment, propagation inside the block, early returns, statement branches,
nested blocks, loops, effects, actions, runtime behavior, and target-specific behavior remain
excluded.

## Checkpoint v0.41.0 complete contract

One public pure method block may place exactly one `T name = propagate(carrier);` declaration as
its first immutable local. `carrier` is the method's sole direct parameter, its carrier type
exactly equals the method return type, and `T` exactly equals the carried payload type. The carrier
is limited to `Optional<T>` for an admitted primitive or record, `Result<List<R>, string>`, or
`Result<string, string>`. Presence or success binds a validated copied payload and continues
through the remaining ordered locals and terminal return; absence or failure immediately returns
the identical canonical carrier.

Typed HIR and target-neutral Core reuse the existing `propagate` and right-nested
`immutable_local` nodes. Core independently validates the first-local placement, single
occurrence, direct-parameter identity, exact carrier and payload types, and bounded carrier matrix.
The evaluator and Core-only Go backend preserve the same short-circuit and copied-value semantics.
Public compiler, semantic, and Application IR schema identities and shapes remain unchanged; only
language-contract metadata advances. No second or nested propagation, computed operand,
propagation from a prior local, arbitrary `Result` including checked arithmetic, inference,
reassignment, early return, statement branch, nested block, loop, effect, action, runtime, target,
adapter, UI, or deployment behavior enters by implication.

## Checkpoint v0.42.0 complete contract

One public pure method block may place exactly
`C carrier = Helper(input); T value = propagate(carrier);` as its first two immutable locals.
The method has one direct parameter `input`; `Helper` resolves to one uniquely named public pure
method on the same class with exactly that parameter type and bounded carrier return `C`. The call
argument is the direct method parameter. `C` exactly equals the enclosing method return type, and
`T` exactly equals its success/presence payload. The carrier matrix remains the v0.41.0 matrix:
an admitted primitive/record Optional, `Result<List<R>, string>`, or
`Result<string, string>`. Helper evaluation occurs once. Presence or success copies the validated
payload into `value` and continues through later ordered locals and the terminal return; absence or
failure immediately returns the identical canonical helper carrier.

Typed HIR and target-neutral Core reuse the existing `call`, `immutable_local`, and `propagate`
nodes. Core independently validates the first-call/second-propagation positions, sole direct
argument, direct reference to the immediately preceding carrier local, exact carrier/payload
types, bounded carrier matrix, resolved same-class callable signature, and closed acyclic call
graph. The evaluator and Core-only Go backend preserve once-only helper evaluation, short-circuit,
and copied-value semantics. Public compiler, semantic, and Application IR schema identities and
shapes remain unchanged; only language-contract metadata advances. No direct
`propagate(Helper(...))`, multiple/nested propagation, additional helper argument, propagation from
another local, arbitrary computed carrier, arbitrary Result, inference, reassignment, early return,
statement branch, nested block, loop, effect, action, runtime, target, adapter, UI, or deployment
behavior enters by implication.

## Checkpoint v0.43.0 complete contract

One public pure method with one direct `string` parameter and `string` return may use exactly one
top-level `match(Helper(input)) { ok(value) => whenOk, err(error) => whenErr }` expression. `Helper`
resolves to one uniquely named public pure same-class method, takes the direct parameter as its
sole argument, and returns exactly `Result<string, string>`. It evaluates once. The complete Result
is validated, the selected payload is copied into its unique arm binding, and only the selected arm
expression evaluates. Arms are exact, exhaustive, source ordered, and wildcard-free.

Typed HIR and target-neutral Core reuse `call` and `match`. Core independently validates placement,
occurrence count, direct argument identity, exact caller/helper signatures, same ownership, closed
acyclic calls, carrier type, ordered arm tags, and bindings. The evaluator and Core-only Go backend
preserve complete-carrier validation, once-only helper evaluation, copied payloads, and lazy arm
selection. Public compiler, semantic, and Application IR schema identities and shapes remain
unchanged; only language-contract metadata advances. Optional/list/arithmetic Result helpers,
extra or computed arguments, extra caller parameters, cross-owner calls, nested matches, reversed
or wildcard arms, guards, new blocks/locals, propagation changes, inference, reassignment,
statements, loops, effects, actions, runtime, targets, adapters, UI, and deployment behavior do not
enter by implication.

## Checkpoint v0.44.0 complete contract

One public pure method with one or more parameters may use exactly one complete top-level
`match(Helper(...))`. Every caller parameter is the corresponding direct helper argument exactly
once and in declaration order. `Helper` is one uniquely resolved public pure same-class method
with that exact parameter list and returns either admitted `Optional<T>`,
`Result<List<R>, string>`, or `Result<string, string>`. Optional arms are exact source-ordered
`some(binding)` then binding-free `none`; Result arms are exact source-ordered `ok(binding)` then
`err(binding)`. Both arm expressions exactly match the caller return type.

Typed HIR and target-neutral Core reuse `call` and `match`. Core independently validates placement,
occurrence count, direct argument positions and types, same ownership, exact target signature,
closed carrier matrix, canonical arms, and bindings. The evaluator and deterministic Core-only Go
backend validate the complete carrier, evaluate the helper once, copy the selected payload, and
evaluate only the selected arm. Compiler, semantic, and Application IR identities and shapes stay
unchanged; only language-contract metadata advances. Arithmetic Results, computed/reordered/
omitted/extra arguments, cross-owner calls, overloads, generics, nested or multiple matches,
wildcards, reversed arms, guards, propagation changes, blocks, locals, statements, effects,
actions, runtimes, targets, adapters, UI, and deployment behavior remain excluded.

## Checkpoint v0.45.0 complete contract

One public pure method with one or more parameters may place exactly one v0.44-compatible
`match(Helper(...))` as the initializer of its first explicitly typed immutable local. Every caller
parameter is the corresponding direct helper argument exactly once in declaration order. The
helper remains one uniquely resolved public pure same-class method with the exact signature and an
admitted `Optional<T>`, `Result<List<R>, string>`, or `Result<string, string>` return. Optional arms
remain exact source-ordered `some(binding)` then binding-free `none`; Result arms remain exact
source-ordered `ok(binding)` then `err(binding)`. Both arm expressions exactly match the declared
local type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`. Core independently
validates the first-local placement, one match, direct helper arguments, same ownership, exact
target signature, closed carrier matrix, canonical arms and bindings, local type, and continuation
scope. The evaluator and deterministic Core-only Go backend validate the complete carrier,
evaluate the helper once, copy the selected result into the local once, evaluate only the selected
arm, and then execute existing ordered locals plus the terminal return. Compiler, semantic, and
Application IR identities and shapes stay unchanged; only language-contract metadata advances.
The exact 45-source lane remains frozen.

Match in later locals, the terminal return, arguments, or nested positions; multiple matches;
computed/reordered/omitted/extra helper arguments; arithmetic Results; cross-owner calls;
overloads; generics; wildcard or reversed arms; guards; propagation changes; inference;
reassignment; early returns; statement branches; loops; effects; actions; runtimes; targets;
adapters; UI; and deployment behavior remain excluded.

## Checkpoint v0.46.0 complete contract

One public pure method with one or more parameters may use exactly one v0.45-compatible
first-local `match(Helper(...))` where the helper returns an existing checked-arithmetic
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`. Every caller parameter remains
the corresponding direct helper argument exactly once in declaration order. The helper is one
uniquely resolved public pure same-class method with the exact caller signature. Arms remain exact
source-ordered `ok(binding)` then `err(binding)`, and both expressions exactly match the declared
local type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`. Core independently
validates first-local placement, one match, direct arguments, same ownership, exact signature,
checked-arithmetic carrier shape, canonical arms/bindings, local type, and continuation scope. The
evaluator and deterministic Core-only Go backend validate the complete Result, evaluate the helper
once, copy the selected payload once, evaluate only the selected arm, and continue through existing
ordered locals and the terminal return. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language-contract metadata
advances, and the exact 45-source lane remains frozen.

Checked-arithmetic matching as the complete method body, in later locals, the terminal return,
arguments, or nested positions remains excluded. Additional matches, computed/reordered/omitted/
extra arguments, arithmetic Result propagation/construction/defaulting, arbitrary Results,
cross-owner calls, overloads, generics, wildcards, reversed arms, guards, inference, reassignment,
early returns, statement branches, loops, effects, actions, runtimes, targets, adapters, UI, and
deployment behavior remain excluded.

## Checkpoint v0.47.0 complete contract

One public pure method with one or more parameters may use exactly one v0.46-compatible
`match(Helper(...))` as any explicitly typed immutable-local initializer after zero or more ordinary
locals. Every caller parameter remains the corresponding direct helper argument exactly once in
declaration order. The helper is one uniquely resolved public pure same-class method with the exact
caller signature. The closed carrier matrix is unchanged: admitted Optional primitive/record,
`Result<List<R>, string>`, `Result<string, string>`, and existing checked-arithmetic
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`. Arms retain their exact
source order, bindings, and declared-local result type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`. Core independently
validates one local match, direct arguments, same ownership, exact signature, the closed carrier
matrix, canonical arms/bindings, local type, and continuation scope. Earlier ordinary locals
evaluate eagerly once in source order; the helper and matched local initialize once; the complete
carrier is validated; only the selected arm evaluates; and later locals plus the terminal return
continue. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities
and shapes remain unchanged; only language-contract metadata advances, and the exact 45-source
lane remains frozen. The Application IR fixture proves a fallback local before selection without
changing the consumer schema.

Terminal-return, argument, nested, or multiple matches remain excluded. Checked-arithmetic matching
as the complete method body remains excluded. Computed/reordered/omitted/extra arguments;
propagation changes; Result construction, defaulting, or arbitrary widening; cross-owner/private/
overloaded/generic helpers; wildcard or reversed arms; guards; inference; reassignment; statements;
effects; actions; runtimes; targets; adapters; UI; and deployment behavior remain excluded.

## Checkpoint v0.48.0 complete contract

After zero or more ordinary locals, one public pure method with one or more parameters may declare
an explicitly typed carrier local initialized by one uniquely resolved public pure same-class
exact-signature `Helper(p1, ..., pn)` and an immediately adjacent explicitly typed local initialized
by exactly one canonical `match(carrier)`. Every caller parameter is passed directly once in
declaration order. The carrier matrix remains closed to admitted Optional primitive/record,
`Result<List<R>, string>`, `Result<string, string>`, and checked-arithmetic
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`. Core
independently validates adjacency, one match, the exact carrier reference, direct arguments,
same-owner exact signature, closed carrier matrix, canonical arms/bindings, local typing, and
continuation scope. Earlier locals, the carrier local, and matched local initialize eagerly once in
source order; the complete carrier is validated; only the selected arm evaluates; and later locals
plus the terminal return continue. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language-contract metadata
advances, and the exact 45-source lane remains frozen. The Docker observability consumer proves a
fallback, adjacent `selection` carrier, `match(selection)`, and normalization continuation.

Non-adjacent, terminal-return, argument, nested, or multiple matches remain excluded. Top-level
checked-arithmetic matching; computed/reordered/omitted/extra arguments; propagation changes;
Result construction/defaulting or arbitrary widening; cross-owner/private/overloaded/generic
helpers; wildcard or reversed arms; guards; inference; reassignment; statements; effects; actions;
runtimes; targets; adapters; UI; and deployment behavior remain excluded. No behavior enters by
implication.

## Application IR checkpoint complete contract

`dockpipe.application.v1` consumes the canonical public semantic projection plus the matching Core
program and an explicit source-located stable-identity spec. It projects typed snapshot, section
Result, row/key/column, optional selection/details, filtering, ordering, and contract metadata into
deterministic JSON. Validation rejects missing identities, mismatched contracts, duplicate
sections, empty columns, and invalid directions. It changes no PipeLang language, HIR, Core,
evaluator, backend, or stable identity; it adds no parsing, inference, runtime, target, Docker,
refresh, action, launcher, or CLI behavior.
