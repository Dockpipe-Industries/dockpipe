# Exact desktop cleanup bridge

`dockpipe remote desktop-cleanup --stdio --request <private-file> --state <worker-state> --auth never`
is a worker-local cleanup bridge for a trusted native provider. It does not open
a session, renew media/input authority, replay signaling, or infer cleanup from
a PID, crash record, missing window or operator acknowledgment.

The private request is bounded to 4096 bytes. It contains the exact original
`reference`, `provider` and authenticated status `ack`, plus optional `proof_basis`.
Unknown/duplicate fields and unsupported proof bases are rejected. The paired
worker node/fingerprint and current frozen owner provider must match. The original
reference's generation and broker epoch are retained for cleanup across a worker
lifetime change; they cannot select another machine or provider.

The bridge holds the worker's exclusive `desktop.lock` through native proof and
the broker's exact Forget acknowledgment. Its authenticated exchange publishes
the original target/provider with view/input capabilities false. Cleanup requires
exactly the original revoked directive, Close=true, lease_millis=0 and no signals.
It never grants a new lease or changes the owner policy. Input may already be
revoked: revocation must still permit strict cleanup of the original operation.
The frozen policy is checked again before publishing closure.

## Proof modes

| Request proof_basis | Required original mode | Native evidence |
| --- | --- | --- |
| Omitted or `provider-process-exited` | `view` | Verified original provider/bridge/channel process exit |
| `native-input-reset-and-provider-exited` | `control` | Fresh challenged native input reset/readback and original provider/bridge/channel exit |

The default view-only protocol is unchanged. Its seven-field `cleanup` frame has
`type`, `reference`, `provider`, `mode:"view"`, `close:true`, `lease_millis:0` and
`proof_basis:"provider-process-exited"`. Its strict three-field proof is
`type:"process-exited"`, `reference` and `provider`. Plain process exit can never
release a control session: injected keys can remain held after the process exits.

The explicit control mode checks the frozen policy before issuing a fresh 256-bit
challenge, encoded as 64 lowercase hexadecimal characters. Its eight-field frame is:

| Field | Required value |
| --- | --- |
| type | `cleanup` |
| reference, provider | Exact original objects |
| mode | `control` |
| close | `true` |
| lease_millis | `0` |
| proof_basis | `native-input-reset-and-provider-exited` |
| challenge | Fresh challenge for this bridge invocation |

The trusted native component may act only after validating that complete frame.
Core contains no platform input/capture implementation. The provider owns actual
reset effects, native readback, frozen image identities and process-exit proof.
Its cleanup operation must be explicitly authorized; this protocol is not an
automatic reset or a claim of native OS attestation by Core.

After completing that evidence, the component returns exactly six fields:
`type:"input-reset-and-provider-exited"`, exact `reference`, exact `provider`, the
matching `challenge`, `reset_verified:true` and `provider_exited:true`. Missing,
false, extra or duplicate fields, stale/empty/wrong challenges and identity changes
are rejected. A reset from a prior invocation cannot satisfy a new challenge.
Input-reset evidence alone cannot replace provider-exit evidence, or vice versa.

After strict proof validation and another frozen-policy check, Core submits one
closed report with the captured original acknowledgment. The broker validates
that acknowledgment. Success requires exactly one matching Forget and no new
directive. The final three-field frame is `type:"forget"`, exact `reference`, and
the selected `proof_basis`. Neither proof mode asserts a native capture-stop ACK.

## Uncertainty and bounds

Proof input is bounded to 4096 bytes, its wait to 20 seconds, and the whole CLI command
to the existing 25-second limit. EOF, cancellation, changed policy, foreign metadata,
an invalid proof or a missing/foreign Forget does not produce a success frame.
There is no proof retry or reset replay inside Core. The challenge is ephemeral;
it is not a grant, media pin, certificate or durable recovery credential.

A lost completion acknowledgment may leave the caller uncertain after the broker
accepted closure. Inspect that exact retained reference before considering another
attempt. Never infer that an error means no reset or closure effect occurred.
Provider-specific verification may have stricter native deadlines and readbacks;
generic Core does not weaken them or replace them with these booleans.

## Normal worker closure

An authenticated Close or an expired received lease switches the live worker
bridge into its bounded cleanup-only drain. The bridge publishes capabilities
false and drops queued ordinary opening/running reports without a separate
network exchange for each report. It forwards only the exact original native
closed/unknown report without signaling. Success still requires the exact closed
report's matching Forget; unknown, missing or foreign acknowledgments remain errors.

Native providers should send reports when state, acknowledgment or signaling
changes. Repeating an unchanged running report for every lease directive can
build a pipe backlog behind which the terminal cleanup proof arrives too late.
Suppressing those identical reports preserves lease processing and terminal
proof delivery; it never permits skipping a changed or closed report.
