# TASK-013 Codex App Server adapter for top-level orchestrators

## Epic

### Problem statement

Pipeon and future top-level DockPipe orchestrators use host codex exec, buffered output and transcript-file discovery. That preserves native workspace sandboxing but cannot represent live turn state, native approvals, interruption completion or connection loss reliably. This is separate from successful disposable codex exec workflow workers.

### Expected value

- replace transcript/timestamp discovery with typed thread ownership;
- give Pipeon live progress, follow-up, user-input, approval, cancellation and recovery states without raw provider protocol;
- improve audit evidence with correlated provider events and decisions;
- preserve Codex sandbox/escalation and the user's selected native approval authority;
- retain a provider-neutral contract and CLI fallback.

### Decision status

**Prototype successful; founder migration direction accepted; implementation in progress.** CAS-01
through CAS-13 proved the constrained, package-owned adapter boundary through controlled
integration. CAS-14 makes App Server the primary/default adapter for one Pipeon consumer while
retaining governed `codex_exec` fallback and bounded workers. Canonical research:
`docs/research/codex-app-server-top-level-orchestrators-2026-07.md`.

Repository has no ADR convention; this task is the accepted product-decision record, not an ADR.

### Current state

- The protocol spike, provider-neutral contracts, supervision, lifecycle, approvals, cancellation,
  recovery, persistence, audit, security fixtures, and controlled Codex integration are complete.
- The boundary remains package-local. The first Pipeon migration seam carries and session-pins an
  explicit Codex adapter choice. New normal Pipeon Codex sessions now capture
  `codex_app_server` by default, while the explicit configured `codex_exec` escape hatch applies to
  later new sessions. Existing and migrated Pipeon sessions retain one immutable closed choice, and
  legacy sessions without explicit resource, workspace, or user evidence conservatively retain the
  historical `codex_exec` default. An explicit `codex_app_server` choice supports a first
  host-resident turn and later turns only after fresh-child model/policy revalidation plus exact
  verified-idle recovery. Its transient approval, user-input, and user-requested cancellation
  controls now render in Pipeon with exact one-time delivery. Eligible failures proven before any
  `turn/start` attempt now roll back only the exact newly claimed turn and fall back once through the
  governed buffered `codex_exec` route. Provider-pool bindings remain authoritative and unchanged.
  Retained App Server sessions now persist and render one strictly normalized post-turn neutral
  status snapshot from the existing closed provider-pool response. Sessions without a snapshot
  remain valid and gain no inferred history. A retained valid `recovery_required` snapshot with an
  exact unknown outcome now blocks another normal direct App Server prompt before messages,
  interactive monitors, MCP, provider-pool state, claims, fallback, replay, retry, or children. The
  host retains the adapter, snapshot, messages, and persistent warning unchanged; the webview only
  disables its existing Send action for ordinary Codex text from the allowlisted display projection.
  A package-private read-only recovery-candidate classifier now recognizes only the exact canonical
  App Server binding, prior strict-loader-validated verified-idle session state, and canonical
  unresolved next-turn claim for the same session. Repeated classification is byte-for-byte inert,
  and candidate status authorizes no child, provider call, reconciliation, claim removal, snapshot
  transition, new turn, or safe-continuation claim. First-turn uncertainty and every incomplete or
  mismatched evidence set remain non-candidates. A package-private recovery-only host operation now
  consumes only that exact candidate classification, strictly reloads the retained state, constructs
  one fresh supervisor with the retained provider-session ID, recovery evidence, model, and reasoning
  values, and invokes the existing `RecoverBaseline` exactly once. It has no prompt input or dispatch,
  always shuts the fresh supervisor down after the attempt, and returns only a non-authorizing
  observation or error. The binding, completed state, unresolved claim, Pipeon snapshot, messages,
  warning, guard, and Send state remain unchanged. The follow-up atomic-transition investigation
  proved that the provider-pool package's three independent binding, completed-state, and claim files
  cannot be safely coordinated with Pipeon's separately owned recovery-required workspace snapshot.
  The accepted product/storage direction now defines the truthful non-terminal post-state as
  `reconciled_outcome_unknown`: the exact retained provider thread satisfied the verified-idle
  recovery baseline, but the prior prompt outcome remains unknown and continuation, replay, retry,
  and normal Send remain unauthorized. It selects the provider-pool coordinator as the sole durable
  transaction owner of one canonical per-session aggregate, consumes the unresolved claim into that
  aggregate as permanent no-replay evidence, and makes Pipeon's retained snapshot a projection only.
  A future exact user decision bound to the aggregate would be required before a fresh later turn
  could be authorized. Maintainer acceptance on 2026-08-04 established the product/storage
  direction, and authorized Slice 1 now implements only the package-private canonical aggregate
  contract, strict bounded loader, and inert package-state path described below. They remain unused
  and unreachable from production, and no aggregate has been written. The dated pre-Slice-2
  primitive decision packet below now records the accepted all-platform research policy and the
  accepted current negative documentation result for Windows/NTFS/amd64, Linux/ext4/amd64, and
  macOS/APFS/arm64. Acceptance covers only the recorded `D`/`R`/`U` documentation findings. A
  focused Windows re-audit now documents an unprivileged NTFS rename-metadata
  sync candidate, but atomic first-publication/replacement and resulting power-loss visibility remain
  unresolved. Linux has a documentation-supported candidate sequence for later native evidence,
  while Windows and macOS retain required undocumented guarantees. The all-or-nothing
  documentation gate is therefore unmet: no storage primitive, API sequence, dependency version,
  platform allowlist, implementation, or evidence harness is accepted, prototype evidence remains
  unauthorized on every platform, and no support reduction or implementation choice is made.
  The authorized bounded transactional-store selection below now accepts the logical-one-aggregate,
  physical-per-session-SQLite direction and fixes an exact design/evidence baseline. The accepted
  dependency baseline is `modernc.org/sqlite v1.56.0`, its required
  `modernc.org/libc v1.74.4`, embedded SQLite 3.53.3, native `win32`/`unix` VFSes, rollback journal,
  exclusive connection locking, and `synchronous=EXTRA`. The closed version-skew qualification below
  proves every SQLite 3.53.4 fix irrelevant to the selected fixed schema, single-database/no-`ATTACH`
  contract, and bounded SQL surface. A separately authorized dependency-pin and test-only Windows
  smoke slice now pins those exact modules and proves the selected contract once on the current
  fixed NTFS Windows/`amd64` host with CGo disabled. A separate opt-in 10,000-cycle Windows native
  reader-publication cohort proves exact old reads, live-owner fail-closed contention, protected
  journals, and exact new reads across independent persistent child processes. The additional opt-in
  1,000-cycle Windows contention/forced-termination cohort below proves exact old-row hot-journal
  recovery and one later clean commit while an independent different-session database continues
  committing. None of these tests changes production storage source, the inert Slice 1 path/loader,
  aggregate, migration, or lifecycle behavior. The deterministic matrix now reaches the exact
  post-COMMIT-invocation/pre-result loss boundary through the pinned driver's native commit hook;
  its required genuine-commit-error row remains proven unreachable under the selected exclusive
  shape, so the complete-matrix claim remains open. The native Linux/`amd64` smoke below now
  qualifies the selected contract once on the current Pop!_OS/local-ext4 host with the exact
  platform compile-option set; broader Linux cohorts, macOS runtime evidence, reboot/power-loss
  evidence, and the production-use gate remain open. A shared cross-session
  database remains rejected because it would couple otherwise independent session writers.
  Migration, operations, storage implementation, observation,
  compare-and-commit, cutover, dispatch guards, decision controls, later-turn claiming, Pipeon
  projection, platform evidence acceptance, compatibility, and rollback remain separately gated.
  Slice 2 has not started; TASK-013 and CAS-14 remain open.
  Operations rollout, live event transport, and durable event-history rendering/retention have not
  started.
- The CAS-14 product direction is accepted. Its provider-neutral contract is complete, and the
  supervisor-only projections now cover stable model/reasoning, native approval/reviewer and
  sandbox policy, safe capability selection, provider-backed bounded user-input prompt
  normalization/lookup, exact one-time response delivery, and strict approval-request decisions.
  The normal Pipeon chat route now has the validated adapter-selection foundation plus bounded
  dispatch, idle-session continuation, the package-local approval-decision controller below, and
  one bounded concurrent MCP server chat slot across stdio and HTTP with transient exact approval
  request/decision transport, exact user-input request/response transport and Pipeon controls, the
  package-private cancellation-controller seam, the transient cancellation MCP transport, and the
  Pipeon cancellation control and immutable Pipeon session adapter cache below.
  The normal CLI and zero-value dispatcher still supply no approval, user-input, or cancellation
  source. The persisted adapter binding remains `codex_app_server` across the one-shot exec fallback;
  the first Pipeon-owned post-turn snapshot is durable, while live event streaming/cursors, full
  durable event history, operations evidence, and cross-platform acceptance remain incomplete.

### Pause / resume checkpoint — 2026-08-13

Feature implementation is intentionally paused at `js/dev` commit
`6752dce7c0540d68cb95e1f718ba0998ea0eae35` after the MCP package-ownership cleanup. CAS-14 and
TASK-013 remain open. Slice 1's package-private canonical aggregate, strict loader, and state-path
helper are implemented but inert and unused; no aggregate has been written. Slice 2 has not started.

The private-per-session SQLite transactional-store, dependency, and evidence direction recorded
below is accepted, but production storage, migration, authority cutover, recovery authority,
dispatch/projection integration, remaining platform evidence, later user-decision/claim lifecycle,
compatibility, rollback, and operations acceptance remain separately gated. Resumption requires a
fresh read of this checkpoint and current repository evidence plus explicit authorization for one
bounded remaining gate. This checkpoint authorizes no implementation, prototype, migration,
cleanup, live evidence run, or successor slice by itself.

### Scope

- Codex App Server adapter for host-resident long-running top-level sessions;
- generic session/state/event/approval contract usable by other providers;
- Pipeon as the first primary/default App Server consumer, retaining an explicit exec escape hatch;
- process supervision, persistence, audit, cancellation and recovery;
- controlled tests using installed Codex.

### Non-goals

- replacing bounded codex exec workflow workers;
- Codex-specific engine changes under src/lib or src/cmd;
- WebSocket/remote-control transport;
- DockPipe-fabricated automatic approval, silent authority expansion, inherited full access,
  thread shell-command, or raw protocol in Pipeon;
- broad production abstraction beyond the named first Pipeon consumer.

### Architectural constraints

- App Server runs on host; Codex native sandbox remains active.
- Codex decides whether escalation is needed and may review it under the user-selected native policy;
  DockPipe validates the policy, projects neutral records, and never fabricates approval.
- Host authority is never expanded silently or inferred from approval automation.
- Adapter owns provider JSON-RPC; generic contracts expose provider-neutral events only.
- Crash/disconnect is never reported as safe continued execution.
- codex_exec remains available and no active prompt is replayed into it.
- Pipeon uses the generic contract and receives first migration advantage.

### Security constraints

- stdio only initially; no external listener;
- workspace-write/declared roots by default; broader access requires conspicuous per-session choice
  and must not be inherited accidentally; thread shell-command remains rejected;
- expose only validated native reviewer modes; automatic review never means DockPipe blindly
  approves, and approval automation never implies broader sandbox access;
- approval uses process/thread/turn/item/request correlation and one-time persistence;
- no credential copying; redact sensitive raw payloads;
- unknown authority-expanding capabilities fail closed; experimental capabilities are gated
  individually rather than by a global enable-all switch;
- default deny on timeout, disconnect, stale event, schema mismatch or malformed message;
- append-only event/approval audit with gap detection and reconciliation.

### Migration and rollback

1. Complete constrained protocol spike for selected Codex version.
2. Add provider-neutral contracts and selectable codex_app_server adapter.
3. Make App Server primary/default for one Pipeon direct top-level session with an explicit
   `codex_exec` escape hatch and session-pinned policy.
4. Run contract/integration/security evidence review.
5. Migrate remaining consumers only after maintainer decision; retain codex_exec and bounded-worker behavior.

Rollback selects `codex_exec` for new sessions or administratively disables App Server. Existing App
Server sessions become Disconnected until explicitly reconciled; never replay an active turn. Retain
audit records and offer user-guided resume/fork only after recovery checks.

### Remaining implementation gates

- Implement the accepted single-consumer adapter and provider-neutral session-control surface.
- Widen the proven fixed model/human-review/workspace-write hardening into validated available
  model/reasoning plus independently selected native approval and sandbox policies.
- Prove fallback, rollback, local event retention, and user-visible reconnect/recovery behavior.
- Cross-platform controlled evidence beyond the completed Windows integration harness.
- Maintainer acceptance of implementation evidence before CAS-14 completion.

### Epic acceptance criteria

- Spike proves launch, initialization, start/resume, stream, approval+denial, interrupt, clean exit, child death, native sandbox and fail-closed recovery.
- Codex types do not leak into generic orchestration/Pipeon APIs.
- Approval cannot replay or cross-apply.
- Pipeon App Server default and governed `codex_exec` escape/fallback work together.
- Bounded worker codex exec remains unchanged.
- Schema gates, audit/security tests and operations/migration docs pass.

## Child backlog items

