# Resumable verification optimization

## Objective contract

```yaml
objective_id: TASK-021-resumable-verification-optimization
state: complete
execution_skill: dorkpipe-objective-execution
execution_authority: explicit_user_request
authorized_objective: Implement and verify the complete PipeLang verification optimization architecture.
checkpoint_policy: automatic_within_objective
handoff_policy: user_requested_only
verification_policy: focused checkpoint proof followed by one complete optimized terminal campaign after final material edits
terminal_policy: complete only after architecture capabilities and unchanged acceptance are verified
```

On 2026-09-10 the user requested: "Lets optimize this fully then create an
architectural plan to achieve all of these goals and then once documented
... to execute", explicitly invoking `dorkpipe-task-handoff`. This authorizes
documentation here and ordinary local implementation/verification in one fresh
task. No second implementation approval is needed. Handoff is transport only.

## Canonical plan and first checkpoint

Read [the architecture](../../../runtime/pipelang-verification.md), then the
current [contained runner contract](../../../../tests/containedexec/README.md).
The architecture owns scope, invariants, acceptance, estimates and all five
checkpoints. Do not load the large historical performance journals on admission.

First implement durable campaign manifests, dependency fingerprints and atomic
per-unit receipts, using small synthetic units to prove crash consistency and
invalidation. Admit the completed v109 proof as baseline; no baseline suite rerun
is required to start. Proceed through the remaining checkpoints in the same
objective after the first succeeds. Do not stop after journaling alone.

Implementation belongs in the verification harness, test-only generated helpers
and docs. The language contract remains v0.109.0. All existing semantic coverage,
independent checks, compiler-resource acceptance and shared/per-unit containment
ceilings are preserved. No commit, push, worktree, cleanup of preserved caches or
proof, actual reboot, publication or external service mutation is authorized.

## Evidence and protected state

- Saved checkout: `/home/jamie/source/dockpipe`, branch `js/pipelang`, HEAD
  `f599a20e2f652f2e33645cba222bbf5d8d256475` at planning admission.
- [Completed v109 record](terminal-combined-selector-arms.md) remains accepted.
  Receipt root: `/home/jamie/.cache/pipelang-v109-resume-20260909`.
  `accepted-verification.json` reconciles 810 functions/6,187 units, two focused
  retries, nine integration checks and 1,944 fresh isolated compiler cases.
  Original suite failures 881/883 and application preparation timeout are
  retained evidence; successful continuations resolved them.
- Durable planning evidence root:
  `/home/jamie/.codex/visualizations/2026/09/09/01a08830-6111-7632-b63f-0de5a2ac9a56/pipelang-verification-optimization`.
  `measurements/` preserves the six profiling attempts, logs and generated
  artifacts copied from `/tmp/pipelang-v109-verification-analysis`.
  `handoff-state.json` captures current ownership, hashes, index, ignored
  inventory identity and protected stashes. Recheck affected live anchors.
  Copied receipts retain original command/path provenance; use the copied
  relative files for inspection, not the vanished `/tmp` paths after reboot.
- All pre-existing implementation changes remain uncommitted. Extend owned
  harness paths in place; do not restore the saved checkout or erase its v109
  production/test/editor/docs changes. No active verification job is transferred.
- Preserve stashes `26ea507907550d2449dc6f9c81b9942bd52d8629` and
  `e3afeea1dad94ca0c63dac434f0873548875bfc5`, all ignored state and original
  receipts/caches. The task-owned `tests/containedexec/__pycache__/job.cpython-310.pyc`
  from v109 is also retained; its presence does not authorize cleanup.

## Completion update

Receiver updates this record and both task indexes with checkpoint evidence and
the final measured outcome. Keep the public architecture/current runner commands
in sync, clearly distinguishing implemented behavior from remaining proposals.
The original language slice stays complete and no successor language feature is
selected by this objective.

## Execution evidence

Checkpoint 1 in progress. Receiver confirmed all 36 inherited postimages, HEAD,
branch, index, stashes and 108168 ignored paths. The status digest matches when
using the handoff format (`git status --porcelain=v1 -z`). Original completed
receipts were admitted without replay. New evidence lives under the receiving
task visualization directory; no protected files were removed.

Checkpoint 1 passed: 11 synthetic ledger/input-guard tests, including all four
atomic publication windows, actual process death, writer contention, stale lock
metadata, boot-only changes, corrupt receipts, lost artifacts, cleanup rejection,
input restoration and dependent-stage invalidation. Three live aggregate probes
passed. Checkpoint 2's focused suite discovered all 810 functions, ran one special
harness check, and a fresh coordinator resumed with zero workload units replayed.
Live jobs completed in 3.13 s / 1.92 s and both cgroup trees were removed. Matrix,
integration, cache, scheduling and terminal adoption evidence remain pending.

