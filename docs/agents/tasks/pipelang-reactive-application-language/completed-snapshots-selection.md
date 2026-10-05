### Completed step 7t read-only snapshot Result contract (2026-08-18)

The founder selected explicit `v0.19.0` bounded snapshot Result values. The accepted public surface
contains only these direct method shapes for one existing public primitive record `R`:

```pipelang
public Result<List<ContainerRow>, string> RowsOk(List<ContainerRow> value) =>
    ok<List<ContainerRow>, string>(value);
public Result<List<ContainerRow>, string> RowsFailed(string error) =>
    err<List<ContainerRow>, string>(error);
public Result<List<ContainerRow>, string> ForwardRows(
    Result<List<ContainerRow>, string> value
) => value;
public bool RowsSucceeded(Result<List<ContainerRow>, string> value) => is_ok(value);
public List<ContainerRow> RowsOr(
    Result<List<ContainerRow>, string> value,
    List<ContainerRow> fallback
) => success_or(value, fallback);
public string ErrorOr(
    Result<List<ContainerRow>, string> value,
    string fallback
) => failure_or(value, fallback);
```

`Result<List<R>, string>` reuses the fixed `pipelang:result` and `pipelang:list` identities, the
existing record identity, and primitive `string`; callable identity preserves each exact parameter
and return shape. `pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and
target-neutral Core carry explicit `result_ok`, `result_err`, `result_is_ok`, `result_success_or`,
and `result_failure_or` nodes. The evaluator and Core-only Go backend use canonical tagged values,
validate every active payload plus every fallback before selection, validate all record fields and
strict UTF-8 strings, and copy list and record storage for construction, identity transport, and
selection.

`PL3006` reports invalid payload, placement, or signature types; `PL3009` reports computed,
reordered, additional, or otherwise non-direct operands and bodies. Malformed typed HIR and Core
remain `PL3026` and `PL3027`. `v0.1.0` through `v0.18.0` reject the source and executable form
without implicit migration, while every frozen earlier contract remains available under
`v0.19.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/snapshot-result.pipe`, with
synchronized typed HIR, Core, and Go goldens. This slice gives TASK-020's first read-only Docker
observability consumer one deterministic success/failure envelope for a complete row snapshot and
bounded cached-snapshot or error fallback without introducing application behavior. General Result
types, arithmetic-Result construction, propagation, matching, chaining, fields, nesting, arbitrary
success/failure payloads, list iteration/filtering/sorting/indexing, Application IR, Step-8 control
flow, effects, and additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang and exact 45-source compatibility suites, including parser spans,
  structured identity/projection, source-derived HIR/Core/Go snapshot-Result goldens, success/
  failure/identity/inspection/fallback evaluator agreement, caller-storage isolation, complete
  selected and unselected payload/fallback validation, strict UTF-8 rejection, malformed HIR/Core
  rejection, deterministic exclusions, explicit `v0.1.0` through `v0.18.0` migration rejection,
  and preserved `v0.1.0` through `v0.18.0` behavior;
- focused application PipeLang check/compile/invoke/materialize, package-compile, and `src/cmd`
  checks through only a temporary `/tmp` modfile mapped to cached `x/sys v0.46.0`;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code grammar/snippet/diagnostic
  validation, and Windows compile-only proof across affected packages; and
- `gofmt`, `git diff --check`, exact 45-source inventory, dependency/generated-state absence, and
  engine/package-boundary review.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7u safe primitive-record list indexing (2026-08-18)

The founder selected explicit `v0.20.0` safe zero-based indexing for one existing public primitive
record `R`. The accepted public surface contains only this direct method shape:

```pipelang
public Optional<ContainerRow> RowAt(
    List<ContainerRow> values,
    int index
) => at(values, index);
```

`at(List<R>, int) -> Optional<R>` reuses the fixed `pipelang:list` and `pipelang:optional`
identities, the existing record identity, and signed 64-bit primitive `int`; callable identity
preserves the exact two-parameter and return shape. `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry one explicit
`list_at` node with direct list and index references.

