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

