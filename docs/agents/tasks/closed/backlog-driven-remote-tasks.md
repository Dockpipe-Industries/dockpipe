# TASK-015 Backlog-Driven Remote Tasks And Multi-Machine Execution — Closed

Status: Closed

Completed: 2026-08-02

## Shipped Summary

- The package-owned, fixture-only `backlog.remote` lifecycle now provides deterministic immutable
  evidence from strict schema-2 explicit or read-only unique-ready selection through request
  compilation, compatibility rejection, fixture dispatch and retrieval, mechanical boundary and
  application proof, bounded validation, independent semantic and checkout decisions, reviewed
  apply, and separately governed runtime checkpoint and publication contracts. Every stage validates
  and replays fail closed; provider, validation, receipt, availability, or prior-stage evidence never
  implies the next authority.
- The package-local `node-execution.v1` proof now covers the fake broker, connector/session and edge
  seams, independent placement and execution decisions, lease-bound delivery, task and graph
  reconciliation, dependency and next-task outcome chains, output delivery, acknowledgement, and
  final post-reconciliation evidence. Interactions remain fixture-only or deterministic local proof;
  immutable machine, capability, lease, receipt, graph, result, and acknowledgement bindings remain
  distinct and restart-safe.
- Canonical `--next` remains read-only and fail closed: it selects only one uniquely eligible
  `decision_ready` plus `unclaimed` entry, never ranks candidates, claims a task, or mutates ownership.
  The canonical open-only backlog has no decision-ready task, so it still rejects with
  `no_decision_ready_task`. The package fixture consumer retains TASK-015 solely as test data.

## Deferred Follow-ups — Not Shipped

The following items are explicitly deferred. They are not shipped, are not decision-ready, and gain
no implementation or runtime authority merely because TASK-015 is closed. Any future work requires a
separately reviewed task with explicit scope, authority, validation, and product decisions.

- live Codex Cloud submission, status, result, or diff integration while the installed CLI lacks a
  documented machine-readable receipt and stable opaque task-ID contract; undocumented terminal
  output parsing is not authorized;
- optional task claiming, leases, owner identity, backlog ownership mutation, automatic readiness
  promotion, or selection among multiple eligible tasks;
- resolver/profile ownership, provider adapters, callbacks, UI integration, live-provider result-
  reconciliation product decisions, or any new scheduling, lifecycle, external-authority, or
  automatic publication hop;
- production broker and node-connector packaging, hosted or managed services, tenant isolation,
  quotas, billing, availability, retention, custom-domain, and production provider policy;
- target-schema location and migration, large-artifact transfer limits, retention and signature
  policy, and standardized wire framing and authentication;
- any generic machine-readable cancellation/status primitive needed by a future production
  connector.

Everything below is retained as the detailed implementation and decision history. Historical proposed
contracts, open decisions, and phased backlog text do not convert deferred work into shipped behavior
or active acceptance criteria.

## Goal

Provide package-owned DorkPipe workflows for two related but distinct remote-execution paths:

1. inspect one explicitly selected or uniquely eligible decision-ready backlog item as a bounded
   remote Codex task input;
2. execute one DorkPipe task graph across user-owned DockPipe nodes through a transport-neutral
   broker contract and pluggable edge adapters.

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
- The first workflow requires an explicit task ID or the literal `--next` selector plus a bounded
  slice reference. It strictly validates the complete index, then reads only the selected entry, its
  linked task document, `AGENTS.md`, and the docs routed for the selected task type.
- It must reject closed, absent, malformed, ambiguous, externally active, or decision-blocked items
  before any remote submission.
- The schema-2 backlog records strict readiness and ownership metadata. Explicit inspection succeeds
  only for `decision_ready` plus `unclaimed`; it must never infer readiness from prose, ordering,
  availability, recent activity, commit history, or task presence.
- Read-only `--next` inspection succeeds only when exactly one complete index entry is
  `decision_ready` plus `unclaimed`; zero or multiple eligible entries reject, and index order never
  breaks a tie. Automatic readiness promotion, claiming, owner identity, and dynamic ownership
  mutation remain unimplemented.

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

1. `backlog.inspect` resolves and validates one explicitly selected or uniquely eligible task/slice.
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

## Current Status (2026-08-02)

The first vertical slice is implemented as the package-owned `backlog.remote` workflow and dedicated
orchestration-helper commands:

- `backlog.inspect` requires one exact `TASK-NNN` or the literal `--next`, one trimmed single-line
  bounded slice, and one exact baseline commit. It strictly loads the complete schema-2
  `docs/agents/task-index.yaml` and fails closed unless explicit selection names, or `--next` uniquely
  identifies, an entry that is `decision_ready` and `unclaimed`. It then resolves one exact linked
  document, verifies that document's heading matches the selected task ID, and records the exact
  selector object, readiness, ownership, automatic-selection status, source digests, and false
  ranking/claim/mutation/scheduling/dispatch/execution/provider/Git/publication authority in
  `dorkpipe.backlog-selection/v3`.
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
- A fresh validation workspace contains only the exact union of the request's 134 declared validation
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
- `backlog.checkout_publication` consumes only that valid runtime checkpoint receipt plus a fourth,
  independent `dorkpipe.checkout-publication-approval-fixture/v1` decision. It revalidates the
  complete immutable chain through checkout application and checkpoint, including exact paths,
  postimages, commit parent, trailers, request fingerprint, runtime checkpoint metadata, clean
  session workspace, and exact session/workspace/branch identity before recording
  `dorkpipe.checkout-publication-approval/v1`.
- The publication decision binds the exact task and canonical immutable-chain fingerprint, exact
  checkpoint approval/request/receipt fingerprints, immutable source commit and parent, runtime
  session/workspace/branch, one configured remote name, a SHA-256 identity of its effective push
  destination, one fully qualified `refs/heads/...` destination, bounded reason, distinct approval
  and replay identities, and fixed `approved_checkpoint_exact_commit_exact_branch_ref_once` scope.
  Rejected decisions create no request and perform no push.
- Approved decisions emit provider-neutral `dockpipe.session-publication-request/v1`. The generic
  strict `dockpipe session publish` request mode is the only component that invokes Git: it requires
  the exact checkpoint request and receipt, clean attached session branch at the approved HEAD,
  unchanged commit/parent/paths/postimages/trailers/metadata, and an effective configured push
  destination whose hashed identity matches the approval. It pushes one non-force refspec formed as
  `<approved-commit>:<approved-fully-qualified-ref>` and never uses a branch source, `-u`, force,
  wildcard, delete, tag, mirror, all-refs, or multiple-ref behavior.
- Requests and receipts persist only the bounded remote name and SHA-256 destination identity, never
  the effective URL, credentials, resolved authentication, or raw push output. Authentication stays
  in the existing host-side runtime Git boundary. Package helpers contain no raw Git or network
  operation and the workflow keeps `publish: none`; the separate request is the sole authority.
- `dockpipe.session-publication-receipt/v1` binds the approval/request fingerprints, checkpoint
  request/receipt/checkpoint identities, session/workspace/branch, immutable commit/parent, remote
  identity, destination ref, fixed scope, exact-refspec mode, no-force/no-upstream/no-credential
  facts, and false checkpoint/sync/fetch/merge/force actions. Runtime metadata stores the same
  sanitized receipt.
- A valid existing receipt is revalidated and returned without another push. Before mutation, a
  failure leaves local Git and the destination unchanged. Push rejection is distinct and cannot
  create a success receipt. Metadata or receipt failure after a successful push reports that remote
  mutation distinctly; restart may recover only after a bounded `ls-remote --refs` observation proves
  the one approved destination equals the exact approved commit while every local binding still
  matches. This does not promise impossible exactly-once transport semantics across an unobservable
  network failure.

The validation-source binding was added after the prior application slice exposed a contract
contradiction: the request's `source_files` contained only context documents, `allowed_paths` bound
only patch-write scope, and the accepted README-only patch could not authorize or bind the Go and
module inputs required by `go test ./packages/dorkpipe/lib/orchestrationhelper`. The checked fixture
now explicitly names the module metadata, target package sources/tests, local Go dependency sources,
embedded schema, exact Go workspace/module manifests, two package-owned Example Brain contract
files read by the target tests, and minimal root-embed matches required to assemble that command
workspace. No directory walk, glob, dependency discovery, whole-checkout copy, Git query, or backlog
prose expands that authority.

The package proof rejects absent, malformed, unknown, and ambiguous IDs; schema-1, malformed, legacy,
missing, partial, or unknown dispatch metadata; missing, escaping, mismatched, or closed linked task
paths; empty, whitespace-padded, multiline, or otherwise malformed bounded slices; invalid baselines;
unclassified or decision-blocked entries; and externally active ownership. For `--next`, it also
proves one unique eligible entry independent of index ordering, exact zero/multiple rejection,
whole-index failure for malformed inactive entries or duplicate IDs/paths, byte-identical replay,
unchanged index bytes, and successful fixture-backed downstream compilation/dispatch. Rejected
inspection writes a deterministic rejection code but no request or
dispatch artifact. Temporary consumer copies prove repeated-run determinism, no consumer mutation,
no live provider invocation, no Git/SSH/network tool invocation, and no live status polling,
live diff/result polling, result interpretation, semantic approval, apply, commit, push, or
publication. Validation-receipt polling and interpretation are also absent; only the immutable
required-validation declaration is executed inside the bounded temporary workspace.

The open-only canonical index now uses the strict schema-2 dispatch contract. Every canonical task is
conservatively initialized to `readiness: unclassified` and `ownership: unclaimed`; presence,
ordering, prose, recent activity, availability, or commit history cannot promote it. Canonical
`--next` inspection therefore rejects with `no_decision_ready_task`. The fixture consumer uses its
single `decision_ready` plus `unclaimed` entry to prove explicit and unique-ready inspection.
Automatic promotion, ranking or selection among multiple tasks, task claiming, owner identity,
dynamic ownership mutation, and scheduling remain unimplemented.

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

Checkout-publication proofs add a fourth independent approved/rejected decision, full immutable
chain and exact checkpoint revalidation, canonical approval/request/receipt contracts, strict
session/workspace/branch/HEAD/clean-state binding, credential-free effective-remote identity, one
fully qualified branch destination, immutable commit-source refspec, local-bare-remote-only push,
no automatic checkpoint or upstream configuration, malformed/ambiguous/tag/wildcard/delete/force/
multi-ref rejection, pre-push and non-fast-forward failure, distinct post-push metadata/receipt
failure, exact-ref recovery, receipt idempotence and tamper rejection, byte-for-byte upstream
preservation, and explicit denial of sync, fetch, merge, task completion, and next-task authority.

The completed remote-backlog lifecycle stops at publication. A separately approved, runtime-owned
fast-forward integration request remains a later bounded follow-up: it must consume only a valid
publication receipt and distinct human review, bind exact source/destination refs, permit at most one
non-force fast-forward, and grant package code no raw Git authority. It is not the current priority.

The package-owned `node-execution.v1` contract and in-process fake broker are now implemented in
`packages/dorkpipe/lib/orchestrationhelper/nodeexecution.go`. The strict JSON schemas are:

| Contract | Schema |
| --- | --- |
| Machine identity | `dorkpipe.node-execution.machine-identity/v1` |
| Capability snapshot | `dorkpipe.node-execution.capability-snapshot/v1` |
| Execution request | `dorkpipe.node-execution.execution-request/v1` |
| Task lease | `dorkpipe.node-execution.task-lease/v1` |
| Event envelope | `dorkpipe.node-execution.event-envelope/v1` |
| Cancellation and acknowledgement | `dorkpipe.node-execution.cancellation/v1`, `dorkpipe.node-execution.cancellation-ack/v1` |
| Artifact manifest | `dorkpipe.node-execution.artifact-manifest/v1` |
| Execution receipt | `dorkpipe.node-execution.execution-receipt/v1` |

The fake broker binds one stable operation and immutable request fingerprint to the enrolled machine,
one immutable capability snapshot, one expiring attempt/lease, one cancellation identity, ordered
event cursors, and one operation-keyed receipt. Connection IDs are process-local presence evidence;
disconnect and reconnect neither issue a lease nor execute, complete, cancel, retry, or transfer the
operation. Capability refresh appends a new snapshot and cannot mutate an accepted request or lease.

Every successful transition publishes a new immutable, fingerprint-linked broker-state JSON artifact
through the package atomic writer. Reopening scans and revalidates the complete artifact chain before
returning the accepted cursor or terminal receipt. Exact request, event, cancellation, capability,
and receipt deliveries are idempotent. Malformed or unknown JSON, non-canonical payloads, changed
duplicates, ordering gaps/regressions, stale/expired/differently bound leases, conflicting receipts,
and tampered capability, request, event, artifact-manifest, receipt, or state fingerprints fail closed
without overwriting prior evidence or advancing in-memory state.

The outer event envelope preserves the canonical `dockpipe.operation_event.v1` payload and permits
only bounded checksum references for stdout/stderr-style output. Artifact entries are sorted logical
names with sizes and SHA-256 digests; remote paths and credential-like URLs are not artifacts.
Cancellation acknowledgement is stored separately from terminal cleanup. Successful cleanup requires
digest evidence, while cleanup failure remains `failed` or `degraded`. The receipt binds the exact
machine, snapshot, lease/attempt, request fingerprint, optional local run, final cursor, result,
manifest, cancellation, cleanup, and completion time.

