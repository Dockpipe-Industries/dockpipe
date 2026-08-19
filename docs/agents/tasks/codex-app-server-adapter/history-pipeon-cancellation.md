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

<!-- END TASK-013 VERBATIM CLOSED HISTORY BLOCK A -->

