# TASK-015 Backlog-Driven Remote Tasks And Multi-Machine Execution

## Goal

Provide package-owned DorkPipe workflows for two related but distinct remote-execution paths:

1. execute one explicitly selected, decision-ready backlog item as a bounded remote Codex task;
2. execute one DorkPipe task graph across user-owned DockPipe nodes and compatibility surfaces.

The first path turns an item from `docs/agents/task-index.yaml` and its linked task document into a
bounded remote Codex task. The second schedules implementation, validation, repair, and aggregation
without turning DockPipe into a cluster scheduler. They share immutable dispatch/result artifacts and
approval discipline, but neither path depends on the other.

The remote Codex task is the authority for asynchronous task state. DorkPipe retains only immutable
dispatch and result artifacts; it must not depend on a recurring master process, prose continuity
memory, or a shared-worktree inference loop to resume work after interruption.

## Why This Is A Separate Task

TASK-007 shipped the generic software-development workflow and repo task-pack model. This task
applies that model to the DockPipe standardized backlog and a remote-task adapter. TASK-013 remains
the separate host-resident App Server path for interactive top-level sessions; this task must not
couple remote Cloud task lifecycle to that adapter.

## Proposed Contract

### Backlog input

- `docs/agents/task-index.yaml` remains the single open-only entrypoint.
- The first workflow requires an explicit task ID and bounded slice reference. It reads only that
  index entry, its linked task document, `AGENTS.md`, and the docs routed for the selected task type.
- It must reject closed, absent, malformed, ambiguous, externally active, or decision-blocked items
  before any remote submission.
- A future `--next` selector is allowed only after the standardized backlog records deterministic
  readiness and ownership metadata. It must never infer readiness from historical prose updates.

### Remote execution

- A package-owned Codex Cloud adapter submits exactly one task through the installed CLI's remote
  task surface, with an explicitly selected Cloud environment and branch.
- The compiled request contains the baseline commit, allowed paths, hard boundaries, task slice,
  linked source-of-truth files, and required validation. It must not serialize local secrets,
  generated state, broad unreviewed workspace context, or a resolved prompt as durable repo config.
- DorkPipe records a stable dispatch artifact containing the remote task ID, request fingerprint,
  selected environment/branch reference, and safe submission metadata. Remote status, diff, and
  result retrieval are keyed by that ID instead of a local master-agent state file.
- Remote creation is explicit. Polling/status is read-only. Applying a remote diff, checkpointing,
  publishing, or starting a new item remains a separate governed action with the existing approval
  and Git lifecycle boundaries.

### Completion and reconciliation

- Treat a provider or agent terminal signal as an untrusted `completion_candidate`, never as an
  authoritative completion or permission to mutate the local checkout.
- A host-side package adapter records the candidate against the dispatched remote task ID, rejects
  stale, duplicate, replayed, or mismatched events, and deterministically verifies the expected
  result/status, allowed artifact references, required validation receipts, diff boundary, and
  absence of pending approval or halt state before emitting `ready_for_review`.
- For a host-resident App Server session, obtain the signal from the supervised provider adapter;
  do not give the agent an arbitrary MCP lifecycle-control tool. For a Cloud task, reconcile its
  remote task ID through the provider's status/diff/result surface (or a proven signed callback),
  without exposing the local MCP server to the remote worker.
- `ready_for_review`, local apply, checkpoint, and publish remain distinct operation-result and
  approval transitions. A completion candidate can never trigger apply or publish directly.

### Workflow shape

1. `backlog.inspect` resolves and validates one selected task/slice.
2. `backlog.compile` materializes a reviewable task request artifact.
3. `backlog.dispatch` submits one remote task and records its identifier.
4. `backlog.status` and `backlog.diff` reconcile only the recorded remote task.
5. `backlog.apply` requests explicit approval, applies the reviewed remote diff locally, and then
   delegates checkpoint/publish to the runtime-owned Git lifecycle.

The implemented vertical slice stops after `inspect`, `compile`, compatibility preflight,
fixture-backed `dispatch`, untrusted `completion_candidate` ingestion, one fixture-backed remote
status observation, one fixture-backed remote diff observation, and one fixture-backed opaque remote
result observation, followed by one fixture-backed opaque validation-receipt observation and one
artifact-only mechanical patch-structure and allowed-path boundary verification plus isolated
temporary-copy mechanical application and isolated execution of the immutable required-validation
declaration, followed by one explicit fixture-backed local semantic-review decision. Only an
affirmative decision bound to passed validation emits a separate readiness artifact. A second,
separately identified fixture-backed local decision may then authorize one exact rollback-safe
application to the consumer checkout and emit an `applied_for_review` receipt. A third separately
identified local decision may authorize submission of one exact-path request to the generic
DockPipe runtime, which independently verifies the runtime session, Git state, and approved
postimages before creating one checkpoint and receipt. It does not create a scheduler, live-poll,
infer any approval from provider/result/receipt/validation evidence, auto-push, sync, publish,
merge, select another task, or create a cross-task orchestrator.

## Current Status (2026-07-24)

The first vertical slice is implemented as the package-owned `backlog.remote` workflow and dedicated
orchestration-helper commands:

- `backlog.inspect` requires one exact `TASK-NNN`, one trimmed single-line bounded slice, and one
  exact baseline commit. It strictly loads `docs/agents/task-index.yaml`, resolves one exact linked
  document, verifies that document's heading matches the selected task ID, and records source
  digests in `backlog-selection.json`.
- `backlog.compile` writes deterministic `dorkpipe.remote-request/v2` JSON and matching markdown.
  Their shared fingerprint binds the selected task/path/slice/baseline, explicit environment and
  branch refs, allowed paths, hard boundaries, required validation, context sources, and a separate
  complete validation-input manifest. `source_files` remains request/context evidence,
  `scope.allowed_paths` remains patch-write scope, and only `validation_input_files` grants bounded
  source authority for a later validation workspace.
- The validation-input declaration is a non-empty, caller-authored, ordinally sorted list of exact
  repository-relative regular files. It permits no directories, globs, inferred walks, whole-
  checkout copies, links/reparse points, root escapes, generated paths, secrets, Git internals, or
  provider-private paths. Every file and ancestor is inspected fail-closed; each entry records path,
  SHA-256, and bytes; and the request records complete-list semantics, count, aggregate bytes, and a
  canonical aggregate fingerprint. The contract caps the list at 256 files and 8 MiB.
- `backlog.dispatch` is fixture-only. It writes `remote-task.json` with one opaque task ID, request
  and compatibility fingerprints, environment/branch refs, deterministic fixture time, and adapter
  identity. The artifact records `provider_invoked: false` and no status, diff, result, apply,
  commit, push, or publication capability.
- `backlog.completion_candidate` ingests one strict fixture with separate candidate and replay
  identities, source adapter identity, remote task ID, request and dispatch fingerprints, explicit
  environment/branch refs, canonical observation time, and exactly one untrusted `completed` claim.
  Its reviewable `completion-candidate.json` has only `state: completion_candidate`, records the
  terminal claim as untrusted, and leaves every retrieval, validation, review, apply, commit, push,
  and publication transition false.
- Artifact-only follow-up and candidate ingestion validate the immutable request, compatibility,
  and dispatch artifacts; neither rereads the checkout or prose state.
