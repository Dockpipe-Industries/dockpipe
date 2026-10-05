##### Accepted post-reconciliation implementation plan — 2026-08-04

**Plan status and boundary.** This is the decision-complete implementation plan authorized by the
accepted product/storage direction above. It does not select schema syntax, add an operation, or
authorize any implementation slice. The working state name remains `reconciled_outcome_unknown`
with exactly the accepted meaning: verified idle for the retained provider thread and an unknown
prior prompt outcome. Nothing in this plan converts verified idle into prompt completion or grants
Send, replay, retry, fallback, claim reuse, or prompt authority.

**Current-to-future ownership map.** The current records are separate authorities only until the
first valid aggregate commit. Paths below are relative to the DorkPipe package-state root returned by
`statepaths.PackageStateDir`; `<session-digest>` is the current SHA-256 naming of the exact Pipeon
session ID.

| Record | Current owner and exact location | Current readers | Current writers | Post-cutover responsibility |
| --- | --- | --- | --- | --- |
| Immutable session adapter binding | provider-pool; `provider-pools/session-adapters/<session-digest>.json` | adapter resolution and the recovery-candidate classifier/operation | first-use adapter pinning only | copied into the aggregate as authoritative identity; the legacy file stays frozen for compatibility/audit and all legacy readers defer to the aggregate |
| Last completed App Server session state | provider-pool; `provider-pools/app-server/sessions/<session-digest>.json` | turn claiming, exact candidate classification, and the recovery-only operation | verified-idle turn completion through same-directory temporary-file rename | copied into the aggregate as the retained completed-turn/provider-session/recovery/model/reasoning baseline; the legacy file stays frozen and is never advanced after cutover |
| Consecutive unresolved turn claim | provider-pool; `provider-pools/app-server/sessions/<session-digest>.json.lock` | turn claiming, candidate classification, exact rollback, and turn completion | exclusive claim creation before dispatch; exact pre-`turn/start` rollback or removal after verified-idle completion | consumed semantically into the aggregate as the permanent unknown-turn/high-water/no-replay record; the legacy file remains frozen rather than being deleted or reused |
| Retained adapter and post-turn guard slice | Pipeon extension host; the session's `codexSessionAdapter` and `codexAppServerPostTurnSnapshot` fields in VS Code workspace state key `pipeon.chatState.v2` | Pipeon restore/normalization, extension-host pre-dispatch guard, view projection, and webview Send guard | Pipeon session creation/migration, closed provider-pool response projection, and workspace-state persistence | the adapter/status slice becomes projection-only after migration; it may be refreshed from the aggregate but cannot authorize, veto, or repair an authoritative transition |
| Pipeon messages and other chat presentation state | Pipeon workspace state outside the exact retained adapter/guard slice | Pipeon extension and webview | Pipeon chat flows | remains Pipeon-owned, mutable presentation data and never enters the aggregate or its fingerprint |

The future authority is one canonical per-session App Server lifecycle aggregate under
provider-pool package state. Its exact filename and directory syntax remain an open schema/storage
choice. Provider-pool alone owns aggregate validation and mutation. Pipeon owns only a bounded
display projection and delivery of an explicit user request to the owner; it never owns lifecycle
truth.

**Aggregate contract plan.** The following are accepted semantic requirements, not field names or a
schema definition:

- **Envelope and identity:** one explicit aggregate schema/version, one exact Pipeon session ID, the
  exact `codex_app_server` binding, exact provider-session ID, recovery-evidence reference, retained
  model and reasoning values, and one monotonically increasing revision. The record must identify
  one session unambiguously and must not permit adapter rebinding.
- **Lifecycle state:** the post-reconciliation state is exactly `reconciled_outcome_unknown`, with
  outcome-unknown true and reconciled-to-verified-idle true, but no terminal outcome, recovered
  content, or completion/failure/cancellation classification.
- **Turn boundary:** retain the last completed turn, the consecutive unknown pending turn, and a
  high-water mark at least equal to that pending turn. At migration the pending turn must equal the
  completed turn plus one and becomes permanently unavailable. Every later fresh claim must be
  strictly greater than the high-water mark.
