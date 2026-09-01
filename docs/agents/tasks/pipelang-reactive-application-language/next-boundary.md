## Exact Next Boundary

Steps 7 and 8a of **Bounded Implementation Order** have completed the fixed numeric, compiler-internal
checked-arithmetic Result, and direct production-source checked add, subtract, multiply, negate,
binary64 divide, first-class arithmetic Result transport, ordinal Unicode text ordering, and
primitive immutable record-identity transport, one-hop primitive-record field projection, and
exact primitive-record construction and structural equality, plus primitive Optional
construction, identity transport, presence inspection, and bounded defaulting slices, plus
deterministic empty, singleton, identity transport, and cardinality of primitive-record lists.
Deterministic immutable append of one primitive record to a primitive-record list is also complete.
Exact Optional construction, identity transport, presence inspection, and bounded defaulting for
one existing public primitive record is also complete.
Exact construction, identity transport, success inspection, and bounded success/failure defaulting
for one `Result<List<R>, string>` read-only snapshot envelope is also complete.
Exact safe zero-based indexing of one primitive-record list into `Optional<R>` is also complete.
Exact first-match lookup of one primitive-record list by one selected public string field into
`Optional<R>` is also complete.
Exact stable-order filtering of one primitive-record list by one selected public string field into
`List<R>` is also complete.
Exact Unicode 17.0.0 full-default case-folded containment of two direct strings is also complete.
Exact stable-order case-folded containment filtering of one primitive-record list by one selected
public string field is also complete.
Exact construction, identity transport, success inspection, and bounded success/failure defaulting
for `Result<string, string>` is also complete.
Exact deterministic trimming of leading and trailing Unicode 17.0.0 `White_Space` from one direct
strict-UTF-8 string is also complete.
Exact stable-order filtering of one primitive-record list by exactly five source-ordered public
string fields and one trimmed case-folded query is also complete.
Exact stable ascending ordinal sorting of one primitive-record list by one selected public string
field is also complete.
Exact stable-order joined case-folded filtering by a record-bounded variable count of two or more
source-ordered distinct public string fields is also complete.
Exact stable ascending lexicographic ordinal sorting of one primitive-record list by a
record-bounded variable count of two or more source-ordered distinct public string fields is also
complete.
`v0.2.0` admits only the exact explicit Result-returning addition;
`v0.3.0` adds only direct subtraction; `v0.4.0` adds only direct multiplication; `v0.5.0` adds only
direct integer negation; `v0.6.0` adds only direct binary64 division; and `v0.7.0` adds only direct
identity transport of one identical existing arithmetic Result parameter and return while preserving
every prior contract. `v0.8.0` adds only `<`, `<=`, `>`, and `>=` ordinal scalar-sequence ordering
in the exact two-parameter direct method shape while preserving prior text concatenation/equality.
`v0.9.0` adds only public nonempty primitive immutable records and exact one-parameter identity
transport while preserving every prior contract.
`v0.10.0` adds only direct one-hop read-only projection of one declared primitive field from the
sole record parameter while preserving every prior contract.
`v0.11.0` adds only direct declaration-ordered construction of one existing public primitive
record from one corresponding primitive parameter per field while preserving every prior contract.
`v0.12.0` adds only direct structural `==` and complementary `!=` between two parameters of the
same existing public primitive record while preserving every prior contract.
`v0.13.0` adds only `Optional<T>` for primitive `T` with exact direct `some(value)`, `none<T>()`,
identity transport, and `has_value(value)` methods while preserving every prior contract.
`v0.14.0` adds only exact two-parameter `value_or(Optional<T>, T) -> T` for primitive `T`, with
both arguments canonically validated before selection, while preserving every prior contract.
`v0.15.0` adds only `List<R>` for one existing public primitive record `R`, with exact direct
`empty_list<R>()`, `list(value)`, and identity transport methods, fixed `pipelang:list` identity,
canonical per-element validation, and copied storage while preserving every prior contract.
`v0.16.0` adds only exact direct `count(List<R>) -> int` cardinality for one existing public
primitive record `R`, with complete list/element validation and no implicit iteration semantics,
while preserving every prior contract.
`v0.17.0` adds only exact direct `append(List<R>, R) -> List<R>` for one existing public primitive
record `R`, with complete input and appended-record validation plus fresh copied storage while
preserving every prior contract.
`v0.18.0` adds only exact direct `some`, `none`, identity transport, `has_value`, and `value_or`
methods for `Optional<R>` where `R` is one existing public primitive record, with complete tagged
value and record validation plus copied result storage while preserving every prior contract.
`v0.19.0` adds only exact direct `ok`, `err`, identity transport, `is_ok`, `success_or`, and
`failure_or` methods for `Result<List<R>, string>` where `R` is one existing public primitive
record, with complete tagged payload/fallback validation plus copied list and record storage while
preserving every prior contract.
`v0.20.0` adds only exact direct `at(List<R>, int) -> Optional<R>` for one existing public
primitive record `R`, with complete list and record validation before zero-based bounds selection,
canonical absence, and copied selected-record storage while preserving every prior contract.
`v0.21.0` adds only exact direct `find_by(List<R>, R.Field, string) -> Optional<R>` for one existing
public primitive record `R` and one selected public string field, with complete list, record, and key
validation before first ordinal-equal selection, canonical absence, and copied selected-record
storage while preserving every prior contract.
`v0.22.0` adds only exact direct `filter_by(List<R>, R.Field, string) -> List<R>` for one existing
public primitive record `R` and one selected public string field, with complete list, record, and
key validation before retaining every ordinal-equal match in stable input order, canonical non-nil
empty output, and fresh copied list/record storage while preserving every prior contract.
`v0.23.0` adds only exact direct `contains_casefolded(string, string) -> bool`, using pinned Unicode
17.0.0 full default C/F mappings after complete UTF-8 validation of both operands. Containment is
over the folded contiguous scalar sequence; an empty query matches. It performs no normalization,
locale tailoring, grapheme segmentation, or host-runtime case conversion and preserves every prior
contract.
`v0.24.0` adds only exact direct
`filter_contains_casefolded(List<R>, R.Field, string) -> List<R>` for one existing public primitive
record `R` and one selected public string field. It completely validates the list, every record,
field, and UTF-8 query before applying the pinned `v0.23.0` Unicode 17.0.0 full-default C/F
containment rule, retaining every match in stable input order with canonical non-nil empty output
and fresh copied list/record storage while preserving every prior contract.
`v0.25.0` adds only exact direct `ok`, `err`, identity transport, `is_ok`, `success_or`, and
`failure_or` methods for `Result<string, string>`. It completely validates tagged payloads and both
selected and unselected fallback text as strict UTF-8, requires the canonical empty success payload
for failures, reuses the existing Result semantic identity and HIR/Core expression kinds, and
preserves every prior contract.
`v0.26.0` adds only exact direct `trim(string) -> string`. It validates the direct parameter as
strict UTF-8, removes the maximal leading and trailing sequence of scalars in the pinned Unicode
17.0.0 `White_Space` set, preserves interior scalars exactly, returns canonical empty text for an
all-whitespace input, carries one explicit `text_trim` HIR/Core node, and preserves every prior
contract.
`v0.27.0` adds only exact direct
`filter_joined_contains_casefolded(List<R>, R.Field1, R.Field2, R.Field3, R.Field4, R.Field5,
string) -> List<R>`. The two runtime operands are direct `List<R>` and `string` parameters; the five
source-ordered selectors are distinct existing public string fields of the same existing primitive
record `R`. It completely validates the query, list, records, and every field before filtering,
joins selected strings with one U+0020 SPACE, trims the query with the pinned `v0.26.0` Unicode
17.0.0 `White_Space` rule, then applies the pinned `v0.23.0` Unicode 17.0.0 full-default C/F
case-folded contiguous containment rule. Empty trimmed query retains all rows; matches preserve
stable order; empty output is canonical non-nil storage; result list and records are fresh copies.
Typed HIR and target-neutral Core carry the explicit
`list_filter_joined_contains_case_folded_text` node with five ordered field identities, names, and
positions. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and every earlier contract remain
unchanged.
`v0.28.0` adds only exact direct `sort_by_ordinal(List<R>, R.Field) -> List<R>` for one existing
public primitive record `R` and one selected public string field. It completely validates the list,
every record, and every selected and unselected field before returning a stable ascending sort under
the existing `v0.8.0` ordinal Unicode scalar-sequence order. Equal keys retain input order; empty
output is canonical non-nil storage; result list and records are fresh copies. Typed HIR and
target-neutral Core carry the explicit `list_sort_by_ordinal_text` node with the field identity,
name, and declaration position. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and every earlier
contract remain unchanged.
`v0.29.0` widens only exact direct
`filter_joined_contains_casefolded(List<R>, R.Field1, R.Field2, ..., string) -> List<R>` to two or
more source-ordered selectors, bounded by the distinct public string fields of the same existing
primitive record `R`. The list and query remain the two direct runtime parameters. It completely
validates the query, list, every record, and every selected and unselected field before joining the
selected strings with one U+0020 SPACE, trimming the query under v0.26.0, and applying the pinned
v0.23.0 Unicode 17.0.0 full-default case-folded containment rule. Matches preserve stable input
order; empty output is canonical non-nil storage; result lists and records are fresh copies. Typed
HIR and target-neutral Core reuse the explicit `list_filter_joined_contains_case_folded_text` node
and its ordered field identities, names, and positions. `v0.27.0` and `v0.28.0` retain their exact
five-selector rule. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and every earlier contract
remain unchanged.
`v0.30.0` widens only exact direct
`sort_by_ordinal(List<R>, R.Field1, R.Field2, ...) -> List<R>` to two or more source-ordered
selectors, bounded by the distinct public string fields of the same existing primitive record `R`.
It completely validates the list, every record, and every selected and unselected field before
returning a stable ascending lexicographic sort under the existing `v0.8.0` ordinal Unicode
scalar-sequence order. The first unequal selected field decides each comparison; rows equal across
all selectors retain input order. Empty output is canonical non-nil storage, and result lists and
records are fresh copies. One-selector source under `v0.30.0` preserves the exact `v0.28.0`
`list_sort_by_ordinal_text` HIR/Core projection; two or more selectors use the explicit
`list_sort_by_ordinal_texts` node with ordered field identities, names, and declaration positions.
`v0.28.0` and `v0.29.0` retain their exact one-selector rule. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and every earlier contract remain unchanged.
`v0.31.0` adds only exact direct
`filter(List<R>, PredicateName, P1, ...) -> List<R>` and a same-public-class public
`bool PredicateName(R item, P1, ...)` whose trailing parameters are primitive. The filter list and
arguments are direct declared parameters in order. The predicate is a bounded pure expression over
literals, trailing primitive parameters, one-hop public primitive fields of `item`, logical and
comparison operators, `contains_casefolded`, and `trim`. Typed HIR and target-neutral Core carry an
explicit `list_filter_predicate` node with the predicate method's existing semantic identity and
ordered operands; local predicate bindings create no new public identity. Core validation resolves
the target within the lowered program. Evaluation and deterministic Core-only Go validate all
arguments and the complete list before iteration, invoke the predicate once per row in input order,
require bool, fail atomically, and produce stable canonical non-nil fresh copied output.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and every earlier contract remain unchanged.
All other numeric arithmetic and every other Result construction, composition, or consumption form
beyond the exact accepted checked-arithmetic helper match remain fail-closed from production source. Any next slice requires a new
synchronized decision for its exact source spelling, type/value handling rule, semantic projection,
migration, and bounded semantics before implementation.

