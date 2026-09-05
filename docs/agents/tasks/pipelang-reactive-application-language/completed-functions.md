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

## Step 8o — bounded two-carrier matching (`v0.49.0`)

Production source widens only the v0.48 match occurrence count. One public pure caller with one or
more parameters may contain exactly two non-overlapping adjacent pairs, each spelled as one
explicitly typed helper-call carrier local immediately followed by one explicitly typed canonical
`match(carrier)` local. Zero or more ordinary locals may appear before, between, or after the pairs,
but cannot split a pair. Every helper independently receives every caller parameter directly once
in declaration order and resolves to a uniquely named public pure same-class method with the exact
caller signature. Both pairs independently retain the v0.48 closed carrier matrix and arm rules.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`; Core
independently validates the two-match bound, pair adjacency and non-overlap, exact carrier
references, direct arguments, helper ownership/signatures, carrier types, canonical arms/bindings,
local typing, and continuation scope. Surrounding locals, helpers, carriers, and selected locals
evaluate eagerly once in source order. Each full carrier is validated and only its selected arm
evaluates. The second pair and its arms may consume prior immutable locals, including the first
selected result. Matching remains non-propagating. The evaluator and deterministic Core-only Go
backend preserve the same behavior and call each helper once.

Docker observability proves two exact-signature selection helpers and two adjacent pairs before
normalization without changing `dockpipe.application.v1`. Public `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain stable; only
language-contract metadata advances, and the exact 45-source legacy lane stays frozen.

Existing zero-match and one-match methods remain exact. A third match, a split or overlapping pair,
terminal-return/argument/nested matching, non-helper or computed carriers, computed/reordered/
omitted/extra arguments, propagation changes, Result construction/defaulting or arbitrary widening,
cross-owner/private/overloaded/generic helpers, wildcard/reversed arms, guards, inference,
reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and deployment behavior
remain excluded. No behavior enters by implication.

## Step 8p — dependent second-carrier matching (`v0.50.0`)

Production source adds one exact four-local form:
`C1 firstCarrier = Helper1(p1, ..., pn); T1 first = match(firstCarrier) { ... };`
`C2 secondCarrier = Helper2(first, p1, ..., pn); T2 second = match(secondCarrier) { ... };`.
The locals are contiguous; ordinary locals may surround but not split the stage. `Helper1` keeps
the exact v0.49 caller signature. `Helper2` resolves uniquely to a public pure same-class method
whose first parameter has the selected local's exact type and whose remaining parameters exactly
match every caller parameter in declaration order. Both pairs keep the closed Optional,
`Result<List<R>, string>`, `Result<string, string>`, and checked-arithmetic Result matrix and
canonical source-ordered arms.

Typed HIR and Core reuse `immutable_local`, `call`, `reference`, and `match`. Core independently
checks binding positions, adjacency, helper identities and signatures, carriers, arms, local types,
and continuation. Evaluator and deterministic Core-only Go prove once-only source order,
complete-carrier validation, and selected-arm-only execution. Docker observability proves
`ConfirmSelection(selected, rows, id)` with no Application IR schema change. Public compiler,
semantic, and Application IR identities and shapes remain stable; metadata advances to `v0.50.0`;
the exact 45-source lane remains frozen.

Existing zero-match, one-match, and independent v0.49 two-pair forms remain exact. Other argument
arrangements, computed/reordered/repeated arguments, split stages, third/nested/terminal matches,
propagation changes, arbitrary Result widening, statements, effects, actions, runtimes, targets,
adapters, UI, and deployment remain excluded. No behavior enters by implication.

## Step 8q — general dependent carrier chains (`v0.51.0`)

Production source admits one exact dependent chain of two or more adjacent carrier/match pairs:
`C1 carrier1 = Helper1(p1, ..., pn); T1 value1 = match(carrier1) { ... };`, followed by
`Ci carrierI = HelperI(valueI-1, p1, ..., pn); Ti valueI = match(carrierI) { ... };` for every
later stage. All `2k` explicitly typed locals are contiguous. Ordinary locals may appear before or
after only. The first helper retains the exact caller signature; every later helper is a uniquely
resolved public pure same-class method receiving only the immediately preceding selected local,
then every caller parameter directly once in declaration order. The existing closed Optional,
bounded Result, and checked-arithmetic carrier matrix and canonical arms remain unchanged.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`. Core
independently verifies the complete contiguous chain, immediate-predecessor dependency, argument
positions, exact helper identities/signatures, carriers, arms, local types, and continuation.
Evaluation and deterministic Core-only Go preserve once-only source order, complete-carrier
validation, and selected-arm-only evaluation. Docker observability proves a third
`FinalizeSelection(confirmed, rows, id)` stage before normalization. Public compiler, semantic,
and Application IR identities and shapes remain stable; metadata advances to `v0.51.0`; the exact
45-source lane remains frozen.

Existing zero-match, one-match, v0.49 independent two-pair, and v0.50 dependent two-stage forms
remain exact. Gaps, mixed independent/dependent chains, non-immediate dependencies, fan-in,
computed/reordered/repeated/omitted/extra arguments, third matches outside this chain, nested,
terminal, or argument matches, propagation changes, arbitrary Result widening, statements,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication.

## Step 8r — cumulative fan-in carrier chains (`v0.52.0`)

Production source additionally admits one cumulative chain of `k >= 3` contiguous carrier/match
pairs. The first helper receives every caller parameter directly once in declaration order. At
stage `i`, every later helper receives `value1, ..., valueI-1` directly once in chain order,
followed by every caller parameter directly once in declaration order. The whole method uses
either this cumulative mode or the inherited v0.51 immediate-only mode; stages cannot mix modes.
The closed carrier matrix, canonical arms, and explicit typed-local spelling remain unchanged.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`. Core
independently verifies the complete contiguous chain, cumulative binding order, exact helper
ownership/signatures, carrier types, arms, local types, and continuation. Evaluation and
deterministic Core-only Go preserve once-only source order, complete-carrier validation, and
selected-arm-only evaluation. Docker observability proves a cumulative third stage receiving both
prior selections before `rows` and `id`. Public compiler, semantic, and Application IR identities
and shapes remain stable; metadata advances to `v0.52.0`; the exact 45-source lane remains frozen.

Existing zero-match, one-match, v0.49 independent two-pair, v0.50 dependent two-stage, and v0.51
immediate-only chain forms remain exact. Fewer than three cumulative pairs, partial/reordered/
repeated/omitted/extra prior selections, per-stage mode mixing, gaps, non-chain dependencies,
computed arguments, matches outside the chain, nested/terminal/argument matches, propagation
changes, arbitrary Result widening, statements, effects, actions, runtimes, targets, adapters, UI,
and deployment remain excluded. No behavior enters by implication.

## Step 8s — multi-parameter helper propagation (`v0.53.0`)

