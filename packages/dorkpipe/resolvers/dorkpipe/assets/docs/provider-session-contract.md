# Provider-neutral top-level session contract

`providersession` is the CAS-02 contract for long-running, top-level provider sessions. It is a
pure type and validation package: it owns no process, transport, persistence, provider client, UI,
or approval delivery behavior.

## Public shape

- `SessionRef` identifies a provider and an opaque provider session.
- `State` is `ready`, `running`, `waiting_for_approval`, `waiting_for_user_input`, `completed`,
  `cancelled`, `failed`, or `disconnected`.
- `Event` is a bounded normalized state/progress/request/cancellation/recovery record. It carries
  safe references and summaries, never raw provider payloads or credentials.
- `ValidateNextSequence` requires contiguous event order and rejects duplicate, stale, and gapped
  records before a future adapter applies them.
- `Correlation` is the one-time decision tuple: process incarnation, connection, session,
  interaction, activity, request, and decision identity.
- `ApprovalRequest` carries only that complete correlation, one closed safe reason, and the exact
  allowed decisions. Command/workspace changes permit `approve` or `deny`; declared permissions are
  deny-only because this contract has no grant-subset surface.
- `RecoveryRequest` binds an opaque bounded recovery-evidence reference to the exact session;
  adapter-local persistence and reconciliation decide whether that evidence is safe.
- `ModelReasoningCatalog` carries at most 128 opaque, validated, currently available stable
  model/reasoning combinations. A selection is valid only against the exact catalog reference and
  exact advertised pair.
- `EffectivePolicySnapshot` carries exact selected/effective model, reasoning, approval/reviewer,
  and sandbox references plus a safe capability projection. It contains no adapter selection.
- `CapabilityRecord` distinguishes support, user enablement, authority expansion, and experimental
  status. Unsupported capabilities remain disabled; every enabled authority-expanding or
  experimental capability requires its own per-session confirmation.
- `UserInputPrompt` is a normalized renderable record with a 512-byte summary, at most 16 bounded
  options, or one text answer bounded to at most 4096 bytes. It never contains a provider question
  or option union.
- `Adapter` describes catalog, start, effective-policy, send, approval, bounded prompt lookup,
  one-time input response, cancel, and recover operations without choosing a provider implementation.

## Safety semantics

`disconnected` is fail-closed. It can return to `ready` only through verified recovery; a terminal
state cannot restart. A human decision requires the complete correlation tuple, preventing replay
or cross-session application. Model/reasoning and approval/sandbox selections cannot be silently
substituted. Approval and sandbox are validated independently, so approval automation never grants
broader sandbox authority. Any authority-expanding policy requires explicit confirmation for that
session; the confirmation is not reusable by a new session.

Prompt summaries and option labels are bounded normalized display values, not retained provider
input. Text answers exist only as transient operation input. Implementations must consume an input
response at most once for its exact process, connection, session, interaction, activity, request,
decision, and prompt references, then exclude it from events, snapshots, diagnostics, and audits.

Approval implementations must defensively pin the exact request. An explicit approve may resume only
that request and only after its matching provider resolution. An explicit deny is fail-closed and
must terminate rather than return the turn to running. Missing, malformed, duplicated, stale,
substituted, cross-session, and cross-request decisions cannot continue. Policy selection,
capabilities, model metadata, connection state, previous approvals, and consumer behavior are never
decision evidence.

## Future adapter mapping

A future App Server adapter maps its provider-specific thread to `SessionRef`, turn to
`InteractionID`, item to `ActivityID`, and approval request to `RequestID`. It owns all protocol
parsing and raw-payload handling; Pipeon receives only `providersession.Event` values.

CAS-03 adds a package-local, host-resident child supervisor. CAS-04 adds its private JSONL
initialization client: a bounded `initialize` request, `initialized` notification, monotonic
correlation, and schema/capability gate. CAS-05 adds bounded private thread/read/resume and
turn/start/steer lifecycle requests after that gate. It maps provider identifiers only into opaque
`SessionRef` and `Correlation` values, permits one active steerable turn, and rejects stale or
mismatched lifecycle references. Every request pins `gpt-5.6-terra` / `high`, workspace-write,
declared roots, network disabled, and human user review; protocol data, prompts, credentials, and
provider error bodies remain package-local and transient. Malformed envelopes, response mismatch,
provider errors, lifecycle/policy rejection, request deadline, transport loss, child exit, and
reroute indications are all one safe `disconnected` state event.