Focused `TestNodeExecution*` coverage proves identity separation, exact replay, capability refresh,
one fake execution, lease expiry/substitution rejection, reconnect and restart resumption, monotonic
events, cancellation/cleanup separation, terminal receipt idempotence/conflict rejection, manifest
binding, strict field/time/identifier validation, full-chain tamper rejection, and no partial atomic
publication. The fake accepts only an injected deterministic test executor and events. It adds no
DockPipe process execution, network/socket, service, Git, provider, workflow, retry/repair, apply,
checkpoint, publication, or external-call surface. Cloudflare, ngrok, direct TLS, private-overlay,
and future edges remain replaceable deployment adapters above the unchanged broker contract.

The package-owned local node-validation connector is now implemented in
`packages/dorkpipe/lib/orchestrationhelper/nodeconnector.go`. It accepts only one configured
`NodeExecutionWorkflowReference` and exact 40-character source revision, invokes an injected
already-prepared read-only validation function at most once, validates the complete returned evidence
before broker publication, and feeds unchanged canonical DockPipe events plus bounded checksum
references, local run identity, terminal result, cancellation acknowledgement, cleanup evidence, and
artifacts into the existing event and receipt contracts. Exact duplicate dispatch, reconnect, broker
reopen, repeated resume, and terminal delivery reuse the durable broker result without validation
re-execution. Workflow/revision mismatch, malformed or reordered events, path-bearing or invalid
artifacts, inconsistent run identity, stale cancellation, missing cleanup evidence, conflicting
terminal results, and changed terminal state fail closed without partial evidence publication. The
connector adds no process, shell, network, provider, Git, workflow, retry/repair, mutation, approval,
checkpoint, push, publication, or next-task authority.

The package-local transport-neutral connector-session contract and deterministic in-process fake are
now implemented in `nodeconnectorsession.go`. Enrollment binds one machine to one bounded opaque
credential identity; explicit rotation and revocation evidence advances a fingerprint-linked durable
transition chain without storing credential material. Session hello/negotiation keeps connection and
session identities separate, preserves one session identity across explicit disconnect/reconnect and
process restart, and reuses exact accepted negotiation replay without calling the injected transport.
Presence, health, and capability refresh are strict evidence with an all-false authority contract.
Stale enrollment, revoked credentials, changed duplicates, replay identities, capability substitution,
conflicting restart identity, malformed input, tampered state, and atomic-write failure reject without
publishing partial session state. The fake opens no socket, listener, process, provider, or network and
does not change or invoke the broker lease/execution or validation-connector authority boundaries.

The package-local session-to-dispatch seam now revalidates the complete immutable broker and session
chains before carrying one exact broker-accepted request and active lease through the current healthy
session into the existing `NodeValidationConnector`. Enrollment, credential, session, connection,
machine, capability, request, lease, and terminal receipt bindings are checked independently. Exact
terminal replay before and after disconnect/reconnect plus broker/session/connector restart returns the
same durable receipt without a second connector invocation or output. Missing/stale session state,
revocation, capability/request/lease/session substitution, expiry, changed replay, and durable-state
tamper reject before validation with no partial publication. Presence and health remain transport
prerequisites only: without the broker operation and lease they initiate nothing and grant no
completion, cancellation, retry, mutation, apply, checkpoint, commit, push, or publication authority.

The package-owned authenticated wire-framing proof is now implemented in `nodeconnectorwire.go`.
`dorkpipe.node-connector.authenticated-frame/v1` is canonical JSON bounded to 64 KiB with a 48 KiB
payload cap and five-minute maximum freshness window. Its strict allowlist wraps the existing
connector-session and `node-execution.v1` messages without changing their bytes or schemas. Every
frame binds direction, sender/receiver roles, peer identities, one opaque credential reference,
distinct frame/replay identities, message kind/schema, exact payload SHA-256, and issued/expiry times
through a direction-specific injected authentication collaborator. Connector proofs cannot
authenticate broker frames and broker proofs cannot authenticate connector frames; no live
credential or cryptographic provider is selected or claimed.

The fingerprint-linked durable replay chain rejects duplicate frames across receiver restart and is
updated only after the framed operation succeeds. The exact broker-accepted request and lease travel
in separate verified broker frames through the unchanged session-to-dispatch seam. The first proof
invokes `NodeValidationConnector` and the injected validator once with no broker executor; fresh frames
after disconnect/reconnect and broker/session/framing restart return the same durable receipt without
a second invocation or output. Complete enrollment, credential, session, presence, health,
capability, request, lease, and receipt bindings are revalidated before dispatch or receipt-frame
acceptance. Revocation, expiry, replay, substitution, malformed/noncanonical/unknown/trailing/oversize
input, and durable framing/session/broker tamper fail closed without partial output. Frame freshness
never extends a session or task lease, and authenticated session or receipt evidence initiates no
execution or lifecycle transition.

The package-owned in-process duplex exchange proof is now implemented in `nodeconnectorduplex.go`.
Connector-to-broker and broker-to-connector directions have independent monotonic accepted,
delivered, and acknowledged frontiers. Immutable configuration fingerprints bind explicit queued,
in-flight, and individual-frame count/byte limits; the primary proof profile allows eight queued
frames/512 KiB, four in-flight frames/256 KiB, and 64 KiB per authenticated frame. Limit, ordering,
cursor, direction, replay, authentication, freshness, configuration, and durable-chain failures reject
before advancing bytes, counters, cursors, acknowledgements, or fingerprints.

Delivery preserves the exact authenticated frame bytes and calls only an injected existing wire/session
acceptance boundary. Successful downstream acceptance durably moves ordered frames in-flight before a
separate acknowledgement, so restart between those boundaries resumes without a second connector or
validator invocation or receipt. Rejected downstream acceptance remains queued for safe retry. Exact
accepted wire identities remain rejected after restart, while fresh frames for the same completed
request/lease return the same durable receipt. Exchange identity, sequence, credit, acknowledgement,
cursor, and reconnect state remain distinct from machine, capability, lease, receipt, connector,
session, frame, enrollment, and credential identities and grant no lifecycle authority.

The package-local process-boundary transport proof is now implemented in
`nodeconnectortransport.go`. The broker binds only an explicit numeric loopback IP at an ephemeral
port, and the separately constructed connector owns only an outbound dialer. A four-byte big-endian
length prefix bounds each canonical resume, authenticated-frame, or acknowledgement record before
allocation. Hostnames, wildcard/unspecified/public endpoints, empty/partial/truncated/malformed/
oversized/trailing records, wrong direction/sequence/exchange/configuration, replay, timeout,
downstream rejection, cursor substitution, and durable-state tamper fail closed.

Both directions preserve the exact authenticated frame bytes across real TCP loopback, including
split writes and coalesced records. Acknowledgement follows only successful existing wire/session
acceptance. Exact durable cursors resume after disconnect and both endpoint restarts; a fresh
authenticated request/lease replay returns the same durable receipt without a second connector or
validator invocation. Transport framing, location, connection, authentication, ordering, credit,
acknowledgement, and resume evidence create no request, lease, receipt, credential, execution, or
lifecycle authority.

The package-local direct-TLS BYO edge proof is now implemented in `nodeconnectordirecttls.go`. The
broker alone owns one explicit numeric listener; the connector performs one outbound TLS 1.3 dial
with explicit trust roots, expected server identity, and bounded handshake. Certificate chain,
private key, and trust roots resolve only through opaque local references. There is no ambient trust,
insecure verification, plaintext downgrade, raw-TCP retry, discovery, or serialized secret material.

Real loopback proofs use ephemeral local certificates and carry the unchanged transport records in
both directions, including split/coalesced writes, downstream-before-ack, exact reconnect/resume, and
completed request/lease receipt replay without re-execution. Wrong trust or identity, invalid-time or
malformed/mismatched material, absent references, timeout, closure, non-TLS peers, and rejected records
fail closed without durable state or acknowledgement. TLS confidentiality, chain, location, and peer
identity evidence grant no request, lease, execution, receipt, mutation, Git, or lifecycle authority.

The package-local Cloudflare Tunnel BYO edge proof is now implemented in
`nodeconnectorcloudflaretunnel.go`. It selects Cloudflare's documented locally managed
credentials-file mode with one temporary ingress rule from the public hostname to the explicit
numeric-loopback direct-TLS broker, plus documented client-side `cloudflared access tcp` bound to an
adapter-owned numeric-loopback proxy. `cloudflared` owns TCP/WebSocket provider translation while the
existing TLS 1.3 connection remains end-to-end inside it. Cloudflare documents no minimum version for
this combined mode, so the compatibility floor is an explicit resolver declaration of locally managed
credentials-file, TCP-ingress, PID-file readiness, and Access TCP capabilities; unknown versions or
missing capabilities fail closed. The executable and credential are opaque references resolved only
before shell-free launch; only the credential file path enters private temporary configuration, never
its bytes, and config/PID state is removed after bounded shutdown or failure. Process, hostname,
provider, readiness, and connection evidence grants no request, lease, execution, receipt, mutation,
Git, or lifecycle authority; reconnect resumes only the existing durable transport cursors.

The package-local managed-broker preview contract is now complete in
`nodeconnectormanagedbrokerpreview.go`. Its strict fixture-only evidence keeps tenant identity,
bounded quota snapshots, audit evidence, retention policy, availability evidence, shared ingress,
machine, capability, connection, provider, lease, and receipt identities separate. Cross-tenant,
unknown, substituted, malformed, unbounded, and conflicting replay evidence fails closed; exact
restart replay is idempotent. Audit, availability, quota, retention, connection, provider, and
receipt evidence remains opaque and cannot prove completion or authorize execution, validation,
mutation, Git, apply, checkpoint, commit, push, or publication. The preview preserves only the exact
broker-accepted request fingerprint and active lease as the execution-authority reference.

This proof is not a hosted service or live multi-tenant implementation. It adds no network, provider
account, billing integration, quota enforcement, scheduler, daemon, node listener, or service
operation, and serializes no managed credentials or private configuration. Shared broker ingress
with tenant/node multiplexing remains the preferred future architecture; one edge credential per
node is not modeled or distributed.

The package-local explicit multi-target repair decision/request contract is now implemented in
`nodeconnectormultitargetrepair.go`. It directly revalidates one failed aggregate, requires an
independent strict fixture-owned approved/rejected decision bound to the exact sorted failed-target
set, and persists the canonical decision before optionally emitting one exact canonical request.
Rejected decisions emit no request. Approved request publication is restart-safe after a durable
decision and binds only the failed targets' immutable profile, machine, capability, operation,
request, lease, event, receipt, artifact, result, and cleanup evidence.

The decision and request invoke no repair worker, executor, validator, scheduler, transport,
provider, network, Git, or lifecycle transition. The request contains no command, repair
instruction, replacement target/profile, retry or scheduling decision, credential, path, raw event,
or raw receipt; `repair_dispatched` and every execution, validation, scheduling, network, repair,
mutation, Git, apply, checkpoint, commit, push, publication, completion, and next-task authority are
false. Aggregate, decision, and request tamper, changed expectations, cross-target substitution,
malformed/noncanonical/unknown/oversized input, conflicting replay, and partial publication fail
closed.

The package-local fixture-only connector service lifecycle and diagnostics contract is now complete
in `nodeconnectorservicelifecycle.go`. It binds one exact service, machine, connector artifact,
immutable service configuration, and either the Windows SCM machine-service profile or Linux
systemd system-service profile. A separate canonical lifecycle intent records only requested
install/start/stop/restart/uninstall follow-up; a separately fingerprint-bound diagnostic records
only one consistent bounded state/health/failure classification and sorted checksum references.
Restart replay, exact expected bindings, encoded/reference/aggregate bounds, and durable intent then
diagnostic publication fail closed under tamper, conflict, or atomic-write failure. Intent and
diagnostics grant no installer, service-manager, process, probe, lifecycle, execution, scheduling,
network, provider, mutation, repair, Git, apply, checkpoint, commit, push, publication, lease, or
completion authority.

The package-local fixture-only node inventory and placement-input snapshot contract is now complete
in `nodeconnectorinventorysnapshot.go`. One strict inventory binds an ordinally sorted bounded set of
Linux-host, Windows-host, and Linux-hosted QEMU Windows-guest nodes to exact machine and immutable
capability identities, target profiles, canonical observation time, and bounded availability, load,
risk, normalized-cost, and checksum-only reference evidence. A second strict artifact revalidates the
durable inventory and binds one workload and immutable requirements fingerprint to the exact complete
sorted inventory node set without filtering, scoring, ranking, recommending, selecting, reserving,
leasing, or dispatching a node. Both artifacts are canonical, fingerprinted, restart-safe, and
atomically published. Availability, load, risk, and cost evidence grants no placement, dispatch,
execution, repair, retry, network, service, or lifecycle authority.

