# Arrow-method boolean selectors

## Approved objective

- Objective: `TASK-021-arrow-boolean-selectors`; state: `completed`.
- Founder selected A and separately said exact `approved` on 2026-09-08.
- Baseline: clean saved checkout `js/pipelang` at `7f5eeb7bfbd7e739e6a576a7c2f80047ad968f0d`.
  All 40 v0.99 source hashes, protected stashes and ignored inventory matched.
- Scope: v0.100.0 admits exactly `=> (a ? b : c) ? x : y;` in public pure
  expression-bodied methods. The selector operands are bool; value arms exactly
  match supported types. All five operands use inherited nonconditional expressions.
- Evaluate a once, selected b/c, then selected x/y. Preserve complete primitive,
  record, list, Optional and Result values without implicit propagation; inherited
  eager once-only source-ordered caller locals, including unused reached locals.
- Preserve all v0.99 forms, lexical rules, generic internal Core locals, existing
  AST/HIR/Core nodes, compiler/semantic/Application IR identities, frozen 45-source
  compatibility and generic engine/package boundaries. Core erases arrow/block syntax.
- Exclude new initializer, statement-condition, argument, matching or propagation
  placements, further nesting, increased depths, inference, mutation, loops, effects,
  backends and performance research.
- Done when arrow/block normalized HIR/semantic and exact Core/Go agree; all eight
  selector vectors, independent values/lazy traces, computed operands, supported
  types/carriers, determinism, malformed/source exclusions and internal Core probes
  pass. All 99 earlier source contracts reject the arrow spelling; equivalent block
  Core stays admitted in v0.98/v0.99, with earlier Core refusal preserved.
- Require fresh discovered compiler proof, executable Application IR, compatibility,
  affected application/CLI/vet/editor/docs/format checks and isolated fixed-helper
  caller scaling through 256 actual locals at unchanged 128 MiB / 5-second warm
  direct compiler ceilings. No claim of arbitrary independent initializer vectors.
- Validation: cached offline Go 1.25.13, existing private caches, canonical temporary
  containment, 30-second units, 25-second children, 700 MiB reclaim, 1 GiB hard,
  800 MiB proactive stop, zero swap, 128 pids, at most two units. Python uses `-B`.
- Execution: `dorkpipe-objective-execution`; automatic in-scope checkpoints and
  user-requested handoff only. Commit, push, publication, cleanup, worktree creation,
  stash mutation, generated-store refresh, installation, credentials and external
  operations remain outside scope.

## Evidence and preservation

Implementation and bounded terminal verification pass. Receipts are retained under `/tmp/pipelang-v100-proof`.
Preserve `/tmp/pipelang-performance-proof/cache`,
`/tmp/pipelang-execution-performance/complete-artifacts` and all prior receipts.
Protected stashes: `26ea507907550d2449dc6f9c81b9942bd52d8629` and
`e3afeea1dad94ca0c63dac434f0873548875bfc5`.
Ignored inventory: 108,167 paths, SHA256
`b39314777ac271ecd1ea58975c53c69983912f932dd95ead0d909b4f98ca2bc2`.
The predecessor refreshed the existing planner bytecode without saving its original
bytes; that historical deviation remains. Preserve its current SHA256
`e631831e38539041b7982cb3553e227155f1b607b1970b04f98747534a07f26c`.

## Completion and reproduction

All 689 freshly discovered compiler tests, fuzz seed functions and examples pass in
1,686 contained units. Thirteen new tests occupy sixteen units, including four
independent memory partitions. Source bytes stayed unchanged throughout the final
suite. Eighteen inherited fixtures preserve normalized HIR/Core/semantic artifacts
and exact Go; all v0.99 coverage remains in the discovered suite. Inherited checked
propagation and chain checks now explicitly extend through v0.100.

Two arrow/block forms cross three input strings and all eight selector vectors
(48 independently expected results). Normalized HIR and semantic identities/types
match; Core and generated Go match exactly. Semantic normalization removes only
source declarations and source hashes; each spelling's actual source hash is checked
separately. Six zero/one/four-local caller layouts cross used/unused tails and all
64 independent selector/initializer vectors (384 values and ordered traces).
Computed checked-Result helpers supply another 48 vectors, including overflow and
unused eager caller bindings. Twelve type families and nine carrier/host-value
families pass, including malformed host values and complete failure transport.
Repeated artifacts remain deterministic. Existing node and public schema identities
are unchanged. No evaluator or Go backend production code changed.

