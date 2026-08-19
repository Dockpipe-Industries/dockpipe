# TASK-013 Closed Implementation and Evidence History

This evidence-only archive contains immutable closed history split verbatim from the active
[TASK-013 Codex App Server adapter record](overview.md). Current state, the
pause/resume checkpoint, open gates, accepted decisions, and authority boundaries remain in the
active record. This archive grants no implementation, evidence-run, retry, cleanup, migration, or
successor authority.

## Archived History

The blocks remain in their original order. Marker comments delimit the exact byte-preserved ranges used by the rank-8 validation.

### Closed CAS-14 Implementation Slice History

<!-- BEGIN TASK-013 VERBATIM CLOSED HISTORY BLOCK A -->
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
