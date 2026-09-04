# PipeLang (v0.0.0.1)

PipeLang is an optional typed authoring layer for DockPipe.

It does not replace YAML workflows. YAML remains first-class.

This page defines the frozen `v0.0.0.1` contract. The accepted future direction is a
general-purpose, deterministic, target-neutral managed language that can eventually compile its own
compiler. That direction is C#-familiar, not C#-compatible, and does not add pointers, manual memory,
unsafe APIs, hidden host access, or target-specific syntax. No future direction described here
changes existing source or artifact behavior without an explicit language-version migration.

Use PipeLang for:
- typed workflow/package configuration models
- defaults and documentation summaries close to the model
- launcher/editor metadata derived from those types

In `v0.0.0.1`, do not use PipeLang as a replacement for workflow execution logic. DockPipe still
runs normal workflow YAML.

## Scope in v0.0.0.1

Supported:
- Primitive types: `string`, `int`, `bool`, `float`
- `Interface` declarations (structural contracts)
- `Class` declarations with optional defaults
- Object/interface-typed fields inside other classes
- Generic list shapes such as `List<string>` and `List<IImageResource>`
- Split declarations across multiple `.pipe` files in the same module tree
- `Class : Interface` conformance checks
- Expression-bodied methods (`=>`) with static type-checking
- Deterministic artifact generation
- CLI-only method invocation

Not supported in this version:
- Side effects in methods
- Runtime/resolver execution through methods
- Hidden execution during compile
- General-purpose scripting/runtime behavior

These are version boundaries, not permanent limits on the language roadmap.

Reserved/plumbed but not fully implemented yet:
- `IComparable` for custom object comparison. The parser/type checker understands the contract boundary, but custom object comparisons should still fail clearly until the runtime semantics land.

## Authoring model

PipeLang is the **data model** layer.

Typical responsibilities:
- define the root workflow config type
- define nested objects such as `General`, `Storage`, or `Network`
- define list-valued fields such as `List<string>`
- define defaults on the implementing class
- keep XML summaries and field docs close to the type

Typical non-responsibilities:
- page layout
- section ordering
- launcher tab/group structure
- select-vs-create UX

Those presentation concerns live in workflow YAML `view:` metadata, not in the PipeLang type system.

## Workflow binding

Workflow YAML binds to PipeLang through top-level `types:`.

Example:

```yaml
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe
```

This means:
- the referenced `.pipe` file is the **entrypoint**
- DockPipe reads the **module tree** rooted beside that file
- sibling `.pipe` files in that module may contribute additional interfaces/classes
- the selected entry type becomes the **root model** for tooling/catalog/launcher use

The workflow still executes through normal YAML/env resolution. PipeLang only shapes the authored model and the generated metadata.

## Environment mapping

Leaf fields bind to environment variables through:
- explicit annotations such as `[EnvName = "DOCKPIPE_VM_DISK"]`, or
- inferred names when the model/catalog layer can derive them

Example:

```pipelang
public Interface IWindowsVmStorage
{
    [EnvName = "DOCKPIPE_VM_DISK"]
    public string Disk;
}
```

This keeps the strong typed model separate from the runtime’s existing env contract.

## Workflow `view:` relationship

PipeLang works together with workflow YAML `view:`.

- PipeLang defines **what the data is**
- workflow YAML `view:` defines **how a launcher may present it**

Example shape:

```yaml
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe

view:
  entry:
    type: choice
    field: General.BootSource
    options:
      - value: image
        pages: [image]
      - value: installer-iso
        pages: [install]
  pages:
    - id: image
      title: Existing Image
      sections:
        - id: media
          title: Existing Image
          fields:
            - Storage.Disk
    - id: install
      title: Install Media
      sections:
        - id: media
          title: Images And Media
          fields:
            - Storage.Cdrom
```

The field paths in `view:` reference the PipeLang root model recursively. Entry routing still binds back to the same model field; it is not a separate runtime state machine.

## Commands

Check a source set without evaluation or artifact emission:

```bash
dockpipe pipelang check --in workflows/mywf/model.pipe --format json
```

Compile artifacts:

```bash
dockpipe pipelang compile --in workflows/mywf/model.pipe --entry DefaultDeployConfig --out bin/.dockpipe/pipelang
```

Invoke method via CLI:

```bash
dockpipe pipelang invoke --in workflows/mywf/model.pipe --class DefaultDeployConfig --method FullImage --format text
```

Materialize all `.pipe` files under configured compile roots:

```bash
dockpipe pipelang materialize --workdir .
```

## Source and diagnostic contract

All compiler entrypoints admit source through one deterministic source-set contract. File identity
is the normalized source path, input must be strict UTF-8, and token/AST spans are half-open UTF-8
byte ranges tied to that identity. Resolved diagnostics also expose one-based line, Unicode scalar
column, and UTF-16 column values so CLI and editor rendering share the same locations.

Diagnostics have stable `code`, `category`, and `severity` fields, one primary span, optional
related spans, and deterministic file/span/code ordering. `pipelang check --format json` exposes
schema 1 of that compiler result. It is offline and inert: it does not evaluate methods, emit
workflow/bindings artifacts, refresh generated stores, or enter any runtime/resolver path. The
legacy `compile`, `invoke`, catalog, and materialize paths use the same parser/source identities;
their `v0.0.0.1` syntax and artifacts remain frozen.

## Structured module-binding foundation

The compiler library also has a syntax-independent module-binding query for the accepted future
contract. `AnalyzeModuleSet` receives an explicit non-legacy language-contract identity, one root
module, complete module source bytes, structured module/symbol imports with durable spans, and a
complete dependency lock. Each locked module records its direct dependencies and a deterministic
SHA-256 digest over normalized source identities plus exact source bytes. Missing bytes, digest
drift, undeclared dependencies, duplicate module owners, unknown/private/ambiguous imports, and
import cycles fail through the same ordered structured-diagnostic contract. Compilation remains
offline and never fetches or repairs dependencies.

This is a binder/input foundation, not a released source surface. No post-`v0.0.0.1` public name,
version, module keyword, import spelling, manifest format, CLI selector, YAML field, or editor grammar
is selected by it. The existing parser and every CLI/catalog/materialize/package/editor consumer
remain on the frozen sibling-source-set lane until a separately reviewed migration chooses those
public spellings explicitly.

The next compiler-only foundation can additionally receive explicit semantic-ID assignments by
declaration span. `AnalyzeSemanticModuleSet` validates lowercase dotted ASCII IDs, requires them for
public modules/types/members, leaves private implementation declarations ID-optional, verifies a
separate semantic-assignment digest in the dependency lock, and carries matching IDs through
structured diagnostics. `BuildSemanticProjection` emits deterministic module, import, type, member,
resolved-type, lock-digest, source-range, and diagnostic records without exposing analysis-local
`SymbolID` values. Its language, compiler, and projection identities are explicit inputs; no first
production values or source spellings have been selected yet.

## Typed executable compiler foundation

The separately governed first executable compiler slice uses the accepted `v0.1.0` structured
semantic-module lane without adding public syntax or a CLI selector. One public expression-bodied
pure method can be selected by its callable semantic identity and lowered through distinct layers:

```text
checked semantic analysis -> typed HIR -> normalized Core IR -> Go backend
```

Typed HIR retains bound ownership, stable semantic identity, resolved types, parameter bindings,
and durable source spans without target details. Core IR removes parser/source and analysis-local
concepts while preserving the typed function signature and normalized literal, parameter-reference,
and operator nodes. The Go backend's only PipeLang dependency is Core IR; an architecture test
rejects parser, AST/compiler-root, or HIR imports in that backend.

The proven fixture is the existing-syntax pure function `Ready(int count) => count > 0`. Its HIR,
Core, and generated-Go bytes are golden-tested; generated Go is compiled and executed under a
temporary offline module, and its result matches the existing pure evaluator. The first backend
fails explicitly on Core capabilities whose exact cross-target behavior belongs to later coherent
language slices.

The first numeric slice normalizes source-level `int` and `float` into explicit target-independent
HIR/Core representations: signed two's-complement 64-bit integer and IEEE-754 binary64. Stable
semantic callable identities retain their existing primitive names, while executable IR no longer
asks a backend to infer numeric width or signedness. The `v0.1.0` semantic lane permits comparison
and equality only between identical numeric representations and never inserts integer/float
conversions. Generated Go is checked against the reference evaluator for ordinary comparisons,
unordered and unequal `NaN`, and equal positive/negative zero. Numeric arithmetic, division, and
negation remain rejected by both semantic analysis and the backend until checked overflow and other
recoverable arithmetic failures can be represented as typed `Result` values. The frozen
`v0.0.0.1` compile/invoke behavior and artifacts remain unchanged, and executable Go is not a
workflow/runtime backend.

The next compiler-internal slice establishes that missing representation without selecting public
syntax. HIR and Core can carry `Result<Success, ArithmeticError>` structurally; Core owns the single
checked-arithmetic signature and failure contract consumed by both an inert Core conformance
evaluator and the Go backend. Signed 64-bit addition, subtraction, multiplication, and negation fail
with `overflow`; binary64 division fails on positive or negative zero with `division_by_zero` and
otherwise retains IEEE-754 behavior. Generated Go exposes an explicit result value and never uses a
panic as a domain outcome. Boundary tests compare integer semantics against exact mathematical
integers and execute the same success/failure cases through Core evaluation and generated Go.

Production source arithmetic remains fail-closed in `v0.1.0`. Existing declarations cannot silently change
from returning a number to returning a result, and the compiler does not invent a `Result` spelling,
an `ArithmeticError` source declaration, or implicit unwrapping. Those public type identities,
spellings, and migration rules require a separately accepted synchronized language slice.

That synchronized decision is now accepted for the first bounded production-source slice. An
explicit `v0.2.0` module may declare
`Result<int, ArithmeticError> Add(int left, int right) => left + right;`. The language-owned semantic
identities are `pipelang:result` and `pipelang:arithmetic.error`; callable identity and
`pipelang.semantic.v1` retain the existing structured applied-type representation. `v0.1.0`
continues to reject arithmetic with `PL3028`, and no source or package migrates implicitly.

In this first slice, the checked addition must be the complete expression-bodied method body and
the declared return must match exactly. The expression itself produces the explicit result; there
is no conversion, wrapping, unwrap, propagation, nesting, extraction, matching, or use as an
ordinary integer. General results, other source arithmetic, and result-consumption syntax remain
outside this contract.

The next explicit contract, `v0.3.0`, preserves that `v0.2.0` addition unchanged and additionally
admits exactly
`Result<int, ArithmeticError> Subtract(int left, int right) => left - right;`. The subtraction must
likewise be the complete expression-bodied method body and produces either an explicit integer
success or the existing closed `overflow` error. It reuses `pipelang:result`,
`pipelang:arithmetic.error`, `pipelang.compiler.v1`, and `pipelang.semantic.v1`; no source or package
migrates implicitly. Multiplication, negation, division, nested fallible expressions, and general
Result handling remain outside the production source contract.

The explicit `v0.4.0` contract preserves the prior addition and subtraction and additionally admits
exactly
`Result<int, ArithmeticError> Multiply(int left, int right) => left * right;`. Multiplication is the
complete expression-bodied method body and produces either an explicit integer success or the same
closed `overflow` error. It reuses the existing Result/error identities and compiler/projection
contracts; no source or package migrates implicitly. Negation, division, nested fallible
expressions, and general Result handling remain outside the production source contract.

The explicit `v0.5.0` contract preserves the prior binary arithmetic and additionally admits exactly
`Result<int, ArithmeticError> Negate(int value) => -value;`. Negation is the complete
expression-bodied method body and produces either an explicit integer success or the same closed
`overflow` error for the minimum integer. It reuses the existing Result/error identities and
compiler/projection contracts; no source or package migrates implicitly. Division, nested fallible
expressions, and general Result handling remain outside the production source contract.

The explicit `v0.6.0` contract preserves `v0.5.0` unchanged and additionally admits exactly
`Result<float, ArithmeticError> Divide(float left, float right) => left / right;`. Division is the
complete expression-bodied method body. A positive or negative zero divisor produces the existing
closed `division_by_zero` error; every nonzero divisor produces an explicit binary64 success while
retaining IEEE-754 behavior including `NaN`, infinities, and signed zero. The widening from the
integer Result slice to this exact float Result return reuses `pipelang:result`,
`pipelang:arithmetic.error`, `pipelang.compiler.v1`, and `pipelang.semantic.v1`. No source or package
migrates implicitly, and nested fallible expressions and general Result handling remain outside the
production source contract.

The explicit `v0.7.0` contract preserves every direct checked-arithmetic method from `v0.6.0` and
additionally makes the two existing arithmetic Result shapes transportable through one pure class
method boundary. The exact admitted forms have one parameter whose type is identical to the method
return and whose identifier is the complete body:

```pipelang
Result<int, ArithmeticError> ForwardInt(Result<int, ArithmeticError> value) => value;
Result<float, ArithmeticError> ForwardFloat(Result<float, ArithmeticError> value) => value;
```

Method and parameter names remain source-owned. Both callable positions reuse `pipelang:result` and
`pipelang:arithmetic.error`; HIR, Core evaluation, and generated Go preserve the same explicit
success payload or closed arithmetic error without wrapping, unwrapping, conversion, or target
exception behavior. `v0.1.0` through `v0.6.0` remain unchanged and no source or package selects
`v0.7.0` implicitly. Result fields, interface signatures, extra or mismatched parameters, nested or
alternate Result types, constructors, inspection, extraction, matching, propagation, and use as an
ordinary `int` or `float` remain outside the production source contract.

The explicit `v0.8.0` contract preserves every earlier numeric and arithmetic Result rule and adds
ordinal ordering for PipeLang `string` values. The exact admitted source shape is one expression-
bodied class method returning `bool`, with exactly two `string` parameters, and one of `<`, `<=`,
`>`, or `>=` comparing those parameters in declared order as the complete body:

```pipelang
bool Before(string left, string right) => left < right;
```

