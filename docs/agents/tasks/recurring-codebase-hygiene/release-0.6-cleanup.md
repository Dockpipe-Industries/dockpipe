# 0.6 DDD/DRY cleanup — 2026-10-04

## Scope and disposition

This checkpoint addresses the three architecture/duplication opportunities in
[the 0.6 audit](release-0.6-audit.md#architecture-and-duplication-opportunities).
It builds on the existing uncommitted release and remediation work at
`67e787eb` in the saved `js/pipelang` checkout. It does not certify the whole
repository as debt-free or retire the recurring hygiene program.

### Remote job policy

`src/lib/domain/remote/queue.go`, `job.go`, and `result.go` own admission,
assignment, ownership checks, cancellation, restart recovery, result acceptance,
and artifact bounds. Queue methods evaluate the current snapshot and return
proposed changes without mutating it. The infrastructure broker authenticates,
decodes, serializes, and durably publishes accepted changes before replying.

The retained-ID limit, exact retry behavior, unresolved-assignment blocking,
worker-session ownership, result digests, legacy inline-result migration, and
unknown-commit shutdown behavior remain intact. `BrokerState` retains its
existing exported field types and JSON shape. Enrollment authentication and
credential handling remain in the infrastructure adapter.

### DorkPipe orchestration responsibilities

The package-owned helper now has dedicated files for:

- `planner_proposal.go`: structured proposal parsing and validation;
- `taskpack_contract.go`: contract normalization and executable compilation;
- `softwaredev_artifacts.go`: staging and materializing executable artifacts;
- `promotion_candidate.go`: loading and publishing promotion evidence;
- `promotion_candidate_policy.go`: promotion eligibility and allowed deltas;
- `atomic_json.go`: JSON publication with its existing target checks.

An AST reconstruction check confirmed that all 256 existing helper functions
other than usage-command dispatch and atomic JSON staging retained their exact
formatted declarations. The main helper decreased from 8,189 to 6,086 lines.
The remaining orchestration work is not implicitly authorized for further
reorganization by this checkpoint.

`orchestrationhelper/internal/cloudusage` owns ledger decoding and exact,
nonnegative int64 counter validation. The adapter owns filesystem reads and CLI
output. Unknown ledger fields remain supported; missing or malformed counters
still fail. Both usage commands now also propagate a failed stdout write.

### Shared persistence mechanics and retained contracts

`src/lib/infrastructure/filepublication.Stage` owns only creation of a unique
0600 temporary file, writing, file syncing, closing, and cleanup on failure.
The caller owns the successfully staged path and its eventual publication.

| Caller | Policy retained outside the shared staging helper |
| --- | --- |
| Remote `WritePrivate` | Private directory checks, regular-target checks, JSON serialization, rename, and directory syncing. |
| Orchestration `writeJSONFileAtomic` | Directory creation, symlink-target rejection, indented JSON with trailing newline, and rename. No directory-sync guarantee is added. |
| Promotion `writePromotionBytesAtomic` | Caller-owned target validation and rollback, byte representation, and rename. |
| Backlog `writeTextFileAtomic` | Directory creation and its existing remove-then-rename replacement behavior. This remains distinct from crash-atomic replacement. |
| Durable-state `writePrivateJSONAtomic` | Remains separate: root/reparse validation, platform privacy before writing, platform replacement, and directory syncing require a stronger contract. |
| Edit `writeJSON` | Remains a checked direct write; it is not silently given staging or durability semantics. |

No universal writer or configurable durability flags were introduced. The shared
engine primitive has no package names, workflow knowledge, or provider logic.

## Verification

- Full affected `orchestrationhelper/...` and `cmd/dorkpipe` Go suites passed.
- Remote domain, shared file staging, remote infrastructure, and remote CLI
  suites passed with the race detector and local loopback HTTP fixtures.
- Focused regressions cover admission/retries at capacity, cancellation and
  recovery, conflicting results, wire limits, failed publication, preservation
  of recovery payloads, private-file checks, exact counters, and output failures.
- The orchestration shell lane suite and cloud-usage failure regression passed
  with a freshly built helper and isolated temporary state; live models were off.
- Dockpipe CLI and orchestration helper builds passed for Linux amd64, Windows
  amd64, and macOS arm64. Cross-builds do not establish native runtime behavior.
- Existing helper-declaration reconstruction, formatting, embed-manifest checks,
  whitespace checks, and preservation of unrelated changed files passed.

Builds, test state, logs, and the before-state comparison are under
`/tmp/dockpipe-ddd-cleanup-20261004/`. No generated repository artifacts were
created or refreshed. No credentials, cloud operations, commit, push, or release
were used. Native Windows/macOS execution, hosted release validation, publication
recovery rehearsal, and the deferred PipeLang campaign remain outside this proof.
