# Native remote desktop — future architecture

Status: first native slice in progress, authorized 2026-11-09. Tracking:
[TASK-046](../agents/tasks/native-remote-desktop/overview.md).
The first target is a Linux Qt viewer controlling one monitor on a paired Darwin
arm64 worker. Following the operator's transport refinement, the initial route is
an authenticated binary WebSocket relay through the existing Cloudflare Tunnel,
with endpoint-pinned TLS 1.3 inside that relay. This document is a design contract;
implementation and live qualification must be reported separately. VNC, RustDesk
integration and WARP enrollment are outside this selected slice.

ScreenCaptureKit/VideoToolbox capture and constrained-baseline H.264 encoding on
macOS, the Qt/FFmpeg viewer, input mapping and a Go standard-library TLS helper
belong to an external package. The existing package display SDK supplies the Qt
surface. Core owns independent desktop grants, bounded session/signaling metadata
and a provider-neutral opaque binary relay. Audio and multi-monitor support remain
outside this slice. The separately requested text-clipboard extension is described
below. Native permission, real Cloudflare transport, viewing/input and
cleanup evidence remain acceptance requirements. WebRTC/ICE/TURN is a later route;
it is not a prerequisite for this first connection.

## First implementation limits

The text-clipboard extension negotiates `clipboard_text: true` only for control
sessions. It transfers at most 64 KiB of valid UTF-8 text inside the encrypted
endpoint channel, rejects unsupported content without truncation, and shares only
while the viewer is active, visible and focused. It does not export existing
clipboard contents when watching starts. A paste shortcut offers current local
text; subsequent copied text may synchronize while focused. Writes require native
acknowledgment before queued paste input is sent, with a fixed timeout and bounded
queues. The host exposes a Share text clipboard setting. A reviewed private Linux X11/Darwin
pair subsequently verified exact generated multiline Unicode text in both
directions and native/Core control-session closure. This is a bounded functional
check, not sustained clipboard throughput or unattended worker-uptime proof.

The image/file follow-up is under qualification. It negotiates independent
`clipboard_images: true` and `clipboard_files: true` control-only options, both
off by default. PNG images and regular files use a separate streamed protocol;
the existing text protocol remains unchanged. File drops mean transfer onto the
remote clipboard and a subsequent user paste. They do not emulate an operating
system drag into the remote app. The receiver publishes its own private file URLs,
never paths supplied by the other machine. Directories, symlinks, promised files,
clipboard history and automatic opening are excluded. Pending bulk transfers do
not queue desktop input or automatically replay paste. These are implementation
requirements, not a claim of paired-worker acceptance.

An optional local Control/Command swap makes physical Ctrl+C/V send Mac
Command+C/V. It maps both left/right modifier pairs and leaves Alt/Option alone.
The viewer releases already-sent keys before changing the map, and the physical
Ctrl+Alt+Shift+Escape release chord retains its meaning. It does not change the
remote operating system's keyboard settings.

The admitted viewer is Linux X11; control requires a readable XKB physical key
map. Physical positions and left/right modifiers are preserved independently of
shifted characters. Unsupported lock, Insert and ISO/JIS-specific usages are
ignored; this does not provide IME/text composition or keyboard-layout matching.
Other viewer platforms and Wayland control remain unqualified.

Media uses constrained-baseline H.264 with selectable capture ceilings through
3840×2160 and frame-rate targets through 60 fps. The default remains High/30;
smaller source desktops are never upscaled. The Mac sender uses hardware
encoding. The viewer defaults to software decoding; Automatic permits a reported
fallback, while Hardware only requires an available CUDA or VAAPI backend.
Profile-specific SPS and allocation bounds apply before decoding. A bounded
four-picture compressed queue absorbs short bursts while the latest presentation
output and mouse-motion queues avoid accumulating stale work. The encrypted channel
has bounded framing, per-session identities and heartbeats; authorization remains
a short renewable lease tied to the original session. A disconnect never reopens
the session or replays pending input.