- `backlog.status` ingests one strict fixture with separate observation and replay identities bound
  to the accepted candidate identity and full candidate fingerprint plus the immutable task,
  request, dispatch, adapter, environment, and branch identity. The canonical observation time must
  be later than both dispatch and candidate observation times. `remote-status.json` records only
  untrusted, non-authoritative fixture status evidence, remains at `state: completion_candidate`, and
  leaves `ready_for_review`, diff/result retrieval, validation, apply, commit, push, and publication
  false.
- Artifact-only status retrieval revalidates the immutable request, compatibility, dispatch, and
  complete accepted candidate artifact; it does not reread the checkout or backlog prose.
- `backlog.diff` ingests package-owned fixture metadata plus adjacent opaque patch bytes. The
  observation/replay identity binds the accepted status observation identity and canonical
  fingerprint, accepted candidate identity and canonical fingerprint, remote task ID, request and
  dispatch fingerprints, adapter identity, and explicit environment/branch refs. Its canonical
  observation time is later than dispatch, candidate, and status times. Accepted
  `remote-diff.json` remains at `state: completion_candidate`; `remote-diff.patch` contains the exact
  untrusted fixture bytes whose declared SHA-256 was checked before either artifact was written.
- Artifact-only diff retrieval revalidates the request, compatibility, dispatch, complete candidate,
  and complete status artifacts without rereading backlog prose or the consumer checkout. It treats
  patch bytes as opaque evidence and performs no semantic or allowed-path verification.
- `backlog.result` ingests one package-owned fixture observation bound to the accepted diff
  observation identity and canonical fingerprint, exact accepted patch SHA-256 and byte count,
  accepted status observation identity and canonical fingerprint, accepted candidate identity and
  canonical fingerprint, and the immutable task/request/dispatch/adapter/environment/branch
  identity. Its deterministic observation time is later than dispatch, candidate, status, and diff.
  `remote-result.json` records only an opaque, untrusted, non-authoritative, uninterpreted fixture
  result string and remains at `state: completion_candidate`; validation-receipt retrieval, review,
  semantic interpretation, validation, apply, commit, push, and publication remain false.
- Artifact-only result retrieval revalidates the full immutable request, compatibility, dispatch,
  candidate, status, diff metadata, and exact accepted patch bytes without rereading backlog prose or
  the consumer checkout. Fixture fields are explicitly package-owned and are not a provider response,
  callback, signed receipt, hidden transcript, or undocumented Codex contract.
- `backlog.validation_receipt` ingests one package-owned fixture observation bound to the accepted
  result observation identity and canonical fingerprint, accepted diff/status/candidate identities
  and canonical fingerprints, exact accepted patch SHA-256 and byte count, immutable task/request/
  compatibility/dispatch/adapter/environment/branch identity, and the exact `required_validation`
  declaration plus its canonical fingerprint. Its deterministic observation time is later than
  dispatch, candidate, status, diff, and result.
- `dorkpipe.validation-receipt/v2` records only an opaque, untrusted, non-authoritative,
  uninterpreted
  fixture receipt and remains at `state: completion_candidate`. It records the required-validation
  declaration and validation-input fingerprint as immutable request evidence with `executed: false`
  and `interpreted: false`; review,
  validation execution, apply, commit, push, and publication remain false. Fixture fields are
  explicitly package-owned and are not a provider response, callback, signed receipt, hidden
  transcript, or undocumented Codex contract.
- Artifact-only validation-receipt retrieval revalidates the complete immutable chain through the
  accepted result and exact patch bytes without rereading backlog prose or the consumer checkout.
- `backlog.patch_boundary` writes `dorkpipe.patch-boundary/v2` after revalidating
  `remote-request.json`, exact `remote-request.md`, adapter
  compatibility, dispatch, candidate, status, diff metadata and exact patch bytes, result, and
  validation receipt before writing deterministic `patch-boundary.json`. The artifact binds the
  canonical accepted receipt/result/diff/status/candidate identities and fingerprints, patch
  SHA-256 and byte count, request/compatibility/dispatch fingerprints, remote task and adapter
  identities, environment/branch refs, exact immutable `allowed_paths` declaration and canonical
  fingerprint, and sorted changed paths. It remains only at `state: completion_candidate`.
- The accepted grammar is deliberately narrow: unquoted `diff --git a/<path> b/<path>`, one ordinary
  non-submodule `index` line, exactly matching `--- a/<path>` and `+++ b/<path>` headers, and one or
  more ordinary unified-diff hunks. Hunk contents are opaque. Add/delete, combined, binary,
  submodule, mode-only, rename/copy, quoted/escaped, repeated-path, mismatched-header, malformed, and
  otherwise unsupported forms fail closed.
- Patch and allowed paths are verified lexically without Git, checkout, or filesystem-existence
  consultation. Paths must be canonical forward-slash repository-relative paths with no absolute
  or drive prefix, backslash, traversal, empty component, control/whitespace, ambiguous quoting, or
  generated, secret-like, Git-internal, or provider-private location. A changed path must equal one
  immutable allowed path or begin with that path plus `/`; prefix collisions do not match.
- `backlog.patch_application` requires that exact verified boundary artifact, revalidates the full
  immutable chain before reading source files, and uses only its sorted `changed_paths`. Each source
  path is joined beneath an explicit consumer root and must be an existing regular non-symlink file
  with no linked ancestor or root escape. Only those files are copied into a new temporary directory.
- The package-owned application engine accepts the same ordinary unified text-modification sections
  and additionally validates hunk coordinates, declared old/new line counts, ordering, overlap, and
  exact context/removal preimages. It applies additions only inside the temporary copy. Source text
  must be LF-terminated UTF-8 without CR or NUL bytes; no-newline markers are rejected as unsupported
  because this slice does not define an unambiguous end-of-file reconstruction rule.
- `dorkpipe.patch-application/v2` binds the canonical boundary fingerprint; receipt/result/diff/status/
  candidate identities and fingerprints; exact patch checksum and byte count; immutable request,
  compatibility, dispatch, task, adapter, target, and baseline declarations; sorted changed paths;
  canonical per-file preimage and postimage manifests; and deterministic file/hunk counts. It records
  only successful mechanical `temporary_copy_only` application and verified cleanup. It explicitly
  denies semantic review, validation execution, checkout mutation, `ready_for_review`, apply to the
  checkout, commit, push, and publication, and remains at `completion_candidate`.
- `backlog.validation_execution` requires the exact accepted `patch-application.json`, revalidates
  the complete immutable chain before source access, and writes deterministic
  `dorkpipe.validation-execution/v1` evidence. A valid existing artifact is accepted after
  artifact-only revalidation without the consumer checkout or command re-execution; a malformed,
  tampered, or non-identical artifact is rejected and never overwritten.
- A fresh validation workspace contains only the exact union of the request's 99 declared validation
  inputs and the boundary's sorted changed-path overlay. Every input is reread from the consumer root
  with canonical-path, regular-file, link/reparse, containment, byte-count, and SHA-256 checks; the
  overlay preimage and reproduced postimage must match `patch-application/v2` exactly.
- Validation accepts only canonical direct `go test ./<exact-package-path>` argv declarations. It
  rejects shell quoting, flags, recursive patterns, metacharacters, redirection, pipelines,
  environment assignment, absolute paths, and traversal before launch. Commands run sequentially,
  bounded and offline, and stop at the first nonzero result.
