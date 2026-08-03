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

**Prototype successful; founder migration direction accepted; implementation pending.** CAS-01
through CAS-13 proved the constrained, package-owned adapter boundary through controlled
integration. CAS-14 makes App Server the primary/default adapter for one Pipeon consumer while
retaining governed `codex_exec` fallback and bounded workers. Canonical research:
`docs/research/codex-app-server-top-level-orchestrators-2026-07.md`.

Repository has no ADR convention; this task is the accepted product-decision record, not an ADR.

### Current state

- The protocol spike, provider-neutral contracts, supervision, lifecycle, approvals, cancellation,
  recovery, persistence, audit, security fixtures, and controlled Codex integration are complete.
- The boundary remains package-local and unused by production consumers. No Pipeon migration,
  adapter selection, fallback policy, operations rollout, or default-provider change has started.
- The CAS-14 product direction is accepted. Its provider-neutral contract is complete, and the
  supervisor-only projections now cover stable model/reasoning, native approval/reviewer and
  sandbox policy, safe capability selection, provider-backed bounded user-input prompt
  normalization/lookup, and exact one-time response delivery. Every production consumer/bridge
  behavior remains unimplemented.

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
neutral contract carries no grant subset. User-input requests have no answer operation in the
existing neutral contract and expire fail-closed.

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

Every non-baseline sandbox choice and every enabled capability remain lifecycle-blocked because their
validated advertisements retain no exact package-owned provider mapping. The smallest evidence-backed
next CAS-14 action is one separately bounded package-local fixture/schema slice proving and retaining
one exact non-baseline sandbox mapping without inferring it from approval automation or widening the
current lifecycle sandbox. Capability wiring, consumers, MCP, Pipeon, fallback, and engine work remain
separate.

`packages/dorkpipe/lib/cmd/dorkpipe/provider_pool.go`,
`packages/dorkpipe/resolvers/dorkpipe/assets/provider-pools/catalog.yml`,
`packages/dorkpipe/mcp/mcpbridge/exec.go`, `dorkpipe.host_codex_chat`, bounded worker code,
schemas/YAML, and `src/lib` / `src/cmd` remain unchanged in this first implementation. The existing
`dorkpipe.provider_pool_chat` request remains the Pipeon entrypoint; the bridge honors the explicit
App Server adapter only for the eligible pinned Pipeon session and otherwise delegates to the
unchanged `codex_exec` provider-pool path.

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
