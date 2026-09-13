# Bounded large-campaign resume

```yaml
objective_id: pipelang-performance-resume-memory-20260913
state: completed
execution_skill: dorkpipe-objective-execution
execution_authority: user_selected_A_bounded_large_campaign_resume
authorized_objective: Repair large-campaign receipt-loading memory growth while preserving all recovery admission and ordered proof.
done_when: Attribution, bounded receipt-scale recovery, adversarial regressions, fresh interruption/resume proof and canonical documentation pass under unchanged containment.
checkpoint_policy: automatic_within_objective
verification_policy: Frozen retained-receipt controls, focused regressions and fresh representative native audits; no broad compiler campaign unless a failure requires it.
handoff_policy: user_requested_only
terminal_policy: Complete without selecting a successor or committing.
```

User selected A from the decision-only handoff. Automatic pairing remains completed
at saved-checkout HEAD `908d9d6323576ceeaded769c7d44aa058ddbc4e5` on `js/pipelang`.
Admission was clean; both protected stashes remain unchanged. Preserve original
evidence, caches and fixtures. Work is confined to the contained verification
harness and its tests/docs. No production engine or language edits, increased
limits, compression, immutable toolchain reuse, worktree, delegation, cleanup,
commit, push or publication is authorized.

Keep two workers, native/raw execution, fresh semantic/oracle and ordered-audit
proof, normal GC/inlining, 128 MiB/5-second isolated compiler limits and canonical
`job.py` enclosing `run.py`, including the 512 MiB coordinator cap.

Evidence root:
`/home/jamie/.codex/visualizations/2026/09/13/01a0988e-e86c-7e80-a485-9b336144d955/resume-memory`.
The original interrupted campaign is read-only input; copied records used for
loader measurement are historical validation fixtures, never fresh semantic proof.

Source inspection finds full attempt payloads retained in `Campaign.history` and
full accepted receipts retained by `StageRunner.prior`. The interrupted corpus has
4,715 attempts and 4,711 receipts, approximately 53 MB of JSON in each set. The
original OOM allocation site was not sampled; the phase measurements below quantify
retained memory without rewriting that historical diagnosis.

## Implementation and validation

`Campaign` retains only retry IDs, sequences and supersession chains. `StageRunner`
uses payload-bound references after full receipt admission; reuse reloads one
receipt and rechecks all artifact/resource/input conditions. Suite result assembly
uses the same consumer. On-disk evidence, output rows and proof limits are unchanged.

All 42 focused Python regressions passed: 19 campaign, six verification, eight
suite/planner and nine scheduling checks. New coverage includes retained-payload
memory, cross-restart retry chains, overlap/corruption, changed sealed payloads,
artifact drift and current host/input checks before reuse.

The matched 512-attempt historical sample retains identical rows for 510 groups /
795 cases. Traced heap after admission falls from 28.17 to 2.37 MiB. The complete
paired corpus (5,687 suite groups / 8,859 cases) passes recovery, admission,
consumption, serialization and reconciliation at 197.3 MiB process peak RSS under
the unchanged 512 MiB coordinator cap. Its job took 692.242 seconds with zero
OOM/swap and full tree removal. File-cache reclaim still reaches the hard cap;
heap/RSS and cgroup peak are different measurements.
An independent contained comparison also matches every full-corpus result row
against the retained completed suite report, removing only the scheduling index
and resumed marker (`full-row-comparison.json`).

The retained interrupted corpus admits 4,709 groups / 7,276 cases and also passes
at 167.3 MiB process peak RSS, with zero
OOM/swap, unchanged source evidence and complete cleanup. Its job took 574.628
seconds. Both corpus checks include all artifact hashing, result serialization
and terminal reconciliation; they use historical recorded identities as explicit
loader fixtures, without admitting those receipts as current source execution.

All three live containment probes passed. A fresh 48-case native control passed,
then a separate paired run deliberately exited its coordinator with code 19 after
12 committed groups. The enclosing job removed its whole tree. A fresh coordinator
preserved those 12 groups, executed the remaining 18, and matched the control and
retained baseline's exact 48-case order, 294 ordered audits and 294 native children.
There are no missing cases; completed receipt/artifact identities are unchanged.
Control, intentional interruption and resume cost 71.557, 49.374 and 33.547 seconds
respectively; the intentional failure remains separately retained. These are
focused recovery checks, not new full-campaign performance measurements.

`final-validation.json`, `validation-result.json`, `memory-comparison.json`,
`candidate-full/result.json`, `candidate-interrupted/result.json` and separate job
receipts retain proof. All eight retained job trees are absent, with no OOM/swap.
Canonical docs and task indexes are current; source postimages, YAML parsing,
whitespace and protected-state checks pass. No full compiler/matrix/integration/
editor campaign was rerun for this harness-only repair. Generic engine/package
boundaries, language semantics and all native/resource controls are preserved.

The eight retained job durations sum to 1,533.780 seconds; the three separate live
containment tests took 2.601 seconds. These are verification costs, not total
investigation wall time or a campaign speedup. The pre-final-receipt evidence
snapshot has 24,336 files / 339,801,442 logical bytes, including copied historical
records and native validation support; no original bytes were freed.

Source changes are the three contained-execution modules and two regression files;
docs describe behavior, proof and task status. Evidence and copied receipt fixtures
remain outside the checkout. Nothing was deleted or committed; both protected
stashes remain intact. No next performance objective is selected. Revalidation
adds resume I/O; result rows and individual decoded receipts still must fit the
unchanged coordinator cap. This does not promise unlimited receipt sizes.