- `validation-execution.json` binds request, compatibility, dispatch, task, adapter, target,
  baseline, validation-input, required-validation, receipt, boundary, patch-application, and exact
  patch identities; sorted changed paths; exact workspace authority; deterministic per-command argv,
  status, and exit code; aggregate status; cleanup success; and a canonical artifact fingerprint.
  It contains no timestamps, durations, output text, environment state, consumer/temporary paths, or
  host identity. Passed and failed evidence both remain at `completion_candidate`; validation success
  is explicitly non-authoritative and cannot imply semantic approval or `ready_for_review`.
- `backlog.semantic_review` consumes only a strict package-owned
  `dorkpipe.semantic-review-decision-fixture/v1` input. It requires one exact `approved` or
  `rejected` decision, the bounded `semantic_correctness_of_bound_candidate` scope token, separate
  decision/replay identities, and exact task/request/compatibility/dispatch/adapter/target/baseline,
  candidate/status/diff/result/receipt/boundary/application/execution, patch, changed-path, and
  validation-status bindings. It contains no reviewer identity, path, timestamp, environment
  snapshot, secret, or review prose.
- Before accepting the decision, the helper revalidates the complete immutable chain through the
  exact `validation-execution.json` and patch bytes. Passing validation alone creates nothing;
  approved review of failed validation is rejected. A rejected decision may bind passed or failed
  validation but never creates readiness.
- `semantic-review-decision.json` uses `dorkpipe.semantic-review-decision/v1`, remains at
  `completion_candidate`, records the explicit local decision and canonical artifact fingerprint,
  and grants no checkout apply, commit, push, publication, or next-task authority.
  `ready-for-review.json` uses `dorkpipe.ready-for-review/v1` and is emitted only for an approved
  decision bound to passed validation. It transitions only to `state: ready_for_review`, binds the
  accepted decision fingerprint and complete candidate identity, and grants no further capability.
- Existing decision/readiness artifacts are accepted only after artifact-only canonical
  revalidation. Rejected decisions restart without creating readiness; duplicate decision IDs,
  replayed replay IDs, changed decisions under an accepted identity, conflicting artifacts, and any
  chain or derived-artifact tampering fail closed without overwrite or repair.
- `backlog.checkout_application` consumes only a canonically valid `ready-for-review.json` and one
  strict `dorkpipe.checkout-application-approval-fixture/v1` decision. It requires separate approval
  and replay identities, one exact `approved` or `rejected` value, the fixed
  `accepted_changed_paths_exact_patch_once` scope, and exact readiness, semantic decision,
  execution, application, boundary, receipt, result, diff, status, candidate, task, request,
  compatibility, dispatch, adapter, target, baseline, patch, changed-path, and consumer-preimage
  bindings. Readiness and semantic approval alone cannot authorize checkout mutation.
- `checkout-application-approval.json` uses
  `dorkpipe.checkout-application-approval/v1`, records the explicit bounded local decision and a
  canonical artifact fingerprint, and grants only the one exact application when approved. Commit,
  push, publication, checkpoint, sync, and next-task capabilities remain false. A rejected decision
  is deterministic, performs no mutation, and cannot create a successful application receipt.
- Approved application revalidates the complete immutable chain and exact patch bytes before source
  access, then requires every accepted path to be an existing contained regular non-link/reparse
  file. The complete consumer set must match either the exact accepted preimage manifest or the exact
  accepted postimage manifest; stale, unexpected, and mixed sets fail closed without repair.
- Before first mutation the helper derives every exact postimage from the accepted patch, verifies
  it against `patch-application/v2`, and prepares same-directory postimage and rollback files with
  the original supported mode. It replaces only accepted changed paths, verifies all resulting
  bytes, removes temporary/rollback files, and writes no receipt until verification and cleanup
  succeed. Any in-process failure after mutation begins restores and verifies every exact preimage;
  rollback and cleanup failures are distinct and never successful.
- `checkout-application.json` uses `dorkpipe.checkout-application/v1`, records package-local
  `state: applied_for_review`, and binds the approval fingerprint, complete readiness identity,
  exact pre/post manifests, sorted applied paths, file/hunk counts, supported mode preservation,
  cleanup, and consumer verification. It explicitly grants no further apply, commit, push,
  publication, checkpoint, sync, or next-task authority.
- Restart is deterministic: a valid receipt requires the complete exact postimage set; an approved
  artifact without a receipt may recover the same receipt only from the complete exact postimage
  set; the complete preimage set performs the approved operation once; mixed/unknown state rejects;
  and rejected approval restarts without mutation. Duplicate/replayed approval identities, changed
  decisions, conflicting artifacts, and tampered upstream or derived artifacts reject without
  overwrite.
- `backlog.checkout_checkpoint` consumes only a canonically valid `checkout-application.json` plus
  one strict `dorkpipe.checkout-checkpoint-approval-fixture/v1` decision. The package revalidates the
  complete immutable chain through checkout application, exact patch bytes and fingerprints,
  sorted changed paths, and every exact consumer postimage before it records
  `dorkpipe.checkout-checkpoint-approval/v1`.
- The checkpoint decision binds the exact task/request/adapter target/baseline/candidate chain,
  checkout-application approval and receipt, consumer postimage manifest, runtime session/workspace
  and branch, expected parent commit, bounded message, and fixed
  `accepted_postimages_exact_runtime_checkpoint_once` scope. A rejected decision creates neither a
  request nor a commit; the package grants only permission to submit one runtime request.
- Approved decisions emit provider-neutral `dockpipe.session-checkpoint-request/v1` with a canonical
  fingerprint, authorization fingerprint, runtime session/workspace identity, exact branch and
  parent, sorted paths, exact postimage manifest, bounded message, and the fixed
  `exact_paths_one_checkpoint` runtime scope. Package helpers and workflow assets invoke no raw Git.
- The generic `dockpipe session checkpoint` runtime operation rejects the wrong session, workspace,
  branch, parent, staged state, extra modified/deleted/renamed/untracked path, unsupported change
  kind, stale/missing/non-regular/linked/reparsed/escaping postimage, and any staged/postimage race.
  It stages only the accepted paths, creates one request-fingerprint-bound checkpoint commit, and
  verifies its exact parent, path set, blobs, trailers, branch, and clean workspace.
- `dockpipe.session-checkpoint-receipt/v1` binds the request and authorization fingerprints,
  checkpoint/session/workspace identities, branch, parent, resulting commit, exact paths and
  postimages, runtime ownership, and false push/publication/sync/merge actions. Runtime checkpoint
  metadata records the same request, parent, branch, workspace, and path bindings.
- Restart is fail closed: a valid receipt is revalidated against the exact live commit; an absent
  receipt may recover only when HEAD is exactly the requested child commit. Pre-commit failure
  restores the previously empty index and leaves HEAD unchanged. Commit, metadata, and receipt
  failures are distinct; metadata/receipt failure after commit preserves the exact commit for
  deterministic recovery and never reports success prematurely.

The validation-source binding was added after the prior application slice exposed a contract
contradiction: the request's `source_files` contained only context documents, `allowed_paths` bound
only patch-write scope, and the accepted README-only patch could not authorize or bind the Go and
module inputs required by `go test ./packages/dorkpipe/lib/orchestrationhelper`. The checked fixture
now explicitly names the module metadata, target package sources/tests, local Go dependency sources,
embedded schema, exact Go workspace/module manifests, two package-owned Example Brain contract
files read by the target tests, and minimal root-embed matches required to assemble that command
workspace. No directory walk, glob, dependency discovery, whole-checkout copy, Git query, or backlog
prose expands that authority.