The evaluator and Core-only Go backend validate the complete list and every record field before
inspecting the index. Negative or out-of-range indexes return canonical `none`; an in-range index
returns canonical `some` with copied record storage. Strict UTF-8 validation therefore rejects an
invalid string in any selected or unselected record even when the index is absent. Nil or malformed
list values remain invalid rather than becoming absence.

`PL3006` reports invalid list, element, index, return, placement, or signature shapes; `PL3009`
reports computed or otherwise non-direct operands and bodies after the exact signature is admitted.
Malformed typed HIR and Core remain `PL3026` and `PL3027`. `v0.1.0` through `v0.19.0` reject the source and executable form
without implicit migration, while every frozen earlier contract remains available under
`v0.20.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/record-list-at.pipe`, with
synchronized typed HIR, Core, and Go goldens. This slice gives TASK-020's first read-only Docker
observability consumer deterministic safe row selection by snapshot position without introducing
application behavior. Key lookup, slicing, filtering, sorting, iteration, mutation, arbitrary
collection consumption, Optional chaining or extraction beyond existing `value_or`, Application
IR, Step-8 control flow, effects, and additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang and exact 45-source compatibility suites, including parser spans,
  structured identity/projection, source-derived HIR/Core/Go list-at goldens, evaluator and
  generated-Go agreement for in-range, negative, and out-of-range indexes, caller-storage
  isolation, complete selected and unselected element validation, strict UTF-8 rejection,
  malformed HIR/Core rejection, deterministic exclusions, explicit `v0.1.0` through `v0.19.0`
  migration rejection, and preserved `v0.19.0` snapshot-Result behavior;
- focused application PipeLang consumer checks, editor grammar/snippet/diagnostic validation,
  `go vet`, and Windows compile-only proof across affected packages; and
- `gofmt`, `git diff --check`, exact 45-source inventory, dependency/generated-state absence,
  Core-only backend/evaluator import-boundary checks, and engine/package-boundary review.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7v stable-key primitive-record list lookup (2026-08-19)

The founder selected explicit `v0.21.0` stable-key lookup for one existing public primitive record
`R`. The accepted public surface contains only this direct method shape:

```pipelang
public Optional<ContainerRow> FindRow(
    List<ContainerRow> values,
    string key
) => find_by(values, ContainerRow.Id, key);
```

