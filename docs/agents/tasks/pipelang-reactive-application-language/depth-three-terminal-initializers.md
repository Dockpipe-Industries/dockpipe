# Depth-three initializers throughout terminal trees

## Approved objective

- Objective: `TASK-021-depth-three-terminal-initializers`; state: `completed`.
- Founder selected A and separately said `approved` on 2026-09-08.
- Baseline: clean saved `<checkout>`, `js/pipelang` at
  `f4c00e56d99d83d7568fb5372a03d2bc9a8d5d13`. Completed v0.96 and shared-framework
  acceptance are admitted; the performance objective is closed.
- Scope: v0.97.0 admits complete depth-three ternary initializers in any subset of
  finite explicitly typed immutable-local sequences at root, intermediate and leaf
  scopes of inherited terminal if/else trees through statement depth three. Either
  or both initializer arms may nest. Later locals, descendant conditions and ordinary
  or inherited depth-three returns may reuse earlier in-scope bindings.
- Preserve exact types, bool conditions, complete supported values/carriers, eager
  once-only source-ordered initialization including unused locals, selected lazy
  conditions/arms/branches, lexical scope and independent source/Core refusal.
  Preserve parser/typechecker -> typed HIR -> target-neutral Core -> evaluator/Core-only
  Go, executable Application IR, frozen 45-source compatibility, public identities,
  internal Core capabilities and generic engine/package boundaries.
- Done when all 25 initializer and statement shapes pass an explicitly bounded
  mixed/dependent placement matrix with independent condition vectors and value/trace
  oracles; types/carriers, scope, refusals, inherited artifacts and consumer checks
  pass. Verify direct compiler scaling through 256 locals with unchanged 128 MiB /
  5 second warm ceilings, affected compiler/consumer/application/CLI, vet, editor,
  authored data, docs and format checks. Do not claim arbitrary independent matrices.
- Validation: cached offline Go 1.25.13, existing private cache, canonical
  `tests/containedexec/run.py`, 30-second units, 25-second children, 700 MiB high,
  1 GiB hard, 800 MiB proactive stop, zero swap, 128 pids, at most two units.
  Sandbox user-manager access is unavailable; use narrow reviewed host execution.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints;
  user-requested handoff only. Implementation is approved; commit/push are not.
- Exclude depth-four expressions/statement trees, new condition/argument or
  matching/propagation placements, inference, mutation, loops, effects, new backends,
  performance research, worktree, stash mutation, cache cleanup/migration, commit,
  push, publication, generated-store refresh, installs, credentials and external operations.

## Evidence

The approved v0.97.0 slice is complete. Temporary receipts are retained under
`/tmp/pipelang-v097-proof`; this durable record owns acceptance if temporary files expire.


## Resource acceptance repair

The first terminal run exposed direct compiler RSS failures at 256 locals with full
both-arm depth-three initializers: intermediate scope 131,256/132,724 KiB and
leaf scope 155,268/157,028 KiB (used/unused). The 128 MiB ceiling remains fixed.
The owned coordinator stopped dispatching after this failure; all 525 dispatched
unit reports prove their cgroups removed. This partial run is retained as failure
and unaffected-case evidence, not terminal acceptance.

Generated Go enclosed the whole local sequence in one closure per terminal branch.
A structural Core-only backend predicate now selects the existing function-level
statement emitter for deep initializer sequences in terminal scopes. It keeps
single-local and shallower inherited emission unchanged, preserves lazy branches
and eager ordered locals, and introduces no source-version or package knowledge.
The repair passes focused, full-suite and isolated compiler verification below.


## Completion and reproduction

All 649 discovered compiler tests, fuzz seed functions and examples pass in
1506 final contained units with no source drift during that run. All eleven new
v0.97 tests pass. The matrix covers all 25 statement shapes and all 25 initializer
shapes across 418 supplied layouts and 569,344 independent expected-value
vectors in the evaluator and pristine generated Go, plus instrumented Go value and
ordered/lazy trace checks. Repeated typed HIR, Core, semantic projection and exact
Go bytes are deterministic.

The matrix rotates initializer shapes across root/intermediate/leaf local sequences,
crosses every subset of three local slots with used/unused final bindings, and adds
all scope-class subsets plus two five-local mixed layouts on the final shape. Each
supplied layout exhausts 128 initializer vectors and both return bits for one
representative of every reachable statement path. Seven statement bits, seven shared
initializer bits and the return bit are separate. Initializer bits are shared among
locals; this is not the Cartesian product of independent choices at arbitrary length.