The package proof rejects absent, malformed, unknown, and ambiguous IDs; malformed index entries;
missing, escaping, mismatched, or closed linked task paths; empty, whitespace-padded, multiline, or
otherwise malformed bounded slices; invalid baselines; and explicitly blocked or externally active
fixture entries. Rejected inspection writes a deterministic rejection code but no request or
dispatch artifact. Temporary consumer copies prove repeated-run determinism, no consumer mutation,
no live provider invocation, no Git/SSH/network tool invocation, and no live status polling,
live diff/result polling, result interpretation, semantic approval, apply, commit, push, or
publication. Validation-receipt polling and interpretation are also absent; only the immutable
required-validation declaration is executed inside the bounded temporary workspace.

The canonical index remains unchanged and open-only. Package-owned fixtures use an optional
`dispatch_state` solely to represent `blocked`, `external_active`, and `closed` deterministically.
Standardizing readiness plus ownership metadata is still required before any future `--next`
selector; prose remains non-authoritative.

The installed CLI documents `codex cloud exec --env <id> --branch <branch> [query]`, but its help
does not expose a machine-readable submission receipt or a stable task-ID response schema. Parsing
undocumented terminal text would not be a safe resumable identity contract, so live submission
remains unimplemented.

The package now has a fail-closed compatibility preflight before fixture dispatch. Its narrow tracked
fixtures record the exact read-only `codex-cli 0.144.1` surfaces from `codex --version`,
`codex cloud --help`, and `codex cloud exec --help`. The resulting
`remote-adapter-compatibility.json` is bound to the immutable `remote-request.json` fingerprint and
explicit environment/branch refs. It records the required command surface, documented
`--env <ENV_ID>` and optional `--branch <BRANCH>` inputs, snapshot digests, receipt/task-ID support,
compatibility status, enabled adapter modes, and the exact fail-closed gap. That CLI version
documents neither a machine-readable submission receipt nor a safely recoverable stable opaque task
ID, so compatibility is `unsupported`, live submission is disabled, and fixture dispatch remains
the only enabled adapter.

Temporary consumer proofs show repeated compatibility and completion-candidate artifacts are
byte-for-byte deterministic; the preflight itself leaves no `remote-task.json`; and malformed
contracts emit deterministic operation-result failure evidence. Completion ingestion rejects an
observation at or before dispatch as stale; rejects wrong task, request, dispatch, adapter,
environment, or branch bindings; rejects duplicate candidate IDs and replayed replay IDs; and fails
closed on malformed fixtures or tampered request, compatibility, or dispatch artifacts. Rejection
writes no candidate, status, diff, result, validation, review, or apply artifact. A duplicate or
replay after acceptance leaves both the accepted candidate and dispatch bytes unchanged.

Remote-status proofs show byte-for-byte deterministic output across clean runs and artifact-only
restart after the consumer checkout is removed. Status ingestion rejects observations at or before
the accepted candidate time; wrong candidate, task, request, dispatch, adapter, environment, or
branch bindings; duplicate observation IDs; replayed replay IDs; malformed fixtures; untrusted
claims other than the one fixture `completed` status; and tampered request, compatibility, dispatch,
or candidate artifacts. Every rejection writes no status, review, diff, result, validation, or apply
artifact and leaves the accepted candidate and dispatch bytes unchanged. Operation-result evidence
records success and deterministic stale, duplicate, replay, mismatch, malformed, and tampered
rejection reason codes.

Remote-diff proofs show byte-for-byte deterministic `remote-diff.json` and `remote-diff.patch`
outputs across repeated clean runs and artifact-only restart after removal of the consumer checkout.
Retrieval rejects observations at or before the accepted status time; wrong status, candidate, task,
request, dispatch, adapter, environment, or branch bindings; duplicate observation IDs; replayed
replay IDs; malformed or missing metadata; missing patch bytes; checksum-tampered patch bytes; and
tampered request, compatibility, dispatch, candidate, or status artifacts. Validation completes
before the two accepted diff artifacts are atomically materialized. Clean-chain rejection leaves no
diff, result, validation, review, or apply artifact; duplicate or replay rejection leaves accepted
diff, status, candidate, and dispatch bytes unchanged. Operation-result evidence records success and
deterministic stale, duplicate, replay, mismatch, malformed, missing, and tampered reason codes.

Remote-result proofs show byte-for-byte deterministic `remote-result.json` output across repeated
clean runs and artifact-only restart after removal of the consumer checkout. Retrieval rejects an
observation at or before the accepted diff time; wrong diff, patch, status, candidate, task, request,
dispatch, adapter, environment, or branch bindings; duplicate observation IDs; replayed replay IDs;
malformed or missing fixtures; and tampered request, compatibility, dispatch, candidate, status, diff
metadata, or accepted patch bytes. All chain and fixture checks complete before the result artifact
is atomically materialized. Clean-chain rejection leaves no result, validation, review, or apply
artifact; duplicate or replay rejection leaves the accepted result, diff, patch, status, candidate,
and dispatch bytes unchanged. Operation-result evidence records success and deterministic stale,
duplicate, replay, mismatch, malformed, missing, and tampered reason codes.

Validation-receipt proofs show byte-for-byte deterministic `validation-receipt.json` output across
repeated clean runs and artifact-only restart after removal of the consumer checkout. Retrieval
rejects an observation at or before the accepted result time; wrong result, diff, patch, status,
candidate, task, request, required-validation, compatibility, dispatch, adapter, environment, or
branch bindings; duplicate observation IDs; replayed replay IDs; malformed or missing fixtures; and
tampered request, compatibility, dispatch, candidate, status, diff metadata, accepted patch bytes,
or result evidence. All checks complete before the receipt artifact is atomically materialized.
Clean-chain rejection leaves no receipt, ready-for-review, validation-execution, or apply artifact;
duplicate or replay rejection leaves the accepted receipt and every upstream artifact byte-for-byte
unchanged. Operation-result evidence records success and deterministic stale, duplicate, replay,
mismatch, malformed, missing, and tampered reason codes.

The package-owned evidence and approval phases invoke no Codex, raw Git, Docker, SSH, network, live
status/diff/result/receipt polling, automatic semantic interpretation, push, publication, sync,
merge, or next-task surface. Only the generic DockPipe runtime invokes local Git for the separately
approved exact checkpoint. Existing selection, request, compatibility, fixture dispatch, follow-up,
completion-candidate, status, diff, patch, result, validation-receipt, and validation-execution artifacts remain
byte-for-byte deterministic, and software.dev/Example Brain behavior is preserved. Patch-boundary
proofs additionally show deterministic and idempotent output, artifact-only restart after consumer
removal, exact and descendant acceptance, segment-aware prefix-collision rejection, fail-closed
unsupported grammar/path handling, complete upstream tamper rejection, and no partial lifecycle
transition. Application proofs additionally show deterministic and idempotent receipts, strict
multi-file/multi-hunk mechanics, exact context and removal matching, missing/symlink/non-regular/
escaping source rejection, no-newline and malformed-count rejection, cleanup on success and failure,
cleanup-failure reporting, upstream and existing-receipt tamper rejection, and byte-for-byte
preservation of the consumer checkout and every upstream artifact. Validation-execution proofs add
exact declared-input-plus-overlay construction, size/hash and link/reparse checks, direct
argv from the patched workspace, an actually passing checked `go test` fixture, deterministic
nonzero evidence, first-failure stopping, command rejection before launch, cleanup across every
path, upstream-before-source tamper rejection, artifact-only restart, and no forbidden tool or
lifecycle action. Semantic-review proofs add strict approved/rejected parsing, exact full-chain and
review-scope binding, failed-validation gating, approved/rejected artifact-only restart,
deterministic decision/readiness artifacts, duplicate/replay/change rejection, existing-artifact
tamper rejection, and explicit denial of apply, commit, push, publication, and next-task authority.
Checkout-application proofs add separate approved/rejected parsing, exact readiness/preimage/scope
binding, readiness-only and semantic-only denial, exact multi-file/multi-hunk checkout mutation,
stale/missing/non-regular/link/reparse/escape/mixed-state rejection, forced post-mutation rollback,
distinct rollback and cleanup failures, exact postimage recovery and idempotent rerun, duplicate/
replay/change rejection, upstream and derived-artifact tamper rejection, unrelated-file and
upstream-evidence preservation, and explicit denial of Git, publication, synchronization, and
next-task authority.