CAS-06 adds package-local structured notification normalization. It emits contiguous supervisor-owned
progress events with opaque thread/turn/item correlation and bounded allow-listed summaries only;
raw frames, token text, item content, messages, error bodies, commands, files, and credentials stay
private and transient. A correlated terminal turn event can release the private active-turn gate,
but does not implement recovery or replay. Approval relay, interruption, persistence, audit,
additional hardening, and Pipeon wiring remain deferred to CAS-07+.

CAS-07 adds package-local approval and user-input request relay. It creates opaque request and
one-time decision references only after exact process, connection, thread, turn, and item
correlation. It projects a closed reason and exact allowed decisions, never command text, patches,
paths, question text, provider request IDs, raw payloads, or permission/policy data. The neutral
contract bounds approval decisions to one-turn `approve` or `deny`: command/file requests can use
both; declared-permission requests are deny-only because a granted subset would need a new neutral
contract surface. Exact approval still waits for matching private resolution before returning to
running; explicit denial closes the supervised turn fail-closed. User-input requests use their
separate bounded response operation. Expiry and every stale, duplicate, malformed, unsupported,
transport, child-exit, provider-error, or reroute condition fail closed.

CAS-08 adds cancellation only through the existing neutral `CancellationIntent`: `user_requested`,
`safety_stop`, or `deadline_exceeded`. The package-local supervisor requires the exact active
process/connection/session/turn correlation, projects the opaque intent, and privately requests
the active turn interrupt. The accepted response is not cancellation completion; only the exact
correlated terminal `interrupted` notification projects `cancelled`. Timeout, transport loss,
child exit, response mismatch, malformed/provider-error/reroute input, a missing or
non-interrupted terminal, and any ambiguity disconnect and invoke the existing bounded shutdown
path. A background-process indication is reduced to the neutral
`background_process_risk_possible` summary only. CAS-09+ persistence, resumption,
reconciliation/recovery, audit, hardening, testing expansion, and Pipeon work remain deferred.

CAS-09 adds bounded package-local idle-session snapshots and an explicit validated
`RecoveryRequest`. The snapshot retains only safe session/policy/incarnation references, a
contiguous event cursor, and closed lifecycle/summary classes. Recovery launches a fresh
initialized supervisor, performs one exact private idle reconciliation read, and emits `ready` only
then. It never reconnects to a prior child or resumes/replays an active, pending, cancelled,
failed, unknown, or non-idle turn. Corrupt/stale evidence, policy/cursor mismatch, response or
transport ambiguity, child exit, provider error, reroute, and timeout remain bounded
`recovery_required`/`disconnected`.

CAS-10 adds an appserversupervisor-local, bounded, versioned audit journal. It stores only safe
operation/event outcome classes, opaque neutral session/correlation references, contiguous event and
journal cursors, and coarse progress/latency buckets. The journal is atomically replaced as bounded
append-only segments and is never an adapter operation surface: it cannot replay, resume, retry,
approve, deny, steer, cancel, or recover a turn. Raw frames/payloads, timestamps, prompts, questions,
commands, patches, paths, credentials, token text, provider IDs/error bodies, account/config data,
and process details are excluded. Missing, corrupt, oversized, stale, gapped, cross-session, or
unsafe audit evidence fails closed. A recovered idle session must match its retained audit cursor;
that evidence still never claims prior active or unknown work survived.

CAS-11 closes the remaining supervisor-local hardening gaps. Only the direct `codex app-server --stdio`
child shape is accepted; bounded constructor, policy, reference, snapshot, and audit values are required.
The policy stays pinned to `gpt-5.6-terra` / `high`, workspace-write, declared in-workspace roots,
network disabled, and human review. Unknown or duplicate initialization, event, server-request, or
MCP-progress extensions fail closed without retaining raw content. Disconnect and rejected recovery clear
private transport and active/pending state before bounded child cleanup. The journal stays descriptive
only and has no replay, retry, resume, recovery, decision, dispatch, or export operation.