Strings remain immutable preserved Unicode scalar sequences. Ordering is lexicographic by scalar
value; equality and ordering do not normalize, case-fold, apply culture rules, or use target
collation. Existing string concatenation, equality, and inequality keep their prior-version source
meaning and now share the same validated Core evaluator/backend text contract. Invalid UTF-8 cannot
enter a PipeLang string value; a malformed host value is an infrastructure boundary failure.
`v0.1.0` through `v0.7.0` continue to reject string relational ordering, and no source or package
selects `v0.8.0` implicitly. Hashing, normalization/case/grapheme APIs, structural value equality,
optionals, general Result handling, records, unions, collections, and broader expressions remain
outside this production source contract.

The explicit `v0.9.0` contract preserves every `v0.8.0` text and arithmetic Result rule and adds
public, nonempty primitive immutable record declarations. Fields are public, have no defaults, and
use only `string`, `int`, `float`, or `bool`:

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

Executable record use is limited to one class method with exactly one parameter, an identical
record return type, and that parameter as the complete body. Record and field semantic identities
are stable; semantic projection exposes the deterministic identity-ordered member surface, while
typed HIR, target-neutral Core, Core evaluation, and generated Go retain declared field order and
exact primitive types. String fields preserve the existing
strict UTF-8 boundary. Records have value semantics: the Core evaluator does not expose a mutable
field-vector alias, and generated Go transports the corresponding struct by value.

`Record` is contextual only under explicit `v0.9.0`, so earlier contracts retain their source
grammar and may still use `Record` as an identifier. No source or package migrates implicitly.
Empty/private records, annotations, implemented interfaces, methods, private/nonprimitive fields,
field defaults, class/interface record fields, mismatched transport signatures, construction,
field access, mutation, equality, hashing, ordering, matching, nesting, Result integration,
optionals, unions, and collections remain outside the production source contract.

The explicit `v0.10.0` contract preserves every `v0.9.0` rule and adds one-hop read-only field
projection from an existing primitive record parameter. The exact admitted form is one class
method with exactly one record parameter, the selected field's exact primitive return type, and a
direct `parameter.Field` body:

```pipelang
public Record Row {
    public string Id;
}
public Class Root {
    public string IdOf(Row value) => value.Id;
}
```

The projection reuses the record and field semantic identities established by `v0.9.0`; it adds no
type identity and does not change `pipelang.semantic.v1`. Typed HIR and target-neutral Core carry
the receiver's identified schema plus the selected field identity, name, declared position, and
exact primitive result type. Core evaluation validates the complete record value before returning
the field, and generated Go validates the record before direct named-field access. String fields
retain the strict UTF-8 boundary.

`v0.1.0` through `v0.9.0` do not accept the new member expression, and no source or package
migrates implicitly. Unknown, inaccessible, or non-record members, extra parameters, mismatched
returns, nested or chained access, construction, mutation, equality, hashing, ordering, matching,
record nesting, Result integration, optionals, unions, collections, calls, indexing, and general
member access remain outside the production source contract.

The explicit `v0.11.0` contract preserves every `v0.10.0` rule and adds exact construction of one
existing public primitive record. The admitted form is one expression-bodied class method returning
the record, with exactly one primitive parameter per field in declaration order and with the exact
field types. Its initializer assigns every field exactly once, in declaration order, from the
corresponding direct parameter:

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

Construction reuses the record and field semantic identities and the enclosing method's callable
identity; it adds no constructor identity and does not change `pipelang.semantic.v1`. Typed HIR and
target-neutral Core use `record_construct` with the record identity and each field's identity, name,
declared position, and direct parameter value. Core evaluation and generated Go both validate the
complete immutable record, including strict UTF-8 string fields, and retain declared field order.

`v0.1.0` through `v0.10.0` reject this expression without implicit migration. Unknown fields use
`PL3004`, duplicates use `PL3005`, and missing, extra, reordered, mismatched, or invalid signatures
use `PL3006`; non-direct bodies or values use `PL3009`. Malformed HIR and Core remain `PL3026` and
`PL3027`. Defaults, omitted or computed values, nesting, construction inside another expression,
mutation, general member access, equality, hashing, ordering, matching, Result integration,
optionals, unions, and collections remain outside the production source contract.

The explicit `v0.12.0` contract preserves every `v0.11.0` rule and adds structural equality and
inequality for one existing public primitive record. The admitted form is one expression-bodied
class method returning `bool`, with exactly two parameters of the same record type and a direct
comparison of those parameters in declaration order:

```pipelang
public Record Row {
    public string Id;
    public int Count;
    public float Ratio;
    public bool Ready;
}
public Class Rows {
    public bool Same(Row left, Row right) => left == right;
    public bool Different(Row left, Row right) => left != right;
}
```

Equality compares fields structurally in declaration order. Strings retain preserved ordinal
scalar-sequence equality, integers and booleans compare exactly, and binary64 retains the pinned
IEEE rules: NaN is unequal, while positive and negative zero compare equal. `!=` is the logical
complement of that complete structural result. Both operands are validated against the identical
record schema before evaluation.

The comparison reuses the existing record, field, and callable identities; it adds no operator or
type identity and does not change `pipelang.semantic.v1`. Typed HIR and target-neutral Core carry
the existing `equal` or `not_equal` binary operator with the identified record operands. Core
evaluation and generated Go agree on the structural result, validate strict UTF-8 string fields,
and reject malformed operand order or schema before execution.

`v0.1.0` through `v0.11.0` reject record equality without implicit migration. Invalid record
signatures or placements use `PL3006`; reversed, repeated, nested, field-only, ordered, or otherwise
non-direct bodies use `PL3009`. Malformed HIR and Core remain `PL3026` and `PL3027`. Hashing, record
ordering, nesting, mutation, general member access, optionals, general Result handling, unions,
collections, blocks, matching, calls, and indexing remain outside the production source contract.

The explicit `v0.13.0` contract preserves every `v0.12.0` rule and adds one primitive optional
value slice. `Optional<T>` has the fixed semantic identity `pipelang:optional`, projected as one
applied argument, and admits only `string`, `int`, `float`, or `bool` for `T`. The complete public
source surface is four exact direct class-method shapes:

```pipelang
public Class Values {
    public Optional<string> Present(string value) => some(value);
    public Optional<string> Absent() => none<string>();
    public Optional<string> Forward(Optional<string> value) => value;
    public bool HasValue(Optional<string> value) => has_value(value);
}
```

`some(value)` carries the sole corresponding primitive parameter; `none<T>()` carries no payload;
identity transport returns the sole identical optional parameter; and `has_value(value)` returns
`true` only for the canonical present variant. Typed HIR and target-neutral Core retain an explicit
tagged present-or-absent value. Core evaluation and generated Go agree on construction, transport,
and presence inspection, validate strict UTF-8 present string payloads, and reject malformed or
zero/nil optional representations rather than interpreting them as absence.

`v0.1.0` through `v0.12.0` reject the type and intrinsic expressions without implicit migration.
Invalid payload types, placements, or method signatures use `PL3006`; literals, computed values,
nested expressions, mismatched construction, and other non-direct bodies use `PL3009`. Malformed
HIR and Core remain `PL3026` and `PL3027`. Extraction or unwrapping, equality, defaults, optional
record fields, nesting, chaining, fallback, propagation, matching, mutation, Result integration,
unions, and collections remain outside the production source contract.

The explicit `v0.14.0` contract preserves every `v0.13.0` rule and adds one primitive Optional
defaulting form. `value_or(Optional<T>, T) -> T` reuses the existing `pipelang:optional` semantic
identity and admits only `string`, `int`, `float`, or `bool` for `T`. The complete added public
source surface is one exact direct class-method shape:

```pipelang
public Class Values {
    public string ValueOr(Optional<string> value, string fallback) =>
        value_or(value, fallback);
}
```

The method has exactly two parameters. Parameter 0 is the Optional operand, parameter 1 is the
identically typed fallback, the return type is that same primitive `T`, and the body directly
references those parameters in that order. Typed HIR and target-neutral Core carry an explicit
`optional_value_or` node. Core evaluation and generated Go canonically validate both arguments
before selecting the present payload or fallback. Strict UTF-8 therefore applies to a string
fallback even when a present payload is selected. Binary64 payloads and fallbacks preserve their
IEEE representation, including NaN and signed zero, without adding equality or ordering semantics.

`v0.1.0` through `v0.13.0` reject `value_or` without implicit migration. Invalid payload types,
placements, or method signatures use `PL3006`; literals, computed operands, reordered parameters,
nested expressions, and other non-direct bodies use `PL3009`. Malformed HIR and Core remain
`PL3026` and `PL3027`. Optional extraction beyond this exact defaulting form, equality, implicit
defaults, record fields, nesting, chaining, propagation, matching, mutation, conversion,
fallibility, Result composition, unions, collections, hashing, and ordering remain outside the
production source contract.

The explicit `v0.15.0` contract preserves every `v0.14.0` rule and adds one immutable record-list
value slice. `List<R>` has the fixed semantic identity `pipelang:list`, projected with the existing
identified public primitive record `R` as its sole applied argument. The complete added public
source surface is three exact direct class-method shapes:

```pipelang
public Class Rows {
    public List<Row> EmptyRows() => empty_list<Row>();
    public List<Row> OneRow(Row value) => list(value);
    public List<Row> ForwardRows(List<Row> values) => values;
}
```

`empty_list<R>()` creates a canonical non-nil empty value, `list(value)` creates one element from
the sole corresponding record parameter, and identity transport returns the sole identical
`List<R>` parameter as an immutable value. Order and every record field value are preserved; list
identity, equality, hashing, and ordering are not observable. Typed HIR and target-neutral Core
carry explicit `list`, `list_empty`, and `list_singleton` representations. Core evaluation and the
Core-only Go backend validate every element, including strict UTF-8 record fields, and copy list
storage before transport so target slice aliasing cannot become PipeLang mutation.

`v0.1.0` through `v0.14.0` reject these value forms without implicit migration. Invalid element
types, placements, or method signatures use `PL3006`; literals, nested construction, mismatched
types, and other non-direct bodies use `PL3009`. Malformed HIR and Core remain `PL3026` and
`PL3027`. Primitive, optional, result, or nested-list elements; list fields; literals; append;
indexing; count; iteration; filtering; sorting; equality; hashing; maps; sets; builders; mutation;
record nesting; Step-8 control flow; effects; and Application IR remain outside the production
source contract.

The explicit `v0.16.0` contract preserves every `v0.15.0` rule and adds one direct immutable
record-list cardinality operation:

```pipelang
public int CountRows(List<Row> values) => count(values);
```

`count(List<R>) -> int` requires exactly one `List<R>` parameter as the complete direct operand and
returns its nonnegative signed-64-bit cardinality. The evaluator and Core-only Go backend validate
the complete non-null list, every record element, and every strict UTF-8 string field before
observing its length. Typed HIR and target-neutral Core carry `list_count`; the existing
`pipelang:list` identity, record identities, `pipelang.compiler.v1`, and `pipelang.semantic.v1`
remain unchanged. `v0.1.0` through `v0.15.0` reject the form without implicit migration.

The explicit `v0.17.0` contract preserves every `v0.16.0` rule and adds one direct immutable
record-list append operation:

```pipelang
public List<Row> AppendRow(List<Row> values, Row value) =>
    append(values, value);
```

`append(List<R>, R) -> List<R>` requires exactly the existing record-list parameter first and one
value of its existing public primitive record element type second. Both direct parameter references
must appear in declaration order as the complete body. The result preserves every existing element
in order and adds the new value last. Evaluation validates the complete input list and appended
record, including every UTF-8 string field, then returns fresh list storage so caller-owned slices
cannot mutate the result. A nil list or a cardinality that cannot grow within the signed-64-bit
language boundary fails closed. Typed HIR and target-neutral Core carry `list_append`; the fixed
`pipelang:list` identity and existing record/field/callable identities remain unchanged, and the Go
backend consumes Core only.

`v0.1.0` through `v0.16.0` reject `append` without implicit migration. Invalid element types,
placements, or signatures use `PL3006`; computed, reordered, nested, or otherwise non-direct
operands use `PL3009`. Malformed HIR and Core remain `PL3026` and `PL3027`. List literals, list
fields, variadic construction, indexing, iteration, filtering, sorting, equality, hashing, maps,
sets, builders, mutation, record nesting, Step-8 control flow, effects, and Application IR remain
outside the production source contract.

The explicit `v0.18.0` contract preserves every `v0.17.0` rule and admits one existing public
primitive record `R` as the payload of the existing fixed `pipelang:optional` identity. The complete
added source surface is five exact direct class-method shapes:

```pipelang
public Optional<Row> PresentRow(Row value) => some(value);
public Optional<Row> AbsentRow() => none<Row>();
public Optional<Row> ForwardRow(Optional<Row> value) => value;
public bool HasRow(Optional<Row> value) => has_value(value);
public Row RowOr(Optional<Row> value, Row fallback) => value_or(value, fallback);
```

The type argument retains the existing record identity, so semantic projection represents
`Optional<R>` as `pipelang:optional<R>` and callable identities retain their structured record and
optional arguments. `some` accepts only the sole corresponding record parameter; `none` names the
same record type; identity transport returns the sole identical Optional parameter; `has_value`
observes only the canonical tag; and `value_or` takes the Optional first and the identical record
fallback second. Both the Optional payload and fallback are validated before selection, including
every record field and strict UTF-8 string value. Evaluation returns copied record storage so
caller-owned values cannot introduce mutation through either the selected payload or fallback.

Typed HIR and target-neutral Core reuse their explicit optional nodes with a structured record
payload. The evaluator and Core-only Go backend consume those nodes without inspecting source or
HIR, reject nil or malformed tagged values, and preserve `pipelang.compiler.v1` and
`pipelang.semantic.v1`. `v0.1.0` through `v0.17.0` reject `Optional<R>` without implicit migration.
Invalid payloads, placements, or signatures use `PL3006`; constructed, computed, reordered,
additional, or otherwise non-direct bodies use `PL3009`; malformed HIR and Core remain `PL3026`
and `PL3027`.

Optional fields, Optional record construction from literals, nesting, chaining, equality,
hashing, ordering, implicit defaults, extraction beyond `value_or`, Optional/list/Result payloads,
record nesting, Step-8 control flow, effects, Application IR, and additional backends remain outside
the production source contract.