The Linux CUDA/NV12 presentation path retains a bounded decoded frame and uses
CUDA/OpenGL interoperability to copy its Y and UV planes into GPU textures.
An OpenGL shader converts and scales those textures. Unsupported interop uses
the existing CPU presentation path. Codec reference pictures remain available
when stale presentation outputs are skipped. Renderer reset releases the CUDA
registrations and retained device reference.

Compressed frame loss requires a fresh verified IDR before decoding resumes.
A healthy decoder retains its codec/device resources during this recovery;
flushing or rebuilding them can introduce large driver stalls. Actual codec
errors still discard the failed decoder. Latest-only presentation does not
establish sustained frame-rate performance.

The private Linux-to-Darwin pair passed High/60, Ultra/30 and Ultra/60 view-only
relay checks with CUDA decoding, synthetic color checks and authoritative closure.
Its captured desktop is 1920×1080. Separate original synthetic samples verified
full-size 4K hardware encoding and both software/CUDA decoding. These short
checks establish profile compatibility, not sustained 60 fps or 4K relay throughput.

The optional native virtual-display candidate keeps the existing desktop as the
default. Selecting `display_source:"virtual"` in the package-owned capture
profile requests the exact preset dimensions: 1280×720, 1600×900, 1920×1080 or
3840×2160. The choice is immutable for the authenticated connection; the default
profile omits this field for compatibility. This adds a display to the existing
logged-in desktop, not a separate user session. No BetterDisplay component is
required. Undocumented macOS display-creation APIs stay inside the native package
adapter and require runtime ABI checks plus qualification on the actual OS.
Capture and pointer mapping must use the same newly owned display ID. Closure
requires capture, encoder and held-input cleanup followed by verified absence of
that display from the online display list. UI/contract fixtures do not establish
native creation, removal or paired acceptance.

The dedicated native worker admits one session and exits after exact cleanup and
Forget. Native qualification must match that lifecycle: create and remove one
display per process, verify online absence before that process returns, and
independently check absence after it is reaped. Repeating display creation inside
one process is not qualified: a private test observed missing current-mode data
on its second display even though a fresh-process test of the same preset passed.
Mode and cleanup checks remain strict; this evidence does not justify treating
process termination as successful native teardown.

The private m6 preview now uses a child of the existing app host to own the
virtual display. A fresh read-only process verifies the exact mode after
creation because the long-lived worker's mode query can lag behind the actual
display. The worker independently checks the display identity and geometry.
On disconnect it stops capture, encoding and input before stopping the display
owner. Cleanup requires reaping that child and confirming the display is no
longer online. The existing app identity and permission grants are unchanged.

On m6, 27 native fixtures passed, followed by two real headless create/remove
cycles for each preset. These checks establish native display lifecycle and
mode selection, including 3840×2160 at 60 Hz. Display refresh rate does not
establish the delivered stream frame rate.

The paired Linux viewer decoded all four virtual presets at their exact
dimensions with CUDA through the encrypted relay at the 30 fps setting.
A separate 1080p control check verified a click against an isolated test
window. Each check verified authoritative closure and display removal.
These short runs do not qualify sustained 4K60 performance.

The keep-awake candidate is a host preference, independent of viewer quality and
tab retention. Its choices are Off, While connected, and While remote desktop is
enabled; an absent preference means Off. The native package owns the private
preference and the system/display idle-sleep assertions. Connected mode follows
an admitted running session. Enabled mode uses a separate background host process
so the assertion survives capture-worker turnover between sessions. It is bounded
by the configured desktop provider and registered desktop service; disabling
desktop access or stopping that companion must release its assertions.
No password, native consent, automatic-login setting or lock policy is changed.
Keeping the display awake does not unlock an existing lock or guarantee suppression
of screen savers, manual sleep or other system policies. Native assertion and
service-lifecycle qualification remain separate from source and UI checks.

