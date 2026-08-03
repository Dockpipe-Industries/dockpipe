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

## Proposed Architecture

### Decision Hypothesis

Do **not** name or model this as `runtime: freenet`. A DockPipe runtime answers where local work runs;
Freenet would coordinate which opted-in node receives an unchanged local task contract. It is also
not a resolver because it does not select a tool or profile that performs the work.

The provisional product shape is a **package-owned decentralized broker/coordination adapter for
`node-execution.v1`**, with a Freenet-specific contract, companion, and optional delegate. A future
name such as a `dorkpipe.freenet` package is descriptive only; naming and package placement require a
later decision. Promotion of any seam into generic DockPipe code requires evidence that it is useful
outside Freenet and cannot be composed from existing package interfaces.

```text
Freenet application / DockPipe client
  -> optional Freenet delegate: identity, signing, local consent
    -> Freenet job contract: mergeable signed coordination records
      -> local Freenet client API
        -> package-owned Freenet companion / node connector
          -> existing node-execution.v1 validation and authority boundary
            -> DockPipe local workflow execution
              -> host step | container runtime | VM runtime with QEMU resolver
```

The companion is not a generic remote shell. It subscribes only to declared contract instances,
accepts only versioned allow-listed DockPipe workflow references, validates the complete signed
chain, and submits one exact accepted request to the existing local execution boundary. It exposes
no public runner listener.

## Reuse And Boundary Map

| Existing DockPipe/DorkPipe surface | Reuse expectation |
| --- | --- |
| `node-execution.v1` machine identity | Bind one enrolled runner independently of Freenet peer address, connection, or delegate identity. |
| Immutable capability snapshot | Keep the current snapshot unchanged. A Freenet-specific signed advertisement should bind its fingerprint, issuer, and expiry; refreshing it creates a new advertisement and snapshot identity. |
| Task lease and operation ID | Reuse attempt, expiry, cancellation, and idempotency bindings, but do not assume Freenet state supplies linearizable lease arbitration. |
| Execution receipt | Bind the exact request, selected machine/capability snapshot, lease grant, local run, terminal result, artifacts, and cleanup. Exact replay returns the same receipt. |
| Operation results/events | Keep `dockpipe.operation_event.v1` unchanged inside the current `node-execution.v1` event envelope. A separate Freenet coordination record should bind that envelope's fingerprint, runner signature, and predecessor reference without changing either existing schema. |
| Artifact manifests | Preserve the current path/URL-free digest, media type, and size manifest unchanged. Put bounded retrieval, availability, encryption, and retention metadata in a separate Freenet-specific record; retrieval failure is failure, not an empty artifact. |
| Local approvals | Keep intent, placement, dispatch, execution, apply, checkpoint, publication, and any privileged host action as distinct decisions. A signature is evidence only for the exact authority it encodes. |
| Runtime-owned Git lifecycle | Prepare exact revisions and governed workspaces locally. Do not put raw Git commands, credentials, or mutable checkout authority in the Freenet adapter. |

Freenet-specific state, contracts, SDK bindings, delegate assets, and compatibility tests belong in
the eventual package. `src/lib/` and `src/cmd/` remain generic.

## Proposed Coordination State

The contract should be designed as bounded immutable record sets with deterministic merge and
materialization rules, not an append-only sequence whose correctness depends on arrival order.
Candidate record families are:

- job request and request revocation
- immutable runner capability snapshot and expiry
- claim proposal
- authoritative lease grant, renewal, expiry, and cancellation
- per-attempt event envelope and acknowledgement watermark
- terminal execution receipt and cleanup outcome
- artifact availability/retention observation
- bounded tombstone or closed-epoch record

Every record needs a schema version, record ID, job/operation ID, issuer identity, signing-key ID,
issued/expiry time where relevant, payload digest, signature, and replay identity. The contract's
validity predicate must revalidate signatures, declared bounds, immutable bindings, and allowed
state transitions that can be expressed safely under merge.

### Identity And Idempotency

- Define `operation_id` as a stable digest over the protocol version, origin identity, origin nonce,
  and finalized DockPipe request fingerprint.
- Keep Freenet contract identity, origin application identity, delegate identity, DockPipe machine
  identity, capability snapshot, lease, attempt, local run, and execution receipt distinct.
