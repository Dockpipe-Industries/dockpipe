# Completed bounded function slice

## Step 8a — named pure record predicate filtering (`v0.31.0`)

The founder accepted one first bounded Step-8 function seam. Production source admits a public
same-class `bool PredicateName(R item, P1, ...)` and exact direct
`filter(List<R>, PredicateName, P1, ...) -> List<R>`. `R` is one existing public primitive record;
trailing parameters are primitive and match the direct filter operands exactly.

Predicate bodies are pure and closed over only literals, primitive predicate parameters, one-hop
public primitive fields of `item`, logical not/and/or, equality and ordering comparisons, and the
accepted `contains_casefolded` and `trim` expressions. Lambdas, closures, function values,
overloads, arbitrary calls, class state, construction, effects, async, Optional/Result predicates,
matching, propagation, Application IR, and runtime behavior remain excluded.

Typed HIR and target-neutral Core use `list_filter_predicate`, carrying the predicate method's
existing semantic identity and ordered operands. Core program validation resolves and checks that
identity. The target-neutral evaluator and Core-only Go backend validate the full input and
primitive arguments before stable one-call-per-row iteration, require bool, fail atomically, and
return canonical non-nil fresh copied output. `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. `v0.1.0` through `v0.30.0` reject the spelling without
implicit migration.

## Step 8b — same-class pure calls (`v0.36.0`)

Production source admits `Method(expression, ...)` only inside public expression-bodied methods.
The target is one uniquely named public method on the same class, resolved during semantic
analysis; ordered argument types and the return type must match the target signature exactly.
Call participants may reference parameters and match-arm bindings, not class-owned state. Calls may
nest. Direct recursion and indirect cycles fail deterministically as `PL3033`.

Typed HIR and target-neutral Core carry an explicit `call` node containing the target's existing
callable semantic identity, source name, and ordered typed operands. Lowering includes the complete
transitive dependency closure once. Core validation independently proves target presence,
same-owner identity, exact signature, and an acyclic graph. The target-neutral evaluator validates
and copies arguments into an isolated call frame. The Core-only Go backend emits ordinary calls
only after that proof; it performs no semantic lookup or inference.

The Docker observability consumer proves
`OrderContainers(FilterContainers(rows, query))` as a source-to-HIR-to-Core composition while
`dockpipe.application.v1` continues to bind its existing filter and order identities separately.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` do not change shape;
only the language-contract value advances. The exact 45-source legacy inventory remains frozen.
Cross-class/module calls, private callers or targets, overloads, generics, function values, lambdas,
recursion, blocks, locals, branches, effects, entrypoints, and target-specific behavior remain
excluded.

## Step 8c — general pure-call composition (`v0.37.0`)

Production source may use the already resolved same-class pure call node throughout expressions
that the language already admits as eager and pure, including match-arm bodies, record and Optional
construction, text operations, and arithmetic or comparison operands. Match carriers and
propagation operands remain direct references. The v0.36.0 same-class ownership, public visibility,
exact signature, participant closure, and acyclic call-graph rules remain exact.