The separate package-local fixture-only explicit node placement decision/request contract is now
complete in `nodeconnectorplacementdecision.go`. It directly revalidates one exact durable inventory
and placement-input snapshot, requires an independent strict canonical local approved or rejected
decision, and never derives selection from availability, load, risk, cost, ordering, scoring,
ranking, recommendation, matching, or connection presence. An approved decision explicitly selects
exactly one member of the complete candidate set and binds its exact node, machine, immutable
capability identity and fingerprint, and host/runtime/guest profile. Only that approved decision may
emit one separately fingerprint-bound placement request; a rejected decision emits none. The request
is evidence of explicit selection only and grants no dispatch, lease, execution, retry, repair,
quarantine, network, provider, service, mutation, validation, Git, apply, checkpoint, commit, push,
publication, lifecycle, completion, or next-task authority. Exact replay is idempotent; conflicting
replay, changed inputs, identity/profile substitution, tamper, and partial or failed publication fail
closed across restart.

The separate package-local fixture-only placement-bound dispatch decision/request contract is now
complete in `nodeconnectorplacementdispatchdecision.go`. It directly revalidates the exact inventory,
placement input, approved placement decision, and placement request before accepting one independent
strict canonical local approved or rejected dispatch decision. The decision binds the complete
candidate set; exact selected node, machine, immutable capability identity and fingerprint, and
host/runtime/guest profile; separate workload and execution-task identities; and the complete
canonical finalized `NodeExecutionRequest`, including its request fingerprint and every operation,
graph, run, task, source, workflow, capability, input, artifact, and request-time field.

Only an approved dispatch decision may emit one separately fingerprint-bound, unconsumed request for
a future one-time submission of that exact execution request to the existing in-process fixture
broker for the exact selected machine/capability binding. A rejected decision emits no request, and
the prior placement request alone authorizes nothing. This slice does not consume the authorization,
connect a node, invoke `NodeExecutionFakeBroker.Dispatch`, issue a task lease, invoke an executor, or
publish broker state. It grants no live/network/provider dispatch, execution, retry, repair,
quarantine, service, mutation, validation, Git, apply, checkpoint, commit, push, publication,
completion, lifecycle, or next-task authority. Exact replay is idempotent; malformed, noncanonical,
unknown, oversized, reordered, substituted, stale, tampered, orphaned, conflicting, and partial or
failed publication inputs fail closed across restart.

The package-local executorless fake-broker submission and lease-materialization boundary is now
complete in `nodeconnectorplacementdispatchsubmission.go`. It directly reloads and revalidates the
complete inventory, placement, and placement-dispatch chain before accepting one strict fixture-owned
submission. Only the exact approved, unconsumed placement-dispatch request may submit its unchanged
canonical `NodeExecutionRequest` through an already-connected transient in-process connection for the
exact selected machine and registered capability snapshot. A broker configured with an executor is
rejected before mutation. The only accepted lease is the exact `NodeExecutionTaskLease` returned by
the unchanged `NodeExecutionFakeBroker`.

The durable canonical submission artifact binds the complete upstream chain, deterministic bounded
issuance policy, post-transition broker-state fingerprint, exact request, and exact lease. It records
authorization consumption, broker invocation, and lease issuance while explicitly recording no
executor, connector, event, receipt, cancellation, execution, network, provider, retry, repair,
service, mutation, validation, Git, publication, completion, or lifecycle authority. If local atomic
publication fails after broker acceptance, no partial artifact is left and broker history is not
rewritten; an exact retry or restart through a fresh transient connection recovers the same lease by
operation/request idempotence and publishes the same evidence without another broker generation.
Changed policy, binding, replay identity, broker state, lease, upstream artifact, or existing
submission fails closed without repair.

This completes the authorized placement-bound fake-broker submission/lease-materialization slice.
The separate package-local fixture-only placement-bound execution-handoff decision/request contract
is now complete in `nodeconnectorplacementexecutionhandoff.go`. It directly revalidates the complete
inventory, placement, dispatch, submission, durable broker, exact operation, canonical execution
request, selected node/machine/capability/profile, and broker-issued lease chain before accepting a
third independent strict local approved or rejected decision. The decision binds a bounded reason
and issuance time within the exact lease interval. Only an approved decision emits one canonical,
fingerprint-bound, unconsumed request whose sole positive authority is a future one-time call through
the existing in-process fixture connector-session seam; a rejected decision emits no request.

This slice does not invoke `NodeConnectorSessionFake.DispatchAcceptedValidation`, a
`NodeValidationConnector`, an executor, the broker, a provider, a network, validation, mutation, Git,
or any lifecycle transition. Connection, health, availability, load, risk, cost, ordering, ranking,
provider evidence, broker acceptance, and lease existence cannot imply approval. Exact replay and
restart are idempotent, request-publication failure after a durable decision recovers identically,
and changed bindings, replay conflicts, stale broker state, substituted leases, upstream tamper,
malformed/noncanonical input, and partial evidence fail closed without repair or mutation.

This completes the authorized placement-bound execution-handoff decision/request slice. The separate
package-local fixture-only placement-bound execution-delivery contract is now complete in
`nodeconnectorplacementexecutiondelivery.go`. It consumes only the exact approved and unconsumed
handoff request, revalidates the complete immutable upstream chain plus the historical broker state
that issued the lease and the current durable broker/session state, and invokes
`NodeConnectorSessionFake.DispatchAcceptedValidation` once with the unchanged execution request,
exact broker lease, and explicitly supplied deterministic validation connector.

The delivery publishes one canonical receipt-bound artifact and records one connector-session and
prepared-validation invocation with zero broker-executor invocations. Exact replay, restart, and
post-receipt atomic-write recovery return the identical terminal evidence without reinvocation.
Changed identities, bindings, policy, session health/presence/negotiation, workflow, revision,
events, receipt, durable broker history, upstream artifacts, or existing delivery evidence fail
closed. Connection, health, presence, availability, load, risk, cost, ordering, ranking, provider
claims, broker acceptance, lease existence, and receipt shape cannot imply approval.

This delivery proof launches no real validation process, shell, workflow runner, Docker container,
provider, network, or remote machine. Cancellation, retries, repair, quarantine, disposable workers,
service mutation, publication, production execution, Mac, GPU, and compatibility adapters remain
later work. A live Codex Cloud adapter remains blocked until a future installed CLI documents a
machine-readable receipt with a stable opaque task ID.

The separate package-local fixture-only placement-bound execution-reconciliation decision/request
contract is now complete in `nodeconnectorplacementexecutionreconciliation.go`. It directly
revalidates the complete immutable placement, dispatch, submission, handoff, delivery, durable
broker operation, exact request/lease, ordered terminal events, and receipt chain before accepting a
fourth independent strict local approved or rejected decision. Only an approved decision emits one
canonical unconsumed request whose sole positive authority is a future one-time local graph-
reconciliation request.

Terminal success, receipt shape, validation, provider, availability, connection, presence, health,
broker acceptance, and lease existence cannot imply approval or graph outcome. The terminal evidence
remains opaque: this slice does not perform reconciliation, claim graph completion, propagate graph
failure, schedule another task, reinvoke the connector or prepared validation, invoke an executor,
or mutate broker history. Exact replay, restart, and decision/request atomic-write recovery are
idempotent; changed bindings, colliding identities, upstream tamper, malformed/noncanonical input,
and orphaned or conflicting artifacts fail closed without repair.

The separate package-local task-level graph-reconciliation consumer is now complete in
`nodeconnectorplacementexecutiongraphreconciliation.go`. It consumes only the exact approved,
unconsumed, one-time request, directly revalidates the complete immutable chain again, and records
one canonical durable consumption artifact. Only the fully validated execution receipt determines
the task outcome: `succeeded` with cleanup `not_required` becomes `passed`; `failed`, `degraded`, or
`cancelled` becomes `failed`, with the exact terminal result and cleanup evidence preserved.

The artifact binds the exact graph run, run, task, operation, attempt, execution request, lease,
ordered event stream, receipt, artifact manifest, decision, delivery, and reconciliation request.
Exact replay, concurrent calls, restart, and atomic-write recovery converge without changing the
immutable request or reinvoking any execution boundary. Whole-graph completion, graph-failure
propagation, dependency release, next-task scheduling, retry, repair, cancellation, publication,
and every execution or lifecycle side effect remain unauthorized and explicitly open.

The bounded backlog validation lane now disables only Go successful test-result caching while
retaining the exact direct `go test` argv, readonly-module policy, offline environment, compilation
cache, and explicit four-minute deadline. This ensures validation is actually executed and avoids
unbounded parent-process test-cache finalization after the test binary exits. Temporary validation
workspace cleanup also tolerates a short bounded transient-lock window before preserving the same
fail-closed cleanup error for a persistent lock; workflow-run scratch data remains outside durable
`bin/.dockpipe` and package state.

## Boundaries

- Keep Codex Cloud CLI integration, backlog parsing, prompt compilation, and task artifacts inside
  DorkPipe package workflows/assets/resolvers. Do not add a `dockpipe backlog` engine command or
  Codex-specific behavior under `src/lib` or `src/cmd`.
- Treat remote task submission as a cloud-backed governed lane: declare cost/attempt policy, record
  selected lane and environment, and halt before unapproved spend or mutation.
- Preserve the schema-2 task index and linked task documents as the source of truth. Keep only the
  small readiness/ownership dispatch contract in the index; keep narrative status in linked task
  documents rather than creating a second queue or copying status into generated state.
- A remote task owns neither the local workspace nor Git publication. It returns a remote diff and
  result; local application, validation, checkpoint, and publication retain their existing explicit
  approval boundaries.
- Do not promise Desktop sidebar visibility as a workflow guarantee until the CLI-to-Desktop task
  identity mapping is proven on supported accounts and environments.

## Required Artifacts

- `backlog-selection.json`: selected task ID, linked task path, bounded slice, baseline, exact
  readiness and ownership, schema-2 index and linked-task digests, exact `explicit_task` or
  `unique_decision_ready` selector object, coherent automatic-selection boolean, explicit false
  authority declarations, and a deterministic rejection reason when not inspectable.
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
- `checkout-publication-approval.json`: separate approved/rejected local publication decision,
  approval/replay identities, complete immutable-chain and checkpoint fingerprints, exact runtime
  identities and commit/parent, reviewed remote name plus credential-free destination identity, one
  fully qualified destination branch ref, fixed scope, canonical fingerprint, and permission only
  to submit one exact runtime request when approved.
- `publication-request.json`: provider-neutral `dockpipe.session-publication-request/v1` binding the
  approval, checkpoint request/receipt, session/workspace/branch, immutable source commit/parent,
  reviewed remote identity, exact destination ref, bounded reason, fixed scope, and canonical
  request fingerprint.
- `publication-receipt.json`: runtime-owned `dockpipe.session-publication-receipt/v1` binding the
  request and checkpoint, exact commit/parent/remote/ref, sanitized exact-refspec result, runtime
  ownership, no credential persistence, no force/upstream configuration, and false checkpoint/sync/
  fetch/merge/force actions.
