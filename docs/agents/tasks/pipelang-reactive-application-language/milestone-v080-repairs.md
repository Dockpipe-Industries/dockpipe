# v0.80 milestone: six repair slices

The 2026-09-04 milestone review at `5adc0eb9801d27168120ab99cbfe7241953e48b0`
reproduced six inherited compiler defects. The founder requested that all six be addressed in
separate slices before further language expansion. This is repair work within the accepted
`v0.80.0` contract; no successor syntax or version is selected.

## Objective and boundaries

- Objective: `TASK-021-PipeLang-v0.80-milestone-repairs`.
- State: **completed** on 2026-09-04; all six repairs and terminal checks passed. R6 is
  uncommitted for founder review; no successor language slice is selected.
- Authority: the founder's 2026-09-04 request to address all six findings and document them here.
- Completion: all six repairs have focused regression proof, compiler/consumer compatibility
  checks pass, and this record contains the completed evidence and remaining limitations.
- Preserve the saved checkout, unrelated user state, public compiler/semantic/Application IR
  identities, frozen 45-source compatibility lane, and package/engine boundaries.
- No new language forms, v0.81 selection, commit, push, publication, worktree, generated-store
  refresh, credentials, or live operations are authorized by this record.
- Each slice has its own implementation, verification, and recorded result. Do not mark a
  finding repaired on the strength of a documentation update or its original reproduction.

## Repair sequence

| Slice | Priority | Finding and bounded repair | Required regression evidence | State |
| --- | --- | --- | --- | --- |
| R1 | P1 | Preserve computed Result carriers when evaluating call arguments; expected failure reaches the callee unless explicit propagation is authored. | Nested arithmetic and bounded Results, success/failure, recovery with a different caller return type, copied values, evaluator/generated-Go agreement. | Complete, committed |
| R2 | P1 | Memoize semantic-to-HIR dependency lowering across one shared dependency graph. | Each reachable method lowered once, shared acyclic graph scaling, deterministic order/output, cycle rejection retained. | Complete, committed |
| R3 | P2 | Enforce supported compiler/language identities and feature-version gates at Core program admission. | Unknown identities independently rejected; representative downgraded features rejected consistently by Core, evaluator, and backend. | Complete, committed |
| R4 | P2 | Make Core type validation exhaustive over supported kinds and representations. | Unknown kinds, unsupported numeric widths, contradictory/nested representations, unused parameters and expression types rejected; accepted types retained. | Complete, committed |
| R5 | P2 | Generate canonical argument validation from parameter types independently of function body shape. | Invalid UTF-8 and malformed arithmetic Results rejected in identity/unused/unselected cases; evaluator/generated-Go agreement and helper emission. | Complete, committed |
| R6 | P2 | Allocate or check generated names against the entire Go package namespace, including runtime declarations. | Legal source/runtime-name collision examples generate compilable deterministic Go; call targets agree with allocated declarations. | Complete, uncommitted |

R1 and R2 are the review's recommended blockers for further compiler expansion. All six remain
required by the founder's repair request. The Core and host-value admission repairs must precede
reliance on those boundaries for untrusted artifacts or values.

## Reproduced baseline

Source paths below are relative to the repository root; line references identify the reviewed
commit and may move during repair.

- **R1:** `src/lib/pipelang/coreeval/evaluate.go:293–310` special-cases direct Result references,
  but passes the payload of computed successful Results and returns early on computed failures.
  Accepted source `Run(int x) => Recover(Make(x))`, where `Make` returns `x + 1` and `Recover`
  matches the arithmetic Result into string, fails on success and leaks overflow from the
  string-returning caller on failure. Generated Go returns `"ok"`/`"recovered"` correctly.
- **R2:** `src/lib/pipelang/hir_lowering.go:99–125` creates a fresh `seen` map in recursive
  lowering and deduplicates only after doing the work. A bool helper graph with
  `Fn(x) => F(n-1)(x) && F(n-2)(x)` took 0.309 s for 21 functions, 2.228 s for 25, and 15.195 s
  for 29 (1,266 source bytes), measured after successful analysis using Go 1.25.13. This is a
  compiler CPU-exhaustion risk; no hosted endpoint or network exploit was tested.