Typed HIR and target-neutral Core reuse the existing call node and dependency closure. Core
recursively validates nested placement, target identity, signature, ownership, and cycles; the
evaluator keeps isolated copied frames, and the Go backend consumes Core only. The Docker
observability consumer proves an `Optional<ContainerRow>` match whose `some` arm calls a text
normalization helper. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` keep their schemas and advance only language-contract metadata. The
45-source legacy inventory remains exact. Cross-class/module calls, blocks, locals, branches,
loops, effects, recursion, and target-specific behavior remain excluded.

## Step 8d — bounded conditional expression (`v0.38.0`)

Production source admits `condition ? whenTrue : whenFalse` with exactly one conditional node per
method. The condition must be `bool`, both branches are fully and statically checked with exactly
the same admitted type, and evaluation executes only the selected branch. Each operand is limited
to existing eager pure expressions; nested conditionals plus match and propagation nodes inside an
operand remain excluded. Existing v0.37.0 resolved same-class pure calls retain their exact
identity, signature, ownership, closure, and cycle rules when used in an operand.

Typed HIR and target-neutral Core carry an explicit conditional node with the typed condition and
both typed branches. Core independently validates count, placement, operand exclusions, condition
type, and exact branch/result equality. The evaluator evaluates one branch, and the Core-only Go
backend emits a typed lazy choice without reconstructing semantics. The Docker observability
consumer proves `name == "" ? fallback : NormalizeName(name)` without adding an Application IR
field or rule. Public schema shapes remain unchanged, language-contract metadata advances to
`v0.38.0`, and the exact 45-source legacy inventory remains frozen. Statements, blocks, locals,
mutation, conversions, guards, effects, actions, runtime behavior, and target-specific behavior
remain excluded.

## Step 8e — immutable local plus terminal return (`v0.39.0`)

Production source admits exactly one public pure method block shaped as
`{ T name = initializer; return expression; }`. `T` is explicit; the eagerly evaluated initializer
must have exactly that type. The local enters scope only after initialization, may not shadow a
field or parameter, is immutable, and has no public semantic identity. The terminal return may use
the local, parameters, and all existing admitted eager pure expressions. An explicit
checked-arithmetic `Result` local provides arithmetic result context. Contextual propagation stays
confined to its established complete-method carrier shape and is excluded inside this block.

Typed HIR and target-neutral Core carry an explicit `immutable_local` node with one analysis-local
binding, its type, initializer, and return. Core independently validates the top-level one-node
bound, canonical scope, and exact types. The evaluator initializes and copies the value once before
evaluating the return; the Core-only Go backend emits the same typed lexical scope. The Docker
observability consumer proves an existing normalization call captured once and consumed by the
existing bounded fallback conditional. Public schema shapes remain unchanged, language-contract
metadata advances to `v0.39.0`, and the exact 45-source legacy inventory remains frozen. Inference,
multiple locals, reassignment, shadowing, propagation, early return, statement branches, nested blocks, loops,
effects, actions, runtime behavior, and target-specific behavior remain excluded.

## Step 8f — ordered immutable locals (`v0.40.0`)

Production source widens the v0.39.0 public pure method block to one or more source-ordered
explicitly typed immutable local declarations followed by one terminal return. Initializers
evaluate eagerly exactly once in source order and must exactly match their declared types. Each
local enters scope only after its initializer; later initializers may use earlier locals, while
self-reference, forward reference, duplicate names, and field/parameter shadowing fail. The
terminal expression must exactly match the method return type.

Typed HIR and target-neutral Core reuse right-nested `immutable_local` nodes with contiguous local
positions. Core independently proves that every local is part of the single top-level sequence,
rejects local nodes inside initializers or other expression positions, and validates exact types
and parameter/prior-local shadowing. The evaluator and Core-only Go backend preserve source order, once-only evaluation,
lexical scope, and copied values. The Docker observability consumer proves normalization into one
local, conditional fallback selection into a later local, and return of that later binding.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` keep their schema
identities and shapes; only language-contract metadata advances. The exact 45-source legacy lane
remains frozen. Inference, reassignment, propagation inside the block, early return, statement
branches, nested blocks, loops, effects, actions, runtime behavior, and target-specific behavior
remain excluded.

## Step 8g — block-scoped bounded propagation (`v0.41.0`)

Production source admits exactly one `T name = propagate(carrier);` declaration as the first local
of an ordered immutable-local method block. `carrier` is the method's sole direct parameter and
its carrier type exactly equals the method return type. `T` exactly equals the payload type. The
carrier matrix is limited to an `Optional` primitive or record, `Result<List<R>, string>`, or
`Result<string, string>`. Presence or success binds a validated copied payload before later locals
and the terminal return; absence or failure immediately returns the identical canonical carrier.

Typed HIR and target-neutral Core reuse the existing `propagate` and `immutable_local` nodes. Core
independently validates placement, occurrence count, direct parameter identity, exact types, and
the bounded carrier matrix. The evaluator and Core-only Go backend preserve short-circuit and
copied-value semantics. The Docker observability Application IR consumer proves the text-Result
shape end to end. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` retain their identities and schema shapes; only language-contract
metadata advances. The exact 45-source legacy lane remains frozen. Additional or nested
propagation, computed operands, propagation from prior locals, arbitrary `Result` carriers,
inference, reassignment, early returns, statement branches, nested blocks, loops, effects, actions,
runtime behavior, and target-specific behavior remain excluded.

## Step 8h — prior-local helper propagation (`v0.42.0`)

Production source admits exactly
`C carrier = Helper(input); T value = propagate(carrier);` as the first two locals of one public
pure ordered-local method block. The method has one direct parameter. The first initializer is one
resolved public same-class pure call using that parameter as its sole direct argument. The helper
return carrier `C` exactly equals the enclosing method return, and `T` exactly equals its payload.
The carrier matrix remains the v0.41.0 Optional primitive/record,
`Result<List<R>, string>`, and `Result<string, string>` matrix. Success or presence binds a
validated copied payload before later locals and the terminal return; failure or absence returns
the identical canonical helper carrier immediately.

Typed HIR and target-neutral Core reuse `call`, `immutable_local`, and `propagate`. Core validates
canonical positions, the sole direct call argument, resolved exact same-owner signature, acyclic
call closure, one propagation, the direct immediately preceding carrier-local reference, and exact
carrier/payload types. The evaluator and Core-only Go backend preserve once-only helper evaluation,
short-circuit, and copied values. The Docker observability consumer proves
`ValidateDetails(string) -> Result<string, string>` composed by `Details(string)` through
`dockpipe.application.v1`. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` retain their identities and schema shapes; only language-contract
metadata advances. The exact 45-source lane and every v0.41.0 source remain unchanged. Direct
call-inside-propagate, additional or nested propagation, computed/extra helper arguments,
propagation from another local, arbitrary Results, inference, reassignment, early returns,
statement branches, nested blocks, loops, effects, actions, runtime behavior, and target-specific
behavior remain excluded.