`find_by(List<R>, R.Field, string) -> Optional<R>` reuses the fixed `pipelang:list` and
`pipelang:optional` identities, primitive `string`, the existing record identity, and the selected
field semantic identity. `R.Field` is a static selector for one public string field on the same
record type, not a runtime value, lambda, predicate, comparer, or general member expression.
Callable identity preserves the exact two-parameter and return shape. `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry one explicit
`list_find_by_text` node with direct list/key references and the selected field identity, name, and
declaration position.

The evaluator and Core-only Go backend validate the complete list, every record field, and the key
before lookup. The first record whose selected field is ordinal-equal to the key returns canonical
`some` with copied record storage; no match returns canonical `none`. Unicode scalar sequences are
preserved without normalization, case-folding, locale, or target collation. Nil or malformed list,
record, Optional, or string values remain invalid rather than becoming absence.

`PL3004` reports unknown or inaccessible selector fields; `PL3006` reports invalid list, selector,
field type, key, return, placement, or signature shapes; `PL3009` reports computed or otherwise
non-direct list/key operands after the exact signature is admitted. Malformed typed HIR and Core
remain `PL3026` and `PL3027`. `v0.1.0` through `v0.20.0` reject the source and executable form
without implicit migration, while every frozen earlier contract remains available under
`v0.21.0`.

The minimal pure source fixture is
`src/lib/pipelang/testdata/record-list-find-by-text.pipe`, with synchronized typed HIR, Core, and Go
goldens. This slice gives TASK-020's first read-only Docker-observability consumer stable-ID row
selection and detail lookup without introducing application behavior. Lambdas, predicates,
composite keys, case-insensitive or normalized lookup, map/index construction, slicing, filtering,
sorting, iteration, mutation, arbitrary collection consumption, Optional chaining or extraction
beyond existing `value_or`, Application IR, Step-8 control flow, effects, and additional backends
remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang and exact 45-source compatibility suites, including parser selector spans,
  structured field identity/projection, source-derived HIR/Core/Go `list_find_by_text` goldens,
  evaluator and generated-Go agreement for first duplicate match, missing keys, alternate declared
  string fields, and composed/decomposed Unicode keys, caller-storage isolation, complete selected
  and unselected element validation, invalid-key UTF-8 rejection, malformed HIR/Core rejection,
  deterministic exclusions, explicit `v0.1.0` through `v0.20.0` migration rejection, and preserved
  `v0.20.0` list-at behavior;
- focused application PipeLang/catalog consumers through a temporary `/tmp` modfile mapped only to
  cached `x/sys v0.46.0`, editor grammar/completion/snippet/diagnostic validation, `go vet`, and
  Windows compile-only proof across affected packages; and
- `gofmt`, `git diff --check`, exact frozen inventory digest/count, dependency/generated-state
  absence, Core-only backend/evaluator import-boundary checks, and engine/package-boundary review.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7w selected-field primitive-record list filtering (2026-08-19)

The founder selected explicit `v0.22.0` stable-order filtering for one existing public primitive
record `R`. The accepted public surface contains only this direct method shape:

```pipelang
public List<ContainerRow> FilterRows(
    List<ContainerRow> values,
    string key
) => filter_by(values, ContainerRow.State, key);
```

`filter_by(List<R>, R.Field, string) -> List<R>` reuses the fixed `pipelang:list` identity,
primitive `string`, the existing record identity, and the selected field semantic identity.
`R.Field` is the same static selector shape established by `find_by`: one public string field on
the same record type, not a runtime value, lambda, predicate, comparer, or general member
expression. Callable identity preserves the exact `(List<R>, string) -> List<R>` shape.
`pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral
Core carry one explicit `list_filter_by_text` node with direct list/key references and the selected
field identity, name, and declaration position.

The evaluator and Core-only Go backend validate the complete list, every record field, and the key
before filtering. Every record whose selected field is ordinal-equal to the key is retained in
input order, including duplicates. No matches return canonical non-nil empty `List<R>` storage.
Results use fresh copied list and record storage and retain the signed-64-bit cardinality boundary.
Unicode scalar sequences remain preserved without normalization, case-folding, locale, or target
collation. Nil or malformed list, record, or string values remain invalid rather than becoming an
empty result.

`PL3004` reports unknown or inaccessible selector fields; `PL3006` reports invalid list, selector,
field type, key, return, placement, or signature shapes; `PL3009` reports computed or otherwise
non-direct list/key operands after the exact signature is admitted. Malformed typed HIR and Core
remain `PL3026` and `PL3027`. `v0.1.0` through `v0.21.0` reject the source and executable form
without implicit migration, while every frozen earlier contract remains available under
`v0.22.0`.

The minimal pure source fixture is
`src/lib/pipelang/testdata/record-list-filter-by-text.pipe`, with synchronized typed HIR, Core, and
Go goldens. This slice gives TASK-020's first read-only Docker-observability consumer deterministic
exact-field snapshot subsets while preserving adapter order. Lambdas, predicates, multi-field or
substring search, case-folding, normalization, sorting, mapping, folding, general iteration,
mutation, arbitrary collection consumption, Application IR, Step-8 control flow, effects, and
additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang and exact 45-source compatibility suites, including parser selector spans,
  structured field identity/projection, source-derived HIR/Core/Go `list_filter_by_text` goldens,
  evaluator and generated-Go agreement for stable duplicate order, empty results, a non-leading
  declared string field, composed/decomposed Unicode keys, caller-storage isolation, complete
  selected and unselected element validation, invalid-key UTF-8 rejection, malformed HIR/Core
  rejection, deterministic exclusions, explicit `v0.1.0` through `v0.21.0` migration rejection,
  and preserved `v0.21.0` stable-key behavior;