- **Fingerprint:** retain an exact pre-state fingerprint over a length-prefixed encoding of every
  named source path/role and exact source byte sequence plus the accepted session, provider-session,
  recovery-evidence, model, reasoning, completed-turn, and pending-turn identities. The digest
  algorithm and serialized field syntax remain gated, but ambiguity, order dependence, or omitted
  source roles are forbidden.
- **Observation consumption:** retain an exact recovery-observation fingerprint bound to that
  pre-state fingerprint and the one successful in-lock recovery attempt. The observation is recorded
  as consumed by the first aggregate commit and cannot be reused after commit, restart, or from
  another process.
- **Replay prohibition:** retain both consumed-unresolved-claim and permanent replay-forbidden
  semantics. Neither absence of a legacy claim nor a later user decision can make the unknown pending
  turn reusable.
- **User decision:** begin in `required`; a future operation may record only one explicit decision to
  acknowledge the unknown outcome and continue with a fresh later turn, bound to the exact current
  aggregate revision and reconciliation fingerprint. Acceptance grants no replay or dispatch by
  itself. At most one strictly later fresh-turn claim may atomically consume that decision; stale,
  duplicate, substituted, or already-consumed decisions fail closed.
- **Validation:** reject missing or extra authority-bearing data, unknown schema/version/state,
  malformed or non-canonical bytes, invalid identities, revision regression, impossible turn
  relationships, fingerprint mismatch, unconsumed/reused observation, weakened replay prohibition,
  invalid decision transitions, oversized/non-regular storage, or any partial record. A missing or
  invalid aggregate after cutover blocks dispatch; no legacy record, marker, projection, label, or
  caller assertion fills the gap.

Exact JSON or other encoding, field names, bounds, revision starting value, digest algorithm, file
name, permissions, and schema-evolution syntax are later schema/storage decisions. They may express
the semantics above but may not change them. No schema is created by this plan.

**Owner-operation boundary.** The future operation belongs to the DorkPipe provider-pool component,
with storage paths supplied by `packages/dorkpipe/lib/statepaths`; it does not belong in DockPipe
engine code, `providersession`, appserversupervisor, MCP, or Pipeon. The same owner operation is
responsible for strict source loading, fingerprint construction, session-scoped cross-process
locking, recovery observation, pre-state comparison, first migration commit, later aggregate
replacement, post-commit reload, observation/replay rejection, and an authority-safe result.

Its semantic request is only: exact Pipeon session identity, the requested owner action, and access
to the exact canonical Pipeon legacy adapter/guard slice when migration requires it. Whether that
slice is read through a trusted host callback, a bounded request value verified against the
authoritative workspace store, or another package-local bridge is an open gate. The operation itself
must load the three provider-pool records, construct the expected fingerprint, mint the recovery
observation, and compare revisions/bytes. A caller-supplied digest, display status, marker file,
aggregate-file existence, process-local mutex, or claimed observation carries no authority.

The semantic result is one closed outcome: unchanged/rejected with a bounded reason, committed with
the exact validated aggregate revision and a projection derived from it, or unknown-commit-result
requiring restart reload. It returns no prompt, provider content, replay token, raw evidence, or
automatic retry authority. Pipeon is the intended first requester because it owns the exact legacy
workspace slice and guard. CLI, MCP, other consumers, and administrative tools are not authorized by
this plan; adding any requester or public surface requires a separate maintainer choice. All callers
request work, while provider-pool remains the only transaction owner.

**Migration and authority cutover sequence.** A future migration must perform these steps as one
owner-controlled transaction:

1. Resolve the exact session-scoped aggregate path and acquire the verified OS-backed exclusive lock.
   Hold it through source observation, comparison, replacement, synchronization, reload, and result
   classification. A current `.json.lock` turn-claim file is evidence, not this transaction lock.
2. Require aggregate absence for initial migration. If an aggregate exists, validate it and use only
   the revision-based aggregate operation; malformed existence blocks rather than falling back to
   legacy files.
3. Strictly read the exact current binding JSON, completed-state JSON, unresolved claim JSON, and the
   canonical Pipeon retained-adapter/unknown-outcome guard slice. Require the accepted exact
   `codex_app_server` binding, strict completed state, consecutive claim, retained Pipeon adapter,
   and `recovery_required` / `outcomeUnknown: true`. Preserve each named path/role and exact byte
   sequence.
