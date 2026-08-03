# backlog.remote

`backlog.remote` is the offline TASK-015 path. It resolves either one explicit schema-2 entry or the
only entry eligible through the literal `--next` selector from `docs/agents/task-index.yaml`, requires
`decision_ready` readiness plus `unclaimed` ownership,
validates its exact linked task document and a one-line bounded slice,
compiles reviewable immutable request artifacts, preflights the Codex Cloud CLI contract from narrow
package-owned help fixtures, records fixture dispatch identity, and ingests one explicitly bound
completion-candidate fixture plus later fixture-backed status and diff observations as untrusted
evidence, followed by one fixture-backed opaque result observation and one fixture-backed opaque
validation-receipt observation, then performs one artifact-only mechanical patch-structure and
allowed-path boundary check followed by isolated temporary-copy mechanical application, exact-input
offline validation execution, and one explicit local semantic-review decision. Only approved review
bound to passed validation emits a separate readiness artifact. A final distinct fixture-backed
decision may then authorize exactly one rollback-safe application of the accepted patch to the
consumer checkout. The workflow never invokes Codex Cloud, live-polls evidence, infers either
approval, commits, pushes, checkpoints, syncs, publishes, or selects another task.

Run it from the consumer repository root with every authority-bearing input explicit:

```bash
dockpipe --package dorkpipe --workflow backlog.remote --workdir . \
  --var DORKPIPE_BACKLOG_TASK_ID=TASK-015 \
  --var 'DORKPIPE_BACKLOG_SLICE=Implement only the offline evidence and temporary-copy application proof.' \
  --var DORKPIPE_BACKLOG_BASELINE=0123456789abcdef0123456789abcdef01234567 \
  --var DORKPIPE_BACKLOG_ENVIRONMENT_REF=codex-environment-id \
  --var DORKPIPE_BACKLOG_BRANCH_REF=js/dev \
  --var 'DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON=["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]' \
  --var 'DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON=["No src/lib or src/cmd changes","No live provider invocation"]' \
  --var 'DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON=["go test ./packages/dorkpipe/lib/orchestrationhelper"]' \
  --var "DORKPIPE_BACKLOG_VALIDATION_INPUTS_JSON=$(tr -d '\r\n' < packages/dorkpipe/tests/fixtures/backlog.remote/validation-input-files.json)" \
  --var 'DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON=["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]' \
  --var DORKPIPE_BACKLOG_COMPLETION_FIXTURE=/reviewed/path/completion-candidate.json \
  --var DORKPIPE_BACKLOG_STATUS_FIXTURE=/reviewed/path/remote-status.json \
  --var DORKPIPE_BACKLOG_DIFF_FIXTURE=/reviewed/path/remote-diff.json \
  --var DORKPIPE_BACKLOG_RESULT_FIXTURE=/reviewed/path/remote-result.json \
  --var DORKPIPE_BACKLOG_VALIDATION_RECEIPT_FIXTURE=/reviewed/path/validation-receipt.json \
  --var DORKPIPE_BACKLOG_SEMANTIC_REVIEW_FIXTURE=/reviewed/path/semantic-review-decision.json \
  --var DORKPIPE_BACKLOG_CHECKOUT_APPLICATION_FIXTURE=/reviewed/path/checkout-application-approval.json
```

Use `DORKPIPE_BACKLOG_TASK_ID=--next` only for read-only inspection when the complete strict index
contains exactly one `decision_ready` plus `unclaimed` entry. Zero eligible entries reject with
`no_decision_ready_task`; multiple eligible entries reject with `ambiguous_decision_ready_tasks`.
Index order never resolves ambiguity, and selection never claims or mutates the index.

`DORKPIPE_BACKLOG_CONSUMER_ROOT` explicitly selects the consumer root; it defaults to the workflow
workdir. Patch rehearsal and validation read only immutable declared authority. Checkout application
may mutate only the accepted changed paths, and only after the separate approval fixture is bound to
the exact readiness, patch, changed paths, and consumer preimage manifest.

The workflow writes under the normal `backlog-remote` artifact scope:

- `backlog-selection.json` uses `dorkpipe.backlog-selection/v3` and records the exact selected task,
  linked path, bounded slice, baseline, `decision_ready` readiness, `unclaimed` ownership, source
  digests, and the exact selector object: `explicit_task` plus its task ID or
  `unique_decision_ready`. `automatic_selection_performed` is false for explicit selection and true
  for literal `--next` inspection, including deterministic rejection evidence. Its authority map explicitly denies ranking, scoring,
  priority, recommendation, claiming, leasing, ownership, mutation, scheduling, dispatch, execution,
  provider, Git, and publication authority. A rejected inspection writes the same contract with a
  deterministic rejection code and no downstream artifact.
- `remote-request.json` and `remote-request.md` use `dorkpipe.remote-request/v2` and bind the explicit
  target, allowed paths, hard boundaries, validation declaration, context-source digests, and a
  separate complete validation-input manifest under one request fingerprint. `source_files` remains
  request/context evidence, `scope.allowed_paths` remains patch-write scope, and only
  `validation_input_files` grants bounded file authority for a later validation workspace.
- Every validation input is an exact sorted repository-relative regular file with a SHA-256 and byte
  count. Directories, globs, inferred walks, duplicates, unsorted declarations, absolute/drive/
  traversal/backslash paths, linked or reparse-point ancestors, root escapes, generated locations,
  secrets, Git internals, and provider-private paths fail closed. The complete list is capped at 256
  files and 8 MiB aggregate. `validation_input_manifest` records `complete_list` semantics, count,
  aggregate bytes, and a canonical fingerprint over the entire ordered list.
- `remote-adapter-compatibility.json` binds the inspected adapter/CLI contract to that request
  fingerprint and the explicit environment/branch refs. It records required commands, documented
  inputs, receipt/task-ID support, the compatibility status and exact fail-closed reason, enabled
  adapter modes, and whether live submission is enabled.
- `remote-task.json` records one opaque fixture task ID, that fingerprint, the target references,
  deterministic fixture time, compatibility fingerprint, and adapter identity with
  `provider_invoked: false`.
- `completion-candidate.json` records one candidate/replay identity, exact task/request/dispatch/
  adapter/environment/branch binding, deterministic observation time, and an untrusted terminal
  claim. Its only authoritative state is `completion_candidate`; every review, retrieval,
  validation, apply, commit, push, and publication transition remains false.
- `remote-status.json` records one status observation/replay identity bound to the full accepted
  candidate fingerprint and candidate identity plus the immutable task/request/dispatch/adapter/
  environment/branch identity. Its canonical observation time must be later than both dispatch and
  candidate observation times. The fixture's `completed` status is explicitly untrusted and
  non-authoritative; the artifact remains at `state: completion_candidate`, with review, diff/result
  retrieval, validation, apply, commit, push, and publication false.
- `remote-diff.json` records one diff observation/replay identity bound to the canonical accepted
  status and candidate fingerprints plus the immutable task/request/dispatch/adapter/environment/
  branch identity. Its observation time is later than dispatch, candidate, and status times. It
  records the exact patch SHA-256 and byte count, package-owned fixture provenance, and only
  `state: completion_candidate`; review, result retrieval, semantic and allowed-path verification,
  validation, apply, commit, push, and publication remain false.
- `remote-diff.patch` contains the exact adjacent fixture patch bytes. They are opaque and untrusted:
  retrieval checks only the declared SHA-256 and does not parse paths, infer authorization, apply the
  patch, or infer lifecycle completion.
- `remote-result.json` records one result observation/replay identity bound to the canonical accepted
  diff, exact patch SHA-256 and byte count, canonical status and candidate fingerprints, and the
  immutable task/request/dispatch/adapter/environment/branch identity. Its fixture-owned result
  string is opaque, untrusted, non-authoritative, and uninterpreted. The artifact remains only at
  `state: completion_candidate`; validation-receipt retrieval, review, semantic interpretation,
  validation, apply, commit, push, and publication remain false.