The first accepted application consumer is TASK-020's one-to-one DockPipe Launcher replacement,
beginning with read-only Docker observability. Its typed snapshot requirements are dependency
evidence when comparing remaining successor options. Completed primitive record transport, one-hop
field projection, exact construction, structural equality, and primitive Optional presence satisfy
five dependencies; bounded primitive Optional defaulting satisfies a sixth. They do not authorize
nested or general record value use, optional extraction beyond `value_or` or composition,
arbitrary multi-element collection construction or collection consumption, failures, UI, actions,
effects, or Qt behavior as one batch. The accepted list foundation now satisfies the read-only
consumer's empty, singleton, pass-through, count-summary, and deterministic multi-row growth
boundary. The accepted record-Optional slice additionally satisfies typed absence/presence and
deterministic whole-record fallback at that read-only boundary. The accepted snapshot-Result slice
adds a typed whole-snapshot success/failure boundary plus deterministic cached-list and error
fallback. The accepted list-at slice additionally provides safe positional row selection. The
accepted stable-key slice additionally provides first-match selection and detail lookup by the
consumer's stable string identity. The accepted selected-field filter slice additionally provides
stable exact-field snapshot subsets. The case-folded text predicate supplies deterministic
human-entered status/log matching, and the selected-field case-folded list filter now applies that
predicate to one public string field while preserving adapter order. Direct deterministic trimming
provides bounded whitespace cleanup for adapter-supplied labels, filters, and diagnostics. The
exact five-field joined filter now supplies deterministic Name/State/Image/Ports/Created search.
The variable-selector extension removes the language-level five-field arity coupling for future
separately accepted projections without changing the frozen launcher behavior by implication.
The exact one-field and multi-key ordinal sorts now supply target-neutral deterministic
collection-ordering primitives, but applying either to the first launcher would require a separate
TASK-020 parity decision because the checked-in oracle does not expose table sorting. The named
predicate filter now supplies reusable deterministic combined row visibility/state filtering
without authorizing any UI or adapter projection. Dynamic field selection, general functions and
lambdas, descending or per-key direction sorting, general indexing,
propagation, matching, and application projection remain later decisions.