Production source widens the exact v0.42 prior-local helper propagation form to methods with two
or more parameters: `C carrier = Helper(p1, ..., pn); T value = propagate(carrier); return
admittedExpression;`. The helper call remains the first typed local initializer, propagation is
immediately adjacent, and every caller parameter is passed directly once in declaration order to a
unique public pure same-class helper. `C` remains identical to the method return carrier and `T` to
its payload. The inherited one-parameter form and closed carrier matrix remain exact.

Typed HIR and target-neutral Core reuse existing nodes and Core independently verifies the full
shape. Evaluation and deterministic Core-only Go call the helper once, validate the complete
carrier, copy success, and preserve canonical absence/failure. Docker observability proves
`ResolveSelection(rows, id)` over `FindSelection(rows, id)`. Public compiler, semantic, and
Application IR identities and shapes remain stable; metadata advances to `v0.53.0`; the exact
45-source lane remains frozen.

Computed/reordered/repeated/omitted/extra arguments, later or split propagation pairs, extra
propagation, mismatched carriers/payloads, cross-owner/private/overloaded/generic helpers,
arbitrary Result widening, inference, reassignment, statements, effects, actions, runtimes,
targets, adapters, UI, and deployment remain excluded. No behavior enters by implication.

## Step 8t — checked-arithmetic helper propagation (`v0.54.0`)

Production source admits exactly `Result<int, ArithmeticError> carrier = Helper(p1, ..., pn); int
value = propagate(carrier); return admittedCheckedArithmeticExpression;`, plus the identical
`float` form. The helper call is the first typed local, propagation is immediately adjacent, and
every caller parameter is passed directly once in declaration order to one unique public pure
same-class helper. The helper carrier equals the method return type and the propagated local equals
its payload.

Typed HIR and target-neutral Core reuse existing nodes and Core independently verifies the exact
composition. Evaluation and deterministic Core-only Go call the helper once, validate the complete
arithmetic Result, copy success, and return canonical overflow or division-by-zero before the
continuation. Public compiler, semantic, and Application IR identities and shapes remain stable;
metadata advances to `v0.54.0`; a compiler-cursor fixture proves checked offset propagation as a
concrete self-hosting consumer; the exact 45-source lane remains frozen.

Direct-parameter or call-inside-propagate forms, later/split pairs, computed/reordered/repeated/
omitted/extra arguments, extra propagation, other Result carriers, inference, reassignment,
statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No
behavior enters by implication.

## Step 8u — direct-parameter checked propagation (`v0.55.0`)

Production source admits exactly `Result<int, ArithmeticError> Continue(Result<int,
ArithmeticError> carrier) { int value = propagate(carrier); return
admittedCheckedArithmeticExpression; }`, plus the identical `float` form. The carrier is the sole
direct parameter and method return, propagation is the first typed local, and the local exactly
matches the success payload.

Typed HIR and target-neutral Core reuse existing nodes and Core independently verifies the exact
composition. Evaluation and deterministic Core-only Go validate the complete arithmetic Result,
copy success, and return canonical overflow or division-by-zero before the continuation. Public
compiler, semantic, and Application IR identities and shapes remain stable; metadata advances to
`v0.55.0`; a compiler-cursor fixture proves consumption of a caller-supplied checked Result; the
exact 45-source lane remains frozen.

Additional parameters, helper or computed operands, later/split propagation, extra propagation,
other Result carriers, inference, reassignment, statements, effects, actions, runtimes, targets,
adapters, UI, and deployment remain excluded. The v0.54 helper form remains exact. No behavior
enters by implication.

## Step 8v — multi-parameter direct checked propagation (`v0.56.0`)

Production source additionally admits exactly `Result<int, ArithmeticError> Advance(Result<int,
ArithmeticError> carrier, int operand) { int value = propagate(carrier); return value + operand; }`.
The integer operator may be add, subtract, or multiply; the identical `float` form admits only
binary64 divide. The carrier is the first parameter and method return, the second and only other
parameter exactly matches its payload, propagation remains the first typed local, and the
continuation uses the local on the left and second parameter on the right.

Typed HIR and target-neutral Core reuse existing nodes and independently verify the exact form.
Evaluation and deterministic Core-only Go validate the complete carrier, copy success, preserve
canonical incoming failure before continuation evaluation, and retain checked continuation
failure. Public compiler, semantic, and Application IR identities and shapes remain stable;
metadata advances to `v0.56.0`; a compiler-cursor fixture proves checked cursor plus width
advancement; the exact 45-source lane remains frozen.

The inherited v0.55 sole-carrier and v0.54 helper forms remain exact. Third/reordered/mismatched
parameters, reversed/repeated/literal/computed operands, helper/computed propagation operands,
later/split or additional propagation, arbitrary Result widening, inference, reassignment,
statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No
behavior enters by implication.

## Step 8w — two-stage checked propagation (`v0.57.0`)

Production source additionally admits exactly `Result<int, ArithmeticError> AdvanceTwice(
Result<int, ArithmeticError> carrier, int first, int second) { int value = propagate(carrier);
Result<int, ArithmeticError> nextCarrier = value + first; int next = propagate(nextCarrier);
return next + second; }`. Each integer checked stage independently admits add, subtract, or
multiply; the identical `float` form admits binary64 divide at both stages. The complete incoming
carrier is validated and propagated, the first checked Result initializes one explicit carrier
local and is validated and propagated, and the terminal checked operation runs only after both
successes.

Typed HIR and target-neutral Core reuse existing nodes and independently verify the exact
three-parameter, three-local, two-propagation form. Evaluation and deterministic Core-only Go copy
success at each stage and preserve incoming, intermediate, and terminal arithmetic failures without
evaluating later stages. Public compiler, semantic, and Application IR identities and shapes remain
stable; metadata advances to `v0.57.0`; a compiler-cursor fixture proves two checked width advances;
the exact 45-source lane remains frozen.

The inherited v0.54-v0.56 forms remain exact. Fourth/reordered/mismatched parameters,
reversed/repeated/literal/computed operands, missing/additional stages, computed or helper
propagation operands, arbitrary Results, inference, reassignment, statements, effects, actions,
runtimes, targets, adapters, UI, and deployment remain excluded. No behavior enters by implication.

## Step 8x — generalized checked-propagation chains (`v0.58.0`)

Production source additionally admits a contiguous chain of `K >= 2` checked stages. One public
pure method has exact signature `Result<T, ArithmeticError> F(Result<T, ArithmeticError> carrier,
T operand1, ..., T operandK)`, begins with `T value0 = propagate(carrier);`, spells every
non-terminal stage as an adjacent explicit checked-Result local followed immediately by propagation
of that local, and ends with the checked operation over the last propagated payload and final direct
parameter. `T` is exactly `int` or `float`; integer stages independently admit add, subtract, or
multiply, while float stages admit binary64 divide only.

