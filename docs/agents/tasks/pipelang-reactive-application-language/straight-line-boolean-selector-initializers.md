# Straight-line boolean-selector initializers

## Approved objective

- Objective: `TASK-021-straight-line-boolean-selector-initializers`; state: `completed`.
- Founder selected A and separately said exact `approved` on 2026-09-08.
- Baseline: clean saved checkout `js/pipelang` at `9af4a6de17a981182ea736f114616dd5190ccfee`.
- Scope: v0.101.0 admits `T value = (a ? b : c) ? x : y;` in any subset of finite
  explicitly typed immutable-local sequences in public pure straight-line block
  methods, including the inherited omitted-visibility public default. Later locals
  and inherited returns may reuse the binding.
- All selector operands are bool; arms exactly match the declared supported type.
  All five operands use inherited nonconditional expressions and in-scope earlier
  names. Complete primitive, record, list, Optional and Result values travel intact.
  Failed Results do not implicitly propagate.
- Evaluate a once, selected b/c, then selected x/y. Locals remain eager once-only
  and source ordered, including unused bindings. Bindings enter scope after their
  initializer; self/forward references, duplicates, shadowing and escaping names fail.
- Exclude selector initializers before or inside terminal statement trees, new
  statement-condition/argument/matching/propagation placements, further nesting,
  increased depths, private-method admission, inference, mutation, loops, effects,
  backends and performance research.
- Preserve all v0.100 forms, lexical rules, generic internal Core locals, existing
  AST/HIR/Core nodes, compiler/semantic/Application IR identities, frozen 45-source
  compatibility and generic engine/package boundaries.
- Done when parser/typechecker, typed HIR, target-neutral Core, evaluator/Core-only
  Go and executable Application IR agree; independent initializer vectors, ordered
  and lazy traces, dependent/unused locals, computed operands, all supported type
  and carrier families, determinism, malformed and lexical/placement rejection pass.
  Earlier source/Core gates are checked independently; internal Core stays valid.
- Require fresh discovered compiler proof, complete affected Core/HIR/evaluator/
  backend and Application IR checks, compatibility, affected application/CLI/vet/
  editor/docs checks, and fresh isolated scaling through 256 actual locals at
  unchanged 128 MiB / 5-second warm direct compiler ceilings. Distinguish small
  independent vectors from bounded scale vectors; never claim exhaustive arbitrary
  initializer sequences.
- Validation: cached offline Go 1.25.13, existing private caches, canonical temporary
  containment, 30-second units, 25-second children, 700 MiB reclaim, 1 GiB hard,
  800 MiB proactive stop, zero swap, 128 pids, at most two units. Python uses `-B`.
- Execution: `dorkpipe-objective-execution`; automatic in-scope checkpoints and
  user-requested handoff only. Commit, push, publication, cleanup, worktree creation,
  stash mutation, generated-store refresh, installation, credentials and external
  operations remain outside scope.

## Evidence and preservation

Receipts: `/tmp/pipelang-v101-proof`. Preserve all prior receipts, ignored/generated
state, `/tmp/pipelang-performance-proof/cache` and
`/tmp/pipelang-execution-performance/complete-artifacts`.
Both protected stashes match: `26ea507907550d2449dc6f9c81b9942bd52d8629` and
`e3afeea1dad94ca0c63dac434f0873548875bfc5`. All 39 v0.100 source hashes match.
Ignored inventory: 108,167 paths; SHA256
`b39314777ac271ecd1ea58975c53c69983912f932dd95ead0d909b4f98ca2bc2`.
The predecessor bytecode deviation remains historical; preserve current hash
`e631831e38539041b7982cb3553e227155f1b607b1970b04f98747534a07f26c`.


## Completion and reproduction

All 701 freshly discovered compiler tests, fuzz seed functions and examples pass
in 1,724 contained units. Twelve new tests occupy 38 units: 24 independent layout
partitions, four scaling partitions and ten other checks. Source bytes remained
unchanged through the final suite. Nineteen inherited fixtures preserve normalized
HIR/Core/semantic artifacts and exact Go; checked propagation and chain inheritance
explicitly extend through v0.101.