The explicit `v0.19.0` contract preserves every `v0.18.0` rule and admits one bounded read-only
snapshot envelope: `Result<List<R>, string>` for one existing public primitive record `R`. The
complete added source surface is six exact direct class-method shapes:

```pipelang
public Result<List<Row>, string> RowsOk(List<Row> value) =>
    ok<List<Row>, string>(value);
public Result<List<Row>, string> RowsFailed(string error) =>
    err<List<Row>, string>(error);
public Result<List<Row>, string> ForwardRows(Result<List<Row>, string> value) => value;
public bool RowsSucceeded(Result<List<Row>, string> value) => is_ok(value);
public List<Row> RowsOr(Result<List<Row>, string> value, List<Row> fallback) =>
    success_or(value, fallback);
public string ErrorOr(Result<List<Row>, string> value, string fallback) =>
    failure_or(value, fallback);
```

The type reuses `pipelang:result`, `pipelang:list`, the existing record identity, and primitive
`string`; `pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. `ok` and `err`
require explicit identical success/failure type arguments and their sole corresponding direct
parameter. Identity transport returns the sole identical Result parameter. `is_ok` observes only
the canonical tag. `success_or` and `failure_or` validate both the Result and fallback before
selection. Evaluation and the Core-only Go backend validate every active payload, list element,
record field, and strict UTF-8 string, then copy list and record storage for construction,
transport, and success selection.

Typed HIR and target-neutral Core carry explicit `result_ok`, `result_err`, `result_is_ok`,
`result_success_or`, and `result_failure_or` nodes. `v0.1.0` through `v0.18.0` reject this envelope
without implicit migration. Invalid payloads, placements, or signatures use `PL3006`; computed,
reordered, additional, or otherwise non-direct bodies use `PL3009`; malformed HIR and Core remain
`PL3026` and `PL3027`. General Result construction or consumption, arithmetic-Result construction,
propagation, matching, chaining, fields, nesting, arbitrary payloads, list iteration/filtering/
sorting/indexing, Step-8 control flow, effects, Application IR, and additional backends remain
outside the production source contract.

The explicit `v0.20.0` contract preserves every `v0.19.0` rule and admits one total, read-only
record-list consumer:

```pipelang
public Optional<Row> RowAt(List<Row> values, int index) => at(values, index);
```

`at(List<R>, int) -> Optional<R>` is admitted only for one existing public primitive record `R`,
with the list as the first direct parameter and a signed-64-bit index as the second direct parameter.
Indexing is zero-based. A negative or out-of-range index returns canonical `none`; an in-range index
returns canonical `some` containing a copied record. The complete list and every record field are
validated before the index is inspected, including strict UTF-8 validation for every string field,
so an invalid unselected element remains an infrastructure-boundary failure.

The type reuses `pipelang:list`, `pipelang:optional`, and the existing record identity. Callable
identity retains the exact `(List<R>, int) -> Optional<R>` shape; `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry an explicit
`list_at` node. The evaluator and Core-only Go backend consume only Core semantics and copy the
selected record so caller-owned list or record storage cannot alias the result.

`v0.1.0` through `v0.19.0` reject `at` without implicit migration. Invalid payloads, placements, or
signature shapes use `PL3006`; computed or otherwise non-direct operands after the exact signature
is admitted use `PL3009`; malformed HIR and Core remain `PL3026` and `PL3027`. Key lookup, slicing, iteration,
filtering, sorting, mapping, folding, mutation, selection preservation, Step-8 control flow,
effects, Application IR, and additional backends remain outside the production source contract.

The explicit `v0.21.0` contract preserves every `v0.20.0` rule and admits one stable-key,
read-only record-list consumer:

```pipelang
public Optional<Row> FindRow(List<Row> values, string key) =>
    find_by(values, Row.Id, key);
```

`find_by(List<R>, R.Field, string) -> Optional<R>` is admitted only for one existing public
primitive record `R`. `R.Field` is a static selector naming one public `string` field on that same
record type; it is not a runtime field value, lambda, predicate, comparer, or general member
expression. The list and key are the first and second direct parameters. The complete list, every
record field, and the key are validated before lookup. The first record whose selected field is
ordinal-equal to the key returns canonical `some` with copied record storage; no match returns
canonical `none`. Ordinal equality preserves Unicode scalar sequences without normalization,
case-folding, locale, or target collation.

The type reuses `pipelang:list`, `pipelang:optional`, primitive `string`, the existing record
identity, and the selected field semantic identity. Callable identity remains exactly
`(List<R>, string) -> Optional<R>`; `pipelang.compiler.v1` and `pipelang.semantic.v1` remain
unchanged. Typed HIR and target-neutral Core carry one explicit `list_find_by_text` node with direct
list/key references plus the selected field identity, name, and declaration position. The
evaluator and Go backend consume only Core semantics.

`v0.1.0` through `v0.20.0` reject `find_by` without implicit migration. Invalid list, selector,
field, key, return, placement, or signature shapes use `PL3004`/`PL3006` as applicable; computed or
otherwise non-direct operands after the exact signature is admitted use `PL3009`; malformed HIR
and Core remain `PL3026` and `PL3027`. Lambdas, predicates, composite keys, normalization,
case-insensitive or locale-sensitive matching, map/index construction, slicing, iteration,
filtering, sorting, mutation, Application IR, Step-8 control flow, effects, and additional backends
remain outside the production source contract.

The explicit `v0.22.0` contract preserves every `v0.21.0` rule and admits one stable-order,
read-only record-list filter:

```pipelang
public List<Row> FilterRows(List<Row> values, string key) =>
    filter_by(values, Row.State, key);
```

`filter_by(List<R>, R.Field, string) -> List<R>` is admitted only for one existing public
primitive record `R`. `R.Field` is the same static selector shape established by `find_by`: it
names one public `string` field on the same record type and is not a runtime value, lambda,
predicate, comparer, or general member expression. The list and key are the first and second direct
parameters. The complete list, every record field, and the key are validated before filtering.
Every ordinal-equal match is retained in input order, including duplicates. No matches return a
canonical non-nil empty list. The result uses fresh copied list and record storage and retains the
signed-64-bit cardinality boundary.

The type reuses `pipelang:list`, primitive `string`, the existing record identity, and the selected
field semantic identity. Callable identity remains exactly `(List<R>, string) -> List<R>`;
`pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral
Core carry one explicit `list_filter_by_text` node with direct list/key references plus the selected
field identity, name, and declaration position. The evaluator and Go backend consume only Core
semantics and preserve ordinal scalar-sequence equality without normalization, case-folding,
locale, or target collation.

`v0.1.0` through `v0.21.0` reject `filter_by` without implicit migration. Invalid list, selector,
field, key, return, placement, or signature shapes use `PL3004`/`PL3006` as applicable; computed or
otherwise non-direct operands after the exact signature is admitted use `PL3009`; malformed HIR
and Core remain `PL3026` and `PL3027`. Lambdas, predicates, multi-field or substring search,
normalization, case-folding, sorting, mapping, folding, general iteration, mutation, Application
IR, Step-8 control flow, effects, and additional backends remain outside the production source
contract.

The explicit `v0.23.0` contract preserves every `v0.22.0` rule and admits one direct, pure text
predicate:

```pipelang
public bool ContainsCaseFolded(string value, string query) =>
    contains_casefolded(value, query);
```

`contains_casefolded(string, string) -> bool` requires exactly two direct `string` parameters in
the declared order, a `bool` return, and the operation as the complete method body. Both operands
are validated as strict UTF-8 before processing. The operation applies Unicode 17.0.0 full default
case folding using only the pinned C and F mappings from `CaseFolding.txt`, then tests contiguous
scalar-sequence containment. An empty folded query matches. It excludes simple-only S mappings,
Turkic T mappings, normalization, locale tailoring, grapheme segmentation, trimming, and host
Unicode/case APIs. Thus `Straße` contains `STRASSE`, while decomposed `e\u0301` does not contain
composed `é` solely through this operation.

Callable identity remains exactly `(string, string) -> bool`; `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry the explicit
`text_contains_case_folded` node. The evaluator and Core-only Go backend consume the same embedded,
digest-checked Unicode table and never substitute a target-runtime case algorithm. `v0.1.0`
through `v0.22.0` reject the source and executable form without implicit migration. Computed,
reordered, literal, nested, extra-parameter, non-string, and wrong-return forms remain rejected.
Case-folded list filtering, multi-field search, normalization, locale-specific matching, lambdas,
composition, Application IR, Step-8 control flow, effects, and additional backends remain outside
the production source contract.

The explicit `v0.24.0` contract preserves every `v0.23.0` rule and admits one selected-field,
stable-order case-folded record-list filter:

```pipelang
public List<Row> SearchRows(List<Row> values, string query) =>
    filter_contains_casefolded(values, Row.Name, query);
```

`filter_contains_casefolded(List<R>, R.Field, string) -> List<R>` is admitted only for one existing
public primitive record `R`. The first and second direct parameters are the complete list and query;
`R.Field` statically identifies one public `string` field on the same record type. The complete list,
every record field, and the strict-UTF-8 query are validated before filtering. The operation applies
the exact pinned Unicode 17.0.0 full-default C/F folding from `contains_casefolded` to the selected
field and query, then retains every contiguous folded match in stable input order, including
duplicates. An empty query retains every element. No matches return a canonical non-nil empty list,
and results use fresh copied list and record storage.

The operation reuses `pipelang:list`, primitive `string`, the existing record identity, and the
selected field semantic identity. Callable identity remains exactly `(List<R>, string) -> List<R>`;
`pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral
Core carry one explicit `list_filter_contains_case_folded_text` node with direct list/query
references plus the selected field identity, name, and declaration position. The evaluator and
Core-only Go backend consume the same digest-checked Unicode table and never infer semantics from a
host case API.

`v0.1.0` through `v0.23.0` reject the source and executable form without implicit migration.
Invalid list, selector, field, query, return, placement, or signature shapes use `PL3004`/`PL3006`
as applicable; computed or otherwise non-direct operands use `PL3009`; malformed HIR and Core
remain `PL3026` and `PL3027`. Trimming, joined or multi-field search, normalization, locale
tailoring, predicates, lambdas, sorting, general iteration, mutation, Application IR, Step-8
control flow, effects, and additional backends remain outside the production source contract.

The explicit `v0.25.0` contract preserves every `v0.24.0` rule and admits only this fallible text
envelope:

```pipelang
public Result<string, string> TextOk(string value) => ok<string, string>(value);
public Result<string, string> TextFailed(string error) => err<string, string>(error);
public Result<string, string> ForwardText(Result<string, string> value) => value;
public bool TextSucceeded(Result<string, string> value) => is_ok(value);
public string TextOr(Result<string, string> value, string fallback) => success_or(value, fallback);
public string ErrorOr(Result<string, string> value, string fallback) => failure_or(value, fallback);
```

These are complete direct method bodies with the exact parameter and return types shown. Success
and failure construction, identity transport, inspection, and both defaulting operations validate
their complete tagged value and all supplied text, including an unselected fallback, as strict
UTF-8. Failed values carry the canonical empty success payload. The type reuses the existing
`pipelang:result` identity with primitive `string` arguments; `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core reuse the existing
Result expression kinds, and the evaluator and Core-only Go backend apply the same rules.

`v0.1.0` through `v0.24.0` reject these forms without implicit migration. Other Result argument
types or source shapes, `is_err`, unwrap, propagation, mapping, matching, effects, composition,
Application IR, Step-8 control flow, and additional backends remain outside the production source
contract.

The explicit `v0.26.0` contract preserves every `v0.25.0` rule and admits only direct deterministic
text trimming:

```pipelang
public string Trim(string value) => trim(value);
```

The method has exactly one direct `string` parameter, returns `string`, and uses `trim(value)` as
its complete body. Both evaluator and Core-only Go backend reject invalid UTF-8, remove the maximal
leading and trailing sequence of scalars in Unicode 17.0.0 `White_Space`, preserve every interior
scalar exactly, and return `""` when the input contains only whitespace. The pinned set contains
exactly 25 scalars in 10 source-ordered ranges; it does not include U+180E, U+200B, or U+FEFF.

Callable and primitive-string identity are unchanged; `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core carry one explicit
`text_trim` node. `v0.1.0` through `v0.25.0` reject this form without implicit migration.
Normalization, case folding, locale tailoring, grapheme segmentation, scalar enumeration,
collapse, replacement, composition, trimming record fields or lists, Application IR, Step-8
control flow, effects, and additional backends remain outside the production source contract.

The explicit `v0.27.0` contract preserves every `v0.26.0` rule and admits only this exact direct
five-field joined record-list search:

```pipelang
public List<ContainerRow> SearchRows(List<ContainerRow> values, string query) =>
    filter_joined_contains_casefolded(
        values,
        ContainerRow.Name,
        ContainerRow.State,
        ContainerRow.Image,
        ContainerRow.Ports,
        ContainerRow.Created,
        query
    );
```

`filter_joined_contains_casefolded` requires exactly two direct parameters, `List<R>` then
`string`, and the identical `List<R>` return. Its five selectors are distinct existing public
`string` fields of the same existing public primitive record `R`; selectors are static source
operands, not runtime field values. The complete list, every record and every field, and the query
are validated as canonical values and strict UTF-8 before filtering. Selected field values are
joined in source order with exactly one U+0020 SPACE. The query is trimmed with the pinned
`v0.26.0` Unicode 17.0.0 `White_Space` rule, then joined text and query use the pinned `v0.23.0`
Unicode 17.0.0 full-default C/F case-folded contiguous containment rule. An empty trimmed query
retains every row; matches retain stable input order; no matches return canonical non-nil empty
storage; results use fresh copied list and record storage.

Callable identity remains `(List<R>, string) -> List<R>` and existing list, record, field, and
primitive identities are reused. `pipelang.compiler.v1` and `pipelang.semantic.v1` remain
unchanged. Typed HIR and target-neutral Core carry one explicit
`list_filter_joined_contains_case_folded_text` node with direct list/query references and the five
ordered field identities, names, and declaration positions. `v0.1.0` through `v0.26.0` reject the
source, HIR, and executable Core forms without implicit migration. Arbitrary selector counts,
field-selector values, predicates, regex, normalization, locale tailoring, sorting, nested/general
composition, Application IR, Step-8 control flow, effects, and additional backends remain outside
the production source contract.

The explicit `v0.28.0` contract preserves every `v0.27.0` rule and admits only this exact direct
stable ordinal record-list sort:

```pipelang
public List<ContainerRow> SortRows(List<ContainerRow> values) =>
    sort_by_ordinal(values, ContainerRow.Name);
