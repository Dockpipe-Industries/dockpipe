### Completed step 7a fixed numeric comparison/equality checkpoint (2026-08-16)

The first separately bounded step-7 slice makes executable numeric representation explicit without
adding source syntax or changing stable semantic identities. Existing source `int` lowers to a
target-independent signed two's-complement 64-bit HIR/Core type; existing source `float` lowers to
an IEEE-754 binary64 HIR/Core type. The Go backend consumes those Core shapes directly and no longer
infers width, signedness, or a mixed numeric conversion from source primitive names.

The `v0.1.0` semantic module lane accepts the existing comparison and equality operators only when
both numeric operands have the same resolved type. Mixed integer/float comparison or equality fails
with deterministic structured diagnostic `PL3028`; the frozen legacy source-set lane retains its
existing behavior. Generated binary64 comparison/equality is compiled and executed under `/tmp` and
matches the existing pure evaluator for ordinary values, unordered and unequal `NaN`, and equal
positive/negative zero. Malformed mixed-type Core operands fail in the backend instead of being
coerced.

Numeric addition, subtraction, multiplication, division, and negation remain fail-closed in both
the `v0.1.0` checker and Go backend. Enabling them would require the accepted checked-overflow and
recoverable-failure contract, so neither layer substitutes unchecked target arithmetic or a panic
before typed `Result` failure semantics exist. This slice changes no syntax, semantic projection,
legacy artifact, public YAML/schema/editor surface, generated store, runtime, or dependency.

### Completed step 7b compiler-internal arithmetic Result checkpoint (2026-08-16)

The next bounded step-7 foundation adds structural `Result<Success, ArithmeticError>` types to typed
HIR and Core without selecting production source spelling. Core owns one target-independent checked
arithmetic signature and validation contract: signed 64-bit addition, subtraction, multiplication,
and negation produce an integer result or `overflow`; binary64 division produces a binary64 result
or `division_by_zero` for positive or negative zero. Other operand widths, signedness, implicit
numeric conversions, nested fallible expressions, and error families fail closed.

An inert `coreeval` package consumes Core IR only and serves as the semantic conformance evaluator.
The Go backend consumes the same validated Core contract and emits explicit generic result values
with stable `overflow` and `division_by_zero` errors; it never converts a target panic into a domain
result. Exact HIR/Core/Go goldens cover checked addition. Boundary tests compare the centralized
integer implementation with exact `math/big` results and prove evaluator/generated-Go agreement for
success, every supported overflow family, negating the minimum integer, division by both signed
zeros, and IEEE `NaN` propagation. Architecture tests prohibit both `coreeval` and the Go backend
from importing parser, AST/compiler-root, HIR, or each other.

The existing `v0.1.0` source checker intentionally continues to reject numeric arithmetic with
structured `PL3028`: a source method declared to return `int` or `float` cannot silently become a
fallible method. The frozen legacy evaluator and artifacts remain unchanged. This checkpoint adds no
production `Result`/`ArithmeticError` spelling, implicit unwrap, branches or matching, public
semantic projection, YAML/schema/editor surface, runtime, generated store, or dependency.

### Completed step 7c public arithmetic Result contract (2026-08-16)

The founder explicitly accepted the first production-source arithmetic result contract as a
versioned migration from the fail-closed seed. The new language contract is `v0.2.0`; `v0.1.0`
remains supported and continues to reject numeric arithmetic with `PL3028`. No source file, module,
or package selects `v0.2.0` implicitly. The display and machine names remain `PipeLang` and
`pipelang`, and the independently versioned compiler and public projection contracts remain
`pipelang.compiler.v1` and `pipelang.semantic.v1`.

The accepted source spelling is
`Result<int, ArithmeticError> Add(int left, int right) => left + right;`. `Result` is the language
built-in type constructor with semantic identity `pipelang:result`; `ArithmeticError` is the closed
language built-in failure type with semantic identity `pipelang:arithmetic.error`. Callable identity
and semantic projection use the existing structured applied-type shape: the identified `Result`
constructor carries ordered success and failure arguments, and the failure argument carries the
identified `ArithmeticError` type. This is not authority for a general Result library or
user-defined generic declarations.

Checked arithmetic already has the declared Result type; there is no conversion or implicit
wrapping. In this bounded slice, the only admitted handling is returning one checked integer
addition as the complete body of an expression-bodied method whose declared return is exactly
`Result<int, ArithmeticError>`. No unwrap, propagation, nesting, extraction, matching, block,
branch, or use as `int` is admitted. The observable outcomes are an explicit success value or the
closed `overflow` error. Other arithmetic operators and the already-proven
`division_by_zero` identity remain compiler-internal until separately synchronized source slices.