No broader Step-8 slice is included here. In particular, this checkpoint does not add general Result
construction, inspection, extraction, wrapping, unwrapping, propagation, or matching beyond the
exact accepted `Result<List<R>, string>` and `Result<string, string>` forms; additional
Unicode text construction/scalar/grapheme APIs, trim variants or composition, normalization,
locale-aware or additional case operations, value/reference,
hashing, general total-order capabilities, optional extraction beyond `value_or`, equality,
implicit defaults, nesting, chaining,
general result, record nesting/chained or general access/mutation/hash/order, union, or additional
deterministic collection production or consumption semantics beyond exact `filter_by`,
`filter_contains_casefolded`, `filter_joined_contains_casefolded`, `sort_by_ordinal`, and the bounded
named-predicate `filter`; accept
namespace, import, migration, `internal`, overload, generic, or ID production syntax; implement
overload resolution;
add new types/declarations/expressions/operators, blocks, locals, branches, or loops; add effects,
entrypoints, actions/state, contracts/replay, executable application/service semantics, Application
IR, Service IR, another backend, or self-hosting; mutate generated stores; or widen Go emission by
guessing successor semantics. Exact successor production spellings remain later synchronized language
slices.

## Accepted v0.32.0 boundary — per-key ordinal sorting direction

`v0.32.0` adds only the paired direct spelling
`sort_by_ordinal(values, R.Field1, ascending|descending, ...) -> List<R>`, with one or more
source-ordered selector/direction pairs. `ascending` and `descending` are contextual identifiers
only in those direction positions. The method still has exactly one direct `List<R>` parameter and
the identical `List<R>` return, and its body is exactly this call. Every selector names a distinct
public `string` field on the same existing primitive record. The complete input list, every record,
and every field are validated before stable lexicographic ordinal Unicode scalar-sequence sorting.
Equality at every key preserves source order. Empty results are non-nil, and all result/list/record
storage is copied. The semantic projection changes only its explicit language-contract value;
field and method stable identities are unchanged.

Typed HIR and target-neutral Core use the dedicated `list_sort_by_ordinal_directions` expression,
whose ordered selectors carry field semantic identity, source name, declaration position, and the
canonical `ascending` or `descending` direction. The evaluator and Core-only Go backend validate
before comparison and apply the same direction independently at each key. The legacy one-key
`list_sort_by_ordinal_text` and ascending multi-key `list_sort_by_ordinal_texts` nodes and every
v0.28.0/v0.30.0 source remain exact under their prior contracts; v0.32.0 intentionally requires
explicit direction pairs and performs no implicit migration.

This independently reviewable slice gives TASK-020 deterministic target-neutral descending and
mixed-key row ordering without introducing dynamic selectors, direction values outside this call,
comparers, normalization, case folding, locale tailoring, mutation, composition, general ordering,
indexing, propagation, matching, Application IR, runtime behavior, or target behavior.


## Accepted v0.33.0 boundary — safe general indexing

`v0.33.0` adds only postfix `values[index] -> Optional<R>` for an existing primitive-record
`List<R>` receiver and signed 64-bit `int` index. The enclosing method is exactly
`Optional<R> M(List<R> values, int index) => values[index];`: two direct parameters in receiver/index
order, no computed operands, nesting, or chaining. Negative and out-of-bounds indices produce canonical
`none`; there are no panics, exceptions, defaults, negative-index translation, or target semantics. The
complete non-nil list and every record/field are validated before selection; a present record and all
list/record storage are copied.

The semantic projection changes only its explicit language-contract value; callable, parameter, record,
field, List, and Optional stable identities are unchanged. Typed HIR and target-neutral Core deliberately
reuse the existing `list_at` node with direct list/index references, so the evaluator and Core-only Go
backend retain identical deterministic validation, absence, and copying behavior. TASK-020 gains concise
safe optional row selection without adapter-inferred bounds behavior. The v0.20.0 `at(values, index)`
spelling and node remain accepted unchanged; earlier contracts do not accept postfix indexing.

This is one coherent independently reviewable syntax-to-existing-semantics slice. It excludes primitive,
nested, Optional, Result, string, map, and arbitrary receivers; non-int indices; literals, slicing, ranges,
negative-index magic, unchecked access, defaults, mutation, composition, chaining, propagation, matching,
Application IR, runtime behavior, and target behavior.

## Accepted v0.34.0 boundary — bounded propagation

`v0.34.0` adds only the contextual expression `propagate(carrier)` as the direct payload of the
complete method body `some(propagate(carrier))` or `ok<T, E>(propagate(carrier))`. `carrier` is one
direct parameter and must have the identical enclosing return carrier type. The Optional matrix is
exactly the already admitted `Optional<T>` primitive and public primitive-record forms; the Result
matrix is exactly the already admitted `Result<List<R>, string>` and `Result<string, string>` forms.
On presence/success, propagation extracts a canonically validated, copied payload and the explicit
outer constructor rebuilds the identical carrier. On absence/failure it returns the identical
canonical carrier immediately without evaluating a later expression. Misuse is source-located as
`PL3032`; `PL3029`–`PL3031` remain reserved for bounded matching diagnostics.

Typed HIR and target-neutral Core carry an explicit `propagate` node containing its operand, inner
success type, carrier type, and source span in HIR. Core validation proves the carrier/inner
relationship. The evaluator and Core-only Go backend validate the complete input carrier before
branching, preserve absence/failure exactly, and copy present/success record/list storage through the
existing constructors and clone helpers. The semantic projection and public identities are
unchanged because `propagate` is method-body control flow, not a declaration. This supplies TASK-020
a bounded way to forward optional selection/details and read-only section failure without defaults.

This independently reviewable slice adds no postfix operator, implicit conversion, exception,
arbitrary Result, nested carrier, async/effect behavior, target-owned error semantics, matching,
blocks, locals, or general early return. Every v0.1.0–v0.33.0 source remains unchanged and does not
recognize `propagate` contextually.

## Accepted v0.35.0 boundary — exhaustive bounded matching

`v0.35.0` adds only `match(directTaggedParameter){ arms }` for every already admitted
`Optional<T>` and `Result<T,E>` carrier, including checked-arithmetic Results. Optional arms are
`some(binding) => expression` and `none => expression`; Result arms are `ok(binding) => expression`
and `err(binding) => expression`. A final `_ => expression` is the only wildcard. Payload patterns
require exactly one arm-local binding, absence has none, and every arm expression must have exactly
the declared method return type—there are no conversions or inferred common supertypes. `PL3029`
reports a missing tag, `PL3030` a duplicate tag, and `PL3031` any arm following `_`, deterministically
at the pattern or complete match span.

Typed HIR and target-neutral Core carry a dedicated `match` node, source-ordered arms, and explicit
arm-local bindings distinct from parameters. The evaluator and Core-only Go backend validate the
complete carrier, select exactly one arm, bind a validated copied payload, and evaluate only that
arm. Matching changes no semantic declaration or stable identity; only the semantic projection's
language-contract value advances. TASK-020 can now consume optional selection/details and explicit
section success/failure without adapter-inferred defaults or tag semantics.