- Key local deduplication by stable operation ID and exact request fingerprint. Re-delivery of the
  same accepted operation resumes or returns the same receipt; a conflicting payload fails closed.
- Never infer machine identity from a Freenet peer address or current connection.

### Lease And Partition Rule

Freenet's convergence model does not by itself provide an exclusive distributed lock. A runner
publishing a claim is therefore not authorized to execute.

For the trusted private-swarm proof, one explicitly trusted job-authority key should publish a
signed lease grant selecting exactly one machine, capability snapshot, request, attempt, and expiry
from the visible claim set. The grant may be produced by the originating client or its delegate; it
is not a centralized hosted service. A runner must observe and validate that exact grant before
execution and must deduplicate locally by operation ID.

Partitions can still delay a cancellation or reveal competing valid-looking histories. The proof
must define a conservative stabilization/expiry rule, reject conflicting grants, and demonstrate
that uncertainty stops new execution. A later threshold or deterministic decentralized grant model
is research; it must not be claimed to provide exactly-once execution without a proof.

### Event Ordering Over Mergeable State

DockPipe's ordered per-run event semantics remain local and authoritative. Each outer event record
binds the operation, attempt, lease, sequence, previous-event digest, inner event digest, and runner
signature. Freenet may deliver these records in any order or more than once. Consumers reconstruct a
contiguous verified chain and expose gaps, forks, and stale attempts explicitly. Contract merge order
does not become execution order, and a status projection does not grant lifecycle authority.

## Delegate And Local Companion Boundary

A DockPipe Freenet delegate may be useful for:

- holding application signing and identity keys
- signing exact job requests, claims, grants, approvals, cancellations, events, and receipts within
  a declared role
- enforcing caller-specific signing policy from Freenet-attributed sender identities
- prompting for consent when an unfamiliar application, runner, permission, or workload is seen
- connecting Freenet application identity to DockPipe audit records without exporting private keys

It must not be assumed to launch a native process, open arbitrary local IPC, access the DockPipe
workspace, or act as the runner. Official documentation describes delegates as sandboxed WebAssembly
message handlers; no supported native-process bridge is established by this task.

The first proof should use a separately installed least-privilege companion that talks to the local
Freenet node through a supported authenticated client API and to DockPipe through a narrow local-only
authenticated interface. If an exact delegate-to-companion message path is required, obtain an
upstream-supported pattern or capability first. Do not bypass the delegate sandbox, reuse a browser
auth token as runner credentials, add ambient localhost trust, or weaken DockPipe approval policy.

A delegate approval and a DockPipe local execution approval may be related but remain separate
artifacts. Each must bind the exact request and scope it authorizes. Neither approval grants apply,
checkpoint, commit, push, deployment, publication, secret access, or a future job unless those
actions are independently requested and approved.

## Secrets, Source, Logs, And Artifacts

- Contract state contains no plaintext secret, credential, private key, access token, private source,
  raw environment, or resolved vault value.
- Secret references remain local and are resolved only by an already authorized runner whose policy
  explicitly allows the workflow. A reference meaningful on one runner must not silently select a
  secret on another.
- Private repositories require an explicitly trusted runner, an exact immutable revision, local
  credential policy, and a runtime-owned workspace. Public or community runners do not receive
  private source in early phases.
- Freenet should carry only bounded status, hashes, signatures, compact diagnostics, and artifact
  metadata. Large logs and artifacts belong in an external content-addressed store or an explicitly
  trusted origin/runner store.
- Artifact references bind digest, size, media type, encryption/access policy, retention class, and
  availability expectations. Retrieval always verifies bytes against the manifest.
- Confidential artifacts require end-to-end encryption for named recipients. Content hashes alone
  can leak equality and must not be treated as confidentiality.
- Retention and garbage collection require explicit ownership. Contract closure can stop new
  references but cannot prove every peer or external store deleted replicated data.

## Security And Abuse Risks

Create a formal threat model before implementation. It must cover at least:

- malicious, malformed, oversized, stale, replayed, or conflicting job records
- botnet recruitment, malware execution, spam, credential attacks, scanning, denial of service, and
  unauthorized cryptocurrency mining