The synchronized implementation gates the grammar on an explicitly selected `v0.2.0` module while
the legacy parser and `v0.1.0` semantic lane retain their prior behavior. The parser preserves full
type and argument spans; resolution fixes both built-in identities; callable identity and
`pipelang.semantic.v1` project the identified applied type without an analysis-local symbol. The
typechecker rejects ordinary integer returns, nesting, other arithmetic, other Result arguments,
Result fields/parameters/interfaces, bare arithmetic errors, and declarations that shadow the two
language-owned names. The accepted method lowers from checked source through typed HIR and Core,
evaluates through `coreeval`, and generates deterministic compilable Go with identical success and
overflow outcomes.

Terminal proof passed with cached Go 1.25.13, the required offline environment, and writable `/tmp`
caches:

- exact `go test ./src/lib/pipelang/... ./tests/pipelangcompat`, including parser/type/identity/
  projection/diagnostic coverage, source-derived HIR/Core/Go goldens, Core evaluation, and generated
  Go compilation/execution under `/tmp`;
- focused application PipeLang check/compile/invoke, catalog, materialize, representative workflow/
  package compile, internal materialize/package-compile, and `src/cmd` tests through only the
  temporary `/tmp` modfile pinned to cached `golang.org/x/sys v0.46.0`; checkout dependencies did
  not change;
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages; and
- VS Code extension diagnostics plus durable Result/ArithmeticError grammar and completion checks.

The broad application suite had only its two unrelated sandbox/topology failures: loopback listen
is prohibited for `TestCmdInstallCoreEmitsOperationResults`, and
`TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap` names nonexistent
`/path/to/your/project`. The focused affected suite is green. The exact 45-source compatibility
inventory, legacy artifacts/evaluation, dependencies, generated stores, and protected ignored bytes
did not change.

### Completed step 7d direct checked-subtraction contract (2026-08-17)

The founder selected the recommended new-version option for the next smallest public arithmetic
slice. `v0.3.0` is an explicit migration that preserves `v0.2.0` as add-only and additionally admits
exactly
`Result<int, ArithmeticError> Subtract(int left, int right) => left - right;`. `v0.1.0` remains
fail-closed with `PL3028`, and no source file, module, or package selects `v0.3.0` implicitly.

The subtraction is the complete body of one expression-bodied class method. Its two operands are
exactly `int`; its expression already has the declared `Result<int, ArithmeticError>` type; and its
only observable outcomes are an explicit integer success or the existing closed `overflow` error.
There is no conversion, wrapping, unwrapping, propagation, nesting, extraction, matching, block,
branch, or use as an ordinary integer. Multiplication, negation, binary64 division, and general
Result handling remain compiler-internal.

The source contract reuses the language-owned `pipelang:result` and
`pipelang:arithmetic.error` identities. Callable identity remains the structured ordered signature
`(int, int) -> Result<int, ArithmeticError>`, and the public compiler/projection contracts remain
`pipelang.compiler.v1` and `pipelang.semantic.v1`. Typed HIR and Core preserve the selected
subtraction operator and target-independent Result type; Core evaluation and the Go backend consume
that same contract rather than redefining failure behavior.

