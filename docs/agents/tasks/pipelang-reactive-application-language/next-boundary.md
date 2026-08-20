## Exact Next Boundary

Step 7 of **Bounded Implementation Order** has completed the fixed numeric, compiler-internal
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
All other numeric arithmetic and every other Result construction, composition, or consumption form
remain fail-closed from production source. Any next slice requires a new
synchronized decision for its exact source spelling, type/value handling rule, semantic projection,
migration, and bounded semantics before implementation.

The first accepted application consumer is TASK-020's one-to-one DockPipe Launcher replacement,
beginning with read-only Docker observability. Its typed snapshot requirements are dependency
evidence when comparing remaining step-7 options. Completed primitive record transport, one-hop
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
The exact one-field ordinal sort now supplies a target-neutral deterministic collection-ordering
primitive, but applying it to the first launcher would require a separate TASK-020 parity decision
because the checked-in oracle does not expose table sorting. Dynamic field selection, general
predicates, descending or multi-key sorting, general indexing,
propagation, matching, and application projection remain later decisions.

No later step-7 slice is included here. In particular, this checkpoint does not add general Result
construction, inspection, extraction, wrapping, unwrapping, propagation, or matching beyond the
exact accepted `Result<List<R>, string>` and `Result<string, string>` forms; additional
Unicode text construction/scalar/grapheme APIs, trim variants or composition, normalization,
locale-aware or additional case operations, value/reference,
hashing, general total-order capabilities, optional extraction beyond `value_or`, equality,
implicit defaults, nesting, chaining,
general result, record nesting/chained or general access/mutation/hash/order, union, or additional
deterministic collection production or consumption semantics beyond exact `filter_by`,
`filter_contains_casefolded`, `filter_joined_contains_casefolded`, and `sort_by_ordinal`; accept
namespace, import, migration, `internal`, overload, generic, or ID production syntax; implement
overload resolution;
add new types/declarations/expressions/operators, blocks, locals, branches, or loops; add effects,
entrypoints, actions/state, contracts/replay, executable application/service semantics, Application
IR, Service IR, another backend, or self-hosting; mutate generated stores; or widen Go emission by
guessing step-7 semantics. Exact successor production spellings remain later synchronized language
slices.