- capability lying, expired capability snapshots, package/runtime version substitution, and policy
  downgrade
- duplicate execution, competing claims/grants, delayed cancellation, lease expiry, and runner loss
  during a partition
- malicious runners that forge status or results, withhold artifacts, leak inputs, or return validly
  signed but false evidence
- malicious origins that attempt sandbox escape, resource exhaustion, network exfiltration,
  persistence, privilege escalation, or lateral movement
- compromised Freenet Core, delegate, companion, DockPipe service, signing key, artifact store, or
  local approval surface
- contract-state flooding, metadata leakage, traffic analysis, stale-but-valid state, Sybil attacks,
  eclipse/routing concentration, and coordinated denial of service
- unsafe artifact parsing, decompression bombs, hash substitution, retention failure, and garbage
  collection races
- UI spoofing or consent fatigue that causes users to approve unfamiliar workloads

Required early mitigations include signed allow-listed workflow identities, exact package/runtime
versions, immutable request digests, short grants, local deduplication, strict input and state bounds,
resource quotas, network-denied-by-default profiles, sandboxed execution, process-tree teardown,
audit records, runner quarantine/revocation, safe cancellation, and explicit human approval for
unfamiliar or privileged work.

Passing validation, a valid signature, a remote attestation, or a reproducible build may improve
confidence but cannot prove arbitrary execution was honest. Phase 1 relies on trusted machines.
Trustless result verification and confidential computing are separate research problems.

## Network Policy Interaction

- A runner whose effective DockPipe policy denies network access cannot participate in live Freenet
  coordination during that execution unless policy explicitly separates the companion's control
  channel from the workload's network namespace.
- Freenet connectivity must not implicitly grant workload egress. The companion may remain connected
  while the spawned workload has no network.
- Fully offline execution can consume a previously materialized, signed, unexpired request/grant only
  if the policy defines how cancellation uncertainty is handled. Default to refusing new work when
  current authority cannot be verified.
- All network policy and effective capability checks remain local and fail closed.

## Phased Delivery Plan

### Phase 0: Feasibility, ADR, And Threat Model

1. Pin and audit current Freenet Core, `fdev`, contract/delegate interfaces, client SDK, auth model,
   state limits, persistence behavior, and supported local companion patterns.
2. Write an ADR choosing package placement, naming, contract boundaries, client API, lease authority,
   identity mapping, upgrade/version strategy, and whether any generic DockPipe seam is justified.
3. Write the threat model, abuse cases, trust assumptions, and private-swarm security profile.
4. Specify and property-test the bounded commutative contract algebra under reorder, duplication,
   loss, partitions, stale state, conflicting claims, expiry, and replay.
5. Define artifact storage/retention and source-transfer choices before any live job execution.

No live native execution is allowed in this phase.

### Phase 1: Trusted Private Swarm

- Two or more explicitly enrolled machines under one trust domain.
- One pinned harmless workflow catalog and non-sensitive source/input set.
- One trusted job-authority key grants leases; no public claiming or payment.
- Package-owned Freenet contract, local companion, and optional signing/consent delegate.
- Exact `node-execution.v1` request, capability, lease, event, receipt, cancellation, and cleanup
  bindings.
- External or origin-hosted content-addressed artifacts; only digests and metadata in Freenet.
- No centralized DockPipe control-plane service.

### Phase 2: Approved Community Runners

- Invitation-based identities and explicit trust domains.
- Per-origin and per-runner quotas, allow-lists, revocation, rate limits, audit, and retention policy.
- Reputation can inform selection but never grants local execution authority by itself.
- Private inputs remain restricted to specifically trusted runners and encrypted transfer paths.
- Admission, dispute, moderation, and incident-response processes are prerequisites.

### Phase 3: Public Compute Research

Explore public runners, compensation, reputation, staking, proof of execution, confidential
computing, and anti-abuse mechanisms only after the earlier phases produce evidence. Do not market
or implement a public marketplace from this task. Freenet's current open Sybil and network-level
abuse questions make this phase especially speculative.

## Proof-Of-Concept Acceptance Criteria

The Phase 1 vertical slice succeeds only when all of the following are demonstrated:

1. Two or more trusted machines run pinned compatible Freenet and DockPipe versions.
2. The origin publishes one signed, non-sensitive, versioned DockPipe job request through a bounded
   Freenet contract.