4. Construct the expected pre-state fingerprint from those bytes and the exact bound identities.
   Reject malformed, missing, mismatched, substituted, non-canonical, oversized, non-regular, or
   changing evidence before provider activity.
5. While still holding the same lock, invoke the recovery-only observation for that exact bound
   pre-state. Bind the successful verified-idle observation to the fingerprint inside the owner;
   failure or ambiguity produces no aggregate and changes no source record.
6. Immediately reread all four legacy inputs from their authoritative stores and byte-compare them
   with the captured values. Revalidate every semantic identity, aggregate absence, and observation
   binding. Any difference rejects without mutation.
7. Build the complete first aggregate: preserved binding/session/policy/completed turn,
   `reconciled_outcome_unknown`, consumed claim/observation, pending-turn high-water mark, permanent
   replay prohibition, and user-decision-required state. Fully write and synchronize a temporary
   file in the aggregate directory.
8. Make the complete aggregate authoritative at the single verified same-directory, same-volume
   atomic replacement/create point; synchronize the resulting aggregate and parent directory before
   acknowledging success. If the platform cannot prove every required guarantee, stop closed before
   cutover.
9. Reload and strictly validate the authoritative aggregate. Before replacement, the legacy state is
   authoritative. After replacement, the aggregate is authoritative even if acknowledgement or
   projection fails. An uncertain replacement result is never retried; restart reload decides which
   complete revision is visible.
10. Freeze the three provider-pool legacy records and the exact Pipeon adapter/guard slice for the
    chosen compatibility/audit retention period. Aggregate-aware legacy readers defer to the
    aggregate; legacy writers reject the session rather than update, remove, roll back, or recreate
    any frozen record. Mixed-version and downgrade enforcement remain an explicit gate.
11. Only after authoritative commit and reload may Pipeon request/receive a projection refresh. A
    missing, stale, reordered, or failed projection write leaves the provider-pool guard intact and
    cannot authorize normal Send.

**Cross-platform storage evidence plan.** No platform guarantee is assumed by this plan. The storage
primitive cannot be selected or activated until fixture-backed and process-backed evidence proves the
following on each currently supported Windows, Linux, and macOS host/filesystem combination:

| Property | Required evidence on every platform | Fail-closed result if absent or ambiguous |
| --- | --- | --- |
| Cross-process exclusion | two independent processes contend for the same session; only one enters the observation/compare/commit region; a different session does not share authority; process exit/crash has documented lock semantics; restart cannot mistake the turn-claim file or a stale marker for ownership | no observation or migration; session remains guarded |
| Same-directory atomic replacement/create | readers racing repeated old-to-new revisions observe only one complete valid old or new aggregate, never missing/truncated/mixed bytes; replacement is on the same volume and rejects symlink/reparse/path substitution as applicable | no replacement; old authority remains, or unknown result is resolved only by reload |
| File synchronization | the temporary file is fully written and synchronized before replacement, and the resulting aggregate is synchronizable before success | do not acknowledge success; if replacement may have occurred, classify unknown commit and reload after restart |
| Parent-directory synchronization | creation/replacement and directory entry durability are demonstrated with the platform's documented supported primitive | platform/filesystem is unsupported for cutover; do not emulate with ordered writes |
| Restart visibility | a separate fresh process after each injected boundary loads exactly the old full revision or new full revision and derives the correct guard; consumed observation and pending-turn high-water survive | block dispatch and replay; require evidence review |
| Unknown commit result | injected failures after replacement but before each synchronization/acknowledgement point yield no automatic retry; fresh reload validates one full revision, rejects duplicates, and preserves no-replay | report unknown commit, block, and reload; never re-observe or replay automatically |

Evidence must include success and injected-failure cases around lock acquisition, source reads,
observation, reread, temporary write, temporary sync, replacement, aggregate sync, directory sync,
reload, and acknowledgement. Platform/version/filesystem support, including whether required
directory synchronization is actually available, remains a separately reviewed gate. Unsupported or
unproven environments must keep the legacy guard and reject migration.