- `validation-receipt.json` uses `dorkpipe.validation-receipt/v2` and records one receipt
  observation/replay identity bound to the canonical
  accepted result, diff, status, and candidate fingerprints; exact accepted patch SHA-256 and byte
  count; immutable task/request/compatibility/dispatch/adapter/target identity; and the exact
  `required_validation` array plus its canonical fingerprint and the immutable aggregate validation-
  input fingerprint. Its receipt string is opaque,
  untrusted, non-authoritative, and uninterpreted. The required-validation declaration is preserved
  as request evidence only and is not executed. The artifact remains only at
  `state: completion_candidate`; review, validation execution, apply, commit, push, and publication
  remain false.
- `patch-boundary.json` uses `dorkpipe.patch-boundary/v2` and revalidates the complete immutable
  chain from `remote-request.json` and its
  exact markdown through compatibility, dispatch, candidate, status, diff and exact patch bytes,
  result, and validation receipt. It binds every accepted identity/fingerprint, the patch SHA-256
  and byte count, target and adapter refs, the exact immutable `allowed_paths` declaration and its
  canonical fingerprint, and the sorted changed paths. Its authority is limited to the supported
  patch grammar and segment-aware lexical containment. It remains at `completion_candidate` and
  explicitly records that semantic correctness, validation, review readiness, apply, commit, push,
  and publication were not performed.
- `patch-application.json` uses `dorkpipe.patch-application/v2` and requires and revalidates that
  complete boundary before reading source
  files. It binds the canonical boundary and upstream identities, exact patch, request/compatibility/
  dispatch/task/adapter/target refs, baseline as an unverified request declaration, sorted changed
  paths, canonical per-file preimage/postimage manifests, and deterministic file/hunk counts. It
  records successful `temporary_copy_only` mechanical application and cleanup while explicitly
  denying semantic review, validation execution, consumer mutation, `ready_for_review`, apply to the
  checkout, commit, push, and publication. It contains no contents, absolute or temporary paths,
  timestamps, process IDs, hostnames, or durable patched source snapshot.
- `validation-execution.json` uses `dorkpipe.validation-execution/v1`, revalidates the complete chain
  through patch application, constructs only the declared validation-input-plus-changed-path
  workspace, reproduces the exact accepted patch, and runs only the bounded direct offline argv
  declaration. Passed and failed evidence both remain at `completion_candidate`; passing validation
  is explicitly non-authoritative and does not imply semantic approval or readiness.
- `semantic-review-decision.json` uses `dorkpipe.semantic-review-decision/v1` and records exactly one
  explicit package-owned local `approved` or `rejected` decision bound to the complete immutable
  chain, exact patch, sorted changed paths, validation status, and bounded review scope. It remains at
  `completion_candidate` and grants no apply, commit, push, publication, or next-task capability.
- `ready-for-review.json` uses `dorkpipe.ready-for-review/v1` and exists only when the explicit
  decision is approved and validation passed. It binds the decision fingerprint and complete
  candidate identity, records only `state: ready_for_review`, and grants no mutation, Git,
  publication, or next-task authority.
- `checkout-application-approval.json` uses `dorkpipe.checkout-application-approval/v1` and records
  one separate explicit `approved` or `rejected` local decision with scope
  `accepted_changed_paths_exact_patch_once`. It binds readiness and the complete immutable chain,
  exact patch, sorted changed paths, and canonical consumer preimage manifest. Approval grants only
  that one bounded application; commit, push, publication, checkpoint, sync, and next-task
  capabilities remain false.
- `checkout-application.json` uses `dorkpipe.checkout-application/v1`, is emitted only after an
  approved operation reaches and verifies the complete postimage set, and records package-local
  `state: applied_for_review`. It binds the approval fingerprint, complete readiness identity,
  exact pre/post manifests, applied paths, file/hunk counts, cleanup, and consumer verification. It
  grants no remaining application, Git, publication, or task-selection authority.

`orchestrate-helper backlog-followup <artifact-root>` validates and recovers identity using only the
immutable request, compatibility, and dispatch artifacts. Completion ingestion uses those same
artifacts and never rereads the repository. A candidate observed at or before dispatch is stale;
wrong bindings, duplicate candidate IDs, replayed replay IDs, malformed fixtures, and tampered
immutable artifacts fail before `completion-candidate.json` is written. Once accepted, later
duplicate or replay rejection leaves both the accepted candidate and dispatch bytes unchanged.

