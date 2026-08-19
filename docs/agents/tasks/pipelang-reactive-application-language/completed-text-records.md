### Completed step 7i ordinal Unicode text ordering contract (2026-08-17)

The founder selected the recommended explicit `v0.8.0` text slice. It preserves every earlier
numeric and arithmetic Result contract and admits exactly one expression-bodied class method shape
returning `bool`, with exactly two `string` parameters and one of `<`, `<=`, `>`, or `>=` comparing
those parameters in declared order as the complete body:

```pipelang
bool Before(string left, string right) => left < right;
```

PipeLang strings remain immutable preserved Unicode scalar sequences. Equality and ordering are
ordinal: comparison is lexicographic by scalar value and never normalizes, case-folds, applies a
culture/locale, or delegates meaning to target collation. Existing string concatenation, equality,
and inequality retain their prior-version source behavior and gain the same deterministic Core
evaluator conformance. Invalid source UTF-8 remains `PL0001`; malformed host-provided string values
fail the Core/backend infrastructure boundary rather than becoming language results.

The slice adds no semantic type identity: callable identity and `pipelang.semantic.v1` continue to
carry the existing primitive `string` parameters and `bool` return. Typed HIR and target-neutral
Core preserve the selected relational operator. `coreeval` validates UTF-8 and compares scalar
sequences explicitly. The Core-only Go backend emits deterministic UTF-8/scalar helpers rather than
using host locale or collation, and malformed host text fails with a fixed infrastructure panic.

`v0.1.0` through `v0.7.0` continue to reject string relational ordering with `PL3009`. `v0.8.0`
also rejects extra or reordered parameters, literals in place of the declared parameters, mixed
operands, wrong return types, and nested ordering. `PL3028` remains exclusively the established
checked-arithmetic diagnostic, and v0.8.0 preserves direct add/subtract/multiply/negate/divide and
v0.7.0 Result transport without changing `pipelang:result`, `pipelang:arithmetic.error`, `overflow`,
`division_by_zero`, `pipelang.compiler.v1`, or `pipelang.semantic.v1`.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including parser/type/identity/projection,
  source-derived HIR/Core/Go text-order goldens, Core/generated-Go scalar-order agreement, invalid
  UTF-8 boundaries, explicit migration rejection, prior text-operation conformance, and frozen
  arithmetic Result behavior;
- affected PipeLang CLI/check/compile/invoke, catalog, materialize, package-compile, internal, and
  `src/cmd` checks through only the existing temporary modfile pinned to cached `x/sys v0.46.0`;
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages; and
- VS Code grammar/diagnostic validation plus durable `v0.8.0` editor guidance, `gofmt`,
  `git diff --check`, frozen inventory, dependency, generated-state, branch/stash, and protected
  ignored-byte proof.

The minimal pure source fixture is `src/lib/pipelang/testdata/text-order.pipe`, with synchronized
typed HIR, Core, and Go goldens. No hash algorithm/capability, normalization/case/grapheme API,
structural value equality, optional, general Result handling, record, union, collection, broader
expression, dependency, generated store, runtime, credential, external state, cleanup, commit, or
publication changed.

### Completed step 7j primitive immutable record contract (2026-08-18)

The founder selected the recommended explicit `v0.9.0` record slice. It preserves every earlier
text and arithmetic Result contract and admits public, nonempty record declarations whose public
fields have no defaults and use only `string`, `int`, `float`, or `bool`. Executable use is exactly
one class method with one parameter, an identical record return type, and that parameter identifier
as the complete expression-bodied method body:

```pipelang
public Record Row {
    public string Id;
    public int Count;
    public float Ratio;
    public bool Ready;
}
public Class Root {
    public Row Forward(Row value) => value;
}
```

`Record` is contextual only under explicit `v0.9.0`; earlier contracts preserve their grammar and
may continue to use `Record` as an identifier. Records and their fields receive stable semantic
identities in the existing single symbol table. `pipelang.semantic.v1` projects the record and its
deterministic identity-ordered member surface. Typed HIR and target-neutral Core carry the
declaration-ordered identified schema and the direct
parameter reference. Core evaluation validates the exact schema and primitive field values,
preserves strict UTF-8 for `string`, and clones its field vector so immutable value transport cannot
leak mutable aliasing. The Core-only Go backend emits one deterministic named struct, validates text
fields, and returns that struct value directly.