Checkout-checkpoint proofs add a third independent approved/rejected decision, complete immutable
chain and exact postimage revalidation, checked fixture and canonical approval/request contracts,
runtime session/workspace/branch/parent binding, exact-path-only staging and commit, wrong/stale/
extra/staged/change-kind/path/link/reparse rejection, pre-commit index restoration, distinct commit/
metadata/receipt failures, exact-commit recovery, idempotent restart, existing-artifact tamper and
replay rejection, byte-for-byte upstream preservation, and explicit denial of push, publication,
sync, merge, and next-task authority.

The single next bounded TASK-015 slice is a separately approved runtime-owned publication request
that consumes only a valid `dockpipe.session-checkpoint-receipt/v1`, binds one exact commit and
session branch to one reviewed remote/ref destination, and permits only one push/publication action.
It must not sync, merge, switch branches, alter the checkpoint, select another task, or give package
code raw Git authority. A live Codex Cloud adapter remains blocked until a future installed CLI
documents a machine-readable receipt with a stable opaque task ID.

## Boundaries

- Keep Codex Cloud CLI integration, backlog parsing, prompt compilation, and task artifacts inside
  DorkPipe package workflows/assets/resolvers. Do not add a `dockpipe backlog` engine command or
  Codex-specific behavior under `src/lib` or `src/cmd`.
- Treat remote task submission as a cloud-backed governed lane: declare cost/attempt policy, record
  selected lane and environment, and halt before unapproved spend or mutation.
- Preserve the existing task-index/task-document format as the source of truth. Add the smallest
  structured readiness field needed for a future automatic selector rather than creating a second
  queue or copying status into generated state.
- A remote task owns neither the local workspace nor Git publication. It returns a remote diff and
  result; local application, validation, checkpoint, and publication retain their existing explicit
  approval boundaries.
- Do not promise Desktop sidebar visibility as a workflow guarantee until the CLI-to-Desktop task
  identity mapping is proven on supported accounts and environments.

## Required Artifacts

- `backlog-selection.json`: selected task ID, linked task path, bounded slice, baseline, and
  deterministic rejection reason when not dispatchable.
- `remote-request.md` and `remote-request.json`: reviewable compiled request and safe metadata.
- `remote-adapter-compatibility.json`: inspected adapter/CLI contract, documented inputs and receipt
  capability, exact fail-closed reason, and immutable request/target binding.
- `remote-task.json`: remote task ID, request fingerprint, environment/branch reference, and
  submission timestamp, plus the compatibility and dispatch fingerprints.
- `completion-candidate.json`: candidate/replay identity, immutable task/request/dispatch/adapter/
  target binding, deterministic observation time, untrusted terminal claim, and only the
  `completion_candidate` lifecycle state.
- `remote-status.json`: status observation/replay identity, full accepted candidate fingerprint,
  immutable task/request/dispatch/adapter/target binding, deterministic later observation time, and
  only untrusted fixture status evidence at the `completion_candidate` lifecycle state.
- `remote-diff.json`: diff observation/replay identity, canonical accepted status and candidate
  fingerprints, immutable task/request/dispatch/adapter/target binding, deterministic later
  observation time, exact patch SHA-256 and byte count, and only the `completion_candidate` state.
- `remote-diff.patch`: exact untrusted fixture patch bytes; retrieval treats them as opaque, while
  the later patch-boundary step parses only the narrow mechanical structure and paths. No semantic
  verification, application, validation, or lifecycle implication.
- `remote-result.json`: result observation/replay identity, canonical accepted diff/status/candidate
  fingerprints, exact accepted patch SHA-256 and byte count, immutable task/request/dispatch/adapter/
  target binding, deterministic later observation time, package-owned fixture provenance, and only
  opaque untrusted evidence at the `completion_candidate` lifecycle state.
- `validation-receipt.json`: receipt observation/replay identity, canonical accepted result/diff/
  status/candidate fingerprints, exact patch SHA-256 and byte count, immutable task/request/
  compatibility/dispatch/adapter/target binding, exact required-validation declaration and
  fingerprint, deterministic later observation time, package-owned fixture provenance, and only
  opaque untrusted evidence at the `completion_candidate` lifecycle state.
- `patch-boundary.json`: canonical accepted receipt/result/diff/status/candidate identities and
  fingerprints, exact patch checksum and size, immutable request/compatibility/dispatch/adapter/
  target bindings, exact allowed-path declaration and fingerprint, sorted changed paths, and a
  narrowly authoritative mechanical structure and segment-aware lexical containment result. It
  explicitly records every semantic, validation, review, apply, commit, push, and publication action
  as not performed and remains at `completion_candidate`.
- `patch-application.json`: canonical boundary and upstream identities, exact accepted patch binding,
  baseline as an unverified request declaration, sorted changed paths, canonical preimage/postimage
  manifests, file/hunk counts, successful temporary-copy-only mechanical application and cleanup,
  and explicit false statements for semantic review, validation, checkout mutation,
  `ready_for_review`, apply, commit, push, and publication. It contains no file contents, absolute or
  temporary paths, timestamps, process/host identity, or durable patched source snapshot.
- `validation-execution.json`: canonical patch-application and upstream identities, exact
  validation-input and required-validation bindings, accepted patch and sorted changed paths, exact
  union-workspace authority, deterministic per-command argv/status/exit code, aggregate passed/failed
  status, cleanup result, canonical fingerprint, and explicit false statements for semantic approval,
  `ready_for_review`, checkout apply, commit, push, and publication.
- `semantic-review-decision.json`: explicit approved/rejected local decision, bounded review scope,
  separate decision/replay identities, complete immutable chain and patch binding, exact validation
  status, canonical fingerprint, and false apply/commit/push/publication/next-task capabilities.
- `ready-for-review.json`: emitted only for approved review plus passed validation; binds the accepted
  decision fingerprint and complete candidate identity, records only `state: ready_for_review`, and
  grants no authority to apply, commit, push, publish, or start another backlog item.
- `checkout-application-approval.json`: explicit approved/rejected local application decision,
  separate approval/replay identities, fixed application scope, complete readiness/chain binding,
  exact patch and changed paths, canonical consumer preimage manifest, canonical fingerprint, and
  false commit/push/publication/checkpoint/sync/next-task capabilities.
- `checkout-application.json`: emitted only after approved exact checkout application or provable
  complete-postimage recovery; binds approval and readiness fingerprints, complete chain identity,
  exact pre/post manifests, paths, file/hunk counts, rollback preparation, mode preservation,
  cleanup, consumer verification, and no remaining lifecycle authority.