Status retrieval revalidates the immutable request, compatibility, dispatch, and the complete
accepted candidate artifact without rereading the repository. An observation at or before the
candidate time is stale. Wrong candidate/task/request/dispatch/adapter/target bindings, duplicate
observation IDs, replayed replay IDs, malformed fixtures, tampered evidence, and tampered immutable
artifacts fail before `remote-status.json` is written. Rejection cannot create review, diff, result,
validation, or apply artifacts and cannot alter the accepted candidate or dispatch identity.

Diff retrieval revalidates the immutable request, compatibility, dispatch, complete candidate, and
complete status artifacts without rereading the repository. An observation at or before status is
stale. Wrong status/candidate/task/request/dispatch/adapter/target bindings, duplicate observation
IDs, replayed replay IDs, malformed or missing metadata, missing patch bytes, checksum-tampered patch
bytes, and tampered immutable artifacts fail before either accepted diff artifact is written. The
two outputs use temporary files and a rollback-on-rename-failure pair write. Clean-chain rejection
cannot create review, result, validation, or apply artifacts; duplicate or replay rejection cannot
change the accepted diff, status, candidate, or dispatch bytes.

Result retrieval revalidates that complete chain plus the accepted diff metadata and the exact
accepted patch bytes. An observation at or before the diff time is stale. Wrong diff, patch, status,
candidate, task, request, dispatch, adapter, or target bindings; duplicate observation IDs; replayed
replay IDs; malformed or missing fixtures; and tampered upstream artifacts or patch bytes fail before
`remote-result.json` is written. Clean-chain rejection cannot create validation, review, or apply
artifacts; duplicate or replay rejection cannot change the accepted result or any upstream bytes.
The fixture metadata is package-owned proof input, not a provider response, callback, signed receipt,
hidden transcript, or undocumented Codex contract.

Validation-receipt retrieval revalidates that complete chain plus the accepted result, the exact
`required_validation` declaration, and the complete validation-input fingerprint without rereading
backlog prose or the consumer checkout. An
observation at or before the result time is stale. Wrong result, diff, patch, status, candidate,
task, request, required-validation, validation-input, compatibility, dispatch, adapter, or target bindings; duplicate
observation IDs; replayed replay IDs; malformed or missing fixtures; and tampered upstream artifacts
or patch bytes fail before `validation-receipt.json` is written. Clean-chain rejection cannot create
review, validation-execution, or apply artifacts; duplicate or replay rejection cannot change the
accepted receipt or any upstream bytes. Fixture fields are package-owned proof input, not a provider
response, callback, signed receipt, hidden transcript, or undocumented Codex contract.

Patch-boundary verification supports only ordinary Git unified text modifications. Every section
must contain exactly one unquoted `diff --git a/<path> b/<path>` header for the same path, one
non-submodule `index <old>..<new> <mode>` line, matching `--- a/<path>` and `+++ b/<path>` headers,
and one or more ordinary `@@ ... @@` hunks. Hunk contents are opaque. Combined diffs, binary or
submodule patches, add/delete forms, mode metadata, rename/copy metadata, quoted/escaped paths,
mismatched headers, repeated path sections, and malformed or otherwise unsupported structures fail
closed before `patch-boundary.json` is written.

Patch paths must be canonical forward-slash repository-relative paths with no absolute or drive
prefix, backslash, traversal, empty component, control/whitespace character, ambiguous quoting, or
generated, secret-like, Git-internal, or provider-private location. A changed path is accepted only
when it equals one immutable allowed path or starts with that allowed path plus `/`; string-prefix
collisions such as `packages/dorkpipe-evil` never match `packages/dorkpipe`. The check never consults
Git, the consumer checkout, or path existence. Repeated verification is idempotent only when the
entire upstream chain and derived artifact are identical; malformed or tampered existing boundary
evidence is rejected.