Twelve supported type families, nine carrier/host-value families and computed checked
arithmetic Results pass. Fifteen inherited source fixtures retain normalized HIR/Core/
semantic bytes and exact Go. All 96 earlier source versions and all 96 earlier Core
versions independently reject the newly admitted placement. Source/Core checks reject
malformed types, depth, placement, scope, hidden locals, identities and downgrades;
evaluator/backend refusal is checked. Well-formed generic internal Core locals remain
valid while the public placements reject them. Selected bool/string locals control
reached descendant conditions; sibling escapes are checked with distinct Core lexical
positions. Executable Application IR keeps its canonical projection and exercises
helper outcomes across six strings and eight boolean vectors.

All 192 new direct initializer scale fixtures pass the regression tests and fresh
isolated normal-inlining compiler measurements through 256 locals. The fixed matrix
crosses four initializer shapes, root/intermediate/depth-three leaf placement,
used/unused final bindings and 1/8/16/24/32/64/128/256 locals. It grows one sequence;
other branches return the original input. Peak isolated compiler RSS is
106.593750 MiB; maximum isolated elapsed time is
0.618360 seconds, below unchanged 128 MiB / 5 second ceilings.
Regression compiler peak is 107.406250 MiB and maximum elapsed is
2.337501 seconds. Observed closure depths are
[2, 3, 4, 5, 6, 7]; no sequence exceeds its one-local baseline. Deep sequences now
use the existing statement emitter at the function boundary. All 1,152 exported
fixture file hashes remain identical across isolated measurements.

Core, evaluator, Go backend, typed HIR, Application IR, frozen 45-source compatibility,
affected application tests, complete CLI and compiler/consumer/application/CLI vet pass.
Editor assertions/syntax, JSON/YAML, task routes, local documentation links, formatting
and diff checks pass. Planner tests confirm 100 disjoint layout partitions and twelve
memory partitions, with exact inventory preserved under grouping. An audit removing
only successor admission and the bounded backend routing repair restores all ten
changed production files exactly to HEAD. Public identities and HIR/Core shapes remain
unchanged; generic engine/package boundaries are preserved.

All 2283 recorded temporary cgroups, including failed attempts and the stopped
partial suite, are removed. No memory max/OOM or swap event increases occurred and
observed swap is zero. Every final workload used cached offline Go 1.25.13, canonical
`run.py`, 30-second units, 25-second test children where applicable, 700 MiB high,
1 GiB hard, 800 MiB proactive stop, zero swap, 128 pids and at most two units.

Reproduce using `tests/containedexec/pipelang_suite.py` with the cached toolchain,
existing private Go/compiled caches, `--shape-batch-size 1 --workers 2 --audit-generated`.
The README routes `TestV970DepthThreeTerminalInitializersLayouts=100` and the runner
exports its twelve memory partitions into `fixtures-v097`. Run each exported direct
compiler command in a fresh canonical `run.py` unit, retaining `--memory-high-mib 700`.
The task-owned `matrix_high.py` is the canonical `matrix.py` with only its launcher
path resolved and this temporary reclaim flag added; `matrix-wrapper.json` binds both
scripts. No ceilings or compiler flags were relaxed.

Key receipts: `terminal-repaired/summary.json`, `terminal-repaired/suite.json`,
`terminal-repaired/source-hashes.json`, `isolated/matrix.json`, `scale-fixture-hashes.json`,
`integration.json`, `production-inheritance-audit.json`, `docs-checks.json`,
`planner-checks.json`, `final-source-hashes.json` and `final-evidence.json`.
Failed development receipts remain: initial unused import/version-list mistakes,
missing inherited dispatch/signature routing, explicit GOENV cache preflight, corrected
Core mutation targets and prior diagnostic expectations, a layout deadline resolved
by disjoint partitioning, and the compiler RSS failure corrected above. A straight-line
closure assertion was made placement-aware while retaining local-count and compiler
resource limits. No failed attempt is counted as terminal acceptance.

The checkout remains `js/pipelang` at `f4c00e56d99d83d7568fb5372a03d2bc9a8d5d13`;
changes are unstaged and uncommitted. Both protected stashes are preserved. Existing
ignored paths and caches are retained; the syntax check added one ignored file,
`tests/containedexec/__pycache__/pipelang_suite.cpython-310.pyc`. Excluding that new
file restores the inherited 108,166-path inventory and SHA-256
`ba9e796198c62a673750896380d10484150d1fad2e496d9ca08a82e496b3b690`.
This checks inventory, not ignored file contents. Native caches gained validated
entries and task-owned build/evidence files; zero cleanup or migration was performed.
Generated repository stores were not refreshed. No full repository build/CI/all-library
suite, sustained fuzzing, interactive editor, non-Linux containment or live operations
were run. Commit, push, publication and successor selection remain separate.
