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
| `packages/dorkpipe/mcp/mcpbridge/catalog.go`, `server.go`, and `tier.go` | Closed adapter request plus provider-neutral session event/decision/input/cancel/recover operations; host-resident supervisor ownership keyed by workspace and Pipeon session; existing exec tier/path enforcement retained. |
| `packages/dorkpipe/mcp/mcpbridge/server_test.go`, `tier_test.go`, and `codex_session_test.go` | Tool schema/tier, session isolation, adapter pinning, safe fallback, stale-decision rejection, redaction, and rollback tests. |
| `packages/dorkpipe/lib/providersession/contract.go` and `contract_test.go` | Opaque stable model/reasoning catalog, exact selected/effective policy and capability records, bounded prompt record, and one-time user-input response contract only; no adapter selection or provider protocol types. |
| `packages/dorkpipe/lib/appserversupervisor/model_policy.go`, `protocol.go`, `lifecycle.go`, `approval.go`, `hardening.go`, `supervisor.go`, and `recovery.go` | Validate available model/reasoning and stable capability catalogs, resolve an opaque turn input, map user-selected native approval/sandbox policy, answer bounded user input, expose neutral events, and retain fail-closed lifecycle/recovery rules. |
| Existing focused tests in `packages/dorkpipe/lib/appserversupervisor` | Extend lifecycle, approval, contract, recovery, hardening, and source-boundary fixtures for the consumer seam. |

The provider-neutral contract row above is complete. It adds no adapter selection, dynamic catalog
discovery, policy mapping, supervisor behavior, MCP operation, Pipeon control, fallback, or production
authorization.

The first supervisor-only CAS-14 foundation is also complete. After the existing initialization gate,
the fixture-backed supervisor can request one bounded model catalog, reject incomplete or unsafe
catalogs, derive an order-independent opaque catalog reference, and pin one exact advertised stable
model/reasoning combination. It returns a validated `EffectivePolicySnapshot` with the selected and
effective values identical, human review and workspace-write unchanged, and no enabled capability
records. The CAS-13 `gpt-5.6-terra` / `high` combination remains in the fixtures as the proven baseline,
while another advertised stable combination proves the catalog is not hard-pinned to that entry.

Empty, duplicate, unavailable, removed, mismatched, changed, paged, malformed, or rerouted catalog and
selection evidence fails closed without substitution. The safe catalog and policy projections are
defensively copied and pinned to one initialized idle supervisor.

The second supervisor-only CAS-14 projection slice is complete. A fixture-backed, order-independent
native policy catalog requires the proven human-review and workspace-write baseline and accepts
additional available stable approval/reviewer and sandbox choices only as exact opaque references.
Each authority-expanding approval or sandbox choice requires its own explicit per-session
confirmation. Empty, duplicate, unavailable, removed, mismatched, changed, unsupported, unconfirmed,
cross-confirmed, shell-command-enabling, policy-bypassing, or silently substituted policy evidence
disconnects fail closed. Approval automation cannot select or confirm sandbox authority, and a broader
sandbox choice cannot change approval authority.

The third supervisor-only CAS-14 projection slice is complete. A fixture-backed, order-independent
capability catalog requires a bounded non-empty set of stable, available opaque references and keeps
availability separate from explicit DockPipe support. Its baseline selection projects every catalog
record disabled. An enabled subset must name exact advertised supported references; every
authority-expanding or experimental reference requires its own per-session confirmation. Empty,
duplicate, unavailable, removed, changed, unsupported, unconfirmed, mismatched, or substituted
catalog/selection evidence disconnects fail closed. Catalog ordering, the selected model, approval or
sandbox policy, and another capability never imply support, selection, confirmation, or enablement.

The fourth supervisor-only CAS-14 projection slice established the bounded prompt-record and exact
lookup foundation. While one exact user-input request is pending, one normalized
`providersession.UserInputPrompt` is pinned to the complete current correlation and opaque prompt
reference. Exact lookup returns only a defensive provider-neutral copy and leaves the supervisor
waiting for input. Empty, duplicate, stale, expired, mismatched, cross-session, substituted,
unsupported, malformed, or oversized lookup evidence disconnects fail closed. The normalized prompt
exists only in the transient pending request; expiry and disconnect clear it, and prompt content never
enters events, snapshots, diagnostics, or audits.

The fifth supervisor-only CAS-14 slice is complete. One validated
`providersession.UserInputResponse` can be delivered once for the exact pending prompt, complete
correlation, and opaque prompt reference. This lane accepts one private provider question at a time;
choice prompts require an explicit complete option-reference-to-provider-answer mapping, so ordering,
display labels, another option, and availability never select or substitute an answer. Text answers,
selected option references, private question identity, and the private mapping exist only through the
bounded write and are cleared from pending state before delivery. Events and audits retain only safe
`user_input_delivered` / `user_input_resolved` classes. Duplicate, stale, expired, malformed,
oversized, mismatched, cross-session, unknown-option, post-disconnect, and replayed responses fail
closed. Multi-question provider batches remain unsupported rather than partially answered.

The sixth supervisor-only CAS-14 slice is complete. The exact validated single-question
`item/tool/requestUserInput` request now creates its normalized prompt and private answer mapping
directly; no caller supplies prompt or mapping evidence. Question whitespace is normalized into the
bounded summary, option labels are normalized into bounded display values, and text input retains the
contract's 4096-byte response ceiling. Each option reference is derived from the complete current
correlation plus the exact private question/option content, so it is independent of provider ordering
and cannot be substituted by a display label. The raw question, option objects, descriptions, question
identity, raw answer labels, and private mapping remain transient supervisor-local values and never
enter events, snapshots, diagnostics, or audits. Empty, malformed, over-bound, control-bearing,
duplicate, display-ambiguous, multi-question, or otherwise unsupported provider input disconnects fail
closed before any partial prompt or answer is exposed.

The opt-in controlled Windows user-input harness is implemented but did not complete its proof on
2026-08-03. A single bounded follow-up diagnostic found the exact request-production blocker in the
installed `codex-cli 0.144.1`: the generated protocol still contains the experimental
`item/tool/requestUserInput` method and accepts `initialize.capabilities.experimentalApi=true`, but
the `default_mode_request_user_input` feature is under development and disabled. The authenticated
no-write turn therefore completed with `user_input_tool_advertised=false`,
`user_input_tool_invoked=false`, `request_method_class=none`, and `schema_shape=none`. The harness sent
no answer, performed no retry, changed no production behavior, retained no provider payload, and
removed its temporary workspace. Provider-backed prompt normalization, non-first option-reference
delivery, the matching resolved transition, and live replay rejection remain unproven; the
fixture-backed contract remains unchanged.

A separately approved request-only follow-up then enabled `default_mode_request_user_input` through
an exact build-tagged launcher wrapper while retaining the experimental initialize capability, the
same deadline, and the same no-write prompt. One actual authenticated turn stopped at the expected
server request with `terminal_class=not_reached`, `user_input_tool_advertised=true`,
`user_input_tool_invoked=true`, `request_method_class=user_input`, and
`schema_shape=single_select_v1`. No response or retry was sent, no provider content was retained, and
the temporary workspace was removed. An initial harness launch was rejected locally before child
spawn because the production launcher intentionally allow-lists only the standard App Server command;
that produced no authenticated turn and was corrected only in the build-tagged diagnostic wrapper.

The separately approved supervisor-backed, feature-enabled request-only probe stopped safely at a
second independent gate. Its one authenticated no-write turn reached a non-input terminal with
`request_class=none`, `parser_class=not_reached`, `prompt_lookup=unavailable`, and
`response_sent=false`. The production supervisor initializer sends only its notification opt-outs and
does not advertise `experimentalApi=true`; enabling `default_mode_request_user_input` only on the App
Server process is therefore insufficient. The direct request-production diagnostic succeeded only
when both the server feature and client experimental capability were present. No answer or retry was
sent, and no production contract changed.

The separately approved build-tagged initialization shim then added only `experimentalApi=true` to the
diagnostic's first initialize frame, while the test-local launcher enabled the server feature. Its one
authenticated no-write turn reached the supervisor but disconnected before exposing a prompt:
`terminal_class=disconnected`, `request_class=none`, `parser_class=not_reached`,
`prompt_lookup=unavailable`, and `response_sent=false`. The shim's offline shape test passed, no answer
or retry was sent, and production initialization remained unchanged. The run did not retain the safe
disconnect sub-class, so it does not yet distinguish request ordering, turn correlation, item
correlation, or another fail-closed handler rejection.

The separately approved classified rerun produced `terminal_class=disconnected` and
`disconnect_class=other`, with `request_class=none`, `parser_class=not_reached`,
`prompt_lookup=unavailable`, and `response_sent=false`. This rules out the four retained lifecycle and
correlation classes. A no-turn comparison of the installed experimental schema then found safe
structural drift beyond the production parser's exact allowlist: request params now define optional
`autoResolutionMs`, and questions define optional `isOther` and `isSecret`. No field values or live
frame were retained. The production parser therefore remains correctly fail-closed, but this run did
not determine which optional field was present on the live request.

The separately approved direct request-only structural probe then isolated the live incompatibility.
It produced `terminal_class=not_reached`, `user_input_tool_advertised=true`,
`user_input_tool_invoked=true`, `request_method_class=user_input`, and
`schema_shape=single_select`; `autoResolutionMs`, `isOther`, and `isSecret` were all present. The
production parser's exact request/question allowlists exclude those three fields, so the
supervisor-backed probe correctly disconnected before correlation or prompt projection. No field
values, identifiers, prompt content, raw frame, answer, or retry were retained or sent.

The offline fixture-only compatibility decision is complete. The installed schema makes
`autoResolutionMs` nullable or a non-negative integer with no default, while `isOther` and `isSecret`
are booleans defaulting to false. None is unconditionally ignorable. Each is independently supportable
only through an exact default-only gate: `autoResolutionMs=null`, `isOther=false`, and
`isSecret=false` are safely ignorable defaults; any non-null auto-resolution value or either true
boolean remains unsupported and must fail closed. Malformed and unknown evidence remains invalid.
Build-tagged fixtures cover each accepted default and rejected active value; production parsing is
unchanged and no authenticated turn occurred.

The explicitly approved production-parser slice is complete. The App Server request parser now accepts
the three fields only when omitted or set to the exact compatible defaults:
`autoResolutionMs=null`, `isOther=false`, and `isSecret=false`. Any non-null auto-resolution value,
either true boolean, null boolean, type substitution, malformed value, unknown field, or broader
experimental shape remains unsupported and disconnects fail closed before prompt projection. Focused
fixtures prove the accepted default combination and each active, null, and substituted rejection.
Neutral prompt/response contracts, lifecycle dispatch, consumers, and broader experimental schema
support remain unchanged.

The separately approved supervisor-backed request-only proof ran exactly once through the existing
build-tagged dual opt-in. Its authenticated no-write turn ended with
`terminal_class=disconnected`, `disconnect_class=other`, `user_input_tool_advertised=true`,
`user_input_tool_invoked=unconfirmed`, `request_method_class=none_observed`, and
`schema_shape=none_observed`; production parsing and prompt lookup were not reached, and no response
or retry was sent. The retained class rules out the harness's inactive, turn-mismatch, item-mismatch,
and not-running cases, but intentionally cannot distinguish another unsupported experimental event
or request shape from an active experimental field value. Naming one of those as the cause would
require provider content that this diagnostic did not retain. The temporary workspace was removed,
and production contracts, parser behavior, lifecycle dispatch, and consumers remain unchanged.

The separately approved offline-only harness extension is complete. The existing build-tagged
dual-opt-in launcher now classifies the supervisor's pre-prompt stream as one of
`event_method_incompatible`, `event_shape_incompatible`, `default_only_request_incompatible`,
`request_shape_incompatible`, or `default_only_request_compatible`, with a bounded malformed-frame
fallback. It also retains only whether the user-input method was observed plus its method and schema
classes. App Server response frames are ignored; partial provider frames are capped at 1 MiB, cleared
after classification, and cleared again on close. Chunked offline fixtures prove every class and
verify that neither the transient reader nor its retained result contains provider content. No App
Server child, authenticated turn, response, retry, or production change occurred.

The separately approved classified rerun then executed exactly one authenticated no-write turn. It
produced `terminal_class=disconnected`, `disconnect_class=other`,
`pre_prompt_class=default_only_request_incompatible`, `user_input_tool_advertised=true`,
`user_input_tool_invoked=true`, `request_method_class=user_input`, `schema_shape=single_select`, and
`request_compatibility=default_only_request_incompatible`; production prompt projection was not
reached and `response_sent=false`. This is the exact safe blocker: the installed App Server emits the
supported method and single-select shape, but at least one experimental field carries active or
otherwise non-default semantics outside the parser's accepted `null`/`false` subset. The classifier
intentionally retained neither the individual field nor its value, so no narrower cause is claimed.
No retry or response occurred, the temporary workspace was removed, and production contracts and
parser behavior remain unchanged.

The smallest evidence-backed next CAS-14 action is no code change: retain the fail-closed blocker and
keep active experimental user-input semantics unsupported until either the installed App Server emits
the already supported default-only subset or a separately approved provider-neutral contract decision
defines safe active semantics. Another diagnostic, parser widening, response proof, lifecycle
dispatch, and consumer wiring are not implied.

The bounded selected-policy lifecycle slice now makes the already validated CAS-14 selection govern
the healthy package-local request path. `StartThread` requires complete pinned model, native-policy,
and capability selections, revalidates the exact catalogs and effective snapshot immediately before
I/O, and dispatches the exact selected model/reasoning values. The validated `human-review` and
`workspace-write` baseline advertisements retain the already proven CAS-13 provider-private mapping
(`untrusted` reviewed by `user`, and `workspaceWrite` with declared roots and network disabled), so
those two independent dimensions are dispatched without deriving wire values from their opaque refs.
Zero enabled capabilities is the only dispatchable capability state. The resolved catalog/snapshot
and caller policy are fingerprinted separately from the existing recovery policy key and bound to the
thread; read, resume, turn start, and steer reject catalog, snapshot, caller, or binding drift before a
request is sent. Existing workspace/root validation, recovery snapshots, no-replay/no-retry behavior,
audit/correlation, disconnect handling, and the consumer boundary remain unchanged.

The bounded non-baseline approval/reviewer mapping slice is complete. The stable protocol schema
generated offline by installed `codex-cli 0.144.1` defines `approvalPolicy` through
`AskForApproval`, including the exact string `untrusted`, and defines `approvalsReviewer` through
`ApprovalsReviewer`, including the exact stable string `auto_review`. The schema binds both fields
directly to `thread/start` and `turn/start`; its description identifies `auto_review` as the native
risk-based reviewing subagent. The fixture-backed `native-auto-review` advertisement now retains only
that proven private pair: `untrusted` plus `auto_review`.

The pair is independently selected, individually session-confirmed, revalidated with the pinned
catalog/effective snapshot immediately before every lifecycle operation, and immutably bound to the
thread. The exact values are dispatched without substitution while the existing `workspace-write` /
`workspaceWrite`, declared-root, network-disabled sandbox and zero-enabled-capability baseline remain
unchanged. Missing, partial, duplicate, ambiguous, changed, removed, unavailable, unconfirmed,
selection/effective-mismatched, or caller-drifted evidence is rejected before protocol I/O for read,
resume, turn start, and steer. The existing recovery policy key and all no-replay, no-retry,
no-fallback, correlation, audit, disconnect, and consumer behavior remain unchanged.

One non-baseline sandbox mapping is now proven and retained without lifecycle authorization. The
stable JSON Schema generated offline by installed `codex-cli 0.144.1`, without `--experimental`,
defines `danger-full-access` in `SandboxMode`, defines the exact `dangerFullAccess` discriminator in
`SandboxPolicy`, and binds those definitions to the stable thread/turn lifecycle parameter shapes.
The fixture-backed `broader-native-sandbox` advertisement therefore retains exactly that private pair;
the opaque reference, display meaning, ordering, availability, approval/reviewer selection, model,
capabilities, connection, and confirmation are never used to derive either wire value.

The option must remain stable, available, exactly selected/effective, and individually
session-confirmed. Its mapping participates in catalog identity, ambiguity and drift rejection,
defensive copies, and pinned-catalog comparison. Missing, partial, malformed, duplicate, second,
changed, removed, unavailable, unconfirmed, cross-confirmed, shell-command-enabling, or
policy-bypassing evidence fails closed. Approval automation cannot select or confirm sandbox
authority. The existing lifecycle resolver remains unchanged and accepts only the proven
`workspace-write` / `workspaceWrite` lane, so `StartThread`, `ReadThread`, `ResumeThread`, `StartTurn`,
and `SteerTurn` reject the mapped non-baseline option before protocol request I/O. Zero enabled
capabilities, the recovery policy key, immutable thread binding, and no-replay/no-retry/no-fallback
behavior remain unchanged.

One stable capability mapping is now proven and retained without enablement or dispatch. The same
non-experimental schema defines `InitializeCapabilities.requestAttestation` as a boolean that defaults
to `false`; `true` opts into `attestation/generate` requests for upstream `x-oai-attestation`. The
fixture-backed `request-attestation` advertisement retains exactly that private field/value pair and
marks it stable, available, supported, authority-expanding, and non-experimental. The opaque reference,
ordering, availability, support, authority classification, another capability, approval, sandbox,
model, or confirmation never derives the wire mapping.

The private pair participates in capability catalog identity, mapping ambiguity/drift detection,
defensive copies, and pinned comparison. Missing, partial, changed, duplicate, second-mapped, removed,
unstable, unavailable, unsupported, non-authority, experimental, or unconfirmed evidence fails closed.
The selected effective-policy baseline keeps it disabled and unconfirmed. Existing initialization
still sends only its notification opt-outs, and unchanged lifecycle resolution rejects every enabled
capability before protocol I/O for thread start/read/resume and turn start/steer. No attestation request,
credential, account value, authentication action, or provider call was made or retained.

The bounded pre-initialization capability decision is now complete without consuming the mapping.
A package-private one-shot planner may be created only for a fresh, not-yet-started supervisor. It
accepts fixture-backed capability evidence plus exactly one `request-attestation` intent and one
independent confirmation bound to the exact package session, supervisor incarnation, complete
order-independent capability-catalog identity, and capability reference. The resulting scalar plan
fingerprints the exact private `requestAttestation=true` mapping and confirmation, but is deliberately
detached from `Supervisor`; it cannot alter initialization, lifecycle dispatch, request handling,
recovery, or a consumer.

Catalog evidence is validated before child startup because it is fixture-backed package evidence;
provider-backed model discovery and native-policy selection remain after initialization. Missing,
malformed, stale, drifted, unsupported, experimental, non-authority, ambiguous, multi-capability,
cross-session, cross-supervisor, replayed, or mutated evidence fails closed. A new or recovered
supervisor has a different incarnation target and retains no plan or confirmation authority. The
production initialize frame remains the exact opt-out-only baseline, and `attestation/generate`
remains unsupported. No provider-neutral contract change is warranted because the plan contains
provider-private initialize sequencing and mapping evidence only.

The bounded offline `attestation/generate` contract decision is complete with no production-code
change. The non-experimental schema generated by installed `codex-cli 0.144.1` defines a server
request that requires an `id`, the exact method `attestation/generate`, and `params`; the request id
may be a string or signed 64-bit integer. Although that generated JSON Schema leaves the object
open, the exact official `rust-v0.144.1` source at commit
`44918ea10c0f99151c6710411b4322c2f5c96bea` defines `AttestationGenerateParams {}` and generates the
TypeScript shape `Record<string, never>`. The request params are therefore exactly empty. They carry
no thread, turn, item, package session, supervisor incarnation, connection, origin, challenge,
upstream destination, or one-shot confirmation binding.

The named success payload remains only a string `token` described as an opaque client attestation
token. The exact source adds no minimum or maximum length, format, provenance, audience, expiry,
replay, challenge, signing, credential, account, or authentication contract. App Server selects the
first attestation-capable live connection already subscribed to the current internal thread, sends
that connection the empty request with no thread id in the request envelope, and waits 100 ms. Thus
thread routing is internal App Server state rather than caller-supplied attestation params.

The exact source also establishes an incompatible upstream failure policy. A successful response is
serialized as `{"v":1,"s":0,"t":"<token>"}` for `x-oai-attestation`; timeout, request failure,
cancellation, and malformed response are serialized as the same header with status `1`, `2`, `3`,
or `4` and no token. If there is no capable connection or no usable header value, the Responses API
request continues without the header. In every case, attestation generation failure is diagnostic
header state rather than a reason to abort the provider request. CAS-14 requires the opposite:
missing, invalid, rejected, canceled, or timed-out evidence must stop before any provider/network
call. A request/result type or validator cannot repair that sequencing, so adding one would create a
misleading partial abstraction even though the empty request shape itself is now exact.

Production initialization therefore still emits only `optOutNotificationMethods` and omits
`requestAttestation`, `experimentalApi`, and MCP capabilities. `attestation/generate` remains absent
from request classification and dispatch, so an incoming request disconnects fail closed without a
response, token, credential/account access, authentication, provider call, network action, or header
mutation. Recovery and restart inherit no plan, confirmation, request, response, or token authority.

The separately approved installed-binary inspection is now corroborated by the exact tagged source.
It establishes only a client callback boundary: App Server can request and transport a
client-produced token. The reviewed tagged App Server, protocol, and core source paths expose no
generator; their only response construction is the integration-test fixture. The installed CLI does
not expose a DockPipe-usable token producer; exec mode reports generation as unsupported and TUI
reports it unavailable.

Neither official source nor installed artifacts define a token algorithm, trust root, challenge,
signing key, credential/account dependency, origin/audience binding, expiry, or replay rule.
DockPipe therefore cannot safely fabricate, retrieve, store, or return a token, and cannot infer
authorization from an authenticated Codex session, account availability, connection, approval,
sandbox, model, catalog, or confirmation. Production initialization and request rejection remain
unchanged.

There is no smaller safe package-local CAS-14 implementation action. The prerequisite is an upstream
Codex contract and implementation that both supplies an authoritative client token generator and
aborts the provider request when attestation is absent, invalid, rejected, canceled, or timed out.
Only after that behavior exists could a separately bounded DockPipe slice reconsider a private
request/result validator and pre-initialization handler. Initialization opt-in, request acceptance,
response dispatch, live probes, and consumer integration remain out of scope and unauthorized.

No second in-scope stable mapping is proven. `experimentalApi` is the prohibited global experimental
opt-in, `mcpServerOpenaiFormElicitation` is outside the no-MCP boundary,
`optOutNotificationMethods` suppresses notifications rather than enabling a capability, and
`SelectedCapabilityRoot` appears only as an unbound shared definition in stable `ThreadStartParams`.
That attestation verification changed neither the blocker nor the authorization boundary for a wire
change, live request, consumer, MCP, Pipeon, provider-pool, fallback, workflow, schema, or engine
action. The separately approved routing seam below does not consume the blocked capability.

The first bounded CAS-14 consumer-routing seam is now complete without App Server dispatch or a
default flip. The resource-scoped `pipeon.codex.sessionAdapter` setting accepts only `codex_exec` and
`codex_app_server` and remains `codex_exec` by default. Only a normal Pipeon Codex chat request reads
and forwards it as the closed `session_adapter` MCP/CLI field; non-Codex chat omits the field, and the
separate `/codex` host tool, bounded workers, workflows, and callers that omit the field retain the
existing exec route for an unpinned session.

An explicit choice requires a bounded non-empty Pipeon session id and is persisted on first use in
DorkPipe package state as one exact schema-versioned record under a SHA-256-derived filename. The
record is created with exclusive first-write semantics and then compared exactly on every explicit
request. Unknown values, malformed or extended records, missing choice on an already pinned session,
cross-adapter drift, a Codex adapter supplied for another provider, and concurrent conflicting first
writes fail closed before readiness checks, lease acquisition, Codex execution, or provider I/O.
`codex_exec` continues through the unchanged exec implementation. At this first routing boundary,
`codex_app_server` could be pinned but returned a visible not-enabled error before execution and never
substituted exec. The separately approved dispatch slice below replaces only that not-enabled branch.