```

`sort_by_ordinal` requires exactly one direct `List<R>` parameter and the identical `List<R>`
return. Its sole selector is one existing public `string` field of the same existing public
primitive record `R`; the selector is a static source operand, not a runtime field value. The
complete list, every record, and every selected and unselected field are validated as canonical
values and strict UTF-8 before sorting. Results are ascending under the existing `v0.8.0` ordinal
Unicode scalar-sequence order. Equal keys retain input order. Empty input returns canonical non-nil
empty storage, and every result uses fresh copied list and record storage. No normalization, case
folding, locale collation, or host-runtime ordering participates.

Callable identity remains `(List<R>) -> List<R>` and existing list, record, field, and primitive
identities are reused. `pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Typed HIR
and target-neutral Core carry one explicit `list_sort_by_ordinal_text` node with the direct list
reference and selected field identity, name, and declaration position. `v0.1.0` through `v0.27.0`
reject the source, HIR, and executable Core forms without implicit migration. Descending or
direction arguments, multi-key sorting, arbitrary comparers or predicates, numeric/record sorting,
in-place mutation, general indexing, nested/general composition, Application IR, Step-8 control
flow or matching, effects, and additional backends remain outside the production source contract.

The explicit `v0.29.0` contract preserves every `v0.28.0` rule and widens only the existing direct
joined record-list search to a record-bounded variable selector count:

```pipelang
public List<NetworkRow> SearchRows(List<NetworkRow> values, string query) =>
    filter_joined_contains_casefolded(
        values,
        NetworkRow.Name,
        NetworkRow.Driver,
        NetworkRow.Scope,
        query
    );
```

`filter_joined_contains_casefolded` still requires exactly two direct runtime parameters,
`List<R>` then `string`, and the identical `List<R>` return. It now accepts two or more distinct
static selectors, bounded by the public `string` fields of the same existing primitive record `R`.
The complete list, every record and selected or unselected field, and the query are validated before
filtering. Selected strings are joined in source order with one U+0020 SPACE; the query is trimmed
under v0.26.0; both operands use the pinned v0.23.0 Unicode 17.0.0 full-default case-folded
containment rule. Stable input order, canonical non-nil empty output, and fresh copied list/record
storage remain mandatory.

Callable, list, record, field, and primitive identities remain unchanged. `pipelang.compiler.v1`
and `pipelang.semantic.v1` remain unchanged. Typed HIR and target-neutral Core reuse the existing
`list_filter_joined_contains_case_folded_text` node and its ordered selector identities, names, and
declaration positions. `v0.27.0` and `v0.28.0` retain their exact five-selector rule; `v0.1.0`
through `v0.26.0` still reject the form. Zero/one selector, selector values, duplicates, predicates,
regex, normalization, locale tailoring, sorting, nested/general composition, Application IR,
Step-8 control flow, effects, and additional backends remain outside the production contract.

The explicit `v0.30.0` contract preserves every `v0.29.0` rule and widens only the existing direct
stable ordinal record-list sort to a record-bounded variable selector count:

```pipelang
public List<ContainerRow> SortRows(List<ContainerRow> values) =>
    sort_by_ordinal(values, ContainerRow.State, ContainerRow.Name);
```

`sort_by_ordinal` still requires one direct `List<R>` parameter, the identical `List<R>` return,
and one or more static selectors naming distinct public `string` fields of the same existing public
primitive record `R`. One selector preserves the exact `v0.28.0` behavior and projection. With two
or more selectors, comparison is stable ascending lexicographic order: selected fields are compared
in source order under the existing ordinal Unicode scalar-sequence rule, and the first unequal field
decides the row order. Rows equal across every selector retain input order. The complete list, every
record, and every selected and unselected field are validated as canonical strict-UTF-8 values
before sorting. Empty input returns canonical non-nil empty storage, and every result uses fresh
copied list and record storage.

Callable, list, record, field, and primitive identities remain unchanged. `pipelang.compiler.v1`
and `pipelang.semantic.v1` remain unchanged. The one-selector form continues to use
`list_sort_by_ordinal_text`; typed HIR and target-neutral Core use the explicit
`list_sort_by_ordinal_texts` node only for two or more selectors, carrying their ordered identities,
names, and declaration positions. `v0.28.0` and `v0.29.0` retain their exact one-selector rule, and
earlier versions continue to reject sorting without implicit migration. Descending or per-key
direction arguments, dynamic or duplicate selectors, arbitrary comparers or predicates,
numeric/record sorting, normalization, case folding, locale collation, mutation, general indexing,
nested/general composition, Application IR, Step-8 control flow or matching, effects, and
additional backends remain outside the production contract.

The explicit `v0.31.0` contract preserves every `v0.30.0` rule and adds the first bounded
Step-8 function seam: a same-class named pure record predicate consumed by a direct stable filter.

```pipelang
public bool Matches(ContainerRow row, string query)
    => contains_casefolded(row.Name, trim(query))
       || contains_casefolded(row.State, trim(query));

public List<ContainerRow> Search(List<ContainerRow> values, string query)
    => filter(values, Matches, query);
```

The exact general spelling is `filter(values, PredicateName, argument1, ...)`. `PredicateName`
must uniquely resolve in the same public class to a public
`bool PredicateName(R item, P1, ...)`, where `R` is the list's existing public primitive record
and every remaining parameter is primitive and exactly matches the corresponding direct filter
argument. The predicate body is limited to literals, its primitive parameters, one-hop public
primitive fields of `item`, logical not/and/or, equality and ordering comparisons, and nested use
of the already accepted pure `contains_casefolded` and `trim` operations. Class state, arbitrary
calls, record construction, lambdas, closures, function values, overloads, effects, async,
Optional/Result predicates, and nested/general collection composition remain excluded.

Typed HIR and target-neutral Core carry an explicit `list_filter_predicate` node with the
predicate's existing semantic method identity, name, direct list operand, and ordered primitive
operands. Predicate parameters are local bindings and create no new public identity kind.
`pipelang.compiler.v1` and `pipelang.semantic.v1` remain unchanged. Core validation resolves the
predicate identity within the same lowered program. The evaluator and deterministic Core-only Go
backend validate the complete list, every record and field, and every primitive argument before
iteration; invoke the predicate exactly once per row in source order; require `bool`; fail
atomically; and return a stable, canonical non-nil, freshly copied list. `v0.1.0` through
`v0.30.0` reject `filter` without implicit migration. General functions/calls, matching,
propagation, Application IR, UI/runtime behavior, and additional backends remain later decisions.

## Artifacts

Compile emits:
- `<Class>.workflow.yml`
- `<Class>.bindings.json`
- `<Class>.bindings.env`

Those artifacts are inspectable authoring outputs. They are not a second execution engine.

`bindings.env` is intended for direct script consumption:

```bash
source bin/.dockpipe/pipelang/DefaultDeployConfig.bindings.env
echo "$PIPELANG_IMAGE"
```

## Architecture boundary

PipeLang `v0.0.0.1` is an authoring/compiler feature.

DockPipe execution still uses compiled YAML and existing workflow execution paths.

`dockpipe run` does not parse PipeLang directly.

Think of the layering as:

- PipeLang: types, defaults, docs
- workflow YAML: execution plus optional authored `view:` metadata
- launcher/tools: render the model/view contract
- runner: consume the existing workflow/env contract

## Accepted future compiler boundary

Future executable PipeLang uses one compiler contract:

```text
source -> syntax -> module binding -> typed HIR -> Core IR
                                             |          |
                                             |          `-> executable backends (Go first)
                                             `-> versioned semantic projection
                                                          |- Application IR
                                                          `- Service IR
```

Syntax trees, bound trees, typed HIR, and Core IR remain distinct compiler representations. The
public semantic projection is independently versioned. Application IR and Service IR specialize
the same semantic/Core foundation; neither reparses `.pipe` files nor defines language behavior.

Pure compilation remains offline and deterministic. Executable entrypoints declare typed effects;
ordinary external work still crosses the governed DockPipe workflow/package -> runtime -> resolver
-> optional strategy boundary. Qt, Go, HTTP, browser, operating-system, and embedded behavior stays
in target resolvers and cannot redefine or silently weaken PipeLang semantics.

Go is the first deterministic native backend and bootstrap seed. Eventual self-hosting uses a
pinned Go stage 0, a PipeLang stage 1, and a PipeLang stage 2; normalized semantic/Core artifacts,
diagnostics, behavior, and target outputs must reproduce between stages 1 and 2. Full, constrained,
and MCU target profiles select validated backend capabilities without changing source syntax.

The complete accepted decisions, compatibility inventory, and bounded implementation order live in
[TASK-021](../agents/tasks/pipelang-reactive-application-language/overview.md).

### PipeLang v0.32.0: explicit per-key ordinal direction

The `v0.32.0` directional form pairs each selected public string field with a contextual direction:

```pipelang
public List<ContainerRow> SortRows(List<ContainerRow> values) =>
    sort_by_ordinal(values, ContainerRow.State, descending, ContainerRow.Name, ascending);