CAS-12 is complete with deterministic fixture-only contract coverage for the existing CAS-03 through
CAS-11 behavior: strict initialization and launcher/policy gates, lifecycle and event ordering,
approval/input one-time correlation, exact cancellation completion, idle-only recovery, bounded
snapshot/audit stores, redaction, and fail-closed cleanup. The fixtures use only a local fake launcher
and private in-memory stdio; no provider executable, account, credential, network, auth, listener, or
integration route is involved. Expanded source-boundary checks keep App Server/raw-protocol vocabulary
out of `providersession` and Pipeon.

CAS-13 controlled Windows integration is complete. Its host-resident harness verified the exact
`gpt-5.6-terra` / `high` catalog combination, one completed no-tool turn, one exact interrupted
terminal, a correlated file-change denial with no requested change, clean shutdown, controlled
transport loss, and direct-child exit. The native turn policy remained workspace-write with declared
roots, network disabled, and human review. No raw frame, provider error, prompt, command, path,
credential, account value, or provider identifier was retained. CAS-14 production migration remains
deferred.

CAS-14 has its provider-neutral implementation foundation. The contract can expose the validated
stable model/reasoning catalog and safe effective-policy snapshot, keeping approval and sandbox
selection independent and requiring explicit per-session confirmation for authority expansion. It
also adds bounded prompt lookup and exact-correlation transient input response operations. Validation
rejects unavailable or silently substituted model/reasoning pairs, substituted approval/sandbox
policies, unsupported enabled capabilities, unconfirmed authority or experimental capabilities,
duplicate capability/options, stale input correlation, unknown choices, and oversized or malformed
display/input values. Adapter choice remains outside this contract.

The first supervisor-only CAS-14 foundation is fixture-backed and unused by a consumer. An initialized,
idle supervisor can project one bounded complete model catalog into an order-independent opaque
catalog reference and pin one exact advertised stable model/reasoning combination. It returns a
validated `EffectivePolicySnapshot` with no substitution, the existing human-review and
workspace-write references, and no enabled capability records. The CAS-13 `gpt-5.6-terra` / `high`
combination remains the proven fixture baseline but is not the only accepted catalog entry. Empty,
duplicate, incomplete, paged, unavailable, removed, mismatched, changed, malformed, or rerouted
catalog/selection evidence disconnects fail closed.

The second supervisor-only CAS-14 projection is also fixture-backed and unused by a consumer. Its
order-independent native policy catalog requires the human-review and workspace-write baseline and
accepts additional available stable approval/reviewer and sandbox choices only through exact opaque
references. Approval and sandbox are selected independently; each authority-expanding choice requires
its own per-session confirmation. Empty, duplicate, unavailable, removed, mismatched, changed,
unsupported, unconfirmed, cross-confirmed, shell-command-enabling, policy-bypassing, or silently
substituted evidence disconnects fail closed. Approval automation cannot select or confirm sandbox
authority, and broader sandbox projection cannot change approval authority.

The third supervisor-only CAS-14 projection is fixture-backed and unused by a consumer. Its bounded,
order-independent capability catalog requires stable available opaque references while keeping
availability distinct from explicit DockPipe support. The baseline projects every advertised record
disabled. An enabled subset must name exact advertised supported references, and every
authority-expanding or experimental capability requires its own per-session confirmation. Empty,
duplicate, unavailable, removed, changed, unsupported, unconfirmed, mismatched, or substituted
evidence disconnects fail closed. Catalog ordering, model, approval/reviewer, sandbox, and another
capability never imply support, selection, confirmation, or enablement.

The fourth supervisor-only CAS-14 projection established the bounded prompt-record and exact lookup
foundation and remains unused by a consumer. While one exact user-input request is pending, it can pin
one bounded normalized `UserInputPrompt` to the complete current correlation and opaque prompt
reference. Exact lookup returns only a defensive provider-neutral copy and leaves the supervisor
waiting for input. Empty, duplicate, stale, expired, mismatched, cross-session, substituted,
unsupported, malformed, or oversized lookup evidence disconnects fail closed. The normalized prompt
remains transient pending-request state; expiry and disconnect clear it, and prompt content never
enters events, snapshots, diagnostics, or audits.