**Guard and continuation integration plan.** Provider-pool must validate the aggregate guard before
any App Server turn claim, prompt dispatch, fallback, provider call, supervisor/child creation, or
claim rollback/reuse. Once cut over, aggregate absence, corruption, unknown state, user-decision
required/accepted-but-unconsumed state, stale revision, or unknown commit result blocks. The legacy
claim remains permanent evidence and is never treated as a live reusable transaction lock.

Pipeon must obtain its status projection from the validated aggregate. Its extension-host preflight
is a convenience defense; authoritative provider-pool validation remains mandatory even if the
webview is stale or bypassed. The webview keeps normal direct Codex Send disabled while the
projection is guarded, and projection failure must render a blocked/unknown condition rather than
enable Send. Slash-command and other existing routing behavior is unchanged unless a later expressly
authorized slice says otherwise.

A future user-decision operation must authenticate one explicit local choice, bind it to the exact
current revision and reconciliation fingerprint, compare under the owner lock, and durably record it
without starting a turn. A later separate claim operation must reacquire the lock, reread the current
aggregate, reject stale/duplicate/consumed decisions, allocate a turn strictly greater than the
high-water mark, and atomically consume the decision with that fresh claim. The unknown pending turn,
consumed recovery observation, and all lower/equal turn numbers remain permanently rejected. No
current guard, claim, binding, snapshot, fallback, retry, or no-replay behavior changes in this plan.

**Validation and failure matrix.** Every future implementation lane must prove that no case below
authorizes replay or normal Send incorrectly.

| Case | Required authoritative result |
| --- | --- |
| Exact legacy migration | one valid first aggregate becomes authority; claim and observation are consumed, the pending turn is the high-water mark, user decision is required, legacy records freeze, and Pipeon refresh follows commit only |
| Failure before/during legacy read or fingerprinting | no observation, aggregate, or legacy mutation; existing host/webview guard remains |
| Recovery observation failure or ambiguity | no aggregate or legacy mutation; no retry, fallback, child continuation, or outcome inference |
| Reread/byte comparison failure | reject stale or substituted evidence without mutation, even when semantic JSON values appear equal |
| Temporary create/write/sync failure | old state remains authoritative; incomplete temporary data is ignorable and never authority |
| Replacement failure known not to commit | old state remains authoritative; no legacy write or projection refresh |
| Replacement/sync/acknowledgement ambiguity | unknown commit result; no retry/re-observation; fresh restart reload accepts only one complete valid revision and otherwise blocks |
| Malformed/missing/unknown-version aggregate after cutover | reject all dispatch and decisions; never defer to legacy or projection authority |
| Two concurrent processes | one serial winner at most; the loser observes changed revision/consumed observation and rejects without mutation |
| Restart before replacement | legacy evidence and guard remain authoritative; no aggregate authority is inferred from temporary/lock files |
| Restart after replacement | the aggregate alone supplies authority; frozen legacy files and stale Pipeon snapshot cannot override it |
| Duplicate/stale/substituted recovery observation | reject permanently, including after restart and from another process |
| Duplicate/stale/substituted/already-consumed user decision | reject without claim or dispatch; revision/fingerprint mismatch cannot be repaired by the caller |
| Pipeon projection missing, reordered, malformed, or unwritable | authoritative guard remains; host dispatch blocks and UI does not enable normal Send |
| Unsupported platform/filesystem guarantee | reject migration before observation/commit or remain blocked on unknown result; never emulate with separate ordered writes |
| Mixed-version legacy reader/writer or downgrade | reject/defer under the chosen compatibility mechanism; never update frozen files or bypass the aggregate |
| Fresh later continuation | only an exact accepted decision is atomically consumed with a strictly greater fresh-turn claim; the unknown turn and consumed observation remain replay-forbidden |

Focused tests must inject every boundary above, compare exact bytes before/after, start independent
processes where process isolation matters, and repeat restart validation. Existing adapter selection,
fallback, classifier, recovery-only operation, claim, Pipeon guard, and webview smoke lanes remain
regression checks rather than sources of new authority.

**Ordered future implementation slices.** Each slice is independently bounded and stops before the
next. Slice 1 is the first possible implementation and requires a new explicit maintainer
authorization because the accepted decision authorized this plan only. No slice inherits authority
for its successor; the authority-changing activation in Slice 4 and the user-decision/continuation
slices require their own explicit selection before work begins.

