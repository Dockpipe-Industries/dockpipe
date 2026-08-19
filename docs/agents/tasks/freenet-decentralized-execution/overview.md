# TASK-019 Freenet Decentralized Execution Coordination

## Goal

Investigate a substantial future integration in which Freenet distributes and synchronizes signed
execution intent while opted-in DockPipe runners perform governed work on participating machines.

> **Freenet distributes and coordinates intent; DockPipe safely turns that intent into governed
> execution.**

This is an exploratory architecture and research backlog contract. It does not authorize a live
network integration, daemon, Freenet contract, delegate, runner, public service, economics system,
or DockPipe engine change.

## Status And Product Posture

- Treat Freenet as **alpha and pre-production for this initiative as of 2026-07-28**, consistent with
  the [official River and Freenet status page](https://freenet.org/river/). Revalidate the current
  Freenet Core, SDK, network, security, and release status before each implementation phase.
- The preferred first proof is a trusted private swarm owned by one person, organization, or
  explicitly trusted group.
- Approved community runners are a later phase. Untrusted public compute is research only and is
  neither safe nor solved by this task.
- The proof should be clean enough to demonstrate to the Freenet founder and project team, but no
  upstream behavior or collaboration is assumed.

Official Freenet material currently documents deployed mechanisms alongside partial, experimental,
and open ones. Its whitepaper identifies open work in scaling characterization, contract-merge
diagnostics, congestion control, Sybil-resistant identity, and network-level abuse/incentives. It
also describes cross-device delegate synchronization as only partially implemented. These are
direct feasibility constraints, not incidental product polish.

## Problem Statement

DockPipe can govern host steps and container or VM runtimes, with package/workflow/resolver surfaces
for QEMU and Windows guests, GPU-enabled inference, browser automation, builds, tests, deployments,
and agent work. Optional WSL forwarding is a host bridge rather than another runtime. None of these
local execution surfaces provides decentralized discovery, replicated coordination, or peer-to-peer
application state by itself.

Freenet provides a promising coordination substrate, but its contracts are not general-purpose
compute workers. Freenet contracts are public WebAssembly-defined replicated data structures that
validate, merge, summarize, and synchronize application state. Delegates are local sandboxed
WebAssembly actors for private state, attributed messages, and consent. Native workloads must run in
a separately installed, explicitly opted-in DockPipe runner under DockPipe policy.

The architectural problem is therefore to bridge replicated, eventually consistent Freenet intent
to a local governed execution boundary without turning a contract update, peer connection,
capability claim, delegate signature, lease claim, event, or remote result into authority it does not
possess.

## Strategic Value

- Let Freenet applications request real-world work that replicated contracts cannot directly
  perform: builds, tests, containers, Windows/QEMU jobs, GPU inference, browser automation, deploy
  preparation, and governed agent workflows.
- Give trusted users a decentralized alternative to a continuously hosted DockPipe control plane for
  discovery and coordination.
- Reuse DockPipe's package, workflow, runtime, policy, approval, event, artifact, cancellation, and
  cleanup boundaries instead of creating an unsafe native command bridge.
- Provide Freenet applications with signed, inspectable execution state and output provenance while
  keeping large payloads, private source, and secrets outside public replicated contract state.
- Test whether DockPipe can become a governed computation layer for peer-to-peer applications
  without coupling its generic local engine to one network.

## Dependency And Feasibility Research

### Confirmed Freenet Capabilities And Current Limits

The following statements are grounded in current official Freenet documentation and source. Any
stronger statement remains a hypothesis until a proof verifies it against a pinned Freenet version.

| Area | Confirmed current shape | Consequence for DockPipe |
| --- | --- | --- |
| Contracts | Content-addressed WebAssembly defines public state validity, merge, summary, and delta behavior. State must converge through an idempotent commutative merge model. | Model coordination as mergeable signed records, not a mutable ordered broker database or native compute process. |
| Subscriptions | Clients can subscribe to contract updates; Freenet uses leased subscription trees and summary/delta synchronization. | Promising for job discovery and live state, but not proof of exclusive assignment, global ordering, or delivery exactly once. |
| Delegates | Local WebAssembly components hold secret state, receive sender-attributed messages, can sign, interact with contracts, and request user consent through a trusted shell prompt. | Promising key and consent boundary. Do not assume filesystem, socket, subprocess, or DockPipe-daemon access. |
| Client API | The official client SDK uses a local Freenet node WebSocket for contract and delegate operations. Lower-level APIs are still stabilizing, and the current TypeScript SDK documents FIFO `get` response matching without request correlation. | A companion must pin and wrap the supported client API, serialize affected reads, and fail closed on API/version drift. |
| Resource controls | Contract and delegate WebAssembly runs with fuel and memory bounds; peers have local resource budgets. | These protect Freenet execution, not native DockPipe workloads. DockPipe must independently enforce local CPU, memory, disk, network, time, and process policy. |
| Trust and abuse | Signatures and validity predicates reject invalid state, but stale valid state, Sybil resistance, coordinated denial of service, spam, and network-wide incentives remain open concerns. | Start with trusted identities and allow-lists. Public scheduling, payments, reputation, or trustless verification are not implied. |
| Consistency fit | Freenet deliberately favors per-contract eventual convergence over global linearizability. | A Freenet claim cannot be treated as a linearizable lock. Duplicate execution under partitions must be designed out or bounded explicitly. |