- **R3:** `src/lib/pipelang/coreir/semantics.go:269` admits Core for `trim(value)` with an unknown
  compiler identity, unknown language identity, or v0.1.0 language identity. These independent
  mutations validate and evaluate but are rejected by the Go backend.
- **R4:** `src/lib/pipelang/coreir/semantics.go:2287` returns success for remaining non-record
  types. An identity function mutated to an invented type or signed 3-bit numeric type passes
  Core validation and evaluates integer 999. These are malformed-Core probes, not source syntax.
- **R5:** `src/lib/pipelang/gobackend/generate.go:252–269` checks string arguments only for
  recognized named predicates and arithmetic Results only for complete match bodies. Generated
  `Echo(string value) => value` returns byte `0xff`; a terminal-if function ignores an arithmetic
  Result with an invented failure tag. The evaluator rejects both inputs.
- **R6:** `src/lib/pipelang/gobackend/generate.go:194–213,2454` does not include runtime types
  in the function-name collision map. A method named `ArithmeticResult` returning checked
  arithmetic produces both a function and a type named `PipeLangArithmeticResult`;
  generation succeeds but the Go compiler rejects the output.

The original full review and temporary Go-overlay probes were written under
`/tmp/pipelang-v080-review`; they are supplementary evidence and may expire. The summaries above
and source-controlled regression tests for each completed slice must be sufficient to continue work
without those temporary files.

## Verification and completion evidence

Use cached Go 1.25.13 with offline lookup and temporary writable caches. Start with each slice's
regressions and affected suites; after the final repair, run full PipeLang and Application IR,
focused application PipeLang integration, CLI and frozen compatibility suites, and appropriate
vet/format/doc checks. Do not infer live-operation or commit authority from successful tests.

The review admitted the original v0.80 full-suite proof at the identical HEAD and additionally
compiled the five-conditional topology with root/leaf/unused locals across nine value types.
No new defect was found in that topology classifier or the unused-local blank-use fix.
This does not replace repair-specific verification. Sustained fuzzing, stack-exhaustion tests,
the exhaustive version cross-product, and a repository-wide DockPipe audit were not performed.

### R1 completed — 2026-09-04

`src/lib/pipelang/coreeval/evaluate.go` now evaluates each call argument once, reconstructs and
copies the complete carrier for every Result-typed argument, checks it against the target
parameter, and validates the value before entering the callee. A failed computed Result reaches
the callee; ordinary arguments retain their existing explicit-propagation behavior. No source
admission rule, Core node, generated-Go representation, or language version changed.

`src/lib/pipelang/result_call_argument_test.go` records:

- the original recovery failure and its repair under both v0.36.0 and v0.80.0;
- successful/failed nested int, float, text, and snapshot Result transport with exact evaluator
  outcome preservation and pristine generated-Go execution;
- a copied snapshot payload that does not alias the incoming carrier.

The recovery and snapshot-copy regressions failed before the repair. Focused regressions then
passed. Full `./src/lib/pipelang/...`, `./src/lib/applicationir`, and `./tests/pipelangcompat`
passed; `./src/lib/application -run PipeLang` passed. Vet for PipeLang/Application IR, Go
formatting, task YAML/route consistency, and diff whitespace passed. All used cached Go 1.25.13
with network lookup disabled. The canonical pure-call description in `docs/concepts/pipelang.md`
now explicitly states Result argument transport semantics.

CLI/editor and unrelated repository suites were not rerun for this evaluator-only repair;
terminal verification for all six slices remains pending. Generated Go and caches/logs are
temporary under `/tmp`. R1 was subsequently committed as
`06a81cac10978ece1cd1ae5f6f8c0aa78984f3ef` (`Preserve Result failures as pure-call arguments`),
verified on receipt of the R2 continuation. That checkpoint supersedes the original uncommitted
status; R1 implementation was admitted without rework.


### R2 completed — 2026-09-04