- operation-result events for inspect, compile, compatibility, dispatch, completion-candidate
  ingestion/rejection, status, diff, result, validation receipt, patch-boundary success/rejection,
  temporary-copy application success/rejection, validation-execution success/rejection,
  semantic-review decision/readiness success/rejection, checkout application, controlled checkpoint
  request/runtime success/rejection, controlled publication request/runtime success/rejection, and
  failure.

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
- Checkout-publication proofs require the exact accepted checkpoint approval/request/receipt,
  complete immutable chain, exact commit/parent/paths/postimages/trailers, and a fourth strict local
  decision. Rejected, malformed, ambiguous, replayed, changed, stale, mismatched, or tampered
  decisions cannot create a request or push. The generic runtime must verify the exact session,
  workspace, branch, HEAD, clean worktree/index, unchanged checkpoint metadata, effective remote
  identity, and one fully qualified branch destination before pushing the immutable commit object as
  one non-force refspec; it must never checkpoint, set upstream, force, delete, tag, sync, fetch,
  merge, or select another task. Restart must revalidate a receipt without pushing again, or recover
  only after the exact approved remote/ref is safely observed at the exact commit.
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
- Whether a separately authorized future contract should ever add claiming after read-only `--next`
  inspection; this selection contract deliberately does not.
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
| [GitHub issue #11](https://github.com/jamie-steele/dockpipe/issues/11) | Broker/worker design feedback calls out separate machine, capability, task-lease, and execution-receipt identities, with idempotent receipts keyed by operation ID. | Make those identities explicit before a real transport so reconnects and UI disconnects cannot duplicate work or transfer responsibility. |

`packages/dorkpipe/` is a first-party package in this DockPipe checkout, not a separate Git checkout
here. Its package boundary is nevertheless the DorkPipe product boundary for this decision.

## Decision

Adopt this responsibility boundary:

```text
DorkPipe scheduler and graph state
  -> broker protocol / outer transport envelope
    -> outbound node connector
      -> DockPipe local workflow execution
        -> host | Docker | QEMU | WSL | future runtime
```

- **DockPipe executes one assigned contract on one node or runtime.** It owns local policy
  enforcement, approvals presented at that node, process-tree termination, runtime teardown,
  artifacts, local operation results, and capability observation.
- **DorkPipe decides where, when, and why work executes across nodes.** It owns the graph, placement,
  dependency state, fan-out, retries/repair, distributed approval state, budgets, aggregation, and
  final graph outcome.
- **Broker and edge are separate boundaries.** `node-execution.v1` defines identity, dispatch,
  leases, events, cancellation, and artifacts over standard HTTPS/WebSocket semantics. Cloudflare,
  ngrok, direct TLS, private overlays, and future providers are replaceable edge adapters; none
  defines the execution contract.

This confirms the hypothesis. The location, availability, and scheduling concepts are orchestration
concepts; putting them in DockPipe core would couple a standalone local executor to a cluster-control
plane it does not need.

## Deployment Modes

| Deployment mode | Ownership | Boundary and verdict |
| --- | --- | --- |
| A. Local/private broker | The user runs the broker and nodes on one machine, LAN, VPN, or private overlay. | Free and local-first. It requires no external edge provider or DockPipe-hosted infrastructure and is the deterministic development/test baseline. |
| B. BYO edge and broker | The user hosts the broker and owns the domain, tunnel/edge account, credentials, and policy. | Cloudflare Tunnel, ngrok, direct TLS, and equivalent adapters expose the same broker protocol. DockPipe may automate setup and diagnostics but does not own or persist provider credentials. |
| C. Managed DockPipe broker | DockPipe hosts the multi-tenant broker and edge; users enroll outbound-only nodes. | Subscription service. Tenant identity, quotas, audit, retention, availability, and billing remain control-plane concerns and cannot widen local execution authority. |

SSH, WinRM, and other remote-shell integrations may exist later as compatibility adapters, but they
are not the target architecture and must not shape `node-execution.v1`. The default node makes an
outbound authenticated connection and exposes no inbound public listener or generic remote shell.
Local/private and BYO modes must not require DockPipe-hosted infrastructure.

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

### Outbound node connector

A narrow node connector maintains an outbound authenticated broker connection for execution,
capability snapshots, event streaming, cancellation, artifact transfer, and health. It should be an
installable Windows service or systemd service, not part of normal `dockpipe` CLI startup. The first
slice may keep it package-owned while the contract is proven; promotion into generic DockPipe code
requires evidence that the primitive is independently reusable.

It owns local request deduplication and cleanup recovery for a request it accepted. It does not own
tenant policy, global leases, task selection, graph persistence, placement, billing, or final graph
success. It invokes the local DockPipe execution boundary and cannot become an independent executor
or arbitrary command relay.

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

### Broker and node-execution contract

Define package-owned design fixtures for a small transport-neutral `node-execution.v1` contract
before integrating a real edge provider. It contains enrollment and node identity, capability
snapshot, execution request/receipt, lease and idempotency identity, event cursor/envelope,
cancellation, artifact manifest, health/presence, and version negotiation. It contains no scheduler
decisions, provider/model details, tunnel credentials, subscription policy, or generic-shell command
field. Do not promote it into a shared engine package until the fake-broker slice proves the shapes.

The protocol keeps four identities distinct:

1. **Machine identity** identifies and authenticates one enrolled node independently of its current
   connection.
2. **Capability snapshot identity** binds the advertised and policy-approved facts used for one
   placement decision; a refresh creates a new snapshot rather than mutating old evidence.
3. **Task lease identity** binds one broker assignment, attempt, expiry, and cancellation authority;
   reconnecting does not silently create or transfer a lease.
4. **Execution receipt identity** is keyed by a stable operation ID and binds the accepted contract,
   local DockPipe run, terminal outcome, events, artifacts, and cleanup. Retrying delivery of the same
   operation returns or resumes the same receipt instead of executing it twice.

Broker responsibility survives UI disconnects, node reconnects, and process restarts. Connection
presence is evidence only; it cannot grant a lease, prove completion, or transfer responsibility.

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

The connector initiates an outbound authenticated broker connection and must bind no new public node
listener. Local/private mode may remain entirely on a user-managed machine, LAN, VPN, or overlay.
BYO and managed edges terminate only the broker connection; they cannot translate requests into
generic commands or widen a node's local policy. The scheduler authorizes a named node for an
allow-listed contract, not an arbitrary command. Requirements include:

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
- edge credentials, tunnel tokens, provider API tokens, and managed-service credentials remain secret
  references and never enter task contracts, receipts, events, or repository configuration.

## Proven Foundation And Next Vertical Slice

The **in-process fake broker, injected validation connector, transport-neutral connector-session
fake, session-to-dispatch seam, authenticated canonical framing profile, bounded durable duplex
exchange, real loopback process-boundary adapter, direct-TLS BYO edge, Cloudflare Tunnel BYO
 adapter, fixture-only managed-broker preview, fixture-only multi-target validation aggregate, and
 explicit fixture-only multi-target repair decision/request contract, fixture-only connector
 service lifecycle and diagnostics contract, fixture-only node inventory and placement-input
 snapshot contract, explicit fixture-only node placement decision/request contract, explicit
 fixture-only placement-bound dispatch decision/request contract, executorless fixture-broker
 submission/lease-materialization contract, explicit fixture-only placement-bound execution-
 handoff decision/request contract, fixture-only placement-bound execution-delivery contract,
 fixture-only placement-bound execution-reconciliation decision/request contract, fixture-only
 graph-finalization decision/request contract, fixture-only graph final-state projection
 decision/request contract, fixture-only graph lifecycle executor policy, atomic fixture-only graph
 lifecycle executor with its durable audit receipt, explicit fixture-only dependency-transition
 policy, atomic fixture-only dependency-transition executor, fixture-only next-task scheduling
 policy, atomic fixture-only next-task scheduling executor with durable evidence, and fixture-only
 next-task launch/new-node-execution authorization policy, and atomic fixture-only next-task launch/
 new-node-execution executor with a durable attempt record and consumption receipt, and fixture-only
 next-task result ingestion and task-level outcome reconciliation with separate durable evidence,
 explicit fixture-only post-reconciliation graph-continuation/finalization policy, and atomic
 fixture-only route-bound graph-continuation/finalization executor with separate durable evidence**
 now prove the durable product
 boundary without
letting a provider edge or managed-service artifact shape it. The package implements broker lease/receipt/event behavior, prepared local
validation delivery, durable enrollment/credential/presence/health/capability/restart evidence, exact
accepted request/lease handoff, mutual peer authentication, independent directional ordering and
acknowledgement, explicit record/frame/byte bounds, TLS 1.3 confidentiality and server identity,
local-only secret references, managed tenant/isolation/quota/audit/retention/availability evidence,
 exact durable resume, three exact target-profile receipt bindings, deterministic aggregate outcomes,
 independent approved/rejected repair decisions, exact failed-target-only repair requests, strict
 Windows SCM and Linux systemd service-target evidence, non-authoritative lifecycle intent, bounded
 health/diagnostic evidence, and
 restart-safe replay rejection without an external network, provider
account, remote machine, or node listener. The inventory proof adds exact Linux-host, Windows-host,
and Linux/QEMU/Windows node bindings plus bounded non-authoritative availability/load/risk/cost
evidence and one exact complete placement-input candidate set. The managed preview and inventory
evidence remain untrusted inputs only and cannot add placement, execution, or lifecycle authority.
The explicit placement proof adds one separate local decision, one exact selected-node binding, and
one evidence-only request without granting dispatch, lease, execution, repair, validation, mutation,
Git, publication, completion, or lifecycle authority.
The placement-bound dispatch proof adds a second independent local decision and one unconsumed
request that binds the complete exact `NodeExecutionRequest`. Its only positive authority is a future
one-time in-process fixture-broker submission. The submission proof consumes only that exact
authorization, uses an existing transient selected-machine connection, and records the exact
broker-issued lease without invoking an executor or starting execution.
The placement-bound execution-handoff proof adds a third independent local decision and one
unconsumed request bound to the exact submission, unchanged broker operation, canonical execution
request, selected node/machine/capability/profile, and task lease. Its only positive authority is a
future one-time call through the existing in-process fixture connector-session seam.
The placement-bound execution-delivery proof consumes that exact authority once, revalidates the
historical lease-issuance state and current broker/session state, calls the existing connector-session
seam once, and records the exact terminal events and receipt with connector and prepared-validation
counts of one and broker-executor count of zero.
The placement-bound execution-reconciliation proof preserves that terminal delivery as opaque
evidence behind a fourth independent local decision. Its approved request authorizes only a future
one-time local graph-reconciliation attempt; it cannot interpret success/failure, complete or fail
the graph, schedule another task, or reinvoke any execution boundary.
The task-level graph-reconciliation proof consumes that exact request once, interprets only the
fully validated receipt into `passed` or `failed`, and records the immutable task outcome without
completing the graph, propagating failure, scheduling another task, retrying, or repairing.
The graph-finalization proof places one nonempty ordinal set of those canonical terminal outcomes
behind a fifth independent local decision and emits only an unconsumed local-finalization request.
The graph final-state projection proof accepts only that exact approved decision/request chain and
its immutable graph-run, run, task, operation, receipt, and outcome bindings behind a sixth
fixture-owned local decision. Its approved request preserves `succeeded` and `failed` as distinct
terminal states and grants only a future one-time local projection attempt; it does not project or
persist a graph lifecycle state.
The graph lifecycle executor policy proof accepts that exact projection chain only as evidence and
places the future executor attempt behind a seventh independent fixture-owned local decision. Its
approved request binds one logical local graph store, one graph record, an immutable preimage
fingerprint/version, the exact terminal post-state, and every predecessor identity/fingerprint. It
requires compare-and-swap, one-record atomicity, exact-replay idempotency, crash recovery, and a
separately durable audit receipt, but it neither invokes an executor nor changes a graph record.
The graph lifecycle executor consumes only that exact approved, unconsumed policy request, revalidates
the complete immutable predecessor chain, and compare-and-swaps one existing strict fixture record
from its exact fingerprinted/versioned `running` preimage to the exact `succeeded` or `failed`
successor version. Its separately durable canonical audit receipt binds both record images, the
store/record identities, every policy/projection/finalization identity and fingerprint, and every
graph-run/run/task/operation/receipt/outcome binding. Exact replay returns the same receipt. If
receipt publication fails after the record replacement, restart accepts only the exact deterministic
postimage and finishes that same receipt without a second transition.
The graph dependency-transition policy consumes only that exact durable executor receipt and its
persisted postimage behind an eighth independent, fixture-owned authenticated decision. `succeeded`
may authorize one future dependency-release transition attempt; `failed` may authorize one future
failure-propagation transition attempt. The routes are mutually exclusive and bind one exact,
ordinally sorted dependency-target precondition set. The policy neither performs either transition
nor releases dependencies, propagates failure, schedules a task, or invokes any callback.
The graph dependency-transition executor consumes only that exact approved, authenticated,
fixture-owned, unconsumed route request. It revalidates the persisted terminal graph postimage and
complete immutable predecessor chain, prevalidates every strict dependency record before mutation,
and replaces only the exact authorized records with deterministic route-specific successor versions.
Its separate canonical durable evidence binds the policy decision/request, route, sorted target set,
every exact preimage/postimage, transition/write counts, consumed authorization, fixture ownership,
and the full predecessor binding. Exact replay returns the same evidence; restart can finish only an
exact same-request partial transition or publish the missing receipt from the complete exact
postimages without repeating any completed transition.
The next-task scheduling policy consumes only that exact durable dependency-transition receipt after
revalidating its complete predecessor chain and every persisted dependency-record postimage. It
requires a ninth independent fixture-owned authenticated decision. Only an exact successful
dependency-release route with a nonempty complete released-candidate set may emit one unconsumed
request for a future local scheduling executor attempt, and the selected task must be explicitly
named by that decision. Failure propagation and rejected decisions emit no request. Released state,
candidate discovery, ordering, readiness, availability, load, risk, cost, ranking, matching,
connection, provider, broker, or ForgePipe evidence never selects a task or implies approval.
The next-task scheduling executor consumes only that exact approved, authenticated, fixture-owned,
unconsumed request after revalidating the complete immutable predecessor chain and every persisted
dependency postimage. It compare-and-swaps one separate strict fixture-owned scheduling record from
its exact `dependency_released` preimage to `scheduled`, leaving every unselected candidate and all
dependency records unchanged. Its separate canonical receipt binds the graph run, terminal task,
policy decision/request, authentication, transition receipt and postimages, full candidate set,
selected task, record preimage/postimage, one transition/write, consumed authorization, and fixture
ownership. Exact replay, restart, same-request concurrency, pre-existing identical output, and
receipt-publication recovery never repeat the transition. The receipt proves only that local state
change and grants no task-launch, node-execution, callback, broker/provider/ForgePipe, network,
validation, checkout, Git, publication, retry, repair, cancellation, or external-action authority.
The next-task launch/new-node-execution authorization policy consumes only that exact durable
scheduling receipt while its exact `scheduled` record postimage remains present. It revalidates the
complete immutable predecessor chain and requires a tenth independent strict fixture-owned
authenticated approved/rejected decision. Only an approved decision emits one deterministic,
unconsumed, one-time request whose sole positive authority is a future local task-launch/new-node-
execution executor attempt for the exact scheduled task. Rejection emits no request. Scheduled
state, dependency release, candidate presence or selection, ordering, readiness, availability,
load, risk, cost, score, ranking, recommendation, matching, graph completion, lifecycle, transition,
receipt, connection, lease, broker, provider, ForgePipe, machine, capability, placement, validation,
or execution evidence never implies approval. The policy creates only its decision/request
artifacts and performs no scheduling mutation, task launch, node execution, dispatch, connector,
broker/provider/ForgePipe activity, callback, retry, repair, cancellation, publication, remote or
network action, validation, checkout mutation, Git action, or external action.
The next-task launch/new-node-execution executor consumes only that exact approved, independently
authenticated, fixture-owned, unconsumed request after revalidating the complete immutable graph,
lifecycle, dependency-transition, scheduling, and authorization chain. It materializes one
deterministic local attempt record and one separate canonical durable receipt that binds the exact
scheduled task, released and scheduled postimages, full candidate and transition evidence, both
authorization identities, and every predecessor fingerprint. The receipt records authorization
consumption without mutating the request. Exact replay, restart, identical concurrency, pre-existing
identical output, and attempt-before-receipt recovery never materialize a second attempt; malformed,
stale, missing, unsafe, conflicting, orphaned, or authority-escalating state fails closed. The
executor starts no task process or node execution and produces no execution result, outcome,
completion, graph progress, placement, dispatch, connector, broker/provider/ForgePipe, callback,
external, network, remote, validation, checkout, Git, retry, repair, cancellation, publication, or
lifecycle-transition effect.
The next-task result reconciler consumes only that exact durable executor receipt and attempt record
plus one separate canonical, independently authenticated, fixture-owned result observation bound to
the exact graph run, terminal task, selected scheduled task, candidate set, released dependency
postimage, and persisted `scheduled` record. It accepts only explicit `succeeded` or `failed`, writes
one canonical accepted-result record, then writes one separate durable reconciliation receipt that
records exactly one ingestion, one accepted-result write, and one reconciliation write. Only
`succeeded` maps to task outcome `passed`; only `failed` maps to task outcome `failed`. The immutable
observation and every predecessor remain unchanged. Exact replay, restart, identical concurrency,
pre-existing identical output, and accepted-result-before-receipt recovery converge without another
ingestion or reconciliation; conflicting observations, outcomes, artifacts, or partial state fail
closed. Attempt existence, authorization consumption, scheduled state, process exit, connector,
lease, broker/provider/ForgePipe, validation, graph, lifecycle, transition, or receipt evidence never
implies a result or outcome. The receipt claims no graph completion, graph failure propagation,
dependency release, next-task scheduling, execution, retry, repair, cancellation, publication,
callback, external action, or adjacent lifecycle authority.

The post-reconciliation graph-continuation/finalization executor consumes only the exact approved,
independently authenticated, fixture-owned, unconsumed policy request after revalidating the complete
immutable result-reconciliation, launch, scheduling, dependency-transition, lifecycle, graph, and
persisted scheduled-record chain. It accepts only `passed` with `graph_continuation`, `passed` with
`successful_graph_finalization`, or `failed` with `failed_graph_finalization`; the route is never
inferred from any result, outcome, graph, candidate, predecessor, provider, or connection evidence.
It atomically materializes one absent-to-exact fixture-owned route-bound transition record without
rewriting any completed lifecycle, dependency, scheduling, attempt, result, or reconciliation record,
then publishes one separate canonical durable receipt recording one transition, one record write, and
consumed authorization while leaving the immutable request unchanged.

`graph_continuation` records only that the passed selected task authoritatively continued the local
graph. `successful_graph_finalization` records successful local graph completion for the exact passed
result. `failed_graph_finalization` records failed local graph finalization and failure propagation for
the exact failed result. Exact replay, restart, identical concurrency, pre-existing identical output,
and transition-before-receipt recovery converge without another transition. Conflicting routes or
requests, missing or changed predecessors, malformed/noncanonical/unknown/trailing/oversized/symlinked
or unsafe evidence, orphaned or partial output, and authority escalation fail closed. The executor
invokes no callback or external collaborator and grants no future scheduling, execution, placement,
dispatch, connector, broker/provider/ForgePipe, retry, repair, cancellation, publication, validation,
checkout, Git, or general lifecycle authority.

Authorized bounded slice status:

1. The separate package-local fixture-only **explicit placement decision/request contract** is
   complete. It consumes one exact inventory and placement-input snapshot, requires an independent
   strict local decision, permits exactly one explicit bound-node selection only when approved, and
   grants no live dispatch or adjacent execution, retry, repair, network, provider, service,
   mutation, validation, Git, apply, checkpoint, commit, push, publication, lifecycle, completion,
   or next-task authority.
2. The separate package-local fixture-only **placement-bound dispatch decision/request contract** is
   complete. It consumes the exact approved placement chain plus one complete canonical finalized
   execution request, requires another independent strict local decision, and emits only an
   unconsumed request for a future one-time in-process fixture-broker submission. It invokes no
   broker dispatch, connection, lease, executor, provider, network, service, mutation, validation,
   Git, apply, checkpoint, commit, push, publication, completion, lifecycle, or next-task action.
3. The separate package-local **executorless fake-broker submission/lease-materialization contract**
   is complete. It consumes the exact approved request once, revalidates the complete immutable
   placement chain and durable broker state, recovers the same broker-issued lease across exact retry
   and restart, and publishes one canonical fingerprint-bound submission artifact. It creates no
   connection and invokes no executor, connector, event, receipt, cancellation, network, provider,
   retry, repair, service, mutation, validation, Git, publication, completion, lifecycle, or
   next-task action.
4. The separate package-local fixture-only **placement-bound execution-handoff decision/request
   contract** is complete. It consumes the exact submission and broker-issued lease only after a
   third independent strict local decision, then emits one unconsumed request whose sole positive
   authority is a future one-time call through the existing in-process connector-session seam. It
   invokes no connector, executor, broker, validation, network, provider, mutation, Git,
   publication, completion, lifecycle, or next-task action.
5. The separate package-local fixture-only **placement-bound execution-delivery contract** is
   complete. It consumes the exact approved handoff request once, invokes only the existing
   connector-session validation seam, materializes the exact terminal events and receipt, and
   recovers exact replay/restart/atomic-write failures without reinvocation. It creates no broker
   operation, lease, attempt, connection, session, enrollment, or credential and grants no
   cancellation, retry, repair, quarantine, service, network, provider, mutation, Git, apply,
   checkpoint, commit, push, publication, completion, lifecycle, or next-task authority.
6. The separate package-local fixture-only **placement-bound execution-reconciliation
   decision/request contract** is complete. It consumes only the exact durable terminal delivery
   behind a fourth independent strict local decision and emits at most one unconsumed request for a
   future local graph owner. It leaves terminal interpretation, graph reconciliation, graph
   completion/failure, retry/repair, and next-task scheduling false and invokes no connector,
   validator, executor, broker, network, provider, mutation, Git, publication, or lifecycle action.
7. The separate package-local fixture-only **task-level graph-reconciliation consumer** is complete.
   It consumes the exact approved one-time request through one canonical durable artifact, derives
   the task outcome only from the fully validated receipt, and preserves exact terminal and cleanup
   evidence plus every immutable identity/fingerprint binding. Whole-graph completion, graph-failure
   propagation, dependency release, next-task scheduling, retry, repair, cancellation, execution,
   lifecycle mutation, Git, and publication remain unauthorized and open.
8. The separate package-local fixture-only **local graph-finalization decision/request contract** is
   complete. It accepts only canonical terminal task outcomes for one exact graph run behind a fifth
   independent approved/rejected decision, and emits at most one unconsumed request that preserves
   terminal success or failure. It grants no projection, lifecycle, dependency, scheduling,
   retry/repair/cancellation, execution, ForgePipe/broker/provider, Git, or publication authority.
9. The separate package-local fixture-only **graph final-state projection decision/request contract**
   is complete. It consumes only the exact approved graph-finalization decision/request and binds
   their authority plus every immutable graph-run, run, task, operation, receipt, and outcome
   identity behind a sixth independent approved/rejected decision. Approved success and failure
   requests remain distinct; rejection emits no request. Exact replay/restart is idempotent, while
   concurrent conflicts, missing/tampered evidence, changed identities, malformed/noncanonical
   inputs, and mismatched terminal states fail closed. It creates only its decision/request
   artifacts and grants no actual graph transition or other lifecycle side effect.
10. The separate package-local fixture-only **graph lifecycle executor policy decision/request
    contract** is complete. It consumes only the exact accepted graph final-state projection and
    its immutable predecessor chain behind a seventh independent approved/rejected policy decision.
    Approved success and failure requests remain distinct and authorize only a future one-time
    local graph-state projection executor attempt constrained to one logical store/record CAS
    precondition. Rejection emits no request. The policy requires one-record atomicity, exact-replay
    idempotency, crash recovery, and a separately durable audit receipt. It creates only its policy
    decision/request artifacts and invokes no executor, mutates no graph record, releases no
    dependency, schedules no task, and grants no retry, repair, cancellation, execution, broker,
    provider, ForgePipe, validation, checkout, Git, publication, or general lifecycle authority.
11. The separate package-local fixture-only **atomic graph lifecycle executor and durable audit
    receipt** is complete. It consumes only the exact approved policy request and revalidates its
    complete immutable projection, finalization, task-outcome, execution-receipt, operation, task,
    run, and graph-run chain before one bound record compare-and-swap. The strict existing record
    must match the exact preimage fingerprint/version; only that record is atomically replaced by
    the exact terminal successor version, preserving `succeeded` and `failed` distinctly. Exact
    replay and same-request concurrency return one receipt without another transition, and a crash
    after replacement can publish the receipt only when the current record is the exact expected
    postimage. Stale, unrelated, malformed, noncanonical, oversized, missing, tampered, orphaned,
    or conflicting record, policy, predecessor, or receipt evidence fails closed. The receipt proves
    only that local record transition and grants no dependency release, failure propagation,
    scheduling, retry, repair, cancellation, execution, broker/provider/ForgePipe, network,
    validation, checkout, Git, commit, push, publication, or general lifecycle authority.
12. The separate package-local fixture-only **graph dependency-transition policy decision/request
    contract** is complete. It consumes only the exact durable graph lifecycle executor audit
    receipt whose persisted postimage remains present, then requires an eighth independent strict
    fixture-owned authenticated decision. `succeeded` may emit exactly one unconsumed request for a
    future dependency-release transition attempt; `failed` may emit exactly one unconsumed request
    for a future failure-propagation transition attempt. The structurally distinct routes are
    mutually exclusive and preserve exact sorted dependency-target preconditions plus the complete
    graph/run/reconciliation/outcome/finalization/projection/lifecycle-policy/transition/execution
    identity chain. The contract performs no dependency mutation or callback and grants no actual
    release, propagation, scheduling, new execution, retry, repair, cancellation, validation,
    checkout mutation, Git, commit, push, publication, broker, provider, ForgePipe, network, or
    remote authority.
13. The separate package-local fixture-only **dependency-transition executor and durable transition
    evidence** is complete. It consumes only the exact approved, authenticated, fixture-owned,
    unconsumed policy request after revalidating the persisted terminal graph postimage, complete
    immutable predecessor chain, policy decision/request fingerprints, route, sorted target-set
    fingerprint, and every strict target preimage identity/fingerprint/version. The success route
    performs only `blocked` to `dependency_released`; the failure route performs only `blocked` to
    `failure_propagated`. Every target is validated before the first replacement. Exact replay,
    restart, same-request concurrency, partial-write recovery, and receipt-publication recovery are
    deterministic and never repeat a completed target transition. The separate canonical receipt
    binds the full predecessor chain, decision/request, route, sorted target set, every preimage and
    postimage, exact transition/write counts, consumed authorization, and fixture ownership. That
    evidence grants no next-task scheduling, new execution, retry, repair, cancellation, callback,
    validation, publication, network, broker, provider, ForgePipe, checkout, Git, commit, or push
    authority.
14. The separate package-local fixture-only **next-task scheduling policy decision/request
    contract** is complete. It consumes only the exact durable dependency-transition receipt and
    persisted postimages after revalidating the full immutable predecessor chain, then requires a
    ninth independent strict fixture-owned authenticated decision. Only the successful dependency-
    release route with an exact nonempty released-candidate set may produce one deterministic,
    unconsumed request authorizing a future local next-task scheduling executor attempt. The request
    binds the graph run, terminal task, transition receipt, route, complete transition postimages,
    candidate set, explicit selected task, authentication, and decision fingerprint. Rejection and
    failure propagation emit no request. Replay, restart, concurrency, malformed or tampered
    evidence, changed bindings, and conflicting outputs fail closed. The policy performs no queue or
    scheduling mutation, task launch, node execution, retry, repair, cancellation, callback,
    publication, broker/provider/ForgePipe, network, validation, checkout, Git, commit, push, or
    remote action.
15. The separate package-local fixture-only **next-task scheduling executor and durable evidence**
    is complete. It consumes only the exact approved, authenticated, fixture-owned, unconsumed
    scheduling-policy request after revalidating the complete transition and graph predecessor
    chain plus every persisted released dependency postimage. It changes only the explicitly
    selected task's separate strict scheduling record from `dependency_released` to `scheduled`,
    atomically and exactly once, while every unselected scheduling record and every dependency
    record remain unchanged. Exact replay, restart, identical concurrency, pre-existing identical
    output, and receipt-publication recovery return the same canonical evidence without another
    transition; conflicts and ambiguous partial state fail closed. The evidence binds the full
    candidate set, exact selected released postimage, record transition, one transition/write,
    consumed authorization, and fixture ownership. It grants no task launch, node execution,
    retry, repair, cancellation, callback, publication, broker/provider/ForgePipe, remote execution,
    network, validation, checkout, Git, commit, push, or external-action authority.
16. The separate package-local fixture-only **next-task launch/new-node-execution authorization
    policy decision/request contract** is complete. It consumes only the exact durable scheduling
    executor receipt while the exact bound `scheduled` record postimage remains present, revalidates
    the complete immutable graph, lifecycle, dependency-transition, and scheduling predecessor
    chain, and requires a tenth independent authenticated fixture-owned approved/rejected decision.
    Approved decisions emit one deterministic, unconsumed, one-time request bound to the exact graph
    run, terminal task, candidate set, explicitly selected task, released dependency postimage,
    scheduled record postimage, scheduling policy decision/request, and both authentication chains.
    Rejection emits no request. Exact replay, restart, identical concurrency, pre-existing identical
    output, and decision-before-request recovery are idempotent; changed, stale, missing, malformed,
    noncanonical, oversized, symlinked, unsafe, orphaned, tampered, replayed, conflicting, consumed,
    authority-escalated, or ambiguous evidence fails closed. The policy invokes no executor and
    performs no scheduling mutation, task launch, node execution, dispatch, connector, callback,
    external action, broker/provider/ForgePipe activity, retry, repair, cancellation, publication,
    network or remote execution, validation, checkout mutation, Git action, commit, or push.
17. The separate package-local fixture-only **next-task launch/new-node-execution executor attempt
    and durable receipt** is complete. It consumes only the exact approved, independently
    authenticated, fixture-owned, unconsumed authorization request after revalidating the complete
    immutable predecessor chain and the exact persisted `scheduled` record. It materializes one
    deterministic local attempt record and then one separate canonical receipt that durably records
    authorization consumption while leaving the request unchanged. Exact replay, restart,
    same-request concurrency, pre-existing identical output, and attempt-before-receipt recovery
    return the same evidence without a second attempt; conflicting, malformed, noncanonical,
    oversized, symlinked, missing, stale, orphaned, tampered, inferred, unauthenticated, consumed,
    authority-escalated, or ambiguous evidence fails closed. The executor starts no task process or
    node execution and grants no result, outcome, completion, graph progress, placement, dispatch,
    connector, broker/provider/ForgePipe, callback, external, network, remote, validation, checkout,
    Git, retry, repair, cancellation, publication, or lifecycle-transition authority.
18. The separate package-local fixture-only **durable next-task result ingestion and task-level
    outcome reconciliation** is complete. It consumes only the exact durable launch/execution
    attempt and receipt plus one independently authenticated, fixture-owned, unconsumed canonical
    result observation bound to the exact authorization, scheduling, dependency-transition,
    lifecycle, graph, selected-task, candidate-set, released-postimage, and persisted `scheduled`
    record chain. It materializes exactly one accepted result and one separate durable receipt,
    mapping only `succeeded` to `passed` and only `failed` to `failed`. Replay, restart, identical
    concurrency, pre-existing identical outputs, and accepted-result-before-receipt recovery are
    idempotent; missing, stale, malformed, noncanonical, unsafe, unauthenticated, inferred, consumed,
    replayed, conflicting, orphaned, authority-escalating, or ambiguous state fails closed. It
    performs no graph continuation/finalization, completion, failure propagation, dependency release,
    scheduling, execution, retry, repair, cancellation, callback, external action, validation,
    network, checkout, Git, commit, push, or publication action.
19. The separate package-local fixture-only **atomic post-reconciliation graph-continuation/
    finalization executor and durable receipt** is complete. It consumes only the exact approved,
    independently authenticated, fixture-owned, unconsumed policy request after revalidating the
    complete immutable chain and persisted `scheduled` postimage. It accepts only the three explicit
    outcome-compatible routes, materializes one deterministic absent-to-exact route-bound transition
    record, and publishes one separate canonical receipt binding the exact post-state, route-specific
    effect, policy authentication, result/reconciliation, launch, scheduling, graph, candidate,
    released-postimage, and scheduled-record evidence. Exact replay, restart, concurrency, identical
    pre-existing output, and transition-before-receipt recovery are idempotent; conflicts, orphans,
    unsafe artifacts, partial state, changed predecessors, and authority escalation fail closed. It
    rewrites no predecessor and invokes no callback, provider, connector, broker, ForgePipe, process,
    validation, checkout, Git, publication, or external action.

The slice deliberately excludes a production daemon/service installer, live edge provider,
auto-discovery, billing, multi-tenancy, QEMU dispatch, dynamic scheduling, and generic remote shell.
Default tests require no network, provider account, tunnel, or remote machine.

## Phased Backlog

1. **Contract and fake broker (complete):** the strict package-owned shapes, four distinct identities,
   exact-revision binding, reconnect/idempotency, canonical events, cancellation/cleanup, artifacts,
   and restart-safe receipts are proven without a real executor or transport.
2. **Connector-session, authenticated framing, duplex, and loopback transport foundation (complete):** transport-neutral
   enrollment, opaque credential rotation/revocation, presence, health, capability refresh,
   disconnect/reconnect, restart negotiation, mutual peer authentication, canonical bounded frames,
   independent directional ordering/acknowledgement, explicit queued and in-flight frame/byte limits,
   bounded TCP records, deterministic resume, and restart-safe replay rejection are proven across an
   explicit ephemeral loopback listener and outbound connector.
3. **BYO edge adapters (complete):** direct TLS proves TLS 1.3, explicit trust and server identity,
   bounded handshake, secret references, and no downgrade. Cloudflare Tunnel independently proves
   locally managed credential-file ingress, outbound origin connectivity, client-side Access TCP,
   PID-file/process lifecycle, unchanged direct-TLS transport semantics, and no provider authority.
4. **Managed broker preview (complete):** the package-local fixture proof separates tenant identity,
   bounded quota snapshots, audit, retention, availability, shared ingress, and all node/broker
   identities without implementing hosting, billing, quota enforcement, or live multi-tenancy.
   Shared broker ingress with tenant/node multiplexing remains preferred over a separate edge
   credential distributed to every customer node.
5. **Multi-target validation (complete):** the fixture-only aggregate binds exact Linux-host,
   Windows-physical-host, and Linux-QEMU Windows-guest receipts, derives success only when all three
   pass, and records failed targets without repair or lifecycle authority.
6. **Explicit repair decision/request (complete):** consume a failed aggregate only after a separate
   strict local decision; emit a fixture request with no live repair execution or other authority.
7. **Connector service lifecycle and diagnostics (complete):** bind strict fixture-only Windows SCM
   machine-service or Linux systemd system-service targets to canonical lifecycle intent and bounded
   diagnostic evidence without performing service mutation or granting service-operation, execution,
   scheduling, network, provider, Git, or lifecycle authority.
8. **Node inventory and placement inputs (complete):** bind exact sorted node identities and profiles
   plus bounded fixture-only availability/load/risk/cost evidence, then revalidate one complete
   placement-input candidate set without making a placement, dispatch, retry, repair, execution,
   network, service, or lifecycle decision.
9. **Explicit placement decision/request (complete):** consume one exact inventory and placement-input
   snapshot behind an independent strict local decision, permit selection of exactly one bound node
   only when approved, and emit one evidence-only fingerprint-bound request with no live dispatch or
   adjacent execution, retry, repair, provider, service, validation, mutation, Git, publication,
   completion, or lifecycle authority. Bounded retries, quarantine, disposable workers, Mac, GPU,
   and third-party compatibility adapters remain later work and are not started by this slice.
10. **Placement-bound dispatch decision/request (complete):** consume the exact approved placement
    decision/request and one complete finalized execution request behind a separate strict local
    approved/rejected decision. Approved decisions emit one fingerprint-bound, unconsumed request
    permitting only a future one-time submission to the existing in-process fixture broker;
    rejected decisions emit none.
11. **Executorless fake-broker submission and lease materialization (complete):** consume the exact
    approved unconsumed placement-dispatch request through one existing transient selected-machine
    connection, submit the unchanged canonical execution request to the existing in-process fake
    broker, and persist one exact lease-bound transition artifact. Broker acceptance precedes local
    publication; exact retry recovers the same lease without a second broker generation. Execution
    handoff, events, receipts, cancellation, retries, quarantine, disposable workers, Mac, GPU, and
    compatibility adapters remain later work.
12. **Placement-bound execution-handoff decision/request (complete):** consume the exact immutable
    placement-dispatch submission, unchanged durable broker operation, canonical execution request,
    selected node/machine/capability/profile, and broker-issued lease behind a third independent
    strict approved/rejected local decision. Approved decisions emit one fingerprint-bound,
    unconsumed request permitting only a future one-time call through the existing in-process
    connector-session seam; rejected decisions emit none. Connector invocation, execution, events,
    receipts, cancellation, retries, quarantine, disposable workers, Mac, GPU, and compatibility
    adapters remain later work.
13. **Placement-bound execution delivery (complete):** consume the exact approved and unconsumed
    handoff request, revalidate the immutable placement/submission/lease chain plus the current
    healthy connector session, invoke the existing deterministic connector-session seam once, and
    publish one canonical receipt-bound delivery artifact. Exact replay and restart do not reinvoke;
    broker executor, production runner, cancellation, retries, repair, quarantine, service,
    network/provider behavior, mutation, Git, publication, completion, and next-task work remain
    excluded.
14. **Placement-bound execution-reconciliation decision/request (complete):** consume the exact
    immutable terminal delivery only behind a fourth independent strict approved/rejected local
    decision. Approved decisions emit one fingerprint-bound unconsumed request permitting only a
    future one-time local graph-reconciliation attempt; rejected decisions emit none. Terminal
    interpretation and task-level reconciliation are performed only by the separate bounded
    consumer; graph completion/failure propagation, retry/repair, next-task scheduling, and every
    execution or lifecycle action remain later work.
15. **Task-level graph reconciliation (complete):** consume the exact approved unconsumed request
    once, revalidate the full immutable execution chain, and persist one canonical task outcome from
    the receipt alone. Whole-graph completion, graph-failure propagation, dependency release,
    next-task scheduling, retry, repair, cancellation, publication, and execution/lifecycle side
    effects remain explicitly open and unimplemented.
16. **Local graph-finalization decision/request (complete):** consume a nonempty, ordinally sorted
    set of durable canonical task outcomes for one exact graph run only behind a fifth independent
    fixture-owned approved/rejected local decision. An approved decision must explicitly name
    `succeeded` only when every task outcome is `passed`, or `failed` when any terminal task outcome
    is `failed`; rejected decisions emit no request. The resulting one-time request is bound to the
    graph run, task/operation/receipt identities, outcome fingerprints, and explicit decision, and
    grants only a future local graph-finalization consumer. It performs no graph completion or
    failure propagation, dependency release, scheduling, retry, repair, cancellation, execution,
    ForgePipe/broker/provider action, checkout/Git action, publication, or other lifecycle side
    effect. Machine identity, capability snapshot, lease, events, connection, provider claims, and
    validation claims remain neither authority nor substitute inputs. Replay, restart, concurrency,
    malformed/noncanonical data, tamper, changed identities, and conflicting evidence fail closed.
17. **Local graph final-state projection decision/request (complete):** consume only the exact
    approved, unconsumed graph-finalization request and its accepted decision, revalidate their
    immutable terminal-outcome set, and require a sixth independent fixture-owned approved/rejected
    local decision. An approved decision emits one unconsumed request bound to the exact graph-run,
    run, task, operation, receipt, task-outcome, outcome-fingerprint, predecessor-decision,
    predecessor-request, and predecessor-authority identities. `succeeded` and `failed` remain
    distinct and must exactly match the accepted finalization; rejected decisions emit no request.
    Provider-like fixtures, events, connections, availability, validation claims, machine identity,
    capability snapshots, leases, and receipts cannot infer approval or projection authority.
    Replays and restarts recover exactly; concurrent/conflicting decisions, missing or tampered
    evidence, changed identities, malformed/noncanonical input, and terminal-state mismatch fail
    closed. This contract creates no graph lifecycle transition, dependency release, next-task
    scheduling, retry, repair, cancellation, publication, new execution, ForgePipe/broker/provider
    action, checkout/Git action, commit, push, or external-service effect.
18. **Local graph lifecycle executor policy decision/request (complete):** consume only the exact
    accepted graph final-state projection decision/request and its revalidated finalization and
    task-outcome predecessors behind a seventh independent fixture-owned approved/rejected policy.
    Bind one explicit logical local graph-store identity, graph-record identity, expected immutable
    preimage fingerprint/version, exact `succeeded` or `failed` projected terminal post-state, and
    every graph-run, run, task, operation, receipt, outcome, finalization, and projection
    identity/fingerprint. An approved policy emits one unconsumed request authorizing only a future
    one-time local graph-state projection executor attempt with mandatory compare-and-swap,
    one-record atomicity, exact-replay idempotency, crash recovery, and a separately durable audit
    receipt. A rejected policy emits no request. Exact replay/restart and same-decision concurrency
    are deterministic; conflicts, stale preimages, changed store/record or predecessor bindings,
    missing/tampered/orphaned evidence, malformed/noncanonical input, terminal mismatch, and partial
    publication fail closed. This policy creates no graph record, invokes no executor, and grants no
    completion/failure propagation, dependency release, next-task scheduling, retry, repair,
    cancellation, publication, broker/provider/ForgePipe, checkout/Git, or general lifecycle action.
19. **Atomic local graph-state projection executor and audit receipt (complete):** consume only the
    exact approved, unconsumed executor-policy request after revalidating the complete immutable
    predecessor chain. Require the one existing bound strict fixture record to match the exact
    expected preimage fingerprint/version, atomically replace only that record with the exact
    terminal successor and deterministic version, and durably publish one canonical audit receipt
    binding the preimage, postimage, store/record, policy, projection, finalization, graph-run, run,
    task, operation, receipt, and outcome identities/fingerprints. Preserve `succeeded` and `failed`
    distinctly. Exact replay/restart and same-request concurrency are idempotent; recovery after
    record replacement accepts only the exact expected postimage and never repeats the transition.
    Stale, unrelated, missing, tampered, orphaned, malformed, noncanonical, unknown-field, trailing,
    oversized, or conflicting evidence fails closed. No adjacent callback or authority is present.
20. **Explicit graph dependency-transition policy decision/request (complete):** consume only the
    exact durable graph lifecycle executor audit receipt while its exact persisted terminal
    postimage remains present, and require a separately authenticated fixture-owned approved or
    rejected decision. An approved `succeeded` decision emits one unconsumed request for only a
    future local dependency-release transition attempt; an approved `failed` decision emits one
    unconsumed request for only a future local failure-propagation transition attempt. Bind the
    exact store, record, postimage fingerprint/version, predecessor identities, intended request,
    route, and nonempty ordinally sorted target preconditions. Rejection emits no request. Exact
    replay/restart/concurrency is deterministic; conflicts, stale records, changed targets or
    identities, malformed/noncanonical artifacts, and ambiguous encodings fail closed. This policy
    performs no transition, mutation, scheduling, execution, or callback.
21. **Local dependency-transition executor and durable evidence (complete):** consume only the exact
    approved, authenticated, fixture-owned, unconsumed route request and revalidate its complete
    immutable predecessor chain, persisted terminal graph postimage, decision/request fingerprints,
    route, sorted target-set fingerprint, and every strict dependency record precondition. Validate
    the entire target set before mutation, then replace only exact bound records with deterministic
    `dependency_released` or `failure_propagated` successor versions. Emit one separate canonical
    receipt binding the full chain, route, targets, every preimage/postimage, transition/write counts,
    consumed authorization, and fixture ownership. Exact replay/restart/concurrency is idempotent;
    exact same-request partial transitions and receipt-publication failures recover without repeating
    completed writes, while ambiguous or unrelated partial state fails closed. The receipt is evidence
    only and grants no adjacent lifecycle action.
22. **Explicit next-task scheduling policy decision/request (complete):** consume only the exact
    durable dependency-transition receipt after revalidating its full immutable predecessor chain
    and every persisted dependency-record postimage. Require a ninth independent fixture-owned
    authenticated approved/rejected decision bound to the exact graph run, terminal task, route,
    transition receipt, complete transition postimages, and explicit candidate set. Only a successful
    dependency-release transition with a nonempty exact released-candidate set may emit one
    deterministic unconsumed request for a future local scheduling executor attempt, and the
    independent decision must explicitly select one member of that set. Rejection and failure
    propagation emit no request. Candidate discovery, release, ordering, readiness, availability,
    load, risk, cost, ranking, matching, connection, provider, broker, ForgePipe, or any predecessor
    evidence cannot infer approval or selection. The policy performs no queue mutation, scheduling
    mutation, task launch, node execution, retry, repair, cancellation, callback, publication,
    network, validation, checkout, Git, commit, push, or remote action.
23. **Local next-task scheduling executor and durable evidence (complete):** consume only one exact
    approved, authenticated, fixture-owned, unconsumed scheduling-policy request after revalidating
    its complete immutable predecessor chain and every persisted released dependency postimage.
    Require one existing strict scheduling record for the independently selected candidate, then
    atomically replace only its exact `dependency_released` preimage with the deterministic
    `scheduled` successor. Leave all unselected candidates and dependency records unchanged. Publish
    separate canonical evidence binding the full transition postimages and candidate set, selected
    task and released postimage, policy authentication and decision/request, exact record images,
    one transition/write, consumed authorization, and fixture ownership. Exact replay, restart,
    concurrency, pre-existing identical output, and receipt-publication recovery are idempotent;
    missing, stale, tampered, malformed, unsafe, conflicting, or ambiguous state fails closed. The
    evidence grants no launch, execution, callback, retry, repair, cancellation, publication,
    broker/provider/ForgePipe, remote, network, validation, checkout, or Git authority.
24. **Explicit next-task launch/new-node-execution authorization policy decision/request
    (complete):** consume only the exact durable next-task scheduling receipt while its exact bound
    `scheduled` postimage remains present, revalidate the complete immutable predecessor chain, and
    require a tenth independent authenticated fixture-owned approved/rejected decision. Bind the
    scheduling receipt, graph run, terminal task, complete candidate set, explicitly selected task,
    released dependency postimage, scheduled record postimage/fingerprint/version, scheduling-policy
    decision/request identities and fingerprints, decision authentication, and the intended
    authorization request identity. Rejection emits no request. Approval emits one deterministic,
    unconsumed, one-time request whose sole positive authority is a future local task-launch/new-node-
    execution executor attempt for the exact scheduled task. Exact replay/restart/concurrency and
    decision-before-request recovery are idempotent; stale, changed, missing, malformed,
    noncanonical, oversized, symlinked, unsafe, orphaned, tampered, replayed, conflicting, consumed,
    authority-escalated, inferred, or ambiguous evidence fails closed. The policy performs no
    scheduling mutation, task launch, node execution, dispatch, connector, callback, external
    action, broker/provider/ForgePipe activity, retry, repair, cancellation, publication, network or
    remote execution, validation, checkout mutation, or Git action.
25. **Local next-task launch/new-node-execution attempt executor and durable receipt (complete):**
    consume only the exact approved, independently authenticated, fixture-owned, unconsumed
    authorization request after revalidating its complete immutable predecessor chain and exact
    persisted `scheduled` record. Materialize one deterministic local attempt record, then publish
    one separate canonical receipt binding the authorization decision/request and authentication,
    scheduling receipt and policy chain, graph run, terminal task, transition postimages, complete
    candidate set, explicitly selected task, released and scheduled postimages, one attempt/write,
    consumed authorization, and fixture ownership. Leave the immutable request and all predecessor
    records unchanged. Exact replay, restart, same-request concurrency, pre-existing identical
    output, and attempt-before-receipt recovery are idempotent; malformed, unsafe, stale, missing,
    conflicting, orphaned, inferred, unauthenticated, consumed, authority-escalated, or ambiguous
    state fails closed. Start no task process or node execution and infer no result, outcome,
    completion, graph progress, or adjacent authority.
26. **Durable next-task result ingestion and task-level outcome reconciliation (complete):** consume
    only the exact durable attempt and executor receipt plus one separate canonical, independently
    authenticated, fixture-owned, initially unconsumed result observation bound to the exact
    authorization, scheduling, dependency-transition, lifecycle, graph, candidate, selected-task,
    released-postimage, and persisted `scheduled` record chain. Materialize one accepted-result
    record and one separate durable reconciliation receipt. Map only explicit `succeeded` to task
    outcome `passed` and explicit `failed` to task outcome `failed`; never infer either from attempt,
    process, connector, lease, broker/provider/ForgePipe, validation, graph, lifecycle, transition,
    or receipt evidence. Preserve every immutable input and predecessor. Exact replay, restart,
    concurrency, pre-existing identical outputs, and accepted-result-before-receipt recovery are
    idempotent; conflicts, orphans, unsafe artifacts, replay, ambiguous partial state, and adjacent
    authority fail closed. Perform no graph continuation/finalization or other lifecycle action.
27. **Explicit post-reconciliation graph-continuation/finalization policy decision/request
    (complete):** consume only the exact durable next-task result-reconciliation receipt and accepted
    result after revalidating their complete immutable predecessor chain. Require one separate,
    deterministic, one-time, independently authenticated, fixture-owned approved/rejected decision
    bound to the exact observation, attempt and executor receipt, launch authorization, scheduling,
    dependency, lifecycle, graph run, terminal and selected tasks, candidate set, released dependency
    postimage, persisted scheduled record, terminal result, task outcome, explicit route, and intended
    request identity. Only three approved combinations are valid: `passed` with
    `graph_continuation`, `passed` with `successful_graph_finalization`, and `failed` with
    `failed_graph_finalization`. Rejection emits no request. Approval emits one deterministic,
    fingerprint-bound, unconsumed request whose sole positive authority is one future local executor
    attempt for that exact route. Exact replay, restart, identical concurrency, decision-before-
    request recovery, and pre-existing identical artifacts are idempotent; conflicting decisions or
    routes, mismatched outcomes, missing or changed predecessors, inference, replayed/consumed or
    unauthenticated evidence, authority escalation, orphaned/partial artifacts, and malformed,
    noncanonical, unknown-field, trailing, oversized, symlinked, unsafe, or tampered evidence fail
    closed. The policy performs no graph continuation/finalization, mutation, scheduling, execution,
    callback, provider/broker/ForgePipe, validation, checkout, Git, or external action.
28. **Atomic local post-reconciliation graph-continuation/finalization executor and durable evidence
    (complete):** consume only the exact approved, independently authenticated, fixture-owned,
    unconsumed policy request for its explicit outcome-compatible route and revalidate the complete
    immutable predecessor chain plus the exact persisted `scheduled` postimage before mutation.
    Materialize one absent-to-exact deterministic route-bound transition record, then publish one
    separate canonical receipt binding its identity, fingerprint, version, post-state, route-specific
    effect, policy decision/request and authentication, reconciliation and accepted result,
    observation, launch attempt and receipt, launch authorization, scheduling receipt and policy,
    graph run, terminal and selected tasks, candidate set, released dependency postimage, scheduled
    record, terminal result, and task outcome. Record exactly one transition and one record write,
    consume authorization only in the receipt, and leave the request and every predecessor unchanged.
    Exact replay/restart/concurrency, identical pre-existing output, and transition-before-receipt
    recovery are idempotent; conflicting routes or requests, stale or changed predecessors, orphaned
    or ambiguous partial state, malformed/noncanonical/unknown/trailing/oversized/symlinked/unsafe
    artifacts, and authority escalation fail closed. Invoke zero callbacks and external actions.
29. **Explicit post-transition graph-output policy decision/request (complete):** consume only the
    exact canonical continuation/finalization executor receipt and transition record after
    revalidating their complete immutable predecessor chain, and require a new independently
    authenticated fixture-owned approved/rejected decision. Bind the exact executor receipt,
    transition record, route, post-state, route-specific effect, graph run, terminal and selected
    tasks, candidate set, accepted result, reconciled outcome, and prior policy authentication.
    Approval emits one deterministic, canonical, initially unconsumed request with exactly one
    mutually exclusive future authority: a continuation-handoff attempt for `graph_continuation`, a
    successful terminal graph-result materialization attempt for `successful_graph_finalization`, or
    a failed terminal graph-result materialization attempt for `failed_graph_finalization`.
    Rejection emits no request. Result, outcome, graph, transition, post-state, scheduling,
    availability, connection, lease, provider/broker/ForgePipe, ranking, cost, risk, or receipt
    evidence cannot infer approval, route, output type, or authority. Exact replay, restart,
    concurrency, identical existing artifacts, and decision-before-request recovery are idempotent;
    conflicts, missing or changed predecessors, consumed/replayed/unauthenticated evidence,
    inference, authority escalation, orphaned/partial state, and malformed, noncanonical,
    unknown-field, trailing, oversized, symlinked, unsafe, or tampered artifacts fail closed. The
    policy performs no handoff or materialization and invokes zero callbacks or external actions.
30. **Route-compatible post-transition graph-output consumer/executor (complete):** consume only the
    exact approved, independently authenticated, fixture-owned, initially unconsumed output-policy
    request after revalidating the complete immutable transition and predecessor chain. Materialize
    one absent-to-exact canonical output record and one separate durable executor receipt. The
    `graph_continuation` route creates only a local `continuation_handoff`; successful and failed
    finalization routes create distinct local terminal graph-result records. Each record binds the
    exact policy decision/request and authentication, transition receipt and record, route,
    post-state, route-specific effect, graph run, terminal and selected tasks, candidate set,
    accepted result, reconciliation receipt, terminal result, reconciled outcome, and prior policy
    authentication. The receipt proves exactly one output action, one output-record write, consumed
    authorization, and fixture ownership. Exact replay, restart, identical concurrency, identical
    existing artifacts, and output-before-receipt recovery are idempotent; conflicts, orphans,
    incompatible route/output/outcome/state/effect combinations, changed predecessors, ambiguous
    partial state, authority escalation, and malformed, noncanonical, unknown-field, trailing,
    oversized, symlinked, unsafe, or tampered artifacts fail closed. The continuation record invokes
    no receiver, and terminal records are not published, delivered, or used to trigger lifecycle
    work. Every predecessor remains unchanged and zero callbacks or external actions are invoked.
31. **Explicit downstream graph-output delivery policy decision/request (complete):** consume only
    the exact durable output record and output-executor receipt after revalidating their complete
    immutable predecessor chain. Require one new independently authenticated, deterministic,
    one-time, fixture-owned approved/rejected decision bound to the exact output and executor
    receipt, output policy, transition, route, post-state, route-specific effect, output type, graph
    run, terminal and selected tasks, candidate set, accepted result, reconciliation receipt,
    terminal result, reconciled task outcome, and one exact downstream consumer identity and
    consumer-contract fingerprint. Rejection names no route, delivery type, request, or consumer and
    emits no request. Approval emits one canonical initially unconsumed request with exactly one
    mutually exclusive future authority: `continuation_handoff_delivery_attempt` for
    `graph_continuation`, `successful_terminal_graph_result_delivery_attempt` for successful
    finalization, or `failed_terminal_graph_result_delivery_attempt` for failed finalization. Output,
    result, graph, transition, scheduling, availability, connection, lease, provider, broker,
    ForgePipe, ranking, cost, risk, validation, or receipt presence cannot infer approval, route,
    output, delivery type, consumer, or authority. Exact replay, restart, identical concurrency,
    pre-existing identical artifacts, and decision-before-request recovery are idempotent; missing,
    changed, malformed, unsafe, orphaned, tampered, conflicting, inferred, consumed, replayed,
    unauthenticated, non-fixture-owned, consumer-ambiguous, or authority-escalated evidence fails
    closed. The policy invokes no consumer or receiver and performs no delivery, acknowledgement,
    lifecycle advancement, callback, publication, network, external, provider, connector, broker,
    ForgePipe, validation, checkout, or Git action.
32. **Route-compatible downstream graph-output delivery/consumer executor (complete):** consume only
    the exact approved, independently authenticated, fixture-owned, initially unconsumed delivery-
    policy request with its exact durable output and complete immutable predecessor chain. Require
    one injected local consumer whose identity and contract fingerprint exactly match the request,
    use the delivery request/replay pair as its durable operation key, and invoke it at most once.
    Persist one canonical fixture-owned acknowledgement and one separate executor receipt binding
    the delivery decision/request and authentication, output record and executor receipt, output
    policy, transition, graph run, terminal and selected tasks, complete candidate set, accepted
    result, reconciliation receipt, exact route/output/delivery/state/effect/outcome, and consumer
    contract. Exact replay, restart, identical concurrency, identical pre-existing artifacts,
    consumer-acceptance-before-local-acknowledgement recovery, and acknowledgement-before-receipt
    recovery are idempotent without reinvoking the consumer. Conflicts, rejection, errors, orphans,
    ambiguous partial state, changed predecessors, incompatible routes, outputs, deliveries,
    consumers, contracts, or authorities, and malformed, noncanonical, unknown-field, trailing,
    partial, empty, oversized, symlinked, unsafe, or tampered evidence fail closed. The executor
    performs no lifecycle advancement, graph mutation, dependency work, scheduling, execution,
    retry, repair, cancellation, callback, publication, provider, connector, broker, ForgePipe,
    process, network, validation, checkout, Git, or external action.

33. **Explicit post-delivery acknowledgement-reconciliation policy decision/request (complete):**
    consume only the exact canonical accepted delivery acknowledgement and its exact durable
    delivery-executor receipt after revalidating their complete immutable predecessor chain and the
    exact downstream consumer identity and contract. Require a new independently authenticated,
    deterministic, one-time, fixture-owned approved/rejected decision. Rejection names no route,
    output, delivery, consumer, request, or authority and emits no request. Approval preserves the
    exact route, post-state, route-specific effect, output type, delivery type, terminal result, task
    outcome, consumer, acknowledgement operation key, and predecessor bindings, and emits one
    canonical initially unconsumed request with exactly one mutually exclusive future authority: a
    continuation-handoff acknowledgement-reconciliation attempt, a successful-terminal-result
    acknowledgement-reconciliation attempt, or a failed-terminal-result acknowledgement-
    reconciliation attempt. Acknowledgement or receipt presence, consumer acceptance, output,
    result, graph, transition, scheduling, availability, connection, lease, provider, connector,
    broker, ForgePipe, ranking, cost, risk, validation, or any adjacent evidence cannot infer
    approval, route, consumer, reconciliation, or authority. Exact replay, restart, identical
    concurrency, pre-existing identical artifacts, and decision-before-request recovery are
    idempotent; conflicts, missing or changed predecessors, inference, consumed/replayed or
    unauthenticated evidence, authority escalation, orphaned/partial state, and malformed,
    noncanonical, unknown-field, trailing, oversized, symlinked, unsafe, or tampered artifacts fail
    closed. The policy performs no acknowledgement reconciliation, lifecycle advancement, graph
    mutation, dependency work, scheduling, execution, retry, repair, cancellation, callback,
    publication, provider, connector, broker, ForgePipe, process, network, validation, checkout,
    Git, or external action.
34. **Local post-delivery acknowledgement-reconciliation executor and durable evidence (complete):**
    consume only the exact approved, independently authenticated, fixture-owned, initially
    unconsumed policy request after revalidating the accepted acknowledgement, delivery-executor
    receipt, downstream consumer identity and contract, and complete immutable predecessor chain.
    Persist one canonical versioned reconciliation record and one separate executor receipt binding
    the policy decision/request and authentication, acknowledgement and operation key, delivery
    receipt, route/state/effect/output/delivery, consumer contract, graph run, terminal and selected
    tasks, complete candidate set, accepted result and prior reconciliation receipt, transition,
    output policy/executor, and delivery policy/executor chain. The receipt records exactly one
    logical reconciliation attempt, one record write, one receipt write, the mutually exclusive
    route authority consumed, complete predecessor revalidation, no consumer reinvocation, and no
    duplicate reconciliation while leaving the immutable request unchanged. Exact replay, restart,
    identical concurrency, identical pre-existing artifacts, and record-before-receipt recovery are
    idempotent; conflicts, missing or changed predecessors, incompatible routes, inference,
    consumed/replayed/unauthenticated/non-fixture-owned evidence, authority escalation, orphaned or
    ambiguous partial state, and malformed, noncanonical, unknown-field, trailing, partial, empty,
    oversized, symlinked, unsafe, or tampered artifacts fail closed. The record and receipt are
    evidence only and perform or grant no lifecycle advancement, graph mutation, dependency work,
    scheduling, execution, delivery, consumer reinvocation, retry, repair, cancellation, callback,
    publication, provider, connector, broker, ForgePipe, process, network, validation, checkout,
    Git, or external action.

35. **Separately authenticated post-reconciliation policy decision/request (complete):** consume
    only the exact durable acknowledgement-reconciliation record and executor receipt after
    revalidating their complete immutable predecessor chain, including the accepted acknowledgement,
    delivery receipt, route/state/effect/output/delivery, downstream consumer identity and contract,
    terminal result and task outcome, and prior reconciliation policy decision/request and
    authentication. Require one deterministic, one-time, independently authenticated, fixture-owned
    approved/rejected decision. An approved decision emits exactly one immutable request granting
    only one opaque future local executor attempt compatible with the exact route:
    `continuation_handoff_post_reconciliation_attempt`,
    `successful_terminal_graph_result_post_reconciliation_attempt`, or
    `failed_terminal_graph_result_post_reconciliation_attempt`. A rejected decision emits no request
    and grants no authority. Exact replay, restart, identical concurrency, identical pre-existing
    artifacts, and decision-before-request recovery are idempotent; conflicts, missing or changed
    predecessors, incompatible route/state/effect/output/delivery/outcome/terminal/consumer/contract
    bindings, inference, consumed/replayed or unauthenticated prior evidence, authority escalation,
    orphaned or ambiguous partial state, and malformed, noncanonical, unknown-field, trailing,
    partial, empty, oversized, symlinked, unsafe, or tampered artifacts fail closed. The policy
    performs or grants no acknowledgement reconciliation, lifecycle advancement, graph mutation,
    dependency work or release, failure propagation, candidate selection, scheduling, execution,
    result collection, delivery, consumer invocation, retry, repair, cancellation, queue processing,
    callback, publication, provider, connector, broker, ForgePipe, process, network, remote
    execution, validation, checkout mutation, Git, checkpoint, commit, push, or external action.

36. **Local post-reconciliation executor with separate durable attempt and consumption evidence
    (complete):** consume only the exact approved, independently authenticated, fixture-owned,
    initially unconsumed post-reconciliation policy request after revalidating the exact
    acknowledgement-reconciliation record and receipt plus their complete immutable predecessor
    chain. Accept exactly one route-compatible opaque attempt authority for continuation handoff,
    successful terminal graph result, or failed terminal graph result. Materialize one deterministic,
    versioned, fixture-owned route-bound attempt record and one separate canonical executor receipt
    proving one logical local attempt, one attempt-record write, one receipt write, exact authority
    consumption, complete predecessor-chain revalidation, no duplicate attempt, and an unchanged
    request. Exact replay, restart, identical concurrency, identical pre-existing artifacts, and
    attempt-before-receipt recovery are idempotent; conflicts, rejection, missing or changed policy,
    reconciliation, acknowledgement, delivery, consumer, result, or predecessor evidence, inferred
    or escalated authority, or malformed, noncanonical, unknown-field, trailing, partial, empty,
    oversized, symlinked, unsafe, or tampered artifacts fail closed. The attempt and receipt are
    evidence only and perform or grant no lifecycle advancement, graph mutation, dependency work,
    scheduling, task launch, node execution, result collection, output materialization, delivery,
    consumer invocation, retry, repair, cancellation, queue processing, callback, publication,
    provider, connector, broker, ForgePipe, process, network, remote execution, validation, checkout
    mutation, Git, checkpoint, commit, push, external action, or future downstream authority.

This completes the post-delivery acknowledgement/reconciliation outcome sub-chain. TASK-015 is
closed; any further bounded slice requires a separately reviewed task and explicit scope, and must
not infer another policy, lifecycle, scheduling, publication, or external-authority hop from this
completed evidence.

The strict schema-2 readiness/ownership metadata contract and read-only unique-ready inspection are
complete. Explicit `backlog.inspect` retains its behavior, while literal `--next` succeeds only for
one uniquely eligible `decision_ready` plus `unclaimed` entry after complete index validation. The
canonical backlog remains conservatively `unclassified` and `unclaimed`, so canonical `--next`
rejects. Automatic promotion, selection among multiple tasks, task claiming, owner identity, and
dynamic ownership mutation remain unimplemented; no prose, ordering, availability, recent activity,
commit history, or task presence implies readiness.

## Acceptance Criteria For This Extension

- Standalone, local-only DockPipe workflows retain their current behavior and require no service.
- DorkPipe owns graph and placement decisions; DockPipe receives only a local execution contract.
- Host/runtime/guest dimensions are separately matched and reported.
- Existing DockPipe operation events/results are reused inside an outer DorkPipe envelope.
- The completed foundation binds one exact commit through the fake broker and injected connector,
  accepts deterministic structured results/artifacts, and proves reconnect, idempotent receipt,
  cancellation, and cleanup without running the commit yet.
- Machine, capability snapshot, lease, and execution receipt identities are separately bound and
  cannot be inferred from connection presence or substituted for one another.
- A placement request alone cannot authorize dispatch. The placement-bound dispatch request binds
  the exact selected machine/capability and complete finalized execution request and cannot widen
  fixture-only submission authority. Only its explicit one-time submission may issue the exact fake-
  broker lease; that evidence cannot invoke execution or become a receipt.
- Default execution needs neither DockPipe-hosted cloud infrastructure, an external edge provider,
  nor a public node listener.
- A failure cannot silently duplicate a task, replay a stale cancellation, hide cleanup residue, or
  automatically publish a change.
- Cloudflare, ngrok, direct TLS, managed hosting, and any compatibility transport exercise the same
  broker/node contract and cannot add execution authority.

## Open Decisions For The Extension

- Whether the current DockPipe process-runner/cancellation primitives need one small generic
  machine-readable cancel/status API before the connector can make its cleanup guarantee.
- The exact target schema location and migration path in the DorkPipe orchestration contract.
- Artifact transfer limits, retention, and checksum/signature policy for large guest logs/images.
- The package/component boundary for the first production broker and node connector after the
  package-owned fake proves the contract.
- Which standard wire framing and authentication profile to use without coupling the contract to one
  edge provider.
- Managed-service tenant, retention, quota, billing, availability, and custom-domain policy; none is
  required for local/private or BYO operation.