3. At least one runner publishes a signed immutable capability snapshot; placement distinguishes
   host OS, runtime, guest OS, toolchain/package versions, and local policy.
4. A compatible runner claims the job, but does not execute until it observes a separate valid
   signed lease grant bound to its machine and capability snapshot.
5. The runner revalidates the full immutable chain, required local approval, and effective sandbox
   capabilities before invoking an allow-listed harmless workflow.
6. The workflow compiles a sample public repository or runs a bounded test suite through an existing
   isolated DockPipe runtime. The contract or delegate executes no native workload.
7. Signed outer status records synchronize through Freenet while preserving the unchanged ordered
   DockPipe event chain and making gaps or duplicates visible.
8. The origin retrieves a terminal signed receipt, cleanup outcome, artifact manifest, and verifies
   the downloaded artifact bytes against their digest.
9. Exact request replay returns/resumes the same operation and never invokes the workload twice.
10. A runner disconnect and lease expiry are demonstrated. Reassignment uses a new attempt and grant;
    conflicting or uncertain grants fail closed rather than starting parallel work.
11. Cancellation and runner disappearance produce bounded cleanup/residue evidence and no false
    completion.
12. No plaintext secret, private repository credential, large log, raw artifact, arbitrary command,
    or unrestricted network authority enters Freenet contract state.
13. No centralized DockPipe broker/control-plane service participates; the trusted origin may still
    act as the explicit lease authority for its own job.
14. The proof runs against deterministic partition/reorder/replay fixtures before the multi-machine
    demonstration and records the pinned versions and known Freenet limitations.

## Open Technical Questions

- Can a bounded job/claim/grant/event/receipt algebra satisfy Freenet's merge requirements without
  hiding forks or violating DockPipe's ordered per-attempt event semantics?
- What stabilization and expiry rule is safe enough for the private-swarm proof, and which duplicate
  execution cases remain impossible to exclude under partition?
- Should the adapter implement a `node-execution.v1` broker interface, a new decentralized
  coordination interface, or a package-local translation between the two?
- Which companion language and official Freenet SDK/API provide a stable authenticated subscription
  and update surface for a native service?
- Can the integration work entirely through current Freenet public APIs, with no Freenet Core
  modification?
- What exact supported mechanism lets a delegate authorize a local native companion, if any, without
  exporting keys or bypassing the delegate sandbox?
- How are contract/delegate code upgrades negotiated when content-addressed code changes identity?
- How are DockPipe workflow, package, schema, runner, and runtime versions negotiated and retired?
- How are clocks, expiry, and lease safety handled when peers disagree on time?
- How are capability facts observed, signed, refreshed, revoked, and independently checked?
- Which event and artifact metadata is safe to replicate publicly without leaking repository names,
  paths, machine identities, workload purpose, or content equality?
- Which content-addressed store works for private swarms, how is availability measured, and who owns
  retention and garbage collection?
- How are private repositories transferred without giving community or public runners durable
  credentials?
- What execution evidence is realistic for builds, GPU inference, Windows/QEMU, browser automation,
  and agent workflows, and what remains only a trusted runner claim?
- How do local DockPipe network-denied/offline policies coexist with a continuously connected Freenet
  companion?
- What contract-state, delta, signature-verification, event-count, artifact-count, byte, and retention
  bounds prevent resource abuse?
- Which current Freenet alpha limitations block the proof, and which block only community/public
  phases?

## Potential Upstream Collaboration With Freenet

Before implementation, discuss or validate these points with the Freenet project:

- the recommended native companion pattern and authentication to the local Freenet node
- whether delegates are intentionally prohibited from native IPC and the safest supported way to
  connect delegate consent/signing to a separately installed service
- contract algebra review for claims, lease grants, event chains, tombstones, and bounded retention
- supported approaches for large state, state chunking, delta bounds, subscriptions, and backpressure
- client request correlation/ordering and reconnect semantics suitable for a long-running companion
- contract/delegate upgrade and identity migration strategy
- deterministic simulation and fault-injection hooks for partition, duplication, reorder, and stale
  state tests