```

There must be one or more distinct selector/direction pairs. Sorting validates the complete value,
uses stable ordinal Unicode scalar-sequence comparison in pair order, returns copied storage and a
canonical non-nil empty list, and preserves input order when all keys compare equal. Typed HIR and
Core expose `list_sort_by_ordinal_directions`; earlier ascending spellings and nodes remain versioned
compatibility contracts.


### PipeLang v0.33.0: safe general indexing

The bounded postfix form `values[index]` is available only in an exact two-parameter method
`Optional<R> M(List<R> values, int index)`, where `R` is an existing public primitive record. It returns
`none` for negative or out-of-bounds indices and a copied `some` record otherwise, after complete input
validation. It lowers to the existing target-neutral `list_at` HIR/Core operation; `at(values, index)`
remains compatible. Other receiver or index types, chaining, slicing, defaults, exceptions, and unchecked
access are excluded.

### PipeLang v0.34.0: bounded propagation

The contextual `propagate(carrier)` expression is accepted only as the direct payload of a complete
`some(...)` or bounded `ok<T, E>(...)` method body whose return type exactly equals the direct
parameter carrier. It extracts presence/success and returns absence/failure through explicit
source-located HIR/Core propagation control flow. Optional primitives and primitive records plus the
existing snapshot/text Result forms are the complete matrix. `PL3032` diagnoses misuse. There are no
exceptions, implicit conversions, arbitrary Results, effects, blocks, or target-specific errors.


PipeLang v0.35.0 adds exhaustive bounded matching over existing Optional and Result values with explicit `some`/`none` or `ok`/`err` arms and a bounded final `_` wildcard. Matching is pure, source-located, target-neutral Core control flow.

### PipeLang v0.36.0: same-class pure calls

Public expression-bodied methods may call one uniquely named public method on the same class using
`Method(expression, ...)`. Ordered argument types and the return type must match the resolved target
exactly. Call participants are closed over parameters and match-arm bindings rather than class-owned
state. Calls may nest and their arguments may use already admitted expressions. Private callers or
targets, missing or ambiguous targets, overloads, cross-class/module calls, and every direct or
indirect recursive cycle are rejected; `PL3033` reports cycles.

Typed HIR and target-neutral Core carry the resolved callable semantic identity, target name, and
ordered typed operands. Lowering includes the closed transitive dependency graph once, Core proves
same-owner identity, signature equality, target presence, and acyclicity, the evaluator uses
isolated copied call frames, and the Go backend emits only Core-validated calls. The compiler,
semantic projection, and Application IR schema versions remain `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1`; their language-contract metadata advances to
`v0.36.0`. Blocks, locals, branches, lambdas, function values, generics, effects, entrypoints, and
target-specific semantics remain outside this contract.

### PipeLang v0.37.0: general pure-call composition

The same resolved calls may appear throughout expressions already admitted as eager and pure,
including match-arm bodies, record construction, Optional construction, text operations, and
arithmetic or comparison operands. Match carriers and propagation operands remain direct
references, so composition does not add hidden control flow or change their established ownership
rules. The uniquely named public same-class target, exact ordered signature, parameter/arm-local
closure, and acyclic call graph rules from `v0.36.0` remain unchanged.

Typed HIR and target-neutral Core reuse the resolved `call` node and recursively validate every
nested expression. The evaluator retains isolated copied call frames, and the Go backend continues
to emit only validated Core. The Docker observability consumer proves an Optional record match arm
calling a text-normalization helper. Compiler, semantic projection, and Application IR schema
shapes remain unchanged; only their language-contract metadata advances to `v0.37.0`. Cross-class
or cross-module calls, private targets, overloads, recursion, blocks, locals, branches, loops,
effects, entrypoints, and target-specific semantics remain excluded.

### PipeLang v0.38.0: bounded conditional expression

One `condition ? whenTrue : whenFalse` expression is admitted per method. The condition must be
`bool`; both branches are statically checked and have exactly the same admitted type; evaluation
executes only the selected branch. Operands may use existing eager pure expressions, including
resolved v0.37.0 same-class calls, but may not contain nested conditionals, match, or propagation.

Typed HIR and target-neutral Core carry the condition and both branches explicitly. Core validates
the bound, placement, operand exclusions, and exact types; the evaluator selects one branch; the
Core-only Go backend emits the same lazy choice without inference. Compiler, semantic projection,
and Application IR schema shapes remain unchanged; only their language-contract metadata advances
to `v0.38.0`. `if` statements, blocks, locals, mutation, conversions, pattern guards, effects,
actions, runtime behavior, and target-specific semantics remain excluded.

### PipeLang v0.39.0: immutable local plus terminal return

One public pure method may replace its expression body with exactly
`{ T name = initializer; return expression; }`. The local type is explicit and must exactly match
the initializer. Initialization is eager and occurs once before the local enters scope. The local
is immutable, cannot shadow a field or parameter, and has no public semantic identity. The return
expression may use the local, parameters, and existing admitted eager pure expressions, including
checked arithmetic whose explicit `Result` type is carried by the local declaration. Contextual
propagation remains confined to its established complete-method carrier shape and is not admitted
inside the local initializer or return.

Typed HIR and target-neutral Core represent the binding, initializer, and return with an explicit
`immutable_local` node. Core validates the one-node top-level shape, lexical scope, and exact types;
the evaluator and Core-only Go backend preserve the same single-evaluation behavior. Compiler,
semantic projection, and Application IR schema shapes remain unchanged; only their
language-contract metadata advances to `v0.39.0`. Type inference, multiple locals, reassignment,
shadowing, propagation, early return, statement branches, nested blocks, loops, effects, actions, runtime
behavior, and target-specific semantics remain excluded.

### PipeLang v0.40.0: ordered immutable locals

One public pure method block may contain one or more source-ordered explicitly typed immutable
locals followed by one terminal return:
`{ T1 first = expression1; T2 second = expression2; ... return expression; }`. Each initializer
must exactly match its declared type and evaluates eagerly exactly once. A binding enters scope
only after its initializer, so later initializers may use earlier locals while self-reference,
forward reference, duplicate names, and field/parameter shadowing fail. The terminal expression
must exactly match the method return type. Contextual propagation remains excluded throughout the
block.

Typed HIR and target-neutral Core encode the source sequence as right-nested `immutable_local`
nodes with contiguous positions. Core independently rejects locals outside the one top-level
sequence, validates exact types and parameter/prior-local shadowing, and preserves v0.39.0's single-local form.
The evaluator and Core-only Go backend evaluate and copy locals once in source order. Compiler,
semantic projection, and Application IR schema identities and shapes remain unchanged; only their
language-contract metadata advances to `v0.40.0`. Inference, reassignment, propagation, early
returns, statement branches, nested blocks, loops, effects, actions, runtime behavior, and
target-specific semantics remain excluded.

### PipeLang v0.41.0: block-scoped bounded propagation

One public pure method block may use exactly one declaration of the form
`T name = propagate(carrier);` as its first immutable local. `carrier` must be the method's sole
direct parameter, and its carrier type must exactly equal the method return type. The declaration
type `T` must exactly equal the carried payload type. Presence or success copies the validated
payload into the local and continues through the remaining ordered locals and terminal return;
absence or failure immediately returns the identical canonical carrier.

The admitted carriers are the existing bounded propagation matrix only: `Optional<T>` for an
already admitted primitive or record `T`, `Result<List<R>, string>`, and
`Result<string, string>`. Typed HIR and target-neutral Core reuse the existing `propagate` and
right-nested `immutable_local` nodes; Core independently validates the first-local placement,
single occurrence, direct-parameter identity, exact carrier and payload types, and bounded carrier
shape. The evaluator and Core-only Go backend preserve the same short-circuit and copied-value
semantics. Compiler, semantic projection, and Application IR schema identities and shapes remain
unchanged; only their language-contract metadata advances to `v0.41.0`.

No second or nested propagation, computed operand, propagation from a prior local, arbitrary
`Result` (including checked-arithmetic results), inference, reassignment, early return, statement
branch, nested block, loop, effect, action, runtime behavior, target behavior, adapter, UI, or
deployment behavior enters by implication.

### PipeLang v0.42.0: prior-local helper propagation

One public pure method block may use exactly
`C carrier = Helper(input); T value = propagate(carrier);` as its first two immutable locals. The
method has one direct parameter `input`; `Helper` resolves to one uniquely named public pure method
on the same class, takes exactly the input type, and returns bounded carrier `C`. The call argument
is the direct parameter, `C` exactly equals the enclosing method return type, and `T` exactly
equals the carrier payload. The helper evaluates once. Presence or success copies the validated
payload into `value` and continues through later ordered locals and the terminal return; absence or
failure returns the identical canonical helper carrier immediately.

The carrier matrix remains the v0.41.0 Optional primitive/record,
`Result<List<R>, string>`, and `Result<string, string>` matrix. Typed HIR and target-neutral Core
reuse `call`, right-nested `immutable_local`, and `propagate`. Core independently validates exact
positions, the sole direct helper argument, resolved same-class callable signature and acyclic call
graph, the direct immediately preceding carrier-local operand, the single propagation occurrence,
and exact carrier/payload types. The evaluator and Core-only Go backend preserve once-only helper
evaluation, short-circuit, canonical carrier return, and copied values. Compiler, semantic
projection, and Application IR schema identities and shapes remain unchanged; only their
language-contract metadata advances to `v0.42.0`. The exact 45-source legacy lane remains frozen.

Direct `propagate(Helper(...))`, multiple/nested propagation, extra or computed helper arguments,
propagation from another local, arbitrary Results including checked arithmetic, inference,
reassignment, early returns, statement branches, nested blocks, loops, effects, actions, runtime,
target, adapter, UI, and deployment behavior remain excluded.

### PipeLang v0.43.0: helper-result matching composition

One public pure method may return the result of exactly one top-level
`match(Helper(input)) { ok(value) => whenOk, err(error) => whenErr }` expression. The method has
one direct `string` parameter and returns `string`. `Helper` resolves to one uniquely named public
pure method on the same class, takes that direct parameter as its sole argument, and returns
`Result<string, string>`. The helper evaluates once; the complete carrier is validated before its
tag is selected, the selected payload is copied into its arm binding, and only the selected arm
evaluates. Arms must appear exactly as source-ordered `ok(binding)` then `err(binding)` with no
wildcard.

Typed HIR and target-neutral Core reuse the existing `call` and `match` nodes. Core independently
validates the top-level placement, sole direct helper argument, exact same-owner callable
signature, text Result carrier, complete ordered arms, unique bindings, and closed acyclic call
graph. The evaluator and deterministic Core-only Go backend preserve once-only helper evaluation,
complete-carrier validation, copied payloads, and lazy arm selection. The Docker observability
consumer proves `DetailsMessage(string)` composing `ValidateDetails(string)` through the unchanged
`dockpipe.application.v1` schema. Compiler, semantic projection, and Application IR schema
identities and shapes remain unchanged; only language-contract metadata advances to `v0.43.0`.
The exact 45-source legacy lane remains frozen.

Optional, list, or arithmetic Result helper carriers; additional or computed helper arguments;
additional caller parameters; cross-class/module calls; nested matches; reversed arms; wildcard
arms; guards; blocks or locals added by this slice; propagation changes; inference; reassignment;
statement branches; loops; effects; actions; runtime; target; adapter; UI; and deployment behavior
remain excluded.

### PipeLang v0.44.0: general bounded helper-carrier matching

One public pure caller with one or more parameters may return exactly one top-level
`match(Helper(...))`. Every caller parameter is passed directly once in declaration order to one
uniquely resolved public pure same-class helper with the exact parameter signature. The helper
returns admitted `Optional<T>`, `Result<List<R>, string>`, or `Result<string, string>`. Optional
arms are exact source-ordered `some(binding)` then binding-free `none`; Result arms are exact
source-ordered `ok(binding)` then `err(binding)`. Both arm expressions exactly match the declared
caller return type.

The helper evaluates once. The complete carrier is validated, the selected payload is copied into
its arm binding, and only the selected arm evaluates. Typed HIR and target-neutral Core reuse the
existing `call` and `match` nodes; Core independently validates the complete contract. The Docker
observability consumer proves `SelectedNameById(List<ContainerRow>, string)` composing
`FindSelection(List<ContainerRow>, string)` through unchanged `dockpipe.application.v1` identity
and shape. `pipelang.compiler.v1` and `pipelang.semantic.v1` also remain unchanged; only
language-contract metadata advances to `v0.44.0`, and the exact 45-source lane stays frozen.

Arithmetic Results, computed/reordered/omitted/extra arguments, cross-owner calls, overloads,
generics, nested or multiple matches, wildcard or reversed arms, guards, propagation changes, new
blocks, locals or statements, effects, actions, runtimes, targets, adapters, UI, and deployment
behavior remain excluded.

### PipeLang v0.45.0: first-local helper-carrier matching

One public pure method with one or more parameters may use exactly one v0.44-compatible
`match(Helper(...))` as the initializer of its first explicitly typed immutable local. Every caller
parameter is still passed directly once in declaration order to one uniquely resolved public pure
same-class helper with the exact parameter signature. The helper carrier remains closed to admitted
`Optional<T>`, `Result<List<R>, string>`, or `Result<string, string>`, with the exact v0.44 arm
ordering and bindings. Both arm expressions have the exact declared local type.

The helper evaluates once, the complete carrier is validated, only the selected arm evaluates, and
its copied result initializes the first local once. Existing v0.40 ordered immutable locals and the
terminal return may then consume that local. Typed HIR and target-neutral Core reuse the existing
`immutable_local`, `call`, and `match` nodes; Core independently validates the first-local placement,
single match, direct argument positions and types, exact same-owner signature, closed carrier
matrix, canonical arms and bindings, local type, and continuation scope. The evaluator and
deterministic Core-only Go backend preserve the same evaluation order and copied-value semantics.

The Docker observability consumer proves `SelectedNameById(List<ContainerRow>, string)` by matching
`FindSelection(List<ContainerRow>, string)` into a first `string selected` local and then calling
`NormalizeName(selected)`. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language-contract metadata
advances to `v0.45.0`, and the exact 45-source lane remains frozen.

Match in later locals, the terminal return, arguments, or nested positions; multiple matches;
computed/reordered/omitted/extra helper arguments; arithmetic Results; cross-owner calls;
overloads; generics; wildcard or reversed arms; guards; propagation changes; inference;
reassignment; early returns; statement branches; loops; effects; actions; runtimes; targets;
adapters; UI; and deployment behavior remain excluded.

### PipeLang v0.46.0: checked-arithmetic helper matching in the first local

One public pure method with one or more parameters may use exactly one v0.45-compatible first-local
`match(Helper(...))` where the helper returns an already admitted
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`. Every caller parameter remains
the corresponding direct helper argument exactly once in declaration order. The helper is one
uniquely resolved public pure same-class method with the exact caller signature. Arms remain exact
source-ordered `ok(binding)` then `err(binding)`, and both expressions exactly match the declared
local type.

The complete checked-arithmetic Result is validated before selection. The helper evaluates once,
only the selected arm evaluates, and its copied result initializes the first local once. Existing
ordered locals and the terminal return may consume that value. Typed HIR and target-neutral Core
reuse `immutable_local`, `call`, and `match`; Core independently validates placement, occurrence,
direct argument positions/types, exact same-owner signature, the int-or-binary64 arithmetic Result
shape, canonical arms/bindings, local typing, and continuation scope. The evaluator and
deterministic Core-only Go backend preserve the same semantics, including deterministic error-arm
selection for integer overflow and binary64 division by zero.

`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language-contract metadata advances to `v0.46.0`, and the exact 45-source
lane remains frozen. The checked-arithmetic form supplies a compiler-shaped pure consumer while the
existing read-only Application IR fixture proves unchanged projection compatibility.

Checked-arithmetic helper matching as a complete method body, later local, terminal return,
argument, or nested expression; multiple matches; computed/reordered/omitted/extra helper
arguments; arithmetic Result propagation, construction, defaulting, or arbitrary Result widening;
cross-owner calls; overloads; generics; wildcards or reversed arms; guards; inference;
reassignment; early returns; statements; loops; effects; actions; runtimes; targets; adapters; UI;
and deployment behavior remain excluded.

### PipeLang v0.47.0: later-local helper-carrier matching

One public pure method with one or more parameters may use exactly one v0.46-compatible
`match(Helper(...))` as any explicitly typed immutable-local initializer after zero or more ordinary
locals. Every caller parameter remains the corresponding direct helper argument exactly once in
declaration order to one uniquely resolved public pure same-class exact-signature helper. The
closed carrier matrix remains admitted Optional primitive/record, `Result<List<R>, string>`,
`Result<string, string>`, and checked-arithmetic `Result<int, ArithmeticError>` or
`Result<float, ArithmeticError>`, with exact canonical arm order, bindings, and local result type.

Earlier ordinary locals evaluate eagerly once in source order. The helper evaluates once, the full
carrier is validated, only the selected arm evaluates, and its copied result initializes the match
local once. Existing later locals and the terminal return then continue. Typed HIR and
target-neutral Core reuse `immutable_local`, `call`, and `match`; Core independently validates the
placement, single occurrence, direct argument positions/types, exact same-owner signature, closed
carrier matrix, canonical arms/bindings, local typing, and continuation scope. The evaluator and
deterministic Core-only Go backend preserve the same semantics.

The Docker observability consumer proves this placement through `SelectedNameById`: an ordinary
`string fallback` local precedes the Optional helper match, and the selected local can consume it.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language-contract metadata advances to `v0.47.0`, and the exact 45-source
lane remains frozen.

Terminal-return, argument, nested, or multiple matches; top-level checked-arithmetic matching;
computed/reordered/omitted/extra helper arguments; propagation changes; Result construction,
defaulting, or arbitrary widening; cross-owner/private/overloaded/generic helpers; wildcard or
reversed arms; guards; inference; reassignment; statements; effects; actions; runtimes; targets;
adapters; UI; and deployment behavior remain excluded.

### PipeLang v0.48.0: prior-local carrier matching

After zero or more ordinary locals, one public pure method with one or more parameters may declare
an explicitly typed carrier local initialized by `Helper(p1, ..., pn)` and then an immediately
adjacent explicitly typed local initialized by exactly one `match(carrier)`. Every caller parameter
is passed directly once in declaration order to one uniquely resolved public pure same-class helper
with the exact caller signature. The closed carrier matrix remains admitted Optional
primitive/record, `Result<List<R>, string>`, `Result<string, string>`, and checked-arithmetic
`Result<int, ArithmeticError>` or `Result<float, ArithmeticError>`, with exact canonical arm order,
bindings, and matched-local result type.

Earlier ordinary locals evaluate eagerly once in source order. The helper initializes the carrier
local once, the full carrier is validated, only the selected arm evaluates, and its copied result
initializes the adjacent matched local once. Existing later locals and the terminal return then
continue. Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and
`match`; Core independently validates adjacency, one match, the exact carrier reference, direct
argument positions/types, exact same-owner signature, the closed carrier matrix, canonical
arms/bindings, local typing, and continuation scope. The evaluator and deterministic Core-only Go
backend preserve the same semantics.

The Docker observability consumer proves the exact source shape through `SelectedNameById`: a
fallback local is followed by `Optional<ContainerRow> selection = FindSelection(rows, id);` and
`string selected = match(selection) { ... };`. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language-contract metadata advances to `v0.48.0`, and the exact 45-source lane remains frozen.

Non-adjacent, terminal-return, argument, nested, or multiple matches; top-level checked-arithmetic
matching; computed/reordered/omitted/extra helper arguments; propagation changes; Result
construction, defaulting, or arbitrary widening; cross-owner/private/overloaded/generic helpers;
wildcard or reversed arms; guards; inference; reassignment; statements; effects; actions; runtimes;
targets; adapters; UI; and deployment behavior remain excluded. No behavior enters by implication.

### PipeLang v0.49.0: bounded two-carrier matching

One public pure method with one or more parameters may contain exactly two non-overlapping
v0.48-compatible adjacent helper-carrier pairs:

```text
C1 firstCarrier = Helper1(p1, ..., pn);
T1 first = match(firstCarrier) { canonicalArms };