The fifth supervisor-only CAS-14 slice is fixture-backed and unused by a consumer. It delivers one
validated `UserInputResponse` exactly once for the complete current correlation and opaque prompt
reference. One private provider question is supported per request; choice answers use an explicit
complete option-reference mapping, never option order, display labels, availability, or substitution.
The response and private question/option mapping remain transient and are cleared before delivery;
events and audits contain only bounded delivery/resolution classes. Duplicate, stale, expired,
malformed, oversized, mismatched, cross-session, unknown-option, post-disconnect, and replayed
responses fail closed. Multi-question provider batches remain unsupported rather than partially
answered.

The sixth supervisor-only CAS-14 slice is fixture-tested and unused by a consumer. The exact validated
single-question `item/tool/requestUserInput` request now creates the normalized prompt and private
answer mapping directly, without caller-supplied prompt or mapping evidence. Question whitespace and
option labels become bounded display values; text answers retain the contract's 4096-byte ceiling.
Each opaque option reference binds the complete current correlation to the exact private
question/option content and is therefore independent of provider option ordering. Raw question text,
provider option objects and descriptions, private question identity, raw answer labels, and the
reference mapping remain transient supervisor-local values and never enter events, snapshots,
diagnostics, or audits. Empty, malformed, over-bound, control-bearing, duplicate, display-ambiguous,
multi-question, or otherwise unsupported provider requests disconnect fail closed before any partial
prompt or answer is exposed.

The opt-in controlled Windows user-input harness is implemented but its 2026-08-03 proof stopped
safely. One bounded follow-up diagnostic found the exact request-production blocker in the installed
`codex-cli 0.144.1`: the generated protocol still contains the experimental
`item/tool/requestUserInput` method and accepts `initialize.capabilities.experimentalApi=true`, but
the `default_mode_request_user_input` feature is under development and disabled. The authenticated
no-write turn therefore completed with `user_input_tool_advertised=false`,
`user_input_tool_invoked=false`, `request_method_class=none`, and `schema_shape=none`. No answer was
sent, no retry occurred, no provider payload was retained, and the temporary workspace was removed.
Live prompt normalization, non-first option-reference delivery, matching resolution, and replay
rejection therefore remain unproven; production contracts and lifecycle dispatch remain unchanged.

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

