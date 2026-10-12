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