- focused application PipeLang/catalog consumers, editor grammar/completion/snippet/diagnostic
  validation, `go vet`, and Windows compile-only proof across affected packages; and
- `gofmt`, `git diff --check`, exact frozen inventory digest/count, dependency/generated-state
  absence, Core-only backend/evaluator import-boundary checks, and engine/package-boundary review.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7x Unicode case-folded text containment (2026-08-19)

The founder selected explicit `v0.23.0` full-default case-folded containment with this sole direct
source shape:

```pipelang
public bool ContainsCaseFolded(string value, string query) =>
    contains_casefolded(value, query);
```

The method requires exactly two direct `string` parameters in declared order, a `bool` return, and
the operation as its complete body. Callable identity remains `(string, string) -> bool`;
`pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral
Core carry one explicit `text_contains_case_folded` node. `v0.1.0` through `v0.22.0` reject the
source, HIR, and Core form without implicit migration.

The evaluator and Core-only Go backend validate both UTF-8 operands before applying the same pinned
Unicode 17.0.0 full-default C/F case-fold table. They then perform contiguous scalar-sequence
containment over the folded strings; an empty query matches. The checked-in Unicode data has
SHA-256 `ff8d8fefbf123574205085d6714c36149eb946d717a0c585c27f0f4ef58c4183` and yields exactly 1,585
source-sorted mappings. S/T mappings, normalization, locale tailoring, grapheme segmentation,
trimming, and host Unicode/case conversion are excluded.

The synchronized fixture and typed HIR, Core, and Go goldens prove full multi-scalar folding,
dotted-I and Kelvin mappings, no normalization, empty-query behavior, invalid UTF-8 rejection,
malformed HIR/Core rejection, deterministic Go emission, exact earlier-version rejection, and
preservation of every earlier contract. This gives TASK-020's first read-only Docker-observability
consumer a deterministic predicate for human-entered status/log matching without adding list
filtering, multi-field search, lambdas, composition, Application IR, Step-8 control flow, effects,
or another backend.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:
the complete PipeLang suite, exact 45-source compatibility suite, affected domain and PipeLang
application tests, editor grammar/completion/snippet assertions, `go vet`, Windows compile-only
proof, `gofmt`, and `git diff --check`. The application dependency seam used only a temporary
`/tmp` modfile selecting the already cached `x/sys v0.46.0`; `go.mod` and `go.sum` are unchanged.

No dependency, generated store, runtime, Docker, VM, credential, cleanup, commit, push,
publication, or other external state changed. The only network action was the separately approved
single download of Unicode 17.0.0 `CaseFolding.txt`; its exact bytes are now pinned by the digest
above.

### Completed step 7y selected-field case-folded record-list filtering (2026-08-19)

The founder selected explicit `v0.24.0` stable-order filtering by Unicode case-folded containment
with this sole direct source shape:

```pipelang
public List<ContainerRow> SearchRows(
    List<ContainerRow> values,
    string query
) => filter_contains_casefolded(values, ContainerRow.Name, query);
```

The method requires exactly one existing public primitive record `R`, direct `List<R>` and
`string` parameters in declared order, a matching `List<R>` return, and one static public string
field selector on `R`. Callable identity remains `(List<R>, string) -> List<R>` and reuses
`pipelang:list`, primitive `string`, the record identity, and the selected field identity.
`pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral
Core carry the explicit `list_filter_contains_case_folded_text` node with direct list/query
references plus field identity, name, and declaration position.

The evaluator and Core-only Go backend completely validate the list, every record field, and query
before applying the same pinned Unicode 17.0.0 full-default C/F case-fold table used by
`contains_casefolded`. Every contiguous folded match is retained in stable input order, including
duplicates. Empty queries retain all elements; no matches produce canonical non-nil empty storage;
results use fresh copied list and record storage. No host Unicode API supplies language semantics.