The synchronized implementation explicitly gates `v0.3.0` through parsing, type resolution and
checking, semantic callable identity and `pipelang.semantic.v1`, typed HIR, target-neutral Core,
`coreeval`, and the deterministic Go backend. Source-derived HIR/Core/Go goldens preserve the
subtraction operator and identified Result return. Core evaluation and compiled generated Go agree
for ordinary and negative successes plus both signed overflow directions. Negative diagnostics
prove `v0.1.0` and `v0.2.0` do not accept subtraction, `v0.2.0` remains add-only, and `v0.3.0`
rejects nesting, multiplication, negation, division, alternate Result arguments, and unsupported
Result placements.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` build and
temporary caches:

- exact `go test -count=1 ./src/lib/pipelang/... ./tests/pipelangcompat`;
- focused application PipeLang/check/compile/invoke, catalog, representative workflow/package
  compile, internal materialize/package-compile, and `src/cmd` tests through only the existing
  temporary modfile pinned to cached `golang.org/x/sys v0.46.0`;
- `go vet` across PipeLang, compatibility, and all affected application/internal/CLI packages;
- VS Code extension diagnostics/completion/grammar validation, including subtraction highlighting;
  and
- `gofmt`, `git diff --check`, exact 45-source inventory and compatibility goldens, dependency-diff,
  branch/stash, and protected ignored-byte proof.

The previously admitted unrelated broad-application loopback and nonexistent-fixture failures were
not reopened; the complete affected set is green. No generated store, dependency, runtime,
credential, external state, commit, or publication changed. No later step-7 operator or
Result-composition rule is authorized by this checkpoint.

### Completed step 7e direct checked-multiplication contract (2026-08-17)

The founder selected a new explicit `v0.4.0` contract for the next smallest public arithmetic
slice. It preserves `v0.3.0` unchanged and admits exactly
`Result<int, ArithmeticError> Multiply(int left, int right) => left * right;`. The multiplication
is the complete body of one expression-bodied class method and produces either an explicit integer
success or the existing closed `overflow` error. It reuses `pipelang:result`,
`pipelang:arithmetic.error`, `pipelang.compiler.v1`, and `pipelang.semantic.v1`; no source, module,
or package migrates implicitly.

The synchronized implementation gates this one binary operator through the existing
version-selected parser/type/identity/projection/HIR/Core/evaluator/Go/docs/editor pipeline.
Source-derived HIR/Core/Go goldens preserve the multiplication operator and identified Result
return. Core evaluation and compiled generated Go agree for positive, negative, and zero successes,
ordinary overflow, and the minimum-integer multiplied by negative one. Negative diagnostics prove
that `v0.1.0` through `v0.3.0` do not admit multiplication and that `v0.4.0` still rejects nesting,
negation, division, alternate Result arguments, and unsupported Result placements.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including parser/type/identity/projection,
  Core evaluation, generated-Go compilation/execution, and deterministic goldens;
- affected PipeLang CLI, catalog, materialize, package-compile, internal, and `src/cmd` tests through
  only the existing temporary modfile pinned to cached `golang.org/x/sys v0.46.0`;
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages;
- VS Code grammar/completion validation plus `gofmt`, `git diff --check`, frozen inventory,
  dependency, branch/stash, and protected ignored-byte proof.

Integer negation, binary64 division, nested fallible expressions, general Result handling, and all
later language work remain excluded until separately accepted. No generated store, dependency,
runtime, credential, external state, commit, or publication changed.

### Completed step 7f direct checked-negation contract (2026-08-17)

The founder selected a new explicit `v0.5.0` contract for the next smallest public arithmetic
slice. It preserves `v0.4.0` unchanged and admits exactly
`Result<int, ArithmeticError> Negate(int value) => -value;`. Negation is the complete body of one
expression-bodied class method and produces either an explicit integer success or the existing
closed `overflow` error for the minimum integer. It reuses `pipelang:result`,
`pipelang:arithmetic.error`, `pipelang.compiler.v1`, and `pipelang.semantic.v1`; no source, module,
or package migrates implicitly.

The synchronized implementation gates this one unary operator through the existing
version-selected parser/type/identity/projection/HIR/Core/evaluator/Go/docs/editor pipeline.
Source-derived HIR/Core/Go goldens preserve the negation operator and identified Result return.
Core evaluation and compiled generated Go agree for positive, negative, and zero successes plus
minimum-integer overflow. Negative diagnostics prove that `v0.1.0` through `v0.4.0` do not admit
negation and that `v0.5.0` still rejects nesting, binary64 division, alternate Result arguments,
and unsupported Result placements.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact PipeLang and 45-source compatibility tests, including parser/type/identity/projection,
  Core evaluation, generated-Go compilation/execution, and deterministic goldens;
- affected PipeLang CLI, catalog, materialize, package-compile, internal, and `src/cmd` tests through
  only the existing temporary modfile pinned to cached `golang.org/x/sys v0.46.0`;
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages;
- VS Code grammar/completion validation plus `gofmt`, `git diff --check`, frozen inventory,
  dependency, branch/stash, and protected ignored-byte proof.

Binary64 division, nested fallible expressions, general Result handling, and all later language
work remain excluded until separately accepted. No generated store, dependency, runtime,
credential, external state, commit, or publication changed.

### Completed step 7g direct checked-binary64-division contract (2026-08-17)

The founder selected a new explicit `v0.6.0` contract for the final compiler-internal arithmetic
source slice. It preserves `v0.5.0` unchanged and admits exactly
`Result<float, ArithmeticError> Divide(float left, float right) => left / right;`. Division is the
complete body of one expression-bodied class method. A positive or negative zero divisor produces
the existing closed `division_by_zero` error; every nonzero divisor produces an explicit binary64
success while retaining IEEE-754 behavior including `NaN`, infinities, and signed zero. It reuses
`pipelang:result`, `pipelang:arithmetic.error`, `pipelang.compiler.v1`, and
`pipelang.semantic.v1`; no source, module, or package migrates implicitly.

The synchronized implementation makes the one contract-specific public widening from
`Result<int, ArithmeticError>` to the exact `Result<float, ArithmeticError>` return and gates
direct division through parsing, type resolution and checking, semantic callable identity and
`pipelang.semantic.v1`, typed HIR, target-neutral Core, `coreeval`, and the deterministic Go
backend. Source-derived HIR/Core/Go goldens preserve the division operator, binary64 operands, and
identified Result return. Core evaluation and compiled generated Go agree for ordinary division,
both signed-zero divisors, `NaN` operands, infinity, and signed-zero success. Negative diagnostics
prove that `v0.1.0` through `v0.5.0` do not admit division and that `v0.6.0` rejects unchecked,
nested, non-division, non-binary64, alternate Result, and unsupported Result-placement forms.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact `go test -count=1 ./src/lib/pipelang/... ./tests/pipelangcompat`, including parser/type/
  identity/projection, Core evaluation, generated-Go compilation/execution, deterministic goldens,
  and the frozen 45-source compatibility lane;
- affected PipeLang CLI, catalog, materialize, package-compile, internal, and `src/cmd` checks
  through only the existing temporary modfile pinned to cached `golang.org/x/sys v0.46.0`;
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages;
- VS Code grammar/completion validation plus `gofmt`, `git diff --check`, frozen inventory,
  dependency, branch/stash, and protected ignored-byte proof.

The admitted unrelated broad-application loopback and nonexistent-fixture failures were not
reopened; the complete affected set is green.

This slice adds no conversion, wrapping, unwrapping, propagation, extraction, matching, block,
branch, general Result handling, or adjacent language surface. The frozen legacy lane, earlier
contracts, compiler/projection identities, generated stores, dependencies, runtime, credentials,
and external state remain unchanged.

### Completed step 7h first-class arithmetic Result transport contract (2026-08-17)

The founder selected the recommended new explicit `v0.7.0` contract. It preserves every
`v0.2.0`-through-`v0.6.0` direct checked-arithmetic method unchanged and additionally admits one
pure transport shape for each existing public arithmetic Result value:

```pipelang
Result<int, ArithmeticError> ForwardInt(Result<int, ArithmeticError> value) => value;
Result<float, ArithmeticError> ForwardFloat(Result<float, ArithmeticError> value) => value;
```

Method and parameter names remain source-owned. The exact language rule requires one parameter,
an identical parameter and return type, and that parameter identifier as the complete
expression-bodied method body. It reuses `pipelang:result`, `pipelang:arithmetic.error`,
`pipelang.compiler.v1`, and `pipelang.semantic.v1`; callable identity and semantic projection carry
the same identified applied type in the ordered parameter and return positions. No source, module,
or package migrates implicitly, and `v0.1.0` through `v0.6.0` retain their prior rejection.

Typed HIR and target-neutral Core preserve the Result-typed parameter binding and direct reference.
`coreeval` accepts only canonical success or closed arithmetic-failure values and returns the same
success payload or error; malformed target-independent Result inputs fail validation. The Core-only
Go backend emits the existing explicit generic arithmetic Result value as both parameter and return
and forwards it directly. Integer success/`overflow`, binary64 signed-zero success, and
`division_by_zero` pass unchanged through Core evaluation and compiled generated Go.

Negative diagnostics keep Result fields, interface signatures, extra or mismatched parameters,
nested or alternate Result types, non-Result returns, different bodies, construction, extraction,
matching, wrapping, unwrapping, propagation, and ordinary numeric use closed. `PL3006` remains the
placement/type diagnostic and `PL3028` remains exclusively the established checked-arithmetic
failure-to-declare rule. No parser call/member/block/control-flow surface was added.

Terminal proof passed with cached Go 1.25.13, offline module lookup, and writable `/tmp` caches:

- exact `go test -count=1 ./src/lib/pipelang/... ./tests/pipelangcompat`, including parser/type/
  identity/projection, HIR/Core/Go goldens, Core Result validation, generated-Go execution, explicit
  migration rejection, and preservation of every earlier checked-arithmetic source slice;
- affected PipeLang CLI/check/compile/invoke, catalog, materialize, representative workflow/package
  compile, internal materialize/package-compile, and `src/cmd` tests through only the existing
  temporary modfile pinned to cached `golang.org/x/sys v0.46.0`;
- `go vet` across PipeLang, compatibility, and all affected application/internal/CLI packages; and
- VS Code grammar/completion/diagnostic validation plus durable `v0.7.0` editor guidance, `gofmt`,
  `git diff --check`, frozen inventory, dependency, generated-state, branch/stash, and protected
  ignored-byte proof.

No dependency, generated store, runtime, credential, external state, cleanup, commit, or
publication changed.