C2 secondCarrier = Helper2(p1, ..., pn);
T2 second = match(secondCarrier) { canonicalArms };

return admittedExpression;
```

Zero or more ordinary explicitly typed immutable locals may appear before, between, or after the
pairs, but no local may split a helper-call carrier local from its immediately adjacent matching
local. Each helper independently receives every caller parameter directly once in declaration
order and resolves uniquely to a public pure same-class method with the exact caller signature.
Each pair independently uses the unchanged v0.48 Optional primitive/record,
`Result<List<R>, string>`, `Result<string, string>`, or checked-arithmetic Result carrier matrix and
its exact canonical source-ordered arms.

All locals, helpers, carriers, and selected locals evaluate eagerly exactly once in source order.
Each complete carrier is validated and only its selected arm evaluates. The second pair and its
arms may reference prior locals, including the first selected local, under existing immutable scope
rules. Matching does not propagate or skip the second pair. Typed HIR and target-neutral Core reuse
`immutable_local`, `call`, `reference`, and `match`; Core independently validates exactly two
matches, both non-overlapping adjacent pairs, carrier references, helper identities/signatures,
direct arguments, carrier types, arm order/bindings, local types, and continuation scope. The
evaluator and deterministic Core-only Go backend preserve the same behavior.

The Docker observability consumer proves `FindSelection(rows, id)` and
`ConfirmSelection(rows, id)` through two adjacent carrier/match pairs before normalization.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language-contract metadata advances to `v0.49.0`, and the exact 45-source
lane remains frozen.

Existing zero-match and one-match methods remain exact. A third match, non-adjacent or overlapping
pairs, terminal-return/argument/nested matching, non-helper or computed carriers, computed/
reordered/omitted/extra helper arguments, propagation changes, Result construction/defaulting or
arbitrary widening, cross-owner/private/overloaded/generic helpers, wildcard or reversed arms,
guards, inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment behavior remain excluded. No behavior enters by implication.

### PipeLang v0.50.0: dependent second-carrier matching

One new two-match form requires exactly four contiguous explicitly typed locals:

```text
C1 firstCarrier = Helper1(p1, ..., pn);
T1 first = match(firstCarrier) { canonicalArms };
C2 secondCarrier = Helper2(first, p1, ..., pn);
T2 second = match(secondCarrier) { canonicalArms };
return admittedExpression;
```

`Helper1` retains the v0.49 exact caller signature. `Helper2` resolves uniquely to a public pure
same-class method whose first parameter exactly matches `T1`, followed by every caller parameter
directly once in declaration order. Ordinary locals may appear only before or after this four-local
stage. Both pairs retain the closed v0.48 carrier matrix and canonical source-ordered arms. All
locals and helpers evaluate eagerly once in source order, every complete carrier is validated, and
only selected arms evaluate.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`; Core
independently validates the four-local stage, local and parameter binding positions, exact helper
ownership/signatures, carrier types, arms, local types, and continuation. The evaluator and
deterministic Core-only Go backend preserve those semantics. Docker observability proves
`ConfirmSelection(selected, rows, id)` after the first selection match. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language-contract metadata advances to `v0.50.0`, and the exact 45-source lane remains frozen.

Existing zero-match, one-match, and v0.49 independent two-pair forms remain exact. Other local
argument arrangements, computed/reordered/repeated arguments, gaps inside the four-local stage,
third/nested/terminal/argument matches, propagation changes, Result construction/defaulting or
arbitrary widening, cross-owner/private/overloaded/generic helpers, wildcards, reversed arms,
guards, inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No behavior enters by implication.

### PipeLang v0.51.0: general dependent carrier chains

The v0.50 dependency rule extends to one contiguous chain of two or more carrier/match pairs:

```text
C1 carrier1 = Helper1(p1, ..., pn);
T1 value1 = match(carrier1) { canonicalArms };
C2 carrier2 = Helper2(value1, p1, ..., pn);
T2 value2 = match(carrier2) { canonicalArms };
...
Ck carrierK = HelperK(valueK-1, p1, ..., pn);
Tk valueK = match(carrierK) { canonicalArms };
return admittedExpression;
```

All `2k` locals are explicitly typed and contiguous; ordinary locals may appear only before or
after. `Helper1` retains the exact caller signature. Each later helper resolves uniquely to a
public pure same-class method receiving the immediately preceding selected local followed by every
caller parameter directly once in declaration order. The closed Optional, bounded Result, and
checked-arithmetic carrier matrix and canonical arm spellings remain unchanged. Every local and
helper evaluates once in source order, every complete carrier is validated, and only each selected
arm evaluates.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`; Core
independently validates the complete chain and immediate-predecessor dependency. The evaluator and
deterministic Core-only Go backend preserve the same semantics. Docker observability proves
`FinalizeSelection(confirmed, rows, id)` as a third stage. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged;
only language-contract metadata advances to `v0.51.0`, and the exact 45-source lane remains frozen.

Existing zero-match, one-match, v0.49 independent two-pair, and v0.50 dependent two-stage forms
remain exact. Gaps, mixed independent/dependent chains, non-immediate dependencies, fan-in,
computed/reordered/repeated/omitted/extra arguments, third matches outside this chain,
nested/terminal/argument matches, propagation changes, arbitrary Result widening, statements,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication.

### PipeLang v0.52.0: cumulative fan-in carrier chains

A method may additionally choose one cumulative chain of at least three contiguous carrier/match
pairs:

```text
C1 carrier1 = Helper1(p1, ..., pn);
T1 value1 = match(carrier1) { canonicalArms };
C2 carrier2 = Helper2(value1, p1, ..., pn);
T2 value2 = match(carrier2) { canonicalArms };
...
Ck carrierK = HelperK(value1, ..., valueK-1, p1, ..., pn);
Tk valueK = match(carrierK) { canonicalArms };
return admittedExpression;
```

Every later helper receives every prior selected local directly once in chain order, followed by
every caller parameter directly once in declaration order. The whole method uses either cumulative
fan-in or the inherited v0.51 immediate-only dependency; stages cannot mix. All `2k` locals remain
explicitly typed and contiguous, and the existing closed carrier matrix and canonical arms remain
unchanged. Every local and helper evaluates once in source order, every complete carrier is
validated, and only each selected arm evaluates.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `match`; Core
independently validates cumulative argument identities and positions. The evaluator and
deterministic Core-only Go backend preserve the same semantics. Docker observability proves
`FinalizeSelectionHistory(selected, confirmed, rows, id)` as a cumulative third stage.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language-contract metadata advances to `v0.52.0`, and the exact 45-source
lane remains frozen.

All inherited forms remain exact. Fewer than three cumulative pairs, partial/reordered/repeated/
omitted/extra prior selections, mixed modes, gaps, non-chain dependencies, computed arguments,
matches outside the chain, nested/terminal/argument matches, propagation changes, arbitrary Result
widening, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No behavior enters by implication.

### PipeLang v0.53.0: multi-parameter helper propagation

A public pure method with at least two parameters may use this exact source form:

```text
C carrier = Helper(p1, ..., pn);
T value = propagate(carrier);
return admittedExpression;
```

The carrier call is the first immutable-local initializer, the propagation local is immediately
adjacent, and the public pure same-class helper receives every caller parameter directly once in
declaration order. `C` is identical to the method return carrier and `T` is its success payload.
The carrier remains limited to Optional primitive/record, `Result<List<R>, string>`, or
`Result<string, string>`. The inherited one-parameter v0.42 form remains exact.

Typed HIR and target-neutral Core reuse `immutable_local`, `call`, `reference`, and `propagate`;
Core independently verifies parameter positions, helper identity/signature, adjacency, carrier,
payload, and once-only propagation. The evaluator and deterministic Core-only Go backend call the
helper once, validate the complete carrier, copy its payload on success, and return the canonical
absent or failure carrier otherwise. Docker observability proves
`ResolveSelection(List<ContainerRow>, string)` calling `FindSelection(rows, id)` without an
Application IR schema change. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language-contract metadata
advances to `v0.53.0`, and the exact 45-source legacy lane remains frozen.

Computed, reordered, repeated, omitted, or extra helper arguments remain rejected, as do a
non-first carrier local, an intervening local, a non-adjacent or additional propagation, mismatched
carrier or payload types, cross-owner/private/overloaded/generic helpers, arbitrary Result widening,
inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment. No behavior enters by implication.

### PipeLang v0.54.0: checked-arithmetic helper propagation

`v0.54.0` adds exactly the existing checked-arithmetic carriers to the v0.53 prior-local helper
propagation form:

```pipe
public Result<int, ArithmeticError> Add(int left, int right) => left + right;