`packages/dorkpipe/resolvers/dorkpipe/assets/provider-pools/catalog.yml`,
`packages/dorkpipe/mcp/mcpbridge/exec.go`, `dorkpipe.host_codex_chat`, bounded worker code,
schemas/YAML, and `src/lib` / `src/cmd` remain unchanged in this first implementation. The existing
`dorkpipe.provider_pool_chat` request remains the Pipeon entrypoint; the bridge honors the explicit
App Server adapter only for the eligible pinned Pipeon session and otherwise delegates to the
unchanged `codex_exec` provider-pool path.

The second bounded CAS-14 consumer slice established explicit one-turn App Server dispatch. After the
existing adapter binding is matched, provider-pool creates an exclusive per-session dispatch claim
before child startup. At that boundary every second request failed before supervisor construction,
provider I/O, or exec selection, so the short-lived command could not silently create a new thread
or replay the prompt while durable policy-aware continuation was absent.

The first request starts exactly one direct host `codex app-server --stdio` child through the existing
supervisor. It validates the caller's exact selected model with the fixed `high` reasoning baseline
against the live catalog, pins the proven human-review/workspace-write native mappings, and selects
zero enabled capabilities. The exec-only `config` model alias is rejected before the dispatch claim;
the App Server route requires one explicit model reference. `request-attestation` remains disabled and initialization remains the
notification-opt-out-only baseline. Missing models, policy drift, initialization failure, protocol
failure, and unsupported output fail visibly without model substitution, retry, or exec fallback.

One bounded prompt is passed only to private turn input and is never retained by the supervisor,
neutral events, audit, or recovery state. One bounded completed agent message may be held in memory
only after the exact correlated completed terminal event; it is cleared on failure, cancellation,
disconnect, or shutdown. Approval or user-input requests are reported as an interactive-control
failure and the turn lock prevents replay. This slice does not yet provide decision controls,
durable event rendering, fallback, rollback, a default flip, another consumer,
or controlled provider evidence.

The third bounded CAS-14 consumer slice replaces only the permanent second-turn block with exact
verified-idle continuation. Package state now retains a canonical bounded record containing the
Pipeon session hash binding, completed-turn counter, opaque provider session reference, deterministic
recovery evidence, and the exact model/reasoning pair. An exclusive per-session turn lock is created
before every child startup. A crash, unknown outcome, terminal failure, interactive request, malformed
state, policy drift, or incomplete persistence leaves that lock in place, so another invocation fails
before child or provider I/O. Only a correlated completion followed by the durable `thread_idle`
snapshot advances the record and releases the lock.

Each short-lived invocation receives a different package-derived process and connection incarnation.
For a later turn, a fresh direct App Server child loads the exact idle snapshot and audit evidence,
accepts at most the one expected clean local-shutdown event after that snapshot, re-queries the live
model catalog, re-selects the pinned human-review/workspace-write/zero-capability policy, and performs
one correlated idle `thread/read`. Only then may it start the next turn on the retained provider
session. A changed/removed model, policy or catalog mismatch, stale process/connection, non-idle
thread, audit gap or alternate suffix, state/session substitution, concurrent invocation, and
recovery failure all stop before turn start with no exec substitution or fallback.

Prompt and completed-text handling remain transient as above. Attestation, interactive approval/input
controls, cancellation UI, durable event rendering in Pipeon, fallback, rollback, default selection,
another consumer, and live/cross-platform provider evidence remain separate slices.

The bounded provider-neutral approval-request slice is now complete at the package-local supervisor
boundary. `providersession.ApprovalRequest` contains only the complete one-time correlation, one
closed safe reason (`command_execution`, `workspace_change`, or `declared_permission`), and the exact
allowed decision set. Command and workspace-change requests permit only `approve` or `deny`;
declared-permission requests remain deny-only. The supervisor defensively pins that record while the
turn is `waiting_for_approval`, and model/policy selection, capability metadata, connection state,
prior decisions, or consumer behavior supply no decision authority.

One explicit local approve is accepted only for the exact process incarnation, connection, session,
turn, item, request, and one-time decision identity. The same request remains paused until the exact
provider resolution arrives, then and only then returns to `running`. An explicit deny writes only
the private decline response and immediately closes the supervised turn as fail-closed
`approval_denied`; it never processes a resolution as continuation. Missing decisions cannot resume,
and malformed, duplicated, stale, substituted, cross-session, or cross-request decisions disconnect
fail closed. Fixture-only tests prove exact approval, terminal denial, decision-free blocking,
replay/substitution rejection, request isolation, and absence of a fallback launch.

This slice adds no provider-pool decision transport, MCP operation, Pipeon rendering or controls,
free-text input handling, automatic approval, fallback, attestation, or live provider execution. The
existing normal Pipeon App Server dispatch therefore still reports an interactive-control failure and
retains its unresolved turn lock when it receives an approval request. Wiring one exact neutral
approval request/decision through that already pinned consumer route is deferred to a separate slice.

The subsequent bounded consumer-controller slice adds only a package-private local decision-source
seam to that existing session-pinned App Server dispatcher. The zero-value production path supplies
no source and therefore preserves the exact interactive-control failure above. A source is invoked
once only after the dispatcher receives and validates an exact neutral `ApprovalRequest`; it receives
a defensive copy of that request and no adapter, policy, capability, model, connection, or prior
approval metadata. The dispatcher retains its own immutable copy, so source-side mutation cannot
widen the supervisor's pinned allowed-decision set.

An explicit returned decision must pass `ApprovalDecision.ValidateFor` against that retained request
before delivery through the same live supervisor. Approval keeps the dispatcher paused until the
matching `approval_resolved` correlation returns, then continues the same event loop and can reach
completion and verified-idle persistence only through the existing terminal and idle gates. Denial
returns a fail-closed `approval_denied` result immediately after delivery; it does not read later
events, report completion, produce verified-idle state, release the turn lock, start a second child,
replay the prompt, substitute exec, or change adapters.

Missing decisions preserve `interactive_control_required`. Malformed, duplicate, replayed, stale,
substituted, cross-session, cross-turn, cross-request, and post-disconnect decisions fail closed
without delivery or persistence. User-input requests retain their separate existing failure path and
never invoke the approval source. Fixture-only controller tests cover the exact call boundary,
approval/resolution continuation, terminal denial, missing/default behavior, correlation rejection,
consumer mutation, and the existing no-fallback/no-replay turn-lock behavior.

External local decision transport and user controls, durable rendering, cancellation controls,
fallback/rollback policy, the primary/default adapter flip, additional consumers, attestation, and
controlled live/cross-platform provider evidence remain deferred.

The bounded MCP stdio concurrency foundation is now complete without adding a decision operation.
One MCP server instance may own at most one asynchronous `dorkpipe.provider_pool_chat` request while
its fixed reader continues accepting frames and servicing ordinary requests such as `ping`. Every
other tool keeps the existing synchronous routing and tier gate. A second concurrent chat receives a
visible error with its exact JSON-RPC id before another DorkPipe runner or child can start; completion
releases the slot. Responses may complete out of request order, but one serialized writer preserves
each exact id and prevents Content-Length frames from interleaving.

The active chat inherits a server-owned cancellable context. Input EOF, framing/read failure, output
failure, or stdio shutdown closes the response writer first, cancels the active chat, and waits for
its handler to finish, so a canceled child cannot leak and no late response is written after closure.
At that foundation boundary, tests injected only a fixture runner at the MCP bridge and no approval
operation or production decision source was added. Provider-pool session binding, the unresolved-turn
lock, and all provider/adapter routing stayed authoritative.

The subsequent bounded MCP approval-transport slice is now complete. The exec-tier-only
`dorkpipe.provider_pool_approval_request` operation returns a defensive copy of the one exact neutral
`ApprovalRequest` currently owned by this MCP server's active chat. It exposes only the complete
opaque correlation, closed reason, and pinned allowed-decision list; repeated reads do not consume or
imply a decision. `dorkpipe.provider_pool_approval_decide` accepts one closed `ApprovalDecision`,
requires every correlation field, validates it through `ApprovalDecision.ValidateFor`, and delivers
it once to that same active chat. Missing, malformed, stale, replayed, duplicate, substituted,
cross-request, cross-turn, cross-session, cross-chat, cross-server, and post-shutdown decisions fail
before delivery.

The MCP bridge launches only this chat through an explicitly negotiated package-private stdio mode.
The DorkPipe child writes one versioned, bounded neutral request frame to its anonymous stderr pipe;
the bridge writes one versioned, bounded neutral decision frame to the child's anonymous stdin pipe;
the final provider-pool JSON remains isolated on stdout. Control frames contain only validated
provider-neutral projections. They contain no provider RPC id, command, prompt, patch, path,
credential, raw frame, provider payload, policy inference, or decision source metadata. No file,
package state, environment decision, polling path, socket, named pipe, listener, localhost endpoint,
or persisted queue participates.

The pending request belongs to the active-chat record, which owns the cancellable context, private
controller, exact runner, and same live child. Only one request can be pending at once. A submitted
decision is hidden from later reads immediately, acknowledged only after the exact child-stdin frame
is written, and cannot be replayed. Exact approval keeps the chat blocked in the existing dispatcher
until the provider's matching resolution arrives; denial keeps the existing terminal
`approval_denied` path. Child exit, frame/input/output failure, cancellation, MCP EOF, response-write
failure, or shutdown invalidates the pending request, cancels the child-side wait, joins the chat
handler, rejects later operations, and emits no late MCP response.

The HTTP JSON-RPC path used by Pipeon establishes the same server-owned active-chat record before
dispatch, so concurrent HTTP approval reads and decisions address the exact same child. A second HTTP
chat is rejected before a second runner starts, ordinary `codex_exec` HTTP chats retain their buffered
runner, and HTTP server shutdown cancels and joins the active chat before returning. This adds no
listener beyond the pre-existing authenticated MCP HTTP server.

Every other MCP tool remains synchronous and retains its prior tier. The normal CLI route and
zero-value provider-pool options still install no decision source. Nothing persists the request or
decision, and no fallback, prompt replay, second child, adapter substitution, policy inference, user
input response, cancellation control, or Pipeon rendering/control was added. The next single safe
slice is Pipeon rendering plus exact user approve/deny controls over this transient transport.

That Pipeon approval-control slice is now complete. Only a normal Codex chat explicitly pinned to
`codex_app_server` starts a per-invocation monitor against the existing authenticated MCP HTTP
server. It reads the existing approval-request operation on a fixed 125 ms cadence for at most ten
minutes and stops when it finds the request, the original chat settles, its request is aborted, or
the HTTP control transport fails. It never outlives or retargets the original Pipeon session/chat
invocation and never submits or implies a decision.

The complete neutral request remains in one extension-host-only registry keyed to that invocation.
The webview receives only a random transient UI reference, the closed reason, the exact allowed
decision set, and a closed UI delivery state. It renders fixed neutral copy and never receives or
renders correlation, provider content, commands, prompts, patches, paths, credentials, raw frames,
policy/model metadata, or connection details. The registry is not part of messages, `pendingAction`,
webview state, workspace/global state, diagnostics, telemetry, artifacts, or session serialization;
reload/restart restores nothing.

One click disables all rendered controls immediately. The extension host accepts it only when the
UI reference still belongs to the same live Pipeon session and invocation and the selected decision
is in the retained allowed set. It then submits the unchanged retained correlation plus that one
decision through `dorkpipe.provider_pool_approval_decide`. Duplicate, stale, substituted,
cross-session, cross-chat, and disallowed values stop before MCP. Confirmed delivery renders only
`Decision delivered; waiting for Codex` while the original chat remains busy until its exact final
response. Denial therefore renders the original fail-closed `approval_denied` result; no decision
leaves the chat blocked. An ambiguous decide failure disables the controls, renders one bounded
transport error, and performs no retry, opposite decision, replay, fallback, new chat, or new child.

Completion, denial, disconnect, failure, external cancellation, and MCP transport loss clear the
registry record and card. Ollama, Claude, `codex_exec`, `/codex`, prepared-edit actions, existing
serial chat behavior, normal CLI routes, and zero-value production routes remain unchanged. No
user-input, cancellation, recovery, remembered-decision, automatic-approval, or persistence control
was added.

The subsequent provider-pool user-input controller seam is now complete. One package-private
`providerPoolUserInputResponseSource` may be supplied only to the existing session-pinned App Server
dispatcher; normal CLI and zero-value production options supply none. On one exact correlated
`user_input_requested` event, the consumer requires the same live controller to remain
`waiting_for_user_input`, looks up and validates the complete neutral `UserInputPrompt`, retains an
immutable defensive copy, and invokes the source at most once with a separate defensive copy. The
source receives only correlation, opaque prompt/option references, closed prompt kind, bounded
summary/labels, and exact selection or text bounds.

Any returned `UserInputResponse` is copied and validated against the consumer's retained prompt
before one delivery through that controller's `RespondUserInput`. Source mutation cannot widen the
option set, selection count, text bound, prompt kind, correlation, or prompt reference. Delivery is
not completion: the unresolved-turn lock remains held and the event loop cannot reach terminal or
verified-idle handling until the exact matching `user_input_resolved` event returns the controller to
running. Missing source or `found=false` remains `interactive_control_required`; source, prompt,
response, delivery, replay, stale-state, and missing-resolution failures use bounded terminal classes
and never include prompt or answer content.

Prompt summaries, option labels/references, selected references, and text answers remain transient;
they enter no package state, snapshot, audit content, diagnostic, log, metadata, environment value,
file, or artifact. Approval requests remain exclusively on the unchanged approval source and decision
path. No MCP operation, Pipeon rendering/control, transport, listener, fallback, retry, replay, second
child, adapter substitution, or authority inference was added. The next single safe slice is transient
MCP user-input request/response transport over the existing active-chat ownership model, still without
Pipeon rendering or controls.

That transient MCP user-input transport is now complete. The closed exec-tier
`dorkpipe.provider_pool_user_input_request` operation returns a defensive, non-consuming copy of the
one exact normalized prompt pending for the same MCP server's one active provider-pool chat. The
closed exec-tier `dorkpipe.provider_pool_user_input_respond` operation accepts one complete neutral
response, validates it with `UserInputResponse.ValidateFor` against the server's immutable retained
prompt plus strict UTF-8/control-text checks, and returns `{"delivered":true}` only after one exact
write to the same live child is acknowledged. It does not report provider resolution, completion,
success, or verified idle; the original chat remains pending through its existing exact
`user_input_resolved`, terminal, and durable-idle sequence.

The explicitly negotiated MCP App Server route now installs one combined child-side interactive
transport over the existing anonymous stderr/stdin boundary. Approval frames retain the unchanged
`DORKPIPE_PRIVATE_APPROVAL_V1` class. User input uses the distinct
`DORKPIPE_PRIVATE_USER_INPUT_V1` class. Both are closed JSON and bounded to 64 KiB, ordinary stderr
remains ordinary stderr, and final provider-pool JSON remains on stdout. The user-input prompt frame
contains only the neutral prompt fields and the response frame only the neutral response fields;
neither can carry provider request IDs, raw/private provider content, commands, patches, paths,
credentials, adapter/policy/capability state, approval state, recovery/audit data, or prior responses.

The active-chat record owns separate approval and user-input controllers. A second prompt, second
chat, malformed or extended frame, invalid option/text answer, stale or substituted correlation,
duplicate/replayed response, cross-chat/server/process/session/turn/item/request/decision value,
child exit, cancellation, EOF, read/write/framing failure, MCP/HTTP/stdio transport loss, or shutdown
fails closed before another child delivery and clears the pending state. Shutdown joins the active
chat and suppresses late output. Prompt summaries, labels/references, selected references, and text
answers remain transient process memory and enter no package state, snapshot, audit, log, diagnostic,
telemetry, metadata, environment value, file, queue, or artifact. Approval behavior is unchanged;
neither request class can observe or answer the other. Normal CLI, zero-value options, `codex_exec`,
Ollama, and Claude acquire no response source. No Pipeon UI, automatic answer, default, retry,
fallback, replay, second child, adapter substitution, or authority inference was added. The next
single safe slice is Pipeon rendering and exact user controls for this transport.

That Pipeon user-input control slice is now complete. Only a normal Pipeon Codex chat explicitly
pinned to `codex_app_server` starts a second invocation-owned bounded monitor alongside the unchanged
approval monitor and original chat request. It polls only the existing authenticated
`dorkpipe.provider_pool_user_input_request` operation, treats only the exact no-prompt and no-active-
chat results as expected misses, and never consumes, answers, defaults, or infers a response. A
monitor transport failure aborts the original chat. Every terminal path aborts and joins both
monitors, then independently clears the approval and user-input registries.

The user-input registry is separate from approvals and binds one complete strictly normalized prompt
to the exact Pipeon session, chat invocation, provider-pool chat invocation, and random prompt UI
reference. Every selectable option receives a separate random UI reference mapped host-only to its
retained opaque option reference. The webview projection contains only the prompt UI reference,
closed kind, bounded display summary/labels, random option references, relevant bound, and closed
transport state. Correlation, `prompt_ref`, opaque option refs, provider/private data, and response
content never enter messages, current or pending-action state, webview state, workspace/global state,
logs, telemetry, files, queues, or artifacts.

Pipeon renders neutral cards for exact single choice, bounded multiple choice, and bounded text. No
selection is preselected. Single choice requires exactly one random option reference; multiple choice
requires one through the retained maximum and disables additional choices at that maximum; text uses
a UTF-8 byte counter and requires nonblank valid Unicode without NUL or control characters. The host
revalidates the exact prompt kind and current ownership, maps only random option references back to
the retained opaque refs, preserves accepted text unchanged, constructs one exact neutral response,
and calls `dorkpipe.provider_pool_user_input_respond` once. All controls disable on the first submit,
and response content is discarded after the call is constructed and sent.

Only an explicit `{"delivered":true}` acknowledgement produces `Response delivered; waiting for
Codex.` Delivery does not resolve the prompt, settle the still-busy chat, imply success or idle,
satisfy approval, or authorize recovery. Duplicate, stale, cross-session, cross-chat,
cross-invocation, cross-UI-reference, wrong-kind, unknown-option, duplicate-option, and excessive
selection submissions fail before MCP. Ambiguous delivery enters permanent `transport_error`, aborts
the original chat, and performs no retry, fallback, replacement response, chat, or child. Completion,
failure, denial, cancellation, transport loss, teardown, view disposal, reset, and extension reload
leave no replayable prompt or response state.

Approval operations, normalization, registry, monitor, acknowledgement, retry posture, ownership, and
denial semantics remain unchanged and fixture-green. Slash commands, `codex_exec`, Ollama, Claude,
normal CLI, and zero-value routes create no user-input registry or monitor. This remains package-owned
Pipeon tooling over the existing MCP transport; no engine, workflow, schema, listener, endpoint,
provider-pool, supervisor, provider-session, cancellation, or recovery behavior changed. After this
slice, the smallest remaining fixture-only TASK-013 gate is a package-private provider-pool
cancellation-controller seam; Pipeon cancellation rendering and controls remain a later independent
slice.

That package-private provider-pool cancellation-controller seam is now complete. One optional
`providerPoolCancellationIntentSource` belongs only to the existing session-pinned App Server
consumer. It starts at most once, and only after the consumer validates the normalized correlated
`turn_started` progress event for the current provider session. Its private scope contains only a
defensive copy of the neutral session reference and exact process-incarnation, connection, session,
and active-turn correlation. A chat-owned child context cancels and joins the concurrent source wait
on completion, failure, parent cancellation, event-channel closure, or teardown. `found=false`
disables that wait and leaves the original chat unchanged; source failure returns only a bounded
unknown-outcome class and is never retried.

A supplied intent is defensively copied, validated through `CancellationIntent.Validate`, rebound to
that immutable active-turn scope, restricted to the three existing closed reasons, and accepted only
while the same controller remains exactly `running` with no approval, user-input, terminal,
disconnect, or prior-cancellation state pending. The consumer calls the same controller's existing
neutral `Cancel` operation once. Delivery is not completion: the original chat and unresolved-turn
lock remain active until the exact same-controller `cancellation_requested` projection, optional one
bounded `background_process_risk_possible` progress event, and exact correlated
`state_changed / cancelled / cancelled` terminal arrive in order. Only that terminal produces the
bounded cancelled result, which never sets verified idle or recovery evidence.

Malformed, incomplete, stale, substituted, cross-process, cross-connection, cross-session,
cross-turn, replayed, or unknown-reason intents stop before delivery. Missing, malformed, duplicate,
reordered, or mismatched cancellation acknowledgement/progress/terminal events; non-cancelled
terminals; controller drift or delivery failure; source/context failure; and transport closure all
fail closed without exposing intent, correlation, provider, process, command, or background-process
details. Approval and user-input sources remain separate and unchanged. Normal CLI, zero-value
options, `codex_exec`, Ollama, Claude, approval-only and user-input-only fixtures, workflows, and
bounded workers install no cancellation source. No transport, MCP operation, listener, Pipeon
control, retry, replay, fallback, recovery, persistence, or engine behavior was added. With this
consumer dependency complete, the smallest remaining TASK-013 gate is transient MCP cancellation
request/delivery over the existing active-chat ownership and private interactive relay; Pipeon
cancellation rendering and controls remain a later independent slice.

That transient MCP cancellation transport is now complete. The closed exec-tier
`dorkpipe.provider_pool_cancellation_request` operation accepts only `{}` and returns a defensive,
non-consuming copy of the exact neutral session/active-turn scope pending for this MCP server's one
active provider-pool chat. The closed exec-tier
`dorkpipe.provider_pool_cancellation_deliver` operation accepts one exact neutral
`CancellationIntent`, validates it against the immutable retained scope and the three existing
reasons, and returns only `{"delivered":true}` after the exact intent frame is written to the same
live child. That acknowledgement does not claim controller acceptance, interruption,
`cancellation_requested`, terminal cancellation, completion, readiness, verified idle, or recovery;
the original consumer seam retains all acknowledgement and terminal authority.

The active-chat record owns a third transient controller independent from approval and user input.
It retains at most one scope containing only `SessionRef` plus process-incarnation, connection,
session, and active-turn correlation, hides it immediately after one valid intent submission, and
persists neither scope nor intent. The explicitly negotiated App Server child route alone installs
the matching source. Its `DORKPIPE_PRIVATE_CANCELLATION_V1` closed `cancellation_scope` and
`cancellation_intent` frames share the existing bounded anonymous stderr/stdin relay but never
ordinary stderr or final stdout.

The child now has one response reader/demultiplexer for approval, user-input, and cancellation plus
serialized request writes, so its long-lived cancellation wait cannot consume or block another
controller's response. The bridge registers the cancellation scope synchronously, waits for the MCP
intent in one chat-owned worker, continues scanning ordinary stderr and approval/user-input frames,
and serializes all child-stdin responses. Duplicate, replayed, stale, substituted, cross-process,
cross-connection, cross-session, cross-turn, cross-chat/server, malformed, post-completion, and
post-shutdown intents fail before another delivery. Child exit, EOF, context cancellation, relay or
write failure, MCP/HTTP/stdio loss, and server shutdown invalidate and join cancellation state with
no retry, replay, fallback, replacement child, terminal polling, or event streaming. Approval and
user-input state machines remain separate and fixture-green; normal CLI, zero-value, `codex_exec`,
Ollama, Claude, workflows, and bounded workers still install no cancellation source. The smallest
remaining TASK-013 gate is Pipeon cancellation monitoring with transient host-only intent ownership
and one explicit cancel control over these two MCP operations; it remains a separate slice.

That Pipeon cancellation control is now complete. Only one normal direct Pipeon Codex chat explicitly
pinned to `codex_app_server`, and already owning the approval and user-input controls, starts the third
monitor in the same invocation-owned wrapper. It polls only
`dorkpipe.provider_pool_cancellation_request` at 125 ms for at most ten minutes, treats only the exact
no-scope and no-active-chat results as expected misses, and stops on scope discovery, chat settlement,
invocation end, abort, teardown, or disposal. A monitor failure creates only a cancellation
`transport_error` projection and neither aborts the original chat nor changes approval or user-input
state.

The independent host-only registry binds one strictly normalized defensive cancellation scope to the
exact Pipeon session, chat invocation, shared provider-pool chat invocation, and one cryptographically
random UI reference. Process-incarnation, connection, provider-session, and turn correlation remain
host-only; activity, request, and decision scope must be empty. The webview receives only the random
reference and one of `pending`, `submitting`, `delivered`, or `transport_error`. It renders one neutral
`Cancel this Codex turn` card, and one click emits only that UI reference and disables the control.

