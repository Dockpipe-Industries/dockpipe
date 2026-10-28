# TASK-053 Adaptive Threat Detection and Automatic Remote Isolation

## Status and priority

**P2; proposed, research and design only; unclaimed.** Added 2026-10-26 from a
backlog proposal and bounded source review. This is planned security architecture
work, not a finding that a supported installation is exposed. P0 remains for an
identified immediately exploitable installation requiring remediation; the
existing P1 authentication and enrollment work remains active independently.

## Objective

Design a lightweight, deterministic threat-signal and containment path for
DockPipe remote workers, broker interactions and managed execution. Explain why
signals change risk, bound their storage and processing, and allow an opted-in
worker to isolate its own DockPipe control-plane session without broker
cooperation. Preserve approved local workloads and provide a separate, auditable
recovery path.

Detection may use adaptive or statistical signals as advisory inputs. Authorization
and enforcement remain deterministic. A score must never delay or weaken a hard
authorization denial.

## Source grounding and current boundaries

This is a design gap, not a claim that the cited components already provide a
security event ledger or threat detection:

| Area | Current evidence | Design consequence |
| --- | --- | --- |
| Execution and process telemetry | `src/lib/infrastructure/telemetry/model.go` bounds snapshots to 256 executions, 4096 processes and 1 MiB; execution records omit command arguments, environment and raw output. `src/lib/infrastructure/execution_observer.go` observes execution while the caller retains supervision. | Reuse privacy-preserving observations where suitable; telemetry is a recent activity view, not a durable threat audit log or enforcement policy. Preserve omissions and explicit truncation/inaccessibility. |
| Remote monitoring | `src/lib/infrastructure/remote/monitoring.go` accepts worker opt-in snapshots, limits them to 1 MiB and 64 enrolled nodes, and stores a latest sample. Worker monitoring errors do not cancel jobs. | Monitoring is optional and broker-visible. It cannot be the only local detection or containment input, and it does not establish durable history. |
| Worker authorization and execution | `src/lib/infrastructure/remote/worker.go` validates worker identity and authority, checks assignment identity, profile approval and a private local job journal before execution. `executor.go` fixes invocation to a locally approved profile; the broker has no generic remote-shell capability. | Keep worker-side checks authoritative. Do not turn behavioral scores into permission to execute, nor let a broker supply the sole policy or disable local checks. |
| Session and cancellation behavior | `worker.go` stops execution when its broker heartbeat fails; remote enrollment revocation is enforced by broker-side authorization paths. | Broker cancellation and revocation alone do not satisfy broker-independent containment. Define a worker-local control-plane isolation boundary and behavior during network loss. |
| Authentication and workload identity | TASK-036 owns remote pairing/enrollment and worker credential lifecycle; TASK-039 owns shared gateway authentication, profiles and scoped grants; TASK-042 owns workload token verification and identity adapters. | Consume those owners' identities, revocation and grant outcomes. Do not create a competing identity, pairing, grant, or token-renewal system here. |

Canonical remote behavior remains in [Remote nodes](../../../runtime/remote-nodes.md).
The completed [execution-observability work](../closed/execution-observability.md)
and its [canonical runtime guide](../../../runtime/execution-observability.md)
are the telemetry baseline; they do not provide threat scoring or isolation.
The related backlog owners are [TASK-036](../remote-node-onboarding/index.yaml),
[TASK-039](../unified-mcp-gateway/index.yaml),
[TASK-042](../oidc-workload-federation/index.yaml).

## Proposed scope

1. Inventory available, privacy-safe signals across authentication/authorization
   failures, invalid or expired grants, rejected out-of-scope actions, remote
   session behavior and process observations. Separate signals from hard policy
   outcomes. Record unsupported or inaccessible sources as unknown.
2. Define deterministic, explainable signal aggregation with per-session,
   identity, broker and worker context where established identifiers permit.
   Bound event rates, queues, retained history, payload sizes and score state;
   deduplicate correlated events and specify decay and reset semantics.
