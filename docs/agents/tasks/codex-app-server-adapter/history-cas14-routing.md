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
`packages/dorkpipe-mcp/mcpbridge/exec.go`, `dorkpipe.host_codex_chat`, bounded worker code,
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

