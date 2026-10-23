# Native remote desktop — future architecture

Status: planned, 2026-10-23. Tracking: [TASK-046](../agents/tasks/native-remote-desktop/overview.md).
This is a design direction and investigation backlog, not an implemented protocol,
dependency selection, rollout approval or authorization to interrupt stabilization.

## Goal and existing foundations

Provide personal remote access that can eventually replace reliance on RustDesk,
while providing repeatable desktop workloads for NLV-1 through Nucleon. Reuse
Dockpipe's machine identity, execution ownership, audit and workspace abstractions.

[Remote nodes](../runtime/remote-nodes.md) remains canonical for current execution,
pairing, delivery permissions and provider behavior. Its documented Windows worker
gap must be qualified explicitly; cross-platform desktop support is a target, not
proof of an already cross-platform remote agent. Inventory any existing passkey
enrollment before proposing changes; do not assume it is implemented or equate
hardware enrollment approval with desktop session permission.

## Control and media planes

Preserve the working public HTTPS Cloudflare Tunnel broker. Extend its existing
provider-neutral session contracts for discovery, pairing, authentication,
authorization, endpoint identity, WebRTC signaling, negotiation, termination and
audit. Do not replace its remote workflow delivery architecture or carry ordinary
desktop video through the public tunnel as the default media route.

The proposed native media path is WebRTC with this connection preference:

1. Direct endpoint connectivity negotiated through ICE/STUN.
2. Cloudflare Realtime TURN as an optional managed relay fallback.
3. An interchangeable self-hosted TURN provider for on-premises or enterprise use.

These are proposed integrations. Verify current service capabilities, pricing,
quotas, credentials, commercial terms and permitted traffic before selecting or
rolling out a provider. No free, unlimited or universally suitable service is assumed.
Expose actual direct/relay state and failures; do not silently downgrade transport
security or imply that TURN terminates session authorization.

Separate control, input, clipboard and media handling. Define encrypted endpoint
channels, authenticated identity binding, reconnection and renegotiation, frame
and input sequencing, queue limits, deadlines, backpressure and cancellation.
Old frames should expire instead of accumulating latency. Reconnection must
revalidate grants and revocation; transport connectivity alone grants no access.

Evaluate native C/C++ WebRTC libraries against Qt integration, supported platforms,
threading, cancellation, licensing, maintenance and binary size. Selection is open.
Do not assume arbitrary encoded video can be inserted into a standard WebRTC video
track. Compare supported encoded-frame integration with bounded binary data-channel
delivery, including congestion control, message limits, fragmentation, reassembly,
reliability choices and head-of-line latency. Record interoperable fallback behavior.

## Ownership and extensibility

| Owner | Responsibility |
| --- | --- |
| Generic core/CLI/API | Shared identity, scoped grants, session lifecycle, process ownership, audit and bounded public diagnostics; only general primitives needed by consumers |
| External packages/resolvers | Native desktop protocol, capture/input backends, WebRTC implementation, TURN provider integration, compatibility protocols and dependency closure |
| Existing Qt desktop and host SDK | Dedicated session view, machine/session selection, Canvas/Tiling presentation and generic native widget lifecycle |
| Nucleon ecosystem | Versioned NLV-1 codec integration and codec-specific implementation; no copied proprietary implementation in Dockpipe core |
| Optional Cloud service | Managed signaling, TURN configuration, identity/session administration; customer compute and self-managed operation remain supported |

Resolve package identities and contract versions during design; names here do not
create new public APIs. Consume declared, pinned artifacts rather than sibling
source trees. Neither an optional media library nor NLV-1 becomes a mandatory core
dependency. Reuse the existing VM native widget seam where suitable, while keeping
local QEMU frame transport distinct from a remote desktop session.

## Desktop protocols and platform backends

Offer capability-driven selection between Dockpipe Native and resolver-provided
Windows RDP, macOS VNC/Screen Sharing, and Linux RDP/VNC compatibility. Inventory
existing adapters before adding another implementation. Disclose whether a mode
controls an existing desktop or creates a separate login session; never present
those as equivalent behavior. Compatibility modes retain their own security and
licensing requirements and need not share WebRTC media internals.

Plan native capture/input backends for Windows, macOS, Linux Wayland and Linux X11.
Investigate permissions and consent, capture APIs, input injection, elevated/secure
desktops, hardware encoding, monitor selection, cursor composition, DPI/scaling,
dynamic resolution and lock/unlock behavior. Unsupported permissions or session
types must be explicit. Start with reliable viewing and mouse/keyboard control;
multi-monitor enhancements and audio forwarding follow separately.

## Codec negotiation and measurement

Negotiate endpoint codec/version capabilities, profiles, pixel formats, acceleration,
resolution and resource limits. Candidates are H.264, AV1 and NLV-1; investigate
hardware and software implementations and their distribution/patent terms before
selection. Choose the best qualified codec for the workload and compatible hardware;
NLV-1 is not automatically the default. Adaptive selection remains a later extension
with bounded switching and visible fallback reasons.

Maintain separate codec-only and transport/end-to-end benchmarks. Use repeatable
inputs and comparable visual quality targets, recording exact versions, settings,
hardware, acceleration, protocol, direct/relay route and network conditions. Include:

- Encode/decode time and input-to-display latency, with clock methodology and
  distributions rather than an unexplained single average.
- Bitrate, total network traffic, frame rate, dropped/late frames and queue depth.
- CPU, GPU and memory use, including capture, transport and presentation costs.
- Text readability and visual quality for static desktops, scrolling and rapidly
  changing content under controlled bandwidth, jitter and loss.

Integrate bounded, sanitized results with existing execution/observability artifacts.
No NLV-1 superiority claim follows from a single workload, unmatched quality or a
transport change. Keep private benchmark content and proprietary details out of the
public repository; use licensed or synthetic fixtures with recorded provenance.

## Authorization, UI and lifecycle

Pairing and workflow-delivery consent are not desktop-control grants. Design explicit
view, input and clipboard scopes, local consent/unattended-access policy, expiry,
revocation and audit. Bind media negotiation to the authenticated endpoint/session;
test replay, stale signaling, identity substitution, reconnect and revoked machines.
Clipboard transfer needs its own permission and bounded content handling without
recording contents in logs, arguments or environment variables. Release pressed
keys/buttons on disconnect. Disconnecting a view must not implicitly terminate
unrelated workflows or log out the remote user.

Assess FIDO2/YubiKey enrollment approval, short-lived pairing windows, single-use
challenges and cryptographic machine binding against existing onboarding first.
Reuse already implemented protections; separate optional future hardening from
required desktop-session authorization.

Use the existing Qt workspace for dedicated views, independent Canvas and Tiling
selection, connection/recovery state, consent, codec/transport details and bounded
diagnostics. Keep session operations available through shared CLI/API contracts;
the GUI does not own a second connection manager or authorization policy.

## Deployment and open decisions

Self-managed desktop use keeps BYOC and the existing Cloudflare integration with
optional TURN configuration. Optional Dockpipe Cloud can manage control-plane and
relay administration without supplying customer compute by default. On-premises
deployments must support self-hosted signaling/TURN without mandatory Cloud access.

Before implementation, decide the native WebRTC dependency, encoded-video channel,
capture/input APIs, agent/session permissions, supported codec combinations, TURN
credential lifecycle, connectivity recovery policy and benchmark acceptance budgets.
Document OS/architecture and session-type support through the existing platform
program. Validate provider terms and total relay costs before any commercial rollout.

The backlog defines staged delivery and acceptance. No dependencies are installed,
services provisioned, runtime behavior changed or remote access enabled by this plan.