3. Define explicit monitoring, escalation and isolation states. Thresholds and
   configuration changes require authorized, reviewable policy. A hard denial
   (invalid authorization, revoked identity, disallowed profile or scope) remains
   immediate regardless of score and cannot be overridden by a low score.
4. Specify worker-local enforcement so a broker cannot disable it or issue the
   only containment decision. The initial isolation boundary is the DockPipe
   remote control-plane session: stop accepting new broker assignments and
   invalidate that local session under policy. Do not terminate unrelated
   processes, healthy sessions, or disable the host. Define what happens to
   already-running work explicitly; no blanket workload kill is implied.
5. Define independent recovery requiring appropriate local or administrative
   reauthorization, with durable state, audit explanation, bounded retry and
   protection against repeated false-positive denial of service. Recovery must
   work when the broker is unreachable and must not silently clear a hard revoke.
6. Evaluate existing open-source tools (including Falco, Tetragon and Wazuh)
   during research for licensing, platform coverage, resource use and integration
   cost. This is an evaluation, not a dependency decision. Keep DockPipe's scope
   to its managed remote sessions and executions; this is not a general EDR,
   host-wide malware scanner or replacement for OS security controls.
7. Consider lightweight statistical baselines only as optional advisory signals.
   No external AI service, GPU, or mandatory AI package; learned outputs never
   authorize privileged work or override deterministic policy.

## Ownership boundaries

- TASK-036 owns pairing, worker enrollment, credential expiry/revocation and
  remote worker lifecycle. Add the local isolation contract there only as an
  integration point; TASK-053 owns score semantics and containment policy.
- TASK-039 owns gateway authentication and capability authorization contracts.
  TASK-042 owns federated workload identity verification. Neither identity nor
  grant semantics are duplicated here.
- Existing telemetry/process audit remains the observation source where its
  privacy and limits fit. TASK-053 defines security-specific retention,
  provenance, failure behavior and local enforcement requirements; it does not
  convert recent telemetry into forensic or tamper-proof records by assertion.
- UI, dashboard and MCP presentation consume shared status and recovery contracts
  after design. They do not own scoring, authorization or worker policy.

## Proposed phases

1. Research and threat model: map actual event producers and local worker control
   boundaries; evaluate existing tools; enumerate broker-compromise, stolen
   session, event-flood, false-positive, restart and recovery scenarios.
2. Contract design: define privacy-safe event schema, bounded state, scoring,
   hard-denial ordering, state transitions and auditable recovery. Record platform
   capability gaps before selecting implementation scope.
3. Deterministic signal prototype and simulation: use fake sources to measure
   correlation, decay, floods, threshold behavior, false positives and resource
   ceilings without enabling live automatic isolation.
4. Separately qualify worker-local enforcement and recovery across supported
   platforms before any opt-in automatic response is proposed for implementation.

## Acceptance criteria

- [ ] Threat model maps the actual Core, worker, broker and identity boundaries;
  evidence gaps are explicit and no unverified exposure is labeled P0.
- [ ] Event and state contracts explain each score transition, preserve privacy,
  identify unknown/truncated sources and define retention and resource bounds.
- [ ] Invalid, expired, revoked or out-of-scope authorization is denied immediately
  and independently of risk score; risk signals cannot grant permissions.
- [ ] Isolation is enforceable on the worker without broker cooperation, has a
  precise effect on new assignments and active work, and leaves unrelated and
  healthy workloads available.
- [ ] Recovery requires independent appropriate authorization, survives the
  documented restart/network cases and cannot clear unrelated revocation.
- [ ] Event floods, correlated duplicates, full storage, unavailable telemetry,
  false positives and interrupted recovery have defined safe outcomes.
- [ ] Tool evaluation documents platform, license, performance and integration
  evidence without silently adding a dependency or claiming EDR coverage.
- [ ] Any adaptive model remains optional and advisory; deterministic policy works
  offline and without AI packages or external services.
- [ ] Implementation proposals identify required cross-owner contracts and
  platform qualification. This planning task does not claim implementation,
  production readiness or live isolation proof.