This is one coherent independently reviewable syntax, typing, control-flow, diagnostics, execution,
editor, and compatibility slice. Earlier source remains exact. Guards, destructuring, literals,
open unions, fallthrough, blocks, effects, implicit conversion, target errors, actions, and
Application IR are excluded.

## Accepted v0.36.0 boundary — same-class pure calls

`v0.36.0` adds only `Method(expression, ...)` inside a public expression-bodied method. The target
must be one uniquely named public method declared by the same class. Semantic analysis resolves the
target before lowering, requires exact ordered argument types and the exact declared return type,
and rejects private callers or targets, missing targets, arity/type mismatches, direct recursion,
and indirect cycles. Every call participant is closed over parameters and match-arm bindings rather
than class-owned state. Calls may nest, and arguments may use expressions already admitted by the
language contract. `PL3033` is the deterministic call-cycle diagnostic.

Typed HIR and target-neutral Core carry an explicit `call` node with the existing callable semantic
identity, source target name, and ordered typed operands. Lowering closes the transitive call graph
and includes each dependency once. Core independently validates target presence, same-class owner,
exact callable identity and signature, and acyclicity. The evaluator validates and copies argument
values into isolated call frames. The deterministic Go backend consumes only validated Core and
emits calls without resolving or inferring source semantics.

TASK-020's Docker observability fixture proves
`OrderContainers(FilterContainers(rows, query))` while the separately versioned
`dockpipe.application.v1` projection keeps its existing explicit filter/order bindings. The public
`pipelang.semantic.v1`, `pipelang.compiler.v1`, and `dockpipe.application.v1` schema shapes remain
unchanged; only their recorded language-contract value advances. All 45 frozen legacy sources
remain exact.

This slice excludes cross-class or cross-module calls, private callers or targets, overloads,
generics, function values, lambdas, recursion, blocks, locals, branches, loops, effects,
entrypoints, async behavior, Application IR semantics, runtime behavior, and target-specific
semantics. Any next language seam requires another source-backed founder decision and separate
implementation approval.

## Accepted v0.37.0 boundary — general pure-call composition

`v0.37.0` removes only the v0.36.0 placement restriction that limited a call to the complete method
body or a directly nested call argument. The same resolved call node may appear throughout
expressions already admitted as eager and pure, including match-arm bodies, record and Optional
construction, text operations, and arithmetic or comparison operands. Match carriers and
propagation operands remain direct references. No new control-flow form or evaluation order is
introduced.

Semantic analysis retains the uniquely named public same-class target, exact ordered signature,
parameter/arm-local closure, and deterministic `PL3033` cycle rejection. Typed HIR and
target-neutral Core reuse the existing call node and transitive dependency closure. Core validates
nested placement recursively; the evaluator uses isolated copied call frames; the Go backend emits
only Core-validated calls and performs no semantic inference.

