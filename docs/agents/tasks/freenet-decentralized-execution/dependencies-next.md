## Dependencies And Related Tasks

- [TASK-015 Backlog-Driven Remote Tasks And Multi-Machine Execution](../closed/backlog-driven-remote-tasks.md)
  owns the transport-neutral machine, capability snapshot, lease, event, cancellation, receipt,
  placement, and connector authority model. TASK-019 should adapt it, not invent a parallel remote
  execution contract.
- [TASK-008 ForgePipe Agentic App UI](../agentic-app-ui/overview.md) owns general DockPipe/DorkPipe run,
  approval, log, artifact, and inspection UX. A Freenet application remains a domain adapter rather
  than a second approval or execution system.
- [TASK-001 Operation Results Contract Rollout](../closed/operation-results-contract.md) owns the
  canonical local operation-result event reused inside distributed envelopes.
- [Safety guardrails](../../runtime/safety-guardrails.md), [artifacts and MCP](../../runtime/artifacts-and-mcp.md),
  and [architecture](../../core/architecture.md) remain authoritative for secrets, generated evidence,
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
