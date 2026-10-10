# Nominal enums — completed approved objective

## Objective contract

```text
objective_id: TASK-021-nominal-enums
state: completed
execution_skill: dorkpipe-objective-execution
execution_authority: founder_selected_option_1_then_explicit_approved
language_contract: v0.113.0
selected_package: P01
checkpoint_policy: automatic_within_objective
handoff_policy: user_requested_only
verification_policy: focused cross-layer checks, then complete fresh contained terminal campaign and independent acceptance
```

The founder selected option 1 (enums), then said `approved`. This authorizes implementing the
coherent nominal-enum capability and its required verification in the saved checkout. The completed
foundation planning objective remains evidence; its other proposals are not implicitly approved.

Baseline branch `js/pipelang`, HEAD `9f5abd00fbc21ae2ab23178cc4fbe761903e5167`.
Preserve the 13 uncommitted foundation planning documents, both protected stashes and all ignored
caches/evidence. No worktree, commit, push, publication, cleanup or external service mutation.
Compiler work stays generic; no package-specific engine behavior or app adapter is authorized.

## Concrete scope

- Contextual `Enum` declaration, public nominal type identity, and one or more payload-free members.
  Each member has an explicit nonempty stable string tag: `Enum Mode { Idle = "idle"; Busy = "busy"; }`.
  Tags are unique within the enum and independent of declaration order. No inferred ordinal or
  implicit integer conversion. Member names and tags are distinct; renaming an API member still
  requires normal compatibility handling even if its tag remains unchanged.
- Qualified member values (`Mode.Idle`), exact nominal type equality/inequality, parameters/returns,
  typed immutable locals, same-class pure calls and supported conditional contexts.
  Enum composition uses primitive or enum parameters/results. Conditional depth and enum-match depth
  are each bounded to three; inherited method/terminal-branch local scopes remain intact.
  Existing zero-local block spelling restrictions remain; a single match uses an arrow body.
- Exhaustive `match(value) { Mode.Idle => ..., Mode.Busy => ... }`: exactly one arm per member,
  exactly matching result types, scrutinee evaluated once and only the selected arm evaluated.
  Diagnose duplicate/missing/unknown/cross-enum patterns and invalid bindings.
- End-to-end source/typechecking, typed HIR, independently validated target-neutral Core,
  evaluator, Core-only Go, semantic identities/spans, editor and Application IR consumer evidence.
  Preserve every inherited accepted language version and frozen compatibility.
- Explicitly retain later owners for serialization (P09/P32), enum payload unions (P09), general
  generic containers/carriers (P08–P10), and value/object field generalization (P03/P12). No integer
  casts, flags/bitwise enums, methods/constructors inside enums, mutation, loops or effectful tasks.

The spelling above is the concrete implementation contract selected for this approved capability;
it is accepted as v0.113.0 after the terminal proof recorded below. No claim of F04-wide
serialization completion follows from P01.

## Done when

1. Positive and rejection examples prove nominal identity, declaration/member validation,
   transport/equality, exact exhaustive matching and lazy once-only execution across supported contexts.
2. HIR/Core preserve declaration/member identity and source spans; malformed or forged Core types,
   values, arms and nominal mismatches fail independently of source validation.
3. Evaluator and generated native Go agree with independent value/order oracles; external invalid
   enum values fail at validation boundaries. Generated output is deterministic and Core-only.
4. Source diagnostics, semantic projection, editor/reference, Application IR and task indexes agree.
5. A fresh complete semantic suite, isolated compiler matrix, integration/editor and independent
   acceptance preserve inherited ordered audits/native children and fixed containment. Direct compile
   ceilings remain 128 MiB / 5 seconds; normal GC/inlining, offline Go 1.25.13 and canonical containedexec.

No acceptance-policy optimization is authorized. Focused failures are repaired within this objective;
full proof runs after the final material change, and affected proof is repeated only after drift/failure.

## Implementation checkpoint — historical progress