`src/lib/pipelang/hir_lowering.go` now separates single-method body lowering from one
request-local dependency traversal. Completed semantic identities skip repeated body lowering;
active identities reject cycles defensively. Ordinary calls and named predicate dependencies
share the same traversal and produce one closed, dependency-first function list in the original
discovery order. There is no persistent cache or cross-root state. Source admission, function
bodies, public identities, language versions, and package/engine boundaries are unchanged.

`src/lib/pipelang/hir_dependency_graph_test.go` records:

- exact single-method lowering counts for shared Fibonacci-shaped graphs of 21, 25, 29, and
  128 reachable methods, excluding an unreachable method;
- fixed dependency order, repeated HIR/Core/generated-Go equality, and evaluator/generated-Go
  agreement for both boolean inputs on the 21-method graph;
- a named predicate shared by two filter helpers reached through nested calls, with deduplicated
  closure and an independent second-root request;
- retained direct/indirect source-cycle diagnostics and failed-analysis rejection, plus defensive
  traversal rejection after deliberately making checked syntax cyclic;
- a benchmark that excludes semantic analysis and measures HIR lowering without timing assertions.

Focused regressions passed. A temporary Go overlay disabling the completed-method guard made the
work-count regression fail immediately on the second lowering of `F1`; the tracked source retained
the guard. Another overlay compared the committed R1 lowerer with this repair on a 13-method
shared graph, nested calls, and shared named predicates. All 12 SHA-256 comparisons matched for
serialized HIR, Core, semantic projection, and generated Go. Existing Application IR goldens and
the frozen 45-source compatibility suite passed without updates.

The benchmark (`-bench '^BenchmarkHIRSharedDependencyGraph$' -benchtime=200ms`) measured about
0.44/0.54/1.00/4.96 ms and 3,092/3,716/4,342/19,800 allocations for 21/25/29/128 methods on this
machine. These are observations, not performance thresholds or a claim that every compiler phase
is linear. The deterministic work-count regression is the scaling acceptance criterion.

Validation passed with cached Go 1.25.13, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and
a writable temporary build cache:

- `go test ./src/lib/pipelang -run '^TestHIR(Shared|Dependency)' -count=1`;
- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`;
- `go test ./src/lib/application -run PipeLang -count=1`;
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir`;
- Go formatting, task YAML/route consistency, and diff whitespace.

Canonical lowering behavior is clarified in `docs/concepts/pipelang.md`; the local and global
TASK-021 indexes mark R1/R2 complete and R3 next. Temporary overlay probes, benchmark output,
and verification logs are under `/tmp/pipelang-r2-proof`; generated Go tests and caches also use
`/tmp`. No generated repository artifacts were added. CLI/editor, unrelated repository suites,
and deep-stack exhaustion testing were not run; all-six-slice terminal verification remains
pending. R2 was subsequently committed as `88c0e97f50d1b577c40ba3d74a713c5f10a2cf99`
(`Memoize PipeLang dependency graph lowering`), verified at R3 admission on branch `js/pipelang`
with a clean saved checkout. This supersedes the original uncommitted status; R2 proof was admitted
without rework. R3–R6 were not implemented in the R2 slice.


### R3 completed — 2026-09-04

`src/lib/pipelang/coreir/admission.go` owns exact compiler/language identity admission and
Core feature availability. `ValidateProgram` rejects missing, malformed, legacy-source-only,
unknown, or future identities, including empty programs. An ordered allowlist recognizes only
`pipelang.compiler.v1` with `v0.1.0` through `v0.80.0`. Feature checks inspect signatures and the
complete expression tree, retaining the existing contextual checks for calls, matching, locals,
propagation, and terminal topology. They include record/text/Optional/list/Result operations,
joined-selector arity, and directional sorting. This is version admission, not R4's exhaustive
validation of type kinds and representations.

`gobackend.Generate` now relies on that Core admission without rewriting later language metadata
to v0.30.0 or maintaining duplicate language gates. `coreeval.EvaluateProgram` already calls
`ValidateProgram`, so all three program entrypoints reject the same invalid contract with the
same underlying message before function lookup, execution, or generated output. The backend keeps
its `PLGO0001` envelope. Function-only APIs have no program metadata and retain their existing
structural validation; versioned artifacts must enter through the program APIs.