| ID | Type | Task | Acceptance criteria |
| --- | --- | --- | --- |
| CAS-01 | Research | App Server protocol spike | Launch existing-auth stdio; initialize; start/read/resume; stream events; record stable schema/version; no production abstraction. |
| CAS-02 | Architecture | Provider-adapter contracts | Define provider-neutral session/state/event/approval interfaces; prove Codex types do not leak into Pipeon/generic layer. |
| CAS-03 | Implementation | Process supervision | Own child/job, JSONL I/O, startup/shutdown/liveness deadlines and fail-closed Disconnected. |
| CAS-04 | Implementation | Protocol client and initialization | Correlate requests; initialize/initialized; schema/capability gate; capture version, identity and config warnings. |
| CAS-05 | Implementation | Thread and turn lifecycle | Implement start/read/resume/follow-up/steer policy, ownership records and no duplicate turn guarantee. |
| CAS-06 | Implementation | Structured event normalization | Convert thread/turn/item/error/warning/token stream to ordered safe generic events; retain restricted raw audit payload. |
| CAS-07 | Implementation | Approval relay | Persist/correlate command/file/permission/MCP/user-input requests; require user decision; test denial and replay rejection. |
| CAS-08 | Implementation | Cancellation/interruption | Implement cancel intent, turn interrupt, terminal wait, bounded kill escalation and background-process risk report. |
| CAS-09 | Implementation | Persistence/resumption | Persist policy/thread/turn/process/event cursor; reconcile through fresh server without claiming work survived. |
| CAS-10 | Implementation | Audit/observability | Add redacted RPC journal, operation-result projection, progress/latency and event-gap alert. |
| CAS-11 | Security | Hardening | Enforce stdio, no shell/full-access/auto-review, policy validation, transport isolation, redaction and MCP allow-list. |
| CAS-12 | Testing | Contract tests | Fixture-test schema, state, duplicate/reorder/malformed messages, approval replay and policy mismatch. |
| CAS-13 | Testing | Controlled Codex integration | Test existing auth, stream, approve/deny, interrupt, sandbox, clean exit and process death. |
| CAS-14 | Migration | First Pipeon migration | Make App Server primary/default for one Pipeon top-level direct session; render normalized status/approval/recovery and selected policy; retain governed exec fallback. |
| CAS-15 | Migration | Remaining top-level orchestrators | Inventory/migrate only compatible consumers after Pipeon evidence review. |
| CAS-16 | Migration | CLI fallback | Make adapter choice, safe fallback, no-replay rules and rollback telemetry explicit/tested. |
| CAS-17 | Documentation | Operations guidance | Document policy, approval, recovery, supported versions, diagnostics, Pipeon UX and rollback. |

### CAS-01 current evidence

On 2026-07-11, three workspace-sandboxed materialization probes reached a correlated `turn/completed` terminal event classified as `failed`; none sent `thread/resume`. All repeated the same terminal diagnostics: nine retriable `responseStreamDisconnected` errors, one `other` error, one warning, and terminal error kind `other`. A failure-gated, redacted `thread/read` reconciliation returned a result, proving that the App Server control plane remained responsive after the failed provider stream. Metadata-only probes on the recorded `codex-cli 0.144.1` baseline also completed `account/read`, `model/list`, `thread/start`, and `thread/read` successfully, retaining only the account-read result class, selected model/effort, catalog-verification flag, and sanitized CLI version. A credential-free TCP baseline then showed `api.openai.com:443` unreachable from the workspace sandbox but reachable from the host. One narrowly approved host-network materialization probe, with the identical pinned `gpt-5.6-terra` / `high` and workspace-write/network-disabled/user-review turn policy, reached correlated `turn/completed: completed` in 6.2 seconds and then returned `thread/resume: result`. This proves that the App Server must be host-resident so it can reach the provider, while the model turn itself remains restricted by its native sandbox policy; safe successful resume is proven in that host-resident shape. The harness validates and pins the selected model and effort on thread and turn start, and halts if a model-reroute event occurs. Its static checks assert the complete policy envelope, reroute correlation and fail-closed state, and rejection of full access, shell-command, auto-review, and empty model/effort inputs. No raw account, catalog, command, or RPC payload was retained.

### CAS-02 current decision

CAS-02 adds the unused `packages/dorkpipe/lib/providersession` contract package and its package-owned documentation. It defines provider-neutral session identity, lifecycle states, contiguous event sequencing, safe normalized events, user-input and approval requests, cancellation intent, recovery reference, and an adapter interface. `Disconnected` is fail-closed and can return to `Ready` only through verified recovery; every human approval requires the complete process-incarnation, connection, session, interaction, activity, request, and one-time decision tuple. The contract carries references and summaries only—no provider protocol unions, raw payloads, credentials, or provider-specific error types. Its source-boundary test rejects provider protocol identifiers, and focused tests cover ordering/duplicate/stale/gap rejection, approval correlation, user-input/cancellation references, and recovery transitions. CAS-03+ retain protocol lifecycle execution, event normalization, approval delivery, persistence, audit, and Pipeon wiring.

### CAS-03 current decision

CAS-03 adds the unused package-local `appserversupervisor` foundation. It directly owns one
host-resident child with private stdio (no listener, socket, shell, fallback process, credentials,
or raw-payload storage), observes JSONL framing only, and bounds startup, liveness, graceful
shutdown, and kill escalation. Startup failure, child exit, closed or malformed stdout, transport
loss, and deadline expiry each produce exactly one provider-neutral `providersession` state event:
fail-closed `Disconnected` with a safe reason class. A stopped supervisor cannot start again, so it
does not retry, resume, replay, or fall back. Native turns remain deferred: CAS-04+ must still set
the pinned `gpt-5.6-terra`/`high` and the workspace-write, declared-root, network-disabled,
user-review policy; host process placement itself grants none of those capabilities.

### CAS-04 current decision

CAS-04 extends only the package-local `appserversupervisor` with a private JSONL protocol client.
The one host-resident direct child now receives a bounded `initialize` request followed by the
`initialized` notification. Request IDs start at one, advance monotonically, and only the active
response can satisfy the request. The initialization gate requires an explicit accepted stable
schema version and required capability set; it retains only an allow-listed projection of the
provider version, `codex_app_server` identity class, bounded configuration-warning classes, and
the pinned `gpt-5.6-terra` / `high` policy configuration. It neither calls the model catalog nor
starts a thread or turn.

Malformed JSONL/envelopes, correlation mismatches, provider errors, rejected initialization,
schema/capability mismatch, model-reroute indications, request deadline expiry, transport loss,
or child exit produce the single safe `Disconnected` projection and stop the child. The protocol
client keeps frames, provider errors, IDs, prompts, account data, commands, stderr, and
credentials private and transient; none crosses into `providersession` or Pipeon. It adds no
retry, resume, replay, reconnect, fallback process, listener, socket, shell, credential access,
or persistence.

Deferred to CAS-05+: all thread/turn start/read/resume/follow-up/steer work; normalized provider
events; approvals/user input; interruption; persistence/resumption; audit; additional hardening;
Pipeon migration; and CLI fallback. Future turns must still explicitly enforce workspace-write,
declared writable roots, network disabled, and user review; host process placement grants none of
those capabilities.

### CAS-05 current decision

CAS-05 extends only the package-local `appserversupervisor` lifecycle client. After the completed
CAS-04 initialization gate it makes bounded `thread/start`, `thread/read`, `thread/resume`,
`turn/start`, and `turn/steer` requests. Provider thread and turn IDs remain private and are
projected only as opaque `providersession.SessionRef` and `Correlation` values. A single
supervisor owns one active steerable turn; duplicate/concurrent starts, stale lifecycle references,
correlation mismatches, non-steerable steering, bad response shapes/states, policy changes, and
any failure leave it safely `Disconnected`.

Every lifecycle request reconstructs the pinned `gpt-5.6-terra` / `high` native-turn envelope:
workspace-write with explicitly declared writable roots, network disabled, and untrusted policy
reviewed by the human user. Full access, shell commands, automatic review, fallback models or
providers, empty roots, and policy/model changes are rejected. The host App Server child remains
host-resident only for provider-stream reachability; that placement grants no additional turn
capabilities. No raw payload, prompt, account, command, error body, credential, retry, replay,
reconnect, fallback, or persistence is retained or introduced.

Deferred to CAS-06+: provider-event normalization and terminal turn state; approval/user-input
relay; interruption; persistence/reconciliation; audit; further hardening; Pipeon migration; and
CLI fallback.

### CAS-06 current decision

CAS-06 extends only the package-local `appserversupervisor` notification path. It accepts a closed
allow-list of correlated thread, turn, item, progress, warning, error, token-usage, and terminal
turn notifications after CAS-05's initialization/thread/turn gates. Raw JSONL frames, token text,
item content, provider messages, error bodies, command/file data, credentials, and provider IDs
remain private and transient. The supervisor projects only contiguous, supervisor-owned neutral
`providersession.Event` progress summaries and opaque `SessionRef`/`Correlation` references.

One private item may be active within the one private active turn. Duplicate, stale, reordered,
cross-thread, cross-turn, cross-item, malformed, uncorrelated, unsupported, rerouted, transport,
or child-exit notifications fail closed through the single `Disconnected` projection. A validated,
exact terminal turn notification alone releases the active-turn invariant; it does not recover,
resume, replay, retry, or cancel work. `providersession` and Pipeon remain free of App Server
protocol/raw-payload types.

Deferred explicitly to CAS-07+: approval and user-input relay (CAS-07), interruption/cancellation
delivery (CAS-08), persistence/reconciliation/recovery (CAS-09), audit journal (CAS-10), broader
hardening (CAS-11), contract/integration expansion (CAS-12/13), Pipeon migration (CAS-14/15), CLI
fallback changes (CAS-16), and operations guidance (CAS-17).

### CAS-07 current decision

CAS-07 extends only the package-local `appserversupervisor` request path. It accepts the stable
command-execution, file-change, declared-permission, and tool user-input request notifications
only when they exactly match the current private process/connection/thread/turn/item invariant.
It projects neutral approval or user-input events with supervisor-generated opaque request and
one-time decision references. Command text, patches, paths, prompt/question text, provider request
IDs, messages, credentials, raw payloads, policy amendments, network requests, and session grants
remain private and transient. Only the bounded neutral one-turn approve/deny operation is exposed:
it maps to private command/file decisions, while permission requests are deny-only because the
neutral contract carries no grant subset. An exact approve still requires the matching private
`serverRequest/resolved` notification before returning to `running`; an explicit deny now closes the
supervised turn as fail-closed `approval_denied` and cannot continue. User-input requests use their
separate bounded response operation and remain outside approval decisions.

Duplicate, stale, reordered, cross-thread, cross-turn, cross-item, uncorrelated, malformed,
unsupported, expired, transport-loss, child-exit, provider-error, and reroute conditions deny or
disconnect fail-closed. A successful private response still requires the exact matching
`serverRequest/resolved` notification before the supervisor returns to `running`; no terminal,
retry, replay, reconnect, cancellation, recovery, persistence, audit journal, or Pipeon behavior
is introduced. CAS-08+ remains explicitly deferred.

### CAS-08 current decision

CAS-08 extends only the package-local `appserversupervisor` with the existing neutral
`providersession.CancellationIntent`. It accepts only bounded neutral reason classes and requires
the exact current process, connection, session, and active-turn correlation before it emits an
opaque cancellation-intent projection and sends the private `turn/interrupt` request. An accepted
interrupt response is delivery acknowledgement only: the session stays non-terminal while it waits
for the exact correlated `turn/completed: interrupted` notification. Only that terminal
notification projects `cancelled`; a request, response, timeout, or item transition never does.

Duplicate, stale, reordered, cross-process, cross-connection, cross-thread, cross-turn,
cross-item, uncorrelated, malformed, unsupported, provider-error, reroute, transport-loss, child
exit, interrupt mismatch, non-interrupted terminal, or missing terminal notification disconnects
through the existing bounded supervisor shutdown/kill path. A private background-process indicator
can produce only the bounded neutral `background_process_risk_possible` summary; command and
process details remain transient. CAS-09 persistence, resumption, reconciliation, recovery, and
all later audit, hardening, migration, and fallback work remain explicitly deferred.

### CAS-09 current decision

CAS-09 adds only package-local, versioned, bounded idle-session snapshots and explicit recovery.
A snapshot contains safe session and policy references, opaque prior process/connection
incarnations, a contiguous event cursor, and closed lifecycle/summary classes; it contains no raw
RPC, prompt, command, patch, path, approval/input payload, credential, provider error, or process
detail. It is committed atomically only after the existing supervisor has observed the thread idle
with no active turn, pending approval/input, or cancellation.

Recovery requires the existing neutral `RecoveryRequest` with the exact session and opaque
evidence. A new supervisor starts a fresh initialized child and makes only a strictly correlated
private idle reconciliation read. It never reconnects to the prior process or continues a prior
turn. Policy mismatch, non-idle/unknown state, cursor ambiguity, corrupt or stale evidence,
response mismatch, timeout, transport/child failure, provider error, reroute, or malformed input
stays fail-closed as bounded `recovery_required` or `disconnected`; only an exact idle result emits
`ready`. CAS-10 audit/telemetry and CAS-11 hardening are now complete; CAS-12+ remain deferred.

### CAS-10 current decision

CAS-10 adds a package-local, versioned, bounded audit journal for the normalized supervisor. Its
atomically replaced bounded segments retain only neutral session/correlation references, contiguous
audit and event cursors, closed lifecycle/operation/outcome/summary classes, and coarse
progress/latency buckets. It stores neither raw timestamps nor JSON-RPC frames, provider IDs,
prompts, questions, commands, paths, patches, credentials, token text, error bodies, account/config
data, or process details. The journal is write-only evidence: it exposes no replay, retry, recovery,
approval, denial, steering, cancellation, or export capability.

Every normalized state/progress/request/cancellation/recovery/disconnect event is paired with a
contiguous audit-event cursor; operation-result evidence covers initialization, lifecycle delivery,
approval and input expiry/resolution, cancellation delivery, persistence, reconciliation, disconnect,
and shutdown outcomes. Corrupt, partial, oversized, reordered, stale, gapped, cross-session, or
unsafe evidence, and every serialization or storage failure, fail closed through bounded shutdown.
Idle persistence commits only after its audit event is durable and before it becomes observable;
recovery requires the retained audit cursor to match the safe idle snapshot and still starts a fresh
initialized supervisor. Audit therefore cannot imply that an active or unknown prior turn survived.

