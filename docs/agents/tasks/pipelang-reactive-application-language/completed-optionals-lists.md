### Completed step 7n primitive Optional contract (2026-08-18)

The founder selected explicit `v0.13.0` primitive optional construction, identity transport, and
presence inspection. `Optional<T>` has the fixed semantic identity `pipelang:optional`, projected
as one applied type argument, and `T` is exactly one of `string`, `int`, `float`, or `bool`. The
complete source surface is these four direct expression-bodied class-method shapes:

```pipelang
public Class Root {
    public Optional<string> Present(string value) => some(value);
    public Optional<string> Absent() => none<string>();
    public Optional<string> Forward(Optional<string> value) => value;
    public bool HasValue(Optional<string> value) => has_value(value);
}
```

Typed HIR and target-neutral Core carry explicit `optional_some`, `optional_none`, and
`optional_has_value` nodes plus identity transport of the tagged value. Core validation admits
only the selected complete direct method shapes. Core evaluation and deterministic Core-only Go
agree on present/absent construction, transport, and inspection, validate strict UTF-8 present
string payloads, and reject malformed or zero/nil representations rather than treating them as
absence. The semantic projection schema remains `pipelang.semantic.v1`.

`PL3006` reports invalid payload types, placements, or signatures; `PL3009` reports literal,
computed, nested, mismatched, or otherwise non-direct bodies. Malformed typed HIR and Core remain
`PL3026` and `PL3027`. `v0.1.0` through `v0.12.0` reject the type and intrinsic expressions without
implicit migration. Every frozen arithmetic Result, ordinal Unicode text, and primitive-record
contract remains available under `v0.13.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/optional-value.pipe`, with
synchronized typed HIR, Core, and Go goldens. Extraction or unwrapping, optional equality,
defaults, record fields, nesting, chaining, fallback, propagation, matching, mutation, Result
integration, unions, deterministic collections, Step-8 control flow, effects, application
behavior, and additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including source-derived Optional HIR/Core/Go
  goldens, semantic identity/projection, all four primitive payloads, Core/generated-Go agreement,
  malformed HIR/Core and zero/nil rejection, invalid UTF-8, excluded forms, explicit migration,
  and every frozen earlier language slice under `v0.13.0`;
- focused PipeLang check/compile/invoke, catalog, materialize, application/internal package-compile,
  domain, and `src/cmd` checks using only the existing temporary modfile pinned to cached
  `x/sys v0.46.0` where required;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code
  grammar/completion/snippet/diagnostic validation, and Windows compile-only proof across the
  affected packages; and
- `gofmt`, `git diff --check`, exact inventory, dependency/generated-state absence, branch/stash,
  TASK-020, and protected ignored-byte proof.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7o primitive Optional defaulting contract (2026-08-18)

The founder selected explicit `v0.14.0` primitive Optional defaulting. The accepted public method
has exactly two parameters and one direct expression body:

```pipelang
public Class Root {
    public string ValueOr(Optional<string> value, string fallback) =>
        value_or(value, fallback);
}
```

`value_or(Optional<T>, T) -> T` admits exactly `string`, `int`, `float`, or `bool` for concrete
`T`. Parameter 0 is the Optional operand, parameter 1 is the fallback, and the body directly
references those parameters in that order. The form reuses the existing `pipelang:optional`
semantic identity and leaves `pipelang.semantic.v1` unchanged. Typed HIR and target-neutral Core
carry an explicit `optional_value_or` node; the deterministic evaluator and Core-only Go backend
canonically validate both arguments before selecting the present payload or fallback. Strict UTF-8
therefore applies even to an unselected string fallback. Binary64 values preserve NaN and signed
zero payload behavior without adding equality or ordering semantics.