TASK-020's Docker observability fixture proves an `Optional<ContainerRow>` selection match whose
`some` arm calls `NormalizeName(row.Name)`. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` retain their schema shapes; only language-contract metadata advances.
All 45 frozen legacy sources remain exact. Cross-class/module calls, private targets, overloads,
generics, function values, lambdas, recursion, blocks, locals, branches, loops, effects,
entrypoints, runtime behavior, and target-specific semantics remain excluded.

## Accepted v0.38.0 boundary — bounded conditional expression

`v0.38.0` admits exactly one `condition ? whenTrue : whenFalse` node per method. The condition is
exactly `bool`; both branches are statically checked and must have the exact same admitted type;
only the selected branch executes. The three operands may contain existing eager pure expressions,
including resolved v0.37.0 same-class calls, but may not contain another conditional, match, or
propagation node. Match and propagation retain their established direct-carrier boundaries.

Typed HIR and target-neutral Core represent the condition and both branches explicitly. Core
independently validates the one-node bound, placement, operand exclusions, condition type, and
branch/result equality. The evaluator selects one branch; the Go backend emits only from validated
Core and performs no semantic inference. TASK-020's Docker observability fixture proves
`name == "" ? fallback : NormalizeName(name)`. Public compiler, semantic projection, and
Application IR schema shapes remain unchanged; only language-contract metadata advances, and all
45 frozen legacy sources remain exact.

This boundary adds no `if` statement, block, local, mutation, implicit conversion, pattern guard,
effect, action, runtime policy, Application IR inference, or target behavior. Any further language
seam requires another source-backed founder decision and separate implementation approval.

## Accepted v0.39.0 boundary — immutable local plus terminal return

`v0.39.0` admits exactly one method block of the form
`{ T name = initializer; return expression; }`. The local type is explicit and must exactly match
the initializer. Initialization is eager and occurs exactly once before the local enters lexical
scope. The return expression may reference parameters, the local, and existing admitted pure
expressions. An explicit checked-arithmetic `Result` local supplies the required result context.
Contextual propagation remains confined to its established complete-method carrier shape and is
excluded from both local expressions. The local cannot shadow a field or parameter and has no
public semantic identity.

Typed HIR and target-neutral Core carry one explicit `immutable_local` node containing the
analysis-local binding, declared type, initializer, and terminal return. Core independently
validates the single top-level node, canonical binding position, initializer scope and type, and
return type. The evaluator copies the initialized value into the local frame once; the Core-only Go
backend emits an equivalent typed lexical binding. TASK-020's Docker observability fixture proves
normalization into one local followed by the existing bounded fallback conditional. Compiler,
semantic projection, and Application IR schema shapes remain unchanged; only language-contract
metadata advances, and all 45 frozen legacy sources remain exact.

This boundary adds no inference, multiple locals, reassignment, shadowing, propagation, early return, `if`
statement, nested block, loop, effect, action, runtime policy, Application IR inference, or target
behavior. Any further language seam requires another source-backed founder decision and separate
implementation approval.

## Accepted v0.40.0 boundary — ordered immutable locals

`v0.40.0` widens only the v0.39.0 method block to one or more source-ordered declarations followed
by one terminal return:
`{ T1 first = expression1; T2 second = expression2; ... return expression; }`. Every local type is
explicit and must exactly match its initializer. Initializers evaluate eagerly exactly once in
source order. A local enters lexical scope only after its initializer, so a later initializer may
reference earlier locals while self-reference and forward reference fail. Local names are unique
within the sequence and cannot shadow fields or parameters. Locals remain immutable and have no
public semantic identity.

Typed HIR and target-neutral Core reuse the explicit `immutable_local` node as one right-nested
top-level sequence with contiguous binding positions after the parameters. Core independently
proves that every local is in that sequence, validates exact initializer and return types, and
rejects parameter/prior-local shadowing or a local in any initializer or non-sequence expression position. The evaluator
and Core-only Go backend evaluate and copy each initialized value once, extend lexical scope in
order, and evaluate the terminal return only after the sequence completes. TASK-020's Docker
observability fixture proves normalization into one local, conditional selection into a second,
and return of the selected value.

Compiler, semantic projection, and Application IR schema identities and shapes remain unchanged;
only language-contract metadata advances, and all 45 frozen legacy sources remain exact. This
boundary adds no inference, reassignment, propagation inside the block, early return, statement
branch, nested block, loop, effect, action, runtime policy, Application IR inference, or target
behavior. Any further language seam requires another source-backed founder decision and separate
implementation approval.

## Accepted v0.41.0 boundary — block-scoped bounded propagation

`v0.41.0` admits exactly one declaration spelled `T name = propagate(carrier);` as the first
immutable local of a public pure method block. `carrier` must be the method's sole direct
parameter, and its carrier type must exactly equal the method return type. `T` must exactly equal
the carried payload type. When the carrier is present or successful, evaluation copies the
validated payload into `name`, then evaluates later ordered locals and the terminal return. When
the carrier is absent or failed, evaluation immediately returns the identical canonical carrier.

The carrier matrix is closed: `Optional<T>` where `T` is an already admitted primitive or record,
`Result<List<R>, string>`, and `Result<string, string>`. Typed HIR and target-neutral Core reuse the
existing `propagate` expression and canonically positioned right-nested `immutable_local` nodes.
Core independently proves the first-local placement, single propagation occurrence, direct sole
parameter operand, exact carrier and payload types, and bounded carrier shape. The evaluator and
Core-only Go backend preserve the same short-circuit and copied-value behavior. TASK-020's Docker
observability consumer proves the text-Result form through `dockpipe.application.v1`.

Compiler, semantic projection, and Application IR schema identities and shapes remain unchanged;
only language-contract metadata advances, and all 45 frozen legacy sources remain exact. No
second or nested propagation, computed operand, propagation from a prior local, arbitrary `Result`
including checked-arithmetic results, inference, reassignment, early return, statement branch,
nested block, loop, effect, action, runtime policy, target behavior, adapter, UI, or deployment
behavior enters by implication. Any further language seam requires another source-backed founder
decision and separate implementation approval.

## Accepted v0.42.0 boundary — prior-local helper propagation

`v0.42.0` admits exactly the first-two-local spelling
`C carrier = Helper(input); T value = propagate(carrier);` in one public pure method block. The
method has exactly one direct parameter `input`. `Helper` must resolve under the existing v0.36.0
same-class public pure-call contract, take exactly that parameter type, and return carrier `C`; the
call argument is exactly the method parameter. `C` must equal the enclosing method return type.
The second initializer propagates a direct reference to the immediately preceding `carrier` local,
and `T` exactly equals the carried payload. Helper evaluation occurs once before propagation.
Presence or success binds a validated copied payload and continues through any later v0.40.0
ordered locals and the terminal return; absence or failure immediately returns the identical
canonical helper carrier.

The carrier matrix remains closed to v0.41.0: `Optional<T>` for an admitted primitive or record,
`Result<List<R>, string>`, and `Result<string, string>`. Typed HIR and target-neutral Core reuse the
existing `call`, right-nested `immutable_local`, and `propagate` nodes. Core independently proves
canonical local positions, one direct helper argument, the resolved same-owner callable and exact
signature, a closed acyclic call graph, one propagation occurrence, the direct prior-local
reference, and exact carrier/payload types. The evaluator and Core-only Go backend preserve
once-only helper evaluation, short-circuit, canonical carrier return, and copied storage.
TASK-020's Docker observability fixture proves `Details(string)` calling
`ValidateDetails(string) -> Result<string, string>`, propagating the helper Result, trimming the
payload, and returning the rebuilt Result through unchanged `dockpipe.application.v1` schema.

Compiler, semantic projection, and Application IR schema identities and shapes remain unchanged;
only language-contract metadata advances, and all 45 frozen legacy sources remain exact. The
v0.41.0 direct-parameter first-local spelling remains unchanged. This slice adds no direct
`propagate(Helper(...))`, multiple or nested propagation, more than one helper argument, computed
helper arguments, propagation from any non-immediately-preceding local, arbitrary Result including
checked arithmetic, inference, reassignment, early return, statement branch, nested block, loop,
effect, action, runtime policy, target behavior, adapter, UI, or deployment behavior. Any further
language seam requires another source-backed founder decision and separate implementation approval.

## Accepted v0.43.0 boundary — helper-result matching composition

`v0.43.0` admits exactly one top-level
`match(Helper(input)) { ok(value) => whenOk, err(error) => whenErr }` expression in a public pure
method with one direct `string` parameter and `string` return. `Helper` resolves to one uniquely
named public pure same-class method, takes that direct parameter as its sole argument, and returns
exactly `Result<string, string>`. It evaluates once. The complete carrier is validated before tag
selection, the selected payload is copied into its unique arm binding, and only the selected arm
expression evaluates. Arms must be exhaustive, wildcard-free, and source ordered `ok` then `err`.

Typed HIR and target-neutral Core reuse the existing `call` and `match` nodes. Core independently
proves top-level placement, one match occurrence, the sole direct helper argument, exact same-owner
caller/helper signatures, a closed acyclic call graph, the text Result carrier, ordered arm tags,
and unique bindings. The evaluator and Core-only Go backend preserve complete-carrier validation,
once-only helper evaluation, copied payloads, and lazy arm selection. TASK-020's Docker observability
fixture proves `DetailsMessage(string)` composing `ValidateDetails(string)` through unchanged
`dockpipe.application.v1` identity and shape.

Compiler, semantic projection, and Application IR schema identities and shapes remain unchanged;
only language-contract metadata advances, and all 45 frozen legacy sources remain exact. This slice
adds no Optional/list/arithmetic Result helper carrier, extra or computed helper argument, extra
caller parameter, cross-class/module call, nested match, reversed or wildcard arm, guard, new block
or local, propagation change, inference, reassignment, statement branch, loop, effect, action,
runtime policy, target behavior, adapter, UI, or deployment behavior. Any further language seam
requires another source-backed founder decision and separate implementation approval.

## Accepted v0.44.0 boundary — general bounded helper-carrier matching

`v0.44.0` admits exactly one complete top-level `match(Helper(...))` in a public pure caller with
one or more parameters. Every caller parameter is passed directly, once, and in declaration order
to a uniquely resolved public pure same-class helper with the exact parameter signature. Its
carrier is closed to admitted `Optional<T>`, `Result<List<R>, string>`, or
`Result<string, string>`. Optional arms are exact source-ordered `some(binding)` then binding-free
`none`; Result arms are exact source-ordered `ok(binding)` then `err(binding)`. Both arm expressions
exactly match the caller return type. The helper evaluates once, the complete carrier is validated,
the selected payload is copied, and only the selected arm evaluates.

Typed HIR and target-neutral Core reuse existing `call` and `match` nodes. Core independently
proves top-level placement, one match, direct parameter position and type, exact same-owner target
signature, closed carrier matrix, canonical arms, bindings, and the closed acyclic call graph. The
evaluator and deterministic Core-only backend preserve once-only evaluation and lazy selection.
TASK-020 proves the exact two-parameter Optional composition through `SelectedNameById` and
`FindSelection` while `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain stable. Only language-contract metadata
advances; the exact 45-source legacy lane remains frozen.