### CAS-11 current decision

CAS-11 hardens only the package-local `appserversupervisor`. Constructor and startup validation now
requires bounded session, deadline, initialization, direct-child launcher, and lifecycle-policy values;
the launcher accepts only the direct `codex app-server --stdio` shape. Lifecycle policy remains exactly
`gpt-5.6-terra` / `high`, workspace-write, declared in-workspace roots, network disabled, and human
review, with shell, full-access, auto-review, fallback, policy drift, and malformed identifiers rejected.
Protocol initialization, notification, server-request, MCP-progress, snapshot, and audit shapes use a
closed package-local allow-list; unknown or duplicate extension data is disconnected without retaining
the raw value. Disconnect, shutdown, and rejected recovery clear private streams and active/pending
approval, input, cancellation, and lifecycle state before bounded child cleanup. Snapshot and file-audit
serialization reject unsafe bounded data. Audit remains descriptive and cannot execute, replay, resume,
retry, approve, deny, steer, cancel, recover, or export anything.

Focused local fixtures cover unsafe launcher/configuration, policy tampering, stale and extended local
state, cross-correlation rejection, snapshot/audit serialization, redaction, and disconnect cleanup.

### CAS-12 current decision

CAS-12 is complete with deterministic local fixture-only contract coverage across the existing CAS-03
through CAS-11 boundary. The fake launcher and private in-memory stdio child exercise malformed,
duplicate, stale, out-of-order, cross-process, cross-connection, cross-session, and cross-turn inputs
for initialization, lifecycle, events, approvals/input, cancellation, recovery, and audit cursors. The
expansion asserts fail-closed projection and cleanup of private transport/client, active lifecycle, pending
approval/input, cancellation, and reuse routes. The source-boundary fixtures also reject expanded
App Server and raw-protocol vocabulary in `providersession` and Pipeon source. No executable, account,
credential, listener, network, or provider call is used.

CAS-13 controlled Codex integration is complete. CAS-14+ migration/operations work remains explicitly deferred.

### CAS-13 bounded status

On 2026-07-13, the opt-in `cas13` harness completed controlled host-resident verification against
the authenticated current App Server with exact `gpt-5.6-terra` / `high` catalog verification. A
workspace-write, declared-root, network-disabled, human-reviewed no-tool turn reached normalized
completion and clean child shutdown; a second turn reached the exact correlated interrupted
terminal after neutral cancellation. A constrained workspace-change turn produced a file-change
approval request, DockPipe denied it, and the exact resolution returned to running without making
the requested change. Controlled stdout loss projected `transport_closed`, while termination of
the direct child projected `child_exit` after a bounded owned-child exit observation. No raw frame,
provider error, prompt, command, path, credential, account value, or provider identifier was
retained; every temporary `C:\tmp` workspace was removed after its test.

The current v2 surface omits the redundant `jsonrpc` member on responses, emits object-shaped
thread statuses, and may attach a numeric `startedAtMs` field to a file-change approval request.
The App Server also uses JSON-RPC ID zero for that incoming server request. The supervisor accepts
only that timestamp's unsigned-numeric shape and preserves the zero ID privately for the matching
one-time deny/resolve exchange; client-originated request IDs remain non-zero. Current unrelated
remote-control and rate-limit snapshots are explicitly opted out. Fixture coverage retains strict
rejection of unexpected approval fields, policy expansion, malformed IDs, retries, replay, and
fallbacks.

The workspace sandbox cannot reach the provider stream, so these direct-child checks run through
a narrowly reviewed host-network path while each native turn remains constrained by the same
workspace-write policy. CAS-13 is complete. CAS-14+ remains deferred; no Pipeon migration,
adapter selection, fallback, or production consumer wiring was started.

### CAS-14 maintainer decision packet — 2026-08-03

**Status: founder product decision accepted; the provider-neutral and supervisor-only CAS-14
implementation foundations are complete, and CAS-14 is not complete.**

#### Recommendation and first consumer

**Decision: go.** App Server becomes the primary/default adapter for new normal Pipeon Codex
direct-session paths. The first and only consumer is the normal Pipeon chat-panel Codex session. The
first implementation must not include the `/codex` command, the standalone
`dorkpipe.host_codex_chat` tool, generic provider-pool callers, bounded workers, workflows, or any
other top-level orchestrator.

The live route trace is:

1. The Pipeon webview posts `ask` with its surface session id. `PipeonChatViewProvider.ask` resolves
   that id to the persisted Pipeon session and passes `session.id` to `executeDorkpipeRequest`.
2. A normal request reaches `executeNaturalLanguageRequest`, which selects the Codex provider and
   calls `executeProviderPoolChat` with the same session id. Codex receives only the new prompt;
   Pipeon does not prepend its rendered conversation history because provider session affinity owns
   the conversation.
3. `executeProviderPoolChat` calls `dorkpipe.provider_pool_chat` with `session_id`. The MCP bridge
   forwards it as `provider-pool prompt --session-id`.
4. `runProviderPoolCodexPrompt` currently maps that Pipeon session id to a discovered Codex session
   id through `statepaths.ProviderPoolSessionsPath`, then uses `codex exec resume` for later turns.

That path is the active user-facing direct chat route and already has the identity needed for a
one-to-one Pipeon-session/adapter-session binding. By contrast, `executeCodexHostChat` is called only
by the explicit `/codex` command, and that call does not supply a Pipeon session id. The existence of
`dorkpipe.host_codex_chat` or the broader provider-pool route is therefore not evidence that either
whole surface should migrate.

#### Adapter, model, approval, sandbox, and capability policy

- Pipeon owns a resource/workspace-scoped VS Code setting `pipeon.codex.sessionAdapter` in the
  extension's `package.json`. Its default is `codex_app_server`; `codex_exec` is the explicit legacy
  escape hatch. The selected adapter is pinned to the Pipeon session on its first Codex turn and is
  retained as package state. Existing `codex_exec` bindings never migrate automatically.
- A missing adapter choice from an existing non-Pipeon caller retains current `codex_exec` behavior.
  `/codex`, bounded workers, workflows, and generic callers do not inherit the Pipeon default. An
  unknown adapter value is rejected rather than guessed. Availability, authentication, catalog
  presence, connection, load, or ordering never selects an adapter implicitly.
- App Server exposes the validated currently available model and reasoning catalog to Pipeon. Users
  may select any advertised stable combination. Pipeon shows the effective model and reasoning level
  before the first turn; an unavailable, removed, unsupported, or mismatched selection fails visibly
  and never silently substitutes another model. The CAS-13 `gpt-5.6-terra` / `high` combination
  remains the proven baseline, not the only production lane.
- New sessions default to the user's native Codex approval configuration and `workspace-write`.
  Model, reasoning level, approval/reviewer mode, and sandbox mode are visible and user-selectable per
  session. Pipeon may expose Codex's native automatic-review modes; DockPipe passes the chosen native
  policy through, validates it, and audits its neutral outcomes. DockPipe never implements automatic
  review by blindly approving requests itself.
- Approval automation and sandbox authority are independent controls. Broader-than-workspace access
  requires a separate conspicuous per-session confirmation, is never inferred from automatic review,
  and is not inherited accidentally by a new session. Model/reasoning preferences may be remembered;
  authority-expanding approval or sandbox choices may not be silently carried forward.
- If the validated stable catalog advertises a full-access sandbox mode, Pipeon may expose it only as
  that explicit per-session choice. Full access never enables thread shell-command, bypasses policy
  validation, or becomes a default merely because native automatic review is selected.
- Advertised stable capabilities may become supported after schema, provider-neutral projection, and
  policy validation. Support does not mean automatic enablement: any capability that expands access,
  execution, approvals, credentials, or transport requires an explicit DockPipe policy mapping and
  user choice. Unknown capabilities fail closed. Experimental capabilities are gated individually
  behind clearly labeled advanced settings; there is no global "enable all experiments" switch.
- `codex_exec` remains the governed legacy adapter, the implementation for bounded workers, and the
  only automatic fallback before an App Server turn begins. This decision does not authorize a
  provider-pool, workflow, CLI, or engine-wide migration.

#### Turn boundary, fallback, and rollback

For fallback purposes, a turn is potentially active as soon as the private App Server `turn/start`
request is sent. Every submitted prompt must be treated as potentially mutating after that point;
text classification is not a safety control.

- Automatic fallback to `codex_exec` is permitted only before `turn/start` is sent, such as a
  rejected version/schema/policy gate or a child that cannot initialize. The bridge records a safe
  pre-turn fallback reason and may submit the prompt once through `codex_exec`.
- Once `turn/start` has been sent, transport loss, child exit, timeout, event gap, malformed input,
  or uncertain terminal state produces `disconnected` / `recovery_required`. The active or possibly
  active prompt is never replayed to App Server or `codex_exec`, and Pipeon must not report it as
  completed, failed safely, or continuing.
- After disconnection, adapter fallback is available only after explicit reconciliation proves the
  prior thread idle and closes every pending approval, input, cancellation, and event-gap question.
  Even then the disconnected prompt is not replayed. A later user-authored prompt may use an
  explicitly selected adapter; a potentially mutating prompt with an unknown outcome is never
  resubmitted automatically.
- Selecting the `codex_exec` escape hatch affects new sessions immediately. Existing `codex_exec`
  sessions are unchanged. An existing App Server session does not change adapters in place and
  receives no new turn after App Server is administratively disabled. If it is running, waiting, or
  already uncertain, it becomes visibly
  `disconnected` / `recovery_required`; pending decisions are denied or expired fail-closed and its
  audit evidence is retained.
- A verified-idle existing App Server session may be continued only after App Server is available
  and selected for that session, or the user may explicitly fork a fresh Pipeon session onto
  `codex_exec`.
  The fork is presented as a new context, not a continuation. The same Pipeon session id is never
  rebound across adapters, and rollback never imports, scrapes, or replays a prompt or transcript.

- Pipeon may automatically start fresh-child reconciliation after a disconnect. It may return to
  `ready` without another prompt only when reconciliation proves the prior thread idle with no
  pending decision or event gap. Any active, unknown, or conflicting outcome remains visibly
  recovery-required and waits for user direction.

#### Provider-neutral Pipeon rendering contract

Pipeon renders only validated `providersession` records and safe bridge metadata. The active adapter,
model, reasoning level, approval mode, and sandbox mode remain visible throughout the session.
`starting` is a local request phase before the first neutral state event; it is not a new provider
state and never implies readiness.

| Neutral condition | Pipeon rendering and allowed controls |
| --- | --- |
| starting | “Starting Codex session”; no completion claim and no decision controls. |
| `ready` | “Ready”; initialized, policy-verified, and known idle. The composer may submit one turn. |
| `running` | “Running”; show bounded progress summaries and cancellation only. |
| `waiting_for_approval` | “Waiting for approval”; render the bounded DockPipe approval record and one-turn controls bound to its complete one-time correlation. Native automatic review may resolve through the selected Codex policy, but Pipeon never fabricates an approval. |
| `waiting_for_user_input` | “Waiting for your input”; render the bounded DockPipe prompt record and its bounded answer controls. No raw App Server question or option object reaches Pipeon. |
| `completed` | “Completed”; only an exact correlated terminal completion permits this label and final text. |
| `cancelled` | “Interrupted”; only the exact correlated interrupted terminal or the already-audited bounded termination path permits this label. |
| `failed` | “Failed”; show a closed safe reason class and no retry claim. |
| `disconnected` or `recovery_required` | “Disconnected — recovery required”; state that the work outcome is unknown, disable approval/input/completion controls, and offer only reconcile, explicit fork, or dismiss. |

The current contract can project a user-input request but deliberately has no user-input answer
operation, and its opaque `PromptRef` alone is not renderable. CAS-14 implementation therefore
requires one bounded provider-neutral prompt-record lookup and one exact-correlation, one-time
user-input response operation. Those operations belong in `providersession` and the adapter; they
must not encode App Server question unions, raw payloads, or session-wide decisions. Pipeon sends
approval and input responses back through the DockPipe bridge only. It never holds a provider RPC id
and never reads or writes App Server JSON-RPC.

#### Retention, redaction, diagnostics, and recovery

- Durable session, recovery, event-cursor, approval/input, and adapter-choice evidence stays in
  DorkPipe package state. Pipeon persists its ordinary chat messages and bounded display state only;
  it does not persist raw provider events or audit payloads.
- Retain the CAS-10 audit bounds unless a later operations decision explicitly changes them: at
  most three segments of 64 records, a 32 KiB journal, and 1 KiB per safe record. Rotation retains
  the prior event cursor so a gap remains detectable.
- Prompts, answers, question text, commands, paths, patches, token text, raw timestamps, raw RPC,
  provider errors, account/config data, credentials, and process details are not diagnostic fields.
  The UI may show only adapter requested/selected, neutral state, contract/schema and safe provider
  version class, effective model/reasoning/approval/sandbox policy, policy fingerprint/class, opaque
  session reference, last contiguous event cursor, bounded latency/progress buckets,
  fallback-before-turn reason, and disconnect/recovery reason.
- A disconnect banner must say that the outcome is unknown and remain present across Pipeon reload.
  Reconcile starts a fresh child and proves an idle thread before returning to `ready`; failure stays
  disconnected. Recovery never claims that active work survived and never enables a stale approval
  or input control.

#### Exact future implementation boundary

The first CAS-14 implementation is limited to these existing repository areas and their focused
tests:

| Area | Allowed CAS-14 responsibility |
| --- | --- |
| `packages/pipeon/resolvers/pipeon/vscode-extension/package.json` and `src/extension.ts` | Default App Server selection, session-pinned adapter/model/reasoning/approval/sandbox controls, individually gated experimental capabilities, neutral rendering, approval/input/cancel/recovery controls, and persistent recovery banner. |
| `packages/pipeon/resolvers/pipeon/vscode-extension/scripts/webview-smoke.js` | Primary/default selection, explicit exec escape hatch, visible effective policy, and provider-neutral rendering/control smoke coverage. |
| `packages/dorkpipe-mcp/mcpbridge/catalog.go`, `server.go`, and `tier.go` | Closed adapter request plus provider-neutral session event/decision/input/cancel/recover operations; host-resident supervisor ownership keyed by workspace and Pipeon session; existing exec tier/path enforcement retained. |
| `packages/dorkpipe-mcp/mcpbridge/server_test.go`, `tier_test.go`, and `codex_session_test.go` | Tool schema/tier, session isolation, adapter pinning, safe fallback, stale-decision rejection, redaction, and rollback tests. |
| `packages/dorkpipe/lib/providersession/contract.go` and `contract_test.go` | Opaque stable model/reasoning catalog, exact selected/effective policy and capability records, bounded prompt record, and one-time user-input response contract only; no adapter selection or provider protocol types. |
| `packages/dorkpipe/lib/appserversupervisor/model_policy.go`, `protocol.go`, `lifecycle.go`, `approval.go`, `hardening.go`, `supervisor.go`, and `recovery.go` | Validate available model/reasoning and stable capability catalogs, resolve an opaque turn input, map user-selected native approval/sandbox policy, answer bounded user input, expose neutral events, and retain fail-closed lifecycle/recovery rules. |
| Existing focused tests in `packages/dorkpipe/lib/appserversupervisor` | Extend lifecycle, approval, contract, recovery, hardening, and source-boundary fixtures for the consumer seam. |