`PL3006` reports invalid payload types, placements, or signatures; `PL3009` reports literals,
computed operands, reordered parameters, nested expressions, or other non-direct bodies.
Malformed typed HIR and Core remain `PL3026` and `PL3027`. `v0.1.0` through `v0.13.0` reject
`value_or` without implicit migration. Every frozen arithmetic Result, ordinal Unicode text,
primitive-record, and `v0.13.0` Optional construction/transport/inspection contract remains
available under `v0.14.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/optional-value-or.pipe`, with
synchronized typed HIR, Core, and Go goldens. Optional extraction beyond this exact defaulting
form, equality, implicit defaults, record fields, nesting, chaining, propagation, matching,
mutation, conversion, fallibility, Result composition, unions, deterministic collections,
hashing, ordering, Step-8 control flow, effects, application behavior, and additional backends
remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including source-derived Optional defaulting
  HIR/Core/Go goldens, semantic identity/projection, all four primitive payloads,
  Core/generated-Go agreement, canonical validation of both arguments, invalid UTF-8 in an
  unselected fallback, NaN and signed-zero transport, malformed HIR/Core rejection, excluded
  forms, explicit migration, and every frozen earlier language slice under `v0.14.0`;
- focused PipeLang check/compile/invoke, catalog, materialize, application/internal
  package-compile, domain, and `src/cmd` checks using only the existing temporary modfile pinned to
  cached `x/sys v0.46.0` where required;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code
  grammar/completion/snippet/diagnostic validation, and Windows compile-only proof across the
  affected packages; and
- `gofmt`, `git diff --check`, exact inventory, dependency/generated-state absence, branch/stash,
  TASK-020, and protected ignored-byte proof.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7p deterministic primitive-record list contract (2026-08-18)

The founder selected explicit `v0.15.0` deterministic primitive-record list values. The accepted
public surface contains only these direct method shapes for one existing public primitive record
`R`:

```pipelang
public List<ContainerRow> EmptyRows() => empty_list<ContainerRow>();
public List<ContainerRow> OneRow(ContainerRow value) => list(value);
public List<ContainerRow> ForwardRows(List<ContainerRow> values) => values;
```

`List<R>` is an immutable, ordered, non-null value whose elements retain the existing record
identity and declaration-ordered primitive field values. It has the fixed semantic identity
`pipelang:list` with the record identity as its sole structured type argument; the
`pipelang.semantic.v1` schema remains unchanged. `empty_list<R>()` constructs the canonical empty
value, `list(value)` constructs exactly one element from the sole `R` parameter, and direct
identity transport returns the sole identical `List<R>` parameter. Typed HIR and target-neutral
Core use explicit `list`, `list_empty`, and `list_singleton` forms. The deterministic evaluator and
Core-only Go backend validate every record element, including strict UTF-8 for every string field,
and copy list plus nested record storage at construction and transport boundaries so caller-owned
storage cannot mutate the result. A nil list representation is invalid.

`PL3006` reports invalid element types, list placement, nesting, fields, or signatures; `PL3009`
reports literals, computed elements, non-direct bodies, or unsupported list operations. Malformed
typed HIR and Core remain `PL3026` and `PL3027`. `v0.1.0` through `v0.14.0` reject the new source
forms and executable representations without implicit migration, while every frozen earlier
language contract remains available under `v0.15.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/record-list.pipe`, with synchronized
typed HIR, Core, and Go goldens. This slice gives TASK-020's first accepted read-only Docker
observability consumer deterministic empty, singleton, and pass-through row collections without
introducing application behavior. Primitive, nested, Optional, or Result list elements; list
fields; multi-element source construction; append, indexing, count, iteration, filtering, sorting,
equality, hashing, maps, sets, builders, mutation, Application IR, Step-8 control flow, effects, and
additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang package suite, including source-derived list HIR/Core/Go goldens, fixed
  semantic identity/projection, parser spans, canonical non-null empty and singleton values,
  list/record storage isolation, strict UTF-8 validation, malformed HIR/Core rejection, excluded
  forms, explicit `v0.1.0` through `v0.14.0` migration rejection, and selected frozen `v0.14.0`
  contracts under `v0.15.0`;
- focused PipeLang check/compile/invoke, catalog, materialize, application/internal packagecompile,
  domain, and `src/cmd` checks using only the existing temporary modfile pinned to cached
  `x/sys v0.46.0` where required;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code
  grammar/completion/snippet/diagnostic validation, and Windows compile-only proof across the
  affected packages; and
- `gofmt`, `git diff --check`, exact 45-source inventory, dependency/generated-state absence,
  branch/stash, TASK-020, and protected ignored-byte proof.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7q primitive-record list cardinality contract (2026-08-18)