Typed HIR and target-neutral Core reuse existing nodes and independently verify the exact parameter
sequence, contiguous alternating locals, carrier and payload types, direct operand positions,
propagation count, and stage operator matrix. Evaluation and deterministic Core-only Go validate
each complete carrier once, copy success at each stage, short-circuit incoming and intermediate
failures before later evaluation, and preserve terminal arithmetic failures. Public compiler,
semantic, and Application IR identities and shapes remain stable; metadata advances to `v0.58.0`;
a three-stage compiler-cursor fixture and metadata-only Docker-observability Application IR
consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.57 forms remain exact. Fewer than two stages, missing or additional chain
locals, ordinary-local gaps, reordered/mismatched parameters, reversed/repeated/literal/computed
operands, computed or helper propagation, arbitrary Results, inference, reassignment, statements,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication.

## Step 8y — bounded cross-payload Result propagation (`v0.59.0`)

Production source additionally admits exactly one public pure method shaped as `Result<U, string>
F(Result<T, string> carrier) { T value = propagate(carrier); return Helper(value); }`. `T` and `U`
are distinct and each is exactly `string` or `List<R>` for an existing public primitive-field
record. The first and only typed local propagates the sole direct parameter; the terminal helper is
one resolved public pure same-class `T -> Result<U, string>` method and receives the direct local
exactly once.

Typed HIR and target-neutral Core reuse existing nodes and independently verify the exact bounded
source/target Results, distinct payloads, string failure, parameter/local positions and types, one
propagation, terminal call, direct local argument, callable owner, and helper signature. Evaluation
and deterministic Core-only Go validate and copy complete carriers, invoke the helper exactly once
on success, and on incoming failure skip it while constructing a canonical target-shaped failure
with the preserved copied error string. Public compiler, semantic, and Application IR identities
and shapes remain stable; metadata advances to `v0.59.0`; compiler-pipeline payload-matrix and
metadata-only Docker-observability Application IR consumption prove the boundary; the exact
45-source lane remains frozen.

The inherited v0.54-v0.58 forms remain exact. Same-payload propagation gains no new spelling.
Arbitrary errors, Optional/arithmetic carriers, extra parameters/locals/propagations, computed
carriers, helper propagation, non-direct/private/cross-class/mismatched/overloaded/generic helpers,
inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No behavior enters by implication.

## Step 8z — two-stage bounded cross-payload Result propagation (`v0.60.0`)

Production source additionally admits exactly one public pure method shaped as `Result<V, string>
F(Result<T, string> carrier) { T first = propagate(carrier); Result<U, string> nextCarrier =
First(first); U second = propagate(nextCarrier); return Second(second); }`. `T`, `U`, and `V` are
text or a list of an existing public primitive-field record; adjacent payloads differ, while `T`
may equal `V`. Both helpers are resolved public pure same-class methods with the exact adjacent
payload-to-Result signatures and receive only their direct preceding payload locals.

Typed HIR and target-neutral Core reuse existing nodes and independently verify the sole direct
carrier, three exact locals, explicit intermediate carrier, adjacent payload inequality, string
failure, two propagations, both direct local arguments, callable owners, and helper signatures.
Evaluation and deterministic Core-only Go validate and copy each complete carrier once. Incoming
failure skips both helpers and intermediate failure skips the terminal helper; both become canonical
target-shaped failures with preserved copied error text. Public compiler, semantic, and Application
IR identities and shapes remain stable; metadata advances to `v0.60.0`; compiler-pipeline and
metadata-only Docker-observability Application IR consumption prove the boundary; the exact
45-source lane remains frozen.

The inherited v0.54-v0.59 forms remain exact. General Result chains, same-payload adjacent stages,
extra parameters/locals, computed or helper propagation, arbitrary errors, Optional/arithmetic
carriers, non-direct/private/cross-class/mismatched/overloaded/generic helpers, inference,
reassignment, statements, branches, loops, effects, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No behavior enters by implication.

## Step 8aa — generalized bounded cross-payload Result propagation chains (`v0.61.0`)

Production source additionally admits a contiguous chain of `K >= 2` bounded cross-payload Result
stages. One public pure method takes exactly one direct `Result<T0, string>` parameter, propagates
it into `T0`, spells every non-terminal stage as an adjacent exact helper-Result local and direct
propagation local, and terminally calls the final helper with the immediately preceding payload
local. Every payload is text or a list of an existing public primitive-field record. Adjacent
payloads differ; non-adjacent payloads may match. Every helper is public, pure, same-class, and has
the exact adjacent payload-to-Result signature.

Typed HIR and target-neutral Core reuse existing nodes and independently verify the sole direct
carrier, arbitrary admitted chain length, contiguous alternating locals, bounded payloads, shared
string failure, adjacent payload inequality, direct carrier and helper-argument references,
callable owners, and exact helper signatures. Evaluation and deterministic Core-only Go validate
and copy every complete carrier once. Any failure skips all later helpers and becomes a canonical
final-target-shaped failure with preserved copied error text. Public compiler, semantic, and
Application IR identities and shapes remain stable; metadata advances to `v0.61.0`; a four-stage
compiler-pipeline fixture and metadata-only Docker-observability Application IR consumption prove
the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.60 forms remain exact. Fewer than two stages in the generalized form,
same-payload adjacent stages, missing/additional/gapped locals, extra parameters, computed or helper
propagation, arbitrary errors, Optional/arithmetic carriers, non-direct/private/cross-class/
mismatched/overloaded/generic helpers, inference, reassignment, statements, branches, loops,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication.

## Step 8ab — contextual bounded cross-payload Result propagation chains (`v0.62.0`)

Production source additionally admits the v0.61 generalized chain with exactly one second direct
`string` context parameter. The caller has exact signature `Result<TK, string>
F(Result<T0, string> carrier, string context)`. Every helper receives the immediately preceding
payload local first and the unchanged `context` parameter second, and has the exact
`(Ti-1, string) -> Result<Ti, string>` signature. All explicit carrier locals, direct propagation
locals, bounded payloads, shared string failure, adjacent payload inequality, chain length,
visibility, purity, and ownership rules remain unchanged.

Typed HIR and target-neutral Core reuse existing nodes and independently verify both caller
parameters, the direct string context reference at every helper, contiguous alternating locals,
bounded carriers and payloads, adjacent payload inequality, callable owners, and exact helper
signatures. Evaluation and deterministic Core-only Go pass the validated context once to every
invoked helper, validate and copy each complete carrier once, and reshape any incoming or
intermediate failure to the canonical final target without invoking later stages. Public compiler,
semantic, and Application IR identities and shapes remain stable; metadata advances to `v0.62.0`;
a four-stage contextual compiler-pipeline fixture and metadata-only Docker-observability Application
IR consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.61 forms remain exact. Missing, reordered, repeated, computed, non-string,
stage-specific, or additional context arguments; a third caller parameter; same-payload adjacent
stages; missing/additional/gapped locals; computed or helper propagation; arbitrary errors;
Optional/arithmetic carriers; non-direct/private/cross-class/mismatched/overloaded/generic helpers;
inference; reassignment; statements; branches; loops; effects; actions; runtimes; targets; adapters;
UI; and deployment remain excluded. No behavior enters by implication.

## Step 8ac — one-stage contextual bounded cross-payload Result propagation (`v0.63.0`)