Closed implementation-slice history from the provider-neutral foundation through the
atomic-transition investigation is retained verbatim in the
[TASK-013 closed history and evidence archive](./codex-app-server-adapter-closed-history.md#closed-cas-14-implementation-slice-history).

#### Proposed post-reconciliation product/storage decision — 2026-08-04

**Status: accepted as product/storage direction on 2026-08-04.** Acceptance establishes
product/storage direction only. Implementation, migration, schemas, controls, and cross-platform
atomicity evidence remain separately gated. This subsection records design only; it adds no
implementation or authority.

**Evidence and current ownership.** Provider-pool currently owns three canonical package-state
records: the immutable Pipeon-session-to-`codex_app_server` binding, the last completed App Server
session state, and the consecutive unresolved turn claim. Pipeon separately owns its retained adapter
and `recovery_required` / `outcomeUnknown: true` snapshot inside VS Code workspace chat state. The
existing recovery-only observation proves only that the exact retained provider thread satisfied the
verified-idle `RecoverBaseline` contract during that attempt. It does not prove what happened to the
unknown prompt. The rejected separate-write-ordering fixture demonstrates that neither current owner
can safely remove its guard evidence before the other.

**Proposed truthful meaning.** Use `reconciled_outcome_unknown` as a working name for one non-terminal,
non-authorizing state. It means only that the exact retained provider thread identified by the bound
pre-state was reconciled to the existing verified-idle recovery baseline. The prior prompt may have
completed, failed, been cancelled, produced content, caused side effects, or never been observed; this
state makes none of those claims. It is not completed, failed, cancelled, dismissed, replayed, retried,
or recovered content. It authorizes no prompt, replay, retry, fallback, normal Send, claim reuse, or
new child.

Continued chat could become eligible only after a separate explicit user decision equivalent to
"acknowledge the unknown prior outcome and continue with a fresh later turn." That decision must be
bound to the exact aggregate revision and reconciliation fingerprint, must never authorize replay of
the unknown turn, and must be consumed at most once when a strictly later fresh turn is claimed. Until
that later decision is durably accepted, both host dispatch and the Pipeon Send projection remain
guarded. Closing or abandoning the session needs no continuation authority.

**Selected owner and authority.** The proposed sole durable transaction owner is the provider-pool
coordinator, using one canonical per-session App Server lifecycle aggregate under provider-pool
package state. This follows the existing ownership of adapter pinning, App Server session state,
recovery evidence, and turn claims. The aggregate, not a companion marker, becomes the authority for
the binding, completed state, unknown turn, recovery guard, observation consumption, and future user
decision. Mere aggregate-file existence supplies no authority: the complete canonical schema,
identity, revision, state, and fingerprints must validate or the session remains blocked.

Pipeon's retained App Server snapshot becomes projection-only. After migration it is rendered from
the authoritative aggregate and may be cached in workspace state for display, but stale, missing,
cleared, malformed, or early/late projection writes can neither authorize nor prevent an otherwise
authoritative transition. Before prompt dispatch, the Pipeon host must query/validate the aggregate's
guard state; absence or failure is closed. Pipeon messages and other chat presentation data remain
Pipeon-owned and do not enter the lifecycle aggregate.

The future aggregate must carry these semantics, independent of production syntax:

- aggregate schema/version and monotonic revision;
- exact Pipeon session ID and exact `codex_app_server` adapter binding;
- exact provider-session ID and recovery-evidence reference;
- retained model and reasoning values;
- last known completed turn, the consecutive unknown pending turn, and a turn high-water mark that
  prevents that pending turn number from ever being reused;
- non-terminal unknown-outcome state, including `outcome unknown = true` and
  `reconciled to verified idle = true` without a terminal outcome;
- an exact recovery-observation fingerprint bound to the exact pre-state fingerprint and the one
  successful recovery attempt;
- unresolved-claim consumption state and permanent replay-forbidden state;
- guarded explicit-user-decision state: initially required, later capable of recording one exact
  accepted decision bound to the current revision/fingerprint, and finally consumed by at most one
  strictly later fresh-turn claim.

**Future compare and commit semantics.** One provider-pool operation must hold a verified
cross-process, OS-backed exclusive lock for the session across observation, comparison, and commit.
A process-local mutex is insufficient because Pipeon, CLI/MCP hosts, restarts, and concurrent
provider-pool processes do not share memory and can all observe durable state.

The expected pre-state fingerprint must be computed from an unambiguous length-prefixed encoding of
the exact bytes (and their named paths/roles) of the current adapter binding, completed session state,
unresolved claim, and canonical Pipeon retained-adapter/unknown-outcome guard, plus the session,
provider-session, recovery-evidence, model, reasoning, completed-turn, and pending-turn identities.
The recovery observation must be minted and used only inside that locked owner operation and bind the
same fingerprint. A display projection, status label, bare lock file, or caller-supplied digest without
the exact source evidence is insufficient.

Immediately before commit, the owner must reread and byte-compare all authoritative source evidence,
reject a stale or substituted observation, verify the aggregate revision or legacy-absence condition,
and reject any semantic or byte mismatch without mutation. The sole commit point is a verified
same-directory, same-volume atomic replacement of the complete canonical aggregate. The owner must
fully write and synchronize the temporary file before replacement, then synchronize the aggregate
and parent directory before reporting success. A platform without verified atomic replacement and
required synchronization must fail closed rather than emulate the transaction with ordered writes.

Failure before or during comparison leaves the old state authoritative. Failure after comparison but
before replacement leaves only an ignorable temporary file and the old state authoritative. The
atomic replacement makes the new aggregate authoritative in one step. Failure after replacement but
before durable acknowledgement is an unknown commit result: no retry or replay is allowed; restart
must reload and validate the aggregate to determine which full revision is visible. Temporary files,
locks, or acknowledgements never override the valid aggregate.

On restart, readers accept only one complete valid aggregate revision and derive the guard from it.
Two concurrent operations using the same observation/pre-state serialize on the cross-process lock;
after the first commit, the second sees a changed revision and consumed observation and is rejected.
The exact observation is likewise rejected after restart. A new reconciliation attempt requires a
new observation over the still-current exact pre-state; it cannot reuse an earlier observation or
the unknown pending turn.

**Migration and mutation.** Initial migration must occur under the same owner lock. With no aggregate
present, the owner must strictly read the three current provider-pool records and the exact canonical
Pipeon retained-adapter/guard slice, bind them into the expected pre-state fingerprint, perform the
recovery observation, reread the sources, and atomically create the first authoritative aggregate.
The future Pipeon integration must make that exact retained slice available to the transaction owner;
display-only state is not migration evidence. The first aggregate commit is the authority cutover.
All legacy readers and writers must then reject or defer to the aggregate. The old provider-pool files
and Pipeon snapshot may remain frozen for audit/compatibility until a later cleanup, but are ignored
for authorization and are not deleted as part of this transition.

The transition preserves the adapter, provider session, recovery evidence, model, reasoning, and last
known completed turn. It consumes the exact unresolved claim into the authoritative guarded recovery
record, records its pending turn as the no-replay high-water mark, replaces the separate Pipeon guard's
authority with `reconciled_outcome_unknown`, records the bound successful observation as consumed, and
sets explicit-user-decision state to required. It does not delete the claim merely because idle was
observed; classify the prompt outcome; clear or authorize Send by projection; mutate messages or
content; start a prompt, provider, exec, fallback, retry, supervisor, or child; or alter the retained
provider thread.

**Rejected alternatives.** Each of these remains unsafe:

- Removing the claim and then clearing Pipeon strands the guard after destroying candidate evidence
  if the second write fails.
- Clearing Pipeon and then removing the claim removes the host/Send guard while the unresolved claim
  survives if the second write fails.
- Mapping the snapshot to `completed`, `failed`, or `cancelled` invents a terminal prompt outcome.
- Keeping multiple authoritative files behind a process-local mutex does not serialize other
  processes or survive restart and therefore preserves the partial-commit problem.
- Adding a transaction or tombstone file whose mere existence becomes authority cannot prove its
  contents bind the exact old records, observation, or committed post-state. Only the complete valid
  aggregate and atomic replacement may carry authority.
- Treating successful `RecoverBaseline` as a prompt result confuses verified idle with terminal
  outcome or recovered content.
- Allowing Pipeon display state to override provider-pool lifecycle evidence lets stale, missing, or
  reordered UI persistence authorize continuation.
- Reusing an exact observation after commit, restart, or in another process can apply the same
  transition twice or revive stale evidence; the aggregate must persist consumption and reject it.

**Maintainer decision:** accepted on 2026-08-04 as product/storage direction only. This acceptance
authorizes only a separately bounded implementation-planning slice, not implementation. Classifier,
recovery-only operation, current records, claims, bindings, provider-pool responses, fallback,
adapter pinning, Pipeon guards and controls, rollback, retry, and no-replay behavior remain unchanged.
TASK-013 and CAS-14 remain open.

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

##### Pre-Slice-2 storage-primitive decision packet — 2026-08-04

**Decision status and boundary.** The maintainer research policy below and the matrix's current
negative documentation result are accepted on 2026-08-04. Acceptance covers only the recorded
`D`/`R`/`U` documentation findings. No storage primitive, API sequence, dependency version, platform
allowlist, implementation, or evidence harness is accepted or authorized. This packet adds
documentation only: no prototype, storage code, lock, temporary file, aggregate, process helper,
generated artifact, or production call site was created. Slice 2 has not started; TASK-013 and
CAS-14 remain open. Provider-pool remains the sole future transaction owner, and
`reconciled_outcome_unknown` remains non-terminal and non-authorizing. The unknown pending turn is
permanently replay-forbidden. Package/engine and provider-neutral boundaries are unchanged.

**Accepted storage-research policy.** These decisions govern qualification and are not primitive
acceptance or implementation authority:

1. No production Slice 2 implementation may begin until Windows, Linux, and macOS all qualify.
2. The complete documentation matrix for all three platforms must be reviewed before any prototype.
3. Published vendor/system documentation is normative; official source may only corroborate it.
4. Any undocumented required guarantee blocks the feature; the contract is neither weakened nor emulated.
5. Initial tuples are Windows/local fixed-disk NTFS/`amd64`, Linux/local fixed-disk ext4/`amd64`,
   and macOS/local fixed-disk APFS/`arm64`.
6. Minimum versions derive from documented primitives and later native evidence. Older hosts may run
   DockPipe, but aggregate cutover must fail closed there.
7. Later implementation may use only the Go standard library plus the newest compatible, reviewed,
   exactly pinned `golang.org/x/sys`; no CGO or portability wrapper is authorized.
8. Each session has one deterministic persistent empty lock file. It is immutable,
   non-authoritative, and never substitutes for the validated live OS-held lock.
9. A lock is never deleted, broken, or replaced as stale.
10. Acquisition uses native nonblocking attempts for at most 30 seconds; caller cancellation or a
    shorter context may stop it, but no caller may extend the cap.
11. Every symlink, junction/reparse point, bind mount, nested mount, and cross-volume component in
    the complete transaction path must be rejected.
12. Authority storage is private: Unix directories `0700` and files `0600`; Windows DACL access is
    limited to the current user and `SYSTEM`; broader write access fails closed.
13. Runtime support uses a versioned package-owned evidence allowlist for OS, architecture,
    filesystem version, and relevant mount/volume properties. Local configuration cannot override it.
14. Commit success requires complete temp write, temp sync, atomic no-replace create or replacement,
    visible aggregate reopen and identity verification, visible-file sync, parent-directory entry
    sync, and exact canonical reload.
15. Publication errors use a conservative documentation-plus-evidence allowlist. Once publication is
    invoked, any outcome not positively proven unchanged is `unknown_commit_result`.
16. Restart classifies an unknown result read-only: exact new revision is committed, exact old is not
    committed, and missing/malformed/substituted/unexpected state is blocked. It never retries,
    re-observes, repairs, or mutates.
17. Missing or malformed authoritative aggregates block with read-only diagnostics only; there is no
    reconstruction, deletion, rollback, legacy fallback, or automatic repair.
18. Stale temporary files remain non-authoritative and untouched; cleanup is a later decision.
19. Aggregate cutover creates a one-way minimum-version boundary. Old binaries cannot operate on
    migrated sessions, and downgrade never restores legacy authority.
20. Future prototype/evidence code must be package-owned, platform-specific, build-tagged,
    test-only, and unreachable from production.
21. Evidence uses deterministic synthetic aggregates only.
22. Git retains only bounded summaries, environment tuples, counts, results, and hashes; raw logs,
    VM images, crash dumps, and generated artifacts remain outside Git.
23. Every accepted tuple requires every deterministic failure hook, 10,000 publication/reader race
    cycles, 1,000 lock/forced-termination cycles, and three controlled VM hard-reboot or power-loss
    trials at every durability boundary.
24. Final acceptance is all-or-nothing: every selected tuple must pass every property before Slice 2
    production implementation can be authorized.

**Go surface and version policy.** The standard library is insufficient: [`os.Rename`](https://pkg.go.dev/os#Rename)
explicitly disclaims atomic rename on non-Unix platforms, while [`File.Sync`](https://pkg.go.dev/os#File.Sync)
does not provide locking, no-replace publication, parent-entry sync, containment, identity, or
filesystem qualification. The checkout currently pins `golang.org/x/sys v0.28.0` indirectly. Its
[`windows`](https://pkg.go.dev/golang.org/x/sys@v0.28.0/windows) and
[`unix`](https://pkg.go.dev/golang.org/x/sys@v0.28.0/unix) packages expose the syscall entry points
listed below, but exposure supplies no semantic guarantee. No exact future version is selected here;
after documentation and evidence gates pass, the newest compatible release must be reviewed and
made an exact direct requirement. Platform-specific package code is still required for handles,
flags, retries, identity, ACL/mode checks, publication classification, and runtime allowlisting.

**Candidate platform profiles and normative documentation.** These are research results, not
implementation approval. Status words below mean documented and evidence-eligible, unsupported by
an explicit published limitation, or unresolved after the listed primary surfaces were exhausted.

- **Windows / local fixed-disk NTFS / `amd64`: not documentation-qualified.** Retain every directory
  handle opened by [`CreateFileW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew)
  with `FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT`; reject every
  `FILE_ATTRIBUTE_REPARSE_POINT`; deny `FILE_SHARE_DELETE`; and compare `FILE_ID_INFO` volume/file
  identity. Microsoft documents that file ID plus volume serial identifies an open file on one
  computer in [`FILE_ID_INFO`](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_info),
  while [`GetVolumeInformationByHandleW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumeinformationbyhandlew),
  [`GetDriveTypeW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdrivetypew),
  and [`FSCTL_GET_NTFS_VOLUME_DATA`](https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ni-winioctl-fsctl_get_ntfs_volume_data)
  expose NTFS, fixed-media, and NTFS major/minor-version facts. The private DACL must contain only the
  current-user and `SYSTEM` allows; Microsoft documents DACL access decisions and implicit denial in
  [DACLs and ACEs](https://learn.microsoft.com/en-us/windows/win32/secauthz/dacls-and-aces).
  Open the empty lock with `OPEN_ALWAYS`, non-inheritable handle, no delete sharing; use
  `LockFileEx(LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY)` over `[0,1)` and bounded polling.
  [`LockFileEx`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
  documents exclusive byte-range exclusion, including beyond EOF, but mapped access ignores it;
  [`UnlockFileEx`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex)
  and [process termination](https://learn.microsoft.com/en-us/windows/win32/procthread/terminating-a-process)
  document handle/lock release.

  Create the sibling temp with `CREATE_NEW | FILE_FLAG_WRITE_THROUGH`, write all bytes, and call
  [`FlushFileBuffers`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers).
  Use that same write-through source handle for
  [`SetFileInformationByHandle(FileRenameInfoEx)`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle):
  flags `0` for first publication and `FILE_RENAME_FLAG_REPLACE_IF_EXISTS |
  FILE_RENAME_FLAG_POSIX_SEMANTICS` for replacement. [`CreateFileW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew#caching-behavior)
  explicitly says a write-through request causes NTFS to flush metadata changes such as a rename,
  and [File Caching](https://learn.microsoft.com/en-us/windows/win32/fileio/file-caching) says a file
  flush or `FILE_FLAG_WRITE_THROUGH` stores file-system metadata changes to disk. Together with the
  documented ordinary file/directory access requirements for rename, this qualifies property 9
  without a directory or administrative volume flush. A retained directory handle remains an
  identity/containment anchor only; neither `FlushFileBuffers` nor the driver flush documentation
  promises parent-entry durability for a directory handle. The documented volume-wide flush requires
  administrative privileges and remains rejected.

  [`FileRenameInformationEx`](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/4217551b-d2c0-42cb-9dc1-69a716cf6d0c)
  requires flags `0` to fail if the target exists and says POSIX replacement leaves old handles valid
  while subsequent opens of the target name open the renamed file. The WDK
  [`FILE_RENAME_INFORMATION`](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information)
  contract also requires same-volume rename and describes same-directory naming. The normative
  [`FileRenameInformation`](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/87f86c9b-6c2a-4803-84b7-131a74a434fa)
  algorithm removes and adds directory links, but none of these sources states that first publication
  or replacement is atomic to concurrent readers, excludes a transient missing target, or guarantees
  one complete old-or-new revision for an open racing the operation. They also do not define a local
  NTFS post-failure state for every error. Therefore properties 5 and 6 remain unresolved, and every
  non-allowlisted outcome after publication is invoked remains `unknown_commit_result`.

  The WDK [`FILE_INFORMATION_CLASS`](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/ne-wdm-_file_information_class)
  page documents `FileRenameInformationEx` from Windows 10 version 1709. The public Win32 enum lists
  `FileRenameInfoEx` without a per-value minimum and `SetFileInformationByHandle` warns that information
  classes can vary by OS release. Candidate minimum therefore remains Windows 10 version 1709, not an
  earlier inferred floor. Microsoft exposes NTFS major/minor values but does not bind these rename and
  durability guarantees to an exact NTFS format version; no Windows/NTFS version tuple is accepted.
  `ReplaceFileW` remains rejected because `REPLACEFILE_WRITE_THROUGH` is unsupported and documented
  failures can remove or move names.
- **Linux / local fixed-disk ext4 / `amd64`: documentation-qualified for later native evidence only.**
  Retain one aggregate-directory fd and use
  [`openat2`](https://man7.org/linux/man-pages/man2/openat2.2.html) with
  `RESOLVE_BENEATH | RESOLVE_NO_SYMLINKS | RESOLVE_NO_MAGICLINKS | RESOLVE_NO_XDEV`; the last flag
  explicitly rejects every mount crossing, including bind mounts. Open the persistent lock with
  `O_RDWR | O_CREAT | O_CLOEXEC | O_NOFOLLOW`, `0600`, and poll
  [`flock(LOCK_EX | LOCK_NB)`](https://man7.org/linux/man-pages/man2/flock.2.html); it is advisory,
  scoped to the open file description, and released only by `LOCK_UN` or last close. Create a sibling
  temp with `O_WRONLY | O_CREAT | O_EXCL | O_CLOEXEC | O_NOFOLLOW`, `0600`; the
  [`open(2)`](https://man7.org/linux/man-pages/man2/open.2.html) contract documents exclusive create
  and final-component no-follow behavior. Write exact bytes, `fsync` the temp, publish with
  [`renameat2`](https://man7.org/linux/man-pages/man2/renameat2.2.html) using `RENAME_NOREPLACE` for
  revision one and flags `0` thereafter, reopen/verify/sync the visible inode, then `fsync` the parent
  dirfd. Linux documents atomic replacement, no-replace (ext4 since Linux 3.15), same-mount limits,
  target preservation on replacement failure, and explicitly states in
  [`fsync(2)`](https://man7.org/linux/man-pages/man2/fsync.2.html) that file sync excludes the directory
  entry and a directory `fsync` is also required; it also states successful sync survives crash or
  reboot and flushes a present disk cache. `statx(STATX_MNT_ID)` (since Linux 5.8), `fstatfs`, and
  `/proc/self/mountinfo` bind exact file, mount, ext4 type, and allowlisted read-write/journal/barrier
  properties; see [`statx(2)`](https://man7.org/linux/man-pages/man2/statx.2.html), the kernel
  [`mountinfo`](https://www.kernel.org/doc/html/latest/filesystems/proc.html#proc-pid-mountinfo-information-about-mounts)
  format, and ext4 [journal](https://www.kernel.org/doc/html/latest/filesystems/ext4/journal.html)
  documentation. Candidate minimum is Linux 5.8 because mount identity is required; the exact kernel
  build, ext4 feature set, and explicit mount-property allowlist still require native evidence and
  maintainer acceptance.
- **macOS / local fixed-disk APFS / `arm64`: not documentation-qualified.** Retain a descriptor walk
  using `openat` with `O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC`; use `fstat`/`fstatfs` to require APFS,
  `MNT_LOCAL`, one retained filesystem ID, and accepted mount flags. Apple documents final-component
  no-follow and exclusive create in
  [`open(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/open.2.html),
  advisory whole-file exclusion in
  [`flock(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/flock.2.html),
  and filesystem ID, type, mount-point, mounted-from, and flag fields in
  [`statfs(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/statfs.2.html).
  Those surfaces support properties 1-4 only; `statfs` names `f_fsid` as a filesystem ID but does not
  make the complete descriptor walk a race-free containment or same-volume proof.

  Open the persistent lock with `O_RDWR | O_CREAT | O_CLOEXEC | O_NOFOLLOW`, `0600`, and poll
  `flock(LOCK_EX | LOCK_NB)`. Create the sibling temp with `O_EXCL` and write exact bytes. The APFS
  [Tools and APIs guide](https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/ToolsandAPIs/ToolsandAPIs.html)
  labels `renamex_np` and `renameatx_np` as safe-save APIs but publishes only their prototypes.
  Foundation's
  [`volumeSupportsExclusiveRenaming`](https://developer.apple.com/documentation/foundation/urlresourcevalues/volumesupportsexclusiverenaming)
  says that a true value means support for `RENAME_EXCL` on path-based `renamex_np` and describes a
  pre-existing-destination warning. General
  [`rename(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/rename.2.html)
  requires one filesystem and says an instance of the new name always exists even across a crash;
  [About Apple File System](https://developer.apple.com/documentation/foundation/about-apple-file-system)
  also advertises atomic safe-save as an APFS feature. None of those contracts defines
  `renameatx_np` flags or errors, binds `RENAME_EXCL` to the descriptor-relative call, establishes an
  atomic no-overwrite first publication, or establishes flags-`0` replacement in which racing readers
  observe exactly one complete old or new revision without a transient missing target. Properties 5
  and 6 therefore remain unresolved.

  Request `fcntl(F_FULLFSYNC)` before publication and after reopening the visible file only as an
  unresolved candidate. Apple's
  [`fsync(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fsync.2.html)
  says ordinary `fsync` permits data loss and write reordering after power loss or an OS crash. Its
  archived
  [`fcntl(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fcntl.2.html)
  says `F_FULLFSYNC` asks the drive to flush buffered data but lists only HFS, FAT, and UDF as
  implemented filesystems. Current Apple
  [disk-write guidance](https://developer.apple.com/documentation/xcode/reducing-disk-writes) calls
  `F_FULLFSYNC` a strong expectation and an iOS best-effort guarantee that can still lose data on
  sudden power loss; it does not publish an APFS/macOS persistence contract. No unprivileged
  parent-directory entry-sync primitive is documented. APFS copy-on-write crash-protection statements
  in Apple's
  [FAQ](https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/FAQ/FAQ.html)
  do not bind this exact file/rename/directory sequence. Properties 7-10 remain unresolved.

  Foundation publishes
  [`isMountTrigger`](https://developer.apple.com/documentation/foundation/urlresourcevalues/ismounttrigger)
  and
  [`volumeIdentifier`](https://developer.apple.com/documentation/foundation/urlresourcevalues/volumeidentifier)
  detection values, but neither those values nor `O_NOFOLLOW`/`fstatfs` rejects every symlink, mount
  substitution, same-filesystem nested mount, and cross-volume movement throughout the retained walk;
  property 12 remains unresolved. The
  [Apple File System Reference](https://developer.apple.com/support/downloads/Apple-File-System-Reference.pdf)
  documents APFS version 2 as implemented in macOS 10.13, backward-incompatible feature flags, and a
  reserved `nx_newest_mounted_version` field recording the newest Apple software to mount a container.
  Apple also documents APFS support from macOS 10.13 in the
  [Disk Utility guide](https://support.apple.com/en-ca/guide/disk-utility/dsku19ed921c/mac), while
  [Apple-silicon porting guidance](https://developer.apple.com/documentation/apple-silicon/porting-your-macos-apps-to-apple-silicon)
  documents native macOS `arm64` in the macOS 11 porting context. These are format and host
  facts, not an unprivileged mounted-volume API or compatibility contract mapping an exact APFS
  feature set, macOS build, and hardware to the required rename, synchronization, containment, and
  power-loss guarantees. Property 13 and an exact allowlist floor remain unresolved.

**Compact 13-property matrix.** `D` means the published docs support the candidate and it is eligible
for later native evidence; `R` means unresolved after the primary surfaces listed above were
exhausted; `U` means a published limitation makes that alternative unsupported. `D` is not executed
proof or primitive acceptance. Locks remain advisory on Unix and mandatory only for Windows byte
I/O (not mapped access); lock bytes/existence never grant authority. Restart means a fresh process,
while durability also requires the separately controlled hard-reboot/power-loss lane.

| # / required property | Windows / NTFS / `amd64` | Linux / ext4 / `amd64` | macOS / APFS / `arm64` | Documentation status |
| --- | --- | --- | --- | --- |
| 1. Session-scoped cross-process exclusive locking | `CreateFileW` + `LockFileEx(EXCLUSIVE|FAIL_IMMEDIATELY)`, byte `[0,1)` | retained inode + advisory `flock(EX|NB)` | retained inode + advisory `flock(EX|NB)` | W:D; L:D; M:D |
| 2. Release after normal exit, crash, termination | exact `UnlockFileEx`/handle close; termination closes kernel handles | `LOCK_UN` or last close; `O_CLOEXEC` prevents exec leak | `LOCK_UN` or last close; `O_CLOEXEC` prevents exec leak | W:D; L:D; M:D |
| 3. Same-session contention; different-session independence | one validated digest-named file per session | one validated digest-named inode per session | one validated digest-named inode per session | W:D; L:D; M:D; mapping still needs process evidence |
| 4. Same-directory exclusive temp creation | sibling `CreateFileW(CREATE_NEW)` | dirfd-relative `openat2(O_CREAT|O_EXCL|O_NOFOLLOW)` | dirfd-relative `openat(O_CREAT|O_EXCL|O_NOFOLLOW)` | W:D; L:D; M:D |
| 5. Atomic first publication without overwrite | `FileRenameInfoEx`, flags `0`; target-exists failure is documented, concurrent complete-file visibility is not | `renameat2(RENAME_NOREPLACE)`; ext4 support since Linux 3.15 | path-based `RENAME_EXCL` capability documents a pre-existing-target warning, but `renameatx_np` publishes only a prototype and no descriptor-relative flag/error/racing-reader atomicity contract | W:R; L:D; M:R |
| 6. Atomic replacement of one complete revision | `FileRenameInfoEx(REPLACE_IF_EXISTS|POSIX_SEMANTICS)` preserves old handles and routes later opens to new, but lacks a racing-reader atomic old/new contract; `ReplaceFileW` is U | same-dirfd `renameat2(..., 0)` documents atomic replacement | general `rename` keeps an instance of the new name through a crash, but flags-`0` `renameatx_np` replacement and exact racing-reader old/new visibility are undocumented | W:R; L:D; M:R |
| 7. File-content sync before publication | exact temp write + `FlushFileBuffers` | exact temp write + `fsync(tempfd)` | `F_FULLFSYNC`; archived support list does not include APFS and current iOS guidance is best-effort, not an APFS/macOS contract | W:D; L:D; M:R |
| 8. Visible aggregate sync after publication | reopen/identity/canonical check + `FlushFileBuffers` | dirfd reopen/identity/canonical check + `fsync` | reopen/identity/canonical check + `F_FULLFSYNC`; no published APFS/macOS persistence guarantee | W:D; L:D; M:R |
| 9. Parent-directory entry sync | source handle opened `FILE_FLAG_WRITE_THROUGH`; NTFS documents rename-metadata flush to disk; no directory or volume flush | `fsync(parent-dirfd)` is explicitly required | no documented directory-entry durability primitive | W:D; L:D; M:R |
| 10. Restart and power-loss visibility | cannot qualify without 5 and 6; write-through hardware support is not universal | full sequence is documented to survive crash/reboot; hard-power evidence remains mandatory | general rename/APFS crash-protection statements do not qualify the exact sequence; cannot qualify without 5-9 | W:R; L:D; M:R |
| 11. Known failure vs unknown result | only pre-publication/proven-unchanged failures are known; all other post-invocation outcomes unknown | same conservative rule; exact old/new restart reload classifies | same conservative rule; unresolved API errors remain unknown | W:D; L:D; M:D as package policy; no automatic retry |
| 12. Reject link/reparse/mount/path/cross-volume substitution | retained non-delete directory handles, `OPEN_REPARSE_POINT`, reparse rejection, file/volume IDs | `openat2` containment + `NO_XDEV`, `statx` mount ID, inode/device checks | component `O_NOFOLLOW`, `fstatfs`, volume ID, and mount-trigger detection lack a documented complete race-free nested/same-filesystem mount-substitution proof | W:D; L:D; M:R |
| 13. Exact local filesystem and minimum-version support | fixed NTFS + NTFS major/minor is detectable; candidate Windows 10 1709+; no version accepted | fixed local ext4; Linux 5.8+ for mount ID; exact kernel/ext4/mount allowlist awaits evidence | APFS v2+ and macOS/`arm64` baseline facts are documented, but no unprivileged exact APFS-feature/OS-build/hardware contract maps them to properties 5-12 | W:D detection only; L:D candidate; M:R |

**Tuple results.** Windows/NTFS/`amd64` is not documentation-qualified because properties 5, 6, and
10 remain unresolved; property 9 now has a documentation-supported write-through candidate, not
implementation acceptance. Linux/ext4/`amd64` is documentation-qualified for a future native evidence
prototype at Linux 5.8+ on an exact later-reviewed allowlist, but that prototype is not authorized.
macOS/APFS/`arm64` is not documentation-qualified because properties 5-10, 12, and the exact
APFS/macOS/host-eligibility contract in 13 remain unresolved. APFS version-2 and `arm64` baseline
facts do not supply that contract. The all-or-nothing documentation gate is unmet; no platform may
begin prototype evidence and Slice 2 remains blocked.

**Documentation gap audit.** The focused Windows re-audit exhausted these Microsoft surfaces:
`CreateFileW` and File Caching; `FlushFileBuffers`, `ZwFlushBuffersFile`, and
`IRP_MJ_FLUSH_BUFFERS`; `SetFileInformationByHandle`, `FILE_INFO_BY_HANDLE_CLASS`,
`FILE_INFORMATION_CLASS`, `FILE_RENAME_INFO`, `FILE_RENAME_INFORMATION`, the MS-FSCC
`FileRenameInformation`/`FileRenameInformationEx` structures, and the MS-FSA
`FileRenameInformation` algorithm; `LockFileEx`/`UnlockFileEx` and process termination;
`FILE_ID_INFO`, `GetVolumeInformationByHandleW`, `GetDriveTypeW`, `FSCTL_GET_NTFS_VOLUME_DATA`,
`NTFS_VOLUME_DATA_BUFFER`, and the NTFS overview; `WRITE_THROUGH` capability reporting;
`IOCTL_VOLSNAP_FLUSH_AND_HOLD_WRITES`; and `ReplaceFileW`. They establish no-overwrite failure,
same-volume/same-directory addressing, old-handle retention, later-open routing, file-content flush,
NTFS rename-metadata write-through, identity/detection surfaces, and the Windows 10 1709 information-
class floor. They do not establish racing-reader atomic first publication or replacement, a complete
post-failure state map, universal hardware power-loss persistence, or an exact NTFS format-version
floor. Property 9 is documentation-supported only through the source handle's
`FILE_FLAG_WRITE_THROUGH`, not a retained directory handle or administrative volume flush;
properties 5, 6, and 10 remain unresolved. For Linux, the reviewed Linux man-pages and
kernel docs explicitly cover `openat2`, `flock`, `open`/`O_EXCL`, `renameat2`, file and directory
`fsync`, `statx`, mountinfo, and ext4 journaling. They qualify the exact candidate for later evidence,
but do not prove the composed application state machine or an accepted environment allowlist. For
macOS, the audit exhausted Apple's published `open(2)`, `flock(2)`, `rename(2)`, `fsync(2)`,
`fcntl(2)`, and `statfs(2)` pages; the APFS Tools and APIs guide; About Apple File System; the APFS
Guide introduction, FAQ, filesystem-details, and volume-comparison pages; current disk-write
guidance; Foundation exclusive-rename, mount-trigger, volume-identifier, and related volume-capability
properties; the 2020-06-22 Apple File System Reference; the current Disk Utility APFS-format guide;
and Apple-silicon porting guidance. The exact results are narrower than the exposed names:
`renameatx_np` has a published prototype but no flag/error/atomicity contract; path-based
`RENAME_EXCL` capability and general `rename` do not establish the descriptor-relative first-create
or replacement reader contract; `F_FULLFSYNC` has no published APFS/macOS guarantee and current iOS
guidance calls it best-effort; no unprivileged parent-directory entry sync is documented; the
identity and mount-trigger values do not provide complete race-free nested-mount containment; and the
APFS format reference's version/feature/software fields are not an unprivileged runtime compatibility
contract binding an exact macOS build and host to the required primitives. Properties 5-10, 12, and
13 therefore remain unresolved. Apple open source can corroborate symbols only and was not used to
fill a documentation gap. Native execution remains required for every `D` cell; cross-compilation
proves only that build tags compile and cannot change a platform result.

**Future independent-process evidence protocol.** The evidence harness remains design only.

1. A parent test controller creates one canonical absolute fixture root under the test framework's
   temporary root, records its resolved identity and a random run token, and passes both explicitly
   to children. Children reject any path outside that exact root, any changed root identity, and any
   relative, parent-traversing, linked, mounted, or reparse-substituted component. Only the parent may
   clean up, after revalidating the exact absolute root and token; no child runs recursive deletion.
2. One test executable exposes private child roles through arguments: `lock-holder`,
   `same-session-contender`, `different-session-contender`, `publisher`, `reader`, `fresh-verifier`,
   and `fault-controller`. Roles communicate readiness and acknowledgement only through inherited
   pipes/handles. They share no mutex, heap, in-process callback, or authoritative marker file.
3. The lock holder acquires the OS lock and reports entry. The same-session contender must not enter
   until release; the different-session contender must enter while the first remains held. Repeat
   after normal exit, forced termination, and crash. Each entrant proves the lock path still names
   the locked inode/file ID. Lock bytes and mere existence are deliberately varied and ignored.
4. Seed exact canonical old and new aggregate byte strings with distinct revisions and hashes.
   Multiple publisher processes repeatedly alternate complete revisions while multiple independent
   readers race the visible target. Every read must equal exactly one allowlisted complete canonical
   byte string and validate its session/revision; missing, empty, truncated, mixed, duplicate-key,
   extra-record, wrong-session, alternate-path, or substituted-inode data fails the run.
5. Fault hooks surround lock open/acquire, source open/read, observation boundary, source reread,
   temp create, every partial/full write, temp sync, publish syscall entry/return, visible reopen,
   identity check, visible sync, parent-directory sync, strict reload, result construction, and
   acknowledgement. The controller may return an injected error, close a handle, substitute a path,
   terminate the child, or crash it at each hook. It never treats a test marker as commit authority.
6. For a known failure before publication, a fresh verifier must see exact legacy/old authority. For
   any loss after publication is invoked and before durable acknowledgement, the result is
   `unknown_commit_result`; the controller must prove no automatic retry or second observation was
   started. A newly launched verifier then accepts exactly one full old or new revision, applies the
   corresponding guard, and rejects every other tree. A selected VM reboot/power-loss lane repeats
   the durability boundaries; process restart alone is not claimed as power-loss proof.
7. First-create races prove exactly one no-replace winner. Replacement races prove one serial winner
   per expected revision; the loser reloads a changed revision and rejects. Temp files, lock files,
   lock contents, pipe acknowledgements, projection files, caller claims, and alternate aggregate
   names are mutated independently to prove they carry no authority.
8. Each case snapshots the exact fixture tree before and after, verifies permissions/type,
   filesystem and mount identity, exact canonical bytes, monotonic revision, consumed observation,
   permanent unknown-turn high-water/no-replay state, and absence of out-of-root writes. Cleanup may
   remove only paths enumerated beneath the revalidated fixture root; failure preserves the fixture
   for review rather than broadening deletion.

The acceptance rule is strict: readers must observe exactly one full old or new canonical revision,
never missing, truncated, mixed, duplicated, substituted, or partially acknowledged authority. An
unknown result prohibits automatic retry even when a later reload shows the old revision; a new
reconciliation attempt requires separately authorized lifecycle semantics outside Slice 2.

**Rejected alternatives.** `os.Rename` is rejected as the cross-platform contract because Go
documents non-Unix non-atomic behavior. Windows `ReplaceFileW` is rejected because its documented
error cases can leave the replaced name absent or move both inputs, and its
`REPLACEFILE_WRITE_THROUGH` flag is unsupported. Directly opening the authoritative target with
`CREATE_NEW`/`O_EXCL` is rejected because partial first-revision bytes become visible before commit.
Create/delete marker locks are rejected because crash leaves stale existence and unlink/recreate can
split lock authority. Process-local mutexes do not cover other hosts/processes. File sync without
parent-directory sync proves content, not namespace durability. `fsync` without `F_FULLFSYNC` is
insufficient for the selected macOS durability claim. Network locks, generic “atomic file” packages,
SQLite, journaling/tombstone files, ordered multi-file writes, and volume-wide privileged flushes are
rejected for this slice because they either change the authority model, do not expose the exact
guarantees, or expand deployment/security scope. Cross-compilation and same-process tests are not
platform evidence.

**Exact unresolved maintainer choices.** Before Slice 2 can begin, a maintainer must explicitly:

- supply or identify normative primary documentation that closes the Windows/NTFS concurrent-reader
  atomicity gaps for first publication and replacement (properties 5 and 6) and the resulting
  restart/power-loss qualification gap (property 10); then accept an exact NTFS/version/host
  allowlist. Property 9's write-through rename-metadata candidate and property 12's containment and
  identity checks are documentation-supported only, not primitive, allowlist, or implementation
  acceptance;
- supply or identify normative primary documentation that closes the macOS/APFS descriptor-relative
  exclusive-rename and replacement atomicity, file and parent-directory durability, complete
  mount-containment, and exact APFS-feature/macOS-build/host-eligibility gaps; documented APFS
  version-2 fields, `arm64` availability, or `F_FULLFSYNC` success alone do not close those gaps;
- accept Linux 5.8 or later, ext4 on one local non-removable mount, and the exact runtime
  filesystem/mount-identification deny policy as the sole documentation-qualified candidate; every
  network, FUSE, overlay, tmpfs, removable, cross-mount, bind-mounted, or unknown filesystem remains
  unsupported by default;
- after all three platform documentation gates pass, select and separately review the exact
  `golang.org/x/sys` version needed to expose the accepted primitives. The currently resolved
  indirect `v0.28.0` is evidence of API availability only and is not approved for promotion; no CGO
  or new third-party library is implied;
- accept exact lock-artifact location, lifetime, permissions/ACLs, inheritance rules, timeout and
  cancellation behavior, and the requirement that cleanup never deletes a live/stale lock inode;
- accept the exact known-failure versus unknown-result error mapping for each syscall and the
  restart/operator response, with no automatic retry, repair, replay, or legacy fallback;
- select the controlled native hosts/filesystems and VM reboot/power-loss method for executable
  durability evidence, including retained evidence artifacts and review criteria; and
- separately authorize Slice 2 implementation and its harness after this packet is accepted.

Until those choices are explicit, every unsupported or ambiguous platform guarantee fails closed,
the legacy guard remains authoritative, and no observation, aggregate creation/replacement,
projection, decision, claim, prompt, fallback, retry, or replay is authorized. Classifier,
recovery-only operation, claims, bindings, responses, fallback, adapter pinning, guards, controls,
rollback, retry, migration, and permanent no-replay behavior remain unchanged.

**Open implementation gates and maintainer choices.** These facts are intentionally unresolved and
must not be silently selected to simplify implementation:

- Slice 1 already selected and implemented the unused package-private aggregate directory/name,
  schema/version syntax, canonical encoding, bounds, revision origin, digest syntax and SHA-256 path
  derivation, and inert package-state path. No production reader or writer uses them and no aggregate
  has been written. Schema evolution, permissions hardening, corrupt-record/operator recovery,
  platform storage primitives, migration, cutover, projection, and later lifecycle activation remain
  unresolved;
- the Windows normative documentation still needed for concurrent-reader atomic first publication
  and replacement (properties 5 and 6) and resulting restart/power-loss qualification (property 10),
  plus maintainer acceptance of an exact NTFS/version/host allowlist. Property 9's write-through
  rename-metadata candidate and property 12's containment and identity checks are documentation-
  supported only. The macOS normative documentation needed for properties 5-10, 12, and the exact
  APFS-feature/macOS-build/host eligibility in 13 also remains open, followed by maintainer acceptance
  of one all-platform primitive/library and supported host/filesystem/version matrix. Linux's
  documentation-qualified candidate does not permit Linux-only implementation or prototype work;
  network/removable/virtual filesystems remain unsupported
  until separately proven;
- the trusted way provider-pool obtains and byte-identifies Pipeon's canonical VS Code workspace
  adapter/guard slice without accepting display state or a caller digest as authority;
- the exact private requester surface and authentication/authorization boundary; CLI, MCP, other
  consumers, and administrative repair are not implicitly approved;
- mixed-version rollout and downgrade behavior that guarantees old readers/writers cannot mutate or
  trust frozen legacy records after aggregate cutover;
- retention duration, audit access, permissions, and eventual cleanup for frozen binding/state/claim
  files, the legacy Pipeon slice, temporary files, and transaction-lock artifacts;
- restart/operator UX for malformed aggregates and unknown commit results, including how a blocked
  session is inspected without providing retry, replay, repair, or legacy-fallback authority;
- exact explicit-user-decision wording, UI, local-authenticity proof, expiry/cancellation semantics,
  durable decision identity, and ambiguous-delivery handling;
- projection versioning, stale-revision display, and Pipeon persistence/query failure UX; projection
  repair can improve display only and cannot change lifecycle authority;
- rollback after authority cutover. Reverting to legacy authority is prohibited by the accepted
  direction, so any supported software rollback/minimum-version mechanism requires an explicit
  compatibility and maintainer decision; and
- final platform evidence acceptance, implementation evidence acceptance, and any later cleanup,
  operations, CAS-15 consumer, or public surface remain separate maintainer gates.

Unresolved gates keep the session blocked. They do not weaken, redesign, or defer the accepted
single-owner aggregate, exact compare-and-commit, projection-only Pipeon, explicit user decision,
strictly later fresh-turn, or permanent no-replay requirements.

**Bounded transactional-store reconsideration — SQLite candidate (2026-08-04).** This subsection
compares SQLite against the accepted safety invariants only. It does not select a dependency or
configuration, authorize implementation or a prototype, reduce Windows/Linux/macOS support, change
lifecycle authority, or supersede the accepted product/storage direction above.

The only candidate shape that survives the comparison is one private database per App Server
session, stored in that session's package-state directory, with one strict authoritative aggregate
record. A single shared database is rejected as a candidate shape: SQLite permits only one writer
per database, so it would serialize unrelated session writers and violate the accepted requirement
that different sessions not share lock authority or needlessly block one another. The database and
any SQLite-required journal, WAL, shared-memory, or temporary support files are collectively the
physical store; no file's existence, including the main database file, independently establishes
lifecycle authority. Authority would still require one successfully committed transaction followed
by strict aggregate schema, identity, revision, fingerprint, and outcome validation.

| Accepted safety invariant | SQLite documentation result | Reconsideration result |
| --- | --- | --- |
| One provider-pool transaction owner; unrelated sessions remain independent | SQLite serializes writers per database, including across processes, while separate databases have separate locking domains ([transaction isolation](https://www.sqlite.org/isolation.html), [transactions](https://www.sqlite.org/lang_transaction.html)). | **Conditional match only with one database per session.** A shared cross-session database is rejected. Provider-pool ownership and the existing single-session mutation boundary remain unchanged. |
| One authoritative aggregate and one indivisible old-or-new commit | SQLite documents serializable ACID transactions and atomic commit; in rollback mode the rollback journal protects the pre-transaction state until the commit point ([transactional guarantee](https://www.sqlite.org/transactional.html), [atomic commit](https://www.sqlite.org/atomiccommit.html)). | **Strong candidate match.** One transaction can replace the aggregate record without composing rename, directory-sync, and replacement primitives in package code. Exact SQL schema and representation remain unselected. |
| Crash/restart and power interruption expose one valid old or new revision, never a partial revision | SQLite documents hot-journal recovery after process or OS failure and durability through power loss, subject to truthful locking, flush, deletion, and storage-device behavior. Rollback mode with `synchronous=EXTRA` additionally syncs the containing directory after journal unlink ([atomic commit](https://www.sqlite.org/atomiccommit.html), [`synchronous`](https://www.sqlite.org/pragma.html#pragma_synchronous)). | **Documentation-supported but not yet accepted.** The guarantee still depends on an exact version, VFS, journal mode, synchronization mode, filesystem, OS, and hardware contract plus native interruption evidence on every supported platform. |
| Cross-process exclusion spans source observation, recovery observation, comparison, commit, and authoritative reload | SQLite's pager and VFS use operating-system locks to coordinate processes; the documented Windows VFS uses `LockFile`/`LockFileEx`, and Unix VFSes use advisory locks ([locking](https://www.sqlite.org/lockingv3.html), [VFS](https://www.sqlite.org/vfs.html)). | **Partial match.** A later design must prove that one write transaction holds the required per-session exclusion for the entire accepted observation/compare/commit window and fails closed on contention, expiry, cancellation, or lock failure. No prototype is authorized by this packet. |
| An ambiguous commit response never authorizes replay, retry, fallback, or inferred terminal outcome | SQLite exposes transaction state on a live connection, while commit and I/O failures can have result-dependent transaction effects ([transactions](https://www.sqlite.org/lang_transaction.html), [`sqlite3_get_autocommit`](https://www.sqlite.org/c3ref/get_autocommit.html)). | **Policy remains application-owned.** After connection, process, OS, or host loss, provider-pool must reopen and strictly reload the exact expected revision. If the result cannot be proven, it must preserve `reconciled_outcome_unknown`; it must never resend, replay, repair, or fall back automatically. Exact binding-specific error mapping remains unresolved. |
| Support files, stale files, and cleanup cannot become independent authority | SQLite rollback journals, WAL files, and shared-memory files can be required for recovery or database integrity; SQLite explicitly treats temporary-file details as implementation-dependent ([temporary files](https://www.sqlite.org/tempfiles.html), [WAL](https://www.sqlite.org/wal.html)). | **Conditional match.** Required sidecars must live in the same private session directory, inherit the accepted protection boundary, survive recovery, and never be parsed as lifecycle authority. Exact allowed files, cleanup rules, and backup/move semantics must be pinned to the selected SQLite version and mode. |
| The store is private to the current user and supported on Windows, Linux, and macOS without weakening the allowlist | SQLite provides built-in Windows and Unix VFSes and supports Windows, Linux, and macOS, but warns that correctness depends on reliable filesystem locks and sync behavior; dangerous no-lock VFS variants exist ([VFS](https://www.sqlite.org/vfs.html), [features](https://www.sqlite.org/features.html), [URI parameters](https://www.sqlite.org/uri.html)). | **Platform breadth match; environment proof gap remains.** The accepted Windows DACL and Unix `0700`/`0600` requirements still apply to the per-session directory and every physical store file. Network/removable filesystems, aliases/links, no-lock VFSes, and unproven host/storage combinations remain excluded pending an exact all-platform allowlist. |
| Storage substitution cannot alter recovery, cutover, dispatch, projection, or explicit-user-decision semantics | SQLite supplies storage transactions and recovery, not App Server lifecycle authority. | **No lifecycle change.** The accepted legacy-source reread and byte comparison, exact compare-and-commit rules, cutover boundary, projection-only Pipeon state, later-turn claiming, explicit user decision, and permanent no-replay guarantees remain mandatory and separately gated. |

**Decision result.** SQLite materially improves the documented transactional-store fit over a
package-owned composition of raw rename and directory-sync primitives, and a private per-session
database is therefore retained as a credible alternative candidate. It is not accepted for
implementation. The comparison does not prove the exact cross-process observation window, select
the physical transaction configuration, close the host/filesystem/hardware evidence gate, or resolve
sidecar protection and ambiguous-error mapping.

Before this candidate could replace the accepted canonical-file direction, maintainers must
separately accept the logical-one-aggregate/physical-database distinction and then authorize a later
selection slice for the exact SQLite version and Go binding, VFS, journal and synchronization modes,
database schema, sidecar/permission/cleanup contract, lock acquisition and timeout behavior,
binding-specific error taxonomy, and Windows/NTFS/amd64, Linux/ext4/amd64, and macOS/APFS/arm64
allowlist and native evidence plan. Until then, the all-platform gate remains unmet, Slice 2 remains
blocked, and implementation and prototype work remain unauthorized.

**Authorized SQLite selection — exact design/evidence baseline (2026-08-04).** This selection accepts
the logical-one-aggregate/physical-database distinction and replaces only the earlier raw-file
storage-primitive dependency direction if a later implementation is authorized. It does not modify
the inert Slice 1 code or path, add dependencies, create a database, authorize an evidence prototype,
start Slice 2, change cutover or lifecycle semantics, reduce DockPipe platform support, or permit a
Linux-only lane.

| Surface | Selected baseline | Required fail-closed check |
| --- | --- | --- |
| Go binding | [`modernc.org/sqlite v1.56.0`](https://pkg.go.dev/modernc.org/sqlite@v1.56.0), a CGo-free `database/sql` driver, with the binding-required [`modernc.org/libc v1.74.4`](https://gitlab.com/cznic/sqlite/-/raw/v1.56.0/go.mod). `github.com/mattn/go-sqlite3` is rejected because it requires CGo and a platform compiler toolchain. | Future module edits must pin both exact versions, retain `CGO_ENABLED=0` builds, review the complete transitive module graph and licenses, and fail if the resolved versions differ. No module edit is authorized here. |
| SQLite engine | The selected binding embeds SQLite 3.53.3 with source ID `2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62` ([3.53.3 release](https://sqlite.org/releaselog/3_53_3.html)). The closed 3.53.4 delta qualification below accepts this exact engine as the production dependency pin for the selected bounded store. | Query and require exact `sqlite_version()` and `sqlite_source_id()` before any store access. Any binding, engine, schema, SQL-operation, extension, `ATTACH`, VFS, journal, or synchronization change requires a fresh delta review. No module edit is authorized here. |
| Physical scope and name | One private directory per validated Pipeon session at the existing package-owned aggregate root, using the existing SHA-256 session-name derivation; future shape `<aggregate-root>/<session-digest>/aggregate.sqlite`. `aggregate.sqlite-journal` is the only selected SQLite sidecar. | Reject raw session IDs in paths, links/reparse points, nested/cross mounts, non-local storage, aliases, substituted identities, unexpected siblings, and any `-wal`, `-shm`, super-journal, attached database, or alternate database file. The existing inert `.json` path is not changed by this slice. |
| Open/VFS contract | One absolute file URI opened `mode=rw&cache=private`, with explicit `vfs=win32` on Windows and `vfs=unix` on Linux/macOS. The main file is first created empty by the future platform-specific private-file operation and parent entry is synchronized before SQLite opens it; empty physical existence never creates lifecycle authority. SQLite documents `win32` and `unix` as the native defaults ([VFS](https://sqlite.org/vfs.html)). | Prohibit `mode=rwc`, shared cache, `immutable`, `nolock`, `psow`, custom/no-lock VFSes, URI authorities, relative paths, `ATTACH`, loadable extensions, and backup/rename/copy while open. Require the opened database and parent identities to match the prevalidated session path. |
| Connection ownership | Exactly one dedicated `database/sql` connection for one provider-pool owner operation; pool limits are one open and one idle connection, and the handle is closed at the operation boundary. Driver parameters are `_txlock=exclusive`, `_dqs=0`, and `_error_rc=1`. | No second connection, helper process, reader pool, global database, long-lived idle connection, or non-provider-pool writer. Close releases the SQLite-held OS lock; close failure cannot upgrade an uncertain result to success. |
| Journal/durability | `PRAGMA journal_mode=DELETE`, `synchronous=EXTRA`, `fullfsync=ON`, `temp_store=MEMORY`, `mmap_size=0`, `busy_timeout=0`, `foreign_keys=ON`, `trusted_schema=OFF`, and `cell_size_check=ON`; database page size is fixed to 4096 before schema creation. SQLite documents rollback `EXTRA` as ACID and its directory sync after a DELETE-mode journal unlink ([synchronous](https://sqlite.org/pragma.html#pragma_synchronous)). | Apply settings on the dedicated connection, query every selected value back, require the exact returned value, record `PRAGMA compile_options`, and abort before observation on an ignored, substituted, unsupported, or mismatched setting. No default value supplies authority. |
| Lock window | Set and verify `PRAGMA locking_mode=EXCLUSIVE`, then acquire `BEGIN EXCLUSIVE` before reading any legacy source or recovery evidence. SQLite documents that exclusive locking mode retains file locks across transaction completion until the connection closes ([locking mode](https://sqlite.org/pragma.html#pragma_locking_mode)); the same connection performs post-commit strict reload before closing. | `busy_timeout=0` keeps each SQLite attempt nonblocking. Provider-pool may poll only lock acquisition, with caller cancellation and the already accepted absolute 30-second cap. `BUSY`/`LOCKED` after observation begins is not retried. Different session databases must remain independently acquirable. |
| Sidecar/privacy | In exclusive locking mode the rollback journal can remain after commit and is part of the physical store, not stale cleanup material. SQLite requires a hot journal to stay paired with its database ([temporary files](https://sqlite.org/tempfiles.html), [corruption hazards](https://sqlite.org/howtocorrupt.html)). Parent directories remain Unix `0700` or Windows current-user/`SYSTEM`; main and journal files must be Unix `0600` or carry the same restricted Windows DACL. | Never parse, move, copy, replace, truncate, quarantine, or delete the journal outside SQLite. Verify type, identity, ownership/DACL/mode, and sibling set before and after the operation. Any unexplained file or protection widening blocks; future native evidence must prove the SQLite-created journal meets the exact protection contract on all three tuples. |

The selected schema preserves the existing Slice 1 canonical JSON as the only lifecycle payload. SQL
columns provide a singleton/CAS envelope and must equal the values decoded from those exact canonical
bytes; neither an envelope field nor the database file's existence is independently authoritative:

```sql
CREATE TABLE app_server_aggregate (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    pipeon_session_id TEXT NOT NULL CHECK (length(pipeon_session_id) BETWEEN 1 AND 256),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    canonical_json BLOB NOT NULL CHECK (length(canonical_json) BETWEEN 1 AND 16384),
    canonical_sha256 BLOB NOT NULL CHECK (length(canonical_sha256) = 32)
) STRICT;
PRAGMA user_version = 1;
```

Initial migration requires no row, then inserts singleton `1` only after the accepted legacy-source
reread and byte comparison. Later mutation uses one conditional update matching singleton, exact
session ID, previous revision, and previous canonical SHA-256; zero or multiple affected rows reject.
Before commit, provider-pool decodes and byte-compares the candidate row. After commit, the same
exclusive-locking connection reloads the row, requires one exact canonical aggregate, revalidates all
existing schema/identity/revision/fingerprint/outcome invariants, and only then classifies committed.

**Selected error classification.** Lock-acquisition `SQLITE_BUSY`/`SQLITE_LOCKED` may be polled only
before observation. A pre-write validation or zero-row CAS failure rolls back and rejects. After a
write starts but before `COMMIT`, success requires an explicit rollback and exact old-row reload;
rollback/reload uncertainty becomes `unknown_commit_result`. Once `COMMIT` is invoked, it is never
retried. A documented `SQLITE_BUSY` commit leaves the transaction active, so one rollback plus exact
old-row reload may prove unchanged; every other commit error, connection loss, process/OS loss, or
failure to prove the exact old row is unknown. Commit success followed by reload, sidecar, permission,
close, or acknowledgement failure is also unknown until a fresh recovery-only restart open permits
SQLite to apply any required hot-journal recovery and then strictly reloads without an application
write. No branch authorizes resend, re-observation, repair, replay, fallback, inferred terminal
outcome, or a second commit attempt.

**Selected native-evidence plan.** The initial evidence cohorts remain Windows/local fixed-disk
NTFS/`amd64`, Linux 5.8+/local ext4/`amd64`, and macOS/local APFS/`arm64`; this selects test cohorts,
not a production allowlist or a reduction of general DockPipe support. Each run records exact OS/build,
kernel, architecture, filesystem/volume version and properties, storage device/virtualization facts,
Go version, module graph, SQLite version/source ID, compile options, VFS, queried pragmas, DACL/modes,
and pre/post file-tree hashes. The existing independent-process protocol and counts remain: 10,000
old/new reader-publication cycles, 1,000 same-session contention/forced-termination cycles while a
different-session writer succeeds, every deterministic failure boundary, and three controlled VM
hard-reboot or power-loss trials at every durability boundary. Readers may fail closed with
`BUSY` while the exclusive owner is live, but every successful post-release read must be exactly the
old or new canonical row; `quick_check`, strict decode, envelope equality, revision monotonicity,
permanent no-replay state, sidecar pairing, and protection checks must all pass.

**Closed SQLite 3.53.4 version-skew qualification (2026-08-04).** The newest exact CGo-free binding
remains `modernc.org/sqlite v1.56.0`; its tagged `go.mod` requires Go 1.25.0 and
`modernc.org/libc v1.74.4`, while its supported matrix retains Windows/`amd64`, Linux/`amd64`, and
macOS/`arm64` on SQLite 3.53.3. The complete official [3.53.3-to-3.53.4 check-in
timeline](https://sqlite.org/src/timeline?from=version-3.53.0&to=version-3.53.4&to2=branch-3.53&y=ci)
was reviewed against the selected schema and operation surface:

- Check-in `bf70dadc2d` changes hot-journal recovery only for a crash-corrupted super-journal record.
  The originating [official report](https://sqlite.org/forum/info/2026-07-20T18:27:00Z) states that
  the defect requires a multi-database `ATTACH` transaction. This store prohibits `ATTACH`, alternate
  databases, and super-journals before open and rejects any unexpected sibling, so that state is
  unreachable without an already-fail-closed contract violation.
- Check-in `a210f6f939` replaces an unchecked double-to-`int64` cast in VFS current-time conversion and
  one window-frame numeric check. The fixed store issues no date/time or window SQL, uses
  `busy_timeout=0`, and does not derive locking, synchronization, commit, recovery, or error authority
  from VFS wall time.
- Check-in `5d7c6fe1e9` affects expression indexes, subtypes, and unary `+`; the selected singleton table
  has no expression index, subtype operation, or unary-`+` SQL.
- Every other intervening check-in is patch metadata or is confined to the CLI/shell, `sqlite3_rsync`,
  tests, FTS3/4/5, RBU, session/rebaser, JSON/JSONB, `fileio`, incremental-integrity-check,
  `normalize`, Fossil-delta, `series`, `amatch`, or `fuzzer` surfaces. None is loaded, invoked, or
  represented by the fixed private schema and bounded SQL; canonical lifecycle JSON remains an opaque
  `BLOB`, loadable extensions are prohibited, unexpected schema is rejected, and `quick_check` does
  not authorize any of those features.

Therefore every 3.53.4 fix is demonstrably outside the selected rollback-journal, exclusive-locking,
native-VFS, synchronization, sidecar, fixed-schema, and error-classification surface. The version-skew
gate is closed: `modernc.org/sqlite v1.56.0` + `modernc.org/libc v1.74.4` + the exact SQLite 3.53.3
source ID above is the accepted production dependency baseline. This acceptance does not authorize a
module edit, evidence harness, implementation, migration, cutover, lifecycle activation, or Slice 2.

**Selection result and remaining gates.** The storage shape, binding family, exact evidence versions,
schema, VFS mapping, transaction settings, lock window, sidecar policy, error policy, and evidence plan
are selected. The prior standard-library-plus-`x/sys` and persistent empty lock-file direction is
superseded only for this SQLite candidate; no engine code is implicated. Production dependency
selection is closed on the exact versions and source ID above. The separately authorized dependency
pin and test-only Windows smoke slice below completes only the module edit and bounded evidence
harness authorization. Production use remains gated by the complete three native cohorts proving
exact lock release, journal protection, power-loss durability, and host eligibility, plus later
maintainer authorization for any Slice 1 path/loader revision, Slice 2 implementation, migration,
cutover, or lifecycle activation. Slice 2 remains blocked.

Closed dependency-pin, native transactional-store, and Linux VM qualification history is retained
verbatim in the
[TASK-013 closed history and evidence archive](./codex-app-server-adapter-closed-history.md#closed-transactional-store-platform-and-vm-evidence).

The implementation test matrix is:

1. **Adapter selection:** a new normal Pipeon Codex session defaults to App Server; the explicit exec
   escape hatch works; `/codex`, other providers, workflows, existing bindings, and callers without a
   Pipeon adapter choice retain current behavior; unknown values fail closed; a session never drifts
   or rebinds adapters.
2. **Model and capability selection:** validated available stable model/reasoning combinations render
   and execute exactly as selected; removal, mismatch, reroute, and unsupported combinations fail
   visibly without substitution. Unknown authority-expanding capabilities remain disabled and each
   experimental capability requires its own advanced opt-in.
3. **Lifecycle/rendering:** starting, ready, running, both waiting states, completed, interrupted,
   failed, disconnected, and recovery-required render from neutral records with contiguous cursors;
   the effective adapter/model/reasoning/approval/sandbox policy stays visible; no raw protocol or
   private payload reaches extension source or persisted state.
4. **Policy and decisions:** native configured approval plus workspace-write are the new-session
   defaults. Manual and native automatic-review modes remain distinct from sandbox authority;
   broader access requires conspicuous per-session confirmation and is not inherited. Approval and
   user-input responses require the complete current one-time
   correlation; duplicate, stale, cross-session, cross-process, and post-disconnect responses fail
   closed. Denial remains denial; DockPipe never blindly approves on Codex's behalf.
5. **Fallback/rollback:** initialization failures may fall back before `turn/start`; every failure
   after dispatch blocks replay; the exec escape hatch and administrative App Server disablement
   handle new, idle, active, waiting, and disconnected sessions exactly as above; automatic
   reconciliation returns ready only after verified idle.
6. **Regression:** the existing provider-pool Codex exec binding/resume tests, host bridge tests,
   bounded-worker tests, package contract tests, and Pipeon webview smoke tests remain green.
7. **Controlled integration:** through the primary/default App Server route, one Pipeon session
   selects an advertised model/reasoning combination, completes a no-tool turn,
   denies a requested file change, answers bounded user input, interrupts a turn, and observes
   transport/child loss without replay. Separate cases prove native automatic review within
   workspace-write, explicit broader-access confirmation, non-inheritance, and the `codex_exec`
   escape hatch.

#### Remaining cross-platform evidence and acceptance gates

CAS-13 is Windows-only evidence. Before CAS-14 is complete, controlled host-resident evidence is
still required on current supported Linux and macOS hosts for initialization/version/schema gate,
advertised model/reasoning validation, workspace-write containment, native manual and automatic
review modes, explicit broader-access confirmation and non-inheritance, bounded user input,
interruption terminal, clean exit, transport loss, direct-child termination, persisted idle recovery,
path/root validation, and process-tree cleanup. Raw payloads and credentials remain excluded on
every platform.

The founder product decision above is accepted. CAS-14 implementation still requires a separately
bounded implementation action and can be called complete only after all of the following are true:

- the primary/default single-consumer adapter, session-pinned model/reasoning/approval/sandbox
  selection, stable/experimental capability gates, neutral input-response seam, rendering contract,
  fallback, rollback, retention, and recovery behavior above are
  implemented and fixture-tested without changing bounded workers or engine code;
- the focused Go/package/Pipeon tests and the controlled Windows, Linux, and macOS evidence pass
  with no raw-protocol or sandbox-policy regression;
- the final evidence review confirms that `codex_exec` remains the governed legacy/fallback adapter,
  the App Server stays host-resident, every selected policy is visible and validated, each turn
  retains its selected native sandbox, and no prompt was replayed; and
- the maintainer explicitly accepts the CAS-14 implementation evidence.

CAS-15 cannot begin merely because CAS-14 is implemented or the default route exists. It requires
CAS-14 to be complete, a reviewed first-consumer evidence packet with rollback exercised, and a new
explicit maintainer decision naming exactly one additional compatible consumer. CAS-16 fallback
surface, CAS-17 operations guidance, ForgePipe, and remaining provider-pool work stay deferred.

## Likely impact map

- packages/dorkpipe/lib: provider-neutral contracts, adapter package, state and tests;
- packages/dorkpipe-mcp/mcpbridge: normalized host session/approval operations;
- packages/dorkpipe/lib/cmd/dorkpipe/provider_pool.go: adapter selection retaining exec;
- packages/dorkpipe/resolvers/dorkpipe/assets/provider-pools/catalog.yml: capability policy;
- Pipeon extension: normalized session/event UI;
- DorkPipe/Pipeon tests and docs.

Do not modify those production areas for this research task.