The canonical Core contract differs from source syntax: checked arithmetic and arithmetic Result
representation were already compiler-internal v0.1.0 capabilities. The first broad run caught an
overly restrictive draft gate for those capabilities and pre-existing string concatenation; the
repair preserves them. Likewise, v0.33.0 postfix indexing reuses v0.20.0 `list_at`. No source form,
compiler/semantic/Application IR identity, or language version changed.

`src/lib/pipelang/core_admission_test.go` records:

- independent invalid compiler/language mutations on simple and trim programs, with and without
  functions, plus the original trim-to-v0.1.0 reproduction;
- 22 existing Core feature fixtures rejected one contract below their accepted boundary;
- unchanged generated bytes and evaluator outcomes for all 28 normalized Core fixtures at their
  original version and v0.80.0, including six arithmetic/transport fixtures also retained at v0.1.0;
- admission and unchanged behavior for a source-lowered baseline function under all 80 supported
  language identities; the tiny numeric-comparison fixture is not used for this evaluator
  comparison (see the R4 clarification below);
- text Results, directional sorting, propagation, matching, and nested trim, with accepted Core
  evaluation and pristine generated-Go compilation before metadata downgrade;
- rejection of a feature in an uncalled function and a type in an unused parameter.

The final focused regressions passed. A temporary overlay restored the committed pre-R3 Core
validator and Go backend while retaining the new tests: 48 independent identity mutations and
22 downgraded feature fixtures failed because Core admitted them. The tracked implementation was
never reverted. Full terminal checks for this slice passed using cached Go 1.25.13 with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and a temporary writable cache:

- `go test ./src/lib/pipelang -run '^TestCoreAdmission' -count=1`;
- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`;
- `go test ./src/lib/application -run PipeLang -count=1`;
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir`;
- Go formatting, task YAML/route consistency, and diff whitespace.

Application IR goldens and the frozen 45-source compatibility lane passed without updates.
`docs/concepts/pipelang.md` documents program admission; both TASK-021 indexes mark R1–R3 complete
and R4 next. Package/engine boundaries remain intact. Logs, the negative-control overlay, and
cache live under `/tmp/pipelang-r3-proof`; generated-Go checks use temporary directories. No
generated repository artifacts were added. CLI/editor and unrelated repository suites, sustained
fuzzing, exhaustive feature/version combinations, and stack-exhaustion checks were not run;
all-six-slice terminal verification remains pending.

R3 was subsequently committed as `0cd0bf719899404d8234b7ca029e98884c377b64`
(`Admit PipeLang program identities and feature versions`), verified at R4 admission on
`js/pipelang` with a clean saved checkout. This supersedes the original uncommitted status;
R3 proof was admitted without rework. R4–R6 were not implemented in the R3 slice.


### R4 completed — 2026-09-04

`src/lib/pipelang/coreir/semantics.go` now admits only supported executable type kinds,
normalized string/bool primitives, signed int64/binary64 numerics, internal ArithmeticError,
and the existing bounded record/Optional/list/Result shapes. It rejects fields belonging to
another kind, unresolved named/applied types, source-only primitive int/float executable types,
missing representations, unknown numeric representations, unsupported widths, and invalid
signedness. Nested Optional payloads and record fields now receive the same complete validation.
The existing Result envelope is checked before descending into its payloads; arbitrary nested
Results remain unsupported. No accepted envelope or compiler-internal arithmetic capability
was expanded or removed.

Every expression enters type validation before its node-specific checks. Existing signature and
local declaration validation uses the same rules, and propagation carrier annotations are checked
explicitly rather than relying on type equality (which intentionally ignores callable metadata
on a record identity). All program consumers retain R3's shared admission and diagnostic behavior.
No backend coupling, language version, source form, public identity, or package boundary changed.

`src/lib/pipelang/core_type_validation_test.go` covers:

- the invented-kind and signed 3-bit identity reproductions, with integer 999 supplied to the
  function evaluator; unknown/empty/named/applied kinds, invalid primitives, missing representations,
  and a representation/width/signedness matrix;