The host rebinds the live ownership tuple, moves `pending` to `submitting`, constructs one unchanged
neutral intent with reason exactly `user_requested`, and calls
`dorkpipe.provider_pool_cancellation_deliver` once. Only exact `{"delivered":true}` renders
`Cancellation intent delivered; waiting for Codex.` This remains a child-frame-write acknowledgement,
not controller acceptance, interruption acknowledgement, terminal cancellation, completion, ready,
idle, or recovery evidence. The original provider-pool chat remains the only terminal authority and
its unresolved-turn posture is unchanged.

Timeout, HTTP/MCP failure, malformed or missing acknowledgement, aborted delivery, and transport loss
after submission permanently produce `transport_error`, discard the prepared intent, and never retry,
replay, choose another reason, abort the original chat, or use approval/user-input as a substitute.
Duplicate, stale, substituted, cross-session, cross-chat, cross-invocation, and cross-reference actions
fail before MCP. Completion, cancellation, denial, failure, disconnect, teardown, reset, session
removal, view disposal, and extension reload clear the scope and UI reference. Normal/default adapter,
`codex_exec`, Ollama, Claude, slash-command, workflow, prepared-action, and bounded-worker routes remain
unchanged. No engine, workflow, schema, listener, endpoint, event/status polling, recovery, persistence,
or provider contract changed.

The bounded provider-pool readiness-classification and one-shot fallback slice is now complete. One
closed package-private classification marks only failures proven to occur before any `turn/start`
attempt as eligible. The provider-pool coordinator consumes only that class, removes only the newly
created App Server `.lock` whose canonical schema, Pipeon session ID, and exact turn number all match,
then submits the unchanged original prompt once through the existing governed buffered `codex_exec`
route. A first-turn fallback creates no App Server session file. Recovered fallback retains the prior
verified-idle session file byte-for-byte. Missing, malformed, extended, substituted, stale,
mismatched, or unremovable claims block exec; the persisted adapter binding remains
`codex_app_server`.

The classification changes to dispatched-or-unknown before `StartPromptTurn` is called, so a start
error and every later consumer failure retain the unresolved-turn claim and remain no-replay. Neither
App Server nor exec is retried, and an exec failure or ambiguous result starts no fallback, replay,
or second child. Local fixtures cover setup, baseline-policy and supervisor construction,
initialization, model/policy selection, thread creation, verified-idle recovery, exact first and
recovered claims, every rejected claim class, removal failure, turn-start ambiguity, post-start
  consumer failure, successful App Server dispatch, direct `codex_exec`, adapter pinning/drift, and
  ambiguous exec completion. Interactive monitors remain limited to an explicitly retained normal
  direct App Server chat. No public/provider-neutral/MCP contract gained fallback classification or
  rollback details. Durable rendering/retention, operations evidence, and cross-platform acceptance
  remain incomplete.

The bounded Pipeon default-and-session-pinning slice is now complete. The resource-scoped manifest
default and the extension fallback are exactly `codex_app_server`. Each new Pipeon chat session
captures the currently configured supported adapter once; an explicit `codex_exec` setting remains
the escape hatch for later new sessions. The host persists that closed choice with the session, sends
it unchanged on every Codex turn, and never re-reads configuration for an existing session. Stored
`codex_exec` and `codex_app_server` choices survive reload and later configuration changes
byte-for-byte.

For a legacy stored session with no retained field, Pipeon inspects explicit resource, workspace, and
user configuration provenance. A supported explicit value is retained exactly; without one, the
session is migrated once to the historical `codex_exec` default. Empty, unknown, malformed,
extended, non-string, omitted post-migration, or substituted retained evidence fails before MCP.
The provider-pool persisted binding remains the final drift check and is neither migrated nor
rewritten. Only a retained `codex_app_server` choice on normal direct Codex chat starts the approval,
user-input, and cancellation monitors; `codex_exec`, slash commands, other providers, workflows,
prepared actions, and bounded workers start none. The proven pre-`turn/start` fallback boundary is
unchanged, neither adapter is retried, ambiguous prompts are never replayed, and no second child is
started. No UI, public/provider-neutral/MCP contract, durable transient control state, or adapter
diagnostic was added. Durable event rendering/retention, operations evidence, and cross-platform
acceptance remain incomplete; TASK-013 and CAS-14 remain open.

The bounded Pipeon post-turn snapshot slice is now complete. A normal direct Codex chat whose
retained adapter is exactly `codex_app_server` derives one host-owned closed display snapshot only
from that retained adapter, the existing closed provider-pool response state, and the existing safe
App Server response metadata. The persisted record contains exactly the App Server adapter, one of
`completed`, `failed`, `cancelled`, or `recovery_required`, the exact `outcomeUnknown` boolean, an
optional bounded terminal-summary identifier, and optional bounded opaque model, reasoning,
approval, and sandbox references when supplied. Missing policy evidence remains absent. Empty,
unknown, malformed, extended, substituted, oversized, non-string, or adapter-mismatched persisted
snapshot evidence fails closed.

The webview receives a separate allowlisted display record with no `codexSessionAdapter`, raw
provider metadata, correlation, prompt, response, control, fallback, or provider-pool binding. A
snapshot with `outcomeUnknown: true` survives reload as `recovery_required` and renders only the
persistent neutral warning that the outcome is unknown and recovery is required; it makes no
completion, cancellation, ready, idle, recovered, or retry claim. A session without snapshot
evidence remains valid and renders no App Server status. Clearing ordinary messages conservatively
retains both the session-pinned adapter and the last post-turn snapshot because message deletion is
not lifecycle evidence. Later configuration changes affect only later new sessions and alter neither
field on an existing session.

This snapshot remains Pipeon host/display state and adds no provider-neutral or MCP authority.
Provider-pool responses and bindings, adapter pinning and legacy migration, transient approval,
user-input and cancellation monitors, one-shot pre-turn fallback, exact rollback, retry, and
no-replay behavior are unchanged. Live event streaming or cursors, polling, replay, full durable
event history, operations evidence, and cross-platform acceptance remain incomplete; TASK-013 and
CAS-14 remain open.

The bounded Pipeon recovery-required turn-start guard slice is now complete. For the existing target
session only, a normal direct non-slash Codex request whose retained adapter is exactly
`codex_app_server` and whose valid retained snapshot is exactly `recovery_required` with
`outcomeUnknown: true` now returns before logging or changing active turn state. It adds no user
message, assistant placeholder, interactive monitor, provider-pool arguments, MCP call, claim,
fallback, replay, retry, pending action, durable record, or child. Repeated blocked attempts remain
inert. The retained adapter, snapshot, messages, and persistent unknown-outcome warning remain
unchanged across blocked attempts, ordinary-message clearing, and later configuration changes.

Completed, failed, and cancelled snapshots and sessions without snapshot evidence do not activate
the guard. Slash commands, explicit `codex_exec`, Ollama, Claude, workflows, prepared actions, and
bounded workers remain unaffected and gain no App Server snapshot. The host guard is authoritative
for stale or directly posted `ask` requests. The webview uses only its existing allowlisted
`appServerStatus` projection to disable the existing Send action for ordinary Codex text while
leaving the prompt editable; a slash draft or another provider re-enables Send without changing the
retained status. No reconciliation or recovery action, new transport, provider-neutral or MCP
authority, live event stream, cursor, polling, retry, ambiguous replay, or second child was added.
Live event streaming/cursors, full durable event history, reconciliation, operations evidence, and
cross-platform acceptance remain incomplete; TASK-013 and CAS-14 remain open.

The bounded provider-pool recovery-candidate classification slice is now complete. One closed
package-private classification is produced only for a valid exact Pipeon session ID whose retained
adapter is exactly `codex_app_server`, whose canonical persisted adapter binding has the supported
schema and matches that session and adapter, whose canonical persisted App Server session state
passes the existing strict loader with a valid provider-session ID, exact bounded recovery evidence,
accepted retained model/reasoning values, and a nonzero non-exhausted completed-turn counter, and
whose canonical bounded companion claim matches the same session with `pending_turn` exactly one
greater than `completed_turn`. The classifier rereads the same bounded files before returning the
candidate classification and repeated calls leave the entire package-state tree byte-for-byte
unchanged.

Missing, malformed, extended, duplicated, reordered, substituted, stale, oversized, partial,
trailing, cross-session, cross-adapter, and cross-turn evidence fails closed. So do `codex_exec`, an
omitted or unknown adapter, invalid session IDs, first-turn uncertainty without prior verified-idle
state, completed state without a claim, a claim without completed state, zero or exhausted counters,
and any claim other than the exact next turn. Configuration, display snapshot/status, terminal
summary, messages, provider availability, response text, catalog order, authentication state, and a
bare `.lock` file supply no authority.

Classification starts no supervisor, child, provider, prompt, exec, MCP, monitor, fallback, replay,
retry, interactive controller, or second child and performs no reconciliation, claim removal,
snapshot transition, durable write, or new turn. Candidate status says only that exact authoritative
evidence exists for a later separately authorized recovery attempt; it does not classify the unknown
prompt as completed, failed, cancelled, idle, recovered, replayable, or safe to continue. Pipeon
therefore remains blocked until a later separately authorized recovery operation proves its required
state and performs an explicitly designed atomic transition. No CLI, MCP, provider-neutral, Pipeon,
workflow, schema, public contract, recovery action, or UI control was added. Reconciliation, recovery
control, atomic claim/status transition, live event streaming/cursors, full durable event history,
operations evidence, and cross-platform acceptance remain incomplete; TASK-013 and CAS-14 remain
open.

The bounded provider-pool recovery-only host-operation slice is now complete. The package-private
operation first requires the exact recovery-candidate classification, strictly reloads the retained
App Server state, revalidates the candidate and unchanged state again before construction, and uses
the retained provider-session ID, recovery evidence, model, and reasoning values without
substitution. It constructs exactly one fresh App Server supervisor through the existing
package-private factory, invokes the existing `RecoverBaseline` exactly once, and always shuts that
supervisor down after the attempt whether recovery succeeds or fails. Only the initialization,
model/policy revalidation, and private idle `thread/read` reconciliation already owned by
`RecoverBaseline` are permitted; the host operation has no prompt input and never invokes
`StartPromptTurn`.

The successful value is only a closed, non-authorizing observation that the existing
`RecoverBaseline` contract returned its required state during that one attempt. It does not authorize
continued chat, claim removal, snapshot clearing, replay, retry, completion, cancellation, a new
turn, or any provider-pool/Pipeon transition. Constructor, recovery, and shutdown failures remain
fail closed and cause no fallback, retry, replay, exec, second supervisor, or second child. Exact
fixtures prove one constructor call, one `RecoverBaseline` call, shutdown after success and failure,
no prompt dispatch or lifecycle start outside `RecoverBaseline`, and byte-for-byte preservation of
the adapter binding, completed session state, unresolved claim, and Pipeon display evidence. The
existing Pipeon `recovery_required` unknown-outcome guard, messages, warning, and Send state remain
unchanged.

No CLI, MCP, provider-neutral, Pipeon, workflow, schema, public contract, recovery control, or UI
surface was added. The atomic claim/status transition, explicit recovery control, live event
streaming/cursors, full durable event history, operations evidence, and cross-platform acceptance
remain incomplete; TASK-013 and CAS-14 remain open.

The bounded atomic-transition contract investigation is now complete and blocked without a
production transition. Its authoritative pre-state is split between the provider-pool package's
canonical adapter-binding file, completed App Server session-state file, and consecutive unresolved
claim file, and Pipeon's separately persisted workspace chat session containing the retained
`codex_app_server` adapter plus exact `recovery_required` / `outcomeUnknown: true` snapshot. The
successful recovery-only observation remains bound operationally to the same session, provider
session, recovery evidence, model, reasoning, completed turn, and pending turn that were strictly
reloaded before `RecoverBaseline`, but its closed success value is intentionally not terminal-outcome
evidence and is not an authorization or a durable compare-and-swap token.

No existing transaction covers those records. Adapter first-use uses exclusive creation, the claim
uses a separate exclusive file, verified-idle completion atomically renames only the App Server
session-state file and then separately removes the claim, and Pipeon persists its chat state through
its own workspace-state owner. Therefore neither claim-first nor Pipeon-first mutation can be made
all-or-nothing across restart or concurrent processes. Fixture-only injected-failure evidence proves
both rejected orderings: claim-first leaves the persistent Pipeon guard in place after recovery
candidate evidence has been destroyed, and Pipeon-first removes the turn-start/Send guard while the
exact unresolved claim still exists. Reload observes those partial mixtures directly; binding and
completed-state bytes remain unchanged, so neither ordering can be reinterpreted as a complete
provider-pool commit.

The existing Pipeon status vocabulary also offers no truthful post-state: `completed`, `failed`, and
`cancelled` would invent the unknown prompt outcome, while removing `recovery_required` would falsely
authorize continuation. Implementation requires one explicit product/storage decision that both
defines a non-terminal post-reconciliation meaning which preserves the unknown outcome and selects
one durable transaction owner capable of comparing and committing the binding, completed state,
claim, and Pipeon guard together. Until then, exact replay, concurrency, restart, and injected commit
failure must remain closed by leaving the complete old state intact. This slice added no production
CAS, transition record, consumer, control, transport, prompt, recovery call, provider dispatch, exec,
MCP, monitor, fallback, retry, supervisor, child, public contract, schema, or Pipeon change. TASK-013
and CAS-14 remain open.

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

**Dependency pin and test-only Windows native smoke evidence — 2026-08-04.** This bounded slice pins
`modernc.org/sqlite v1.56.0` and `modernc.org/libc v1.74.4` in
`packages/dorkpipe/lib/go.mod` / `go.sum` and adds only `_test.go` files under
`appserversupervisor/sqliteevidence`. The package remains opt-in through
`DORKPIPE_SQLITE_EVIDENCE=1`; ordinary package regression runs skip the native host probe. The module
directive remains Go `1.25`. The Windows-only DACL/volume evidence directly uses the selected graph's
`golang.org/x/sys v0.47.0`, replacing the prior indirect `v0.28.0`; no other pre-existing selected
module version changed. No `go mod tidy` rewrite was accepted because its proposed checksum cleanup
removed unrelated historical entries.

With `GOWORK=off`, the exact resolved non-main module graph contains 32 entries:

```text
dockpipe v0.0.0 => ../../..
github.com/dustin/go-humanize v1.0.1
github.com/google/pprof v0.0.0-20260802141513-ef3492d7dac3
github.com/google/uuid v1.6.0
github.com/hashicorp/golang-lru/v2 v2.0.7
github.com/lib/pq v1.10.9
github.com/mattn/go-isatty v0.0.24
github.com/mattn/go-shellwords v1.0.12
github.com/ncruces/go-strftime v1.0.0
github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec
github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
golang.org/x/mod v0.37.0
golang.org/x/sync v0.21.0
golang.org/x/sys v0.47.0
golang.org/x/term v0.27.0
golang.org/x/tools v0.47.0
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405
gopkg.in/yaml.v3 v3.0.1
modernc.org/cc/v4 v4.29.1
modernc.org/ccgo/v4 v4.34.6
modernc.org/fileutil v1.4.0
modernc.org/gc/v2 v2.6.5
modernc.org/gc/v3 v3.1.4
modernc.org/goabi0 v0.2.0
modernc.org/libc v1.74.4
modernc.org/mathutil v1.7.1
modernc.org/memory v1.11.0
modernc.org/opt v0.2.0
modernc.org/sortutil v1.2.1
modernc.org/sqlite v1.56.0
modernc.org/strutil v1.2.1
modernc.org/token v1.1.0
```

Every one of those 32 module directories exposed a root license/notice file. The complete scan found
only the repository's existing Apache-2.0 dependency plus permissive Apache-2.0, BSD-style, MIT, and
dual MIT/Apache terms; no module lacked license material. The two selected modernc modules each carry
their BSD-style three-clause license.

The native smoke passed on this exact host and toolchain:

- Windows build `10.0.26200`, `amd64`, Go `go1.26.4`, module language baseline Go `1.25`, and
  `CGO_ENABLED=0`;
- fixed local drive, filesystem `NTFS`, volume
  `\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\`, serial `88c9a133`, label `OS`; the optional
  unprivileged NTFS-version query was unavailable, so no NTFS version is claimed;
- one canonical absolute test-framework fixture root with a protected DACL; the root, pre-created
  empty main/other databases, and every observed journal were owned by current-user SID
  `S-1-5-21-2729925100-2499202611-1015899381-1002` and granted full access only to that SID and
  `SYSTEM` (`S-1-5-18`).

The exact queried runtime identity was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`.
The test opened one absolute `file:` URI per database with `mode=rw`, `cache=private`, `vfs=win32`,
`_txlock=exclusive`, `_dqs=0`, and `_error_rc=1`; an unresolved double-quoted string was rejected,
proving DQS remained disabled. The bounded compile-option record contained exactly 57 entries:

```text
ATOMIC_INTRINSICS=1,COMPILER=gcc-12-win32,DEFAULT_AUTOVACUUM,DEFAULT_CACHE_SIZE=-2000,
DEFAULT_FILE_FORMAT=4,DEFAULT_JOURNAL_SIZE_LIMIT=-1,DEFAULT_MEMSTATUS=0,DEFAULT_MMAP_SIZE=0,
DEFAULT_PAGE_SIZE=4096,DEFAULT_PCACHE_INITSZ=20,DEFAULT_RECURSIVE_TRIGGERS,
DEFAULT_SECTOR_SIZE=4096,DEFAULT_SYNCHRONOUS=2,DEFAULT_WAL_AUTOCHECKPOINT=1000,
DEFAULT_WAL_SYNCHRONOUS=2,DEFAULT_WORKER_THREADS=0,DIRECT_OVERFLOW_READ,DISABLE_INTRINSIC,
ENABLE_COLUMN_METADATA,ENABLE_DBPAGE_VTAB,ENABLE_DBSTAT_VTAB,ENABLE_FTS5,ENABLE_GEOPOLY,
ENABLE_MATH_FUNCTIONS,ENABLE_MEMORY_MANAGEMENT,ENABLE_OFFSET_SQL_FUNC,ENABLE_PREUPDATE_HOOK,
ENABLE_RBU,ENABLE_RTREE,ENABLE_SESSION,ENABLE_SNAPSHOT,ENABLE_STAT4,ENABLE_UNLOCK_NOTIFY,
LIKE_DOESNT_MATCH_BLOBS,MALLOC_SOFT_LIMIT=1024,MAX_ATTACHED=10,MAX_COLUMN=2000,
MAX_COMPOUND_SELECT=500,MAX_DEFAULT_PAGE_SIZE=8192,MAX_EXPR_DEPTH=1000,MAX_FUNCTION_ARG=1000,
MAX_LENGTH=1000000000,MAX_LIKE_PATTERN_LENGTH=50000,MAX_MMAP_SIZE=0x7fff0000,
MAX_PAGE_COUNT=0xfffffffe,MAX_PAGE_SIZE=65536,MAX_SQL_LENGTH=1000000000,
MAX_TRIGGER_DEPTH=1000,MAX_VARIABLE_NUMBER=32766,MAX_VDBE_OP=250000000,
MAX_WORKER_THREADS=8,MUTEX_NOOP,OMIT_SEH,SOUNDEX,SYSTEM_MALLOC,TEMP_STORE=1,THREADSAFE=1
```

Every selected pragma was applied and read back on the same dedicated connection:

| Setting | Exact readback |
| --- | --- |
| `journal_mode=DELETE` | `delete` |
| `synchronous=EXTRA` | `3` |
| `fullfsync=ON` | `1` |
| `temp_store=MEMORY` | `2` |
| `mmap_size=0` | `0` |
| `busy_timeout=0` | `0` |
| `foreign_keys=ON` | `1` |
| `trusted_schema=OFF` | `0` |
| `cell_size_check=ON` | `1` |
| `locking_mode=EXCLUSIVE` | `exclusive` |
| pre-schema `page_size=4096` | `4096` |

The test created exactly the selected singleton STRICT table and `user_version=1`, rejected every
unexpected schema object/database/sibling, and kept canonical JSON opaque in the BLOB column. The
revision-1 insert used SHA-256
`5bacd33f5355f1a64a096841fe3fceeca28a40f211723e2ce4bb9b56988e6fe8`; the exact revision-2 CAS
used SHA-256 `37572e06825751539b2e65c19034a23950925abbbe795d296a52ecf1e6e2aca4`. Each commit reloaded
the exact singleton, session ID, revision, payload bytes, and digest through the same connection.
`PRAGMA database_list` contained only `main`.

The SQLite-created rollback journal was a regular file while each write transaction was live and
remained present after both commits at 4,616 bytes. It retained the exact current-user/`SYSTEM`
full-control boundary before and after commit, after forced termination, and after recovery. The test
never opened it for content, parsed it, or altered, truncated, moved, or deleted it; the database
directory contained only `aggregate.sqlite` and `aggregate.sqlite-journal`.

An independent owner child staged revision 3 and held the exclusive transaction. A second process
received primary `SQLITE_BUSY` (`5`) for the same database while a different-database process opened,
ran `quick_check`, and committed successfully. Forced owner termination released the first database
lock. A fresh recovery process opened the database, allowed SQLite recovery, returned exactly one
`quick_check=ok`, revalidated the exact schema, and reloaded the allowlisted old revision 2 (not the
uncommitted revision 3). Child processes performed no cleanup; only the parent test framework removed
the exact temporary root.

With `CGO_ENABLED=0`, the test-only package cross-compiled successfully to temporary binaries outside
the repository for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; embedded build settings confirmed
each `GOOS`, `GOARCH`, and `CGO_ENABLED=0`. The inspected binary sizes were 11,464,704 bytes,
11,044,675 bytes, and 10,943,842 bytes respectively. All binaries and the final unique temporary
directory `dockpipe-sqlite-cross-5028fe64ad384dbe8eb341a24d032556` were removed after inspection.
The Linux and macOS results are compile compatibility only; neither is runtime evidence.

The exact native smoke command passed. The required focused regression command separately failed in
protected predecessor work: `TestProtocolBoundaryContainsNoGenericOrPipeonProtocolLeak` observed 17
Pipeon adapter-selector occurrences in protected `extension.ts` while the protected
`protocol_test.go` expected 1. The full `go test ./... -count=1` command then exceeded its five-minute
execution bound without package output. A diagnostic rerun with a 90-second per-package timeout
confirmed the same App Server failure and a separate `orchestrationhelper` timeout in
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRevalidatesImmutableBindings`
while it blocked in the pre-existing reconciliation fixture's `os.ReadFile`; all other reported
packages passed, including `appserversupervisor/sqliteevidence`. Neither protected Pipeon/App Server
file is authorized for this slice, and both still match the accepted protected manifest; no
orchestration-helper file was changed. These validation results do not weaken or expand the
successful SQLite smoke evidence.

**Windows 10,000-cycle native reader-publication cohort — 2026-08-04.** The separate Windows-only
`TestWindowsNativeSQLitePublicationCohort`, gated by
`DORKPIPE_SQLITE_PUBLICATION_COHORT=1`, passed with `CGO_ENABLED=0`, `-mod=readonly`, and the fixed
30-minute timeout. It used the pinned SQLite 3.53.3/source-ID baseline, native `win32` VFS, selected
URI parameters, queried pragmas, singleton STRICT schema, `user_version=1`, fixed local NTFS volume,
and current-user/`SYSTEM` DACL contract already proved by the smoke lane.

One persistent writer child and one persistent reader child used a bounded strict JSON-line protocol.
Every command and response carried the exact cycle number; the reader opened a fresh connection for
each observation, and the writer opened one connection for each staged transaction and closed it
after commit and exact reload. Duplicate, missing, malformed, substituted, or out-of-order commands
and responses fail closed. Children never deleted fixture paths; only the parent test framework
cleaned the exact temporary root.

The exact aggregate result was:

- cycles: `10000`;
- successful pre-publication exact old reads: `10000`;
- live-owner primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `10000`;
- successful post-release exact new reads: `10000`;
- protected live-journal observations: `10000`;
- ambiguous or partial reads, revision gaps/duplicates, digest mismatches, and child-protocol loss,
  duplication, or reordering: `0`;
- initial revision/digest: `1` /
  `aa5cf90832cf7e71136cfa92208ef923e141d7d8103cab900f642ed02e50b3fb`;
- final revision/digest: `10001` /
  `3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838`;
- cohort elapsed time: `4m33.39s`.

Every live journal was a regular exact-basename sibling with the selected current-user/`SYSTEM`
full-control DACL, and no unexpected sibling appeared. The quiescent pre/post metadata-tree hash—over
ordinal relative path, entry type, size, and exact DACL evidence without opening or parsing journal
content—was stable and equal before and after the cohort:
`dd678add8ff983d5b8794ab62907ed89b3c162c32fa6d988a29a57e0462b0aaa`.

The existing native smoke rerun passed unchanged. With CGo disabled, the complete test-only package
cross-compiled with embedded target settings confirmed for Windows/`amd64`, Linux/`amd64`, and
macOS/`arm64`; binary sizes were 11,949,568, 11,044,675, and 10,943,842 bytes respectively, and the
temporary directory `dockpipe-sqlite-publication-cross-7d65b0ee4d0d4add952e5130031b3f78` was
removed. These cross-target builds are compile compatibility only.

The focused App Server/provider-session regression result matched the protected baseline exactly:
`providersession` passed and `TestProtocolBoundaryContainsNoGenericOrPipeonProtocolLeak` still found
17 protected Pipeon selector occurrences instead of 1. The bounded full `go test -mod=readonly ./...
-count=1 -timeout=90s` run reported that same App Server failure, and the protected
`orchestrationhelper` suite again timed out in the existing placement-execution graph fixture chain.
The exact timed-out subtest moved from the prior immutable-binding run to
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRejectsMalformedAndUnsafeArtifacts/receipt_noncanonical`
while decoding its pre-existing fixture; all other reported packages passed, including
`appserversupervisor/sqliteevidence`. No protected App Server, Pipeon, or orchestration-helper file
changed, and the timeout difference is not attributable to this isolated Windows-only test file.