On 2026-11-10 the private Darwin pair passed eighteen native component fixtures
and a real IOKit acquire/readback/release probe. Its separate background companion
then passed Off/Enabled/Off/Enabled transitions, verified assertion removal on
companion shutdown, and restoration from the saved Enabled preference after
restart. The app identity, native consent and existing desktop worker were
preserved. The old worker does not implement Connected mode, so the native
settings disable that choice until a matching upgraded worker publishes its
capability. Connected session behavior still requires live qualification.

The published display SDK requires synchronous cancellation/join on release.
Normal remote-close proof is observed briefly; late proof retains the exact
metadata-only reference for explicit inspection. A local UI close is not native
cleanup proof. Forced helper termination still has a bounded join budget.

Component/native build evidence is separate from real paired capture/input,
Cloudflare route, permissions and deployment evidence. The current implementation
is a review candidate, not a release or a broad support/performance claim.

On 2026-11-09, the paired Linux X11 / Darwin arm64 view test verified known
colors from an isolated native window through the existing Cloudflare route,
including endpoint TLS and native H.264 decoding. Its final disconnect initially
remained unknown. A subsequent exact view-only recovery verified original image
absence and received the broker's forget acknowledgment; an independent metadata
read confirmed closure. This proves viewing and that recovery basis, not native
capture-stop acknowledgment or input support. The corrected worker passed five
Mac fixtures; fresh live view/control qualification awaits its native permissions.

The reviewed macOS preview uses ad-hoc signing with a designated code-hash
requirement. Native checks confirmed that an enabled consent entry for the old
build did not authorize the corrected build with the same bundle identifier.
Preview setup must bind consent to the exact app and verify permission flags in
its LaunchServices context. Stable release signing and consent continuity across
host updates remain unqualified.

Setup consolidation is in progress: the existing Dockpipe app identity
(`com.dockpipe.launcher`) hosts the package's privacy-sensitive worker in-process
through a small, Qt-independent C interface. The package retains capture, input,
codec and native permission behavior. The app owns verified module admission and
process lifetime; the generic CLI remains a bundled child. Provider updates can
then replace an external module without changing the permission-owning app's
code identity. Native user consent still belongs to macOS, with distinct capture
and computer-control categories. Component compilation is not installed-app
identity or consent qualification.

The operator selected a private preview without an Apple signing identity. The
canonical app builds natively with its bundled CLI and validated dependency
closure; ad-hoc review stays explicitly hash-pinned. It is not a notarized or
Developer ID distribution, and a host rebuild can require fresh native consent.
A real app-host test admitted the native package and returned a clean failure
for an intentionally absent owner file, before native permission or session
APIs. The private app was subsequently installed and its native screen/control
permissions verified. Its first in-process view rendered the isolated window,
but encoder teardown crashed because escaping blocks borrowed a shared-state
reference. The package fix retains that state by value; regression tests reject
both old lifetime patterns and pass the corrected helpers. The crashed view was
subsequently reconciled through exact view-only process-exit proof and an
independent closed-status read. The replacement module retained the installed
host's permissions and passed a fresh view and authoritative close. A control
test lasting more than 30 seconds verified exact text, a button click and Shift
release. Earlier uncertain sessions were reconciled through qualified native
release verification, original process-exit proof and independent broker closure;
process exit alone cannot resolve a control session.

A subsequent package candidate routes authenticated channel EOF through normal
cleanup and exact broker acknowledgment, preserving failure for malformed or
truncated input. It adds bounded failure diagnostics that distinguish capture,
encoder, input release, child-process and broker cleanup. Duplicate local status
reports were found to delay the final close report; retaining only changed reports
preserves every lease and acknowledgment transition without that backlog. Nine
native fixtures passed, and the installed worker subsequently completed a live
control session, received authoritative closure and restarted ready automatically.
The installed app, bundled CLI and native permission identity remain unchanged.
Client completion handling passed a subsequent live test through the existing
Cloudflare route: the package's asynchronous pre-close operation and an
independent broker read both confirmed exact closure after a control session
lasting more than 35 seconds. Text, button input, held-Shift release, known pixels,
empty pending-reference journals and automatic worker readiness passed. The GUI
adapter retains the surface/module while closing; SDK 1.0 keeps its synchronous
cancel/join contract for final disposal and legacy callers. This qualifies the
private Linux X11 viewer / Darwin arm64 pair, not the broader platform roadmap
or a published SDK release.