- sender attestation, consent prompt, key rotation, revocation, and cross-device delegate roadmap
- security review of stale-valid state, Sybil/eclipsing, spam, coordinated denial of service, and
  public-runner abuse assumptions
- a possible official example showing Freenet coordinating an external governed executor without
  making Freenet contracts or delegates native compute workers

Upstream collaboration is a prerequisite for claims about delegate/native integration and a strong
input to the ADR. It is not permission to weaken either project's sandbox.

## Dependencies And Related Tasks

- [TASK-015 Backlog-Driven Remote Tasks And Multi-Machine Execution](closed/backlog-driven-remote-tasks.md)
  owns the transport-neutral machine, capability snapshot, lease, event, cancellation, receipt,
  placement, and connector authority model. TASK-019 should adapt it, not invent a parallel remote
  execution contract.
- [TASK-008 ForgePipe Agentic App UI](agentic-app-ui.md) owns general DockPipe/DorkPipe run,
  approval, log, artifact, and inspection UX. A Freenet application remains a domain adapter rather
  than a second approval or execution system.
- [TASK-001 Operation Results Contract Rollout](closed/operation-results-contract.md) owns the
  canonical local operation-result event reused inside distributed envelopes.
- [Safety guardrails](../runtime/safety-guardrails.md), [artifacts and MCP](../runtime/artifacts-and-mcp.md),
  and [architecture](../core/architecture.md) remain authoritative for secrets, generated evidence,
  and runtime/resolver/package boundaries.

## Explicit Non-Goals

- implementing any part of the integration in this backlog-only task
- describing Freenet contracts or delegates as general-purpose native compute workers
- adding a `freenet` DockPipe runtime or resolver without a later ADR and generic evidence
- modifying Freenet Core for the first proof unless current public APIs are proven insufficient and
  upstream explicitly agrees on the seam
- adding Freenet-specific behavior to `src/lib/` or `src/cmd/`
- exposing an inbound public DockPipe listener, generic remote shell, or arbitrary command field
- putting secrets, private source, credentials, large logs, or raw artifacts in public contract state
- treating a claim, connection, capability advertisement, status, signature, delegate approval,
  attestation, or artifact reference as proof of honest execution or broad lifecycle authority
- promising exactly-once execution, global ordering, linearizable leases, trustless verification,
  confidential public computation, Sybil resistance, spam resistance, or safe public economics
- public-runner payments, staking, token design, mining, marketplace operation, or production hosting
- automatic apply, checkpoint, commit, push, deployment, publication, or execution of a next job

## Required Research Deliverables Before Implementation

- current-version feasibility report with pinned Freenet Core/SDK/interface evidence
- ADR for naming, ownership, package/core boundary, state algebra, lease authority, versioning, and
  local companion/delegate communication
- formal threat model and abuse-prevention plan
- contract-state schema and merge/property-test design
- identity, signing, rotation, revocation, and approval mapping
- artifact/source storage, encryption, retention, availability, and garbage-collection design
- deterministic partition/replay/duplicate/expiry test plan
- updated Phase 1 proof plan with explicit blockers from current Freenet alpha limitations

## Official Freenet References

- [Freenet contracts](https://freenet.org/build/manual/components/contracts/)
- [Freenet delegates](https://freenet.org/build/manual/components/delegates/)
- [Official River and Freenet alpha status](https://freenet.org/river/)
- [Freenet TypeScript SDK and local WebSocket API](https://freenet.org/build/manual/typescript-sdk/)
- [Freenet contract interface](https://freenet.org/build/manual/contract-interface/)
- [Freenet contract and delegate upgrade model](https://freenet.org/build/manual/upgrading-contracts/)
- [Freenet application tutorial](https://freenet.org/build/manual/tutorial/)
- [Freenet 2026 architecture whitepaper](https://freenet.org/whitepaper/)
- [Freenet Core source](https://github.com/freenet/freenet-core)

These references were reviewed on 2026-07-28. Recheck them before starting Phase 0 because the APIs
and implementation status are actively evolving.

## Next Bounded Slice

Perform Phase 0 research only: pin current Freenet versions, prototype no native execution, specify
the mergeable signed record algebra, and draft the ADR plus threat model. Stop if the supported
delegate-to-native companion boundary cannot be established without weakening either sandbox.