Arithmetic Results, computed/reordered/omitted/extra helper arguments, cross-owner calls,
overloads, generics, nested or multiple matches, wildcard or reversed arms, guards, propagation
changes, new blocks, locals or statements, effects, actions, runtimes, targets, adapters, UI, and
deployment behavior remain excluded. Any successor requires a new founder decision and separate
implementation approval.

## Accepted v0.45.0 boundary — first-local helper-carrier matching

`v0.45.0` admits exactly one v0.44-compatible `match(Helper(...))` as the initializer of the first
explicitly typed immutable local in a public pure caller with one or more parameters. Every caller
parameter is passed directly, once, and in declaration order to one uniquely resolved public pure
same-class helper with the exact parameter signature. Its carrier remains closed to admitted
`Optional<T>`, `Result<List<R>, string>`, or `Result<string, string>`. Optional arms remain exact
source-ordered `some(binding)` then binding-free `none`; Result arms remain exact source-ordered
`ok(binding)` then `err(binding)`. Both arm expressions exactly match the declared local type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`. Core independently
proves first-local placement, one match, direct parameter position and type, exact same-owner target
signature, closed carrier matrix, canonical arms and bindings, exact local typing, and continuation
scope. The evaluator and deterministic Core-only backend validate the complete carrier, evaluate
the helper once, copy the selected result into the local once, evaluate only the selected arm, then
execute existing ordered locals and the terminal return. TASK-020 proves selection followed by
existing normalization through unchanged `dockpipe.application.v1`; `pipelang.compiler.v1` and
`pipelang.semantic.v1` also retain their identities and shapes. Only language-contract metadata
advances, and the exact 45-source legacy lane remains frozen.

Match in later locals, the terminal return, arguments, or nested positions; multiple matches;
computed/reordered/omitted/extra helper arguments; arithmetic Results; cross-owner calls;
overloads; generics; wildcard or reversed arms; guards; propagation changes; inference;
reassignment; early returns; statement branches; loops; effects; actions; runtimes; targets;
adapters; UI; and deployment behavior remain excluded. Any successor requires a new founder
decision and separate implementation approval.

## Accepted v0.46.0 boundary — checked-arithmetic helper matching in the first local

`v0.46.0` widens only the v0.45 first-local helper-carrier matrix to the already admitted
checked-arithmetic `Result<int, ArithmeticError>` and `Result<float, ArithmeticError>` shapes. One
public pure caller with one or more parameters may initialize its first explicitly typed immutable
local with exactly one `match(Helper(...))`. Every caller parameter is passed directly once in
declaration order to one uniquely resolved public pure same-class helper with the exact caller
signature. Arms are exact source-ordered `ok(binding)` then `err(binding)`, and both arm expressions
exactly match the declared local type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`. Core independently
proves first-local placement, one match, direct parameter positions/types, same-owner target and
exact signature, an existing int-or-binary64 checked-arithmetic carrier, canonical arms/bindings,
exact local typing, and continuation scope. The evaluator and deterministic Core-only backend
validate the complete Result, evaluate the helper once, copy the selected payload into the local
once, evaluate only the selected arm, and execute existing ordered locals plus the terminal return.
Integer overflow and binary64 division-by-zero select the error arm deterministically.

`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain stable; only language-contract metadata advances, and the exact 45-source legacy lane
remains frozen. TASK-020 continues to project unchanged through `dockpipe.application.v1`; the new
checked-arithmetic composition supplies compiler-shaped self-hosting value rather than UI or adapter
semantics.

Checked-arithmetic helper matching as the complete method body, in a later local, the terminal
return, arguments, or nested positions; multiple matches; computed/reordered/omitted/extra helper
arguments; arithmetic Result propagation, construction, defaulting, or arbitrary Result widening;
cross-owner calls; overloads; generics; wildcards or reversed arms; guards; inference;
reassignment; early returns; statement branches; loops; effects; actions; runtimes; targets;
adapters; UI; and deployment behavior remain excluded. Any successor requires a new founder
decision and separate implementation approval.

## Accepted v0.47.0 boundary — later-local helper-carrier matching

`v0.47.0` widens only the v0.45/v0.46 local placement: one existing helper-carrier match may
initialize any explicitly typed immutable local after zero or more ordinary locals in the existing
ordered sequence. One public pure caller has one or more parameters, and every parameter is passed
directly once in declaration order to one uniquely resolved public pure same-class exact-signature
helper. The carrier matrix stays closed to admitted Optional primitive/record,
`Result<List<R>, string>`, `Result<string, string>`, and checked-arithmetic
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`. Optional arms remain exact
source-ordered `some(binding)` then binding-free `none`; Result arms remain exact source-ordered
`ok(binding)` then `err(binding)`. Both arms exactly match the declared local type.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, and `match`. Core independently
proves one local match, direct parameter positions/types, same-owner target and exact signature,
the closed carrier matrix, canonical arms/bindings, exact local typing, and continuation scope.
Earlier ordinary locals evaluate eagerly once in source order. The helper evaluates once, the full
carrier is validated, only the selected arm evaluates, and its copied result initializes the
matched local once before later locals and the terminal return continue.

`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain stable; only language-contract metadata advances, and the exact 45-source legacy lane
remains frozen. TASK-020 proves the widened placement through `SelectedNameById`: an ordinary
fallback local precedes the existing Optional helper match, while the Application IR schema and
semantic identities remain unchanged.

Terminal-return, argument, nested, or multiple matches; checked-arithmetic matching as the complete
method body; computed/reordered/omitted/extra helper arguments; propagation changes; Result
construction/defaulting or arbitrary widening; cross-owner/private/overloaded/generic helpers;
wildcards or reversed arms; guards; inference; reassignment; statements; effects; actions;
runtimes; targets; adapters; UI; and deployment behavior remain excluded. No statement, effect,
runtime, action, target, adapter, UI, or deployment behavior enters by implication. Any successor
requires a new founder decision and separate implementation approval.

## Accepted v0.48.0 boundary — prior-local carrier matching

`v0.48.0` widens only the local spelling accepted through v0.47. After zero or more ordinary
locals, one explicitly typed carrier local is initialized by `Helper(p1, ..., pn)` and the
immediately adjacent explicitly typed local is initialized by exactly one `match(carrier)`. The
caller is public and pure with one or more parameters; every parameter is passed directly once in
declaration order to one uniquely resolved public pure same-class exact-signature helper. The
carrier matrix stays closed to admitted Optional primitive/record, `Result<List<R>, string>`,
`Result<string, string>`, and checked-arithmetic `Result<int, ArithmeticError>` or
`Result<float, ArithmeticError>`. Arms retain exact canonical source order and bindings.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`. Core
independently proves adjacency, the exact carrier-local reference, one match, direct parameter
positions/types, same-owner exact signature, the closed carrier matrix, canonical arms/bindings,
exact local typing, and continuation scope. Earlier locals evaluate eagerly once; the helper and
carrier local evaluate once; the complete carrier is validated; only the selected arm evaluates;
the copied result initializes the matched local once; and later locals plus the terminal return
continue. The evaluator and deterministic Core-only Go backend preserve the same semantics.