## Step 8i — helper-result matching composition (`v0.43.0`)

Production source admits exactly one top-level
`match(Helper(input)) { ok(value) => whenOk, err(error) => whenErr }` expression in a public pure
method with one direct `string` parameter and `string` return. The uniquely named public pure
same-class helper takes that direct parameter as its sole argument and returns exactly
`Result<string, string>`. It evaluates once; the complete carrier is validated, the selected
payload is copied into its unique binding, and only the selected arm evaluates. Arms are exact,
exhaustive, wildcard-free, and source ordered `ok` then `err`.

Typed HIR and target-neutral Core reuse `call` and `match`. Core independently validates canonical
placement, one match, the sole direct argument, exact same-owner signatures, closed acyclic calls,
the text Result carrier, ordered arm tags, and unique bindings. The evaluator and Core-only Go
backend preserve complete-carrier validation, once-only helper evaluation, copied payloads, and
lazy arm selection. The Docker observability consumer proves `DetailsMessage(string)` composing
`ValidateDetails(string)` without changing `dockpipe.application.v1`. Public compiler, semantic,
and Application IR schema identities and shapes remain unchanged; only language-contract metadata
advances. The exact 45-source lane remains frozen. Other helper carriers, extra/computed arguments,
extra caller parameters, cross-owner calls, nested matches, reversed/wildcard arms, guards, new
blocks/locals, propagation changes, inference, reassignment, statements, loops, effects, actions,
runtime behavior, and target-specific behavior remain excluded.

## Step 8j — general bounded helper-carrier matching (`v0.44.0`)

Production source admits exactly one complete top-level `match(Helper(...))` in a public pure
caller with one or more parameters. Every caller parameter is passed directly, once, and in
declaration order to one uniquely resolved public pure same-class helper with the exact parameter
signature. The helper returns admitted `Optional<T>`, `Result<List<R>, string>`, or
`Result<string, string>`. Optional arms are exact source-ordered `some(binding)` then binding-free
`none`; Result arms are exact source-ordered `ok(binding)` then `err(binding)`. Both arms exactly
typecheck to the caller return. The helper evaluates once, the complete carrier is validated, the
selected payload is copied, and only the selected arm evaluates.

Typed HIR and target-neutral Core reuse `call` and `match`; Core independently validates the
source contract. The evaluator and deterministic Core-only Go backend preserve its value and lazy
selection semantics. The Docker observability consumer proves two-parameter
`SelectedNameById(List<ContainerRow>, string)` composing `FindSelection(List<ContainerRow>, string)`
without changing `dockpipe.application.v1`. Public schema identities and shapes remain stable;
only language-contract metadata advances, and the exact 45-source lane stays frozen. Arithmetic
Results, computed/reordered/omitted/extra arguments, cross-owner calls, overloads, generics,
nested/multiple matches, wildcard/reversed arms, guards, propagation changes, new blocks/locals,
statements, effects, actions, runtime, target, adapter, UI, and deployment behavior remain excluded.

## Step 8k — first-local helper-carrier matching (`v0.45.0`)