Source, HIR/Core, evaluator, Core-only Go, semantic projection and editor changes are implemented.
The expanded native/evaluator parity tests exposed and repaired literal `_` tag handling. Direct
compiler scaling exposed switch-validator inlining growth; a local fixed-array loop restores the
unchanged compiler ceiling with normal GC and inlining. All 18 enum resource fixtures (2/8/32 members,
0/1/8/32/128/256 locals) passed the focused scaling run. The Application IR enum consumer and editor
checks passed. Final focused checks include module order/identity and branch-local composition.

Evidence root: `<local-evidence>/2026/09/12/01a09387-ddcf-71d1-94b7-9395966a4500`.
Focused receipts: `enums-focused-7-job.json`, `enums-focused-8-job.json`,
`enums-consumer-1-job.json`; final focused and complete terminal receipts follow below.
Earlier failed attempts remain diagnostic evidence and are not acceptance claims.

Remaining: final integration preflight, fresh complete terminal suite/matrix/integration/editor,
independent inherited ordered-proof comparison, final source/doc/index reconciliation. No full
v0.113.0 acceptance is claimed at this checkpoint.

Final focused checkpoint: `enums-focused-10-job.json` passed the enum suite, all 18 scaling fixtures
and six inherited scalar/carrier partitions. The nine integration checks and editor check passed in
`integration-preflight-job.json`. Harness planner checks passed; mixed harness self-test discovery
requires different caller containment per family. All cases other than the transcript containment
check passed from the host; that check passed separately in `transcript-selftests-job.json`.
No harness implementation or acceptance policy was changed for those environment requirements.

The first full campaign was intentionally interrupted after 99.74 seconds for a final independent
Core review finding: a forged terminal-statement flag inside an expression-only position.
`terminal-job.json` proves aggregate cleanup, zero swap and no OOM. An explicit placement refusal
and nil-child/placement regression now cover that gap. A new fresh `terminal-2` campaign is required;
none of the interrupted campaign's semantic receipts count as final proof. Expected final inventory:
871 compiler test functions, 8859 logical cases, 2922 isolated fixtures, nine integration checks and
one editor check. `terminal-source-postimages.json` pins 33 owned/inherited source anchors.

The final focused run `enums-focused-11-job.json` passed in 38.88 seconds with no OOM/swap and
all child cgroups removed. Fresh `terminal-2` is now running; its live inventory independently
confirms 871 functions and 8859 logical cases. Source inputs must remain fixed until acceptance.

## Accepted completion — v0.113.0

The approved P01 nominal-enum capability is complete. The fresh terminal campaign and independent
acceptance passed: **871 compiler test functions, 8859 logical cases,
2922 isolated compiler fixtures, nine integration checks and one editor check**.
The inherited ordered audit digest, 49465 source/fixture audits
and 49465 fresh native children match the v0.112.0 baseline.
All 33 pinned source anchors match. Isolated compiler maxima were
**88.340 MiB / 1.577 seconds**, within unchanged
128 MiB / 5-second ceilings, with normal GC/inlining. Aggregate and child cgroups were removed;
no OOM or swap occurred. The terminal campaign took 6933.34 seconds and
independent acceptance took 232.42 seconds, totaling
119.43 minutes for final proof.

Receipts under `<local-evidence>/2026/09/12/01a09387-ddcf-71d1-94b7-9395966a4500`:
`terminal-2-job.json`, `terminal-accept-job.json`, `terminal-comparison.json`,
`verification-data/campaigns/terminal-2/accepted-verification.json`,
`terminal-source-postimages.json` and `enum-doc-validation.json`.
Generated binaries, fixture exports, measurements and logs remain in the evidence/cache locations;
no generated artifacts were added to tracked source. Generic compiler/package boundaries are preserved.
The saved checkout remains uncommitted at the admitted HEAD; both protected stashes remain intact.

P01 is complete; F04 serialization and enum container/field integration remain assigned to later
packages. No successor is selected, and no commit, push, publication or worktree action is authorized.


Measured enum-focused verification jobs, including repaired failed attempts, totaled 245.43 seconds. The sum of enum-focused, integration-preflight and both terminal-attempt/acceptance job runtimes was 7623.44 seconds. These are per-job sums, not end-to-end engineering wall time; concurrent jobs may overlap. `enum-verification-cost.json` preserves the individual receipts.