- contradictory fields across all supported kinds in identity, unused-parameter, and return
  positions, plus malformed nested payloads/fields and unsupported container nesting;
- malformed comparison literal types outside signatures, including an unselected branch and an
  uncalled function; malformed local and propagation-carrier annotations;
- retained supported types through Core function/program admission, evaluator identity transport,
  and Go generation (standalone internal ArithmeticError is tested only at Core admission);
- the unchanged tiny fixture's normalized numeric type, source-level semantic identity, Core
  admission, and exact generated-Go golden.

The R3 handoff described the tiny fixture as pre-normalization. Direct inspection corrected that:
its executable types already use signed int64; only semantic identity metadata says primitive int.
Attempting to include it in R3's evaluator fixture loop exposed the existing
`primitive comparison type is unsupported` numeric-comparison limitation. R4 does not repair
numeric comparison execution. The loop's comment now gives that actual reason for exclusion;
R4 separately proves this fixture's admission and generated bytes. No fixture or golden was changed.
The other 28 fixtures retain their evaluator/generated-Go comparisons and internal v0.1 arithmetic
coverage from R3.

Focused checks passed. A temporary Go overlay restored the pre-R4 validator while retaining the
new regressions; the invented-kind/3-bit identity, contradictory/nested representation, and all
three expression-location regressions failed because the old validator admitted malformed types.
A separate negative control also confirmed admission of the malformed propagation-carrier annotation.
Tracked source was never reverted. Validation used cached Go 1.25.13 with `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, and a writable temporary cache:

- `go test ./src/lib/pipelang -run '^TestCore(TypeValidation|Admission)' -count=1`;
- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`;
- `go test ./src/lib/application -run PipeLang -count=1`;
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir`;
- the additional local/carrier regression, Go formatting, task YAML/route consistency, and
  diff whitespace.

The full suites passed, including pristine generated-Go execution, unchanged Application IR
goldens, and the frozen 45-source compatibility lane. Canonical type-admission behavior is in
`docs/concepts/pipelang.md`; both TASK-021 indexes mark R1–R4 complete and R5 next. Logs, the
negative-control overlay, and cache are under `/tmp/pipelang-r4-proof`; generated Go uses temporary
directories. No generated repository artifacts were added. CLI/editor, unrelated repository suites,
sustained fuzzing, and stack-exhaustion checks were not run; terminal verification for all six
repairs remains pending. The numeric-comparison evaluator limitation remains outside this slice.

R4 was completed for founder review, then committed as
`b60e5b50aae499707afe0c739a0ac14c02753a3a` (`Validate executable Core types exhaustively`).
The R5 receiver verified that commit and a clean saved checkout on 2026-09-04; the earlier
uncommitted status is superseded. R5–R6 were still pending at that checkpoint.


### R5 completed — 2026-09-04

`src/lib/pipelang/gobackend/generate.go` now emits canonical string and arithmetic Result
argument validation from parameter types before any function body executes. The obsolete
named-predicate string gate and top-level-match arithmetic gate are removed. Arithmetic validator
discovery includes parameter types while retaining operation-driven helper discovery. Existing
record, Optional, list, and bounded Result validators continue to validate their supported nested
payloads; the current Core envelopes do not permit arithmetic Results inside those containers.
No source form, language version, public compiler/semantic/Application IR identity, or backend
package dependency changed.

The R5 matrix also exposed a necessary evaluator consistency repair:
`src/lib/pipelang/coreeval/evaluate.go` previously accepted nonzero numeric success payloads on
failed arithmetic Results, although the existing generated validator rejected them. The evaluator
now requires numeric zero and no nested carrier on those failures. Both signs of floating-point
zero remain accepted; successful NaN and infinity payloads remain accepted and transported.
This does not change numeric-comparison execution or the supported Core type envelopes.

`src/lib/pipelang/argument_validation_test.go` verifies the source analysis -> typed HIR -> Core
pipeline, both evaluator APIs, deterministic generation, and compiled generated-Go behavior for:

- direct strings, int64/binary64 arithmetic Results, primitive records, Optional<string>,
  Optional<Record>, record lists, text Results, and snapshot Results;
- identity, unused-parameter, and both selected/unselected terminal-branch paths;
- invalid UTF-8 byte sequences, unknown/missing arithmetic failure tags, errors on successful
  carriers, and nonzero/NaN/infinity payloads on failed arithmetic carriers;
- valid Unicode and preserved values, both supported arithmetic failure tags, signed zero,
  successful NaN/infinity, present/absent Optionals, and empty/nonempty lists;
- isolated unused-only string/arithmetic signatures with bool returns, proving helper/import
  emission without another function, return type, match, propagation, or text operation.

Temporary Go overlays restored the pre-R5 backend and evaluator separately while retaining the
new regression matrix. The old backend accepted malformed direct strings and arithmetic Results
in all three body shapes. The old evaluator accepted nonzero numeric payloads, NaN, and infinity
on failed arithmetic carriers. Both negative controls failed as expected; tracked source was
never reverted.

Thirteen generated-Go goldens were updated after reviewing their exact diffs: only entry
validation calls and the newly required arithmetic validation helper were added. Source, HIR,
Core JSON, semantic, and Application IR goldens remain unchanged. The tiny fixture's exact Go
bytes and normalized types remain unchanged, and internal v0.1 arithmetic capability is retained.

Validation passed using cached Go 1.25.13 with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`,
and a writable temporary cache:

- `go test ./src/lib/pipelang -run '^TestCanonicalArgumentValidation' -count=1`, including the
  final isolated helper checks;
- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`;
- `go test ./src/lib/application -run PipeLang -count=1`;
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir`;
- Go formatting, task YAML/document-route consistency, and diff whitespace.

The full suites retain generated-Go execution, unchanged Application IR goldens, and the frozen
45-source compatibility lane. Canonical host-argument behavior is documented in
`docs/concepts/pipelang.md`; both TASK-021 indexes mark R1–R5 complete and R6 next. Logs, negative
controls, candidate goldens, and cache are under `/tmp/pipelang-r5-proof`; generated execution uses
temporary modules and the existing temporary generated-Go cache. The 13 tracked Go goldens are
intentional test artifacts; no generated store or runtime artifact was refreshed.

R5 was complete and uncommitted at its original review boundary. It was subsequently committed as
`163a6effba408bc8ad6c4b5d88dc594b8817d483` (`Enforce canonical argument validation across evaluator
and Go backend`), verified with a clean saved checkout at R6 admission on 2026-09-04. This
supersedes the earlier uncommitted status. R6 was still pending at that checkpoint.
The pre-existing numeric-comparison evaluator limitation remains outside this slice. CLI/editor,
unrelated repository suites, sustained fuzzing, and stack-exhaustion checks were not run; the
six-repair terminal verification remains pending. Package/engine boundaries were preserved.
No commit, push, publication, worktree, destructive cleanup, credentials, or live operation was
performed in R5.


### R6 completed — 2026-09-04

The receiver verified `/home/jamie/source/dockpipe`, branch `js/pipelang`, HEAD
`163a6effba408bc8ad6c4b5d88dc594b8817d483`, and no staged, unstaged, or untracked entries.
R1–R5's durable proof was admitted. No worktree, cleanup, stash operation, or commit was performed.

`src/lib/pipelang/gobackend/names.go` adds per-generation identity-based name allocation.
It inventories the actual emitted Go support with the Go token scanner, including types,
constants, variables, helpers, and import names, before allocating source functions. A final
namespace check rejects any duplicated emitted declaration. Receiver methods stay outside the
package namespace; comments, strings, fields, generic parameters, and local declarations do not
reserve package names. The backend retains the parser/AST import prohibition and accepts Core only.

`generate.go` now resolves ordinary and predicate calls through the same semantic identity map
as function declarations. Record identities receive deterministic distinct Go type names, and
record constructors/validators, Optional payload checks, list helpers, bounded Result helpers,
and all type references consume those allocated names through the generation context. Preferred
noncolliding names and the existing Optional fallback remain unchanged. Collisions receive
stable numeric suffixes in sorted identity order. No source-name restriction, Core mutation,
language form, compiler/semantic/Application IR identity, or package/engine boundary changed.