Production source additionally admits exactly one contextual helper stage. One public pure method
has exact signature `Result<U, string> F(Result<T, string> carrier, string context)`, propagates the
direct carrier into its first and only typed `T` local, then terminally calls one resolved public
pure same-class `(T, string) -> Result<U, string>` helper with the local followed by the unchanged
direct context. `T` and `U` remain distinct and each is text or a list of an existing public
primitive-field record.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates the two exact caller parameters, one direct propagation, local
position and type, bounded distinct payloads, shared string failure, direct context identity,
callable owner, and exact helper signature. Evaluation and deterministic Core-only Go validate and
copy the incoming carrier once, skip the helper on failure, construct the canonical target-shaped
failure with copied error text, and pass the validated context once on success. Public compiler,
semantic, and Application IR identities and shapes remain stable; metadata advances to `v0.63.0`;
compiler-pipeline text/list matrix fixtures and metadata-only Docker-observability Application IR
consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.62 forms remain exact. Same-payload flow; missing, reordered, repeated,
computed, non-string, or additional context arguments; a third caller parameter; extra locals or
propagation; computed or helper propagation; arbitrary errors; Optional/arithmetic carriers;
non-direct/private/cross-class/mismatched/overloaded/generic helpers; inference; reassignment;
statements; branches; loops; effects; actions; runtimes; targets; adapters; UI; and deployment
remain excluded. No behavior enters by implication.

## Step 8ad — one-stage contextual same-payload Result propagation (`v0.64.0`)

Production source additionally admits the payload-preserving counterpart to the exact v0.63
one-stage contextual form. One public pure method has exact signature
`Result<T, string> F(Result<T, string> carrier, string context)`, propagates the direct carrier into
its first and only typed `T` local, then terminally calls one resolved public pure same-class
`(T, string) -> Result<T, string>` helper with the local followed by the unchanged direct context.
`T` remains exactly text or a list of an existing public primitive-field record.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates the two exact caller parameters, one direct propagation, local
position and type, bounded equal payloads, shared string failure, direct context identity, callable
owner, and exact helper signature. Evaluation and deterministic Core-only Go validate and copy the
incoming carrier once, skip the helper on failure, return canonical same-shaped failure with copied
error text, and pass the validated context once on success. Public compiler, semantic, and
Application IR identities and shapes remain stable; metadata advances to `v0.64.0`; compiler-
pipeline string/list fixtures and metadata-only Docker-observability Application IR consumption
prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.63 forms remain exact. Same-payload contextual chains with two or more
stages; missing, reordered, repeated, computed, non-string, or additional context arguments; a
third caller parameter; extra locals or propagation; computed or helper propagation; arbitrary
errors; Optional/arithmetic carriers; non-direct/private/cross-class/mismatched/overloaded/generic
helpers; inference; reassignment; statements; branches; loops; effects; actions; runtimes; targets;
adapters; UI; and deployment remain excluded. No behavior enters by implication.

## Step 8ae — exact two-stage contextual bounded Result propagation (`v0.65.0`)

Production source additionally admits the exact v0.62 two-stage contextual chain when either or
both adjacent payload transitions preserve their payload type. One public pure method has exact
signature `Result<T2, string> F(Result<T0, string> carrier, string context)`, propagates the direct
carrier, calls a first helper into an immediately propagated helper-Result local, and terminally
calls a second helper. `T0`, `T1`, and `T2` each remain exactly text or a list of an existing public
primitive-field record. Both helpers are resolved public pure same-class methods with exact
`(Ti, string) -> Result<Ti+1, string>` signatures and receive the unchanged direct context second.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates the two exact caller parameters, three exact locals, two direct
propagations, adjacency, bounded payloads, shared string failure, direct context identity, helper
ownership, callable identities, and exact signatures. Evaluation and deterministic Core-only Go
validate and copy each carrier once, short-circuit failures into the canonical final Result shape,
and pass validated context once per reached helper. Public compiler, semantic, and Application IR
identities and shapes remain stable; metadata advances to `v0.65.0`; compiler-pipeline fixtures and
metadata-only Docker-observability Application IR consumption prove the boundary; the exact
45-source lane remains frozen.

The inherited v0.54-v0.64 forms remain exact. Same-payload transitions in contextual chains with
three or more stages; missing, reordered, repeated, computed, non-string, or additional context;
a third caller parameter; extra or gapped locals; additional propagation; computed carriers;
`propagate(Helper(...))`; arbitrary errors; Optional/arithmetic carriers; private, cross-class,
mismatched, overloaded, or generic helpers; inference; reassignment; statements; branches; loops;
effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded. No behavior
enters by implication.

## Step 8af — generalized contextual bounded Result propagation (`v0.66.0`)

Production source generalizes the v0.62 contextual chain so any adjacent payload transition may
preserve or change its payload type. One public pure method has exact signature
`Result<TK, string> F(Result<T0, string> carrier, string context)` with `K >= 2`. It begins by
directly propagating the carrier. Every non-terminal helper Result is stored in an explicit local
immediately followed by its direct propagation local, and the final stage is one terminal helper
call. Every `Ti` remains exactly text or a list of an existing public primitive-field record. Every
helper remains resolved, public, pure, same-class, and exact
`(Ti, string) -> Result<Ti+1, string>`, with the immediately preceding payload first and the same
unchanged direct context second.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and call
nodes. Core independently validates arbitrary admitted chain length, the two exact caller
parameters, contiguous alternating locals, bounded payloads, shared string failure, direct
carrier/payload/context identities, helper ownership, callable identities, and exact signatures.
Evaluation and deterministic Core-only Go validate and copy every reached carrier once, pass the
validated context once to each reached helper, and reshape every failure into the canonical final
Result before later helpers can run. Public compiler, semantic, and Application IR identities and
shapes remain stable; metadata advances to `v0.66.0`; mixed and all-equal compiler-pipeline fixtures
plus metadata-only Docker-observability Application IR consumption prove the boundary; the exact
45-source lane remains frozen.

The inherited v0.54-v0.65 forms remain exact. Fewer than two contextual helper stages; missing,
reordered, repeated, computed, non-string, stage-specific, or additional context; a third caller
parameter; missing, additional, or gapped locals; computed carriers; `propagate(Helper(...))`;
arbitrary errors; Optional/arithmetic carriers; private, cross-class, mismatched, overloaded, or
generic helpers; inference; reassignment; statements; branches; loops; effects; actions; runtimes;
targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

## Step 8ag — generalized shared-context Result propagation (`v0.67.0`)

Production source generalizes the v0.66 contextual `K >= 2` chain to one bounded Result carrier
followed by `N >= 1` direct `string` context parameters. Every helper receives the immediately
preceding payload followed by every unchanged context exactly once in caller declaration order and
has the exact `(Ti, string...) -> Result<Ti+1, string>` signature. The explicit alternating
helper-Result and propagation locals, terminal helper call, bounded text or primitive-record-list
payloads, shared string failure, and public pure same-class ownership remain exact.

