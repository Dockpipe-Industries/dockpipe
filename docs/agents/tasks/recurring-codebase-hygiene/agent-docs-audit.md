# Agent documentation audit — 2026-10-10

## Scope and findings

Reviewed the root/router guidance, all 30 open task summaries and their routing,
plus personal-reference and link scans across the agent documentation tree.
This pass changes documentation and routing only.

- Removed contributor home/checkout paths, personal account names and a specific
  Mac model assumption. Historical locations now use the labels documented in
  [Task maintenance](../../TASKS.md). Exact receipt suffixes, hashes, revisions,
  toolchain versions and meaningful platform evidence remain preserved.
- Reduced TASK-021's global entry from 80 fields to the same four fields used by
  other tasks. Completed metadata remains in its task-local historical record.
- Reduced PipeLang orientation from 41 required documents to two: its current
  overview and mutable-locals objective. Archived the former overview and moved
  historical state out of the active router. Removed repeated milestone preambles
  from the language surface, compiler contract and implementation plan.
- Closed obsolete "ready for execution" guidance for completed verification work,
  corrected the old P04.a active label, and kept proposed v0.115.0 explicitly
  unaccepted. The historical boundary file no longer presents itself as current
  execution authority.
- Closed the stale VM fixture/reusable-workflow-secret blockers in the current
  hygiene summary using its later successful staging delivery record. Old release
  candidates and push/retry instructions remain evidence, not pending operations.
- Fixed two missing package-model routing references, stale PipeLang anchors and
  missing TASK-036 dispatch metadata. Skill routing names repository-curated IDs
  without assuming they are installed on a particular host.
- Consolidated duplicated artifact-freshness guidance and documented portable
  paths, task closure and historical authority in the task-maintenance guide.

## Backlog disposition

No whole task was moved to `closed/`: none of the reviewed summaries established
complete acceptance or an explicit superseding/cancellation decision. Deferred
product proposals were not discarded merely because they are old.

| Tasks | Remaining boundary |
| --- | --- |
| TASK-003 | Retained-volume inspection/pruning and additional failure cleanup |
| TASK-004 | Measured Qt/extension build budgets and invalidation improvements |
| TASK-008 | ForgePipe product implementation and acceptance |
| TASK-009 | General tool resolver/preflight beyond existing test-helper fixes |
| TASK-010 | Trusted installer policy, install-time checks and dependency coverage |
| TASK-011 | Windows guest bootstrap and local CI usability |
| TASK-012 | Startup/provisioning performance and provider-pool work |
| TASK-013 | Remaining App Server and cross-platform acceptance gates |
| TASK-014 | Live Dev Container lifecycle and consumer integration beyond fixtures |
| TASK-016–019 | Deferred game, embedded, MIDI and decentralized-execution proposals |
| TASK-020–022 | Application/service projections and unfinished language foundation; TASK-021 terminal proof still blocked |
| TASK-023 | Native host-sandbox implementation, distinct from closed consumer work |
| TASK-024–033 | Platform support and qualification beyond package publication |
| TASK-034 | Deferred remote-provider research and selection |
| TASK-035 | Recurring hygiene practice, with completed passes kept as evidence |
| TASK-036 | Real worker pairing/delivery acceptance and remaining native integration |

## Verification and limits

- Parsed the agent YAML files and checked file routes, Markdown links/anchors,
  task IDs and global/local dispatch consistency: 686 route references and 363
  Markdown links/anchors passed with no issues.
- Checked changed documentation for personal identifiers and absolute personal
  paths; retained generic examples and actual platform qualification boundaries.
- The repository path guard, seven-skill source listing and whitespace checks
  passed. The earlier public-docs link/shell checks still pass (156 links and
  36 shell blocks). Historical PipeLang state matches the original metadata.
- Retained closed task history, original proof identifiers and active acceptance
  limits. No compiler, engine, package implementation or generated runtime state
  changed. No old verification campaigns were replayed.

Validation receipts and temporary scripts live under `/tmp`; they are not repo
artifacts. The unrelated local `.vscode/` directory was left untouched. This audit
neither commits nor publishes the earlier public-site documentation changes.