**Windows 1,000-cycle native contention/forced-termination cohort — 2026-08-04.** The Windows-only
`TestWindowsNativeSQLiteContentionCohort`, gated by
`DORKPIPE_SQLITE_CONTENTION_COHORT=1`, passed with `CGO_ENABLED=0`, `-mod=readonly`, `-count=1`,
verbose output, and the fixed 30-minute timeout. It ran on Windows build `10.0.26200`, `amd64`, Go
`go1.26.4`, fixed NTFS volume `\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\` with serial
`88c9a133` and label `OS`; the unprivileged NTFS-version query remained unavailable. It revalidated
SQLite `3.53.3`, source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`,
the native `win32` VFS, all 57 compile options, the selected absolute URI and queried pragmas, the
singleton STRICT schema and `user_version=1`, and one database per synthetic session. The exact
current-user SID was `S-1-5-21-2729925100-2499202611-1015899381-1002`; the canonical temporary root,
both session directories and main files, and every observed journal granted full control only to
that SID and `SYSTEM` (`S-1-5-18`).

Each cycle used a fresh owner process that loaded the exact committed same-session row, began the
selected exclusive transaction, applied the exact next-revision CAS, validated the complete staged
row, and remained live after reporting `staged_live`. The parent observed the exact regular protected
journal, a fresh contender returned only primary `SQLITE_BUSY`/`SQLITE_LOCKED`, and a fresh independent
different-session writer validated, committed, reloaded, integrity-checked, and closed its different
database while the first owner remained live. The parent then killed the owner before any commit
command existed. A fresh recovery-only process allowed hot-journal recovery, required exactly one
`quick_check=ok`, revalidated schema, database identity, protections, and siblings, and returned the
exact old row rather than the killed owner's staged row. A separate fresh clean writer then committed
that same-session next revision exactly once. Deterministic opaque canonical BLOBs carried exact
adapter, session, revision, unknown-outcome, and permanent no-replay values; complete row and SHA-256
equality was required at every boundary. Children never deleted or altered journals or fixture paths;
only the parent test framework owned temporary-root cleanup.

The exact aggregate result was:

- owner transactions staged: `1000`;
- protected live journals: `1000`;
- same-session primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `1000`;
- different-session commits while the owner remained live: `1000`;
- forced owner terminations before commit invocation: `1000`;
- exact old-row recoveries: `1000`;
- successful post-recovery same-session commits: `1000`;
- ambiguous recoveries, staged-row leaks, revision gaps/duplicates, digest/envelope mismatches,
  unexpected siblings/protection widening, and child-protocol loss/duplication/reordering: `0`;
- same-session initial revision/digest: `1` /
  `bb0b0fa448e6532a65b420e128470a70fe5e32e15e94634b8c4fcf64a0b1e5ed`;
- same-session final revision/digest: `1001` /
  `e024c4e5dafc3841e26abbc2df7618f2fd78fcabd3b41bd364485b2ad56ff693`;
- different-session initial revision/digest: `1` /
  `8b351be57c3b6f86535ca6c2c3f6ef159175513013f7ac6608413e4e411dedfe`;
- different-session final revision/digest: `1001` /
  `5c78a969b9d47e07ef749d58c6b0fa3311512435141191d79376dd50e2f62f26`;
- elapsed time: `3m19.25s`.

The stable quiescent pre/post metadata-tree hash was equal:
`9e4b6e98a9ce839c24ee20cb21f56ecc379eff03133782b593fb10b936e511b8`. The hash is SHA-256 over
LF-terminated rows sorted by ordinal relative path; each row contains relative path, entry type,
byte size, and exact owner/DACL evidence. It opens and hashes no journal content.

The unchanged 10,000-cycle publication cohort rerun passed in `4m19.979s` with its existing counters,
initial/final revisions and digests, and equal metadata-tree hash
`dd678add8ff983d5b8794ab62907ed89b3c162c32fa6d988a29a57e0462b0aaa`. The unchanged native Windows
smoke passed. The complete test package cross-compiled with embedded target settings confirming
`CGO_ENABLED=0` for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; binary sizes were 12,056,064,
11,044,675, and 10,943,842 bytes. The verified temporary directory
`dockpipe-sqlite-contention-cross-da2f4b7dfff54e789acf71baa98b4890` was removed.

The focused regression again passed `providersession` and failed only the protected
`TestProtocolBoundaryContainsNoGenericOrPipeonProtocolLeak` assertion at 17 occurrences instead of 1.
The bounded full suite reported that same failure, passed `appserversupervisor/sqliteevidence`, and
again timed out in the protected orchestration-helper reconciliation fixture chain. This run's exact
active subtest moved from the preceding `receipt_noncanonical` case to
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRequiresExactAuthorization/inferred_decision`,
blocked in the same pre-existing `os.ReadFile` path. `go mod verify` returned `all modules verified`;
`gofmt -d` for the new file was empty, and `git diff --check` passed. No protected predecessor file
was edited.

The exact validation commands were run from `packages/dorkpipe/lib` (environment assignments shown
in portable prefix form):

```text
DORKPIPE_SQLITE_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
DORKPIPE_SQLITE_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
DORKPIPE_SQLITE_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteSmoke$' -count=1 -v
CGO_ENABLED=0 GOOS=<windows|linux|darwin> GOARCH=<amd64|amd64|arm64> go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
go version -m <verified-temporary-binary>
go test -mod=readonly ./appserversupervisor ./providersession -count=1
go test -mod=readonly ./... -count=1 -timeout=90s
go mod verify
gofmt -d appserversupervisor/sqliteevidence/windows_contention_cohort_test.go
git diff --check
```

**Windows native deterministic SQLite failure-boundary matrix — 2026-08-04.** The new Windows-only
`TestWindowsNativeSQLiteFailureBoundaryMatrix`, gated by
`DORKPIPE_SQLITE_FAILURE_MATRIX=1`, passed with `CGO_ENABLED=0`, `-mod=readonly`, `-count=1`, verbose
output, and the fixed 30-minute timeout. It ran on Windows build `10.0.26200`, `amd64`, Go
`go1.26.4`, fixed NTFS volume `\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\` with serial
`88c9a133` and label `OS`; the unprivileged NTFS-version query remained unavailable. The canonical
temporary root and every scenario directory, main database, and observed journal were owned by
current-user SID `S-1-5-21-2729925100-2499202611-1015899381-1002` and granted full control only to
that SID and `SYSTEM` (`S-1-5-18`).

Every fresh per-attempt database revalidated SQLite `3.53.3`, source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`,
native `win32`, the selected absolute `mode=rw` / `cache=private` / `_txlock=exclusive` / `_dqs=0` /
`_error_rc=1` URI, every selected pragma and exact readback, the singleton STRICT schema and
`user_version=1`, and the exact 57-entry compile-option set listed above. The LF-terminated exact-set
SHA-256 was `e08918d66caa484a6929317e24d92db1d6a078fc115dbf15adb10994f869babf`.
Each database held exactly one bounded canonical JSON BLOB with adapter, session, revision,
unknown-outcome, permanent-no-replay, envelope, and SHA-256 equality. Child processes used a strict
bounded JSON-line protocol on `stderr`, isolated from Go test output on `stdout`; every command and
response carried exact scenario, cycle, attempt, operation, checkpoint, database, and session
identity. Missing, duplicate, substituted, cross-scenario, malformed, or out-of-order traffic failed
closed. Children performed only ordinary SQLite operations and never cleaned or altered physical
store files.

The complete 22-attempt result table follows. `Harness` means the named application checkpoint or
response loss was injected and is not a SQLite/OS result. `Native` means the recorded outcome was
genuinely returned by SQLite or Windows process termination/recovery. Each row began at exact
revision `1`; the table binds its session-specific initial and final digest exactly:

| Scenario | Attempt and boundary | Evidence kind | Authoritative classification | Initial revision / SHA-256 | Final physical revision / SHA-256 |
| --- | --- | --- | --- | --- | --- |
| `01_before_open` | 1; reject before database open | Harness | rejected | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` |
| `02_contract_reject` | 1; substituted contract evidence before observation | Harness | rejected | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` |
| `03_contention` | 1; same-session lock before observation | Native primary `SQLITE_BUSY` (`5`) | rejected | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` |
| `04_cancel_after_observation` | 1; cancellation after exact old observation | Harness plus genuine rollback/reload | rejected | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` |
| `05_stale_cas` | 1; stale session | Native zero rows plus rollback/reload | known unchanged | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` |
| `05_stale_cas` | 2; stale revision | Native zero rows plus rollback/reload | known unchanged | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` |
| `05_stale_cas` | 3; stale digest | Native zero rows plus rollback/reload | known unchanged | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` |
| `06_after_begin` | 1; injected loss after begin before CAS | Harness plus genuine rollback/reload | known unchanged | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` |
| `07_after_stage` | 1; injected loss after exact CAS staging before commit | Harness plus genuine rollback/reload | known unchanged | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` |
| `08_terminate_precommit` | 1; forced termination after staging before commit | Native termination/hot-journal recovery | known unchanged | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` |
| `09_rollback_proof_loss` | 1; forced loss prevents rollback/old-row proof | Harness loss plus later native physical recovery | `unknown_commit_result` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` |
| `10_commit_call_loss` | 1; forced termination from inside SQLite's commit hook after the write-transaction and exclusive-lock checks, before commit phase one or result availability | Native SQLite commit-hook observation plus harness termination | `unknown_commit_result` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` |
| `11_genuine_commit_error` | 1; genuine commit error attempt | Proven unreachable under selected exclusive shape; control commit genuinely succeeded | committed | 1 / `538c917b0fc4360a9f1337b5f04a3f0baf9e7436adcc21e1f6374e679587216e` | 2 / `7c0dd65cc2a2850d7fb6dfde8e3bd9142cf26503b74ddcc575fc9c170588e4d8` |
| `12_response_loss` | 1; genuine commit success, caller response lost before reload | Harness | `unknown_commit_result` | 1 / `32097cd580bc4bb23bbdb3a84dc6ea953d9233840965514258386a3bf66e5410` | 2 / `fa586651472644b528aff5c042e98cc78284b0508048e4d63d124de173ff895d` |
| `13_validation_loss` | 1; schema validation result lost after exact reload | Harness | `unknown_commit_result` | 1 / `604abe29c761b3c1f6fd4d303e2d59e2b97414dac714d3d361a76d28124af792` | 2 / `53f1d118dc76593ec110233467c23e292fe769bf480317957cfc908be043a214` |
| `13_validation_loss` | 2; identity validation result lost | Harness | `unknown_commit_result` | 1 / `95f5a71c86ed8215865cf6880460dc1b954aebfde94d882b4378db91f7379eed` | 2 / `37ba916c7c427488bb0482f5e9490a176a1ec91157a74028748df1c5240fc7e5` |
| `13_validation_loss` | 3; digest/envelope validation result lost | Harness | `unknown_commit_result` | 1 / `e2351ed2ad87340ac3671ad728083354787307b52e0b021f441ea31f506f2972` | 2 / `0790b0985f1a1d5782a9dc46cca025a204a952c6a5352c0e5147dadccaf57da3` |
| `13_validation_loss` | 4; sibling validation result lost | Harness | `unknown_commit_result` | 1 / `b9fbd37af1ec09c17bade131abbadd653640c9ce8c4e41ef1f1d457cd89cf9a9` | 2 / `73f98f28c9ba21837cfe1045e7c3cc165a6100030f44c808e6ac04dc777df357` |
| `13_validation_loss` | 5; DACL validation result lost | Harness | `unknown_commit_result` | 1 / `0ede316defffda98a5b2751ade8a26607433a6721809e5f0e2223694ebb9ec9e` | 2 / `39651e0ab7acd41405c527582dc669fae31372148a959e6b1149a379b4669781` |
| `14_close_result_loss` | 1; successful close result lost | Harness | `unknown_commit_result` | 1 / `308fa6348158fc37f3d4f8e639f63666065850bda7d2c7dd306637e70b71a936` | 2 / `d4ad3b44fc10c58caeed27efbee783e3a7bef2188e88b1e793f17d56eb43f0ae` |
| `15_ack_loss` | 1; complete path followed by acknowledgement loss | Harness | `unknown_commit_result` | 1 / `09d2cabd2f6b285735e8b9206d463c89036333c547a08134d7417f5f31e42877` | 2 / `8711029d8629c4ac052c7a963f7f0d15e99f39a7184eab6a772e67e1febf9c9d` |
| `16_success` | 1; full validated path and one acknowledgement | Native | committed | 1 / `2a10f300002f05f312429cfc3c9ee12629fb6c127927be866a23096b15640717` | 2 / `792629f5e19b1b26eb3ca65bb91f19b0f122f2f0b9485236b90ae0615b1f5927` |

The row-9 fresh recovery described physical old state only and did not retroactively invent an
earlier acknowledgement. Row 10 is now deterministically reached through the pinned driver's public
`sqlite.HookRegisterer.RegisterCommitHook` surface obtained from the dedicated
`database/sql.Conn.Raw` connection. The generated SQLite engine invokes that callback from inside
`_vdbeCommit` only after it has found the live write transaction and acquired the pager's exclusive
lock, and before `_sqlite3BtreeCommitPhaseOne`. The callback writes the one strict child-protocol
checkpoint from that native call stack and then blocks without returning. Only after the parent has
validated the exact scenario, cycle, attempt, operation, checkpoint, database, session, and
`commit_invoked=true` / `commit_returned=false` evidence does it terminate the child. Fresh
recovery returned the exact old row. The application outcome remains `unknown_commit_result`; the
injected process loss is not reported as a SQLite error or Windows storage result. Row 11 is
genuinely unreachable under this exact shape without changing SQLite, the
driver, filesystem, or protected code: an independent same-session owner is rejected with
`BUSY/LOCKED` at acquisition before observation and therefore cannot retain a conflicting lock at
commit; the control transaction returned genuine success. No error code was fabricated.

**Exact local commit call-chain qualification — 2026-08-04.** The reviewed standard-library source
was Go `go1.26.4` at `C:\Program Files\Go\src\database\sql\sql.go:2287-2319` and
`C:\Program Files\Go\src\database\sql\driver\driver.go:518-522`. The reviewed pinned module source
was `modernc.org/sqlite v1.56.0` at
`C:\Users\Jamie\go\pkg\mod\modernc.org\sqlite@v1.56.0`, with its required
`modernc.org/libc v1.74.4`. The exact path is:

```text
(*database/sql.Tx).Commit
  -> driver.Tx.Commit through tx.txi under database/sql's driverConn lock
  -> (*modernc.org/sqlite.tx).Commit
  -> (*sqlite.tx).exec(context.Background(), "commit")
  -> sqlite3.Xsqlite3_exec
  -> sqlite3.Xsqlite3_prepare_v2
  -> sqlite3.Xsqlite3_step
  -> _sqlite3Step
  -> _sqlite3VdbeExec
  -> _sqlite3VdbeHalt
  -> _vdbeCommit
  -> detect the live write transaction and acquire the pager exclusive lock
  -> FxCommitCallback / modernc commitHookTrampoline / test callback
  -> _sqlite3BtreeCommitPhaseOne
  -> _sqlite3BtreeCommitPhaseTwo
  -> return through sqlite3_exec, modernc tx.Commit, and database/sql Tx.Commit
```

The driver locations were `tx.go:34-78` for its commit/exec path,
`sqlite.go:618-622` for `HookRegisterer`, and `pre_update_hook.go:53-67,205-215` for commit-hook
registration and dispatch. The generated Windows/amd64 engine locations were
`lib/sqlite_windows.go:5803-5924` for `Xsqlite3_exec`,
`lib/sqlite.go:11963-12022` for `Xsqlite3_step`, and
`lib/sqlite_windows.go:93901-93973,104342-104538,116055-116163` for `_sqlite3Step`, VDBE halt,
the commit-hook boundary, and the two commit phases.

Every candidate observation or interception point was classified explicitly:

- a marker immediately before `Tx.Commit`, entry to a wrapper `driver.Tx.Commit`, goroutine start,
  timer, sleep, or parent-side kill race is too early because the underlying commit may not have
  begun;
- the Go `database/sql` test hooks cover connection return, transaction connection grabbing, and
  rollback, but expose no post-driver-commit-entry/pre-result hook;
- driver connection hooks run at connection setup; pre-update/update hooks run while staging the
  row; rollback hooks run on rollback; authorizer and statement-trace callbacks can run at prepare or
  statement-start boundaries; and progress callbacks are opcode-cadence dependent. None proves the
  selected exact commit boundary as strongly as the native commit hook;
- suppressing a result after `Tx.Commit`, dropping the child response, or relabeling the existing
  response-loss row is too late because the commit result was already observed in the child;
- debugger breakpoints, runtime/symbol patching, a replacement driver, a custom VFS, and direct
  SQLite/pager instrumentation would require a different lower-level harness or dependency surface;
  filesystem/journal observation is both insufficient and prohibited by this evidence contract; and
- the accepted `RegisterCommitHook` callback is the exact seam: SQLite itself calls it from
  `_vdbeCommit` after the write-transaction and exclusive-lock checks and before phase one. The test
  callback never returns, never supplies a nonzero abort code, and never observes or suppresses a
  commit result.

The exact aggregate counters were: rows attempted `22`; rows proven natively `7`; harness-injected
application-boundary rows `14`; rows proven unreachable `1`; rows still unproven `0`; known unchanged
`6`; committed `2`; rejected `4`; `unknown_commit_result` `10`; recovery-only opens `22`; exact old
recoveries `12`; exact new recoveries `10`; genuine `BUSY/LOCKED` before observation `1`, after
observation `0`, and at commit `0`; successful different-session commits `1`; rollback attempts and
exact-old proofs `6` / `6`; forced terminations `4`; commit invocations and genuine return
observations `12` / `11`; success acknowledgements `2` (the independent different-session control
and the full-success row). Duplicate commits, retries, replays, repairs, fallbacks, partial rows,
ambiguous pre-commit recoveries, staged-row leaks, revision gaps/duplicates, digest/envelope
mismatches, unexpected siblings/protection widening, and protocol loss/duplication/reordering were
all `0`.

The clean matrix elapsed time was `3.893s`. The canonical-root pre/post metadata-tree SHA-256 values
were `01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b` and
`527048ffa7ddd5c489824413b8b38a23d70609ace1e728e58dcc8966e38e765e`. The rollups over every
scenario's exact pre/post metadata hash were
`7ba2eb2fd9acaf76c15a9139f494fd31c2515498757db1d162002dcc3e05b7a5` and
`596eb9e3504a83a46846de9e79a4c9de04de66c37b224513f9a42e50590dccc7`.
Each metadata-tree hash is SHA-256 over LF-terminated, ordinally sorted rows containing relative path,
entry type, byte size, and exact owner/DACL evidence. The rollup binds scenario ID, attempt, and its
tree hash. No journal was opened or hashed for contents. Every scenario admitted only exact
`aggregate.sqlite` / `aggregate.sqlite-journal` regular siblings; journals retained the exact private
DACL. Only the parent test framework removed the canonical temporary root after all children and
connections closed.

Required validation then produced these exact results, in order:

```text
DORKPIPE_SQLITE_FAILURE_MATRIX=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteFailureBoundaryMatrix$' -count=1 -v -timeout=30m
PASS; matrix elapsed 3.893s; rows proven natively 7; rows still unproven 0; commit invocations/returns 12/11
DORKPIPE_SQLITE_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
PASS; cohort elapsed 2m26.596s; all existing counters and 9e4b...11b8 pre/post hash unchanged
DORKPIPE_SQLITE_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
PASS; cohort elapsed 3m56.453s; all existing counters and dd67...0aaa pre/post hash unchanged
DORKPIPE_SQLITE_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteSmoke$' -count=1 -v
PASS; smoke elapsed 1.46s; primary SQLITE_BUSY 5 and exact revision-2 recovery unchanged
CGO_ENABLED=0 GOOS=<windows|linux|darwin> GOARCH=<amd64|amd64|arm64> go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED/GOOS/GOARCH matched; binary sizes 12205568 / 11044675 / 10943842 bytes; verified temporary directory removed
go test -mod=readonly ./appserversupervisor ./providersession -count=1
EXPECTED PROTECTED FAILURE; providersession passed; selector assertion remained 17 instead of 1
go test -mod=readonly ./... -count=1 -timeout=90s
EXPECTED PROTECTED FAILURES; selector assertion unchanged; sqliteevidence passed; orchestrationhelper timed out in TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMalformedDecisionFixtures/malformed
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/windows_failure_boundary_matrix_test.go
PASS; empty output
git diff --check
PASS
```

The full-suite timeout remained in the same protected placement-execution fixture chain and the same
pre-existing `os.ReadFile` path, but its active subtest moved from the preceding
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRejectsTargetSetConflicts/stale_version`
case to
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMalformedDecisionFixtures/malformed`.
No protected App Server, Pipeon, or orchestration-helper path was edited. Clean cross-target evidence
binaries were inspected outside the repository and the verified directory
`dockpipe-sqlite-failure-cross-769ef3630a344e2e9359f6df6603a836` was removed. The updated
failure-matrix file SHA-256 is
`e15d602c8945a0852a6c388702c8242dfce0a9c9e17959caf4f7a18d9b933077`.

Row 10 is now genuinely reachable and proven. The complete deterministic matrix is still not
claimed closed because the required, genuinely unreachable row 11 remains recorded rather than
simulated. Windows reboot/power-loss trials, broader Linux publication/contention/failure cohorts,
macOS/arm64 GitHub Actions evidence intentionally scheduled last, macOS VM
disruption evidence if still required, complete production host/sidecar acceptance, production
storage, migration, cutover, recovery authority, dispatch/projection/decision integration, and Slice
2 all remain open. TASK-013 and CAS-14 remain open.

The completed dependency-pin/smoke, publication, and contention/forced-termination slices do not
claim the deterministic failure-boundary matrix, Windows VM reboot or hard-power-loss durability,
complete sidecar qualification, a production host allowlist, broader Linux native cohorts,
macOS/arm64 GitHub Actions evidence (intentionally last), or macOS VM disruption evidence if still
required. They add no production store, migration, cutover, recovery authority, lifecycle dispatch,
Pipeon projection, or Slice 2 work. Those gates remain open; TASK-013 and CAS-14 remain open.