`v0.1.0` through `v0.23.0` reject the source, HIR, and Core form without implicit migration. The
synchronized source fixture and typed HIR, Core, and Go goldens cover full multi-scalar folding,
Kelvin and dotted-I mappings, no normalization, empty-query behavior, invalid UTF-8 and nil-list
rejection, stable duplicate order, copied storage, selector identity validation, malformed HIR/Core
rejection, deterministic emission, and preservation of the exact `v0.23.0` predicate.

This supplies TASK-020's read-only Docker-observability consumer with deterministic search over one
selected visible string field. Trimming, joined or multi-field search, normalization, locale
tailoring, predicates, lambdas, sorting, general iteration, mutation, Application IR, Step-8
control flow, effects, and additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13 and local/offline module inputs: the complete PipeLang
suite; exact 45-source compatibility suite; affected domain, materialization, and application
PipeLang tests; editor grammar/completion/snippet assertions; `go vet`; Windows/amd64 compile-only
proof; `gofmt`; `git diff --check`; frozen inventory digest/count; unchanged dependency files; and
Core-only evaluator/backend import-boundary checks. The application check used only a temporary
`/tmp` modfile and local cached module proxy selecting cached `x/sys v0.46.0`; no checkout
dependency, generated store, network, runtime, Docker, VM, credential, cleanup, commit, push,
publication, or other external state changed.

### Completed step 7z fallible text envelope (2026-08-19)

The founder selected explicit `v0.25.0` construction, identity transport, success inspection, and
bounded defaulting for `Result<string, string>` through the exact direct `ok<string, string>`,
`err<string, string>`, Result parameter identity, `is_ok`, `success_or`, and `failure_or` method
forms. The slice reuses `pipelang:result`, primitive `string`, and the existing Result HIR/Core
expression kinds; `pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged.

The evaluator and Core-only Go backend completely validate tagged values and every supplied text
payload or fallback as strict UTF-8 before selection. Failures carry a canonical empty success
payload. `v0.1.0` through `v0.24.0` reject the source, HIR, and executable Core forms without
implicit migration. Other Result types or source forms, `is_err`, unwrap, propagation, mapping,
matching, effects, composition, Application IR, Step-8 control flow, and additional backends remain
excluded.

The synchronized fixture and tests cover semantic projection, all six typed HIR/Core expression
kinds, evaluator behavior, deterministic generated Go and generated-code execution, strict UTF-8
validation including unselected fallbacks, canonical tagged-value rejection, malformed HIR/Core,
parser spans, explicit migration, and excluded source shapes. This gives TASK-020's first read-only
Docker-observability consumer a bounded fallible text value for adapter status and diagnostic
transport without introducing general Result composition or application semantics.

Terminal proof passed with cached Go 1.25.13 and local/offline module inputs: the complete PipeLang
suite; exact 45-source compatibility suite; affected application and `src/cmd` tests; editor
grammar/completion/snippet assertions; `go vet`; Windows/amd64 compile-only proof; `gofmt`; `git
diff --check`; frozen inventory digest/count; unchanged dependency files; and Core-only
evaluator/backend import-boundary checks. The application and CLI checks used only a temporary
`/tmp` modfile and the local cached module proxy selecting cached `x/sys v0.46.0`; no checkout
dependency, generated store, network, runtime, Docker, VM, credential, cleanup, commit, push,
publication, or other external state changed.

### Completed step 7aa deterministic direct text trimming (2026-08-19)

The founder selected explicit `v0.26.0` direct deterministic text trimming with this sole source
shape:

```pipelang
public string Trim(string value) => trim(value);
```

The method requires exactly one direct `string` parameter, the identical primitive `string`
return, and `trim(value)` as its complete body. Callable and primitive-string identities remain
unchanged; `pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and
target-neutral Core carry one explicit `text_trim` node. `v0.1.0` through `v0.25.0` reject the
source, HIR, and executable Core form without implicit migration.

