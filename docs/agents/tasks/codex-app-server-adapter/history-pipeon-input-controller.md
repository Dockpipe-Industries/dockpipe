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