`Generate` retains its existing source/error API. `GenerateWithNames` additionally returns
function identity/name bindings for host callers; `FunctionName` remains a context-free preferred
name and is documented accordingly. Existing uses of that helper are noncolliding fixture tests;
there is no production call target left that reconstructs a function name from source spelling.
The canonical contract is documented in `docs/concepts/pipelang.md`.

Tracked regressions in `go_namespace_test.go` and `gobackend/names_test.go` cover:

- legal source functions colliding with arithmetic runtime types and both error constants,
  including direct host entrypoints, ordinary calls, success, overflow, and evaluator agreement;
- a source function named `Result` alongside the text Result runtime;
- same-named methods owned by different classes, with distinct calls and returned values;
- a legal source predicate whose name collides with its emitted record type, with compiled
  filtering proving identity-based target selection;
- distinct supported Core record identities that normalize to the same Go spelling, including
  constructors, primitive fields, Optional validation, lists, and snapshot Results (this identity
  mutation is a Core probe, not newly accepted source syntax);
- byte-identical output and bindings under reversed function order, repeated generation,
  agreement between both generation APIs, and unchanged serialized Core inputs;
- every emitted package declaration category, grouped declarations, imports, method scope,
  and rejection of duplicate declarations without confusing strings or locals with package names.

The focused regression command passed:
`go test ./src/lib/pipelang/gobackend ./src/lib/pipelang -run '^Test(GoNamespace|PackageNamespace|GoBackendCannotImport|Optional.*Collision)' -count=1`.
The existing `TestV130GoOptionalSupportNameAvoidsSourceFunctionCollisions` also passed in the full
suite. A temporary overlay restored the pre-R6 backend emission without reverting tracked source.
The original plain-generation reproduction failed Go compilation with
`PipeLangArithmeticResult redeclared in this block`; the same-name method and normalized record
cases failed the old backend's collision checks. The overlay adapter only connects the new
result API to old emission for the negative control; it is not production code.

### Six-repair terminal verification — passed, 2026-09-04

Cached Go 1.25.13 was used with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and a writable
`/tmp/pipelang-r6-proof/cache`. Generated test modules also set `GOWORK=off` and use the existing
temporary generated-Go cache. Commands completed successfully:

- `go test ./src/lib/pipelang/... ./src/lib/applicationir ./tests/pipelangcompat -count=1`;
- `go test ./src/lib/application -run 'PipeLang|TestCompileWorkflowsBatchSupportsConfigPipe' -count=1`;
- `go test ./src/cmd -count=1`;
- `go vet ./src/lib/pipelang/... ./src/lib/applicationir`;
- JavaScript syntax checking of the language-support extension, changed Go formatting, task
  YAML and document-route consistency, and `git diff --check`.

These suites include R1–R6 regressions, Core-only backend dependency guards, generated-Go
execution, unchanged HIR/Core/Go/semantic/Application IR goldens, accepted v0.80 and internal
v0.1 capabilities, and the exact frozen 45-source compatibility inventory and digests. R6 adds
no golden changes. The tiny-pure-function fixture retains its exact Go golden and normalized
int64 executable types; its pre-existing numeric-comparison evaluator limitation remains outside
this objective. The other 28 admission fixtures retain evaluator/generated-Go comparisons.

Logs, the negative-control overlay, a temporary mechanical-edit helper, and caches are under
`/tmp/pipelang-r6-proof`; compiled generated Go uses temporary test modules. No generated store,
runtime artifact, or tracked golden was refreshed. R6 implementation, regressions, canonical
documentation, this record, the overview, and both TASK-021 indexes are uncommitted for founder
review. No push, publication, credentials, Docker/cloud/live operation, or successor selection
occurred. Repository-wide suites/builds, standalone editor execution tests, sustained fuzzing,
stack-exhaustion testing, and the exhaustive feature/version cross-product were not run.
All six requested repairs meet this objective's completion criteria; remaining language work
requires its own founder decision and authority.