All 99 earlier source versions reject the new arrow spelling. All 97 pre-v0.98 Core
contracts independently reject its representation; v0.98 and v0.99 still validate,
generate and evaluate the equivalent block Core. Malformed Core/source probes retain
placement, depth, type and scope refusal. Five hidden-local operand probes preserve
valid generic internal Core while refusing public placement. New selector arrows
require public visibility, including the inherited omitted-visibility public default;
explicitly private new selector arrows are refused by the parser. All earlier source
visibility behavior remains unchanged.

All 64 scaling fixtures cross four nonconditional arm families, used/unused tails
and 0/1/8/16/32/64/128/256 actual caller locals, exercising eight selector vectors
per fixture (512 results). The arrow helper stays fixed and caller invocations share
input bits; this is not arbitrary independent assignments across a local sequence.
Closure depth stays two. Fresh isolated direct compiler maxima are 22.24609375 MiB
RSS and 0.021360965038184077 seconds, under unchanged 128 MiB / 5-second ceilings
with normal inlining. All 320 nonmeasurement exported files match the focused
fixtures; 384 final fixture files, including measurement receipts, are hashed.

Complete Core/HIR/evaluator/backend, Application IR consumer, frozen 45-source
compatibility, affected application, complete CLI and compiler/consumer/application/
CLI vet pass. The executable consumer checks six strings and eight boolean vectors
and preserves the canonical Application IR projection. Editor assertions/syntax,
JSON/YAML/Python, local documentation links, formatting and planner partition checks
pass. The production audit removes only v0.100 inheritance, admission and diagnostic
additions and restores all eight changed production files exactly to baseline.

All 1,795 recorded temporary cgroups are removed, with no max/OOM or swap-event
increases and zero observed swap. The final suite and integrations use 700 MiB soft
reclaim, 1 GiB hard memory, 800 MiB proactive stop, zero swap, 128 pids, 30-second
units and 25-second children; at most two units run concurrently. Fresh isolated
compiler measurements use the canonical matrix defaults. Earlier focused attempts
used canonical hard limits before the prescribed soft reclaim option was restored.

Retained failed receipts document missed version-inheritance gates, unsupported
fixture operands, zero-local caller spelling, an inappropriate standalone consumer
call probe, source-metadata normalization, an unused parser variable and the inherited
implicit-public default. Corrections did not broaden admitted operands or remove
coverage. An affected-application build stopped proactively at 811.8203125 MiB;
its successful retry restored the prescribed 700 MiB reclaim option without raising
hard limits or changing compiler flags. The sandbox preflight/build started no
workload; host checks used reviewed canonical containment. Failed attempts are not
terminal acceptance.

Reproduce with `python3 -B tests/containedexec/pipelang_suite.py`, cached offline
Go 1.25.13, `--cache /tmp/pipelang-performance-proof/cache`,
`--compiled-cache /tmp/pipelang-execution-performance/complete-artifacts`,
`--shape-batch-size 1 --workers 2 --audit-generated` and a fresh output directory.
Then run `python3 -B tests/containedexec/matrix.py` against its `fixtures-v100`
using the same cache and cached toolchain compiler. Use `python3 -B` for imports
and checks. Retain all receipts and caches; no generated repository store was refreshed.

Key receipts: `terminal/summary.json`, `terminal/suite.json`,
`terminal/source-hashes.json`, `terminal/fixtures-v100`, `isolated/matrix.json`,
`scale-fixture-hashes.json`, `production-inheritance-audit.json`, `integration.json`,
`docs-checks.json`, `final-evidence.json` and `final-source-hashes.json` under
`/tmp/pipelang-v100-proof`. This durable record owns acceptance if temporary receipts
expire. New native/compiler-cache entries and task-owned temporary fixtures/build
artifacts were created; no cleanup or cache migration was performed.

The saved checkout remains `js/pipelang` at `7f5eeb7bfbd7e739e6a576a7c2f80047ad968f0d`.
Owned changes are unstaged and uncommitted; the protected stashes, ignored inventory
and current predecessor bytecode hash still match. No successor is selected.
Full repository build/CI/all-library tests, sustained fuzzing, interactive editor,
non-Linux containment and live operations were not run. Commit, push, publication,
cleanup and any successor remain separate decisions.
