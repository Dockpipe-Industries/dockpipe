# Automatic pair scheduling for complete verification

```yaml
objective_id: pipelang-performance-default-pairing-20260912
state: completed
execution_authority: user_requested_outright_implementation_after_accepted_pair_comparison
authorized_objective: Enable measured pairing by default for complete campaigns with verified profiles and singleton fallback.
done_when: Default integration, profile lifecycle regressions and fresh representative ordered audits pass; canonical docs and task state are current.
verification_policy: Admit prior full scheduler proof; validate changed orchestration with focused regressions and fresh representative executions.
checkpoint_policy: automatic_within_objective
handoff_policy: user_requested_only
terminal_policy: Complete without selecting another objective or committing.
```

The user requested implementation after the completed [pair comparison](pair-scheduling-performance.md).
Use the saved checkout and preserve both protected stashes and existing owned docs.
The existing planner, two workers, native execution, independent oracles, fresh
compiler proof and all resource limits remain unchanged. Large-resume memory
repair, streaming, immutable toolchain reuse and language changes are outside scope.

Implementation: complete campaigns select a saved root profile or baseline hint,
fall back to singletons for absent/stale/invalid hints, and atomically save fresh
complete-suite observations. Explicit profiles remain strict and the complete
driver has a singleton override. Repeated paired runs carry only verified original
singleton measurements for successful cases, rechecking identity and executable
digests on each run. Standalone suites retain their singleton default.

Evidence is retained under
`/home/jamie/.codex/visualizations/2026/09/12/01a09603-329e-74d1-a913-8619e5e68b64/default-pairing`.
Prior full proof remains historical: harness edits intentionally invalidate its
profile identity. No profile identity is rewritten and no prior receipt is claimed
as execution of the modified orchestration.

## Validation accounting

The first external lifecycle validator incorrectly required identical pair counts
on the second paired run. All three suites passed their exact ordered audit and
fresh-native comparisons; refreshed singleton timings legitimately increased the
last plan from 18 pairs to 19. Its final assertion failed after 216.330 seconds,
with aggregate cleanup and zero OOM/swap. That failed wrapper remains separately
retained as `validation-job.json`; no runner implementation was changed to satisfy it.
The corrected validator checks preservation of the original singleton measurements
for grouped cases and repeats the lifecycle under `confirmed/`, with a fresh job
receipt. This is validation-script recovery, not a scheduler failure or performance
measurement. No full compiler campaign is rerun for this default-orchestration change.

## Accepted result

All 22 regressions passed (13 existing and nine new). The successful fresh lifecycle
used 48 singletons, then 18 pairs plus 12 singletons, then 19 pairs plus 10 singletons.
Every run preserved the exact 48-case order, 294 ordered audits and 294 fresh native
children against the accepted baseline, with zero reused groups. Original admitted
singleton measurements survived both paired runs; refreshed singleton observations
can legitimately change later eligibility. The successful job took 192.349 seconds
with zero OOM/swap, unchanged resource limits and complete tree removal. This is
focused correctness evidence, not a new estimate of full-campaign speedup.

`confirmed/results.json`, `confirmed-job.json` and `final-validation.json` retain
successful proof. Independent final checks rehashed source identities, accepted all
unit resource receipts, verified cleanup and confirmed the two protected stashes.
YAML parsing and whitespace checks pass. The earlier full 49,526-audit scheduler
proof supports the unchanged grouping mechanism; no new full compiler, integration
or editor campaign was run for this orchestration-only change.

Changed source is confined to `tests/containedexec`: campaign/suite profile wiring,
scheduling admission and observation persistence, focused tests and runner usage.
Canonical runtime/research docs and both task indexes are current. No language,
production engine or package behavior changed; generic boundaries are preserved.
Generated reports, fixtures and retained build support stay in the external evidence
and existing cache roots. Nothing was deleted. Work remains uncommitted on the saved
checkout; no push, publication, worktree, delegation or further objective occurred.
The large-resume memory defect was unresolved at this checkpoint. Its subsequent
user-selected repair is tracked in [bounded resume memory](bounded-resume-memory.md).