- `checkout-checkpoint-approval.json`: separate approved/rejected local checkpoint decision,
  approval/replay identities, full checkout-application and upstream chain binding, exact consumer
  postimage manifest, runtime session/workspace/branch and expected parent, fixed scope, canonical
  fingerprint, and permission only to submit one exact runtime request when approved.
- `checkpoint-request.json`: provider-neutral `dockpipe.session-checkpoint-request/v1` binding the
  approval fingerprint, runtime identities, exact branch/parent, sorted paths, exact postimages,
  fixed checkpoint scope, bounded message, and canonical request fingerprint.
- `checkpoint-receipt.json`: runtime-owned `dockpipe.session-checkpoint-receipt/v1` binding the
  request/authorization fingerprints, checkpoint/session/workspace, branch, parent, resulting
  commit, exact committed paths/postimages, runtime ownership, and false push/publication/sync/merge
  actions.
- operation-result events for inspect, compile, compatibility, dispatch, completion-candidate
  ingestion/rejection, status, diff, result, validation receipt, patch-boundary success/rejection,
  temporary-copy application success/rejection, validation-execution success/rejection,
  semantic-review decision/readiness success/rejection, checkout application, controlled checkpoint
  request/runtime success/rejection, and failure.

## Acceptance Criteria

- A fixture task index plus linked task document deterministically compiles one explicit bounded
  request and rejects closed, unknown, malformed, ambiguous, active-external, and blocked entries.
- The adapter fixtures prove dispatch, completion-candidate, status-observation, diff-observation,
  opaque result-observation, and opaque validation-receipt-observation parsing, record one opaque
  remote task ID, and never call a live provider by default.
- The compatibility fixture proves the exact documented CLI submission surface, fails closed when a
  machine-readable receipt or stable opaque task ID is absent, and cannot create a task identity.
- Completion-candidate fixtures prove that stale, duplicated, replayed, mismatched, malformed, or
  tampered signals cannot create or advance beyond `completion_candidate`. Reconciled evidence is
  still required before a future slice can define `ready_for_review`.
- Remote-status fixtures prove that stale, duplicated, replayed, mismatched, malformed, or tampered
  observations cannot create an artifact or advance beyond `completion_candidate`; accepted status
  evidence remains explicitly untrusted and cannot authorize result retrieval or review.
- Remote-diff fixtures prove that stale, duplicated, replayed, mismatched, malformed, missing, or
  tampered observations and patch bytes cannot create accepted diff artifacts or advance beyond
  `completion_candidate`; accepted patch bytes remain opaque and cannot authorize validation,
  review, apply, commit, push, or publication.
- Remote-result fixtures prove that stale, duplicated, replayed, mismatched, malformed, missing, or
  tampered observations and upstream patch bytes cannot create accepted result evidence or advance
  beyond `completion_candidate`; accepted result claims remain opaque and cannot authorize
  validation-receipt retrieval, review, validation, apply, commit, push, or publication.
- Validation-receipt fixtures prove that stale, duplicated, replayed, mismatched, malformed, missing,
  or tampered observations and any tampered upstream artifact or patch bytes cannot create accepted
  receipt evidence or advance beyond `completion_candidate`; accepted receipt claims and the exact
  required-validation declaration remain opaque, unexecuted evidence and cannot authorize review,
  validation, apply, commit, push, or publication.
- Patch-boundary proofs accept ordinary in-scope text modifications, exact allowed-path matches, and
  true descendants; reject prefix collisions, invalid/forbidden paths, malformed or unsupported
  patch structures, every tampered upstream binding, exact patch-byte tampering, and a malformed or
  tampered existing derived artifact; and leave every lifecycle-bearing artifact only at
  `completion_candidate`. Success is mechanical evidence only and cannot imply semantic correctness,
  validation success, approval, `ready_for_review`, apply, commit, push, or publication.
- Patch-application proofs require the exact verified boundary and source preimages, deterministically
  apply ordinary multi-file/multi-hunk text modifications only inside a cleaned temporary directory,
  reject malformed counts/coordinates, context or removal mismatches, unsupported no-newline forms,
  missing/symlink/non-regular/escaping sources, upstream or receipt tampering, and cleanup failure,
  and cannot mutate the consumer checkout or advance beyond `completion_candidate`.
- Validation-execution proofs require the immutable chain through patch application, construct only
  the exact declared-input-plus-changed-overlay workspace, reproduce the patch there, execute the
  declaration as direct bounded offline argv, stop on first nonzero, clean on every path, publish only
  canonical pass/fail evidence after successful cleanup, support artifact-only restart, reject
  tampering rather than repair it, and cannot interpret success as semantic approval or advance
  beyond `completion_candidate`.
- Semantic-review proofs require an explicit strict local decision, revalidate the immutable chain
  through exact validation execution and patch bytes, reject malformed/ambiguous/unknown decisions
  and every stale binding, emit readiness only for approved review plus passed validation, preserve
  rejected decisions without readiness, support artifact-only restart, reject duplicate/replay/
  changed identities and tampered existing artifacts without overwrite, and grant no apply, commit,
  push, publication, or next-task capability.
- Checkout-application proofs require a distinct strict local decision and valid readiness,
  revalidate the complete immutable chain and exact preimages before mutation, apply only accepted
  paths and bytes, verify exact postimages, roll back every mutation on in-process failure, report
  rollback/cleanup failures distinctly, support deterministic complete-postimage recovery and
  idempotent restart, reject mixed or tampered state without repair, preserve unrelated files and
  upstream evidence, and grant no commit, push, publication, checkpoint, sync, or next-task
  capability.
- Checkout-checkpoint proofs require the exact accepted checkout-application approval and receipt,
  complete upstream chain, exact postimages, and a third strict local decision. Rejected, malformed,
  ambiguous, replayed, changed, stale, mismatched, or tampered decisions cannot create a request or
  commit. The generic runtime must verify the exact session/workspace/branch/parent, empty index,
  complete change set, contained regular non-link postimages, and sorted paths before committing
  only those paths; it must restore the index after pre-commit failure, distinguish commit from
  metadata/receipt failure, recover only the exact resulting commit, remain idempotent after
  success, and grant no push, publication, sync, merge, or next-task authority.
- A restart can recover identity solely from the immutable request, compatibility, and
  `remote-task.json` artifacts; it does not need the consumer checkout, a cron worker, or a mutable
  prose status record.
- Remote apply is impossible without an explicit reviewed action. Checkpoint and publish remain
  separate runtime-owned requests.
- The workflow is package-owned, uses standard artifact scopes, and introduces no engine or
  Pipeon-specific special case.
- An opt-in live compatibility test proves the installed CLI/environment contract and separately
  records whether submitted tasks become visible in the intended Codex remote UI.

## Open Decisions

- Which resolver/profile owns the configured Cloud environment and selected branch policy.
- The minimum standardized readiness/ownership fields needed before a safe `--next` backlog
  selector can replace explicit task selection.
- Whether remote task results should be applied only through the Codex CLI or through a
  provider-neutral DorkPipe remote-task adapter contract.
- Whether any provider callback can meet the correlation and signature requirements; otherwise
  status/diff/result reconciliation remains the sole completion source for remote tasks.
- Which user-facing surface first consumes task status: CLI only, Pipeon, or Codex Desktop mapping
  once the identity bridge is proven.

---

# Multi-Machine DockPipe Execution Extension

## Scope And Evidence

This extension is a DorkPipe scheduling concern, not a request to make every DockPipe installation a
network worker. It was assessed against the current monorepo surfaces:

| Area inspected | Existing evidence | Consequence for this task |
| --- | --- | --- |
| DockPipe engine (`src/lib/`, `src/core/runtimes/`) | Generic workflow execution, host/Docker/VM runtimes, QEMU Windows helper assets, WSL guidance, runtime-owned Git sessions, run-scoped cleanup. | A node already has the local execution/lifecycle boundary; core must stay generic. |
| DockPipe results/events (`docs/runtime/operation-results.md`, `src/lib/infrastructure/operation_event.go`) | Canonical `OperationResult` records can be emitted as JSONL operation events with IDs, status, timing, and errors. | Keep the inner event unchanged; add distributed correlation outside it. |
| DockPipe artifacts and sessions (`docs/runtime/artifacts.md`, `docs/runtime/git-runtime-sessions.md`) | Scoped artifacts, session metadata, worker leases, checkpoints, sync/publish lifecycle, and future distributed-session intent. | Exact-commit execution and cleanup can reuse runtime primitives; graph ownership must remain above them. |
| DorkPipe package (`packages/dorkpipe/`) | DAG parsing/validation, topological scheduling, bounded parallel task execution, dependency artifacts, follow-up reruns, repair, budgets, approval, merge, and verification. | DorkPipe is the natural owner of node selection, graph state, retries, and final aggregation. |

`packages/dorkpipe/` is a first-party package in this DockPipe checkout, not a separate Git checkout
here. Its package boundary is nevertheless the DorkPipe product boundary for this decision.

## Decision

Adopt this responsibility boundary:

```text
DorkPipe scheduler and graph state
  -> node-execution adapter / outer transport envelope
    -> optional DockPipe node endpoint or existing trusted transport
      -> DockPipe local workflow execution
        -> host | Docker | QEMU | WSL | future runtime
```

- **DockPipe executes one assigned contract on one node or runtime.** It owns local policy
  enforcement, approvals presented at that node, process-tree termination, runtime teardown,
  artifacts, local operation results, and capability observation.
- **DorkPipe decides where, when, and why work executes across nodes.** It owns the graph, placement,
  dependency state, fan-out, retries/repair, distributed approval state, budgets, aggregation, and
  final graph outcome.
- **Transport is replaceable.** The first version uses a user-managed trusted transport; a persistent
  endpoint is an optional later capability, not an implied DockPipe daemon.

This confirms the hypothesis. The location, availability, and scheduling concepts are orchestration
concepts; putting them in DockPipe core would couple a standalone local executor to a cluster-control
plane it does not need.

## Architecture Options

| Option | Layering and portability | Security and operations | Verdict |
| --- | --- | --- | --- |
| A. Optional DockPipe node service | Clean local-executor endpoint if it exposes only a narrow execution contract; preserves third-party use. | Requires service install, mTLS/enrollment, revocation, binding/firewall UX, reconnect semantics, and durable local request state. | Viable Phase 4+ option; do not make it the first dependency. |
| B. DorkPipe-owned worker service calling local DockPipe | Keeps DockPipe networking-free, but duplicates local execution wrapping, health, cancellation, and policy translation in DorkPipe. | DorkPipe becomes responsible for a privileged long-running host agent and risks bypassing DockPipe semantics. | Do not use as the permanent default. It is acceptable only as a thin transport adapter with no independent executor. |
| C. Shared protocol with separate implementations | Can avoid a DorkPipe-specific wire format and permit optional DockPipe or third-party endpoints. | A protocol package still needs versioning, identity, authorization, and replay protection; premature sharing can freeze an immature design. | Define a small versioned contract after the existing-transport slice proves it; keep it transport-neutral. |
| D. Existing trusted transport first | Maximally local-first and portable: invoke the installed local DockPipe CLI through SSH or another user-managed private path. | Reuses user-owned network/auth/firewall controls; requires a careful wrapper for event streaming, cancellation, and artifact retrieval. | Recommended first vertical slice. |

The default must not expose a public listener, require DockPipe-hosted infrastructure, or introduce a
generic remote shell. A later service, if justified, accepts only authenticated, allow-listed
DockPipe execution requests and cannot be a general command relay.

## Responsibilities

### DockPipe core

Keep or add only reusable local-node primitives:

- execute an assigned workflow/task contract locally through a selected host, Docker, QEMU, WSL, or
  future runtime;
- report observed host, runtime, guest, toolchain, and policy capabilities without making placement
  decisions;
- emit canonical operation results/events, logs, artifact references, run identity, cancellation
  outcome, and cleanup outcome;
- honour cancellation by terminating the local process tree and performing the same scoped teardown
  as a local run;
- support exact source revision/workspace preparation through runtime-owned Git lifecycle APIs;
- optionally expose these same local primitives through a narrowly scoped endpoint in the future.

DockPipe core must **not** gain node enrollment, scheduler persistence, task-graph state, dispatch
queues, leases between machines, retries, repair policy, health-based placement, cost/risk placement,
distributed approvals, artifact fan-in, coordinator hosting, or a DorkPipe-specific protocol.

### Optional DockPipe node component

Only after a transport-neutral contract is proven, an optional `dockpipe node` component may provide
an authenticated endpoint for local execution, capability snapshots, event streaming, cancellation,
artifact retrieval, and health. It should be an installable Windows service or systemd service, not
part of normal `dockpipe` CLI startup.

It owns local request deduplication and cleanup recovery for a request it accepted. It does not own
worker enrollment policy, global leases, task selection, graph persistence, or final success.

### DorkPipe scheduler/orchestration

DorkPipe must add the distributed layer:

- node inventory/configuration and availability state;
- target/capability matching, including separate host and guest facts;
- graph-level execution leases, idempotency keys, dispatch, cancellation propagation, and reconnect
  handling;
- fan-out, dependency tracking, result reconciliation, targeted retry/repair, and budget/policy
  decisions;
- exact-commit source plan and per-target checkout receipt;
- outer envelopes for event/result/artifact correlation and immutable graph-run audit records;
- graph-level approval gates and the final success/failure decision.

No two edit tasks may receive the same mutable workspace by default. Version one has one
implementation owner; all validation targets use the exact resulting commit. Concurrent edits need
explicit ownership, isolated branches/workspaces, and a reconciliation task before they are enabled.

### Shared protocol package (conditional)

Do not create a shared package in Phase 1. If a second transport or an optional DockPipe endpoint is
implemented, extract a small transport-neutral `node-execution.v1` contract owned jointly by the
projects. It contains capability snapshot, execution request/receipt, event envelope, cancellation,
artifact manifest, health, identity, and version negotiation. It contains no scheduler decisions,
provider/model details, or generic-shell command field.

## Target Model And Authoring Boundary

Placement belongs in a **DorkPipe scheduler extension over DockPipe task contracts**, not in generic
DockPipe workflow YAML. A DockPipe workflow specifies *what local work does*; DorkPipe specifies
which compatible surface should receive that local contract. This keeps existing local workflows
backward compatible and allows the same workflow to be placed on several nodes.

The target model must preserve the compatibility surface:

```yaml
# DorkPipe task/scheduler authoring; not a DockPipe runtime selector.
target:
  requires:
    host_os: linux
    runtime: qemu
    guest_os: windows
    capabilities: [qemu, windows-ci-image]
```