Typed HIR and target-neutral Core reuse existing nodes and preserve parameter positions. Core
independently validates arbitrary admitted stage and context counts plus the complete ordered
context vector at every helper. Evaluation and deterministic Core-only Go validate and copy every
reached carrier once, pass every validated context unchanged, and reshape failures into the
canonical final Result before later helpers run. Public compiler, semantic, and Application IR
identities and shapes remain stable; metadata advances to `v0.67.0`; multi-context
compiler-pipeline fixtures and metadata-only Application IR consumption prove the boundary; the
exact 45-source lane remains frozen.

The inherited v0.54-v0.66 forms remain exact. No-context contextual chains; one-stage multi-context
chains; missing, reordered, repeated, computed, non-string, or stage-specific contexts; missing,
additional, or gapped locals; computed carriers; `propagate(Helper(...))`; arbitrary errors;
Optional/arithmetic carriers; private, cross-class, mismatched, overloaded, or generic helpers;
inference; reassignment; statements; branches; loops; effects; actions; runtimes; targets; adapters;
UI; and deployment remain excluded. No behavior enters by implication.

## Step 8ah — generalized one-stage shared-context Result propagation (`v0.68.0`)

Production source generalizes the v0.64 one-stage contextual bounded Result form to one direct
bounded Result carrier followed by `N >= 1` direct `string` context parameters. The first and only
typed local directly propagates the carrier, and the terminal helper receives that payload followed
by every unchanged context exactly once in caller declaration order. Source and target payloads may
match or differ and remain exactly `string` or `List<R>` for an existing public primitive-field
record. The helper remains resolved, public, pure, same-class, and exact
`(T0, string...) -> Result<T1, string>`.

Typed HIR and target-neutral Core reuse existing nodes and preserve parameter positions. Core
independently validates arbitrary admitted context count, the complete ordered context vector, the
single direct propagation, bounded payloads, shared string failure, direct references, helper
ownership, callable identity, and exact signature. Evaluation and deterministic Core-only Go
validate the direct inputs, copy the reached carrier once, preserve every context unchanged, and
short-circuit incoming failure into the canonical target Result without invoking the helper. Public
compiler, semantic, and Application IR identities and shapes remain stable; metadata advances to
`v0.68.0`; one-stage multi-context compiler-pipeline fixtures and metadata-only Application IR
consumption prove the boundary; the exact 45-source lane remains frozen.

The inherited v0.54-v0.67 forms remain exact. Missing, reordered, repeated, computed, non-string, or
stage-specific contexts; additional or gapped locals; computed carriers; `propagate(Helper(...))`;
arbitrary errors; Optional/arithmetic carriers; private, cross-class, mismatched, overloaded, or
generic helpers; inference; reassignment; statements; branches; loops; effects; actions; runtimes;
targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

## Step 8ai — terminal statement-level `if/else` (`v0.69.0`)

Production source admits exactly one terminal statement-level conditional after one or more
existing ordered immutable locals in a public pure method. Its exact shape is
`if (condition) { return whenTrue; } else { return whenFalse; }`. The condition is `bool`; both
branches have the declared method return type; and locals, condition, and branches remain
already-admitted eager pure expressions and calls. At most one preceding local initializer may use
the inherited bounded conditional expression.

Typed HIR and target-neutral Core reuse the existing conditional representation with an explicit
terminal-statement marker. Core independently validates the required preceding local, unique
terminal placement, boolean condition, branch result types, and exclusion of propagation and
matching within the method. Evaluation and deterministic Core-only Go reuse existing lazy branch
selection, and generated Go remains Core-only. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` stay stable; metadata advances to `v0.69.0`; terminal-if pipeline fixtures
and metadata-only Application IR consumption prove the boundary; the exact 45-source lane remains
frozen.

The inherited v0.54-v0.68 forms remain exact. Zero-local forms, branch locals, nested branches,
missing `else`, fallthrough or returns elsewhere, propagation or matching in the method,
assignment, reassignment, shadowing, inference, loops, effects, actions, runtimes, targets,
adapters, UI, and deployment remain excluded. No behavior enters by implication. Any successor
requires a new founder decision and separate implementation approval.

## Step 8aj — lexical terminal-branch locals (`v0.70.0`)

Production source preserves the exact v0.69 terminal statement-level `if/else` and additionally
permits either terminal branch to contain exactly one explicitly typed immutable local immediately
followed by its return. At least one top-level ordered immutable local remains required. One or both
branches may use the local form; opposing branches have independent lexical scopes and may reuse a
name. Each branch local evaluates only when its branch is selected and cannot escape to the
condition, sibling branch, or surrounding method scope.

Typed HIR and target-neutral Core reuse the existing `immutable_local` expression nested within
the terminal `conditional`; no new node or schema is introduced. Source and Core validation admit
at most one root branch local per branch, preserve explicit declared types and bool conditions, and
reject nested branch-local topology. Evaluation and deterministic Core-only Go preserve lexical
scope and selected-branch evaluation. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` stay stable; Application IR changes only its language metadata to
`v0.70.0`; the exact 45-source compatibility lane remains frozen.

The inherited direct-return terminal form remains valid. Nested branches, multiple locals per
branch, escaping branch bindings, propagation, match, assignment, fallthrough, other early
returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No successor is implied; another slice requires a new founder decision and separate
implementation approval.

## Step 8ak — two-local terminal-branch sequences (`v0.71.0`)

Production source widens only the v0.70 terminal-branch local limit. Either branch may now contain
at most two explicitly typed ordered immutable locals immediately before its return. The second
local may reference the first local in that branch. Opposing branches retain independent lexical
scopes, may reuse the same names and binding positions, evaluate only when selected, and cannot
leak bindings. One or more top-level ordered immutable locals remain required.

Typed HIR and target-neutral Core represent the sequence as nested `immutable_local` expressions
inside the existing terminal `conditional`; no new node or schema is introduced. Source and Core
validation cap each branch independently at two locals and preserve declaration order, exact
typing, canonical positions, lexical references, and terminal placement. Evaluation and
deterministic Core-only Go preserve source order and lazy branch selection.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` stay stable;
Application IR changes only its language metadata to `v0.71.0`; the exact 45-source compatibility
lane remains frozen.

The inherited direct-return and one-local branch forms remain valid. A third branch local, nested
branches, zero top-level locals, escaping bindings, propagation, match, assignment, fallthrough,
other early returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No successor is implied; another slice requires a new founder decision
and separate implementation approval.

## Step 8al — general terminal-branch local sequences (`v0.72.0`)

Production source removes only the v0.71 per-branch local count ceiling. Either terminal branch may
contain any finite source-ordered sequence of explicitly typed immutable locals followed by its
return. Each local enters scope only after its initializer, so later locals may reference earlier
locals in the same branch while self-reference, forward reference, duplicate names, and shadowing
remain invalid. Opposing branches retain independent lexical scopes, may reuse names and canonical
binding positions, evaluate only when selected, and cannot leak bindings. One or more top-level
ordered immutable locals remain required.

Typed HIR and target-neutral Core continue to use nested `immutable_local` expressions inside the
existing terminal `conditional`; no new node or schema is introduced. Source and Core validation
prove the complete root sequence, declaration order, exact typing, canonical positions, lexical
references, and unique terminal placement. Evaluation and deterministic Core-only Go preserve
source order and lazy branch selection. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` stay stable; Application IR changes only its language metadata to
`v0.72.0`; the exact 45-source compatibility lane remains frozen.

