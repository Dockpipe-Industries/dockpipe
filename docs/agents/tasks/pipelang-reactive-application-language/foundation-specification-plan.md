# Foundation specification and dependency planning objective

## Objective contract

```text
objective_id: TASK-021-foundation-specification-and-dependency-plan
state: completed
execution_skill: dorkpipe-objective-execution
execution_authority: explicit_user_request_to_plan_and_start
authorized_objective: Specify F01-F34, dependency-sized delivery, verification economics and the app-start milestone.
done_when: Complete capability ledger, proposed contracts/examples, dependency slices, milestone estimates, receipt-based costs, consistent routing and founder choices pass documentation validation.
inherited_invariants: Accepted v0.112.0 proof; frozen compatibility; Core-only backend; fixed verification limits; generic engine/package boundaries; protected checkout state.
explicit_exclusions: New language/runtime/editor implementation, adapters, benchmark campaigns, policy weakening, commit, push, publication, external mutation, cleanup, worktree and automatic handoff.
checkpoint_policy: automatic_within_objective
verification_policy: Read existing proof; inspect relevant source; validate documentation links, coverage, dependencies, indexes and diff. No broad compiler rerun for docs.
handoff_policy: user_requested_only
context_pressure_policy: warn_and_continue
checkpoint_output_policy: quiet_success_bounded_failure
terminal_conditions: completed | blocked | failed_verification | cancelled
```

## Admission

Saved checkout `<checkout>`, branch `js/pipelang`, HEAD
`9f5abd00fbc21ae2ab23178cc4fbe761903e5167`; staged, unstaged and untracked state empty.
Protected stashes `26ea507907550d2449dc6f9c81b9942bd52d8629` and
`e3afeea1dad94ca0c63dac434f0873548875bfc5` present. Preserve ignored caches and proof.
All 19 entries in the preceding objective's `terminal-source-postimages.json` match
current files. Existing accepted v0.112.0 receipts are admitted without replay;
[the completion record](arrow-selector-value-arms.md) owns their scope and location.
The later checkpoint commit supersedes its historical uncommitted-state prose.

Owned work is canonical foundation planning documentation and directly related TASK-021
routing/status and TASK-020/TASK-022 dependency links. No executable semantics are accepted
by this planning objective. Founder selection and implementation approval remain pending.

## Checkpoints

1. Admission and objective record established; accepted proof and protected anchors checked.
2. Completed the F01–F34 evidence/disposition ledger and C1–C8 contract proposals with worked
   positive/rejection examples; D1–D6 remain explicit founder choices.
3. Completed 33 dependency packages totaling 52–84 proposed foundation slices, including minimum
   library and M-app; separate compiler port/bootstrap is 10–19. Costs use admitted v0.112 receipts.
4. Documentation validation passed; founder implementation choices are P01 enums (recommended),
   P04 general blocks/mutable locals, or P03 nested values. None is selected.


## Deliverables and completion proof

- [Inventory](../../../concepts/pipelang-foundation.md): current baseline and required capabilities.
- [Contract proposals](../../../concepts/pipelang-foundation-contracts.md): object/type, control flow,
  memory/sharing, effects/tasks, synchronization, state/contracts, replay/tooling and profiles.
- [Delivery plan](../../../concepts/pipelang-foundation-delivery.md): every capability's evidence,
  deliverables, dependencies and disposition; bounded slice owners/acceptance, graph, estimates,
  verification economics and recommendation-first implementation options.
- TASK-021 main/local indexes and current entrypoints route to this result. TASK-020/TASK-022
  dependency links now name the proposed M-app milestone; their implementation remains unapproved.

Validation artifact:
`<local-evidence>/2026/09/12/01a09387-ddcf-71d1-94b7-9395966a4500/foundation-doc-validation.json`.
Checks cover all 34 unique inventory/ledger rows, all 33 P packages and five B packages, exact
slice-range arithmetic, the 34-node/58-edge acyclic summary graph, new/changed Markdown links,
209 task route/document paths, parsed YAML and synchronized dispatch/approval metadata.
`git diff --check` passes. All 19 accepted source hashes, HEAD/branch, empty staging and both
protected stashes remain unchanged. No ignored cache or old evidence was edited or removed.

The routed `./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --list` check stopped
before rendering because the sandbox cannot chmod `<dockpipe-state>` (read-only
filesystem). This is an unavailable host launcher check, not a documentation or compiler failure.
No skill source/routing IDs changed; YAML paths and referenced installed skill IDs were checked
read-only. No host permission change or generated skill refresh was attempted. Broad compiler,
editor and runtime execution were deliberately not repeated for this documentation-only change;
the admitted implementation proof is unchanged. No live target/concurrency behavior is newly proven.

Owned scope is 13 documentation/index files. The only new output outside the checkout is the local
validation JSON; a temporary validation script lives under `/tmp`. No generated artifacts were added
to source, and engine/package boundaries are preserved. Changes are uncommitted. User-requested
transport was received; no further task/handoff was created.

Planning is complete. Proposed semantics and milestone dispositions await founder review, including
single-base/value semantics, sharing enforcement, lifecycle races, synchronization/queue scope and
profile/physical-target boundaries. M-app before compiler bootstrap is a recommendation, not accepted
ordering. The prior v0.112 approval does not authorize any successor source changes.