Checkpoint 2/3 implementation now includes all-stage orchestration, isolated matrix
and integration/editor drivers, transitive module discovery, content-verified build
objects, cache-specific preparation archives and preservation-first disk admission.
The real coordinator crash probe exited 19 after one committed unit; a fresh capped
coordinator accepted it and executed only the second unit (one execution each).
Five final per-unit containment probes passed. The first 12-case cold sample found
a nested-memory receipt matcher defect and expensive repeated disk scans; both
were repaired. The next warm sample passed all 12 cases in 29.71 s including setup,
with 623427584-byte shared peak and zero high/max/OOM/swap events. The test binary
was reused, while all 12 semantic units executed. The original failed sample is
retained. Final matched scheduling/timing checks, stage resumption probes and the
complete terminal campaign remain required; this is not completion evidence.


Checkpoints 1–4 focused acceptance passed. Final synthetic coverage is 13 ledger
checks, five verification/scheduling/budget checks and four legacy planner checks.
The final bytecode-safe unit runner passed all five live containment probes.
Changed harness inputs caused all 18 sample matrix cases to execute in a new
revision; an unchanged follow-up reused all 18 with zero execution. The matched
suite resumed nine groups covering all 12 cases with zero execution. Integration
resumption separately reused all nine checks, preparation and the editor check.

The final two-worker matched sample took 22.238 s as 12 singleton units and
20.431 s as nine groups. Flattened logical case order, 84 ordered source/fixture
audits, 84 native executions and 27 artifact uses were identical. Retain only the
three measured warm pairs; unknown, heavy, memory and special cases stay single.
Earlier telemetry-on/off measurements showed no resolved positive overhead;
wall-clock differences are noisy and nested phase totals are not additive.
Evidence: `terminal-batching-acceptance.json`, `telemetry-comparison.json`,
`matrix-invalidation-job.json`, `matrix-resume-job.json` and
`suite-final-resume-job.json` under the receiving task visualization directory.

Protected-state exception: standalone containment probes launched Python without
bytecode suppression, adding ignored `campaign.cpython-310.pyc` and regenerating
the existing ignored `job.cpython-310.pyc`. Both remain in place; original bytecode
contents were not captured, so byte-for-byte restoration is not claimed. No ignored
paths were removed. Both the unit runner and its probe now pass `-B` to child
Python. `ignored-state-incident.json` records this incident.

Checkpoint 5 is running: `terminal-job.json` supervises
`verification-data/campaigns/terminal/` under the unchanged shared limits. The
complete discovered inventory is 811 functions / 6,188 logical cases (one added
preparation-cache regression check). Source is frozen for this campaign. Final
adoption still requires full suite, 1,944 fresh isolated compiles, nine integration
checks, editor proof, aggregate cleanup and independent receipt reconciliation.


## Terminal adoption — 2026-09-10

All five checkpoints are complete. The supported all-stage driver executed the
complete suite after the final material harness edits: 811 functions / 6,188
logical cases, all fresh, no failures, no linked retries required. The additional
function is the preparation-cache corruption regression; all 810 / 6,187 baseline
functions/cases remain covered. All 1,944 fresh isolated compiler cases passed
(maximum 71.980 MiB RSS and 0.985 s), with the unchanged 128 MiB / 5 s ceilings and
no matrix memory.high. Nine integration checks and the editor check passed.

The stage job took 4,729.18 s and peaked at 1,612,947,456 bytes. Zero swap and OOM
events were recorded. Reclaim/high/max counters remain visible; coordinator
file-cache reclaim did not exceed the shared hard ceiling. The supervisor proved
cgroup removal. A separate 172.33-second capped audit rehashed current dependencies,
required artifacts and complete inventories, then published
`verification-data/campaigns/terminal/accepted-verification.json`. Its own cgroup
was also removed. `terminal-job.json` and `terminal-acceptance-job.json` retain the
actual commands, environment policy, counters and cleanup proof.

Suite wall time fell from 10,802.49 s to 4,297.08 s (60.22%, 108.42 minutes saved).
The terminal run used 13,041 verified executable hits and no misses, with 43,725
fresh native children; the original run populated most executables. This is a
warm retained-cache workflow result, not a cold or pure batching comparison.
The full campaign used singleton fallback because the baseline binaries differed
from the matched sample's digests. The matched 12-case sample independently saved
8.13% through three safe pairs. The terminal run emitted matching warm singleton
profiles for future bounded grouping. Logical suite serialization fell from the
estimated 81.14 GB to 153.40 MB; this does not measure physical disk traffic.
`terminal-comparison.json` preserves these numbers and qualifications.

Final protected-state verification found no unexpected inherited-file changes.
HEAD, branch, index, both stashes and baseline receipt identities are unchanged.
No ignored paths were removed. The bytecode incident described above remains the
only recorded ignored-state exception, retained without claiming restoration.
`protected-state-final.json` contains the read-back. All work remains uncommitted.
No worktree, push, cache cleanup, reboot, machine configuration, external mutation
or successor language selection occurred. Engine/package boundaries are preserved.

The canonical architecture and both task indexes now record completion. The
language contract remains v0.109.0. No required implementation or verification
work remains in this objective.

Final documentation validation passed: `git diff --check`, Python AST parsing,
YAML completion-state checks, local documentation links, proof-input separation
for the final docs-only updates, and both completed job cgroup absence checks.
