# Verification performance round two

## Objective contract

```yaml
objective_id: TASK-021-verification-performance-round-2
state: completed
execution_skill: dorkpipe-objective-execution
execution_authority: explicit_user_request
authorized_objective: Profile the remaining verification cost, implement the strongest bounded improvement, and adopt or reject it using matched measurements.
done_when: Matched controls justify retained benefit, focused regressions pass, and one complete terminal campaign plus independent acceptance reconciles unchanged coverage.
checkpoint_policy: automatic_within_objective
verification_policy: profile first, matched samples, focused regressions, complete terminal campaign and independent audit
handoff_policy: user_requested_only
context_pressure_policy: warn_and_continue
checkpoint_output_policy: quiet_success_bounded_failure
terminal_conditions: completed | blocked | failed_verification | cancelled
```

The user requested “clean worktree lets do one more performance optimization round”
and a fresh task. This completes that one performance objective. The preceding
[resumable verification objective](conformance-verification-optimization.md) and
v0.109.0 language slice remain complete. No successor was selected or created.

## Scope and protected state

Used the saved `/home/jamie/source/dockpipe` checkout on `js/pipelang`, admitted clean
at `3154030bdaa22ef8a5ef6570088e8528180719ff`. Index, stashes
`26ea507907550d2449dc6f9c81b9942bd52d8629` and
`e3afeea1dad94ca0c63dac434f0873548875bfc5`, all 108169 ignored paths and the inherited
acceptance receipt matched the handoff. Preserve the predecessor bytecode incident;
this round used `python3 -B` and performed no cache cleanup.

Only canonical documentation and the two task indexes changed, plus this record.
The implementation is adoption of the existing verified profile-driven pair
configuration for full unchanged warm campaigns. Compiler and harness source,
engine/package boundaries and every resource ceiling remain unchanged. No commit,
push, worktree, cache deletion, reboot, machine configuration or external mutation
was performed. The user-requested transport was admission to this task only.

## Decision and completed proof

The [canonical results](../../../runtime/pipelang-verification.md#measured-schedule-adoption--performance-round-two)
own measurements, caveats and current usage. Adopt measured pairing as an explicit
option for full unchanged warm campaigns; retain singleton execution by default.
No second scheduler, hash shortcut or production compiler change was introduced.

The admitted timing profile identified artifact identity/verification as the
largest measured avoidable cost. Exact input/host/policy/native-binary matching
admitted 2297 pairs plus 1594 singletons. The isolated copy-buffer experiment
remains unretained because its roughly 7% hash-only gain did not justify selecting
it over full per-process hashing/launch amortization in this bounded round.

The six 27-case samples preserved exact ordered proof. The two serial warm-build
controls and two sample-profile pair runs were inconclusive (pairs averaged 1.34%
slower); no small-workload benefit is claimed. The initial control included build
preparation and briefly overlapped the small exploratory microprobe, so it was
excluded from the serial warm-build comparison. A full-profile sample separately
exposed the entire admission cost. All observations remain preserved.

The complete candidate suite executed all 6188 cases / 811 functions in 3891 groups,
without retries. Independent acceptance then rehashed current dependencies and
required artifacts and reconciled the complete inventory against prior proof.
All 43725 ordered source/fixture audits and fresh native children match exactly;
13041 executable hits / zero misses are unchanged. All 1944 fresh isolated compiler
cases, nine integration checks and editor proof passed. Compiler maxima were
71.949 MiB / 0.666 s under unchanged 128 MiB / 5 s ceilings. Both capped job trees
were removed; zero OOM and swap events were recorded.

The suite saved 7.03%. Job plus independent acceptance saved 5.76%
(282.56 s) against the admitted warm control. This includes full-profile
admission, preparation, suite reconciliation and independent acceptance. Background
host load was not controlled; no cold-cache result is claimed. The earlier prefix
workload regression and noisy samples remain evidence, not discarded observations.

Focused checks passed: 13 campaign/identity/recovery tests, five verification,
scheduling and budget tests, and four planner tests. Accepted live containment
and invalidation proof remained applicable because no harness source changed.
The terminal campaign used the unchanged shared and per-unit limits throughout.
Final documentation and protected-state checks are recorded in `final-checks.json`.

## Durable evidence

Current root:
`/home/jamie/.codex/visualizations/2026/09/10/01a08c0e-03d5-7212-ad5f-51f9a86eaff9`.

- `selection.json`, `hash-probe.go`, `hash-probe.output`, `hash-probe.json`:
  predeclared sample and unretained allocation experiment.
- `control*`, `paired*`, `matched-comparison.json`, `prefix-profile.json`:
  every sample, timing caveat and exact sample proof comparison.
- `terminal-job.json`, `verification-data/campaigns/terminal/`: complete fresh
  suite, compiler fixtures, matrix, integration/editor receipts and timings.
- `round2-accept.py`, `terminal-acceptance-job.json`, `terminal-comparison.json`:
  independent acceptance, exact ordered comparison and cleanup.
- `verification-data/campaigns/terminal/accepted-verification.json`,
  `final-adoption.json`, `final-checks.json`: accepted coverage, limited adoption,
  total costs and final protected-state/documentation checks.

Predecessor root:
`/home/jamie/.codex/visualizations/2026/09/10/01a089a6-9615-79f1-bfb2-fd72e289205d`.
Its `round-two-handoff-state.json` and accepted terminal campaign were admitted
without rerunning the baseline full suite. Original caches and all receipts,
failures and ignored state remain preserved. No required work remains in this round.