The standalone preview worker's consent and the review CLI's unnecessary desktop
capture/control consent were reset with bundle-targeted OS commands. Active
worker services were preserved. Local Network entries have a separate OS-managed
lifecycle; cleanup must use supported controls for the actual macOS version and
identify the code behind a row before treating a duplicate name as obsolete.

Recovery distinguishes native stop acknowledgment from `provider-process-exited`
proof. The latter is restricted to an authenticated, revoked original view-only
session, with the worker lock held while a trusted native helper verifies that
the original provider, channel and bridge processes have exited. Core accepts
only the exact original reference/provider and requires the broker's exact
forget acknowledgment. Process exit cannot release a control session: injected
keys may remain held after the provider exits. Missing or mismatched proof keeps
the session unknown and its admission slot occupied.

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
authorization, endpoint identity, negotiation, termination and audit. Preserve its
remote workflow delivery architecture.

The first slice uses an explicitly selected WebSocket compatibility route through
the existing public hostname. A single-use, short-lived relay ticket binds each
socket to its endpoint role, exact session reference and current authorization.
The relay carries bounded binary records with backpressure and its own deadlines;
normal broker HTTP deadlines remain intact. Native endpoints negotiate mutually
pinned TLS 1.3 inside that opaque transport, binding the session reference, roles
and mode. This protects media/input from the tunnel forwarder under the existing
trusted-signaling model; it does not protect against a malicious trusted broker
substituting both endpoint bindings.

This route must be measured for TCP head-of-line stalls and disconnect behavior.
A lost connection requires a new authorized session and never replays input.
Cloudflare supports WebSockets and documents RDP-over-WebSocket routing, but public
hostname streaming terms and account entitlement still require qualification.
A bounded transport test does not establish free, unlimited or production-approved
video delivery.

Future alternate routes are direct ICE/STUN connectivity, optional Cloudflare
Realtime TURN and interchangeable self-hosted TURN. Verify service capabilities,
pricing, quotas, credentials and terms before selecting or rolling out a provider.
Expose the actual route and failures without silently weakening transport security.

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
| External packages/resolvers | Native desktop protocol, capture/input backends, endpoint TLS/media adapters, later WebRTC/TURN integration, compatibility protocols and dependency closure |
| Existing Qt desktop and host SDK | Dedicated session view, machine/session selection, Canvas/Tiling presentation and generic native widget lifecycle |
| Nucleon ecosystem | Versioned NLV-1 codec integration and codec-specific implementation; no copied proprietary implementation in Dockpipe core |
| Optional Cloud service | Managed signaling, TURN configuration, identity/session administration; customer compute and self-managed operation remain supported |

Resolve package identities and contract versions during design; names here do not
create new public APIs. Consume declared, pinned artifacts rather than sibling
source trees. Neither an optional media library nor NLV-1 becomes a mandatory core
dependency. Reuse the existing VM native widget seam where suitable, while keeping
local QEMU frame transport distinct from a remote desktop session.

## Desktop protocols and platform backends

The [paired connectivity and storage proposal](paired-storage-mounts.md) records
shared setup through existing paired workflow delivery. Reuse provider connectivity
and authentication/provisioning seams for RDP, while keeping desktop grants and
storage mounts independent. The user requests a dedicated RDP/remote-desktop tab,
separate from the VM tab; resolve top-level tab overload before final navigation
placement. This adds no storage prerequisite to remote desktop.

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