**Linux/amd64 native SQLite smoke qualification — 2026-08-04.** The Linux-only opt-in
`TestLinuxNativeSQLiteSmoke` passed natively with `CGO_ENABLED=0` on Pop!_OS 22.04 LTS,
Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`. The pinned module graph remained unchanged:
`golang.org/x/sys v0.47.0`, `modernc.org/libc v1.74.4`, and `modernc.org/sqlite v1.56.0`; the
`go.mod` / `go.sum` SHA-256 values were respectively
`f59ee93b1feb390705c790649a6ac36de360053aa5260818885c78df19881d19` and
`b426dc8754abc50973fbae78d32642746de09cc6c6b6485a24727572cbf610a9`.

The parent created one new private temporary parent
`/tmp/dockpipe-sqlite-linux-fYSJ5FKK` outside the repository, set and revalidated it as a current-user
owned `0700` directory, and used it as `TMPDIR`. The successful test-framework fixture root was
`/tmp/dockpipe-sqlite-linux-fYSJ5FKK/TestLinuxNativeSQLiteSmoke3890145937/001`, also current-user
owned and `0700`. `statx(STATX_MNT_ID)`, a metadata-only `O_PATH|O_NOFOLLOW` handle plus `fstatfs`,
and `/proc/self/mountinfo` agreed on mount ID `33`, device `259:7`, ext4 magic `0xef53`, source
`/dev/nvme0n1p3`, mount root/point `/` / `/`, options `rw,noatime`, and super-options
`rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The source block device and its parent were non-removable. The lane rejected bind, nested, overlay,
FUSE, network, removable, shared-host, `drvfs`, `9p`, `tmpfs`, symlinked/substituted, and cross-mount
storage. Every fixture/session directory was an owned regular `0700` directory; every database and
observed journal was an owned regular `0600` file on the same exact mount/device. Only the selected
`aggregate.sqlite` and `aggregate.sqlite-journal` siblings were admitted. The metadata checks never
opened a journal for content and never parsed, copied, moved, truncated, deleted, or hashed one.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `unix` VFS. The exact main-database URI was:

```text
file:///tmp/dockpipe-sqlite-linux-fYSJ5FKK/TestLinuxNativeSQLiteSmoke3890145937/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

The lane applied and read back `journal_mode=delete`, `synchronous=3` (`EXTRA`), `fullfsync=1`,
`temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`, `foreign_keys=1`,
`trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and pre-schema
`page_size=4096`. It also rejected unresolved double-quoted SQL, required only `main` in
`PRAGMA database_list`, created exactly the selected singleton STRICT
`app_server_aggregate` table plus `user_version=1`, and used only the selected insert and exact
session/revision/digest conditional-update shape. Every staged and committed row reloaded with exact
canonical payload, envelope, and SHA-256 equality; revisions were strictly monotonic.

Linux exposed this exact sorted 56-entry compile-option set:

```text
ATOMIC_INTRINSICS=1,COMPILER=gcc-12.2.0,DEFAULT_AUTOVACUUM,DEFAULT_CACHE_SIZE=-2000,
DEFAULT_FILE_FORMAT=4,DEFAULT_JOURNAL_SIZE_LIMIT=-1,DEFAULT_MEMSTATUS=0,DEFAULT_MMAP_SIZE=0,
DEFAULT_PAGE_SIZE=4096,DEFAULT_PCACHE_INITSZ=20,DEFAULT_RECURSIVE_TRIGGERS,
DEFAULT_SECTOR_SIZE=4096,DEFAULT_SYNCHRONOUS=2,DEFAULT_WAL_AUTOCHECKPOINT=1000,
DEFAULT_WAL_SYNCHRONOUS=2,DEFAULT_WORKER_THREADS=0,DIRECT_OVERFLOW_READ,DISABLE_INTRINSIC,
ENABLE_COLUMN_METADATA,ENABLE_DBPAGE_VTAB,ENABLE_DBSTAT_VTAB,ENABLE_FTS5,ENABLE_GEOPOLY,
ENABLE_MATH_FUNCTIONS,ENABLE_MEMORY_MANAGEMENT,ENABLE_OFFSET_SQL_FUNC,ENABLE_PREUPDATE_HOOK,
ENABLE_RBU,ENABLE_RTREE,ENABLE_SESSION,ENABLE_SNAPSHOT,ENABLE_STAT4,ENABLE_UNLOCK_NOTIFY,
LIKE_DOESNT_MATCH_BLOBS,MALLOC_SOFT_LIMIT=1024,MAX_ATTACHED=10,MAX_COLUMN=2000,
MAX_COMPOUND_SELECT=500,MAX_DEFAULT_PAGE_SIZE=8192,MAX_EXPR_DEPTH=1000,MAX_FUNCTION_ARG=1000,
MAX_LENGTH=1000000000,MAX_LIKE_PATTERN_LENGTH=50000,MAX_MMAP_SIZE=0x7fff0000,
MAX_PAGE_COUNT=0xfffffffe,MAX_PAGE_SIZE=65536,MAX_SQL_LENGTH=1000000000,
MAX_TRIGGER_DEPTH=1000,MAX_VARIABLE_NUMBER=32766,MAX_VDBE_OP=250000000,
MAX_WORKER_THREADS=8,MUTEX_PTHREADS,SOUNDEX,SYSTEM_MALLOC,TEMP_STORE=1,THREADSAFE=1
```

This is an exact native platform allowlist, not a weakened count-only or subset check: count,
ordering, and every entry fail closed. Windows independently retains its existing exact 57-entry
set and evidence contract. Linux uses `MUTEX_PTHREADS`; Windows uses `MUTEX_NOOP` and additionally
has the Windows-only `OMIT_SEH` entry. Their separately pinned compiler identity entries also remain
platform-specific (`gcc-12.2.0` on Linux and `gcc-12-win32` on Windows). No Windows cohort or
protected Windows evidence contract changed.

The revision-1 insert digest was
`5bacd33f5355f1a64a096841fe3fceeca28a40f211723e2ce4bb9b56988e6fe8`; the exact revision-2 CAS
digest was `37572e06825751539b2e65c19034a23950925abbbe795d296a52ecf1e6e2aca4`.
An independent owner process staged revision 3 and retained the live rollback journal. A fresh
same-session contender returned genuine primary `SQLITE_BUSY` (`5`), while a different-session
database remained independently writable and passed its own `quick_check`. Forced owner
termination occurred before any commit. A fresh recovery child returned `quick_check=ok` and the
exact old revision-2 row. One fresh parent-held dedicated connection then committed revision 3
exactly once, reloaded and integrity-checked it, and produced final digest
`557edb00816e95dbc84b0bba0f347cdaf6087fc49a6661534de903646cd3ec66`.
There were zero retries, replays, repairs, fallbacks, inferred acknowledgements, staged-row leaks,
revision gaps/duplicates, or ambiguous recoveries. Journal metadata was observed at 4,616 bytes for
the initial commits and 8,720 bytes while the clean revision-3 connection remained open.

The pre-contention and post-clean-commit metadata-tree SHA-256 values were respectively
`0a388a03d9be383266d97f96a101f39b28e94b87f00c52f52cf6700f3ae13dc2` and
`6fcf39acb8a48a782087cccba2f97958578d6077633c3fdf4618f96cfe627bc2`.
Each hash covered LF-terminated ordinally sorted rows containing relative path, entry type, size,
mode, owner, device, inode, mount ID, filesystem type/magic, source, and mount point. Evidence elapsed
time was `50ms`; the package result was `ok` in `0.052s`. Only the parent test framework cleaned the
fixture. After the pass, the caller removed the exact now-empty temporary parent and verified it no
longer existed. No child process, fixture, binary, or evidence artifact remained.

The exact successful native command, run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-fYSJ5FKK DORKPIPE_SQLITE_LINUX_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
```

A preceding sandboxed setup attempt never started the test because the existing Go build cache was
read-only in that sandbox; it supplied no native evidence. The successful command above ran with the
required host access and revalidated a newly created fixture path rather than reusing the removed
attempt fixture. This Linux smoke qualification changes no dependency, production storage, Slice 2
surface, publication/contention/failure cohort, migration, cutover, lifecycle, or support decision.
The shared smoke wrapper has now passed both the native Linux qualification above and the final
native Windows rerun below.

**Final Windows/amd64 shared-wrapper rerun — 2026-08-04.** The Windows-only opt-in
`TestWindowsNativeSQLiteSmoke` passed natively with `CGO_ENABLED=0` on Windows build `10.0.26200`,
`amd64`, and Go `go1.26.4`. The fixture used qualifying fixed local NTFS storage on volume
`\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\` with serial `88c9a133` and label `OS`; the
unprivileged NTFS-version query remained unavailable. The fixture root, both database files, and
observed journals were owned by current-user SID
`S-1-5-21-2729925100-2499202611-1015899381-1002`, admitted only that SID and `SYSTEM`
(`S-1-5-18`) as trustees, and granted them full access.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `win32` VFS. The exact selected main-database URI was:

```text
file:///C:/Users/Jamie/AppData/Local/Temp/TestWindowsNativeSQLiteSmoke827599481/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=win32
```

The lane read back the selected `journal_mode=delete`, `synchronous=3` (`EXTRA`), `fullfsync=1`,
`temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`, `foreign_keys=1`,
`trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and pre-schema
`page_size=4096` pragmas. It required only `main`, the exact singleton STRICT
`app_server_aggregate` schema with `user_version=1`, and the selected absolute URI. Windows retained
its exact sorted 57-option allowlist, including `COMPILER=gcc-12-win32`, `MUTEX_NOOP`, and
`OMIT_SEH`, with no `MUTEX_PTHREADS`. Linux remains exactly 56 options with
`COMPILER=gcc-12.2.0` and `MUTEX_PTHREADS`, without the two Windows-only mutex/SEH entries.

The exact revision-1 insert payload and digest reloaded equal at
`5bacd33f5355f1a64a096841fe3fceeca28a40f211723e2ce4bb9b56988e6fe8`; the exact revision-2 CAS
payload and digest reloaded equal at
`37572e06825751539b2e65c19034a23950925abbbe795d296a52ecf1e6e2aca4`. An independent owner staged
revision 3 and held the same database and its protected rollback journal. A fresh contender returned
genuine primary `SQLITE_BUSY` (`5`), while the different-session database remained independently
writable. The parent forcibly terminated the owner before commit; a fresh recovery child returned
`quick_check=ok` and the exact old revision-2 row. There were zero retries, replays, repairs,
fallbacks, or inferred acknowledgements. The observed journals remained protected siblings, were
4,616 bytes after commit, and were never opened or hashed for content. Cleanup remained
parent-test-only, and the test left no fixture, evidence artifact, binary, or child process.

The exact successful PowerShell command, run from `packages/dorkpipe/lib`, was:

```powershell
$env:DORKPIPE_SQLITE_EVIDENCE = "1"
$env:CGO_ENABLED = "0"
go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteSmoke$' -count=1 -v -timeout=10m
```

The evidence lane elapsed time was `1.429s`; the package result was `ok` in `3.929s`. This completes
the shared-wrapper native Linux and Windows qualification only. It does not qualify the deferred
publication, contention, or failure cohorts on Linux or other platforms, power-loss evidence,
production storage, migration, Slice 2, or macOS.

The remaining required validation produced these exact results from `packages/dorkpipe/lib`:

```text
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED=0, GOOS=windows, GOARCH=amd64; size 12,087,808 bytes
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED=0, GOOS=linux, GOARCH=amd64; size 11,137,781 bytes
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED=0, GOOS=darwin, GOARCH=arm64; size 10,953,058 bytes
go test -mod=readonly ./appserversupervisor ./providersession -count=1
PASS; appserversupervisor 0.712s; providersession 0.002s
go test -mod=readonly ./... -count=1 -timeout=90s
EXPECTED PROTECTED FAILURES; sqliteevidence passed; cmd/dorkpipe failed its existing Windows-style path-normalization candidate assertion; orchestrationhelper timed out after 90s
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/host_other_test.go appserversupervisor/sqliteevidence/sqlite_smoke_test.go appserversupervisor/sqliteevidence/host_linux_test.go appserversupervisor/sqliteevidence/linux_smoke_test.go
PASS; empty output
git diff --check
PASS
```

All three cross-target binaries were written under the one revalidated private ext4 directory
`/tmp/dockpipe-sqlite-cross-0fVZ2Rme`, inspected with `go version -m`, then removed with that exact
directory. The full suite's protected `cmd/dorkpipe` failure was
`TestProviderPoolWorkdirHashCandidatesIncludeWindowsStyleNormalizations`: the candidate list retained
the original and lowercase Windows paths plus Linux-working-directory-prefixed forms, but lacked the
expected normalized variants. The exact protected timeout was
`TestSoftwareDevPromotionPatchGenerationAndApprovedApply`; at the timeout it was inside the
pre-existing bundled-cache fingerprint/extraction path reached through
`ValidateResolvedWorkflowYAML`, promotion patch compilation, and approved apply. This moved from the
Windows baseline timeout in
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMalformedDecisionFixtures/malformed`,
which was blocked in its pre-existing `os.ReadFile` fixture path. Neither protected failure is in an
authorized path for this Linux qualification, and `appserversupervisor/sqliteevidence` passed in the
same full-suite run.

**Linux/amd64 10,000-cycle native reader-publication cohort — 2026-08-04.** The Linux-only opt-in
`TestLinuxNativeSQLitePublicationCohort`, gated by
`DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1`, passed natively with `CGO_ENABLED=0`,
`-mod=readonly`, `-count=1`, verbose output, and the fixed 30-minute timeout. It ran on Pop!_OS
22.04 LTS, Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`.

The caller created the new private parent
`/tmp/dockpipe-sqlite-linux-publication-vqoEPdfl` outside the repository with mode `0700`, owner
UID/GID `1000:1000`, device `259:7`, and inode `57176041`, and used it as `TMPDIR`. The test-owned
fixture root was
`/tmp/dockpipe-sqlite-linux-publication-vqoEPdfl/TestLinuxNativeSQLitePublicationCohort2406397794/001`,
also owned `1000:1000` with mode `0700`. Its retained root identity was mount ID `33`, device
`259:7`, inode `57176196`, and kind `directory`. Metadata-only `statx`, `O_PATH|O_NOFOLLOW` plus
`fstatfs`, and `/proc/self/mountinfo` agreed on ext4 magic `0xef53`, source `/dev/nvme0n1p3`, mount
root/point `/` / `/`, mount options `rw,noatime`, and super-options
`rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The source block device and parent were non-removable. The lane rejected bind, nested, overlay,
FUSE, network, removable, shared-host, `drvfs`, `9p`, `tmpfs`, symlinked/substituted, and cross-mount
storage. Every fixture/session directory remained an owned `0700` directory; the main database and
every observed rollback journal remained owned regular `0600` files on the exact qualified
mount/device. Only `aggregate.sqlite` and `aggregate.sqlite-journal` were admitted.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `unix` VFS. The exact selected URI was:

```text
file:///tmp/dockpipe-sqlite-linux-publication-vqoEPdfl/TestLinuxNativeSQLitePublicationCohort2406397794/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

Every connection applied and read back `journal_mode=delete`, `synchronous=3` (`EXTRA`),
`fullfsync=1`, `temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`, `foreign_keys=1`,
`trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and pre-schema `page_size=4096`.
The lane rejected unresolved double-quoted SQL, required only `main`, and retained exactly the
singleton STRICT `app_server_aggregate` schema with `user_version=1`. The initial connection
fail-closed validated the exact sorted 56-option Linux allowlist recorded in the preceding Linux
smoke qualification: it includes `COMPILER=gcc-12.2.0` and `MUTEX_PTHREADS`, with no `MUTEX_NOOP`
or `OMIT_SEH`. Windows independently remains exactly 57 options with `COMPILER=gcc-12-win32`,
`MUTEX_NOOP`, and `OMIT_SEH`, and without `MUTEX_PTHREADS`.

One persistent writer child and one persistent reader child used a bounded strict JSON-line
protocol. Every command and response retained its exact cycle number and operation. Missing,
duplicate, malformed, substituted, unknown-field, multiple-value, or out-of-order protocol data
failed closed. For every cycle, the reader returned the exact old row, the writer staged exactly the
next revision and held the protected live rollback journal, a fresh same-database reader connection
returned only genuine primary `SQLITE_BUSY` (`5`) or `SQLITE_LOCKED` (`6`), the writer committed
exactly once, and the reader then returned the exact new row and `quick_check=ok`. Complete canonical
payload, envelope, session ID, revision, and SHA-256 equality was required throughout.

The exact aggregate result was:

- cycles: `10000`;
- successful pre-publication exact old reads: `10000`;
- live-owner primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `10000`;
- successful post-release exact new reads: `10000`;
- protected live-journal observations: `10000`;
- ambiguous or partial reads, revision gaps/duplicates, digest mismatches, and child-protocol loss,
  duplication, substitution, or reordering: `0`;
- retries, replays, repairs, fallbacks, and inferred acknowledgements: `0`;
- initial revision/digest: `1` /
  `aa5cf90832cf7e71136cfa92208ef923e141d7d8103cab900f642ed02e50b3fb`;
- final revision/digest: `10001` /
  `3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838`;
- cohort elapsed time: `1m33.87s`; package result: `ok` in `93.891s`.

The quiescent pre/post Linux metadata-tree SHA-256 was stable and equal:
`ccaaef3dc1a4eab9ab808bd5ec040fcdedbde14ab4202ad540aee9fb9f362e90`. The hash covered
LF-terminated ordinally sorted rows containing relative path, entry type, byte size, mode, owner,
device, inode, mount ID, filesystem type/magic, source, and mount point. Journal checks were
metadata-only: no journal was opened or hashed for content. Both children exited through the exact
shutdown protocol; only the parent test framework cleaned the fixture. The caller found the private
temporary parent empty, removed that exact directory, verified it no longer existed, and found no
remaining fixture, child process, binary, or evidence artifact.

The exact successful native command, run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-publication-vqoEPdfl DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
```

The required focused validation, run from `packages/dorkpipe/lib`, produced:

```text
CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -count=1
PASS; ok in 0.006s
TMPDIR=/tmp/dockpipe-sqlite-linux-publication-smoke-BJ0KRAtZ CGO_ENABLED=0 DORKPIPE_SQLITE_LINUX_EVIDENCE=1 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
PASS; evidence 47ms; package 0.049s; primary contention code SQLITE_BUSY (5); recovery quick_check=ok
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/linux_publication_cohort_test.go appserversupervisor/sqliteevidence/host_linux_test.go appserversupervisor/sqliteevidence/linux_smoke_test.go appserversupervisor/sqliteevidence/sqlite_smoke_test.go
PASS; empty output
git diff --check
PASS; empty output
```

The smoke fixture used a separately created private ext4 parent, which was empty after parent-test
cleanup and then removed exactly. The complete test package also cross-compiled with `CGO_ENABLED=0`
to one new verified private directory outside the repository. `go version -m` confirmed embedded
settings for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; the binaries were respectively
12,087,808, 11,588,685, and 10,953,058 bytes. The three exact binaries and their now-empty parent
`/tmp/dockpipe-sqlite-linux-publication-cross-OGrCa3sR` were removed. Cross-compilation remains
compatibility evidence only.

The protected Windows publication cohort remains unchanged and independently qualified. This pass
qualifies only the Linux reader-publication cohort. It does not qualify Linux contention or failure
cohorts, power-loss evidence, production storage, migration, Slice 2, or macOS. Linux contention is
the next native cohort; macOS evidence remains intentionally last. TASK-013 and CAS-14 remain open.

**Linux/amd64 1,000-cycle native contention/forced-termination cohort — 2026-08-04.** The new
Linux-only `TestLinuxNativeSQLiteContentionCohort`, gated by
`DORKPIPE_SQLITE_LINUX_CONTENTION_COHORT=1`, passed natively with `CGO_ENABLED=0`,
`-mod=readonly`, `-count=1`, verbose output, and the fixed 30-minute timeout. It ran on Pop!_OS
22.04 LTS, Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`.

The caller created the new private parent
`/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq` outside the repository with mode `0700`, owner
UID/GID `1000:1000`, stat device `66311`, and inode `57176054`, and used it as `TMPDIR`.
The test-owned fixture root was
`/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq/TestLinuxNativeSQLiteContentionCohort1544489198/001`,
also owned `1000:1000` with mode `0700`. Its retained root identity was mount ID `33`, device
`259:7`, inode `57176199`, and kind `directory`. Metadata-only `statx`,
`O_PATH|O_NOFOLLOW` plus `fstatfs`, and `/proc/self/mountinfo` agreed on ext4 magic `0xef53`,
source `/dev/nvme0n1p3`, mount root/point `/` / `/`, mount options `rw,noatime`, and
super-options `rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The source block device and temporary parent were non-removable. The lane rejected bind, nested,
overlay, FUSE, network, removable, shared-host, `drvfs`, `9p`, `tmpfs`,
symlinked/substituted, and cross-mount storage. The fixture root plus the Linux-qualified `main`
and `other` session directories remained owned `0700` directories; both main databases and every
observed rollback journal remained owned regular `0600` files on the exact retained
mount/device/inode identities. Only `aggregate.sqlite` and `aggregate.sqlite-journal` were
admitted as database siblings.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `unix` VFS. The exact selected absolute URIs were:

```text
file:///tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq/TestLinuxNativeSQLiteContentionCohort1544489198/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
file:///tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq/TestLinuxNativeSQLiteContentionCohort1544489198/001/other/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

Every connection applied and read back `journal_mode=delete`, `synchronous=3` (`EXTRA`),
`fullfsync=1`, `temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`,
`foreign_keys=1`, `trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and
pre-schema `page_size=4096`. The lane rejected unresolved double-quoted SQL, required only
`main`, and retained exactly the singleton STRICT `app_server_aggregate` schema with
`user_version=1`. Both initial database connections fail-closed validated this exact sorted
56-option Linux contract:

```text
ATOMIC_INTRINSICS=1,COMPILER=gcc-12.2.0,DEFAULT_AUTOVACUUM,DEFAULT_CACHE_SIZE=-2000,
DEFAULT_FILE_FORMAT=4,DEFAULT_JOURNAL_SIZE_LIMIT=-1,DEFAULT_MEMSTATUS=0,DEFAULT_MMAP_SIZE=0,
DEFAULT_PAGE_SIZE=4096,DEFAULT_PCACHE_INITSZ=20,DEFAULT_RECURSIVE_TRIGGERS,
DEFAULT_SECTOR_SIZE=4096,DEFAULT_SYNCHRONOUS=2,DEFAULT_WAL_AUTOCHECKPOINT=1000,
DEFAULT_WAL_SYNCHRONOUS=2,DEFAULT_WORKER_THREADS=0,DIRECT_OVERFLOW_READ,DISABLE_INTRINSIC,
ENABLE_COLUMN_METADATA,ENABLE_DBPAGE_VTAB,ENABLE_DBSTAT_VTAB,ENABLE_FTS5,ENABLE_GEOPOLY,
ENABLE_MATH_FUNCTIONS,ENABLE_MEMORY_MANAGEMENT,ENABLE_OFFSET_SQL_FUNC,ENABLE_PREUPDATE_HOOK,
ENABLE_RBU,ENABLE_RTREE,ENABLE_SESSION,ENABLE_SNAPSHOT,ENABLE_STAT4,ENABLE_UNLOCK_NOTIFY,
LIKE_DOESNT_MATCH_BLOBS,MALLOC_SOFT_LIMIT=1024,MAX_ATTACHED=10,MAX_COLUMN=2000,
MAX_COMPOUND_SELECT=500,MAX_DEFAULT_PAGE_SIZE=8192,MAX_EXPR_DEPTH=1000,MAX_FUNCTION_ARG=1000,
MAX_LENGTH=1000000000,MAX_LIKE_PATTERN_LENGTH=50000,MAX_MMAP_SIZE=0x7fff0000,
MAX_PAGE_COUNT=0xfffffffe,MAX_PAGE_SIZE=65536,MAX_SQL_LENGTH=1000000000,
MAX_TRIGGER_DEPTH=1000,MAX_VARIABLE_NUMBER=32766,MAX_VDBE_OP=250000000,
MAX_WORKER_THREADS=8,MUTEX_PTHREADS,SOUNDEX,SYSTEM_MALLOC,TEMP_STORE=1,THREADSAFE=1
```