The inherited direct-return, one-local, and two-local branch forms remain valid. Nested branches,
zero top-level locals, escaping bindings, propagation, match, assignment, fallthrough, other early
returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No successor is implied; another slice requires a new founder decision and separate
implementation approval.

## Step 8am — direct terminal `if/else` (`v0.73.0`)

Production source removes only the v0.72 top-level-local prerequisite. One public pure method may
use the existing terminal statement-level `if/else` as its complete body. Either branch may return
directly or contain any finite source-ordered sequence of explicitly typed immutable locals followed
by its return. Each local enters scope only after its initializer; later locals may reference
earlier locals in the same branch. Opposing branches remain independent lexical scopes, may reuse
names and canonical binding positions, evaluate only when selected, and cannot leak bindings.

Typed HIR and target-neutral Core reuse the existing root `conditional` and nested
`immutable_local` expressions; no new node or schema is introduced. Source and Core validation
prove unique root terminal placement, complete branch sequences, exact typing, canonical positions,
lexical references, and absence of nested branching. Evaluation and deterministic Core-only Go
preserve source order and lazy selected-branch execution. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` stay stable; Application IR changes only its
language metadata to `v0.73.0`; the exact 45-source compatibility lane remains frozen.

The inherited v0.69-v0.72 forms with top-level locals remain valid. Ordinary zero-local blocks,
nested branches, escaping bindings, propagation, matching, assignment, fallthrough, other early
returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No successor is implied; another slice requires a new founder decision and separate
implementation approval.

## Step 8an — bounded nested terminal `if/else` (`v0.74.0`)

Production source adds one bounded nested decision to the complete v0.73 terminal form. The new
topology has no top-level locals. Exactly one outer branch may end in exactly one inner terminal
`if/else` after zero or more explicitly typed ordered immutable locals; the sibling outer branch is
an inherited v0.73 branch, and both inner leaves return directly. Both conditions are `bool`, every
leaf has the exact declared return type, and outer-branch locals enter scope in order and remain
visible to the inner condition and leaves. Evaluation is selected-branch-only at both levels.

Typed HIR and target-neutral Core reuse nested terminal `conditional` and `immutable_local` nodes.
Source and Core validation prove the exact two-conditional topology, one nested outer branch, exact
typing, canonical positions, lexical references, and direct inner leaves. Evaluation and
deterministic Core-only Go prove all selected paths. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` stay stable; Application IR changes only its
language metadata to `v0.74.0`; the exact 45-source compatibility lane remains frozen.

The inherited v0.69-v0.73 forms remain valid. Top-level locals for the nested topology, inner
locals, nesting in both outer branches, another nested decision, third-level nesting, conditional
expressions within the topology, propagation, matching, assignment, fallthrough, other early
returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No successor is implied; another slice requires a new founder decision and separate
implementation approval.

## Step 8ao — inner terminal-leaf immutable-local sequences (`v0.75.0`)

Production source widens only the inner leaves of the exact v0.74 one-branch nested topology.
Either inner leaf may contain any finite source-ordered sequence of explicitly typed immutable
locals before its return. Each local enters scope only after its initializer; later locals may
reference earlier locals in the same leaf, while self-reference, forward reference, duplicate
names, shadowing, cross-leaf references, and escaping bindings remain invalid. Outer-branch locals
remain visible to the inner condition and both leaves. Both conditions remain `bool`, every return
retains the exact declared method type, and evaluation remains selected-branch-only.

Typed HIR and target-neutral Core reuse terminal `conditional` and nested `immutable_local` nodes.
Source and Core validation prove the exact two-conditional topology, one nested outer branch,
complete inner-leaf local sequences, exact typing, canonical positions, lexical references, and
the excluded shapes. Evaluation and deterministic Core-only Go prove every selected path.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` stay stable;
Application IR changes only its language metadata to `v0.75.0`; the exact 45-source compatibility
lane remains frozen.

The inherited v0.69-v0.74 forms remain valid. Top-level locals for the nested topology, nesting in
both outer branches, another nested decision, third-level nesting, conditional expressions within
the topology, propagation, matching, assignment, fallthrough, other early returns, loops, effects,
inference, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No successor
is implied; another slice requires a new founder decision and separate implementation approval.

## Step 8ap — shared root immutable-local sequences before bounded nested terminal branching (`v0.76.0`)

Production source permits one or more source-ordered explicitly typed immutable locals before the
exact v0.75 one-branch bounded nested terminal topology. Each root local enters scope only after
its initializer, evaluates eagerly once before the outer condition, and remains visible to that
condition, both outer branches, and every descendant branch. Self-reference, forward reference,
duplicate names, shadowing, and escaping bindings remain invalid. Both conditions remain `bool`,
every return retains the exact declared method type, and branch execution remains selected-only
after the eager root sequence.

Typed HIR and target-neutral Core reuse terminal `conditional` and nested `immutable_local` nodes.
Source and Core validation prove the root sequence, exact two-conditional topology, one nested
outer branch, complete branch-local sequences, exact typing, canonical positions, lexical
references, and excluded shapes. Evaluation and deterministic Core-only Go prove every selected
path after eager root evaluation. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` stay stable; Application IR advances only its language metadata to
`v0.76.0` while exercising a harmless shared root local; the exact 45-source compatibility lane
remains frozen.

The inherited rootless v0.75 and v0.69-v0.74 forms remain valid. Nesting in both outer branches,
another nested decision, third-level nesting, conditional expressions within the topology or
root-local nested form, propagation, matching, assignment, fallthrough, other early returns,
loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No successor is implied; another slice requires a new founder decision and separate
implementation approval.

## Step 8aq — symmetric depth-two terminal branching (`v0.77.0`)

Production source permits one or more source-ordered explicitly typed immutable root locals before
an outer terminal `if/else` whose two branches each end in exactly one inner terminal `if/else`.
Each outer branch and each inner leaf may contain any finite source-ordered immutable-local sequence.
Root locals evaluate eagerly once before the outer condition and remain visible everywhere.
Outer-branch locals are visible only to that branch's inner condition and leaves; inner-leaf locals
remain local to their selected leaf. All three conditions are `bool`, every return has the exact
declared method type, and only the selected outer branch, its inner condition, and selected leaf
execute.