Temporary-copy application uses only the boundary's sorted changed paths. The consumer root, every
ancestor, and every changed source must pass root-containment and non-symlink checks; each source must
be a regular file. Only those files are copied into a fresh temporary directory, and cleanup is
required before a receipt can be materialized. Success and every ordinary failure remove the
temporary directory; a cleanup error rejects the operation and prevents receipt creation.

Application supports multiple files and multiple ordinary hunks. Hunk coordinates must be ordered,
non-overlapping, consistent between old and new images, and exactly match declared line counts.
Context and removed lines must exactly match LF-terminated UTF-8 preimage text without CR or NUL
bytes; additions are copied exactly. No-newline markers are rejected fail-closed because this slice
does not define an unambiguous end-of-file reconstruction rule. Any boundary-accepted form that this
application grammar cannot apply fails before `patch-application.json` is written.

The checked fixture's complete input list names `go.mod`, `go.sum`, the root embed metadata and
minimal checked embed matches, every target-package Go and test file, the local `domain`,
`infrastructure/packagebuild`, and `infrastructure` source dependencies, and the embedded workflow
schema required by `go test ./packages/dorkpipe/lib/orchestrationhelper`. It does not use a directory,
glob, dependency walk, whole-checkout copy, generated binary, or cache as authority.

The temporary-copy application receipt is idempotent only when the immutable chain, exact boundary
and patch bytes, source preimages, and derived postimages all match. Mechanical and validation
success are not semantic or checkout-application approval.

Checkout application completes every chain, readiness, path, link/reparse, containment, patch, and
preimage check before mutation. Exact postimages and byte-for-byte rollback copies are prepared in
the same directories before the first replacement. Every postimage is verified before cleanup and
receipt publication. Any in-process mutation failure restores and verifies every exact preimage;
rollback and cleanup failures have distinct rejection codes and can never emit success. Missing,
stale, unexpected, mixed pre/post, linked, escaping, or non-regular files fail closed without
automatic repair.

An accepted receipt plus the complete postimage set is an idempotent restart. An accepted approval
without a receipt may recover only when every changed file exactly matches the complete accepted
postimage manifest; a mixed or unknown set rejects. Rejected decisions restart without mutation.
Duplicate or replayed approval identities, attempts to change an accepted decision, and tampered or
conflicting artifacts reject without overwrite.

The bounded read-only selection slice accepts the literal `--next` through the existing task-ID
input. It does not add claiming, promotion, scheduling, another acknowledgement-chain step, raw Git,
or workflow mutation.

The strict schema-2 backlog contract requires every entry to declare `dispatch.readiness` as one of
`unclassified`, `decision_ready`, or `decision_blocked` and `dispatch.ownership` as either
`unclaimed` or `external_active`. Explicit inspection succeeds only for `decision_ready` plus
`unclaimed`; unclassified, blocked, externally active, missing, partial, legacy, or unknown metadata
fails closed. Read-only `--next` inspection succeeds only after the complete strict index contains
exactly one such eligible entry; zero or multiple eligible entries reject, and index order never
breaks a tie. The canonical backlog is conservatively initialized to `unclassified` and `unclaimed`,
so canonical `--next` inspection rejects. Narrative status remains in each linked task document.
Prose, ordering, availability, recent activity, commit history, or task presence cannot imply
readiness. Automatic promotion, ranking, claiming, owner identity, dynamic ownership mutation,
scheduling, and dispatch remain unimplemented.

The checked package fixture records the exact read-only inspection of `codex-cli 0.144.1` through
`codex --version`, `codex cloud --help`, and `codex cloud exec --help`. The documented submit surface
is `codex cloud exec --env <ENV_ID> [--branch <BRANCH>] [QUERY]`; it exposes no machine-readable
submission receipt or stable opaque task-ID response contract. Compatibility is therefore
`unsupported`, live submission remains disabled, and fixture dispatch remains the only enabled
adapter. The preflight never parses submission terminal text, credentials, authentication state, or
environment listings. A malformed compatibility contract fails before fixture dispatch and leaves
no `remote-task.json`. Completion, status, diff, result, and validation-receipt fixtures are
package-test evidence, not undocumented provider responses or callback schemas.