The evaluator and Core-only Go backend first reject invalid UTF-8, then remove the maximal leading
and trailing sequence of scalars from the pinned Unicode 17.0.0 `White_Space` property. The exact
table contains 25 scalars in 10 source-ordered ranges. Interior scalars are preserved byte-for-byte;
all-whitespace input becomes canonical empty text. U+180E, U+200B, and U+FEFF remain ordinary text.
No host whitespace API supplies the language semantics.

The synchronized source fixture and typed HIR, Core, and Go goldens cover ASCII and non-ASCII
boundaries, interior preservation, all-whitespace and empty values, explicit non-whitespace
exclusions, strict UTF-8 rejection, malformed HIR/Core rejection, deterministic generated Go,
generated-code execution, exact earlier-version rejection, and preservation of `v0.25.0` text
Results. This gives TASK-020's first read-only Docker-observability consumer deterministic cleanup
for adapter-supplied labels, filter text, and diagnostics without adding normalization, case
folding, locale tailoring, grapheme segmentation, collapse, replacement, composition, field/list
trimming, Application IR, Step-8 control flow, effects, or another backend.

Terminal proof passed with cached Go 1.25.13 and local/offline module inputs: the complete PipeLang
compiler, Core, evaluator, and backend suite; the exact 45-source compatibility suite; affected
application PipeLang tests and materializer/package-compile/`src/cmd` compile seams; editor
grammar/completion/snippet assertions; `go vet`; Windows/amd64 compile-only proof; `gofmt`; `git
diff --check`; frozen inventory digest/count; unchanged dependency files; and Core-only
evaluator/backend import-boundary checks. Application and CLI validation used only a temporary
`/tmp` modfile and the local cached module proxy selecting cached `x/sys v0.46.0`; no checkout
dependency, generated store, network, runtime, Docker, VM, credential, cleanup, commit, push,
publication, or other external state changed.

### Completed step 7ab exact five-field joined case-folded filtering (2026-08-19)

The founder selected explicit `v0.27.0` stable-order filtering of one primitive-record list by the
joined values of exactly five distinct public string fields. The sole source form takes direct
`List<R>` and `string` parameters, returns the same `List<R>`, and uses
`filter_joined_contains_casefolded(values, R.Field1, R.Field2, R.Field3, R.Field4, R.Field5,
query)` as its complete body. TASK-020's first read-only Docker-observability consumer fixes the
five selectors as `ContainerRow.Name`, `.State`, `.Image`, `.Ports`, and `.Created`.

The evaluator and Core-only Go backend validate the complete query and list, every record, and all
selected and unselected fields before filtering. They join selected strings in source order with
one U+0020 SPACE, trim the query with the pinned Unicode 17.0.0 `White_Space` contract, and apply
the pinned Unicode 17.0.0 full-default C/F case-folded contiguous containment contract. An empty
trimmed query retains all rows; stable input order and duplicates are preserved; no match produces
canonical non-nil empty storage; and results use fresh copied list and record storage.