The founder selected explicit `v0.16.0` primitive-record list cardinality. The accepted public
surface contains only this direct method shape for one existing public primitive record `R`:

```pipelang
public int CountRows(List<ContainerRow> values) => count(values);
```

`count(List<R>) -> int` requires exactly one `List<R>` parameter as the complete direct operand and
returns its nonnegative signed-64-bit cardinality. The input remains an immutable, ordered,
non-null list whose elements retain their existing record identities and declaration-ordered
primitive fields. The evaluator and backend validate the complete list, every record element, and
strict UTF-8 string field before observing its length; malformed or nil list representations fail
closed.

The expression reuses the existing `pipelang:list` and record identities. Callable identity remains
the structured `(List<R>) -> int` signature, and `pipelang.compiler.v1` plus
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry an explicit
`list_count` node over the sole direct parameter. Core evaluation returns the validated cardinality,
and the Core-only Go backend validates the parameter before deterministic `int64(len(...))`
emission without inferring iteration or target collection semantics.

`PL3006` reports invalid element, placement, or signature types; `PL3009` reports computed operands,
extra expressions, or otherwise non-direct bodies. Malformed typed HIR and Core remain `PL3026` and
`PL3027`. `v0.1.0` through `v0.15.0` reject the source and executable form without implicit
migration, while every frozen earlier contract remains available under `v0.16.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/record-list-count.pipe`, with
synchronized typed HIR, Core, and Go goldens. This slice directly supplies TASK-020's frozen
container, network, and volume count summaries. Multi-element construction, append, indexing,
iteration, filtering, sorting, list equality/hash, maps, sets, builders, mutation, Application IR,
Step-8 control flow, effects, and additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang and exact 45-source compatibility suites, including parser spans,
  semantic identity/projection, source-derived HIR/Core/Go cardinality goldens, empty and multi-row
  evaluator agreement, per-element/UTF-8 validation, nil/malformed HIR/Core rejection, explicit
  `v0.1.0` through `v0.15.0` migration rejection, and preserved `v0.15.0` list behavior;
- focused application PipeLang check/compile/invoke, catalog, materialize, package-compile, domain,
  and `src/cmd` checks through only a temporary `/tmp` modfile pinned to cached `x/sys v0.46.0`;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code
  grammar/completion/snippet/diagnostic validation, and Windows compile-only proof across affected
  packages; and
- `gofmt`, `git diff --check`, exact 45-source inventory, dependency/generated-state absence, and
  engine/package-boundary review.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7r primitive-record list append contract (2026-08-18)

The founder selected explicit `v0.17.0` immutable primitive-record list append. The accepted public
surface contains only this direct method shape for one existing public primitive record `R`:

```pipelang
public List<ContainerRow> AppendRow(
  List<ContainerRow> values,
  ContainerRow value
) => append(values, value);
```

`append(List<R>, R) -> List<R>` requires exactly the direct list parameter followed by the direct
matching record parameter as the complete expression. It returns a fresh immutable, ordered,
non-null list containing the validated input elements in their original order followed by the
validated record. The evaluator and backend validate the complete input list, every record
element, the appended record, and strict UTF-8 for every string field before producing the result.
They copy list and nested record storage, reject nil representations, and fail closed if the list
cannot grow within signed-64-bit cardinality.

