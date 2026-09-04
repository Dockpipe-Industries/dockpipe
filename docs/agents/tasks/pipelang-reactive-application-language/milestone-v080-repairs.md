# v0.80 milestone: six repair slices

The 2026-09-04 milestone review at `5adc0eb9801d27168120ab99cbfe7241953e48b0`
reproduced six inherited compiler defects. The founder requested that all six be addressed in
separate slices before further language expansion. This is repair work within the accepted
`v0.80.0` contract; no successor syntax or version is selected.

## Objective and boundaries

- Objective: `TASK-021-PipeLang-v0.80-milestone-repairs`.
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
| R3 | P2 | Enforce supported compiler/language identities and feature-version gates at Core program admission. | Unknown identities independently rejected; representative downgraded features rejected consistently by Core, evaluator, and backend. | Complete, uncommitted |
| R4 | P2 | Make Core type validation exhaustive over supported kinds and representations. | Unknown kinds, unsupported numeric widths, contradictory/nested representations, unused parameters and expression types rejected; accepted types retained. | Pending |
| R5 | P2 | Generate canonical argument validation from parameter types independently of function body shape. | Invalid UTF-8 and malformed arithmetic Results rejected in identity/unused/unselected cases; evaluator/generated-Go agreement and helper emission. | Pending |
| R6 | P2 | Allocate or check generated names against the entire Go package namespace, including runtime declarations. | Legal source/runtime-name collision examples generate compilable deterministic Go; call targets agree with allocated declarations. | Pending |

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
  language identities; the historical pre-normalization tiny fixture is not used for this numeric
  evaluator comparison;
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

R3 is complete and uncommitted for founder review. R4–R6 remain pending and were not implemented.
No commit, push, publication, worktree, generated-store refresh, or live operation was performed.
