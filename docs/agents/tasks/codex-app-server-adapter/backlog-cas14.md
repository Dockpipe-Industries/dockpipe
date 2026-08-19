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
[TASK-013 closed history and evidence archive](history-cas14-foundation.md#closed-cas-14-implementation-slice-history).