Existing semantic identities and callable identity remain unchanged; `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry the explicit
`list_filter_joined_contains_case_folded_text` node with five ordered field identities, names, and
declaration positions. `v0.1.0` through `v0.26.0` reject the source, HIR, and executable Core form
without implicit migration. Arbitrary selector counts, runtime field-selector values, predicates,
regex, normalization, locale tailoring, sorting, nested/general composition, Application IR,
Step-8 behavior, effects, and additional backends remain excluded.

The synchronized source fixture and tests cover semantic projection, AST spans and selector order,
HIR/Core field identity and positions, cross-field and full-default Unicode matching, Unicode query
trimming, empty-query and empty-result behavior, strict whole-list UTF-8 and nil-list rejection,
stable copied results, malformed selector count/identity rejection, explicit v0.26 migration
rejection, deterministic Go generation, and generated-code execution.

Terminal proof passed with cached Go 1.25.0 and local/offline module inputs: the uncached complete
PipeLang compiler, Core, evaluator, and backend suite; the frozen exact 45-source compatibility
lane and inventory digest; affected application PipeLang tests and the `src/cmd` compile seam;
editor grammar/completion/snippet assertions; `go vet`; Windows/amd64 compile-only proof; `gofmt`;
`git diff --check`; unchanged dependency files; and Core-only evaluator/backend import boundaries.
Application and CLI validation used only a temporary `/tmp` modfile and isolated module cache backed
by the local cached module proxy selecting cached `x/sys v0.46.0`; no checkout dependency,
generated store, network, runtime, Docker, VM, credential, cleanup, commit, push, publication, or
other external state changed.

### Completed step 7ac exact stable ordinal record-list sorting (2026-08-19)

The founder selected explicit `v0.28.0` stable ascending sorting of one primitive-record list by
one selected public string field. The sole source form takes one direct `List<R>` parameter,
returns the same `List<R>`, and uses `sort_by_ordinal(values, R.Field)` as its complete body.

The evaluator and Core-only Go backend validate the complete list, every record, and all selected
and unselected fields before sorting. They use the existing `v0.8.0` ordinal Unicode
scalar-sequence order with no normalization, case folding, locale collation, or host-runtime
ordering. Equal keys preserve input order. Empty input produces canonical non-nil empty storage;
all results use fresh copied list and record storage.

Existing semantic identities and callable identity remain unchanged; `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry the explicit
`list_sort_by_ordinal_text` node with the selected field identity, name, and declaration position.
`v0.1.0` through `v0.27.0` reject the source, HIR, and executable Core form without implicit
migration. Descending or direction arguments, multi-key sorting, arbitrary comparers or predicates,
numeric/record sorting, mutation, general indexing, nested/general composition, Application IR,
Step-8 behavior, effects, and additional backends remain excluded.

The synchronized source fixture and tests cover semantic projection, AST and source spans,
HIR/Core field identity and position, ascending ordinal and non-normalized ordering, equal-key
stability, strict whole-list UTF-8 and nil-list rejection, canonical empty and copied results,
malformed selector identity/position rejection, explicit v0.27 migration rejection, deterministic
Go generation, generated-code execution, and preservation of the complete v0.27 joined-filter
contract.

Terminal proof passed with cached Go 1.25.13 and local/offline module inputs: the uncached complete
PipeLang compiler, Core, evaluator, and backend suite; the exact 45-source compatibility suite and
frozen inventory digest; affected application PipeLang tests and the `src/cmd` compile seam; editor
grammar/completion/snippet assertions; `go vet`; Windows/amd64 compile-only proof; `gofmt`; `git
diff --check`; unchanged dependency files; and Core-only evaluator/backend import-boundary checks.
Application and CLI validation used only a temporary `/tmp` modfile and the local cached module
proxy selecting cached `x/sys v0.46.0`; no checkout dependency, generated store, network, runtime,
Docker, VM, credential, cleanup, push, publication, or other external state changed.

### Completed step 7ad variable-count joined case-folded filtering (2026-08-19)

The founder selected explicit `v0.29.0` variable-count joined case-folded filtering of one
primitive-record list by two or more distinct public string fields. The source form keeps
`filter_joined_contains_casefolded(values, R.Field1, R.Field2, ..., query)` as its complete body,
with direct `List<R>` and `string` parameters and the same `List<R>` return type. Selector count is
bounded by the selected record's declared public string fields. `v0.27.0` and `v0.28.0` retain
their exact five-selector contract without implicit migration.

Existing AST, typed HIR, and target-neutral Core joined-filter nodes now carry the complete ordered
selector slice; no semantic or compiler schema identity changed. The evaluator and Core-only Go
backend preserve full input validation, source-order joining with U+0020 SPACE, pinned Unicode
17.0.0 query trimming and full-default case-folded containment, stable result order, canonical
non-nil empty output, and fresh copied list and record storage. Field identities, names, and
declaration positions remain explicit through HIR and Core.

Zero- or one-selector forms, duplicate/private/non-string/cross-record selectors, runtime selector
values, normalization, locale tailoring, regex, arbitrary predicates, sorting changes, nested or
general composition, Application IR, Step-8 behavior, effects, and additional backends remain
excluded.

