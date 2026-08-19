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