The bounded selected-policy lifecycle seam now requires complete pinned model, native-policy, and
capability selections before `StartThread`, revalidates every exact catalog plus the effective snapshot
immediately before lifecycle use, and dispatches the selected model/reasoning values without fallback
or substitution. The `human-review` and `workspace-write` baseline advertisements retain the exact
already proven private App Server mapping (`untrusted` plus `user`, and `workspaceWrite` with the
caller's validated declared roots and network disabled). Those independent dimensions may therefore
be dispatched; their opaque refs, display meaning, ordering, availability, or confirmation are never
used to derive a wire value. Zero enabled capabilities is the safe lifecycle baseline.

The complete resolved policy receives a separate immutable thread binding while the existing recovery
policy key remains unchanged. `thread/read`, `thread/resume`, `turn/start`, and `turn/steer` revalidate
the pinned catalogs/snapshot and reject caller-policy, catalog, effective-value, or binding drift before
I/O. Removed or unavailable options, selection/effective mismatch, reroute/substitution evidence,
missing confirmation, and incomplete selection continue to fail closed with no replay, retry, or
fallback.

One non-baseline approval/reviewer mapping is now proven and dispatchable. The stable JSON Schema
generated offline by installed `codex-cli 0.144.1` defines `untrusted` in `AskForApproval`, defines
`auto_review` in `ApprovalsReviewer`, and binds those definitions directly to the `approvalPolicy` and
`approvalsReviewer` fields on thread and turn start parameters. Its description identifies
`auto_review` as the native risk-based reviewing subagent. The package-private `native-auto-review`
fixture therefore retains exactly `untrusted` plus `auto_review`; no opaque reference, display label,
ordering, availability, confirmation, sandbox choice, model, or capability is used to derive either
wire value.

The option remains stable, available, exactly selected/effective, and individually session-confirmed.
Its private mapping participates in catalog identity, ambiguity detection, immediate pre-I/O catalog
and snapshot revalidation, and the immutable thread binding. `thread/read`, `thread/resume`,
`turn/start`, and `turn/steer` reject missing, partial, ambiguous, changed, removed, unavailable,
unconfirmed, selection/effective-mismatched, or caller-drifted evidence before a request is sent. The
independent workspace-write mapping, declared roots, network-disabled policy, and zero enabled
capabilities remain the only sandbox/capability lifecycle state; the recovery policy key and existing
no-replay/no-retry/no-fallback behavior are unchanged.

One non-baseline sandbox mapping is now proven and retained without lifecycle authorization. The
stable JSON Schema generated offline by installed `codex-cli 0.144.1`, without `--experimental`,
defines the exact lifecycle sandbox value `danger-full-access` and the exact `SandboxPolicy.type`
value `dangerFullAccess`. The fixture-backed `broader-native-sandbox` advertisement retains that
private pair directly; no opaque reference, display label, ordering, availability, approval/reviewer
selection, confirmation, model, capability, connection, or authority status derives either value.

The catalog requires exactly one complete mapped non-baseline sandbox and includes its private pair
in identity, ambiguity/drift detection, defensive copies, and pinned comparison. The option must be
stable, available, exactly selected/effective, and individually session-confirmed. Missing, partial,
malformed, duplicate, second, changed, removed, unavailable, unconfirmed, cross-confirmed,
shell-command-enabling, or policy-bypassing evidence fails closed. Approval automation cannot select
or confirm sandbox authority. Production lifecycle validation, parameter construction, mapping
resolution, and dispatch remain unchanged and accept only `workspace-write` / `workspaceWrite`;
thread start/read/resume and turn start/steer therefore reject the newly mapped option before protocol
request I/O. Zero enabled capabilities and all recovery/no-replay behavior remain unchanged.

One stable capability mapping is now proven and retained without enablement or dispatch. The stable
schema generated without `--experimental` defines `InitializeCapabilities.requestAttestation` as a
boolean defaulting to `false`; `true` opts into `attestation/generate` requests for upstream
`x-oai-attestation`. The fixture-backed `request-attestation` advertisement retains exactly that
private field/value pair as stable, available, supported, authority-expanding, and non-experimental.
No opaque reference, order, availability, support flag, authority classification, another policy
dimension, or confirmation derives the mapping.

The private mapping participates in catalog identity, ambiguity/drift detection, defensive copies,
and pinned comparison. Missing, partial, changed, duplicate, second-mapped, removed, unstable,
unavailable, unsupported, non-authority, experimental, or unconfirmed evidence fails closed. The
effective-policy baseline keeps it disabled and unconfirmed. Initialization still sends only the
existing notification opt-outs, and unchanged lifecycle resolution rejects every enabled capability
before thread start/read/resume or turn start/steer request I/O. No attestation request, credential,
account value, authentication action, or provider call occurred.

The package-local pre-initialization decision adds no provider-neutral surface. A one-shot private
planner can validate the fixture-backed capability catalog before child startup and bind exactly one
`request-attestation` intent plus one independent confirmation to the exact package session,
supervisor incarnation, catalog identity, and private `requestAttestation=true` mapping. Its
fingerprinted scalar plan is not stored on the supervisor and is not consumed by initialize,
lifecycle, recovery, request handling, or a consumer. Catalog order is irrelevant while content or
mapping drift, reuse, cross-session or cross-supervisor substitution, multiple requests, a second
mapping, unsupported/experimental evidence, and post-validation mutation are rejected. Fresh and
recovered supervisors retain neither the plan nor confirmation authority; production initialization
and attestation-request rejection remain unchanged.

The subsequent offline contract decision also adds no provider-neutral surface or production code.
In `codex-cli 0.144.1`, `attestation/generate` requires a JSON-RPC id, that exact method, and params.
The generated JSON Schema leaves the params object open, but the exact official `rust-v0.144.1`
source at commit `44918ea10c0f99151c6710411b4322c2f5c96bea` defines
`AttestationGenerateParams {}` and generates `Record<string, never>`, so the params are exactly empty.
The named result remains only an opaque string token and defines no bounds, provenance, audience,
expiry, replay, challenge, signing, credential, account, or authentication contract.

App Server binds the request internally to the first attestation-capable live connection subscribed
to the current thread, sends that connection empty params without a thread id in the request
envelope, and waits 100 ms. Success becomes `{"v":1,"s":0,"t":"<token>"}` in the upstream
`x-oai-attestation` header. Request failure, cancellation, timeout, and malformed response instead
become status `2`, `3`, `1`, and `4` headers without a token; no capable connection or unusable header
value simply omits the header. The provider request continues in all of those cases.

That sequencing is incompatible with the neutral contract's fail-closed boundary, which requires
missing or invalid authority to stop before provider/network action. A private request/result type
cannot make App Server abort upstream I/O, so adding one would be an unsafe partial abstraction even
though the empty request is exact. Initialization therefore continues to omit
`requestAttestation`, request dispatch remains unsupported, and no token, credential/account access,
authentication, provider/network action, or upstream header mutation is permitted. This remains
provider-private initialization and transport sequencing; the neutral `providersession` contract
does not change.

The approved installed-binary inspection is corroborated by the exact tagged source. It proves only
that App Server delegates generation to its client and transports a result. The reviewed tagged App
Server, protocol, and core source paths expose no generator; their only response construction is the
integration-test fixture. The installed CLI exposes no DockPipe-usable producer, reports generation
as unsupported in exec mode, and reports it unavailable in TUI.

Neither official source nor installed artifacts define a token algorithm, trust root, challenge,
signing key, credential/account dependency, origin/audience binding, expiry, or replay rule. There
is no smaller safe package-local action. An upstream Codex contract and implementation must first
supply an authoritative client generator and abort provider I/O on absent, invalid, rejected,
canceled, or timed-out attestation. Only then could a separately bounded DockPipe slice reconsider a
private request/result validator or runtime authority. The neutral contract still requires no
change.

The first consumer-routing seam remains outside the neutral session contract. Normal Pipeon Codex
chat now reads a resource-scoped `codex_exec` / `codex_app_server` choice, passes it through one closed
MCP/CLI field, and requires a bounded Pipeon session id before persisting an explicit first-use
choice. The exact schema-versioned binding is stored in DorkPipe package state under a SHA-256-derived
filename with exclusive creation. Repeated exact choices are accepted; omission after pinning,
adapter drift, unknown values, malformed or extended state, cross-provider use, and conflicting
first writes fail before readiness, lease, execution, or provider activity.

The setting still defaults to `codex_exec`, and an unpinned caller that omits the field keeps the
existing exec behavior without creating adapter state. A separately bounded consumer slice now lets
an explicit pinned `codex_app_server` choice dispatch host-resident turns only through the
verified-idle continuation rules below. It does not change this neutral contract or the default.

Before every child startup, provider-pool creates one exclusive per-session turn lock. The first turn
has no prior provider session. A later turn is eligible only when canonical package state records the
previous correlated completion followed by a durable idle snapshot, the exact completed-turn counter,
opaque provider session and recovery references, and the unchanged model/reasoning pair. Unknown or
failed outcomes, interactive requests, crashes, malformed/substituted state, policy drift, concurrent
invocations, and incomplete persistence leave the lock in place; another request then fails before
supervisor, provider, or exec activity.

A later invocation uses a new process/connection incarnation and fresh direct App Server child. It
loads the exact snapshot and audit cursor, accepts no event after the idle snapshot except one clean
local shutdown disconnect, revalidates the exact model against the fresh live catalog, re-selects the
human-review/workspace-write/zero-capability policy, and proves the retained thread idle through one
correlated read. Only that sequence authorizes the next turn. It does not claim that active work
survived and never substitutes a new thread or exec route.

The one-turn path validates the exact requested model plus the fixed `high` reasoning baseline against
the live catalog, then selects the proven human-review/workspace-write mappings with zero enabled
capabilities. The exec-only `config` alias is rejected before a claim; an explicit model reference is
required. Attestation remains disabled. The prompt is private transient input. Only one bounded
completed agent message may be retained in memory after an exactly correlated completed terminal
event, and disconnect/shutdown clears it; neither value enters neutral events, audit, recovery, or
adapter state. Interactive approval/input control, durable rendering, fallback, rollback, default
selection, other consumers, and controlled provider evidence remain separate work.

The subsequent package-local approval-request slice tightens the existing relay without adding a
consumer operation. The normalized `ApprovalRequest` now has exactly three fields: complete
correlation identity, a closed reason (`command_execution`, `workspace_change`, or
`declared_permission`), and an exact allowed-decision list. The supervisor retains a defensive copy
while paused. An exact approve writes the private accept response and remains paused until the exact
`serverRequest/resolved` correlation returns; only then is `running` projected. An explicit deny
writes the private decline response and immediately disconnects as `approval_denied`, stopping the
child so the turn cannot continue. Without a decision, the request cannot resume. Malformed,
duplicated, stale, substituted, cross-session, or cross-request decisions fail closed, and no policy, capability,
metadata, connection, prior approval, or consumer state can imply approval.

Fixture-only supervisor and neutral-contract tests cover exact resume, terminal denial, missing
decision, replay/substitution, request isolation, and no fallback launch. Provider-pool/MCP decision
transport, Pipeon rendering and controls, free-text input, automatic approval, fallback, attestation,
and live provider execution remain deferred. The existing explicit App Server consumer route still
stops with an interactive-control failure and retains its unresolved turn lock.

The next consumer-only slice leaves this neutral contract unchanged and adds a package-private local
decision-source seam to the session-pinned App Server dispatcher. The production/default route has no
source. Only after a validated `EventApprovalRequested` exists may the dispatcher call the source,
passing a defensive copy of the exact `ApprovalRequest` and no policy, capability, model, connection,
adapter, or prior-decision metadata. The dispatcher validates any returned decision with
`ApprovalDecision.ValidateFor` against its own retained request copy before delivering it through the
same live supervisor; source mutation cannot widen the supervisor's pinned decision set.

Exact approval remains paused until the matching provider resolution and then continues the same
event loop. Completion and verified-idle persistence still require the existing correlated terminal
and durable idle gates. Denial terminates immediately as `approval_denied` without reading later
continuation, reporting success, persisting verified idle, releasing the unresolved turn lock,
starting a second child, replaying the prompt, substituting exec, or changing the pinned adapter.
Missing decisions preserve `interactive_control_required`; malformed, duplicate, replayed, stale,
substituted, cross-session, cross-turn, cross-request, and post-disconnect decisions fail closed.
User-input requests remain on their separate existing path and never invoke this source.

No MCP operation, Pipeon UI/control transport, free-text approval input, automatic decision,
persistence of approval content or decisions, fallback, rollback, attestation, default-adapter flip,
or live provider execution is added. Those production control surfaces remain deferred.

The subsequent MCP stdio concurrency foundation leaves this neutral contract unchanged. A single MCP
server may run at most one asynchronous `dorkpipe.provider_pool_chat` request while continuing to read
and service lightweight requests such as `ping`. A second concurrent chat is rejected before another
DorkPipe runner or child starts. All responses retain their exact JSON-RPC ids and pass through one
serialized Content-Length writer even when they complete out of order. Input EOF, read/framing or
write failure, and stdio shutdown cancel and join the active handler before return and suppress a late
response after transport closure.

Every non-chat tool remains synchronous and keeps its existing tier enforcement. At that foundation
boundary the fixture runner used by MCP tests was not a production approval source, provider-pool
session and unresolved-turn locks remained authoritative, and no approval operation was introduced.

The next bounded MCP slice leaves the neutral contract unchanged and transports only its validated
`ApprovalRequest` and `ApprovalDecision` projections. The exec-tier request operation reads a
defensive copy of the exact request currently pending for the one active chat on the same MCP server;
repeated reads do not consume it. The exec-tier decide operation requires the complete exact
correlation, accepts only the request's pinned allowed-decision set, validates through
`ApprovalDecision.ValidateFor`, and delivers once. Malformed, stale, replayed, duplicate, substituted,
cross-request, cross-turn, cross-session, cross-chat, cross-server, and post-shutdown values fail
before delivery.

Only an explicitly negotiated package-private mode installs the child-side source. The child emits a
versioned bounded neutral request frame on its anonymous stderr pipe, receives one versioned bounded
neutral decision frame on its anonymous stdin pipe, and leaves final provider-pool JSON on stdout.
Frames contain no provider RPC id, command, prompt, patch, path, credential, raw frame, provider
payload, or inferred authority. No file, package state, environment decision, polling, socket, named
pipe, listener, localhost endpoint, or persisted queue is used.

The server-owned active-chat record binds the transient controller, request, runner, context, and
same live child. Only one request may be pending. Exact approval remains blocked until the existing
matching provider resolution; denial retains terminal `approval_denied`. Child exit, control or MCP
transport failure, cancellation, EOF, or shutdown invalidates the request, cancels the source wait,
joins the chat, rejects later operations, and suppresses late output. Normal CLI and zero-value
provider-pool routes still install no source. No request or decision is persisted, and no fallback,
replay, second child, adapter substitution, user-input response, cancellation control, or Pipeon UI is
added. The next safe slice is Pipeon rendering and exact user controls over this transient transport.

The pre-existing authenticated MCP HTTP handler used by Pipeon creates that same active-chat record
before provider-pool dispatch. Concurrent HTTP reads and decisions therefore address the exact same
child; a second HTTP chat fails before another runner starts, ordinary exec-adapter chats keep their
buffered route, and server shutdown cancels and joins the active handler. No new listener or HTTP
authority is introduced.

The subsequent Pipeon-only control slice leaves this neutral contract and MCP operations unchanged.
For one normal Codex chat explicitly pinned to `codex_app_server`, the extension host runs one
invocation-owned bounded monitor over that authenticated HTTP transport. It retains the complete
validated `ApprovalRequest` only in host memory. The webview projection contains only a random
transient UI reference, the closed reason, the exact allowed-decision list, and a closed UI delivery
state; correlation and provider content never enter webview state, messages, `pendingAction`,
workspace/global persistence, logs, telemetry, or artifacts.

One exact click is checked against the same live Pipeon session, chat invocation, UI reference, and
retained allowed-decision set before the extension submits the unchanged correlation and selected
decision through `dorkpipe.provider_pool_approval_decide`. Confirmed delivery is not completion: the
original chat remains pending until its exact provider resolution and final response. Denial retains
the original `approval_denied` result, absence of a decision remains blocked, and ambiguous decision
delivery disables the controls without retry, fallback, replay, an opposite decision, another chat,
or another child. Chat completion, denial, failure, disconnect, cancellation, transport loss, and
extension restart retain nothing. Other providers, exec-adapter chats, prepared edits, normal CLI,
and zero-value production routes remain unchanged and acquire no decision source.

The subsequent provider-pool user-input controller seam also leaves this neutral contract unchanged.
One package-private response source may be supplied only to the existing session-pinned App Server
dispatcher; normal CLI and zero-value production options supply none. After one exact validated
`EventUserInputRequested`, the consumer requires the same live controller to remain
`waiting_for_user_input`, obtains the exact normalized `UserInputPrompt` through
`UserInputPrompt`, validates it for the request, retains an immutable defensive copy, and calls the
source at most once with a separate defensive copy. Only complete neutral prompt fields cross that
boundary; provider request IDs, raw/private provider content, prompt or command material, policy,
model, capability, approval, recovery, connection, and prior-response metadata do not.

A returned `UserInputResponse` must validate against the retained prompt before one delivery through
that controller's `RespondUserInput`. Source mutation cannot widen options, selections, text bounds,
kind, correlation, or prompt reference. Delivery leaves the turn pending until the exact correlated
`user_input_resolved` event; only then can existing terminal and verified-idle gates run. Missing
source or `found=false` remains `interactive_control_required`; source, prompt, response, delivery,
replay, stale-state, and missing-resolution failures are bounded and contain no prompt or answer
content. No prompt or answer field enters package state, snapshots, audit content, diagnostics, logs,
metadata, environment, files, or artifacts. Approval handling is unchanged and remains on its
separate source. No MCP/Pipeon transport, fallback, retry, replay, second child, adapter substitution,
or authority inference is introduced. The next safe slice is transient MCP user-input
request/response transport over the existing active-chat ownership model.

No second in-scope stable mapping is proven: `experimentalApi` is a global experimental opt-in,
`mcpServerOpenaiFormElicitation` is outside the no-MCP boundary, `optOutNotificationMethods` is a
suppression list rather than enablement, and `SelectedCapabilityRoot` is not bound to a stable thread
parameter. None changes the blocker above or authorizes initialization opt-in or a live request.
Provider discovery persistence, adapter selection, MCP, Pipeon, fallback, rollback, and all
consumer/bridge paths remain separate work.