Production source admits exactly one v0.44-compatible `match(Helper(...))` as the initializer of
the first explicitly typed immutable local in a public pure caller with one or more parameters.
Every caller parameter is passed directly, once, and in declaration order to one uniquely resolved
public pure same-class helper with the exact parameter signature. The helper returns admitted
`Optional<T>`, `Result<List<R>, string>`, or `Result<string, string>`. Optional arms remain exact
source-ordered `some(binding)` then binding-free `none`; Result arms remain exact source-ordered
`ok(binding)` then `err(binding)`. Both arms exactly typecheck to the declared local type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`; Core independently
validates the complete source contract and continuation scope. The evaluator and deterministic
Core-only Go backend preserve complete-carrier validation, once-only helper/local evaluation,
copied selected payloads, lazy arm selection, and existing ordered-local continuation semantics.
The Docker observability consumer proves selection followed by `NormalizeName` without changing
`dockpipe.application.v1`. Public schema identities and shapes remain stable; only
language-contract metadata advances, and the exact 45-source lane stays frozen. Match outside the
first local initializer, multiple matches, computed/reordered/omitted/extra arguments, arithmetic
Results, cross-owner calls, overloads, generics, wildcard/reversed arms, guards, propagation
changes, inference, reassignment, early returns, statement branches, loops, effects, actions,
runtime, target, adapter, UI, and deployment behavior remain excluded.

## Step 8l — checked-arithmetic helper matching in the first local (`v0.46.0`)

Production source widens only the v0.45 first-local helper-carrier matrix to the already admitted
checked-arithmetic `Result<int, ArithmeticError>` and `Result<float, ArithmeticError>` shapes. The
caller still has one or more parameters, passes every parameter directly once in declaration order,
and resolves one public pure same-class helper with that exact signature. The one match remains the
first explicitly typed immutable-local initializer, with exact source-ordered `ok(binding)` then
`err(binding)` arms whose expressions exactly match the declared local type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`; Core independently
validates the complete checked-arithmetic composition. The evaluator and deterministic Core-only Go
backend validate the complete Result, call the helper once, copy the selected payload, evaluate only
the selected arm, and preserve existing ordered continuation. Integer overflow and binary64
division-by-zero consumer cases select the error arm deterministically. Public compiler, semantic,
and Application IR identities/shapes remain stable; only language-contract metadata advances, and
the exact 45-source lane stays frozen.

Top-level, later-local, terminal-return, argument, nested, or multiple checked-arithmetic matches;
computed/reordered/omitted/extra arguments; propagation, new Result construction/defaulting,
arbitrary Results, cross-owner calls, overloads, generics, wildcards, reversed arms, guards,
inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment behavior remain excluded.

## Step 8m — later-local helper-carrier matching (`v0.47.0`)

Production source widens only the v0.45/v0.46 local placement. One public pure caller with one or
more parameters may use exactly one existing `match(Helper(...))` as any explicitly typed
immutable-local initializer after zero or more ordinary locals. Every caller parameter is passed
directly once in declaration order to one uniquely resolved public pure same-class helper with the
exact caller signature. The carrier matrix remains closed to admitted Optional primitive/record,
`Result<List<R>, string>`, `Result<string, string>`, and checked-arithmetic
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`, with exact canonical arm order,
bindings, and declared-local result typing.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`; Core independently
validates the complete source contract and continuation scope. Earlier ordinary locals evaluate
eagerly once in source order. The evaluator and deterministic Core-only Go backend validate the
full carrier, call the helper once, copy the selected result into the local once, evaluate only the
selected arm, and preserve later ordered locals plus the terminal return. The Docker observability
consumer proves an ordinary fallback local before `SelectedNameById` matches `FindSelection`, with
unchanged `dockpipe.application.v1` shape. Public compiler, semantic, and Application IR identities
remain stable; only language-contract metadata advances, and the exact 45-source lane stays frozen.

Terminal-return, argument, nested, or multiple matches; top-level checked-arithmetic matches;
computed/reordered/omitted/extra arguments; propagation changes; Result construction/defaulting or
arbitrary widening; cross-owner/private/overloaded/generic helpers; wildcards or reversed arms;
guards; inference; reassignment; statements; effects; actions; runtimes; targets; adapters; UI;
and deployment behavior remain excluded.

## Step 8n — prior-local carrier matching (`v0.48.0`)

Production source widens only the existing local spelling. After zero or more ordinary locals, one
public pure caller with one or more parameters may declare an explicitly typed carrier local
initialized by one public pure same-class exact-signature helper call and an immediately adjacent
explicitly typed local initialized by exactly one canonical `match(carrier)`. Every caller parameter
is passed directly once in declaration order. The carrier matrix remains closed to admitted
Optional primitive/record, `Result<List<R>, string>`, `Result<string, string>`, and checked-arithmetic
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`; Core
independently validates the complete source contract and continuation scope. Earlier locals, the
carrier local, and matched local initialize eagerly once in order. The evaluator and deterministic
Core-only Go backend validate the full carrier, evaluate only the selected arm, copy its result,
and preserve later locals plus the terminal return. The Docker observability consumer proves a
fallback local, adjacent `selection` carrier and `match(selection)` locals, then normalization,
without changing `dockpipe.application.v1`. Public compiler, semantic, and Application IR
identities/shapes remain stable; only language-contract metadata advances, and the exact 45-source
lane stays frozen.

Non-adjacent, terminal-return, argument, nested, or multiple matches; top-level checked-arithmetic
matches; computed/reordered/omitted/extra arguments; propagation changes; Result construction,
defaulting, or arbitrary widening; cross-owner/private/overloaded/generic helpers; wildcards or
reversed arms; guards; inference; reassignment; statements; effects; actions; runtimes; targets;
adapters; UI; and deployment behavior remain excluded. No behavior enters by implication.