Each cycle used a fresh owner child that loaded the exact committed same-session row, began the
selected exclusive transaction, applied exactly one next-revision CAS, validated the complete staged
row, reported `staged_live` with the exact cycle/revision/digest, and remained live with the
protected rollback journal. The parent validated journal metadata and allowed siblings without
opening journal content. A fresh same-session contender returned only genuine primary
`SQLITE_BUSY` (`5`) or `SQLITE_LOCKED` (`6`). A fresh different-session writer validated,
committed, reloaded, integrity-checked, and closed its independent database while the owner remained
live. The parent then killed the owner before any commit command or acknowledgement existed. A fresh
recovery child performed hot-journal recovery, required exactly one `quick_check=ok`, and returned
the exact old same-session row; the killed owner's staged row did not leak. A separate fresh clean
writer committed the same-session next revision exactly once. Complete canonical payload, envelope,
session ID, revision, and SHA-256 equality was required at every boundary.

Every child accepted exactly one bounded strict JSON-line command, returned exactly one bounded
response carrying the exact cycle and operation, and failed closed on missing, duplicate, malformed,
substituted, unknown-field, multiple-value, or out-of-order data. There were no retries, replays,
repairs, fallbacks, inferred acknowledgements, ambiguous recoveries, revision gaps/duplicates, or
child cleanup.

The exact aggregate result was:

- owner transactions staged: `1000`;
- protected live journals: `1000`;
- same-session primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `1000`;
- primary code `5`: `1000`; primary code `6`: `0`; sum: `1000`;
- different-session commits while the owner remained live: `1000`;
- forced owner terminations before commit invocation: `1000`;
- exact old-row recoveries: `1000`;
- successful post-recovery same-session commits: `1000`;
- same-session initial revision/digest: `1` /
  `bb0b0fa448e6532a65b420e128470a70fe5e32e15e94634b8c4fcf64a0b1e5ed`;
- same-session final revision/digest: `1001` /
  `e024c4e5dafc3841e26abbc2df7618f2fd78fcabd3b41bd364485b2ad56ff693`;
- different-session initial revision/digest: `1` /
  `8b351be57c3b6f86535ca6c2c3f6ef159175513013f7ac6608413e4e411dedfe`;
- different-session final revision/digest: `1001` /
  `5c78a969b9d47e07ef749d58c6b0fa3311512435141191d79376dd50e2f62f26`;
- cohort elapsed time: `46.672s`; package result: `ok` in `46.703s`.

The stable quiescent pre/post metadata-tree SHA-256 was equal:
`8bc08f6ab798ecde1fb8393281e1b4cef975fc11514634c1deb8b2d641ad37b9`. The hash covered
LF-terminated ordinally sorted metadata rows containing relative path, entry type, byte size, mode,
owner, device, inode, mount ID, filesystem type/magic, source, and mount point. Journal checks were
metadata-only: no journal was opened, parsed, copied, moved, truncated, deleted, or hashed for
content. Only the parent test framework cleaned the fixture. After the pass the caller found the
private temporary parent empty and no contention child process or fixture remained.

The exact successful native command, run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq DORKPIPE_SQLITE_LINUX_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
```

The required focused validation, run from `packages/dorkpipe/lib`, produced:

```text
CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -count=1
PASS; ok in 0.002s
TMPDIR=/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
PASS; evidence 1m26.551s; package 86.568s; old reads, BUSY/LOCKED results, new reads, and protected journals 10000 each
TMPDIR=/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq DORKPIPE_SQLITE_LINUX_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
PASS; evidence 50ms; package 0.052s; primary contention code SQLITE_BUSY (5); recovery quick_check=ok
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/linux_contention_cohort_test.go appserversupervisor/sqliteevidence/linux_publication_cohort_test.go appserversupervisor/sqliteevidence/host_linux_test.go appserversupervisor/sqliteevidence/linux_smoke_test.go appserversupervisor/sqliteevidence/sqlite_smoke_test.go
PASS; empty output
git diff --check
PASS; empty output
```

The unchanged publication rerun retained its exact initial revision/digest `1` /
`aa5cf90832cf7e71136cfa92208ef923e141d7d8103cab900f642ed02e50b3fb` and final
revision/digest `10001` /
`3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838`. Its fresh-fixture
pre/post metadata-tree SHA-256 remained stable and equal:
`203464c60a2224e636e380d42821d0b9fc15a0f1de67efa462ad4992eaf688f8`.

The complete test package cross-compiled with `CGO_ENABLED=0` to one separate newly verified
private ext4 directory outside the repository. `go version -m` confirmed exact embedded settings
for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; the binaries were respectively
`12,087,808`, `11,694,037`, and `10,953,058` bytes. The caller removed the three exact
binaries, then their now-empty parent
`/tmp/dockpipe-sqlite-linux-contention-cross-KCL300mg`. Cross-compilation is compatibility evidence
only, not native Windows or macOS evidence. After all validation the caller also removed the empty
verified fixture parent `/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq`. Neither directory, nor
any binary, fixture, child process, or evidence artifact remained.

The protected Windows contention cohort remains unchanged and independently qualified. Windows
retains exactly 57 options with `COMPILER=gcc-12-win32`, `MUTEX_NOOP`, and `OMIT_SEH`, while
Linux retains exactly 56 options with `COMPILER=gcc-12.2.0` and `MUTEX_PTHREADS`, and no
`MUTEX_NOOP` or `OMIT_SEH`. The Linux publication cohort also remains unchanged and independently
qualified. This pass qualifies only the Linux contention/forced-termination cohort. It does not
qualify Linux failure-boundary or power-loss evidence, production storage, migration, Slice 2, or
macOS. Linux failure-boundary qualification is next; macOS evidence remains intentionally last.
TASK-013 and CAS-14 remain open.

**Linux/amd64 native deterministic SQLite failure-boundary matrix — 2026-08-05.** The new
Linux-only `TestLinuxNativeSQLiteFailureBoundaryMatrix`, gated by
`DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX=1`, passed natively with `CGO_ENABLED=0`,
`-mod=readonly`, `-count=1`, verbose output, and the fixed 30-minute timeout. The exact command,
run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteFailureBoundaryMatrix$' -count=1 -v -timeout=30m
PASS; matrix elapsed 739ms; package ok in 0.748s
```

The caller created the new private parent `/tmp/dockpipe-sqlite-linux-failure-0a13OJYL` outside
the repository with mode `0700`, owner UID/GID `1000:1000`, stat device `66311`, and inode
`57176043`. The test-owned canonical root was
`/tmp/dockpipe-sqlite-linux-failure-0a13OJYL/TestLinuxNativeSQLiteFailureBoundaryMatrix1728090748/001`,
also owned `1000:1000` with mode `0700`. Its retained identity was mount ID `33`, device
`259:7`, inode `57176201`, and kind `directory`. Metadata-only `statx`,
`O_PATH|O_NOFOLLOW` plus `fstatfs`, and `/proc/self/mountinfo` agreed on ext4 magic
`0xef53`, source `/dev/nvme0n1p3`, mount root/point `/` / `/`, mount options
`rw,noatime`, and super-options `rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The run used Pop!_OS 22.04 LTS, Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`. The source block device and temporary
parent were non-removable. The lane rejected symlink, substitution, bind, nested, overlay, FUSE,
network, removable, shared-host, `drvfs`, `9p`, `tmpfs`, and cross-mount storage. Every
scenario root and `main` / `other` database directory remained an owned `0700` directory.
Every database and observed rollback journal remained an owned regular `0600` file on the exact
retained mount/device identity. Only `aggregate.sqlite` and
`aggregate.sqlite-journal` were admitted as database siblings.

Every fresh attempt revalidated SQLite `3.53.3`, source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`,
native `unix` VFS, the selected absolute `mode=rw` / `cache=private` /
`_txlock=exclusive` / `_dqs=0` / `_error_rc=1` URI, exact pragma readbacks, the singleton
STRICT schema, and `user_version=1`. The exact sorted 56-option Linux contract was:

```text
ATOMIC_INTRINSICS=1,COMPILER=gcc-12.2.0,DEFAULT_AUTOVACUUM,DEFAULT_CACHE_SIZE=-2000,
DEFAULT_FILE_FORMAT=4,DEFAULT_JOURNAL_SIZE_LIMIT=-1,DEFAULT_MEMSTATUS=0,DEFAULT_MMAP_SIZE=0,
DEFAULT_PAGE_SIZE=4096,DEFAULT_PCACHE_INITSZ=20,DEFAULT_RECURSIVE_TRIGGERS,
DEFAULT_SECTOR_SIZE=4096,DEFAULT_SYNCHRONOUS=2,DEFAULT_WAL_AUTOCHECKPOINT=1000,
DEFAULT_WAL_SYNCHRONOUS=2,DEFAULT_WORKER_THREADS=0,DIRECT_OVERFLOW_READ,DISABLE_INTRINSIC,
ENABLE_COLUMN_METADATA,ENABLE_DBPAGE_VTAB,ENABLE_DBSTAT_VTAB,ENABLE_FTS5,ENABLE_GEOPOLY,
ENABLE_MATH_FUNCTIONS,ENABLE_MEMORY_MANAGEMENT,ENABLE_OFFSET_SQL_FUNC,ENABLE_PREUPDATE_HOOK,
ENABLE_RBU,ENABLE_RTREE,ENABLE_SESSION,ENABLE_SNAPSHOT,ENABLE_STAT4,ENABLE_UNLOCK_NOTIFY,
LIKE_DOESNT_MATCH_BLOBS,MALLOC_SOFT_LIMIT=1024,MAX_ATTACHED=10,MAX_COLUMN=2000,
MAX_COMPOUND_SELECT=500,MAX_DEFAULT_PAGE_SIZE=8192,MAX_EXPR_DEPTH=1000,MAX_FUNCTION_ARG=1000,
MAX_LENGTH=1000000000,MAX_LIKE_PATTERN_LENGTH=50000,MAX_MMAP_SIZE=0x7fff0000,
MAX_PAGE_COUNT=0xfffffffe,MAX_PAGE_SIZE=65536,MAX_SQL_LENGTH=1000000000,
MAX_TRIGGER_DEPTH=1000,MAX_VARIABLE_NUMBER=32766,MAX_VDBE_OP=250000000,
MAX_WORKER_THREADS=8,MUTEX_PTHREADS,SOUNDEX,SYSTEM_MALLOC,TEMP_STORE=1,THREADSAFE=1
```

Its LF-terminated exact-set SHA-256 was
`8b9138f0970b0a9548b57112d02cecf88d573574977d4d0dbc106c4d8cdb7ac0`. It included
`MUTEX_PTHREADS` and excluded `MUTEX_NOOP` and `OMIT_SEH`.

All primary-row URIs used the exact canonical root above, the database suffix in the table below,
and the exact query
`?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix`. Therefore, for
example, row 10 used:

```text
file:///tmp/dockpipe-sqlite-linux-failure-0a13OJYL/TestLinuxNativeSQLiteFailureBoundaryMatrix1728090748/001/10_commit_call_loss-01/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

The independent different-session control in row 3 used:

```text
file:///tmp/dockpipe-sqlite-linux-failure-0a13OJYL/TestLinuxNativeSQLiteFailureBoundaryMatrix1728090748/001/03_contention-01/other/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

The complete 22-attempt result table follows. Each row began with the same platform-independent
session, payload, envelope, revision, and SHA-256 as the protected Windows matrix. The metadata
column records the exact per-scenario pre/post SHA-256 values; unequal values in the three
forced-termination rows reflect journal recovery metadata transitions rather than content access.

| Scenario | Attempt and boundary | Evidence / classification | Initial revision / SHA-256 | Final revision / SHA-256 | Database suffix | Pre / post metadata SHA-256 |
| --- | --- | --- | --- | --- | --- | --- |
| `01_before_open` | 1; before open | Harness / rejected | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` | `01_before_open-01/main/aggregate.sqlite` | `f5a51e99811dc9807dbe0c708c63a82c92300f7d07c930c401e922a12ebe072b` / `f5a51e99811dc9807dbe0c708c63a82c92300f7d07c930c401e922a12ebe072b` |
| `02_contract_reject` | 1; substituted contract evidence | Harness / rejected | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` | `02_contract_reject-01/main/aggregate.sqlite` | `0de287cb4be9d98f1c24218becce0515c720c681d71b2dfde02edc516731cd91` / `0de287cb4be9d98f1c24218becce0515c720c681d71b2dfde02edc516731cd91` |
| `03_contention` | 1; same-session lock before observation | Native `SQLITE_BUSY`/`LOCKED` / rejected | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` | `03_contention-01/main/aggregate.sqlite` | `aee2494bdb08f713391250219f4ac30a125cabca5d48af00513d88fdd620ae92` / `aee2494bdb08f713391250219f4ac30a125cabca5d48af00513d88fdd620ae92` |
| `04_cancel_after_observation` | 1; cancellation after observation | Harness plus rollback/reload / rejected | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` | `04_cancel_after_observation-01/main/aggregate.sqlite` | `c5d8d893a9768f9316af0757589fdb6f66d56cad258585db02fa91de6284508e` / `c5d8d893a9768f9316af0757589fdb6f66d56cad258585db02fa91de6284508e` |
| `05_stale_cas` | 1; stale session | Native zero rows plus rollback/reload / known unchanged | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` | `05_stale_cas-01/main/aggregate.sqlite` | `5b528d57cdad0efd4f7e195a29bdb996bc865929bbdc0aecf1141e32d9519ea7` / `5b528d57cdad0efd4f7e195a29bdb996bc865929bbdc0aecf1141e32d9519ea7` |
| `05_stale_cas` | 2; stale revision | Native zero rows plus rollback/reload / known unchanged | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` | `05_stale_cas-02/main/aggregate.sqlite` | `5c1c9a053d80e6535b7ad5817cf9d65fc97b06533bf295213753ec81fb92e4a3` / `5c1c9a053d80e6535b7ad5817cf9d65fc97b06533bf295213753ec81fb92e4a3` |
| `05_stale_cas` | 3; stale digest | Native zero rows plus rollback/reload / known unchanged | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` | `05_stale_cas-03/main/aggregate.sqlite` | `6b5f051cf6d136c5f1f1c1a225884cebbdbf10f81f7027a1bc723ebf291ce082` / `6b5f051cf6d136c5f1f1c1a225884cebbdbf10f81f7027a1bc723ebf291ce082` |
| `06_after_begin` | 1; after begin before CAS | Harness plus rollback/reload / known unchanged | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` | `06_after_begin-01/main/aggregate.sqlite` | `2211a93e5a8aca0e237f71c59bcac7295aa772df38c22ae3418f7a0fddf621bc` / `2211a93e5a8aca0e237f71c59bcac7295aa772df38c22ae3418f7a0fddf621bc` |
| `07_after_stage` | 1; after exact staging before commit | Harness plus rollback/reload / known unchanged | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` | `07_after_stage-01/main/aggregate.sqlite` | `0d1aa4d5fe96f1291e78801fad759e941b29bd743790194713d25c9e79220c45` / `0d1aa4d5fe96f1291e78801fad759e941b29bd743790194713d25c9e79220c45` |
| `08_terminate_precommit` | 1; termination after staging before commit | Native termination/recovery / known unchanged | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` | `08_terminate_precommit-01/main/aggregate.sqlite` | `50b89159e039bd1e2d3c2312120a870db2fcdcef49a2cffc353cbf912ec3bd5a` / `0a533a3dd66cffb5e8c37a937372f95c8d18cc111464f71dc197b4abd38d1e49` |
| `09_rollback_proof_loss` | 1; rollback/result proof lost | Harness loss plus native recovery / `unknown_commit_result` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` | `09_rollback_proof_loss-01/main/aggregate.sqlite` | `d37d200ce3ec270343211fb9050d2798a8f22f957040fc42202beadfbd4864f4` / `2a72d28e0fe942ae9ece65b644034d6fef953f01391b50a7bf90defe87ccbdac` |
| `10_commit_call_loss` | 1; termination inside commit hook before phase one/result | Native commit-hook checkpoint plus termination / `unknown_commit_result` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` | `10_commit_call_loss-01/main/aggregate.sqlite` | `b8bf8e7810404eaae220b0718542ee3d4fb8c31e72feff7c8bd2d916fc2d367c` / `0cb377a6dab30e6815508806f9c091030eec613b26f86c09992ba3d4f2fae5bd` |
| `11_genuine_commit_error` | 1; genuine error attempt | Proven unreachable; control commit / committed | 1 / `538c917b0fc4360a9f1337b5f04a3f0baf9e7436adcc21e1f6374e679587216e` | 2 / `7c0dd65cc2a2850d7fb6dfde8e3bd9142cf26503b74ddcc575fc9c170588e4d8` | `11_genuine_commit_error-01/main/aggregate.sqlite` | `b055c0ed4ddb1432ae08d2200740b7f8bf5ca17134fa03bb09c235df5c0843ff` / `b055c0ed4ddb1432ae08d2200740b7f8bf5ca17134fa03bb09c235df5c0843ff` |
| `12_response_loss` | 1; success then response loss before reload | Harness / `unknown_commit_result` | 1 / `32097cd580bc4bb23bbdb3a84dc6ea953d9233840965514258386a3bf66e5410` | 2 / `fa586651472644b528aff5c042e98cc78284b0508048e4d63d124de173ff895d` | `12_response_loss-01/main/aggregate.sqlite` | `2b994844ee2c4802b8171db997748ff19b33328a1b0163322a0f241793a1c044` / `2b994844ee2c4802b8171db997748ff19b33328a1b0163322a0f241793a1c044` |
| `13_validation_loss` | 1; schema validation result lost | Harness / `unknown_commit_result` | 1 / `604abe29c761b3c1f6fd4d303e2d59e2b97414dac714d3d361a76d28124af792` | 2 / `53f1d118dc76593ec110233467c23e292fe769bf480317957cfc908be043a214` | `13_validation_loss-01/main/aggregate.sqlite` | `ee81208c81f73046d21c7f2337c10a59bb0794dcb867e549a95de4b7b5461bb6` / `ee81208c81f73046d21c7f2337c10a59bb0794dcb867e549a95de4b7b5461bb6` |
| `13_validation_loss` | 2; identity validation result lost | Harness / `unknown_commit_result` | 1 / `95f5a71c86ed8215865cf6880460dc1b954aebfde94d882b4378db91f7379eed` | 2 / `37ba916c7c427488bb0482f5e9490a176a1ec91157a74028748df1c5240fc7e5` | `13_validation_loss-02/main/aggregate.sqlite` | `7ddef3904e5d6dc069cf67261bf9ad995cb12b8c558765f88529481772f1ad2c` / `7ddef3904e5d6dc069cf67261bf9ad995cb12b8c558765f88529481772f1ad2c` |
| `13_validation_loss` | 3; digest/envelope result lost | Harness / `unknown_commit_result` | 1 / `e2351ed2ad87340ac3671ad728083354787307b52e0b021f441ea31f506f2972` | 2 / `0790b0985f1a1d5782a9dc46cca025a204a952c6a5352c0e5147dadccaf57da3` | `13_validation_loss-03/main/aggregate.sqlite` | `e7eb52752ced92f09658dbdc45fdde061186005e6696d13e19ef8719b3c01bb2` / `e7eb52752ced92f09658dbdc45fdde061186005e6696d13e19ef8719b3c01bb2` |
| `13_validation_loss` | 4; sibling validation result lost | Harness / `unknown_commit_result` | 1 / `b9fbd37af1ec09c17bade131abbadd653640c9ce8c4e41ef1f1d457cd89cf9a9` | 2 / `73f98f28c9ba21837cfe1045e7c3cc165a6100030f44c808e6ac04dc777df357` | `13_validation_loss-04/main/aggregate.sqlite` | `b103d13123e1f41e104fc4e4ef23d7e63e861ac49be4bc26bcba23959d9adf19` / `b103d13123e1f41e104fc4e4ef23d7e63e861ac49be4bc26bcba23959d9adf19` |
| `13_validation_loss` | 5; Linux ownership/mode/mount/path validation result lost | Harness / `unknown_commit_result` | 1 / `0ede316defffda98a5b2751ade8a26607433a6721809e5f0e2223694ebb9ec9e` | 2 / `39651e0ab7acd41405c527582dc669fae31372148a959e6b1149a379b4669781` | `13_validation_loss-05/main/aggregate.sqlite` | `ca2c341ea0f3ce0263e1eb5da6ac1a2df4a9d2617d88ec402678133c3bbf6bbd` / `ca2c341ea0f3ce0263e1eb5da6ac1a2df4a9d2617d88ec402678133c3bbf6bbd` |
| `14_close_result_loss` | 1; close result lost | Harness / `unknown_commit_result` | 1 / `308fa6348158fc37f3d4f8e639f63666065850bda7d2c7dd306637e70b71a936` | 2 / `d4ad3b44fc10c58caeed27efbee783e3a7bef2188e88b1e793f17d56eb43f0ae` | `14_close_result_loss-01/main/aggregate.sqlite` | `24e51599db81bcb21a25b3a1d1bf831c34804ee1abe7da982aec96a8192f0f4c` / `24e51599db81bcb21a25b3a1d1bf831c34804ee1abe7da982aec96a8192f0f4c` |
| `15_ack_loss` | 1; acknowledgement lost | Harness / `unknown_commit_result` | 1 / `09d2cabd2f6b285735e8b9206d463c89036333c547a08134d7417f5f31e42877` | 2 / `8711029d8629c4ac052c7a963f7f0d15e99f39a7184eab6a772e67e1febf9c9d` | `15_ack_loss-01/main/aggregate.sqlite` | `7e0e223ecd5bc5d4575db55cfc50ff0d5d566e1e877156d841a10844dd9826ae` / `7e0e223ecd5bc5d4575db55cfc50ff0d5d566e1e877156d841a10844dd9826ae` |
| `16_success` | 1; full validated path | Native / committed | 1 / `2a10f300002f05f312429cfc3c9ee12629fb6c127927be866a23096b15640717` | 2 / `792629f5e19b1b26eb3ca65bb91f19b0f122f2f0b9485236b90ae0615b1f5927` | `16_success-01/main/aggregate.sqlite` | `3e0f13b21495a35f684eea2a1ba4d8bd31009eeff376140dfa2036f1f2927b1b` / `3e0f13b21495a35f684eea2a1ba4d8bd31009eeff376140dfa2036f1f2927b1b` |

The exact aggregate counters were: rows attempted `22`; rows proven natively `7`;
harness-injected application-boundary rows `14`; rows proven unreachable `1`; rows still
unproven `0`; known unchanged `6`; committed `2`; rejected `4`;
`unknown_commit_result` `10`; recovery-only opens `22`; exact old/new recoveries `12` /
`10`; genuine `BUSY/LOCKED` before observation `1`, after observation `0`, and at commit
`0`; different-session commits `1`; rollback attempts/exact-old proofs `6` / `6`; forced
terminations `4`; commit invocations/return observations `12` / `11`; success
acknowledgements `2`. Duplicate commits, retries, replays, repairs, fallbacks, partial rows,
ambiguous pre-commit recoveries, staged-row leaks, revision gaps/duplicates, digest/envelope
mismatches, unexpected siblings/protection widening, and protocol loss/duplication/substitution/
reordering were all `0`.

Every command and response retained exact scenario, cycle, attempt, operation, checkpoint,
database, session, root, and retained-root identity. Strict bounded JSON-line decoding rejected
missing, unknown, duplicate, malformed, multiple-value, substituted, cross-scenario, and
out-of-order data. Children performed ordinary SQLite operations only. No child cleaned, deleted,
renamed, truncated, copied, moved, parsed, or hashed a physical database or journal file. Journal
validation was metadata-only; only the parent test framework removed fixtures after all children and
connections closed.

The canonical-root metadata-tree SHA-256 changed from
`97db40d6fe59408eb182465033f6eed119c76228eb677d0735173f4e058ec9d9` before scenario
creation to `44edd13e72f4edb1be4307e362e74a4e69f776c9af990f77d14df69e92a3798d`
after the matrix. The exact scenario pre/post rollups were
`f6482ec043d37551f246bc6938f350a45ddc0871ef5f2bc6b1ec62e38bd8f96e` and
`2cb51ffafd93743a833d33daad8247b56c859b09660a967acba5843b3cf65b47`.
Each hash covered LF-terminated ordinally sorted rows containing relative path, entry type, byte
size, mode, UID/GID, device major/minor, inode, mount ID, filesystem type/magic, source, and mount
point. Rollups bound scenario ID, attempt, and hash.

**Exact Linux commit call-chain qualification — 2026-08-05.** The reviewed standard-library source
was Go `go1.25.0` under
`/home/jamie/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64`, with
`database/sql/sql.go:2287-2319` and `database/sql/driver/driver.go:519-522`. The reviewed
pinned module source was `modernc.org/sqlite v1.56.0` with `modernc.org/libc v1.74.4` under
`/home/jamie/go/pkg/mod`. The exact path was:

```text
(*database/sql.Tx).Commit
  -> driver.Tx.Commit under database/sql's driverConn lock
  -> (*modernc.org/sqlite.tx).Commit
  -> (*sqlite.tx).exec(context.Background(), "commit")
  -> sqlite3.Xsqlite3_exec
  -> sqlite3.Xsqlite3_prepare_v2
  -> sqlite3.Xsqlite3_step
  -> _sqlite3Step
  -> _sqlite3VdbeExec
  -> _sqlite3VdbeHalt
  -> _vdbeCommit
  -> detect the live write transaction
  -> _sqlite3PagerExclusiveLock
  -> FxCommitCallback / modernc commitHookTrampoline / test callback
  -> _sqlite3BtreeCommitPhaseOne
  -> _sqlite3BtreeCommitPhaseTwo
  -> return through sqlite3_exec, modernc tx.Commit, and database/sql Tx.Commit
