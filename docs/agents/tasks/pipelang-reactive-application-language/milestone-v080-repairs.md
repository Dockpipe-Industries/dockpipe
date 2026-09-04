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
| R1 | P1 | Preserve computed Result carriers when evaluating call arguments; expected failure reaches the callee unless explicit propagation is authored. | Nested arithmetic and bounded Results, success/failure, recovery with a different caller return type, copied values, evaluator/generated-Go agreement. | Complete, uncommitted |
| R2 | P1 | Memoize semantic-to-HIR dependency lowering across one shared dependency graph. | Each reachable method lowered once, shared acyclic graph scaling, deterministic order/output, cycle rejection retained. | Pending |
| R3 | P2 | Enforce supported compiler/language identities and feature-version gates at Core program admission. | Unknown identities independently rejected; representative downgraded features rejected consistently by Core, evaluator, and backend. | Pending |
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
temporary under `/tmp`. The repair and documentation are uncommitted. R2–R6 are still pending;
R2 is the next repair slice. Current checkpoint: R1 is ready for founder review; no later slice
has started.