`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain stable; only language-contract metadata advances, and the exact 45-source legacy lane
remains frozen. TASK-020 proves the exact source spelling through `SelectedNameById`: a fallback
local precedes `Optional<ContainerRow> selection = FindSelection(rows, id);`, immediately followed
by `string selected = match(selection) { ... };`, without Application IR schema change.

Non-adjacent, terminal-return, argument, nested, or multiple matches; checked-arithmetic matching
as a complete method body; computed/reordered/omitted/extra helper arguments; propagation changes;
Result construction/defaulting or arbitrary widening; cross-owner/private/overloaded/generic
helpers; wildcards or reversed arms; guards; inference; reassignment; statements; effects; actions;
runtimes; targets; adapters; UI; and deployment behavior remain excluded. No statement, effect,
runtime, action, target, adapter, UI, or deployment behavior enters by implication. Any successor
requires a new founder decision and separate implementation approval.

## Accepted v0.50.0 boundary — dependent second-carrier matching

`v0.50.0` adds one new exact two-match spelling:
`C1 firstCarrier = Helper1(p1, ..., pn); T1 first = match(firstCarrier) { ... };`
`C2 secondCarrier = Helper2(first, p1, ..., pn); T2 second = match(secondCarrier) { ... };`.
These are four contiguous explicitly typed locals. Ordinary locals may appear before or after the
stage only. `Helper1` keeps the v0.49 exact caller signature. `Helper2` resolves uniquely to a
public pure same-class method whose first parameter exactly matches the first selected local,
followed by every caller parameter directly once in declaration order. Both pairs keep the closed
carrier matrix and canonical arms.

HIR and Core reuse existing nodes; Core independently verifies the four-local stage, binding
positions, exact helper ownership/signatures, carriers, arms, local types, and continuation. All
locals and helpers evaluate once in source order, every complete carrier is validated, and only
selected arms evaluate. Docker observability proves `ConfirmSelection(selected, rows, id)` without
schema change. Public compiler, semantic, and Application IR identities and shapes stay stable;
only language metadata advances, and the exact 45-source lane remains frozen.

Existing zero-match, one-match, and independent v0.49 two-pair forms remain exact. Other local
argument arrangements, computed/reordered/repeated arguments, split stages, third/nested/terminal
matches, propagation changes, arbitrary Result widening, statements, effects, actions, runtimes,
targets, adapters, UI, and deployment remain excluded. No behavior enters by implication. Any
successor requires a new founder decision and separate implementation approval.

## Accepted v0.51.0 boundary — general dependent carrier chains

`v0.51.0` extends the v0.50 dependency rule to one exact chain of `k >= 2` pairs:
`C1 carrier1 = Helper1(p1, ..., pn); T1 value1 = match(carrier1) { ... };`, then
`Ci carrierI = HelperI(valueI-1, p1, ..., pn); Ti valueI = match(carrierI) { ... };` for every
later stage. All `2k` explicitly typed locals are contiguous. Ordinary locals may appear before or
after only. `Helper1` keeps the exact caller signature. Every later helper resolves uniquely to a
public pure same-class method receiving the immediately preceding selected local, followed by every
caller parameter directly once in declaration order. All pairs retain the closed carrier matrix and
canonical arms.

HIR and Core reuse `immutable_local`, `call`, `reference`, and `match`; Core independently verifies
the complete chain, immediate-predecessor dependencies, binding and argument positions, exact
helper ownership/signatures, carriers, arms, local types, and continuation. All locals and helpers
evaluate once in source order, every complete carrier is validated, and only selected arms
evaluate. Docker observability proves `FinalizeSelection(confirmed, rows, id)` as a third stage
without schema change. Public compiler, semantic, and Application IR identities and shapes stay
stable; only language metadata advances, and the exact 45-source lane remains frozen.

Existing zero-match, one-match, v0.49 independent two-pair, and v0.50 dependent two-stage forms
remain exact. Gaps, mixed independent/dependent chains, non-immediate dependencies, fan-in,
computed/reordered/repeated/omitted/extra arguments, third matches outside the exact chain,
nested/terminal/argument matches, propagation changes, arbitrary Result widening, statements,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication. Any successor requires a new founder decision and separate implementation
approval.

## Accepted v0.52.0 boundary — cumulative fan-in carrier chains

`v0.52.0` adds one cumulative chain of `k >= 3` contiguous carrier/match pairs. Stage one is
`C1 carrier1 = Helper1(p1, ..., pn); T1 value1 = match(carrier1) { canonical arms };`. Every later
stage is exactly `Ci carrierI = HelperI(value1, ..., valueI-1, p1, ..., pn); Ti valueI =
match(carrierI) { canonical arms };`: all prior selected locals occur directly once in chain order,
followed by every caller parameter directly once in declaration order. A method uses either this
cumulative mode or the inherited v0.51 immediate-only mode; modes cannot mix between stages.

HIR and Core reuse `immutable_local`, `call`, `reference`, and `match`; Core independently verifies
the complete chain, cumulative binding positions, helper ownership/signatures, carriers, arms,
local types, and continuation. All locals and helpers evaluate once in source order, every complete
carrier is validated, and only selected arms evaluate. Docker observability proves
`FinalizeSelectionHistory(selected, confirmed, rows, id)` as the cumulative third stage without
schema change. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1`
identities and shapes stay stable; only language metadata advances, and the exact 45-source lane
remains frozen.

All inherited forms remain exact. Fewer than three cumulative pairs, partial/reordered/repeated/
omitted/extra prior selections, per-stage mode mixing, gaps, non-chain dependencies, computed
arguments, matches outside the chain, nested/terminal/argument matches, propagation changes,
arbitrary Result widening, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No behavior enters by implication. Any successor requires a new
founder decision and separate implementation approval.

## Accepted v0.53.0 boundary — multi-parameter helper propagation

`v0.53.0` widens only the v0.42 prior-local propagation helper signature. One public pure method
with at least two parameters may spell `C carrier = Helper(p1, ..., pn); T value =
propagate(carrier); return admittedExpression;`. The helper call is the first immutable-local
initializer, the propagation local is immediately adjacent, and the uniquely resolved public pure
same-class helper receives every caller parameter directly once in declaration order. `C` equals
the method return carrier and `T` equals its payload. The inherited one-parameter v0.42 form and
closed Optional primitive/record, `Result<List<R>, string>`, and `Result<string, string>` carrier
matrix remain exact.