1. **Canonical aggregate contract and inert storage paths.** Responsibility: define the package-private
   schema syntax, strict validator/canonical encoder, revision/state/turn/fingerprint/decision
   invariants, size and file-type checks, and package-state path helper without any production reader
   or writer. Likely areas: a new provider-pool-owned Go file and focused tests under
   `packages/dorkpipe/lib/cmd/dorkpipe`, plus `packages/dorkpipe/lib/statepaths` only for the selected
   path. Invariants: the accepted semantics above are represented exactly; malformed/unknown records
   fail closed; no provider-neutral or engine type gains product knowledge. Validation lane: focused
   schema/path unit tests, existing statepaths tests, `git diff --check`. Non-goals: locks, file
   replacement, migration, observation, callers, Pipeon, controls, or dispatch. Stop when unused
   contract/path fixtures pass and no runtime reference exists.

   **Slice 1 status — implemented 2026-08-04.** The unused provider-pool contract selects schema
   `dorkpipe.provider-pool.app-server-lifecycle-aggregate` with numeric version `1`, revision origin
   `1`, and deterministic JSON with exactly one trailing newline. Aggregate files are bounded to
   16 KiB; identities are bounded to 256 UTF-8 bytes and allow only graphic, non-whitespace,
   non-control characters so whitespace substitution is rejected. Pre-state, recovery-observation,
   reconciliation, and future-decision fingerprints use the explicit
   `sha256:<64-lowercase-hex>` syntax, with duplicated binding references validated exactly; this
   slice defines syntax only and constructs no digest from legacy evidence. The state path is
   `provider-pools/app-server/aggregates/<sha256-exact-session-id>.json`, derived through
   `statepaths` after identity validation so the raw session ID is not exposed. The schema includes
   exact required, accepted-but-unconsumed, and consumed decision shapes, but provides no transition
   operation. The package-private regular-file loader is test-exercised and enforces type, size,
   canonical bytes, revision advancement when a prior revision is supplied, and exact session/path
   binding. No aggregate file has been written, and no runtime reader, writer, temporary file,
   replacement, synchronization, lock, migration, observation, compare-and-commit, cutover, guard,
   decision control, later-turn claim, or Pipeon projection exists. Slice 2 has not started;
   TASK-013 and CAS-14 remain open. Platform primitives/evidence, migration input/fingerprint
   construction, permissions/evolution/operator recovery, mixed-version compatibility, authority
   cutover, projection, decision UX/authenticity, later-turn consumption, and rollback remain open.
2. **Cross-process transaction/storage primitive and platform evidence.** Responsibility: implement
   and prove the selected session lock, same-directory write/sync/replace, directory sync, reload, and
   unknown-result classifications independently of lifecycle semantics. Likely areas:
   provider-pool package-local storage files/tests and narrowly separated OS-specific files if
   required. Invariants: one full old/new revision only; process-local mutexes and marker authority are
   forbidden; unsupported guarantees fail closed. Validation lane: unit fault injection plus
   independent-process Windows/Linux/macOS evidence for the matrix above. Non-goals: legacy reads,
   recovery observation, aggregate migration, Pipeon, user decisions, or dispatch. Stop at a reviewed
   evidence packet; do not activate the primitive for sessions.
3. **Locked reconciliation compare/commit operation, fixture-only.** Responsibility: compose strict
   legacy reads, internal fingerprinting, in-lock recovery observation, exact reread/byte comparison,
   first aggregate build, atomic commit, reload, consumption, and closed result classifications.
   Likely areas: provider-pool command package and focused fake-supervisor/storage tests; existing
   recovery-only code is called but not weakened. Invariants: no caller observation/digest authority,
   no legacy mutation, no prompt, no replay, and no production caller. Validation lane: full partial-
   failure matrix with exact tree snapshots, concurrency, and restart fixtures. Non-goals: cutover
   activation, public API/CLI/MCP, Pipeon projection, user decisions, or new turns. Stop when the
   operation remains package-private and unreachable outside tests.