Typed HIR and target-neutral Core reuse terminal `conditional` and nested `immutable_local` nodes.
Source and Core validation prove the required root sequence, exact three-conditional symmetric
topology, complete local sequences, exact typing, canonical positions, lexical references, and
excluded shapes. Evaluation and deterministic Core-only Go prove all four selected paths.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` stay stable;
Application IR advances only its language metadata to `v0.77.0` while exercising both inner
decisions; the exact 45-source compatibility lane remains frozen.

All v0.69-v0.76 forms remain valid. The symmetric topology without a root local, third-level or
additional nesting, conditional expressions within the topology, propagation, matching,
assignment, fallthrough, other early returns, loops, effects, inference, actions, runtimes,
targets, adapters, UI, and deployment remain excluded. No successor is implied; another slice
requires a new founder decision and separate implementation approval.

## Step 8ar — rootless symmetric depth-two terminal branching (`v0.78.0`)

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

The founder selected option A and separately approved implementation in the receiving task.
Offline proof with cached Go 1.25.14 passed: the PipeLang/Core and Application IR suites;
focused PipeLang application CLI tests and the full `src/cmd` suite; the exact 45-source inventory, source, language, artifact, and
execution compatibility checks; and vet for affected compiler/Application IR/application/CLI
packages. New tests cover all four evaluator and deterministic generated-Go compile/run paths,
v0.77 source/Core/backend gates, v0.69-v0.77 inheritance, zero and multiple branch/leaf locals,
source scope/type exclusions, and malformed Core/backend refusal. Editor tests, JavaScript syntax,
JSON/YAML parsing, and diff checks passed. TASK-020's rootless `DisplayMode` consumer retains all
four outcomes; its canonical golden changes only language metadata to v0.78.0.

A broader application-suite run was stopped without a result; it is not claimed as proof.
Generated Go, logs, and caches were temporary. No generated store refresh, commit, push,
publication, credentials, or external operation was performed. Any successor requires a fresh
founder decision and separate implementation approval.

## Step 8as — bounded depth-three terminal branching (`v0.79.0`)

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
runtime, action, target, adapter, UI, or deployment behavior.

The founder selected option A and separately approved implementation. Offline proof with the
cached Go 1.25.0 toolchain passed the full PipeLang/Core and Application IR suites, focused
application PipeLang CLI tests, the full `src/cmd` suite, the exact frozen 45-source compatibility
test, and vet for the affected compiler, Application IR, application, and CLI packages. New tests
cover all five evaluator and deterministic generated-Go compile/run paths, rootful and rootless
forms, the v0.78 source/Core/backend gate, v0.69-v0.78 inheritance, zero and multiple local
sequences, source exclusions, and malformed Core/backend refusal. Editor assertions, JavaScript
syntax, JSON/YAML parsing, and diff checks passed. TASK-020's `DisplayMode` consumer exercises all
five paths; its canonical Application IR golden changes only language metadata to `v0.79.0`.

Generated Go and Go caches were temporary under `/tmp`. No generated store refresh, commit, push,
publication, credentials, or external operation was performed. Any successor requires a fresh
founder decision and separate implementation approval.

## Step 8at — two expanded terminal leaves (`v0.80.0`)

The founder selected option A and separately approved implementation. The new form expands
exactly two of four leaves on the symmetric depth-two base: five conditionals, six return paths,
maximum depth three, optional root locals, and finite ordered typed locals in every lexical scope.
All six leaf pairs are supported. Existing versioned forms and the frozen 45-source lane remain
unchanged. See the [canonical contract](../../../concepts/pipelang.md#pipelang-v0800-two-expanded-terminal-leaves)
for semantics and exclusions.

The source and independent Core validators share their respective bounded topology classifier
between v0.79 (one expansion) and v0.80 (two). HIR/Core keep their existing conditional and local
nodes. Generated Go now emits a blank use for immutable locals, preserving initializer timing
while allowing unused locals; this also repairs that backend limitation in inherited forms.

Offline validation passed using cached Go 1.25.13 with network lookup disabled:

- Full `./src/lib/pipelang/...` and `./src/lib/applicationir` suites.
- Focused `./src/lib/application -run PipeLang`, full `./src/cmd`, and `./tests/pipelangcompat`.
- Vet for PipeLang, Application IR, application, and CLI packages.
- Editor tests, JavaScript syntax, changed JSON/YAML parsing, Go formatting, and diff whitespace.

New proof covers all six pairs with rootful/rootless and zero/multiple branch-local sequences,
all six evaluator and pristine generated-Go paths, v0.79 source/Core/backend rejection, inherited
terminal forms, type/scope failures, malformed Core refusal, and deterministic output. A separate
test instruments a copy of the generated pure helper to check root/path/leaf initializer order
and that unselected branches do not execute. Pristine output is independently compiled and run.
TASK-020's DisplayMode consumer covers all six paths; its regenerated Application IR golden
changes only language metadata. Public identities and the exact 45-source compatibility hashes pass.

Generated Go and test caches/logs are temporary under `/tmp`; the intended Application IR golden
is the only regenerated tracked artifact. The broader application/repository suites and live
operations were not run. Package/engine boundaries are preserved. No commit, push, publication,
worktree, generated-store refresh, credential change, or external operation occurred.
Implementation is complete; any successor remains a separate founder decision.

## Step 8au: terminal trees through depth three (v0.81.0)

The founder selected option A and separately approved implementation on 2026-09-04.
`TASK-021-next-compiler-slice` is complete in the saved checkout on `js/pipelang`, based on
`acbd10f88207d0a226ec24a073948362691adb8c`; later founder review and commit placed the
complete slice at `c8dd8a9c70837eb6313240552e5172a78f76565c`, superseding its uncommitted status.
The [canonical contract](../../../concepts/pipelang.md#pipelang-v0810-terminal-trees-through-depth-three)
owns source behavior and exclusions. Any terminal tree through depth three is admitted, including
asymmetric trees and three/four expanded leaves, with finite ordered typed immutable locals.
The compiler reuses terminal `conditional` and `immutable_local` HIR/Core nodes. Core independently
validates topology; structural type/binding checks remain mandatory before evaluation or generation.
Earlier exact version gates, internal Core capabilities, compiler/semantic/Application IR
identities, and the frozen 45-source lane remain intact. Package/engine boundaries are preserved.

### Completion proof — 2026-09-04

`terminal_tree_test.go` enumerates all 25 branching shapes with root and branch/leaf locals
independently present/absent (100 configurations). Eight boolean input combinations cover every
return path per configuration, comparing Core evaluation with executed pristine generated Go.
Separate instrumented Go copies observe initializer and condition order, eager root execution,
and unselected-path laziness for every shape. Repeat compilation checks Core/semantic/Go byte
identity. Source rejection tests check located diagnostics; forged Core tests check missing
branches, terminal flags, exact types, binding positions, shadowing, and depth-four rejection,
including backend refusal. Existing terminal shapes retain exact Go bytes under the new version;
new shapes remain rejected under `v0.80.0`. Core admission checks cover all 81 versions and the
existing feature fixtures; source-level expression inheritance covers propagation, matching,
helper composition, and checked Result argument transport.

The Application IR test compiles an asymmetric depth-three snapshot entrypoint through the
canonical semantic/HIR/Core pipeline, projects it through the existing application spec, checks
stable application identity/schema and deterministic output, and executes all four paths through
both Core evaluation and generated Go. Existing application and compiler goldens are unchanged.
The editor gains an asymmetric terminal-tree snippet and matching documentation/checks.

Pinned cached Go 1.25.13, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and writable temporary
caches were used. Terminal commands passed:

- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`;
- `go test ./src/lib/application -run 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe' -count=1`;
- `go test ./src/cmd -count=1`;
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir`;
- `node --check src/app/tooling/vscode-extensions/dockpipe-language-support/extension.js` and
  `node src/app/tooling/vscode-extensions/dockpipe-language-support/extension.test.js`;
- changed Go formatting, task YAML/state/document-route checks, and `git diff --check`.

Logs and isolated baseline overlays are under `/tmp/pipelang-v081-proof`; generated Go test
modules are temporary and removed by test helpers. No generated store or tracked golden was
refreshed. No worktree, stash operation, commit, push, publication, credential, or live operation
occurred. Repository-wide builds/suites, interactive editor execution, sustained fuzzing,
stack-exhaustion testing, and exhaustive feature/version combinations were not run.

### Deferred findings

The previously documented normalized numeric-comparison evaluator limitation remains open.
An additional inheritance probe of the existing v0.56 multi-parameter checked-propagation fixture,
changing only its language metadata to `v0.80.0`, fails with `PL3028` at `return value + operand`.
A read-only Go overlay of the committed `acbd10f8` production sources reproduces the rejection;
the same probe at `v0.81.0` retains it. The fixture still passes at its original accepted version
in the full suite. This is a pre-existing source-admission gap, distinct from numeric comparison,
and was not repaired or admitted as new syntax by the terminal-tree objective. Evidence:
`baseline-checked-probe.log` and `current-checked-probe.log` in the temporary proof directory.
Neither deferred finding selects or authorizes a successor objective.

## Step 8av — conditional local in terminal trees (`v0.82.0`)

The founder selected A and separately approved implementation on 2026-09-04.
`TASK-021-next-compiler-slice-after-v081` is complete; implementation and terminal verification
passed on 2026-09-04. Changes remain uncommitted for founder review.
The saved checkout remains on `js/pipelang`, based on committed v0.81 at `c8dd8a9c`.
The [canonical contract](../../../concepts/pipelang.md#pipelang-v0820-conditional-local-in-terminal-trees)
owns behavior and exclusions; [next-boundary.md](next-boundary.md) owns the approved objective.

Source and independent Core admission add one nonterminal conditional only as a complete
immutable-local initializer within a terminal tree through depth three. The occurrence bound
covers the whole method. Existing HIR/Core nodes, evaluator, and Core-only Go emission provide
exact typing, lexical bindings, source-ordered initialization, and lazy selection. Explicit
inherited-version predicates include v0.82; earlier exact forms retain their rules. Existing
composite-signature restrictions are retained; the composite type matrix uses a root local.

`conditional_local_tree_test.go` enumerates 25 shapes and all 235 lexical scope placements.
Three local layouts produce 705 methods: the choice alone, surrounded by ordered locals, and
unused after an earlier initializer. Sixteen boolean combinations per method give 11,280
Core-evaluator/pristine-generated-Go comparisons and the same number of separate instrumented-Go
trace checks. These cover both arms, every terminal path, eager root/local order, condition order,
selected-arm execution, unused initializers, and unselected-terminal-path laziness. Repeat builds
compare Core, semantic, and generated-Go artifacts. The 12-type matrix covers primitives,
records, record lists, primitive/record Optionals, text/snapshot Results, and arithmetic Results.

Located source diagnostics reject invalid placement, count, operand types, binding scope, depth,
and excluded statements. Independent forged Core checks reject incomplete conditionals, terminal
initializer flags, wrong types/positions, shadowing/self/escaping references, second/nested choices,
argument/return/condition placement, and depth four; evaluator and backend both refuse them.
Version proof rejects new composition under v0.81, preserves old terminal/expression Go bytes,
and checks Core admission through all 82 accepted versions and existing feature fixtures.

The Application IR consumer uses a conditional local in an asymmetric snapshot-entrypoint tree,
executes all four terminal paths and both value-choice arms through Core and pristine Go, and
checks stable schema/identity and deterministic projection. Existing goldens are unchanged.
The editor adds `pipe-conditional-local-tree`; canonical docs and task routes record the slice.

Focused shape, rejection, type/inheritance, Core admission, and consumer checks passed.
Terminal commands passed on 2026-09-04:

- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`;
- `go test ./src/lib/application -run 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe' -count=1`;
- `go test ./src/cmd -count=1`;
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir ./src/lib/application ./src/cmd`;
- editor `node --check .../extension.js` and `node .../extension.test.js`;
- changed Go formatting, snippet JSON, task YAML/state/document routes, and `git diff --check`.

Cached Go 1.25.13 runs with `GOTOOLCHAIN=local`, `GOPROXY=off`,
`GOSUMDB=off`, and writable temporary caches. Logs/cache are under `/tmp/pipelang-v082-proof`;
generated-Go test modules are temporary. No tracked golden or generated store is refreshed.

The normalized numeric-comparison evaluator limitation and v0.56 checked-propagation fixture's
later-version source-admission gap remain deferred. This slice does not repair either finding.
Repository-wide builds/suites, interactive editor execution, sustained fuzzing, stack-exhaustion
proof, and the exhaustive feature/version cross-product are outside this proof. Package/engine
boundaries are preserved. No commit, push, publication, worktree, stash mutation, credential, or
live operation is performed. No successor or automatic handoff is authorized.

## Numeric-comparison evaluator parity after v0.82 — completed

The founder selected A and separately approved the conformance repair on 2026-09-04.
`TASK-021-next-compiler-slice-after-v082` is complete; the
[repair record](numeric-comparison-repair.md) owns the reproduced failure, implementation,
3,552 differential numeric cases, restored fixture coverage, consumer proof, and passed terminal
verification. This resolves the numeric-comparison limitation recorded above without advancing
the language version. The checked-propagation source-admission gap remains deferred. Changes are
uncommitted for founder review; no successor is selected or authorized.


## Step 8aw: two conditional locals in terminal trees (v0.83.0)

Completed after founder selection of A and separate implementation approval. The
[objective record](two-conditional-locals.md) owns scope, implementation findings, and passed
focused/terminal proof. The [canonical contract](../../../concepts/pipelang.md#pipelang-v0830-two-conditional-locals-in-terminal-trees)
owns semantics and exclusions. All 25 tree shapes and 2,652 scope-pair/layout methods pass
84,864 evaluator/pristine-Go cases plus ordered traces; type/carrier, malformed-source/Core,
version/inheritance, and executable Application IR checks pass. Public identities, internal Core
capabilities, generic engine/package boundaries, and the frozen 45-source lane remain unchanged.
Changes are uncommitted for review; no commit, push, live action, or successor is authorized.
