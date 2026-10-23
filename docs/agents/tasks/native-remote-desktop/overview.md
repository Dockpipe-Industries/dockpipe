# TASK-046 — Native remote desktop and codec benchmarking

Status: **planned / future work**, added 2026-10-23. Ownership: unclaimed.
Authority covers architecture/backlog documentation only. Do not implement, install
dependencies, provision services, change remote access or interrupt the current
refactor and stabilization. No commit, publication or external issue creation is implied.

The [canonical proposal](../../../architecture/native-remote-desktop.md) records
the design. Goal: eventually replace reliance on RustDesk for personal access and
provide representative, comparable NLV-1/Nucleon benchmarks. Native desktop support
is a future capability; existing remote execution support is not desktop qualification.

## Reuse and dependencies

| Existing work | Reuse / prerequisite; do not duplicate |
| --- | --- |
| [TASK-044](../ecosystem-extraction/index.yaml) | Finish package extraction and stabilization before implementation; keep provider/codec implementation in its owning external repository |
| [TASK-036](../remote-node-onboarding/index.yaml), [remote nodes](../../../runtime/remote-nodes.md) | Existing broker, pairing, revocation, identity and delivery lifecycle; audit enrollment/passkey assumptions and add scoped desktop grants without redesigning the broker |
| [TASK-043](../configurable-networking/index.yaml) | Shared ingress/egress/proxy policy and diagnostics; own only desktop signaling/media extensions here |
| [TASK-045](../vm-guest-tools/index.yaml) | Reuse generic native display/Canvas/Tiling lifecycle; remote desktop is separate from local QEMU transport and Guest Tools |
| [TASK-025](../supported-platform-program/index.yaml), [TASK-032](../windows-platform-support/index.yaml), [TASK-033](../macos-platform-support/index.yaml) | Platform qualification and truthful support matrix; Windows remote workers are still a documented gap, not an assumed foundation |
| [Execution observability](../../../runtime/execution-observability.md) | Reuse run/session relationships and bounded diagnostics; keep codec and transport measurements distinguishable |

No existing native remote desktop epic was found in the inspected backlog. These
dependencies remain owned by their existing tasks; none is reopened or reclassified.

## Grouped implementation stories

All stories are **planned**. Story IDs below are local to this epic, not new global tasks.

| ID | Scope | Depends on | Acceptance before completion |
| --- | --- | --- | --- |
| RD-01 | Architecture and native transport/codec/session interfaces | Stabilization; reuse inventory | Record ownership, native C/C++ WebRTC library assessment, track versus binary-channel feasibility, bounded queues/cancellation, protocol versions, support matrix and open decisions; no mandatory media dependency in generic core |
| RD-02 | Desktop authorization and broker signaling | RD-01; TASK-036 identity audit | Extend existing broker for authenticated session negotiation/termination and audit; separate view/input/clipboard grants from execution delivery; test expiry, revocation, replay, endpoint substitution and reconnect. Assess enrollment hardening without rebuilding existing protections |
| RD-03 | ICE/STUN, managed and self-hosted TURN | RD-01, RD-02; TASK-043 | Prove direct connectivity and explicit relay fallback under controlled NAT/failure scenarios; interchangeable Cloudflare and self-hosted providers, short-lived credential handling, bounded recovery and truthful route diagnostics; verify pricing/terms/quotas before rollout |
| RD-04 | Windows, macOS, Wayland and X11 capture/input | RD-01, RD-02; platform qualification | Qualify real desktop viewing and mouse/keyboard input per backend with consent/permissions, cursor/DPI/resolution handling, lock/secure-session limits and key release on disconnect; stage monitor/audio enhancements separately |
| RD-05 | Codec abstraction, H.264/AV1 and NLV-1 benchmarking | RD-01, RD-03, RD-04 | Capability negotiation and explicit compatibility fallback; qualified H.264/AV1 paths where practical; versioned Nucleon integration; repeatable codec-only and end-to-end comparisons at comparable visual quality, without assuming NLV-1 wins |
| RD-06 | RDP/VNC compatibility providers | RD-01, RD-02; existing adapter inventory | Resolver capabilities for Windows RDP, macOS Screen Sharing/VNC and Linux RDP/VNC; disclose existing-desktop versus separate-login semantics; test each provider's authentication, permissions and lifecycle without forcing its media through WebRTC |
| RD-07 | Dedicated Qt view, Canvas, Tiling and CLI/API | RD-02, RD-03, RD-04; TASK-045 generic seams | Select machine/session independently in each workspace surface; show status, route/codec and recovery; shared lifecycle/authorization through CLI/API; disconnect releases the view/session as intended without stopping unrelated workloads |
| RD-08 | Observability, integration/performance qualification and deployment | RD-02–RD-07, incrementally | Publish sanitized diagnostics and repeatable native acceptance evidence across the supported matrix; test network degradation, resource bounds and cleanup; document self-managed BYOC, optional Cloud administration and Cloud-independent on-prem signaling/TURN |