4. **Legacy migration activation and provider-pool authority cutover.** Responsibility: add the
   explicitly selected trusted request path, execute first migration, make aggregate-aware
   provider-pool readers authoritative, freeze/defer/reject legacy writers, and enforce aggregate
   guard/replay rejection before dispatch. Likely areas: provider-pool dispatch/state files and
   tests, with the minimal trusted Pipeon-host bridge chosen at the open gate; no provider-neutral
   contract widening by default. Invariants: first aggregate replacement is the only cutover;
   aggregate sessions never fall back to legacy authority; projection is not required for safety.
   Validation lane: migration, mixed-reader/writer, dispatch-guard, fallback/claim regression,
   concurrency, restart, and unknown-result fixtures. Non-goals: display refresh, decision control,
   normal Send enablement, later-turn claiming, cleanup, or deletion. Stop with every migrated session
   still guarded in `reconciled_outcome_unknown` and Pipeon's old guard still safe if stale.
5. **Pipeon projection-only integration.** Responsibility: query a validated aggregate projection
   after commit, persist/render it as non-authoritative workspace state, and keep host/webview Send
   fail-closed on missing/stale/malformed/reordered/query-failed projections. Likely areas: Pipeon
   extension host, webview, and smoke tests, plus only the previously authorized read-only provider-
   pool projection surface. Invariants: messages remain Pipeon-owned; projection never mutates
   lifecycle authority; refresh failure cannot enable Send. Validation lane: extension unit/smoke
   cases for reload, reordering, stale revision, persistence failure, and host-vs-webview bypass.
   Non-goals: user-decision UX, aggregate mutation, claim consumption, dispatch, or legacy cleanup.
   Stop while every reconciled session still blocks normal Send.
6. **Explicit user-decision recording.** Responsibility: define the separately approved local
   decision control, exact revision/fingerprint binding, owner-side validation, and one durable
   accepted-but-unconsumed transition. Likely areas: provider-pool package, the chosen private
   transport/control, Pipeon host/webview, and focused tests; `providersession` remains unchanged
   unless a separate provider-neutral need is proven and authorized. Invariants: one explicit choice,
   no inference from idle/projection/messages, and recording never claims or dispatches a turn.
   Validation lane: stale/duplicate/substituted/cross-session/restart/ambiguous-delivery tests.
   Non-goals: replay, Send enablement, fresh claim, provider call, or unknown-turn deletion. Stop with
   the accepted decision durable but normal Send still guarded.
7. **Strictly later fresh-turn claim and one-time decision consumption.** Responsibility: under the
   owner lock, compare the exact decision/revision/fingerprint, allocate above the high-water mark,
   atomically consume the decision with the fresh claim, and integrate provider-pool pre-dispatch
   guard/replay rejection. Likely areas: provider-pool lifecycle/dispatch files and focused tests,
   followed by the minimal Pipeon Send projection update. Invariants: the unknown pending turn and
   observation remain permanently forbidden; no decision can authorize two claims; no exec fallback
   reuses the unknown turn. Validation lane: concurrent claim, restart, stale/duplicate decision,
   fallback boundary, exact turn numbering, host bypass, and Pipeon Send tests. Non-goals: legacy
   deletion, automatic decisions, retry/replay, additional consumers, or operations rollout. Stop
   after one fresh later turn can be claimed only through the exact decision path.
8. **Final cross-platform cutover and restart acceptance evidence.** Responsibility: run the reviewed
   controlled Windows/Linux/macOS matrix against the complete guarded flow, including lock
   contention, every injected storage boundary, unknown commit reload, frozen legacy behavior,
   projection failure, decision consumption, and permanent replay rejection. Likely areas: dedicated
   package-owned evidence fixtures/artifacts and the TASK-013 evidence record; production semantics
   change only if a separately authorized defect slice is required. Invariants: raw payloads and
   credentials remain excluded; unsupported platforms remain blocked; no failed case authorizes Send
   or replay. Validation lane: the full platform matrix plus existing focused Go/package/Pipeon
   regressions. Non-goals: fixing discovered defects in the evidence slice, new consumers, cleanup,
   operations guidance, or CAS-15. Stop for explicit maintainer acceptance; CAS-14 remains open until
   that acceptance.