```

The driver locations were `tx.go:35-78`, `sqlite.go:618-622`, and
`pre_update_hook.go:53-67,205-215`. The generated Linux/amd64 locations were
`lib/sqlite_linux_amd64.go:82-210` for `Xsqlite3_exec`,
`lib/sqlite.go:11963-12029` for `Xsqlite3_step`,
`lib/sqlite_g_000000000001deab.go:3898-4017,4075-4293` for `_sqlite3Step` and VDBE halt,
`lib/sqlite_g_0000000000003a80.go:18923-19204` for `_vdbeCommit`, its live-write test,
exclusive-lock acquisition, commit callback, and phase calls, and
`lib/sqlite_g_0000000000060000.go:104277-104369` for the two B-tree commit phases.

For row 10 the public `sqlite.HookRegisterer.RegisterCommitHook` callback emitted the strict
`sqlite_commit_hook_entered` checkpoint from inside SQLite after the live write-transaction and
exclusive-lock checks and before phase one or result availability, then blocked without returning.
Only after the parent validated the full checkpoint did it terminate the child. Fresh recovery
returned the exact old row. No timer, sleep, goroutine-start marker, parent kill race, wrapper-entry
marker, debugger, patched dependency, replacement driver, custom VFS, or filesystem/journal
observation substituted for this boundary.

Row 11 remains genuinely unreachable under the selected exclusive shape without changing the
driver, SQLite, filesystem, or protected code: another same-session owner is rejected at lock
acquisition before observation and cannot retain a conflicting lock at commit. Its successful
control commit was genuine; no error was fabricated. Therefore rows still unproven is zero, but the
matrix is not described as closed by pretending the unreachable row executed.

The required unchanged Linux regression cohorts then passed in order:

```text
CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -count=1
PASS; ok in 0.002s
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
PASS; evidence 39.607s; package 39.633s; all seven counters 1000; primary code 5 = 1000; code 6 = 0
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
PASS; evidence 1m22.493s; package 82.516s; old/BUSY/new/journal counts 10000 each
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
PASS; evidence 46ms; package 0.047s; genuine primary SQLITE_BUSY (5); recovery quick_check=ok
```

Contention retained exact same-session final revision/digest `1001` /
`e024c4e5dafc3841e26abbc2df7618f2fd78fcabd3b41bd364485b2ad56ff693` and
different-session final revision/digest `1001` /
`5c78a969b9d47e07ef749d58c6b0fa3311512435141191d79376dd50e2f62f26`.
Its fresh-fixture pre/post metadata hash was equal:
`e19110e6836098f82ffd74fedb6b0ee30b9b7cc771f1aa3305ac4e2423a76c76`.
Publication retained final revision/digest `10001` /
`3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838` and
equal fresh-fixture pre/post metadata hash
`ccaaef3dc1a4eab9ab808bd5ec040fcdedbde14ab4202ad540aee9fb9f362e90`.

The complete test package then cross-compiled with `CGO_ENABLED=0` into the separate newly
verified private directory `/tmp/dockpipe-sqlite-linux-failure-cross-ZeqVuuAu`, outside the
repository with mode `0700` and owner `1000:1000`. `go version -m` confirmed exact embedded
settings for Windows/`amd64`, Linux/`amd64`, and macOS (`darwin`)/`arm64`. The binaries
were respectively `12,087,808`, `11,861,347`, and `10,953,058` bytes. The caller removed
those three exact binaries, then their empty parent. It also removed the now-empty native-fixture
parent `/tmp/dockpipe-sqlite-linux-failure-0a13OJYL`. Neither directory, nor any fixture, test
child, binary, or evidence artifact remained. Cross-compilation is compatibility evidence only; it
is not native Windows or macOS evidence.

The protected Windows matrix remains unchanged and independently qualified with 57 options,
`COMPILER=gcc-12-win32`, `MUTEX_NOOP`, and `OMIT_SEH`. Linux remains independently
qualified with 56 options, `COMPILER=gcc-12.2.0`, and `MUTEX_PTHREADS`. Linux smoke,
publication, and contention remain separate qualifications. This matrix does not qualify Linux
reboot/power-loss durability, production storage, migration, cutover, recovery authority,
dispatch/projection integration, Slice 2, or macOS. Linux reboot/power-loss remains open and macOS
remains intentionally last. TASK-013 and CAS-14 remain open.

#### Linux VM reboot/power-loss package foundation (2026-08-05)

The package-owned foundation needed before a controlled Linux VM trial is now implemented under
`packages/vm/**` as VM package version `0.8.0`. It adds the `linux-vm` workflow and a separate
`LinuxQemuVmResolverConfig` root alongside the unchanged Windows model, keeps `runtime: vm` and
resolver `qemu`, and does not add VM-product policy to `src/**`. The Ubuntu 24.04 LTS amd64 profile
pins the immutable `20260801` cloud-image URL and SHA-256. It requires local NoCloud, disables
qualification networking and SSH, performs no `apt` update or upgrade, requests no additional debs,
uses only the XDG cache/state/config/runtime layout, and has no checkout-generated-state fallback.

The offline qualification manifest fails closed unless it identifies a disposable KVM guest with
host CPU, two vCPUs, 4096 MiB, no swap, distinct host/guest identities, exactly one private OS clone
and one private 4 GiB sparse raw data disk, and no physical disk, passthrough, share, extra disk,
network, SSH, or arbitrary command surface. The whole-device ext4 tuple fixes UUID mounting at
`/var/lib/dockpipe-qualification`, disables lazy inode/journal initialization, and requires exactly
`rw,noatime,nodev,nosuid,noexec,data=ordered`. Package Go sources provide per-instance Ed25519 keys,
mutual pinning, bounded length-prefixed canonical JSON, signed identity and phase context, replay and
substitution rejection, recovery-only pending-ticket semantics, safe QMP parsing, exact process
authorization, inert pidfd/SIGKILL planning, exact QEMU/block argument planning, trial isolation, and
exact cleanup planning. The current Windows guest agent remains the active Windows path; there is no
cutover in this slice.

The offline Gate 2 prerequisite is now implemented as a strict typed provisioning contract for
exactly one disposable qualification instance. It requires the pinned XDG cache image path, byte
size, regular non-symlink owner-only file, and SHA-256; rejects checkout, `.dockpipe`, `.dorkpipe`,
relative, overlapping, and pre-existing generated roots; and reserves fresh run, cohort, machine,
disk, filesystem, nonce, and Ed25519 identities exclusively without replacement. Fresh keypairs are
generated before authorization, their public hashes are bound into the contract and plan, and
reservation accepts only those same keys. The controller
deterministically emits the closed set of inert operations for the private OS clone, private 4 GiB
sparse raw data disk, reviewed NoCloud rendering and seed, hash-pinned assets, stable format/mount,
QEMU launch, signed verification, controlled shutdown, failure preservation, and exact later
cleanup. Planning invokes no subprocess and always emits `execute=false`. A distinct short-lived
authorization can bind only to the exact contract and plan digest; it does not add an executor.
The separate package authorization template defaults to `approved=false` and therefore grants no
authority until a later reviewed gate supplies both exact digests and a fresh bounded lifetime.

The reviewed NoCloud and systemd assets are now exact renderer inputs rather than design-only
placeholders, and their six source hashes are compiled into the controller and checked during both
planning and rendering. Rendering pins the controller and guest binaries and both Ed25519 public identities,
disables network and SSH, performs no apt update or upgrade, installs no package, fixes the exact
data-disk serial, filesystem UUID, mount path, and options, and exposes no user-provided command
field. The systemd-referenced guest-agent service mode is implemented with the existing canonical,
signed, length-prefixed protocol, accepts only signed `request` frames, and exposes only the five
reviewed capabilities. Identity, health, and
binary-pin verification respond; checkpoint and recovery validate their reviewed signed payload
shape and then fail closed because this Gate 2 foundation does not own a harness adapter. Invalid framing,
signature, public-key or binary pins, freshness, replay, sequence, nonce, identity, capability, and
payload substitutions fail closed. Windows workflow behavior remains unchanged.

Validation for this foundation covers package Go tests with `CGO_ENABLED=0 -mod=readonly`, strict
protocol negatives, fake filesystem and socket behavior, manifest/isolation failures, XDG paths,
two-disk and QEMU tuples, cleanup identity, legacy Windows workflow validation, both workflow/model
compiles, Linux/amd64 controller and guest-agent builds, and Windows/amd64 shared guest-agent source
compatibility. Cross-compilation is compatibility evidence only. It is not a Linux VM run, native
Windows evidence, or macOS evidence.

No image was downloaded or modified in this slice. The existing Gate 1 cache artifact was verified
read-only at the pinned path, type, ownership, mode, byte size, and SHA-256. No VM, disk, filesystem,
NoCloud seed, guest service, process, SQLite
fixture, or evidence tree was created, cloned, modified, attached, started, stopped, rebooted,
provisioned, killed, destroyed, or cleaned. No QMP power command or real signal was issued. Therefore
Linux reboot/power-loss remains open, macOS remains intentionally last, TASK-013 remains open, and
Slice 2 has not started.

The remaining live work stays split into separate maintainer approvals:

1. create/provision one disposable VM and install the hash-pinned agent assets;
2. run a non-destructive identity, controller, and recovery dry run;
3. run one bounded Linux destructive cohort and preserve/read back its complete evidence; and
4. consider a separate Windows VM gate later. macOS remains the final platform gate.

#### Linux VM task-owned executor/toolchain contract (2026-08-06)

The missing offline contract between the inert Gate 2 provisioning plan and any future live runner
is now implemented under `packages/vm/**` as VM package version `0.9.0`. “Package-owned” here means
the VM-specific source, policy, templates, and tests remain in the VM package. It does not wire the
task into DockPipe package installation, release, registry, signing, global-store, or version
resolution. Those package-layer capabilities remain separate backlog work. The future QEMU bundle
is instead a separately prepared task-owned local artifact, like the pinned image and task-owned
controller/guest builds.

The provisioning and plan contracts are now v2 and bind an exact absolute bundle root plus its raw
manifest SHA-256. The new `dockpipe.vm.toolchain.v1` manifest accepts only QEMU `11.0.3` on
Linux/amd64 with KVM, exactly `qemu-img`, `qemu-system-x86_64`, and a non-empty complete
runtime-library/ROM/data closure. It pins the official source and signature URLs, release-manager
fingerprint, source-archive hash, build-recipe hash, exact relative paths, version output, file
hashes, and finalized owner-only read/execute modes. Validation rejects checkout, `.dockpipe`,
`.dorkpipe`, VM instance/evidence/config/runtime overlap, symlinks, extra or missing files, widened
modes, changed hashes or versions, substituted tools, and fallback lookup. The bundle root and all
directories are finalized mode `0500`, the manifest/runtime data are `0400`, and executables/loaders
are `0500`.

The exact OS clone command is the bundle-pinned `qemu-img create -f qcow2 -F qcow2 -b <pinned
source> <fresh private target>` with a 120-second bound and no alternate tool. The Go controller owns
exclusive mode-`0600` 4 GiB sparse raw creation and deterministic `dockpipe-go-iso9660-v1` NoCloud
construction, so `cloud-localds`, `xorriso`, and `genisoimage` are absent. QEMU startup is bounded to
120 seconds, controller-signed/guest-signed identity, health, and launch-pin verification to 60
seconds, and QMP `system_powerdown` to 120 seconds with no fallback signal.

The new `dockpipe.vm.executor.v1` contract is deterministically derived from only the exact
authorized contract, plan digest, immutable toolchain manifest, QEMU argv, and reviewed NoCloud
rendering. Its injected runner has typed methods only for private clone, sparse raw disk, NoCloud
seed, QEMU launch, signed verification, controlled shutdown, preservation, and cleanup. It has no
generic command, shell, environment, network, SSH, passthrough, share, physical-disk, or fallback
surface. Any failure stops once, performs no retry or cleanup, and requests preservation of the
complete instance/evidence/config/runtime roots. Cleanup is never automatic and requires a separate
fresh authorization bound to the contract, plan, executor digest, run/cohort, and exact ordered
resource list.

Only fake runners exist and tests launch no subprocess. There is no `os/exec` adapter or live CLI
execution flag. Gate 1 materialization completed on 2026-08-07 using the exact QEMU 11.0.3 source
archive SHA-256 `da5fcffc32762820568b828ed430a728864d34d50b6d2f30358597760cbb0523`,
detached-signature SHA-256 `719f32c491ee724629f7d5918a6ff04ddc115d92a597b504cc4f12191e4a5e77`,
signer `CEACC9E15534EBABB82D3FA03353C9CEF108B584`, and pinned builder manifest
`sha256:9108d3cbdacbaf442f8b8938a2e94a7cdf04c0b093953866726c5734cb478f2e`.
The builder configuration digest is
`sha256:ae716e47ccf0cde02ef2b290116ddc2a7c66ac0a912a6f1b74f28a5670a3dd21`,
its complete 36,551-entry inventory SHA-256 is
`ecb649e86e299e6dd0e569f15a2c4fa207e6dc03bcddf540460453b819a48cb5`, and the
reviewed recipe SHA-256 is
`669021bd42c5a47c7173821e68ec9e37143c7406e9093338318504e79b502a69`.

Two independent no-network builds produced byte-identical 125-entry output inventories with
SHA-256 `22f24ba020b98b0802d67956bd5d7699bcd9d12a99773e185165087b8b1aedec`.
The immutable `jamie:jamie` bundle is
`/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1`; its
`toolchain.json` SHA-256 is
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`.
`qemu-img` is `8f136e6f9550ca0c4d0bed73c7fb761537425c4bd0e4f95c0fd8ee93b6b2ed81`, and
`qemu-system-x86_64` is
`3544680aaeaf8087bbf3ef693ff185c2691831560c767672defccd784ec37140`.
The exact bundle-owned musl interpreter, literal `$ORIGIN/../lib` RPATH, `NODEFLIB`, recursive
declared library closure, owner-only modes, absence of symlinks/writable/group/world entries, and
absolute-path `env -i` version/help execution were verified. The exact checked-in manifest and build
evidence contain no replacement markers.

Gate 1 is complete. Gate 2 and Gate 3 have not started; no VM, VM disk, NoCloud seed, live root,
QEMU process, socket, or cleanup action was created. A separately reviewed slice may add a real
runner and Gate 2 execution prompt later.

Offline validation passed the VM package test (including the new executor and toolchain negatives),
workflow and resolver compiles, `CGO_ENABLED=0 go test -mod=readonly ./...`, `go vet
-mod=readonly ./...`, two byte-identical Linux/amd64 `-trimpath -buildvcs=false` builds, and the
Windows/amd64 guest-agent compatibility build. The new controller SHA-256 is
`ccefd4daaa2394748b08c5f3ec21efe5298aba848b4b819b1b491aa2287c6549`; the unchanged guest
agent retains SHA-256 `cb99865a1f628083a0c732341dddff1c0ecbb6ba5609a55fd78ed3a4bee3856f`.
The two Linux builds matched byte-for-byte. Cross-compilation remains compatibility evidence only.
No VM, disk, NoCloud seed, XDG VM root, process, socket, or live cleanup was created or operated.

#### Accepted Linux guest boot-identity bootstrap (2026-08-07)

The blocked boot-ID decision is closed with a guest-first signed identity frame. A predetermined
kernel boot ID and an unsigned identity exchange are both rejected. Qualification manifest v2 no
longer accepts a pre-launch `boot_id`; it fixes only the reviewed source
`/proc/sys/kernel/random/boot_id`. Provisioning/plan/live-authorization v3 renames the already fresh
32-byte value to `bootstrap_nonce` and binds it, the per-instance Ed25519 key pins, static
machine/disk/run/scenario/durability identities, and rendered NoCloud bytes into the existing sealed
contracts. Guest-agent config v2 receives that exact nonce and source.

On opening virtio-serial, the guest reads the kernel value and writes before it reads. Its first and
only bootstrap is a canonical length-prefixed `dockpipe.vm.v2` frame with kind `bootstrap`,
capability `identity/v1`, sequence 1, phase `bootstrap`, the sealed bootstrap nonce, the actual boot
UUID, every static authenticated context field, and a payload containing the boot-ID source plus the
controller-public, guest-public, controller-binary, and guest-binary SHA-256 pins. The frame is
signed by the pinned guest private key. There is no unsigned frame, controller-signed frame without
a boot ID, alternate framing path, or fallback identity authority.

The future controller must read first. Within the existing 60-second bound it verifies canonical
framing, time window, pinned guest signature, bootstrap nonce, sequence/phase, static context, boot
UUID, source, and all four pins. Before writing any controller request, it exclusively creates
mode-`0600` `bootstrap.json`, records the verified frame and learned boot ID, and fsyncs the file and
parent evidence directory. Existing evidence or any verification/write/fsync failure stops once,
preserves the complete roots, and permits no retry, reconnect, fallback, or cleanup. The first
controller-signed `identity/v1` request then uses the authenticated boot ID at sequence 2 with a new
nonce; later requests are contiguous and cannot reuse the bootstrap or another request nonce.
Guest-signed results echo the request context. This second signed exchange proves the guest's pinned
controller identity and preserves mutual Ed25519 pinning.

The offline implementation covers protocol framing and negative verification, guest-first service
ordering, manifest/config rendering, sealed executor-v2 fields, and tamper rejection. It adds no
production runner or generic command surface. No VM or Gate 2 operation is authorized by this
decision, and no Gate 2 execution prompt is emitted here.

Post-decision offline validation passed the complete VM package Go suite, `go vet -mod=readonly
./...`, the VM package test, both Linux and Windows workflow validations, VM workflow/resolver
compiles into an isolated temporary workdir, two byte-identical Linux/amd64
`-trimpath -buildvcs=false` builds, and a Windows/amd64 guest-agent compatibility build.
The current source builds are controller
`f0f6b17ab730dc69d3638a39cf8dfb082cc8d288f2257c3cbd97ba38cf5d509d`, Linux guest agent
`3f9354ff666a21a5b1fc05b2089ffe523fe4123a5d3ef04968c6af00ac66a328`, and Windows guest agent
`a0666c4e00b1725944ffbe75f8fa3e9a26f9971d6e3710a66ceff74f1a1f5957`. These temporary builds
supersede the earlier source-build hashes for this uncommitted protocol revision but do not alter the
immutable Gate 1 QEMU bundle or authorize publication or live use.

#### Linux VM production qualification runner (2026-08-07)

The package-owned production runner is implemented under `packages/vm/**` as VM package version
`1.0.0`; `src/**` remains unchanged. The accepted pre-authorization identity-material decision uses
one new mode-`0700` task staging root outside the checkout and all live XDG roots. Preparation
generates the 32-byte bootstrap nonce plus controller and guest Ed25519 keypairs in memory,
exclusively writes exactly five mode-`0600` files, fsyncs every file and directory, and emits only
non-secret identity metadata, the nonce, and public-key hashes. The later live invocation strictly reloads that inventory, verifies
keypair integrity and the provisioning-v3 pins, durably reserves the same keys under the final
configuration root, and consumes the staging copy only afterward. A failure before durable
reservation preserves the staging bundle; a later failure preserves the final configuration root.
The staging descriptor expires after exactly 24 hours; expiry performs no automatic deletion or
fallback and requires explicit removal plus fresh preparation.

The controller CLI now separates identity preparation, inert planning/authorization review, typed
qualification execution, and separately authorized cleanup. The production adapter exposes no
generic command method and gives both subprocesses an empty environment. It revalidates the pinned
source and absolute binaries, runs only the sealed `qemu-img` clone argv, creates the exclusive
mode-`0600` 4 GiB sparse disk in Go, builds the deterministic `dockpipe-go-iso9660-v1` seed in Go,
launches only the pinned QEMU argv, and records the exact owned process identity. It reads and verifies
the guest-signed bootstrap before exclusively creating and fsyncing `bootstrap.json`, then sends
controller-signed identity, health, and launch-pin requests beginning at sequence 2 with fresh
contiguous nonces. Every guest result must echo the complete request context. Shutdown negotiates QMP
and sends only `system_powerdown`; there is no fallback signal. Any failure stops once, fsyncs the
complete roots, and performs no retry or cleanup. Cleanup requires a new owner-only authorization
matching the exact contract, plan, executor digest, run/cohort, and ordered resource list, and refuses
an active recorded QEMU process.

Offline proof passed focused executor/protocol/guest/identity/QMP tests, `CGO_ENABLED=0 go test
-mod=readonly ./...`, `CGO_ENABLED=0 go vet -mod=readonly ./...`, the VM package test, Linux and
Windows workflow validation, and workflow/resolver compilation into an isolated `/tmp` workdir. Two
independent Linux/amd64 `-trimpath -buildvcs=false` builds with separate caches were byte-identical.
The controller SHA-256 is `a079350a68649c2350122fe81d4617d13aebb4c09dec960cc3279ce196002fa8`;
the Linux guest-agent SHA-256 is
`df1a55c45ddcb367e803129e712bb2c926c4c5c5f0a42c5be9e1c5a2632ace96`; and the Windows/amd64
compatibility build is `5858a6cc18d89f1a6bdd2a6bb75515c5c628d62cc65cd08e2892d94bcb65e1f9`.
No QEMU process, real VM disk, NoCloud seed, live XDG root, QMP command, signal, cleanup, Gate 2, or
Gate 3 action occurred. Gate 2 remains a separate explicit execution approval.

#### Linux VM Gate 2 launch-path correction (2026-08-08)

One exact Gate 2 invocation was authorized for run
`gate2-run-cbc0d22e-ae56-4f80-aaf0-92b6b02531e3`, cohort
`gate2-cohort-567199c4-312f-439e-8f83-f694b34d76e1`, contract SHA-256
`b201995ab497d3f131cd899418a46097c2b2f4f84b886d61297e567c6794f01a`, and plan SHA-256
`cbc9d8b7cd376187ca8eea69ab6bea3ef9aed3f175fd359f3bc6aaa2a4878418`. It ran once and failed
closed at `launch-qemu`: QEMU exited with status 1 before its exact sockets became ready. No retry,
reconnect, fallback signal, private-payload inspection, or automatic cleanup occurred. All four
owner-only live roots were preserved and the exact owned QEMU process count was zero.

Offline read-back proved the authorized QMP pathname was 174 bytes and the agent pathname was 176
bytes. Linux pathname Unix sockets have only 107 usable bytes in `sockaddr_un.sun_path` once the
terminating NUL is reserved, so the authorized launch plan could not create either socket. A
separate exact cleanup authorization bound to executor SHA-256
`6280d29d100076181f55ebd54fd1c0fba1deeab06b34487f09dc8acb4d0ccfc5` ran once and removed the
stored executor's ordered 11-resource list. Narrow read-back confirmed every authorized path absent
and no owned QEMU process active; the checkout and the separate `/tmp` review/build root were not
cleanup targets.

The package-owned correction now rejects QMP or agent pathname sockets longer than 107 bytes during
inert QEMU plan construction, before plan authorization, identity reservation, or any subprocess.
Exact boundary tests accept 107 bytes and reject 108 bytes; provisioning coverage proves long but
otherwise schema-valid run/cohort identities fail before authorization. Existing tests that used
long framework-generated temporary runtime paths now use short, unique, platform-absolute runtime
roots while their disk, toolchain, and artifact fixtures remain isolated. The corrected controller
rejects the preserved failed contract offline with the exact safe error class
`QMP Unix socket path is 174 bytes; Linux pathname sockets permit at most 107`.

Post-correction offline validation passed focused manifest/provisioning tests, the complete
`CGO_ENABLED=0 go test -mod=readonly ./...` suite, `CGO_ENABLED=0 go vet -mod=readonly ./...`, the
VM package test, and workflow/resolver compilation into an isolated `/tmp` workdir. Two independent
Linux/amd64 `-trimpath -buildvcs=false` builds were byte-identical. The current controller SHA-256
is `f49ac43a78b3589c1375ab2c67c664be42a78140b43fa1919cc1e48df1dc2984`; the current Linux guest
agent SHA-256 is `fa83a65b89d76303e808578ba7b872a33f6bd6c2d122c9ca3ba8174d531fd8f6`; and the Windows/amd64
guest compatibility build is `677a71cd966599bb8d01a8481b107fd03b1b2b06f5fe571636c139d9c2e611e8`.

Gate 2 is not qualified and was not retried. Any future attempt must start from fresh run/cohort,
machine, filesystem, disk, nonce, key, staging, build, input, contract, plan, authorization, live-root,
socket, disk, and evidence identities. Gate 3 remains blocked behind a successful separately
authorized Gate 2.

#### Linux VM Gate 2 verify-guest failure and first-boot observation wiring (2026-08-08)

After the launch-path correction, one fresh qualification invocation ran for run
`g2r-970fd15c42e793bb`, cohort `g2c-6013982ee1e49710`, contract SHA-256
`01bf24d6f792608ed9c124e737ac175efa4816ed7a8bbfc001931eba25c61d2a`, plan SHA-256
`7565df9f071d21751b87d9ee46c785bb0e6210a5161adf2718b9296bd99b247c`, executor SHA-256
`94c258b22714a9d3ab6a57a66753ee28e3d69fe40e55d84eb3fedc3f3eb672bc`, toolchain SHA-256
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`, and bootstrap nonce
`c50fcbac0ec1f1f6b79278a9f433807bf6f2336f0c2ed8fa23d6d0d56c2124c7`. It ran once and failed
closed at `verify-guest` with
`read unix @->/run/user/1000/dockpipe/vm/g2r-970fd15c42e793bb/g2c-6013982ee1e49710/g2r-970fd15c42e793bb.agent: i/o timeout`.
The exact live authorization is spent and permanently non-reusable.

QEMU created both exact Unix sockets, and the controller connected to the agent chardev. The
executor created one 60-second verification context and the Linux runner copied that deadline to the
Unix connection. `protocol.ReadFramed` timed out before a complete four-byte-length-prefixed signed
bootstrap. Bootstrap verification, `bootstrap.json`, `verification.json`, and every controller
request were never reached. `qemu.log` was empty; `shutdown.json` was absent; `qemu-img.log` recorded
successful private-clone creation; and the recorded QEMU PID was no longer active. The complete
owner-only instance, evidence, configuration, and runtime roots remain preserved with their clone,
raw disk, NoCloud ISO, seed tree, final identity material, sockets, and executor contract.

A separately authorized offline forensic artifact with SHA-256
`e90fec92a46fb6e3e21ddb923b8a960866bb72437507ef9da92b97b04104fe68` ran once through network and
mount namespaces using kernel NBD read-only and ext4 `ro,noload,nodev,nosuid,noexec` mounts. It
uniquely identified `/dev/nbd15p1` as root, found `/var/lib/cloud` absent, found no cloud-init
`status.json`, `result.json`, or `boot-finished`, found no matching persistent-journal agent entries,
and found no DockPipe-specific udev ownership/mode rule. It did not record the actual runtime
virtio-port ownership or mode. NBD and mounts were detached; the preserved disk metadata tuple was
unchanged. The owner-only report SHA-256 is
`504e8e68acc91ace97eba74a676c1e9675d5a5c1d13a216ed93a27a3ad0e7565`. Earlier v1-v3 forensic
authorizations are spent and non-reusable.

The evidence therefore leaves unprivileged virtio-port access plausible but unproven. The earliest
missing observable milestone is cloud-init/first-boot state, before any evidenced agent-service
attempt. It does not establish a service, device-permission, cloud-init, or boot cause.

The bounded offline production-wiring slice is now implemented under `packages/vm/**` as VM package
version `1.1.0`. `dockpipe.vm.first-boot-observation.v1` is bound into every fresh deterministic
provisioning plan and digest, the sealed executor-v3 digest, the exact QEMU launch argv, the typed
Linux runner, and the new cleanup resource enumeration. QEMU exposes only the existing
`isa-serial/ttyS0` byte stream as a one-shot Unix client with reconnect disabled. Before launch, the
controller creates the exact Unix listener and exclusively creates the cohort
`first-boot-console.log` as mode `0600`; QEMU is never pointed at an ordinary output file.

The controller-owned sink retains exactly the first 4 MiB, fails closed if another byte arrives,
preserves the prefix, and propagates capture, file-sync, parent-directory-sync, and close errors. The
runner deterministically closes and joins capture before guest verification returns and again guards
shutdown and failure preservation. Planning and executor validation bind the evidence/runtime roots,
paths, source, transport, client/listener roles, no-reconnect setting, mode, exclusive creation, cap,
overflow policy, fsync policy, and stop/join requirement. Offline negatives reject path, transport,
mode, cap, replacement, reconnect, lifecycle, operation, and argv substitution before a runner can
be reached. No NoCloud/user-data or guest asset changed, and no private seed/key payload read,
network, generic command, shell, retry, reconnect, fallback, signal, or automatic cleanup was added.

Compatibility is explicit: fresh qualification execution requires executor-v3 and the exact
observation policy, while a stored executor-v2 contract with no observation field retains its
original digest and remains loadable only for separately authorized exact cleanup. It cannot regain
qualification execution. The checked-in live authorization template remains `approved=false`, the
reviewed plan remains `execute=false`, and this slice minted no identity or authorization and created
no live root, disk, seed, QEMU process, socket, or evidence. It did not read or change the preserved
Gate 2 instance. Gate 2 is still unqualified. Renewed source builds/hashes and one fresh separately
authorized Gate 2 invocation are the next live gate; Gate 3 remains blocked.

Offline validation passed `git diff --check`, the complete `GOWORK=off CGO_ENABLED=0 go test
-mod=readonly ./...` and `go vet -mod=readonly ./...` suites, the VM package test, Linux and Windows
workflow validation, and workflow/resolver compilation into a fresh isolated `/tmp` workdir. Focused
observation lifecycle tests also passed under the race detector and across ten repeated runs. They
cover deterministic plan binding, exact QEMU transport, path/mode/cap/overflow substitution,
exclusive owner-only evidence, exact-cap and one-byte-overflow behavior, file and directory sync
and close error propagation including listener-setup failure, goroutine joining, file-descriptor
closure, pre-authorization rejection, and independently checked historical executor-v2 digest and
cleanup compatibility. The sandbox prohibits binding a real Unix socket, so the
listener-ownership unit uses an injected inert listener; no QEMU process or VM was used. No build
intended for live use was produced.

A separately authorized offline source-build/review gate then used the fresh private root
`/tmp/dockpipe-vm-source-review.ZStX82CE`. The complete current source diff and both target dependency
closures were reviewed; every non-standard build input resolves under `packages/vm/tools/**`, with
no `src/**` input. Two independent Linux/amd64 lanes used separate build caches, temporary
directories, and output directories with Go 1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `-trimpath`, and
`-buildvcs=false`. Both controller outputs were byte-identical at SHA-256
`b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc`; both guest-agent outputs
were byte-identical at SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`. The separate Windows/amd64
guest-agent compatibility build has SHA-256
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`; it is cross-compilation
evidence only.

Fresh Go build/temp-cache validation passed `git diff --check`, the complete VM Go test and vet suites, focused
observation race tests across ten repeated runs, the VM package test, Linux and Windows workflow
validation, and workflow/resolver compilation into separate isolated workdirs under that task root.
The build outputs remain only under the offline task root and were not promoted, copied into any live
or preserved root, or used to prepare identity, provisioning, plan, or authorization material. No
preserved Gate 2 root was accessed, no live artifact or socket was created, and no VM, cleanup,
Gate 2, or Gate 3 action ran. These hashes are reviewed deterministic source-build evidence only;
any fresh Gate 2 invocation still requires another explicit authorization.

#### Linux VM reviewed source-build offline-promotion contract (2026-08-08)

The missing boundary between deterministic source-build evidence and Gate 2 inputs is now a
separately authorized offline promotion. This decision records the already reviewed source root
`/tmp/dockpipe-vm-source-review.ZStX82CE` without re-reading it. The reviewed repository checkpoint
is `15f0ea3f027877221b78f637a00ab010d0a8be1d`; both Linux/amd64 build pairs were byte-identical;
the controller is `5447054` bytes with SHA-256
`b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc`; the guest agent is
`3870038` bytes with SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`; and the Windows/amd64
guest-agent compatibility output has SHA-256
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`. The builds used Go
1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOARCH=amd64`, `-trimpath`, and
`-buildvcs=false`, and all non-standard build inputs resolve below `packages/vm/tools/**`. The
Windows artifact is compatibility evidence only and is never promoted for Linux Gate 2.

The fixed non-live namespace is `/home/jamie/.local/share/dockpipe-vm-gates`. It is task-owned VM
qualification input, not the checkout, DockPipe global package/install root, `.dockpipe`,
`.dorkpipe`, an image/toolchain cache, a live instance/evidence/configuration/runtime XDG root, or
any preserved Gate 2 root. Promotion IDs match `vmp-[0-9a-f]{16}`. Each future gate must provide
the exact ID and all source and destination paths before execution and may not discover, substitute,
increment, or fall back to another destination. The proposed first promotion is
`vmp-2026080815f0ea3f`, with exact documentation-only paths:

- promotion root:
  `/home/jamie/.local/share/dockpipe-vm-gates/promotions/vmp-2026080815f0ea3f`;
- evidence directory:
  `/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f`; and
- evidence file:
  `/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f/promotion.evidence.json`.

The promotion root is exclusively created mode `0700`, owned by the effective promotion user and
that user's primary group. Its closed inventory is exactly two regular, non-symlink, single-link,
mode-`0500` files with that same owner and group: `dockpipe-qemu-controller`, copied from the
reviewed `linux-a` controller output with the size/hash above, and `dockpipe-guest-agent`, copied
from the reviewed `linux-a` guest output with the size/hash above. Both are static Linux/amd64 ELF
executables, created exclusively without following links, immutable by lifecycle after success,
and read-only inputs to later Gate 2 work. No directory, manifest, evidence, Windows binary,
temporary file, cache, log, key, authorization, or other artifact may remain inside the promotion
root. The corresponding `linux-b` outputs must be compared byte-for-byte against `linux-a`
immediately before promotion; they are comparison inputs only and are not copied.

Before publishing, the gate verifies every fixed namespace component is an owned non-symlink
directory and rejects checkout, generated-store, global-install, live-XDG, preserved-root,
source-review-root, and mutual overlap. Missing namespace directories are created one component at
a time as mode `0700`. The exact promotion root and evidence directory are exclusively created;
either task-owned destination already existing fails closed. No fallback, alternate lane,
overwrite, rename-over-existing, retry, repair, replacement, or automatic cleanup exists. Every
created directory and file is owned by the effective promotion user and that user's primary group.

The exact durability sequence is:

1. revalidate both Linux pairs, hashes, sizes, types, modes, ownership, link counts, and embedded Go
   build metadata;
2. exclusively create the mode-`0700` promotion root and synchronize its parent directory after
   publishing the new directory entry;
3. exclusively create each destination without following links as mode `0500`, copy exact
   `linux-a` bytes, synchronize the file, close it, reopen without following links, and verify its
   identity, type, size, ownership, mode, link count, SHA-256, and embedded Go metadata;
4. verify the promotion root's exact closed inventory and synchronize that directory;
5. exclusively create the mode-`0700` evidence directory and synchronize its parent;
6. exclusively create mode-`0600` `promotion.evidence.json`, synchronize it, reopen and verify it,
   then synchronize the evidence directory; and
7. immediately read back both roots and report the evidence-file SHA-256.

Any failure stops once and preserves the exact partial promotion and evidence roots. It performs no
retry, replacement, repair, fallback, or cleanup. A failed promotion ID is prohibited from reuse;
inspection or cleanup is another separately authorized exact offline gate.

Canonical evidence uses schema `dockpipe.vm.offline-promotion.v1`, stable snake-case field names,
and ordered artifact entries. It records `schema`, `promotion_id`, `repository_checkpoint`,
`source_review_root`, exact `linux_a_sources` and `linux_b_comparison_sources`, successful
`byte_comparisons`, `build_provenance` (`go_version`, `gowork`, `cgo_enabled`, `goos`, `goarch`,
`goamd64`, `trimpath`, and `buildvcs`), `promotion_root`, `evidence_path`, `effective_uid`,
`effective_gid`, `promotion_root_mode`, `evidence_directory_mode`, and ordered two-entry
`promoted_inventory`. Each inventory entry contains `relative_name`, `absolute_path`,
`source_path`, `sha256`, `byte_size`, `file_type`, `mode`, `uid`, `gid`, `link_count`,
`go_package_path`, and `go_build_settings`. The remaining stable fields are
`windows_amd64_compatibility` with `promoted: false`, `file_syncs`, `directory_syncs`,
`closed_inventory`, `package_engine_boundary`, and `actions_performed`.

`actions_performed` explicitly records `false` for identity preparation, live input, plan,
authorization, disk, seed, socket, QEMU process, Gate 2, cleanup, and Gate 3. Evidence never contains
secrets, private keys, live authorization material, or preserved-root contents. The promotion gate
records the exact embedded `GOAMD64`, Go package paths, and build settings from immediate
revalidation rather than assuming or substituting them.

A successful promotion is immutable task-owned input for a later separately authorized Gate 2
preparation/execution chain. Gate 2 references it read-only; it does not consume, mutate, silently
replace, automatically expire, delete, or include it in qualification cleanup. Removal requires a
separately authorized exact offline-promotion cleanup gate. A later preparation gate, not this
decision, binds the promoted paths into a task-owned provisioning input outside the checkout. The
machine-specific path is intentionally absent from the checked-in provisioning template.

This docs-only gate created no external root, promotion output, or promotion evidence and granted no
Gate 2 authority. It did not inspect the source-review root or any preserved Gate 2 root. It did not
prepare identity or live input, emit a plan or authorization, create a disk, seed, socket, or QEMU
process, execute Gate 2, perform cleanup, or begin Gate 3. Deterministic source-build review, offline
promotion, Gate 2 preparation, Gate 2 live authorization/execution, cleanup, and Gate 3 remain
separate gates. The package/engine boundary remains preserved with no `src/**` change.

#### Linux VM promotion, fresh Gate 2 evidence, and deadline correction (2026-08-08)

The separately authorized promotion `vmp-2026080815f0ea3f` completed once under
`/home/jamie/.local/share/dockpipe-vm-gates`. Its immutable closed inventory contains only the
mode-`0500` Linux/amd64 controller and guest agent with the reviewed hashes above. Canonical evidence
is stored at
`/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f/promotion.evidence.json`
with SHA-256 `c411a6cfa326d61c6bfd9663a7f063d21dcb364520c2274fb3fe34d1f951889b`.

Fresh offline preparation then produced run `g2r-e58b5061e0e69e7e`, cohort
`g2c-b725086f6d664d7d`, provisioning contract SHA-256
`a47cd2f0f9cac67770add46fdf687b67bfc75301f75c6e143d6e45e630d95ce3`, and plan SHA-256
`8f2bbc6418315248fd15220cfd7998dabb2e453dd087cbe5460e9aaba7ac53c5`. One exact live
authorization ran once and is permanently spent. Identity staging was consumed only after durable
reservation, and all four fresh owner-only XDG roots were created. The controller invocation lost
supervision while its exact QEMU child remained active, so no retry, signal, reconnect, fallback,
cleanup, or second live authorization occurred. A sandbox PID-namespace read initially and
incorrectly appeared to show that PID absent; the later escalated host read proved the original QEMU
still active with exact recorded PID `1884350`, start ticks `7105843`, executable SHA-256
`3544680aaeaf8087bbf3ef693ff185c2691831560c767672defccd784ec37140`, and sealed command SHA-256
`86ef04336070f9645355193318a64f368ba7752fa68790b5e5db7a974f6af6d8`. Cleanup correctly refused
the active process without deleting any resource.

The owner-only `first-boot-console.log` captured 58,824 bytes. It proves the exact pinned Ubuntu
guest reached `cloud-init-local.service`, `network-pre.target`, `systemd-networkd.service`, and
`network.target`, then remained in `systemd-networkd-wait-online.service` through the end of capture.
An offline read-only extraction of the exact pinned source image proved that
`systemd-networkd-wait-online.service` is enabled under `network-online.target`, invokes
`systemd-networkd-wait-online` with its documented 120-second default, and is an explicit
predecessor of `cloud-init.service`. NoCloud installs the signed guest agent only after this boundary.
The prior 60-second guest-verification deadline therefore could not observe a bootstrap on the
reviewed networkless `-nic none` boot path.

VM package 1.1.1 corrects only that sealed deadline: guest verification is now 180 seconds. Clone,
QEMU launch, and QMP shutdown remain 120 seconds; networking and SSH remain disabled; complete
failure preservation remains required; and retry, cleanup, and fallback signals remain prohibited.
The timeout remains part of provisioning-v3 and the deterministic plan digest; executor-v4 owns the
new 180-second contract. Preserved executor-v3 and executor-v2 contracts remain loadable only for
their separately authorized exact cleanup lists, and all old inputs fail closed for fresh execution.
Offline tests must pass before new deterministic builds.
The current promotion is immutable historical input; a fresh source-build review, new promotion ID,
fresh run/cohort and identities, new preparation, and a separately authorized live Gate 2 invocation
are required. A separately authorized recovery then negotiated only QMP capabilities and sent
`system_powerdown`; the exact recorded QEMU exited within the 120-second bound and no fallback signal
was sent. A final fresh cleanup authorization bound to executor SHA-256
`adcd4b1e4ea2bcd48078d0545a67699702b00dc344ca79eeeb9e49adefab0926` completed once. Immediate
host read-back confirmed all ordered 12 resources absent. The cleanup did not touch immutable
promotion `vmp-2026080815f0ea3f`, the post-correction source-review root, the checkout, or concurrent
task docs. Gate 2 is not yet qualified and Gate 3 remains blocked.

The post-correction offline source-build review used private root
`/tmp/dockpipe-vm-source-review.2ikAeuDJ` and exact repository checkpoint
`f6d5c19c24613945f5cbcf190aca50725ab51fdf`. Two independent Linux/amd64 lanes
used separate caches and temporary directories with Go 1.25.0, `GOWORK=off`, `CGO_ENABLED=0`,
`GOAMD64=v1`, `-trimpath`, and `-buildvcs=false`. Their controller outputs are byte-identical at
`5447246` bytes and SHA-256
`564d57937bef2856777dc3a3d05a57649e8918a0572f9f7f4d758308e9a7089c`; their guest-agent outputs
are byte-identical at `3870038` bytes and SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`. The Windows/amd64
compatibility output is
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e` and is not a Linux
promotion input. Go dependency closure inspection found only the standard library and packages under
`packages/vm/tools/**`. This gate created no live identity, input, plan, authorization, XDG root,
disk, seed, socket, process, cleanup, Gate 2, or Gate 3 action. The builds remain offline review
artifacts until a separately authorized fresh promotion.

The separately authorized immutable promotion `vmp-20260808f6d5c19c` then completed once. Its exact
mode-`0700` root contains only the mode-`0500` controller and guest-agent files with the hashes and
sizes above. Canonical evidence is
`/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-20260808f6d5c19c/promotion.evidence.json`
with SHA-256 `71827ec3cb32d35b92773b74fc0e0e2a68f0ba223341811c5e9da6b2de0f271d`.
Immediate read-back revalidated the independent build comparisons, embedded Go metadata, closed
inventories, owner-only modes, file and directory synchronization boundaries, and package/engine
separation. The earlier promotion remains untouched. This gate created no identity material, live
input, plan, authorization, disk, seed, socket, QEMU process, cleanup, Gate 2, or Gate 3 action.
Fresh Gate 2 preparation remains separately authorized.

The separately authorized offline preparation then created fresh task root
`/tmp/dockpipe-vm-gate2-prep-20260808-d8fb7b9e`, run `g2r-1f9bdb5dd11545a4`, and cohort
`g2c-17706e2c6519c7b0`. It generated a new 24-hour identity bundle, qualification input SHA-256
`c1583eff7db0049a6fb7692d36b153bbe285801232d26ed4b91c5e4df6965ab3`, provisioning input
SHA-256 `460b759b050a68a500aa6ea4e2c2e503ba2317d955e4e8b2d47d6eb6b93b39ec`, contract SHA-256
`656c5bca0ae6d0f994ecdad799b4a4d58354b955396e547d29981c0980521f1c`, and inert plan SHA-256
`bb1670208553885674e698b86bd0fee103ccda4e4cadee0497420b7913c09edc`. The plan retains
`live_authorized=false`, `execute=false`, `authorization_required=true`, and the reviewed
180-second guest-verification deadline. QMP, agent, and console socket paths are respectively 93,
95, and 92 bytes, below Linux's 107-byte bound. All four exact live roots remain absent. No live
authorization, identity reservation, disk, seed, socket, process, Gate 2, cleanup, or Gate 3 action
occurred. Live Gate 2 remains a separate exact authorization.

#### Linux VM Gate 2 NoCloud and virtio correction (2026-08-08)

The exact live authorization for run `g2r-1f9bdb5dd11545a4` and cohort
`g2c-17706e2c6519c7b0` executed once and is permanently spent. Guest
verification failed closed after the complete 180-second executor-v4 deadline;
no signed bootstrap, verification evidence, controller request, retry, signal,
fallback, or cleanup occurred. All task roots remain preserved. An escalated
host read proved the recorded QEMU process absent after failure. The owner-only
`first-boot-console.log` is `86335` bytes.

The captured console and an unprivileged read-only sparse copy of the preserved
OS overlay prove three independent causes. The pinned image spent about 120
seconds in `systemd-networkd-wait-online.service`, then started the agent only
at about 176.2 seconds. Cloud-init `write_files` failed with `Unknown user or
group: dockpipe-agent` because three agent-owned files were rendered before the
users module. Disk setup failed because `/dev/disk/by-id/virtio-g2data-63a5654952ec9a88`
did not exist: the requested serial is 23 characters while Linux exposes only
20 bytes for a virtio-blk serial. Cloud-init's pinned schema additionally
rejected the empty `packages`, `ssh_genkeytypes`, and `ssh_authorized_keys`
arrays, the user `create_home` field, and the misplaced user-data `network`
field. The forensic copy touched neither the preserved overlay nor any live
root.

VM package 1.1.2 corrects the package-owned contract. The three agent-owned
key/config files use cloud-init `defer: true`; the invalid fields are removed or
replaced while the separate NoCloud network-config still declares no Ethernet
interfaces. Both OS and data serial validation now fail closed above the
20-byte virtio-blk limit. Guest verification is 240 seconds: the observed
networkless 120-second wait plus the original 60-second post-agent verification
allowance. Executor-v5 owns the new policy. Preserved executor-v4, executor-v3,
and executor-v2 files remain cleanup-only with their respective 180/60/60-second
shapes, and compatibility tests pin each exact cleanup path. No `src/**` file
changed; the package/engine boundary remains preserved. The spent run still
requires separately authorized exact cleanup, and fresh builds, promotion,
preparation, and one new live authorization are required before Gate 2 can be
qualified. Gate 3 remains blocked.

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
- packages/dorkpipe/mcp/mcpbridge: normalized host session/approval operations;
- packages/dorkpipe/lib/cmd/dorkpipe/provider_pool.go: adapter selection retaining exec;
- packages/dorkpipe/resolvers/dorkpipe/assets/provider-pools/catalog.yml: capability policy;
- Pipeon extension: normalized session/event UI;
- DorkPipe/Pipeon tests and docs.

Do not modify those production areas for this research task.