public Result<int, ArithmeticError> Resolve(int left, int right) {
    Result<int, ArithmeticError> carrier = Add(left, right);
    int value = propagate(carrier);
    return value + 0;
}
```

The identical form is admitted for `Result<float, ArithmeticError>` with a `float` payload and an
already admitted checked binary64 division expression as the terminal return. The helper call is
the first typed local, propagation is the immediately adjacent second local, and every caller
parameter is passed directly once in declaration order to one uniquely resolved public pure
same-class helper. The helper carrier exactly equals the caller return type and the propagated local
exactly equals its success payload.

The helper evaluates once and the complete arithmetic Result is validated. Canonical success is
copied into the payload local; canonical `overflow` or `division_by_zero` returns immediately and
the continuation is not evaluated. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language-contract metadata
advances to `v0.54.0`. A compiler-cursor fixture proves `Advance(cursor, width)` can call
`AddOffset(cursor, width)` and propagate overflow unchanged, providing a concrete self-hosting
consumer. The exact 45-source legacy lane remains frozen.

Direct-parameter arithmetic propagation, `propagate(Helper(...))`, later or split pairs, computed,
reordered, repeated, omitted, or extra helper arguments, additional propagation, arbitrary Result
widening, inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI,
and deployment remain excluded. No behavior enters by implication.

### PipeLang v0.55.0: direct-parameter checked propagation

`v0.55.0` adds the existing checked-arithmetic Results to the inherited first-local direct-carrier
propagation form:

```pipe
public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) {
    int value = propagate(carrier);
    return value + 0;
}
```

The identical form is admitted for `Result<float, ArithmeticError>` with a `float` payload and an
already admitted checked binary64 division continuation. The carrier is the method's sole direct
parameter and exactly equals its return type. Propagation is the first typed local, whose type
exactly equals the success payload. The complete carrier is validated before branching; canonical
success is copied, while canonical `overflow` or `division_by_zero` returns immediately without
evaluating the continuation.

Typed HIR and target-neutral Core reuse `immutable_local`, `reference`, `propagate`, and checked
arithmetic nodes. The evaluator and deterministic Core-only Go preserve the same validation,
copying, and short-circuit semantics. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language-contract metadata
advances to `v0.55.0`. A compiler-cursor fixture proves a checked cursor Result supplied by a
caller can be consumed without losing overflow. The exact 45-source legacy lane remains frozen.

Additional parameters, helper or computed propagation operands, later or split placement,
additional propagation, mismatched carrier/payload types, arbitrary Result widening, inference,
reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. The v0.54 helper form and every earlier source contract remain exact. No behavior enters
by implication.

### PipeLang v0.56.0: multi-parameter direct checked propagation

`v0.56.0` adds one exact two-parameter form to direct checked propagation:

```pipe
public Result<int, ArithmeticError> Advance(
    Result<int, ArithmeticError> carrier,
    int operand
) {
    int value = propagate(carrier);
    return value + operand;
}
```

The integer operator may be `+`, `-`, or `*`. The identical `float` form admits only `/`. The
arithmetic Result is the first parameter and exactly equals the method return type; the second and
only other parameter exactly equals its success payload. Propagation remains the first typed local.
The continuation uses that local as its left operand and the second direct parameter as its right
operand. The complete carrier is validated before branching; canonical incoming failure returns
without evaluating the continuation, while the continuation retains checked overflow or
division-by-zero behavior.

Typed HIR and target-neutral Core reuse `immutable_local`, `reference`, `propagate`, and checked
arithmetic nodes. The evaluator and deterministic Core-only Go preserve the same validation,
copying, operand order, and short-circuit semantics. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language-contract metadata advances to `v0.56.0`. A compiler-cursor fixture proves checked cursor
plus width advancement. The exact 45-source legacy lane remains frozen.

The inherited v0.55 sole-carrier and v0.54 helper forms remain exact. A third parameter, reordered
carrier/operand, mismatched operand type, reversed/repeated/literal/computed operands, unary
negation as the new two-parameter form, helper or computed propagation operands, later/split or
additional propagation, arbitrary Result widening, inference, reassignment, statements, effects,
actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior enters by
implication.

### PipeLang v0.57.0: two-stage checked propagation

`v0.57.0` adds one exact two-stage form:

```pipe
public Result<int, ArithmeticError> AdvanceTwice(
    Result<int, ArithmeticError> carrier,
    int first,
    int second
) {
    int value = propagate(carrier);
    Result<int, ArithmeticError> nextCarrier = value + first;
    int next = propagate(nextCarrier);
    return next + second;
}
```

Each integer checked stage independently admits `+`, `-`, or `*`; the identical `float` form uses
`/` at both stages. The incoming Result is the first parameter and method return, and the second and
third parameters exactly equal its payload. The complete incoming carrier is validated and
propagated, the first checked operation initializes one explicit Result local, that complete carrier
is validated and propagated, and only then does the terminal checked operation evaluate. Incoming,
intermediate, and terminal failures retain canonical `overflow` or `division_by_zero` behavior.

Typed HIR and target-neutral Core reuse existing immutable-local, reference, propagation, and
checked-arithmetic nodes. The evaluator and deterministic Core-only Go preserve exact evaluation
order, copied success values, validation, and short-circuiting. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language-contract metadata advances to `v0.57.0`. A compiler-cursor fixture proves two checked
width advances. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.56 forms remain exact. A fourth parameter, reordered/mismatched parameters,
reversed/repeated/literal/computed operands, missing/additional stages, direct propagation of a
computed expression, helper propagation, arbitrary Results, inference, reassignment, statements,
effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded. No behavior
enters by implication.

### PipeLang v0.58.0: generalized checked-propagation chains

`v0.58.0` generalizes the v0.57 shape to a contiguous chain of two or more checked stages:

```pipe
public Result<int, ArithmeticError> AdvanceThree(
    Result<int, ArithmeticError> carrier,
    int first,
    int second,
    int third
) {
    int value = propagate(carrier);
    Result<int, ArithmeticError> secondCarrier = value + first;
    int secondValue = propagate(secondCarrier);
    Result<int, ArithmeticError> thirdCarrier = secondValue - second;
    int thirdValue = propagate(thirdCarrier);
    return thirdValue * third;
}
```

For `K >= 2` stages, the method takes the arithmetic Result first and exactly `K` matching payload
parameters. Every non-terminal checked operation must initialize an explicit Result local and the
immediately following local must propagate that carrier; the terminal checked operation uses the
last propagated payload and final parameter. Integer stages independently admit `+`, `-`, or `*`;
the identical `float` form uses `/` at every stage. Each complete carrier is validated once,
success is copied, and incoming or intermediate failure returns before any later operation.

Typed HIR and target-neutral Core reuse existing nodes and validate the full parameter/local chain
independently. The evaluator and deterministic Core-only Go preserve exact source order and
canonical overflow or division-by-zero. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language-contract metadata
advances to `v0.58.0`. A three-stage compiler-cursor fixture and metadata-only Application IR
consumer prove the boundary. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.57 forms remain exact. Fewer than two stages, missing/additional chain
locals, ordinary-local gaps, reordered/mismatched parameters, reversed/repeated/literal/computed
operands, direct computed propagation, helper propagation, arbitrary Results, inference,
reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. No behavior enters by implication.

### PipeLang v0.59.0: bounded cross-payload Result propagation

`v0.59.0` adds one exact target-shaping propagation form:

```pipe
public Result<List<Token>, string> Parse(Result<string, string> scanned) {
    string source = propagate(scanned);
    return ParseTokens(source);
}
```

The public pure method has exactly one direct `Result<T, string>` parameter and returns
`Result<U, string>`, where `T` and `U` are distinct and each is either `string` or `List<R>` for
an existing public primitive-field record `R`. Its first and only local propagates the direct
parameter into an explicitly typed `T` value. Its terminal expression calls one resolved public
pure same-class helper exactly once with that direct local; the helper signature is exactly
`T -> Result<U, string>`.

The complete incoming carrier is validated once. Success copies `T` and invokes the helper once;
the helper's complete target carrier is then validated and copied normally. Failure skips the
helper and constructs the canonical target-shaped `Result<U, string>` failure, preserving the
validated copied error text while using the canonical zero `U` payload (empty text or a nil list).
Typed HIR and target-neutral Core reuse existing immutable-local, propagation, reference, and call
nodes. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and
shapes remain unchanged; only language metadata advances to `v0.59.0`. The exact 45-source legacy
lane remains frozen.

The inherited v0.54-v0.58 forms remain exact. Same-payload propagation gains no new spelling.
Arbitrary failure types, Optional or arithmetic carriers, extra parameters or locals, additional
propagation, computed carriers, `propagate(Helper(...))`, helper propagation, helper arguments other
than the direct propagated local, private/cross-class/mismatched/overloaded/generic helpers,
inference, reassignment, statements, effects, actions, runtimes, targets, adapters, UI, and
deployment remain excluded. No behavior enters by implication.

### PipeLang v0.60.0: two-stage bounded cross-payload Result propagation

`v0.60.0` adds one exact two-stage target-shaping propagation form:

```pipe
public Result<List<SyntaxNode>, string> Compile(Result<string, string> scanned) {
    string source = propagate(scanned);
    Result<List<Token>, string> tokenized = BuildTokens(source);
    List<Token> tokens = propagate(tokenized);
    return BuildSyntax(tokens);
}
```

The public pure method has exactly one direct `Result<T, string>` parameter and returns
`Result<V, string>`. Its first local propagates that carrier to `T`; its second local stores one
exact public pure same-class `T -> Result<U, string>` helper call; its third local directly
propagates that explicit carrier to `U`; and its terminal expression calls one exact public pure
same-class `U -> Result<V, string>` helper. `T`, `U`, and `V` are each `string` or `List<R>` for an
existing public primitive-field record. Adjacent payloads must differ; the source and target may be
equal.

Every complete carrier is validated once. Each successful text or list payload is copied before
the next helper receives it. Incoming failure skips both helpers; intermediate failure skips the
terminal helper. Either failure is reshaped to the canonical target `Result<V, string>`, preserving
copied validated error text and the target payload's canonical zero. Typed HIR and target-neutral
Core reuse existing immutable-local, propagation, reference, and call nodes.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language metadata advances to `v0.60.0`. The exact 45-source legacy lane
remains frozen.

The inherited v0.54-v0.59 forms remain exact. General `K`-stage Result chains, same-payload adjacent
stages, extra parameters or locals, arbitrary failure types, computed carriers,
`propagate(Helper(...))`, helper propagation, helper arguments other than the direct preceding
payload local, private/cross-class/mismatched/overloaded/generic helpers, inference, reassignment,
statements, branches, loops, effects, actions, runtimes, targets, adapters, UI, and deployment
remain excluded. No behavior enters by implication.

### PipeLang v0.61.0: generalized bounded cross-payload Result propagation chains

`v0.61.0` generalizes the v0.60 form to `K >= 2` adjacent target-shaping stages:

```pipe
public Result<string, string> Compile(Result<string, string> scanned) {
    string source = propagate(scanned);
    Result<List<Token>, string> tokenized = BuildTokens(source);
    List<Token> tokens = propagate(tokenized);
    Result<List<SyntaxNode>, string> parsed = BuildSyntax(tokens);
    List<SyntaxNode> syntax = propagate(parsed);
    return EmitSource(syntax);
}
```

The public pure method has exactly one direct `Result<T0, string>` parameter and returns
`Result<TK, string>`. Its first local directly propagates the parameter. Every non-terminal helper
stage is an adjacent pair containing an explicit `Result<Ti, string>` helper-call local followed
immediately by a direct propagation local of type `Ti`; the terminal expression calls the final
helper with the immediately preceding payload local. Every helper is public, pure, same-class, and
has the exact adjacent `Ti-1 -> Result<Ti, string>` signature. Every payload is `string` or
`List<R>` for an existing public primitive-field record, and adjacent payloads differ. Non-adjacent
payloads may match.

Every complete carrier is validated and copied once. Incoming or intermediate failure skips every
later helper and is reshaped to the canonical final `Result<TK, string>` failure, preserving copied
validated error text and the final payload's canonical zero. Typed HIR and target-neutral Core
reuse existing immutable-local, propagation, reference, and call nodes. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language metadata advances to `v0.61.0`. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.60 forms remain exact. Fewer than two stages in the generalized form,
same-payload adjacent stages, missing/additional/gapped locals, extra parameters, arbitrary failure
types, computed carriers, `propagate(Helper(...))`, helper propagation, helper arguments other than
the direct preceding payload local, private/cross-class/mismatched/overloaded/generic helpers,
inference, reassignment, statements, branches, loops, effects, actions, runtimes, targets, adapters,
UI, and deployment remain excluded. No behavior enters by implication.

### PipeLang v0.62.0: contextual bounded cross-payload Result propagation chains

`v0.62.0` adds exactly one direct `string` context parameter to the v0.61 generalized chain:

```pipe
public Result<string, string> Compile(Result<string, string> scanned, string context) {
    string source = propagate(scanned);
    Result<List<Token>, string> tokenized = BuildTokens(source, context);
    List<Token> tokens = propagate(tokenized);
    Result<List<SyntaxNode>, string> parsed = BuildSyntax(tokens, context);
    List<SyntaxNode> syntax = propagate(parsed);
    return EmitSource(syntax, context);
}
```

Every helper receives the immediately preceding payload local first and the same direct `context`
parameter second. Its exact signature is `(Ti-1, string) -> Result<Ti, string>`. The chain remains
public, pure, same-class, contiguous, and at least two stages long. Every non-terminal Result is an
explicit local immediately followed by its direct propagation local; payloads remain `string` or
`List<R>` for an existing public primitive-field record; adjacent payloads differ; and the shared
failure type remains `string`.

Every invoked helper receives the validated context exactly once. Every complete carrier is
validated and copied once, and incoming or intermediate failure skips every later helper while
producing the canonical final-target-shaped failure. Typed HIR and target-neutral Core reuse the
existing immutable-local, reference, propagation, and call nodes. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language metadata advances to `v0.62.0`. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.61 forms remain exact. Missing, reordered, repeated, computed, non-string,
stage-specific, or additional context arguments; a third caller parameter; same-payload adjacent
stages; gapped or additional locals; computed carriers; `propagate(Helper(...))`; helper
propagation; arbitrary error types; Optional/arithmetic carriers; private/cross-class/mismatched/
overloaded/generic helpers; inference; reassignment; statements; branches; loops; effects; actions;
runtimes; targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

### PipeLang v0.63.0: one-stage contextual bounded cross-payload Result propagation

`v0.63.0` closes the one-stage contextual boundary while retaining every v0.62 chain rule:

```pipe
public Result<List<Token>, string> Compile(Result<string, string> scanned, string context) {
    string source = propagate(scanned);
    return BuildTokens(source, context);
}
```

The caller has exactly one direct bounded Result carrier followed by one direct `string` context.
The carrier is propagated into the first and only typed local. One resolved public pure same-class
helper receives that direct local followed by the unchanged context and has exact signature
`(T, string) -> Result<U, string>`. `T` and `U` are distinct and remain text or lists of existing
public primitive-field records; the shared failure type remains `string`.

The incoming carrier is validated and copied once. Failure skips the helper and becomes the
canonical target-shaped Result with copied validated error text. Success passes validated context
once and validates the helper Result once. Typed HIR and target-neutral Core reuse existing nodes;
Core independently verifies the complete shape. `pipelang.compiler.v1`, `pipelang.semantic.v1`,
and `dockpipe.application.v1` identities and shapes remain unchanged; only language metadata
advances to `v0.63.0`. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.62 forms remain exact. Same-payload flow; a missing, reordered, repeated,
computed, non-string, or additional context; a third caller parameter; extra locals or propagation;
computed carriers; `propagate(Helper(...))`; helper propagation; arbitrary error types;
Optional/arithmetic carriers; private/cross-class/mismatched/overloaded/generic helpers; inference;
reassignment; statements; branches; loops; effects; actions; runtimes; targets; adapters; UI; and
deployment remain excluded. No behavior enters by implication.

### PipeLang v0.64.0: one-stage contextual same-payload Result propagation

`v0.64.0` adds the payload-preserving counterpart to the exact v0.63 one-stage contextual form:

```pipe
public Result<string, string> Normalize(Result<string, string> input, string context) {
    string value = propagate(input);
    return NormalizeValue(value, context);
}
```

The caller still has exactly one direct bounded Result carrier followed by one direct `string`
context. The carrier is propagated into the first and only typed local. One resolved public pure
same-class helper receives that direct local followed by the unchanged context and has exact
signature `(T, string) -> Result<T, string>`. `T` is exactly `string` or `List<R>` for an existing
public primitive-field record, and the shared failure type remains `string`.

The incoming carrier is validated and copied once. Failure skips the helper and returns the
canonical same-shaped Result with copied validated error text. Success passes validated context
once and validates the helper Result once. Typed HIR and target-neutral Core reuse existing nodes;
Core independently verifies the complete shape. `pipelang.compiler.v1`, `pipelang.semantic.v1`,
and `dockpipe.application.v1` identities and shapes remain unchanged; only language metadata
advances to `v0.64.0`. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.63 forms remain exact. Same-payload contextual chains with two or more
stages; a missing, reordered, repeated, computed, non-string, or additional context; a third caller
parameter; extra locals or propagation; computed carriers; `propagate(Helper(...))`; helper
propagation; arbitrary error types; Optional/arithmetic carriers; private/cross-class/mismatched/
overloaded/generic helpers; inference; reassignment; statements; branches; loops; effects; actions;
runtimes; targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

### PipeLang v0.65.0: exact two-stage contextual bounded Result propagation

`v0.65.0` extends the exact v0.62 two-stage contextual form so either or both adjacent payload
transitions may preserve their payload type:

```pipe
public Result<string, string> Normalize(Result<string, string> input, string context) {
    string first = propagate(input);
    Result<string, string> checked = Check(first, context);
    string second = propagate(checked);
    return Finish(second, context);
}
```

The caller still has exactly one direct bounded Result carrier followed by one direct `string`
context. It has exactly two helper stages: the direct carrier propagation, an immediately following
helper-Result local, its immediately following propagation local, and a terminal helper call.
`T0`, `T1`, and `T2` are each exactly `string` or `List<R>` for an existing public primitive-field
record. Both resolved helpers are public, pure, same-class, and exact
`(Ti, string) -> Result<Ti+1, string>` methods. Every reached helper receives the unchanged direct
context once.

Each carrier is validated and copied once. Incoming or first-helper failure skips all later helpers
and returns a canonical final-shaped Result with copied validated error text. Typed HIR and
target-neutral Core reuse existing nodes; Core independently verifies the complete shape.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language metadata advances to `v0.65.0`. The exact 45-source legacy lane
remains frozen.

The inherited v0.54-v0.64 forms remain exact. Same-payload transitions in contextual chains with
three or more stages; a missing, reordered, repeated, computed, non-string, or additional context;
a third caller parameter; extra or gapped locals; additional propagation; computed carriers;
`propagate(Helper(...))`; arbitrary error types; Optional/arithmetic carriers; private,
cross-class, mismatched, overloaded, or generic helpers; inference; reassignment; statements;
branches; loops; effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded.
No behavior enters by implication.

### PipeLang v0.66.0: generalized contextual bounded Result propagation

`v0.66.0` generalizes the contextual `K >= 2` chain so any adjacent payload transition may preserve
or change its payload type:

```pipe
public Result<List<SyntaxNode>, string> Compile(Result<string, string> input, string context) {
    string source = propagate(input);
    Result<string, string> normalizedCarrier = Normalize(source, context);
    string normalized = propagate(normalizedCarrier);
    Result<List<Token>, string> tokenCarrier = Tokenize(normalized, context);
    List<Token> tokens = propagate(tokenCarrier);
    Result<List<Token>, string> preservedCarrier = PreserveTokens(tokens, context);
    List<Token> preserved = propagate(preservedCarrier);
    return Parse(preserved, context);
}
```

The caller still has exactly one direct bounded Result carrier followed by one direct `string`
context. The chain has at least two helper stages. Every non-terminal helper Result is stored in an
explicit local immediately followed by its direct propagation local, and the final stage is one
terminal helper call. Every payload is exactly `string` or `List<R>` for an existing public
primitive-field record. Every resolved helper is public, pure, same-class, and exact
`(Ti, string) -> Result<Ti+1, string>`. Every reached helper receives the immediately preceding
payload and unchanged direct context once.

Every carrier is validated and copied once. Incoming or intermediate failure skips all later
helpers and returns a canonical final-shaped Result with copied validated error text. Typed HIR and
target-neutral Core reuse existing nodes; Core independently verifies arbitrary admitted chain
length and the complete local, type, identity, and helper shape. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes remain unchanged; only
language metadata advances to `v0.66.0`. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.65 forms remain exact. Fewer than two contextual helper stages; a missing,
reordered, repeated, computed, non-string, stage-specific, or additional context; a third caller
parameter; missing, additional, or gapped locals; computed carriers; `propagate(Helper(...))`;
arbitrary error types; Optional/arithmetic carriers; private, cross-class, mismatched, overloaded,
or generic helpers; inference; reassignment; statements; branches; loops; effects; actions;
runtimes; targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

### PipeLang v0.67.0: generalized shared-context Result propagation

`v0.67.0` generalizes the v0.66 contextual `K >= 2` chain to one or more direct `string` context
parameters:

```pipe
public Result<List<SyntaxNode>, string> Compile(
    Result<string, string> input,
    string phase,
    string scope) {
    string source = propagate(input);
    Result<string, string> normalizedCarrier = Normalize(source, phase, scope);
    string normalized = propagate(normalizedCarrier);
    Result<List<Token>, string> tokenCarrier = Tokenize(normalized, phase, scope);
    List<Token> tokens = propagate(tokenCarrier);
    return Parse(tokens, phase, scope);
}
```

The caller has one direct bounded Result carrier followed by `N >= 1` direct `string` contexts. The
chain still has `K >= 2` helper stages. Every helper receives the immediately preceding payload
first, then every unchanged context exactly once in caller declaration order, and has the exact
`(Ti, string...) -> Result<Ti+1, string>` signature. Every payload remains `string` or `List<R>` for
an existing public primitive-field record. Every non-terminal helper Result remains an explicit
local immediately followed by direct propagation, and the final stage remains a terminal helper
call.

Typed HIR and target-neutral Core reuse existing nodes and preserve parameter positions. Core
independently validates the complete ordered context vector on every stage. Evaluation and
deterministic Core-only Go validate and copy every reached carrier once, pass every validated
context unchanged to each reached helper, and reshape failures into the canonical final Result
before later helpers can run. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` identities and shapes remain unchanged; only language metadata advances
to `v0.67.0`. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.66 forms remain exact. A contextual chain with no context; a one-stage chain
with multiple contexts; missing, reordered, repeated, computed, non-string, or stage-specific
contexts; missing, additional, or gapped locals; computed carriers; `propagate(Helper(...))`;
arbitrary failure types; Optional/arithmetic carriers; private, cross-class, mismatched, overloaded,
or generic helpers; inference; reassignment; statements; branches; loops; effects; actions;
runtimes; targets; adapters; UI; and deployment remain excluded. No behavior enters by implication.