The expression reuses the fixed `pipelang:list` identity and existing record identity; callable
identity remains the structured `(List<R>, R) -> List<R>` signature. `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry an explicit
`list_append` node over the two direct parameters. The evaluator implements deterministic append,
and the Core-only Go backend emits a bounded validation-and-copy helper only for programs whose
Core contains append.

`PL3006` reports invalid element, placement, or signature types; `PL3009` reports constructed,
computed, reordered, additional, or otherwise non-direct operands and bodies. Malformed typed HIR
and Core remain `PL3026` and `PL3027`. `v0.1.0` through `v0.16.0` reject the source and executable
form without implicit migration, while every frozen earlier contract remains available under
`v0.17.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/record-list-append.pipe`, with
synchronized typed HIR, Core, and Go goldens. This slice gives TASK-020's first read-only Docker
observability consumer deterministic multi-row snapshot growth without introducing iteration or
application behavior. Multi-element literals, prepend, insert, remove, update, concatenation,
indexing, iteration, filtering, sorting, list equality/hash, maps, sets, builders, mutation,
Application IR, Step-8 control flow, effects, and additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang and exact 45-source compatibility suites, including parser spans,
  semantic identity/projection, source-derived HIR/Core/Go append goldens, empty and multi-row
  evaluator order, caller-storage isolation, full element/appended-record UTF-8 validation,
  nil/malformed HIR/Core rejection, explicit `v0.1.0` through `v0.16.0` migration rejection, and
  preserved `v0.16.0` count behavior;
- focused application PipeLang check/compile/invoke, catalog, materialize, package-compile, domain,
  and `src/cmd` checks through only a temporary `/tmp` modfile mapped to cached `x/sys v0.46.0`;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code
  grammar/completion/snippet/diagnostic validation, and Windows compile-only proof across affected
  packages; and
- `gofmt`, `git diff --check`, exact 45-source inventory, dependency/generated-state absence, and
  engine/package-boundary review.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

### Completed step 7s primitive-record Optional contract (2026-08-18)

The founder selected explicit `v0.18.0` primitive-record Optional values. The accepted public
surface contains only these direct method shapes for one existing public primitive record `R`:

```pipelang
public Optional<ContainerRow> PresentRow(ContainerRow value) => some(value);
public Optional<ContainerRow> AbsentRow() => none<ContainerRow>();
public Optional<ContainerRow> ForwardRow(Optional<ContainerRow> value) => value;
public bool HasRow(Optional<ContainerRow> value) => has_value(value);
public ContainerRow RowOr(Optional<ContainerRow> value, ContainerRow fallback) =>
    value_or(value, fallback);
```

`Optional<R>` reuses the fixed `pipelang:optional` identity with the existing record identity as
its sole structured argument. Callable identity preserves the exact parameter and return shapes;
`pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral
Core reuse explicit `optional_some`, `optional_none`, `optional_has_value`, and `optional_value_or`
nodes with a structured record payload. The evaluator and Core-only Go backend validate canonical
non-null tagged values, every primitive record field, and strict UTF-8 string fields. `value_or`
validates both its Optional and fallback before selection, and evaluator results copy record
storage so caller-owned values cannot mutate a transported or defaulted result.

`PL3006` reports invalid payload, placement, or signature types; `PL3009` reports constructed,
computed, reordered, additional, or otherwise non-direct operands and bodies. Malformed typed HIR
and Core remain `PL3026` and `PL3027`. `v0.1.0` through `v0.17.0` reject the source and executable
form without implicit migration, while every frozen earlier contract remains available under
`v0.18.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/optional-record.pipe`, with
synchronized typed HIR, Core, and Go goldens. This slice gives TASK-020's first read-only Docker
observability consumer a direct typed absent/present row boundary and deterministic fallback without
introducing application behavior. Optional fields, constructed record payloads, nesting, chaining,
equality, hashing, ordering, implicit defaults, extraction beyond `value_or`, Optional/list/Result
payloads, record nesting, Application IR, Step-8 control flow, effects, and additional backends
remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- the complete PipeLang and exact 45-source compatibility suites, including semantic identity and
  projection, source-derived HIR/Core/Go Optional-record goldens, present/absent/identity/presence/
  fallback evaluator agreement, caller-storage isolation, selected and unselected strict UTF-8
  validation, nil/malformed HIR/Core rejection, deterministic exclusion diagnostics, explicit
  `v0.1.0` through `v0.17.0` migration rejection, and preserved primitive Optional and `v0.17.0`
  append behavior;
- focused application PipeLang check/compile/invoke, catalog, materialize, package-compile, domain,
  and `src/cmd` checks through only a temporary `/tmp` modfile mapped to cached `x/sys v0.46.0`;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code
  grammar/completion/snippet/diagnostic validation, and Windows compile-only proof across affected
  packages; and
- `gofmt`, `git diff --check`, exact 45-source inventory, dependency/generated-state absence, and
  engine/package-boundary review.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