`host_os: windows, runtime: host`, `host_os: linux, runtime: qemu, guest_os: windows`, and
`host_os: windows, runtime: wsl, guest_os: linux` are distinct targets. `node:` is an explicit
pin; `target: local` is a DorkPipe shorthand for the scheduler host. Capability records also need
runtime version/profile, relevant guest image or snapshot identity, available toolchains, policy
class, and freshness timestamp.

Fan-out is likewise DorkPipe authoring:

```yaml
strategy:
  matrix:
    target:
      - requires: {host_os: linux, runtime: host}
      - requires: {host_os: windows, runtime: host}
      - requires: {host_os: linux, runtime: qemu, guest_os: windows}
task:
  workflow: test.cross-platform
```

It expands into separate graph nodes, each dispatching the unchanged local DockPipe workflow.
The syntax remains a proposal until DorkPipe's current task schema and package contract are extended
with fixtures, validation, docs, and migration rules.

## Source, Event, Result, And Artifact Contract

Use commit-based synchronization initially. The implementation task creates a reviewed checkpoint or
commit; each validation target fetches and checks out that immutable commit in a runtime-owned
workspace. A shared remote Git repository is the preferred transfer mechanism. Git bundles are a
fallback for air-gapped/private-LAN cases. Patch, network-file, and workspace-snapshot transfer are
not version-one defaults because they weaken reproducibility and authority boundaries.

Keep DockPipe's inner events unchanged. DorkPipe records an outer immutable envelope, for example:

```yaml
graph_run_id: dorkpipe-912
node_id: office-windows
dockpipe_run_id: dp-412
task_id: verify-windows-host
sequence: 184
event: # unchanged DockPipe operation event
  schema: dockpipe.operation_event.v1
  type: operation_result
  status: done
```

At dispatch time, generic correlation keys such as `run_id`, `request_id`, and `task_id` may be
injected into the DockPipe execution context and its existing ID map. `graph_run_id`, placement, and
other DorkPipe concepts stay in the outer envelope. Every terminal target receipt must record:

- DorkPipe graph/run/task IDs, node identity, local DockPipe run ID, and sequence boundaries;
- tested commit/ref and checkout verification receipt;
- physical node, host OS, runtime, guest OS, QEMU image/snapshot, tool/capability snapshot;
- policy and approval context, event log/artifact manifest integrity, cancellation/cleanup outcome;
- normalized status, failure classification, and safe diagnostic references.

Artifacts remain content-addressed or checksum-manifested where transferred. The scheduler preserves
the original node artifact manifest and records retrieval failures rather than silently treating a
remote path as an artifact.

## Security Boundary

The first transport inherits a user-managed private network (LAN, VPN, or overlay) and must bind no
new public listener. The scheduler authorizes a named node for an allow-listed contract, not an
arbitrary command. Requirements before a persistent endpoint is accepted include:

- separate node and scheduler identities, mutual authentication, enrollment/revocation, and scoped
  task authorization;
- request IDs, short leases, expiry, idempotency/deduplication, monotonic event sequence numbers,
  and replay rejection;
- capability claims treated as observed/signed evidence, not a reason to grant extra authority;
- secrets resolved only at the authorized local execution boundary and never serialized into graph
  artifacts, envelopes, or logs;
- cancellation authenticated and bound to the original request; local cleanup reports success or
  residue explicitly;
- least-privilege service accounts, private-interface binding, optional narrowly scoped firewall
  assistance, audit logs, and node quarantine/revocation after suspected compromise;
- artifact/event integrity checks and no remote provider callback that can apply, publish, or expand
  authority without local reconciliation and approval.

## Recommended First Vertical Slice

Use **SSH over a user-owned LAN/VPN/overlay to Windows OpenSSH** as the initial transport. It avoids
new public infrastructure and is available from a Linux scheduler without requiring WinRM firewall
and remoting policy setup. PowerShell may run inside the remote DockPipe Windows workflow; it is not
the control-plane transport. WinRM/PowerShell Remoting remains a later adapter for organizations
that standardize it.

Slice:

1. Linux DorkPipe receives one implementation commit and one manually configured `office-windows`
   target with a static, reviewed capability manifest.
2. DorkPipe opens a single SSH execution session, prepares a Windows runtime-owned workspace at the
   exact commit, and invokes the installed local DockPipe validation workflow in the foreground.
3. A thin package-owned remote adapter streams/collects DockPipe's existing JSONL operation events,
   stdout/stderr references, terminal result, and artifact manifest into a DorkPipe outer envelope.
4. Cancellation is sent against the same request; the Windows adapter proves foreground process-tree
   termination and reports the DockPipe cleanup receipt. A failed cleanup is a distinct failed or
   degraded terminal result, never a successful cancellation.
5. DorkPipe records the per-target receipt and does no automatic repair, retry, apply, commit, or
   publish in this slice.

The slice deliberately excludes a daemon, auto-discovery, enrollment, QEMU dispatch, dynamic
scheduling, and generic remote shell. It must use a fixture/local fake for transport and an opt-in
Windows compatibility test; no live remote machine is required in the default package test suite.

## Phased Backlog

1. **Architecture decision:** define `node-execution.v1` shapes as package-owned design fixtures,
   target capabilities, outer correlation, security gates, and SSH acceptance tests.
2. **One remote Windows target:** implement the recommended slice above with exact-commit checkout,
   canonical event collection, cancellation/cleanup verification, artifacts, and restart-safe receipts.
3. **Multi-target validation:** add concurrent Linux-host, Windows-physical-host, and Linux-QEMU
   Windows-guest targets; aggregate per-target results and dispatch only an explicit repair task.
4. **Persistent nodes (conditional):** prove whether SSH limitations justify an optional endpoint;
   then add enrollment, health, capability refresh, reconnect, revocation, and offline behavior.
5. **Installer/UX (conditional):** role selection, Windows service/systemd management, private
   binding, optional firewall assistance, diagnostics, node naming, and node-management commands.
6. **Advanced scheduling:** leases, availability/load/risk/cost placement, retries, quarantine,
   disposable VMs/cloud workers, Mac, GPU, and third-party endpoint adapters.

## Acceptance Criteria For This Extension

- Standalone, local-only DockPipe workflows retain their current behavior and require no service.
- DorkPipe owns graph and placement decisions; DockPipe receives only a local execution contract.
- Host/runtime/guest dimensions are separately matched and reported.
- Existing DockPipe operation events/results are reused inside an outer DorkPipe envelope.
- The first slice runs one exact commit on a configured physical Windows target through SSH, returns
  structured results/artifacts, and proves cancellation plus cleanup handling.
- Default execution needs neither DockPipe-hosted cloud infrastructure nor a public listener.
- A failure cannot silently duplicate a task, replay a stale cancellation, hide cleanup residue, or
  automatically publish a change.
- Persistent-service work cannot start until its added value over the SSH adapter is documented with
  concrete cancellation, reconnect, capability-refresh, or UX evidence.

## Open Decisions For The Extension

- Whether the current DockPipe process-runner/cancellation primitives need one small generic
  machine-readable cancel/status API before the SSH slice can make its cleanup guarantee.
- The exact target schema location and migration path in the DorkPipe orchestration contract.
- Whether the first Windows target requires a preinstalled system OpenSSH service, a user-launched
  SSH endpoint, or an organization-owned private overlay; all remain user-managed prerequisites.
- Artifact transfer limits, retention, and checksum/signature policy for large guest logs/images.
- When a second real transport is sufficient evidence to extract `node-execution.v1`, and whether an
  optional DockPipe endpoint or a DorkPipe adapter should implement it first.