Negative diagnostics retain empty/private records, annotations, implemented interfaces, methods,
private/nonprimitive/defaulted fields, class/interface record fields, extra or mismatched transport
parameters, different bodies, construction, field access, mutation, equality, hashing, ordering,
matching, nesting, Result integration, optionals, unions, and collections as excluded forms.
`v0.1.0` through `v0.8.0` reject the new declaration without implicit migration. `v0.9.0` preserves
direct add/subtract/multiply/negate/divide, arithmetic Result transport, and ordinal string ordering
without changing `pipelang:result`, `pipelang:arithmetic.error`, `overflow`, `division_by_zero`,
`pipelang.compiler.v1`, or `pipelang.semantic.v1`.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including parser/type/identity/projection,
  source-derived HIR/Core/Go record goldens, Core/generated-Go agreement, malformed schema and UTF-8
  boundaries, immutable Core value transport, explicit migration rejection, excluded forms, and
  every frozen arithmetic Result/text behavior under `v0.9.0`;
- affected catalog, PipeLang check/compile/invoke, materialize, package-compile, internal, and
  `src/cmd` consumer checks through only the existing temporary modfile pinned to cached
  `x/sys v0.46.0`;
- `go vet` over PipeLang, compatibility, and affected application/internal/CLI packages, plus
  Windows compile-only proof for those packages; and
- VS Code grammar/completion/snippet/diagnostic validation, `gofmt`, `git diff --check`, engine
  boundary, frozen inventory, dependency/generated-state, branch/stash, and protected ignored-byte
  proof.

The minimal pure source fixture is `src/lib/pipelang/testdata/record-transport.pipe`, with
synchronized typed HIR, Core, and Go goldens. No constructor/member-access/operator/control-flow
surface, dependency, generated store, runtime, credential, external state, cleanup, commit, or
publication changed.

### Completed step 7k one-hop primitive-record field projection contract (2026-08-18)

The founder selected explicit `v0.10.0` one-hop read-only record field projection. It preserves
every earlier arithmetic Result, ordinal text, and primitive immutable record identity-transport
contract. The only new source form is one class method with exactly one existing primitive record
parameter, the selected field's exact primitive return type, and direct `parameter.Field` as the
complete body:

```pipelang
public Record Row {
    public string Id;
}
public Class Root {
    public string IdOf(Row value) => value.Id;
}
```

The selected field reuses its `v0.9.0` semantic identity and declared record position; no new type
identity or `pipelang.semantic.v1` schema was added. Typed HIR and target-neutral Core carry the
receiver's identified record schema, field identity/name/position, and exact primitive result type.
Core rejects schema, identity, position, name, or result-type drift before evaluation. `coreeval`
validates the complete record and returns the declared field value. The Core-only Go backend
validates every record parameter, then emits deterministic direct named-field access. Existing
strict UTF-8 validation therefore applies before projecting a `string` field in both executable
paths.

`PL3004` reports unknown, inaccessible, chained, or non-record member selection; `PL3006` retains
invalid record placement and signature rejection; `PL3009` reports a wrong return, extra expression,
or non-direct body. Malformed typed HIR and Core remain `PL3026` and `PL3027` at their compiler
boundaries. `v0.1.0` through `v0.9.0` reject member access without implicit migration. Construction,
mutation, chaining, calls, indexing, record equality/hash/order, optionals, Result expansion,
unions, deterministic collections, Step-8 control flow, effects, and application behavior remain
excluded.

The minimal pure source fixture is
`src/lib/pipelang/testdata/record-field-projection.pipe`, with synchronized typed HIR, Core, and Go
goldens. The focused proof covers all four primitive field types, semantic identity reuse, exact
projection shape, Core/evaluator/backend agreement, deterministic generation, malformed IR,
invalid UTF-8, excluded forms, explicit migration, and preservation of every frozen earlier
language slice.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including the source-derived HIR/Core/Go
  projection goldens, Core/generated-Go agreement, malformed HIR/Core rejection, explicit migration,
  excluded forms, and all frozen arithmetic Result/text/record behavior under `v0.10.0`;
- focused PipeLang check/compile/invoke, catalog, materialize, application/internal package-compile,
  domain, and `src/cmd` checks using only the existing temporary modfile pinned to cached
  `x/sys v0.46.0` where required;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code grammar/snippet/diagnostic
  validation, and Windows compile-only proof across the affected packages; and
- `gofmt`, `git diff --check`, exact inventory, dependency/generated-state absence, branch/stash,
  TASK-020, and protected ignored-byte proof.

The broad `src/lib/application` run again reached only its unrelated host-sensitive failures: the
local-listener test is denied by this sandbox and the established inherited-workdir fixture names
`/path/to/your/project`. Every affected focused application test and every application/internal
package passed. No dependency, generated store, runtime, external state, cleanup, commit, push, or
publication changed.

### Completed step 7l primitive-record construction contract (2026-08-18)

The founder selected the smallest independently useful record-value slice: explicit `v0.11.0`
construction of one existing public primitive record. The exact source surface is one
expression-bodied class method returning that record, with exactly one primitive parameter per
field in declaration order and with the exact field types:

```pipelang
public Record Row {
    public string Id;
    public bool Healthy;
}
public Class Rows {
    public Row Create(string id, bool healthy) =>
        new Row { Id = id, Healthy = healthy };
}
```

The initializer assigns every field exactly once, in declaration order, from the corresponding
direct parameter. Omitted, extra, duplicate, reordered, defaulted, nested, or computed field
values remain excluded. The expression reuses the existing record and field semantic identities
and the enclosing callable identity; it creates no constructor identity and leaves
`pipelang.semantic.v1` unchanged. Typed HIR and target-neutral Core carry `record_construct` with
the record identity and ordered field identity/name/position/value projection. Core evaluation and
the Core-only Go backend validate the complete immutable value and agree on all four primitive
field types, including strict UTF-8 strings.

`PL3004` reports unknown fields, `PL3005` reports duplicates, `PL3006` reports missing, extra,
reordered, type, return, or signature mismatches, and `PL3009` reports non-direct bodies or values.
Malformed typed HIR and Core remain `PL3026` and `PL3027`. `v0.1.0` through `v0.10.0` reject the
new expression without implicit migration. All frozen checked arithmetic Result, Unicode text,
record transport, and one-hop field-projection contracts remain available under `v0.11.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/record-construction.pipe`, with
synchronized typed HIR, Core, and Go goldens. Production construction syntax, semantic projection,
Core evaluation, deterministic Go emission, diagnostics, editor grammar/completion/snippet support,
and public documentation are implemented as this single slice. No application feature, nested
record value, equality/hash/order, optional, general Result operation, union, collection, Step-8
control flow, effect, backend, or generated-store behavior is included.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including source-derived HIR/Core/Go
  construction goldens, semantic identity and projection checks, Core/generated-Go agreement,
  malformed HIR/Core rejection, invalid UTF-8, excluded forms, explicit migration, and every frozen
  earlier language slice under `v0.11.0`;
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

### Completed step 7m primitive-record structural equality contract (2026-08-18)

The founder selected explicit `v0.12.0` structural equality for one existing public primitive
record. The exact source surface is one expression-bodied class method returning `bool`, with
exactly two parameters of the same record type and the two direct parameter references in declared
order as the complete body:

```pipelang
public Record Row {
    public string Id;
    public int Count;
}
public Class Rows {
    public bool Same(Row left, Row right) => left == right;
    public bool Different(Row left, Row right) => left != right;
}
```

Equality compares fields structurally in declaration order. Strings retain exact ordinal Unicode
scalar-sequence equality, integers and booleans compare exactly, and binary64 fields use IEEE
equality: NaN is unequal, including to itself, while positive and negative zero are equal. `!=` is
the exact complement. The expression reuses the existing record, field, and callable semantic
identities; it adds no operator or type identity and leaves `pipelang.semantic.v1` unchanged. Typed
HIR and target-neutral Core carry the existing identified record operands and `eq` or `ne` binary
operator. Core validation admits only the selected direct two-parameter shape. Core evaluation and
the Core-only Go backend validate complete immutable values and agree on every primitive field
kind, strict UTF-8 rejection, signed zero, and NaN.

`PL3006` reports invalid record placement, parameter, or return signatures; `PL3009` reports
ordered, repeated, projected, constructed, nested, or otherwise non-direct bodies. Malformed typed
HIR and Core remain `PL3026` and `PL3027`. `v0.1.0` through `v0.11.0` reject record equality without
implicit migration. All frozen checked-arithmetic Result, Unicode text, record identity transport,
one-hop projection, and exact construction contracts remain available under `v0.12.0`.

The minimal pure source fixture is `src/lib/pipelang/testdata/record-equality.pipe`, with
synchronized typed HIR, Core, and Go goldens. Hashing, record ordering, nesting, general member
access or comparison, mutation, optionals, Result expansion, unions, deterministic collections,
Step-8 control flow, effects, application behavior, and additional backends remain excluded.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including source-derived HIR/Core/Go equality
  goldens, identity/projection stability, Core/generated-Go agreement, malformed HIR/Core
  rejection, invalid UTF-8, signed-zero and NaN behavior, excluded forms, explicit migration, and
  every frozen earlier language slice under `v0.12.0`;
- focused PipeLang check/compile/invoke, catalog, materialize, application/internal package-compile,
  and `src/cmd` checks using only the existing temporary modfile pinned to cached `x/sys v0.46.0`
  where required;
- `go vet`, Core-only backend/evaluator import-boundary checks, VS Code
  grammar/completion/snippet/diagnostic validation, and Windows compile-only proof across the
  affected packages; and
- `gofmt`, `git diff --check`, exact inventory, dependency/generated-state absence, branch/stash,
  TASK-020, and protected ignored-byte proof.

No dependency, generated store, runtime, external state, cleanup, commit, push, or publication
changed.

