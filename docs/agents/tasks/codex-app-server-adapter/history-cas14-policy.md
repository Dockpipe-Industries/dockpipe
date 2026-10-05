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