The synchronized source fixture and tests cover semantic projection, parser spans and exclusions,
three-field ordered HIR/Core identity, cross-field and pinned Unicode matching, Unicode query
trimming, empty-query and empty-result behavior, strict whole-list UTF-8 and nil-list rejection,
stable copied results, malformed HIR/Core rejection, explicit `v0.28.0` migration rejection,
deterministic Go generation, generated-code execution, and preservation of `v0.28.0` ordinal
sorting and the accepted five-field joined filter under `v0.29.0`.

Terminal proof passed with cached Go 1.25.13 and local/offline module inputs: the uncached complete
PipeLang compiler, Core, evaluator, backend, and exact 45-source compatibility suites; affected
application PipeLang and package-compile tests; the `src/cmd` compile seam; editor assertions; `go
vet`; Windows/amd64 compile-only proof; `gofmt`; `git diff --check`; unchanged dependency files;
the frozen inventory digest; and Core-only evaluator/backend import boundaries. Application and CLI
validation used only a temporary `/tmp` modfile selecting cached `x/sys v0.46.0`; no checkout
dependency, generated store, network, runtime, Docker, VM, credential, cleanup, commit, push,
publication, or other external state changed.

### Completed step 7ae variable-count multi-key ordinal record-list sorting (2026-08-19)

The founder selected explicit `v0.30.0` stable ascending lexicographic ordinal sorting of one
primitive-record list by two or more distinct public string fields. The source form keeps
`sort_by_ordinal(values, R.Field1, R.Field2, ...)` as its complete body, with one direct `List<R>`
parameter and the same `List<R>` return type. Selector count is bounded by the selected record's
declared public string fields. `v0.28.0` and `v0.29.0` retain their exact one-selector contract
without implicit migration; one-selector source under `v0.30.0` also preserves that exact behavior
and projection.

The evaluator and Core-only Go backend validate the complete list, every record, and every selected
and unselected field before sorting. Selectors are compared in source order using the existing
ordinal Unicode scalar-sequence rule, and the first unequal field decides the result. Rows equal
across all selected fields preserve input order. Empty input produces canonical non-nil empty
storage, and every result uses fresh copied list and record storage.

Existing semantic identities and callable identity remain unchanged; `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. One-key source continues to lower through
`list_sort_by_ordinal_text`; typed HIR and target-neutral Core use the explicit
`list_sort_by_ordinal_texts` node only for two or more selectors and carry the complete ordered
field identities, names, and declaration positions.

Zero-selector and duplicate/private/non-string/cross-record selectors, runtime selector values,
descending or per-key direction, arbitrary comparers or predicates, numeric/record sorting,
normalization, case folding, locale tailoring, mutation, nested or general composition,
Application IR, Step-8 behavior, effects, and additional backends remain excluded.

The synchronized source fixture and typed HIR, Core, and Go goldens cover semantic projection,
parser spans and selector order, ordered HIR/Core field identities and declaration positions,
lexicographic ordinal sorting and equal-key stability, complete strict-UTF-8 and nil-list rejection,
canonical non-nil empty and copied results, malformed HIR/Core rejection, explicit `v0.29.0`
migration rejection, deterministic Go generation, generated-code execution, and preservation of
the variable joined-filter and one-key sort contracts under `v0.30.0`.

Terminal proof passed with cached Go 1.25.0 and local/offline module inputs: the complete PipeLang
compiler, Core, evaluator, and backend suite; the frozen exact 45-source compatibility lane and
inventory digest; affected application PipeLang/materialization tests; the `src/cmd` compile seam;
editor assertions; PipeLang `go vet`; Windows/amd64 compile-only proof; `gofmt`; `git diff --check`;
unchanged dependency files; unchanged protected ignored bytes; and Core-only evaluator/backend
import boundaries. Application and CLI validation used only a temporary `/tmp` modfile selecting
cached `x/sys v0.46.0`; no checkout dependency, generated store, network, runtime, Docker, VM,
credential, cleanup, commit, push, publication, or other external state changed.