HIR and Core reuse `immutable_local`, `call`, `reference`, and `propagate`; Core independently
checks argument positions, helper identity/signature, adjacency, carrier/payload types, and the
single propagation. Evaluation and Core-only Go call the helper once, validate the complete
carrier, copy success, and return canonical absence/failure otherwise. Docker observability proves
`ResolveSelection(rows, id)` calling `FindSelection(rows, id)` without schema change.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
stay stable; only language metadata advances, and the exact 45-source lane remains frozen.

Computed/reordered/repeated/omitted/extra arguments, a non-first carrier, an intervening local,
non-adjacent or additional propagation, mismatched carriers/payloads,
cross-owner/private/overloaded/generic helpers, arbitrary Result widening, inference,
reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No behavior enters by implication. Any successor requires a new founder decision and
separate implementation approval.

## Accepted v0.54.0 boundary — checked-arithmetic helper propagation

`v0.54.0` widens only the exact v0.53 prior-local helper propagation carrier matrix. One public
pure method spells `Result<int, ArithmeticError> carrier = Helper(p1, ..., pn); int value =
propagate(carrier); return admittedCheckedArithmeticExpression;`, or the identical `float` form.
The helper call is the first typed local, propagation is the immediately adjacent second local,
and every caller parameter is passed directly once in declaration order to one uniquely resolved
public pure same-class helper. The helper carrier equals the method return type and the propagated
local equals its success payload.

HIR and Core reuse `immutable_local`, `call`, `reference`, `propagate`, and checked arithmetic;
Core independently checks exact placement, argument positions, helper identity/signature,
carrier/payload types, one propagation, and the continuation. Evaluation and deterministic
Core-only Go call the helper once, validate the complete arithmetic Result, copy success, and
return canonical overflow or division-by-zero before evaluating the continuation.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
stay stable; only language metadata advances. A compiler-cursor fixture proves checked offset
propagation as a concrete self-hosting consumer, and the exact 45-source lane remains frozen.

Direct-parameter arithmetic propagation, `propagate(Helper(...))`, later or split pairs, computed,
reordered, repeated, omitted, or extra arguments, additional propagation, arbitrary Result
widening, inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI,
and deployment remain excluded. No behavior enters by implication. Any successor requires a new
founder decision and separate implementation approval.

## Accepted v0.55.0 boundary — direct-parameter checked propagation

`v0.55.0` widens only the inherited v0.41 first-local direct-parameter propagation carrier matrix.
One public pure method spells `Result<int, ArithmeticError> Continue(Result<int, ArithmeticError>
carrier) { int value = propagate(carrier); return admittedCheckedArithmeticExpression; }`, or the
identical `float` form. The arithmetic Result is the method's sole direct parameter and exactly
equals its return type. Propagation is the first typed local, whose declared type exactly equals the
success payload. The terminal return is one already admitted checked add, subtract, multiply,
negate, or binary64 divide expression.

HIR and Core reuse `immutable_local`, `reference`, `propagate`, and checked arithmetic. Core
independently verifies first-local placement, the sole direct carrier reference, carrier/return and
payload/local equality, one propagation, and the checked continuation. Evaluation and
deterministic Core-only Go validate the complete arithmetic Result, copy success, and return
canonical overflow or division-by-zero before evaluating the continuation.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
stay stable; only language metadata advances. A compiler-cursor fixture proves consumption of a
checked cursor Result passed by the caller, and the exact 45-source lane remains frozen.

Additional parameters, helper or computed propagation operands, later or split placement,
additional propagation, mismatched carrier/payload types, arbitrary Result widening, inference,
reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. The v0.54 helper form and every earlier contract remain exact. No behavior enters by
implication. Any successor requires a new founder decision and separate implementation approval.

## Accepted v0.49.0 boundary — bounded two-carrier matching

`v0.49.0` widens only the occurrence count accepted through v0.48. One public pure method with one
or more parameters may contain exactly two non-overlapping adjacent helper-carrier pairs:
`C1 firstCarrier = Helper1(p1, ..., pn); T1 first = match(firstCarrier) { ... };` followed later by
`C2 secondCarrier = Helper2(p1, ..., pn); T2 second = match(secondCarrier) { ... };`. Zero or more
ordinary immutable locals may appear before, between, or after the pairs, but no local may split a
carrier from its match local. Every helper receives every caller parameter directly once in
declaration order and resolves uniquely to a public pure same-class method with that exact signature.
Each pair independently retains the v0.48 Optional primitive/record, `Result<List<R>, string>`,
`Result<string, string>`, and checked-arithmetic Result matrix plus canonical source-ordered arms.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`. Core
independently proves exactly two matches, exactly two non-overlapping adjacent pairs, each exact
carrier reference, direct parameter positions/types, same-owner exact signatures, closed carrier
types, canonical arms/bindings, exact local typing, and continuation scope. All surrounding locals,
helpers, carriers, and selected locals evaluate eagerly once in source order. Each complete carrier
is validated and only its selected arm evaluates. Both pairs complete before later locals and the
terminal return; the second pair and its arms may use prior locals under existing immutable scope.

`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain stable; only language-contract metadata advances, and the exact 45-source lane remains
frozen. TASK-020's Docker observability fixture proves `FindSelection(rows, id)` and
`ConfirmSelection(rows, id)` as two exact-signature helpers, each stored in an adjacent carrier and
canonical match-local pair before the normalized terminal return, without Application IR schema
change.

Existing zero-match and one-match forms remain exact. A third match, non-adjacent or overlapping
pairs, terminal-return/argument/nested matching, non-helper or computed carriers, computed/
reordered/omitted/extra helper arguments, propagation changes, Result construction/defaulting or
arbitrary widening, cross-owner/private/overloaded/generic helpers, wildcard or reversed arms,
guards, inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment behavior remain excluded. No statement, effect, runtime, action, target, adapter, UI,
or deployment behavior enters by implication. Any successor requires a new founder decision and
separate implementation approval.

## Accepted first Application IR boundary — `dockpipe.application.v1`

The first target-neutral read-only Application IR consumes only a canonical public
`pipelang.semantic.v1` projection, its contract-matching Core program, and an explicit
source-located stable-identity spec. The spec names one application Core function and typed
snapshot record; each section names its Result type, row record, stable key, visible columns,
filter fields, and ascending/descending ordinal order; Optional row selection and Result text
details/log identities are explicit. All identities must exist in the semantic projection and the application identity
must exist in Core. Sections are sorted by identity, declared column/filter/order sequences remain
stable, empty slices are non-nil, and canonical indented JSON is deterministic. Invalid inputs are
rejected at the spec source range. PipeLang semantics and identities are unchanged. Parsing,
evaluation, semantic inference, targets, Docker, refresh, actions, services, launcher migration,
and CLI behavior are excluded.

Filter and order behavior is bound, not inferred: their explicit identities must resolve both to
semantic callables and Core functions with exact `(List<Row>, string) -> List<Row>` and
`(List<Row>) -> List<Row>` signatures. Section Result, selection, details, and logs roles are also
explicit Core-backed method identities with their recorded structured return types.
