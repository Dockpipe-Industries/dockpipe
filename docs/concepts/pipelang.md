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
### Target-neutral Application IR

`dockpipe.application.v1` is not a language feature or target generator. It consumes the public
`pipelang.semantic.v1` projection, its matching Core program, and explicit stable-identity choices
for read-only snapshot sections, rows, keys, columns, filtering, ordering, selection, details, and logs.
Consumers therefore cannot reparse source or infer language semantics in an adapter.
Filter, order, section Result, selection, details, and logs roles are explicit semantic identities
that must also resolve to contract-matching Core functions.
