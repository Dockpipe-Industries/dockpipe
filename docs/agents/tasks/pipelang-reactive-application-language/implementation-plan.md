## Bounded Implementation Order

The latest completed work is the [multi-stage checked-chain inheritance repair](checked-chain-inheritance-repair.md).
It restores the existing v0.57/v0.58 chains through v0.82 without adding a language boundary.
The prior checked-propagation inheritance repair is committed at `46ce299a`.

The prior completed work is the [checked-propagation inheritance repair](checked-propagation-inheritance-repair.md).
It restores the exact v0.56 two-parameter form through v0.82 without adding a language boundary.

The prior completed work is the [numeric-comparison evaluator repair](numeric-comparison-repair.md)
under existing language versions. That record owns its focused and terminal proof; it adds no
implementation-order step or new language boundary.

The [six v0.80 milestone repair slices](milestone-v080-repairs.md) requested on 2026-09-04
are complete and committed. The separately selected and approved `v0.81.0` terminal-tree slice
is also complete; [step 8au](completed-functions.md#step-8au-terminal-trees-through-depth-three-v0810)
owns its proof and deferred findings.

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
`match(carrier)`; the carrier local and selected local each initialize once. Step 8o is complete
under `v0.49.0`: one public pure method may contain exactly two non-overlapping
v0.48-compatible adjacent helper-call carrier and match-local pairs. Ordinary locals may surround
or separate the pairs but cannot split either pair. Both pairs evaluate in source order, each
helper and selected local initializes once, each complete carrier is validated, and only each
selected arm evaluates. Existing zero-match and one-match forms remain unchanged. Step 8p is
complete under `v0.50.0`: one new form makes those two pairs a contiguous four-local stage and
passes the first selected local as the second helper's first direct argument, followed by every
caller parameter once in declaration order. Existing v0.49 independent pairs remain exact. Step
8q is complete under `v0.51.0`: the same dependency rule extends to one contiguous chain of
two or more carrier/match pairs. Every later helper receives only the immediately preceding
selected local before the unchanged direct caller signature; all pairs remain contiguous and use
the existing closed carrier matrix. Step 8r is complete under `v0.52.0`: a chain of at least three
pairs may instead use one method-wide cumulative mode where every later helper receives all prior
selected locals exactly once in chain order, followed by the unchanged direct caller signature.
The inherited immediate-only mode remains exact and modes cannot mix. Step 8s is complete under
`v0.53.0`: the v0.42 prior-local propagation form accepts two or more caller parameters when its
first helper call receives every parameter directly once in declaration order. The inherited
one-parameter form remains exact. Step 8t is complete under `v0.54.0`: that exact first-two-local
form additionally admits an existing `Result<int, ArithmeticError>` or
`Result<float, ArithmeticError>` helper carrier. The helper evaluates once, the complete carrier
is validated, success is copied, and canonical arithmetic failure returns before the terminal
checked arithmetic expression. Step 8u is complete under `v0.55.0`: the inherited v0.41
sole-direct-parameter first-local propagation form additionally admits an existing
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>` carrier. The complete direct
carrier is validated, success is copied, and canonical arithmetic failure returns before the
terminal checked arithmetic expression. Step 8v is complete under `v0.56.0`: that direct checked
form additionally admits exactly one second direct parameter whose type equals the carrier payload.
The propagated local is the left operand and the second parameter is the right operand of one
checked integer add, subtract, or multiply, or checked binary64 divide. `propagate(Helper(...))`
remains excluded. Step 8w is complete under `v0.57.0`: one exact three-parameter method propagates
the incoming arithmetic Result, stores one checked Result from the first payload operation,
propagates that explicit carrier, and returns a second checked payload operation. These seams do
not complete or authorize inference. Step 8x is complete under `v0.58.0`: that two-stage shape
generalizes to a contiguous chain of `K >= 2` checked stages, with one incoming arithmetic Result
followed by exactly `K` payload parameters; every non-terminal stage explicitly stores and then
propagates its checked Result before the terminal checked operation. Step 8y is complete under
`v0.59.0`: one public pure method may propagate a sole direct bounded `Result<T, string>` into its
first and only typed local and terminally call one exact same-class `T -> Result<U, string>` helper,
where `T != U` and both are text or an existing primitive-record list. Incoming failure is
canonically reshaped to the target Result without invoking the helper. Step 8z is complete under
`v0.60.0`: that bounded cross-payload flow admits exactly two adjacent stages through one explicit
intermediate `Result<U, string>` local and its direct propagation. Both helpers remain exact public
pure same-class calls; adjacent payloads differ, while source and target payloads may match.
Incoming and intermediate failures are canonically reshaped to the target Result and skip every
later helper. Step 8aa is complete under `v0.61.0`: the exact two-stage form generalizes to a
contiguous chain of `K >= 2` adjacent cross-payload helper stages. Every non-terminal stage remains
an explicit helper-Result local immediately followed by its direct propagation local; the terminal
helper receives the direct preceding payload local. Adjacent payloads differ, non-adjacent payloads
may match, and every failure is canonically reshaped to the final target Result before later helpers
can run. Step 8ab is complete under `v0.62.0`: the v0.61 generalized form additionally admits
exactly one second direct `string` context parameter. Every helper receives the immediately
preceding payload local first and that unchanged context parameter second. Context is passed once
to every stage in the same order; all carrier, payload, failure, adjacency, placement, ownership,
and helper constraints remain exact. Step 8ac is complete under `v0.63.0`: the contextual form
additionally admits exactly one helper stage. The caller still takes only the direct bounded Result
carrier and one direct string context; it propagates the carrier into its first and only typed
local, then terminally calls one exact public pure same-class helper with that local followed by the
unchanged context. Source and target payloads remain distinct. These seams do not complete or
authorize inference. Step 8ad is complete under `v0.64.0`: the exact v0.63 one-stage contextual
form additionally admits equal source and target payloads. The payload remains exactly `string` or
`List<R>` for an existing public primitive-field record; the helper remains exact, public, pure,
same-class, and `(T, string) -> Result<T, string>`. Multi-stage same-payload contextual chains remain
excluded. These seams do not complete or authorize inference,
reassignment, shadowing, cross-class/module calls, overloads, generics, function values, lambdas,
recursion, general blocks, branch statements, loops, effects, entrypoints, additional or nested
propagation, arbitrary computed propagation operands, propagation from any other prior local,
arbitrary `Result` carriers, or the rest of step 8.

Step 8ae is complete under `v0.65.0`: the exact v0.62 two-stage contextual form additionally
admits equality at either or both adjacent payload transitions. Each payload remains exactly
`string` or `List<R>` for an existing public primitive-field record. The two helpers remain exact,
public, pure, same-class, and `(Ti, string) -> Result<Ti+1, string>`; every stage receives the
unchanged direct string context. Contextual chains with three or more stages still require every
adjacent payload to differ. This seam does not complete or authorize broader same-payload chains.

Step 8af is complete under `v0.66.0`: the v0.62 contextual `K >= 2` chain now permits any adjacent
payload transition to preserve or change its bounded payload type. Every non-terminal helper Result
remains an explicit local immediately followed by direct propagation; the terminal helper receives
the directly preceding payload. Every helper remains exact, public, pure, same-class, and
`(Ti, string) -> Result<Ti+1, string>`, and every reached stage receives the same unchanged direct
string context. This seam does not admit new carriers, context shapes, blocks, effects, or inference.

Step 8ag is complete under `v0.67.0`: the v0.66 contextual `K >= 2` chain now accepts `N >= 1`
direct `string` context parameters after the carrier. Every helper receives the immediately
preceding payload followed by every unchanged context exactly once in caller declaration order and
has the exact `(Ti, string...) -> Result<Ti+1, string>` signature. The explicit alternating
carrier/propagation locals, bounded payloads, shared string failure, same-class public pure helper
ownership, and canonical final-shaped short-circuiting remain exact. This seam does not admit
one-stage multi-context chains, computed or stage-specific contexts, new context types, new
carriers, blocks, effects, or inference.

Step 8ah is complete under `v0.68.0`: the v0.64 one-stage contextual bounded Result form now accepts
`N >= 1` direct `string` context parameters after the carrier. Its single terminal helper receives
the directly propagated payload followed by every unchanged context exactly once in caller
declaration order and has the exact `(T0, string...) -> Result<T1, string>` signature. Same- and
cross-payload flows remain bounded to text or existing primitive-record lists with shared string
failure. This seam does not admit computed or stage-specific contexts, new context types, new
carriers, additional locals or propagation, blocks, effects, or inference.

Step 8ai is complete under `v0.69.0`: one public pure method may contain one or more existing
ordered immutable locals followed by exactly one terminal statement-level conditional of the form
`if (condition) { return whenTrue; } else { return whenFalse; }`. The condition is `bool`, both
branches have the declared method return type, and every local, condition, and branch value remains
an already-admitted eager pure expression. Typed HIR and Core reuse the existing conditional node
with an explicit terminal-statement marker; at most one preceding local initializer may retain the
inherited bounded conditional expression. This seam does not admit a zero-local form, branch
locals, nesting, missing `else`, fallthrough, returns elsewhere, propagation or matching in the
method, assignment, loops, effects, or inference.

Step 8aj is complete under `v0.70.0`: the v0.69 terminal statement-level `if/else` remains exact,
while either terminal branch may now declare at most one explicitly typed immutable local and
immediately return from that branch. One or more top-level ordered immutable locals remain
required. Branch scopes are independent, may reuse a local name, evaluate only when selected, and
cannot escape. Typed HIR and Core reuse the existing `immutable_local` nested in the terminal
`conditional`; no schema identity changes. This seam does not admit nested branches, multiple
locals in one branch, propagation, matching, assignment, fallthrough, other early returns, loops,
effects, or inference.

Step 8ak is complete using `v0.71.0`: either v0.70 terminal branch may now contain at most two
explicitly typed ordered immutable locals before its return. A second local may reference the
first in the same branch; opposing branches remain independent lexical scopes, may reuse names,
and only the selected branch evaluates. Typed HIR and Core continue to represent each sequence as
nested `immutable_local` nodes inside the terminal `conditional`; no schema identity changes. This
seam does not admit a third branch local, nested branches, zero top-level locals, propagation,
matching, assignment, fallthrough, other early returns, loops, effects, or inference.

Step 8al is complete under `v0.72.0`: the v0.71 per-branch cap generalizes to any finite
source-ordered sequence of explicitly typed immutable locals before the branch return. Each local
enters scope only after its initializer, so later branch locals may reference earlier ones while
self-reference, forward reference, duplicate names, and shadowing remain invalid. Opposing branches
retain independent lexical scopes, may reuse names and canonical positions, and only the selected
branch evaluates. Typed HIR and Core continue to use nested `immutable_local` nodes inside the
terminal `conditional`; no schema identity changes. This seam does not admit nested branches, zero
top-level locals, propagation, matching, assignment, fallthrough, other early returns, loops,
effects, or inference.

Step 8am is complete using `v0.73.0`: the v0.72 terminal statement-level `if/else` may now be the
complete public pure method body without a preceding top-level immutable local. Either branch
retains its existing finite source-ordered sequence of explicitly typed immutable locals followed
by return, including the direct-return form. Branch scopes, eager source order within the selected
branch, and selected-branch-only evaluation remain exact. This seam does not admit nested branches,
propagation, matching, assignment, fallthrough, other early returns, loops, effects, or inference.

Step 8an is complete using `v0.74.0`: the complete v0.73 terminal form may contain exactly one
inner terminal `if/else` at the end of exactly one outer branch after zero or more explicitly typed
ordered immutable locals. The sibling outer branch retains the v0.73 branch form, inner leaves
return directly, outer-branch locals remain visible to the inner condition and leaves, and both
levels evaluate only their selected branch. This seam does not admit top-level locals for the new
topology, inner locals, nesting in both outer branches, another nested decision, third-level
nesting, conditional expressions within the topology, propagation, matching, assignment,
fallthrough, other early returns, loops, effects, or inference.

Step 8ao is complete using `v0.75.0`: either inner leaf of the exact v0.74 one-branch nested
topology may contain any finite source-ordered sequence of explicitly typed immutable locals before
its return. Inner-leaf locals enter scope only after their initializer, later locals may reference
earlier locals in the same leaf, and no binding crosses to the sibling leaf or escapes the inner
decision. Outer-branch locals remain visible to the inner condition and both leaves, and evaluation
remains selected-branch-only. This seam does not admit top-level locals for the topology, nesting in
both outer branches, another nested decision, third-level nesting, conditional expressions within
the topology, propagation, matching, assignment, fallthrough, other early returns, loops, effects,
or inference.

Step 8ap is complete using `v0.76.0`: one or more source-ordered explicitly typed immutable locals
may precede the exact v0.75 one-branch bounded nested terminal topology. Root locals enter scope
only after their initializer, evaluate eagerly once before the outer condition, and remain visible
to that condition and every descendant branch. The inherited rootless v0.75 form remains valid.
This seam does not admit nesting in both outer branches, another nested decision, third-level
nesting, conditional expressions within the topology or root-local nested form, propagation,
matching, assignment, fallthrough, other early returns, loops, effects, or inference.

Step 8aq is complete using `v0.77.0`: after one or more source-ordered explicitly typed immutable
root locals, both branches of the outer terminal `if/else` may now end in exactly one inner terminal
`if/else`. Each outer branch and inner leaf retains its finite immutable-local sequence. Root locals
evaluate eagerly once before the outer condition; only the selected outer branch, its inner
condition, and its selected leaf evaluate. This seam does not admit the symmetric topology without
a root local, third-level nesting, conditional expressions within the topology, propagation,
matching, assignment, fallthrough, other early returns, loops, effects, or inference.

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

## Checkpoint v0.49.0 complete contract

One public pure method with one or more parameters may contain exactly two non-overlapping
v0.48-compatible adjacent pairs:
`C1 firstCarrier = Helper1(p1, ..., pn); T1 first = match(firstCarrier) { ... };` and
`C2 secondCarrier = Helper2(p1, ..., pn); T2 second = match(secondCarrier) { ... };`.
Zero or more ordinary immutable locals may appear before, between, or after the pairs, but no local
may split a carrier from its matching local. Each helper independently receives every caller
parameter directly once in declaration order and resolves to a public pure same-class exact-signature
method. Each pair independently uses the unchanged v0.48 carrier matrix and canonical arms.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`. Core
independently validates exactly two top-level matches, two non-overlapping adjacent pairs, exact
carrier references, direct arguments, same-owner signatures, closed carriers, canonical arms and
bindings, local typing, and continuation scope. Locals and pairs evaluate eagerly once in source
order; each full carrier is validated; only its selected arm evaluates; and both matches complete
before the terminal return. The second pair and its arms may consume prior locals, including the
first selected local, under existing immutable scope rules. Matching remains non-propagating.

`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language-contract metadata advances, and the exact 45-source lane remains
frozen. Docker observability proves two exact-signature selection helpers, two adjacent carrier/match
pairs, prior-local reuse, evaluator behavior, and deterministic Core-only Go without schema change.

Existing zero-match and one-match methods remain exact. A third match, a split or overlapping pair,
terminal-return/argument/nested matching, non-helper or computed carriers, computed/reordered/
omitted/extra helper arguments, propagation changes, Result construction/defaulting or arbitrary
widening, cross-owner/private/overloaded/generic helpers, wildcard or reversed arms, guards,
inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment behavior remain excluded. No behavior enters by implication.

## Checkpoint v0.50.0 complete contract

One new public pure method form is exactly
`C1 firstCarrier = Helper1(p1, ..., pn); T1 first = match(firstCarrier) { ... };`
`C2 secondCarrier = Helper2(first, p1, ..., pn); T2 second = match(secondCarrier) { ... };`
as four contiguous explicitly typed locals. Ordinary locals may appear before or after the stage,
not inside it. `Helper1` retains the v0.49 caller signature. `Helper2` is a uniquely resolved public
pure same-class method whose first parameter exactly matches `T1`, followed by the caller's exact
parameter signature. Both pairs retain the closed carrier matrix and canonical arms.

Typed HIR and Core reuse existing nodes. Core independently validates the four-local stage,
binding positions, helper ownership and signatures, carrier types, arms, local typing, and
continuation. Evaluation and Core-only Go preserve once-only source order, complete-carrier
validation, and selected-arm-only evaluation. Public compiler, semantic, and Application IR schema
identities and shapes remain unchanged; metadata advances to `v0.50.0`; the exact 45-source lane is
frozen. Docker observability proves `ConfirmSelection(selected, rows, id)` before normalization.

Existing zero-match, one-match, and v0.49 independent two-pair forms remain exact. Other local
argument arrangements, computed/reordered/repeated arguments, split stages, third/nested/terminal
matches, propagation changes, arbitrary Result widening, statements, effects, actions, runtimes,
targets, adapters, UI, and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.51.0 complete contract

One public pure method may contain one contiguous dependent chain of `k >= 2` carrier/match pairs:
`C1 carrier1 = Helper1(p1, ..., pn); T1 value1 = match(carrier1) { ... };`, followed by
`Ci carrierI = HelperI(valueI-1, p1, ..., pn); Ti valueI = match(carrierI) { ... };` for each later
stage. All `2k` locals are explicitly typed and contiguous; ordinary locals may occur only before
or after. `Helper1` keeps the exact caller signature. Every later helper resolves uniquely to a
public pure same-class method whose first parameter exactly matches the immediately preceding
selected local and whose remaining parameters exactly match the caller declaration order. Every
pair retains the closed carrier matrix and canonical arms.

Typed HIR and Core reuse existing nodes. Core independently validates chain contiguity,
immediate-predecessor dependencies, binding positions, helper ownership/signatures, carriers,
arms, local types, and continuation. Evaluation and Core-only Go preserve once-only source order,
complete-carrier validation, and selected-arm-only evaluation. Public compiler, semantic, and
Application IR schema identities and shapes remain unchanged; metadata advances to `v0.51.0`;
the exact 45-source lane is frozen. Docker observability proves
`FinalizeSelection(confirmed, rows, id)` as the third stage before normalization.

Existing zero-match, one-match, v0.49 independent two-pair, and v0.50 dependent two-stage forms
remain exact. Gaps, mixed independent/dependent chains, non-immediate dependencies, fan-in,
computed/reordered/repeated/omitted/extra arguments, third matches outside this exact chain,
nested/terminal/argument matches, propagation changes, arbitrary Result widening, statements,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication.

## Checkpoint v0.52.0 complete contract

One public pure method may additionally contain one cumulative chain of `k >= 3` contiguous
carrier/match pairs. Stage one retains `Helper1(p1, ..., pn)`. Each later stage has the exact source
spelling `Ci carrierI = HelperI(value1, ..., valueI-1, p1, ..., pn); Ti valueI =
match(carrierI) { canonical arms };`: every prior selected local appears directly once in chain
order, followed by every caller parameter directly once in declaration order. A method uses either
this cumulative mode or the inherited v0.51 immediate-only mode; per-stage mixing is rejected.

Typed HIR and Core reuse existing nodes. Core independently validates chain contiguity, cumulative
binding positions, exact helper ownership and signatures, carrier types, arms, local typing, and
continuation. Evaluation and Core-only Go preserve once-only source order, complete-carrier
validation, and selected-arm-only evaluation. Public compiler, semantic, and Application IR schema
identities and shapes remain unchanged; metadata advances to `v0.52.0`; the exact 45-source lane is
frozen. Docker observability proves the cumulative third helper receives `selected, confirmed,
rows, id` before normalization.

All inherited forms remain exact. Fewer than three cumulative pairs, partial/reordered/repeated/
omitted/extra prior selections, mixed modes, gaps, non-chain dependencies, computed arguments,
matches outside the exact chain, nested/terminal/argument matches, propagation changes, arbitrary
Result widening, statements, effects, actions, runtimes, targets, adapters, UI, and deployment
remain excluded. No behavior enters by implication.

## Checkpoint v0.53.0 complete contract

One public pure method with at least two parameters may use exactly `C carrier = Helper(p1, ...,
pn); T value = propagate(carrier); return admittedExpression;`. The helper call is the first typed
local initializer; the propagation local is immediately adjacent; and every caller parameter is
passed directly once in declaration order. The helper is a uniquely resolved public pure
same-class method. `C` equals the method return carrier, `T` equals its success payload, and the
carrier remains Optional primitive/record, `Result<List<R>, string>`, or
`Result<string, string>`. The inherited one-parameter v0.42 form remains exact.

Typed HIR and Core reuse existing nodes. Core independently verifies direct parameter positions,
helper ownership/signature, adjacency, carrier/payload types, and exactly one propagation.
Evaluation and Core-only Go call the helper once, validate the full carrier, copy the payload on
success, and return canonical absence/failure otherwise. Public compiler, semantic, and
Application IR identities and shapes remain unchanged; metadata advances to `v0.53.0`; the exact
45-source lane is frozen. Docker observability proves `ResolveSelection(rows, id)` without
Application IR schema change.

Computed/reordered/repeated/omitted/extra arguments, a later or split carrier pair, a non-carrier
helper result, extra propagation, cross-owner/private/overloaded/generic helpers, arbitrary Result
widening, inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI,
and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.54.0 complete contract

One public pure method uses exactly `Result<int, ArithmeticError> carrier = Helper(p1, ..., pn);
int value = propagate(carrier); return admittedCheckedArithmeticExpression;`, or the identical
`float` form. The helper call is the first typed local, propagation is the immediately adjacent
second local, and every caller parameter is passed directly once in declaration order to one
uniquely resolved public pure same-class helper. The helper carrier equals the method return type
and the propagated local equals its success payload.

Typed HIR and target-neutral Core reuse existing immutable-local, call, reference, propagation,
and arithmetic nodes. Core independently verifies the exact pair, parameter positions, helper
identity/signature, carrier/payload types, one propagation, and checked continuation. Evaluation
and deterministic Core-only Go call the helper once, validate the complete arithmetic Result, copy
success, and return canonical overflow or division-by-zero before the continuation. Public
compiler, semantic, and Application IR identities and shapes remain unchanged; metadata advances
to `v0.54.0`; a compiler-cursor consumer proves checked offset propagation; the exact 45-source
lane remains frozen.

Direct-parameter arithmetic propagation, call-inside-propagate, later or split pairs, computed,
reordered, repeated, omitted, or extra helper arguments, additional propagation, arbitrary Result
widening, inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI,
and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.55.0 complete contract

One public pure method uses exactly `Result<int, ArithmeticError> Continue(Result<int,
ArithmeticError> carrier) { int value = propagate(carrier); return
admittedCheckedArithmeticExpression; }`, or the identical `float` form. The carrier is the sole
direct parameter and exactly equals the method return type. Propagation is the first typed local,
its payload local is exactly `int` or `float`, and the continuation is one already admitted checked
add, subtract, multiply, negate, or binary64 divide expression of that payload type.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and
arithmetic nodes. Core independently verifies the first-local placement, sole direct parameter,
carrier/payload equality, one propagation, and checked continuation. Evaluation and deterministic
Core-only Go validate the complete arithmetic Result, copy success, and return canonical overflow
or division-by-zero before the continuation. Public compiler, semantic, and Application IR
identities and shapes remain unchanged; metadata advances to `v0.55.0`; a compiler-cursor fixture
proves consumption of a checked cursor Result supplied by its caller; the exact 45-source lane
remains frozen.

Additional parameters, helper or computed operands, later or split propagation, additional
propagation, mismatched carrier/payload types, arbitrary Result widening, inference, reassignment,
statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. The
v0.54 helper form and every earlier contract remain exact. No behavior enters by implication.

## Checkpoint v0.56.0 complete contract

One public pure method additionally uses exactly `Result<int, ArithmeticError> Advance(Result<int,
ArithmeticError> carrier, int operand) { int value = propagate(carrier); return value + operand; }`.
The integer operator may be add, subtract, or multiply. The identical `float` form admits only
binary64 divide. The arithmetic Result is the first parameter and equals the return type; the
second and only other parameter exactly equals its payload. Propagation is the first typed local,
and the terminal expression uses that local as its left operand and the second parameter as its
right operand.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and
checked-arithmetic nodes. Core independently validates parameter order/count/types, first-local
placement, the direct carrier reference, exact operands, one propagation, and the operator matrix.
Evaluation and deterministic Core-only Go validate the complete carrier, copy success, preserve
canonical incoming failure without evaluating the continuation, and produce canonical overflow or
division-by-zero from the continuation. Public compiler, semantic, and Application IR identities
and shapes remain unchanged; metadata advances to `v0.56.0`; a compiler-cursor fixture proves
checked cursor plus width advancement; the exact 45-source lane remains frozen.

The inherited v0.55 sole-carrier and v0.54 helper forms remain exact. A third parameter, reordered
carrier/operand, mismatched operand type, reversed/repeated/literal/computed operands, unary
negation as the new two-parameter form, helper or computed propagation operands, later/split or
additional propagation, arbitrary Result widening, inference, reassignment, statements, effects,
actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior enters by
implication.

## Checkpoint v0.57.0 complete contract

One public pure method additionally uses exactly `Result<int, ArithmeticError> AdvanceTwice(
Result<int, ArithmeticError> carrier, int first, int second) { int value = propagate(carrier);
Result<int, ArithmeticError> nextCarrier = value + first; int next = propagate(nextCarrier);
return next + second; }`. Each integer checked stage independently admits add, subtract, or
multiply. The identical `float` form admits binary64 divide at both stages. The carrier is the
first parameter and method return; the second and third parameters exactly equal its payload.

Typed HIR and target-neutral Core reuse the existing immutable-local, reference, propagation, and
checked-arithmetic nodes. Core independently validates parameter order/count/types, both direct
carrier references, all three local positions/types, the two exact local/parameter operand pairs,
exactly two propagation points, and the operator matrix. Evaluation and deterministic Core-only Go
validate the incoming carrier, copy success, evaluate and validate the intermediate checked Result
once, copy its success, and then evaluate the terminal checked operation. Incoming failure and
first-stage failure return before any later stage; terminal overflow or division-by-zero remains
canonical. Public compiler, semantic, and Application IR identities and shapes remain unchanged;
metadata advances to `v0.57.0`; a compiler-cursor fixture proves two checked width advances; the
exact 45-source lane remains frozen.

The inherited v0.54-v0.56 forms remain exact. A fourth parameter, reordered or mismatched
parameters, reversed/repeated/literal/computed operands, a missing or additional carrier/local/
propagation stage, direct propagation of a computed expression, helper propagation, arbitrary
Result widening, inference, reassignment, statements, effects, actions, runtimes, targets,
adapters, UI, and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.58.0 complete contract

One public pure method additionally uses a contiguous checked-propagation chain of `K >= 2`
stages. Its exact source shape is `Result<T, ArithmeticError> F(Result<T, ArithmeticError>
carrier, T operand1, ..., T operandK)`, followed by `T value0 = propagate(carrier);` and, for every
non-terminal stage `i`, the adjacent pair `Result<T, ArithmeticError> carrierI = valueI-1 opI
operandI; T valueI = propagate(carrierI);`; the terminal return is `valueK-1 opK operandK`.
`T` is exactly `int` or `float`. Every integer stage independently admits add, subtract, or
multiply; every float stage admits binary64 divide only.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and
checked-arithmetic nodes. Core independently validates the parameter sequence, exact contiguous
alternating local pairs, direct preceding-payload/local and matching parameter operands, propagation
count, carrier types, and operator matrix for arbitrary admitted chain length. Evaluation and
deterministic Core-only Go validate every complete carrier once, copy each success, and return a
canonical incoming or intermediate failure before any later stage; the terminal checked failure is
preserved. Public compiler, semantic, and Application IR identities and shapes remain unchanged;
metadata advances to `v0.58.0`; a compiler-cursor fixture proves three checked advances and the
Docker-observability Application IR consumer advances metadata only; the exact 45-source lane
remains frozen.

The inherited v0.54-v0.57 forms remain exact. Fewer than two stages, a missing or additional local
inside the chain, ordinary-local gaps, reordered/mismatched parameters, reversed/repeated/literal/
computed operands, direct propagation of a computed expression, helper propagation, arbitrary
Result widening, inference, reassignment, statements, effects, actions, runtimes, targets,
adapters, UI, and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.59.0 complete contract

One public pure method additionally takes exactly one direct `Result<T, string>` parameter and
returns `Result<U, string>`, where `T != U` and each payload is exactly `string` or `List<R>` for an
existing public primitive-field record. Its body is exactly `T value = propagate(carrier); return
Helper(value);`. The propagation initializes the first and only typed local; the terminal helper is
one resolved public pure same-class method with exact signature `T -> Result<U, string>` and receives
only the direct propagated local.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates the source and target bounded Results, distinct payloads,
shared string failure type, direct parameter/local positions and types, sole propagation, terminal
call placement, direct local argument, same-owner callable identity, and exact helper signature.
Evaluation and deterministic Core-only Go validate the complete input carrier once, copy success,
invoke and validate the helper once, or on failure skip the helper and construct the canonical
target-shaped failure with copied validated error text and zero target payload. Public compiler,
semantic, and Application IR identities and shapes remain unchanged; metadata advances to
`v0.59.0`; a compiler-pipeline fixture proves text-to-record-list, record-list-to-text, and
record-list-to-distinct-record-list forms; the Docker-observability Application IR consumer advances
metadata only; the exact 45-source lane remains frozen.

The inherited v0.54-v0.58 forms remain exact. Same-payload propagation receives no new spelling.
Arbitrary error types, Optional or arithmetic carriers, extra parameters/locals/propagations,
computed carriers, `propagate(Helper(...))`, helper propagation, non-direct helper arguments,
private/cross-class/mismatched/overloaded/generic helpers, inference, reassignment, statements,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication.

## Checkpoint v0.60.0 complete contract

One public pure method additionally takes exactly one direct `Result<T, string>` parameter and
returns `Result<V, string>`. Its body is exactly `T first = propagate(carrier);
Result<U, string> nextCarrier = First(first); U second = propagate(nextCarrier); return
Second(second);`. `T`, `U`, and `V` are each exactly `string` or `List<R>` for an existing public
primitive-field record. Adjacent payloads differ (`T != U` and `U != V`); `T` may equal `V`.
`First` and `Second` are resolved public pure same-class helpers with exact signatures
`T -> Result<U, string>` and `U -> Result<V, string>` and receive only their direct preceding
payload locals.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates the sole carrier, three exact local positions and types, two
direct propagations, explicit intermediate carrier, adjacent payload inequality, shared string
failure type, terminal call placement, same-owner callable identities, and both exact helper
signatures. Evaluation and deterministic Core-only Go validate and copy each complete carrier once.
Incoming failure skips both helpers; intermediate failure skips `Second`; both construct canonical
target-shaped failures preserving copied validated error text. Public compiler, semantic, and
Application IR identities and shapes remain unchanged; metadata advances to `v0.60.0`; a
compiler-pipeline fixture proves text-to-list-to-distinct-list plus list-to-text-to-original-list;
the Docker-observability Application IR consumer advances metadata only; the exact 45-source lane
remains frozen.

The inherited v0.54-v0.59 forms remain exact. General `K`-stage Result chains, same-payload adjacent
stages, extra parameters or locals, computed carriers, `propagate(Helper(...))`, helper propagation,
non-direct helper arguments, arbitrary failure types, Optional/arithmetic carriers,
private/cross-class/mismatched/overloaded/generic helpers, inference, reassignment, statements,
branches, loops, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded.
No behavior enters by implication.

## Checkpoint v0.61.0 complete contract

One public pure method additionally admits a contiguous bounded cross-payload Result chain of
`K >= 2` stages. It takes exactly one direct `Result<T0, string>` parameter and returns
`Result<TK, string>`. The body begins with `T0 value0 = propagate(carrier);`. Every non-terminal
stage `i` is the adjacent pair `Result<Ti, string> carrierI = HelperI(valueI-1); Ti valueI =
propagate(carrierI);`; the terminal return is `HelperK(valueK-1)`. Every payload is exactly
`string` or `List<R>` for an existing public primitive-field record. Adjacent payloads differ;
non-adjacent payloads may match. Every helper is resolved, public, pure, same-class, and has the
exact adjacent `Ti-1 -> Result<Ti, string>` signature.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates the sole direct carrier, bounded payloads, shared string
failure, arbitrary admitted chain length, contiguous alternating local pairs, direct preceding
carrier and payload references, local positions and types, adjacent payload inequality, callable
owners, and exact helper signatures. Evaluation and deterministic Core-only Go validate and copy
every complete carrier once. Any incoming or intermediate failure skips all later helpers and
constructs the canonical final-target-shaped failure with preserved copied error text. Public
compiler, semantic, and Application IR identities and shapes remain unchanged; metadata advances
to `v0.61.0`; a four-stage compiler-pipeline fixture and metadata-only Docker-observability
Application IR consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.60 forms remain exact. Fewer than two stages in the generalized form,
same-payload adjacent stages, missing/additional/gapped locals, extra parameters, computed carriers,
`propagate(Helper(...))`, helper propagation, non-direct helper arguments, arbitrary failure types,
Optional/arithmetic carriers, private/cross-class/mismatched/overloaded/generic helpers, inference,
reassignment, statements, branches, loops, effects, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.62.0 complete contract

One public pure method additionally admits the v0.61 generalized cross-payload chain with exactly
one second direct `string` context parameter. Its signature is `Result<TK, string>
F(Result<T0, string> carrier, string context)`. The first local directly propagates `carrier`.
Every helper has the exact signature `(Ti-1, string) -> Result<Ti, string>` and receives the
immediately preceding payload local first and the unchanged direct `context` parameter second.
The chain still has `K >= 2` stages; every non-terminal helper Result remains an explicit local
immediately followed by its direct propagation local.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates the two exact caller parameters, direct context identity and
string type, arbitrary admitted chain length, contiguous alternating locals, direct preceding
payload and context arguments, bounded payloads, shared string failure, adjacent payload
inequality, callable owners, and exact helper signatures. Evaluation and deterministic Core-only Go
validate and copy every complete carrier once and pass the validated context once to every invoked
helper. Any incoming or intermediate failure skips every later helper and produces the canonical
final-target-shaped failure with preserved copied error text. Public compiler, semantic, and
Application IR identities and shapes remain unchanged; metadata advances to `v0.62.0`; a
four-stage contextual compiler-pipeline fixture and metadata-only Docker-observability Application
IR consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.61 forms remain exact. A missing, reordered, repeated, computed, non-string,
stage-specific, or additional context argument; a third caller parameter; fewer than two stages;
same-payload adjacent stages; missing/additional/gapped locals; computed carriers;
`propagate(Helper(...))`; helper propagation; arbitrary failure types; Optional/arithmetic carriers;
private/cross-class/mismatched/overloaded/generic helpers; inference; reassignment; statements;
branches; loops; effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded.
No behavior enters by implication.

## Checkpoint v0.63.0 complete contract

One public pure method additionally admits exactly
`Result<U, string> F(Result<T, string> carrier, string context) { T value = propagate(carrier);
return Helper(value, context); }`. `T` and `U` are distinct and each remains exactly `string` or
`List<R>` for an existing public primitive-field record. `Helper` is one resolved public pure
same-class method with exact signature `(T, string) -> Result<U, string>`. The propagated local and
unchanged direct string context are its only arguments and remain in that order.

Typed HIR and target-neutral Core reuse `immutable_local`, `reference`, `propagate`, and `call`.
Core independently verifies the two exact caller parameters, single first-local propagation,
bounded distinct payloads, shared string failure, direct carrier/local/context identities, helper
ownership, callable identity, and exact signature. Evaluation and deterministic Core-only Go
validate and copy the complete incoming carrier once, skip the helper on failure, reshape failure
to the canonical target Result, and pass validated context once on success. Public compiler,
semantic, and Application IR identities and shapes remain unchanged; metadata advances to
`v0.63.0`; text-to-list and list-to-text compiler-pipeline fixtures plus metadata-only
Docker-observability Application IR consumption prove the boundary; the exact 45-source lane
remains frozen.

The inherited v0.54-v0.62 forms remain exact. Same-payload flow; a missing, reordered, repeated,
computed, non-string, or additional context; a third caller parameter; extra locals or propagation;
computed carriers; `propagate(Helper(...))`; helper propagation; arbitrary failure types;
Optional/arithmetic carriers; private/cross-class/mismatched/overloaded/generic helpers; inference;
reassignment; statements; branches; loops; effects; actions; runtimes; targets; adapters; UI; and
deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.64.0 complete contract

One public pure method additionally admits exactly
`Result<T, string> F(Result<T, string> carrier, string context) { T value = propagate(carrier);
return Helper(value, context); }`. `T` remains exactly `string` or `List<R>` for an existing public
primitive-field record. `Helper` is one resolved public pure same-class method with exact signature
`(T, string) -> Result<T, string>`. The propagated local and unchanged direct string context are its
only arguments and remain in that order.

Typed HIR and target-neutral Core reuse `immutable_local`, `reference`, `propagate`, and `call`.
Core independently verifies the two exact caller parameters, single first-local propagation,
bounded equal payloads, shared string failure, direct carrier/local/context identities, helper
ownership, callable identity, and exact signature. Evaluation and deterministic Core-only Go
validate and copy the complete incoming carrier once, skip the helper on failure, return canonical
same-shaped failure, and pass validated context once on success. Public compiler, semantic, and
Application IR identities and shapes remain unchanged; metadata advances to `v0.64.0`; string and
record-list compiler-pipeline fixtures plus metadata-only Docker-observability Application IR
consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.63 forms remain exact. Same-payload contextual chains with two or more
stages; a missing, reordered, repeated, computed, non-string, or additional context; a third caller
parameter; extra locals or propagation; computed carriers; `propagate(Helper(...))`; helper
propagation; arbitrary failure types; Optional/arithmetic carriers; private/cross-class/mismatched/
overloaded/generic helpers; inference; reassignment; statements; branches; loops; effects; actions;
runtimes; targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.65.0 complete contract

One public pure method additionally admits exactly
`Result<T2, string> F(Result<T0, string> carrier, string context) { T0 first =
propagate(carrier); Result<T1, string> nextCarrier = First(first, context); T1 second =
propagate(nextCarrier); return Second(second, context); }`. `T0`, `T1`, and `T2` each remain exactly
`string` or `List<R>` for an existing public primitive-field record. Either or both adjacent
payload pairs may be equal. Both helpers are resolved public pure same-class methods with exact
signatures `(T0, string) -> Result<T1, string>` and `(T1, string) -> Result<T2, string>`.

Typed HIR and target-neutral Core reuse `immutable_local`, `reference`, `propagate`, and `call`.
Core independently verifies the two exact caller parameters, exact three-local/two-stage shape,
bounded payloads, shared string failure, direct carrier/local/context identities, helper ownership,
callable identities, and exact signatures. Evaluation and deterministic Core-only Go validate and
copy carriers once, short-circuit each failure into the canonical final Result shape, and pass the
unchanged validated context once per reached helper. Public compiler, semantic, and Application IR
identities and shapes remain unchanged; metadata advances to `v0.65.0`; compiler-pipeline fixtures
plus metadata-only Docker-observability Application IR consumption prove the boundary; the exact
45-source lane remains frozen.

The inherited v0.54-v0.64 forms remain exact. Same-payload transitions in contextual chains with
three or more stages; a missing, reordered, repeated, computed, non-string, or additional context;
a third caller parameter; extra or gapped locals; additional propagation; computed carriers;
`propagate(Helper(...))`; arbitrary failure types; Optional/arithmetic carriers; private,
cross-class, mismatched, overloaded, or generic helpers; inference; reassignment; statements;
branches; loops; effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded.
No behavior enters by implication.

## Checkpoint v0.66.0 complete contract

One public pure method additionally admits a contiguous contextual bounded Result chain of
`K >= 2` stages in which any adjacent payloads may be equal or different. Its exact signature is
`Result<TK, string> F(Result<T0, string> carrier, string context)`. The body begins with direct
propagation of `carrier`; every non-terminal helper Result is stored in an explicit local immediately
followed by its direct propagation local; the terminal expression is the final helper call. Every
payload remains exactly `string` or `List<R>` for an existing public primitive-field record. Every
helper is resolved, public, pure, same-class, and exact
`(Ti, string) -> Result<Ti+1, string>`, receiving the immediately preceding payload and unchanged
direct context in that order.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently verifies the two exact caller parameters, arbitrary admitted chain
length, contiguous alternating locals, bounded payloads, shared string failure, direct
carrier/payload/context identities, helper ownership, callable identities, and exact signatures.
Evaluation and deterministic Core-only Go validate and copy every reached carrier once, preserve
the unchanged validated context, and short-circuit every failure into the canonical final Result
shape. Public compiler, semantic, and Application IR identities and shapes remain unchanged;
metadata advances to `v0.66.0`; mixed and all-equal compiler-pipeline fixtures plus metadata-only
Docker-observability Application IR consumption prove the boundary; the exact 45-source lane
remains frozen.

The inherited v0.54-v0.65 forms remain exact. Fewer than two contextual helper stages; a missing,
reordered, repeated, computed, non-string, stage-specific, or additional context; a third caller
parameter; missing, additional, or gapped locals; computed carriers; `propagate(Helper(...))`;
arbitrary failure types; Optional/arithmetic carriers; private, cross-class, mismatched, overloaded,
or generic helpers; inference; reassignment; statements; branches; loops; effects; actions;
runtimes; targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.67.0 complete contract

One public pure method additionally admits a contiguous contextual bounded Result chain with
`K >= 2` helper stages and `N >= 1` direct `string` context parameters. Its signature is
`Result<TK, string> F(Result<T0, string> carrier, string c1, ..., string cN)`. The body begins with
direct propagation of `carrier`; every non-terminal helper Result remains an explicit local
immediately followed by direct propagation; the terminal expression remains the final helper call.
Every payload is exactly `string` or `List<R>` for an existing public primitive-field record. Every
helper is resolved, public, pure, same-class, and exact `(Ti, string...) -> Result<Ti+1, string>`,
receiving the immediately preceding payload and every unchanged direct context in declaration
order.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently verifies arbitrary admitted chain and context counts, contiguous
alternating locals, bounded payloads, shared string failure, direct carrier/payload/context
identities, helper ownership, callable identities, and exact signatures. Evaluation and
deterministic Core-only Go validate and copy every reached carrier once, preserve every validated
context, and short-circuit every failure into the canonical final Result shape. Public compiler,
semantic, and Application IR identities and shapes remain unchanged; metadata advances to
`v0.67.0`; multi-context compiler-pipeline fixtures plus metadata-only Docker-observability
Application IR consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.66 forms remain exact. No-context contextual chains; one-stage multi-context
chains; missing, reordered, repeated, computed, non-string, or stage-specific contexts; missing,
additional, or gapped locals; computed carriers; `propagate(Helper(...))`; arbitrary failure types;
Optional/arithmetic carriers; private, cross-class, mismatched, overloaded, or generic helpers;
inference; reassignment; statements; branches; loops; effects; actions; runtimes; targets; adapters;
UI; and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.68.0 complete contract

One public pure method additionally admits the one-stage contextual bounded Result form with one
direct bounded Result carrier followed by `N >= 1` direct `string` context parameters. Its exact
shape is `Result<T1, string> F(Result<T0, string> carrier, string c1, ..., string cN) { T0 value =
propagate(carrier); return Helper(value, c1, ..., cN); }`. `T0` and `T1` may match or differ and each
remains exactly `string` or `List<R>` for an existing public primitive-field record. The helper is
resolved, public, pure, same-class, and exact `(T0, string...) -> Result<T1, string>`.

Typed HIR and target-neutral Core reuse `immutable_local`, `reference`, `propagate`, and `call`.
Core independently verifies arbitrary admitted context count, the complete ordered direct context
vector, the single propagation/local placement, bounded payloads, shared string failure, helper
ownership, callable identity, and exact signature. Evaluation and deterministic Core-only Go
validate direct inputs, copy the reached carrier once, pass every context unchanged, and reshape
incoming failure into the canonical target Result before the helper can run. Public compiler,
semantic, and Application IR identities and shapes remain unchanged; metadata advances to
`v0.68.0`; one-stage multi-context compiler-pipeline fixtures plus metadata-only Docker-
observability Application IR consumption prove the boundary; the exact 45-source lane remains
frozen.

The inherited v0.54-v0.67 forms remain exact. Missing, reordered, repeated, computed, non-string, or
stage-specific contexts; additional or gapped locals; computed carriers; `propagate(Helper(...))`;
arbitrary failure types; Optional/arithmetic carriers; private, cross-class, mismatched, overloaded,
or generic helpers; inference; reassignment; statements; branches; loops; effects; actions;
runtimes; targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

## Checkpoint v0.69.0 complete contract

One public pure method additionally admits one or more existing ordered immutable locals followed by
exactly one terminal statement-level `if/else`. The exact terminal shape is
`if (condition) { return whenTrue; } else { return whenFalse; }`: `condition` is `bool`, and both
branch values have the declared method return type. Locals, the condition, and both branch values
remain already-admitted eager pure expressions and calls. At most one preceding local initializer
may use the inherited bounded conditional expression.

Typed HIR and target-neutral Core reuse `immutable_local` and `conditional`, with an explicit
terminal-statement marker that is preserved through projection. Core independently verifies the
required preceding local, unique terminal conditional, boolean condition, matching branch types,
and absence of propagation or matching in the method. Evaluation and deterministic Core-only Go
reuse existing conditional semantics and evaluate only the selected branch. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language metadata advances to `v0.69.0`. Terminal-if compiler-pipeline fixtures and metadata-only
Docker-observability Application IR consumption prove the boundary; the exact 45-source lane
remains frozen.

The inherited v0.54-v0.68 forms remain exact. A zero-local terminal conditional; branch locals;
nested conditionals; missing `else`; fallthrough or returns elsewhere; propagation or matching in
the method; assignment, reassignment, shadowing, or inference; loops; effects; actions; runtimes;
targets; adapters; UI; and deployment remain excluded. No behavior enters by implication. Any
successor requires a new founder decision and separate implementation approval.

## Checkpoint v0.70.0 complete contract

One public pure method additionally admits at most one explicitly typed immutable local in either
terminal `if/else` branch. The method still begins with one or more top-level ordered immutable
locals and ends in the exact v0.69 terminal conditional. A branch is either `return expression;`
or `Type name = expression; return expression;`. Both branches may use the local form; their scopes
are independent and may reuse the same name. A branch initializer runs only when selected, and its
binding cannot escape the branch. Conditions remain `bool`, explicit types remain exact, and only
previously admitted eager pure expressions and calls are available.

Typed HIR and target-neutral Core reuse `immutable_local` under the existing terminal
`conditional`; Core independently verifies one root local at most in each branch, canonical binding
positions, lexical references, matching return types, and the existing terminal placement.
Evaluation and deterministic Core-only Go preserve selected-branch evaluation and lexical scope.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain stable; only
language metadata advances to `v0.70.0`. Compiler pipeline, malformed-Core, generated-Go,
editor, and metadata-only Application IR fixtures prove the boundary; the exact 45-source lane
remains frozen.

The inherited v0.69 direct-return form remains exact. Nested branches, multiple locals per branch,
escaping bindings, propagation, matching, assignment, fallthrough, other early returns, loops,
effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No
behavior enters by implication. Any successor requires a new founder decision and separate
implementation approval.

## Checkpoint v0.71.0 complete contract

One public pure method additionally admits at most two explicitly typed ordered immutable locals
inside either terminal `if/else` branch. One or more top-level ordered immutable locals and the
exact v0.69 terminal placement remain required. A second branch local may reference the first;
opposing branches remain independent lexical scopes, may reuse names and canonical binding
positions, evaluate only when selected, and cannot leak bindings.

Typed HIR and target-neutral Core reuse nested `immutable_local` expressions inside the existing
terminal `conditional`. Core independently verifies a maximum of two root branch locals, exact
types, canonical positions, lexical references, and terminal placement. Evaluation and
deterministic Core-only Go preserve source order and lazy branch selection.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain stable;
metadata advances to `v0.71.0`; compiler-pipeline, malformed-Core, generated-Go, editor, and
metadata-only Application IR fixtures prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.69 direct-return and v0.70 one-local forms remain exact. A third branch local,
nested branches, zero top-level locals, escaping bindings, propagation, matching, assignment,
fallthrough, other early returns, loops, effects, inference, actions, runtimes, targets, adapters,
UI, and deployment remain excluded. No behavior enters by implication. Any successor requires a
new founder decision and separate implementation approval.

## Checkpoint v0.72.0 complete contract

One public pure method additionally admits any finite source-ordered sequence of explicitly typed
immutable locals inside either terminal `if/else` branch. One or more top-level ordered immutable
locals and the exact v0.69 terminal placement remain required. Each branch local enters scope only
after its initializer; later locals may reference earlier locals in that branch. Self-reference,
forward reference, duplicate names, and shadowing remain invalid. Opposing branches retain
independent lexical scopes, may reuse names and canonical binding positions, evaluate only when
selected, and cannot leak bindings.

Typed HIR and target-neutral Core reuse nested `immutable_local` expressions inside the existing
terminal `conditional`. Core independently verifies the complete root sequence, exact types,
canonical positions, lexical references, and terminal placement. Evaluation and deterministic
Core-only Go preserve source order and lazy branch selection. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` remain stable; metadata advances to
`v0.72.0`; compiler-pipeline, malformed-Core, generated-Go, editor, and metadata-only Application
IR fixtures prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.69 direct-return, v0.70 one-local, and v0.71 two-local forms remain exact. Nested
branches, zero top-level locals, escaping bindings, propagation, matching, assignment, fallthrough,
other early returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No behavior enters by implication. Any successor requires a new
founder decision and separate implementation approval.

## Checkpoint v0.73.0 complete contract

One public pure method additionally admits the existing terminal statement-level `if/else` as its
complete body, without a preceding top-level immutable local. Either branch may return directly or
retain any finite source-ordered sequence of explicitly typed immutable locals followed by return.
Each local enters scope only after its initializer; later locals may reference earlier locals within
the same branch. Opposing branches remain independent lexical scopes, may reuse names and canonical
binding positions, evaluate only when selected, and cannot leak bindings.

Typed HIR and target-neutral Core reuse the existing terminal `conditional` root plus nested
`immutable_local` expressions; no node or schema identity changes. Core independently validates
the root terminal placement, complete branch sequences, exact types, canonical positions, lexical
references, and absence of nested branching. Evaluation and deterministic Core-only Go preserve
source order and selected-branch execution. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` remain stable; metadata advances to `v0.73.0`; compiler-pipeline,
malformed-Core, generated-Go, editor, and metadata-only Application IR fixtures prove the boundary;
the exact 45-source lane remains frozen.

The inherited v0.69-v0.72 forms with top-level locals remain exact. Ordinary zero-local blocks,
nested branches, escaping bindings, propagation, matching, assignment, fallthrough, other early
returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No behavior enters by implication. Any successor requires a new founder decision and
separate implementation approval.

## Checkpoint v0.74.0 complete contract

One public pure method additionally admits exactly one nested terminal decision at the end of
exactly one outer branch of a complete terminal `if/else` body. Zero or more explicitly typed
immutable locals may precede the inner decision in that selected outer branch. The sibling outer
branch retains the v0.73 direct-or-local sequence; both inner leaves return directly. Both
conditions are `bool`, every leaf has the exact declared return type, and outer-branch locals are
lexically visible to the inner condition and leaves after their initializer.

Typed HIR and target-neutral Core reuse the existing terminal `conditional` and nested
`immutable_local` expressions; no node or schema identity changes. Core independently validates
exactly two terminal conditionals, nesting in exactly one outer branch, direct inner leaves, exact
types, canonical positions, lexical references, and the excluded topology. Evaluation and
deterministic Core-only Go preserve source order and selected-branch execution at both levels.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain stable;
metadata advances to `v0.74.0`; compiler-pipeline, malformed-Core, generated-Go, editor, and
metadata-only Application IR fixtures prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.69-v0.73 forms remain exact. Top-level locals for the nested topology, inner
locals, nesting in both outer branches, another nested decision, third-level nesting, conditional
expressions within the topology, propagation, matching, assignment, fallthrough, other early
returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No behavior enters by implication. Any successor requires a new founder decision and
separate implementation approval.

## Checkpoint v0.75.0 complete contract

One public pure method additionally permits either inner leaf of the exact v0.74 nested terminal
topology to contain any finite source-ordered sequence of explicitly typed immutable locals before
its return. Each local enters scope only after its initializer; later locals may reference earlier
locals in the same inner leaf, while self-reference, forward reference, duplicate names, shadowing,
cross-leaf references, and escaping bindings remain invalid. Outer-branch locals remain visible to
the inner condition and both leaves. Both conditions remain `bool`, and all returns retain the exact
declared method type.

Typed HIR and target-neutral Core reuse the existing terminal `conditional` and nested
`immutable_local` expressions; no node or schema identity changes. Core independently validates
the exact two-conditional topology, nesting in exactly one outer branch, complete inner-leaf local
sequences, exact types, canonical positions, lexical references, and the excluded topology.
Evaluation and deterministic Core-only Go preserve source order and selected-branch execution at
both levels. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain
stable; metadata advances to `v0.75.0`; compiler-pipeline, malformed-Core, generated-Go, editor, and
Application IR consumer fixtures prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.69-v0.74 forms remain exact. Top-level locals for the nested topology, nesting in
both outer branches, another nested decision, third-level nesting, conditional expressions within
the topology, propagation, matching, assignment, fallthrough, other early returns, loops, effects,
inference, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication. Any successor requires a new founder decision and separate implementation
approval.

## Checkpoint v0.76.0 complete contract

One public pure method additionally permits one or more source-ordered explicitly typed immutable
locals before the exact v0.75 one-branch bounded nested terminal topology. Each root local enters
scope only after its initializer, evaluates eagerly once before the outer condition, and remains
visible to that condition, both outer branches, and every descendant branch. Self-reference,
forward reference, duplicate names, shadowing, and escaping bindings remain invalid. Both
conditions remain `bool`, and all returns retain the exact declared method type.

Typed HIR and target-neutral Core reuse the existing terminal `conditional` and nested
`immutable_local` expressions; no node or schema identity changes. Core independently validates
the root sequence, exact two-conditional topology, nesting in exactly one outer branch, complete
branch-local sequences, exact types, canonical positions, lexical references, and excluded shapes.
Evaluation and deterministic Core-only Go preserve eager source order before selected-branch
execution. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain
stable; metadata advances to `v0.76.0`; compiler-pipeline, malformed-Core, generated-Go, editor,
and Application IR consumer fixtures prove the boundary; the exact 45-source lane remains frozen.

The inherited rootless v0.75 and v0.69-v0.74 forms remain exact. Nesting in both outer branches,
another nested decision, third-level nesting, conditional expressions within the topology or
root-local nested form, propagation, matching, assignment, fallthrough, other early returns,
loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No behavior enters by implication. Any successor requires a new founder decision and
separate implementation approval.

## Checkpoint v0.77.0 complete contract

One public pure method additionally permits one or more source-ordered explicitly typed immutable
locals before an outer terminal `if/else` whose two branches each end in exactly one inner terminal
`if/else`. Each outer branch and each inner leaf may retain any finite source-ordered sequence of
explicitly typed immutable locals. Root bindings remain visible throughout; outer-branch bindings
remain visible only to that branch's inner condition and leaves; inner-leaf bindings remain local
to that leaf. All three conditions are `bool`, and every return has the exact method return type.

Typed HIR and target-neutral Core reuse the existing terminal `conditional` and nested
`immutable_local` expressions; no node or schema identity changes. Core independently validates
the required root sequence, exact three-conditional symmetric topology, complete branch-local
sequences, exact types, canonical positions, lexical references, and excluded shapes. Evaluation
and deterministic Core-only Go preserve eager root order and selected-branch execution.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain stable;
metadata advances to `v0.77.0`; compiler-pipeline, malformed-Core, generated-Go, editor, and
Application IR consumer fixtures prove the boundary; the exact 45-source lane remains frozen.

All v0.69-v0.76 forms remain exact. The symmetric topology without a root local, third-level or
additional nesting, conditional expressions within the topology, propagation, matching,
assignment, fallthrough, other early returns, loops, effects, inference, actions, runtimes,
targets, adapters, UI, and deployment remain excluded. No behavior enters by implication. Any
successor requires a new founder decision and separate implementation approval.

## Application IR checkpoint complete contract

`dockpipe.application.v1` consumes the canonical public semantic projection plus the matching Core
program and an explicit source-located stable-identity spec. It projects typed snapshot, section
Result, row/key/column, optional selection/details, filtering, ordering, and contract metadata into
deterministic JSON. Validation rejects missing identities, mismatched contracts, duplicate
sections, empty columns, and invalid directions. It changes no PipeLang language, HIR, Core,
evaluator, backend, or stable identity; it adds no parsing, inference, runtime, target, Docker,
refresh, action, launcher, or CLI behavior.

## Checkpoint v0.78.0 complete contract

`v0.78.0` additionally permits the exact symmetric depth-two terminal `if/else` topology
without a preceding root immutable local. Both outer branches end in exactly one inner terminal
`if/else`. Each outer branch and each inner leaf admits zero or more source-ordered, explicitly
typed immutable locals. All three conditions are `bool`; every leaf returns exactly the declared
method type. Bindings enter scope after their initializer. Self/forward references, duplicates,
shadowing, cross-branch references, and escaping bindings remain invalid. Only the selected outer
branch, its inner condition, and its selected leaf execute.

The compiler reuses terminal `conditional` and `immutable_local` HIR/Core representations. Core
independently checks exact topology, types, lexical bindings, and canonical positions; the Go
backend refuses malformed Core. Existing rootful v0.77 and earlier accepted forms retain their
behavior. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities
and shapes remain stable; only language metadata advances. The exact 45-source compatibility
lane remains frozen.

Third-level/additional nesting, conditional expressions within this topology, propagation,
matching, assignment, fallthrough, other early returns, loops, effects, and inference remain
excluded. This slice adds no workflow, runtime, action, target, adapter, UI, or deployment behavior.

## Checkpoint v0.79.0 complete contract

`v0.79.0` additionally permits an inherited rootful v0.77 or rootless v0.78 symmetric depth-two
terminal topology to replace exactly one of its four terminal leaves with one additional terminal
`if/else`. The result has exactly four terminal conditionals and five terminal return paths. The
expanded path may contain any finite source-ordered sequence of explicitly typed immutable locals
before the third-level decision, and both new leaves may contain the same kind of local sequence.
All conditions are `bool`; every leaf returns exactly the declared method type. Root, outer-branch,
expanded-path, and leaf bindings retain their lexical descendant scopes and enter scope only after
their initializer. Only the selected path executes.

Typed HIR and target-neutral Core reuse the existing terminal `conditional` and `immutable_local`
representations. Core independently validates the exact four-conditional topology, the single
expanded leaf, all types, binding positions, lexical references, and terminal placement; the Go
backend refuses malformed Core. Existing v0.78 and earlier forms remain exact.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain stable; only language metadata advances. The exact 45-source compatibility lane remains
frozen.

A second expanded depth-two leaf, depth four or additional nesting, a non-symmetric depth-two base,
conditional expressions within the topology, propagation, matching, assignment, fallthrough,
other early returns, loops, effects, and inference remain excluded. This slice adds no workflow,
runtime, action, target, adapter, UI, or deployment behavior. Any successor requires a new founder
decision and separate implementation approval.

## Checkpoint v0.80.0 complete contract

The founder selected option A and separately approved implementation. The new form expands
exactly two of four leaves on the symmetric depth-two base: five conditionals, six return paths,
maximum depth three, optional root locals, and finite ordered typed locals in every lexical scope.
All six leaf pairs are supported. Existing versioned forms and the frozen 45-source lane remain
unchanged. See the [canonical contract](../../../concepts/pipelang.md#pipelang-v0800-two-expanded-terminal-leaves)
for semantics and exclusions.

Core independently validates topology, types, scope, and terminal placement; the backend rejects
malformed Core. HIR/Core node shapes and public compiler, semantic, and Application IR identities
remain stable. A third expansion, depth four, or an asymmetric depth-three base is not admitted.
Any successor requires a fresh founder selection and separate implementation approval.

## Checkpoint v0.81.0 complete contract

Step 8au admits any terminal binary `if/else` tree through depth three, including asymmetric
shapes, with finite typed immutable-local sequences in each scope. See the
[canonical contract](../../../concepts/pipelang.md#pipelang-v0810-terminal-trees-through-depth-three)
for exact semantics and exclusions and the [completion record](completed-functions.md#step-8au-terminal-trees-through-depth-three-v0810)
for compiler/consumer proof. Existing HIR/Core nodes and public identities remain stable.
No successor, commit, push, publication, or live operation is authorized by completion.

## Checkpoint v0.82.0 complete contract

Step 8av is complete; its [completion record](completed-functions.md#step-8av-conditional-local-in-terminal-trees-v0820)
owns passed proof and deferred findings. The slice admits one lazy conditional only as a complete typed immutable-local initializer in
an existing terminal tree through depth three. See the
[canonical contract](../../../concepts/pipelang.md#pipelang-v0820-conditional-local-in-terminal-trees)
for semantics and exclusions. Prove all 25 shapes, all lexical scopes, both arms and every path,
ordered/unused locals, malformed source/Core refusal, exact type transport, deterministic
artifacts, and Application IR consumption; then run terminal compiler/consumer/compatibility,
CLI, vet, formatting, editor, and task documentation checks. Completion does not authorize
commit, push, publication, live operations, or another slice.


## Checkpoint v0.83.0 complete contract

Step 8aw completed two conditional locals in terminal trees under the
[canonical contract](../../../concepts/pipelang.md#pipelang-v0830-two-conditional-locals-in-terminal-trees).
[two-conditional-locals.md](two-conditional-locals.md) owns scope, exclusions, and focused then
terminal compiler/consumer proof. All 25 tree shapes, lexical scope pairs, dependent selections,
ordered/unused locals, lazy arms, exact value transport, malformed source/Core rejection,
version inheritance, deterministic artifacts, and executable Application IR consumption passed with terminal verification.