### PipeLang v0.68.0: generalized one-stage shared-context Result propagation

`v0.68.0` generalizes the v0.64 one-stage contextual form to one or more direct `string` context
parameters:

```pipe
public Result<List<Token>, string> Tokenize(
    Result<string, string> input,
    string phase,
    string scope) {
    string source = propagate(input);
    return ScanTokens(source, phase, scope);
}
```

The caller has one direct bounded Result carrier followed by `N >= 1` direct string contexts. Its
first and only typed local directly propagates the carrier, and its terminal helper receives that
payload followed by every unchanged context exactly once in caller declaration order. Source and
target payloads may match or differ and remain `string` or `List<R>` for an existing public
primitive-field record. The helper remains resolved, public, pure, same-class, and exact
`(T0, string...) -> Result<T1, string>`.

Typed HIR and target-neutral Core reuse existing nodes and preserve parameter positions. Core
independently validates the complete ordered context vector, single propagation, exact helper
signature, and bounded Result shape. Evaluation and deterministic Core-only Go validate direct
inputs, copy the reached carrier once, pass every context unchanged, and return canonical
target-shaped failure without invoking the helper when the incoming carrier fails.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` identities and shapes
remain unchanged; only language metadata advances to `v0.68.0`. The exact 45-source legacy lane
remains frozen.

The inherited v0.54-v0.67 forms remain exact. A contextual form with no context; missing,
reordered, repeated, computed, non-string, or stage-specific contexts; additional or gapped locals;
computed carriers; `propagate(Helper(...))`; arbitrary failure types; Optional/arithmetic carriers;
private, cross-class, mismatched, overloaded, or generic helpers; inference; reassignment;
statements; branches; loops; effects; actions; runtimes; targets; adapters; UI; and deployment remain
excluded. No behavior enters by implication.

### PipeLang v0.69.0: terminal statement-level `if/else`

`v0.69.0` admits one terminal conditional after one or more ordered immutable locals:

```pipe
public string Select(string raw, bool normalize) {
    string cleaned = trim(raw);
    if (normalize) { return cleaned; }
    else { return raw; }
}
```

The condition is `bool`, both branches have the declared method return type, and all locals,
condition values, and branch values remain already-admitted eager pure expressions or calls. At
most one preceding local initializer may use the inherited bounded conditional expression. Typed
HIR and target-neutral Core reuse the existing conditional representation with an explicit
terminal-statement marker. Core independently validates the required preceding local, unique
terminal placement, boolean condition, branch types, and absence of propagation or matching in the
method. Evaluation and deterministic Core-only Go evaluate only the selected branch.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain unchanged; only
language metadata advances to `v0.69.0`. The exact 45-source legacy lane remains frozen.

The inherited v0.54-v0.68 forms remain exact. Zero-local terminal conditionals, branch locals,
nested branches, missing `else`, fallthrough or returns elsewhere, propagation or matching in the
method, assignment, reassignment, shadowing, inference, loops, effects, actions, runtimes, targets,
adapters, UI, and deployment remain excluded. No behavior enters by implication.

### PipeLang v0.70.0: lexical terminal-branch locals

`v0.70.0` preserves the v0.69 terminal `if/else` shape and permits either branch to declare at most
one explicitly typed immutable local immediately before its return:

```pipe
public string Select(string raw, bool normalize) {
    string cleaned = raw;
    if (normalize) {
        string selected = trim(cleaned);
        return selected;
    } else {
        return raw;
    }
}
```

At least one top-level ordered immutable local still precedes the terminal branch. A branch may
retain the direct-return form or use exactly one local; both branches may use locals, and equal
names in opposing branches denote independent lexical bindings. A branch local is initialized only
when its branch is selected and cannot be referenced by the condition, the opposing branch, or
outside the terminal conditional. The condition remains `bool`, all explicit local and method
types must match their values, and existing eager pure expressions and calls remain the only
admitted computations. HIR and Core reuse `immutable_local` inside the existing terminal
`conditional`; the evaluator and deterministic Core-only Go preserve selected-branch evaluation.

Nested branches, multiple locals in one branch, escaping bindings, propagation, matching,
assignment, fallthrough, other early returns, loops, effects, inference, actions, runtimes,
targets, adapters, UI, and deployment remain excluded. `pipelang.compiler.v1`,
`pipelang.semantic.v1`, and `dockpipe.application.v1` remain unchanged; only language metadata
advances to `v0.70.0`. The exact 45-source legacy lane remains frozen.

### PipeLang v0.71.0: two-local terminal-branch sequences

`v0.71.0` widens only the v0.70 per-branch local cap from one to two:

```pipe
public string Select(string raw, bool normalize) {
    string cleaned = raw;
    if (normalize) {
        string prepared = trim(cleaned);
        string selected = Normalize(prepared);
        return selected;
    } else {
        return raw;
    }
}
```

Either branch may contain zero, one, or two explicitly typed ordered immutable locals immediately
before its return. A second local may reference the first local in that branch. Opposing branches
remain independent lexical scopes and may reuse names and binding positions. Branch initializers
run in source order only when their branch is selected, and bindings cannot escape. One or more
top-level ordered immutable locals remain required.

A third branch local, nested branches, zero top-level locals, propagation, matching, assignment,
fallthrough, other early returns, loops, effects, inference, actions, runtimes, targets, adapters,
UI, and deployment remain excluded. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` remain unchanged; only language metadata advances to `v0.71.0`. The
exact 45-source legacy lane remains frozen.

### PipeLang v0.72.0: general terminal-branch local sequences

`v0.72.0` removes only the v0.71 per-branch local count ceiling:

```pipe
public string Select(string raw, bool normalize) {
    string cleaned = raw;
    if (normalize) {
        string first = trim(cleaned);
        string second = Normalize(first);
        string third = second;
        return third;
    } else {
        return raw;
    }
}
```

Either branch may contain any finite source-ordered sequence of explicitly typed immutable locals
immediately before its return. Each local enters scope only after its initializer, so later locals
may reference earlier locals in the same branch. Opposing branches remain independent lexical
scopes and may reuse names and canonical binding positions. Branch initializers run in source order
only when their branch is selected, and bindings cannot escape. One or more top-level ordered
immutable locals remain required.

Nested branches, zero top-level locals, propagation, matching, assignment, fallthrough, other early
returns, loops, effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain
excluded. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain
unchanged; only language metadata advances to `v0.72.0`. The exact 45-source legacy lane remains
frozen.

### PipeLang v0.73.0: direct terminal `if/else`

`v0.73.0` removes only the v0.72 top-level-local prerequisite. The existing terminal conditional
may be the complete public pure method body:

```pipe
public string Select(string raw, bool normalize) {
    if (normalize) {
        string cleaned = trim(raw);
        string selected = cleaned;
        return selected;
    } else {
        return raw;
    }
}
```

Either branch may return directly or retain any finite source-ordered sequence of explicitly typed
immutable locals followed by its return. Each local enters scope only after its initializer; later
locals may reference earlier locals in the same branch. Opposing branches remain independent
lexical scopes, may reuse names and canonical binding positions, execute only when selected, and
cannot leak bindings.

Typed HIR and target-neutral Core reuse the existing root `conditional` and nested
`immutable_local` nodes. Nested branches, propagation, matching, assignment, fallthrough, other
early returns, ordinary zero-local blocks, loops, effects, inference, actions, runtimes, targets,
adapters, UI, and deployment remain excluded. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` remain unchanged; only language metadata advances to `v0.73.0`. The exact
45-source legacy lane remains frozen.

### PipeLang v0.74.0: bounded nested terminal `if/else`

`v0.74.0` adds one bounded nested decision to the direct v0.73 terminal form. The complete method
body has no top-level immutable locals. Exactly one outer branch may end in exactly one inner
terminal `if/else` after zero or more source-ordered, explicitly typed immutable locals; the sibling
outer branch retains the v0.73 branch form, and both inner leaves return directly:

```pipe
public string Select(string raw, bool enabled, bool normalize) {
    if (enabled) {
        string cleaned = trim(raw);
        if (normalize) {
            return cleaned;
        } else {
            return raw;
        }
    } else {
        return "disabled";
    }
}
```

Both conditions must be `bool`, and every leaf must have the exact declared return type. Outer-
branch locals enter scope after their initializer and remain visible to the inner condition and
both inner leaves. Evaluation remains source ordered and selected-branch-only at both levels.

Typed HIR and target-neutral Core reuse the existing terminal `conditional` and `immutable_local`
nodes; no node or schema identity changes. Inner locals, nesting in both outer branches, a second
nested decision, third-level nesting, top-level locals for the new topology, propagation, matching,
conditional expressions within the topology, assignment, fallthrough, other early returns, loops,
effects, inference, actions, runtimes, targets, adapters, UI, and deployment remain excluded.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` remain unchanged; only
language metadata advances to `v0.74.0`. The inherited v0.69-v0.73 forms remain exact, and the
45-source legacy lane remains frozen.

### Target-neutral Application IR

`dockpipe.application.v1` is not a language feature or target generator. It consumes the public
`pipelang.semantic.v1` projection, its matching Core program, and explicit stable-identity choices
for read-only snapshot sections, rows, keys, columns, filtering, ordering, selection, details, and logs.
Consumers therefore cannot reparse source or infer language semantics in an adapter.
Filter, order, section Result, selection, details, and logs roles are explicit semantic identities
that must also resolve to contract-matching Core functions.