The 24 layouts cross all nonempty subsets of two selector slots, a mixed four-local
sequence, used/unused tails and ordinary/depth-three/selector returns. Each exhausts
512 independently supplied boolean vectors for two initializer selector triples
and the return/depth-three triple: 12,288 evaluator/Go values and generated-Go
ordered/lazy traces. Not every input bit is consumed in every subset. Three dependent
boolean/string initializers add 24 vectors over three strings. Computed checked
Result helpers add 48 vectors, including overflow. Twelve type families and nine
carrier/host-value families pass, including complete failure transport and malformed
host inputs. Repeated HIR/Core/semantic/Go artifacts are deterministic.

All 100 earlier source and Core contracts independently reject the new initializer
placement. Thirty-two malformed-Core cases, 27 excluded source bodies, two private
source refusals and five hidden-local operand probes pass. Generic internal Core
locals remain valid. All v0.100 coverage remains in the fresh discovered suite.

All 64 fresh isolated scaling cases pass. Eight are zero-local inherited controls;
56 contain 1/8/16/32/64/128/256 actual selector initializers across four nonconditional
arm families and used/unused tails. The 512 result vectors share selector input bits
across each scaled sequence; this is not arbitrary independent assignments at every
local. Maximum direct compiler RSS is 75.94140625 MiB and elapsed time is
0.2914381449809298 seconds, within unchanged 128 MiB / 5-second ceilings with normal
inlining. Zero-local controls have closure depth two; actual sequences have fixed
depth three (one local wrapper plus two choices), without growth from one through
256 locals. All 320 nonmeasurement exports match focused fixtures exactly; all 384
final fixture files, including measurements, are hashed.

Complete Core/HIR/evaluator/backend, Application IR consumer, frozen 45-source
compatibility, affected application, complete CLI and compiler/consumer/application/
CLI vet pass. The executable consumer checks six strings and eight selector vectors
and preserves canonical Application IR. Editor assertions/syntax, JSON/YAML/Python,
new documentation links, formatting and planner partitions pass. Removing only the
v0.101 additions restores all eight changed production files exactly to baseline.
Evaluator and Go backend production code are unchanged; generic engine/package
boundaries and public identities remain intact.

All 1,846 recorded temporary cgroups are removed, with no max/OOM or swap-event
increases and zero observed swap. Suite/integration limits remain 700 MiB reclaim,
1 GiB hard memory, 800 MiB proactive stop, zero swap, 128 pids, 30-second units and
25-second children; at most two units ran concurrently. Isolated measurements use
the canonical matrix defaults.

Retained failed attempts document an incomplete test-file import during the first
build, a 30-second build timeout, missed inherited HIR version admission for list/
Optional/Result types, and an inappropriate arrow-helper closure baseline in the
new initializer scaling fixture. These were corrected without broadening source
scope, removing coverage or raising compiler/resource ceilings. The one-local
scaling baseline follows the existing backend's fixed wrapper emission. Failed
attempts are not terminal acceptance.

Reproduce with `python3 -B tests/containedexec/pipelang_suite.py`,
`--go "$GO_TOOLCHAIN"`,
`--cache /tmp/pipelang-performance-proof/cache`,
`--compiled-cache /tmp/pipelang-execution-performance/complete-artifacts`,
`--shape-batch-size 1 --workers 2 --audit-generated` and a fresh `--output` directory.
Then run `python3 -B tests/containedexec/matrix.py` against its `fixtures-v101`,
using the same cache, a fresh output and the cached toolchain's direct compiler.
Use `python3 -B` for imports/checks. Preserve all receipts and existing caches.

Key receipts under `/tmp/pipelang-v101-proof`: `terminal/summary.json`,
`terminal/suite.json`, `terminal/source-hashes.json`, `terminal/fixtures-v101`,
`isolated/matrix.json`, `scale-fixture-hashes.json`, `production-inheritance-audit.json`,
`integration2.json`, `docs-checks.json`, `final-evidence.json` and
`final-source-hashes.json`. This durable record owns acceptance if temporary receipts
expire. Task-owned temporary fixtures/build artifacts and native/compiler cache
entries were created; no generated repository store was refreshed or migrated.

The saved checkout remains `js/pipelang` at
`9af4a6de17a981182ea736f114616dd5190ccfee`. Owned changes are unstaged and uncommitted.
Protected stashes, ignored inventory and current predecessor bytecode hash match.
Full repository build/CI/all-library tests, sustained fuzzing, interactive editor,
non-Linux containment and live operations were not run. No successor is selected.
Commit, push, publication and cleanup remain separate decisions.