RD-08 instrumentation starts with the first vertical slice; its final acceptance
depends on the selected release scope. RD-05 starts with a baseline codec and adds
AV1/NLV-1 comparisons later. RDP/VNC compatibility can progress independently once
shared authorization and provider contracts exist; it does not block a native MVP.

## Milestones and recommended MVP

1. **M0 — Design admission after stabilization:** audit existing capabilities and
   enrollment, choose a maintained native WebRTC dependency and viable encoded-frame
   path, define resource/latency budgets and desktop authorization. Verify provider
   terms, codec distribution obligations and the first supported endpoint pair.
2. **M1 — Native MVP:** one explicitly qualified sender/viewer pair, one monitor,
   reliable viewing and mouse/keyboard control, one negotiated baseline codec,
   direct ICE/STUN plus TURN fallback, consent/revocation/disconnect, a dedicated Qt
   view and CLI/API diagnostics. Prefer an existing Windows desktop viewed from the
   Linux Qt host for the first target if the Windows agent/permission prerequisites
   can be qualified; confirm this at M0 rather than assume worker support. Cloudflare
   TURN is the first proposed managed adapter, subject to service review. Exclude
   clipboard, audio, advanced multi-monitor and adaptive codec switching from MVP.
3. **M2 — Workspace and platform parity:** independent Canvas/Tiling selection,
   scoped clipboard, remaining native capture/input backends, self-hosted TURN and
   RDP/VNC compatibility, with native permission/session-type acceptance evidence.
4. **M3 — Codec and deployment qualification:** comparable H.264/AV1/NLV-1 results,
   controlled network impairment and resource tests, BYOC/Cloud/on-prem operations,
   and a support matrix. Adaptive codec switching and audio remain later candidates.

Minimum measurement dimensions: encode/decode and input-to-display latency, network
traffic/bitrate, CPU/GPU/memory, frame rate/dropped frames and text readability for
static and changing desktops under bandwidth, jitter and loss variation. Keep
codec-only evidence separate from transport and full-session evidence.

## Decisions still open

- Native WebRTC library and custom-frame track versus binary-channel integration;
  congestion control, bounded frame reassembly and reliability behavior.
- Capture/input APIs, OS permissions, elevated/locked sessions, service versus
  logged-in agent operation, and exact OS/architecture support.
- Existing enrollment/passkey capability inventory, additional desktop scopes,
  unattended-access policy, future FIDO2 approval and session identity binding.
- TURN credentials, provider capabilities/costs/terms and on-prem distribution.
- Codec licensing/acceleration, versioned Nucleon API, comparable quality targets,
  clock methodology and quantitative performance/resource acceptance budgets.

This record selects no library, purchases no service, claims no codec advantage
and changes no running process or current VM task. Implementation needs fresh,
explicit authorization after the current refactor/stabilization milestone.
